# Alert dialog

`ui.AlertDialog` asks about something important over the window while a
`*bool` is true, as AppKit's alerts: a title, a message, and buttons, the
last of which is the default, in the accent color, with the focus, which
Enter clicks. Escape clicks a button labeled Cancel, if any; a click outside
the alert does nothing. It returns the index of the button clicked, in the
frame it is, which closes the alert, and -1 otherwise.

```go
if ui.Button(c, "Delete").Clicked() {
	app.asking = true
}
switch ui.AlertDialog(c, &app.asking, "Delete “Notes”?", "You can't undo this.", "Cancel", "Delete") {
case 1:
	app.delete()
}
```

For a dialog of your own content, use a [dialog](dialog.md); for a choice
the user can undo, act at once and offer it in a [toast](toast.md).

## Accessibility

Assistive technology sees an alert dialog named by its title and described
by its message, with the focus on its default button.
