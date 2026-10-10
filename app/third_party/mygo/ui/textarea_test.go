package ui

import (
	"math"
	"math/rand/v2"
	"strings"
	"testing"
	"unicode/utf8"
	"unsafe"

	"github.com/egoist/mygo/internal/text"
)

// checkBuffer compares b with the runes it should hold.
func checkBuffer(t *testing.T, b *buffer, want []rune) {
	t.Helper()
	if b.s != string(want) || b.n != len(want) {
		t.Fatalf("buffer holds %q (%d runes), want %q", b.s, b.n, string(want))
	}
	if b.units != countUnits(b.s) {
		t.Fatalf("UTF-16 length %d want %d", b.units, countUnits(b.s))
	}
	p := 0
	for i := 0; i <= len(want); i++ {
		if i == 0 || want[i-1] == '\n' {
			if p >= len(b.paras) || b.paras[p].rune != i || b.paras[p].byte != len(string(want[:i])) {
				t.Fatalf("paragraph %d does not start at rune %d: %+v", p, i, b.paras)
			}
			p++
		}
		if got := b.para(i); got != p-1 {
			t.Fatalf("rune %d is in paragraph %d, want %d", i, got, p-1)
		}
		if got := b.byteOf(i); got != len(string(want[:i])) {
			t.Fatalf("rune %d starts at byte %d, want %d", i, got, len(string(want[:i])))
		}
		units := countUnits(string(want[:i]))
		if b.utf16At(i) != units || b.runeAtUTF16(units) != i {
			t.Fatalf("UTF-16 offset at rune %d", i)
		}
		if i < len(want) && want[i] > 0xffff && b.runeAtUTF16(units+1) != i {
			t.Fatal("UTF-16 index split surrogate")
		}
	}
	if p != len(b.paras) {
		t.Fatalf("%d paragraphs, want %d", len(b.paras), p)
	}
}

func TestBufferEdits(t *testing.T) {
	alphabet := []rune("ab \n\r€😀é_")
	rng := rand.New(rand.NewPCG(1, 2))
	var b buffer
	var want []rune
	b.set("")
	for range 3000 {
		a := rng.IntN(len(want) + 1)
		z := a + rng.IntN(min(len(want)-a, 6)+1)
		ins := make([]rune, rng.IntN(5))
		for i := range ins {
			ins[i] = alphabet[rng.IntN(len(alphabet))]
		}
		if got := b.slice(a, z); got != string(want[a:z]) {
			t.Fatalf("slice(%d, %d) = %q, want %q", a, z, got, string(want[a:z]))
		}
		b.replace(a, z, string(ins))
		want = append(want[:a:a], append(ins, want[z:]...)...)
		checkBuffer(t, &b, want)
	}
}

func TestBufferSetUTF8(t *testing.T) {
	for _, s := range []string{
		"", "\n", "ascii\ncode\n", "long ASCII line without a newline",
		"é\n日本語\n😀\r\n", "1234567é\n12345678😀tail",
		"bad\xff\xfe\nutf8\xc0\xaf\n", "\xf0\x9f\n\x80\x00",
	} {
		var b buffer
		b.set(s)
		// Invalid UTF-8 preserves the source bytes but decodes to RuneError
		// as range does; compare positions against the source independently.
		if b.n != len([]rune(s)) {
			t.Fatalf("%q: %d runes, want %d", s, b.n, len([]rune(s)))
		}
		p, runes := 1, 0
		for at, r := range s {
			runes++
			if r == '\n' {
				if b.paras[p].rune != runes || b.paras[p].byte != at+1 {
					t.Fatalf("%q: paragraph %d starts at %+v", s, p, b.paras[p])
				}
				p++
			}
		}
		if p != len(b.paras) {
			t.Fatalf("%q: %d paragraphs, want %d", s, len(b.paras), p)
		}
	}
}

