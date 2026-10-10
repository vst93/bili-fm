# Button

`ui.Button` creates a push button showing a label, and `ui.PrimaryButton`
one in the accent color, for the main action of a window or a dialog.
`Clicked` reports a click, or Enter or Space while the button has the
keyboard focus:

```go
ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
	if ui.Button(c, "Cancel").Clicked() {
		app.editing = false
	}
	if ui.PrimaryButton(c, "Save").Clicked() {
		app.save()
	}
})
```

## Content

A button lays out its children in a row and gives them its text color.
Give it other content, as an icon before the label, with `Children` and an
empty label:

```go
ui.PrimaryButton(c, "").Children(func() {
	ui.Icon(c, save)
	ui.Text(c, "Save").SingleLine()
})
```

A button showing only an icon needs a `Label`, which names it for
assistive technology, and a `Tooltip`:

```go
if ui.Button(c, "").Label("Delete").Tooltip("Delete").Children(func() { ui.Icon(c, trash) }).Clicked() {
	app.delete()
}
```

## States

`Disabled(true)` grays a button out: it takes neither clicks nor the focus.
In a [toolbar](toolbar.md), buttons take no face of their own until
hovered. A default button that Enter presses wherever the focus is takes
the key with a [shortcut](input.md#shortcuts):

```go
if ui.PrimaryButton(c, "Sign in").Clicked() || c.Shortcut(0, ui.KeyEnter) {
	app.signIn()
}
```

A focused button keeps Enter and Space for itself, so the shortcut fires
only when the focus is elsewhere.

## Style

The button returns its element: calls after it change its look, as
`ui.Button(c, "Save").Padding(10, 20).Radius(999)`. `ui.ButtonBase` is a
button without a look, a row that takes the focus and reports `Clicked`,
for a design of your own: see [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a button named by its text or its `Label`, which
it presses as a click does.

## Click actions

Use `Clicked()` for inline control flow or `OnClick` to run an action after
construction:

```go
ui.Button(c, "Save").Disabled(app.saving).OnClick(app.save)
```

The callback sees the final configuration and runs after bound-value input.
It can change the model without interrupting a loop that is building controls.
Handled input is consumed before the view rebuilds. See
[Events and actions](views.md#events-and-actions) for choosing between queries
and callbacks.
