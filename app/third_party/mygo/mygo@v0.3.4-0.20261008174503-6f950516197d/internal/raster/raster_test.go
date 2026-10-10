package raster

import (
	"image"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

// writePNG saves an image for inspection when MYGO_RASTER_PNG names a
// directory.
func writePNG(t *testing.T, name string, m *Image) {
	dir := os.Getenv("MYGO_RASTER_PNG")
	if dir == "" {
		return
	}
	img := &image.RGBA{Pix: m.RGBA(), Stride: 4 * m.W, Rect: image.Rect(0, 0, m.W, m.H)}
	f, err := os.Create(dir + "/" + name + ".png")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func pixel(m *Image, x, y int) [4]byte {
	p := m.Pix[y*m.Stride+4*x:]
	return [4]byte{p[2], p[1], p[0], p[3]} // RGBA
}

func near(a, b byte) bool { return a >= b-2 && a <= b+2 || a == b }

func TestRenderShapes(t *testing.T) {
	s := &scene.Scene{Width: 200, Height: 120, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}}
	s.Ops = append(s.Ops,
		scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 20, Y: 24, W: 80, H: 60}, Radii: [4]float32{12, 12, 12, 12}, Color: scene.Color{R: 0, G: 0, B: 0, A: 80}, Blur: 16},
		scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 20, Y: 20, W: 80, H: 60}, Radii: [4]float32{12, 12, 12, 12}, Color: scene.Color{R: 37, G: 99, B: 235, A: 255}, Border: scene.Uniform(2), BorderColor: scene.Color{R: 0, G: 0, B: 0, A: 255}},
		scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 120, Y: 20, W: 60, H: 60}, Radii: [4]float32{30, 30, 30, 30}},
		scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 110, Y: 10, W: 80, H: 80}, Color: scene.Color{R: 220, G: 38, B: 38, A: 255}, Color2: scene.Color{R: 250, G: 204, B: 21, A: 255}, Paint: scene.PaintLinear, Gradient: [4]float32{110, 10, 190, 90}},
		scene.Op{Kind: scene.OpPopClip},
	)
	dst := NewImage(s.Width, s.Height)
	Render(dst, s)
	writePNG(t, "shapes", dst)

	if p := pixel(dst, 60, 50); p != [4]byte{37, 99, 235, 255} {
		t.Errorf("inside the rectangle: %v", p)
	}
	if p := pixel(dst, 20, 50); p[0] > 40 || p[2] > 40 {
		t.Errorf("on the border: %v", p)
	}
	if p := pixel(dst, 21, 21); p[0] < 200 {
		t.Errorf("outside the rounded corner: %v", p)
	}
	if p := pixel(dst, 60, 90); p[0] > 250 || p[0] < 150 {
		t.Errorf("in the shadow below: %v", p)
	}
	if p := pixel(dst, 150, 50); p[3] != 255 || p[0] < 200 {
		t.Errorf("in the clipped gradient: %v", p)
	}
	if p := pixel(dst, 122, 22); p != [4]byte{255, 255, 255, 255} {
		t.Errorf("outside the round clip: %v", p)
	}
}

func TestRenderText(t *testing.T) {
	sys := text.Shared()
	l := sys.Layout(text.Params{Text: "Hello, MyGo", Style: text.Style{Size: 20}})
	s := &scene.Scene{Width: 160, Height: 40, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}, MaskAtlas: sys.MaskAtlas, ColorAtlas: sys.ColorAtlas}
	start := len(s.Glyphs)
	for _, g := range l.Lines[0].Glyphs {
		gi := sys.Glyph(g.Font, g.ID, 1, 0, 0, false)
		if !gi.OK {
			continue
		}
		s.Glyphs = append(s.Glyphs, scene.Glyph{X: 8 + g.X + gi.Left, Y: 4 + g.Y + gi.Top, W: float32(gi.W), H: float32(gi.H), U: gi.X, V: gi.Y, UW: gi.W, VH: gi.H, Color: scene.Color{R: 17, G: 24, B: 39, A: 255}})
	}
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpGlyphs, Start: int32(start), End: int32(len(s.Glyphs))})
	dst := NewImage(s.Width, s.Height)
	Render(dst, s)
	writePNG(t, "text", dst)
	dark := 0
	for y := 0; y < dst.H; y++ {
		for x := 0; x < dst.W; x++ {
			if pixel(dst, x, y)[0] < 100 {
				dark++
			}
		}
	}
	if dark < 100 {
		t.Errorf("%d dark pixels", dark)
	}
}

