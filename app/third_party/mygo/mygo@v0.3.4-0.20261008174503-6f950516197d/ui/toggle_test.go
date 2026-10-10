package ui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// focusedOf returns which of labels has the focus, "" for none.
func focusedOf(tt *Tester, labels ...string) string {
	for _, l := range labels {
		if tt.Focused(l) {
			return l
		}
	}
	return ""
}

// tabs presses Tab (or Shift+Tab) n times and returns where the focus
// went each time.
func tabs(tt *Tester, mods Modifiers, n int, labels ...string) []string {
	var got []string
	for range n {
		tt.Key(mods, KeyTab)
		got = append(got, focusedOf(tt, labels...))
	}
	return got
}

func TestFocusGroupIsOneTabStop(t *testing.T) {
	bold := false
	labels := []string{"Before", "Cut", "Bold", "Paste", "After"}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Gap(8).Children(func() {
			coreButton(c, "Before")
			coreToolbar(c, func() {
				coreButton(c, "Cut")
				coreToggle(c, &bold, "Bold")
				coreButton(c, "Paste")
			}).Label("Edit")
			coreButton(c, "After")
		})
	}, 500, 300)
	if got := tabs(tt, 0, 4, labels...); !slices.Equal(got, []string{"Before", "Cut", "After", "Before"}) {
		t.Fatalf("Tab went %q", got)
	}
	tt.Key(0, KeyTab) // into the toolbar
	var arrows []string
	for _, k := range []Key{KeyRight, KeyRight, KeyRight, KeyLeft, KeyEnd, KeyHome, KeyEnd} {
		tt.Key(0, k)
		arrows = append(arrows, focusedOf(tt, labels...))
	}
	if want := []string{"Bold", "Paste", "Cut", "Paste", "Paste", "Cut", "Paste"}; !slices.Equal(arrows, want) {
		t.Fatalf("the arrows went %q, not %q", arrows, want)
	}
	// Tab leaves the toolbar, and comes back where it left it.
	if got := tabs(tt, 0, 1, labels...); got[0] != "After" {
		t.Fatalf("Tab from the toolbar went to %q", got[0])
	}
	if got := tabs(tt, Shift, 1, labels...); got[0] != "Paste" {
		t.Errorf("Shift+Tab back into the toolbar went to %q", got[0])
	}
	if got := tabs(tt, Shift, 1, labels...); got[0] != "Before" {
		t.Errorf("Shift+Tab out of the toolbar went to %q", got[0])
	}
}

func TestFocusGroupLeavesKeysToWhatTakesThem(t *testing.T) {
	name, volume := "abc", 0.5
	tt := coreNewTester(func(c *context) {
		coreToolbar(c, func() {
			coreButton(c, "Cut")
			coreTextInput(c, &name).Label("Name").Width(100)
			coreSlider(c, &volume, 0, 1).Label("Volume").Width(100)
			coreButton(c, "Paste")
		})
	}, 600, 200)
	tt.Key(0, KeyTab)
	tt.Key(0, KeyRight)
	if !tt.Focused("Name") {
		t.Fatal("Right from Cut did not go to the input")
	}
	tt.Key(0, KeyLeft) // the caret moves
	if !tt.Focused("Name") {
		t.Error("Left in the input left it")
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tt.rt.focused = accessNode(t, tt.h.access, platform.RoleSlider, "Volume").ID
	tt.Frame()
	tt.Key(0, KeyRight)
	if !tt.Focused("Volume") || volume <= 0.5 {
		t.Errorf("Right on the slider moved the focus, or not the slider (%v)", volume)
	}
}

func TestRadioGroupArrowsChoose(t *testing.T) {
	size := "Medium"
	labels := []string{"Before", "Small", "Medium", "Large", "After"}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Gap(8).Children(func() {
			coreButton(c, "Before")
			coreRadioGroup(c, func() {
				for _, s := range []string{"Small", "Medium", "Large"} {
					coreRadio(c, &size, s, s)
				}
			}).Label("Size")
			coreButton(c, "After")
		})
	}, 400, 300)
	// Tab goes to the one chosen.
	if got := tabs(tt, 0, 2, labels...); !slices.Equal(got, []string{"Before", "Medium"}) {
		t.Fatalf("Tab went %q", got)
	}
	tt.Key(0, KeyDown)
	if size != "Large" || !tt.Focused("Large") {
		t.Fatalf("Down chose %q", size)
	}
	tt.Key(0, KeyDown) // round to the first
	tt.Key(0, KeyRight)
	if size != "Medium" {
		t.Errorf("Down, Right chose %q", size)
	}
	if got := tabs(tt, 0, 1, labels...); got[0] != "After" {
		t.Errorf("Tab went to %q", got[0])
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	g := accessNode(t, tt.h.access, platform.RoleRadioGroup, "Size")
	n := 0
	for _, x := range tt.h.access.Nodes {
		if x.Role == platform.RoleRadio && tt.h.access.Nodes[x.Parent].ID == g.ID {
			n++
		}
	}
	if n != 3 {
		t.Errorf("the radio group holds %d radio buttons", n)
	}
}

