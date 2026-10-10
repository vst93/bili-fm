# Toggle

`ui.Toggle` creates a button that stays pressed while a `*bool` is true, as
Bold in an editor's toolbar: a click presses it or lets it go, as does
Space while it has the keyboard focus, and `Changed` reports a change.

```go
ui.Toggle(c, &app.inspector, "Inspector")
```

Give it other content, such as an icon, with `Children` and an empty label,
and name it with `Label`:

```go
ui.Toggle(c, &app.bold, "").Label("Bold").Children(func() { ui.Icon(c, boldIcon) })
```

## Toggle groups

`ui.ToggleGroup` draws the toggles and buttons inside it joined, as the
segments of one control, as Bold, Italic and Underline. They are one stop
of Tab, among which the arrows move the focus, as in a
[toolbar](toolbar.md):

```go
ui.ToggleGroup(c, func() {
	ui.Toggle(c, &app.bold, "B")
	ui.Toggle(c, &app.italic, "I")
	ui.Toggle(c, &app.underline, "U")
}).Label("Style")
```

For a choice of one among several, as a view mode, use a
[segmented control](segmented.md).

## Without a look

`ui.ToggleBase` is a toggle without a look: a row that turns its `*bool`
over when clicked: see [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a toggle button, pressed while on, and a toggle
group as a group named by its `Label`.
