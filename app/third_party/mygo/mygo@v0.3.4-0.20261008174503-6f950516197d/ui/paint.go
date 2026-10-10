package ui

import (
	"math"
	"runtime"
	"slices"
	"time"

	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/text"
)

// Painter draws an element's own content in Element.Draw and DrawOver
// callbacks, in DIPs relative to the window.
type Painter struct {
	rt      *engine
	s       *scene.Scene
	scale   float32
	opacity float32
	clip    Rect
	// opaque tells that the glyphs painted now land on an opaque
	// background, which subpixel glyphs need: the root's, covering the
	// window, or that of an element around them, as a pane beside a
	// sidebar over the window's material. under holds the boxes of the
	// elements painting with an opaque background around the one painting.
	opaque bool
	under  []Rect
}

// continuousCorners curves rounded corners as Apple does, on macOS, where
// AppKit's and SwiftUI's controls have them (scene.Op.Continuous): circular
// elsewhere, as Windows and GTK draw them.
const continuousCorners = runtime.GOOS == "darwin"

func (rt *engine) paint(root *node, w, h, scale float32) {
	s := &rt.scene
	// The root paints the theme's background: frames start transparent, so
	// that a transparent root shows what is behind the content, such as a
	// window's vibrancy.
	s.Reset(int(math.Ceil(float64(w*scale))), int(math.Ceil(float64(h*scale))), scene.Color{})
	s.Scale = scale
	s.MaskAtlas, s.ColorAtlas = rt.text.MaskAtlas, rt.text.ColorAtlas
	s.Text = rt.text.TextParams()
	opaque := root.bg.A == 255 && root.fill == fillColor && (!root.opacitySet || root.opacity >= 1)
	// The painter is the engine's, which draw callbacks get, so that
	// painting allocates none.
	under := rt.under[:0]
	if opaque {
		// Under all the window paints, as what it drags.
		under = append(under, Rect{0, 0, w, h})
	}
	rt.painter = Painter{rt: rt, s: s, scale: scale, opacity: 1, clip: Rect{0, 0, w, h}, opaque: opaque, under: under}
	p := &rt.painter
	p.element(root)
	if rt.insp.open {
		rt.insp.paintHighlight(rt, p, h)
	}
	rt.paintDrag(p, w, h)
	rt.under = p.under[:0] // kept for the next frame's
}

// Now returns the time of the frame being painted, which drawings that
// move compute from.
func (p *Painter) Now() time.Time { return p.rt.c.now }

// AnimationFrame asks for the element to be painted again as soon as the
// display can show it, for a drawing that moves with Now while the layout
// stays, as a spinner's: unless something else changed meanwhile, the next
// frame paints the elements of this one again without building the view.
// Call it in every frame while the drawing moves; an element out of view
// is not painted, so it asks for none.
func (p *Painter) AnimationFrame() { p.rt.repainting = true }

// After asks for the element to be painted again after d, for a drawing
// that changes then, as a spinner's next step: unless something else
// changed meanwhile, that frame paints the elements of this one again
// without building the view. An element out of view is not painted, so
// it asks for none.
func (p *Painter) After(d time.Duration) {
	at := p.rt.c.now.Add(d)
	if p.rt.repaintAt.IsZero() || at.Before(p.rt.repaintAt) {
		p.rt.repaintAt = at
	}
}

