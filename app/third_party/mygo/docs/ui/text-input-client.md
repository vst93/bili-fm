# Text-input primitives

`TextInputBase` and `TextAreaBase` provide ordinary editable string controls
without a look. For a control with its own storage, selections, rendering or
editing policy, attach a `TextInputClient` to an element:

```go
ui.Box(c).
    HandleTextInput(app.input).
    HandleInput(app.input.HandleInput).
    Draw(app.input.Draw).
    Label("Document").
    Role(ui.RoleTextField)
```

The primitive connects your state to the platform's input methods and text
queries. Your client owns its buffer, selection model, marked text and undo
history. It can use a rope, persistent tree or incremental paragraph store;
MyGo does not copy a whole document on each edit or choose formatting behavior.
The existing `HandleInput`, `TextCaret`, `TextInput` and `TextArea` APIs remain
available. `TextCaret` is useful for terminals and other widgets that need a
candidate-window location but do not provide a text document.

The built-in `TextInput` and `TextArea` also implement this client contract.
Their `*string` bindings publish committed text when the widget builds;
native queries already see queued navigation and edits before that frame.
Preedit is a virtual insertion into the native document until committed,
and its replacement and commit share one undo transaction. Password fields
provide no committed text to native queries; read-only fields disable input.
Paragraph indexes include UTF-16 offsets so native queries near the end of
a large document do not scan its prefix.

String bindings still produce a whole string on each committed edit. For
indexed storage, use `TextBuffer` with `TextInputBuffer` or `TextAreaBuffer`:

```go
// Keep the buffer across frames; its zero value is also usable.
document := ui.NewTextBuffer(initialText)
// In the view:
input := ui.TextAreaBuffer(c, document).Fill().Font("monospace")
if input.Changed() {
    // Length/version queries read tree summaries without flattening text.
    version := document.Version()
    _ = version
}
```

`TextInputBufferBase` and `TextAreaBufferBase` provide the unstyled variants.
They share the string controls' selection, styling, clipboard, IME and undo
behavior. Native mutations update the buffer immediately, and `Changed`
reports them when the widget builds. A programmatic buffer edit refreshes the
control and clears its stale selection/undo state; use `Window.Update` to
schedule such changes and invalidate the view.

`TextBuffer` is a balanced tree of bounded, owned UTF-8 chunks. It indexes
bytes, runes, UTF-16 units and newline counts. Small edits copy affected chunks
and tree paths; they do not copy the whole document or shift every following
paragraph's absolute offsets. `Snapshot` shares unchanged chunks and remains
valid after edits; `Restore` adopts one in constant time. Buffer methods are
safe from any goroutine, and snapshots can be read concurrently with edits.

`Replace(start, end, text)` and `Slice(start, end)` count runes, matching
`Element.TextSelection`. `ReplaceUTF16`, `TextForRange`, `UTF16Offset` and
`RuneOffset` bridge native ranges and keep surrogate pairs whole. Snapshots
also supply `Line`, `LineRange` and `LineAt` for custom editor layout.

`String` explicitly materializes the document. For file export, `WriteTo`
streams chunks to an `io.Writer`. Ordinary multiline editing and bounded IME
queries do not flatten the document; an explicit full-document query or the
current accessibility Value protocol can request its complete text. Layout
continues to shape one logical paragraph at a time. Changes to the number of
lines update the control's paragraph/height metadata.

The [text-buffer example](../../examples/text-buffer) shows a 100,000-line
editable document.
Custom editors can use the same buffer through their `TextInputClient` and
choose their own selection, history and rendering policy.

The [text-input example](../../examples/text-input) shows a small
application-owned text field.
It demonstrates composition, pointer selection, Edit-menu commands and custom
rendering. It is a primitive example, not an editor implementation.

## Client contract

Pass a pointer to a client kept across frames. `HandleTextInput(nil)` (including
a typed nil pointer) disconnects it. The element is focusable and has the text
cursor; its size, appearance and other input behavior are yours.

