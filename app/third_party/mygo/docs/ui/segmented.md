# Segmented control

`ui.Segmented` creates a segmented control showing labels, of which an
`*int` is the index of the one chosen, as a switch between views: a click
chooses a segment, as do the arrows while it has the keyboard focus, and
`Changed` reports a new choice.

```go
ui.Segmented(c, &app.view, "List", "Grid").Label("View")
switch app.view {
case 0:
	app.list(c)
case 1:
	app.grid(c)
}
```

It is one stop of Tab, which goes to the segment chosen.

## Icons and other content

For segments of icons or of your own look, build them with
`ui.SegmentedBase`, a `Track` of `Segment`s without a look:

```go
seg := ui.SegmentedBase(c, &app.view, 2)
seg.Track.Padding(3).Radius(8).Background(t.Surface).Children(func() {
	for i, icon := range []*ui.SVG{listIcon, gridIcon} {
		s := seg.Segment(i).Padding(4, 10).Radius(6).Label([]string{"List", "Grid"}[i])
		if i == app.view {
			s.Background(t.Background)
		}
		s.Children(func() { ui.Icon(c, icon) })
	}
})
```

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a radio group, named by its `Label`, of radio
buttons named by their labels.
