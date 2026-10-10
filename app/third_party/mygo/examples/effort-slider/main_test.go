package main

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// The thumb stays under the pointer while it drags, choosing the nearest
// level, and eases to that level when let go; the arrow keys step.
func TestSlider(t *testing.T) {
	s := newSlider()
	now := time.Unix(1000, 0)
	s.clock = func() time.Time { return now }
	tt := ui.NewTester(s.view, windowW, windowH)
	ox, oy := float32(126), float32(cardY)
	at := func(pos float32) (float32, float32) { return ox + stopX + stopGap*pos, oy + brainY }

	tt.Press(at(0))
	tt.Move(at(1.3))
	if pos, _ := s.thumb(now); math.Abs(pos-1.3) > 0.01 || s.level != 1 {
		t.Errorf("dragged to 1.3, the thumb is at %.2f, level %d", pos, s.level)
	}
	tt.Move(at(2.7))
	if pos, _ := s.thumb(now); math.Abs(pos-2.7) > 0.01 || s.level != 3 || levels[s.level] != "Extra" {
		t.Errorf("dragged to 2.7, the thumb is at %.2f, level %d", pos, s.level)
	}
	tt.Release(at(2.7))
	if pos, moving := s.thumb(now); math.Abs(pos-2.7) > 0.01 || !moving {
		t.Errorf("let go at 2.7, the thumb is at %.2f, moving %v", pos, moving)
	}
	now = now.Add(moveTime)
	if pos, moving := s.thumb(now); pos != 3 || moving {
		t.Errorf("after easing, the thumb is at %.2f, moving %v; want level 3", pos, moving)
	}

	tt.Key(0, ui.KeyRight)
	if s.level != 4 {
		t.Errorf("right arrow went to level %d, want 4", s.level)
	}
	tt.Key(0, ui.KeyHome)
	if s.level != 0 {
		t.Errorf("Home went to level %d, want 0", s.level)
	}
}

// End goes Galaxy, whose effects come in and go when the slider leaves it.
func TestGalaxy(t *testing.T) {
	s := newSlider()
	now := time.Unix(1000, 0)
	s.clock = func() time.Time { return now }
	tt := ui.NewTester(s.view, windowW, windowH)
	tt.ClickAt(126+stopX, cardY+brainY)
	tt.Key(0, ui.KeyEnd)
	if s.level != galaxy || s.galaxyLevel(now) != 0 {
		t.Fatalf("End went to level %d, Galaxy %.2f in", s.level, s.galaxyLevel(now))
	}
	now = now.Add(time.Second)
	tt.Frame()
	if g := s.galaxyLevel(now); g != 1 {
		t.Errorf("a second into Galaxy, it is %.2f in", g)
	}
	tt.Key(0, ui.KeyLeft)
	now = now.Add(galaxyOut)
	tt.Frame()
	if g := s.galaxyLevel(now); s.level != 4 || g != 0 {
		t.Errorf("after leaving Galaxy, level %d and Galaxy %.2f in", s.level, g)
	}
}
