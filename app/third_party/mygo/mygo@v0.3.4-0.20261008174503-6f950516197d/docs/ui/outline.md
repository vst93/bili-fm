# Outline

`ui.Outline` shows a tree of items, as AppKit's outline view and Finder's
list view: items at the top, and below each item open its children,
indented, with an arrow that opens and closes it. The app gives the items at
the top and `children`, which returns an item's, nil for one without any;
the outline builds only the rows in view, as a [list](list.md) does, so
trees of any size are as fast as small ones.

```go
type fileTree struct {
	children map[string][]string // the paths in each directory
	files    ui.OutlineState[string]
	row      int
}

func (app *fileTree) view(c *ui.Context) {
	app.files.List.Selected = &app.row
	ui.Outline(c, &app.files, []string{"/"}, func(dir string) []string {
		return app.children[dir] // nil for a file
	}, func(path string) {
		ui.Text(c, filepath.Base(path))
	}).Grow(1)
	chosen := app.files.Item(app.row)
	// …
}
```

## Outline state

Its state, an `OutlineState`, holds the rows' `ListState`, which chooses
rows as a list's does (`Selected`, `Selection`), and the items `Open`, a
`Selection` of the items showing their children. The rows' keys are the
items, so their choice and state follow them as items open and close above
them. `Rows`, `Item` and `Depth` read the rows shown.

## Opening and closing

- A click on an item's arrow opens or closes it; Option-click (Alt-click)
  opens or closes all inside it too.
- While the outline has the focus, Right opens the item chosen or goes to
  its first child, and Left closes it or goes to its parent; with Option on
  macOS and Shift elsewhere, as in GTK, they open and close all inside.
- Closing an item whose child was chosen chooses the item.

## In columns

`ui.OutlineTable` is the same in a [table](table.md)'s columns, the first of
which shows the arrows, indented as the items are deep: the columns are a
table's, which the user sorts, resizes and moves.

```go
ui.OutlineTable(c, &app.files, cols, roots, children, func(path string, col int) {
	switch col {
	case 0:
		ui.Text(c, filepath.Base(path)).SingleLine()
	case 1:
		ui.Text(c, app.size(path))
	}
})
```

## Accessibility

Assistive technology sees a tree, or a table with rows that open and close,
whose rows say their level, whether they open, and whether they are open,
and which of all the rows they are; it opens and closes them as the arrows
do.
