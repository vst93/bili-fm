package update

import (
	"bytes"
	"io/fs"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

// writeTree creates files (path → content; "->target" makes a link, a
// trailing slash a directory) in a new directory.
func writeTree(t testing.TB, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		var err error
		switch {
		case strings.HasSuffix(name, "/"):
			err = os.MkdirAll(p, 0o755)
		case strings.HasPrefix(content, "->"):
			err = os.Symlink(content[2:], p)
		default:
			mode := fs.FileMode(0o644)
			if strings.HasSuffix(name, ".exe") {
				mode = 0o755
			}
			err = os.WriteFile(p, []byte(content), mode)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// readTree lists what dir holds, as writeTree takes it, with modes.
func readTree(t testing.TB, dir string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return err
		}
		rel := relPath(dir, p)
		info, _ := d.Info()
		switch {
		case d.IsDir():
			tree[rel+"/"] = ""
		case d.Type()&fs.ModeSymlink != 0:
			link, _ := os.Readlink(p)
			tree[rel] = "->" + link
		default:
			b, _ := os.ReadFile(p)
			tree[rel] = string(b)
			if runtime.GOOS != "windows" && info.Mode().Perm()&0o100 != 0 {
				tree[rel] += " (executable)"
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

func TestDelta(t *testing.T) {
	r := rand.New(rand.NewPCG(5, 6))
	code := make([]byte, 100_000)
	for i := range code {
		code[i] = byte(r.Uint32())
	}
	newCode := slices.Clone(code)
	for range 100 {
		newCode[r.IntN(len(newCode))]++
	}
	oldApp := map[string]string{
		"app.exe":           string(code),
		"res/same.txt":      "unchanged",
		"res/moved.txt":     "moves to another directory",
		"res/removed.txt":   "gone in the new version",
		"res/small.txt":     "short",
		"res/empty/":        "",
		"lib/v1/lib.so":     "library",
		"lib/current":       "->v1",
		"res/becomes-a-dir": "file",
	}
	newApp := map[string]string{
		"app.exe":                string(newCode),
		"res/same.txt":           "unchanged",
		"res/other/moved.txt":    "moves to another directory",
		"res/small.txt":          "changed",
		"res/added.txt":          "new in this version",
		"res/empty/":             "",
		"res/new-empty/":         "",
		"res/empty-file":         "",
		"lib/v1/lib.so":          "library",
		"lib/current":            "->v1",
		"res/becomes-a-dir/file": "file",
	}
	if runtime.GOOS == "windows" {
		// Creating links needs privileges.
		delete(oldApp, "lib/current")
		delete(newApp, "lib/current")
	}
	oldDir, newDir := writeTree(t, oldApp), writeTree(t, newApp)

	var delta bytes.Buffer
	if err := WriteDelta(&delta, "1.0.0", "1.1.0", oldDir, newDir, nil); err != nil {
		t.Fatal(err)
	}
	if delta.Len() > 5000 {
		t.Errorf("the delta takes %d bytes", delta.Len())
	}
	apply := func(d []byte, from, to, oldDir string) (string, error) {
		out := filepath.Join(t.TempDir(), "app")
		return out, ApplyDelta(bytes.NewReader(d), int64(len(d)), from, to, oldDir, out)
	}
	out, err := apply(delta.Bytes(), "1.0.0", "1.1.0", oldDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := readTree(t, out), readTree(t, newDir); !maps.Equal(got, want) {
		t.Errorf("applying the delta made\n%v\nwant\n%v", slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)))
	}

	// Only the entries named.
	delta.Reset()
	if err := WriteDelta(&delta, "1.0.0", "1.1.0", oldDir, newDir, []string{"app.exe"}); err != nil {
		t.Fatal(err)
	}
	if out, err := apply(delta.Bytes(), "1.0.0", "1.1.0", oldDir); err != nil {
		t.Fatal(err)
	} else if got := slices.Sorted(maps.Keys(readTree(t, out))); !slices.Equal(got, []string{"app.exe"}) {
		t.Errorf("entries made: %v", got)
	}

	// Another app than the delta was made from, another version, and a
	// damaged delta all fail.
	changed := writeTree(t, oldApp)
	os.WriteFile(filepath.Join(changed, "app.exe"), newCode[:5000], 0o755)
	if _, err := apply(delta.Bytes(), "1.0.0", "1.1.0", changed); err == nil || !strings.Contains(err.Error(), "app.exe wrong") {
		t.Errorf("applied to another app: %v", err)
	}
	if _, err := apply(delta.Bytes(), "0.9.0", "1.1.0", oldDir); err == nil || !strings.Contains(err.Error(), "not 0.9.0 to 1.1.0") {
		t.Errorf("applied to another version: %v", err)
	}
	for _, n := range []int{0, 10, delta.Len() / 2, delta.Len() - 1} {
		if _, err := apply(delta.Bytes()[:n], "1.0.0", "1.1.0", oldDir); err == nil {
			t.Errorf("a delta cut at %d bytes applied", n)
		}
	}
	damaged := slices.Clone(delta.Bytes())
	damaged[len(damaged)-10] ^= 0xff
	if _, err := apply(damaged, "1.0.0", "1.1.0", oldDir); err == nil {
		t.Error("a damaged delta applied")
	}
}

func FuzzApplyDelta(f *testing.F) {
	old := map[string]string{"a": "old a", "b/": "", "b/c": "old c"}
	var delta bytes.Buffer
	if err := WriteDelta(&delta, "1", "2", writeTree(f, old), writeTree(f, map[string]string{"a": "new a", "b/c": "old c", "d": "d"}), nil); err != nil {
		f.Fatal(err)
	}
	f.Add(delta.Bytes())
	f.Fuzz(func(t *testing.T, d []byte) {
		oldDir := writeTree(t, old)
		out := filepath.Join(t.TempDir(), "app")
		if ApplyDelta(bytes.NewReader(d), int64(len(d)), "1", "2", oldDir, out) == nil {
			// Whatever it made is inside out.
			if got := readTree(t, oldDir); !maps.Equal(got, old) {
				t.Errorf("the old app changed: %v", got)
			}
		}
	})
}
