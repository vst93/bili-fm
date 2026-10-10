# Number input

`ui.NumberInput` creates a text input editing a `*float64` as a number
between two bounds, which Up and Down, and the buttons beside it, change by
a step. What is typed applies as soon as it is a number in range, and shows
rounded to the decimals of the step once the input loses the focus.
`Changed` reports a new value.

```go
ui.NumberInput(c, &app.copies, 1, 99, 1).Label("Copies")
ui.NumberInput(c, &app.opacity, 0, 1, 0.05).Label("Opacity")
```

For a value the user only steps, use a [stepper](stepper.md); for one
chosen roughly, a [slider](slider.md).

## Accessibility

Assistive technology sees a spin button of the value, named by its `Label`
or its [field](form.md), which it increments and decrements by the step.
