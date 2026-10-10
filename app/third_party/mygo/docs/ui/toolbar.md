# Toolbar

`ui.Toolbar` creates a row of controls, as along the top of a window:
buttons, [toggles](toggle.md), toggle groups, [segmented
controls](segmented.md) and [menu buttons](menu-button.md), which take no
face of their own until hovered. `ui.Spacer` pushes those after it to the
end.

```go
ui.Toolbar(c, func() {
	ui.BackButton(c, app.router)
	ui.ForwardButton(c, app.router)
	ui.ToggleGroup(c, func() {
		ui.Toggle(c, &app.bold, "Bold")
		ui.Toggle(c, &app.italic, "Italic")
	})
	ui.Spacer(c)
	if ui.Button(c, "Share").Clicked() {
		app.share()
	}
}).Label("Format")
```

## Keyboard

A toolbar is one stop of Tab: Left and Right move the focus among its
controls, Home and End to the first and the last, and Tab goes on past it.

## Overflow

Controls that do not fit go, from the last, into a menu at the toolbar's
end, where choosing one clicks it, toggles a toggle, or opens a menu
button's menu.

## Accessibility

Assistive technology sees a toolbar, which `Label` names, holding its
controls.
