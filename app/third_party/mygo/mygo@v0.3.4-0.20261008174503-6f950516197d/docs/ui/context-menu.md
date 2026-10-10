# Context menu

`ContextMenu` gives an element a menu of the system's, which opens where
the element is right-clicked (Control-clicked on macOS), and below it when
the menu key or Shift+F10 is pressed while it or one inside it has the
focus. The function builds the items when the menu opens, and runs again in
the frame after one was chosen, where its `Chosen` reports it:

```go
row.ContextMenu(func(m *ui.Menu) {
	if m.Item("Rename").Shortcut(0, ui.KeyF2).Chosen() {
		app.renaming = i
	}
	if m.Item("Pinned").Checked(note.pinned).Chosen() {
		note.pinned = !note.pinned
	}
	m.Submenu("Move to", func(m *ui.Menu) {
		for _, f := range app.folders {
			if m.Item(f.name).Chosen() {
				moveTo, moved = f, i
			}
		}
	})
	m.Separator()
	if m.Item("Delete").Disabled(note.locked).Chosen() {
		deleted = i
	}
})
```

The function runs as the element builds, in the loop building the rows:
a choice that takes the row out of the notes, as moving or deleting it,
notes it, for the change once the loop is done (see
[views](views.md#events-and-actions)).

## Items

- `Item(label)` adds an item; `Chosen` reports whether it was chosen.
- `Checked(on)` shows a check mark by it, as for a setting that choosing the
  item toggles.
- `Disabled(d)` grays it out: it cannot be chosen.
- `Shortcut(mods, key)` shows a key by it, as the shortcut that does the
  same. It only shows the key: handle it with `Shortcut` on the context or
  an element.
- `Separator()` adds a line between groups of items, and
  `Submenu(label, build)` an item opening a submenu, which `build` fills.

## Which element

The innermost element with a menu gets the click, as a row's button with a
menu of its own inside a row with another. [Text inputs](text-input.md)
have the editing commands of their platform's text fields, and selectable
[text](text.md) has Copy and Select All. `ContextMenu` on the input, the
text, or the `Selectable` container replaces that menu, and `EditItems`
adds those commands to a menu of one's own, acting on the text that was
clicked, as a chat message's text offers Reply above Copy:

```go
message.Selectable().ContextMenu(func(m *ui.Menu) {
	if m.Item("Reply").Chosen() {
		app.reply(msg)
	}
	m.Separator()
	m.EditItems()
})
```

The same menu opens below a button with `Element.Menu`: see
[menu button](menu-button.md). In tests, `tt.RightClick` opens a context
menu, which `tt.Menu` lists and `tt.ChooseMenuItem` chooses from.
