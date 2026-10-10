package ui

import (
	"bytes"
	"math/rand/v2"
	"runtime"
	"strings"
	"sync"
	"testing"
	"unsafe"
	"weak"
)

func checkTextTree(t *testing.T, n *textNode) textSummary {
	t.Helper()
	if n == nil {
		return textSummary{}
	}
	if n.left == nil {
		if len(n.text) > textChunkBytes+3 || n.sum != summarizeText(n.text) || n.height != 1 {
			t.Fatalf("invalid leaf %+v", n)
		}
		return n.sum
	}
	a, b := checkTextTree(t, n.left), checkTextTree(t, n.right)
	difference := n.left.height - n.right.height
	if difference < -1 || difference > 1 || n.height != 1+max(n.left.height, n.right.height) || n.sum != addText(a, b) {
		t.Fatalf("invalid AVL node %+v", n)
	}
	return n.sum
}

func TestTextBufferRandomEditsAndSnapshots(t *testing.T) {
	s := strings.Repeat("ab😀é\n日本語", 600)
	b := NewTextBuffer(s)
	original := b.Snapshot()
	rng := rand.New(rand.NewPCG(11, 71))
	alphabet := []rune("ab \n\r😀é日本語אבג")
	want := []rune(s)
	for step := range 2500 {
		a := rng.IntN(len(want) + 1)
		z := min(len(want), a+rng.IntN(12))
		var inserted []rune
		for range rng.IntN(10) {
			inserted = append(inserted, alphabet[rng.IntN(len(alphabet))])
		}
		b.Replace(a, z, string(inserted))
		want = append(want[:a:a], append(inserted, want[z:]...)...)
		view := b.Snapshot()
		if view.String() != string(want) || view.Len() != len(want) || view.UTF16Len() != UTF16Len(string(want)) || view.LineCount() != strings.Count(string(want), "\n")+1 {
			t.Fatalf("edit %d corrupted text or indexes", step)
		}
		checkTextTree(t, view.root)
		for range 3 {
			i := rng.IntN(len(want) + 1)
			units := UTF16Len(string(want[:i]))
			if view.UTF16Offset(i) != units || view.RuneOffset(units) != i {
				t.Fatalf("edit %d: native offset at %d", step, i)
			}
			line := strings.Count(string(want[:i]), "\n")
			if view.LineAt(i) != line {
				t.Fatal("line index", step, i, line, view.LineAt(i))
			}
			parts := strings.Split(string(want), "\n")
			if view.Line(line) != parts[line] {
				t.Fatal("line text", step, line)
			}
		}
	}
	if original.String() != s {
		t.Fatal("edits changed a snapshot")
	}
	b.Restore(original)
	if b.String() != s {
		t.Fatal("snapshot restore")
	}
}

func TestTextBufferNativeRangesMalformedAndZero(t *testing.T) {
	var empty TextBuffer
	if empty.Len() != 0 || empty.LineCount() != 1 || empty.Slice(-10, 100) != "" {
		t.Fatal("zero buffer")
	}
	empty.Replace(0, 0, "A😀\n\xfftail")
	actual := empty.ReplaceUTF16(TextInputRange{Start: 2, End: 3}, "é")
	if actual != (TextInputRange{Start: 1, End: 3}) || empty.String() != "Aé\n\xfftail" {
		t.Fatal("surrogate range", actual, empty.String())
	}
	doc, r := empty.TextForRange(TextInputRange{Start: 0, End: 100})
	if doc != "Aé\n�tail" || r.End != 8 {
		t.Fatal("malformed native query", doc, r)
	}
	var out bytes.Buffer
	written, err := empty.WriteTo(&out)
	if err != nil || written != int64(empty.ByteLen()) || out.String() != empty.String() {
		t.Fatal("streamed export", written, err)
	}
}

func bufferOwningInput() (*TextBuffer, weak.Pointer[byte]) {
	s := strings.Repeat("a", 1<<20)
	old := weak.Make(unsafe.StringData(s))
	return NewTextBuffer(s), old
}

func TestTextBufferOwnsBoundedChunks(t *testing.T) {
	b, old := bufferOwningInput()
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("chunks pinned the original input allocation")
	}
	b.Replace(1, b.Len()-1, "")
	if b.String() != "aa" {
		t.Fatal(b.String())
	}
	checkTextTree(t, b.Snapshot().root)
	runtime.KeepAlive(b)
}

func TestTextBufferConcurrentSnapshots(t *testing.T) {
	b := NewTextBuffer(strings.Repeat("a", 10000))
	var workers sync.WaitGroup
	for worker := range 5 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 100 {
				if worker == 0 {
					b.Replace(0, 1, "b")
				} else {
					view := b.Snapshot()
					if view.Len() != 10000 || len(view.String()) != 10000 {
						t.Error("inconsistent concurrent snapshot")
					}
				}
			}
		}()
	}
	workers.Wait()
}
