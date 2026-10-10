# Image

`ui.Image` shows a bitmap, or an SVG in its own colors, by default at its
size in DIPs, scaled to fit when given another size.

```go
//go:embed logo.png
var logoPNG []byte

var logo, _ = ui.DecodeBitmap(logoPNG)

ui.Image(c, logo).Size(64, 64).Fit(ui.Contain).Radius(12)
```

## Bitmaps

`ui.NewBitmap` makes a bitmap of an `image.Image`, and `ui.DecodeBitmap` of
PNG, JPEG, GIF (its first frame), WebP or BMP data, turning photos upright
as their camera's EXIF orientation says. Make bitmaps once, not in the
view: MyGo keeps a bitmap on the GPU as long as you use it. A bitmap shown
much smaller than its pixels, as a photo in a thumbnail, is drawn from a
copy halved as many times as that keeps it no smaller, made once, so that
it shows every pixel's part rather than shimmering.

## Fitting

`Fit` says how an image fills its box: `Contain` (the default) fits it
inside, `Cover` covers the box and crops the rest, `FillBox` stretches it,
`ScaleDown` shows it at its own size unless that does not fit, and
`NaturalSize` at its own size, cropped. `Grayscale` draws it in shades of
gray, as for what is disabled. `Radius` rounds its corners, and
`AspectRatio` keeps its proportions in a box that grows.

```go
ui.Image(c, photo).AspectRatio(16.0 / 9).Fit(ui.Cover).Radius(8)
```

## SVGs

`ui.ParseSVG` parses an SVG, and `ui.MustParseSVG` one that is part of the
program, such as an embedded file, panicking when it is in error. `Image`
shows an SVG in its own colors, with the text color for its
`currentColor`, at the size the SVG gives (its `width` and `height`, or
those of its `viewBox`) unless given another:

```go
ui.Image(c, illustration).Width(240)
```

To show an SVG in the color of the text, as an icon, use [Icon](icon.md).

MyGo draws an SVG's shapes into the GPU's atlas once for each size it shows
at, the way it draws text, and the GPU draws them from there: moving an
image, scrolling it or changing its color draws nothing again. MyGo draws
paths and the basic shapes, groups, `use` and `symbol` elements,
transforms, fills and strokes (with their joins, caps and dashes) of
colors, `currentColor` and linear and radial gradients, opacity, clip
paths, masks and style sheets of simple selectors (type, class and id). It
leaves out text, embedded images, patterns, markers and filters: convert
text to paths in your editor before exporting.

In `Draw` callbacks, `Painter.Image` draws bitmaps and SVGs too.

## Accessibility

Images are decorations that assistive technology does not see, unless
`Label` names them, as a photo that matters.
