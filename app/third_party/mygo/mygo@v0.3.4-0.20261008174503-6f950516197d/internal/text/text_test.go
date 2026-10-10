package text

import (
	"image"
	"math"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/scene"
	"golang.org/x/image/font/gofont/goregular"
)

func TestLayoutWraps(t *testing.T) {
	s := Shared()
	start := time.Now()
	s.Preload()
	t.Logf("system fonts loaded in %v", time.Since(start))

	one := s.Layout(Params{Text: "Hello, world", Style: Style{Size: 16}})
	if len(one.Lines) != 1 || one.Width <= 50 || one.Height <= 16 {
		t.Fatalf("single line: %d lines, %vx%v", len(one.Lines), one.Width, one.Height)
	}
	long := strings.Repeat("The quick brown fox jumps over the lazy dog. ", 6)
	wrapped := s.Layout(Params{Text: long, Style: Style{Size: 16}, Width: 200})
	if len(wrapped.Lines) < 4 {
		t.Fatalf("wrapped to %d lines", len(wrapped.Lines))
	}
	for i, line := range wrapped.Lines {
		if line.Width > 200.5 {
			t.Errorf("line %d is %v wide", i, line.Width)
		}
		if i > 0 && line.Start != wrapped.Lines[i-1].End {
			t.Errorf("line %d starts at %d, the previous ended at %d", i, line.Start, wrapped.Lines[i-1].End)
		}
	}
	if last := wrapped.Lines[len(wrapped.Lines)-1]; last.End != len(wrapped.Runes) {
		t.Errorf("last line ends at %d of %d", last.End, len(wrapped.Runes))
	}
}

func TestLayoutNewlinesAndEmpty(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: "a\n\nb", Style: Style{Size: 14}})
	if len(l.Lines) != 3 {
		t.Fatalf("%d lines", len(l.Lines))
	}
	if l.Lines[1].Start != 2 || l.Lines[1].End != 2 || l.Lines[2].Start != 3 {
		t.Errorf("lines %+v %+v", l.Lines[1], l.Lines[2])
	}
	empty := s.Layout(Params{Style: Style{Size: 14}})
	if len(empty.Lines) != 1 || empty.Height <= 0 {
		t.Errorf("empty text: %d lines, height %v", len(empty.Lines), empty.Height)
	}
}

func TestCarets(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: "office files", Style: Style{Size: 20}})
	prev := float32(-1)
	for i := 0; i <= len(l.Runes); i++ {
		x, _, h := l.Caret(i)
		if x < prev || h <= 0 {
			t.Fatalf("caret %d at %v after %v (height %v)", i, x, prev, h)
		}
		prev = x
		if got := l.IndexAt(x+0.1, 5); got != i {
			t.Errorf("IndexAt(caret %d) = %d", i, got)
		}
	}
	if rects := l.Selection(0, 6); len(rects) != 1 || rects[0].W <= 0 {
		t.Errorf("selection %+v", rects)
	}
}

func TestTruncation(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: strings.Repeat("word ", 50), Style: Style{Size: 14}, Width: 120, MaxLines: 2})
	if len(l.Lines) != 2 || !l.Truncated {
		t.Fatalf("%d lines, truncated %v", len(l.Lines), l.Truncated)
	}
}

func TestGlyphRaster(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: "Ag", Style: Style{Size: 32}})
	n := 0
	for _, g := range l.Lines[0].Glyphs {
		img := s.Glyph(g.Font, g.ID, 1, 0, 0, false)
		if !img.OK || img.W == 0 || img.H == 0 || img.Top >= 0 {
			t.Fatalf("glyph %v: %+v", g.ID, img)
		}
		var ink int
		for y := 0; y < int(img.H); y++ {
			for x := 0; x < int(img.W); x++ {
				if s.MaskAtlas.Pix[(int(img.Y)+y)*s.MaskAtlas.W+int(img.X)+x] > 128 {
					ink++
				}
			}
		}
		if ink < 20 {
			t.Errorf("glyph %v has %d inked pixels", g.ID, ink)
		}
		n++
	}
	if n != 2 {
		t.Errorf("%d glyphs", n)
	}
}

