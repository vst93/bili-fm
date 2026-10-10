package ui

import (
	"runtime"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// tableView shows a table of files: name, kind and size.
func tableView(s *ListState, cols []TableColumn, names []string) func(c *context) {
	return func(c *context) {
		coreColumn(c).Fill().Padding(10).Children(func() {
			coreTable(c, s, cols, len(names), func(row, col int) {
				switch col {
				case 0:
					coreText(c, names[row]).SingleLine()
				case 1:
					coreText(c, "Document")
				case 2:
					coreTextf(c, "%d KB", row+1)
				}
			}).Grow(1)
		})
	}
}

func TestTableSort(t *testing.T) {
	sort := SortOrder{Column: "Size"}
	sel := -1
	s := ListState{Sort: &sort, Selected: &sel}
	cols := []TableColumn{{Title: "Name", Sortable: true}, {Title: "Kind", Width: 100}, {Title: "Size", Width: 80, Align: End, Sortable: true}}
	tt := coreNewTester(tableView(&s, cols, []string{"a.txt", "b.txt"}), 500, 300)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleColumnHeader, "Size"); n.States&platform.AccessSortAscending == 0 {
		t.Errorf("the column sorted by: %+v", n)
	}
	tt.Click("Name")
	if sort != (SortOrder{Column: "Name"}) {
		t.Fatalf("a click on Name: %+v", sort)
	}
	tt.Click("Name")
	if sort != (SortOrder{Column: "Name", Descending: true}) {
		t.Fatalf("a second click on Name: %+v", sort)
	}
	tree := tt.h.access
	if n := accessNode(t, tree, platform.RoleColumnHeader, "Name"); n.States&platform.AccessSortDescending == 0 || n.Actions&platform.ActionPress == 0 {
		t.Errorf("Name, sorted descending: %+v", n)
	}
	if n := accessNode(t, tree, platform.RoleColumnHeader, "Size"); n.States&(platform.AccessSortAscending|platform.AccessSortDescending) != 0 {
		t.Errorf("Size, no longer sorted by: %+v", n)
	}
	// A column that does not sort.
	tt.Click("Kind")
	if sort != (SortOrder{Column: "Name", Descending: true}) || sel != -1 {
		t.Errorf("a click on Kind: %+v, chose %d", sort, sel)
	}
}

// header returns the box of a column's header.
func header(t *testing.T, tt *Tester, title string) platform.RectF {
	t.Helper()
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	return accessNode(t, tt.h.access, platform.RoleColumnHeader, title).Bounds
}

func TestTableResize(t *testing.T) {
	s := ListState{}
	cols := []TableColumn{{Title: "Name"}, {Title: "Kind", Width: 100, MinWidth: 60}, {Title: "Size", Width: 80, Fixed: true}}
	tt := coreNewTester(tableView(&s, cols, []string{"a.txt", "a much longer name.txt"}), 500, 300)
	kind := header(t, tt, "Kind")
	// Dragging the right edge of Kind's header.
	edge := float32(kind.X+kind.W) - 2
	tt.Press(edge, float32(kind.Y+kind.H/2))
	tt.Move(edge+30, float32(kind.Y+kind.H/2))
	tt.Release(edge+30, float32(kind.Y+kind.H/2))
	if w, h := s.Columns.Widths["Kind"], header(t, tt, "Kind"); w != 130 || h.W != 130 || float32(h.X+h.W)-2 != edge+30 {
		t.Fatalf("dragged 30 DIPs wider: %v, the header %v", w, h)
	}
	// The column before it, sharing the room left, kept its width, for the
	// edge to follow the pointer.
	if w := s.Columns.Widths["Name"]; w != 300 {
		t.Errorf("Name took %v", w)
	}
	edge += 30
	// No narrower than its least.
	tt.Press(edge, float32(kind.Y+kind.H/2))
	tt.Move(edge-130, float32(kind.Y+kind.H/2))
	tt.Release(edge-130, float32(kind.Y+kind.H/2))
	if w := header(t, tt, "Kind").W; w != 60 {
		t.Errorf("dragged narrower than its least: %v", w)
	}
	// A double click fits it to its cells, and the cells follow.
	kind = header(t, tt, "Kind")
	edge = float32(kind.X+kind.W) - 2
	tt.ClickAt(edge, float32(kind.Y+kind.H/2))
	tt.ClickAt(edge, float32(kind.Y+kind.H/2))
	doc, _ := tt.Find("Document")
	if w := float32(header(t, tt, "Kind").W); w < doc.W+20 || w > doc.W+21 || w < 70 {
		t.Errorf("fitted to %v for a text of %v", w, doc.W)
	}
	// A fixed column has no edge to drag.
	size := header(t, tt, "Size")
	tt.Press(float32(size.X+size.W-1), float32(size.Y+size.H/2))
	tt.Move(float32(size.X+size.W+40), float32(size.Y+size.H/2))
	tt.Release(float32(size.X+size.W+40), float32(size.Y+size.H/2))
	if w := header(t, tt, "Size").W; w != 80 {
		t.Errorf("a fixed column resized to %v", w)
	}
}