func TestBufferWords(t *testing.T) {
	s := "one, two_3\n  four\n\nfive"
	var b buffer
	b.set(s)
	var tb text.Boundaries
	tb.Reset([]rune(s))
	for i := 0; i <= b.n; i++ {
		if got, want := b.nextWord(i), tb.NextWord(i); got != want {
			t.Errorf("nextWord(%d) = %d, want %d", i, got, want)
		}
		if got, want := b.prevWord(i), tb.PrevWord(i); got != want {
			t.Errorf("prevWord(%d) = %d, want %d", i, got, want)
		}
		gs, ge := b.wordAt(i)
		ws, we := tb.WordAt(i)
		if gs != ws || ge != we {
			t.Errorf("wordAt(%d) = %d, %d, want %d, %d", i, gs, ge, ws, we)
		}
	}
}

func TestBufferGraphemes(t *testing.T) {
	s := "éx\r\n👍🏽\n\nz"
	var b buffer
	b.set(s)
	var tb text.Boundaries
	tb.Reset([]rune(s))
	var g graphemes
	for i := 0; i <= b.n; i++ {
		if got, want := g.next(&b, i), tb.NextGrapheme(i); got != want {
			t.Errorf("next(%d) = %d, want %d", i, got, want)
		}
		if got, want := g.prev(&b, i), tb.PrevGrapheme(i); got != want {
			t.Errorf("prev(%d) = %d, want %d", i, got, want)
		}
	}
}

func TestHeights(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	paras := make([]paragraph, 300)
	for i := range paras {
		if rng.IntN(3) > 0 {
			paras[i].h = float32(10 + rng.IntN(40))
		}
	}
	var h heights
	h.reset(paras)
	h.est = 17
	naiveTop := func(p int) float64 {
		var y float64
		for _, q := range paras[:p] {
			if q.h > 0 {
				y += float64(q.h)
			} else {
				y += h.est
			}
		}
		return y
	}
	for range 200 {
		p := rng.IntN(len(paras))
		if paras[p].h > 0 {
			nh := float32(10 + rng.IntN(40))
			h.add(p, float64(nh-paras[p].h), 0)
			paras[p].h = nh
		} else {
			paras[p].h = float32(10 + rng.IntN(40))
			h.add(p, float64(paras[p].h), -1)
		}
		for q := 0; q <= len(paras); q++ {
			if got, want := h.top(q), naiveTop(q); math.Abs(got-want) > 1e-6 {
				t.Fatalf("top(%d) = %v, want %v", q, got, want)
			}
		}
		for range 20 {
			y := rng.Float64() * (naiveTop(len(paras)) + 50)
			got := h.at(y)
			if got < len(paras)-1 && !(naiveTop(got) <= y && y < naiveTop(got+1)) || got == len(paras)-1 && y < naiveTop(got) {
				t.Fatalf("at(%v) = %d, whose top is %v", y, got, naiveTop(got))
			}
		}
	}
}

// TestTextAreaLaysOutAsWholeText checks that a text area laid out a
// paragraph at a time puts its carets where the text laid out whole has
// them.
func TestTextAreaLaysOutAsWholeText(t *testing.T) {
	s := strings.Repeat("A paragraph long enough to wrap in the text area, twice over at least, with words.\n\nshort\n", 4)
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 300, 2000)
	var e *node
	for _, st := range tt.rt.states {
		if st.editor != nil {
			e = &node{st: st}
		}
	}
	ed := e.st.editor
	params := ed.area.params
	params.Text = s
	whole := textSystem().Layout(params)
	for i := 0; i <= ed.buf.n; i++ {
		x, y, h := ed.area.caretAt(ed, i, 0)
		wx, wy, wh := whole.Caret(i)
		if x != wx || float32(y) != wy || h != wh {
			t.Fatalf("caret %d at %v, %v, %v; laid out whole, at %v, %v, %v", i, x, y, h, wx, wy, wh)
		}
	}
	if got := ed.area.hs.top(len(ed.buf.paras)); float32(got) != whole.Height {
		t.Errorf("the text is %v high, laid out whole %v", got, whole.Height)
	}
}

