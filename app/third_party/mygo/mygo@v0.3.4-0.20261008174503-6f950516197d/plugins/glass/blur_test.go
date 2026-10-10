package glass

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/internal/raster"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/ui"
)

// TestBlurLevels checks that a varying blur's levels add up to its most,
// halving from there down to the finest, and that one that does not vary
// is a level showing everywhere.
func TestBlurLevels(t *testing.T) {
	for _, c := range []struct {
		least, most float32
		n           int
	}{{0, 24, 5}, {0, 2, 1}, {0, 0.5, 1}, {5, 24, 4}, {3, 3, 1}, {0, 0, 0}} {
		levels := blurLevels(c.least, c.most)
		if len(levels) != c.n {
			t.Errorf("%v to %v: %d levels, not %d: %v", c.least, c.most, len(levels), c.n, levels)
			continue
		}
		if c.n == 0 {
			continue
		}
		// The variances add up to the most's.
		var v float64
		for _, l := range levels {
			v += float64(l.blur * l.blur)
		}
		if math.Abs(math.Sqrt(v)-float64(max(c.most, c.least))) > 1e-3 {
			t.Errorf("%v to %v: the levels blur by %v in all: %v", c.least, c.most, math.Sqrt(v), levels)
		}
		last := levels[len(levels)-1]
		if c.most > c.least && last.hi != c.most {
			t.Errorf("%v to %v: the last level ends at %v", c.least, c.most, last.hi)
		}
		for i := 1; i < len(levels); i++ {
			if levels[i].lo != levels[i-1].hi && levels[i-1].lo >= 0 {
				t.Errorf("%v to %v: level %d starts at %v, after %v", c.least, c.most, i, levels[i].lo, levels[i-1].hi)
			}
		}
		if first := levels[0]; first.lo >= 0 && first.hi > finestLevel && first.lo == 0 {
			t.Errorf("%v to %v: the first level blurs by %v, over what is not blurred", c.least, c.most, first.hi)
		}
	}
}

// TestBlurWanted checks that a level's rectangle holds every pixel it
// shows at, and little more, at any angle of the mask.
func TestBlurWanted(t *testing.T) {
	r := scene.Rect{X: 10, Y: 20, W: 200, H: 80}
	for _, g := range []ui.LinearGradient{
		{From: ui.RGB(0, 0, 0), Angle: 180},
		{From: ui.RGB(0, 0, 0), Angle: 0},
		{From: ui.RGB(0, 0, 0), Angle: 90, Start: 0.2, End: 0.6},
		{From: ui.RGB(0, 0, 0), Angle: 30},
		{From: ui.RGBA(0, 0, 0, 0.3), To: ui.RGB(0, 0, 0), Angle: 225},
		{From: ui.RGB(0, 0, 0), Angle: 180, Start: 0.5, End: 0.5},
	} {
		at0, at1 := 20*float32(g.From.A)/255, 20*float32(g.To.A)/255
		line := maskLine(r, g)
		for _, l := range blurLevels(min(at0, at1), max(at0, at1)) {
			if l.lo < 0 {
				continue
			}
			px := blurPixels{line: line, band: [4]float32{at0, at1, l.lo, l.hi}}
			got, ok := blurWanted(r, line, at0, at1, l.lo)
			shown := image.Rectangle{}
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					if px.weight(float32(x)+0.5, float32(y)+0.5) > 0 {
						shown = shown.Union(image.Rect(x, y, x+1, y+1))
					}
				}
			}
			want := image.Rect(int(got.X), int(got.Y), int(got.X+got.W), int(got.Y+got.H))
			if !ok {
				want = image.Rectangle{}
			}
			if !shown.In(want) {
				t.Errorf("%+v, level %v: shows at %v, outside %v", g, l, shown, want)
			}
			// At most two pixels more on each side, where the edge of the
			// level, slanted, crosses the pixels' rows between their
			// centers.
			if !shown.Empty() && !want.In(shown.Inset(-2)) {
				t.Errorf("%+v, level %v: %v is much larger than where it shows, %v", g, l, want, shown)
			}
		}
	}
}

// stripes paints black columns period/2 DIPs wide, period apart, over
// white: what blurs to gray.
func stripes(c *ui.Context, period float32) {
	ui.Box(c).Fill().Background(ui.RGB(255, 255, 255)).Draw(func(p *ui.Painter, r ui.Rect) {
		for x := float32(0); x < r.W; x += period {
			p.Fill(ui.Rect{X: r.X + x, Y: r.Y, W: period / 2, H: r.H}, ui.RGB(0, 0, 0), 0)
		}
	})
}

