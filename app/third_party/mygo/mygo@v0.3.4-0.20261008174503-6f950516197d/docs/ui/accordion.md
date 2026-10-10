# Accordion

`ui.Accordion` creates sections, one above the other in a bordered box,
that `ui.AccordionItem` builds: a header showing a title, which a click
opens and closes, as do Enter and Space, and below it what its function
builds while its `*bool` is true. Each opens and closes on its own; Up and
Down move the focus between their headers, as do Home and End. An item's
`Changed` reports a click.

```go
ui.Accordion(c, func() {
	ui.AccordionItem(c, "General", &app.general, func() { app.generalSettings(c) })
	ui.AccordionItem(c, "Privacy", &app.privacy, func() { app.privacySettings(c) })
	ui.AccordionItem(c, "Advanced", &app.advanced, func() { app.advancedSettings(c) })
})
```

For one section open at a time, close the others as one opens:

```go
for i, s := range sections {
	open := app.section == i
	ui.AccordionItem(c, s.Title, &open, s.Build).OnChange(func() {
		app.section = -1
		if open {
			app.section = i
		}
	})
}
```

Tab goes from a header into the content of its section while it is open.

## Accessibility

Assistive technology sees each header as a disclosure, expanded or
collapsed, before the content of its section.
