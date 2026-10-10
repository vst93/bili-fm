# Text input

`ui.TextInput` creates a single-line text input editing a `*string`, and
`ui.TextArea` a multi-line one. They change the string as the user types:
read it, or ask `Changed` for work that follows, and `Submitted` for Enter
in a single-line input.

```go
if ui.TextInput(c, &app.query).Placeholder("Search").Label("Search").Changed() {
	app.results = search(app.query)
}

if ui.TextInput(c, &app.name).AutoFocus().Submitted() {
	app.rename()
}

ui.TextArea(c, &app.notes).Height(160)
```

- `Placeholder` shows a text while the input is empty, on one line in a
  `TextInput`, wrapped in a text area.
- `Password` hides what it holds, and keeps it from the clipboard and input
  methods, in the frames that call it: an eye button that shows the text
  stops calling it. Only a `TextInput` takes it: as on every platform, a text
  area has no password mode.
- `AutoFocus` gives it the keyboard focus as it appears, as the first field
  of a dialog.
- `ReadOnly(true)` shows the text without letting the user change it: it
  still takes the focus, without a caret, and its text can be selected and
  copied.
- `Disabled(true)` grays it out.

A `TextInput` too narrow for its text keeps the caret in view while it has
the focus, and shows the start of its text without it.

`TextRanges` colors runs of the text, and sets their weight, in the frames
that call it, as a message field shows the mentions in what is typed:

```go
in := ui.TextAreaBase(c, &app.draft).Lines(1, 8)
for _, m := range mentions(app.draft) {
	in.TextRanges(ui.TextRange{Start: m.start, End: m.end, Color: m.color, Weight: 600})
}
```

The ranges are runes of the text, found in it each frame, and do not
overlap; a password shows none.

A text area is at least a few lines high and grows with its text; given a
height, it scrolls within it, with the wheel and a scroll bar, and keeps
the caret in view as it moves. `Lines(min, max)` makes it as high as its
text wraps at its width, from `min` lines up to `max`, past which it
scrolls, as a message field grows with what is typed:

```go
ui.TextArea(c, &app.draft).Lines(1, 8)
``` It lays out only the paragraphs in view and
keeps their layouts until they change, so that it holds texts of hundreds
of thousands of lines, as a log or a source file, and stays as quick to
type in.

## Editing

Text inputs edit as the platform's text fields do: selection with the
pointer (a double click selects a word, a triple click a line), with Shift
and the arrows, and by words and lines with the platform's keys; undo and
redo; cut, copy and paste, also from the Edit menu's roles; and a context
menu of the editing commands. They take text composed with input methods,
which see the text around the caret, so that press and hold, Japanese
conversion and predictions work as in other apps. The keys typed while an
input method composes are its own, as Enter choosing a candidate or Escape
giving the composition up: they submit nothing, press no shortcut and
close no dialog. `Composing` reports whether it composes, its text not yet
in the string.

The input keeps the text being edited, its selection and its undo history
from frame to frame; setting the string from elsewhere replaces the text.

## The caret

`TextSelection` returns the selection as offsets in runes into the text,
the caret where they are equal, and `SetTextSelection` moves it, as an app
completing the word being typed puts the caret after it:

```go
input := ui.TextArea(c, &app.draft)
if start, _ := input.TextSelection(); app.completed != "" {
	app.draft, start = complete(app.draft, start, app.completed)
	input.SetTextSelection(start, start)
}
```

## Errors

In a [field](form.md), `Error` marks the value invalid: the input draws its
border in the theme's `Danger` color, and assistive technology reads the
message with it.

```go
ui.Field(c, "Email", func() {
	ui.TextInput(c, &app.email)
}).Error(app.emailError)
```

## Without a look

`ui.TextInputBase` and `ui.TextAreaBase` are text inputs without padding,
background, border or corners, for inputs of your own design: see
[custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a text field, or a text area, named by its
`Label` or its field, whose value is its text, with the caret and the
selection; it edits it as typing does, unless it is read-only. A password
field hides its value.
