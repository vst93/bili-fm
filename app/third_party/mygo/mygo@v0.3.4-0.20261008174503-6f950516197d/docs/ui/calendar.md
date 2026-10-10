# Calendar

`ui.Calendar` creates a month's calendar choosing the day of a
`*time.Time`, as SwiftUI's graphical date picker: a click chooses a day, as
the arrows do while the calendar has the keyboard focus; Page Up and Page
Down, or its buttons, move by months, and Home and End go to the first and
the last day of the month. `Changed` reports a new date, which keeps the
time of day and the location of the value.

```go
ui.Calendar(c, &app.day).Label("Due date")
```

It shows six weeks from Monday, the days of the months around in gray, and
marks today.

For a date in a compact field, use a [date input](date-input.md), which
opens this calendar below it.

## Accessibility

Assistive technology sees a table named by its month, whose days are
buttons named by their dates, and reads the day chosen as the arrows move.
