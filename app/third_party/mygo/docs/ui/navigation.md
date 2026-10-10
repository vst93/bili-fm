# Pages and navigation

A `ui.Router` keeps the history of the pages of a window, or of a part of
one, as a browser does for a tab. A page is a path, as `/notes/42`, which
`View` matches to build the page shown, a `switch` over patterns whose
`{name}` takes one part of the path and `{name...}` the rest.

The router is part of your app's state: make it once, with the page to
show first, and keep it in a field. The view, which runs for every frame,
builds the page it shows:

```go
// notesApp is the app's state.
type notesApp struct {
	router *ui.Router
	// … the notes and the files
}

func main() {
	app := &notesApp{router: ui.NewRouter("/notes")}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{Title: "Notes", Content: ui.View(app.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func (app *notesApp) view(c *ui.Context) {
	ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
		app.sidebar(c) // stays as the pages change, see below
		app.router.View(c, func(r *ui.Route) {
			switch {
			case r.Match("/notes"):
				r.Title("Notes")
				app.notes(c)
			case r.Match("/notes/{id}"):
				r.Title("Note")
				app.note(c, r.Param("id"))
			case r.Match("/files/{path...}"):
				app.files(c, r.Param("path"))
			default:
				ui.Text(c, "Not found")
			}
		})
	})
}
```

`app.notes`, `app.note` and `app.files` are methods of `notesApp` that
build those pages, as `view` builds the window.

## Going to pages

`Push` goes to a page, after the page shown, `Replace` shows one in its
place, and `Back`, `Forward` and `Go` move through the history; paths
relative to the page shown resolve as links in a web page do, so
`Push("?tab=info")` changes the query and `Push("edit")` goes to a sibling.
A [link](link.md) to a path in a page goes there in its router, and
choosing an item of a [sidebar](sidebar.md) pushes its page, as the sidebar
of the app above does:

```go
func (app *notesApp) sidebar(c *ui.Context) {
	page := app.router.Path()
	ui.Sidebar(c, &page, func() {
		ui.SidebarItem(c, "/notes", nil, "Notes")
		ui.SidebarItem(c, "/files", nil, "Files")
	}).Width(220).OnChange(func() {
		app.router.Push(page)
	})
}

// In a page:
ui.Link(c, "Open note", "/notes/42")
```

`Path`, `Query` and `Location` read the page shown; `Location` with its
query, which `NewRouter` takes to show it again when the app starts. A deep
link (`mygo.App.OnOpenURL`) pushes its page.

## Back and forward

Users go back and forward as in browsers and Finder: Cmd+[ and Cmd+] on
macOS, Alt+Left and Alt+Right on Linux and Windows, the back and forward
buttons of a mouse and the keys of keyboards that have them, and the
[back and forward buttons](history-buttons.md), which a toolbar holds, and
whose right click lists the pages to go back or forward to, by their
titles. The keys go to the router holding the keyboard focus, else to the
first of the window.

## Layouts

Anything built around `View`, as a sidebar or a toolbar, stays as pages
change. A page can also be a layout around the pages of the rest of its
path, as a page of settings with a list of its sections beside the section
shown: `r.View` on its route builds them, matching what the pattern's
`{name...}` took. The layout stays as they change, keeping its state, and
they slide or fade in its place; `Param` reads the layout's wildcards too.
A page of settings is another case of the `switch` in the view above:

```go
case r.Match("/settings/{section...}"):
	ui.Row(c).Grow(1).Children(func() {
		app.sectionList(c) // pushes "/settings/general", "/settings/fonts"
		r.View(c, func(r *ui.Route) {
			switch {
			case r.Match("/general"):
				r.Title("General")
				app.general(c)
			case r.Match("/fonts"):
				r.Title("Fonts")
				app.fonts(c)
			}
		})
	})
```

A second `Router` inside a page, in a field of its own, is a history of its
own, as the detail of a split view going deeper while the list beside it
stays.

## Pages keep their state

A page keeps the state of its elements while it is in the history, ten
pages either way: going back to one finds it scrolled where it was, with
its text and the keyboard focus where they were. A path with another query
is the same page, in another entry of the history, keeping its state, as a
page whose tabs or search are in its query.

## The focus and screen readers

Going to a page moves the keyboard focus into it, to the element that had
it there, else to the page itself, which Tab goes into; screen readers read
the page's `Title`. With the focus outside the page that changes, as in a
sidebar or a layout's list choosing the pages, it stays there, and screen
readers hear the title of the page.

## Transitions

A page deeper in the paths, as `/notes/42` after `/notes`, slides in from
the right over the page it leaves, and back up from the left; others fade
in. With less motion asked of the desktop, all fade, and
`router.Transition = ui.TransitionNone` shows them at once. The page going
away takes neither the pointer nor the keyboard as it slides.
