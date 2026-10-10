package ui

import (
	"image"
	"runtime"
	"strings"
	"testing"
	"time"
	"unsafe"
	"weak"

	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

// Removed elements must give up their resources while the window stays
// alive, even when the next frame has fewer elements and drawing ops.
func TestRemovedContentReleasesResources(t *testing.T) {
	for _, exits := range []bool{false, true} {
		t.Run(map[bool]string{false: "plain", true: "exit"}[exits], func(t *testing.T) {
			show := true
			var bitmap weak.Pointer[Bitmap]
			var pixels weak.Pointer[scene.Image]
			tt, now := clockTester(func(c *context) {
				coreColumn(c).Children(func() {
					coreText(c, "Keep the window alive")
					if !show {
						return
					}
					e := coreBox(c).Key("photo")
					if exits {
						e.Transition(ElementTransition{Exit: &Motion{}, Duration: 100 * time.Millisecond})
					}
					b := *coreLocal(e, "bitmap", func() *Bitmap {
						return NewBitmap(image.NewRGBA(image.Rect(0, 0, 64, 64)))
					})
					bitmap, pixels = weak.Make(b), weak.Make(b.img)
					e.Children(func() { coreImage(c, b).Size(64, 64) })
				})
			}, 200, 150)
			show = false
			tt.Frame()
			if exits {
				runtime.GC()
				if bitmap.Value() == nil || pixels.Value() == nil {
					t.Fatal("the exit transition lost the image before it finished")
				}
				*now = now.Add(150 * time.Millisecond)
				tt.Frame()
			}
			runtime.GC()
			if bitmap.Value() != nil || pixels.Value() != nil {
				t.Fatal("the removed view still retains its bitmap or pixels")
			}
			runtime.KeepAlive(tt)
		})
	}
}

// A short-lived large view must not leave its entire element arena in a
// small window. A few rows may leave and enter without fresh allocations.
func TestElementArenaFollowsViewSize(t *testing.T) {
	rows := 1000
	tt := coreNewTester(func(c *context) {
		for i := range rows {
			coreBox(c).Key(i).Height(1)
		}
	}, 100, 100)
	large := len(tt.rt.c.chunks)
	rows = 1
	tt.Frame()
	if n := len(tt.rt.c.chunks); n > 2 || n >= large {
		t.Fatalf("the small view kept %d chunks of the large view's %d", n, large)
	}
	if n := testing.AllocsPerRun(20, tt.Frame); n > 2 {
		t.Fatalf("the steady view allocates %.0f times per frame", n)
	}
}

func editorWithLargeText() (*editor, weak.Pointer[byte]) {
	ed := newEditor()
	ed.multiline = true
	ed.buf.set(strings.Repeat("a", 64<<10))
	return ed, weak.Make(unsafe.StringData(ed.buf.s))
}

// A deletion's undo holds its removed bytes, not every old document that
// contains them. Undo and redo must still restore the exact text.
func TestEditorUndoReleasesOldDocument(t *testing.T) {
	ed, old := editorWithLargeText()
	ed.deleteRange(10, 11)
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("a one-character deletion keeps the old document alive")
	}
	ed.takeBack(false)
	if ed.buf.s != strings.Repeat("a", 64<<10) {
		t.Fatal("undo did not restore the document")
	}
	ed.takeBack(true)
	if len(ed.buf.s) != (64<<10)-1 {
		t.Fatal("redo did not restore the deletion")
	}
	runtime.KeepAlive(ed)
}

func editorWithLargeRedo() (*editor, weak.Pointer[byte]) {
	ed := newEditor()
	ed.multiline = true
	ed.buf.set("small")
	s := strings.Repeat("x", 64<<10)
	old := weak.Make(unsafe.StringData(s))
	ed.insert(s)
	ed.takeBack(false)
	return ed, old
}

func TestEditorDiscardedRedoReleasesDocument(t *testing.T) {
	ed, old := editorWithLargeRedo()
	ed.insert("z")
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("discarded redo still keeps its document alive")
	}
	if len(ed.redo) != 0 {
		t.Fatal("an edit did not discard redo")
	}
	runtime.KeepAlive(ed)
}

func editorWithLargeIndexes() (*editor, weak.Pointer[paragraph], weak.Pointer[float64]) {
	ed := newEditor()
	ed.buf.set(strings.Repeat("a\n", 10000))
	ed.area = &area{}
	ed.area.hs.reset(ed.buf.paras)
	return ed, weak.Make(&ed.buf.paras[0]), weak.Make(&ed.area.hs.measured[0])
}

func TestEditorSmallDocumentReleasesIndexes(t *testing.T) {
	ed, paragraphs, heights := editorWithLargeIndexes()
	ed.setText("small\nfile")
	ed.area.hs.reset(ed.buf.paras)
	runtime.GC()
	if paragraphs.Value() != nil || heights.Value() != nil {
		t.Fatal("the small document keeps the old paragraph or height index")
	}
	runtime.KeepAlive(ed)
}

func editorWithParagraphLayout() (*editor, weak.Pointer[byte]) {
	ed := newEditor()
	ed.multiline = true
	ed.buf.set(strings.Repeat("short line\n", 10000))
	ed.area = &area{params: text.Params{Style: text.Style{Size: 14}}, version: ed.buf.version}
	ed.area.hs.reset(ed.buf.paras)
	ed.area.paraLayout(ed, 5)
	return ed, weak.Make(unsafe.StringData(ed.buf.s))
}

func TestEditorParagraphLayoutReleasesOldDocument(t *testing.T) {
	ed, old := editorWithParagraphLayout()
	ed.deleteRange(1, 2)
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("an unchanged paragraph's layout keeps the old document alive")
	}
	if ed.buf.paras[5].layout == nil {
		t.Fatal("the edit discarded the unchanged paragraph's layout")
	}
	runtime.KeepAlive(ed)
}
