package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestScratchTimeGeom(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Pos = 65
	a.Dur = 3725
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	for _, s := range []string{"1:05", "1:02:05"} {
		r, ok := tst.Find(s)
		t.Logf("time %q: ok=%v rect=%+v center=%.1f", s, ok, r, r.X+r.W/2)
	}
	// dump all labels and rects
	for _, s := range tst.Texts() {
		r, _ := tst.Find(s)
		t.Logf("label %q rect=%+v", s, r)
	}
}
