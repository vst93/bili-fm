# Meter

`ui.Meter` creates a bar showing a value between two bounds, as AppKit's
level indicator and the web's `meter`: in the theme's `Success` color, or
`Warning` and `Danger` from the levels given, nil for none.

```go
size := app.disk.Size
ui.Meter(c, app.disk.Used, 0, size, &ui.MeterLevels{Warning: 0.8 * size, Critical: 0.95 * size}).Label("Disk")
```

`MeterLevels` are the levels from which the value shows it is worse: the
`Warning` color from `Warning`, the `Danger` color from `Critical`. A
`Critical` below `Warning` makes low values the bad ones, as a battery's:

```go
ui.Meter(c, app.battery, 0, 100, &ui.MeterLevels{Warning: 20, Critical: 10}).Label("Battery")
```

For work going on, use a [progress bar](progress.md).

## Accessibility

Assistive technology sees a level indicator of the value, named by its
`Label`.