// TestShadeOf checks shades against the steps of Core Text's smoothing,
// measured on macOS 27.
func TestShadeOf(t *testing.T) {
	for _, c := range []struct {
		r, g, b uint8
		want    Shade
	}{
		{0, 0, 0, 0}, {0x4d, 0x4d, 0x4d, 0}, {0, 0, 255, 0},
		{0x73, 0x73, 0x73, 1}, {0x99, 0x99, 0x99, 1}, {255, 0, 0, 1}, {0x25, 0x63, 0xeb, 1},
		{0xb3, 0xb3, 0xb3, 2},
		{0xd9, 0xd9, 0xd9, 3}, {0, 255, 0, 3}, {255, 255, 255, 3},
	} {
		if got := ShadeOf(c.r, c.g, c.b); got != c.want {
			t.Errorf("ShadeOf(%#x, %#x, %#x) = %d, want %d", c.r, c.g, c.b, got, c.want)
		}
	}
}

// TestGlyphShades rasterizes lighter text bolder where the engine does, as
// AppKit draws it, and color glyphs once.
func TestGlyphShades(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "O", Style: Style{Size: 13}})
	g := l.Lines[0].Glyphs[0]
	if !g.Font.shaded {
		t.Skip("the engine draws glyphs the same in any color")
	}
	var ink [Shades]int
	for shade := range Shade(Shades) {
		img := s.Glyph(g.Font, g.ID, 2, 0, shade, false)
		for y := range int(img.H) {
			for x := range int(img.W) {
				ink[shade] += int(s.MaskAtlas.Pix[(int(img.Y)+y)*s.MaskAtlas.W+int(img.X)+x])
			}
		}
	}
	if ink[Shades-1] == ink[0] {
		t.Skip("font smoothing is off")
	}
	for i := 1; i < Shades; i++ {
		if ink[i] <= ink[i-1] {
			t.Errorf("ink of the shades %v", ink)
		}
	}
	l = s.Layout(Params{Text: "🎉", Style: Style{Size: 32}})
	if g := l.Lines[0].Glyphs[0]; g.Font.shaded {
		t.Errorf("the emoji font is shaded")
	}
}

// TestThick rasterizes Thick text as bold as the lightest shade where the
// engine thickens glyphs (Core Graphics' smoothing has four strengths),
// whether or not the user smooths fonts, and as other text elsewhere.
func TestThick(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "O", Style: Style{Size: 13}})
	g := l.Lines[0].Glyphs[0]
	ink := func(f *Font, shade Shade) (n int) {
		img := s.Glyph(f, g.ID, 2, 0, shade, false)
		for y := range int(img.H) {
			for x := range int(img.W) {
				n += int(s.MaskAtlas.Pix[(int(img.Y)+y)*s.MaskAtlas.W+int(img.X)+x])
			}
		}
		return n
	}
	dark, light, thick := ink(g.Font, 0), ink(g.Font, Shades-1), ink(g.Font, Thick)
	if !g.Font.thickens {
		if thick != dark {
			t.Errorf("thick text has ink %d, other text %d", thick, dark)
		}
		return
	}
	if thick < light || thick <= dark {
		t.Errorf("thick text has ink %d, the lightest shade %d, the darkest %d", thick, light, dark)
	}
	// Without smoothing, as when the user turns it off.
	unsmoothed := *g.Font
	unsmoothed.shaded = false
	if plain, thick := ink(&unsmoothed, Shades-1), ink(&unsmoothed, Thick); thick <= plain {
		t.Errorf("thick text has ink %d unsmoothed, other text %d", thick, plain)
	}
}

