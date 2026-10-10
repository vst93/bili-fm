package ui

import (
	"github.com/egoist/mygo/internal/platform"
	"strings"
	"testing"
)

// Test-only application state; the framework owns no document buffer.
type primitiveClient struct {
	text             string
	selection        TextInputSelection
	mark             *TextInputRange
	queries          []TextInputRange
	changes, unmarks int
}

func (c *primitiveClient) TextForRange(r TextInputRange) (string, TextInputRange) {
	c.queries = append(c.queries, r)
	a, z := UTF16ByteOffset(c.text, r.Start), UTF16ByteOffset(c.text, r.End)
	return c.text[a:z], TextInputRange{Start: UTF16Len(c.text[:a]), End: UTF16Len(c.text[:z])}
}
func (c *primitiveClient) Selection() TextInputSelection { return c.selection }
func (c *primitiveClient) MarkedRange() (TextInputRange, bool) {
	if c.mark != nil {
		return *c.mark, true
	}
	return TextInputRange{}, false
}
func (c *primitiveClient) rangeFor(r *TextInputRange) TextInputRange {
	if r != nil {
		return *r
	}
	if c.mark != nil {
		return *c.mark
	}
	return c.selection.Range
}
func (c *primitiveClient) replace(r TextInputRange, text string) {
	a, z := UTF16ByteOffset(c.text, r.Start), UTF16ByteOffset(c.text, r.End)
	c.text = c.text[:a] + text + c.text[z:]
	end := r.Start + UTF16Len(text)
	c.selection = TextInputSelection{Range: TextInputRange{Start: end, End: end}}
	c.changes++
}
func (c *primitiveClient) ReplaceText(r *TextInputRange, text string) {
	c.replace(c.rangeFor(r), text)
	c.mark = nil
}
func (c *primitiveClient) SetMarkedText(r *TextInputRange, text string, selected TextInputRange) {
	wanted := c.rangeFor(r)
	c.replace(wanted, text)
	mark := TextInputRange{Start: wanted.Start, End: wanted.Start + UTF16Len(text)}
	c.mark = &mark
	c.selection = TextInputSelection{Range: TextInputRange{Start: mark.Start + selected.Start, End: mark.Start + selected.End}}
}
func (c *primitiveClient) UnmarkText() { c.mark = nil; c.unmarks++ }
func (c *primitiveClient) BoundsForRange(r TextInputRange) (Rect, TextInputRange, bool) {
	return Rect{float32(r.Start) * 4, 5, max(1, float32(r.End-r.Start)*4), 20}, r, true
}
func (c *primitiveClient) IndexForPoint(p Point) (int, bool) {
	return max(0, min(int(p.X/4), UTF16Len(c.text))), true
}
func primitiveTester(c *primitiveClient) *Tester {
	return coreNewTester(func(ctx *context) {
		coreBox(ctx).Size(250, 80).HandleTextInput(c).AutoFocus().HandleInput(func(ev InputEvent) bool { return ev.Kind == InputCommand })
	}, 300, 100)
}

func TestTextInputClientCompositionAndReplacement(t *testing.T) {
	c := &primitiveClient{text: "A😀cafe", selection: TextInputSelection{Range: TextInputRange{Start: 7, End: 7}}}
	tt := primitiveTester(c)
	defer tt.rt.close()
	if tt.h.ime.Client == nil || tt.h.ime.Text != "" {
		t.Fatal("client copied a document snapshot")
	}
	tt.Type("!")
	if c.text != "A😀cafe!" {
		t.Fatal(c.text)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.TextInput, Text: "é", Replace: true, From: 6, To: 7})
	if c.text != "A😀café!" {
		t.Fatal("absolute UTF-16 replacement", c.text)
	}
	c.selection = TextInputSelection{Range: TextInputRange{Start: 6, End: 7}}
	tt.Frame()
	tt.Compose("é", 2)
	if c.text != "A😀café!" || c.mark == nil || *c.mark != (TextInputRange{Start: 6, End: 8}) {
		t.Fatal("preedit", c.text, c.mark)
	}
	tt.Compose("😀", 1)
	if c.selection.Range != (TextInputRange{Start: 8, End: 8}) {
		t.Fatal("caret confused runes with UTF-16", c.selection)
	}
	tt.Type("é")
	if c.text != "A😀café!" || c.mark != nil {
		t.Fatal("commit", c.text)
	}
}

func TestTextInputClientNativeEndBeforeCommit(t *testing.T) {
	c := &primitiveClient{text: "abc", selection: TextInputSelection{Range: TextInputRange{Start: 3, End: 3}}}
	tt := primitiveTester(c)
	defer tt.rt.close()
	client := tt.h.ime.Client
	client.SetMarkedText(nil, "😀", TextInputRange{Start: 2, End: 2})
	var composition platform.InputComposition
	composition.End(client)
	if c.mark != nil || c.text != "abc😀" {
		t.Fatal("unmark removed document content")
	}
	composition.Replace(client, nil, "é")
	if c.text != "abcé" {
		t.Fatal("end-before-commit duplicated preedit", c.text)
	}
	client.SetMarkedText(nil, "候補", TextInputRange{Start: 2, End: 2})
	composition.End(client)
	composition.Reset() // the native client was switched or reset
	composition.Replace(client, nil, "!")
	if c.text != "abcé候補!" {
		t.Fatal("stale commit range survived reset", c.text)
	}
}

