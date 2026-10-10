package ui

import (
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// expectChosen fails unless sel holds exactly keys.
func expectChosen[K int | string](t *testing.T, sel *Selection[K], keys ...K) {
	t.Helper()
	got := slices.Sorted(sel.All())
	slices.Sort(keys)
	if !slices.Equal(got, keys) {
		t.Fatalf("chosen %v, not %v", got, keys)
	}
}

func TestClickModifiers(t *testing.T) {
	var got Modifiers = 0xff
	tt := coreNewTester(func(c *context) {
		if b := coreButton(c, "Go"); b.Clicked() {
			got = b.ClickModifiers()
		}
	}, 300, 200)
	tt.ClickWith(Shift|Alt, "Go")
	if got != Shift|Alt {
		t.Errorf("Shift+Alt-click: %v", got)
	}
	tt.Click("Go")
	if got != 0 {
		t.Errorf("click: %v", got)
	}
	tt.ClickWith(Cmd, "Go")
	tt.Key(0, KeyEnter) // focused by the click
	if got != 0 {
		t.Errorf("Enter: %v", got)
	}
}

// severalView is a list of 20 rows of which the user chooses several.
func severalView(s *ListState, changes *int) func(c *context) {
	return func(c *context) {
		if coreList(c, s, 20, func(i int) { coreTextf(c, "Row %d", i).Height(30) }).Grow(1).Changed() {
			*changes++
		}
	}
}

func TestListChoosesSeveral(t *testing.T) {
	var sel Selection[int]
	s := ListState{Selection: &sel}
	changes := 0
	tt := coreNewTester(severalView(&s, &changes), 300, 400)
	mac := runtime.GOOS == "darwin"
	tt.Click("Row 2")
	tt.ClickWith(Cmd, "Row 5")
	expectChosen(t, &sel, 2, 5)
	if changes != 2 {
		t.Errorf("%d changes", changes)
	}
	// Shift chooses from the row last chosen: on macOS with the others,
	// elsewhere in their place.
	tt.ClickWith(Shift, "Row 7")
	if mac {
		expectChosen(t, &sel, 2, 5, 6, 7)
	} else {
		expectChosen(t, &sel, 5, 6, 7)
	}
	// Again, from the same row: in place of the rows it chose.
	tt.ClickWith(Shift, "Row 4")
	if mac {
		expectChosen(t, &sel, 2, 4, 5)
	} else {
		expectChosen(t, &sel, 4, 5)
	}
	tt.ClickWith(Cmd, "Row 5")
	if mac {
		expectChosen(t, &sel, 2, 4)
	} else {
		expectChosen(t, &sel, 4)
	}
	// Cmd+Shift keeps the others everywhere.
	tt.ClickWith(Cmd|Shift, "Row 7")
	if mac {
		expectChosen(t, &sel, 2, 4, 5, 6, 7)
	} else {
		expectChosen(t, &sel, 4, 5, 6, 7)
	}
	tt.Click("Row 9")
	expectChosen(t, &sel, 9)
	changes = 0
	tt.Click("Row 9")
	if changes != 0 {
		t.Error("choosing the row chosen alone again is a change")
	}
}

func TestListExtendsTheChoiceWithTheKeys(t *testing.T) {
	var sel Selection[int]
	s := ListState{Selection: &sel}
	changes := 0
	tt := coreNewTester(severalView(&s, &changes), 300, 400)
	tt.Click("Row 3")
	tt.Key(Shift, KeyDown)
	tt.Key(Shift, KeyDown)
	expectChosen(t, &sel, 3, 4, 5)
	for range 3 {
		tt.Key(Shift, KeyUp)
	}
	expectChosen(t, &sel, 2, 3)
	tt.Key(Cmd, KeyA)
	if sel.Len() != 20 {
		t.Fatalf("Cmd+A chose %d", sel.Len())
	}
	// Down goes from the row last chosen, alone.
	tt.Key(0, KeyDown)
	expectChosen(t, &sel, 3)
	tt.Key(Shift, KeyEnd)
	if sel.Len() != 17 || !sel.Has(19) {
		t.Fatalf("Shift+End: %v", slices.Sorted(sel.All()))
	}
	tt.Key(Shift, KeyHome)
	expectChosen(t, &sel, 0, 1, 2, 3)
	if _, last := s.Visible(); last >= 19 {
		t.Error("the list did not show the start again")
	}
}

func TestListMovesWithoutChoosing(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("Ctrl with the arrows is for Linux and Windows")
	}
	var sel Selection[int]
	s := ListState{Selection: &sel}
	changes := 0
	tt := coreNewTester(severalView(&s, &changes), 300, 400)
	tt.Click("Row 3")
	tt.Key(Ctrl, KeyDown)
	tt.Key(Ctrl, KeyDown)
	expectChosen(t, &sel, 3)
	tt.Key(Ctrl, KeySpace)
	expectChosen(t, &sel, 3, 5)
	tt.Key(Ctrl, KeySpace)
	expectChosen(t, &sel, 3)
	tt.Key(Ctrl, KeySpace)
	tt.Key(Shift, KeyDown)
	expectChosen(t, &sel, 5, 6)
}

