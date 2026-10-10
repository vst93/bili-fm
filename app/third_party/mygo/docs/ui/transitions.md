# Transitions

`Transition` makes an element move smoothly where the layout changes it:
from where the last frame showed it to where it is now, in its parent, at
its size, and in its background and border colors. The view says only where
things are; the transition animates the way there.

```go
var rowMotion = ui.ElementTransition{
	Enter: &ui.Motion{Collapse: true},
	Exit:  &ui.Motion{Collapse: true},
}

ui.Column(c).Children(func() {
	removed := -1
	for _, it := range app.items {
		ui.Row(c).Key(it.ID).Padding(8).Transition(rowMotion).Children(func() {
			ui.Text(c, it.Title).Grow(1)
			if ui.Button(c, "Remove").Clicked() {
				removed = it.ID // once the loop is done (see Views)
			}
		})
	}
	if removed >= 0 {
		app.remove(removed)
	}
})
```

Rows added grow in, rows removed collapse, and the others slide to their
places, whether the items were added, removed, sorted or shuffled.

## What moves

An `ElementTransition` animates over its `Duration` (200 ms when zero),
along its `Ease` (`ui.EaseOut` when nil, or any [easing](drawing.md#animation)):

- **Position**: where the element is in its parent. Moving with its parent,
  as when a scroll view scrolls, is not a change.
- **Size**: its width and height. The content lays out at each size on the
  way, as a panel opening wraps its text anew.
- **Colors**: its background and border colors, as a hover fades in and
  out.

All three animate unless `Position`, `Size` or `Colors` choose some:

```go
tile := ui.Box(c).Size(64, 48).Radius(8).Background(t.Surface)
if tile.Hovered() {
	tile.Background(t.SurfaceHover)
}
tile.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
```

The element needs the same ID from frame to frame: a `Key`, or the same
place among its siblings. A transition animates the element alone: the
elements that a change moves too need a transition of their own to move
smoothly, the same one to move in step, as the rows above, and the
column holding them if its size changes:

```go
open := app.sidebar
w := float32(56)
if open {
	w = 240
}
ui.Row(c).Fill().Children(func() {
	ui.Column(c).Width(w).ClipX().Transition(ui.ElementTransition{Size: true})
	ui.Column(c).Grow(1).Transition(ui.ElementTransition{})
})
```

## Coming and going

`Enter` is where an element comes from as it appears, and `Exit` where it
goes as it leaves, from its place in the layout: a `Motion` moved by `X`
and `Y`, at an `Opacity` (0, transparent, when not set), and with
`Collapse`, without height in a column or width in a row, its content
clipped.

```go
toast := ui.ElementTransition{
	Enter: &ui.Motion{Y: 12},
	Exit:  &ui.Motion{Opacity: 1, X: 300},
}
```

An element appears with its `Enter` only in a parent that was there in the
frame before, so that a page or a dialog showing all at once does not
animate every element in it. When an element is no longer built while its
parent still is, a copy of it, as the last frame showed it, goes to its
`Exit` under its siblings: it takes no room, so the siblings move into its
place at once, smoothly if they have transitions, and it takes neither the
pointer nor the focus. Built again before its exit ends, the element comes
back from where its copy was.

## When nothing moves

Changes that resizing the window makes do not animate, nor does anything
while the desktop asks for less motion (`c.Preferences().ReduceMotion`).
Frames are drawn only while something moves.

Rows of a [`List`](list.md) are placed by the list, which builds only
those in view: their content can have transitions, but the rows do not
slide, come or go with one.