// TestFlat rasterizes Flat text unsmoothed, thinner than the darkest shade
// where the engine smooths fonts, and as other text elsewhere.
func TestFlat(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "O", Style: Style{Size: 13}})
	g := l.Lines[0].Glyphs[0]
	ink := func(f *Font, shade Shade) (n int) {
		img := s.Glyph(f, g.ID, 2, 0, shade, false)
		for y := range int(img.H) {
			for x := range int(img.W) {
				n += int(s.MaskAtlas.Pix[(int(img.Y)+y)*s.MaskAtlas.W+int(img.X)+x])
			}
		}
		return n
	}
	dark, flat := ink(g.Font, 0), ink(g.Font, Flat)
	unsmoothed := *g.Font
	unsmoothed.shaded = false
	if plain := ink(&unsmoothed, 0); flat != plain {
		t.Errorf("flat text has ink %d, unsmoothed text %d", flat, plain)
	}
	if g.Font.shaded && flat >= dark {
		t.Errorf("flat text has ink %d, the darkest shade %d", flat, dark)
	}
}

// square draws a w×w mask filled with v.
func square(w int, v byte, calls *int) func() (int, int, []byte) {
	return func() (int, int, []byte) {
		*calls++
		pix := make([]byte, w*w)
		for i := range pix {
			pix[i] = v
		}
		return w, w, pix
	}
}

func (s *System) maskPixel(g GlyphImage) byte {
	return s.MaskAtlas.Pix[int(g.Y)*s.MaskAtlas.W+int(g.X)]
}

func TestMaskLastsOnceDrawnAgain(t *testing.T) {
	s := newSystem()
	width := s.MaskAtlas.W
	var calls int
	s.BeginFrame()
	first := s.Mask(1, square(8, 7, &calls))
	again := s.Mask(1, square(8, 7, &calls))
	s.EndFrame()
	if !first.OK || again != first || calls != 1 {
		t.Fatalf("first frame: %+v %+v, %d draws", first, again, calls)
	}
	if int(first.Y) < s.MaskAtlas.H/2 {
		t.Errorf("a new mask went to row %d, not to the transient rows", first.Y)
	}
	s.BeginFrame()
	second := s.Mask(1, square(8, 7, &calls))
	s.EndFrame()
	if !second.OK || calls != 2 || second.Y != 0 || s.maskPixel(second) != 7 {
		t.Fatalf("second frame: %+v, %d draws", second, calls)
	}
	s.BeginFrame()
	third := s.Mask(1, square(8, 7, &calls))
	s.EndFrame()
	if third != second || calls != 2 {
		t.Errorf("third frame: %+v, %d draws", third, calls)
	}
	// Masks drawn by one frame each, as in an animation, take the same
	// transient rows again and again.
	var y uint16
	for i := range 100 {
		s.BeginFrame()
		g := s.Mask(uint64(100+i), square(30, 9, &calls))
		s.EndFrame()
		if i == 0 {
			y = g.Y
		}
		if !g.OK || s.maskPixel(g) != 9 || g.Y != y || int(y) < s.MaskAtlas.H/2 {
			t.Fatalf("frame %d: %+v", i, g)
		}
	}
	if s.MaskAtlas.W != width {
		t.Errorf("the atlas grew to %d", s.MaskAtlas.W)
	}
}