// TestTextAreaUndo edits a text area at random, then undoes every edit and
// redoes them.
func TestTextAreaUndo(t *testing.T) {
	s := "first line\nsecond line\nthird"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	var ed *editor
	for _, st := range tt.rt.states {
		if st.editor != nil {
			ed = st.editor
		}
	}
	rng := rand.New(rand.NewPCG(5, 6))
	// The text before each step of undo, which may change nothing, as
	// typing over a selection what it holds.
	texts := []string{s}
	keys := []Key{KeyLeft, KeyRight, KeyUp, KeyDown, KeyHome, KeyEnd}
	for range 60 {
		switch rng.IntN(4) {
		case 0:
			tt.Key(0, keys[rng.IntN(len(keys))])
			continue
		case 1:
			tt.Type("xy\nz")
		case 2:
			tt.Key(0, KeyBackspace)
		case 3:
			tt.Key(Shift, KeyLeft)
			tt.Key(Shift, KeyUp)
			tt.Type("é")
		}
		// Typing in a row makes one step: end it.
		tt.Key(0, KeyLeft)
		tt.Key(0, KeyRight)
		for len(texts) <= len(ed.undo) {
			texts = append(texts, s)
		}
	}
	final := s
	for i := len(texts) - 2; i >= 0; i-- {
		tt.Key(Cmd, KeyZ)
		if s != texts[i] {
			t.Fatalf("undo %d: %q, want %q", len(texts)-1-i, s, texts[i])
		}
	}
	for range len(texts) - 1 {
		tt.Key(Cmd|Shift, KeyZ)
	}
	if s != final {
		t.Errorf("redoing everything made %q, want %q", s, final)
	}
}

// TestTextAreaScrolls scrolls a long text area with the wheel, and keeps
// the caret in view as it moves.
func TestTextAreaScrolls(t *testing.T) {
	var b strings.Builder
	for i := range 2000 {
		b.WriteString("line ")
		b.WriteString(strings.Repeat("x", i%7))
		b.WriteByte('\n')
	}
	s := b.String()
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	// A text area opens with the caret at the end, in view.
	tt.Key(Ctrl, KeyHome)
	var st *state
	for _, x := range tt.rt.states {
		if x.editor != nil {
			st = x
		}
	}
	a := st.editor.area
	tt.Scroll(100, 100, 0, 5000)
	tt.Frame()
	if st.scrollY < 4000 || a.scroll != st.scrollY {
		t.Fatalf("the wheel scrolled to %v (area %v)", st.scrollY, a.scroll)
	}
	if a.first == 0 {
		t.Error("the area lays out the paragraphs at the top, not in view")
	}
	// The view stays where the wheel put it: the caret, at the top, does
	// not bring it back.
	tt.Frame()
	if st.scrollY < 4000 {
		t.Fatalf("the view went back to %v", st.scrollY)
	}
	// Moving the caret shows it.
	tt.Key(0, KeyDown)
	if st.scrollY > 100 {
		t.Errorf("the caret moved out of view: the view is at %v", st.scrollY)
	}
	// The end of the text, with Cmd+Down (Ctrl+End elsewhere).
	tt.Key(Ctrl, KeyEnd)
	_, y, h := a.caretAt(st.editor, st.editor.caret, 0)
	if st.editor.caret != utf8.RuneCountInString(s) || y+float64(h) > a.scroll+float64(st.h)+1 || y < a.scroll {
		t.Errorf("caret %d at %v, the view from %v", st.editor.caret, y, a.scroll)
	}
	if a.laid > maxLaid+a.last-a.first+1 {
		t.Errorf("%d paragraphs keep their layouts", a.laid)
	}
}

