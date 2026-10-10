package ui

import (
	"runtime"
	"slices"
	"strings"
	"testing"
	"unsafe"
	"weak"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/text"
)

func inputTester(value *string, multiline bool) *Tester {
	return coreNewTester(func(c *context) { textInput(c, value, multiline).Fill().AutoFocus() }, 300, 120)
}

func TestWidgetInputUsesClientBetweenFrames(t *testing.T) {
	s := "A😀cafe"
	tt := inputTester(&s, false)
	defer tt.rt.close()
	c := tt.h.ime.Client
	if c == nil || tt.h.ime.Text != "" {
		t.Fatal("widget retained a snapshot input path")
	}
	tt.rt.event(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: platform.KeyLeft})
	c.ReplaceText(nil, "!")
	got, _ := c.TextForRange(TextInputRange{End: 100})
	if got != "A😀caf!e" {
		t.Fatalf("queued key not visible to native input: %q", got)
	}
	tt.Frame()
	if s != got {
		t.Fatalf("bound value %q, native document %q", s, got)
	}
	c.ReplaceText(&TextInputRange{Start: 1, End: 3}, "é")
	tt.Frame()
	if s != "Aécaf!e" {
		t.Fatal("UTF-16 replacement split a surrogate", s)
	}
}

func TestWidgetCompositionHasOneUndoTransaction(t *testing.T) {
	s := "cafe"
	tt := inputTester(&s, false)
	defer tt.rt.close()
	c := tt.h.ime.Client
	c.SetMarkedText(&TextInputRange{Start: 3, End: 4}, "😀", TextInputRange{Start: 2, End: 2})
	tt.Frame()
	if s != "caf" || c.Selection().Caret() != 5 {
		t.Fatalf("preedit value %q selection %+v", s, c.Selection())
	}
	doc, actual := c.TextForRange(TextInputRange{Start: 3, End: 5})
	if doc != "😀" || actual != (TextInputRange{Start: 3, End: 5}) {
		t.Fatalf("preedit query %q %+v", doc, actual)
	}
	var order platform.InputComposition
	order.End(c)
	order.Replace(c, nil, "é")
	tt.Frame()
	if s != "café" {
		t.Fatal("end-before-commit duplicated preedit", s)
	}
	tt.Key(Cmd, KeyZ)
	if s != "cafe" {
		t.Fatal("composition was not one undo step", s)
	}
	tt.Key(Cmd|Shift, KeyZ)
	if s != "café" {
		t.Fatal("composition redo", s)
	}
}

func TestWidgetInputPrivacyAndReadonly(t *testing.T) {
	s := "secret😀"
	readonly := false
	tt := coreNewTester(func(c *context) { coreTextInput(c, &s).Fill().AutoFocus().Password().ReadOnly(readonly) }, 300, 100)
	defer tt.rt.close()
	c := tt.h.ime.Client
	if got, _ := c.TextForRange(TextInputRange{End: 1000}); got != "" {
		t.Fatalf("password exposed %q", got)
	}
	c.ReplaceText(nil, "!")
	tt.Frame()
	if s != "secret😀!" {
		t.Fatal(s)
	}
	readonly = true
	tt.Frame()
	if tt.h.ime.Active {
		t.Fatal("readonly input enabled IME")
	}
	c.ReplaceText(nil, "wrong")
	tt.Frame()
	if s != "secret😀!" {
		t.Fatal("stale callback edited readonly input", s)
	}
}

func TestWidgetBidiSelectionCopyReplaceAndUndo(t *testing.T) {
	for _, multiline := range []bool{false, true} {
		s := "abc אבג def"
		tt := inputTester(&s, multiline)
		ed := tt.rt.states[tt.rt.focused].editor
		ed.move(6, false)
		tt.Frame()
		for range 3 {
			tt.Key(Shift, KeyRight)
		}
		if !slices.Equal(ed.selectedRanges(), []text.Range{{Start: 4, End: 6}, {Start: 7, End: 8}}) {
			t.Fatalf("selection %v", ed.selectedRanges())
		}
		tt.Key(Cmd, KeyC)
		if tt.h.clipboard != "אב " {
			t.Fatalf("copied unselected text %q", tt.h.clipboard)
		}
		tt.Type("X")
		if s != "abc Xגdef" {
			t.Fatalf("replaced unselected text %q", s)
		}
		tt.Key(Cmd, KeyZ)
		if s != "abc אבג def" || ed.selectedText() != "אב " {
			t.Fatalf("undo lost text or selection: %q %v", s, ed.selectedRanges())
		}
		tt.h.ime.Client.SetMarkedText(nil, "候補", TextInputRange{Start: 2, End: 2})
		tt.Frame()
		if s != "abc גdef" {
			t.Fatalf("composition removed a bidi gap: %q", s)
		}
		tt.h.ime.Client.ReplaceText(nil, "é")
		tt.Frame()
		if s != "abc éגdef" {
			t.Fatalf("composition commit: %q", s)
		}
		tt.Key(Cmd, KeyZ)
		if s != "abc אבג def" || ed.selectedText() != "אב " {
			t.Fatal("composition undo lost visual ranges")
		}
		tt.rt.close()
	}
}

func queryOldDocument() (*Tester, *string, weak.Pointer[byte], string) {
	s := strings.Repeat("short line\n", 10000)
	tt := inputTester(&s, true)
	old := weak.Make(unsafe.StringData(s))
	query, _ := tt.h.ime.Client.TextForRange(TextInputRange{Start: 1, End: 2})
	tt.h.ime.Client.ReplaceText(&TextInputRange{Start: 1, End: 2}, "")
	tt.Frame()
	return tt, &s, old, query
}

func TestWidgetNativeQueryDoesNotRetainOldDocument(t *testing.T) {
	tt, value, old, query := queryOldDocument()
	defer tt.rt.close()
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("bounded native query retained an old document")
	}
	if query != "h" {
		t.Fatal(query)
	}
	runtime.KeepAlive(tt)
	runtime.KeepAlive(value)
	runtime.KeepAlive(query)
}

func singleLineOldDocument() (*Tester, *string, weak.Pointer[byte]) {
	s := strings.Repeat("a", 64<<10)
	tt := inputTester(&s, false)
	old := weak.Make(unsafe.StringData(s))
	tt.h.ime.Client.ReplaceText(&TextInputRange{Start: 10, End: 11}, "")
	tt.Frame()
	return tt, &s, old
}

func TestWidgetSingleLineLayoutReleasesOldDocument(t *testing.T) {
	tt, value, old := singleLineOldDocument()
	defer tt.rt.close()
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("single-line layout cache retained an old document")
	}
	runtime.KeepAlive(tt)
	runtime.KeepAlive(value)
}

func TestWidgetNativeQueryMalformedUTF8(t *testing.T) {
	s := "\xffbad\xf0\x9f"
	tt := inputTester(&s, true)
	defer tt.rt.close()
	doc, actual := tt.h.ime.Client.TextForRange(TextInputRange{End: 100})
	if doc != "�bad��" || actual.End != 6 {
		t.Fatalf("native query %q %+v", doc, actual)
	}
}
