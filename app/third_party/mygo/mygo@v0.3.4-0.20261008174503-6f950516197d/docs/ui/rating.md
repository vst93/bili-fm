# Rating

`ui.Rating` creates a row of stars, as AppKit's rating level indicator, of
which an `*int` are filled: a click on a star sets the rating, a click on
the star set clears it, and the arrows, Home and End change it while the
rating has the keyboard focus. `Changed` reports a new value.

```go
ui.Rating(c, &app.stars, 5).Label("Rating")
```

The stars take their size from the theme's font size, and show the rating
a click would set as the pointer rests on them.

## Accessibility

Assistive technology sees a slider from 0 to the number of stars, named by
its `Label`.
