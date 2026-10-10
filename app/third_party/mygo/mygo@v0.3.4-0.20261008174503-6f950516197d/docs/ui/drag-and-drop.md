# Drag and drop

`Drag(value)` makes an element the source of a value dragged within the
window: once the pointer pressing it moves a few DIPs, a translucent copy
of the element follows the pointer, and the press is no click.
`ui.Drop[T](e)` makes `e` take values of type `T`, returning the one
dropped on it, and `ui.DragOver[T](e)` the one over it, for showing it
would take it:

```go
for _, task := range app.todo {
	ui.Row(c).Key(task.ID).Drag(task).Children(func() {
		ui.Text(c, task.Name)
	})
}

done := ui.Column(c).Grow(1).Padding(12).Radius(8).Border(1, t.Border)
if _, over := ui.DragOver[*Task](done); over {
	done.Border(2, t.Accent)
}
if task, ok := ui.Drop[*Task](done); ok {
	task.Done = true
}
```

- **Targets.** The innermost element under the pointer that takes the
  value's type gets it, so a target inside another that takes other types
  does not block it.
- **Canceling.** Escape gives up a drag.
- **Scrolling.** Scroll containers scroll as a drag nears their top or
  bottom, faster nearer to the edge.
- **Dragging several.** The copy shows how many values a drag holds, beside
  the pointer, as when the rows chosen in a list are dragged together.

`Dragging` reports that an element is dragging its value, to dim it where
it was.

## Reordering

Lists, tables and grid views reorder their rows by dragging with
`ListState.Reorder` and `GridState.Reorder`: the rows chosen move together
when the row dragged is one of them, the list shows where they would go
with a line, and the rows go where the pointer let them go.

```go
app.list.Reorder = func(rows []int, to int) {
	app.items = move(app.items, rows, to) // before row to, len for the end
}
```