func TestMakeRoom(t *testing.T) {
	s := newSystem()
	// Room for several old masks, so the test exercises eviction and
	// repacking rather than the small atlas's first growth.
	s.MaskAtlas = scene.NewAtlas(1, 1024, 1024)
	var calls int
	// Fill the lasting rows with masks of earlier frames.
	var old []uint64
	for i := 0; ; i++ {
		key := uint64(i + 1)
		for range 2 { // the second frame makes it last
			s.BeginFrame()
			s.Mask(key, square(200, byte(i+1), &calls))
			s.EndFrame()
		}
		if s.Full() {
			break
		}
		old = append(old, key)
	}
	// A frame drawing one old mask and new ones runs out of room...
	s.BeginFrame()
	kept := s.Mask(old[2], square(200, 3, &calls))
	var left int
	for i := range 3 {
		key := uint64(1000 + i)
		s.recent[key] = s.frame - 1 // drawn by the previous frame: lasting
		if !s.Mask(key, square(300, 50, &calls)).OK {
			left++
		}
	}
	if !s.Full() || left == 0 {
		t.Fatalf("full %v, %d left out", s.Full(), left)
	}
	// ...and has it when painted again.
	s.MakeRoom()
	if s.Full() {
		t.Fatal("still full")
	}
	if g := s.Mask(old[2], square(200, 3, &calls)); g == kept || s.maskPixel(g) != 3 {
		t.Errorf("the kept mask is at %d,%d with %d", g.X, g.Y, s.maskPixel(g))
	}
	for i := range 3 {
		if g := s.Mask(uint64(1000+i), square(300, 50, &calls)); !g.OK || s.maskPixel(g) != 50 {
			t.Errorf("mask %d: %+v", i, g)
		}
	}
	if s.Full() {
		t.Error("full again")
	}
	if _, ok := s.masks[old[0]]; ok {
		t.Error("kept a mask the frame did not draw")
	}
	s.EndFrame()
}

// Thin masks can need a much wider or taller atlas without taking much
// area: one MakeRoom must accommodate them, whatever the starting size.
func TestMakeRoomForLongMasks(t *testing.T) {
	for _, size := range []image.Point{{X: 2000, Y: 3}, {X: 3, Y: 2000}} {
		s := newSystem()
		s.BeginFrame()
		draw := func() (int, int, []byte) {
			pix := make([]byte, size.X*size.Y)
			for i := range pix {
				pix[i] = 123
			}
			return size.X, size.Y, pix
		}
		if s.Mask(1, draw).OK || !s.Full() {
			t.Fatal("the long mask fit the initial atlas")
		}
		s.MakeRoom()
		if g := s.Mask(1, draw); !g.OK || s.maskPixel(g) != 123 || s.Full() {
			t.Fatalf("mask %v still does not fit after making room: %+v", size, g)
		}
		s.EndFrame()
	}
}

func TestRightToLeft(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: "שלום עולם", Style: Style{Size: 16}})
	if len(l.Lines) != 1 || !l.Lines[0].RTL || len(l.Lines[0].Glyphs) == 0 {
		t.Fatalf("%+v", l.Lines)
	}
	// The caret before the first rune is on the right.
	first, _, _ := l.Caret(0)
	last, _, _ := l.Caret(len(l.Runes))
	if first <= last {
		t.Errorf("carets at %v and %v", first, last)
	}
	// Start aligns right-to-left lines to the right.
	wide := s.Layout(Params{Text: "שלום", Style: Style{Size: 16}, Width: 300})
	if line := wide.Lines[0]; line.X+line.Width < 299 {
		t.Errorf("line at %v, %v wide", line.X, line.Width)
	}
}

func TestKeepSpaces(t *testing.T) {
	s := Shared()
	p := Params{Text: "aaaa bbbb", Style: Style{Size: 16}}
	p.Width = s.Layout(p).Width - 1
	trimmed := s.Layout(p)
	p.KeepSpaces = true
	kept := s.Layout(p)
	if len(trimmed.Lines) != 2 || len(kept.Lines) != 2 {
		t.Fatalf("%d and %d lines", len(trimmed.Lines), len(kept.Lines))
	}
	if trimmed.Lines[0].End != 5 || kept.Lines[0].End != 5 || trimmed.Lines[1].Start != 5 {
		t.Errorf("lines %d-%d and %d-%d", trimmed.Lines[0].Start, trimmed.Lines[0].End, trimmed.Lines[1].Start, trimmed.Lines[1].End)
	}
	if kept.Lines[0].Width <= trimmed.Lines[0].Width {
		t.Errorf("with the space %v wide, without %v", kept.Lines[0].Width, trimmed.Lines[0].Width)
	}
}

