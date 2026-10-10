package update

import (
	"bytes"
	"math/rand/v2"
	"os"
	"slices"
	"testing"
)

func TestSuffixArray(t *testing.T) {
	r := rand.New(rand.NewPCG(1, 2))
	inputs := [][]byte{nil, []byte("a"), []byte("banana"), bytes.Repeat([]byte{0}, 1000), bytes.Repeat([]byte("abcab"), 300)}
	for range 50 {
		b := make([]byte, r.IntN(3000))
		alphabet := 1 + r.IntN(256)
		for i := range b {
			b[i] = byte(r.IntN(alphabet))
		}
		inputs = append(inputs, b)
	}
	for _, b := range inputs {
		want := make([]int32, len(b)+1)
		for i := range want {
			want[i] = int32(i)
		}
		slices.SortFunc(want, func(x, y int32) int { return bytes.Compare(b[x:], b[y:]) })
		if got := suffixArray(b); !slices.Equal(got, want) {
			t.Fatalf("suffix array of %q:\n got %v\nwant %v", b, got, want)
		}
	}
}

func TestDiffPatch(t *testing.T) {
	r := rand.New(rand.NewPCG(3, 4))
	random := func(n int) []byte {
		b := make([]byte, n)
		for i := range b {
			b[i] = byte(r.Uint32())
		}
		return b
	}
	base := random(200_000)
	// A new version: blocks moved, bytes changed here and there (like
	// addresses in code), some inserted and removed.
	edited := slices.Clone(base)
	for range 500 {
		edited[r.IntN(len(edited))] += byte(1 + r.IntN(3))
	}
	edited = slices.Concat(edited[100_000:150_000], random(3000), edited[:100_000], edited[160_000:])
	cases := []struct {
		name     string
		old, new []byte
	}{
		{"empty", nil, nil},
		{"from empty", nil, []byte("hello")},
		{"to empty", []byte("hello"), nil},
		{"same", base, base},
		{"edited", base, edited},
		{"unrelated", random(5000), random(7000)},
		{"zeros", make([]byte, 100_000), append(make([]byte, 50_000), 1)},
	}
	for _, c := range cases {
		patch := Diff(c.old, c.new)
		var out bytes.Buffer
		if err := Patch(&out, int64(len(c.new)), bytes.NewReader(c.old), int64(len(c.old)), bytes.NewReader(patch), int64(len(patch))); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if !bytes.Equal(out.Bytes(), c.new) {
			t.Fatalf("%s: patching made %d bytes that differ from the new %d", c.name, out.Len(), len(c.new))
		}
		if c.name == "edited" && len(patch) > len(c.new)/10 {
			t.Errorf("the patch of an edited file takes %d bytes, for %d", len(patch), len(c.new))
		}
	}
}

func FuzzPatch(f *testing.F) {
	f.Add([]byte("old file"), Diff([]byte("old file"), []byte("new file, longer")))
	f.Add([]byte("old file"), Diff([]byte("old file, longer"), []byte("new file, longer")))
	f.Add([]byte{}, []byte{0, 0})
	f.Fuzz(func(t *testing.T, old, patch []byte) {
		var out bytes.Buffer
		const size = 16
		err := Patch(&out, size, bytes.NewReader(old), int64(len(old)), bytes.NewReader(patch), int64(len(patch)))
		if out.Len() > size || err == nil && out.Len() != size {
			t.Errorf("a patch made %d bytes, not %d (%v)", out.Len(), size, err)
		}
	})
}

// BenchmarkDiff diffs the files of MYGO_DIFF_OLD and MYGO_DIFF_NEW, such
// as two builds of an app.
func BenchmarkDiff(b *testing.B) {
	oldName, newName := os.Getenv("MYGO_DIFF_OLD"), os.Getenv("MYGO_DIFF_NEW")
	if oldName == "" || newName == "" {
		b.Skip("set MYGO_DIFF_OLD and MYGO_DIFF_NEW")
	}
	old, _ := os.ReadFile(oldName)
	new, _ := os.ReadFile(newName)
	for b.Loop() {
		patch := Diff(old, new)
		b.ReportMetric(float64(len(patch)), "patch-bytes")
		b.ReportMetric(float64(len(deflate(new))), "file-bytes")
	}
}
