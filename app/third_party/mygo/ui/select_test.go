package ui

import (
	"slices"
	"testing"
)

func TestSelectableText(t *testing.T) {
	const sentence = "Order 12345 shipped"
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(20).Gap(10).AlignItems(Start).Children(func() {
			coreText(c, sentence).Selectable()
			coreText(c, "Plain text")
		})
	}, 400, 200)
	var ed *editor
	for _, s := range tt.rt.states {
		if s.editor != nil && s.editor.readOnly {
			ed = s.editor
		}
	}
	if ed == nil || ed.layout == nil {
		t.Fatal("the selectable text has no editor laid out")
	}
	r, _ := tt.Find(sentence)
	// at returns where rune i of the sentence is, in the window.
	at := func(i int) (float32, float32) {
		x, y, h := ed.layout.Caret(i)
		return r.X + ed.originX + x, r.Y + ed.originY + y + h/2
	}

	// Double-clicking a word selects it, which Cmd+C copies.
	x, y := at(8)
	tt.ClickAt(x, y)
	tt.ClickAt(x, y)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "12345" {
		t.Errorf("a double click copies %q", tt.Clipboard())
	}
	// Dragging selects what it goes over.
	x0, y0 := at(0)
	x1, _ := at(5)
	tt.Press(x0, y0)
	tt.Move(x1, y0)
	tt.Release(x1, y0)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Order" {
		t.Errorf("a drag copies %q", tt.Clipboard())
	}
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != sentence {
		t.Errorf("Cmd+A then Cmd+C copies %q", tt.Clipboard())
	}
	// It cannot be edited.
	tt.Type("x")
	tt.Key(0, KeyBackspace)
	tt.SetClipboard("pasted")
	tt.Key(Cmd, KeyV)
	if !tt.HasText(sentence) || ed.buf.s != sentence {
		t.Errorf("typing changed the text to %q", ed.buf.s)
	}
	if tt.Move(x, y); tt.Cursor() != CursorText {
		t.Errorf("the pointer is %v over selectable text", tt.Cursor())
	}
	tt.RightClickAt(x, y)
	if m := tt.Menu(); !slices.Equal(m, []string{"Copy", "-", "Select All"}) {
		t.Errorf("the context menu is %q", m)
	}
	tt.CloseMenu()

	// A click elsewhere takes the focus, and copying leaves the clipboard.
	if p, ok := tt.Find("Plain text"); ok {
		tt.ClickAt(p.X+p.W/2, p.Y+p.H/2)
	}
	tt.SetClipboard("kept")
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Errorf("Cmd+C away from the selection copies %q", tt.Clipboard())
	}
}
