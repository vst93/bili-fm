package main

import (
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// The window shows the demo's frame: the card a little above the middle.
const (
	windowW, windowH = 472, 258
	cardW, cardH     = 220, 129.5
	cardY            = 62 // from the top of the window
)

// The card's parts, in DIPs from its top left corner, as the demo's page
// lays them out.
const (
	cardRadius = 8

	titleSize     = 13.25
	titleBaseline = 26
	effortX       = 12.2
	valueX        = 54.6

	helpX, helpY, helpRadius = 200, 20.9, 6
	helpMarkDX               = 0.3

	labelSize     = 12
	labelBaseline = 60.4
	fasterX       = 12.1
	smarterRight  = 208.1

	trackX, trackY, trackW, trackH, trackRadius = 12, 75, 196, 20, 5

	// The ticks of the levels, 34.8 DIPs apart; the second, the one
	// recommended, is a bar.
	tickX, tickGap, tickSize = 23, 34.8, 3.0
	barH                     = 10

	recommendedX, recommendedBaseline = 24.6, 114

	// The thumb's center goes 36 DIPs a level from stopX.
	stopX, stopGap = 20, 36
	brainSize      = 24
	galaxyBrain    = 32.0 // the brain's size in Galaxy
	galaxyBrainY   = 85.1
	brainY         = 85.5 // the brain's center

	// The fill ends under the brain, short of its center.
	fillInset = 10
)

var levels = []string{"Low", "Medium", "High", "Extra", "Max", "Galaxy"}

const galaxy = 5

var (
	pageColor      = ui.Hex("#101010")
	cardColor      = ui.Hex("#171717")
	borderColor    = ui.Hex("#262626")
	trackColor     = ui.Hex("#262626")
	titleColor     = ui.Hex("#767676")
	valueColor     = ui.Hex("#e5e5e5")
	labelColor     = ui.Hex("#6f6f6f")
	recommendColor = ui.Hex("#777777")
	dotColor       = ui.Hex("#525252")
	barColor       = ui.Hex("#5f5f5f")
	brainFill      = ui.Hex("#fd92ba")
	brainStroke    = ui.Hex("#611836")

	// The fill shows a gradient across the whole track, pink to blue
	// violet, more opaque toward the thumb.
	fillFrom, fillTo         = ui.Hex("#9f5a7a"), ui.Hex("#6958a7")
	fillAlpha        float32 = 0.071
)

// The thumb eases to a level as CSS's cubic-bezier(0.19, 1, 0.22, 1)
// does, in half a second.
const moveTime = 500 * time.Millisecond

var moveEase = cubicBezier(0.19, 1, 0.22, 1)

// The label's letters come in one after another, rising, sharpening and
// fading in, as CSS's ease eases them; the old label goes up, blurs and
// fades out at once.
const (
	inStagger = 14500 * time.Microsecond
	inTime    = 320 * time.Millisecond
	outDelay  = 25 * time.Millisecond
	outTime   = 355 * time.Millisecond

	inY, inBlur   = 3.0, 1.8
	outY, outBlur = 3.4, 1.87
)

var labelEase = cubicBezier(0.25, 0.1, 0.25, 1)

// slider is the demo's state: the level chosen, and the moves and changes
// still animating.
type slider struct {
	value float64 // the level SliderBase sets, 0 to 5

	level   int       // the level the thumb goes to
	from    float64   // where the thumb was when it set off
	movedAt time.Time // when it set off

	prev      int       // the level the label showed before
	changedAt time.Time // when the label changed

	galaxyAt, galaxyLeft time.Time // when the slider went Galaxy, and left it

	sheet *letterSheet // the label's letters, for blurring them

	// While the pointer holds the thumb, it stays under the pointer, at
	// held levels, and the level is the nearest; let go, it eases there.
	holding bool
	held    float64

	// clock, when set, gives the time instead of the frames', for tests
	// that draw the demo at a moment.
	clock func() time.Time
}

func newSlider() *slider {
	return &slider{sheet: newLetterSheet(titleFont, levels...)}
}

var (
	titleFont = ui.Font{Family: "system-ui", Size: titleSize, Antialiased: true}
	labelFont = ui.Font{Family: "system-ui", Size: labelSize, Antialiased: true}
)

func (s *slider) now(t time.Time) time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return t
}