func TestTextInputClientQueriesGeometryAndLifetime(t *testing.T) {
	c := &primitiveClient{text: strings.Repeat("a", 10000), selection: TextInputSelection{Range: TextInputRange{Start: 8000, End: 8000}}}
	show := true
	tt := coreNewTester(func(ctx *context) {
		coreColumn(ctx).Padding(10).Children(func() {
			if show {
				coreBox(ctx).Size(200, 40).HandleTextInput(c).AutoFocus()
			}
			coreButton(ctx, "Other")
		})
	}, 300, 100)
	defer tt.rt.close()
	client := tt.h.ime.Client
	text, actual := client.TextForRange(TextInputRange{Start: 6000, End: 6004})
	if text != "aaaa" || actual != (TextInputRange{Start: 6000, End: 6004}) {
		t.Fatal("query clipped to IME context")
	}
	if b, used, ok := client.BoundsForRange(TextInputRange{Start: 3, End: 5}); !ok || b.X != 22 || b.Y != 15 || used != (TextInputRange{Start: 3, End: 5}) {
		t.Fatal("element-to-surface geometry", b, used, ok)
	}
	if index, ok := client.IndexForPoint(26, 15); !ok || index != 4 {
		t.Fatal("surface-to-element hit test", index, ok)
	}
	c.mark = &TextInputRange{Start: 3, End: 4}
	tt.Click("Other")
	if c.mark != nil || c.unmarks != 1 {
		t.Fatal("focus loss composition cleanup", c.unmarks)
	}
	before := c.changes
	client.ReplaceText(nil, "stale")
	if c.changes != before {
		t.Fatal("unfocused client mutated")
	}
	show = false
	tt.Frame()
	client.ReplaceText(nil, "removed")
	if c.changes != before {
		t.Fatal("disposed client mutated")
	}
}

func TestTextInputClientSwappingAndTypedNil(t *testing.T) {
	first, second := &primitiveClient{text: "one"}, &primitiveClient{text: "two"}
	current := first
	tt := coreNewTester(func(ctx *context) { coreBox(ctx).Size(200, 50).HandleTextInput(current).AutoFocus() }, 250, 60)
	defer tt.rt.close()
	old := tt.h.ime.Client
	current = second
	tt.Frame()
	old.ReplaceText(nil, "wrong")
	if first.changes != 0 || second.changes != 0 {
		t.Fatal("old callback targeted replacement")
	}
	tt.Type("new")
	if second.text != "newtwo" {
		t.Fatal(second.text)
	}
	current = nil
	tt.Frame()
	if tt.h.ime.Active {
		t.Fatal("typed nil did not disconnect")
	}
}

func TestTextInputSurroundingUnicodeAndReversedSelection(t *testing.T) {
	c := &primitiveClient{text: "A😀bc", selection: TextInputSelection{Range: TextInputRange{Start: 3, End: 5}, Reversed: true}}
	tt := primitiveTester(c)
	defer tt.rt.close()
	ctx := platform.ClientTextContext(tt.h.ime.Client)
	if ctx.Caret != 3 || UTF16ByteOffset(ctx.Text, ctx.Caret) != 5 {
		t.Fatal("reversed caret", ctx)
	}
	if got := platform.ClientDeleteRange(tt.h.ime.Client, -1, 1); got != (TextInputRange{Start: 1, End: 3}) {
		t.Fatal("GTK delete split surrogate", got)
	}
	c.text = strings.Repeat("a", 6000) + "😀"
	c.selection = TextInputSelection{Range: TextInputRange{Start: 6002, End: 6002}}
	if got := platform.ClientDeleteRange(tt.h.ime.Client, -5000, 5000); got != (TextInputRange{Start: 1001, End: 6002}) {
		t.Fatal("large delete clipped", got)
	}
}

func TestTextLayoutUTF16GraphemesAndBidiRanges(t *testing.T) {
	l := ShapeText("é😀", Font{Size: 18}, 0)
	if l.Caret(1) != l.Caret(0) || l.Caret(3) != l.Caret(2) {
		t.Fatal("caret split grapheme or surrogate")
	}
	for _, index := range []int{0, 2, 4} {
		r := l.Caret(index)
		if got := l.IndexAt(Point{r.X, r.Y + r.H/2}); got != index {
			t.Fatal("caret hit test", index, got)
		}
	}
	bidi := ShapeText("abc אבג def", Font{Size: 18}, 0)
	rects := bidi.SelectionRects(TextInputRange{Start: 4, End: 6}, TextInputRange{Start: 7, End: 8})
	if len(rects) < 2 {
		t.Fatal("bidi range set collapsed", rects)
	}
	for _, r := range rects {
		if r.W <= 0 || r.H <= 0 {
			t.Fatal("invalid selection rectangle", r)
		}
	}
}

func TestTextLayoutVisualCaretUTF16(t *testing.T) {
	l := ShapeText("😀 abc אבג def", Font{Size: 18}, 0)
	up := l.CaretAt(TextCaretPosition{Index: 7, Affinity: TextUpstream})
	down := l.CaretAt(TextCaretPosition{Index: 7, Affinity: TextDownstream})
	if up.X == down.X {
		t.Fatal("lost secondary bidi caret")
	}
	p := TextCaretPosition{Index: 9}
	for _, index := range []int{8, 7, 11} {
		p = l.MoveCaret(p, 1)
		if p.Index != index {
			t.Fatalf("visual UTF-16 navigation %+v, want %d", p, index)
		}
	}
	ranges := l.SelectionRanges(TextCaretPosition{Index: 9}, p)
	if len(ranges) != 2 || ranges[0] != (TextInputRange{Start: 7, End: 9}) || ranges[1] != (TextInputRange{Start: 10, End: 11}) {
		t.Fatal("visual UTF-16 ranges", ranges)
	}
	position := l.PositionAt(Point{X: up.X, Y: up.Y + up.H/2})
	if got := l.CaretAt(position); got.X != up.X || got.Y != up.Y {
		t.Fatalf("hit-test lost visual edge: %+v %+v", up, got)
	}
}
