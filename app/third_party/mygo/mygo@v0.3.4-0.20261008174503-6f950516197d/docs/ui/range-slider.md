# Range slider

`ui.RangeSlider` creates a slider of two knobs setting a low and a high
`*float64` between two bounds, as a price range: each knob takes the
keyboard focus, which Tab moves between, and the arrows, Home and End move
it while it has the focus; a press on the track moves the knob nearest to
it. The knobs do not cross. `Changed` reports a new value.

```go
ui.RangeSlider(c, &app.minPrice, &app.maxPrice, 0, 1000, 10).Label("Price")
ui.Textf(c, "$%.0f – $%.0f", app.minPrice, app.maxPrice)
```

A step above 0 snaps the values to the low bound and multiples of the step
from it, with tick marks; 0 leaves them free.

## Accessibility

Assistive technology sees two sliders, named by the range slider's `Label`
and "minimum" and "maximum".