// contrast returns how far apart the darkest and the lightest pixels of
// row y are, from column x0 to x1: how much of the stripes shows there.
func contrast(img *image.RGBA, y, x0, x1 int) int {
	lo, hi := 255, 0
	for x := x0; x < x1; x++ {
		v := int(img.RGBAAt(x, y).R)
		lo, hi = min(lo, v), max(hi, v)
	}
	return hi - lo
}

func TestBlurBlursWhatIsBehind(t *testing.T) {
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			stripes(c, 2)
			ui.Box(c).Absolute().Left(0).Top(0).Size(200, 30).Material(Blur{Radius: 4})
		})
	}
	tt := ui.NewTester(view, 200, 60)
	img := tt.Image()
	if got := contrast(img, 15, 20, 180); got > 2 {
		t.Errorf("under the blur, the stripes' contrast is %d", got)
	}
	// Stripes average to gray.
	if g := pixelAt(img, 100, 15)[0]; g < 115 || g > 140 {
		t.Errorf("under the blur, the stripes are %d", g)
	}
	if got := contrast(img, 45, 20, 180); got != 255 {
		t.Errorf("below the blur, the stripes' contrast is %d", got)
	}
}

func TestProgressiveBlur(t *testing.T) {
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			stripes(c, 8)
			ui.Box(c).Absolute().Left(0).Top(10).Right(0).Height(80).
				Material(Blur{Radius: 6, Mask: &ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180}})
		})
	}
	tt := ui.NewTester(view, 100, 100)
	tt.SetScale(2)
	img := tt.Image()
	// The blur fades from its top, 12 pixels, to nothing at its bottom:
	// the stripes, 16 pixels apart, show more and more toward its bottom,
	// and wholly below it.
	last := -1
	for y := 20; y < 180; y += 4 {
		got := contrast(img, y, 40, 160)
		if got+1 < last {
			t.Errorf("row %d has a contrast of %d, less than above it (%d)", y, got, last)
		}
		last = max(last, got)
	}
	if top := contrast(img, 22, 40, 160); top > 1 {
		t.Errorf("at the top, the stripes' contrast is %d", top)
	}
	// Halfway, the blur is 6 pixels, which leaves 6% of the stripes'
	// first harmonic: e^(-2π²·6²/16²).
	if mid := contrast(img, 100, 40, 160); mid < 10 || mid > 40 {
		t.Errorf("halfway, the stripes' contrast is %d", mid)
	}
	if low := contrast(img, 176, 40, 160); low < 200 {
		t.Errorf("at the bottom, the stripes' contrast is %d", low)
	}
	if below := contrast(img, 182, 40, 160); below != 255 {
		t.Errorf("below the blur, the stripes' contrast is %d", below)
	}
}

func TestBlurString(t *testing.T) {
	if s := (Blur{Radius: 12}).String(); s != "blur(12)" {
		t.Errorf("String: %q", s)
	}
	b := Blur{Radius: 8, Mask: &ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180}}
	if s := b.String(); s != "blur(8) mask(100%→0% 180°)" {
		t.Errorf("String: %q", s)
	}
}

// blurOp appends the levels of a blur of most pixels over r to s, along
// mask, as paintBlur paints them.
func blurOp(s *scene.Scene, r scene.Rect, radii [4]float32, most float32, mask *ui.LinearGradient, opacity float32) {
	blurOps(r, radii == [4]float32{}, most, mask, func(lr scene.Rect, fx scene.EffectOp) {
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpEffect, Rect: lr, Radii: radii, Start: int32(len(s.Effects)), Opacity: opacity})
		s.Effects = append(s.Effects, fx)
	})
}

