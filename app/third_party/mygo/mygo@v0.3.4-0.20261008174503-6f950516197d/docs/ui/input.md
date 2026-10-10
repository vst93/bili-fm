# Input

Elements report what the user did to them since the last frame, as you
build them: ask, and handle it where the element is built.

A `ui.Element` value is valid for one build pass. Its zero value is absent,
and queries on that value return false or zero. Use `Valid` to check optional
controls. Stale use is diagnosed in development and `Tester`; production
methods return empty results or ignore the operation.

Store a `ui.Handle` for focus or shortcuts across builds. Each window has
its own binding for the handle; use that window's Context when querying it.

```go
card := ui.Column(c).Padding(12).Radius(8).Focusable()
if card.Hovered() {
	card.Background(t.SurfaceHover)
}
if card.DoubleClicked() {
	app.open(item)
}
```

## The pointer

`Hovered`, `Pressed`, `Clicked`, `DoubleClicked`, `RightClicked`, `Dragged`
(how far the pointer moved since the last frame while pressing the
element) and `PointerPosition`. `ClickModifiers` returns the modifier keys
held for the last click, as Shift for a Shift-click. `c.Modifiers()` returns the modifier
keys held now, as one goes down or up on its own too, and the view draws
again as they change: a list showing each row's Cmd+1–9 while Cmd is held
reads it. `PassThrough` lets the
pointer through to what is below.

Elements that take the pointer give it to the innermost under it: a button
in a clickable row takes its own clicks.

`Pressed` stays true while the pointer is held inside the element. Observing
the start of a press rebuilds the view before painting, so a row chosen by
`if row.Pressed() || row.Clicked()` shows its new selection and focus colors
in that frame. Guard the choice against selecting the same item again.
`Clicked` reports activation on release inside the element, or through the
keyboard or assistive technology; buttons use that activation.

While the pointer presses an element, the elements it was over as the
press began, as the row around a button, stay `Hovered` as long as it is
over them, as in CSS, and the others hover no more: dragging over other
elements does not light them up. A button that shows while its row is
hovered stays as it is pressed, to take its click:

```go
row := ui.Row(c).Padding(8)
hovered := row.Hovered()
row.Children(func() {
	ui.Text(c, item.Title).Grow(1)
	if hovered && ui.Button(c, "Remove").Clicked() {
		removed = item.ID
	}
})
```

## The keyboard focus

`Focusable` elements take the focus when clicked, and Tab and Shift+Tab
move it between them, with a focus ring when it moves by keyboard.
`AutoFocus` gives an element the focus when it appears, such as the first
field of a dialog, and `Focus` keeps it there while you call it; `Focused`,
`FocusVisible` and `FocusWithin` report it. Enter and Space press a focused
button or link, and Space toggles a focused check box, switch, toggle or
radio button.

## Persistent focus

Bind an app-owned handle to a control on each build:

```go
ui.TextInput(c.Key("search"), &app.query).Bind(&app.search)
```

Declare `search ui.Handle` in your app state. `app.search.Focus()` requests
focus and waits while the control is hidden. `CancelFocus()` cancels the
request. `Focused(c)` and `FocusWithin(c)` read the control's identity before
or after it is built. `c.Resolve(app.search)` returns only this pass's element.
`app.search.Bounds(c)` reads its box from the committed frame, including
between builds in an input callback. Closing a window cancels its focus
request.

A handle can bind in several windows. Queries take that window's Context;
use `Focus(c)` or `CancelFocus(c)` to select a window explicitly. The
no-argument focus form requires at most one open binding.

## Focus bound to data

```go
type pane int
const (none pane = iota; files; diff)

ui.Column(c.Key("files")).FocusBind(&app.pane, files).Children(func() { app.filesView(c) })
ui.Column(c.Key("diff")).FocusBind(&app.pane, diff).Children(func() { app.diffView(c) })

// In an action:
app.pane = diff

// Actual focus in this window:
focused := ui.FocusedValue(c, &app.pane)
```

`FocusBind` takes a pointer to a comparable field and a matching value.
Each value names one control in the window; reserve zero for no focus.
The field holds desired focus. A request waits if its control is hidden,
so use `FocusedValue` to read actual focus while it waits. Assigning zero
clears focus. User focus changes update the field when no request is pending.
Use separate fields for separate windows.

## Focus groups

