# Styling and themes

`Background`, `Border`, `Radius`, `Shadow` and `Opacity` style an
element's box, and `Cursor` sets the pointer over it:

```go
ui.Column(c).Padding(18).Gap(12).Radius(10).
	Background(t.Background).Border(1, t.Border).
	Shadow(0, 1, 3, 0, ui.RGBA(0, 0, 0, 0.06))
```

- **Borders.** `Border(1, c)` draws one inside every edge; `BorderWidth`
  sets the sides apart, CSS style, with `BorderColor`, as a line under a
  header with `BorderWidth(0, 0, 1, 0)`, and `BorderStyle(ui.BorderDashed)`
  dashes it. As with CSS's `box-sizing: border-box`, the border takes room
  within the element's size, and the padding and the children are inside
  it.
- **Shadows.** `Shadow(x, y, blur, spread, c)` casts a box shadow, as CSS's
  `box-shadow` does: several stack, and each shows only outside the box,
  so a translucent background never shows its own shadow through.
- **Gradients and stripes.** `Gradient(from, to, angle)` fills the box with
  a linear gradient; `LinearGradient` also places its colors along the line
  (`Start`, `End`) and mixes them in Oklab, which keeps their lightness,
  instead of sRGB. `Stripes(c, width, gap, angle)` draws stripes over the
  background, as on what is unavailable.
- **Materials.** `Material(m)` fills the box with a material a package
  paints, in place of a background, as the [glass
  plugin](../plugins/glass.md)'s Liquid Glass, which what is under it shows
  through, frosted and bent along its edges, its scroll edges and its
  backdrop blur, which may fade along a gradient.
- **Pointer.** `Cursor` takes the shapes of the platforms: `CursorPointer`,
  `CursorText`, `CursorMove`, `CursorGrab` and `CursorGrabbing`, the
  resize cursors (both ways, toward one side as `CursorResizeE`, and of
  columns and rows), `CursorCopy` and `CursorAlias` for drops,
  `CursorContextMenu`, `CursorVerticalText`, `CursorNotAllowed`,
  `CursorCrosshair`, and `CursorNone`, which hides it.

`ui.Oklch(l, c, h)` is CSS's `oklch()`; `.Alpha(a)` is its `/ a`. It
returns a `ui.Color` like any other, so fills, borders, gradients,
stripes, shadows, dividers, text, decorations, text backgrounds, the
glass's tint and the theme (`t.Accent = ui.Oklch(0.6, 0.2, 240)`) all take
it. It can name colors outside sRGB, more vivid than most screens show.
Such a color keeps both: its real value, drawn when the window is on a
screen that shows a wider range (Display P3 on recent Macs), and the
nearest sRGB color, chosen as CSS does (less saturated, same lightness and
hue). That fallback is its `R`, `G` and `B`, and is what sRGB screens,
Linux, Windows and SVG pictures draw; `SRGB()` returns it without the real
value. `Mix` and transitions mix the real values: a mix back inside sRGB
is an ordinary color. Don't assign `R`, `G` or `B` of such a color, as the
real value would no longer match: make a new one.

Colors come from `ui.RGB`, `ui.RGBA` and `ui.Hex("#2563eb")`; `Mix` blends
two, and `Alpha` makes one translucent.

## State, not selectors

There are no style sheets and no state selectors: the view is code, so an
element's look follows the state in the view itself, as
`if row.Hovered() { row.Background(t.SurfaceHover) }`, and one element's
state can style another, as a group's hover does in CSS.

A widget returns its element, so a call after it styles it differently
from the rest: `ui.Button(c, "Save").Padding(10, 20).Radius(999)`. For a
look of your own, build on the widgets' bases, which have none: see
[custom widgets](custom-widgets.md).

## Themes

Widgets take their colors and metrics from the theme, `c.Theme()`: the
light or the dark theme, following the system's appearance as it changes,
and the settings of the desktop that the system's own controls follow. Use
its colors in your own elements so they follow too. To change it, set a
copy:

```go
t := *c.Theme() // the default, which follows the system
t.Accent, t.Radius = ui.Hex("#7c3aed"), 8
t.Spacing = 3 // compact
c.SetTheme(&t)
```

`Spacing` is the unit of the room widgets leave: their paddings and gaps,
and the sizes of check boxes, switches, sliders and the rows of tables and
trees, are multiples of it. It is 4 by default; 3 makes every widget
compact, 5 roomy. `FontSize` sizes their text, `Radius` rounds their
corners, and `ScrollbarWidth` sets the width of scroll bars. On macOS,
rounded corners, the widgets' and your elements', curve continuously as
AppKit's and SwiftUI's do, and elsewhere as quarter circles, as Windows
and GTK draw them. Size your own
elements with the theme too, and they follow it: `t.Space(3)` is three
units of its spacing, and `t.Rem(2)` twice its font size, as CSS's rem.

Tooltips and toasts are the theme turned over, so that they stand out from
what they are over: filled with `Inverse`, with `InverseText` on it, which
left zero are the theme's `Text` and `Background`. A dark app that would
rather keep them dark sets both:

```go
t.Inverse, t.InverseText = ui.Hex("#3f3f46"), ui.Hex("#fafafa")
```

## The desktop's preferences

`c.Preferences()` returns the settings of the desktop, which the default
theme follows and a frame follows the changes of:

- **`Accent`.** The accent color the user chose (macOS, Windows, and
  desktops whose portal gives one, as GNOME 47 and KDE do), which colors
  primary buttons, the choice and the focus ring, with text on it that
  stands out.
- **`HighContrast`.** macOS's Increase Contrast, Windows's contrast themes,
  the portal's higher contrast: borders and secondary text are darker
  (lighter in the dark), and the focus ring opaque.
- **`TextScale`.** Windows's and GNOME's text size: `FontSize` is that many
  times larger.
- **`ReduceMotion`.** macOS's Reduce Motion, Windows's animation effects and
  GNOME's animations turned off: `Animate` goes to its target at once.