// snap converts a rectangle to device pixels, rounding its edges to whole
// pixels so that edges stay crisp.
func (p *Painter) snap(r Rect) scene.Rect {
	s := p.scale
	x0, y0 := round(r.X*s), round(r.Y*s)
	x1, y1 := round((r.X+r.W)*s), round((r.Y+r.H)*s)
	return scene.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

func round(v float32) float32 { return float32(math.Round(float64(v))) }

func (p *Painter) radii(r [4]float32) [4]float32 {
	return [4]float32{r[0] * p.scale, r[1] * p.scale, r[2] * p.scale, r[3] * p.scale}
}

func (p *Painter) visible(r Rect, margin float32) bool {
	return r.X-margin < p.clip.X+p.clip.W && r.Y-margin < p.clip.Y+p.clip.H &&
		r.X+r.W+margin > p.clip.X && r.Y+r.H+margin > p.clip.Y
}

func (p *Painter) element(e *node) {
	if e.styleFn != nil {
		e.styleFn(e)
	}
	if r := e.st.trec; r != nil && r.built == p.rt.frame && r.elem == e {
		p.rt.animateColors(e, r)
	}
	if e.flags&flagInvisible != 0 {
		return
	}
	if e.isInline() {
		p.paintInline(e)
		return
	}
	saved := p.opacity
	if e.flags&flagDisabled != 0 {
		p.opacity *= 0.5
	}
	if e.opacitySet {
		p.opacity *= e.opacity
	}
	if p.opacity <= 0.001 {
		p.opacity = saved
		return
	}
	box := Rect{e.x, e.y, e.w, e.h}
	savedOpaque, under := p.opaque, len(p.under)
	if e.fill == fillColor && e.bg.A == 255 && p.opacity >= 1 {
		p.under = append(p.under, box)
	}
	// What the element paints lands on an opaque background when all of
	// it that shows does.
	p.opaque = p.covered(intersect(box, p.clip))
	margin := float32(0)
	for _, sh := range e.shadows {
		margin = max(margin, abs32(sh.x)+abs32(sh.y)+sh.blur+sh.spread)
	}
	clips := e.flags&(flagClipX|flagClipY|flagScrollX|flagScrollY) != 0
	// What an element clips meets its border where both are smoothed, on
	// rounded corners: painted over it there, it would thin the border. The
	// border goes over them instead.
	borderOver := clips && e.first != nil && scene.HasBorder(e.border) && e.borderC.A > 0
	own := p.visible(box, margin+4)
	if own {
		for _, sh := range e.shadows {
			p.shadow(box, e.radius, sh)
		}
		p.background(e, box, !borderOver)
		if e.paintFn != nil {
			e.paintFn(p, box)
		}
		switch e.kind {
		case kindText:
			ts := e.resolvedText()
			ox, oy := e.x+e.contentX(), e.y+e.contentY()
			if ed := e.st.editor; ed != nil && e.flags&flagSelectable != 0 && p.rt.textSelectionVisible(e.st) {
				if a, b := ed.selection(); a != b {
					for _, r := range e.tl.Selection(a, b) {
						p.Fill(Rect{ox + r.X, oy + r.Y, r.W, r.H}, ts.selectionColor(e.c.theme), 0)
					}
				}
			}
			var sp spanPaint
			p.textLayout(e.tl, ox, oy, ts.color, ts, e.paintSpans(&sp))
		case kindImage:
			p.image(e)
		case kindIcon:
			c := e.resolvedText().color
			if e.gray {
				c = c.gray()
			}
			p.drawIcon(e.svg, e.contentBox(), c, e.rotate)
		case kindInput:
			e.paintInput(p)
		}
	}
	savedClip := p.clip
	if clips {
		inner, rad := e.clipRect()
		p.pushClip(inner, rad)
	}
	if !clips || p.clip.W > 0 && p.clip.H > 0 {
		if e.ghosts {
			// Elements leaving go under their siblings.
			for c := e.first; c != nil; c = c.next {
				if c.leaving != 0 {
					p.element(c)
				}
			}
		}
		for c := e.first; c != nil; c = c.next {
			if c.flags&flagAbsolute == 0 {
				p.element(c)
			}
		}
		if e.flags&flagDividers != 0 && !e.grid {
			p.dividers(e)
		}
		for c := e.first; c != nil; c = c.next {
			if c.flags&flagAbsolute != 0 && c.leaving == 0 {
				p.element(c)
			}
		}
	}
	if clips {
		p.popClip()
		p.clip = savedClip
	}
	if e.scrolls() && own {
		p.scrollbars(e)
	}
	if own && borderOver {
		p.border(e, box)
	}
	if own && e.paintAfterFn != nil {
		e.paintAfterFn(p, box)
	}
	if e.flags&flagDebug != 0 && (e.parent == nil || e.parent.flags&flagDebug == 0) {
		p.debug(e)
	}
	if own && e.kind != kindInput && e.ringShown() {
		p.FocusRing(box, e.radius)
	}
	p.opacity = saved
	p.opaque, p.under = savedOpaque, p.under[:under]
}

// covered reports whether an opaque background around the element painting
// holds all of r, when r has an area.
func (p *Painter) covered(r Rect) bool {
	if r.W <= 0 || r.H <= 0 {
		return true
	}
	for _, b := range slices.Backward(p.under) {
		if r.X >= b.X && r.Y >= b.Y && r.X+r.W <= b.X+b.W && r.Y+r.H <= b.Y+b.H {
			return true
		}
	}
	return false
}

// clipRect returns the box an element clips its children to, inside its
// border, and its radii: as far as the clip reaches along an axis it does
// not clip.
func (e *node) clipRect() (Rect, [4]float32) {
	r := Rect{e.x + e.border[3], e.y + e.border[0], e.w - e.border[1] - e.border[3], e.h - e.border[0] - e.border[2]}
	var rad [4]float32
	all := e.flags&(flagScrollX|flagScrollY) != 0 || e.flags&flagClip == flagClip
	if all {
		for i, side := range [4][2]int{{0, 3}, {0, 1}, {2, 1}, {2, 3}} {
			rad[i] = max(e.radius[i]-max(e.border[side[0]], e.border[side[1]]), 0)
		}
		return r, rad
	}
	const far = 1e6
	if e.flags&flagClipX == 0 {
		r.X, r.W = -far, 2*far
	}
	if e.flags&flagClipY == 0 {
		r.Y, r.H = -far, 2*far
	}
	return r, rad
}

// borders returns border widths in device pixels, whole ones at least one
// wide where they are not zero.
func (p *Painter) borders(w [4]float32) [4]float32 {
	for i, v := range w {
		if v > 0 {
			w[i] = max(round(v*p.scale), 1)
		}
	}
	return w
}

// fill paints a rounded rectangle with a solid border of bw DIPs.
func (p *Painter) fill(r Rect, radius [4]float32, bg Color, bw float32, bc Color) {
	op := scene.Op{Kind: scene.OpFill, Rect: p.snap(r), Radii: p.radii(radius), Continuous: continuousCorners, Color: bg.scene(), BorderColor: bc.scene(), Opacity: p.opacity}
	var drawn Color // the border's, if any
	if bw > 0 {
		op.Border = p.borders([4]float32{bw, bw, bw, bw})
		drawn = bc
	}
	op.Wide = p.wide(bg, Color{}, drawn)
	p.s.Ops = append(p.s.Ops, op)
}

// background paints the background of an element, and its border with it
// unless withBorder is false.
func (p *Painter) background(e *node, box Rect, withBorder bool) {
	border := withBorder && scene.HasBorder(e.border) && e.borderC.A > 0
	if e.fill == fillMaterial {
		e.material.PaintMaterial(p, box, e.radius)
		if border {
			p.border(e, box)
		}
		return
	}
	var visible bool
	switch e.fill {
	case fillColor:
		visible = e.bg.A > 0
	case fillGradient:
		visible = e.grad.From.A > 0 || e.grad.To.A > 0
	case fillStripes:
		visible = e.bg.A > 0 || e.stripes.c.A > 0
	}
	if !visible && !border {
		return
	}
	op := scene.Op{Kind: scene.OpFill, Rect: p.snap(box), Radii: p.radii(e.radius), Continuous: continuousCorners, Color: e.bg.scene(), Opacity: p.opacity}
	var bc Color // the border's, if op draws it
	if border {
		op.Border, op.BorderColor, op.Dashed = p.borders(e.border), e.borderC.scene(), e.borderStyle == BorderDashed
		bc = e.borderC
	}
	c, c2 := e.bg, Color{}
	switch e.fill {
	case fillGradient:
		p.gradient(&op, e.grad)
		c, c2 = e.grad.From, e.grad.To
	case fillStripes:
		st := e.stripes
		c, c2 = st.c, e.bg
		a := float64(st.angle) * math.Pi / 180
		w := max(st.width*p.scale, 0.5)
		op.Paint, op.Color, op.Color2 = scene.PaintStripes, st.c.scene(), e.bg.scene()
		op.Gradient = [4]float32{float32(math.Cos(a)), float32(math.Sin(a)), w, w + max(st.gap*p.scale, 0)}
	}
	op.Wide = p.wide(c, c2, bc)
	p.s.Ops = append(p.s.Ops, op)
}

// border paints e's border alone.
func (p *Painter) border(e *node, box Rect) {
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpFill, Rect: p.snap(box), Radii: p.radii(e.radius), Continuous: continuousCorners, Opacity: p.opacity,
		Border: p.borders(e.border), BorderColor: e.borderC.scene(), Dashed: e.borderStyle == BorderDashed, Wide: p.wide(Color{}, Color{}, e.borderC)})
}