func TestNoBreakWords(t *testing.T) {
	s := Shared()
	p := Params{Text: "Supercalifragilistic word", Style: Style{Size: 16}, Width: 40}
	if l := s.Layout(p); len(l.Lines) < 3 {
		t.Errorf("a long word took %d lines", len(l.Lines))
	}
	p.NoBreakWords = true
	if l := s.Layout(p); len(l.Lines) != 2 || l.Lines[0].Width <= 40 || l.Lines[0].End != 21 {
		t.Errorf("%d lines, the first %v wide, ending at %d", len(l.Lines), l.Lines[0].Width, l.Lines[0].End)
	}
}

func TestEllipsis(t *testing.T) {
	s := Shared()
	l := s.Layout(Params{Text: "The quick brown fox jumps", Style: Style{Size: 16}, Width: 100, MaxLines: 1})
	line := l.Lines[0]
	if !l.Truncated || len(line.Glyphs) == 0 || line.Width > 100 || line.End >= len(l.Runes) {
		t.Fatalf("truncated %v, %d glyphs, %v wide, ending at %d", l.Truncated, len(line.Glyphs), line.Width, line.End)
	}
	if g := line.Glyphs[len(line.Glyphs)-1]; g.Runes != 0 || g.Cluster != line.End {
		t.Errorf("the last glyph is %+v", g)
	}
}

func TestColorEmoji(t *testing.T) {
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		t.Skip("emoji fonts vary")
	}
	s := Shared()
	l := s.Layout(Params{Text: "🎉", Style: Style{Size: 32}})
	g := l.Lines[0].Glyphs[0]
	if img := s.Glyph(g.Font, g.ID, 1, 0, 0, false); !img.OK || !img.Colored || img.W < 16 {
		t.Errorf("%+v", img)
	}
}

func TestRegisterFont(t *testing.T) {
	s := newSystem()
	if err := s.RegisterFont(goregular.TTF, "MyGo Test Sans"); err != nil {
		t.Fatal(err)
	}
	font := func(family string) *Font {
		return s.Layout(Params{Text: "Hello", Style: Style{Size: 20, Family: family}}).Lines[0].Glyphs[0].Font
	}
	system := font("")
	if font("MyGo Test Sans") == system || font("Go") == system {
		t.Error("text does not use the registered font")
	}
}

// TestUIFamily gives system-ui the desktop's interface font where the
// engine takes it, as Pango does: the family before fontconfig's default.
func TestUIFamily(t *testing.T) {
	s := newSystem()
	if err := s.RegisterFont(goregular.TTF, "MyGo Test UI"); err != nil {
		t.Fatal(err)
	}
	font := func(family string) *Font {
		return s.Layout(Params{Text: "Hello", Style: Style{Size: 20, Family: family}}).Lines[0].Glyphs[0].Font
	}
	before := font("")
	registered := font("MyGo Test UI")
	s.SetUIFamily("MyGo Test UI")
	_, takes := s.engine().(uiFamilySetter)
	for _, family := range []string{"", "system-ui"} {
		if got := font(family); takes && got != registered || !takes && got != before {
			t.Errorf("%q after SetUIFamily: the font of the interface family %v, of the system %v", family, got == registered, got == before)
		}
	}
	s.SetUIFamily("")
	if font("") != before {
		t.Error("system-ui keeps the interface family once it is unset")
	}
}

func TestLetterSpacing(t *testing.T) {
	s := newSystem()
	width := func(spacing float32) float32 {
		return s.Layout(Params{Text: "Hello", Style: Style{Size: 20, LetterSpacing: spacing}}).Width
	}
	plain, spaced, tight := width(0), width(4), width(-1)
	// Four DIPs after each of the five letters, or of the four between
	// them, as engines differ.
	if d := spaced - plain; d < 4*4-0.5 || d > 4*5+0.5 {
		t.Errorf("letter spacing of 4 widens Hello from %v to %v", plain, spaced)
	}
	if tight >= plain {
		t.Errorf("negative letter spacing makes Hello %v wide, not less than %v", tight, plain)
	}
}

