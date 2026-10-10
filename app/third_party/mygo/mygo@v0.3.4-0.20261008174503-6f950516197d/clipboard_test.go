package mygo

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

func clipboardRelease(t *testing.T, ch <-chan bool) {
	t.Helper()
	select {
	case main := <-ch:
		if !main {
			t.Fatal("release hook ran off the main thread")
		}
	case <-time.After(time.Second):
		t.Fatal("clipboard providers were not released")
	}
}

func TestClipboardItemsAndNegotiation(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	custom := transfer.Format("application/vnd.mygo.test+json")
	var calls atomic.Int32
	d := transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.HTML, []byte("<b>one</b>")), transfer.Bytes(transfer.Text, []byte("one")), transfer.Lazy(custom, func() ([]byte, error) {
			calls.Add(1)
			if !isMainThread() {
				t.Error("provider off main thread")
			}
			return []byte{1, 2, 0}, nil
		})),
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("two")), transfer.Bytes(custom, []byte{3, 0})),
	)
	if err := Clipboard.Write(d); err != nil {
		t.Fatal(err)
	}
	if f := Clipboard.Formats(); !slices.Equal(f, []transfer.Format{transfer.HTML, transfer.Text, custom}) || calls.Load() != 0 {
		t.Fatalf("discovery: %v, calls %d", f, calls.Load())
	}
	if f, ok := d.Preferred(custom, transfer.HTML); !ok || f != custom || calls.Load() != 0 {
		t.Fatal("negotiation requested bytes")
	}
	if got := Clipboard.ReadText(); got != "one\ntwo" || calls.Load() != 0 {
		t.Fatalf("plain fallback %q, calls %d", got, calls.Load())
	}
	read, err := Clipboard.Read(custom, custom)
	if err != nil || len(read.Items()) != 2 || calls.Load() != 1 {
		t.Fatalf("selected items %d, calls %d, err %v", len(read.Items()), calls.Load(), err)
	}
	for n, want := range [][]byte{{1, 2, 0}, {3, 0}} {
		b, err := read.Items()[n].Read(custom)
		if err != nil || !bytes.Equal(b, want) {
			t.Fatalf("item %d: %v, %v", n, b, err)
		}
		b[0] = 9
	}
	if _, err := Clipboard.ReadFormat(transfer.PNG); !errors.Is(err, transfer.ErrFormat) {
		t.Fatalf("missing format %v", err)
	}
	Clipboard.Clear()
	if b, _ := read.Items()[0].Read(custom); !bytes.Equal(b, []byte{1, 2, 0}) || calls.Load() != 1 {
		t.Fatal("copied read depended on native ownership")
	}
	if err := Clipboard.Write(d); err != nil {
		t.Fatal(err)
	}
	if _, err := Clipboard.ReadFormat(custom); err != nil || calls.Load() != 2 {
		t.Fatalf("fresh write cache: %d, %v", calls.Load(), err)
	}
}

func TestClipboardFilesAndConveniences(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	if err := Clipboard.Write(transfer.New(transfer.Item{}, transfer.NewItem(transfer.Bytes(transfer.Text, []byte("nonempty"))))); err != nil {
		t.Fatal(err)
	}
	if d, err := Clipboard.Read(); err != nil || len(d.Items()) != 1 || Clipboard.ReadText() != "nonempty" {
		t.Fatal("empty items should not reach native pasteboards")
	}
	paths := []string{filepath.Join(t.TempDir(), "space #%.txt"), filepath.Join(t.TempDir(), "日本語.txt")}
	if err := Clipboard.WriteFiles(paths...); err != nil {
		t.Fatal(err)
	}
	if got, err := Clipboard.ReadFiles(); err != nil || !slices.Equal(got, paths) {
		t.Fatalf("file list %q: %v", got, err)
	}
	if err := Clipboard.WriteFiles("relative"); err == nil {
		t.Fatal("relative file path accepted")
	}
	if got, _ := Clipboard.ReadFiles(); !slices.Equal(got, paths) {
		t.Fatal("failed validation changed the clipboard")
	}
	Clipboard.WriteHTML("<b>日本語</b>")
	if Clipboard.ReadHTML() != "<b>日本語</b>" || Clipboard.ReadText() != "<b>日本語</b>" {
		t.Fatal("HTML convenience did not offer HTML and text")
	}
	var pngData bytes.Buffer
	_ = png.Encode(&pngData, image.NewNRGBA(image.Rect(0, 0, 2, 2)))
	if err := Clipboard.WriteImage(pngData.Bytes()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(Clipboard.ReadImage(), pngData.Bytes()) {
		t.Fatal("image convenience changed PNG bytes")
	}
	if err := Clipboard.WriteImage([]byte("broken")); err == nil {
		t.Fatal("invalid PNG accepted")
	}
	if !bytes.Equal(Clipboard.ReadImage(), pngData.Bytes()) {
		t.Fatal("image validation cleared valid data")
	}
	Clipboard.Clear()
	if d, err := Clipboard.Read(); err != nil || len(d.Items()) != 0 {
		t.Fatalf("empty read %v, %v", d.Formats(), err)
	}
	if _, err := Clipboard.ReadFormat(transfer.Text); !errors.Is(err, transfer.ErrFormat) {
		t.Fatal("explicit empty format read should fail")
	}
}

func TestClipboardProviderFailures(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	for _, panics := range []bool{false, true} {
		boom := errors.New("encoding failed")
		var calls, releases atomic.Int32
		d := transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte("fallback")), transfer.Lazy(transfer.HTML, func() ([]byte, error) {
			calls.Add(1)
			if panics {
				panic("encoding panic")
			}
			return []byte("partial"), boom
		})))
		if err := Clipboard.Write(d, ClipboardOptions{OnRelease: func() { releases.Add(1) }}); err != nil {
			t.Fatal(err)
		}
		want := boom
		if panics {
			want = transfer.ErrProviderPanic
		}
		for range 2 {
			b, err := Clipboard.ReadFormat(transfer.HTML)
			if !errors.Is(err, want) || len(b) != 0 {
				t.Fatalf("failed representation %q, %v", b, err)
			}
			if err := Clipboard.Flush(); !errors.Is(err, want) {
				t.Fatalf("failed flush %v", err)
			}
		}
		if calls.Load() != 1 || releases.Load() != 0 || Clipboard.ReadText() != "fallback" {
			t.Fatal("failure lost ownership, escaped, or retried")
		}
		Clipboard.Clear()
		onMain(func() {}) // drain deferred release hook
		if releases.Load() != 1 {
			t.Fatal("failed provider resources were not released exactly once")
		}
	}
}