The controls of a toolbar, a radio group, a segmented control or a tab list
are one stop of Tab, as they are natively: Tab moves the focus into the
group, to the control that had it last (or the radio button or tab chosen),
and on out of it, while the arrows move it among them, Home and End to the
first and last. `FocusGroup` does it for a container of your own, with the
arrows `ui.Horizontal`, `ui.Vertical` or both; a slider or a text input
inside keeps the arrows it takes.

```go
ui.Row(c).Gap(4).FocusGroup(ui.Horizontal).Children(func() {
	for _, tool := range tools {
		if ui.Button(c, tool.Name).Clicked() {
			app.tool = tool
		}
	}
})
```

## Shortcuts

`c.Shortcut(ui.Cmd, ui.KeyS)` reports a key pressed with exactly those
modifiers anywhere in the window, and `Element.Shortcut` only while the
element or one inside it has the focus, which comes first. A focused button
or link keeps Enter and Space, and a toggle Space, so
`c.Shortcut(0, ui.KeyEnter)` presses a dialog's default button wherever
else the focus is.

```go
if c.Shortcut(ui.Cmd, ui.KeyS) {
	app.save()
}
```

`ui.Cmd` is Command on macOS and Ctrl elsewhere. `ui.KeyBack` and
`ui.KeyForward` are the back and forward buttons of a mouse and the keys of
keyboards that have them, which a [router](navigation.md) takes. Shortcuts
of [menus](../menus.md) still work, and the Edit menu's roles (cut, copy,
paste, select all, undo, redo) act on the focused text input.

A focused text input takes the editing keys of the platform first: on
macOS, Option and Command with the arrows and Backspace, and Control with
A, E, B, F, N, P, D, H and K, as in other Mac apps; elsewhere, it leaves Alt
and the arrows, which go back and forward, and the function keys.

A shortcut action can be declared before binding its control:

```go
app.filesView.OnShortcut(c, ui.Cmd, ui.KeyK, app.openSelected)
ui.List(c.Key("files"), &app.list, len(app.files)).Bind(&app.filesView).
    Rows(func(row ui.ListRow) {
        ui.Text(row.Context, app.files[row.Index].Name)
    })
```

`Handle.OnShortcut` runs after construction only when the window built an
enabled control for that handle. Hidden controls take no command.
`c.OnShortcut` declares an action in the current parent scope; the root
Context handles keys left by the focused control and active overlays.
Use `Element.OnShortcut` for an action inside an element's focus subtree.

## Input methods

Text inputs take text composed with input methods, which see the text
around the caret: macOS's press and hold replaces the letter it accents,
Japanese input methods convert typed text again, and others predict from
what comes before.

## Dropped files

`DroppedFiles` returns the paths of the files dropped on the element from
Finder, Explorer or a file manager, and `FileDragOver` reports files dragged
over it, to show it would take them. The window takes files only over such
elements, or anywhere when it has `OnFileDrop` listeners, which get the
files no element takes, with where they were dropped:

```go
zone := ui.Column(c).Size(260, 80).Border(1, t.Border)
if files := zone.DroppedFiles(); files != nil {
	app.files = files
}
if zone.FileDragOver() {
	zone.Border(2, t.Accent)
}
```

Values dragged within or between windows, and serialized data exchanged
with other applications, are [drag and drop](drag-and-drop.md).

## Every key, as it comes

Widgets that take every key themselves, as the
[terminal](../plugins/terminal.md) does, get their input as it comes with
`HandleInput`, before the next frame: keys pressed and released, text typed
and composed while they have the focus, the edit commands of the menus,
and the pointer pressed on them, moving over them or scrolling over them.
The function reports whether it took the event; one it leaves goes on to
shortcuts, Tab, context menus and scroll containers as usual, and keys that
the window or an element around the focus handles with `Shortcut` go there
first. `TextCaret` turns on the system's input methods for such an element
while it has the focus, composing at the caret it gives. `c.ReadClipboard`,
`c.WriteClipboard` and `c.OpenURL` copy, paste and open links for them;
`c.OpenURLThen` opens a link too, and its function gets what came of it a
moment later, as the system opens it, with an error when no app could,
and the view builds a frame anew:

```go
ui.Box(c).Fill().Focusable().HandleInput(func(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		return app.key(ev.Mods, ev.Key)
	case ui.InputText:
		app.insert(ev.Text)
		return true
	}
	return false
}).TextCaret(app.caretRect())

c.OpenURLThen(url, func(err error) {
	if err != nil {
		c.Toast("No app opens " + url)
	}
})
```

## See also

- [Context menus](context-menu.md) and [menu buttons](menu-button.md).
- [Tooltips](tooltip.md).
- Custom title bars: [windows with native UI](windows.md).
