package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func bufferTester(value *TextBuffer, multiline bool) *Tester {
	return coreNewTester(func(c *context) { bufferInputBase(c, value, multiline).Fill().AutoFocus() }, 400, 140)
}

func TestBufferInputsEditComposeAndUndo(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		b := NewTextBuffer("A😀cafe")
		tt := bufferTester(b, multiline)
		c := tt.h.ime.Client
		c.ReplaceText(&TextInputRange{Start: 6, End: 7}, "é")
		tt.Frame()
		if b.String() != "A😀café" {
			t.Fatal("indexed replacement", b.String())
		}
		c.SetMarkedText(&TextInputRange{Start: 6, End: 7}, "😀", TextInputRange{Start: 2, End: 2})
		tt.Frame()
		if b.String() != "A😀caf" {
			t.Fatal("preedit in committed buffer", b.String())
		}
		var order platform.InputComposition
		order.End(c)
		order.Replace(c, nil, "e")
		tt.Frame()
		if b.String() != "A😀cafe" {
			t.Fatal("commit", b.String())
		}
		tt.Key(Cmd, KeyZ)
		if b.String() != "A😀café" {
			t.Fatal("composition undo", b.String())
		}
		tt.Key(Cmd, KeyZ)
		if b.String() != "A😀cafe" {
			t.Fatal("replacement undo", b.String())
		}
		tt.Key(Cmd|Shift, KeyZ)
		if b.String() != "A😀café" {
			t.Fatal("redo", b.String())
		}
		tt.rt.close()
	}
}

func TestBufferAreaNativeQueriesAndLineEditing(t *testing.T) {
	b := NewTextBuffer(strings.Repeat("line😀\n", 30000) + "tail")
	tt := bufferTester(b, true)
	defer tt.rt.close()
	ed := tt.rt.states[tt.rt.focused].editor
	if ed.buf.s != "" || !ed.buf.indexed {
		t.Fatal("buffer control materialized the document")
	}
	c := tt.h.ime.Client
	end := b.UTF16Len()
	c.ReplaceText(&TextInputRange{Start: end - 4, End: end}, "last\nnext")
	tt.Frame()
	if b.Snapshot().Line(30000) != "last" || b.Snapshot().Line(30001) != "next" {
		t.Fatal("newline indexes")
	}
	c.ReplaceText(&TextInputRange{Start: 0, End: 4}, "first")
	tt.Frame()
	if b.Snapshot().Line(0) != "first😀" || b.Snapshot().Line(20000) != "line😀" {
		t.Fatal("later paragraph positions")
	}
	query, r := c.TextForRange(TextInputRange{Start: b.UTF16Len() - 4, End: b.UTF16Len()})
	if query != "next" || r.End != b.UTF16Len() {
		t.Fatal("late document query", query, r)
	}
	tt.Key(Cmd, KeyA)
	tt.Key(0, KeyBackspace)
	if b.Len() != 0 || b.LineCount() != 1 {
		t.Fatal("delete all")
	}
	tt.Key(Cmd, KeyZ)
	if b.Snapshot().Line(0) != "first😀" {
		t.Fatal("large delete undo")
	}
}

func TestBufferInputBidiReplacementAndExternalChange(t *testing.T) {
	b := NewTextBuffer("abc אבג def")
	tt := bufferTester(b, true)
	defer tt.rt.close()
	ed := tt.rt.states[tt.rt.focused].editor
	ed.move(6, false)
	tt.Frame()
	for range 3 {
		tt.Key(Shift, KeyRight)
	}
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "אב " {
		t.Fatal(tt.Clipboard())
	}
	tt.Type("X")
	if b.String() != "abc Xגdef" {
		t.Fatal("bidi replacement", b.String())
	}
	b.Replace(0, b.Len(), "external😀")
	tt.Frame()
	if ed.buf.n != b.Len() || len(ed.undo) != 0 {
		t.Fatal("external edit did not reset stale widget state")
	}
	tt.Key(0, KeyEnd)
	tt.Type("!")
	if b.String() != "external😀!" {
		t.Fatal("edit after external change", b.String())
	}
}

func TestBufferInputBindingSwapAndStringCompatibility(t *testing.T) {
	b := NewTextBuffer("buffer")
	s := "string"
	indexed := true
	tt := coreNewTester(func(c *context) {
		if indexed {
			coreTextInputBuffer(c, b).AutoFocus()
		} else {
			coreTextInput(c, &s).AutoFocus()
		}
	}, 300, 100)
	defer tt.rt.close()
	indexed = false
	tt.Frame()
	tt.Key(0, KeyEnd)
	tt.Type("!")
	if s != "string!" || b.String() != "buffer" {
		t.Fatal("binding swap changed the other document")
	}
}