| Method | Purpose |
|---|---|
| `TextForRange(range)` | Return requested text and the actual clamped/adjusted range. Full-document queries are supported; no fixed surrounding-window restriction applies. |
| `Selection()` | Report the primary native selection and its direction. Additional cursors or discontiguous ranges stay in application state. |
| `MarkedRange()` | Report preedit in the current document, or false when none exists. |
| `ReplaceText(range, text)` | Commit text in an explicit range; a nil range means preedit if present, otherwise the selection. Clear preedit according to your editing policy. |
| `SetMarkedText(range, text, selected)` | Replace preedit, retaining its marked range. `selected` is relative to the newly marked text. |
| `UnmarkText()` | Clear marked status while retaining document text. Also called when focus is lost, the client changes, or the element is disposed. |
| `BoundsForRange(range)` | Return element-relative text/caret geometry, the actual range represented, and whether it is available. Return false for unavailable/offscreen text. |
| `IndexForPoint(point)` | Hit-test an element-relative point and return a document offset, or false. |

Ranges count **UTF-16 code units**, matching native text services and GPUI's
input-handler contract. This is an interop coordinate system, not a storage
format. `ui.UTF16Len` and `ui.UTF16ByteOffset` help simple UTF-8 buffers convert
at the boundary; an indexed buffer can provide faster conversions itself.
A surrogate pair must stay whole, and your editing policy should respect
Unicode grapheme boundaries. `TextForRange` returns its adjusted range so the
platform knows what text it received.

Callbacks run synchronously on the UI thread, before the next frame. They must
not wait for work requiring that thread. The client owns synchronization with
other goroutines; schedule application state changes with `Window.Update` as
for other native UI views. After a native mutation, MyGo asks for a frame and
reads the latest selection/caret before the next input event. Stale native
references cannot modify an unfocused, replaced or disposed client.

On macOS the client supplies `NSTextInputClient` selection, marked ranges,
substring queries, range bounds and point hit testing. GTK uses bounded
surrounding text, preserves UTF-8/rune/UTF-16 conversions, and forwards
surrounding deletions and preedit/commit callbacks. Windows IMM32 uses bounded
document-feed/reconversion context and forwards composition and commit ranges.
Changing the client resets native preedit so a candidate cannot move to a
newly focused field. The platform callbacks are allocated once at startup.

Keyboard navigation, pointer selection, clipboard policy and Edit-menu
commands come through `HandleInput`. They belong to your control. This permits
an editor to choose multiple selections, custom keymaps, grouping and history
without overriding behavior built into a framework-owned editor.

## Retained text geometry

`ShapeText(text, font, width)` and `ShapeRichText(spans, font, width)` produce a
`TextLayout` using Core Text, Pango or DirectWrite. Positive `width` wraps text;
zero only breaks at newlines. Cache layouts by paragraph or line and replace
only those affected by an edit.

```go
layout := ui.ShapeText(paragraph, ui.Font{Size: 16}, availableWidth)
caret := layout.Caret(offset)
offset = layout.IndexAt(ui.Point{X: pointerX, Y: pointerY})
rects := layout.SelectionRects(selectedRanges...)
// Inside Draw:
p.TextLayout(layout, originX, originY, color)
```

Layout offsets are UTF-16 too. `Caret` and `IndexAt` snap to whole graphemes;
`PreviousBoundary` and `NextBoundary` support logical navigation/deletion.
`SelectionRects` accepts multiple ranges and keeps the separate visual pieces
of bidi selections. It supplies geometry without inventing a single-range
selection model or choosing an editor's navigation policy. Size and geometry
queries are safe from any goroutine; painting is part of a UI frame.

For visual navigation, retain `TextCaretPosition`, including its `Affinity`.
`TextDownstream` chooses the following logical text's edge; `TextUpstream`
chooses the preceding edge. The edges may differ at a bidi boundary or wrap.
`CaretAt` paints that edge, `PositionAt` preserves it when hit-testing, and
`MoveCaret(position, direction)` moves one visual grapheme left (negative)
or right (positive). `SelectionRanges(anchor, caret)` maps a visual gesture
to logical UTF-16 ranges, which can have gaps in mixed-direction text:

```go
caret = layout.MoveCaret(caret, 1)
ranges := layout.SelectionRanges(anchor, caret)
rects := layout.SelectionRects(ranges...)
// Use the same ranges for copy, deletion and replacement in your buffer.
```

The built-in controls use these visual edges and range sets too. Copy joins
the selected fragments in logical order; replacement preserves text in the
gaps, and undo restores the visual selection as well as the text.
