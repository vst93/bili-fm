// Package gputest has what the tests of the GPU renderers share: a scene
// that draws every kind of operation, and a comparison of what a renderer
// drew with what the CPU renderer draws, which the renderers must match.
package gputest

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/internal/raster"
	"github.com/egoist/mygo/internal/scene"
)

// Scene returns a 320×540 scene with fills, borders of every width and
// dashed, gradients mixed in sRGB and Oklab, stripes, shadows, blurred or
// not, and cut by the boxes casting them, nested rounded clips, glyphs from
// both atlases and subpixel ones, plain and in gradients, with Direct2D's
// gamma and contrast, images, in color and in gray, and effects, reading
// their backdrop blurred at each size or not, over each other, clipped
// and at the frame's edge, or reading none.
func Scene() *scene.Scene {
	s := &scene.Scene{Width: 320, Height: 540, Clear: scene.Color{R: 246, G: 247, B: 249, A: 255},
		Text: scene.TextParams{GammaRatios: scene.GammaRatios(1.8), Contrast: 1, SubpixelContrast: 0.5}}
	mask := scene.NewAtlas(1, 64, 64)
	color := scene.NewAtlas(4, 64, 32)
	s.MaskAtlas, s.ColorAtlas = mask, color
	// A disc and a ring as glyph masks, and a color glyph.
	disc := make([]byte, 16*16)
	ring := make([]byte, 12*12)
	for y := range 16 {
		for x := range 16 {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			disc[y*16+x] = byte(255 * clamp(7.5-math.Sqrt(dx*dx+dy*dy)))
			if x < 12 && y < 12 {
				ex, ey := float64(x)-5.5, float64(y)-5.5
				d := math.Sqrt(ex*ex + ey*ey)
				ring[y*12+x] = byte(255 * clamp(1.5-math.Abs(d-4)))
			}
		}
	}
	dx, dy, _ := mask.Alloc(16, 16)
	mask.Put(dx, dy, 16, 16, disc, 16)
	rx, ry, _ := mask.Alloc(12, 12)
	mask.Put(rx, ry, 12, 12, ring, 12)
	emoji := make([]byte, 10*10*4)
	for i := 0; i < len(emoji); i += 4 {
		k := i / 4
		// Premultiplied: half transparent in the corner rows.
		a := byte(255)
		if k < 10 || k >= 90 {
			a = 128
		}
		emoji[i], emoji[i+1], emoji[i+2], emoji[i+3] = byte(uint16(240)*uint16(a)/255), byte(uint16(160)*uint16(a)/255), byte(uint16(20)*uint16(a)/255), a
	}
	cx, cy, _ := color.Alloc(10, 10)
	color.Put(cx, cy, 10, 10, emoji, 40)
	// A subpixel disc: its red subpixels a third of a pixel left of its
	// green ones, its blue ones a third right.
	lcd := make([]byte, 16*16*4)
	for y := range 16 {
		for x := range 16 {
			for c := range 3 {
				dx, dy := float64(x)-7.5+float64(c-1)/3, float64(y)-7.5
				lcd[(y*16+x)*4+c] = byte(255 * clamp(6.5-math.Sqrt(dx*dx+dy*dy)))
			}
		}
	}
	lx, ly, _ := color.Alloc(16, 16)
	color.Put(lx, ly, 16, 16, lcd, 64)
	pix := make([]byte, 8*8*4)
	for y := range 8 {
		for x := range 8 {
			i := (y*8 + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(x*32), byte(y*32), 160, 255
		}
	}
	img := scene.NewImageRGBA(8, 8, pix)

	red := scene.Color{R: 220, G: 40, B: 40, A: 255}
	blue := scene.Color{R: 37, G: 99, B: 235, A: 255}
	ink := scene.Color{R: 20, G: 24, B: 32, A: 255}
	r4 := func(r float32) [4]float32 { return [4]float32{r, r, r, r} }
	add := func(op scene.Op) { s.Ops = append(s.Ops, op) }
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 20, Y: 24, W: 120, H: 80}, Radii: r4(12), Color: scene.Color{A: 90}, Blur: 16})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 20, Y: 20, W: 120, H: 80}, Radii: r4(12), Color: scene.Color{R: 255, G: 255, B: 255, A: 255},
		Border: scene.Uniform(1), BorderColor: scene.Color{R: 200, G: 204, B: 210, A: 255}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 160.5, Y: 20.25, W: 140, H: 30}, Radii: [4]float32{15, 4, 15, 4},
		Color: red, Color2: blue, Paint: scene.PaintLinear, Gradient: [4]float32{160, 20, 300, 50}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 170, Y: 60, W: 60, H: 40}, Color: scene.Color{R: 37, G: 99, B: 235, A: 128}, Border: scene.Uniform(3), BorderColor: red, Radii: r4(8)})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 240, Y: 60, W: 40, H: 40}, Radii: r4(20), Color: blue, Opacity: 0.5})
	add(scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 20, Y: 120, W: 200, H: 100}, Radii: r4(16)})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 110, W: 320, H: 130}, Color: scene.Color{R: 230, G: 236, B: 250, A: 255}})
	add(scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 100, Y: 130, W: 200, H: 70}})
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 120, Y: 150, W: 60, H: 30}, Radii: r4(6), Color: scene.Color{B: 80, A: 140}, Blur: 10})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 90, Y: 140, W: 50, H: 50}, Color: red, Radii: r4(4)})
	add(scene.Op{Kind: scene.OpPopClip})
	start := int32(len(s.Glyphs))
	for i := range 6 {
		s.Glyphs = append(s.Glyphs, scene.Glyph{X: float32(30 + i*20), Y: 200, W: 16, H: 16, U: uint16(dx), V: uint16(dy), UW: 16, VH: 16, Color: ink},
			scene.Glyph{X: float32(32 + i*20), Y: 182, W: 12, H: 12, U: uint16(rx), V: uint16(ry), UW: 12, VH: 12, Color: blue})
	}
	s.Glyphs = append(s.Glyphs, scene.Glyph{X: 160, Y: 180, W: 10, H: 10, U: uint16(cx), V: uint16(cy), UW: 10, VH: 10, Color: scene.Color{A: 255}, Colored: true})
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs))})
	add(scene.Op{Kind: scene.OpPopClip})
	add(scene.Op{Kind: scene.OpImage, Rect: scene.Rect{X: 240, Y: 120, W: 8, H: 8}, Image: img, Src: scene.Rect{W: 8, H: 8}})
	add(scene.Op{Kind: scene.OpImage, Rect: scene.Rect{X: 240, Y: 140, W: 64, H: 64}, Radii: r4(10), Image: img, Src: scene.Rect{W: 8, H: 8}})

	// Borders of different widths, dashed ones, Oklab gradients, stripes,
	// glyphs in gradients and a gray image.
	yellow := scene.Color{R: 250, G: 204, B: 21, A: 255}
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 20, Y: 240, W: 60, H: 30}, Radii: r4(10), Color: scene.Color{R: 255, G: 255, B: 255, A: 255},
		Border: [4]float32{1, 4, 2, 0}, BorderColor: ink})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 90, Y: 240, W: 60, H: 30}, Radii: r4(8), Border: scene.Uniform(2), BorderColor: blue, Dashed: true})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 160, Y: 240, W: 70, H: 30}, Color: yellow, Border: [4]float32{0, 0, 3, 1}, BorderColor: red, Dashed: true})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 240, Y: 240, W: 64, H: 30}, Radii: r4(4), Color: blue, Color2: yellow, Paint: scene.PaintOklab, Gradient: [4]float32{240, 0, 304, 0}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 20, Y: 280, W: 80, H: 30}, Color: red, Color2: scene.Color{}, Paint: scene.PaintLinear, Gradient: [4]float32{20, 0, 100, 0}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 110, Y: 280, W: 80, H: 30}, Radii: r4(6), Color: scene.Color{R: 37, G: 99, B: 235, A: 200}, Color2: yellow,
		Paint: scene.PaintStripes, Gradient: [4]float32{0.7071, -0.7071, 3, 8}, Border: scene.Uniform(1), BorderColor: ink})
	start = int32(len(s.Glyphs))
	for i := range 3 {
		s.Glyphs = append(s.Glyphs, scene.Glyph{X: float32(200 + i*18), Y: 288, W: 16, H: 16, U: uint16(dx), V: uint16(dy), UW: 16, VH: 16, Color: ink})
	}
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs)), Paint: scene.PaintOklab, Color: red, Color2: blue, Gradient: [4]float32{200, 288, 252, 304}})
	add(scene.Op{Kind: scene.OpImage, Rect: scene.Rect{X: 270, Y: 280, W: 32, H: 32}, Radii: r4(6), Image: img, Src: scene.Rect{W: 8, H: 8}, Grayscale: true})

	// Shadows that show only outside the boxes casting them: under a
	// translucent card, without blur, and alone.
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 320, W: 320, H: 60}, Color: yellow})
	card := scene.Rect{X: 20, Y: 330, W: 90, H: 36}
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 20, Y: 334, W: 90, H: 36}, Radii: r4(10), Color: scene.Color{A: 120}, Blur: 12, Cast: card, CastRadii: r4(10)})
	add(scene.Op{Kind: scene.OpFill, Rect: card, Radii: r4(10), Color: scene.Color{R: 255, G: 255, B: 255, A: 140}})
	card = scene.Rect{X: 130.5, Y: 330.25, W: 70, H: 30}
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 133.5, Y: 333.25, W: 70, H: 30}, Radii: r4(8), Color: ink, Cast: card, CastRadii: r4(8)})
	add(scene.Op{Kind: scene.OpFill, Rect: card, Radii: r4(8), Color: scene.Color{R: 37, G: 99, B: 235, A: 90}})
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 218, Y: 334, W: 84, H: 32}, Radii: r4(16), Color: red, Blur: 6,
		Cast: scene.Rect{X: 220, Y: 334, W: 80, H: 28}, CastRadii: r4(14)})

	// Subpixel glyphs, dark on white and light on dark, half transparent,
	// in a gradient, and clipped; thin mask glyphs, dark and light.
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 380, W: 160, H: 30}, Color: scene.Color{R: 255, G: 255, B: 255, A: 255}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 160, Y: 380, W: 160, H: 30}, Color: scene.Color{R: 24, G: 24, B: 27, A: 255}})
	sub := func(x float32, c scene.Color) scene.Glyph {
		return scene.Glyph{X: x, Y: 387, W: 16, H: 16, U: uint16(lx), V: uint16(ly), UW: 16, VH: 16, Color: c, Subpixel: true}
	}
	start = int32(len(s.Glyphs))
	s.Glyphs = append(s.Glyphs, sub(8, ink), sub(28, blue), sub(48, scene.Color{R: 20, G: 24, B: 32, A: 128}),
		sub(168, scene.Color{R: 255, G: 255, B: 255, A: 255}), sub(188, yellow), sub(208, scene.Color{R: 113, G: 113, B: 122, A: 255}),
		scene.Glyph{X: 68, Y: 387, W: 16, H: 16, U: uint16(dx), V: uint16(dy), UW: 16, VH: 16, Color: ink, Thin: true},
		scene.Glyph{X: 228, Y: 387, W: 16, H: 16, U: uint16(dx), V: uint16(dy), UW: 16, VH: 16, Color: scene.Color{R: 244, G: 244, B: 245, A: 255}, Thin: true})
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs))})
	start = int32(len(s.Glyphs))
	s.Glyphs = append(s.Glyphs, sub(92, ink), sub(110, ink), sub(128, ink))
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs)), Paint: scene.PaintLinear, Color: red, Color2: blue, Gradient: [4]float32{92, 387, 144, 403}})
	add(scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 250, Y: 384, W: 40, H: 22}, Radii: r4(8)})
	start = int32(len(s.Glyphs))
	s.Glyphs = append(s.Glyphs, sub(248, scene.Color{R: 255, G: 255, B: 255, A: 255}), sub(266, scene.Color{R: 255, G: 255, B: 255, A: 255}), sub(284, red))
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs))})
	add(scene.Op{Kind: scene.OpPopClip})

	// Panes of glass over stripes, a gradient and glyphs.
	for i := range 8 {
		c := []scene.Color{red, blue, yellow, ink}[i%4]
		add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: float32(i * 40), Y: 410, W: 20, H: 130}, Color: c})
	}
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 0, Y: 470, W: 320, H: 20}, Color: red, Color2: blue, Paint: scene.PaintOklab, Gradient: [4]float32{0, 0, 320, 0}})
	start = int32(len(s.Glyphs))
	for i := range 14 {
		s.Glyphs = append(s.Glyphs, scene.Glyph{X: float32(6 + i*22), Y: 516, W: 16, H: 16, U: uint16(dx), V: uint16(dy), UW: 16, VH: 16, Color: ink})
	}
	add(scene.Op{Kind: scene.OpGlyphs, Start: start, End: int32(len(s.Glyphs))})
	// Effects over them: reading their backdrop blurred at a quarter of
	// the size, over each other at the same size, unblurred, clipped and
	// half transparent at full size, past the frame's edge at half the
	// size; and one reading none.
	effect := func(e *scene.Effect, r scene.Rect, radii [4]float32, blur, shift float32, tint scene.Color, opacity float32) {
		fx := scene.EffectOp{Effect: e, Blur: blur}
		fx.Params[0] = [4]float32{shift}
		fx.Params[1] = [4]float32{float32(tint.R) / 255, float32(tint.G) / 255, float32(tint.B) / 255, float32(tint.A) / 255}
		add(scene.Op{Kind: scene.OpEffect, Rect: r, Radii: radii, Start: int32(len(s.Effects)), Opacity: opacity})
		s.Effects = append(s.Effects, fx)
	}
	white := scene.Color{R: 255, G: 255, B: 255, A: 90}
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 12, Y: 424, W: 140, H: 44}, Radii: r4(22), Color: scene.Color{A: 40}, Blur: 16,
		Cast: scene.Rect{X: 12, Y: 420, W: 140, H: 44}, CastRadii: r4(22)})
	effect(LensEffect, scene.Rect{X: 12, Y: 420, W: 140, H: 44}, r4(22), 8, 2, white, 0)
	effect(LensEffect, scene.Rect{X: 120, Y: 446, W: 60, H: 60}, r4(30), 12, 3, white, 0)
	effect(LensEffect, scene.Rect{X: 190.5, Y: 418.25, W: 110, H: 50}, r4(16), 0, 1.5, scene.Color{R: 37, G: 99, B: 235, A: 120}, 0)
	add(scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 30, Y: 480, W: 200, H: 50}, Radii: r4(12)})
	effect(LensEffect, scene.Rect{X: 40, Y: 476, W: 240, H: 44}, [4]float32{22, 6, 22, 6}, 3, 1, scene.Color{A: 60}, 0.7)
	add(scene.Op{Kind: scene.OpPopClip})
	effect(LensEffect, scene.Rect{X: 284, Y: 494, W: 50, H: 50}, r4(14), 6, 2, white, 0)
	effect(TintEffect, scene.Rect{X: 250, Y: 420, W: 30, H: 30}, r4(8), 0, 0, scene.Color{R: 220, G: 40, B: 40, A: 160}, 0)
	return s
}

