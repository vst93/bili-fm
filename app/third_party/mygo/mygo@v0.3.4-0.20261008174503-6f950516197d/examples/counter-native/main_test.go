package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The view runs without a window in tests, which click and press keys as a
// user would.
func TestCounter(t *testing.T) {
	s := &counter{}
	tt := ui.NewTester(s.view, 360, 260)
	for _, label := range []string{"Increment", "Increment", "Decrement"} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if s.n != 1 || !tt.HasText("1") {
		t.Errorf("count %d after the buttons, texts %q", s.n, tt.Texts())
	}
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyUp)
	tt.Key(0, ui.KeyDown)
	if s.n != 2 {
		t.Errorf("count %d after the arrow keys, want 2", s.n)
	}
	if err := tt.Click("Reset"); err != nil {
		t.Fatal(err)
	}
	if s.n != 0 {
		t.Errorf("count %d after Reset, want 0", s.n)
	}
}
