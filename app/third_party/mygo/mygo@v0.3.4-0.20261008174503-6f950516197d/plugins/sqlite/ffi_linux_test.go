//go:build linux && (amd64 || arm64)

package sqlite

import (
	"os"
	"os/exec"
	"reflect"
	"testing"

	"github.com/ebitengine/purego"
)

// The webview loads the system SQLite globally before the page uses the
// plugin. Run in a fresh process so other tests cannot preload our library
// and conceal symbol interposition between the two SQLite versions.
func TestSystemSQLiteLoadedFirst(t *testing.T) {
	if testing.Short() {
		t.Skip("requires the native plugin and system SQLite libraries")
	}
	const child = "MYGO_SQLITE_TEST_SYSTEM_FIRST"
	if os.Getenv(child) != "1" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestSystemSQLiteLoadedFirst$", "-test.v")
		cmd.Env = append(os.Environ(), child+"=1")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("system SQLite loaded first: %v\n%s", err, output)
		}
		return
	}
	h, err := purego.Dlopen("libsqlite3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skipf("system SQLite is unavailable: %v", err)
	}
	t.Cleanup(func() { _ = purego.Dlclose(h) })
	db := testDB(t, ":memory:")
	execute(t, db, "CREATE TABLE items (n INTEGER, real REAL, bytes BLOB)")
	execute(t, db, "INSERT INTO items VALUES (?, ?, ?)", int64(42), 0.25, []byte{0, 255})
	rows := query(t, db, "SELECT sqlite_version(), n, real, bytes FROM items")
	want := []any{"3.53.4", int64(42), 0.25, []byte{0, 255}}
	if !reflect.DeepEqual(rows.Values[0], want) {
		t.Fatalf("query used another SQLite implementation: %#v", rows.Values[0])
	}
}
