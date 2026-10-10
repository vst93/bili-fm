# Progress bar

`ui.Progress` creates a progress bar filled to a value between 0 and 1; a
negative value shows activity of unknown length, a segment sliding across.

```go
ui.Progress(c, float64(app.done)/float64(app.total)).Label("Uploading")
ui.Progress(c, -1).Label("Connecting")
```

`Reverse` fills it from the right, for interfaces laid out from right to
left. It stretches across its column; give it a `Width` in a row.

A bar of unknown length moves only while it is in view, and costs only its
painting: the window paints the last frame again at the display's rate,
without running the view.

For a spinning indicator that takes less room, use a
[spinner](spinner.md); for a level that is no progress, as a disk's use, a
[meter](meter.md).

## Accessibility

Assistive technology sees a progress indicator of the value, or of unknown
length, named by its `Label`.
