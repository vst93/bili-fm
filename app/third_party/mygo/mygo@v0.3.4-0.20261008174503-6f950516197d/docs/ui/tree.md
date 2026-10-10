# Tree

`ui.Tree` creates a tree whose items `ui.TreeItem` builds, each inside the
item it belongs to. An item's children show while its `*bool` is true; a
click on its arrow opens or closes it, and `Clicked` reports a click
elsewhere on it, or Enter, for choosing it, which `Selected` shows.

```go
item := func(path, label string, open *bool, children func()) {
	if ui.TreeItem(c, label, open, children).Selected(app.chosen == path).Clicked() {
		app.chosen = path
	}
}
ui.Tree(c, func() {
	item("src", "src", &app.srcOpen, func() {
		item("src/main.go", "main.go", nil, nil)
	})
	item("go.mod", "go.mod", nil, nil)
})
```

Nil children, or a nil open, makes a leaf.

## Keyboard

While an item has the keyboard focus, Up and Down move it to the items
around, Right opens the item or moves to its first child, and Left closes it
or moves to its parent.

A tree builds every item it shows. For trees of thousands of items, or
with columns, use an [outline](outline.md), which builds only the rows in
view.

## Accessibility

Assistive technology sees a tree of items, each with its level, open or
closed when it has children.
