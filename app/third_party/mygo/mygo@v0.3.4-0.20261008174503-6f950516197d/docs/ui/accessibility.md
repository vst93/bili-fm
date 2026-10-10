# Accessibility

Assistive technology, such as VoiceOver on macOS, Orca on Linux and
Narrator or NVDA on Windows, reads a window's native UI as it reads other
apps: the widgets with their roles, names, values and states, the texts,
and the elements that take the focus, as the content changes. It acts on
them as the keyboard and the pointer would: it presses buttons, checks
boxes, moves sliders, edits text inputs and moves the focus.

## Names

Widgets describe themselves, and the text inside an element names it, as a
button's does. Elements without text need a `Label`, which also finds them
in [tests](testing.md):

```go
ui.Slider(c, &app.volume, 0, 100).Label("Volume")
ui.TextInput(c, &app.query).Placeholder("Search").Label("Search")
ui.Box(c).Size(24, 24).Draw(drawIcon).Label("Unread messages")
```

A [field](form.md) names its control with its label.

## Roles

Other elements get a role from what they do: one that is clickable and
takes the focus is a button, a scroll container a scroll area, and one with
a `Label`, or that takes the focus, a group. `Role` sets it for an element
drawn as a widget it is not built from, and `ui.RoleNone` leaves an element
out but not its children:

```go
toggle := ui.Box(c).Size(36, 20).Focusable().Role(ui.RoleSwitch).Label("Wi-Fi")
```

Besides the roles of MyGo's widgets, `ui.RoleHeading` is the title of a
section, ranked with `Level`, and `ui.RoleMenu`, `ui.RoleMenuBar`,
`ui.RoleMenuItem`, `ui.RoleMenuItemCheckBox` and `ui.RoleMenuItemRadio`
make menus drawn in the window, as a drop-down menu or a menu bar of your
own:

```go
ui.Text(c, "Appearance").Bold().Role(ui.RoleHeading).Level(2)
```

## States

The bases tell assistive technology what their widgets show. A widget of
your own tells it with these, as ARIA's states do on the web:

| Method | What it tells |
|---|---|
| `Checked(on)`, `Mixed()` | whether a check box, switch, radio button, toggle or menu item is on, or partly on |
| `Expanded(open)` | whether what the element opens shows, as a popup or a section |
| `Value(s)` | its value, as the choice a button opening a popup shows |
| `Range(lo, hi, value)`, `Step(step)` | the range and value of a slider, progress bar, meter or stepper, and how far the keys move it |
| `Level(n)` | the rank of a heading, or how deep an item of a tree is |
| `ActiveDescendant(e)` | the option that has the focus while the element keeps it, as the one the arrows are on in a menu or a list |

```go
all := ui.CheckboxBase(c, &app.all).Label("Select all")
if app.some {
	all.Mixed()
}

date := ui.ButtonBase(c).Role(ui.RolePopUpButton).Label("Due").Value(app.due.Format("Jan 2")).Expanded(app.picking)
```

## Descriptions and errors

`Description` tells more than the name, as help text read after it, as a
[tooltip](tooltip.md) does, and `Error` marks a value as invalid, for a
message read with it; a [field](form.md) gives its control the texts below
it.

## Lists and tables

A [list](list.md)'s rows are list items, and a [table](table.md)'s rows,
named by the text inside them, and each says which of all the rows it is,
"5 of 10,000", although the list builds only those in view. A list choosing
its rows gives assistive technology the focus on the row chosen: Up and
Down are read as they move the choice, and focusing a row chooses it. A
screen reader moving out of view asks the list to scroll there, which
builds the rows it reaches. `Label` names a list or a table.

## Announcements

News that does not move the focus, as a search done or a file saved,
reaches screen readers with `c.Announce(text)`, which they read once, after
what they are reading: VoiceOver's and Orca's announcements, and a live
region that Narrator and NVDA read on Windows. [Toasts](toast.md) announce
their text, and a [router](navigation.md) the title of a page shown while
the focus stays outside it.

```go
if ui.Button(c, "Search").Clicked() {
	app.results = search(app.query)
	c.Announce(fmt.Sprintf("%d results", len(app.results)))
}
```

MyGo describes frames only once assistive technology asked, so apps pay
nothing for it otherwise.

Each component's page says what assistive technology sees of it.
