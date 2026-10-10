package ui

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestToastAction(t *testing.T) {
	notes, undone := 3, 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Padding(20).Children(func() {
			coreTextf(c, "%d notes", notes)
			if coreButton(c, "Delete").Clicked() {
				notes--
				c.ToastAction("Note deleted", "Undo", func() {
					notes++
					undone++
				})
			}
		})
	}, 400, 300)
	tt.Click("Delete")
	if notes != 2 || !tt.HasText("Note deleted") || !tt.HasText("Undo") {
		t.Fatalf("after Delete: %d notes, %q", notes, tt.Texts())
	}
	// Assistive technology reads the toast, and reaches its button.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	accessNode(t, tt.h.access, platform.RoleStatus, "Note deleted")
	accessNode(t, tt.h.access, platform.RoleButton, "Undo")
	// The time stops while the pointer rests on the toast.
	r, _ := tt.Find("Note deleted")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if !tt.rt.toastsPaused {
		t.Error("the pointer on the toast did not stop its time")
	}
	tt.Click("Undo")
	if notes != 3 || undone != 1 || tt.HasText("Note deleted") || !tt.HasText("3 notes") {
		t.Errorf("after Undo: %d notes, %d undone, %q", notes, undone, tt.Texts())
	}
}

func TestCheckboxGroup(t *testing.T) {
	mail, calendar := false, false
	changes := 0
	tt := coreNewTester(func(c *context) {
		if coreCheckboxGroup(c, "Notifications", func() {
			coreCheckbox(c, &mail, "Mail")
			coreCheckbox(c, &calendar, "Calendar")
		}).Changed() {
			changes++
		}
	}, 300, 200)
	tt.Click("Notifications")
	if !mail || !calendar || changes != 1 {
		t.Fatalf("a click on the group: %v %v", mail, calendar)
	}
	tt.Click("Mail")
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleCheckBox, "Notifications"); n.States&platform.AccessMixed == 0 {
		t.Fatalf("with one of two: %+v", n)
	}
	// Mixed, a click checks them all; all, it checks none.
	tt.Click("Notifications")
	if !mail || !calendar {
		t.Fatalf("from mixed: %v %v", mail, calendar)
	}
	tt.Key(0, KeySpace)
	if mail || calendar {
		t.Errorf("Space with all checked: %v %v", mail, calendar)
	}
	accessNode(t, tt.h.access, platform.RoleGroup, "Notifications")
}

func TestBreadcrumbs(t *testing.T) {
	path := []string{"Macintosh HD", "Users", "ada", "Documents"}
	chosen := -1
	tt := coreNewTester(func(c *context) {
		coreBreadcrumbs(c, path, &chosen).Label("Path")
	}, 500, 100)
	tt.Click("Users")
	if chosen != 1 {
		t.Fatalf("a click on Users: %d", chosen)
	}
	tt.Click("Documents")
	if chosen != 1 {
		t.Errorf("a click on the last item: %d", chosen)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	accessNode(t, tt.h.access, platform.RoleGroup, "Path")
	accessNode(t, tt.h.access, platform.RoleLink, "ada")
	for _, n := range tt.h.access.Nodes {
		if n.Role == platform.RoleLink && n.Label == "Documents" {
			t.Error("the last item is a link")
		}
	}
}

func TestAlertDialog(t *testing.T) {
	open, deleted, canceled := true, 0, 0
	tt := coreNewTester(func(c *context) {
		coreButton(c, "Behind")
		switch coreAlertDialog(c, &open, "Delete Notes?", "You can't undo this.", "Cancel", "Delete") {
		case 0:
			canceled++
		case 1:
			deleted++
		}
	}, 500, 400)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleAlertDialog, "Delete Notes?"); n.Description != "You can't undo this." {
		t.Errorf("the alert: %+v", n)
	}
	if !tt.Focused("Delete") {
		t.Fatal("the default button has no focus")
	}
	// Assistive technology sees what is in the alert, the focus with it.
	if n := accessNode(t, tt.h.access, platform.RoleButton, "Delete"); tt.h.access.Focus != n.ID {
		t.Errorf("the focus on %d, not the default button", tt.h.access.Focus)
	}
	// A click outside does nothing.
	tt.ClickAt(5, 5)
	if !open {
		t.Fatal("a click outside closed the alert")
	}
	tt.Key(0, KeyEscape)
	if open || canceled != 1 {
		t.Fatalf("Escape: open %v, %d canceled", open, canceled)
	}
	open = true
	tt.Frame()
	tt.Key(0, KeyEnter)
	if open || deleted != 1 {
		t.Errorf("Enter: open %v, %d deleted", open, deleted)
	}
}

func TestFindBar(t *testing.T) {
	open := false
	query, current, moves := "", 0, 0
	text := "the cat sat on the mat with the hat"
	count := func() int {
		if query == "" {
			return 0
		}
		n := 0
		for i := 0; i+len(query) <= len(text); i++ {
			if text[i:i+len(query)] == query {
				n++
			}
		}
		return n
	}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			if c.Shortcut(Cmd, KeyF) {
				open = true
			}
			if coreFindBar(c, &open, &query, count(), &current).Changed() {
				moves++
			}
			coreText(c, text).Padding(10)
		})
	}, 600, 300)
	tt.Key(Cmd, KeyF)
	if !open || !tt.Focused("Find") {
		t.Fatalf("Cmd+F: open %v, the field focused %v", open, tt.Focused("Find"))
	}
	tt.Type("the")
	if count() != 3 || !tt.HasText("1 of 3") {
		t.Fatalf("typed: %d matches, %q", count(), tt.Texts())
	}
	tt.Key(0, KeyEnter)
	tt.Key(0, KeyEnter)
	if current != 2 || !tt.HasText("3 of 3") {
		t.Fatalf("Enter twice: %d", current)
	}
	tt.Key(0, KeyEnter)
	if current != 0 {
		t.Fatalf("round the end: %d", current)
	}
	tt.Key(Shift, KeyEnter)
	if current != 2 || moves != 4 {
		t.Fatalf("Shift+Enter: %d (%d moves)", current, moves)
	}
	if runtime.GOOS == "darwin" {
		tt.Key(Cmd, KeyG)
	} else {
		// From the field, which leaves function keys to shortcuts.
		tt.Key(0, KeyF3)
	}
	if current != 0 {
		t.Errorf("the next match's shortcut: %d", current)
	}
	tt.Click("Next")
	if current != 1 {
		t.Errorf("Next: %d", current)
	}
	tt.Click("Find")
	tt.Key(0, KeyEscape)
	if open {
		t.Error("Escape left the bar open")
	}
	// Opened again, it focuses the field with what was found before chosen.
	tt.Key(Cmd, KeyF)
	tt.Type("x")
	if query != "x" {
		t.Errorf("typed over the last query: %q", query)
	}
	tt.Type("yz")
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if !tt.HasText("No matches") {
		t.Errorf("no matches: %q", tt.Texts())
	}
	accessNode(t, tt.h.access, platform.RoleStatus, "No matches")
}
