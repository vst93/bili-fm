# Dialog

`ui.Modal` shows a dialog its function builds over the dimmed window while
a `*bool` is true; clicking outside it or pressing Escape sets it to false.

```go
if ui.Button(c, "Rename…").Clicked() {
	app.renaming = true
}
ui.Modal(c, &app.renaming, func() {
	ui.Text(c, "Rename").Bold()
	if ui.TextInput(c, &app.name).AutoFocus().Submitted() {
		app.renaming = false
	}
	ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
		if ui.Button(c, "Cancel").Clicked() {
			app.renaming = false
		}
		if ui.PrimaryButton(c, "Rename").Clicked() {
			app.rename()
			app.renaming = false
		}
	})
})
```

## The keyboard

A dialog keeps the keyboard: it takes the focus as it opens, on its first
element that takes it unless one in it asked for it (`AutoFocus`), Tab goes
round its elements, and the window's shortcuts built outside it wait, as
what is behind it is inert. As it closes, it gives the focus back to the
element that had it, the button that opened it say. See
[overlays](overlays.md).

To ask about something important, with buttons and no closing by a click
outside, use an [alert dialog](alert-dialog.md).

## Without a look

`ui.DialogBase` is a dialog without a look: its function styles the
backdrop covering the window, which centers the panel, and the panel, and
builds the panel's content:

```go
ui.DialogBase(c, &app.open, func(backdrop, panel ui.Element) {
	backdrop.Background(ui.RGBA(0, 0, 0, 0.5))
	panel.Width(360).Padding(24).Radius(16).Background(t.Background)
	ui.Text(c, "Welcome").FontSize(20).Bold()
})
```

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a dialog, and nothing behind it while it shows.