// gradient sets the paint of op to g, across its rectangle; the caller
// sets op.Wide.
func (p *Painter) gradient(op *scene.Op, g LinearGradient) {
	// CSS angles: 0deg points up, 90deg right.
	a := float64(g.Angle) * math.Pi / 180
	dx, dy := float32(math.Sin(a)), float32(-math.Cos(a))
	half := (abs32(op.Rect.W*dx) + abs32(op.Rect.H*dy)) / 2
	cx, cy := op.Rect.X+op.Rect.W/2, op.Rect.Y+op.Rect.H/2
	x0, y0, x1, y1 := cx-dx*half, cy-dy*half, cx+dx*half, cy+dy*half
	start, end := g.Start, g.End
	if start == 0 && end == 0 {
		end = 1
	}
	op.Paint = scene.PaintLinear
	if g.Oklab {
		op.Paint = scene.PaintOklab
	}
	op.Color, op.Color2 = g.From.scene(), g.To.scene()
	sx, sy := x0+(x1-x0)*start, y0+(y1-y0)*start
	ex, ey := x0+(x1-x0)*end, y0+(y1-y0)*end
	if end-start < 0.5/max(2*half, 1) {
		// Stops at one place: a hard edge, half a pixel wide.
		ex, ey = sx+dx*0.5, sy+dy*0.5
	}
	op.Gradient = [4]float32{sx, sy, ex, ey}
}