func TestFontFeatures(t *testing.T) {
	s := newSystem()
	width := func(features string) float32 {
		return s.Layout(Params{Text: "AVAVAVAVAV", Style: Style{Size: 40, Features: features}}).Width
	}
	// The system's interface font kerns A and V together.
	kerned, plain := width(""), width("kern=0")
	if plain <= kerned {
		t.Errorf("AVAVAVAVAV is %v wide without kerning, %v with it", plain, kerned)
	}
	if w := width("kern=0, bogus, liga"); w != plain {
		t.Errorf("a list with an invalid tag is %v wide, not %v", w, plain)
	}
}

func TestParseFeatures(t *testing.T) {
	got := features(" tnum, liga=0 ,salt=2,toolong, bad=x, ss01")
	want := []feature{{[4]byte{'t', 'n', 'u', 'm'}, 1}, {[4]byte{'l', 'i', 'g', 'a'}, 0}, {[4]byte{'s', 'a', 'l', 't'}, 2}, {[4]byte{'s', 's', '0', '1'}, 1}}
	if !slices.Equal(got, want) {
		t.Errorf("features = %v, want %v", got, want)
	}
}

func TestSpans(t *testing.T) {
	s := newSystem()
	if err := s.RegisterFont(goregular.TTF, "MyGo Test Span"); err != nil {
		t.Fatal(err)
	}
	const text = "Small LARGE small"
	lay := func(spans ...Span) *Layout {
		return s.Layout(Params{Text: text, Style: Style{Size: 14}, Spans: EncodeSpans(spans)})
	}
	glyph := func(l *Layout, r int) Glyph {
		for _, g := range l.Lines[0].Glyphs {
			if g.Cluster == r {
				return g
			}
		}
		t.Fatalf("no glyph of rune %d", r)
		return Glyph{}
	}
	plain := lay()
	big := lay(Span{End: 6}, Span{End: 11, Size: 28, Weight: 700})
	if glyph(big, 0).Size != 14 || glyph(big, 7).Size != 28 || glyph(big, 13).Size != 14 {
		t.Errorf("glyph sizes %v, %v, %v", glyph(big, 0).Size, glyph(big, 7).Size, glyph(big, 13).Size)
	}
	if big.Height < plain.Height*1.5 || big.Width <= plain.Width {
		t.Errorf("a span twice the size leaves the text %v×%v, from %v×%v", big.Width, big.Height, plain.Width, plain.Height)
	}
	reg := lay(Span{End: 6}, Span{End: 11, Family: "MyGo Test Span"})
	want := s.Layout(Params{Text: "L", Style: Style{Size: 14, Family: "MyGo Test Span"}}).Lines[0].Glyphs[0].Font
	if glyph(reg, 7).Font != want || glyph(reg, 0).Font == want {
		t.Error("a span's family does not draw its runes, or draws the others")
	}
	// Truncated, the last line keeps its spans' styles.
	cut := s.Layout(Params{Text: text, Style: Style{Size: 14}, Width: glyph(big, 9).X, MaxLines: 1, Spans: EncodeSpans([]Span{{End: 6}, {End: 11, Size: 28}})})
	if !cut.Truncated || glyph(cut, 7).Size != 28 {
		t.Errorf("truncated: %v, the large run's size %v", cut.Truncated, glyph(cut, 7).Size)
	}
}

