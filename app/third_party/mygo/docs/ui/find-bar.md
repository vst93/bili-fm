# Find bar

`ui.FindBar` creates a bar for finding text while a `*bool` is true, as
Safari's and Xcode's: a field of the query, which takes the focus as the
bar opens with the last query selected, how many matches there are,
previous and next buttons choosing the current match, and Done.

```go
if c.Shortcut(ui.Cmd, ui.KeyF) {
	app.finding = true
}
matches := app.find(app.query) // the app counts them
ui.FindBar(c, &app.finding, &app.query, len(matches), &app.match)
if app.finding && len(matches) > 0 {
	app.show(matches[app.match])
}
```

The app finds: it counts the matches of the query and shows the current
one, which `Changed` reports moving.

## Keyboard

Enter in the field goes to the next match, Shift+Enter to the previous, as
do Cmd+G and Shift+Cmd+G on macOS, F3 and Shift+F3 elsewhere, round the
ends; Escape and Done close the bar.

While closed, it returns an element that shows nothing.

## Accessibility

Assistive technology sees a toolbar named "Find", holding a search field,
and hears the count of matches as it changes.
