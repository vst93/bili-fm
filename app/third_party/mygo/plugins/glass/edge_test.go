package glass

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// edgeOver returns a tester of a window of black content 200 DIPs wide
// and 100 high, with the scroll edge e over its top 40 DIPs, or its
// bottom 40.
func edgeOver(e ScrollEdge) *ui.Tester {
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).Children(func() {
			b := ui.Box(c).Absolute().Left(0).Right(0).Height(40).Material(e)
			if e.Bottom {
				b.Bottom(0)
			} else {
				b.Top(0)
			}
		})
	}
	tt := ui.NewTester(view, 200, 100)
	tt.SetScale(2)
	return tt
}

func TestSoftScrollEdge(t *testing.T) {
	// The background fades over the content from 85% at the top to
	// nothing at the edge's bottom, linearly.
	img := edgeOver(ScrollEdge{}).Image()
	for _, c := range []struct{ y, want int }{{0, 215}, {40, 108}, {79, 3}, {90, 0}} {
		if got := pixelAt(img, 100, c.y)[0]; got < c.want-3 || got > c.want+3 {
			t.Errorf("row %d is %d, not about %d", c.y, got, c.want)
		}
	}
	// From the bottom, the other way.
	img = edgeOver(ScrollEdge{Bottom: true}).Image()
	if top, bottom := pixelAt(img, 100, 121)[0], pixelAt(img, 100, 199)[0]; top > 6 || bottom < 210 {
		t.Errorf("from the bottom, the edge goes from %d to %d", top, bottom)
	}
	// Its own background, dark.
	img = edgeOver(ScrollEdge{Background: ui.RGB(0, 0, 200)}).Image()
	if p := pixelAt(img, 100, 0); p[0] != 0 || p[2] < 165 || p[2] > 172 {
		t.Errorf("over a blue background, the top is %v", p)
	}
}

func TestHardScrollEdge(t *testing.T) {
	// Black content, toned to 0.03, under 82.45% of white: 212, evenly,
	// over a hairline of black at 10%.
	tt := edgeOver(ScrollEdge{Hard: true})
	img := tt.Image()
	for _, y := range []int{2, 40, 77} {
		if got := pixelAt(img, 100, y)[0]; got < 210 || got > 213 {
			t.Errorf("row %d is %d", y, got)
		}
	}
	if got := pixelAt(img, 100, 80)[0]; got != 0 {
		t.Errorf("the hairline over black is %d", got)
	}
	// Over white, the hairline shows: 90% of it.
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Background(ui.RGB(255, 255, 255)).Children(func() {
			ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(40).Material(ScrollEdge{Hard: true})
		})
	}
	wt := ui.NewTester(view, 200, 100)
	wt.SetScale(2)
	if got := pixelAt(wt.Image(), 100, 80)[0]; got < 228 || got > 231 {
		t.Errorf("the hairline over white is %d", got)
	}
	if got := pixelAt(wt.Image(), 100, 81)[0]; got != 255 {
		t.Errorf("below the hairline, white is %d", got)
	}
	// In dark mode, the hairline is white at 7%, over the theme's dark
	// background.
	wt.SetDark(true)
	if got := pixelAt(wt.Image(), 100, 80)[0]; got != 255 {
		t.Errorf("the dark hairline over white is %d", got)
	}
	if got := pixelAt(wt.Image(), 100, 40)[0]; got > 80 {
		t.Errorf("a dark hard edge over white is %d", got)
	}
}

func TestScrollEdgeString(t *testing.T) {
	if s := (ScrollEdge{}).String(); s != "scroll-edge soft" {
		t.Errorf("String: %q", s)
	}
	if s := (ScrollEdge{Hard: true, Bottom: true}).String(); s != "scroll-edge hard bottom" {
		t.Errorf("String: %q", s)
	}
}
