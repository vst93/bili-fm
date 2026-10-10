# Link

`ui.Link` creates a text that opens a URL in the browser, or the app
registered for its scheme, when clicked, or with Enter while it has the
keyboard focus. It shows in the accent color, underlined while the pointer
is over it:

```go
ui.Link(c, "Privacy policy", "https://example.com/privacy")
```

## Paths in a router

In a page of a [router](navigation.md), a path without a scheme, as
`/notes/42`, `edit` or `?tab=info`, goes there in the router, as `Push`
does, relative to the page shown as links in a web page are:

```go
ui.Link(c, "Next note", fmt.Sprint(id+1))
```

## Within a sentence

Inside a [rich text](text.md#elements-within-a-sentence), a link is part of
the paragraph, wrapping with it; Tab reaches it all the same:

```go
ui.RichText(c).Children(func() {
	ui.Text(c, "Read ")
	ui.Link(c, "the guide", "https://example.com/guide")
	ui.Text(c, " to get started.")
})
```

Give it an empty label and `Children` to style parts of its text.

## Other actions

A link that does something in the app rather than opening a URL is a text
that is clicked: give `Link` an empty URL and ask `Clicked`, or use a
[button](button.md).

```go
if ui.Link(c, "Show details", "").Clicked() {
	app.details = true
}
```

## Accessibility

Assistive technology sees a link named by its text, which it follows as a
click does.