// TestTextAreaSelectsAsWholeText checks that the selection painted a
// paragraph at a time covers what it covers in the text laid out whole,
// selected newlines and empty lines included.
func TestTextAreaSelectsAsWholeText(t *testing.T) {
	s := "A paragraph long enough to wrap in the text area, twice over.\n\nshort\n\nlast one"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 260, 2000)
	var ed *editor
	for _, st := range tt.rt.states {
		if st.editor != nil {
			ed = st.editor
		}
	}
	a := ed.area
	params := a.params
	params.Text = s
	whole := textSystem().Layout(params)
	n := ed.buf.n
	for from := 0; from <= n; from++ {
		for to := from + 1; to <= n; to++ {
			want := whole.Selection(from, to)
			var got []text.Rect
			last := len(ed.buf.paras) - 1
			for i := range ed.buf.paras {
				start, end := ed.buf.paras[i].rune, ed.buf.end(i)
				if from > end || to < start || to == start && i > 0 && from < start {
					continue
				}
				l := a.paraLayout(ed, i)
				top := float32(a.hs.top(i))
				for _, r := range l.SelectionOn(max(from, start)-start, min(to, end)-start, to > end && i < last) {
					got = append(got, text.Rect{X: r.X, Y: r.Y + top, W: r.W, H: r.H})
				}
			}
			if len(got) != len(want) {
				t.Fatalf("selection %d-%d: %v, laid out whole %v", from, to, got, want)
			}
			for k := range got {
				if got[k] != want[k] {
					t.Fatalf("selection %d-%d: %v, laid out whole %v", from, to, got, want)
				}
			}
		}
	}
}

// TestTextAreaUndoAppChanges undoes typing that the app reformatted: the
// step takes the app's change back with the typing.
func TestTextAreaUndoAppChanges(t *testing.T) {
	s := ""
	tt := coreNewTester(func(c *context) {
		coreTextArea(c, &s).Fill()
		s = strings.ToUpper(s)
	}, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	tt.Type("ab")
	tt.Frame()
	if s != "AB" {
		t.Fatalf("typed %q", s)
	}
	tt.Key(Cmd, KeyZ)
	if s != "" {
		t.Errorf("undo made %q", s)
	}
}

// textAreaState returns the state of the only text area of tt.
func textAreaState(tt *Tester) *state {
	for _, st := range tt.rt.states {
		if st.editor != nil && st.editor.area != nil {
			return st
		}
	}
	return nil
}

// TestTextAreaUndoAppLog checks that the last step of undo keeps one
// change however often the app sets the text after it, as a log does, and
// that undoing the step still takes the app's texts back.
func TestTextAreaUndoAppLog(t *testing.T) {
	s := ""
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	tt.Type("x")
	tt.Frame()
	for range 100 {
		s += "a line of the log\n"
		tt.Frame()
	}
	ed := textAreaState(tt).editor
	if n := len(ed.undo[len(ed.undo)-1].changes); n != 1 {
		t.Errorf("the last step holds %d changes", n)
	}
	// Typing after the app's text is a change of its own.
	tt.Type("y")
	tt.Frame()
	if c := ed.undo[len(ed.undo)-1].changes; len(c) != 2 || c[1].inserted != "y" {
		t.Errorf("typing after the app's text made %+v", c[len(c)-1])
	}
	tt.Key(Cmd, KeyZ)
	if s != "" {
		t.Errorf("undo made %q", s)
	}
}

// TestTextAreaSharesValue checks that an edit leaving the text as it was
// still gives the value the text's string, which the next frames compare
// at once.
func TestTextAreaSharesValue(t *testing.T) {
	s := strings.Repeat("abc\n", 100) + "x"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	ed := textAreaState(tt).editor
	ed.anchor, ed.caret = ed.buf.n-1, ed.buf.n
	tt.Type("x")
	tt.Frame()
	if ed.buf.s != s || unsafe.StringData(ed.buf.s) != unsafe.StringData(s) {
		t.Error("the value does not share the text's string")
	}
}

// TestTextAreaRevealsWrapped checks that the caret comes into view when the
// paragraphs above it are taller than estimated, as long ones wrapping
// after short ones.
func TestTextAreaRevealsWrapped(t *testing.T) {
	s := strings.Repeat("short\n", 2000) + strings.Repeat(strings.Repeat("word ", 80)+"\n", 300) + "end"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	st := textAreaState(tt)
	ed, a := st.editor, st.editor.area
	check := func(what string) {
		t.Helper()
		_, y, h := a.caretAt(ed, ed.caret, 0)
		if y < a.scroll || y+float64(h) > a.scroll+float64(st.h) {
			t.Errorf("%s: the caret is at %v, the view from %v", what, y, a.scroll)
		}
	}
	check("open")
	tt.Press(20, 15)
	tt.Release(20, 15)
	tt.Key(Ctrl, KeyHome)
	check("Ctrl+Home")
	tt.Key(Ctrl, KeyEnd)
	check("Ctrl+End")
}

// TestTextAreaKeepsViewOnAppText checks that a text the app sets, as a log
// growing, leaves the view where the wheel put it.
func TestTextAreaKeepsViewOnAppText(t *testing.T) {
	s := strings.Repeat("a line of the log\n", 2000)
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	tt.Key(Ctrl, KeyHome)
	st := textAreaState(tt)
	tt.Scroll(100, 100, 0, 5000)
	tt.Frame()
	s += "another line\n"
	tt.Frame()
	if st.scrollY != 5000 {
		t.Errorf("the view went to %v", st.scrollY)
	}
}

// TestTextAreaShowsThroughPadding checks that the paragraph showing in the
// padding above the view is laid out, as the text is drawn there.
func TestTextAreaShowsThroughPadding(t *testing.T) {
	s := strings.Repeat("line\n", 200)
	tt := coreNewTester(func(c *context) { coreTextArea(c, &s).Fill() }, 400, 300)
	tt.Press(20, 15)
	tt.Release(20, 15)
	tt.Key(Ctrl, KeyHome)
	a := textAreaState(tt).editor.area
	tt.Scroll(100, 100, 0, 50*a.line.Height+2) // 2 DIPs into paragraph 50
	tt.Frame()
	if a.anchor != 50 || a.first != 49 {
		t.Errorf("the view starts in paragraph %d, the first laid out is %d", a.anchor, a.first)
	}
}

// TestTextAreaPassword checks that Password leaves a text area as it is,
// as multi-line fields have no password mode on any platform.
func TestTextAreaPassword(t *testing.T) {
	notes := "first\nsecond"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &notes).Password().Height(120) }, 400, 200)
	ed := textAreaState(tt).editor
	if ed.password {
		t.Fatal("Password made a text area a password field")
	}
	if l := ed.buf.paras[1].layout; l == nil || string(l.Runes) != "second" {
		t.Errorf("the second paragraph shows %v", l)
	}
}

