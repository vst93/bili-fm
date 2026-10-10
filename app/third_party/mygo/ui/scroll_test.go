package ui

import (
	"math"
	"testing"
)

func TestTrackScroll(t *testing.T) {
	var s ScrollState
	var top *node
	seen := float32(-1)
	tt := coreNewTester(func(c *context) {
		seen = s.Y
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			top = coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	if s.Y != 0 || s.MaxY != 800 || s.MaxX != 0 {
		t.Fatalf("at the start: %+v", s)
	}
	// The user scrolls: the state follows, before the frame builds.
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, 120)
	if s.Y != 120 || seen != 120 || top.y != -120 {
		t.Errorf("scrolled 120 down: %+v, the view saw %v, the content is at %v", s, seen, top.y)
	}
	// The app scrolls.
	s.Y = 300
	tt.Frame()
	if top.y != -300 {
		t.Errorf("set to 300: the content is at %v", top.y)
	}
	// Past the end is the end, before the start the start, and the axis it
	// does not scroll stays put.
	s.Y = math.MaxFloat32
	tt.Frame()
	if s.Y != 800 || top.y != -800 {
		t.Errorf("set past the end: %+v, the content is at %v", s, top.y)
	}
	s.X, s.Y = 50, -10
	tt.Frame()
	if s.X != 0 || s.Y != 0 || top.x != 0 || top.y != 0 {
		t.Errorf("set to (50, -10): %+v, the content is at %v, %v", s, top.x, top.y)
	}
}

func TestTrackScrollSettles(t *testing.T) {
	var s ScrollState
	long := true
	tt := coreNewTester(func(c *context) {
		// Built from the state before the layout keeps the offset within
		// the content: the next frame builds from the kept one.
		if s.Y > 0 {
			coreText(c, "Back to top")
		}
		coreScroll(c).Grow(1).TrackScroll(&s).Children(func() {
			if long {
				coreBox(c).Height(1000).Shrink(0)
			}
		})
	}, 200, 200)
	s.Y = math.MaxFloat32
	tt.Frame()
	if !tt.HasText("Back to top") {
		t.Fatalf("scrolled to the end, %+v", s)
	}
	long = false
	tt.Frame()
	if tt.HasText("Back to top") {
		t.Errorf("the content shrank to nothing, yet %+v", s)
	}
}

func TestTrackScrollFollowsTheEnd(t *testing.T) {
	var s ScrollState
	lines, builds := 10, 0
	tt := coreNewTester(func(c *context) {
		builds++
		if s.Y >= s.MaxY {
			s.Y = math.MaxFloat32
		}
		coreScroll(c).Fill().TrackScroll(&s).Children(func() {
			for range lines {
				coreBox(c).Height(50).Shrink(0)
			}
		})
	}, 200, 200)
	if s.Y != 300 || s.MaxY != 300 {
		t.Fatalf("ten lines of 50 in 200: %+v", s)
	}
	lines, builds = 20, 0
	tt.Frame()
	if s.Y != 800 || s.MaxY != 800 {
		t.Errorf("ten lines more: %+v", s)
	}
	if builds > 3 {
		t.Errorf("following the end took %d builds", builds)
	}
	// Scrolled up from the end, it stays where the user left it.
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, -100)
	lines = 30
	tt.Frame()
	if s.Y != 700 || s.MaxY != 1300 {
		t.Errorf("scrolled up, then ten lines more: %+v", s)
	}
}

func TestTrackScrollPerPage(t *testing.T) {
	pages := map[string]*ScrollState{"a": {}, "b": {}}
	page := "a"
	var top *node
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().TrackScroll(pages[page]).Children(func() {
			top = coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, 400)
	page = "b"
	tt.Frame()
	if top.y != 0 || pages["a"].Y != 400 {
		t.Errorf("page b shows from %v; page a kept %v", -top.y, pages["a"].Y)
	}
	page = "a"
	tt.Frame()
	if top.y != -400 {
		t.Errorf("back on page a, it shows from %v", -top.y)
	}
}

func TestTrackScrollList(t *testing.T) {
	var s ScrollState
	tt := coreNewTester(func(c *context) {
		coreList(c, nil, 1000, func(i int) { coreTextf(c, "Row %d", i).Height(20) }).Fill().TrackScroll(&s)
	}, 200, 200)
	// The frame that scrolls builds the rows it shows.
	s.Y = 500 * 20
	tt.rt.runFrame()
	if r, ok := tt.Find("Row 500"); !ok || r.Y != 0 {
		t.Errorf("scrolled to row 500, it is at %v (%v)", r.Y, ok)
	}
	s.Y = math.MaxFloat32
	tt.rt.runFrame()
	if r, ok := tt.Find("Row 999"); !ok || r.Y+r.H > 200 {
		t.Errorf("scrolled to the end, the last row is at %v (%v)", r, ok)
	}
}

func TestScrollIntoView(t *testing.T) {
	n, show := 2, -1
	rows := map[int]*node{}
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().Children(func() {
			for i := range n {
				h := float32(100)
				if i == 7 {
					h = 400
				}
				rows[i] = coreBox(c).Height(h).Shrink(0)
				if i == show {
					rows[i].ScrollIntoView()
					show = -1
				}
			}
		})
	}, 200, 250)
	// A row added below shows in the frame that adds it, at the bottom.
	n, show = 10, 9
	tt.rt.runFrame()
	if r := rows[9]; r.y != 150 {
		t.Errorf("row 9 shows at %v", r.y)
	}
	// A row above comes down to the top; one in view stays.
	show = 4
	tt.Frame()
	if r := rows[4]; r.y != 0 {
		t.Errorf("row 4 shows at %v", r.y)
	}
	show = 5
	tt.Frame()
	if r := rows[4]; r.y != 0 {
		t.Errorf("showing row 5, in view, moved row 4 to %v", r.y)
	}
	// What is taller than the view shows from its start.
	show = 7
	tt.Frame()
	if r := rows[7]; r.y != 0 {
		t.Errorf("row 7, 400 high, shows from %v", r.y)
	}
}

func TestScrollIntoViewNested(t *testing.T) {
	show := false
	var row *node
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().Children(func() {
			coreBox(c).Height(300).Shrink(0)
			coreScroll(c).Height(150).Shrink(0).Children(func() {
				for i := range 10 {
					r := coreBox(c).Height(50).Shrink(0)
					if i == 8 {
						row = r
						if show {
							r.ScrollIntoView()
						}
					}
				}
			})
		})
	}, 200, 200)
	show = true
	tt.Frame()
	// The inner list shows row 8 at its bottom, and the window the list's
	// bottom at its own.
	if row.y != 150 {
		t.Errorf("row 8 shows at %v", row.y)
	}
}

func TestTabScrollsTheFocusIntoView(t *testing.T) {
	var button *node
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Fill().Children(func() {
			coreBox(c).Height(300).Shrink(0)
			coreScroll(c).Height(150).Shrink(0).Children(func() {
				coreBox(c).Height(400).Shrink(0)
				button = coreButton(c, "Deep")
			})
			coreBox(c).Height(1000).Shrink(0)
		})
	}, 200, 200)
	tt.Key(0, KeyTab)
	if !tt.Focused("Deep") || button.y < 0 || button.y+button.h > 200 {
		t.Errorf("Tab to a button in a list below the window: it is at %v..%v", button.y, button.y+button.h)
	}
}
