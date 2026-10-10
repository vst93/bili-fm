# Tabs

`ui.Tabs` creates a row of tabs showing labels, of which an `*int` is the
index of the one chosen. A click chooses a tab, as do the arrows, Home and
End while one has the keyboard focus, which follows the choice; `Changed`
reports a new choice. Build the page of the tab chosen below them:

```go
ui.Tabs(c, &app.tab, "General", "Appearance", "Advanced")
switch app.tab {
case 0:
	app.general(c)
case 1:
	app.appearance(c)
case 2:
	app.advanced(c)
}
```

The tabs are one stop of Tab, which goes to the tab chosen. For pages with a
history, which Back returns to, use a [router](navigation.md); for a choice
of view in a toolbar, a [segmented control](segmented.md).

## Without a look

`ui.TabsBase` is a tab list without a look: the `List` holding the tabs,
whose `Tab`s choose the index, with the arrows moving the choice and the
focus:

```go
tabs := ui.TabsBase(c, &app.tab, len(names))
tabs.List.Gap(4).Children(func() {
	for i, name := range names {
		tab := tabs.Tab(i).Padding(6, 12).Radius(6)
		if i == app.tab {
			tab.Background(t.Surface)
		}
		tab.Children(func() { ui.Text(c, name) })
	}
})
```

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a tab list of tabs named by their labels, the one
chosen selected.
