package ui

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/scene"
)

var wideGreen = Oklch(0.85, 0.3, 145)

func TestOklchInsideSRGBHasNoWideColor(t *testing.T) {
	g := Oklch(0.6, 0, 0)
	if g.wide.ok() || g.A != 255 || g.R != g.G || g.G != g.B {
		t.Errorf("gray = %+v", g)
	}
	if got := Oklch(1, 0.2, 30); got != RGB(255, 255, 255) {
		t.Errorf("white = %+v", got)
	}
}

func TestOklchOutsideSRGBKeepsWideColor(t *testing.T) {
	a, ok := wideGreen.wideRGBA()
	if !ok {
		t.Fatal("vivid green has no wide color")
	}
	if !(a[0] < 0 || a[1] > 1 || a[2] < 0 || a[0] > 1 || a[1] < 0 || a[2] > 1) {
		t.Errorf("wide = %v, want a component outside 0 to 1", a)
	}
	if wideGreen.G <= wideGreen.R || wideGreen.G <= wideGreen.B {
		t.Errorf("sRGB fallback = %+v", wideGreen)
	}
	if wideGreen.SRGB().wide.ok() {
		t.Error("SRGB keeps the wide color")
	}
}

func TestOklchOddInput(t *testing.T) {
	c := Oklch(0.85, 0.3, 145).Alpha(0.5)
	if a, ok := c.wideRGBA(); !ok || a[3] < 0.49 || a[3] > 0.51 || c.A != 128 {
		t.Errorf("alpha: %v %v A=%d", a, ok, c.A)
	}
	nan, inf := float32(math.NaN()), float32(math.Inf(1))
	if n := Oklch(nan, 0.3, 145); n.wide.ok() || n != RGB(0, 0, 0) {
		t.Errorf("NaN lightness = %+v", n)
	}
	// A negative chroma is 0, as in CSS: a gray, not the opposite hue.
	if g := Oklch(0.7, -0.1, 30); g.wide.ok() || g.R != g.G || g.G != g.B {
		t.Errorf("negative chroma = %+v, want a gray", g)
	}
	// An infinite chroma is the most vivid color of the hue, at once.
	done := make(chan Color, 1)
	go func() { done <- Oklch(0.5, inf, 30) }()
	select {
	case c := <-done:
		if !c.wide.ok() {
			t.Errorf("infinite chroma = %+v, want a wide color", c)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Oklch with an infinite chroma does not return")
	}
	if c := Oklch(0.5, 0.2, inf); c.A != 255 {
		t.Errorf("infinite hue = %+v", c)
	}
}

func TestMixWide(t *testing.T) {
	white := RGB(255, 255, 255)
	// Halfway to white, the green is inside sRGB: its R, G and B are the
	// mix of the wide color, as a wide gamut draws it, and it has none.
	m := wideGreen.Mix(white, 0.5)
	wr, wg, wb := wideGreen.extended()
	want := func(v float32) uint8 { return uint8(max(0, min((v+1)/2, 1))*255 + 0.5) }
	if m.wide.ok() || m.R != want(wr) || m.G != want(wg) || m.B != want(wb) {
		t.Errorf("Mix(white, 0.5) = %+v, want %d %d %d without a wide color", m, want(wr), want(wg), want(wb))
	}
	if !wideGreen.Mix(RGB(0, 0, 0), 0.1).wide.ok() {
		t.Error("a mix outside sRGB drops the wide color")
	}
	// The ends are the colors mixed.
	if got := wideGreen.Mix(white, 0); got != wideGreen {
		t.Errorf("Mix(o, 0) = %+v, want %+v", got, wideGreen)
	}
	if got := white.Mix(wideGreen, 0); got != white {
		t.Errorf("an sRGB color mixed 0 of the way to a wide one = %+v", got)
	}
	if RGB(1, 2, 3).Mix(RGB(4, 5, 6), 0.5).wide.ok() {
		t.Error("Mix of sRGB colors has a wide color")
	}
	p := mixPremultiplied(Transparent, wideGreen, 0.5)
	if !p.wide.ok() || p.A != 128 || p.SRGB() != wideGreen.SRGB().Alpha(0.5) {
		t.Errorf("fading in from transparent = %+v, want %+v", p, wideGreen.Alpha(0.5))
	}
	if got := mixPremultiplied(wideGreen, wideGreen, 0.3); got != wideGreen {
		t.Errorf("a color mixed with itself = %+v", got)
	}
}

func TestBackgroundDrawsNearestSRGB(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(20, 20).Background(wideGreen)
	}, 20, 20)
	want, got := wideGreen, tt.Image().RGBAAt(10, 10)
	d := func(a, b uint8) int { return max(int(a)-int(b), int(b)-int(a)) }
	if d(got.R, want.R) > 1 || d(got.G, want.G) > 1 || d(got.B, want.B) > 1 {
		t.Errorf("drew %v, want %v", got, want)
	}
}