See [List](list.md#reordering) and [Grid view](grid-view.md).

Files dragged from other apps are [dropped files](input.md#dropped-files).

## Between windows and applications

`DragData` starts a system drag with explicit representations from package
`transfer`. Files, text, URLs, HTML, PNG and custom serialized formats can
leave the app or come from another app. `DropData` and `DataDragOver` declare
the formats and operations an element accepts:

```go
data := transfer.TextData("Hello from MyGo")
ui.Text(c, "Drag me").Padding(12).DragData(data)

zone := ui.Box(c).Size(260, 140).Border(1, c.Theme().Border)
options := transfer.DropOptions{Formats: []transfer.Format{transfer.Text}}
if _, over := ui.DataDragOver(zone, options); over {
	zone.Border(2, c.Theme().Accent)
}
if drop, ok := ui.DropData(zone, options); ok {
	text, err := drop.Data.Read(transfer.Text)
	if err == nil {
		app.message = string(text)
	}
}
```

The innermost matching enabled target receives the drop once, in the next
frame. At least one advertised format must match, and the source and target
must share an operation. Hover advertises formats without reading bytes.
Accepted representations are copied on drop; the resulting data is safe to
keep and read from a goroutine after the native session ends.

### Representations and lazy providers

An item can offer alternatives to the same content. Custom formats use
MIME names and explicit encodings, such as JSON:

```go
const taskFormat transfer.Format = "application/vnd.example.task+json"
frozen := *task // snapshot the value that all representations describe
data := transfer.New(transfer.NewItem(
	transfer.Bytes(transfer.Text, []byte(frozen.Title)),
	transfer.Lazy(taskFormat, func() ([]byte, error) {
		return json.Marshal(frozen)
	}),
))
row.Drag(task).DragData(data)
```

`Bytes` copies its input. `Lazy` advertises a format and caches its result
or error once per drag session, when requested by a receiver. Providers
run on the main thread and must return promptly. Prepare expensive data on
a goroutine before offering it; a provider must never wait for work needing
the main thread. External applications can request data before release.

Use `DragDataFrom(func() transfer.Data { ... })` to snapshot a changing
selection when the gesture starts. Use `transfer.URLData(url)` and
`transfer.FileData(absolutePaths...)` for URLs and file lists. File paths
are escaped as file URLs; the receiver's `Data.Files()` returns local
paths, while `Data.URLs()` returns URI-list entries. `FileData` transfers
existing paths; it does not create promised files or stream file contents.

`Drag(value)` alone stays within its window. Combining it with `DragData`
also offers its Go value to existing `Drop[T]` and `DragOver[T]` targets in
other windows of the same process. Those typed targets accept copy. A
temporary random session token identifies the value; native representations
contain no Go pointer and the token stops resolving when the source ends.
Other processes use the serialized representations and decode them explicitly.

### Copy, move, completion and cancellation

Sources and destinations default to copy. To allow moves, give both
`Operations: transfer.Copy | transfer.Move`. The modifier-key suggestion
wins when both ends allow it; otherwise copy is preferred, then move.
An incompatible offer is rejected. Option requests copy on macOS and Ctrl
elsewhere; Cmd requests move on macOS and Shift elsewhere.

```go
row.DragData(data, transfer.DragOptions{
	Operations: transfer.Copy | transfer.Move,
	Done: func(result transfer.Result) {
		if result.Err != nil {
			app.report(result.Err)
		} else if result.Operation == transfer.Move {
			app.removeOriginal(task.ID)
		}
	},
})
```

`Done` runs exactly once on the main thread, including cancellation,
native startup failure, source removal and window destruction. Escape
cancels through the system drag tracker. `Window.CancelDataDrag()` cancels
a source owned by that window, from any goroutine. `Window.StartDataDrag`
starts a transfer directly from a current native pointer gesture; UI
elements normally use `DragData` so the movement threshold is applied.

`Result.Operation` is the destination's final effect. `Canceled` means no
drop occurred; `Err` reports a native/provider failure. MyGo never deletes
files. Remove application records only after a successful move, and account
for file managers that move a file themselves. Existing `DroppedFiles` and
`OnFileDrop` listeners keep accepting files with copy semantics.

The default preview is a snapshot of the source's visible element, bounded
to 512 DIPs. Set `DragOptions.Preview` to an `image.Image` and `Hotspot` to
its pointer offset for a custom preview (up to 4096 pixels per dimension).
The image is copied at drag start; one image pixel represents one DIP.

### Native format mapping

| Platform | Native transfer | Representations |
|---|---|---|
| macOS | AppKit dragging sessions and pasteboard item providers | Text/HTML/PNG/file URLs use system UTIs. MIME names map through `UTType`, with a reversible identifier fallback. Item boundaries are preserved. |
| Linux | GTK 3 drag contexts and asynchronous selections | MIME targets, UTF8_STRING for text, text/uri-list for URLs and files. Accepted formats are requested after drop. |
| Windows | OLE IDataObject / IDropSource / IDropTarget | Unicode text, CF_HDROP, URL, CF_HTML, PNG and registered custom names; delayed rendering and Shell drag images. Streams preserve exact binary lengths; HGLOBAL sources may include padding. |

GTK and OLE have a single selection/data object: multiple text items are
joined by newlines, URI/file lists by CRLF, and other formats use the first
item offering them. Transfers between windows in the same MyGo process
preserve all items through the core's local registry. Incoming GTK/OLE
representations are limited to 64 MiB each; native providers must supply
memory or stream representations (virtual-file promises are not supported).

See [`examples/drag-drop`](../../examples/drag-drop) for two windows exchanging
notes, files, text and URLs, including a lazy JSON representation and move
completion. The transfer model is reusable without native UI; existing
clipboard convenience methods keep their signatures. `Clipboard.Write` and
`Clipboard.Read` exchange this same `transfer.Data`; each clipboard write and
drag gets an independent provider cache. See [Clipboard and drag data](../data-transfer.md)
for clipboard negotiation, persistence, and provider lifetimes.
