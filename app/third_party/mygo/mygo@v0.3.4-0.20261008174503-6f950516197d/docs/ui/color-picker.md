# Color picker

`ui.ColorWell` creates a swatch of a `*ui.Color` that opens a color picker
below it, as AppKit's color well: a click, Enter or Space opens it, and
Escape or a click outside closes it. `Changed` reports a new color.

```go
ui.Field(c, "Tint", func() {
	ui.ColorWell(c, &app.tint)
})
```

`ui.ColorPicker` is the picker itself, to show in a panel of your own, as
AppKit's color panel: a square choosing the saturation, across, and the
brightness, up, of the hue its slider chooses below it, a slider of
opacity, the color in hex, which the user edits, and swatches of common
colors. The arrows move across the square while it has the focus, Up and
Down changing the brightness and Left and Right the saturation.

```go
ui.ColorPicker(c, &app.background)
```

## Accessibility

Assistive technology sees a color well whose value is the color in hex,
named by its `Label` or its [field](form.md). In the picker, it sees a
slider of the saturation and brightness, sliders of the hue and the
opacity, a text field named "Hex", and buttons named by the swatches'
colors.
