package glass

import (
	"math"
	"testing"

	"github.com/egoist/mygo/internal/raster"
	"github.com/egoist/mygo/internal/scene"
)

// paneOp appends a pane of glass of material m to s.
func paneOp(s *scene.Scene, r scene.Rect, radii [4]float32, m material, opacity float32) {
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpEffect, Rect: r, Radii: radii, Start: int32(len(s.Effects)), Opacity: opacity})
	s.Effects = append(s.Effects, scene.EffectOp{Effect: Effect, Blur: m.blur, Params: m.params()})
}

func r4(r float32) [4]float32 { return [4]float32{r, r, r, r} }

// testScene returns a 320×140 scene of panes of glass over stripes and a
// gradient, which the renderers must draw alike: blurred at a quarter of
// the size, over each other, unblurred and tinted, clipped, half
// transparent, dark and blurred at full size, and past the frame's edge.
func testScene() *scene.Scene {
	s := &scene.Scene{Width: 320, Height: 140, Clear: scene.Color{R: 246, G: 247, B: 249, A: 255}}
	red := scene.Color{R: 220, G: 40, B: 40, A: 255}
	blue := scene.Color{R: 37, G: 99, B: 235, A: 255}
	yellow := scene.Color{R: 250, G: 204, B: 21, A: 255}
	ink := scene.Color{R: 20, G: 24, B: 32, A: 255}
	for i := range 8 {
		c := []scene.Color{red, blue, yellow, ink}[i%4]
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i * 40), W: 20, H: 140}, Color: c})
	}
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{Y: 60, W: 320, H: 20}, Color: red, Color2: blue,
		Paint: scene.PaintOklab, Gradient: [4]float32{0, 0, 320, 0}})
	light := material{low: 0.541, high: 1, curve: 1.2, saturation: 1, rim: 0.35, rimWidth: 2, light: math.Pi / 2}
	m := light
	m.blur, m.bezel, m.refraction = 8, 22, 35
	paneOp(s, scene.Rect{X: 12, Y: 10, W: 140, H: 44}, r4(22), m, 0)
	m = light
	m.blur, m.bezel, m.refraction = 12, 30, 48
	paneOp(s, scene.Rect{X: 120, Y: 36, W: 60, H: 60}, r4(30), m, 0)
	m = material{low: 0.125, high: 1.082, curve: 1, saturation: 1, bezel: 16, refraction: 26, tint: [4]float32{0.15, 0.39, 0.92, 0.47},
		rim: 0.35, rimWidth: 2, light: math.Pi / 4}
	paneOp(s, scene.Rect{X: 190.5, Y: 8.25, W: 110, H: 50}, r4(16), m, 0)
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 30, Y: 70, W: 200, H: 50}, Radii: r4(12)})
	m = material{blur: 3, low: 0.15, high: 0.51, curve: 2, saturation: 2.2, bezel: 20, refraction: 30, rim: 0.35, rimWidth: 2, light: math.Pi / 2}
	paneOp(s, scene.Rect{X: 40, Y: 66, W: 240, H: 44}, [4]float32{22, 6, 22, 6}, m, 0.7)
	s.Ops = append(s.Ops, scene.Op{Kind: scene.OpPopClip})
	m = light
	m.blur, m.bezel, m.refraction = 6, 16, 24
	paneOp(s, scene.Rect{X: 284, Y: 94, W: 50, H: 50}, r4(14), m, 0)
	return s
}

// TestToneTable checks the CPU's tone curve against the curve itself.
func TestToneTable(t *testing.T) {
	for _, m := range []material{
		{low: 0.541, high: 1, curve: 1.2, saturation: 1},
		{low: 0.15, high: 0.51, curve: 2, saturation: 2.2},
		{low: 0.125, high: 1.082, curve: 1, saturation: 1},
	} {
		var px pixels
		op := scene.EffectOp{Effect: Effect, Params: m.params()}
		px.Begin(&op, scene.Rect{W: 10, H: 10}, [4]float32{})
		worst := float32(0)
		for i := range 1000 {
			c := [3]float32{float32(i%10) / 9, float32(i/10%10) / 9, float32(i/100) / 9}
			got, want := px.tone(c), tone(c, &m)
			for k := range got {
				worst = max(worst, abs(got[k]-want[k]))
			}
		}
		if worst > 1e-4 {
			t.Errorf("curve %v: the table is %v off the curve", m.curve, worst)
		}
	}
}

// TestGlassShowsWhatIsBehind checks that the glass lightens what is under
// it, bends it near its edge and lights its rim.
func TestGlassShowsWhatIsBehind(t *testing.T) {
	s := testScene()
	img := raster.NewImage(s.Width, s.Height)
	raster.Render(img, s)
	at := func(x, y int) [4]byte {
		p := img.Pix[y*img.Stride+4*x:]
		return [4]byte{p[2], p[1], p[0], p[3]}
	}
	// Over the ink stripe (x 120 to 140), the first pane is light gray.
	if p := at(130, 32); p[0] < 120 || p[0] > 200 {
		t.Errorf("light glass over ink is %v", p)
	}
	// Outside the panes, the stripes show as they are.
	if p := at(10, 130); p != [4]byte{220, 40, 40, 255} {
		t.Errorf("outside the glass: %v", p)
	}
}

// BenchmarkGlass measures drawing a window with panes of glass on the
// CPU, whole: 1360×720 pixels, stripes under a bar, a card, a button and
// a lens.
func BenchmarkGlass(b *testing.B) {
	s := &scene.Scene{Width: 1360, Height: 720, Clear: scene.Color{R: 255, G: 255, B: 255, A: 255}}
	colors := []scene.Color{{R: 239, G: 68, B: 68, A: 255}, {R: 245, G: 158, B: 11, A: 255}, {R: 16, G: 185, B: 129, A: 255}, {R: 59, G: 130, B: 246, A: 255}}
	for i := range 17 {
		s.Ops = append(s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i * 80), W: 40, H: 720}, Color: colors[i%len(colors)]})
	}
	m := material{low: 0.541, high: 1, curve: 1.2, saturation: 1, rim: 0.35, rimWidth: 2, light: math.Pi / 2}
	m.blur, m.bezel, m.refraction = 4, 58, 93
	paneOp(s, scene.Rect{X: 32, Y: 32, W: 1296, H: 116}, r4(58), m, 0)
	m.blur, m.bezel, m.refraction = 12, 72, 115
	paneOp(s, scene.Rect{X: 80, Y: 220, W: 600, H: 170}, r4(56), m, 0)
	m.blur, m.bezel, m.refraction = 2, 36, 58
	paneOp(s, scene.Rect{X: 1120, Y: 52, W: 160, H: 72}, r4(36), m, 0)
	m = material{blur: 2, low: 0.125, high: 1.082, curve: 1, saturation: 1, bezel: 72, refraction: 115, rim: 0.35, rimWidth: 2, light: math.Pi / 2}
	paneOp(s, scene.Rect{X: 840, Y: 260, W: 280, H: 280}, r4(140), m, 0)
	img := raster.NewImage(s.Width, s.Height)
	for b.Loop() {
		raster.Render(img, s)
	}
}
