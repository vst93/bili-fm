# Slider

`ui.Slider` creates a slider setting a `*float64` between a low and a high
value: dragging the knob, or a press on the track, sets it, as do the
arrows, Page Up and Page Down (a tenth of the range), Home and End while
it has the keyboard focus. `Changed` reports a new value.

```go
ui.Slider(c, &app.volume, 0, 100).Label("Volume")
```

## Steps

`ui.StepSlider` snaps the value to the low value and multiples of a step
from it, with a tick mark at each, as AppKit's sliders with tick marks:
dragging snaps to them, and the arrows step by step.

```go
ui.StepSlider(c, &app.quality, 0, 100, 25).Label("Quality")
```

For a low and a high value at once, as a price range, use a
[range slider](range-slider.md); for exact numbers, a
[number input](number-input.md) or a [stepper](stepper.md).

## Without a look

`ui.SliderBase` is a slider without a look: dragging across its content
box, within its padding, sets the value. Draw its track and thumb where
`(value-lo)/(hi-lo)` puts them, and pad it by half the thumb's width to
keep the thumb inside it:

```go
s := ui.SliderBase(c, &app.volume, 0, 100).Height(24).PaddingX(12)
s.Draw(func(p *ui.Painter, r ui.Rect) {
	x := r.X + 12 + (r.W-24)*float32(app.volume/100)
	p.Fill(ui.Rect{X: r.X + 12, Y: r.Y + 10, W: r.W - 24, H: 4}, gray, 2)
	p.Fill(ui.Rect{X: x - 12, Y: r.Y, W: 24, H: 24}, blue, 12)
})
```

`Step` snaps its values to the low value and multiples of a step, which
the arrows move between, rather than by a hundredth of the range, and
`Vertical` makes it go up, from the low value at the bottom of its content
box; pad it by half the thumb's height then:

```go
ui.SliderBase(c, &app.level, 0, 10).Step(1).Vertical().Size(24, 120).PaddingY(12).Draw(drawLevel)
```

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a slider of its range and value, named by its
`Label`, horizontal or vertical, which it increments and decrements as the
arrows do.
