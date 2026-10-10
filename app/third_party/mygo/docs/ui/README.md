# Native UI

A window can show a user interface that MyGo draws itself instead of a web
page. You write it in Go with package `ui`: there is no HTML, no JavaScript
and no frontend build, and the window starts no webview, so it opens at
once and uses little memory. MyGo draws it on the GPU, with Metal on macOS,
Direct3D 11 on Windows and OpenGL on Linux.

```go
package main

import (
	"fmt"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// counter is the app's state.
type counter struct{ n int }

// view builds the window's interface from the state, for every frame.
func (app *counter) view(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, fmt.Sprint(app.n)).FontSize(40).Bold()
		if ui.PrimaryButton(c, "Increment").Clicked() {
			app.n++
		}
	})
}

func main() {
	app := &counter{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "Counter",
			Width:   320,
			Height:  240,
			Content: ui.View(app.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
```

**Reading the examples.** The examples in these guides are parts of a view
such as `counter.view`: `c` is the view's `*ui.Context`, and `app` its
receiver, the value of your own type that holds the state. A field such as
`app.volume` or a method such as `app.save()` is one you declare on that
type, as `counter` declares `n`; [Views](views.md) says more.

`mygo init -template native my-app` starts a native UI project (see
[the CLI](../cli.md#mygo-init)). The [counter example](../../examples/counter-native)
includes a view test, and the [gallery](../../examples/gallery) shows the
controls and layouts.


One app can have windows of both kinds. Native UI suits tools, settings,
inspectors and utilities, and apps that must start instantly; a web page
suits rich documents, existing web code and anything that needs what only
a browser has. Screen readers and other assistive technology read native
UI as they read other apps (see [Accessibility](accessibility.md)).

## Guides

- [Views](views.md): build lifetimes, event queries and callbacks, keys,
  and changing the state from other goroutines.
- [Layout](layout.md): rows and columns, sizes, spacing, alignment and
  positioning.
- [Text](text.md): text and its style, rich text, links within sentences,
  and fonts.
- [Styling and themes](styling.md): backgrounds, borders, shadows and
  gradients, and the theme that follows the desktop.
- [Input](input.md): the pointer, the keyboard focus, shortcuts, input
  methods, dropped files, and widgets that take every key.
- [Drag and drop](drag-and-drop.md): values dragged within the window, and
  rows reordered by dragging.
- [Pages and navigation](navigation.md): a router of pages with a history,
  layouts, transitions, and going back and forward.
- [Overlays](overlays.md): what dialogs, popovers and the layer above the
  window share: the focus, Escape, and what is behind them.
- [Accessibility](accessibility.md): what screen readers see, naming
  elements, roles, descriptions and announcements.
- [Drawing and animation](drawing.md): painting on elements, shaping text,
  and values that ease to their targets.
- [Transitions](transitions.md): elements that move, resize and recolor
  smoothly as the layout changes, and come and go.
- [Custom widgets](custom-widgets.md): the widgets without a look that every
  widget is built on, and widgets of your own.
- [Windows with native UI](windows.md): window options, title bars drawn by
  the view, and vibrancy.
- [Testing](testing.md): running a view without a window, as fast as a unit
  test.
- [Inspector](inspector.md): the elements of a window, their boxes and
  styles, and the time frames take, beside the content.
- [Rendering](rendering.md): how MyGo draws, on the GPU or the CPU.
- [Migration](migration.md): element values, persistent handles, focus
  bindings and source migration.
- [Performance](performance.md): measured frame costs and retained memory.

## Actions

- [Button](button.md): a push button, plain or in the accent color.
- [Link](link.md): text that opens a URL, or a page of a router.
- [Menu button](menu-button.md): a button opening a menu below it.
- [Context menu](context-menu.md): a menu of the system's for any element,
  on a right click.
- [Toggle](toggle.md): a button that stays pressed, and toggles joined in a
  group.
- [Segmented control](segmented.md): segments choosing one of a few views.
- [Toolbar](toolbar.md): a row of controls along a window, which overflow
  into a menu.

## Choices

- [Checkbox](checkbox.md): a check box, and a check box over a group of
  others.
- [Switch](switch.md): a switch that turns a setting on and off.
- [Radio](radio.md): radio buttons choosing one value, in a group.
- [Select](select.md): a drop-down choosing one of a list.
- [Slider](slider.md): a slider setting a value in a range, with or without
  steps.
- [Range slider](range-slider.md): two knobs setting a low and a high value.
- [Stepper](stepper.md): arrows stepping a value up and down.
- [Rating](rating.md): stars setting a rating.

## Text input

- [Text input](text-input.md): text on one line or several, with selection,
  undo, the clipboard and input methods.
- [Text-input primitives](text-input-client.md): application-owned text,
  native composition and queries, and retained geometry for custom controls.
- [Number input](number-input.md): a number in a range, typed or stepped.
- [Search field](search-field.md): a field for searching, which Escape
  clears.
- [Combobox](combobox.md): a field choosing one of a list, which typing
  filters.
- [Autocomplete](autocomplete.md): a text input suggesting what was typed.
- [Token field](token-field.md): a list of strings as chips, as tags or
  recipients.
- [Editable text](editable-text.md): a text renamed in place, as a file's
  name in Finder.
- [Find bar](find-bar.md): a bar for finding text, with the count of
  matches and the next and previous ones.

## Dates and colors

- [Calendar](calendar.md): a month choosing a day.
- [Date input](date-input.md): a date in a field, chosen in a calendar below
  it.
- [Time input](time-input.md): the time of day, its hours and minutes typed
  or stepped.
- [Color picker](color-picker.md): a color well, and the picker it opens.

## Collections

- [List](list.md): rows of any number and height, built only while in view.
- [Table](table.md): rows in columns that the user sorts, resizes and moves.
- [Tree](tree.md): items that open and close, built inside the items they
  belong to.
- [Outline](outline.md): a tree of any size, as a list or a table.
- [Grid view](grid-view.md): items in a grid of any size, as a photo
  library.

## Containers

- [Scroll view](scroll.md): containers that scroll their children.
- [Grid](grid.md): children in columns and rows, as CSS grid lays them out.
- [Split view](split.md): two panes with a divider the user drags.
- [Collapsible](collapsible.md): content that a label shows and hides.
- [Accordion](accordion.md): sections that open and close, one above the
  other.
- [Form](form.md): labeled controls with descriptions and errors.

## Navigation

- [Tabs](tabs.md): a row of tabs choosing a page.
- [Sidebar](sidebar.md): a source list of items in sections, as Finder's.
- [Breadcrumbs](breadcrumbs.md): a path whose items take the user back.
- [Back and forward buttons](history-buttons.md): buttons going through a
  router's history.

## Dialogs and messages

- [Dialog](dialog.md): a dialog over the dimmed window.
- [Alert dialog](alert-dialog.md): a question about something important,
  with buttons.
- [Popover](popover.md): a panel below an element.
- [Tooltip](tooltip.md): a tip shown as the pointer rests on an element, or
  the keyboard focus comes to it.
- [Toast](toast.md): a short message near the bottom of the window, with an
  action or not.

## Status

- [Progress bar](progress.md): work done so far, or of unknown length.
- [Spinner](spinner.md): spokes turning while work of unknown length goes
  on.
- [Meter](meter.md): a value in a range, colored by its level.
- [Badge](badge.md): a short text in a pill, as a count.

## Images

- [Image](image.md): bitmaps and SVGs in their own colors.
- [Icon](icon.md): SVG icons in the color of the text.
- [Avatar](avatar.md): a picture of a person, or their initials.