// thumb returns where the thumb is at t, in levels, and whether it is
// still moving.
func (s *slider) thumb(t time.Time) (float64, bool) {
	if s.holding {
		return s.held, false
	}
	k := float64(t.Sub(s.movedAt)) / float64(moveTime)
	if s.movedAt.IsZero() || k >= 1 {
		return float64(s.level), false
	}
	return s.from + (float64(s.level)-s.from)*moveEase(max(k, 0)), true
}

// setLevel sends the thumb to level l at t.
func (s *slider) setLevel(l int, t time.Time) {
	if l == s.level {
		return
	}
	s.from, _ = s.thumb(t)
	s.prev, s.changedAt = s.level, t
	switch {
	case l == galaxy:
		s.galaxyAt, s.galaxyLeft = t, time.Time{}
	case s.level == galaxy:
		s.galaxyLeft = t
	}
	s.level, s.movedAt = l, t
	s.value = float64(l)
}

// hold puts the thumb under the pointer, at pos levels, at t.
func (s *slider) hold(pos float64, t time.Time) {
	s.holding, s.held = true, min(max(pos, 0), galaxy)
	s.setLevel(int(math.Round(s.held)), t)
}

// release lets the thumb go at t, to ease from where it was held to its
// level.
func (s *slider) release(t time.Time) {
	if s.holding {
		s.holding = false
		s.from, s.movedAt = s.held, t
	}
}

func (s *slider) view(c *ui.Context) {
	c.Root().Background(pageColor)
	w, h := c.Size()
	// Centered across, on whole DIPs so that the card's edges are crisp.
	ox, oy := float32(math.Round(float64(w-cardW)/2)), float32(math.Round(float64(cardY+(h-windowH)/2)))
	ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
		t := s.now(p.Now())
		if s.drawCard(p, ox, oy, t) {
			p.AnimationFrame()
		}
	})
	// The title bar, under the window controls, drags the window.
	bar := c.TitleBar()
	ui.Box(c).Absolute().Left(0).Top(0).Size(w, max(bar.Height, 32)).DragWindow()
	// The slider takes the keys, as assistive technology sees it, and
	// the pointer across the track; its content box spans the thumb's
	// travel. A drag moves the thumb with the pointer rather than from
	// level to level.
	sb := ui.SliderBase(c, &s.value, 0, galaxy).Step(1).FocusRing(false).Absolute().
		Left(ox+trackX).Top(oy+trackY-6).Size(trackW, trackH+12).PaddingX(stopX - trackX)
	t := s.now(c.Now())
	if sb.Changed() && !s.holding {
		s.setLevel(int(math.Round(s.value)), t)
	}
	if _, _, held := sb.Dragged(); held {
		x, _, _ := sb.PointerPosition()
		s.hold(float64(x-(stopX-trackX))/stopGap, t)
	} else {
		s.release(t)
	}
}

