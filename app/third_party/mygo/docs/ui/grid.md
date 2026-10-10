# Grid

`ui.Grid` lays its children out in columns and rows, as CSS grid does: they
fill its cells row by row, and it adds rows as they need them. `Columns(3)`
makes three columns of equal width; `ColumnTracks` sets them one by one, as
`ui.Fixed(220)` DIPs, `ui.Fr(1)`, a share of the room the others leave, or
`ui.FitContent()`, as wide as their content:

```go
ui.Grid(c).Columns(3).Gap(12).Children(func() {
	for _, p := range app.photos {
		ui.Image(c, p).AspectRatio(1).Fit(ui.Cover).Radius(8)
	}
})

ui.Grid(c).ColumnTracks(ui.FitContent(), ui.Fr(1)).GapX(16).GapY(8).Children(func() {
	ui.Text(c, "Name").Bold().ColumnSpan(-1) // across every column
	ui.Text(c, "Email")
	ui.TextInput(c, &app.email)
	ui.Text(c, "Bio")
	ui.TextArea(c, &app.bio).RowSpan(2)
})
```

## Placing children

`ColumnSpan` and `RowSpan` make a child span tracks, a negative span every
track to the last, and `ColumnStart` and `RowStart` put it in a given column
and row, counting from 1. `RowTracks` and `Rows` set the first rows, which
`Fr` makes share the grid's height.

## Aligning

Children stretch to fill their cells unless `JustifyItems` (horizontally)
and `AlignItems` (vertically) say otherwise, or `JustifySelf` and
`AlignSelf` for one; automatic margins center them. Tracks that fit their
content share the room left over unless `Justify` or `AlignContent` place
them instead.

A grid builds all its children. For a grid of many items, built only as
they show, use a [grid view](grid-view.md); for labels beside controls, a
[form](form.md).
