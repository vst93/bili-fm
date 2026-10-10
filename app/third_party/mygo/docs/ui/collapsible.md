# Collapsible

`ui.Collapsible` creates a disclosure, as SwiftUI's `DisclosureGroup`: a
label beside an arrow, which a click opens and closes, as do Enter and
Space, showing below it what its function builds while a `*bool` is true.
The arrow turns and the content grows into view as it opens. `Changed`
reports a click.

```go
ui.Collapsible(c, "Advanced", &app.advanced, func() {
	ui.Checkbox(c, &app.verbose, "Verbose logging")
	ui.Checkbox(c, &app.experimental, "Experimental features")
})
```

With less motion asked of the desktop, the content shows at once.

For several sections in a box, use an [accordion](accordion.md).

## Without a look

`ui.CollapsibleBase` is a collapsible without a look: a `Trigger` that
shows and hides what `Panel` builds, growing and shrinking it with
`Progress`, which goes from 0 closed to 1 open, for a look and a motion of
your own:

```go
p := ui.CollapsibleBase(c, &app.open)
p.Trigger.Gap(6).Children(func() {
	ui.Icon(c, chevron).Rotate(90 * p.Progress())
	ui.Text(c, "Details")
})
p.Panel(func() {
	ui.Text(c, app.details)
})
```

See [custom widgets](custom-widgets.md).

## Accessibility

Assistive technology sees a disclosure named by its label, expanded or
collapsed, which it opens and closes as a click does.
