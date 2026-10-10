# Views

The view is a function from your app's state to its interface. MyGo calls
it on the main thread to build frames after input, model changes and while
something animates. A frame can contain several build passes, so it can
show the outcome of an action before it is painted.

The window has one stable `*ui.Context`. Child builders share it and
switch its current parent for the duration of the callback. Pass it to
helpers that build part of the view.

Each `ui.Element` value belongs to one build pass. Its owner, slot and
generation are checked before accessing the control. Use `Valid` for an
optional element and `ui.Element{}` for absence. Stale use is diagnosed in
development and `Tester`; production methods return empty results or
ignore the operation.

Keep build values out of app state and background work. Store `ui.Handle`
for persistent control identity and `ui.Services` for callbacks between
builds. Model data and widget state such as `ListState` belong in your app.

```go
// todoList is the app's state, which lasts.
type todoList struct {
	todos []Todo
	draft string
}

// view builds the interface from it.
func (app *todoList) view(c *ui.Context) {
	ui.Column(c).Fill().Padding(16).Gap(8).Children(func() {
		for i := range app.todos {
			ui.Checkbox(c, &app.todos[i].Done, app.todos[i].Title)
		}
		if ui.TextInput(c, &app.draft).Placeholder("New to-do").Submitted() {
			app.todos = append(app.todos, Todo{Title: app.draft})
			app.draft = ""
		}
	})
}
```

Make the state once, as the app starts, and give its view to the window:

```go
app := &todoList{}
mygo.NewWindow(mygo.WindowOptions{Title: "To-dos", Content: ui.View(app.view)})
```

`ui.View(app.view)` is the window's content (`mygo.WindowOptions.Content`);
a `Content` can serve several windows, each with its own element state.

The examples in these guides are parts of such a view: `c` is the view's
`*ui.Context`, and `app` its receiver, the value of your own type that
holds the state, as `todoList` here. A field such as `app.volume` or a
method such as `app.save()` is one you declare on that type. Values that
last, such as a [router](navigation.md), are fields made with the rest of
the state, never in the view, which runs for every frame.

## Events and actions

`Clicked` reports a click since the last frame and keeps ordinary control
flow beside the control:

```go
if ui.Button(c, "Save").Clicked() {
    app.save()
}
```

Click and shortcut queries apply pending bound input to controls built so
far before returning, so an inline save sees the latest edits. Configure
those controls before asking for an action response.

`OnClick` schedules the action after the view finishes building and bound
input has been applied:

```go
ui.Button(c, "Save").Disabled(app.saving).OnClick(app.save)
```

Use polling when inline `return`, `break` or `continue` is useful. Use
callbacks for actions that modify the collection being built, so the loop
finishes before its model changes:

```go
for _, item := range app.items {
    ui.Row(c.Key(item.ID)).Children(func() {
        ui.Text(c, item.Title).Grow(1)
        ui.Button(c, "Delete").OnClick(func() {
            app.removeItem(item.ID)
        })
    })
}
```

Handled input is consumed before rebuilding, so an action runs once for
that input. MyGo rebuilds changed state before painting the frame.

Widgets that change a value take a pointer to it, so they need no handler:
`ui.Checkbox(c, &app.settings.Sync, "Sync")` binds the field to the control.
`Changed` and `Submitted` apply pending input to the controls built so far
before returning. Their bound values are available immediately in the same
build pass. Set `Disabled`, `ReadOnly` and other input options before querying
responses; configuration after a query cannot undo input already applied.

```go
if ui.TextInput(c, &app.query).Placeholder("Search").Changed() {
	app.results = search(app.query)
}
```

Local copies work too, including values read from a map:

```go
viewed := app.viewed[file.ID]
if ui.Checkbox(c, &viewed, "Viewed").Changed() {
    app.viewed[file.ID] = viewed
}
```

`OnChange` and `OnSubmit` run after construction and bound input, before the
rebuild. Use callbacks when an action should wait until the view has finished
building, such as modifying the collection being rendered. They can also
commit local copies:

```go
viewed := app.viewed[file.ID]
ui.Checkbox(c, &viewed, "Viewed").OnChange(func() {
    app.viewed[file.ID] = viewed
})
```

## Keys

Give a control its key before construction, so its state follows the item
when the surrounding collection changes:

```go
ui.Checkbox(c.Key(todo.ID), &todo.Done, todo.Title)
ui.TextInput(c.Key("search"), &app.query)
```

Keys are unique among siblings. `Element.Key` can key a container before
its children or local state are built. Use `Context.Key` for stateful
controls and custom base widgets.

## Change the state from other goroutines

The view reads your state on the main thread. Change it from other
goroutines with `Window.Update`, which runs a function on the main thread
and then draws a new frame:

```go
win := mygo.NewWindow(mygo.WindowOptions{Content: ui.View(app.view)})
go func() {
	items, err := fetchItems()
	win.Update(func() { app.items, app.err = items, err })
}()
```

`Window.Invalidate` only draws a new frame, for state you guard yourself.
Within the view, `c.Invalidate()` asks for another frame and `c.After(d)` for
one after a delay, such as a clock's next second.

## State of an element

An element keeps state of its own from frame to frame with `ui.Local`, for
widgets you build yourself; see [custom widgets](custom-widgets.md).

## Check build lifetimes

Run `mygo vet` from your app directory to find build values stored in
structs, package variables or goroutines. See [Migration](migration.md)
for the source migration command and [Performance](performance.md) for
measured frame costs.
