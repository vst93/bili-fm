package ui

import (
	"fmt"
	"strings"
	"testing"
)

// Compare edit allocation for two document sizes; allocation should follow
// chunk size and tree depth rather than document byte length.
func BenchmarkTextBufferReplace(b *testing.B) {
	for _, size := range []int{1 << 20, 8 << 20} {
		b.Run(fmt.Sprint(size), func(b *testing.B) {
			doc := NewTextBuffer(strings.Repeat("a", size))
			b.ReportAllocs()
			for b.Loop() {
				doc.Replace(size/2, size/2+1, "b")
			}
		})
	}
}

func BenchmarkTextAreaBufferType(b *testing.B) {
	for _, lines := range []int{10000, 100000} {
		b.Run(fmt.Sprint(lines), func(b *testing.B) {
			doc := NewTextBuffer(codeLines(lines))
			tt := bufferTester(doc, true)
			ed := tt.rt.states[tt.rt.focused].editor
			ed.move(10, false)
			tt.Frame()
			b.ReportAllocs()
			for b.Loop() {
				tt.Type("a")
			}
		})
	}
}

func TestTextBufferEditAllocationIsBounded(t *testing.T) {
	doc := NewTextBuffer(strings.Repeat("a", 8<<20))
	allocated := testing.AllocsPerRun(20, func() { doc.Replace(100, 101, "b") })
	if allocated > 100 {
		t.Fatalf("small indexed edit allocates %.0f objects", allocated)
	}
	if doc.ByteLen() != 8<<20 || doc.Slice(99, 102) != "aba" {
		t.Fatal("small edit corrupted document")
	}
}