// dividers draws the lines between the children of e (Dividers), in the
// middle of the room between each two: after each row of a List but its
// last, else between children next to each other in the tree, on the same
// line.
func (p *Painter) dividers(e *node) {
	var d dividers
	for _, d = range e.c.dividers {
		if d.e == e {
			break
		}
	}
	if d.e != e {
		return
	}
	row := e.row
	if e.list != nil {
		row = false
	}
	// Across the element inside its border, or as far as children reach
	// beyond it, as the rows of a table scrolling sideways.
	lo, hi := e.y+e.border[0], e.y+e.h-e.border[2]
	if !row {
		lo, hi = e.x+e.border[3], e.x+e.w-e.border[1]
	}
	var prev *node
	for c := e.first; c != nil; c = c.next {
		if c.flags&flagAbsolute != 0 || c.collapsed {
			continue
		}
		if f := e.list; f != nil {
			// A List places its rows in any order: a line goes below each.
			if c.listRow && c.rowIndex < f.n-1 {
				p.divider(row, c.y+c.h+max(e.gapY, 0)/2, min(lo, c.x), max(hi, c.x+c.w), d.width, d.color)
			}
			continue
		}
		if a := prev; a != nil {
			// Main-axis start and end of a and c, and their cross extents.
			as, ae, cs, ce := a.y, a.y+a.h, c.y, c.y+c.h
			ax0, ax1, cx0, cx1 := a.x, a.x+a.w, c.x, c.x+c.w
			if row {
				as, ae, cs, ce = a.x, a.x+a.w, c.x, c.x+c.w
				ax0, ax1, cx0, cx1 = a.y, a.y+a.h, c.y, c.y+c.h
			}
			// On the same line of a row or a column that wraps.
			if !e.wrap || ax0 < cx1 && cx0 < ax1 {
				mid := (ae + cs) / 2
				if cs < as {
					mid = (ce + as) / 2 // a reversed row or column
				}
				p.divider(row, mid, min(lo, ax0, cx0), max(hi, ax1, cx1), d.width, d.color)
			}
		}
		prev = c
	}
}

// divider draws a line width DIPs thick centered on at, from lo to hi
// across: vertical in a row, horizontal in a column, whole pixels thick.
func (p *Painter) divider(row bool, at, lo, hi, width float32, c Color) {
	s := p.scale
	t := max(round(width*s), 1)
	a := round(at*s - t/2)
	r := scene.Rect{X: round(lo * s), Y: a, W: round(hi*s) - round(lo*s), H: t}
	box := Rect{lo, a / s, hi - lo, t / s}
	if row {
		r = scene.Rect{X: a, Y: round(lo * s), W: t, H: round(hi*s) - round(lo*s)}
		box = Rect{a / s, lo, t / s, hi - lo}
	}
	if !p.visible(box, 0) {
		return
	}
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpFill, Rect: r, Color: c.scene(), Wide: p.wide(c, Color{}, Color{}), Opacity: p.opacity})
}

// debug outlines an element and the elements inside it: their margins in
// orange, borders and padding in green, and content in blue.
func (p *Painter) debug(e *node) {
	saved := p.opacity
	p.opacity = 1
	var walk func(e *node)
	walk = func(e *node) {
		if e.flags&flagInvisible != 0 {
			return
		}
		box := Rect{e.x, e.y, e.w, e.h}
		if e.marginX() > 0 || e.marginY() > 0 {
			p.fill(Rect{box.X - e.m(3), box.Y - e.m(0), box.W + e.marginX(), box.H + e.marginY()}, [4]float32{}, Color{}, 1, RGBA(249, 115, 22, 0.8))
		}
		if e.padX() > 0 || e.padY() > 0 {
			p.fill(e.contentBox(), [4]float32{}, Color{}, 1, RGBA(34, 197, 94, 0.8))
		}
		p.fill(box, [4]float32{}, Color{}, 1, RGBA(59, 130, 246, 0.9))
		for c := e.first; c != nil; c = c.next {
			walk(c)
		}
	}
	walk(e)
	p.opacity = saved
}

func (p *Painter) pushClip(r Rect, radius [4]float32) {
	p.clip = intersect(p.clip, r)
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpPushClip, Rect: p.snap(r), Radii: p.radii(radius), Continuous: continuousCorners})
}

func (p *Painter) popClip() {
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpPopClip})
}

