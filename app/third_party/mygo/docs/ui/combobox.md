# Combobox

`ui.Combobox` creates a combobox choosing a `*string` among options: a
field showing the choice, whose text filters the options below it as the
user types, those starting with it first, while a click or Down shows them
all. Up and Down move among them and Enter chooses, as does a click; Escape
closes them, and the text goes back to the choice when the field loses the
focus. `Changed` reports a new choice.

```go
ui.Combobox(c, &app.font, fonts).Label("Font")
```

The field keeps the keyboard focus while the user types and picks.

For a text that may be anything, with suggestions, use
[autocomplete](autocomplete.md); for a short list without typing, a
[select](select.md).

## Without a look

`ui.ComboboxBase` is a combobox without a look: an `Input` whose text
filters the `Item`s of a `Popup` below it, which the arrows highlight and
Enter or a click chooses (`Chosen`): see [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a combo box named by its `Label`, editable as a
text field, which expands; the option the arrows are on has the focus for
it, and reads as which of how many it is.
