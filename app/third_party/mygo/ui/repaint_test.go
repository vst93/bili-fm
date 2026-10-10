package ui

import (
	"bytes"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// repaintTester runs view as a window does, with the frames the surface
// gives (engine.surfaceFrame), at a clock the test sets.
type repaintTester struct {
	*Tester
	clock  time.Time
	builds int
}

func newRepaintTester(view func(c *context)) *repaintTester {
	rt := &repaintTester{clock: time.Unix(1000, 0)}
	rt.Tester = coreNewTester(func(c *context) {
		rt.builds++
		view(c)
	}, 200, 100)
	rt.rt.clock = func() time.Time { return rt.clock }
	rt.Frame()
	return rt
}

// frame draws the frame the surface asks for after d, and reports whether
// the view built it and whether it asked for another.
func (t *repaintTester) frame(d time.Duration) (built, more bool) {
	t.clock = t.clock.Add(d)
	n := t.builds
	t.h.requested.Store(false)
	t.rt.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame})
	return t.builds > n, t.h.requested.Load()
}

func TestRepaintFrames(t *testing.T) {
	blue := RGB(0, 0, 255)
	draws := 0
	var drawn time.Time
	tt := newRepaintTester(func(c *context) {
		coreBox(c).Size(100, 20).Draw(func(p *Painter, r Rect) {
			// A bar moving a DIP a millisecond.
			draws++
			drawn = p.Now()
			p.AnimationFrame()
			p.Fill(Rect{r.X + float32(p.Now().UnixMilli()%50), r.Y, 2, r.H}, blue, 0)
		})
	})
	at := func(x int) bool {
		r, g, b, _ := tt.Image().At(x, 10).RGBA()
		return r == 0 && g == 0 && b == 0xffff
	}
	// Frames paint the bar again, where the time puts it, without building
	// the view.
	for i, x := range []int{8, 16} {
		n := draws
		built, more := tt.frame(8 * time.Millisecond)
		if built || !more || draws != n+1 || !drawn.Equal(tt.clock) || !at(x) || at(x-8) {
			t.Fatalf("frame %d: built %v, asked for another %v, painted %d times at %v, bar at %d: %v, at %d: %v",
				i, built, more, draws-n, drawn.Sub(tt.clock), x, at(x), x-8, at(x-8))
		}
	}
	// Anything else that may change what the view builds builds it anew.
	for _, change := range []struct {
		what string
		do   func()
	}{
		{"an event", func() { tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerMove, X: 150, Y: 50}) }},
		{"a frame asked for", func() { tt.rt.requestFrame() }},
		{"a change of the app's state", func() { tt.rt.changed() }},
		{"a change of the appearance", func() { tt.rt.themeChanged() }},
		{"the text system forgetting its layouts", func() { tt.rt.gen-- }},
		{"another size", func() { tt.h.w = 210 }},
	} {
		change.do()
		if built, _ := tt.frame(8 * time.Millisecond); !built {
			t.Errorf("after %s, the frame did not build the view", change.what)
		}
		if built, _ := tt.frame(8 * time.Millisecond); built {
			t.Errorf("after %s, the next frame built the view", change.what)
		}
	}
}

func TestRepaintOnlyInView(t *testing.T) {
	draws := 0
	tt := newRepaintTester(func(c *context) {
		coreScroll(c).Height(100).Children(func() {
			coreBox(c).Height(300).Shrink(0)
			coreBox(c).Size(100, 20).Shrink(0).Draw(func(p *Painter, r Rect) {
				draws++
				p.AnimationFrame()
			})
		})
	})
	if draws != 0 || tt.rt.redraw || tt.h.requested.Load() {
		t.Errorf("out of view: painted %d times, asks to be painted again %v, asks for a frame %v", draws, tt.rt.redraw, tt.h.requested.Load())
	}
}

