package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestSidebarNavigation(t *testing.T) {
	g := &gallery{router: ui.NewRouter("/overview"), sectionsOpen: [2]bool{true, true}}
	g.router.Transition = ui.TransitionNone
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			g.sidebar(c)
			ui.Column(c).Grow(1).Children(func() {
				g.toolbar(c)
				g.router.View(c, func(r *ui.Route) {
					r.Title(pageOf(r.Path()))
					ui.Text(c, "Page: "+r.Path())
				})
			})
		})
	}, 980, 720)
	check := func(want string) {
		t.Helper()
		if got := g.router.Path(); got != want || !tt.HasText("Page: "+want) {
			t.Fatalf("want %s, router at %s, texts %q", want, got, tt.Texts())
		}
	}
	click := func(label string) {
		t.Helper()
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	for _, page := range pages {
		click(page)
		check(pagePath(page))
	}
	// Clicking the current page adds no history entry.
	click("Motion")
	click("Back")
	check("/overlays")
	click("Forward")
	check("/motion")
	// Arrow navigation uses the selection that survived the click's rebuild.
	click("Text")
	tt.Key(0, ui.KeyDown)
	check("/list")
	// History and other navigation synchronize the sidebar's selection.
	g.router.Back()
	tt.Frame()
	check("/text")
	tt.Key(0, ui.KeyDown)
	check("/list")
	g.router.Push("/list/42")
	tt.Frame()
	tt.Key(0, ui.KeyDown)
	check("/styling")
}

func TestRowStarAndBreadcrumb(t *testing.T) {
	g := &gallery{router: ui.NewRouter("/list/42"), starred: map[int]bool{}}
	g.router.Transition = ui.TransitionNone
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Fill().Children(func() {
			g.toolbar(c)
			g.router.View(c, func(r *ui.Route) {
				if r.Match("/list/{row}") {
					g.listRow(c, r)
				}
			})
		})
	}, 600, 400)
	for _, want := range []bool{true, false} {
		if err := tt.Click("Starred"); err != nil {
			t.Fatal(err)
		}
		if g.starred[42] != want {
			t.Fatalf("row's star = %v, want %v", g.starred[42], want)
		}
	}
	if err := tt.Click("List"); err != nil {
		t.Fatal(err)
	}
	if got := g.router.Path(); got != "/list" {
		t.Fatalf("breadcrumb went to %q", got)
	}
}

func TestListDerivedSelection(t *testing.T) {
	g := &gallery{router: ui.NewRouter("/list"), picked: -1, tree: map[string]bool{}, starred: map[int]bool{}}
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Scroll(c).Fill().Children(func() { g.list(c) })
	}, 980, 720)
	if err := tt.Click("Row 2"); err != nil {
		t.Fatal(err)
	}
	if g.picked != 2 {
		t.Fatalf("click picked %d", g.picked)
	}
	tt.Key(0, ui.KeyDown)
	if g.picked != 3 {
		t.Fatalf("Down picked %d", g.picked)
	}
	if err := tt.Click("Filter"); err != nil {
		t.Fatal(err)
	}
	tt.Type("99")
	if err := tt.Click("Row 99"); err != nil {
		t.Fatal(err)
	}
	if g.picked != 99 {
		t.Fatalf("filtered row picked %d", g.picked)
	}
	tt.Key(0, ui.KeyEnter)
	if got := g.router.Path(); got != "/list/99" {
		t.Fatalf("Enter went to %q", got)
	}
}
