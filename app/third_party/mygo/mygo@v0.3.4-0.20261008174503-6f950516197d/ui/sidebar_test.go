package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestSidebar(t *testing.T) {
	place, changes := "recents", 0
	locations := true
	tt := coreNewTester(func(c *context) {
		coreRow(c).Fill().AlignItems(Stretch).Children(func() {
			if coreSidebar(c, &place, func() {
				coreSidebarSection(c, "Favorites", nil, func() {
					coreSidebarItem(c, "recents", nil, "Recents")
					coreSidebarItem(c, "desktop", nil, "Desktop").Children(func() { coreBadge(c, "3") })
					coreSidebarItem(c, "documents", nil, "Documents")
				})
				coreSidebarSection(c, "Locations", &locations, func() {
					coreSidebarItem(c, "mac", nil, "Macintosh HD")
				})
			}).Width(200).Changed() {
				changes++
			}
			coreButton(c, "Other")
		})
	}, 500, 400)
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	tt.Click("Desktop")
	if place != "desktop" || changes != 1 {
		t.Fatalf("a click: %q (%d changes)", place, changes)
	}
	// The keys choose among the items, across sections.
	for _, step := range []struct {
		key  Key
		want string
	}{{KeyDown, "documents"}, {KeyDown, "mac"}, {KeyDown, "mac"}, {KeyUp, "documents"}, {KeyHome, "recents"}, {KeyEnd, "mac"}} {
		tt.Key(0, step.key)
		if place != step.want {
			t.Fatalf("%v chose %q", step.key, place)
		}
	}
	// Typing the first letters.
	tt.Key(0, KeyD)
	if place != "desktop" {
		t.Fatalf("d chose %q", place)
	}
	tt.Key(0, KeyO)
	if place != "documents" {
		t.Fatalf("do chose %q", place)
	}
	// One stop of Tab.
	tt.Key(0, KeyTab)
	if !tt.Focused("Other") {
		t.Fatal("Tab did not leave the sidebar")
	}
	tt.Key(Shift, KeyTab)
	// A click on a section's title hides its items, which the keys skip.
	tt.Click("Locations")
	if locations || tt.HasText("Macintosh HD") {
		t.Fatal("a click on the title left the section open")
	}
	tt.Key(0, KeyEnd)
	if place != "documents" {
		t.Errorf("End with Locations hidden chose %q", place)
	}
	// Assistive technology: a tree whose item chosen has the focus.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	side := accessNode(t, tree, platform.RoleTree, "")
	docs := accessNode(t, tree, platform.RoleTreeItem, "Documents")
	if side.States&platform.AccessSelectable == 0 || docs.Level != 2 || docs.States&platform.AccessChecked == 0 || tree.Focus != docs.ID {
		t.Errorf("the sidebar %+v, Documents %+v, the focus on %d", side, docs, tree.Focus)
	}
	if n := accessNode(t, tree, platform.RoleTreeItem, "Desktop"); n.Label != "Desktop 3" || n.States&platform.AccessChecked != 0 {
		t.Errorf("Desktop: %+v", n)
	}
	head := accessNode(t, tree, platform.RoleTreeItem, "Locations")
	if head.States&(platform.AccessExpandable|platform.AccessExpanded) != platform.AccessExpandable || head.Level != 1 {
		t.Errorf("Locations: %+v", head)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: head.ID, Action: platform.AccessExpand})
	if !locations || !tt.HasText("Macintosh HD") {
		t.Error("expanding Locations")
	}
	// Focusing an item chooses it.
	mac := accessNode(t, tt.h.access, platform.RoleTreeItem, "Macintosh")
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: mac.ID, Action: platform.AccessFocus})
	if place != "mac" {
		t.Errorf("focusing Macintosh HD chose %q", place)
	}
}
