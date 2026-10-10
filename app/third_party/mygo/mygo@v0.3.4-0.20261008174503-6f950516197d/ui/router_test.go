package ui

import (
	"fmt"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestRouteMatch(t *testing.T) {
	route := func(p string) *Route {
		r := &Route{}
		for _, s := range pathParts(p) {
			r.segments = append(r.segments, s)
		}
		return r
	}
	for _, c := range []struct {
		path, pattern string
		match         bool
		params        map[string]string
	}{
		{"/", "/", true, nil},
		{"/", "/notes", false, nil},
		{"/notes", "/notes", true, nil},
		{"/notes", "/notes/", true, nil},
		{"/notes/42", "/notes", false, nil},
		{"/notes/42", "/notes/{id}", true, map[string]string{"id": "42"}},
		{"/notes/42/edit", "/notes/{id}", false, nil},
		{"/notes/42/edit", "/notes/{id}/edit", true, map[string]string{"id": "42"}},
		{"/files", "/files/{path...}", true, map[string]string{"path": ""}},
		{"/files/docs/a.txt", "/files/{path...}", true, map[string]string{"path": "docs/a.txt"}},
		{"/users/ada/files/x", "/users/{name}/files/{rest...}", true, map[string]string{"name": "ada", "rest": "x"}},
		{"/a b", "/a b", true, nil},
	} {
		r := route(c.path)
		if got := r.Match(c.pattern); got != c.match {
			t.Errorf("%q matching %q: %v", c.path, c.pattern, got)
			continue
		}
		for k, v := range c.params {
			if got := r.Param(k); got != v {
				t.Errorf("%q matching %q: %s is %q", c.path, c.pattern, k, got)
			}
		}
	}
	// A pattern that does not match leaves the parameters of the last that
	// did.
	r := route("/notes/42")
	r.Match("/notes/{id}")
	if r.Match("/notes/{id}/edit") || r.Param("id") != "42" {
		t.Errorf("after a miss, id is %q", r.Param("id"))
	}
	defer func() {
		if recover() == nil {
			t.Error("{rest...} before the end did not panic")
		}
	}()
	r.Match("/{rest...}/edit")
}

func TestRouterHistory(t *testing.T) {
	var zero Router
	if zero.Path() != "/" || zero.CanGoBack() || zero.CanGoForward() {
		t.Fatalf("the zero router: %q", zero.Path())
	}
	r := NewRouter("/notes")
	steps := []struct {
		do        func()
		loc       string
		back, fwd bool
	}{
		{func() {}, "/notes", false, false},
		{func() { r.Push("/notes/42") }, "/notes/42", true, false},
		// Relative to the page, as links in web pages.
		{func() { r.Push("edit") }, "/notes/edit", true, false},
		{func() { r.Push("../settings/") }, "/settings", true, false},
		{func() { r.Push("?tab=fonts") }, "/settings?tab=fonts", true, false},
		{func() { r.Push("/settings?tab=fonts") }, "/settings?tab=fonts", true, false}, // the page shown
		{func() { r.Back() }, "/settings", true, true},
		{func() { r.Go(-2) }, "/notes/42", true, true},
		{func() { r.Forward() }, "/notes/edit", true, true},
		// Pushing forgets the pages after.
		{func() { r.Push("/a%20b") }, "/a%20b", true, false},
		{func() { r.Replace("/c") }, "/c", true, false},
		{func() { r.Back() }, "/notes/edit", true, true},
		{func() { r.Go(-10) }, "/notes", false, true},
		{func() { r.Go(10) }, "/c", true, false},
	}
	for i, s := range steps {
		s.do()
		if r.Location() != s.loc || r.CanGoBack() != s.back || r.CanGoForward() != s.fwd {
			t.Fatalf("step %d: at %q, back %v, forward %v", i, r.Location(), r.CanGoBack(), r.CanGoForward())
		}
	}
	r.Push("/search?q=go%20ui")
	if r.Path() != "/search" || r.Query("q") != "go ui" {
		t.Errorf("path %q, q %q", r.Path(), r.Query("q"))
	}
	r.Push("/a%20b")
	if r.Path() != "/a b" {
		t.Errorf("an escaped path: %q", r.Path())
	}
	// Pages of the same path are one, with their state; replacing with
	// another path is a new page.
	page := r.pageAt(r.current(), 0)
	r.Push("?x=1")
	r.Replace("?x=2")
	if r.pageAt(r.current(), 0) != page {
		t.Error("another query made another page")
	}
	r.Replace("/other")
	if r.pageAt(r.current(), 0) == page {
		t.Error("replacing with another path kept the page")
	}
	// The history holds a hundred entries.
	for i := range 150 {
		r.Push(fmt.Sprintf("/n/%d", i))
	}
	r.Go(-1000)
	if len(r.entries) != maxHistory || r.Location() != "/n/50" {
		t.Errorf("%d entries, the first %q", len(r.entries), r.Location())
	}
}

// backKey and forwardKey go back and forward in a router.
func backKey() (Modifiers, Key) {
	if runtime.GOOS == "darwin" {
		return Super, KeyBracketLeft
	}
	return Alt, KeyLeft
}

func forwardKey() (Modifiers, Key) {
	if runtime.GOOS == "darwin" {
		return Super, KeyBracketRight
	}
	return Alt, KeyRight
}

func TestRouterView(t *testing.T) {
	r := NewRouter("/")
	r.Transition = TransitionNone
	var note string
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			coreRow(c).Children(func() {
				coreBackButton(c, r)
				coreForwardButton(c, r)
			})
			r.coreView(c, func(rt *Route) {
				switch {
				case rt.Match("/"):
					rt.Title("Home")
					coreText(c, "Welcome")
					coreLink(c, "First note", "/notes/1")
					coreLink(c, "Site", "https://example.com")
				case rt.Match("/notes/{id}"):
					note = rt.Param("id")
					rt.Title("Note " + note)
					coreText(c, "Note "+note)
					coreLink(c, "Next", fmt.Sprint(atoi(note)+1))
				default:
					coreText(c, "Not found")
				}
			})
		})
	}, 400, 300)
	if !tt.HasText("Welcome") || r.Title() != "Home" {
		t.Fatalf("shows %q, titled %q", tt.Texts(), r.Title())
	}
	// Links go to paths in the router, and open URLs.
	tt.Click("Site")
	tt.Click("First note")
	if !tt.HasText("Note 1") || tt.HasText("Welcome") || r.Location() != "/notes/1" {
		t.Fatalf("after the link: %q at %q", tt.Texts(), r.Location())
	}
	if got := tt.OpenedURLs(); !slices.Equal(got, []string{"https://example.com"}) {
		t.Errorf("opened %q", got)
	}
	tt.Click("Next")
	tt.Click("Next")
	if note != "3" {
		t.Fatalf("two relative links: note %q", note)
	}
	// The keys and the buttons go back and forward.
	tt.Key(backKey())
	if note != "2" {
		t.Fatalf("back: note %q", note)
	}
	tt.Key(0, KeyBack)
	if note != "1" {
		t.Fatalf("the back button of a mouse: note %q", note)
	}
	tt.Key(forwardKey())
	tt.Key(0, KeyForward)
	if note != "3" || r.CanGoForward() {
		t.Fatalf("forward twice: note %q", note)
	}
	tt.Click("Back")
	if note != "2" {
		t.Fatalf("the Back button: note %q", note)
	}
	tt.Click("Forward")
	if note != "3" {
		t.Fatalf("the Forward button: note %q", note)
	}
	// A right click lists the pages before.
	tt.RightClick("Back")
	if got := tt.Menu(); !slices.Equal(got, []string{"Note 2", "Note 1", "Home"}) {
		t.Fatalf("the Back button's menu: %q", got)
	}
	if err := tt.ChooseMenuItem("Home"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Welcome") || r.CanGoBack() {
		t.Fatalf("after choosing Home: %q", tt.Texts())
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleButton, "Back"); n.States&platform.AccessDisabled == 0 {
		t.Error("Back is not disabled at the start of the history")
	}
	accessNode(t, tt.h.access, platform.RoleGroup, "Home")
	// Pages change from outside the frame, as after a menu item.
	r.Push("/nowhere")
	tt.Frame()
	if !tt.HasText("Not found") {
		t.Errorf("a push outside the frame shows %q", tt.Texts())
	}
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func TestRouterKeepsPages(t *testing.T) {
	r := NewRouter("/list")
	r.Transition = TransitionNone
	var scroll *node
	tt := coreNewTester(func(c *context) {
		r.coreView(c, func(rt *Route) {
			switch {
			case rt.Match("/list"):
				scroll = coreScroll(c).Grow(1)
				scroll.Children(func() {
					for i := range 50 {
						coreTextf(c, "Row %d", i)
					}
				})
				count := coreLocal(scroll, "count", func() int { return 0 })
				if coreButton(c, fmt.Sprintf("Clicked %d", *count)).Clicked() {
					*count++
				}
			default:
				coreText(c, "Page "+rt.Path())
			}
		})
	}, 400, 300)
	tt.Scroll(100, 100, 0, 300)
	tt.Click("Clicked 0")
	if scroll.st.scrollY != 300 || !tt.HasText("Clicked 1") {
		t.Fatalf("scrolled to %v, %q", scroll.st.scrollY, tt.Texts())
	}
	// Away and back: the page is as it was.
	r.Push("/other")
	tt.Frame()
	r.Back()
	tt.Frame()
	if scroll.st.scrollY != 300 || !tt.HasText("Clicked 1") {
		t.Fatalf("back, scrolled to %v, %q", scroll.st.scrollY, tt.Texts())
	}
	// Ten pages away, it is too far to keep.
	for i := range keptPages + 1 {
		r.Push(fmt.Sprintf("/p/%d", i))
		tt.Frame()
	}
	r.Go(-(keptPages + 1))
	tt.Frame()
	if scroll.st.scrollY != 0 || !tt.HasText("Clicked 0") {
		t.Fatalf("far back, scrolled to %v, %q", scroll.st.scrollY, tt.Texts())
	}
	// A page replaced is gone.
	tt.Click("Clicked 0")
	r.Replace("/elsewhere")
	tt.Frame()
	r.Replace("/list")
	tt.Frame()
	if !tt.HasText("Clicked 0") {
		t.Fatalf("a page replaced kept its state: %q", tt.Texts())
	}
}