// drawCard draws the card with its top left corner at (ox, oy), as it is
// at t, and reports whether it is still animating.
func (s *slider) drawCard(p *ui.Painter, ox, oy float32, t time.Time) bool {
	pos, moving := s.thumb(t)
	g := s.galaxyLevel(t)
	secs := t.Sub(s.galaxyAt).Seconds()
	if g > 0 {
		moving = true
	}
	dx, dy := s.shakeAt(t)
	ox, oy = ox+dx, oy+dy

	// The card, as CSS rounds it, with a 1 DIP border, on Tailwind's
	// shadow-lg.
	card := ui.Rect{X: ox, Y: oy, W: cardW, H: cardH}
	p.Shadow(card, cardRadius, 0, 10, 15, -3, ui.RGBA(0, 0, 0, 0.1))
	p.Shadow(card, cardRadius, 0, 4, 6, -4, ui.RGBA(0, 0, 0, 0.1))
	var path ui.Path
	roundRect(&path, ox, oy, cardW, cardH, cardRadius)
	p.FillPath(&path, borderColor)
	path = ui.Path{}
	roundRect(&path, ox+1, oy+1, cardW-2, cardH-2, cardRadius-1)
	p.FillPath(&path, cardColor)

	drawWord(p, kerned("Effort", titleFont), ox+effortX, oy+titleBaseline, titleColor)
	if s.drawLabel(p, ox+valueX, oy+titleBaseline, t) {
		moving = true
	}
	drawHelp(p, ox+helpX, oy+helpY, titleColor)
	drawWord(p, kerned("Faster", labelFont), ox+fasterX, oy+labelBaseline, labelColor)
	smarter := kerned("Smarter", labelFont)
	drawWord(p, smarter, ox+smarterRight-smarter.width, oy+labelBaseline, labelColor)

	// The track, and the fill up to the thumb.
	path = ui.Path{}
	roundRect(&path, ox+trackX, oy+trackY, trackW, trackH, trackRadius)
	p.FillPath(&path, trackColor)
	thumbX := float32(stopX + stopGap*pos)
	if end := thumbX - fillInset; end > trackX {
		w := end - trackX
		path = ui.Path{}
		roundRect(&path, ox+trackX, oy+trackY, w, trackH, min(trackRadius, w/2))
		p.FillPathGradient(&path, ui.LinearGradient{From: fillFrom, To: fillTo, Angle: 90, End: trackW / w, Oklab: true})
		p.FillPathGradient(&path, ui.LinearGradient{From: trackColor.Alpha(1 - fillAlpha), To: trackColor.Alpha(0), Angle: 90})
	}
	gt := g
	if s.galaxyLeft.IsZero() && !s.galaxyAt.IsZero() {
		gt = min(float32(t.Sub(s.galaxyAt))/float32(trackIn), 1)
	}
	drawGalaxyTrack(p, ox, oy, thumbX, gt, secs)

	// The ticks.
	for i := range len(levels) {
		x := ox + tickX + tickGap*float32(i)
		if i == 1 {
			var bar ui.Path
			roundRect(&bar, x-tickSize/2, oy+trackY+trackH/2-barH/2, tickSize, barH, tickSize/2)
			p.FillPath(&bar, barColor.Mix(ui.Hex("#c8c4dc"), g))
			continue
		}
		var dot ui.Path
		dot.Circle(x, oy+trackY+trackH/2, tickSize/2)
		p.FillPath(&dot, dotColor.Alpha(1-g))
	}
	drawWord(p, kerned("Recommended", labelFont), ox+recommendedX, oy+recommendedBaseline, recommendColor)

	// The brain, and in Galaxy its rays and glow behind it, the burst
	// and the flash over it. In Galaxy the brain grows by a third, a
	// little past it at first.
	grow := float32(1 + (galaxyBrain/brainSize-1)*backOut(float64(g)))
	bx, by := ox+thumbX, oy+brainY+(galaxyBrainY-brainY)*g
	if g > 0 {
		drawRays(p, bx, by, g, secs)
		drawGlow(p, bx, by, 34, ui.Hex("#7c74ff"), 0.4*g)
	}
	if g < 1 {
		drawBrain(p, bx, by, brainSize*grow, brainFill, brainStroke)
	}
	drawGalaxyBrain(p, bx, by, brainSize*grow, g)
	if g > 0 {
		drawSparkles(p, bx, by, g, secs)
	}
	if s.galaxyLeft.IsZero() && !s.galaxyAt.IsZero() {
		since := t.Sub(s.galaxyAt)
		drawBurst(p, bx, by, float32(since)/float32(burstTime))
		if k := float32(since) / float32(flashTime); k < 1 {
			drawGlow(p, bx, by, 44, ui.Hex("#f4f2ff"), 0.6*float32(math.Pow(float64(1-k), 1.6)))
		}
	}
	return moving
}

