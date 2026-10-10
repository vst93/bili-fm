# Glass

The glass plugin draws Liquid Glass in native UI, the material of macOS 26
and later: what is under an element shows through it, frosted, bent near
its edges as through the rim of a lens, and lit along its hairline rim. It
also draws macOS's scroll edges, which content scrolling under
a bar fades or frosts under ([Scroll edges](#scroll-edges)), and blurs
what is under an element without the glass, evenly or fading along a
gradient ([Blur](#blur)). MyGo draws them with the plugin's shaders, on
the GPU or the CPU, so they look the same on macOS, Windows and Linux. It
is all Go: no JavaScript package, and nothing to `mygo.Use`.

```go
import "github.com/egoist/mygo/plugins/glass"

ui.Row(c).Padding(8, 16).Gap(8).Radius(22).Material(glass.Glass{}).Children(func() {
	ui.Text(c, "On glass")
})
```

`glass.Glass` is a material (`ui.Material`): `Material` fills the element
with it in place of a background, and its `Radius` is its shape; its
children draw on it. What shows through is what was painted under it in
the same window: the elements before it, as content that scrolls under a
toolbar placed over it with `Absolute`, and other glass.

```go
ui.Box(c).Fill().Children(func() {
	ui.Scroll(c).Fill().Padding(64, 16, 16).ScrollbarInsets(64, 0, 0).Children(func() { /* ... */ })
	// The toolbar floats over the content, which scrolls under it.
	ui.Row(c).Absolute().Top(12).Left(12).Right(12).Padding(6).Radius(26).Material(glass.Glass{}).Children(func() {
		// ...
	})
})
```

## Styles

- **`glass.Regular`**, the default, frosts what shows through and
  lightens it, or darkens it in dark mode, so that what is on the glass
  reads over anything: bars, buttons and panels. Larger panes are
  frostier, as on macOS.
- **`glass.Clear`** barely blurs or tones what shows through: glass over
  photos and video, where what is on it brings its own contrast.

`Tint` colors the glass toward a color, by its alpha, as a prominent
button: `glass.Glass{Tint: t.Accent}`, with text in `t.AccentText`.
`Interactive` makes the glass grow a little while it is pressed, as
AppKit's interactive glass does on macOS 27: about 1.1 DIPs left and
right and 0.45 above and below, whatever its size, within 150 ms. AppKit
stretches the content too, by a pixel or so, which the children here are
not.

```go
done := ui.Row(c).Padding(8, 16).Radius(20).Material(glass.Glass{Tint: t.Accent, Interactive: true}).Children(func() {
	ui.Text(c, "Done").Bold().TextColor(t.AccentText)
})
if done.Clicked() {
	// ...
}
```

In a drawing, `glass.Paint(p, r, radius, g)` paints a pane of glass over
what the painter painted before it.

## Scroll edges

`glass.ScrollEdge` is the scroll edge effect of macOS 26 and later, a
material for an element over the edge of content that scrolls under a bar
of controls, so that they stand out from it, as SwiftUI's `safeAreaBar`
with `scrollEdgeEffectStyle` draws it:

```go
ui.Box(c).Fill().Children(func() {
	ui.Scroll(c).Fill().Padding(64, 16, 16).ScrollbarInsets(64, 0, 0).Children(func() { /* ... */ })
	// The content fades into the background under the bar.
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(74).PassThrough().Material(glass.ScrollEdge{})
	// Buttons of glass float over it.
	ui.Row(c).Absolute().Top(12).Left(12).Right(12).Gap(6).PassThrough().Children(func() {
		// ...
	})
})
```

- **Soft**, the default, fades what the content scrolls over into it:
  the background at 85% at the bar's edge, falling to nothing at the
  element's other edge, with no blur. Make the element the bar's height
  and 10 DIPs more, as macOS does.
- **`Hard`** frosts the content under the bar evenly: blurred by about 6
  DIPs, its colors saturated by a quarter, under 82% of the background,
  with a hairline along the element's other edge, black at 10% in light
  mode and white at 7% in dark mode. Make the element the bar's height.

`Bottom` puts the bar at the element's bottom, and `Background` sets what
the content scrolls over, the theme's `Background` unless set. Paint the
edge before what floats on it, and let `PassThrough` take the pointer to
the content.

They are macOS 27's, measured from SwiftUI's over test patterns: in its
layers, a soft edge replays the window's background under a gradient mask
from 85% to nothing, 10 points past the bar, and a hard edge blurs the
content (a Gaussian of radius 6, saturation 1.25 and 0.03 more), then
replays the background at 82.45% over it. Gray content matches AppKit's
within 1 of 255 in every row, light and dark; saturated colors differ by
up to 0.05, as AppKit mixes them in the display's wider color space and
MyGo's renderers in sRGB. macOS's window toolbars draw an edge of their
own, an even blur under 60% of the background, which a bar of native UI
does not have.

## Blur

`glass.Blur` is a backdrop blur, a material too: what is under the element
shows through it blurred by `Radius` DIPs (the standard deviation, as CSS's
`blur()` takes), with no tint, rim or shadow.

`Mask` makes it a progressive blur: a `LinearGradient` whose alpha says how
much of `Radius` each place blurs by, as CSS's `mask-image` does. Where the
gradient is opaque, what shows through blurs by `Radius`, where it is
translucent by less, and where it is transparent not at all; its colors do
not show. Under a bar, content then blurs the most at its edge, and less
and less further from it.

```go
ui.Box(c).Fill().Children(func() {
	ui.Scroll(c).Fill().Padding(64, 16, 16).ScrollbarInsets(88, 0, 0).Children(func() { /* ... */ })
	// What scrolls under the toolbar blurs, the more the nearer the top.
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(88).PassThrough().
		Material(glass.Blur{Radius: 6, Mask: &ui.LinearGradient{
			From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180, Start: 0.3, End: 1,
		}})
	// Buttons of glass float over it.
	ui.Row(c).Absolute().Top(12).Left(12).Right(12).Gap(6).PassThrough().Children(func() {
		// ...
	})
})
```

Here the blur is whole over the top 30% of the strip and fades out below,
`PassThrough` lets the pointer reach the content under the strip and
around the buttons, and `ScrollbarInsets` starts the scroll bar below the
strip, as AppKit starts a scroller below its toolbar's edge effect. Paint the blur before what floats on it: glass over
it shows the blurred content through. `Start` and `End` place the
gradient's colors along it, as everywhere; leaving `End` 0 puts both at
`Start`, a hard edge.

A blur that varies is drawn in steps, each blurring what the step before
drew a little more, shown where the blur wanted is more than the step's
own, mixed with the next between them: one step for each doubling of the
blur from two pixels, five for 12 DIPs on a Retina display. A pixel
between two steps shows both blurs mixed, as Core Animation's variable
blur does between the levels of its pyramid, and as the stacked
`backdrop-filter` layers of a progressive blur on the web do. Where the
gradient is opaque at an edge of the element in the middle of the window,
rather than at the window's edge, the blur differs a little within a few
pixels of that edge from one blurring each pixel by its own amount, as
each step reads what is around the element unblurred.

## How it looks like macOS

The glass follows macOS 27's, measured from AppKit's `NSGlassEffectView`,
with the optics of the open-source reproductions of Liquid Glass:

- **The bezel.** The surface is flat in the middle and curves down to its
  edges along Apple's squircle profile, over the last 36 DIPs, at most half
  of the pane. Light coming straight down refracts entering it, as through
  glass of refractive index 1.5, so what is near the edge comes from
  further inside: the rim mirrors what is just inside it, as a lens's
  does.
- **The material.** The regular glass blurs what shows through by up to 10
  DIPs, more for larger panes, and maps its lightness, black to 54% and
  white to 100% in light mode, and to 15% and 51% in dark mode, keeping
  its colors. The clear glass adds an eighth to it.
- **The light.** The rim is lit where it faces up or down and shaded where
  it faces the sides, within a pixel of the edge: a hairline, as AppKit's.
- **No shadow.** Neither `NSGlassEffectView` nor a button of the glass
  bezel casts one: the pane's edge is its rim alone.

## Performance

Each pane reads what is under it, averages and blurs it, then draws with
its own shader: a few small passes on the GPU, and a pane changes when
anything under it does, as content scrolling under a bar. When a window
draws on the CPU, without a GPU, as [Rendering](../ui/rendering.md)
describes, a change under a pane redraws the pane and what its blur
reaches around it. A
progressive blur does this for each of its steps, where each shows: on
the CPU, a strip of 1360×176 pixels blurred by 24 takes 9 ms on an M5
fading, 1.7 evenly, and on the GPU a few small passes for each step.

Panes are rounded rectangles. They do not merge into each other when
close, as AppKit's `NSGlassEffectContainerView` does.
