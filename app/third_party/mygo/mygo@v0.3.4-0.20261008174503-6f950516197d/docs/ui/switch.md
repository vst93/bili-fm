# Switch

`ui.Switch` creates a switch turning a `*bool` on and off, for a setting
that applies at once: a click turns it over, as does Space while it has the
keyboard focus, and its knob slides across. `Changed` reports a click.

```go
ui.Row(c).Gap(12).Children(func() {
	ui.Text(c, "Wi-Fi").Grow(1)
	ui.Switch(c, &app.wifi).Label("Wi-Fi")
})
```

A switch shows no text of its own: give it a `Label` for assistive
technology, or put it in a [field](form.md), whose label names it and
toggles it when clicked:

```go
ui.Form(c, func() {
	ui.Field(c, "Sound", func() { ui.Switch(c, &app.sound) })
})
```

For a choice that applies when a form is saved, use a
[check box](checkbox.md).

## Without a look

`ui.SwitchBase` is a switch without a look, which you draw from the value:
see [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a switch, on or off, named by its `Label` or its
field.