func TestPainterAfter(t *testing.T) {
	draws := 0
	tt := newRepaintTester(func(c *context) {
		coreBox(c).Size(100, 20).Draw(func(p *Painter, r Rect) {
			draws++
			p.After(30 * time.Millisecond)
		})
	})
	due := tt.clock.Add(30 * time.Millisecond)
	if !tt.rt.redraw || !tt.rt.repaintDue.Equal(due) || tt.h.requested.Load() {
		t.Fatalf("asks to be painted again %v at %v, for a frame now %v", tt.rt.redraw, tt.rt.repaintDue.Sub(tt.clock), tt.h.requested.Load())
	}
	// The frame comes when it is due.
	tt.clock = due.Add(-10 * time.Millisecond)
	tt.rt.repaintNow()
	if tt.h.requested.Load() {
		t.Fatal("a frame asked for before it is due")
	}
	tt.clock = due
	tt.rt.repaintNow()
	if !tt.h.requested.Load() {
		t.Fatal("no frame asked for once due")
	}
	n := draws
	if built, more := tt.frame(time.Millisecond); built || more || draws != n+1 || !tt.rt.repaintDue.Equal(tt.clock.Add(30*time.Millisecond)) {
		t.Errorf("the frame: built %v, asked for another now %v, painted %d times, next due in %v", built, more, draws-n, tt.rt.repaintDue.Sub(tt.clock))
	}
	// A frame built since replaces the one due.
	due = tt.rt.repaintDue
	tt.rt.changed()
	tt.clock = due
	tt.h.requested.Store(false)
	tt.rt.repaintNow()
	if tt.h.requested.Load() {
		t.Error("a frame asked for though one building the view comes")
	}
	if built, _ := tt.frame(time.Millisecond); !built {
		t.Fatal("the frame asked for did not build the view")
	}
	// An event that asks for no frame, as the pointer moving over an
	// element that does not look at it, leaves the frame due: it comes,
	// and builds the view anew (issue 149).
	due = tt.rt.repaintDue
	tt.h.requested.Store(false)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerMove, X: 50, Y: 10})
	if tt.h.requested.Load() {
		t.Fatal("the pointer moving asked for a frame")
	}
	tt.clock = due
	tt.rt.repaintNow()
	if !tt.h.requested.Load() {
		t.Fatal("no frame asked for once due, after the pointer moved")
	}
	n = draws
	if built, _ := tt.frame(time.Millisecond); !built || draws != n+1 || !tt.rt.repaintDue.Equal(tt.clock.Add(30*time.Millisecond)) {
		t.Errorf("the frame after the pointer moved: built %v, painted %d times, next due in %v", built, draws-n, tt.rt.repaintDue.Sub(tt.clock))
	}
}

func TestIndicatorsRepaint(t *testing.T) {
	var value float64
	tt := newRepaintTester(func(c *context) {
		coreColumn(c).Gap(10).Padding(10).Children(func() {
			coreSpinner(c)
			coreProgress(c, value).Width(100)
		})
	})
	// The spinner is painted again for its next spoke, the progress bar of
	// unknown length as soon as the display can show it.
	if !tt.rt.redraw || tt.rt.animating || tt.rt.repaintDue.IsZero() || tt.rt.repaintDue.Sub(tt.clock) > 75*time.Millisecond {
		t.Errorf("the spinner alone: asks to be painted again %v (at %v), keeps building frames %v", tt.rt.redraw, tt.rt.repaintDue.Sub(tt.clock), tt.rt.animating)
	}
	value = -1
	tt.Frame()
	if !tt.rt.redraw || tt.rt.animating || !tt.h.requested.Load() {
		t.Errorf("with a moving progress bar: asks to be painted again %v, keeps building frames %v, asks for a frame %v", tt.rt.redraw, tt.rt.animating, tt.h.requested.Load())
	}
	before := tt.Image()
	if built, _ := tt.frame(400 * time.Millisecond); built || bytes.Equal(before.Pix, tt.Image().Pix) {
		t.Errorf("400 ms later, the frame built the view %v, or shows the same", built)
	}
}

// TestHeldWhileOccluded checks that what moves draws no frames while
// nothing of the window shows, and goes on once some of it shows again,
// while changes of the state still build frames.
func TestHeldWhileOccluded(t *testing.T) {
	count := 0
	tt := newRepaintTester(func(c *context) {
		coreTextf(c, "%d", count)
		coreProgress(c, -1).Width(100)
		coreBox(c).Size(10, 10).Draw(func(p *Painter, r Rect) { p.After(50 * time.Millisecond) })
		spin := coreBox(c).Size(10, 10)
		spin.Rotate(spin.Loop("spin", time.Second, Linear) * 360)
	})
	// The frame the view animating asked for comes, and asks for no more.
	tt.h.hidden = true
	if _, more := tt.frame(8 * time.Millisecond); more || !tt.rt.held || !tt.rt.repaintDue.IsZero() {
		t.Fatalf("out of sight: asked for another %v, held %v, repaint due %v", more, tt.rt.held, tt.rt.repaintDue)
	}
	// The app's state changes: the frame builds it, and asks for no more.
	count++
	tt.rt.changed()
	if built, more := tt.frame(8 * time.Millisecond); !built || more || !tt.HasText("1") {
		t.Fatalf("a change out of sight: built %v, asked for another %v, shows %q", built, more, tt.Texts())
	}
	// Some of the window shows again: what moves goes on.
	tt.h.hidden = false
	tt.rt.event(platform.SurfaceEvent{Kind: platform.SurfaceShown})
	if tt.rt.held || !tt.h.requested.Load() {
		t.Fatalf("shown again: held %v, asked for a frame %v", tt.rt.held, tt.h.requested.Load())
	}
	if built, more := tt.frame(8 * time.Millisecond); !built || !more {
		t.Errorf("the frame after showing: built %v, asked for another %v", built, more)
	}
	// Shown while nothing waited, it asks for nothing.
	tt.h.requested.Store(false)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.SurfaceShown})
	if tt.h.requested.Load() {
		t.Error("shown again while nothing waited, it asked for a frame")
	}
}