func TestListChoiceOfSeveralFollowsItsItems(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f", "g", "h"}
	var sel Selection[string]
	s := ListState{Key: func(i int) any { return items[i] }, Selection: &sel}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(items), func(i int) { coreText(c, items[i]).Height(30) }).Grow(1)
	}, 300, 400)
	tt.Click("c")
	tt.ClickWith(Shift, "e")
	expectChosen(t, &sel, "c", "d", "e")
	items = append([]string{"x", "y"}, items...)
	tt.Frame()
	// Shift goes on from where it was, at c, the keys from e.
	tt.Key(Shift, KeyDown)
	expectChosen(t, &sel, "c", "d", "e", "f")
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	var shown []string
	for _, n := range tt.h.access.Nodes {
		if n.Role == platform.RoleListItem && n.States&platform.AccessChecked != 0 {
			shown = append(shown, n.Label)
		}
	}
	if !slices.Equal(shown, []string{"c", "d", "e", "f"}) {
		t.Errorf("the rows shown chosen are %q", shown)
	}
}

func TestListTypeToChoose(t *testing.T) {
	names := []string{"Apple", "Banana", "Cherry", "Chestnut", "Date", "Citrus"}
	sel := -1
	s := ListState{Selected: &sel, Label: func(i int) string { return names[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(names), func(i int) { coreText(c, names[i]).Height(30) }).Grow(1).AutoFocus()
	}, 300, 400)
	tt.Key(0, KeyA)
	if sel != 0 {
		t.Fatalf("A with no row chosen went to %d", sel)
	}
	pause := func() { tt.rt.states[s.frame.owner.id].typedAt = time.Time{} }
	pause()
	// The same letter again goes to the next row starting with it.
	var went []int
	for range 4 {
		tt.Key(0, KeyC)
		went = append(went, sel)
	}
	if !slices.Equal(went, []int{2, 3, 5, 2}) {
		t.Fatalf("c, c, c, c went to %v", went)
	}
	pause()
	tt.Key(Shift, KeyD)
	if sel != 4 {
		t.Fatalf("D went to %d", sel)
	}
	// Letters typed quickly add up.
	pause()
	went = went[:0]
	for _, k := range []Key{KeyC, KeyH, KeyE, KeyS} {
		tt.Key(0, k)
		went = append(went, sel)
	}
	if !slices.Equal(went, []int{5, 2, 2, 3}) {
		t.Fatalf("c, h, e, s went to %v", went)
	}
	// Assistive technology reads the label.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleListItem, "Chestnut"); n.Label != "Chestnut" {
		t.Errorf("the row reads %q", n.Label)
	}
}

func TestListTypeToChooseLeavesShortcuts(t *testing.T) {
	names := []string{"Apple", "Banana", "Cherry"}
	sel, j := -1, 0
	s := ListState{Selected: &sel, Label: func(i int) string { return names[i] }}
	tt := coreNewTester(func(c *context) {
		if c.Shortcut(0, KeyJ) {
			j++
		}
		coreList(c, &s, len(names), func(i int) { coreText(c, names[i]).Height(30) }).Grow(1)
	}, 300, 400)
	tt.Click("Apple")
	tt.Key(0, KeyJ)
	tt.Key(0, KeyB)
	if j != 1 || sel != 1 {
		t.Errorf("the window took J %d times; B chose %d", j, sel)
	}
	// A space starts nothing, but goes on what was typed.
	tt.Key(0, KeySpace)
	if st := tt.rt.states[s.frame.owner.id]; st.typed != "b " {
		t.Errorf("typed %q", st.typed)
	}
}

