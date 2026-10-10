# Toast

`c.Toast` shows a message near the bottom of the window for a few seconds,
as the outcome of what the user just did:

```go
if ui.Button(c, "Save").Clicked() {
	app.save()
	c.Toast("Saved")
}
```

`c.ToastAction` shows one with a button, as Undo after deleting, for longer:
the button runs the action, on the main thread as the view does, and closes
the toast; Tab reaches it.

```go
app.trash(note)
c.ToastAction("Note deleted", "Undo", func() { app.restore(note) })
```

A message already showing shows anew; others stack above it, the newest
three showing, while older ones wait hidden, their time running. The time
of the toasts stops while the pointer rests on them, the keyboard focus is
in them, or the window is in the background. Escape closes the toast the
focus is in. Toasts fade in and out.

A toast takes the theme's `Inverse` and `InverseText` colors, its text and
background turned over unless the theme sets them (see
[themes](styling.md#themes)).

## Adding and closing toasts

`c.AddToast` shows a `ui.Toast`, as Base UI's toast manager adds one, and
returns its ID; `c.Toast` and `c.ToastAction` are `AddToast` with the
message as the ID and the title:

```go
id := c.AddToast(ui.Toast{
	Title:       "Uploading",
	Description: "report.pdf",
	Timeout:     -1, // until closed
})
// ...
c.CloseToast(id)
```

| Field | What it does |
|---|---|
| `ID` | names the toast: adding one with the ID of a toast showing changes that toast, which shows anew; one is made up when empty |
| `Title`, `Description` | what it says, which screen readers read out as it shows |
| `Type` | what kind of toast it is, as `"error"`, for the look to tell |
| `Timeout` | how long it shows: 4 seconds when zero, until closed when negative |
| `Action`, `OnAction` | a button that runs `OnAction` and closes the toast |
| `OnClose` | runs as it closes: its time over, or the user, its action or `CloseToast` closed it |
| `Data` | what else the look needs |

`c.CloseToast("")` closes every toast.

## Without a look

`ToastViewportBase` builds the window's toasts with a look of your own, in
place of the theme's, as Base UI's Toast parts do: its `fn` styles the
viewport, a column covering the window that the pointer goes through, which
holds the toasts at its bottom (`Justify` and `AlignItems` move them), and
builds each toast showing with `ToastBase`, whose `Root` assistive
technology sees as a status, and whose `ActionButton` and `CloseButton` run
the action and close it. Call it once in the view, wherever. Toasts in the
bottom right corner:

```go
ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
	viewport.Padding(16).AlignItems(ui.End).Gap(8)
	for _, t := range toasts {
		toast := ui.ToastBase(c, t)
		toast.Root.Row().Gap(12).Padding(10, 14).Radius(8).Background(surface).Border(1, border)
		toast.Root.Transition(ui.ElementTransition{Enter: &ui.Motion{Y: 8}, Exit: &ui.Motion{}})
		toast.Root.Children(func() {
			if t.Type == "error" {
				ui.Icon(c, alert).TextColor(danger)
			}
			ui.Text(c, t.Title).Grow(1)
			if t.Action != "" {
				toast.ActionButton().Children(func() { ui.Text(c, t.Action) })
			}
			toast.CloseButton().Label("Close").Children(func() { ui.Icon(c, x) })
		})
	}
})
```

`toasts` are those showing, the newest three, oldest first. `toast.Left()`
is how long one shows still, to count down, 0 for one showing until closed.
The time and the pausing are the same as with the theme's look. The
viewport stays while no toast shows, for the last one to go with its exit
transition.

## Accessibility

Screen readers read a toast's title and description as it shows (see
[announcements](accessibility.md#announcements)); assistive technology sees
it as a status, which holds its buttons.
