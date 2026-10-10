# Tooltip

`Tooltip` shows a tip once the pointer rests on an element, or the keyboard
focus comes to it, and describes the element to assistive technology:

```go
ui.Button(c, "").Label("Share").Tooltip("Share with others").Children(func() {
	ui.Icon(c, share)
})
```

The tip shows once the pointer has rested on the element for 0.6 seconds,
below and to the right of it, kept in the window, and at once below the
element as the keyboard focus comes to it from the keyboard. Within 0.4
seconds of a tip going, the next one shows at once, as the pointer moves
along a toolbar. It goes as the pointer leaves, and a press or a click on
the element, or Escape, hides it until the pointer leaves and comes back,
or the focus does: it does not show again after a click, nor beside the
menu a menu button opens. Over elements inside one another, the innermost
one's tip shows, and one tip shows at a time. Give one to buttons showing
only an icon, and to what a label alone does not explain.

The tip takes the theme's `Inverse` and `InverseText` colors, its text and
background turned over unless the theme sets them (see
[themes](styling.md#themes)).

## Without a look

`TooltipBase` shows a tip of your own by an element, as Base UI's Tooltip
does: it decides when the tip shows and goes, as above, and `fn` styles the
tip and builds its content. The tip goes above the element, centered, or
below it where there is no room above; place it elsewhere with `AttachTo`,
its margins keeping it apart from the element:

```go
b := ui.Button(c, "").Label("Share").Description("Share with others")
b.Children(func() { ui.Icon(c, share) })
ui.TooltipBase(c, b, func(tip ui.Element) {
	tip.AttachTo(b, ui.AnchorRight, ui.AnchorLeft).Margin(0, 0, 0, 6)
	tip.Padding(4, 8).Radius(6).Background(ink).TextColor(paper)
	tip.Children(func() {
		ui.Text(c, "Share with others")
		ui.Text(c, "⌘S").TextColor(muted)
	})
})
```

It returns the tip, or nil while it does not show. The pointer goes
through the tip.

## Accessibility

Assistive technology reads the tip of `Tooltip` as the element's
description, after its name, as help text, unless `Description` gives
another. It sees the tip of `TooltipBase` as a tooltip, which it does not
read: give the element a `Description` telling what the tip does.