// drawLabel draws the level's label, and the last one while it goes, with
// the pen at x on the baseline at y, and reports whether they move.
func (s *slider) drawLabel(p *ui.Painter, x, y float32, t time.Time) bool {
	since := t.Sub(s.changedAt)
	if s.changedAt.IsZero() || since >= outDelay+outTime && since >= inStagger*time.Duration(len(levels[s.level])-1)+inTime {
		drawWord(p, shapeWord(levels[s.level], titleFont), x, y, labelColorOf(levels[s.level]))
		return false
	}
	// text draws str with its pen at x in color c, k opaque, as glyphs or
	// blurred.
	text := func(str string, glyphs []ui.Glyph, x float32, c ui.Color, k, dy, blur float32) {
		if k <= 0 {
			return
		}
		c = c.Alpha(k)
		if blur*p.Scale() < 0.05 {
			p.Glyphs(glyphs, x, y+dy, c)
			return
		}
		bmp, bx, by := s.sheet.blurred(str, p.Scale(), blur, c)
		if bmp != nil {
			bw, bh := bmp.Size()
			p.Image(bmp, ui.Rect{X: x + bx, Y: y + dy + by, W: float32(bw) / p.Scale(), H: float32(bh) / p.Scale()}, ui.FillBox)
		}
	}
	// The old label goes as a whole, kerned, as a text of its own.
	old := levels[s.prev]
	k := float32(labelEase(float64(since-outDelay) / float64(outTime)))
	text(old, ui.Shape(old, titleFont), x, labelColorOf(old), 1-k, -outY*k, outBlur*k)
	// The new one comes letter by letter.
	cur := shapeWord(levels[s.level], titleFont)
	c := labelColorOf(levels[s.level])
	for i, r := range []rune(levels[s.level]) {
		k := float32(labelEase(float64(since-inStagger*time.Duration(i)) / float64(inTime)))
		text(string(r), cur.letters[i], x+cur.x[i], c, k, inY*(1-k), inBlur*(1-k))
	}
	return true
}

// backOut eases to 1 a little past it, as CSS's cubic-bezier(0.34, 1.56,
// 0.64, 1) does.
var backOut = cubicBezier(0.34, 1.56, 0.64, 1)

// labelColorOf returns the color of a level's label: Galaxy's is sky blue.
func labelColorOf(label string) ui.Color {
	if label == levels[galaxy] {
		return galaxyLabel
	}
	return valueColor
}

// roundRect adds a rectangle with corners rounded by circles of radius r,
// as CSS's border-radius rounds them (the painter's rounded rectangles
// curve continuously on macOS, as AppKit's do).
func roundRect(p *ui.Path, x, y, w, h, r float32) {
	const k = 1 - 0.5522847498
	p.MoveTo(x+r, y)
	p.LineTo(x+w-r, y)
	p.CubeTo(x+w-r*k, y, x+w, y+r*k, x+w, y+r)
	p.LineTo(x+w, y+h-r)
	p.CubeTo(x+w, y+h-r*k, x+w-r*k, y+h, x+w-r, y+h)
	p.LineTo(x+r, y+h)
	p.CubeTo(x+r*k, y+h, x, y+h-r*k, x, y+h-r)
	p.LineTo(x, y+r)
	p.CubeTo(x, y+r*k, x+r*k, y, x+r, y)
	p.Close()
}

// cubicBezier returns CSS's cubic-bezier(x1, y1, x2, y2) timing function.
func cubicBezier(x1, y1, x2, y2 float64) func(float64) float64 {
	curve := func(a, b, s float64) float64 { return 3*a*s*(1-s)*(1-s) + 3*b*s*s*(1-s) + s*s*s }
	return func(t float64) float64 {
		if t <= 0 || t >= 1 {
			return min(max(t, 0), 1)
		}
		// Newton's method for the s at which the curve is at t, then
		// bisection where it stalls.
		s := t
		for range 8 {
			x := curve(x1, x2, s) - t
			d := 3*x1*(1-s)*(1-s) + 6*(x2-x1)*s*(1-s) + 3*(1-x2)*s*s
			if math.Abs(x) < 1e-7 || math.Abs(d) < 1e-6 {
				break
			}
			s -= x / d
		}
		lo, hi := 0.0, 1.0
		for range 40 {
			if x := curve(x1, x2, s); math.Abs(x-t) < 1e-7 {
				break
			} else if x < t {
				lo = s
			} else {
				hi = s
			}
			s = (lo + hi) / 2
		}
		return curve(y1, y2, s)
	}
}