func TestListPageKeys(t *testing.T) {
	sel := -1
	s := ListState{Selected: &sel}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 100, func(i int) { coreTextf(c, "Row %d", i).Height(30) }).Grow(1)
	}, 300, 400)
	tt.Click("Row 1")
	first, last := s.Visible()
	tt.Key(0, KeyPageDown)
	if runtime.GOOS == "darwin" {
		// Page Down scrolls, as in AppKit.
		if f, _ := s.Visible(); sel != 1 || f <= first {
			t.Errorf("Page Down chose %d, scrolled to %d", sel, f)
		}
		return
	}
	if sel != last {
		t.Fatalf("Page Down chose %d, not the last row in view, %d", sel, last)
	}
	tt.Key(0, KeyPageDown)
	if f, l := s.Visible(); sel < last+10 || sel < f || sel > l {
		t.Fatalf("Page Down again chose %d, showing %d to %d", sel, f, l)
	}
	first, _ = s.Visible()
	tt.Key(0, KeyPageUp)
	if sel != first {
		t.Errorf("Page Up chose %d, not the first row in view, %d", sel, first)
	}
}

func TestAccessibilityOfListsChoosingSeveral(t *testing.T) {
	var sel Selection[int]
	s := ListState{Selection: &sel}
	changes := 0
	tt := coreNewTester(severalView(&s, &changes), 300, 400)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tt.Click("Row 2")
	tt.ClickWith(Shift, "Row 4")
	tree := tt.h.access
	if l := accessNode(t, tree, platform.RoleList, ""); l.States&platform.AccessMultiselectable == 0 {
		t.Errorf("the list: %+v", l)
	}
	var chosen []int
	for _, n := range tree.Nodes {
		if n.Role == platform.RoleListItem && n.States&platform.AccessChecked != 0 {
			chosen = append(chosen, n.PosInSet-1)
		}
	}
	if !slices.Equal(chosen, []int{2, 3, 4}) {
		t.Errorf("assistive technology sees %v chosen", chosen)
	}
	// The focus is on the row last chosen.
	if n, ok := byID(tree, tree.Focus); !ok || n.PosInSet != 5 {
		t.Errorf("the focus is on %+v", n)
	}
}

func TestTableChoosesSeveral(t *testing.T) {
	var sel Selection[int]
	s := ListState{Selection: &sel}
	cols := []TableColumn{{Title: "Name"}, {Title: "Size", Width: 80}}
	tt := coreNewTester(func(c *context) {
		coreTable(c, &s, cols, 50, func(row, col int) {
			if col == 0 {
				coreTextf(c, "File %d", row)
			} else {
				coreTextf(c, "%d KB", row)
			}
		}).Grow(1)
	}, 400, 400)
	tt.Click("File 1")
	tt.ClickWith(Shift, "File 3")
	tt.ClickWith(Cmd, "File 6")
	expectChosen(t, &sel, 1, 2, 3, 6)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleTable, ""); n.States&platform.AccessMultiselectable == 0 {
		t.Errorf("the table: %+v", n)
	}
}

func TestSelectionOfAnotherKeyPanics(t *testing.T) {
	var sel Selection[string]
	defer func() {
		if r := recover(); r == nil {
			t.Error("no panic")
		}
	}()
	sel.has(3)
}

func TestListTypeRightAfterAClick(t *testing.T) {
	names := []string{"Apple", "Banana", "Cherry"}
	sel := -1
	s := ListState{Selected: &sel, Label: func(i int) string { return names[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(names), func(i int) { coreText(c, names[i]).Height(30) }).Grow(1)
	}, 300, 400)
	r, _ := tt.Find("Apple")
	x, y := float64(r.X+r.W/2), float64(r.Y+r.H/2)
	// The click and the letter come before the next frame, as from a quick
	// hand: the letter goes on from the row clicked.
	for _, ev := range []platform.SurfaceEvent{
		{Kind: platform.PointerDown, X: x, Y: y},
		{Kind: platform.PointerUp, X: x, Y: y},
		{Kind: platform.KeyPressed, Key: platform.Key(KeyC)},
		{Kind: platform.KeyReleased, Key: platform.Key(KeyC)},
	} {
		tt.rt.event(ev)
	}
	tt.settle()
	if sel != 2 {
		t.Errorf("a click on Apple, then c, chose %d", sel)
	}
}
