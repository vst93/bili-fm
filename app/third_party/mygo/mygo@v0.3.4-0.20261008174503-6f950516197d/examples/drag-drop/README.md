# Native drag and drop

Run `go run ./examples/drag-drop`, or package it with
`go run ./cmd/mygo build examples/drag-drop`.

Drag notes between the two windows. Both allow copy and move; the source
removes its note only after a successful move. Drag a note to a text
editor to receive its plain text, a URL to a browser, and the sample file
to Finder, Explorer, or a Linux file manager. Sample files always copy.
Drop files, selected text, URLs, or notes from a second instance back into
either window. Escape cancels; the status and log show completion.

The note has plain text and a lazy JSON representation. The JSON is
decoded explicitly by the receiver. `Drag(note)` also makes its Go value
available to typed destinations in the same process; this example uses
serialized destinations so it works across application instances too.
