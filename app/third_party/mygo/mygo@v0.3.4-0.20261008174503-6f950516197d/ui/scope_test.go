package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// dialogView is a window with buttons behind a dialog of a text input, a
// select and two buttons, and a window shortcut.
type dialogView struct {
	open           bool
	name, size     string
	behind, inside int
}

func (d *dialogView) view(c *context) {
	coreRow(c).Gap(8).Children(func() {
		coreButton(c, "Before")
		if coreButton(c, "Open").Clicked() {
			d.open = true
		}
		coreButton(c, "After")
	})
	if c.Shortcut(Cmd, KeyN) {
		d.behind++
	}
	coreModal(c, &d.open, func() {
		coreTextInput(c, &d.name).Label("Name")
		coreSelect(c, &d.size, []string{"Small", "Large"})
		if c.Shortcut(Cmd, KeyS) {
			d.inside++
		}
		coreRow(c).Gap(8).Children(func() {
			if coreButton(c, "OK").Clicked() {
				d.open = false
			}
			coreButton(c, "Cancel")
		})
	})
}

func TestDialogKeepsTheFocus(t *testing.T) {
	d := &dialogView{size: "Small"}
	tt := coreNewTester(d.view, 500, 400)
	tt.Click("Open")
	// The dialog takes the focus as it opens, on its first element.
	if !tt.Focused("Name") {
		t.Fatal("the dialog opened without the focus")
	}
	// Tab goes round its elements, never to the window behind.
	var order []string
	for range 6 {
		tt.Key(0, KeyTab)
		for _, l := range []string{"Name", "Small", "OK", "Cancel", "Before", "Open", "After"} {
			if tt.Focused(l) {
				order = append(order, l)
			}
		}
	}
	want := []string{"Small", "OK", "Cancel", "Name", "Small", "OK"}
	if len(order) != len(want) {
		t.Fatalf("Tab went %q", order)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("Tab went %q, not %q", order, want)
		}
	}
	tt.Key(Shift, KeyTab)
	tt.Key(Shift, KeyTab)
	tt.Key(Shift, KeyTab)
	if !tt.Focused("Cancel") {
		t.Error("Shift+Tab from Name does not go round to Cancel")
	}
}

func TestEscapeClosesTheOverlayOnTop(t *testing.T) {
	d := &dialogView{size: "Small"}
	tt := coreNewTester(d.view, 500, 400)
	tt.Click("Open")
	tt.Click("Small") // the select's popup opens
	if !tt.HasText("Large") {
		t.Fatal("the select did not open")
	}
	tt.Key(0, KeyEscape)
	if tt.HasText("Large") || !d.open {
		t.Fatalf("Escape: the popup shows %v, the dialog open %v", tt.HasText("Large"), d.open)
	}
	tt.Key(0, KeyEscape)
	if d.open {
		t.Error("a second Escape did not close the dialog")
	}
}

func TestOverlayGivesTheFocusBack(t *testing.T) {
	d := &dialogView{size: "Small"}
	tt := coreNewTester(d.view, 500, 400)
	tt.Click("Open")
	tt.Key(0, KeyEscape)
	if d.open || !tt.Focused("Open") {
		t.Errorf("closed by Escape: the focus is not back on Open (open %v)", d.open)
	}
	tt.Click("Open")
	tt.Click("OK")
	if d.open || !tt.Focused("Open") {
		t.Errorf("closed by OK: the focus is not back on Open")
	}
}

func TestShortcutsWaitBehindADialog(t *testing.T) {
	d := &dialogView{size: "Small"}
	tt := coreNewTester(d.view, 500, 400)
	tt.Key(Cmd, KeyN)
	if d.behind != 1 {
		t.Fatalf("Cmd+N with no dialog: %d", d.behind)
	}
	tt.Click("Open")
	tt.Key(Cmd, KeyN)
	tt.Key(Cmd, KeyS)
	if d.behind != 1 || d.inside != 1 {
		t.Errorf("with the dialog open: Cmd+N behind it %d, Cmd+S in it %d", d.behind, d.inside)
	}
}

func TestPopoverFollowsItsAnchor(t *testing.T) {
	open := false
	tt := coreNewTester(func(c *context) {
		coreRow(c).Gap(8).Children(func() {
			coreButton(c, "A")
			menu := coreButton(c, "Menu")
			coreButton(c, "B")
			corePopover(c, menu, &open, func() {
				coreButton(c, "P1")
				coreButton(c, "P2")
			})
		})
	}, 500, 300)
	tt.Click("Menu")
	open = true
	tt.Frame()
	var order []string
	for range 4 {
		tt.Key(0, KeyTab)
		for _, l := range []string{"A", "Menu", "B", "P1", "P2"} {
			if tt.Focused(l) {
				order = append(order, l)
			}
		}
	}
	want := []string{"P1", "P2", "B", "A"}
	for i := range want {
		if i >= len(order) || order[i] != want[i] {
			t.Fatalf("Tab from the anchor went %q, not %q", order, want)
		}
	}
}

func TestBehindADialogIsHidden(t *testing.T) {
	d := &dialogView{size: "Small"}
	tt := coreNewTester(d.view, 500, 400)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tt.Click("Open")
	labels := map[string]bool{}
	for _, n := range tt.h.access.Nodes {
		labels[n.Label] = true
	}
	if labels["Before"] || !labels["OK"] || !labels["Name"] {
		t.Errorf("with the dialog open, assistive technology sees %v", labels)
	}
	if n, ok := byID(tt.h.access, tt.h.access.Focus); !ok || n.Label != "Name" {
		t.Errorf("its focus is on %+v", n)
	}
}

func TestEscapeGoesToTheFocusedElementFirst(t *testing.T) {
	// A terminal in a dialog takes Escape, as vim needs it.
	open, escapes := true, 0
	tt := coreNewTester(func(c *context) {
		coreModal(c, &open, func() {
			coreBox(c).Size(100, 100).Focusable().Label("Terminal").AutoFocus().HandleInput(func(ev InputEvent) bool {
				if ev.Kind == InputKeyDown && ev.Key == KeyEscape {
					escapes++
					return true
				}
				return false
			})
		})
	}, 500, 400)
	tt.Key(0, KeyEscape)
	if !open || escapes != 1 {
		t.Errorf("Escape in the terminal: dialog open %v, the terminal took %d", open, escapes)
	}
}