func TestRouterFocus(t *testing.T) {
	r := NewRouter("/")
	r.Transition = TransitionNone
	var name string
	tt := coreNewTester(func(c *context) {
		coreRow(c).Fill().Children(func() {
			coreColumn(c).Children(func() {
				if coreButton(c, "Open settings").Clicked() {
					r.Push("/settings")
				}
			})
			r.coreView(c, func(rt *Route) {
				switch {
				case rt.Match("/"):
					rt.Title("Home")
					coreButton(c, "First")
					coreLink(c, "Profile", "/profile")
				case rt.Match("/profile"):
					rt.Title("Your profile")
					coreTextInput(c, &name).AutoFocus()
				case rt.Match("/settings"):
					rt.Title("Settings")
					coreButton(c, "Reset")
					coreLink(c, "Fonts", "fonts")
				case rt.Match("/fonts"):
					rt.Title("Font settings")
					coreButton(c, "Bigger")
				}
			})
		})
	}, 500, 300)
	// A link in the page: the focus goes into the new page, to the field
	// that takes it there.
	tt.Click("First")
	tt.Key(0, KeyTab)
	if !tt.Focused("Profile") {
		t.Fatal("Tab did not reach the link")
	}
	tt.Key(0, KeyEnter)
	tt.Type("Ada")
	if name != "Ada" {
		t.Fatalf("the new page's field did not take the focus: %q", name)
	}
	// Back, the focus is where it was; forward, where it was there.
	tt.Key(0, KeyBack)
	if !tt.Focused("Profile") {
		t.Fatal("back, the focus is not on the link")
	}
	tt.Key(0, KeyForward)
	tt.Type("!")
	if name != "Ada!" {
		t.Fatalf("forward, the field does not have the focus: %q", name)
	}
	tt.Key(backKey())
	// A button outside: the focus stays on it, and screen readers hear the
	// page's title.
	tt.Announcements()
	tt.Click("Open settings")
	if !tt.Focused("Open settings") || !tt.HasText("Reset") {
		t.Fatal("the button outside lost the focus")
	}
	if got := tt.Announcements(); !slices.Equal(got, []string{"Settings"}) {
		t.Errorf("announced %q", got)
	}
	tt.Click("Reset")
	tt.Key(backKey())
	if !tt.Focused("Profile") {
		t.Fatal("back to Home, the focus is not on the link it left from")
	}
	tt.Key(forwardKey())
	if !tt.Focused("Reset") {
		t.Fatal("forward, the focus is not on Reset")
	}
	// To a page whose elements do not take the focus, it goes to the page,
	// which Tab goes into, and Shift+Tab past.
	tt.Key(0, KeyTab)
	tt.Key(0, KeyEnter)
	if s := tt.rt.states[tt.rt.focused]; !tt.Focused("Font settings") || s == nil || s.flags&flagPage == 0 {
		t.Fatal("the focus is not on the new page")
	}
	if got := tt.Announcements(); len(got) != 0 {
		t.Errorf("with the focus moving, announced %q", got)
	}
	tt.Key(0, KeyTab)
	if !tt.Focused("Bigger") {
		t.Fatal("Tab from the page did not go into it")
	}
	tt.Key(Shift, KeyTab)
	if !tt.Focused("Open settings") {
		t.Fatal("Shift+Tab did not go past the page")
	}
	// Screen readers see the page, named by its title.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	accessNode(t, tt.h.access, platform.RoleGroup, "Font settings")
}

