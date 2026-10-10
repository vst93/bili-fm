package text

import (
	"slices"
	"testing"
)

// Fixed advances keep geometry regressions independent of installed fonts.
func mixedLayout() *Layout {
	runes := []rune("abc אבג def")
	l := &Layout{Runes: runes, Lines: []Line{{Start: 0, End: len(runes), Width: 110, Height: 20}}}
	for i := range runes {
		x, rtl := float32(i*10), i >= 4 && i < 7
		if rtl {
			x = float32((10 - i) * 10)
		}
		l.Lines[0].Glyphs = append(l.Lines[0].Glyphs, Glyph{X: x, Advance: 10, Cluster: i, Runes: 1, RTL: rtl})
	}
	return l
}

func TestVisualCaretAffinityAndNavigation(t *testing.T) {
	l := mixedLayout()
	a, _, _ := l.CaretAt(CaretPosition{4, Upstream})
	b, _, _ := l.CaretAt(CaretPosition{4, Downstream})
	if a != 40 || b != 70 {
		t.Fatalf("bidi edges %v %v", a, b)
	}
	p := CaretPosition{3, Downstream}
	for _, want := range []CaretPosition{{4, Upstream}, {6, Downstream}, {5, Downstream}, {4, Downstream}, {8, Upstream}} {
		p = l.MoveCaret(p, 1)
		if p != want {
			t.Fatalf("right: got %+v want %+v", p, want)
		}
	}
	for _, want := range []CaretPosition{{7, Downstream}, {5, Upstream}, {6, Upstream}, {7, Upstream}, {3, Downstream}} {
		p = l.MoveCaret(p, -1)
		if p != want {
			t.Fatalf("left: got %+v want %+v", p, want)
		}
	}
}

func TestVisualSelectionKeepsBidiGaps(t *testing.T) {
	l := mixedLayout()
	ranges := l.SelectionRanges(CaretPosition{6, Downstream}, CaretPosition{8, Upstream})
	if !slices.Equal(ranges, []Range{{4, 6}, {7, 8}}) {
		t.Fatalf("selection %v", ranges)
	}
	var rects []Rect
	for _, r := range ranges {
		rects = append(rects, l.SelectionVisual(r.Start, r.End, false)...)
	}
	if len(rects) != 2 || rects[0].X != 50 || rects[1].X != 70 {
		t.Fatalf("bidi rectangles %v", rects)
	}
}

func TestVisualCaretFractionalRunEdges(t *testing.T) {
	l := mixedLayout()
	l.Lines[0].Glyphs[4].Advance += 0.00001
	p := l.MoveCaret(CaretPosition{Index: 5}, 1)
	if p != (CaretPosition{Index: 4, Affinity: Downstream}) {
		t.Fatalf("rounding selected the wrong run edge: %+v", p)
	}
}

func TestVisualCaretWrapAndGraphemes(t *testing.T) {
	l := &Layout{Runes: []rune("é😀x"), Lines: []Line{
		{Start: 0, End: 3, Width: 20, Height: 20, Glyphs: []Glyph{{Advance: 10, Cluster: 0, Runes: 2}, {X: 10, Advance: 10, Cluster: 2, Runes: 1}}},
		{Start: 3, End: 4, Y: 20, Width: 10, Height: 20, Glyphs: []Glyph{{Advance: 10, Cluster: 3, Runes: 1}}},
	}}
	p := l.MoveCaret(CaretPosition{}, 1)
	if p.Index != 2 {
		t.Fatalf("split grapheme %+v", p)
	}
	_, up, _ := l.CaretAt(CaretPosition{3, Upstream})
	_, down, _ := l.CaretAt(CaretPosition{3, Downstream})
	if up != 0 || down != 20 {
		t.Fatalf("wrap affinity %v %v", up, down)
	}
	p = l.PositionAt(20, 10)
	if p != (CaretPosition{3, Upstream}) {
		t.Fatalf("wrap hit %+v", p)
	}
	p = l.MoveCaret(p, 1)
	if p.Index != 3 || p.Affinity != Downstream {
		t.Fatalf("wrap movement %+v", p)
	}
}

// A trimmed wrap space shares the last visible glyph's edge. Choosing the
// later offset on a tie would copy whitespace the pointer never crossed.
func TestVisualCaretHitBeforeTrimmedWrapSpace(t *testing.T) {
	l := &Layout{Runes: []rune("ab cd"), Lines: []Line{
		{Start: 0, End: 3, Width: 20, Height: 20, Glyphs: []Glyph{{Advance: 10, Cluster: 0, Runes: 1}, {X: 10, Advance: 10, Cluster: 1, Runes: 1}}},
		{Start: 3, End: 5, Y: 20, Width: 20, Height: 20, Glyphs: []Glyph{{Advance: 10, Cluster: 3, Runes: 1}, {X: 10, Advance: 10, Cluster: 4, Runes: 1}}},
	}}
	x, y, h := l.Caret(2)
	if p := l.PositionAt(x, y+h/2); p != (CaretPosition{Index: 2, Affinity: Upstream}) {
		t.Fatalf("hit before trimmed space selected %+v", p)
	}
}
