# Search field

`ui.SearchField` creates a field for searching, editing a `*string`: a
magnifying glass, the text, and a button clearing it while it holds any,
as Escape does then. It shows "Search" while empty. `Changed` reports a
change, and `Submitted` Enter.

```go
if ui.SearchField(c, &app.query).Label("Search mail").Changed() {
	app.filter()
}
```

To search as the user types, filter on `Changed`; for searches that take
time, search on `Submitted`, or after a pause with `c.After`.

## Accessibility

Assistive technology sees a search field (`AXSearchField` on macOS), named
by its `Label`.
