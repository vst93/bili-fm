# Autocomplete

`ui.Autocomplete` creates a text input editing a `*string` that suggests,
below it, those of a list of strings containing what was typed, those
starting with it first: Up and Down move among them and Enter takes one, as
does a click, while Enter on none is `Submitted`. `Changed` reports a
change, typed or taken.

```go
ui.Autocomplete(c, &app.city, cities).Label("City")
```

Unlike a [combobox](combobox.md), whose value is one of its options, an
autocomplete keeps what the user typed.

## Accessibility

Assistive technology sees a combo box named by its `Label`, editable as a
text field; the suggestion the arrows are on has the focus for it.
