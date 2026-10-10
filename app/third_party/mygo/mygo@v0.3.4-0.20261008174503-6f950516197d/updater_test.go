package mygo

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/rand/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/update"
)

// updateServer serves a signed update whose app holds main with content,
// and with deltaFrom, a delta of it from that version of installedApp.
// requests counts the requests of each file.
func updateServer(t *testing.T, version, content, deltaFrom string) (srv *httptest.Server, key string, m update.Manifest, requests func(string) int) {
	t.Helper()
	pub, priv, _ := ed25519.GenerateKey(nil)
	src := t.TempDir()
	app := "app"
	if runtime.GOOS == "darwin" {
		app = "Test.app"
	}
	main := filepath.Join(src, app, "main")
	if err := os.MkdirAll(filepath.Dir(main), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, app, "resource"), []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, app, "code"), newCode, 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []string{app}
	if runtime.GOOS != "darwin" {
		// Elsewhere the archive holds the files of the app directory.
		src, entries = filepath.Join(src, app), []string{"code", "main", "resource"}
	}
	var archive bytes.Buffer
	if err := update.WriteArchive(&archive, src, entries); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(archive.Bytes())
	m = update.Manifest{Version: version, Notes: "- Faster", Date: "2026-09-27T10:00:00Z", Size: int64(archive.Len()), Signature: update.Sign(priv, sum[:])}
	var mu sync.Mutex
	counts := map[string]int{}
	mux := http.NewServeMux()
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		counts[r.URL.Path]++
		mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	m.URL = srv.URL + "/app.tar.gz"
	mux.HandleFunc("/app.tar.gz", func(w http.ResponseWriter, r *http.Request) { w.Write(archive.Bytes()) })
	if deltaFrom != "" {
		old, _ := installedApp(t)
		var delta bytes.Buffer
		if err := update.WriteDelta(&delta, deltaFrom, version, old, filepath.Join(src, entries[0]), nil); runtime.GOOS != "darwin" {
			delta.Reset()
			err = update.WriteDelta(&delta, deltaFrom, version, old, src, entries)
		} else if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(delta.Bytes())
		m.Deltas = []update.Delta{{From: deltaFrom, URL: srv.URL + "/app.delta", Size: int64(delta.Len()), Signature: update.Sign(priv, sum[:])}}
		mux.HandleFunc("/app.delta", func(w http.ResponseWriter, r *http.Request) { w.Write(delta.Bytes()) })
	}
	mux.HandleFunc("/update.json", func(w http.ResponseWriter, r *http.Request) { json.NewEncoder(w).Encode(m) })
	return srv, base64.StdEncoding.EncodeToString(pub), m, func(path string) int {
		mu.Lock()
		defer mu.Unlock()
		return counts[path]
	}
}

// installedApp creates what an update replaces, holding main with "v1".
func installedApp(t *testing.T) (target, main string) {
	t.Helper()
	dir := t.TempDir()
	target = dir
	if runtime.GOOS == "darwin" {
		target = filepath.Join(dir, "My App.app")
	}
	main = filepath.Join(target, "main")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(main, []byte("v1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "resource"), []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "code"), oldCode, 0o644); err != nil {
		t.Fatal(err)
	}
	return target, main
}

// oldCode is a file of installedApp, which newCode, of the update, changes
// a little: a delta patches it.
var oldCode, newCode = func() ([]byte, []byte) {
	r := rand.New(rand.NewPCG(1, 2))
	old := make([]byte, 64<<10)
	for i := range old {
		old[i] = byte(r.Uint32())
	}
	new := bytes.Clone(old)
	for range 20 {
		new[r.IntN(len(new))]++
	}
	return old, new
}()

func TestUpdater(t *testing.T) {
	ctx := context.Background()
	srv, key, m, _ := updateServer(t, "1.2.0", "v2", "")

	if _, err := Updater.Check(ctx); err != ErrUpdatesDisabled {
		t.Errorf("Check without updates: %v", err)
	}
	up, err := checkUpdate(ctx, srv.URL+"/update.json", key, "1.1.9")
	if err != nil || up == nil || up.Version != "1.2.0" || up.Notes != "- Faster" || up.Date.Year() != 2026 {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	for _, current := range []string{"1.2.0", "1.10.0"} {
		if up, err := checkUpdate(ctx, srv.URL+"/update.json", key, current); up != nil || err != nil {
			t.Errorf("checkUpdate from %s = %+v, %v", current, up, err)
		}
	}
	if _, err := checkUpdate(ctx, "http://example.com/update.json", key, "1.0.0"); err == nil || !strings.Contains(err.Error(), "HTTPS") {
		t.Errorf("plain HTTP: %v", err)
	}

	target, main := installedApp(t)
	var progress [2]int64
	err = installUpdate(ctx, up.manifest, key, up.current, target, func(n, total int64) { progress = [2]int64{n, total} })
	if err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the update main = %q", b)
	}
	if progress != [2]int64{m.Size, m.Size} {
		t.Errorf("progress = %v, want %d", progress, m.Size)
	}
	if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".*")); runtime.GOOS == "darwin" && len(leftovers) > 0 {
		t.Errorf("left behind: %q", leftovers)
	}

	// A tampered archive, or one signed with another key, is refused and
	// changes nothing.
	target, main = installedApp(t)
	bad := up.manifest
	bad.Signature = base64.StdEncoding.EncodeToString(make([]byte, 64))
	if err := installUpdate(ctx, bad, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "signed") {
		t.Errorf("bad signature: %v", err)
	}
	otherKey, _, _ := ed25519.GenerateKey(nil)
	if err := installUpdate(ctx, up.manifest, base64.StdEncoding.EncodeToString(otherKey), up.current, target, nil); err == nil {
		t.Error("an update signed with another key was installed")
	}
	short := up.manifest
	short.Size++
	if err := installUpdate(ctx, short, key, up.current, target, nil); err == nil || !strings.Contains(err.Error(), "bytes") {
		t.Errorf("wrong size: %v", err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v1" {
		t.Errorf("a refused update changed main to %q", b)
	}
}

