# Breadcrumbs

`ui.Breadcrumbs` creates a path of items, as Finder's path bar and AppKit's
path control, separated by chevrons: the items but the last are links,
which a click chooses, as Enter does, setting an `*int` to its index, which
`Changed` reports. Long items shorten with an ellipsis as room runs short.

```go
path := []string{"Macintosh HD", "Users", "ada", "Documents"}
if ui.Breadcrumbs(c, path, &app.chosen).Label("Path").Changed() {
	app.open(path[:app.chosen+1])
}
```

The last item is where the user is, which is no link.

## Accessibility

Assistive technology sees links in a group, which `Label` names.