func TestRouterTransition(t *testing.T) {
	r := NewRouter("/notes")
	var shortcuts int
	tt := coreNewTester(func(c *context) {
		r.coreView(c, func(rt *Route) {
			coreText(c, "Page "+rt.Path())
			if c.Shortcut(Cmd, KeyS) {
				shortcuts++
			}
		})
	}, 400, 300)
	partway := func() {
		time.Sleep(60 * time.Millisecond)
		tt.Frame()
	}
	settle := func() {
		time.Sleep(300 * time.Millisecond)
		tt.Frame()
	}
	// at returns where the text s is, unclipped.
	at := func(s string) float32 {
		for _, n := range tt.rt.labels {
			if n.text == s {
				return tt.rt.states[n.id].x
			}
		}
		t.Fatalf("no %q among %q", s, tt.Texts())
		return 0
	}
	// Deeper, the page slides in from the right, over the page going away,
	// which takes neither the pointer nor shortcuts.
	r.Push("/notes/1")
	tt.Frame()
	partway()
	if x := at("Page /notes/1"); x <= 0 || x >= 400 {
		t.Fatalf("sliding in, at %v", x)
	}
	if got := tt.Texts(); !slices.Equal(got, []string{"Page /notes/1"}) {
		t.Errorf("the page going away has text to find: %q", got)
	}
	if n := len(tt.rt.c.root.first.children()); n != 2 {
		t.Errorf("%d pages built", n)
	}
	tt.Key(Cmd, KeyS)
	if shortcuts != 1 {
		t.Errorf("Cmd+S ran %d shortcuts", shortcuts)
	}
	settle()
	if x := at("Page /notes/1"); x != 0 {
		t.Fatalf("in, at %v", x)
	}
	// Back up, it comes from the left.
	r.Back()
	tt.Frame()
	partway()
	if x := at("Page /notes"); x >= 0 {
		t.Fatalf("coming back, at %v", x)
	}
	settle()
	// Across, it fades in where it is, in place of the other.
	r.Push("/settings")
	tt.Frame()
	partway()
	pages := tt.rt.c.root.first.children()
	if x := at("Page /settings"); x != 0 || len(pages) != 1 || pages[0].opacity <= 0 || pages[0].opacity >= 1 {
		t.Fatalf("fading in, at %v, %d pages", x, len(pages))
	}
	settle()
	if pages := tt.rt.c.root.first.children(); len(pages) != 1 || pages[0].opacitySet && pages[0].opacity != 1 {
		t.Fatal("the page did not end opaque")
	}
	// With less motion, deeper pages fade in too.
	tt.SetPreferences(Preferences{ReduceMotion: true, TextScale: 1})
	r.Push("/settings/fonts")
	tt.Frame()
	partway()
	if x := at("Page /settings/fonts"); x != 0 {
		t.Fatalf("with less motion, at %v", x)
	}
	settle()
	// None: at once.
	r.Transition = TransitionNone
	r.Back()
	tt.Frame()
	if len(tt.rt.c.root.first.children()) != 1 {
		t.Fatal("without a transition, two pages")
	}
}