// wideOf returns the wide colors of an op or a glyph whose Wide is i.
func wideOf(t *testing.T, s *scene.Scene, i uint16) scene.WideColors {
	t.Helper()
	if i == 0 {
		return scene.WideColors{}
	}
	if int(i) > len(s.Wide) {
		t.Fatalf("Wide %d of %d", i, len(s.Wide))
	}
	return s.Wide[i-1]
}

func TestWideShadowStripesGradientAndText(t *testing.T) {
	w := wideGreen
	wa, _ := w.wideRGBA()
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(10).Padding(20).Children(func() {
			coreBox(c).Size(40, 20).Shadow(0, 4, 8, 0, RGB(0, 0, 0)).Shadow(0, 4, 8, 0, w).Background(RGB(255, 255, 255))
			coreBox(c).Size(40, 20).Background(RGB(255, 255, 255)).Stripes(w, 4, 4, 0)
			coreBox(c).Size(40, 20).Gradient(w, RGB(0, 0, 0), 90)
			coreText(c, "wide").TextColor(w)
			coreRichText(c, Span{Text: "span", Weight: 700, Color: w}, Span{Text: " plain"})
		})
	}, 200, 250)
	s := tt.h.last
	var shadows, stripes, gradients int
	for _, op := range s.Ops {
		ow := wideOf(t, s, op.Wide)
		switch {
		case op.Kind == scene.OpShadow && ow.Set&scene.WideColor != 0:
			shadows++
			if ow.Color != wa {
				t.Errorf("shadow wide color = %v, want %v", ow.Color, wa)
			}
		case op.Kind == scene.OpShadow:
			if op.Wide != 0 {
				t.Errorf("plain shadow has wide colors: %+v", ow)
			}
		case op.Paint == scene.PaintStripes && ow.Set == scene.WideColor:
			stripes++
			if ow.Color != wa || op.Color != w.scene() {
				t.Errorf("stripes: wide %v, color %v", ow.Color, op.Color)
			}
		case op.Paint == scene.PaintLinear && ow.Set == scene.WideColor:
			gradients++
		}
	}
	if shadows != 1 || stripes != 1 || gradients != 1 {
		t.Errorf("wide shadows %d, stripes %d, gradients %d, want 1 each", shadows, stripes, gradients)
	}
	var wideGlyphs, plainGlyphs int
	for _, g := range s.Glyphs {
		if g.Wide == 0 {
			plainGlyphs++
			continue
		}
		wideGlyphs++
		if gw := wideOf(t, s, g.Wide); gw.Color != wa {
			t.Fatalf("glyph wide color = %v, want %v", gw.Color, wa)
		}
		if g.Color != w.scene() {
			t.Errorf("glyph fallback color = %v", g.Color)
		}
	}
	if wideGlyphs < 8 || plainGlyphs == 0 {
		t.Errorf("wide glyphs %d, plain glyphs %d", wideGlyphs, plainGlyphs)
	}
	// Glyphs of one color share their wide colors.
	if len(s.Wide) > 6 {
		t.Errorf("%d wide colors", len(s.Wide))
	}
}

func TestWideOnlyWhereItShows(t *testing.T) {
	cases := map[string]func(c *context){
		"an sRGB text color after a wide one": func(c *context) {
			coreText(c, "plain").TextColor(wideGreen).TextColor(RGB(10, 20, 30))
		},
		"a border color without a border": func(c *context) {
			coreBox(c).Size(20, 20).Background(RGB(255, 255, 255)).BorderColor(wideGreen)
		},
		"a transparent wide color": func(c *context) {
			coreBox(c).Size(20, 20).Background(wideGreen.Alpha(0)).Border(1, RGB(0, 0, 0))
		},
	}
	for name, view := range cases {
		if tt := coreNewTester(view, 100, 40); len(tt.h.last.Wide) != 0 {
			t.Errorf("%s: wide colors %+v", name, tt.h.last.Wide)
		}
	}
}

func TestWideDecorationsDividersAndTextBackground(t *testing.T) {
	wa, _ := wideGreen.wideRGBA()
	tt := coreNewTester(func(c *context) {
		coreRichText(c, Span{Text: "marked", Underline: true, DecorationColor: wideGreen}, Span{Text: "bg", Background: wideGreen})
		coreColumn(c).Dividers(1, wideGreen).Children(func() {
			coreBox(c).Size(20, 10)
			coreBox(c).Size(20, 10)
		})
	}, 200, 80)
	s := tt.h.last
	var wide int
	for _, op := range s.Ops {
		if ow := wideOf(t, s, op.Wide); op.Kind == scene.OpFill && ow.Set&scene.WideColor != 0 {
			wide++
			if ow.Color != wa {
				t.Errorf("wide color = %v, want %v", ow.Color, wa)
			}
		}
	}
	if wide < 3 {
		t.Errorf("wide fills = %d, want at least 3 (underline, background, divider)", wide)
	}
}

