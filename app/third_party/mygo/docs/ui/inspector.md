# Inspector

The inspector shows the elements of a window of native UI in a panel
docked beside the content, as Chrome's developer tools show a page's: the
tree of elements as markup, the styles, box model and properties of the
one chosen, highlighted over the content, how long frames take, and
issues.

Windows whose developer tools are on open it with View → Toggle Developer
Tools, `F12`, or `Alt+Cmd+I` on macOS and `Ctrl+Shift+I` elsewhere, and
`Shift+Cmd+C` (`Ctrl+Shift+C`) opens it picking an element. Developer
tools are on in development builds, as the web inspector is, and
`WindowOptions.Page.DevTools` turns them on or off for a window:

```go
mygo.NewWindow(mygo.WindowOptions{
	Content: ui.View(app.view),
	Page:    mygo.PageOptions{DevTools: mygo.DevToolsDisabled},
})
```

The content gives the panel the right of the window: the view gets a
narrower window, as `c.Size()` tells.

Production builds of `mygo build` leave the inspector out, as their
developer tools are off: it takes some 250 KB of a binary. `mygo build
-debug` keeps it, as `MYGO_INSPECTOR=1 mygo build` does in a production
build, for a window with `DevToolsEnabled` to open it there. Apps built
with `go build` keep it unless `-tags mygo_noinspector` leaves it out. Drag the panel's left edge to resize
it. The panel follows the appearance, light or dark.

## Elements

The tree shows each element as a tag: the widget it is, as `<Button>` or
`<List>`, else `<Row>`, `<Column>`, `<Text>` and the like, with its `key`
and `label` as attributes, and the text of a text inside it. Elements near
the root show what is inside them; the arrow of an element opens and
closes it, and Alt-click opens or closes everything inside.

- Hovering a row highlights its element over the content, as Chrome does:
  its content in blue, padding in green, border in yellow and margins in
  orange, with a tooltip telling its size, colors, font and spacing, and
  what assistive technology sees: its name and role, and whether the
  keyboard can focus it. Rows of elements with a
  [transition](transitions.md) say so.
- Clicking a row chooses its element; the arrows move the choice, Left
  and Right close and open the element chosen, or go to its parent and
  into it. The breadcrumbs below the tree show the path to it, and choose
  an element of the path.
- The picker of the toolbar, then a click on the content, chooses the
  element under the pointer: the click goes to the inspector, not to the
  element. Escape stops picking.
- `Cmd+F` (`Ctrl+F`) finds elements by their tag, text, label or key.

Below the tree, the element chosen has three tabs:

- **Styles**: the styles it sets, as a CSS rule, with where the app built
  it (the file and line of the call that created it, or gave it its
  `Key`), then the text styles it inherits, by the element setting them.
  MyGo's layout is CSS's, so the properties are CSS's: `flex-direction`,
  `gap`, `padding`, `flex-grow`, `width`, `color`, …
- **Computed**: the box model, its margins, border, padding and content,
  with their sizes, and the values the element has as laid out: its size
  and place in the window, its font, its scroll offset.
- **Properties**: what it is (its type, ID, key, text), its state
  (focused, under the pointer, pressed, disabled, moving), and what
  assistive technology sees of it.

## Performance and issues

**Performance** shows how long the last frames took to build, lay out and
paint, as bars against the time between two refreshes of the window's
display (8.3 ms at 120 Hz, 16.7 ms at 60 Hz), with the last, the average
and the slowest. Set `MYGO_FRAME_STATS` to log slow frames as well
(see [Rendering](rendering.md)).

**Issues** lists mistakes found while building, which the toolbar counts:
two elements given the same `Key` under one parent, say, share one state,
so that a click, the focus or scrolling meant for one goes to the other.
Apps log each one once; a [Tester](testing.md) panics where the second
key was given, so that the test fails there.
