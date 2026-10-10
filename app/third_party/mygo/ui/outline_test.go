package ui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// files is a tree of paths: a directory's children, nil for a file.
var files = map[string][]string{
	"src":       {"src/ui", "src/main.go"},
	"src/ui":    {"src/ui/list.go", "src/ui/tree.go"},
	"docs":      {},
	"README.md": nil,
}

func children(path string) []string { return files[path] }

func baseName(path string) string { return path[strings.LastIndexByte(path, '/')+1:] }

func TestOutline(t *testing.T) {
	sel := -1
	var s OutlineState[string]
	s.List.Selected = &sel
	roots := []string{"src", "docs", "README.md"}
	tt := coreNewTester(func(c *context) {
		coreOutline(c, &s, roots, children, func(path string) {
			coreText(c, baseName(path))
		}).Grow(1)
	}, 400, 400)
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	if tt.HasText("main.go") {
		t.Fatal("a closed item shows its children")
	}
	// A click on the arrow opens; the row is not chosen.
	src, _ := tt.Find("src")
	tt.ClickAt(src.X-10, src.Y+src.H/2)
	if !s.Open.Has("src") || !tt.HasText("main.go") || sel != -1 {
		t.Fatalf("a click on the arrow: open %v, shows %q, chose %d", s.Open.Has("src"), tt.Texts(), sel)
	}
	// Right opens the item chosen, then goes to its first child; Left goes
	// to the parent, then closes it.
	tt.Click("ui")
	steps := []struct {
		mods   Modifiers
		key    Key
		chosen string
		open   bool
	}{
		{0, KeyRight, "src/ui", true},
		{0, KeyRight, "src/ui/list.go", true},
		{0, KeyLeft, "src/ui", true},
		{0, KeyLeft, "src/ui", false},
		{0, KeyLeft, "src", false},
		{0, KeyLeft, "src", false},
		{allMod, KeyRight, "src", true},
	}
	for _, step := range steps {
		tt.Key(step.mods, step.key)
		if got := s.Item(sel); got != step.chosen || s.Open.Has("src/ui") != step.open {
			t.Fatalf("%v %v: chose %q, src/ui open %v", step.mods, step.key, got, s.Open.Has("src/ui"))
		}
	}
	// Option opened all inside src, which closes with them.
	tt.Key(allMod, KeyLeft)
	if s.Open.Has("src") || s.Open.Has("src/ui") {
		t.Fatalf("Option-Left left open %v %v", s.Open.Has("src"), s.Open.Has("src/ui"))
	}
	// Closing an item moves the choice to it from a row inside.
	tt.Key(allMod, KeyRight)
	tt.Click("tree.go")
	src, _ = tt.Find("src")
	tt.ClickAt(src.X-10, src.Y+src.H/2)
	if s.Item(sel) != "src" || tt.HasText("tree.go") {
		t.Errorf("after closing src, chose %q", s.Item(sel))
	}
	// Assistive technology: a tree of items with their levels, open or not.
	tt.Key(allMod, KeyRight)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	accessNode(t, tree, platform.RoleTree, "")
	for _, want := range []struct {
		name  string
		level int
		state platform.AccessStates
	}{
		{"src", 1, platform.AccessExpandable | platform.AccessExpanded},
		{"ui", 2, platform.AccessExpandable | platform.AccessExpanded},
		{"list.go", 3, 0},
		{"docs", 1, platform.AccessExpandable},
		{"README.md", 1, 0},
	} {
		n := accessNode(t, tree, platform.RoleTreeItem, want.name)
		if got := n.States & (platform.AccessExpandable | platform.AccessExpanded); n.Level != want.level || got != want.state {
			t.Errorf("%s: level %d, states %b", want.name, n.Level, got)
		}
	}
}