func TestClipboardOwnershipAndWindowClosure(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	w, _ := testWindow(t, WindowOptions{Content: ui.View(func(c *ui.Context) { ui.Box(c) })})
	ch := make(chan bool, 4)
	var calls atomic.Int32
	d := transfer.New(transfer.NewItem(transfer.Lazy(transfer.Text, func() ([]byte, error) { calls.Add(1); return []byte("application-owned"), nil })))
	if err := Clipboard.Write(d, ClipboardOptions{OnRelease: func() { ch <- isMainThread() }}); err != nil {
		t.Fatal(err)
	}
	w.Destroy()
	if calls.Load() != 0 || len(ch) != 0 {
		t.Fatal("closing a window disposed or requested clipboard data")
	}
	if err := Clipboard.Flush(); err != nil {
		t.Fatal(err)
	}
	clipboardRelease(t, ch)
	if calls.Load() != 1 || Clipboard.ReadText() != "application-owned" {
		t.Fatal("flush lost contents")
	}
	Clipboard.Clear()
	onMain(func() {})
	if len(ch) != 0 {
		t.Fatal("flush and clear released the same source twice")
	}
	if err := Clipboard.Write(d, ClipboardOptions{OnRelease: func() { ch <- isMainThread(); Clipboard.WriteText("release hook") }}); err != nil {
		t.Fatal(err)
	}
	Clipboard.WriteText("replacement")
	clipboardRelease(t, ch)
	if got := Clipboard.ReadText(); got != "release hook" {
		t.Fatalf("reentrant release hook: %q", got)
	}
	if calls.Load() != 1 {
		t.Fatal("replacement invoked an unrequested provider")
	}
}

func TestClipboardFailedWriteAndReservedSession(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	Clipboard.WriteText("keep")
	boom := errors.New("clipboard is busy")
	onMain(func() { fb.ClipboardError = boom })
	var released atomic.Int32
	err := Clipboard.Write(transfer.TextData("failed"), ClipboardOptions{OnRelease: func() { released.Add(1) }})
	onMain(func() { fb.ClipboardError = nil })
	if !errors.Is(err, boom) || Clipboard.ReadText() != "keep" || released.Load() != 0 {
		t.Fatal("failed write changed ownership")
	}
	err = Clipboard.Write(transfer.New(transfer.NewItem(transfer.Bytes(platform.DragSessionFormat, []byte("local")))))
	if err == nil || Clipboard.ReadText() != "keep" {
		t.Fatal("clipboard accepted process-local drag identity")
	}
}

func TestClipboardProviderReentry(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	d := transfer.New(transfer.NewItem(transfer.Lazy(transfer.Text, func() ([]byte, error) {
		if _, err := Clipboard.ReadFormat(transfer.Text); !errors.Is(err, ErrClipboardReentrant) {
			t.Errorf("recursive read: %v", err)
		}
		if err := Clipboard.Write(transfer.TextData("nested")); !errors.Is(err, ErrClipboardReentrant) {
			t.Errorf("recursive write: %v", err)
		}
		if err := Clipboard.Flush(); !errors.Is(err, ErrClipboardReentrant) {
			t.Errorf("recursive flush: %v", err)
		}
		Clipboard.Clear()
		Clipboard.WriteText("nested")
		return []byte("outer"), nil
	})))
	if err := Clipboard.Write(d); err != nil {
		t.Fatal(err)
	}
	if b, err := Clipboard.ReadFormat(transfer.Text); err != nil || string(b) != "outer" {
		t.Fatalf("provider reentry corrupted contents %q: %v", b, err)
	}
}

func TestClipboardReleaseWaitsForNestedStorage(t *testing.T) {
	t.Cleanup(Clipboard.Clear)
	var released atomic.Bool
	if err := Clipboard.Write(transfer.TextData("stored"), ClipboardOptions{OnRelease: func() { released.Store(true); Clipboard.WriteText("next") }}); err != nil {
		t.Fatal(err)
	}
	onMain(func() {
		fb.ClipboardDuringFlush = func() {
			loop.drain() // as a native nested storage loop does
			if released.Load() || Clipboard.ReadText() != "stored" {
				t.Error("release hook interfered with unfinished storage")
			}
		}
	})
	if err := Clipboard.Flush(); err != nil {
		t.Fatal(err)
	}
	onMain(func() { fb.ClipboardDuringFlush = nil })
	if !released.Load() || Clipboard.ReadText() != "next" {
		t.Fatal("release hook did not run safely after flush")
	}
}
