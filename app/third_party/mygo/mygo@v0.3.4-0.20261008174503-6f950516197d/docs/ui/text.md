# Text

`ui.Text` shows text that wraps at the width it gets, and `ui.Textf` formats
it:

```go
ui.Text(c, "Settings").FontSize(20).Bold()
ui.Textf(c, "%d files, %s", n, size).TextColor(c.Theme().TextMuted)
```

## Style

`FontSize`, `FontWeight`, `Bold`, `Italic`, `Font`, `LineHeight`,
`FixedLineHeight` (in DIPs, whatever the font size), `TextColor`,
`TextAlign`, `Underline`, `WavyUnderline`, `Strikethrough`,
`DecorationColor` and `DecorationThickness` (of the underline and
strikethrough), `TextBackground` (a highlight behind the lines),
`LetterSpacing` and `FontFeatures` style it, set on the text or on any
element above it, whose texts inherit them. `FontFeatures` turns on
OpenType features of the font by tag, or sets them with `tag=value`:
`FontFeatures("tnum")` gives digits of one width for numbers that change,
`FontFeatures("liga=0")` turns ligatures off.

`SingleLine` keeps text on one line, cut with an ellipsis, `MaxLines`
limits it to a few, and `Ellipsis(" →")` ends what they cut with another
mark; `NoWrap` keeps its lines whole, breaking them only at newlines.
`Selectable` lets the user select a text with the pointer, Shift and the
arrows, and copy it, as an error message or an identifier to paste
elsewhere.

Put `Selectable` on a container to share one selection across its `Text`
and `RichText` descendants:

```go
ui.Column(c).Selectable().Gap(12).Children(func() {
	ui.Text(c, "First paragraph.")
	ui.Text(c, "Second paragraph.")
})
```

A drag can cross paragraphs in either direction. Shift-click and Shift
with the arrows extend it, and dragging near a scroll container's edge
scrolls it. Cmd+C (Ctrl+C on Linux and Windows), the Edit menu's Copy,
and the context menu copy the selected text, joined with newlines in the
order the paragraphs were built. Select All selects the whole container,
including built paragraphs outside its viewport. Inline text and links
inside a paragraph are copied as part of that paragraph.

Nested `Selectable` containers have separate selections. Buttons and
text inputs keep their own interaction; their labels do not join the
container's selection. Use `Unselectable` on a text or a container to
exclude its subtree; inline elements follow their paragraph's selection.
Changing the selected text or removing an endpoint clears the shared
selection. A virtualized list only includes the paragraphs it has built,
so use a regular scroll container when selecting an entire document.

The highlight is the theme's `Selection`; `SelectionColor` sets another
for an element and the text inside it, as text on a colored bubble needs
one that shows on it. A right-click on selectable text shows Copy and
Select All; a `ContextMenu` on the text or its `Selectable` container
replaces that menu, and `Menu.EditItems` puts Copy and Select All in it.

## Rich text

`ui.RichText` mixes styles in one paragraph: each `ui.Span` sets what it
changes (font, size, weight, italics, color, underlines, strikethrough,
their color and thickness, a background, letter spacing, features) over
the style of the text, and the spans wrap together:

```go
ui.RichText(c,
	ui.Span{Text: "Saved "},
	ui.Span{Text: "report.pdf", Weight: 600},
	ui.Span{Text: " to "},
	ui.Span{Text: "Documents", Color: t.Accent, Underline: true},
)

ui.RichText(c,
	ui.Span{Text: "Did you mean "},
	ui.Span{Text: "recieve", WavyUnderline: true, DecorationColor: t.Danger},
	ui.Span{Text: "? "},
	ui.Span{Text: "match", Background: ui.RGBA(250, 204, 21, 0.4)},
)
```

## Elements within a sentence

Text elements built in a text's `Children` continue its paragraph, as
HTML's inline elements do: each styles its own text over the paragraph's,
and keeps what elements do (`Clicked`, `Hovered`, the keyboard focus, a
`Tooltip`, a `ContextMenu`), with its words, on every line they take, as
its area. A [link](link.md) inside is a link within the sentence, which Tab
reaches and assistive technology reads as a link:

```go
ui.RichText(c).Children(func() {
	ui.Text(c, "Read ")
	ui.Link(c, "the guide", "https://example.com/guide")
	ui.Text(c, " or ")
	if ui.Text(c, "show an example").TextColor(t.Accent).Clicked() {
		app.example = true
	}
	ui.Text(c, ".")
})
```

Only text elements (`Text`, `Link`, `RichText`) go inside a text; their
sizes, padding, borders and corners do not apply, and a `Background`
highlights their text.

## Fonts

Text is laid out and drawn by the system's own text engine (DirectWrite on
Windows, Core Text on macOS, Pango on Linux) in the system's font (Segoe UI,
SF, the desktop's interface font, as GTK apps have it), falling back to the
system's fonts for other scripts and emoji as native apps do, with
right-to-left text in its order. `Font("monospace")` picks the system's
monospaced font, and `ui.RegisterFont` adds your own:

```go
//go:embed Inter.ttf
var inter []byte

func init() {
	if err := ui.RegisterFont(inter, "Inter"); err != nil {
		log.Fatal(err)
	}
}
```

Then `Font("Inter")` uses it, or set it for every element in the theme's
`Font`. A list of families, as `Font("Inter, Noto Sans JP")`, draws with
the first the system or the app has, and what it lacks with the next that
has it, before the system's own choice.

Text that a widget lays out itself, as in the cells of a terminal, is
shaped once with `ui.Shape`: see [drawing](drawing.md#shaping-text).