// Compare compares pixels a renderer drew, premultiplied BGRA rows of
// stride bytes, with what the CPU renderer draws of s. Antialiased edges
// and interpolated images may differ a little; anything else fails t. With
// MYGO_GPU_PNG naming a directory, it saves both images there.
func Compare(t testing.TB, name string, got []byte, stride int, s *scene.Scene) {
	t.Helper()
	want := raster.NewImage(s.Width, s.Height)
	raster.Render(want, s)
	worst, off := 0, 0
	worstAt := image.Point{}
	for y := range s.Height {
		for x := range s.Width {
			g := got[y*stride+4*x:][:4]
			w := want.Pix[y*want.Stride+4*x:][:4]
			d := 0
			for c := range 4 {
				d = max(d, absInt(int(g[c])-int(w[c])))
			}
			if d > worst {
				worst, worstAt = d, image.Pt(x, y)
			}
			if d > 2 {
				off++
			}
		}
	}
	if dir := os.Getenv("MYGO_GPU_PNG"); dir != "" {
		g := &raster.Image{W: s.Width, H: s.Height, Stride: stride, Pix: got}
		save(t, filepath.Join(dir, name+".png"), g)
		save(t, filepath.Join(dir, name+"-cpu.png"), want)
	}
	// One pixel in two hundred may differ by more than 2 of 255, none by
	// more than 24.
	if worst > 24 || off*200 > s.Width*s.Height {
		t.Errorf("%s: %d pixels differ from the CPU renderer's by more than 2, the most by %d at %v", name, off, worst, worstAt)
	}
}

