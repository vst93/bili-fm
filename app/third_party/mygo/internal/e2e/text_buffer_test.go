package e2e

import (
	"runtime"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestContentWindowTextBufferInputMethod(t *testing.T) {
	var frames atomic.Int32
	doc := ui.NewTextBuffer("first\ncafe")
	w := newWindow(t, mygo.WindowOptions{Title: "Indexed input", Width: 400, Height: 200, Content: ui.View(func(c *ui.Context) {
		ui.Column(c).Fill().Padding(20).Children(func() { ui.TextAreaBuffer(c, doc).Height(120).AutoFocus() })
		frames.Add(1)
	})})
	eventually(t, "a buffer frame", func() bool { return frames.Load() > 0 })
	if _, _, ok := inputClient(w); !ok {
		t.Skip("native input automation unavailable")
	}
	if !click(w, 300, 56) {
		t.Skip("click automation unavailable")
	}
	eventually(t, "buffer caret", func() bool { sel, text, _ := inputClient(w); return sel == [2]int{10, 0} && text == "first\ncafe" })
	composeOver(w, "e", 1, false, 9, 1)
	expected := "first\ncafe"
	if runtime.GOOS != "darwin" {
		expected = "first\ncaf"
	}
	eventually(t, "buffer preedit", func() bool { _, text, _ := inputClient(w); return doc.String() == "first\ncaf" && text == expected })
	composeOver(w, "é", 1, true, -1, 0)
	eventually(t, "buffer commit", func() bool { return doc.String() == "first\ncafé" })
}
