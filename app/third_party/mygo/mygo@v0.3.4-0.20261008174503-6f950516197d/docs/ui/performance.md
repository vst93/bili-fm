# Native UI performance

These measurements cover steady frames rendered on the CPU with `ui.Tester`.
They were taken on October 8, 2026 on Apple M5, macOS arm64, Go 1.27.1, with
`CGO_ENABLED=0 GOMAXPROCS=1`. Each timing is the median of six sequential
300 ms samples after warming the view.

| Fixture | Time per frame | Allocations per frame | Allocated bytes per frame |
|---|---:|---:|---:|
| 1,000 static labels | 397 µs | 0 | 0 |
| Visible rows of a million-row list | 14.5 µs | 20 | 176 |
| Diff view with no changes | 389 µs | 2 | 72 |

Element handles validate their owner, arena slot and build generation.
Static label handles do not allocate. Input bindings, action queues and
persistent identity bindings reuse storage once warmed. Composite controls
can allocate callback closures; these fixtures do not cover every widget
or interaction.

The label and list fixtures use the public API. The diff fixture uses the
private renderer API and measures its steady-frame allocation floor.

## Retained memory

A warmed view containing 1,000 labels retained approximately **2.50 MB of
Go heap**, measured over three samples. The measurement warms fonts and
rendering, releases a warm view, forces collection, then measures a second
retained view after ten frames and another collection.

This includes the headless rendering host. It does not measure native window
process memory or GPU resources.

## Measurement scope

Timing depends on the machine, rendering backend, content and activity in
other processes. Small timing differences should be rechecked on several
machines; allocation counts are more stable. These results describe steady
frames and do not compare MyGo with another GUI framework.

<!-- repository-only:start -->

## Run the benchmark fixtures

From the MyGo repository root:

```sh
CGO_ENABLED=0 GOMAXPROCS=1 go test ./ui -run '^$' \
  -bench '^(BenchmarkValueFrame|BenchmarkValueList|BenchmarkDiffSteady)$' \
  -benchmem -count=6 -benchtime=300ms
CGO_ENABLED=0 GOMAXPROCS=1 go test ./ui \
  -run '^TestValueFrameRetainedHeap$' -count=3 -v
```

Keep compilation and other tests out of the timed runs.

<!-- repository-only:end -->
