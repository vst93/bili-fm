package ui

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

func (ed *editor) lineHeight() float32 {
	if ed.area != nil && ed.area.line.Height > 0 {
		return ed.area.line.Height
	}
	if ed.layout != nil && len(ed.layout.Lines) > 0 {
		return ed.layout.Lines[0].Height
	}
	return 18
}

// caretRect returns the caret's box relative to the surface, where input
// methods show their candidates.
func (ed *editor) caretRect(st *state) Rect {
	if a := ed.area; a != nil && a.version != 0 {
		x, y, h := a.caretAt(ed, ed.caret, ed.composeCaret)
		return Rect{st.x + ed.originX + x, st.y + ed.originY + float32(y-a.scroll), 1, h}
	}
	if ed.layout == nil {
		return Rect{st.x, st.y, 1, st.h}
	}
	x, y, h := ed.layout.CaretAt(text.CaretPosition{Index: ed.displayIndex(ed.caret) + ed.composeCaret, Affinity: ed.caretAffinity})
	return Rect{st.x + ed.originX + x - ed.scrollX, st.y + ed.originY + y, 1, h}
}

// displayText returns what the input shows: the text with the
// composition at the caret, bullets for a password.
func (ed *editor) displayText() string {
	t := ed.buf.string()
	if ed.compose != "" {
		at := runeOffset(t, ed.caret)
		t = t[:at] + ed.compose + t[at:]
	}
	if ed.password {
		return strings.Repeat("•", utf8.RuneCountInString(t))
	}
	return t
}

func (e *node) inputParams(width float32) text.Params {
	p := e.textParams(width)
	ed := e.st.editor
	p.Text = ed.displayText()
	if !ed.password && ed.compose == "" {
		p.Spans, _ = ed.rangeSpans(0, ed.buf.n)
	}
	p.KeepSpaces = true
	p.MaxLines = 0
	if !e.st.editor.multiline {
		p.Width = 0
	}
	return p
}

// Each control owns its current layout. Caching changing input strings in
// the system's display-text cache would retain old documents across edits.
func (ed *editor) shapeInput(params text.Params) *text.Layout {
	gen := textSystem().Generation()
	if ed.layout == nil || ed.layout.Params != params || ed.layoutGeneration != gen {
		ed.layout = textSystem().Shape(params)
		ed.layoutGeneration = gen
		ed.layoutVersion, ed.layoutCompose, ed.layoutPassword = ed.buf.version, ed.compose, ed.password
	}
	return ed.layout
}

func (e *node) inputHeight(cw float32) float32 {
	ed := e.st.editor
	if ed.area != nil {
		p := e.textParams(0)
		p.Text = ""
		line := textSystem().Layout(p).Lines[0].Height
		if lo, hi := ed.lines[0], ed.lines[1]; hi > 0 {
			// As high as its text wrapped at the width it gets, between its
			// least and most lines.
			h := ed.area.wrappedHeight(e, ed, cw)
			return min(max(h, float32(lo)*line), float32(hi)*line)
		}
		// As high as its paragraphs unwrapped, without laying them out.
		return float32(max(len(ed.buf.paras), 3)) * line
	}
	l := ed.shapeInput(e.inputParams(0))
	return l.Lines[0].Height
}

func (e *node) layoutInput(cw, ch float32) {
	ed := e.st.editor
	if a := ed.area; a != nil {
		ed.contentW = cw
		ed.originX, ed.originY = e.contentX(), e.contentY()
		a.layout(e, cw, ch)
		return
	}
	l := ed.shapeInput(e.inputParams(cw))
	ed.layout = l
	ed.layoutVersion, ed.layoutCompose, ed.layoutPassword = ed.buf.version, ed.compose, ed.password
	ed.contentW = cw
	ed.originX, ed.originY = e.contentX(), e.contentY()
	// A single line, centered vertically in a taller box: text areas lay
	// out in area.
	if len(l.Lines) > 0 {
		ed.originY += max((ch-l.Lines[0].Height)/2, 0)
	}
	// Text narrower than the box goes where TextAlign puts it.
	if room := cw - l.Width; room > 0 {
		switch e.resolvedText().align {
		case End:
			ed.originX += room
		case Center:
			ed.originX += room / 2
		}
	}
	// Keep the caret in view while the input has the focus. Without it, the
	// input shows the start of its text, as fields do on macOS and the web.
	if e.c.rt.focused != e.id {
		ed.scrollX = 0
	} else if x, _, _ := l.Caret(ed.displayIndex(ed.caret) + ed.composeCaret); x-ed.scrollX < 0 {
		ed.scrollX = x
	} else if x-ed.scrollX > cw-1 {
		ed.scrollX = x - cw + 1
	}
	ed.scrollX = max(0, min(ed.scrollX, max(l.Width-cw+1, 0)))
}