// textLayout paints a laid out text from (x, y), in color, with the
// background, underline or strikethrough of ts, and the colors, backgrounds
// and lines of the spans of sp, if any.
func (p *Painter) textLayout(l *text.Layout, x, y float32, color Color, ts textStyle, sp *spanPaint) {
	if l == nil {
		return
	}
	sys := p.rt.text
	s := p.scale
	deco := decoration{underline: ts.underline, wavy: ts.wavy, strike: ts.strike, color: ts.decoColor, thick: ts.decoThick}
	shade := text.ShadeOf(color.R, color.G, color.B)
	start := int32(len(p.s.Glyphs))
	var run *glyphRun
	if sys.JoinsGlyphs() {
		// The engine's, whose buffers each text reuses.
		run = &p.rt.glyphRun
		run.ids, run.pens, run.glyphs = run.ids[:0], run.pens[:0], run.glyphs[:0]
	}
	for li := range l.Lines {
		line := &l.Lines[li]
		if y+line.Y > p.clip.Y+p.clip.H || y+line.Y+line.Height < p.clip.Y {
			continue
		}
		if ts.background.A > 0 && line.Width > 0 {
			p.Fill(Rect{x + line.X, y + line.Y, line.Width, line.Height}, ts.background, 0)
		}
		if sp != nil {
			sp.backgrounds(p, line, x, y)
		}
		baseline := sys.Baseline((y + line.Baseline) * s)
		for _, g := range line.Glyphs {
			pen := (x + g.X) * s
			if pen > (p.clip.X+p.clip.W)*s || pen+(g.Advance+g.Size)*s < p.clip.X*s {
				continue
			}
			ix := float32(math.Floor(float64(pen)))
			glyphColor, glyphShade := color, shade
			if sp != nil {
				if glyphColor = sp.color(sp.at(g.Cluster), color); glyphColor != color {
					glyphShade = text.ShadeOf(glyphColor.R, glyphColor.G, glyphColor.B)
				}
			}
			gi := sys.Glyph(g.Font, g.ID, s, pen, glyphShade, p.opaque)
			if !gi.OK {
				continue
			}
			glyphColor = glyphColor.Alpha(p.opacity)
			sg := scene.Glyph{
				X: ix + gi.Left, Y: baseline + gi.Top, W: float32(gi.W), H: float32(gi.H),
				U: gi.X, V: gi.Y, UW: gi.W, VH: gi.H,
				Color: glyphColor.scene(), Wide: p.glyphWide(glyphColor), Colored: gi.Colored, Subpixel: gi.Subpixel, Thin: gi.Thin,
			}
			if run != nil {
				run.add(p, g, gi, pen, sg, glyphShade, baseline)
				continue
			}
			p.s.Glyphs = append(p.s.Glyphs, sg)
		}
		if run != nil {
			run.flush(p, baseline)
		}
		if sp != nil {
			sp.lines(p, l, li, x, y, color, deco)
		}
		if deco.underline || deco.strike {
			p.decorations(l, li, 0, len(line.Glyphs), x, y, deco, color)
		}
	}
	if end := int32(len(p.s.Glyphs)); end > start {
		p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpGlyphs, Start: start, End: end})
	}
}

// glyphRun gathers glyphs of a line in one font and color whose
// antialiased edges share pixels, which the text system joins where the
// system's text stack draws them joined (System.JoinsGlyphs).
type glyphRun struct {
	font   *text.Font
	shade  text.Shade
	ids    []uint32
	pens   []float32
	glyphs []scene.Glyph
	last   text.GlyphImage
}

// add takes a glyph, its pen at pen and its scene glyph sg, into the run
// if it shares pixels with the last, else starts a run with it.
func (r *glyphRun) add(p *Painter, g text.Glyph, gi text.GlyphImage, pen float32, sg scene.Glyph, shade text.Shade, baseline float32) {
	if n := len(r.glyphs); n > 0 {
		prev := &r.glyphs[n-1]
		if g.Font == r.font && sg.Color == prev.Color && sg.Wide == prev.Wide && !gi.Colored && p.rt.text.Shares(r.last, prev.X, prev.Y, gi, sg.X, sg.Y) {
			r.ids, r.pens, r.glyphs, r.last = append(r.ids, g.ID), append(r.pens, pen), append(r.glyphs, sg), gi
			return
		}
		r.flush(p, baseline)
	}
	r.font, r.shade, r.last = g.Font, shade, gi
	r.ids, r.pens, r.glyphs = append(r.ids, g.ID), append(r.pens, pen), append(r.glyphs, sg)
}

// flush paints the run: one glyph as it is, more joined.
func (r *glyphRun) flush(p *Painter, baseline float32) {
	switch n := len(r.glyphs); {
	case n == 1:
		p.s.Glyphs = append(p.s.Glyphs, r.glyphs[0])
	case n > 1:
		gi := p.rt.text.GlyphRun(r.font, r.ids, r.pens, p.scale, r.shade, p.opaque)
		if !gi.OK {
			p.s.Glyphs = append(p.s.Glyphs, r.glyphs...)
			break
		}
		sg := r.glyphs[0]
		sg.X, sg.Y = float32(math.Floor(float64(r.pens[0])))+gi.Left, baseline+gi.Top
		sg.W, sg.H, sg.U, sg.V, sg.UW, sg.VH = float32(gi.W), float32(gi.H), gi.X, gi.Y, gi.W, gi.H
		sg.Subpixel, sg.Thin = gi.Subpixel, gi.Thin
		p.s.Glyphs = append(p.s.Glyphs, sg)
	}
	r.ids, r.pens, r.glyphs = r.ids[:0], r.pens[:0], r.glyphs[:0]
}

