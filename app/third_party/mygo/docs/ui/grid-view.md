# Grid view

`ui.GridView` shows items in a grid, as Photos' library and Finder's icon
view: as many columns as fit items at least as wide as asked, sharing the
width left, in rows of a height, built only as they show, so grids of
millions of items are as fast as small ones. `Gap` spaces the items.

```go
app.photos.Selected = &app.photo
ui.GridView(c, &app.photos, len(photos), 140, 120, func(i int) {
	ui.Image(c, thumbs[i]).Fit(ui.Contain).Grow(1)
	ui.Text(c, photos[i].Name).SingleLine()
}).Grow(1)
```

## Grid state

A `GridState` keeps the grid's place and says how items are chosen, as a
`ListState` does rows, or `nil`:

- `Selected` lets a click choose an item, as the arrows do in both
  directions while the grid has the keyboard focus, with Home, End, and
  Enter (`Submitted`).
- `Selection` lets the user choose several, by `Key`: Cmd-click adds an
  item or takes it out, Shift-click and Shift with the arrows choose those
  from the item last chosen, and Cmd+A chooses all.
- `Label` returns an item's text: typing its first letters chooses it, and
  assistive technology reads it as the item's name.
- `Reorder` lets the user drag items to another place, as a list's rows:
  see [drag and drop](drag-and-drop.md).

## Accessibility

Assistive technology reads the grid as a list of items, each saying which
of all it is, built or not.
