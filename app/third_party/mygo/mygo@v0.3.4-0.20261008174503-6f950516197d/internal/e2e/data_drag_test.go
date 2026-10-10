package e2e

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// TestNativeDataRepresentations drives native serialized providers and
// destination callbacks. The Linux test below also drives the GTK tracker
// and asynchronous selection exchange with XTEST input.
func TestNativeDataRepresentations(t *testing.T) {
	custom := transfer.Format("application/vnd.mygo.e2e+json")
	opts := transfer.DropOptions{Formats: []transfer.Format{custom, transfer.Text, transfer.FileList, transfer.URIList}, Operations: transfer.Copy | transfer.Move}
	var frames atomic.Int32
	var drops []transfer.Drop
	w := newWindow(t, mygo.WindowOptions{Title: "Data destinations", Width: 400, Height: 220, Content: ui.View(func(c *ui.Context) {
		frames.Add(1)
		e := ui.Box(c).Size(220, 150)
		if d, ok := ui.DropData(e, opts); ok {
			drops = append(drops, d)
		}
	})})
	eventually(t, "a destination frame", func() bool { return frames.Load() > 0 })
	url, _ := transfer.URLData("https://example.com/a?q=1#b")
	file := filepath.Join(t.TempDir(), "sp ace#%.txt")
	files, _ := transfer.FileData(file)
	var calls atomic.Int32
	for n, data := range []transfer.Data{transfer.TextData("hello 日本語"), url, files, transfer.New(transfer.NewItem(transfer.Lazy(custom, func() ([]byte, error) { calls.Add(1); return []byte(`{"id":7}`), nil })))} {
		op, dropped, supported := dropNativeData(w, 50, 50, data, transfer.Copy|transfer.Move)
		if !supported {
			t.Skip("native serialized drop hook unavailable; GTK uses TestNativeGTKDataDrag")
		}
		if !dropped || op != transfer.Copy {
			t.Fatalf("case %d: effect %v, dropped %v", n, op, dropped)
		}
		eventually(t, "the delivered data", func() bool { count := 0; mygo.RunOnMain(func() { count = len(drops) }); return count == n+1 })
		var got transfer.Data
		mygo.RunOnMain(func() { got = drops[n].Data })
		for _, f := range data.Formats() {
			want, _ := data.Read(f)
			actual, err := got.Read(f)
			if err != nil || !bytes.Equal(actual, want) {
				t.Errorf("case %d %s: %q, want %q (%v)", n, f, actual, want, err)
			}
		}
	}
	if calls.Load() != 2 {
		t.Fatalf("provider requests %d; native snapshot and original each read once", calls.Load())
	}
	mygo.RunOnMain(func() { opts.Operations = transfer.Move })
	w.Invalidate()
	before := frames.Load()
	eventually(t, "move-only destination frame", func() bool { return frames.Load() > before })
	if op, dropped, _ := dropNativeData(w, 50, 50, transfer.TextData("move"), transfer.Copy|transfer.Move); op != transfer.Move || !dropped {
		t.Fatalf("move negotiation: %v, %v", op, dropped)
	}
	if op, dropped, _ := dropNativeData(w, 50, 50, transfer.TextData("copy only"), transfer.Copy); op != transfer.None || dropped {
		t.Fatalf("incompatible effects: %v, %v", op, dropped)
	}
	mygo.RunOnMain(func() { opts = transfer.DropOptions{Formats: []transfer.Format{custom}} })
	w.Invalidate()
	before = frames.Load()
	eventually(t, "custom-only destination", func() bool { return frames.Load() > before })
	broken := transfer.New(transfer.NewItem(transfer.Lazy(custom, func() ([]byte, error) { return nil, errors.New("cannot serialize") })))
	if _, dropped, _ := dropNativeData(w, 50, 50, broken, transfer.Copy); dropped {
		t.Fatal("failed native provider acknowledged a drop")
	}
	if data, _ := files.Files(); !slices.Equal(data, []string{file}) {
		t.Fatal("file transfer changed its source")
	}
}
