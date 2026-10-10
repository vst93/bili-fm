# Menu button

`ui.MenuButton` creates a button showing a label and an arrow, which opens
a menu of the system's below it, as the system's pop-up buttons do: as the
pointer goes down on it, or for Enter, Space or Down while it has the
keyboard focus. The function builds the items when the menu opens, and runs
again in the frame after one was chosen, where its `Chosen` reports it:

```go
ui.MenuButton(c, "Sort by", func(m *ui.Menu) {
	for _, by := range []string{"Name", "Date", "Size"} {
		if m.Item(by).Checked(app.sort == by).Chosen() {
			app.sort = by
		}
	}
})
```

## Items

Menus take the same items as [context menus](context-menu.md#items):
`Item`, with `Checked`, `Disabled` and `Shortcut`, `Separator` and
`Submenu`.

## Any button

`Element.Menu` gives the same menu to any button, as one showing only an
icon:

```go
ui.Button(c, "").Label("More").Children(func() { ui.Icon(c, dots) }).Menu(func(m *ui.Menu) {
	if m.Item("Duplicate").Chosen() {
		app.duplicate()
	}
	if m.Item("Delete").Chosen() {
		app.delete()
	}
})
```

In a [toolbar](toolbar.md), a menu button takes no face of its own until
hovered.

## Accessibility

Assistive technology sees a menu button: `AXMenuButton` on macOS, a push
button with a menu on Linux, and a button that expands on Windows.