// Opaque mask text must blend as the floating-point path does, including
// over translucent destinations. Rounding differs by at most one byte.
func TestOpaqueMaskCoverage(t *testing.T) {
	colors := []scene.Color{{A: 255}, {R: 255, G: 255, B: 255, A: 255}, {R: 37, G: 99, B: 235, A: 255}}
	for _, c := range colors {
		for m := range 256 {
			for dst := range 256 {
				p := [4]byte{byte(dst), byte(dst), byte(dst), byte(dst)}
				want := p
				blend(want[:], c.Premul(1), float32(m)/255)
				p[0], p[1], p[2], p[3] = mixMask(c.B, p[0], uint32(m)), mixMask(c.G, p[1], uint32(m)), mixMask(c.R, p[2], uint32(m)), over(byte(m), p[3], uint32(255-m))
				for i := range p {
					if d := int(p[i]) - int(want[i]); d < -1 || d > 1 {
						t.Fatalf("color %+v, mask %d, destination %d: %v, want %v", c, m, dst, p, want)
					}
				}
			}
		}
	}
}

// BenchmarkRenderFullFrame renders a frame of 1600×1000 pixels with twenty
// tall rounded boxes on the CPU.
func BenchmarkRenderFullFrame(b *testing.B) {
	s := &scene.Scene{Width: 1600, Height: 1000, Clear: scene.Color{R: 250, G: 250, B: 250, A: 255}}
	for i := 0; i < 20; i++ {
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(40 + i*60), Y: 40, W: 50, H: 900}, Radii: [4]float32{8, 8, 8, 8}, Color: scene.Color{R: 37, G: 99, B: 235, A: 255}, Border: scene.Uniform(1), BorderColor: scene.Color{R: 0, G: 0, B: 0, A: 40}})
	}
	dst := NewImage(s.Width, s.Height)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Render(dst, s)
	}
}

// shadowAt is the blurred rounded box of Evan Wallace, computed for every
// pixel with the exact error function: what shadow approximates.
func shadowAt(px, py, hx, hy, sigma, corner float64) float64 {
	low, high := py-hy, py+hy
	start := math.Min(math.Max(-3*sigma, low), high)
	end := math.Min(math.Max(3*sigma, low), high)
	step := (end - start) / 4
	y := start + step/2
	var v float64
	for range 4 {
		delta := math.Min(hy-corner-math.Abs(py-y), 0)
		curved := hx - corner + math.Sqrt(math.Max(0, corner*corner-delta*delta))
		k := math.Sqrt(0.5) / sigma
		g := math.Exp(-y*y/(2*sigma*sigma)) / (math.Sqrt(2*math.Pi) * sigma)
		v += 0.5 * (math.Erf((px+curved)*k) - math.Erf((px-curved)*k)) * g * step
		y += step
	}
	return v
}

