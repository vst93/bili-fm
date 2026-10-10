# Select

`ui.Select` creates a drop-down choosing one of a list of strings into a
`*string`: a click on it, or Enter, Space, Up or Down while it has the
keyboard focus, opens its options below it, where the arrows, Home and End
move among them and Enter, Space or a click chooses one; Escape closes
them. `Changed` reports a new
choice.

```go
ui.Select(c, &app.plan, []string{"Free", "Pro", "Team"}).Label("Plan")
```

The popup shows above the select where there is no room below it, and gives
the focus back to the select as it closes.

For a long list that typing filters, use a [combobox](combobox.md); for a
few options shown at once, [radio buttons](radio.md).

## Without a look

`ui.SelectBase` is a select without a look, choosing a value of any
comparable type: a `Trigger` opening a `Popup` of `Item`s, which the arrows
highlight (`Highlighted`) and Enter chooses. `Highlight` moves the
highlight to an option, as a select of yours may as the user types its
first letters:

```go
sel := ui.SelectBase(c, &app.size)
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

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a pop-up button whose value is the choice, named
by its `Label`, and, while it is open, its options, following the one the
arrows are on.