func save(t testing.TB, path string, m *raster.Image) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img := &image.RGBA{Pix: m.RGBA(), Stride: 4 * m.W, Rect: image.Rect(0, 0, m.W, m.H)}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(fmt.Errorf("%s: %w", path, err))
	}
}

func clamp(v float64) float64 { return max(0, min(1, v)) }

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// ContinuousScene returns a 320×70 scene with continuous corners, which
// only the renderers of macOS draw (scene.Op.Continuous): a card with a
// border and its shadow, a pill, a circle, which stays one, sides too short
// for their corners' curves, a corner reaching past the middle of its
// sides, a clip and an image, a shadow without blur, cut by the box
// casting it, and an effect reading its backdrop.
func ContinuousScene() *scene.Scene {
	s := &scene.Scene{Width: 320, Height: 70, Clear: scene.Color{R: 236, G: 238, B: 242, A: 255}}
	pix := make([]byte, 8*8*4)
	for y := range 8 {
		for x := range 8 {
			i := (y*8 + x) * 4
			pix[i], pix[i+1], pix[i+2], pix[i+3] = byte(x*32), byte(y*32), 160, 255
		}
	}
	img := scene.NewImageRGBA(8, 8, pix)
	red := scene.Color{R: 220, G: 40, B: 40, A: 255}
	blue := scene.Color{R: 37, G: 99, B: 235, A: 255}
	ink := scene.Color{R: 20, G: 24, B: 32, A: 255}
	yellow := scene.Color{R: 250, G: 204, B: 21, A: 255}
	r4 := func(r float32) [4]float32 { return [4]float32{r, r, r, r} }
	add := func(op scene.Op) {
		op.Continuous = true
		s.Ops = append(s.Ops, op)
	}
	card := scene.Rect{X: 10, Y: 8, W: 70, H: 44}
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 10, Y: 12, W: 70, H: 44}, Radii: r4(14), Color: scene.Color{A: 110}, Blur: 10,
		Cast: card, CastRadii: r4(14)})
	add(scene.Op{Kind: scene.OpFill, Rect: card, Radii: r4(14), Color: scene.Color{R: 255, G: 255, B: 255, A: 255},
		Border: scene.Uniform(2), BorderColor: blue})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 88.5, Y: 10.25, W: 70, H: 22}, Radii: r4(999), Color: red, Color2: blue,
		Paint: scene.PaintLinear, Gradient: [4]float32{88, 10, 158, 32}})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 88, Y: 38, W: 24, H: 24}, Radii: r4(12), Color: ink})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 118, Y: 38, W: 40, H: 24}, Radii: r4(10), Color: blue, Border: scene.Uniform(1), BorderColor: ink})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 166.5, Y: 8.25, W: 40, H: 54}, Radii: [4]float32{26, 6, 0, 0}, Color: red})
	add(scene.Op{Kind: scene.OpPushClip, Rect: scene.Rect{X: 214, Y: 6, W: 50, H: 58}, Radii: r4(16)})
	add(scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: 204, Y: 0, W: 70, H: 70}, Color: yellow})
	add(scene.Op{Kind: scene.OpImage, Rect: scene.Rect{X: 222, Y: 14, W: 34, H: 34}, Radii: r4(9), Image: img, Src: scene.Rect{W: 8, H: 8}})
	add(scene.Op{Kind: scene.OpPopClip})
	card = scene.Rect{X: 272.5, Y: 10.25, W: 40, H: 40}
	add(scene.Op{Kind: scene.OpShadow, Rect: scene.Rect{X: 275.5, Y: 14.25, W: 40, H: 40}, Radii: r4(12), Color: ink, Cast: card, CastRadii: r4(12)})
	// An effect reading its backdrop, over the pill and the card.
	fx := scene.EffectOp{Effect: LensEffect, Blur: 4}
	fx.Params[0] = [4]float32{2}
	fx.Params[1] = [4]float32{1, 1, 1, 0.35}
	add(scene.Op{Kind: scene.OpEffect, Rect: scene.Rect{X: 60, Y: 18, W: 70, H: 30}, Radii: r4(15), Start: int32(len(s.Effects))})
	s.Effects = append(s.Effects, fx)
	return s
}
