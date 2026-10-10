# Clipboard and drag data

Package `transfer` describes serialized content shared by the system clipboard
and native [drag and drop](ui/drag-and-drop.md). A `Data` value contains ordered
items. Each item offers alternative formats for the **same** content; separate
items describe separate values. Custom formats use MIME names and explicit
encodings. Native data never contains a Go pointer or a process-local Go value.

```go
const noteFormat transfer.Format = "application/vnd.example.note+json"
frozen := *note
data := transfer.New(transfer.NewItem(
    transfer.Bytes(transfer.Text, []byte(frozen.Title)),
    transfer.Bytes(transfer.HTML, []byte("<b>"+html.EscapeString(frozen.Title)+"</b>")),
    transfer.Lazy(noteFormat, func() ([]byte, error) {
        return json.Marshal(frozen)
    }),
))

err := mygo.Clipboard.Write(data, mygo.ClipboardOptions{
    OnRelease: func() { /* release resources captured by these providers */ },
})
row.DragData(data) // the same model, with an independent provider cache
```

`Bytes` copies its input. `Data.Items` and format lists return independent
slices, and reads return independent bytes. Empty bytes are valid. `Text`,
`HTML`, `PNG`, `URIList`, and `FileList` name the common representations.
`TextData`, `URLData`, and `FileData` construct common offers. Files are absolute
paths encoded as escaped file URLs; `Data.Files` accepts local paths only,
ignoring remote authorities and non-file URLs. Files are neither read nor
deleted. Use one item per path (`FileData` does this automatically).

## Clipboard discovery and reading

`Clipboard.Formats` returns portable names without requesting representations.
`transfer.PreferredFormat` and `Offer.Preferred` negotiate in receiver preference order.
The older `AvailableFormats` retains its platform conventions: native UTIs on
macOS, GTK targets on Linux, and canonical names on Windows (now including
custom formats and file lists).

Request only formats the receiver can use:

```go
formats := mygo.Clipboard.Formats()
if format, ok := transfer.PreferredFormat(formats, noteFormat, transfer.HTML, transfer.Text); ok {
    bytes, err := mygo.Clipboard.ReadFormat(format)
    // Decode the explicitly agreed encoding; handle err.
}

// Read accepted alternatives together; other providers are not requested.
data, err := mygo.Clipboard.Read(noteFormat, transfer.HTML, transfer.Text)
if err == nil {
    if format, ok := data.Preferred(noteFormat, transfer.HTML, transfer.Text); ok {
        bytes, err := data.Read(format)
        // Use the receiver's preferred representation.
    }
}
```

`Read()` without arguments copies all advertised representations. An empty
clipboard returns empty data. An explicit request with no matching format
returns `transfer.ErrFormat`. `ReadFormat` coalesces items as `Data.Read` does:
text joins with newlines, URI/file lists with CRLF, and other formats use the
first matching item. Use `Data.Items()` to read individual items.

Reads return materialized data with no native providers. It remains usable
from any goroutine after the owner changes, a window closes, or the source
application exits, and can be offered in a later drag. Native operations run
on the main thread. Reads that observe ownership changing return
`mygo.ErrClipboardChanged`; Windows holds the clipboard lock during a foreign
read. Clipboard access from a clipboard provider returns
`mygo.ErrClipboardReentrant`, preventing recursive reads and replacement.

Existing `ReadText`, `WriteText`, `ReadHTML`, `WriteHTML`, `ReadImage`,
`WriteImage`, and `Clear` keep their signatures. `WriteHTML` offers HTML and a
text fallback on all platforms, fixing Linux's previous text-only behavior.
`WriteImage` validates PNG before replacing existing data; native image readers
still accept TIFF, GTK image targets, and Windows DIBs. `WriteFiles` and
`ReadFiles` exchange native file lists. Empty `Write`/`WriteFiles` clears the
clipboard. A failed write does not install its release hook.

## Lazy providers and ownership

Each write and each drag takes a new `Data.Snapshot`. A lazy provider runs at
most once for that snapshot when requested; success, errors, and recovered
panics (`transfer.ErrProviderPanic`) are cached. Discovery and preference
negotiation request no bytes. Native receivers, including clipboard managers,
may request bytes immediately after publication. `Read` requests only its
selected formats. A provider error rejects that read; other representations
remain available.

Providers execute on the main thread and must return promptly. Prepare costly
content beforehand on a goroutine, and freeze the value shared by alternative
representations. Never wait for another main-thread action, read the same
provider recursively, or access the clipboard from a clipboard provider.
Standalone `Data.Read` executes its provider on the calling goroutine.

Clipboard ownership belongs to the **application**, independent of any source
window. Closing a window leaves its copied content and providers available.
Keep provider resources alive until `ClipboardOptions.OnRelease`, which runs
once on the main thread after the native callback and any enclosing clipboard
operation unwind (including a storage operation running a nested event loop). Replacement,
`Clear`, successful `Flush`, and app shutdown release providers. macOS may also
finish them after fulfilling all promises. The hook can write a new clipboard
while the app runs; clipboard calls after app shutdown return the stopped-loop
error or the convenience method's empty result. It must not depend on a window
still existing. Drag sources instead have their session's `Done` callback.

`Clipboard.Flush` resolves every owned representation before handing it to
native storage. Provider failures leave ownership intact and are returned;
handle them before quitting if persistence is essential. App quitting flushes
automatically on a best-effort basis and then releases remaining provider
resources. Flushing while another app owns the clipboard leaves its content
alone. On Linux a clipboard manager is required for persistence after exit.
Without one, `Flush` returns `mygo.ErrClipboardPersistence`, releases lazy
resources, and keeps serving eager bytes while the app runs.

## Native interoperability

| Platform | Clipboard implementation | Item boundaries and formats |
|---|---|---|
| macOS | NSPasteboard items and application-owned data providers; eager items on flush | Item boundaries survive native reads and persistence. Text/HTML/PNG/file URLs use system UTIs; custom MIME names share drag/drop's UTType mapping. Full URI/file lists have serialized types alongside single-URL fallbacks. UTF-16 text and TIFF images are converted when reading. |
| Linux | GtkClipboard selection targets, shared drag format mapping, get/clear callbacks, owner-change tracking; GTK clipboard-manager storage | Within the owning MyGo process all items survive. Native selections coalesce text and URI/file lists; custom formats use the first matching item. HTML is offered as `text/html`. Persistence depends on the desktop clipboard manager. |
| Windows | The same delayed-rendering IDataObject used for drags, with OleSetClipboard/OleFlushClipboard; locked Win32 reads | Within the owning process all items survive before flush. Native reads coalesce text/URI/file lists and use the first custom item. Unicode text, CF_HTML, PNG/DIB, CF_HDROP, URL, and registered MIME formats interoperate with system apps. Optional private length hints preserve MyGo's empty/binary payloads after HGLOBAL persistence; the actual native formats remain unwrapped bytes. |

Foreign clipboard representations are limited to 64 MiB each. Foreign Windows HGLOBAL
custom data without a byte-length convention may contain allocator padding;
use an encoding with its own length when interoperating with such sources.
Virtual files and file-content promises are outside the current model. Go
values used by `Drag(value)` are process-local; their temporary drag registry
token is rejected by clipboard writes and omitted from clipboard discovery.

The [clipboard example](../examples/clipboard) copies and drags a note as text,
HTML and lazy JSON, shows provider invocation/release counts, pastes file lists
and flushes clipboard data.