// TestTextAreaInForm checks that a field lines its label up with the first
// line of its text area.
func TestTextAreaInForm(t *testing.T) {
	bio := "The first line"
	tt := coreNewTester(func(c *context) {
		coreForm(c, func() {
			coreField(c, "About you", func() { coreTextArea(c, &bio).Height(110) })
		})
	}, 500, 300)
	label, _ := tt.Find("About you")
	st := textAreaState(tt)
	if y := st.y + st.editor.originY; abs32(label.Y-y) > 0.5 {
		t.Errorf("the label is at %v, the first line of the text area at %v", label.Y, y)
	}
}

// A text area with Lines is as high as its text wraps at its width, from its
// least lines up to its most, past which it scrolls.
func TestTextAreaLinesFollowWrappedText(t *testing.T) {
	draft := ""
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Width(200).Children(func() {
			coreTextAreaBase(c, &draft).Lines(1, 4).Label("Draft")
		})
	}, 400, 600)
	empty, _ := tt.Find("Draft")
	if empty.H <= 0 {
		t.Fatalf("empty text area is %v high", empty.H)
	}
	draft = "one line"
	tt.Frame()
	if r, _ := tt.Find("Draft"); r.H != empty.H {
		t.Errorf("one short line is %v high, want %v", r.H, empty.H)
	}
	// One paragraph long enough to wrap: higher, though it has no newline.
	draft = strings.Repeat("word ", 12)
	tt.Frame()
	wrapped, _ := tt.Find("Draft")
	if wrapped.H <= empty.H*1.5 {
		t.Errorf("a wrapping paragraph is %v high, one line %v", wrapped.H, empty.H)
	}
	draft = strings.Repeat("word ", 400)
	tt.Frame()
	long, _ := tt.Find("Draft")
	if math.Abs(float64(long.H-4*empty.H)) > 1 {
		t.Errorf("a long text is %v high, want the most lines, %v", long.H, 4*empty.H)
	}
}