func TestThemeAccentTakesOklch(t *testing.T) {
	th := *LightTheme()
	th.Accent = wideGreen
	if !th.Accent.wide.ok() {
		t.Error("Theme.Accent lost the wide color")
	}
}

// TestPictureOfAWideColor checks that a picture, drawn in sRGB, is not
// drawn again for a wide color of the same sRGB value.
func TestPictureOfAWideColor(t *testing.T) {
	logo := MustParseSVG([]byte(logoSVG))
	col := Oklch(0.85, 0.3, 145)
	tt := coreNewTester(func(c *context) { coreImage(c, logo).TextColor(col) }, 100, 100)
	other := col
	other.wide[1]++
	var version uint64
	for _, p := range tt.rt.svgs.pictures {
		version = p.img.Version()
	}
	col = other
	tt.Frame()
	for _, p := range tt.rt.svgs.pictures {
		if p.img.Version() != version {
			t.Error("the picture was drawn again")
		}
	}
}

// wideGPU is a GPU renderer that draws wide colors, as Metal's does.
type wideGPU struct {
	testGPU
	can, wide bool
	set       []bool
}

func (g *wideGPU) SetWide(on bool) bool {
	g.set = append(g.set, on)
	g.wide = on && g.can
	return g.wide
}

// wideSurface is a test surface on a screen that may show a wide gamut.
type wideSurface struct {
	testSurface
	wide bool
}

func (s *wideSurface) WideGamut() bool { return s.wide }

var _ platform.WideGamutSurface = (*wideSurface)(nil)

// TestWideFramesOnGPU checks that frames with wide colors draw in a wide
// gamut, only on a screen showing one, and for a while after the last.
func TestWideFramesOnGPU(t *testing.T) {
	for _, c := range []struct {
		name        string
		screen, can bool
	}{{"a wide screen", true, true}, {"an sRGB screen", false, true}, {"a renderer that cannot", true, false}} {
		t.Run(c.name, func(t *testing.T) {
			g := &wideGPU{can: c.can}
			h, _, frame := gpuHost(t, func() (gpuRenderer, error) { return g, nil })
			h.conn.Surface = &wideSurface{wide: c.screen}
			caret := false
			h.rt = newRuntime(func(c *context) {
				coreBox(c).Fill().Background(RGB(200, 200, 200)).Children(func() {
					b := coreBox(c).Size(2, 10)
					if caret {
						b.Background(wideGreen)
					}
				})
			}, h)
			wide := c.screen && c.can
			step := func(what string, show, inWide bool) {
				t.Helper()
				caret = show
				frames := g.frames
				frame()
				if g.frames != frames+1 || g.wide != inWide {
					t.Fatalf("%s: %d frames on the GPU, wide %v", what, g.frames-frames, g.wide)
				}
			}
			step("no wide color", false, false)
			step("a wide color", true, wide)
			// The caret blinks: the frames go on in a wide gamut.
			step("the caret off", false, wide)
			step("the caret on", true, wide)
			step("the caret off again", false, wide)
			// Long after the last, the frames are sRGB again.
			h.wideUntil = time.Now().Add(-time.Millisecond)
			step("long after", false, false)
			switch {
			case !c.screen && len(g.set) != 0:
				t.Errorf("SetWide on an sRGB screen: %v", g.set)
			case wide && len(g.set) != 2:
				t.Errorf("SetWide %v, want once on and once off", g.set)
			}
		})
	}
}

// TestEffectColor checks that an effect's color outside the sRGB gamut
// comes in extended sRGB, and has the frame draw in a wide gamut.
func TestEffectColor(t *testing.T) {
	var srgb, wide [4]float32
	var plain bool
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(10, 10).Draw(func(p *Painter, r Rect) {
			_, w := p.EffectColor(RGB(255, 0, 0))
			plain = w == [4]float32{1, 0, 0, 1} && len(p.s.Wide) == 0
			srgb, wide = p.EffectColor(wideGreen.Alpha(0.5))
		})
	}, 20, 20)
	wa, _ := wideGreen.Alpha(0.5).wideRGBA()
	if !plain || wide != wa || srgb[1] != float32(wideGreen.G)/255 || srgb[3] != float32(128)/255 {
		t.Errorf("EffectColor: %v and %v, plain %v", srgb, wide, plain)
	}
	if len(tt.h.last.Wide) != 1 {
		t.Errorf("%d wide colors", len(tt.h.last.Wide))
	}
}