// children returns the children of an element.
func (e *node) children() []*node {
	var out []*node
	for ch := e.first; ch != nil; ch = ch.next {
		out = append(out, ch)
	}
	return out
}

func TestRouterNested(t *testing.T) {
	outer, inner := NewRouter("/settings"), NewRouter("/general")
	outer.Transition, inner.Transition = TransitionNone, TransitionNone
	tt := coreNewTester(func(c *context) {
		outer.coreView(c, func(r *Route) {
			switch {
			case r.Match("/settings"):
				coreText(c, "Settings")
				inner.coreView(c, func(r *Route) {
					coreText(c, "Section "+r.Path())
					coreButton(c, "Inside "+r.Path())
				})
			default:
				coreText(c, "Elsewhere")
			}
		})
	}, 400, 300)
	inner.Push("/fonts")
	tt.Frame()
	// The focus in the inner router: its keys go back in it.
	tt.Click("Inside /fonts")
	tt.Key(backKey())
	if inner.Path() != "/general" || outer.Path() != "/settings" {
		t.Fatalf("back in the inner router: %q, %q", outer.Path(), inner.Path())
	}
	inner.Forward()
	tt.Frame()
	// Away from the outer page and back: the inner one is as it was.
	tt.Click("Inside /fonts")
	focused := tt.rt.focused
	outer.Push("/away")
	tt.Frame()
	tt.Key(backKey())
	if !tt.HasText("Section /fonts") || tt.rt.focused != focused {
		t.Fatalf("back in the outer router: %q", tt.Texts())
	}
}

