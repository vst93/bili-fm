package ui_test

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/egoist/mygo/ui"
)

func BenchmarkValueFrame(b *testing.B) {
	labels := make([]string, 1000)
	for i := range labels {
		labels[i] = fmt.Sprintf("Item %d", i)
	}
	tt := ui.NewTester(func(f *ui.Context) {
		ui.Column(f).Fill().Children(func() {
			for _, label := range labels {
				ui.Text(f, label).FontSize(12).Padding(1, 2)
			}
		})
	}, 300, 600)
	for range 10 {
		tt.Frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}

func TestValueFrameRetainedHeap(t *testing.T) {
	labels := make([]string, 1000)
	for i := range labels {
		labels[i] = fmt.Sprintf("Item %d", i)
	}
	view := func(f *ui.Context) {
		ui.Column(f).Fill().Children(func() {
			for _, label := range labels {
				ui.Text(f, label).FontSize(12).Padding(1, 2)
			}
		})
	}
	warm := ui.NewTester(view, 300, 600)
	for range 10 {
		warm.Frame()
	}
	runtime.KeepAlive(warm)
	warm = nil
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	tt := ui.NewTester(view, 300, 600)
	for range 10 {
		tt.Frame()
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("retained Go heap for the warmed 1000-label view: %d bytes", int64(after.HeapAlloc)-int64(before.HeapAlloc))
	runtime.KeepAlive(tt)
}

func BenchmarkValueList(b *testing.B) {
	var list ui.ListState
	tt := ui.NewTester(func(f *ui.Context) {
		ui.List(f.Key("items"), &list, 1_000_000).Grow(1).
			Rows(func(row ui.ListRow) { ui.Textf(row.Context, "Row %d", row.Index).Height(24) })
	}, 300, 400)
	for range 10 {
		tt.Frame()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tt.Frame()
	}
}