func TestEncodeSpans(t *testing.T) {
	spans := []Span{{End: 3, Family: "A, B", Size: 1.5, Weight: 600, Italic: true, LetterSpacing: -0.5, Features: "tnum"}, {End: 9}}
	if got := decodeSpans(EncodeSpans(spans)); !slices.Equal(got, spans) {
		t.Errorf("decoded %+v, want %+v", got, spans)
	}
	if got := decodeSpans(EncodeSpans(spans)[:5]); len(got) != 0 {
		t.Errorf("a cut encoding decodes to %+v", got)
	}
	// The spans of a paragraph, relative to it.
	in := spansIn([]Span{{End: 3, Weight: 1}, {End: 8, Weight: 2}, {End: 12, Weight: 3}}, 5, 10)
	if want := []Span{{End: 3, Weight: 2}, {End: 5, Weight: 3}}; !slices.Equal(in, want) {
		t.Errorf("spansIn = %+v, want %+v", in, want)
	}
}

func TestSpansAcrossParagraphs(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "ab\ncd", Style: Style{Size: 14}, Spans: EncodeSpans([]Span{{End: 1}, {End: 4, Size: 30}})})
	sizes := map[int]float32{}
	for _, line := range l.Lines {
		for _, g := range line.Glyphs {
			sizes[g.Cluster] = g.Size
		}
	}
	if sizes[0] != 14 || sizes[1] != 30 || sizes[3] != 30 || sizes[4] != 14 {
		t.Errorf("sizes by rune %v", sizes)
	}
}

// TestDigitFeatures picks proportional or tabular figures of the system's
// font, where it has both.
func TestDigitFeatures(t *testing.T) {
	s := newSystem()
	width := func(features string) float32 {
		return s.Layout(Params{Text: "1111111111", Style: Style{Size: 40, Features: features}}).Width
	}
	tabular, proportional := width("tnum"), width("pnum")
	if tabular == proportional {
		t.Skip("the system's font has digits of one kind")
	}
	if proportional >= tabular {
		t.Errorf("proportional ones are %v wide, tabular ones %v", proportional, tabular)
	}
}

// TestFontListFallback draws what the first family of a list lacks with
// the next family that has it, before the system's choice.
func TestFontListFallback(t *testing.T) {
	second := map[string]string{"windows": "MS Gothic", "darwin": "Hiragino Mincho ProN", "linux": "Noto Serif CJK JP"}[runtime.GOOS]
	s := newSystem()
	if err := s.RegisterFont(goregular.TTF, "MyGo Latin"); err != nil {
		t.Fatal(err)
	}
	// drawn tells fonts apart by what they draw, as engines may make a
	// Font of one face for each family list.
	drawn := func(text, family string, rune int) [4]float32 {
		for _, g := range s.Layout(Params{Text: text, Style: Style{Size: 20, Family: family}}).Lines[0].Glyphs {
			if g.Cluster == rune {
				return [4]float32{float32(g.ID), g.Advance, g.Font.Ascent, g.Font.Descent}
			}
		}
		return [4]float32{}
	}
	want := drawn("日", second, 0)
	// Without a font that has it, engines draw a missing glyph: 0, or one
	// of Pango's PANGO_GLYPH_UNKNOWN_FLAG.
	if id := uint32(want[0]); id == 0 || id&0x10000000 != 0 || want == drawn("日", "MyGo Latin", 0) {
		t.Skipf("%s is missing, or what the system falls back to", second)
	}
	list := "MyGo Latin, " + second
	if got := drawn("A日", list, 1); got != want {
		t.Errorf("日 in %q is not drawn with %s", list, second)
	}
	if drawn("A日", list, 0) != drawn("A", "MyGo Latin", 0) {
		t.Errorf("A in %q is not drawn with MyGo Latin", list)
	}
}

// TestFontsOfManySizes lets go of the fonts of sizes once they are many,
// as with an animated size, and draws on.
func TestFontsOfManySizes(t *testing.T) {
	s := newSystem()
	f, ok := s.engine().(fontForgetter)
	if !ok {
		t.Skip("the engine keeps no fonts")
	}
	for i := range 2 * maxFonts {
		l := s.Layout(Params{Text: "Hi", Style: Style{Size: 10 + float32(i)/10}})
		g := l.Lines[0].Glyphs[0]
		img := s.Glyph(g.Font, g.ID, 1, 0, 0, false)
		if !img.OK && s.Full() {
			s.MakeRoom() // as frames do once the atlas fills
			img = s.Glyph(g.Font, g.ID, 1, 0, 0, false)
		}
		if !img.OK {
			t.Fatalf("no glyph at size %v", g.Size)
		}
		s.EndFrame()
		if n := f.fontCount(); n > maxFonts {
			t.Fatalf("%d fonts after a frame", n)
		}
	}
}

