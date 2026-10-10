# Migrating to checked elements

Native UI views receive `*ui.Context`, child builders use `func()` callbacks,
and widget constructors return `ui.Element` values. This guide covers source
migration, build lifetimes, persistent control identity and focus bindings.

## Apply the source migration

Run the migration from your app directory:

```sh
go tool mygo migrate-ui .          # preview affected files
go tool mygo migrate-ui -write .   # write the source migration
go tool mygo vet .
go test ./...
```

Review the diff before committing it. The codemod uses Go syntax and the
actual UI import alias, sharing field and helper-return declarations across
files in the same package. It converts element and custom-parts pointer types,
element nil checks and zero assignments, moves fluent constructor keys into
`Context.Key`, and renames grid `Rows(n)` to `GridRows(n)`. Context pointers,
child callback signatures, ordinary model pointers and unrelated nil checks
are preserved. Dot imports and keys on separately stored elements need
manual review. It does not move arbitrary polling control flow into callbacks.
`Changed` and `Submitted` polling can commit local bound values in the same
build pass, as described below.

## Element values and keys

```go
var element ui.Element
if element.Valid() {
    element.Focus()
}
```

Use `ui.Element{}` instead of assigning or returning `nil`. Custom parts
such as `ui.SelectParts[T]` are values containing checked elements too.

An element expires before the next build pass, including a rebuild within
the same frame. Generation validation rejects it even when its arena slot
has been reused. `Tester` and development builds panic with a message naming
the passes and recommending `ui.Handle`; production builds with
`mygo_noinspector` return empty query results or ignore stale mutations.
The absent zero value remains safe in either mode. `Valid` does not panic.

Assign keys before state initialization:

```go
ui.TextInput(c.Key("search"), &a.query).ReadOnly(a.readOnly)
ui.Checkbox(c.Key(todo.ID), &todo.Done, todo.Title)
parts := ui.SelectBase(c.Key("choice"), &a.choice)
```

`Element.Key` assigns a container's key before its children or local state
are built. Use `Context.Key` for stateful widgets, including
custom base controls. A key names the next outer control, and is consumed
before it initializes its state.

## Input and actions

`Changed` and `Submitted` apply pending input to controls built so far before
returning, so the bound values can be read immediately. Set `Disabled`,
`ReadOnly` and slider settings before querying responses. Controls whose
responses are not queried apply input after construction. Configuration
after a response query cannot undo input already applied.

```go
ui.Button(c, "Save").Disabled(a.saving).OnClick(a.save)
ui.TextInput(c.Key("query"), &a.query).OnChange(a.search)
ui.TextInput(c.Key("draft"), &a.draft).OnSubmit(a.send)
c.OnShortcut(ui.Cmd, ui.KeyS, a.save)
```

Click, shortcut, change and submit actions run after configuration and bound
input, before rebuilding. Handled input is consumed before another pass so
actions do not repeat. Polling methods remain available; prefer callbacks
when an action changes the collection being built. Keep I/O in workers and publish model results with
`Window.Update`.

Bind controls directly to persistent model fields when possible. A local
value recomputed on every build can be committed with `Changed` or
`Submitted` immediately, or with a callback after construction:

```go
viewed := a.isViewed(file)
if ui.Checkbox(c, &viewed, "Viewed").Changed() {
    a.setViewed(file, viewed)
}
```

The same rule applies to segmented controls whose selected value is derived
from several model fields. Review these bindings manually after running the
codemod; it cannot infer how to write a derived value back to your model.

## Persistent identity

Store `ui.Handle` instead of an element:

```go
type app struct {
    query string
    search ui.Handle
}

func (a *app) view(c *ui.Context) {
    ui.TextInput(c.Key("search"), &a.query).Bind(&a.search)
}

// On the UI thread, including inside Window.Update:
a.search.Focus()
```

Focus requests coalesce and wait while the control is hidden. Closing its
window cancels that window's request. `CancelFocus` cancels earlier.

`handle.Focused(c)` and `handle.FocusWithin(c)` read persistent identity,
including before the control is constructed. `c.Resolve(handle)` returns
only this pass's element. `handle.Bounds(c)` reads the committed control box
on the UI thread, including from an input callback. These are different queries: previous focus can
still be observed in the build that removes a control; committing that build
removes its actual focus.

A handle supports independent bindings in several windows. Queries take the
window's Context. Use `handle.Focus(c)` and `CancelFocus(c)` to select a
window; the no-argument Focus form requires at most one open binding. Before
the first binding, it waits for that first control. Give each window its own
widget state and focus field. `Ref`/`RequestFocus` are aliases for
`Handle`/`Focus`.

`ListState`, `ScrollState`, `GridState` and `Router` carry a Handle; other
controls can bind any app-owned handle. Use `state.Handle` for persistent focus and commands.

## Focus bound to app data

```go
type pane int
const (none pane = iota; files; diff)

ui.List(c, &a.list, len(a.files)).FocusBind(&a.pane, files).
    Rows(func(row ui.ListRow) { ui.Text(row.Context, a.files[row.Index].Name) })

// An action requests the diff pane, waiting while hidden:
a.pane = diff

// Actual focus can differ while that request waits:
focused := ui.FocusedValue(c, &a.pane)
```

`FocusBind` requires a pointer to a comparable field and a matching value.
Values are unique within that field in one window; reserve its zero value
for no focus. The field holds desired focus. User focus changes update it
when no request is waiting. Setting zero clears focus. `FocusedValue` reads
actual focus using committed identities, regardless of construction order.
A hidden binding keeps its desired request, while actual focus can be on
another control or absent. Use separate fields for separate windows.

## Lists and keyboard commands

```go
a.filesView.OnShortcut(c, ui.Cmd, ui.KeyK, a.openSelected)
ui.List(c.Key("files"), &a.list, len(a.files)).Bind(&a.filesView).Grow(1).
    ItemKey(func(i int) any { return a.files[i].ID }).
    Selection(&a.selection).
    Rows(func(row ui.ListRow) {
        text := ui.Text(row.Context, a.files[row.Index].Name)
        if row.Selected() && row.ListFocused() {
            text.TextColor(c.Theme().Accent)
        }
    })
```

A Handle's shortcut may be declared before Bind. It runs after construction
only if that window built an enabled control for the handle, so a hidden
control takes no command. `ListRow` supplies the shared Context, index,
selection and list focus. `List(c, state, n, func(i int))` also accepts a row
callback directly. Configure fluent lists before calling Rows.

## Persistent services and verification

Capture `c.Services()` for clipboard/URL callbacks and background redraws.
Services has weak window ownership and no build data. Clipboard and URL
methods run on the UI thread; Invalidate is safe from another goroutine.
Use `Window.Update` when publishing model changes.

`mygo vet` runs ordinary Go vet and type-aware checks for elements, parts,
and Context pointers stored in struct fields/package variables or captured
by/passed to goroutines. It follows imports, type aliases and inferred
variables; Handle, Services and view callback signatures are allowed.
These checks help enforce build lifetimes; they do not prove all lifetimes
or goroutine safety. Runtime generation checks remain necessary.

Exercise controls disappearing and returning, pending focus, list reordering,
text editing/IME, disabled and read-only settings, and command routing. Run
normal tests and inspect the app. See [Performance](performance.md) for
measured costs and reproduction commands.
