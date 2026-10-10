package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestNatives puts the native libraries that a package of the app names in
// its mygo-plugin.json among the app's resources, downloaded once into
// the user's cache: a universal app gets both architectures in one file.
func TestNatives(t *testing.T) {
	if testing.Short() {
		t.Skip("runs go list")
	}
	arm := thinMachO(binary.LittleEndian, 0x19)
	files := map[string][]byte{
		"/libx-darwin-arm64.dylib": arm,
		"/libx-darwin-amd64.dylib": withCPU(arm, cpuAMD64),
		"/libx-linux-amd64.so":     []byte("\x7fELF linux"),
		"/libx-bad.so":             []byte("not what the sum says"),
	}
	downloads := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloads++
		w.Write(files[r.URL.Path])
	}))
	defer srv.Close()
	entry := func(name, path string, content []byte) string {
		sum := sha256.Sum256(content)
		return fmt.Sprintf(`{"name": %q, "url": %q, "sha256": %q}`, name, srv.URL+path, hex.EncodeToString(sum[:]))
	}
	manifest := `{"libraries": [{"name": "libx", "files": {` +
		`"darwin-arm64": ` + entry("libx.dylib", "/libx-darwin-arm64.dylib", files["/libx-darwin-arm64.dylib"]) + `,` +
		`"darwin-amd64": ` + entry("libx.dylib", "/libx-darwin-amd64.dylib", files["/libx-darwin-amd64.dylib"]) + `,` +
		`"linux-amd64": ` + entry("libx.so", "/libx-linux-amd64.so", files["/libx-linux-amd64.so"]) + `,` +
		`"linux-arm64": ` + entry("libx.so", "/libx-bad.so", []byte("the right file")) + `}}]}`
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":              "module example.com/app\n\ngo 1.27\n",
		"main.go":             "package main\n\nimport _ \"example.com/app/lib\"\n\nfunc main() {}\n",
		"lib/lib.go":          "package lib\n",
		"lib/" + pluginFile:   manifest,
		"other/other.go":      "package other\n", // not imported
		"other/" + pluginFile: `{"libraries": [{"name": "unused", "files": {}}]}`,
		"mygo.json":           `{"name": "App"}`,
	})
	cache := t.TempDir()
	switch runtime.GOOS {
	case "darwin", "ios":
		t.Setenv("HOME", cache)
		cache = filepath.Join(cache, "Library", "Caches")
	case "windows":
		t.Setenv("LocalAppData", cache)
	default:
		t.Setenv("XDG_CACHE_HOME", cache)
	}
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}

	list, err := c.natives("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].name != "libx.so" {
		t.Fatalf("natives = %+v", list)
	}
	if b, err := os.ReadFile(list[0].path); err != nil || !bytes.Equal(b, files["/libx-linux-amd64.so"]) {
		t.Errorf("the library holds %q, %v", b, err)
	}
	if !strings.HasPrefix(list[0].path, filepath.Join(cache, "mygo", "natives")+string(filepath.Separator)) {
		t.Errorf("the library is at %s, not in the cache %s", list[0].path, cache)
	}
	// It is downloaded once.
	if _, err := c.natives("linux", "amd64"); err != nil || downloads != 1 {
		t.Errorf("downloads = %d, %v", downloads, err)
	}

	natives, err := c.natives("darwin", "universal")
	if err != nil {
		t.Fatal(err)
	}
	res, err := c.resourcesWith("darwin", "universal", natives)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range res {
		if r.name == "libx.dylib" {
			found = r.lipo != ""
		}
	}
	if !found {
		t.Errorf("no universal libx.dylib among %+v", res)
	}

	if _, err := c.natives("windows", "amd64"); err == nil || !strings.Contains(err.Error(), "libx, which has no build for windows/amd64") {
		t.Errorf("a platform without a build: %v", err)
	}
	if _, err := c.natives("linux", "arm64"); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Errorf("a download with another SHA-256: %v", err)
	}
	// A cached library that changed is downloaded again.
	if err := os.WriteFile(list[0].path, []byte("changed"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := c.natives("linux", "amd64"); err != nil || downloads != 5 {
		t.Errorf("downloads = %d, %v", downloads, err)
	}
	// Names and sums that would reach outside the cache are refused.
	for _, f := range []nativeFile{
		{Name: "..", URL: srv.URL, SHA256: strings.Repeat("a", 64)},
		{Name: "lib/../../x", URL: srv.URL, SHA256: strings.Repeat("a", 64)},
		{Name: "x.so", URL: srv.URL, SHA256: strings.Repeat("../", 20) + "etc/"},
		{Name: "x.so", URL: srv.URL, SHA256: strings.Repeat("A", 64)},
	} {
		if _, err := fetchNative(f); err == nil || !strings.Contains(err.Error(), "invalid native library") {
			t.Errorf("fetchNative(%+v) = %v", f, err)
		}
	}
	// A resource of the app may not take the library's place.
	writeFiles(t, dir, map[string]string{"resources/libx.so": "mine"})
	if _, err := c.appResources("linux", "amd64"); err == nil || !strings.Contains(err.Error(), "would both be installed as libx.so") {
		t.Errorf("a resource named as the library: %v", err)
	}
}