func TestAnnounce(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		if coreButton(c, "Save").Clicked() {
			c.Toast("Saved")
			c.Announce("3 files saved")
		}
	}, 300, 200)
	// Assistive technology gets them with the tree, once.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tt.Click("Save")
	tt.Frame()
	if got := tt.h.told; !slices.Equal(got, []string{"Saved", "3 files saved"}) {
		t.Errorf("the trees announce %q", got)
	}
	if got := tt.Announcements(); !slices.Equal(got, []string{"Saved", "3 files saved"}) {
		t.Errorf("announced %q", got)
	}
	tt.Frame()
	if len(tt.Announcements()) != 0 {
		t.Error("announced again")
	}
}

func TestEditorLeavesHistoryKeys(t *testing.T) {
	r := NewRouter("/a")
	r.Transition = TransitionNone
	text := ""
	tt := coreNewTester(func(c *context) {
		r.coreView(c, func(rt *Route) {
			coreTextInput(c, &text)
		})
	}, 300, 200)
	r.Push("/b")
	tt.Frame()
	tt.Key(0, KeyTab)
	tt.Type("hi")
	tt.Key(backKey())
	if r.Path() != "/a" {
		t.Errorf("from a text input, the back keys left %q", r.Path())
	}
	tt.Key(0, KeyForward)
	if r.Path() != "/b" {
		t.Errorf("from a text input, the forward key left %q", r.Path())
	}
}