// The app reads where the caret is and puts it elsewhere, as a mention
// completed where it was typed.
func TestTextSelection(t *testing.T) {
	draft := "hello world"
	var input *node
	move := -1
	tt := coreNewTester(func(c *context) {
		input = coreTextAreaBase(c, &draft).Label("Draft")
		if move >= 0 {
			input.SetTextSelection(move, move)
			move = -1
		}
	}, 400, 300)
	if err := tt.Click("Draft"); err != nil {
		t.Fatal(err)
	}
	move = 5
	tt.Frame()
	if start, end := input.TextSelection(); start != 5 || end != 5 {
		t.Fatalf("caret at %d–%d, want 5", start, end)
	}
	tt.Type(",")
	if draft != "hello, world" {
		t.Fatalf("typed into %q", draft)
	}
	if start, end := input.TextSelection(); start != 6 || end != 6 {
		t.Errorf("caret at %d–%d after typing, want 6", start, end)
	}
	tt.Frame()
	input.SetTextSelection(0, 99)
	if start, end := input.TextSelection(); start != 0 || end != utf8.RuneCountInString(draft) {
		t.Errorf("selection %d–%d, want the whole text", start, end)
	}
}

// A placeholder shows in the lines of its input, whose line height is
// fixed or not.
func TestPlaceholderWithFixedLineHeight(t *testing.T) {
	var a, b string
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Gap(10).Children(func() {
			coreTextAreaBase(c, &a).Lines(1, 4).FixedLineHeight(19).Placeholder("Area").Width(200).Label("A")
			coreTextInputBase(c, &b).FixedLineHeight(19).Placeholder("Input").Width(200).Label("B")
		})
	}, 300, 120)
	img := tt.Image()
	for _, name := range []string{"A", "B"} {
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("no %s", name)
		}
		inked := false
		for y := int(r.Y); y < int(r.Y+r.H) && !inked; y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if px := img.RGBAAt(x, y); px.R < 200 {
					inked = true
					break
				}
			}
		}
		if !inked {
			t.Errorf("the placeholder of %s draws nothing in %v", name, r)
		}
	}
}

// A single-line input's text goes where TextAlign puts it while it fits.
func TestInputTextAlign(t *testing.T) {
	left, right := "abc", "abc"
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Width(200).Children(func() {
			coreTextInputBase(c, &left).Label("Left")
			coreTextInputBase(c, &right).TextAlign(End).Label("Right")
		})
	}, 300, 100)
	img := tt.Image()
	inked := func(name string) (first, last int) {
		r, _ := tt.Find(name)
		first, last = -1, -1
		for x := int(r.X); x < int(r.X+r.W); x++ {
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				if img.RGBAAt(x, y).R < 128 {
					if first < 0 {
						first = x
					}
					last = x
					break
				}
			}
		}
		return first, last
	}
	l0, _ := inked("Left")
	r0, r1 := inked("Right")
	box, _ := tt.Find("Right")
	if l0 < 0 || r0 < 0 {
		t.Fatalf("no text drawn: %d, %d", l0, r0)
	}
	if r0 <= l0+50 || float32(r1) < box.X+box.W-8 {
		t.Errorf("right-aligned text spans %d–%d in %v; left-aligned starts at %d", r0, r1, box, l0)
	}
}

// An input that stops calling Password shows its text again.
func TestInputPasswordToggles(t *testing.T) {
	value, hidden := "secret", true
	var in *node
	tt := coreNewTester(func(c *context) {
		in = coreTextInputBase(c, &value).Width(200).Label("Key")
		if hidden {
			in.Password()
		}
	}, 300, 60)
	if got := in.st.editor.displayText(); got == value {
		t.Fatalf("a password shows %q", got)
	}
	hidden = false
	tt.Frame()
	tt.Frame()
	if got := in.st.editor.displayText(); got != value {
		t.Errorf("the shown key reads %q", got)
	}
}

