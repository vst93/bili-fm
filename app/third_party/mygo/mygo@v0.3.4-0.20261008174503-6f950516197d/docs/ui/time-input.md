# Time input

`ui.TimeInput` creates a field of the time of day of a `*time.Time`, as
15:04, whose hours and minutes each take the keyboard focus, as the
segments of AppKit's date picker: Up and Down step the one with the focus
round the clock, digits typed set it, going on to the minutes once the
hours are typed, and Left and Right move between them. `Changed` reports a
new time, which keeps the date and the location of the value.

```go
ui.TimeInput(c, &app.alarm).Label("Alarm")
```

A click on the hours or the minutes gives them the focus. With a
[date input](date-input.md), the two edit one `time.Time`.

## Accessibility

Assistive technology sees two spin buttons, named after the field's
`Label` and "hours" and "minutes".
