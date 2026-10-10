package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestPluginBindings(t *testing.T) {
	mygo.Use(New(Options{Directory: t.TempDir()}))
	ts, err := mygo.GenerateTypeScript()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ts), "plugin:sqlite") {
		t.Fatal("plugin leaked into generated client")
	}
}

func TestPageOwnershipAndCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("requires native library")
	}
	s := New(Options{Directory: t.TempDir()}).Service.(*service)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	id, err := s.open(ctx, 1, ":memory:", openOptions{})
	if err != nil {
		t.Fatal(err)
	}
	db, err := s.database(1, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.database(2, id); err == nil {
		t.Fatal("another window got the connection")
	}
	if err := s.close(key{2, id}); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "CREATE TABLE items(n)")
	cancel()
	if _, err := s.database(1, id); err == nil {
		t.Fatal("a canceled page retained access before asynchronous cleanup")
	}
	select {
	case <-db.life.Done():
	case <-time.After(time.Second):
		t.Fatal("page navigation did not close connection")
	}
	if _, err := db.Query(context.Background(), "SELECT 1"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	id, err = s.open(context.Background(), 1, ":memory:", openOptions{})
	if err != nil {
		t.Fatal(err)
	}
	db, _ = s.database(1, id)
	s.shutdown()
	if _, err := db.Query(context.Background(), "SELECT 1"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	if _, err := s.open(context.Background(), 1, ":memory:", openOptions{}); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestPageDatabaseNames(t *testing.T) {
	for _, name := range []string{"", ".", "..", "../a.db", "/tmp/a.db", `C:\a.db`, `dir\a.db`, "a\x00b", "CON.db", "nul", "LPT1.db", "a.db.", " a.db", "file:other.db"} {
		if err := databaseName(name); err == nil {
			t.Errorf("accepted %q", name)
		}
	}
	for _, name := range []string{"app.db", "中文.db", "company.db", "a-b.db"} {
		if err := databaseName(name); err != nil {
			t.Errorf("%q: %v", name, err)
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "link.db")
	if err := os.Symlink(filepath.Join(t.TempDir(), "outside.db"), path); err == nil {
		s := New(Options{Directory: dir}).Service.(*service)
		if _, err := s.open(context.Background(), 1, "link.db", openOptions{}); err == nil || !strings.Contains(err.Error(), "symbolic link") {
			t.Fatal(err)
		}
	}
}

func TestWireRoundTrip(t *testing.T) {
	values := []any{nil, int64(-9223372036854775808), int64(9223372036854775807), 0.125, "中文\x00", []byte{0, 255}, []byte{}, "", float64(0)}
	wire := rowsWire(Rows{Values: [][]any{values}})
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireRows
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	got, err := parameters(decoded.Rows[0])
	if err != nil {
		t.Fatal(err)
	}
	// Omitted empty BLOB fields decode as a nil []byte, still an empty BLOB.
	got[6] = []byte{}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("got %#v, want %#v", got, values)
	}
	for _, value := range []wireValue{{Type: "integer", Integer: "9223372036854775808"}, {Type: "integer"}, {Type: "oops"}} {
		if _, err := parameters([]wireValue{value}); err == nil {
			t.Fatal("invalid value accepted", value)
		}
	}
}

func TestDownloadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("library")) }))
	defer srv.Close()
	sum := sha256.Sum256([]byte("library"))
	file := nativeFile{Name: "library.so", URL: srv.URL, SHA256: hex.EncodeToString(sum[:])}
	path := filepath.Join(t.TempDir(), "cache", file.Name)
	if err := downloadLibrary(file, path); err != nil || !checkFile(path, file.SHA256) {
		t.Fatalf("download: %v", err)
	}
	file.SHA256 = strings.Repeat("0", 64)
	if err := downloadLibrary(file, path); err == nil {
		t.Fatal("bad checksum accepted")
	}
	if contents, _ := os.ReadFile(path); string(contents) != "library" {
		t.Fatal("failed download replaced the cache")
	}
}

func TestInfiniteQueryResult(t *testing.T) {
	db := testDB(t, ":memory:")
	rows := query(t, db, "SELECT 1e999, -1e999")
	if !math.IsInf(rows.Values[0][0].(float64), 1) || !math.IsInf(rows.Values[0][1].(float64), -1) {
		t.Fatal("lost SQLite's infinite REAL values", rows)
	}
	wire := rowsWire(rows)
	data, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded wireRows
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Rows[0][0].Special != "Infinity" || decoded.Rows[0][1].Special != "-Infinity" {
		t.Fatal(string(data))
	}
	if _, err := parameters(decoded.Rows[0]); err == nil {
		t.Fatal("nonfinite parameters should still be rejected")
	}
}