// decoration is how lines go through or under text.
type decoration struct {
	underline, wavy, strike bool
	// color, if set, is the lines', and thick their thickness in DIPs.
	color Color
	thick float32
}

// decorations draws the lines of d along glyphs [i, j) of line li of l,
// laid out from (x, y) in DIPs, where the system's text stack places
// underlines and strikethroughs, in the color of d or c. Wavy underlines
// follow the underline, at least a pixel thick.
func (p *Painter) decorations(l *text.Layout, li, i, j int, x, y float32, d decoration, c Color) {
	if d.color.A > 0 {
		c = d.color
	}
	for _, k := range [2]text.Decoration{text.Underline, text.Strikethrough} {
		if k == text.Underline && !d.underline || k == text.Strikethrough && !d.strike {
			continue
		}
		for _, st := range p.rt.text.Decorate(l, li, i, j, x, y, p.scale, k, d.thick) {
			if st.X1 <= st.X0 || st.Bottom <= st.Top {
				continue
			}
			if k == text.Underline && d.wavy {
				p.wave(st.X0, st.X1, (st.Top+st.Bottom)/2, max(st.Bottom-st.Top, 1), c)
				continue
			}
			p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpFill, Rect: scene.Rect{X: st.X0, Y: st.Top, W: st.X1 - st.X0, H: st.Bottom - st.Top}, Color: c.scene(), Wide: p.wide(c, Color{}, Color{}), Opacity: p.opacity})
		}
	}
}

// wave strokes a wave from x0 to x1 around y, thick device pixels wide,
// one and a half times as high, as spell checkers underline words.
func (p *Painter) wave(x0, x1, y, thick float32, c Color) {
	s := p.scale
	amp := thick * 1.5 / 2
	length := thick * 6
	y += amp
	var path Path
	// Eight points a wave, the last at x1.
	n := max(int(math.Ceil(float64((x1-x0)/(length/8)))), 1)
	path.MoveTo(x0/s, y/s)
	for i := 1; i <= n; i++ {
		x := x0 + (x1-x0)*float32(i)/float32(n)
		path.LineTo(x/s, (y-amp*float32(math.Sin(float64((x-x0)/length*2*math.Pi))))/s)
	}
	p.StrokePath(&path, thick/s, c)
}

// contentBox returns the element's box inside its padding and border.
func (e *node) contentBox() Rect {
	return Rect{e.x + e.contentX(), e.y + e.contentY(), e.w - e.padX(), e.h - e.padY()}
}

func (p *Painter) image(e *node) {
	if s := e.svg; s != nil {
		p.drawSVG(s, e.contentBox(), e.fit, e.radius, e.resolvedText().color, e.gray)
		return
	}
	img := e.image
	if img == nil || img.w == 0 || img.h == 0 {
		return
	}
	p.drawBitmap(img, e.contentBox(), e.fit, e.radius, e.gray)
}

// fitIn returns where a picture w×h DIPs goes in box as fit says, and
// which part of it shows there, as fractions of its size.
func fitIn(box Rect, w, h float32, fit Fit) (dst, src Rect) {
	dst, src = box, Rect{0, 0, 1, 1}
	scale := float32(1)
	switch fit {
	case FillBox:
		return dst, src
	case Contain:
		scale = min(box.W/w, box.H/h)
	case Cover:
		scale = max(box.W/w, box.H/h)
	case ScaleDown:
		scale = min(box.W/w, box.H/h, 1)
	}
	dst.W, dst.H = w*scale, h*scale
	dst.X += (box.W - dst.W) / 2
	dst.Y += (box.H - dst.H) / 2
	// Only what falls in the box shows.
	shown := intersect(dst, box)
	src = Rect{(shown.X - dst.X) / dst.W, (shown.Y - dst.Y) / dst.H, shown.W / dst.W, shown.H / dst.H}
	return shown, src
}

