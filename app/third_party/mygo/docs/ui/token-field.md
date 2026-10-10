# Token field

`ui.TokenField` creates a field of tokens, as of tags or the recipients of
a mail, editing a `*[]string`: the tokens show as chips, each with a button
taking it out, before an input where Enter or a comma adds what was typed,
and Backspace in it while empty takes out the last. The suggestions
containing what was typed show below it, those not already tokens: Up and
Down move among them, and Enter or a click adds one. `Changed` reports a
change.

```go
ui.TokenField(c, &app.tags, allTags).Label("Tags")
```

Chips wrap onto more lines as they fill the field.

## Accessibility

Assistive technology sees the field named by its `Label`, its input as a
text field, and each chip's button, named "Remove" and its token.
