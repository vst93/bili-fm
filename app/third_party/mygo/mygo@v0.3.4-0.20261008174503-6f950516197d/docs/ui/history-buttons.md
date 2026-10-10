# Back and forward buttons

`ui.BackButton` and `ui.ForwardButton` create buttons going back and forward
in a [router](navigation.md)'s history, as in the toolbar of Finder or a
browser: disabled at the ends of the history, and showing a chevron. Give
them the router, a `*ui.Router` in your state:

```go
ui.Toolbar(c, func() {
	ui.BackButton(c, app.router)
	ui.ForwardButton(c, app.router)
	ui.Text(c, app.router.Title()).Bold()
})
```

A right click lists the pages to go back or forward to, nearest first, by
their titles (a route's `Title`), or their paths without one; choosing one
goes there at once.

The buttons are a shortcut for what the keys already do: Cmd+[ and Cmd+] on
macOS, Alt+Left and Alt+Right elsewhere, and the side buttons of a mouse.

## Accessibility

Assistive technology sees buttons named "Back" and "Forward", unavailable at
the ends of the history.
