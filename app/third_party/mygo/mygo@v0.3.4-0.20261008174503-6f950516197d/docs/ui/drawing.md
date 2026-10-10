# Drawing and animation

`Draw` paints on an element after its background, and `DrawOver` after its
children, with a `*ui.Painter` in DIPs of the window: rectangles with
`Fill`, `FillGradient`, `Stroke` and `StrokeDashed`, `Shadow`, `Line`,
`Image`, `Clip`, paths of lines and curves with `FillPath` and
`StrokePath`, or in a gradient with `FillPathGradient` and
`StrokePathGradient`, and text with `Text`, or `RichText` in spans of any
style, which `MeasureText` measures first, to center or align it:

```go
ui.Box(c).Height(120).Draw(func(p *ui.Painter, r ui.Rect) {
	var wave ui.Path
	for i := 0; i <= 100; i++ {
		x := r.X + r.W*float32(i)/100
		y := r.Y + r.H/2 + 40*float32(math.Sin(float64(i)/8))
		if i == 0 {
			wave.MoveTo(x, y)
		} else {
			wave.LineTo(x, y)
		}
	}
	p.StrokePath(&wave, 2, c.Theme().Accent)
})
```

```go
ui.Box(c).Size(200, 40).Draw(func(p *ui.Painter, r ui.Rect) {
	label := ui.Span{Text: "42%", Weight: 600, Color: c.Theme().TextMuted}
	w, h := p.MeasureText(0, label)
	p.RichText(r.X+(r.W-w)/2, r.Y+(r.H-h)/2, 0, label)
})
```

Draw functions only paint: MyGo may call them more than once a frame.
`Painter.Icon` and `Painter.Image` draw SVGs and bitmaps, and
`Painter.FocusRing` the ring of the keyboard focus, for widgets that draw
their own.

## Shaping text

Text that a widget lays out itself, as in the cells of a grid, is shaped
once with `ui.Shape`, which returns glyphs placed along a line by the
system's text engine, with ligatures, kerning and fallback fonts, and the
runes each comes from. Move their `X` and draw them with `Painter.Glyphs`;
`Font.Metrics` returns the font's ascent, descent and line gap, and
`Painter.Scale` the device pixels of a DIP, to line things up with the
display's pixels. `Shape` caches nothing, unlike the text of elements: keep
the glyphs of text drawn in many frames. `Font.Features` turns OpenType
features on and off, and `Font.Thicken` draws text with a thicker stroke on
macOS, as Ghostty's `font-thicken`, and `Font.Antialiased` a thinner one,
without font smoothing, as browsers draw text styled
`-webkit-font-smoothing: antialiased`.

```go
font := ui.Font{Family: "monospace", Size: 13}
glyphs := ui.Shape("grid", font)
for i := range glyphs {
	glyphs[i].X = float32(glyphs[i].Cluster) * cellWidth // one per cell
}
ui.Box(c).Height(20).Draw(func(p *ui.Painter, r ui.Rect) {
	p.Glyphs(glyphs, r.X, r.Y+font.Metrics().Ascent, c.Theme().Text)
})
```

## Animation

For motion, `Element.Animate` returns a value that eases to a target and
draws frames until it gets there:

```go
panel := ui.Column(c).Clip()
width := float32(0)
if app.sidebar {
	width = 280
}
panel.Width(panel.Animate("width", width, 200*time.Millisecond))
```

`AnimateWith` takes the easing: `ui.Linear`, `ui.EaseIn`, `ui.EaseOut` (as
`Animate`), `ui.EaseInOut`, any `func(t float32) float32`, or
`ui.Bounce(e)`, which goes along `e` and comes back. `Loop` returns the
progress of an animation that starts over every period, for spinners and
pulses; `Rotate` turns an element, as an icon:

```go
spin := ui.Icon(c, loader)
spin.Rotate(spin.Loop("spin", time.Second, ui.Linear) * 360)

skeleton := ui.Box(c).Height(14).Radius(7).Background(t.Border)
skeleton.Opacity(0.4 + 0.6*skeleton.Loop("pulse", 1600*time.Millisecond, ui.Bounce(ui.EaseInOut)))
```

To animate in other ways, compute from `c.Now()` and call
`c.AnimationFrame()` in every frame that moves: MyGo draws the next frame
when the display can show it, and draws nothing while nothing changes.

A drawing that moves while the layout stays, as a spinner or a chart
scrolling by, animates from its `Draw` instead: compute it from `p.Now()`
and call `p.AnimationFrame()` while it moves, or `p.After(d)` when it
changes in steps. Unless something else changed, those frames paint the
elements of the last frame again without running the view, so they cost
only the painting; and an element out of view is not painted, so it asks
for none. While other windows cover the window, nothing that moves draws
frames, from `Draw` or the view, until it shows again:

```go
ui.Box(c).Height(4).Draw(func(p *ui.Painter, r ui.Rect) {
	p.AnimationFrame()
	x := r.X + (r.W-20)*float32(p.Now().UnixMilli()%1000)/1000
	p.Fill(ui.Rect{X: x, Y: r.Y, W: 20, H: r.H}, c.Theme().Accent, 2)
})
```

Such a `Draw` sees the state as the view last ran: what it reads that
changes otherwise is the view's to build anew, after `Window.Update`.

When the desktop asks for less motion (`c.Preferences().ReduceMotion`),
`Animate` and `AnimateWith` go to their target at once. `Loop` goes on, as
the system's spinners do: it shows that something is going on. Motion you
compute yourself should read the preference too.