// blurScene returns a 320×140 scene of blurs over stripes, which the
// renderers must draw alike: an even blur, rounded; a progressive blur
// from the top, and one at an angle within a rounded clip; a blur with a
// plateau, half transparent; a blur over what is partly transparent; and
// a hard scroll edge's tone.
func blurScene() *scene.Scene {
	s := &scene.Scene{Width: 320, Height: 140}
	red := scene.Color{R: 220, G: 40, B: 40, A: 255}
	blue := scene.Color{R: 37, G: 99, B: 235, A: 255}
	yellow := scene.Color{R: 250, G: 204, B: 21, A: 255}
	ink := scene.Color{R: 20, G: 24, B: 32, A: 255}
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{W: 260, H: 140}, Color: scene.Color{R: 246, G: 247, B: 249, A: 255}})
	for i := range 13 {
		c := []scene.Color{red, blue, yellow, ink}[i%4]
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i * 20), W: 8, H: 140}, Color: c})
	}
	for i := range 14 {
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{Y: float32(i * 10), W: 260, H: 2}, Color: ink})
	}
	black := ui.RGB(0, 0, 0)
	blurOp(s, scene.Rect{X: 10, Y: 10, W: 90, H: 50}, r4(14), 6, nil, 0)
	blurOp(s, scene.Rect{X: 110, Y: 0, W: 120, H: 60}, [4]float32{}, 16, &ui.LinearGradient{From: black, To: ui.Transparent, Angle: 180}, 0)
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 20, Y: 70, W: 120, H: 60}, Radii: r4(16)})
	blurOp(s, scene.Rect{X: 10, Y: 66, W: 140, H: 70}, [4]float32{}, 10, &ui.LinearGradient{From: ui.Transparent, To: black, Angle: 60}, 0)
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPopClip})
	blurOp(s, scene.Rect{X: 150.5, Y: 70.25, W: 60, H: 60}, [4]float32{}, 8,
		&ui.LinearGradient{From: ui.RGBA(0, 0, 0, 0.4), To: black, Angle: 90, Start: 0.3, End: 0.7}, 0.6)
	blurOp(s, scene.Rect{X: 230, Y: 20, W: 70, H: 100}, r4(10), 5, nil, 0)
	// A hard scroll edge's tone, over the edge of nothing.
	p2, p3 := blurTone{saturation: 0.25, offset: 0.03, mix: 0.6, color: [3]float32{0.9, 0.95, 1}}.params()
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpEffect, Rect: scene.Rect{X: 234, Y: 0, W: 86, H: 16}, Start: int32(len(s.Effects))})
	s.Effects = append(s.Effects, scene.EffectOp{Effect: blurEffect, Blur: 4, Params: [5][4]float32{{}, {4, 4, -1, 0}, p2, p3}})
	return s
}

// TestBlurScene checks the CPU's drawing of the blurs: what is not under
// them shows as it is, what is under the most blurred is a mix of the
// stripes, and the nothing beside the stripes stays nothing.
func TestBlurScene(t *testing.T) {
	s := blurScene()
	img := raster.NewImage(s.Width, s.Height)
	raster.Render(img, s)
	at := func(x, y int) [4]byte {
		p := img.Pix[y*img.Stride+4*x:]
		return [4]byte{p[2], p[1], p[0], p[3]}
	}
	if p := at(5, 105); p != [4]byte{220, 40, 40, 255} {
		t.Errorf("outside the blurs: %v", p)
	}
	// Over nothing, the blur paints nothing; along the edge of what is
	// under it, that and nothing blur together, translucent.
	if p := at(290, 70); p[3] != 0 {
		t.Errorf("the blur over nothing: %v", p)
	}
	if p := at(260, 70); p[3] < 90 || p[3] > 165 {
		t.Errorf("the blur along the edge of nothing: %v", p)
	}
	// Under the top of the progressive blur, the stripes are gone.
	lo, hi := 255, 0
	for x := 112; x < 228; x++ {
		v := int(at(x, 2)[1])
		lo, hi = min(lo, v), max(hi, v)
	}
	if hi-lo > 40 {
		t.Errorf("under the top of the progressive blur, the row varies from %d to %d", lo, hi)
	}
}

// BenchmarkBlur measures drawing a window with a blur under its toolbar
// on the CPU, whole: 1360×720 pixels, stripes under a strip 176 pixels
// high blurred by 24 pixels, evenly or fading from the top.
func BenchmarkBlur(b *testing.B) {
	for _, bc := range []struct {
		name string
		mask *ui.LinearGradient
	}{{"even", nil}, {"progressive", &ui.LinearGradient{From: ui.RGB(0, 0, 0), To: ui.Transparent, Angle: 180}}} {
		b.Run(bc.name, func(b *testing.B) {
			s := &scene.Scene{Width: 1360, Height: 720, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}}
			colors := []scene.Color{{R: 239, G: 68, B: 68, A: 255}, {R: 245, G: 158, B: 11, A: 255}, {R: 16, G: 185, B: 129, A: 255}, {R: 59, G: 130, B: 246, A: 255}}
			for i := range 17 {
				s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i * 80), W: 40, H: 720}, Color: colors[i%len(colors)]})
			}
			blurOp(s, scene.Rect{W: 1360, H: 176}, [4]float32{}, 24, bc.mask, 0)
			img := raster.NewImage(s.Width, s.Height)
			for b.Loop() {
				raster.Render(img, s)
			}
		})
	}
}