func TestToggle(t *testing.T) {
	bold, changes := false, 0
	tt := coreNewTester(func(c *context) {
		if coreToggle(c, &bold, "Bold").Changed() {
			changes++
		}
	}, 300, 200)
	tt.Click("Bold")
	if !bold || changes != 1 {
		t.Fatalf("a click: bold %v, %d changes", bold, changes)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	n := accessNode(t, tt.h.access, platform.RoleToggleButton, "Bold")
	if n.States&platform.AccessChecked == 0 {
		t.Errorf("pressed, it reads %+v", n)
	}
	tt.Key(0, KeySpace)
	if bold {
		t.Error("Space did not let it go")
	}
}

func TestSegmented(t *testing.T) {
	view, changes := 0, 0
	tt := coreNewTester(func(c *context) {
		if coreSegmented(c, &view, "List", "Grid", "Columns").Label("View").Changed() {
			changes++
		}
	}, 400, 200)
	tt.Click("Grid")
	if view != 1 || changes != 1 {
		t.Fatalf("a click on Grid: view %d, %d changes", view, changes)
	}
	tt.Key(0, KeyRight)
	if view != 2 || !tt.Focused("Columns") {
		t.Errorf("Right chose %d", view)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	accessNode(t, tt.h.access, platform.RoleRadioGroup, "View")
	if n := accessNode(t, tt.h.access, platform.RoleRadio, "Columns"); n.States&platform.AccessChecked == 0 {
		t.Errorf("Columns reads %+v", n)
	}
}

func TestTabsAreOneTabStop(t *testing.T) {
	tab := 1
	labels := []string{"Before", "One", "Two", "Three", "After"}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(8).Children(func() {
			coreButton(c, "Before")
			coreTabs(c, &tab, "One", "Two", "Three")
			coreButton(c, "After")
		})
	}, 400, 200)
	if got := tabs(tt, 0, 3, labels...); !slices.Equal(got, []string{"Before", "Two", "After"}) {
		t.Errorf("Tab went %q", got)
	}
}

func TestToolbarOverflows(t *testing.T) {
	clicked := ""
	italic := false
	names := []string{"Alpha", "Bravo", "Charlie", "Delta", "Echo"}
	tt := coreNewTester(func(c *context) {
		coreToolbar(c, func() {
			for _, n := range names {
				if coreButton(c, n).Clicked() {
					clicked = n
				}
			}
			coreToggle(c, &italic, "Italic")
		}).Fill()
	}, 260, 100)
	// What does not fit goes into the menu, from the last.
	shown := []string{}
	for _, n := range append(names, "Italic") {
		if tt.HasText(n) {
			shown = append(shown, n)
		}
	}
	if len(shown) == 0 || len(shown) == 6 || shown[0] != "Alpha" {
		t.Fatalf("shows %q", shown)
	}
	if _, ok := tt.Find("More"); !ok {
		t.Fatal("no overflow button")
	}
	tt.Click("More")
	menu := tt.Menu()
	if len(menu) == 0 || menu[len(menu)-1] != "Italic" || slices.Contains(menu, shown[0]) {
		t.Fatalf("the menu has %q", menu)
	}
	if err := tt.ChooseMenuItem("Echo"); err != nil {
		t.Fatal(err)
	}
	if clicked != "Echo" {
		t.Errorf("choosing Echo clicked %q", clicked)
	}
	tt.Click("More")
	tt.ChooseMenuItem("Italic")
	if !italic {
		t.Error("choosing Italic did not press it")
	}
	tt.Click("More")
	if !tt.h.menu.Items[len(tt.h.menu.Items)-1].Checked {
		t.Error("Italic is not checked in the menu once pressed")
	}
	tt.CloseMenu()
	// With room, all show and the button goes.
	tt.SetSize(900, 100)
	for _, n := range names {
		if !tt.HasText(n) {
			t.Errorf("%s does not show with room", n)
		}
	}
	if r, ok := tt.Find("More"); ok && r.W > 0 {
		t.Error("the overflow button shows with room")
	}
}

func TestToolbarAccessibility(t *testing.T) {
	bold := true
	tt := coreNewTester(func(c *context) {
		coreToolbar(c, func() {
			coreToggleGroup(c, func() {
				coreToggle(c, &bold, "Bold")
			}).Label("Style")
		}).Label("Format")
	}, 400, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tb := accessNode(t, tt.h.access, platform.RoleToolbar, "Format")
	b := accessNode(t, tt.h.access, platform.RoleToggleButton, "Bold")
	if b.States&platform.AccessChecked == 0 {
		t.Errorf("Bold reads %+v", b)
	}
	// The toolbar holds the group, which holds the toggle.
	group := tt.h.access.Nodes[b.Parent]
	if group.Label != "Style" || tt.h.access.Nodes[group.Parent].ID != tb.ID {
		t.Errorf("the toggle is in %+v", group)
	}
}

func TestToolbarEntersAtItsFirst(t *testing.T) {
	bold := true
	tt := coreNewTester(func(c *context) {
		coreToolbar(c, func() {
			coreButton(c, "Cut")
			coreToggle(c, &bold, "Bold")
		})
	}, 400, 100)
	tt.Key(0, KeyTab)
	if !tt.Focused("Cut") {
		t.Error("Tab went to the toggle that is on, not to the first control")
	}
}
