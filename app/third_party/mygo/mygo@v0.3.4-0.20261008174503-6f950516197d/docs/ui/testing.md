# Testing

`ui.NewTester` runs a view without a window, as fast as a unit test: it
renders frames in memory, finds elements by their text, and clicks, types,
scrolls and presses keys.

```go
func TestCounter(t *testing.T) {
	app := &counter{}
	tt := ui.NewTester(app.view, 320, 240)
	if err := tt.Click("Increment"); err != nil {
		t.Fatal(err)
	}
	if app.n != 1 || !tt.HasText("1") {
		t.Errorf("count %d, texts %q", app.n, tt.Texts())
	}
}
```

Elements are found by their text, or by their `Label` for those without,
as an icon button: `tt.Click("Close")`, `tt.Find("Volume")`,
`tt.Focused("Name")`.

Two elements given the same `Key` under one parent share one state, so
that a click or the focus meant for one goes to the other: a tester
panics where the second key was given, as apps only log it.

## What a tester does

- **Pointer.** `tt.Click`, `tt.ClickAt`, `tt.ClickWith(ui.Shift, "Row 3")`,
  `tt.Press`, `tt.Move` and `tt.Release`, and `tt.Scroll`.
- **Keyboard.** `tt.Key(ui.Cmd, ui.KeyS)` presses a key, `tt.TypeKey` a key
  with the text it types, and `tt.Type` types text into the focused
  element. `tt.Compose` shows the composition of an input method,
  `tt.Command` performs an edit command of the menus, such as `"copy"`, and
  `tt.TextCaret` returns where input methods would compose.
- **Menus.** `tt.RightClick` opens a context menu, which `tt.Menu` lists and
  `tt.ChooseMenuItem("Move to", "Archive")` chooses from.
- **The window.** `tt.SetSize`, `tt.SetScale`, `tt.SetDark` and
  `tt.SetPreferences` change the window and the desktop's settings;
  `tt.SetFocused` takes the keyboard from the window and gives it back.
- **What it shows.** `tt.Texts`, `tt.HasText`, `tt.Find` (the box of a
  text), `tt.Focused`, `tt.Cursor`, `tt.Clipboard`, `tt.OpenedURLs` and
  `tt.Announcements`, what the view asked screen readers to read out;
  `tt.FailOpenURL` makes the links it opens fail, as those no app opens.
- **Frames.** `tt.Frame` draws a frame, for what changed outside the view,
  and `tt.Image()` is the last frame, for snapshots; `ui.Render` draws a
  view once at a given scale.
