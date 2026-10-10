# Badge

`ui.Badge` creates a short text in a pill, as a count of unread messages
beside a [sidebar](sidebar.md)'s item:

```go
ui.SidebarItem(c, "inbox", inboxIcon, "Inbox").Children(func() {
	ui.Badge(c, fmt.Sprint(app.unread))
})
```

It returns its element: `Background` and `TextColor` color it, as a badge
in the accent color for what needs attention.

```go
ui.Badge(c, "New").Background(t.Accent).TextColor(t.AccentText)
```

## Accessibility

Assistive technology reads its text with the item it is in.