func TestShadowShowsOutsideItsCast(t *testing.T) {
	r4 := func(r float32) [4]float32 { return [4]float32{r, r, r, r} }
	for _, c := range []struct {
		shadow, cast scene.Rect
		radius, blur float32
		clip         bool
	}{
		{scene.Rect{X: 40, Y: 46, W: 120, H: 80}, scene.Rect{X: 40, Y: 40, W: 120, H: 80}, 12, 16, false},
		{scene.Rect{X: 63.5, Y: 70.25, W: 50, H: 40}, scene.Rect{X: 60.5, Y: 60.25, W: 56, H: 46}, 20, 9, true},
		{scene.Rect{X: 30, Y: 30, W: 200, H: 20}, scene.Rect{X: 100, Y: 20, W: 40, H: 80}, 0, 30, false},
		{scene.Rect{X: 52, Y: 54, W: 60, H: 30}, scene.Rect{X: 50, Y: 50, W: 60, H: 30}, 8, 0, true},
	} {
		render := func(cast scene.Rect) *Image {
			s := &scene.Scene{Width: 240, Height: 160, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}}
			if c.clip {
				s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 20, Y: 20, W: 110.5, H: 100}, Radii: r4(10)})
			}
			s.Ops = append(s.Ops, scene.Op{Kind: scene.OpShadow, Rect: c.shadow, Radii: r4(c.radius), Color: scene.Color{A: 255},
				Blur: c.blur, Cast: cast, CastRadii: r4(c.radius)})
			if c.clip {
				s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPopClip})
			}
			m := NewImage(s.Width, s.Height)
			Render(m, s)
			return m
		}
		plain, cut := render(scene.Rect{}), render(c.cast)
		cast := newShape(c.cast, scene.FitRadii(c.cast, r4(c.radius)))
		worst := 0.0
		for y := range 160 {
			for x := range 240 {
				out := 1 - float64(coverage(&cast, float32(x)+0.5, float32(y)+0.5))
				want := 255 - (255-float64(pixel(plain, x, y)[0]))*out
				worst = math.Max(worst, math.Abs(float64(pixel(cut, x, y)[0])-want))
			}
		}
		if worst > 2 {
			t.Errorf("%+v: off by %.1f of 255", c, worst)
		}
	}
}

func TestShadowMatchesItsFormula(t *testing.T) {
	for _, c := range []struct {
		rect         scene.Rect
		radius, blur float32
	}{
		{scene.Rect{X: 40, Y: 30, W: 200, H: 120}, 16, 24},
		{scene.Rect{X: 60, Y: 50, W: 120, H: 40}, 4, 6},
		{scene.Rect{X: 30.5, Y: 20.25, W: 50, H: 180}, 30, 60},
		{scene.Rect{X: 80, Y: 80, W: 10, H: 10}, 0, 40},
	} {
		s := &scene.Scene{Width: 320, Height: 240, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}}
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpShadow, Rect: c.rect, Radii: [4]float32{c.radius, c.radius, c.radius, c.radius},
			Color: scene.Color{A: 255}, Blur: c.blur})
		m := NewImage(s.Width, s.Height)
		Render(m, s)
		cx, cy := float64(c.rect.X+c.rect.W/2), float64(c.rect.Y+c.rect.H/2)
		corner := float64(min(c.radius, c.rect.W/2, c.rect.H/2))
		worst := 0.0
		for y := range s.Height {
			for x := range s.Width {
				v := shadowAt(float64(x)+0.5-cx, float64(y)+0.5-cy, float64(c.rect.W/2), float64(c.rect.H/2), float64(c.blur/2), corner)
				want := 255 * (1 - math.Min(v, 1))
				got := float64(pixel(m, x, y)[0])
				worst = math.Max(worst, math.Abs(got-want))
			}
		}
		if worst > 3 {
			t.Errorf("%+v: off by %.1f of 255", c, worst)
		}
	}
}

// TestThinLineCoverage checks that boxes with square corners cover pixels
// by area, as lines thinner than a pixel need: 0.7 of a pixel inside one
// row, and a fifth and a half across two.
func TestThinLineCoverage(t *testing.T) {
	m := NewImage(10, 10)
	s := &scene.Scene{Width: 10, Height: 10, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}, Ops: []scene.Op{
		{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 2.2, W: 10, H: 0.7}, Color: scene.Color{A: 255}},
		{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 5.8, W: 10, H: 0.7}, Color: scene.Color{A: 255}},
	}}
	Render(m, s)
	for _, c := range []struct {
		y    int
		want float64
	}{{2, 0.7}, {5, 0.2}, {6, 0.5}} {
		if got := 1 - float64(pixel(m, 5, c.y)[0])/255; math.Abs(got-c.want) > 0.01 {
			t.Errorf("row %d covered %.3f, want %.3f", c.y, got, c.want)
		}
	}
}
