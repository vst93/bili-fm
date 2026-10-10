package raster

import "github.com/egoist/mygo/internal/scene"

// Effects for tests: lensEffect shows its backdrop, sampled further in
// near its edge by Params[0][0] pixels a pixel, mixed with the color
// Params[1] by its alpha; tintEffect paints Params[1] alone.
var (
	lensEffect = &scene.Effect{Name: "lens", Backdrop: true, Pixels: func() scene.EffectPixels { return &testPixels{} }}
	tintEffect = &scene.Effect{Name: "tint", Pixels: func() scene.EffectPixels { return &testPixels{} }}
)

type testPixels struct {
	op    *scene.EffectOp
	rect  scene.Rect
	radii [4]float32
}

func (t *testPixels) Begin(op *scene.EffectOp, rect scene.Rect, radii [4]float32) {
	t.op, t.rect, t.radii = op, rect, radii
}

func (t *testPixels) Color(x, y float32, b *scene.BackdropImage) [4]float32 {
	tint := t.op.Params[1]
	if b == nil {
		return [4]float32{tint[0] * tint[3], tint[1] * tint[3], tint[2] * tint[3], tint[3]}
	}
	d := max(8+scene.SDRoundRect(t.rect, t.radii, x, y), 0) * t.op.Params[0][0]
	c := b.Sample(x+d, y)
	for i := range c {
		c[i] += (tint[i] - c[i]) * tint[3]
	}
	return [4]float32{c[0], c[1], c[2], 1}
}