// TestPlacement checks that glyphs go to the positions within a pixel the
// system's text stack draws them at, and baselines to its whole pixels:
// Core Graphics' two positions a pixel at 26 pixels an em, left of the pen,
// and baselines down to the next pixel; GTK's whole pixels.
func TestPlacement(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "l", Style: Style{Size: 13}})
	g := l.Lines[0].Glyphs[0]
	at := func(x float32) GlyphImage { return s.Glyph(g.Font, g.ID, 2, x, 0, false) }
	same := func(a, b GlyphImage) bool { return a.X == b.X && a.Y == b.Y && a.Left == b.Left }
	switch runtime.GOOS {
	case "darwin":
		if !same(at(10), at(10.49)) || same(at(10), at(10.5)) {
			t.Error("not two positions a pixel")
		}
		if b := s.Baseline(20.01); b != 21 {
			t.Errorf("baseline 20.01 drawn at %v", b)
		}
	case "linux":
		if !same(at(10), at(10.9)) {
			t.Error("not on whole pixels")
		}
		if b := s.Baseline(20.9); b != 20 {
			t.Errorf("baseline 20.9 drawn at %v", b)
		}
	case "windows":
		if b := s.Baseline(20.6); b != 21 {
			t.Errorf("baseline 20.6 drawn at %v", b)
		}
	}
}

// TestDecorate checks underlines and strikethroughs: below and above the
// baseline, at least a fraction of a pixel thick, from the text's start,
// on whole pixels with a thickness of the app's, and on macOS where AppKit draws them for the system font at 13 points,
// skipping the ink of descenders.
func TestDecorate(t *testing.T) {
	s := newSystem()
	l := s.Layout(Params{Text: "Hello", Style: Style{Size: 13}})
	under := s.Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0, 1, Underline, 0)
	strike := s.Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0, 1, Strikethrough, 0)
	if len(under) != 1 || len(strike) != 1 {
		t.Fatalf("%d underlines, %d strikethroughs", len(under), len(strike))
	}
	base := s.Baseline(l.Lines[0].Baseline)
	u, st := under[0], strike[0]
	if u.Top < base || u.Bottom <= u.Top || st.Bottom > base || st.Bottom <= st.Top || u.X0 > 1 || u.X1 < l.Width-1 {
		t.Errorf("underline %+v, strikethrough %+v, baseline %v, width %v", u, st, base, l.Width)
	}
	// Lines of a thickness of the app's cover whole pixels, wherever the
	// text is.
	for _, scale := range []float32{1, 1.5} {
		for _, d := range []Decoration{Underline, Strikethrough} {
			for _, st := range s.Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0.3, scale, d, 3) {
				if st.Bottom-st.Top != float32(math.Round(float64(3*scale))) || st.Top != float32(math.Floor(float64(st.Top))) {
					t.Errorf("a line 3 DIPs thick at scale %v: %+v", scale, st)
				}
			}
		}
	}
	if runtime.GOOS == "darwin" {
		if u.Top != base+1 || u.Bottom != base+2 || st.Top != base-4 || st.Bottom != base-3 {
			t.Errorf("underline %+v, strikethrough %+v from baseline %v", u, st, base)
		}
		l = s.Layout(Params{Text: "gypsy", Style: Style{Size: 24}})
		if n := len(s.Decorate(l, 0, 0, len(l.Lines[0].Glyphs), 0, 0, 2, Underline, 0)); n < 3 {
			t.Errorf("an underline of gypsy in %d pieces", n)
		}
	}
}