func TestRouterLayouts(t *testing.T) {
	r := NewRouter("/settings/general")
	var outer, inner *node
	var section, what string
	tt := coreNewTester(func(c *context) {
		outer = r.coreView(c, func(rt *Route) {
			switch {
			case rt.Match("/settings/{section...}"):
				rt.Title("Settings")
				coreRow(c).Grow(1).AlignItems(Stretch).Children(func() {
					side := coreColumn(c).Width(140)
					side.Children(func() {
						clicks := coreLocal(side, "clicks", func() int { return 0 })
						if coreButton(c, fmt.Sprintf("Layout %d", *clicks)).Clicked() {
							*clicks++
						}
						for _, s := range []string{"general", "fonts", "fonts/size"} {
							if coreButton(c, "Go "+s).Clicked() {
								r.Push("/settings/" + s)
							}
						}
					})
					inner = rt.coreView(c, func(rt *Route) {
						section = rt.Param("section")
						switch {
						case rt.Match("/general"):
							rt.Title("General")
							coreText(c, "General page")
						case rt.Match("/fonts"):
							rt.Title("Fonts")
							pg := rt.corePage()
							n := coreLocal(pg, "n", func() int { return 0 })
							if coreButton(c, fmt.Sprintf("Fonts %d", *n)).Clicked() {
								*n++
							}
						case rt.Match("/fonts/{what}"):
							what = rt.Param("what")
							rt.Title("Size")
							coreText(c, "Size page")
						}
					})
				})
			default:
				rt.Title("Notes")
				coreText(c, "Notes page")
			}
		})
	}, 500, 300)
	pages := func(e *node) int { return len(e.children()) }
	settle := func() {
		time.Sleep(300 * time.Millisecond)
		tt.Frame()
	}
	tt.Click("Layout 0")
	if !tt.HasText("General page") || !tt.HasText("Layout 1") || section != "general" {
		t.Fatalf("shows %q, section %q", tt.Texts(), section)
	}
	// A page inside the layout: the layout stays, with its state, and the
	// focus on its button, the title of the page announced; only the page
	// inside fades in.
	tt.Announcements()
	tt.Click("Go fonts")
	if !tt.HasText("Fonts 0") || !tt.HasText("Layout 1") || !tt.Focused("Go fonts") {
		t.Fatalf("inside: %q", tt.Texts())
	}
	if got := tt.Announcements(); !slices.Equal(got, []string{"Fonts"}) {
		t.Errorf("announced %q", got)
	}
	if pages(outer) != 1 || pages(inner) != 1 || inner.first.opacity >= 1 {
		t.Errorf("fading in: %d pages, %d inside, opacity %v", pages(outer), pages(inner), inner.first.opacity)
	}
	settle()
	tt.Click("Fonts 0")
	// Deeper inside: the page inside slides in, the layout stays.
	tt.Click("Go fonts/size")
	if pages(outer) != 1 || pages(inner) != 2 {
		t.Errorf("sliding in: %d pages, %d inside", pages(outer), pages(inner))
	}
	settle()
	if what != "size" || section != "fonts/size" {
		t.Errorf("the size page's parameters: %q, %q", what, section)
	}
	// Away from the layout and back: it is as it was, and so are the pages
	// inside.
	r.Push("/notes")
	tt.Frame()
	if pages(outer) != 1 || !tt.HasText("Notes page") {
		t.Fatalf("away: %q", tt.Texts())
	}
	settle()
	tt.Key(backKey())
	settle()
	if !tt.HasText("Size page") || !tt.HasText("Layout 1") {
		t.Fatalf("back: %q", tt.Texts())
	}
	tt.Key(backKey())
	settle()
	if !tt.HasText("Fonts 1") || !tt.HasText("Layout 1") {
		t.Fatalf("back again: %q", tt.Texts())
	}
	// Another query of a page inside is the same page.
	r.Push("?size=12")
	tt.Frame()
	if pages(inner) != 1 || inner.first.opacitySet || !tt.HasText("Fonts 1") {
		t.Errorf("another query: %d pages inside, %q", pages(inner), tt.Texts())
	}
	// A layout's pattern needs a rest.
	defer func() {
		if recover() == nil {
			t.Error("View after a pattern without {name...} did not panic")
		}
	}()
	rt := &Route{router: r, entry: r.current(), rest: -1}
	rt.coreView(&tt.rt.c, func(*Route) {})
}
