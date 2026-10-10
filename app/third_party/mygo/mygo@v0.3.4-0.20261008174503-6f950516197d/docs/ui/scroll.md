# Scroll view

`ui.Scroll` and `ui.ScrollHorizontal` scroll their children with the wheel,
the touchpad, their scroll bar and the keyboard: the arrow keys, Page Up and
Page Down, Space, Home and End scroll the container around the focus, or
under the pointer, unless the focused element takes those keys, as a text
input does. `ui.ScrollBoth` scrolls both ways, as a canvas or a wide table
does.

```go
ui.Scroll(c).Grow(1).Padding(16).Gap(12).Children(func() {
	for _, p := range app.posts {
		post(c, p)
	}
})
```

Give them a size, or grow them in their parent. For the rows of a
collection, use a [list](list.md), which builds only those in view.

## Keeping the offset

A scroll container keeps its offset as it keeps its other state, by its
place in the view or its `Key`. To read the offset, set it or keep it in
the app's state, give the container a `ui.ScrollState` with `TrackScroll`:
the container shows its content from the state's `X` and `Y`, writes them
as the user scrolls, and sets `MaxX` and `MaxY` to how far they go. `Y = 0`
scrolls to the top and `math.MaxFloat32` to the end, and a `ScrollState`
for each page a container shows keeps each page's place. A log that follows
its end, unless the user scrolled up from it:

```go
if app.log.Y >= app.log.MaxY {
	app.log.Y = math.MaxFloat32
}
ui.Scroll(c).TrackScroll(&app.log).Grow(1).Children(app.lines)
```

## Scrolling into view

`ScrollIntoView` scrolls the containers around an element as little as
shows it, once the frame is laid out, so that it works in the frame that
adds the element: a new message, or the item the keys chose. Call it in that
frame only, or the user could not scroll the element away. Rows a `List`
has not built scroll into view with its `ListState`.

```go
if app.added {
	ui.Text(c, msg.Text).ScrollIntoView()
}
```

The keyboard focus scrolls into view as it moves.

## Scroll bars

A scroll container whose content overflows it shows the thumbs of its
scroll bars over its content while the pointer is over it, which the user
drags; they take no room. The theme's `ScrollbarWidth` sets their width and
its `Scrollbar` their color.

`ScrollbarInsets` moves the bars in from the container's edges, CSS style
as `Padding`: the vertical bar runs from the top inset to the bottom one,
the right inset in from the right, and the horizontal bar from the left
inset to the right one, the bottom inset up. So the bars keep clear of
what floats over the content, as a toolbar it scrolls under, as AppKit's
and UIKit's do:

```go
ui.Scroll(c).Fill().Padding(64, 16, 16).ScrollbarInsets(64, 0, 0).Children(func() { /* ... */ })
// The toolbar floats over the top 64 DIPs.
ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(64).Children(func() { /* ... */ })
```

## Accessibility

Assistive technology sees a scroll area, which it scrolls to show what it
reads.
