# Icon

`ui.Icon` shows an SVG as an icon: in the color of the text, which it takes
from its ancestors as text does, whatever colors the file has, and as high
as the font size, keeping the SVG's aspect ratio. Icon sets such as Lucide,
Heroicons, Tabler or Material Symbols work as they are:

```go
//go:embed icons/save.svg
var saveSVG []byte

var save = ui.MustParseSVG(saveSVG)

ui.Row(c).Gap(6).Children(func() {
	ui.Icon(c, save)
	ui.Text(c, "Save")
})
ui.Icon(c, save).FontSize(24).TextColor(c.Theme().Danger)
```

`FontSize` and `TextColor` size and color it, as they do text; `Size` gives
it another size. An icon does not stretch across a column.

## In buttons

Inside a button, which lays out its children in a row and gives them its
text color, an icon goes before the label:

```go
ui.PrimaryButton(c, "").Children(func() {
	ui.Icon(c, save)
	ui.Text(c, "Save").SingleLine()
})
```

`Rotate` turns an icon, as a spinner or an arrow that opens:

```go
spin := ui.Icon(c, loader)
spin.Rotate(spin.Loop("spin", time.Second, ui.Linear) * 360)
```

How MyGo draws SVGs, and what of them it draws, is on the
[Image](image.md#svgs) page. In `Draw` callbacks, `Painter.Icon` draws an
icon.

## Accessibility

Icons are decorations that assistive technology does not see, unless
`Label` names them: name a button showing only an icon instead.
