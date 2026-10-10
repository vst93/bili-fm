# Radio

`ui.Radio` creates a radio button that selects its value into a `*T`, with
a label: a click selects it, as does Space while it has the keyboard focus,
and `Changed` reports it. The value can be of any comparable type.

`ui.RadioGroup` holds radio buttons as one stop of Tab: Tab goes to the
one chosen, and the arrows choose among them, as do Home and End.

```go
ui.RadioGroup(c, func() {
	for _, size := range []string{"Small", "Medium", "Large"} {
		ui.Radio(c, &app.size, size, size)
	}
}).Label("Size")
```

A radio group is a column; make it a row with `Row`:

```go
ui.RadioGroup(c, func() {
	ui.Radio(c, &app.align, ui.Start, "Left")
	ui.Radio(c, &app.align, ui.Center, "Center")
	ui.Radio(c, &app.align, ui.End, "Right")
}).Row().Gap(16).Label("Alignment")
```

For a choice among many, use a [select](select.md) or a
[combobox](combobox.md); between views, a [segmented control](segmented.md).

## Without a look

`ui.RadioBase` is a radio button without a look: a row that selects its
value when clicked: see [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a radio group named by its `Label`, of radio
buttons named by their labels, on for the one chosen.