func (p *Painter) drawBitmap(img *Bitmap, box Rect, fit Fit, radius [4]float32, gray bool) {
	// A bitmap's own size is its pixels in DIPs, as an Image lays it out.
	iw, ih := float32(img.w), float32(img.h)
	dst, frac := fitIn(box, iw, ih, fit)
	if dst.W <= 0 || dst.H <= 0 {
		return
	}
	// Far smaller than its pixels, a level of it, as smooth.
	shown := img.smaller(min(frac.W*iw/(dst.W*p.scale), frac.H*ih/(dst.H*p.scale)))
	sw, sh := float32(shown.W), float32(shown.H)
	src := scene.Rect{X: frac.X * sw, Y: frac.Y * sh, W: frac.W * sw, H: frac.H * sh}
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpImage, Rect: p.snap(dst), Radii: p.radii(radius), Continuous: continuousCorners, Image: shown, Src: src, Opacity: p.opacity, Grayscale: gray})
}

// scrollbars draws the thumbs of a scroll container whose content
// overflows it.
func (p *Painter) scrollbars(e *node) {
	st := e.st
	rt := e.c.rt
	theme := e.c.theme
	hovered := false
	for _, id := range rt.hover {
		if id == e.id {
			hovered = true
			break
		}
	}
	dragging := rt.scrollDrag.st == st
	if !hovered && !dragging {
		return
	}
	color := theme.Scrollbar
	w, h := e.contentW, e.contentH
	x, y := st.scrollX, st.scrollY
	if dragging {
		// The thumb keeps its length, and goes as far along the track as
		// the offset is through the content now (dragTo).
		d := &rt.scrollDrag
		w, h = d.contentW, d.contentH
		x = rescale(x, e.contentW-float64(e.w), w-float64(e.w))
		y = rescale(y, e.contentH-float64(e.h), h-float64(e.h))
	}
	g := scrollBars(Rect{e.x, e.y, e.w, e.h}, e.barInset, float32(w), float32(h), float32(x), float32(y), e.flags, theme.scrollbarWidth())
	if g.vertical {
		bar := g.v
		if dragging && !rt.scrollDrag.horizontal {
			bar.X, bar.W = bar.X-2, bar.W+2
		}
		p.fill(bar, [4]float32{bar.W / 2, bar.W / 2, bar.W / 2, bar.W / 2}, color, 0, Color{})
	}
	if g.horizontal {
		bar := g.h
		if dragging && rt.scrollDrag.horizontal {
			bar.Y, bar.H = bar.Y-2, bar.H+2
		}
		p.fill(bar, [4]float32{bar.H / 2, bar.H / 2, bar.H / 2, bar.H / 2}, color, 0, Color{})
	}
}

// rescale returns offset off of content scrolling as far as now as far
// through content scrolling as far as then.
func rescale(off, now, then float64) float64 {
	if now <= 0 {
		return 0
	}
	return off / now * then
}

// scrollGeometry is where the scroll bars of a container go.
type scrollGeometry struct {
	vertical, horizontal bool
	// v and h are the thumbs, vTrack and hTrack the tracks they move on.
	v, h, vTrack, hTrack Rect
}

// scrollBars returns the scroll bars of a container box scrolled by
// (x, y) over content w×h, with thumbs width DIPs wide, moved in from its
// edges by inset (top, right, bottom, left, ScrollbarInsets): those of the
// directions its flags scroll that overflow, which leave each other the
// corner where both show.
func scrollBars(box Rect, inset [4]float32, w, h, x, y float32, flags uint32, width float32) scrollGeometry {
	var g scrollGeometry
	g.vertical = flags&flagScrollY != 0 && h > box.H+0.5
	g.horizontal = flags&flagScrollX != 0 && w > box.W+0.5
	corner := float32(0)
	if g.vertical && g.horizontal {
		corner = width + 3
	}
	top, right, bottom, left := inset[0], inset[1], inset[2], inset[3]
	if g.vertical {
		track := max(box.H-top-bottom-corner, 0)
		g.vTrack = Rect{box.X + box.W - right - width - 6, box.Y + top, width + 6, track}
		t := scrollThumb(box.Y+top, track, box.H, h, y)
		g.v = Rect{box.X + box.W - right - width - 3, t.Y, width, t.H}
	}
	if g.horizontal {
		track := max(box.W-left-right-corner, 0)
		g.hTrack = Rect{box.X + left, box.Y + box.H - bottom - width - 6, track, width + 6}
		t := scrollThumb(box.X+left, track, box.W, w, x)
		g.h = Rect{t.Y, box.Y + box.H - bottom - width - 3, t.H, width}
	}
	return g
}

// scrollThumb returns the thumb's position (Y) and length (H) along a
// track starting at pos, track long, for a view of content of size content
// scrolled by offset.
func scrollThumb(pos, track, view, content, offset float32) Rect {
	inner := track - 4
	thumb := min(max(inner*view/content, 24), inner)
	travel := inner - thumb
	at := float32(0)
	if content > view {
		at = max(0, min(travel*offset/(content-view), travel))
	}
	return Rect{Y: pos + 2 + at, H: thumb}
}

// Fill paints a rounded rectangle.
func (p *Painter) Fill(r Rect, c Color, radius float32) {
	p.fill(r, [4]float32{radius, radius, radius, radius}, c, 0, Color{})
}

