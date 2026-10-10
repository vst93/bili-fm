# Split view

`ui.Split` lays out two panes side by side, with a divider between them that
the user drags to resize them, or moves with the arrows once it has the
keyboard focus: a `*float32` is the width of the first, which the divider
keeps 40 DIPs from either edge. `Changed` reports a move.

```go
ui.Split(c, &app.sidebarWidth, func() { app.files(c) }, func() { app.editor(c) }).Fill()
```

`ui.SplitVertical` puts the first above the second, the `*float32` its
height:

```go
ui.SplitVertical(c, &app.consoleHeight, func() { app.editor(c) }, func() { app.console(c) }).Grow(1)
```

Keep the size in the app's state, and save it to restore the layout when
the app starts again. The pointer shows a resize cursor over the divider.

## Accessibility

Assistive technology sees the divider as a splitter named "Divider",
which takes the focus and moves with the arrows.