func TestTableReorder(t *testing.T) {
	sort := SortOrder{}
	s := ListState{Sort: &sort}
	cols := []TableColumn{{Title: "Name", Fixed: true}, {Title: "Kind", Width: 100, Sortable: true}, {Title: "Size", Width: 80}}
	tt := coreNewTester(tableView(&s, cols, []string{"a.txt"}), 500, 300)
	kind := header(t, tt, "Kind")
	y := float32(kind.Y + kind.H/2)
	// Kind dragged past the middle of Size.
	x := float32(kind.X + 20)
	tt.Press(x, y)
	for _, d := range []float32{5, 20, 50, 60} {
		tt.Move(x+d, y)
	}
	tt.Release(x+60, y)
	if got := s.Columns.Order; len(got) != 3 || got[1] != "Size" || got[2] != "Kind" {
		t.Fatalf("the order: %q", got)
	}
	if sort.Column != "" {
		t.Errorf("dragging the header sorted by %q", sort.Column)
	}
	kind, size := header(t, tt, "Kind"), header(t, tt, "Size")
	doc, _ := tt.Find("Document")
	if !(size.X < kind.X) || float64(doc.X) < kind.X {
		t.Errorf("Size at %v, Kind at %v, its cell at %v", size.X, kind.X, doc.X)
	}
	// Name is fixed: it neither moves nor lets others past it.
	tt.Press(float32(size.X+10), y)
	for _, d := range []float32{-5, -40, -100, -200} {
		tt.Move(float32(size.X+10)+d, y)
	}
	tt.Release(float32(size.X+10)-200, y)
	if got := s.Columns.Order; got[0] != "Name" {
		t.Errorf("moved past a fixed column: %q", got)
	}
}

func TestTableScrollsSideways(t *testing.T) {
	s := ListState{}
	cols := []TableColumn{{Title: "Name", Width: 200}, {Title: "Kind", Width: 200}, {Title: "Size", Width: 200}}
	names := make([]string, 50)
	for i := range names {
		names[i] = "file"
	}
	tt := coreNewTester(tableView(&s, cols, names), 400, 300)
	if tt.HasText("Size") {
		size := header(t, tt, "Size")
		if size.X < 390 {
			t.Fatalf("the third column of 200 shows at %v in a table 380 wide", size.X)
		}
	}
	tt.Scroll(200, 150, 300, 0)
	size := header(t, tt, "Size")
	cell, _ := tt.Find("1 KB")
	if size.X > 300 || float64(cell.X) != size.X+10 {
		t.Errorf("scrolled sideways: the header at %v, the text of its cell at %v", size.X, cell.X)
	}
	// Down still moves the rows, and the header stays.
	before := size.Y
	tt.Scroll(200, 150, 0, 300)
	if size := header(t, tt, "Size"); size.Y != before || s.first == 0 {
		t.Errorf("scrolled down: the header at %v (was %v), first row %d", size.Y, before, s.first)
	}
}

func TestEditableTextInTable(t *testing.T) {
	names := []string{"notes.txt", "photo.jpeg", "README"}
	sel, opened, renames := -1, -1, 0
	s := ListState{Selected: &sel}
	cols := []TableColumn{{Title: "Name"}, {Title: "Size", Width: 80}}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Padding(10).Children(func() {
			if coreTable(c, &s, cols, len(names), func(row, col int) {
				if col == 0 {
					if coreEditableText(c, &names[row]).Changed() {
						renames++
					}
				} else {
					coreText(c, "1 KB")
				}
			}).Grow(1).Submitted() {
				opened = sel
			}
		})
	}, 400, 300)
	rename := func() {
		if runtime.GOOS == "darwin" {
			tt.Key(0, KeyEnter)
		} else {
			tt.Key(0, KeyF2)
		}
	}
	tt.Click("photo.jpeg")
	rename()
	if opened != -1 {
		t.Fatalf("the key editing the name opened the row")
	}
	// The name before its extension is selected: typing replaces it.
	tt.Type("beach")
	tt.Key(0, KeyEnter)
	if names[1] != "beach.jpeg" || renames != 1 {
		t.Fatalf("renamed to %q (%d)", names[1], renames)
	}
	if !tt.Focused("beach.jpeg") {
		t.Error("the table did not take the focus back")
	}
	// Escape goes back.
	tt.Key(0, KeyF2)
	tt.Type("gone")
	tt.Key(0, KeyEscape)
	if names[1] != "beach.jpeg" || !tt.HasText("beach.jpeg") {
		t.Errorf("Escape left %q", names[1])
	}
	// A click on the name of the row chosen edits it, once a double click
	// is out of the question; the focus leaving keeps what was typed.
	tt.Click("beach.jpeg")
	if tt.HasText("beach.jpeg") != true {
		t.Fatal("the click edited at once")
	}
	time.Sleep(renameDelay + 50*time.Millisecond)
	tt.Frame()
	tt.Type("sea")
	tt.Click("notes.txt")
	if names[1] != "sea.jpeg" || sel != 0 {
		t.Errorf("after a click elsewhere: %q, chose %d", names[1], sel)
	}
	// A double click opens, and does not edit.
	tt.Click("notes.txt")
	tt.Click("notes.txt")
	time.Sleep(renameDelay + 50*time.Millisecond)
	tt.Frame()
	tt.Type("x")
	if opened != 0 || names[0] != "notes.txt" {
		t.Errorf("a double click: opened %d, the name %q", opened, names[0])
	}
}

func TestEditableText(t *testing.T) {
	title := "Untitled.md"
	changes := 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(8).Children(func() {
			if coreEditableText(c, &title).Changed() {
				changes++
			}
			coreButton(c, "Other")
		})
	}, 400, 300)
	r, _ := tt.Find("Untitled.md")
	tt.ClickAt(r.X+5, r.Y+5)
	tt.ClickAt(r.X+5, r.Y+5)
	tt.Type("Notes")
	tt.Key(0, KeyTab)
	if title != "Notes.md" || changes != 1 {
		t.Fatalf("after Tab: %q (%d)", title, changes)
	}
	tt.Click("Notes.md")
	tt.Key(0, KeyEnter)
	tt.Key(Cmd, KeyA)
	tt.Type("Plan")
	tt.Key(0, KeyEnter)
	if title != "Plan" || !tt.Focused("Plan") {
		t.Errorf("after Enter: %q, focused %v", title, tt.Focused("Plan"))
	}
}
