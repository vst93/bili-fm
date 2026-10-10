package ui

import "testing"

// A view sees the modifier keys held on their own, draws again as they change, and lets them
// go with the window's focus.
func TestModifiersHeld(t *testing.T) {
	var seen []Modifiers
	tt := coreNewTester(func(c *context) {
		seen = append(seen, c.Modifiers())
		if c.Modifiers()&Cmd != 0 {
			coreText(c, "Shortcuts")
		}
	}, 200, 100)
	if tt.HasText("Shortcuts") {
		t.Fatal("shortcuts with nothing held")
	}
	tt.HoldModifiers(Cmd)
	if !tt.HasText("Shortcuts") || seen[len(seen)-1] != Cmd {
		t.Fatalf("holding Cmd drew %v", seen)
	}
	// A key pressed with them says what is held too.
	tt.Key(Cmd|Shift, KeyA)
	if seen[len(seen)-1] != Cmd|Shift {
		t.Errorf("after Cmd+Shift+A: %v", seen[len(seen)-1])
	}
	tt.HoldModifiers(0)
	if tt.HasText("Shortcuts") {
		t.Error("letting go kept the shortcuts")
	}
	tt.HoldModifiers(Cmd)
	tt.SetFocused(false)
	if tt.HasText("Shortcuts") {
		t.Error("the window losing the keyboard kept Cmd held")
	}
}
