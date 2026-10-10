# Popover

`ui.Popover` shows a panel its function builds below an element, its
anchor, while a `*bool` is true; pressing outside it or Escape sets it to
false. The press goes on to what is under the pointer, as with the web's
popovers: a click on another button closes the popover and presses the
button, and one on the anchor closes it, as the anchor toggles it. Where
there is no room below the anchor, the panel shows above it, and it
follows the anchor as the window scrolls.

```go
more := ui.Button(c, "More ▾")
if more.Clicked() {
	app.menu = !app.menu
}
ui.Popover(c, more, &app.menu, func() {
	if ui.Button(c, "Rename").Clicked() {
		app.menu, app.renaming = false, true
	}
	if ui.Button(c, "Duplicate").Clicked() {
		app.menu = false
		app.duplicate()
	}
})
```

A popover's elements follow its anchor as Tab moves, and as it closes with
the focus in it, it gives the focus back to the element that had it. See
[overlays](overlays.md).

For a menu of the system's, use a [menu button](menu-button.md); for a tip,
a [tooltip](tooltip.md).

## Without a look

`ui.PopoverBase` is a popover without a look: its function styles the panel
and builds its content. A top margin keeps the panel apart from the anchor,
on either side:

```go
ui.PopoverBase(c, anchor, &app.open, func(panel ui.Element) {
	panel.Margin(6, 0, 0, 0).Padding(8).Radius(12).Background(t.Background).Shadow(0, 8, 24, 0, ui.RGBA(0, 0, 0, 0.2))
	app.filters(c)
})
```

The panel goes elsewhere with `AttachTo`: to the right of the anchor, its
middles lined up, and to the left where there is no room on the right:

```go
ui.PopoverBase(c, anchor, &app.open, func(panel ui.Element) {
	panel.AttachTo(anchor, ui.AnchorRight, ui.AnchorLeft).Margin(0, 0, 0, 6)
	app.details(c)
})
```

Popovers of your own, as a hover card, are built in an overlay with
`AttachTo` and `PressedOutside`: see [overlays](overlays.md#your-own-overlays).

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees the panel as a popup after its anchor.
