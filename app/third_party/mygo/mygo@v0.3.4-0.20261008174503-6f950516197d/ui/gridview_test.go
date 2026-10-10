package ui

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestGridView(t *testing.T) {
	const n = 10000
	sel, opened, built := -1, -1, 0
	s := GridState{Selected: &sel, Label: func(i int) string { return fmt.Sprintf("Photo %d", i) }}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			if coreGridView(c, &s, n, 100, 80, func(i int) {
				built++
				coreTextf(c, "Photo %d", i)
			}).Grow(1).Submitted() {
				opened = sel
			}
		})
	}, 448, 400)
	// (448 + 8) / (100 + 8): 4 columns, each (448 - 3*8) / 4 = 106 wide.
	if s.cols != 4 {
		t.Fatalf("%d columns", s.cols)
	}
	built = 0
	tt.Frame()
	if built == 0 || built > 40 {
		t.Errorf("a frame built %d items of %d", built, n)
	}
	r, _ := tt.Find("Photo 5")
	tt.ClickAt(r.X, r.Y)
	if sel != 5 {
		t.Fatalf("a click chose %d", sel)
	}
	for _, step := range []struct {
		key  Key
		want int
	}{{KeyRight, 6}, {KeyDown, 10}, {KeyLeft, 9}, {KeyUp, 5}, {KeyUp, 1}, {KeyUp, 1}, {KeyEnd, n - 1}, {KeyHome, 0}} {
		tt.Key(0, step.key)
		if sel != step.want {
			t.Fatalf("%v chose %d, not %d", step.key, sel, step.want)
		}
	}
	tt.Key(0, KeyEnter)
	if opened != 0 {
		t.Errorf("Enter opened %d", opened)
	}
	// End shows the last item.
	tt.Key(0, KeyEnd)
	if !tt.HasText(fmt.Sprintf("Photo %d", n-1)) {
		t.Error("End did not show the last item")
	}
	// The cells share the width.
	cell := accessNode(t, func() *platform.AccessTree {
		tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
		return tt.h.access
	}(), platform.RoleListItem, fmt.Sprintf("Photo %d", n-1))
	if cell.Bounds.W != 106 || cell.Bounds.H != 80 {
		t.Errorf("a cell is %v", cell.Bounds)
	}
	// Assistive technology: a list of n items, the one chosen with the focus.
	tree := tt.h.access
	list := accessNode(t, tree, platform.RoleList, "")
	if list.SetSize != n || list.States&platform.AccessSelectable == 0 || cell.PosInSet != n || cell.SetSize != n || cell.States&platform.AccessChecked == 0 || tree.Focus != cell.ID {
		t.Errorf("the list %+v, the item %+v, the focus on %d", list, cell, tree.Focus)
	}
}

func TestGridViewChoosesSeveral(t *testing.T) {
	sel := -1
	var chosen Selection[string]
	names := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	s := GridState{Selected: &sel, Selection: &chosen, Key: func(i int) any { return names[i] }}
	tt := coreNewTester(func(c *context) {
		coreGridView(c, &s, len(names), 100, 60, func(i int) {
			coreText(c, names[i])
		}).Grow(1)
	}, 448, 300)
	click := func(mods Modifiers, name string) {
		r, _ := tt.Find(name)
		tt.ClickAtWith(mods, r.X, r.Y)
	}
	click(0, "b")
	click(Cmd, "d")
	if chosen.Len() != 2 || !chosen.Has("b") || !chosen.Has("d") {
		t.Fatalf("Cmd-click: %d chosen", chosen.Len())
	}
	click(Shift, "g")
	if chosen.Len() != 4 || !chosen.Has("e") || chosen.Has("b") {
		t.Fatalf("Shift-click from d to g: %d chosen", chosen.Len())
	}
	tt.Key(Shift, KeyUp)
	if sel != 2 || chosen.Len() != 2 || !chosen.Has("c") || !chosen.Has("d") {
		t.Fatalf("Shift-Up from g: chose %d, %d chosen", sel, chosen.Len())
	}
	tt.Key(Cmd, KeyA)
	if chosen.Len() != len(names) {
		t.Errorf("Cmd+A chose %d", chosen.Len())
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleList, ""); n.States&platform.AccessMultiselectable == 0 {
		t.Errorf("the list %+v", n)
	}
}

func TestGridViewResizes(t *testing.T) {
	sel := 13
	s := GridState{Selected: &sel}
	tt := coreNewTester(func(c *context) {
		coreGridView(c, &s, 100, 100, 80, func(i int) {
			coreTextf(c, "Item %d", i)
		}).Grow(1)
	}, 448, 300)
	if s.cols != 4 {
		t.Fatalf("%d columns", s.cols)
	}
	tt.SetSize(664, 300)
	// (664 + 8) / 108: 6 columns.
	if s.cols != 6 {
		t.Fatalf("after widening, %d columns", s.cols)
	}
	tt.Click("Item 13")
	tt.Key(0, KeyDown)
	if sel != 19 {
		t.Errorf("Down from 13 in 6 columns chose %d", sel)
	}
}