// placeholderParams lays out the placeholder of an empty input in a
// content box width wide: in the input's style, its line height too, fixed
// or not. A text area's placeholder wraps; a single-line input's stays on
// its line, cut off at the box.
func (e *node) placeholderParams(width float32) text.Params {
	ed := e.st.editor
	params := e.textParams(width)
	params.Text, params.Spans, params.MaxLines, params.NoWrap, params.Ellipsis = ed.placeholder, "", 0, !ed.multiline, ""
	return params
}

func (e *node) paintInput(p *Painter) {
	ed := e.st.editor
	l := ed.layout
	t := e.c.theme
	if l == nil && ed.area == nil {
		return
	}
	box := e.contentBox()
	clip := Rect{e.x + e.border[3], e.y + e.border[0], e.w - e.border[1] - e.border[3], e.h - e.border[0] - e.border[2]}
	saved := p.clip
	p.pushClip(clip, [4]float32{})
	ox, oy := e.x+ed.originX-ed.scrollX, e.y+ed.originY
	focused := e.Focused()
	ts := e.resolvedText()
	if ed.buf.n == 0 && ed.compose == "" && ed.placeholder != "" {
		pl := textSystem().Layout(e.placeholderParams(box.W))
		// The placeholder aligns itself in the content box, as its layout has the box's width.
		p.textLayout(pl, e.x+e.contentX(), oy, t.TextMuted, ts, nil)
	}
	if ed.area != nil {
		ed.area.paint(e, p, e.x+ed.originX, e.y+ed.originY)
		p.popClip()
		p.clip = saved
		return
	}
	if focused {
		for _, selected := range ed.selectedRanges() {
			for _, r := range l.SelectionVisual(ed.displayIndex(selected.Start), ed.displayIndex(selected.End), false) {
				p.Fill(Rect{ox + r.X, oy + r.Y, r.W, r.H}, ts.selectionColor(t), 0)
			}
		}
	}
	var sp *spanPaint
	if !ed.password && ed.compose == "" {
		_, sp = ed.rangeSpans(0, ed.buf.n)
	}
	p.textLayout(l, ox, oy, ts.color, ts, sp)
	if ed.compose != "" {
		start := ed.caret
		end := start + utf8.RuneCountInString(ed.compose)
		for _, r := range l.Selection(start, end) {
			p.Fill(Rect{ox + r.X, oy + r.Y + r.H - 2, r.W, 1}, ts.color, 0)
		}
	}
	if focused && !ed.readOnly {
		rt := e.c.rt
		phase := time.Since(rt.blinkStart)
		const blink = 530 * time.Millisecond
		if (phase/blink)%2 == 0 {
			x, y, h := l.CaretAt(text.CaretPosition{Index: ed.displayIndex(ed.caret) + ed.composeCaret, Affinity: ed.caretAffinity})
			p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpFill, Rect: p.snap(Rect{ox + x, oy + y, 0, h}), Color: t.Accent.scene(), Wide: p.wide(t.Accent, Color{}, Color{}), Opacity: p.opacity})
			op := &p.s.Ops[len(p.s.Ops)-1]
			op.Rect.W = max(round(p.scale), 1)
		}
		if phase < 30*time.Second {
			e.c.After(blink - phase%blink)
		}
	}
	p.popClip()
	p.clip = saved
}

// rangeSpans returns the TextRanges in runes start up to end of the text,
// from start: the spans of their weights for the layout ("" when none
// sets one), and how to paint their colors (nil when none sets one).
func (ed *editor) rangeSpans(start, end int) (string, *spanPaint) {
	if len(ed.ranges) == 0 {
		return "", nil
	}
	var styles []text.Span
	var spans []Span
	weighted, colored := false, false
	at := start
	for _, r := range ed.ranges {
		from, to := max(r.Start, start), min(r.End, end)
		if from >= to || from < at {
			continue
		}
		if from > at {
			styles = append(styles, text.Span{End: from - start})
			spans = append(spans, Span{})
		}
		styles = append(styles, text.Span{End: to - start, Weight: r.Weight})
		spans = append(spans, Span{Color: r.Color, Weight: r.Weight})
		weighted = weighted || r.Weight > 0
		colored = colored || r.Color.A > 0
		at = to
	}
	key := ""
	if weighted {
		key = text.EncodeSpans(styles)
	}
	var sp *spanPaint
	if colored {
		sp = &spanPaint{spans: spans, styles: styles}
	}
	return key, sp
}
