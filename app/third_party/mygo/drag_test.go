package mygo

import (
	"errors"
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func TestDataDragAcrossContentWindows(t *testing.T) {
	value := &struct{ Name string }{"original"}
	var received any
	var results []transfer.Result
	w, _, src := contentWindow(t, func(c *ui.Context) {
		ui.Box(c).Size(100, 50).Drag(value).DragData(transfer.TextData(value.Name), transfer.DragOptions{Done: func(r transfer.Result) { results = append(results, r) }})
	})
	_, _, dst := contentWindow(t, func(c *ui.Context) {
		e := ui.Box(c).Size(100, 100)
		if v, ok := ui.Drop[*struct{ Name string }](e); ok {
			received = v
		}
	})
	onMain(func() {
		src.Send(platform.SurfaceEvent{Kind: platform.PointerDown, X: 10, Y: 10})
		src.Send(platform.SurfaceEvent{Kind: platform.PointerMove, X: 30, Y: 20})
	})
	onMain(func() {
		r := src.DataDrag()
		if r == nil {
			t.Error("native source was not scheduled")
			return
		}
		if r.Operations != transfer.Copy || len(r.Preview) == 0 {
			t.Errorf("request %+v", r)
		}
		d := &platform.DataDragEvent{Session: r.Session, Offer: transfer.Offer{Formats: r.Data.Formats(), Operations: r.Operations}}
		if !dst.Send(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 10, Y: 10, Drag: d}) || d.Local != value {
			t.Error("process-local value did not resolve")
		}
		if !dst.Send(platform.SurfaceEvent{Kind: platform.DataDrop, X: 10, Y: 10, Drag: d}) {
			t.Error("typed destination rejected source")
		}
		dst.Frame()
		src.FinishDataDrag(transfer.Result{Operation: d.Operation})
		// Reusing the ended session cannot recover the Go value or its data.
		resolveDataDrag(d)
		if d.Local != nil || len(d.Data.Formats()) != 0 || activeDataDrag != nil {
			t.Error("registry retained a completed session")
		}
	})
	if received != value || len(results) != 1 || results[0].Operation != transfer.Copy {
		t.Fatalf("received %v, results %v", received, results)
	}
	w.CancelDataDrag()
	if len(results) != 1 {
		t.Fatal("completion called twice")
	}
}

func TestDataDragCancellationLifecycle(t *testing.T) {
	for _, destroy := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "destroy"}[destroy], func(t *testing.T) {
			w, _, s := contentWindow(t, func(c *ui.Context) { ui.Text(c, "source") })
			var results []transfer.Result
			if err := w.StartDataDrag(transfer.TextData("data"), DragOptions{Done: func(r transfer.Result) { results = append(results, r) }}); err != nil {
				t.Fatal(err)
			}
			onMain(func() {
				if s.DataDrag() == nil {
					t.Error("missing source")
				}
			})
			if err := w.StartDataDrag(transfer.TextData("another"), DragOptions{}); !errors.Is(err, errDragBusy) {
				t.Fatalf("concurrent source: %v", err)
			}
			if destroy {
				w.Destroy()
			} else {
				w.CancelDataDrag()
			}
			onMain(func() {
				s.FinishDataDrag(transfer.Result{Operation: transfer.Move})
				if activeDataDrag != nil {
					t.Error("session leaked")
				}
			})
			if len(results) != 1 || !results[0].Canceled {
				t.Fatalf("results %v", results)
			}
		})
	}
}

func TestDataDragPreservesFileDropFallback(t *testing.T) {
	var received []string
	var listener *FileDropEvent
	w, _, s := contentWindow(t, func(c *ui.Context) {
		e := ui.Box(c).Size(100, 100)
		if files := e.DroppedFiles(); files != nil {
			received = append(received, files...)
		}
	})
	paths := []string{filepath.Join(t.TempDir(), "a.txt")}
	data, _ := transfer.FileData(paths...)
	onMain(func() {
		d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: data.Formats(), Operations: transfer.Copy | transfer.Move}, Data: data}
		if !s.Send(platform.SurfaceEvent{Kind: platform.DataDragOver, X: 20, Y: 20, Drag: d}) || d.Operation != transfer.Copy {
			t.Error("legacy target did not negotiate copy")
		}
		if !s.Send(platform.SurfaceEvent{Kind: platform.DataDrop, X: 20, Y: 20, Drag: d}) {
			t.Error("legacy target rejected files")
		}
		s.Frame()
	})
	if !slices.Equal(received, paths) {
		t.Fatalf("files %q", received)
	}
	w.OnFileDrop(func(e *FileDropEvent) { listener = e })
	onMain(func() {
		d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: data.Formats(), Operations: transfer.Copy | transfer.Move}, Data: data}
		if !s.Send(platform.SurfaceEvent{Kind: platform.DataDrop, X: 200, Y: 150, Drag: d}) {
			t.Error("listener did not take files")
		}
	})
	if listener == nil || !slices.Equal(listener.Paths, paths) {
		t.Fatalf("listener %+v", listener)
	}
}

