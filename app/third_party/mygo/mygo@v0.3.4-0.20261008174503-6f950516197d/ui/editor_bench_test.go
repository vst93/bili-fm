package ui

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// codeLines returns n lines of code, as a text area of a code editor or a
// log viewer holds.
func codeLines(n int) string {
	var b strings.Builder
	for i := range n {
		fmt.Fprintf(&b, "\tif err := step%d(ctx, items[%d]); err != nil { return err } // line %d\n", i%97, i%13, i)
	}
	return b.String()
}

// textAreaTester shows a text area of lines lines filling a 1200×800
// window, with the keyboard focus and the caret in its first line.
func textAreaTester(lines int) (*Tester, *string) {
	s := codeLines(lines)
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 1200, 800)
	tt.Press(40, 20)
	tt.Release(40, 20)
	return tt, &s
}

// benchLines are the sizes of the texts of the benchmarks, in lines.
var benchLines = []int{1000, 10000, 100000, 1000000}

// BenchmarkEditorRetainedDeletion measures live Go heap after GC, separately
// from allocation churn. Keep an 8 MiB document and twenty small undo steps.
// Run with -benchtime=1x -count=3; retained-B is the heap above the baseline.
func BenchmarkEditorRetainedDeletion(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		runtime.GC()
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		ed := newEditor()
		ed.multiline = true
		ed.buf.set(strings.Repeat("a", 8<<20))
		for range 20 {
			ed.deleteRange(10, 11)
		}
		runtime.GC()
		runtime.ReadMemStats(&after)
		retained := int64(after.HeapAlloc) - int64(before.HeapAlloc)
		b.ReportMetric(float64(max(retained, 0)), "retained-B")
		runtime.KeepAlive(ed)
	}
}

func BenchmarkTextBufferNativeOffsets(b *testing.B) {
	var buf buffer
	buf.set(codeLines(1000000) + "😀")
	index := buf.n - 1
	b.ReportAllocs()
	for b.Loop() {
		units := buf.utf16At(index)
		if buf.runeAtUTF16(units) != index {
			b.Fatal("native offset mismatch")
		}
	}
}

// BenchmarkTextBufferOpen indexes a code document's paragraph boundaries,
// before layout, to measure the work that scales with the entire file.
func BenchmarkTextBufferOpen(b *testing.B) {
	for _, n := range benchLines {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := codeLines(n)
			var buf buffer
			buf.set(s)
			b.ReportAllocs()
			b.SetBytes(int64(len(s)))
			for b.Loop() {
				buf.set(s)
			}
		})
	}
}

// BenchmarkTextAreaType types a letter into a text area of many lines: a
// frame that edits the text, lays it out and paints it.
func BenchmarkTextAreaType(b *testing.B) {
	for _, n := range benchLines {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			tt, _ := textAreaTester(n)
			for b.Loop() {
				tt.Type("a")
			}
		})
	}
}

// BenchmarkTextAreaSteady draws frames of a text area of many lines in
// which nothing changed, as the caret blinking asks for.
func BenchmarkTextAreaSteady(b *testing.B) {
	for _, n := range benchLines {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			tt, _ := textAreaTester(n)
			for b.Loop() {
				tt.Frame()
			}
		})
	}
}

// BenchmarkTextAreaOpen gives a text area a new text of many lines, as an
// app opening a file does, and draws the frame showing it.
func BenchmarkTextAreaOpen(b *testing.B) {
	for _, n := range benchLines {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			tt, s := textAreaTester(n)
			texts := [2]string{codeLines(n) + "// the end", *s}
			i := 0
			for b.Loop() {
				*s = texts[i%2]
				i++
				tt.Frame()
			}
		})
	}
}
