# Editable text

`ui.EditableText` shows a `*string` as a text that the user edits in place,
as a file's name in Finder: a double click on it, or Enter while it has the
focus, shows a text input of it, with the text before its extension
selected. Enter keeps what was typed, as moving the focus away does, and
Escape goes back; `Changed` reports a new value.

```go
if ui.EditableText(c, &app.title).Changed() {
	app.save()
}
```

## In lists and tables

In a [list](list.md)'s or a [table](table.md)'s rows, the list keeps the
focus and the text takes none: Return on macOS, and F2 elsewhere, edit the
text of the row chosen, while a double click on the row is `Submitted`; a
click on the text of a row already chosen edits it, once a second click
would no longer make a double click.

```go
ui.Table(c, &app.files, cols, len(files), func(row, col int) {
	if col == 0 && ui.EditableText(c, &files[row].Name).Changed() {
		app.rename(files[row])
	}
})
```

## Accessibility

Assistive technology reads the text, and the text field while it is
edited.