func TestUpdaterDelta(t *testing.T) {
	ctx := context.Background()
	srv, key, m, requests := updateServer(t, "1.2.0", "v2, a bigger version", "1.1.9")
	up, err := checkUpdate(ctx, srv.URL+"/update.json", key, "1.1.9")
	if err != nil || len(up.manifest.Deltas) != 1 {
		t.Fatalf("checkUpdate = %+v, %v", up, err)
	}
	install := func(m update.Manifest, current string, corrupt bool) (progress [][2]int64) {
		t.Helper()
		target, main := installedApp(t)
		if corrupt {
			os.WriteFile(filepath.Join(target, "code"), newCode[:1000], 0o644)
		}
		err := installUpdate(ctx, m, key, current, target, func(n, total int64) { progress = append(progress, [2]int64{n, total}) })
		if err != nil {
			t.Fatal(err)
		}
		if b, _ := os.ReadFile(main); string(b) != "v2, a bigger version" {
			t.Errorf("after the update main = %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(target, "resource")); string(b) != "unchanged" {
			t.Errorf("after the update resource = %q", b)
		}
		if b, _ := os.ReadFile(filepath.Join(target, "code")); !bytes.Equal(b, newCode) {
			t.Error("after the update code is not the new one")
		}
		if leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(target), ".*")); runtime.GOOS == "darwin" && len(leftovers) > 0 {
			t.Errorf("left behind: %q", leftovers)
		}
		return progress
	}
	delta := m.Deltas[0]

	// The delta of the running version is all it downloads.
	if delta.Size > m.Size/10 {
		t.Errorf("the delta takes %d bytes, the archive %d", delta.Size, m.Size)
	}
	if progress := install(up.manifest, up.current, false); progress[len(progress)-1] != [2]int64{delta.Size, delta.Size} {
		t.Errorf("progress = %v, want the delta's %d bytes", progress, delta.Size)
	}
	if requests("/app.delta") != 1 || requests("/app.tar.gz") != 0 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}

	// An app that the delta was not made from gets the archive, and so do
	// other versions and deltas that are not signed.
	if progress := install(up.manifest, up.current, true); progress[len(progress)-1] != [2]int64{m.Size, m.Size} {
		t.Errorf("progress = %v, want the archive's %d bytes last", progress, m.Size)
	}
	if requests("/app.delta") != 2 || requests("/app.tar.gz") != 1 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
	install(up.manifest, "1.1.8", false)
	if requests("/app.delta") != 2 || requests("/app.tar.gz") != 2 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
	unsigned := up.manifest
	unsigned.Deltas = []update.Delta{delta}
	unsigned.Deltas[0].Signature = m.Signature
	install(unsigned, up.current, false)
	if requests("/app.delta") != 3 || requests("/app.tar.gz") != 3 {
		t.Errorf("downloaded the delta %d times and the archive %d", requests("/app.delta"), requests("/app.tar.gz"))
	}
}

func TestUpdaterWaitsForOpenFiles(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows keeps open files from being renamed")
	}
	ctx := context.Background()
	srv, key, _, _ := updateServer(t, "1.2.0", "v2", "")
	up, err := checkUpdate(ctx, srv.URL+"/update.json", key, "1.1.9")
	if err != nil {
		t.Fatal(err)
	}
	target, main := installedApp(t)
	// Open main as antivirus software does to scan it: Go opens files
	// without sharing their deletion, which renaming needs.
	f, err := os.Open(main)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	time.AfterFunc(500*time.Millisecond, func() { f.Close() })
	if err := installUpdate(ctx, up.manifest, key, up.current, target, nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(main); string(b) != "v2" {
		t.Errorf("after the update main = %q", b)
	}
}
