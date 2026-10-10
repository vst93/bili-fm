# Custom widgets

Every widget is built on a base, the same widget without a look: the base
handles the pointer, the keyboard and the focus, and tells assistive
technology what it is, and leaves every color, size and shape to the
elements it returns, which you style as any other. Build a design of your
own on them, as headless component libraries do on the web.

| Base | What it does |
|---|---|
| `ButtonBase` | a row that takes the focus, and reports `Clicked` for the pointer, Enter and Space |
| `CheckboxBase`, `SwitchBase`, `ToggleBase` | a row that toggles a `*bool` |
| `RadioBase` | a row that selects its value into a `*T` |
| `SliderBase` | sets a `*float64` from where the pointer is across its content box, inside its padding, or up it (`Vertical`), and with the arrows, Page Up and Down, Home and End, by its `Step` |
| `TabsBase` | the tab `List`, whose `Tab`s choose a `*int`, with the arrows moving the choice and the focus |
| `SegmentedBase` | a `Track` of `Segment`s, a radio group choosing a `*int` |
| `CollapsibleBase` | a `Trigger` that shows and hides what `Panel` builds, growing and shrinking it with `Progress`, which goes from 0 closed to 1 open |
| `SelectBase` | a `Trigger` opening a `Popup` of `Item`s choosing a `*T`, which the arrows highlight (`Highlighted`) and Enter chooses |
| `ComboboxBase` | an `Input` whose text filters the `Item`s of a `Popup` below it, which the arrows highlight and Enter or a click chooses (`Chosen`) |
| `PopoverBase`, `DialogBase` | a panel beside an anchor, or over a backdrop covering the window, that a press outside or Escape closes |
| `TooltipBase` | a tip beside an anchor while the pointer rests on it or the keyboard focus is on it, which a press or Escape hides ([Tooltip](tooltip.md#without-a-look)) |
| `ToastViewportBase`, `ToastBase` | the window's toasts in a viewport of your own, each a status with an `ActionButton` and a `CloseButton`, their time stopping while the pointer or the focus is on them ([Toast](toast.md#without-a-look)) |
| `TextInputBase`, `TextAreaBase` | text inputs without padding, background, border or corners, `ReadOnly` or not |

Base constructors return value parts with the same build lifetime as other
elements. Give stateful bases a key with `Context.Key` before construction.

A segmented control on `SegmentedBase`, and a select on `SelectBase`:

```go
t := c.Theme()
view := ui.SegmentedBase(c.Key("period"), &app.view, 3)
view.Track.Padding(3).Radius(999).Background(t.Surface).Children(func() {
	for i, name := range []string{"Day", "Week", "Month"} {
		seg := view.Segment(i).Padding(5, 14).Radius(999)
		if i == app.view {
			seg.Background(t.Background).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.15))
		}
		seg.Children(func() { ui.Text(c, name) })
	}
})

sel := ui.SelectBase(c.Key("size"), &app.size)
sel.Trigger.Gap(6).Padding(6, 10).Radius(8).Border(1, t.Border).Children(func() {
	ui.Text(c, app.size)
	ui.Icon(c, chevron)
})
sel.Popup(func(panel ui.Element) {
	panel.Margin(4, 0, 0, 0).Padding(4).Radius(10).Background(t.Background).Border(1, t.Border)
	for _, size := range sizes {
		item := sel.Item(size).Padding(6, 10).Radius(6)
		if item.Highlighted() {
			item.Background(t.Accent).TextColor(t.AccentText)
		}
		item.Children(func() { ui.Text(c, size) })
	}
})
```

Bases ring the element with the keyboard focus, as every element taking
the focus is; `FocusRing(false)` turns the ring off for a widget that draws
its own, with `Painter.FocusRing` while `FocusVisible`.

## Widgets of your own

For text controls with their own buffer or selection model, use
[`HandleTextInput` and retained text geometry](text-input-client.md).

Build your own widgets from elements. An element keeps state of its own
from frame to frame with `ui.Local`:

```go
// Spoiler hides text until clicked.
func Spoiler(c *ui.Context, text string) {
	t := c.Theme()
	box := ui.Box(c).Padding(2, 6).Radius(4).Focusable()
	shown := ui.Local(box, "shown", func() bool { return false })
	if box.Clicked() {
		*shown = !*shown
	}
	if *shown {
		box.Background(t.Surface)
	} else {
		box.Background(t.Text) // the color of the text, hiding it
	}
	box.Children(func() { ui.Text(c, text) })
}
```

Give a widget a [role](accessibility.md#roles), a name and its
[states](accessibility.md#states) for assistive technology, take the keys
it needs with `Shortcut` on its element, or every key as it comes with
[`HandleInput`](input.md#every-key-as-it-comes), and paint what elements
do not with [`Draw`](drawing.md). Overlays of your own, as drawers, hover
cards and menus, are placed with `AttachTo` and close with
`PressedOutside` and `OverlayShortcut`; `Modal` keeps the focus in one:
see [overlays](overlays.md#your-own-overlays).
