# List

`ui.List` shows rows of a collection of any length: it builds only the rows
in view, and a few beyond, so a list of millions of rows is as fast as one
of ten. Rows take the height of their content, which may differ from row to
row: the list measures rows as they show and estimates the others from
them.

```go
ui.List(c, nil, len(app.files), func(i int) {
	ui.Text(c, app.files[i].Name).Padding(6, 12)
}).Grow(1)
```

Give a list a size, or grow it in its parent. A list without a size, or
with only a `MaxHeight`, is as high as its rows.

## Layout

`Gap` spaces the rows, `Padding` pads the content and scrolls with it, and
`Justify(ui.End)` puts rows that do not fill the list at its bottom, as a
chat's first messages. What else you build in a list shows while it has no
rows, as a message that it is empty:

```go
ui.List(c, nil, len(results), func(i int) {
	ui.Text(c, results[i].Title).Padding(6, 12)
}).Grow(1).Children(func() {
	if len(results) == 0 {
		ui.Text(c, "No results").TextColor(c.Theme().TextMuted).Padding(12)
	}
})
```

## List state

A list keeps its place by a row rather than by an offset, so the rows in
view stay where they are while the rows around them are measured, grow,
come and go, and its offsets count millions of rows to a fraction of a DIP.
Give it a `ui.ListState` in your state, in place of `nil`, to say how it
behaves and to move it:

- `Key` returns an identity for the item of a row, such as its ID: the
  state of a row (focus, text being edited, …) follows its item, and the
  list keeps its place, and its choice, when rows are added or removed
  above them, as when older messages load.
- `FollowEnd` starts the list at its end, and keeps the end in view as rows
  come or grow, until the user scrolls away from it, as a chat or a log
  does; scrolling back to the end follows it again.
- `Header` names the rows heading sections: the header of the section at
  the top stays pinned there while its rows scroll under it, until the next
  header pushes it away. Give headers a background.
- `Label` returns the text of a row: typing its first letters while the
  list has the focus chooses it, as in Finder or Explorer, and assistive
  technology reads it as the row's name.

Give each list a `ListState` of its own, and set its fields before
building the list.

## Choosing rows

- `Selected` lets a click, or Up, Down, Home and End while the list has the
  keyboard focus (and Page Up and Page Down on Linux and Windows), choose a
  row, which shows in the accent color; the list's `Changed` reports a new
  choice, and `Submitted` a double click or Enter.
- `Selection` lets the user choose several rows, held in a
  `ui.Selection[K]` by the keys of their items (`Key`), or by their indices
  without one: a click chooses one, Cmd-click (Ctrl-click on Linux and
  Windows) adds a row or takes it out, and Shift-click or Shift with the
  keys chooses the rows from the one last chosen; Cmd+A chooses all. On
  Linux and Windows, Ctrl with the keys moves without choosing, and
  Ctrl+Space adds the row there or takes it out. `Selected`, set as well, is
  the row last chosen.

Files to choose several of, by their paths:

```go
type fileList struct {
	files  []File
	chosen ui.Selection[string]
	list   ui.ListState
}

func (app *fileList) view(c *ui.Context) {
	app.list.Key = func(i int) any { return app.files[i].Path }
	app.list.Label = func(i int) string { return app.files[i].Name }
	app.list.Selection = &app.chosen
	ui.List(c, &app.list, len(app.files), func(i int) {
		ui.Text(c, app.files[i].Name).Padding(6, 12)
	}).Grow(1)
	if ui.Button(c, fmt.Sprintf("Delete %d", app.chosen.Len())).Clicked() {
		for path := range app.chosen.All() {
			app.delete(path)
		}
		app.chosen.Clear()
	}
}
```

The keys of items gone from the list stay in the selection until you take
them out. `Element.ClickModifiers` gives the modifier keys of any click, for
widgets of your own that do the same.

## Scrolling

`ScrollTo(row, align)` shows a row at the `ui.Start`, `ui.Center` or
`ui.End` of the list, `ScrollIntoView(row)` scrolls as little as shows it,
and `ScrollToEnd` scrolls to the end, rows not built yet included: the list
scrolls as the frame is laid out, to where the row is once measured.
`Visible` returns the first and last rows in view and `AtEnd` whether the
end shows, to load more as the list nears the end of what was loaded.

A chat:

```go
app.chat.Key = func(i int) any { return app.messages[i].ID }
app.chat.FollowEnd = true
if first, _ := app.chat.Visible(); first < 5 && app.more {
	app.loadOlder() // above the messages in view, which stay put
}
if !app.chat.AtEnd() && ui.Button(c, "Jump to latest").Clicked() {
	app.chat.ScrollToEnd()
}
ui.List(c, &app.chat, len(app.messages), func(i int) {
	message(c, app.messages[i])
}).Grow(1).Justify(ui.End)
```

## Reordering

`Reorder` lets the user drag rows to another place in the list: the rows
chosen, when the row dragged is one of them, else that row, which the list
passes in order, and the row they go before, the number of rows for the
end. Move them there: the list shows where they would go as they are
dragged, and scrolls near its edges.

```go
app.list.Reorder = func(rows []int, to int) {
	app.items = move(app.items, rows, to)
}
```

See [drag and drop](drag-and-drop.md).

## The focus

A list keeps its control identity in `ListState.Handle`. Focus queries work
before or after constructing the list, and a focus request waits while the
list is hidden:

```go
app.list.Handle.OnShortcut(c, ui.Cmd, ui.KeyK, app.openCurrentFile)
ui.List(c.Key("files"), &app.list, len(app.files)).Grow(1).
    Rows(func(row ui.ListRow) {
        text := ui.Text(row.Context, app.files[row.Index].Name).Padding(6, 12)
        if row.Selected() && row.ListFocused() {
            text.TextColor(c.Theme().Accent)
        }
    })
```

Call `app.list.Handle.Focus()` on the UI thread to request focus, or
`Focus(c)` to select a window explicitly. `Focused(c)` reports focus on the
list and `FocusWithin(c)` includes its descendants. A handle's shortcut
action runs after construction only if its enabled control is present.

`ListRow` supplies the current Context, row index, selection and list focus.
`ListState.Selected` or `Selection` enables selection. `ItemKey` gives each
item a stable key. Configure them before calling Rows.

For custom row builders using `List(c, state, n, func(i int))`, the
ListState polling helpers resolve this pass's list after construction or
inside its row callback. Persistent focus and command handling use Handle.
A Table shares this identity through its ListState, and an Outline through
its `List` state.

The row holding keyboard focus stays built when it scrolls out of view,
so editing continues and Tab can scroll the next focused row into view.

## Accessibility

Assistive technology sees a list, named by its `Label`, of list items named
by the text inside them or the state's `Label`, each saying which of all
the rows it is, "5 of 10,000", although the list builds only those in view.
A list choosing its rows gives assistive technology the focus on the row
chosen: Up and Down are read as they move the choice, and focusing a row
chooses it. A screen reader moving out of view asks the list to scroll
there, which builds the rows it reaches.

For rows in columns, see [Table](table.md); for a tree, [Outline](outline.md).
