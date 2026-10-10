# Sidebar

`ui.Sidebar` is a source list, as the sidebars of Finder and Mail: items
with icons, in sections whose titles hide and show them, choosing the ID of
one into a `*string`. It scrolls when they do not fit. Give it a width:

```go
ui.Sidebar(c, &app.mailbox, func() {
	ui.SidebarSection(c, "Mailboxes", &app.mailboxesShown, func() {
		ui.SidebarItem(c, "inbox", inboxIcon, "Inbox").Children(func() {
			ui.Badge(c, fmt.Sprint(app.unread))
		})
		ui.SidebarItem(c, "sent", sentIcon, "Sent")
	})
	ui.SidebarSection(c, "Tags", nil, func() {
		ui.SidebarItem(c, "work", nil, "Work")
	})
}).Width(220)
```

- `ui.SidebarItem` chooses its ID, showing its label after its icon, nil
  for none; add to it with `Children`, as a [badge](badge.md).
- `ui.SidebarSection` holds the items its function builds under a title
  while its `*bool` is true: a click on the title shows and hides them, as
  does its arrow, which shows as the pointer rests on the title. A nil
  `*bool` keeps the section open.

## Choosing

A click chooses an item; while the sidebar has the focus, Up, Down, Home,
End and the first letters of an item choose among them, and Tab leaves it,
as it is one stop. `Changed` reports a new choice. The item chosen shows in
the accent color while the sidebar has the focus, in gray otherwise, as
AppKit's.

With a [router](navigation.md), a `*ui.Router` in your state, the items'
IDs are paths, and a new choice pushes its page:

```go
page := app.router.Path()
if ui.Sidebar(c, &page, func() {
	ui.SidebarItem(c, "/notes", nil, "Notes")
	ui.SidebarItem(c, "/files", nil, "Files")
}).Width(220).Changed() {
	app.router.Push(page)
}
```

## Accessibility

Assistive technology sees a tree: the sections' titles are items that open
and close, holding their items, and the item chosen has the focus while
the sidebar does.
