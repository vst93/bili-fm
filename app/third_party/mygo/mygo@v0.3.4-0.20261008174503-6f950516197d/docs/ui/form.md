# Form

A `ui.Field` labels the control its function builds: the label names the
control for assistive technology, and a click on it focuses the control, or
clicks a check box, a switch or a radio button. `Description` and `Error`
add texts below the control, which assistive technology reads with it; an
error also marks the control's value invalid, and a text input draws its
border in the theme's `Danger` color. An empty error leaves the control
valid:

```go
var invalid string
if app.email != "" && !strings.Contains(app.email, "@") {
	invalid = "Enter an email address."
}
ui.Field(c, "Email", func() {
	ui.TextInput(c, &app.email)
}).Description("For receipts.").Error(invalid)
```

The control is the first element the function builds that takes the focus,
or a group of them, such as a [radio group](radio.md).

## Forms

A field's label sits above its control, as on the web. In a `ui.Form`, the
labels of its fields sit in a column of their own beside the controls, as in
macOS's forms: aligned to the right, as wide as the widest, and level with
the first line of text of their control, or centered on a control without
text, such as a switch.

```go
ui.Form(c, func() {
	ui.Field(c, "Name", func() { ui.TextInput(c, &app.name) })
	ui.Field(c, "Plan", func() { ui.Select(c, &app.plan, plans) })
	ui.Field(c, "Sound", func() { ui.Switch(c, &app.sound) })
})
```

## Fieldsets

A `ui.Fieldset` groups fields under a legend, which names the group; their
labels line up with the form's others, and `Disabled` disables them all:

```go
ui.Form(c, func() {
	ui.Field(c, "Name", func() { ui.TextInput(c, &app.name) })
	ui.Fieldset(c, "Notifications", func() {
		ui.Field(c, "Email", func() { ui.Checkbox(c, &app.mail, "Weekly summary") })
		ui.Field(c, "Sound", func() { ui.Switch(c, &app.sound) })
	}).Disabled(!app.signedIn)
})
```

A control with a name of its own, as a check box with its text, keeps it,
and the field's label names the group around them.

## Accessibility

Assistive technology reads a control by its field's label, with the
description and the error after it, and the value invalid while there is an
error; a fieldset is a group named by its legend.