func TestOutlineBuildsWhatShows(t *testing.T) {
	sel := -1
	var s OutlineState[int]
	s.List.Selected = &sel
	const n = 100000
	kids := make([]int, n)
	for i := range kids {
		kids[i] = i + 1
	}
	built := 0
	tt := coreNewTester(func(c *context) {
		coreOutline(c, &s, []int{0}, func(item int) []int {
			if item == 0 {
				return kids
			}
			return nil
		}, func(item int) {
			built++
			coreTextf(c, "Item %d", item)
		}).Grow(1)
	}, 400, 400)
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	s.Open.Add(0)
	tt.Frame()
	built = 0
	tt.Frame()
	if s.Rows() != n+1 || built == 0 || built > 40 {
		t.Fatalf("%d rows, built %d", s.Rows(), built)
	}
	tt.Click("Item 3")
	tt.Key(0, KeyEnd)
	if sel != n || !tt.HasText(fmt.Sprintf("Item %d", n)) {
		t.Errorf("End chose %d", sel)
	}
	// The choice follows its item as rows close above it.
	tt.Key(0, KeyHome)
	tt.Key(0, KeyDown)
	tt.Key(0, KeyDown)
	if s.Item(sel) != 2 {
		t.Fatalf("chose %d", s.Item(sel))
	}
}

func TestOutlineTable(t *testing.T) {
	sel := -1
	sort := SortOrder{Column: "Name"}
	var s OutlineState[string]
	s.List.Selected, s.List.Sort = &sel, &sort
	roots := []string{"src", "docs", "README.md"}
	cols := []TableColumn{{Title: "Name", Sortable: true}, {Title: "Kind", Width: 100}}
	tt := coreNewTester(func(c *context) {
		coreOutlineTable(c, &s, cols, roots, children, func(path string, col int) {
			if col == 0 {
				coreText(c, baseName(path)).SingleLine()
			} else if files[path] != nil {
				coreText(c, "Folder")
			} else {
				coreText(c, "File")
			}
		}).Grow(1)
	}, 400, 400)
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	tt.Click("src")
	tt.Key(0, KeyRight)
	tt.Key(0, KeyRight)
	tt.Key(0, KeyRight)
	if s.Item(sel) != "src/ui" || !s.Open.Has("src/ui") || !tt.HasText("list.go") {
		t.Fatalf("chose %q", s.Item(sel))
	}
	// The first column is indented as its items are deep.
	ui, _ := tt.Find("ui")
	srcText, _ := tt.Find("src")
	if ui.X-srcText.X != 16 {
		t.Errorf("ui is %v right of src", ui.X-srcText.X)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	accessNode(t, tree, platform.RoleColumnHeader, "Name")
	if n := accessNode(t, tree, platform.RoleTreeItem, "ui"); n.Level != 2 || n.States&platform.AccessChecked == 0 {
		t.Errorf("ui: %+v", n)
	}
	// Sorting is the app's, the header shows it.
	tt.Click("Name")
	if !sort.Descending {
		t.Error("a click on Name did not reverse the order")
	}
}

func TestOutlineExpandsForAssistiveTechnology(t *testing.T) {
	sel := -1
	var s OutlineState[string]
	s.List.Selected = &sel
	open := false
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			coreOutline(c, &s, []string{"src", "README.md"}, children, func(path string) {
				coreText(c, baseName(path))
			}).Grow(1)
			coreTree(c, func() {
				coreTreeItem(c, "lib", &open, func() { coreTreeItem(c, "util.go", nil, nil) })
			})
		})
	}, 400, 400)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	act := func(name string, a platform.AccessActionKind) {
		t.Helper()
		n := accessNode(t, tt.h.access, platform.RoleTreeItem, name)
		if n.Actions&platform.ActionExpand == 0 {
			t.Fatalf("%s cannot expand: %+v", name, n)
		}
		tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: n.ID, Action: a})
	}
	if n := accessNode(t, tt.h.access, platform.RoleTreeItem, "README.md"); n.Actions&platform.ActionExpand != 0 {
		t.Errorf("a file can expand")
	}
	// Opening and closing, not choosing, which pressing does.
	act("src", platform.AccessExpand)
	if !s.Open.Has("src") || sel != -1 || !tt.HasText("main.go") {
		t.Fatalf("expanding src: open %v, chose %d", s.Open.Has("src"), sel)
	}
	act("src", platform.AccessCollapse)
	if s.Open.Has("src") {
		t.Fatal("collapsing src left it open")
	}
	act("lib", platform.AccessExpand)
	if !open || !tt.HasText("util.go") {
		t.Fatal("expanding a tree's item")
	}
}
