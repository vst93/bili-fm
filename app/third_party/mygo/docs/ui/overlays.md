# Overlays

Overlays show above the window's content: [dialogs](dialog.md) over the
dimmed window, [alert dialogs](alert-dialog.md), [popovers](popover.md)
below an element, the popups of [selects](select.md) and
[comboboxes](combobox.md), [tooltips](tooltip.md) and [toasts](toast.md).
They share how they treat the keyboard and what is behind them.

```go
more := ui.Button(c, "More ▾")
if more.Clicked() {
	app.menu = !app.menu
}
ui.Popover(c, more, &app.menu, func() {
	if ui.Button(c, "Rename").Clicked() {
		app.menu, app.renaming = false, true
	}
})
ui.Modal(c, &app.renaming, func() {
	ui.Text(c, "Rename").Bold()
	if ui.TextInput(c, &app.name).AutoFocus().Submitted() {
		app.renaming = false
	}
})
```

## The keyboard

A dialog keeps the keyboard: it takes the focus as it opens, on its first
element that takes it unless one in it asked for it (`AutoFocus`), Tab goes
round its elements, and the window's shortcuts built outside it wait, as
what is behind it is inert, which screen readers do not see either. A
popover's elements follow its anchor as Tab moves.

Escape closes the overlay on top, a select's popup before the dialog it is
in, unless the focused element takes it, as a terminal does; and an
overlay that closes with the focus in it gives the focus back to the
element that had it as it opened, its button say.

A press outside a popover closes it and goes on to what is under the
pointer, as with the web's popovers: a click on another button closes the
popover and presses the button. A select's popup takes the press, as the
system's pop-up menus do, and a dialog's backdrop closes it.

## Your own overlays

`ui.Overlay` builds elements above everything else, placed with `Absolute`
in DIPs of the window:

```go
ui.Overlay(c, func() {
	ui.Text(c, "Offline").Absolute().Top(12).Right(12).
		Padding(4, 10).Radius(999).Background(t.Danger).TextColor(t.AccentText)
})
```

`PopoverBase` and `DialogBase` are a popover and a dialog without a look,
for overlays of your own design: see [custom widgets](custom-widgets.md).
Overlays that neither fits, as a drawer, a hover card or a menu bar's
menus, are built from what those are made of:

- `AttachTo(target, at, self)` places an element beside another, wherever
  that one is: its point `self` on the point `at` of the target, as
  `AttachTo(button, ui.AnchorBottomLeft, ui.AnchorTopLeft)` below a button.
  Where it would leave the window, it goes to the other side, or the other
  way along the target, then moves into the window. Its margins keep it
  apart from the target. It is the target's popover: its elements follow
  the target as Tab moves.
- `PressedOutside` reports a press outside an element, its target and the
  popovers of what is inside it, as a popover closes for.
- `OverlayShortcut` takes the keys the focused element and those around it
  leave, wherever the focus is, the overlay built last first, as Escape
  closing the one on top.
- `Modal` makes an element a dialog's backdrop: the focus moves into it
  and stays there, the window's shortcuts outside it wait, and screen
  readers see only it and what is above it.

Each element built at the top of `ui.Overlay` gives the focus back to the
element that had it as it came, when it goes with the focus in it. A
drawer on the right, closing with Escape or a click on its backdrop:

```go
if app.drawer {
	ui.Overlay(c, func() {
		back := ui.Row(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).Justify(ui.End).
			Background(ui.RGBA(0, 0, 0, 0.3)).Modal()
		back.Children(func() {
			panel := ui.Column(c).Width(320).FillHeight().Padding(16).Background(t.Background).
				Role(ui.RoleDialog).Label("Filters").Children(app.filters)
			if panel.PressedOutside() || back.OverlayShortcut(0, ui.KeyEscape) {
				app.drawer = false
			}
		})
	})
}
```

Native [dialogs](../native.md#dialogs) work too: call them from a goroutine,
so that the view does not wait for them.
