package ui

import (
	"testing"
	"time"
)

// ticker returns tick, which moves the clock of a clockTester by d and
// builds a frame.
func ticker(tt *Tester, now *time.Time) (tick func(d time.Duration)) {
	return func(d time.Duration) {
		*now = now.Add(d)
		tt.Frame()
	}
}

func TestTooltipBase(t *testing.T) {
	var save, tip *node
	tt, now := clockTester(func(c *context) {
		coreColumn(c).Padding(80, 40).Gap(20).Children(func() {
			save = coreButton(c, "Save").Label("save")
			tip = coreTooltipBase(c, save, func(tip *node) {
				tip.Padding(4).Children(func() { coreText(c, "Saves the file") })
			})
			coreButton(c, "Open").Label("open").Tooltip("Opens a file")
		})
	}, 400, 300)
	tick := ticker(tt, now)
	s, _ := tt.Find("save")
	tt.Move(s.X+s.W/2, s.Y+s.H/2)
	tick(500 * time.Millisecond)
	if tt.HasText("Saves the file") || tip != nil {
		t.Fatal("the tip shows before the pointer rested 0.6 seconds")
	}
	tick(150 * time.Millisecond)
	if !tt.HasText("Saves the file") || tip == nil {
		t.Fatal("no tip once the pointer rested")
	}
	// Above the anchor, centered.
	tt.Frame()
	if r := tip.Bounds(); r.Y+r.H > s.Y+0.5 || abs32(r.X+r.W/2-(s.X+s.W/2)) > 0.5 {
		t.Errorf("the tip is at %v, not above the anchor %v", r, s)
	}
	// Moving to the next one shows its tip at once.
	o, _ := tt.Find("open")
	tt.Move(o.X+o.W/2, o.Y+o.H/2)
	tick(16 * time.Millisecond)
	if !tt.HasText("Opens a file") || tt.HasText("Saves the file") {
		t.Errorf("moving along: texts %q", tt.Texts())
	}
	// Not once the tips have been gone a while.
	tt.Move(390, 290)
	tick(time.Second)
	tt.Move(s.X+s.W/2, s.Y+s.H/2)
	tick(16 * time.Millisecond)
	if tt.HasText("Saves the file") {
		t.Error("the tip shows at once, long after the last one went")
	}
	// Escape hides it until the pointer leaves.
	tick(time.Second)
	tt.Key(0, KeyEscape)
	tick(time.Second)
	if tt.HasText("Saves the file") {
		t.Error("the tip shows after Escape")
	}
	tt.Move(390, 290)
	tt.Frame()
	tt.Move(s.X+s.W/2, s.Y+s.H/2)
	tick(time.Second)
	if !tt.HasText("Saves the file") {
		t.Error("no tip as the pointer comes back after Escape")
	}
}

func TestTooltipFocus(t *testing.T) {
	disabled := false
	tt, now := clockTester(func(c *context) {
		coreColumn(c).Padding(40).Gap(20).Children(func() {
			coreButton(c, "Save").Tooltip("Saves the file").Disabled(disabled)
			coreButton(c, "Open").Tooltip("Opens a file")
		})
	}, 400, 300)
	tick := ticker(tt, now)
	// The keyboard focus shows the tip at once, below the element.
	tt.Key(0, KeyTab)
	if !tt.HasText("Saves the file") {
		t.Fatalf("no tip for the focus; texts %q", tt.Texts())
	}
	b, _ := tt.Find("Save")
	if r, _ := tt.Find("Saves the file"); r.Y < b.Y+b.H {
		t.Errorf("the tip of the focus is at %v, not below %v", r, b)
	}
	tt.Key(0, KeyTab)
	if tt.HasText("Saves the file") || !tt.HasText("Opens a file") {
		t.Errorf("after Tab: texts %q", tt.Texts())
	}
	// The pointer coming to another element later takes the tip.
	tick(time.Second)
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tick(time.Second)
	if !tt.HasText("Saves the file") || tt.HasText("Opens a file") {
		t.Errorf("the pointer after the focus: texts %q", tt.Texts())
	}
	// Escape hides the tip of the focus until the focus leaves.
	tt.Move(390, 290)
	tick(time.Second)
	if !tt.HasText("Opens a file") {
		t.Fatalf("the tip of the focus does not come back; texts %q", tt.Texts())
	}
	tt.Key(0, KeyEscape)
	if tt.HasText("Opens a file") {
		t.Error("the tip of the focus shows after Escape")
	}
	tt.Key(Shift, KeyTab)
	tt.Key(0, KeyTab)
	if !tt.HasText("Opens a file") {
		t.Error("no tip as the focus comes back after Escape")
	}
	// A disabled element shows none.
	disabled = true
	tt.Frame()
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tick(time.Second)
	if tt.HasText("Saves the file") {
		t.Error("a disabled element shows its tip")
	}
}

func TestTooltipInnermost(t *testing.T) {
	tt, now := clockTester(func(c *context) {
		coreRow(c).Padding(20).Tooltip("The toolbar").Children(func() {
			coreButton(c, "Cut").Label("cut").Tooltip("Cut the selection")
		})
	}, 300, 200)
	tick := ticker(tt, now)
	b, _ := tt.Find("cut")
	tt.Move(b.X+b.W/2, b.Y+b.H/2)
	tick(time.Second)
	if !tt.HasText("Cut the selection") || tt.HasText("The toolbar") {
		t.Errorf("over the button: texts %q", tt.Texts())
	}
	tt.Move(b.X+b.W+10, b.Y+b.H/2)
	tick(time.Second)
	if tt.HasText("Cut the selection") || !tt.HasText("The toolbar") {
		t.Errorf("over the toolbar: texts %q", tt.Texts())
	}
}
