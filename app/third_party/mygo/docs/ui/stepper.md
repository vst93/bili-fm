# Stepper

`ui.Stepper` creates a pair of arrows changing a `*float64` by a step
between two bounds, as AppKit's stepper beside a field: a click on an arrow
steps up or down, as do Up and Down while the stepper has the keyboard
focus, and holding an arrow keeps stepping, faster the longer it is held.
`Changed` reports a new value.

```go
ui.Row(c).Gap(6).Children(func() {
	ui.Textf(c, "%.0f copies", app.copies)
	ui.Stepper(c, &app.copies, 1, 99, 1).Label("Copies")
})
```

For a number the user also types, use a [number input](number-input.md),
which has steppers of its own.

## Accessibility

Assistive technology sees a spin button of the value, named by its
`Label`, which it increments and decrements.
