package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func diffPix(a, b []uint8) int {
	n := 0
	for i := range a {
		if a[i] != b[i] {
			n++
		}
	}
	return n
}

func TestExperimentSmall(t *testing.T) {
	orig := volumePopoverEnterDuration
	volumePopoverEnterDuration = 10 * time.Second
	t.Cleanup(func() { volumePopoverEnterDuration = orig })

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	a.Volume = 0.5
	tst := ui.NewTester(func(c *ui.Context) { a.playerBar(c) }, 500, 120)
	tst.Frame()
	a.ShowVolume = true
	tst.Frame()
	f2 := tst.Image()
	tst.Frame()
	f3 := tst.Image()
	t.Logf("f2 vs f3 = %d", diffPix(f2.Pix, f3.Pix))
}