// inkSpan is the first and last columns of `name`'s box with dark pixels in them.
func inkSpan(tt *Tester, name string) (first, last int, box Rect) {
	img := tt.Image()
	box, _ = tt.Find(name)
	first, last = -1, -1
	for x := int(box.X); x < int(box.X+box.W); x++ {
		for y := int(box.Y); y < int(box.Y+box.H); y++ {
			if px := img.RGBAAt(x, y); px.R < 160 && px.G < 160 && px.B < 160 {
				if first < 0 {
					first = x
				}
				last = x
				break
			}
		}
	}
	return first, last, box
}

// A single-line input's placeholder stays on its line, cut off at the box, where a wrapped one
// would stop at the last word that fits; a text area's wraps.
func TestInputPlaceholderStaysOnItsLine(t *testing.T) {
	var line, area string
	var lineEl, areaEl *node
	coreNewTester(func(c *context) {
		lineEl = coreTextInputBase(c, &line).Width(120).Placeholder("mmmm mmmm mmmm mmmm mmmm mmmm").Label("Field")
		areaEl = coreTextAreaBase(c, &area).Width(120).Placeholder("mmmm mmmm mmmm mmmm mmmm mmmm").Label("Area")
	}, 300, 160)
	if n := len(textSystem().Layout(lineEl.placeholderParams(120)).Lines); n != 1 {
		t.Errorf("the input's placeholder takes %d lines", n)
	}
	if n := len(textSystem().Layout(areaEl.placeholderParams(120)).Lines); n < 2 {
		t.Errorf("the text area's placeholder takes %d lines", n)
	}
}

// An input without the focus shows the start of a text too long for it; with the focus, the
// caret's end.
func TestInputShowsItsStartUnfocused(t *testing.T) {
	long, other := strings.Repeat("abc ", 60), ""
	var in *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Children(func() {
			in = coreTextInputBase(c, &long).Width(120).Label("Long")
			coreTextInputBase(c, &other).Width(120).Label("Other")
		})
	}, 300, 80)
	if x := in.st.editor.scrollX; x != 0 {
		t.Errorf("the unfocused input is scrolled %v to its caret at the end", x)
	}
	in.Focus()
	tt.Frame()
	tt.Frame()
	if in.st.editor.scrollX == 0 {
		t.Error("the focused input keeps its start, not its caret, in view")
	}
	tt.Key(0, KeyTab)
	tt.Frame()
	if x := in.st.editor.scrollX; x != 0 {
		t.Errorf("the input left by the focus stays scrolled %v", x)
	}
}

// TextRanges paint runs of an input's text in their color and lay them out in their weight, in a
// text area and a single-line input alike, and only in the frames that call it.
func TestTextRanges(t *testing.T) {
	area, line := "hi @Scout there\nnext", "to @Scout now"
	styled := true
	red := RGB(220, 0, 0)
	var areaEl, lineEl *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(10).Padding(10).Children(func() {
			areaEl = coreTextAreaBase(c, &area).Width(260).Label("Area")
			lineEl = coreTextInputBase(c, &line).Width(260).Label("Line")
			if styled {
				areaEl.TextRanges(TextRange{Start: 3, End: 9, Color: red, Weight: 700})
				lineEl.TextRanges(TextRange{Start: 3, End: 9, Color: red})
			}
		})
	}, 320, 160)
	// The glyphs painted red, read from the scene: pixels would count the colored edges that
	// subpixel antialiasing, as ClearType's, gives black text too.
	redGlyphs := func() int {
		n := 0
		for _, g := range tt.h.last.Glyphs {
			if g.Color == red.scene() {
				n++
			}
		}
		return n
	}
	if n := redGlyphs(); n < 2 {
		t.Fatalf("%d glyphs red with the ranges", n)
	}
	ed := areaEl.st.editor
	bold := ed.area.paraLayout(ed, 0).Lines[0].Width
	styled = false
	tt.Frame()
	tt.Frame()
	if n := redGlyphs(); n != 0 {
		t.Errorf("%d glyphs stayed red without the ranges", n)
	}
	if plain := ed.area.paraLayout(ed, 0).Lines[0].Width; plain >= bold {
		t.Errorf("the bold mention is %v wide, plain %v", bold, plain)
	}
}
