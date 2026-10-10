package ui

import "github.com/egoist/mygo/internal/scene"

// Material is what Element.Material fills an element with in place of a
// background: a look a package paints, as the glass plugin's Liquid Glass
// (github.com/egoist/mygo/plugins/glass).
type Material interface {
	// PaintMaterial paints the material over box, rounded by radii, in
	// DIPs, under the element's border and children.
	PaintMaterial(p *Painter, box Rect, radii [4]float32)
}

// MaterialBuilder is a Material that follows the element it fills, as
// its state: Element.Material calls BuildMaterial as the element is
// built, and paints the material it returns.
type nodeMaterialBuilder interface {
	Material
	BuildMaterial(e *node) Material
}

// Material fills the element with m in place of a background, shaped by
// its Radius:
//
//	ui.Row(c).Padding(8, 16).Radius(22).Material(glass.Glass{})
func (e *node) Material(m Material) *node {
	if b, ok := m.(nodeMaterialBuilder); ok {
		m = b.BuildMaterial(e)
	}
	e.material, e.fill = m, fillMaterial
	return e
}

// Theme returns the theme of the view being painted.
func (p *Painter) Theme() *Theme { return p.rt.c.theme }

// EffectColor returns c as a parameter of an effect: straight RGBA from 0
// to 1, its sRGB value, and wide, its color outside the sRGB gamut in
// extended sRGB, which the effect's Metal shader takes where e.wide tells
// that the target keeps it (wide is the sRGB value for a color inside the
// gamut). Like a color an element paints, it has the frame drawn in a wide
// gamut where the window shows one.
func (p *Painter) EffectColor(c Color) (srgb, wide [4]float32) {
	srgb = [4]float32{float32(c.R) / 255, float32(c.G) / 255, float32(c.B) / 255, float32(c.A) / 255}
	wide = srgb
	if a, ok := c.wideRGBA(); ok {
		wide = a
		p.addWide(scene.WideColors{Color: a, Set: scene.WideColor})
	}
	return srgb, wide
}

// Effect paints the rectangle r rounded by radii, in DIPs, with an effect:
// shaders the renderers draw it with (see scene.Effect), which only this
// module's packages define, as the official plugins do. params are the
// effect's own, in device pixels (Scale), and blur the standard deviation
// of the blur of its backdrop, in device pixels, for an effect reading one.
func (p *Painter) Effect(fx *scene.Effect, r Rect, radii [4]float32, blur float32, params [5][4]float32) {
	if fx == nil || r.W <= 0 || r.H <= 0 {
		return
	}
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpEffect, Rect: p.snap(r), Radii: p.radii(radii), Continuous: continuousCorners,
		Start: int32(len(p.s.Effects)), Opacity: p.opacity})
	p.s.Effects = append(p.s.Effects, scene.EffectOp{Effect: fx, Blur: blur, Params: params})
}