func TestDataDragValidationAndPreviewOwnership(t *testing.T) {
	w, _, s := contentWindow(t, func(c *ui.Context) { ui.Text(c, "source") })
	if err := w.StartDataDrag(transfer.Data{}, DragOptions{}); err == nil {
		t.Fatal("empty source accepted")
	}
	reserved := transfer.New(transfer.NewItem(transfer.Bytes(platform.DragSessionFormat, []byte("fake"))))
	if err := w.StartDataDrag(reserved, DragOptions{}); err == nil {
		t.Fatal("reserved format accepted")
	}
	preview := image.NewRGBA(image.Rect(0, 0, 10, 10))
	preview.Set(0, 0, color.RGBA{255, 0, 0, 255})
	if err := w.StartDataDrag(transfer.TextData("data"), DragOptions{Preview: preview}); err != nil {
		t.Fatal(err)
	}
	preview.Set(0, 0, color.RGBA{})
	onMain(func() {
		r := s.DataDrag()
		if r == nil || r.Preview[2] != 255 {
			t.Error("preview was not copied")
		}
		for _, token := range []string{"", "forged"} {
			d := &platform.DataDragEvent{Session: token}
			resolveDataDrag(d)
			if d.Local != nil || len(d.Data.Formats()) != 0 {
				t.Error("forged session resolved")
			}
		}
	})
	w.CancelDataDrag()
}

func TestDataDragCompletionCannotExceedSourceOperations(t *testing.T) {
	w, _, s := contentWindow(t, func(c *ui.Context) { ui.Text(c, "source") })
	var result transfer.Result
	if err := w.StartDataDrag(transfer.TextData("copy only"), DragOptions{Done: func(r transfer.Result) { result = r }}); err != nil {
		t.Fatal(err)
	}
	onMain(func() { s.FinishDataDrag(transfer.Result{Operation: transfer.Move}) })
	if result.Operation != transfer.None || result.Err == nil {
		t.Fatalf("disallowed move completed: %+v", result)
	}
}

func TestDataDragPreservesRawNativeFilePaths(t *testing.T) {
	var received []string
	w, _, s := contentWindow(t, func(c *ui.Context) {
		if files := ui.Box(c).Size(100, 100).DroppedFiles(); files != nil {
			received = files
		}
	})
	// Legacy native deliveries preserve paths verbatim, including paths
	// that cannot be advertised by transfer.FileData as absolute file URLs.
	paths := []string{"native/path/a", "native/path/b"}
	onMain(func() {
		d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: []transfer.Format{transfer.FileList}, Operations: transfer.Copy | transfer.Move}, HasFiles: true}
		if !s.Send(platform.SurfaceEvent{Kind: platform.DataDrop, X: 20, Y: 20, Drag: d, Files: paths}) || d.Operation != transfer.Copy {
			t.Error("native file paths were not accepted as a copy")
		}
		s.Frame()
	})
	if !slices.Equal(received, paths) {
		t.Fatalf("native paths %q, want %q", received, paths)
	}
	var listener *FileDropEvent
	w.OnFileDrop(func(e *FileDropEvent) { listener = e })
	onMain(func() {
		d := &platform.DataDragEvent{Offer: transfer.Offer{Formats: []transfer.Format{transfer.FileList}, Operations: transfer.Copy}, HasFiles: true}
		if !s.Send(platform.SurfaceEvent{Kind: platform.DataDrop, X: 200, Y: 150, Drag: d, Files: paths}) {
			t.Error("raw paths did not reach OnFileDrop")
		}
	})
	if listener == nil || !slices.Equal(listener.Paths, paths) {
		t.Fatalf("listener %+v", listener)
	}
}