// FillGradient paints a rounded rectangle with a gradient.
func (p *Painter) FillGradient(r Rect, g LinearGradient, radius float32) {
	op := scene.Op{Kind: scene.OpFill, Rect: p.snap(r), Radii: p.radii([4]float32{radius, radius, radius, radius}), Continuous: continuousCorners, Opacity: p.opacity}
	p.gradient(&op, g)
	op.Wide = p.wide(g.From, g.To, Color{})
	p.s.Ops = append(p.s.Ops, op)
}

// Stroke paints the outline of a rounded rectangle, width DIPs wide inside
// its edge.
func (p *Painter) Stroke(r Rect, c Color, radius, width float32) {
	p.fill(r, [4]float32{radius, radius, radius, radius}, Color{}, width, c)
}

// StrokeDashed paints the outline of a rounded rectangle in dashes, as a
// dashed border.
func (p *Painter) StrokeDashed(r Rect, c Color, radius, width float32) {
	op := scene.Op{Kind: scene.OpFill, Rect: p.snap(r), Radii: p.radii([4]float32{radius, radius, radius, radius}), Continuous: continuousCorners,
		Border: p.borders([4]float32{width, width, width, width}), BorderColor: c.scene(), Dashed: true, Opacity: p.opacity, Wide: p.wide(Color{}, Color{}, c)}
	p.s.Ops = append(p.s.Ops, op)
}

// Shadow paints the box shadow of a rounded rectangle as Element.Shadow
// does: offset by x and y, blurred by blur and grown by spread DIPs, and
// only outside the rectangle, so that it does not show through what is
// drawn there.
func (p *Painter) Shadow(r Rect, radius, x, y, blur, spread float32, c Color) {
	p.shadow(r, [4]float32{radius, radius, radius, radius}, shadow{x, y, blur, spread, c})
}

// shadow paints the shadow sh of box, rounded by rad, outside box, as
// CSS's box-shadow does.
func (p *Painter) shadow(box Rect, rad [4]float32, sh shadow) {
	r := Rect{box.X + sh.x - sh.spread, box.Y + sh.y - sh.spread, box.W + 2*sh.spread, box.H + 2*sh.spread}
	grown := rad
	for i := range grown {
		if grown[i] > 0 {
			grown[i] = max(grown[i]+sh.spread, 0)
		}
	}
	p.s.Ops = append(p.s.Ops, scene.Op{Kind: scene.OpShadow, Rect: p.snap(r), Radii: p.radii(grown), Continuous: continuousCorners, Color: sh.color.scene(),
		Wide: p.wide(sh.color, Color{}, Color{}), Blur: sh.blur * p.scale, Cast: p.snap(box), CastRadii: p.radii(rad), Opacity: p.opacity})
}

// Line paints a straight horizontal or vertical line between two points,
// width DIPs thick.
func (p *Painter) Line(x0, y0, x1, y1, width float32, c Color) {
	r := Rect{min(x0, x1), min(y0, y1), abs32(x1 - x0), abs32(y1 - y0)}
	if r.W < r.H {
		r.X -= width / 2
		r.W = width
	} else {
		r.Y -= width / 2
		r.H = width
	}
	p.Fill(r, c, 0)
}

// Text draws a line of text with its top-left corner at (x, y).
func (p *Painter) Text(x, y float32, s string, size float32, c Color) {
	l := p.rt.text.Layout(text.Params{Text: s, Style: text.Style{Size: size}})
	p.textLayout(l, x, y, c, textStyle{size: size}, nil)
}

// Image draws a bitmap, or an SVG in its own colors (with the theme's text
// color for its currentColor), scaled to fit r.
func (p *Painter) Image(src ImageSource, r Rect, fit Fit) {
	switch s := src.(type) {
	case *Bitmap:
		if s != nil && s.w > 0 && s.h > 0 {
			p.drawBitmap(s, r, fit, [4]float32{}, false)
		}
	case *SVG:
		p.drawSVG(s, r, fit, [4]float32{}, p.rt.c.theme.Text, false)
	}
}

// FocusRing draws the ring that shows the keyboard focus around r.
func (p *Painter) FocusRing(r Rect, radius [4]float32) {
	const w = 2
	o := Rect{r.X - w - 1, r.Y - w - 1, r.W + 2*w + 2, r.H + 2*w + 2}
	for i := range radius {
		radius[i] += w + 1
	}
	p.fill(o, radius, Color{}, w, p.rt.c.theme.Focus)
}

// Clip restricts what fn draws to r.
func (p *Painter) Clip(r Rect, radius float32, fn func()) {
	saved := p.clip
	p.pushClip(r, [4]float32{radius, radius, radius, radius})
	fn()
	p.popClip()
	p.clip = saved
}
