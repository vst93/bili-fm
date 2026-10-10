package main

import (
	"image"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// Past Max, the slider goes Galaxy: the brain turns blue violet in a burst
// of pixels and a flash, the card shakes, the track fills with twinkling
// pixels, and dotted rays stream out of the brain.

var (
	galaxyStroke              = ui.Hex("#22137c")
	galaxyFrom, galaxyTo      = ui.Hex("#8b91fd"), ui.Hex("#b781f3")
	galaxyTrack, galaxyTrack2 = ui.Hex("#0f0a21"), ui.Hex("#3b2a78")
	galaxyLabel               = ui.Hex("#8fd0f6")

	// The colors of the pixels, the burst's and the rays'.
	pixelColors = []ui.Color{
		ui.Hex("#f472b6"), ui.Hex("#c084fc"), ui.Hex("#a78bfa"), ui.Hex("#818cf8"),
		ui.Hex("#67e8f9"), ui.Hex("#7dd3fc"), ui.Hex("#f5f3ff"), ui.Hex("#e9d5ff"),
	}
	burstColors = []ui.Color{
		ui.Hex("#5b3cf0"), ui.Hex("#67e8f9"), ui.Hex("#ffffff"), ui.Hex("#f472b6"), ui.Hex("#a78bfa"),
	}
	rayColors = []ui.Color{ui.Hex("#5fb4bd"), ui.Hex("#9a9aac"), ui.Hex("#b3a8f0")}
)

const (
	galaxyIn   = 160 * time.Millisecond // the brain's colors change
	galaxyOut  = 250 * time.Millisecond // everything fades on leaving
	burstTime  = 650 * time.Millisecond
	flashTime  = 550 * time.Millisecond
	trackIn    = 50 * time.Millisecond // the pixels fill the track at once
	pixelPitch = 4                     // the pixels of the track, 3 DIPs square
)

// shake is how far the card jumps in the frames after Galaxy starts, a
// 60th of a second apart, as the demo's card does, in DIPs.
var shake = [][2]float32{
	{0.75, 1}, {-0.78, 0.3}, {0.9, -1.03}, {0.4, 0.37}, {0.25, 0}, {0.15, -0.2},
	{0.15, 0.32}, {-0.5, -0.22}, {-0.22, 0}, {0.15, -0.25}, {-0.3, 0.35},
	{-0.22, -0.15}, {-0.15, -0.08}, {-0.05, -0.02},
}

// hash returns a number from 0 to 1 that only depends on its arguments.
func hash(a, b, c int) float32 {
	h := uint32(a)*0x9e3779b1 ^ uint32(b)*0x85ebca77 ^ uint32(c)*0xc2b2ae3d
	h ^= h >> 15
	h *= 0x2c1b3c6d
	h ^= h >> 12
	h *= 0x297a2d39
	h ^= h >> 15
	return float32(h&0xffffff) / 0xffffff
}

// twinkle is how lit a pixel seeded by a and b is at t seconds: it blinks
// on and off at a pace of its own.
func twinkle(a, b int, t float64) float32 {
	period := 0.7 + 1.1*float64(hash(a, b, 1))
	phase := float64(hash(a, b, 2))
	k := math.Mod(t/period+phase, 1)
	// Lit for part of each period, easing in and out.
	on := 0.35 + 0.4*float64(hash(a, b, 3))
	if k > on {
		return 0
	}
	return float32(math.Sin(k / on * math.Pi))
}

// galaxyLevel returns how far into Galaxy the slider is at t: 0 out of
// it, 1 in it, and between while the brain changes.
func (s *slider) galaxyLevel(t time.Time) float32 {
	if s.galaxyAt.IsZero() {
		return 0
	}
	if !s.galaxyLeft.IsZero() {
		k := float32(t.Sub(s.galaxyLeft)) / float32(galaxyOut)
		return max(1-k, 0)
	}
	return min(float32(t.Sub(s.galaxyAt))/float32(galaxyIn), 1)
}

// shakeAt returns how far the card is off its place at t.
func (s *slider) shakeAt(t time.Time) (float32, float32) {
	if s.galaxyAt.IsZero() || !s.galaxyLeft.IsZero() {
		return 0, 0
	}
	f := t.Sub(s.galaxyAt).Seconds() * 60
	i := int(f)
	if f < 0 || i >= len(shake) {
		return 0, 0
	}
	a, b := shake[i], [2]float32{}
	if i+1 < len(shake) {
		b = shake[i+1]
	}
	k := float32(f - float64(i))
	return a[0] + (b[0]-a[0])*k, a[1] + (b[1]-a[1])*k
}

// drawGalaxyTrack draws the track's fill in Galaxy, up to the thumb at
// thumbX: deep violet, with a grid of pixels shimmering, dim far from the
// brain and bright next to it.
func drawGalaxyTrack(p *ui.Painter, ox, oy, thumbX, g float32, secs float64) {
	end := thumbX - fillInset
	if end <= trackX || g <= 0 {
		return
	}
	var path ui.Path
	roundRect(&path, ox+trackX, oy+trackY, end-trackX, trackH, trackRadius)
	p.FillPathGradient(&path, ui.LinearGradient{From: galaxyTrack.Alpha(g), To: galaxyTrack2.Alpha(g), Angle: 90, Oklab: true})
	cols := int((thumbX - trackX - 2) / pixelPitch)
	for c := range cols {
		x := trackX + 0.5 + float32(c)*pixelPitch
		u := (x - trackX) / (thumbX - trackX) // toward the brain
		for r := range 5 {
			if hash(c, r, 0) > 0.45+u {
				continue // gaps, far from the brain
			}
			bright := 0.12 + 0.88*smoothstep(0.3, 0.95, u)
			a := bright * (0.5 + 0.5*shimmer(c, r, secs)) * g
			if a < 0.03 {
				continue
			}
			color := pixelColors[int(hash(c, r, 4)*float32(len(pixelColors)))%len(pixelColors)]
			p.Fill(ui.Rect{X: ox + x, Y: oy + trackY + 0.5 + float32(r)*pixelPitch, W: 3, H: 3}, color.Alpha(a), 0.8)
		}
	}
}

// shimmer is how bright a pixel seeded by a and b is at t seconds, from 0
// to 1, waving at a pace of its own.
func shimmer(a, b int, t float64) float32 {
	period := 0.5 + float64(hash(a, b, 1))
	return float32(0.5 + 0.5*math.Sin(2*math.Pi*(t/period+float64(hash(a, b, 2)))))
}

func smoothstep(lo, hi, x float32) float32 {
	k := min(max((x-lo)/(hi-lo), 0), 1)
	return k * k * (3 - 2*k)
}

// drawRays draws the dotted rays streaming out of the brain centered on
// (cx, cy).
func drawRays(p *ui.Painter, cx, cy, g float32, secs float64) {
	const rays, from = 16, 19
	for i := range rays {
		angle := (float64(i) + 0.4*float64(hash(i, 0, 5))) * 2 * math.Pi / rays
		length := 10 + 26*hash(i, 0, 6)*hash(i, 0, 12)
		sin, cos := math.Sincos(angle)
		for d := float32(from); d < from+length; d += 2 {
			k := (d - from) / length // along the ray
			lit := 0.4 + 0.6*twinkle(i, int(d), secs*1.6)
			a := (1 - k) * 0.65 * lit * g
			if a < 0.03 {
				continue
			}
			color := rayColors[int(hash(i, int(d), 7)*float32(len(rayColors)))%len(rayColors)]
			x, y := cx+d*float32(cos), cy+d*float32(sin)
			p.Fill(ui.Rect{X: x - 0.5, Y: y - 0.5, W: 1, H: 1}, color.Alpha(a), 0)
		}
	}
}

// drawBurst draws the ring of pixels bursting out of the brain centered
// on (cx, cy), k into the burst.
func drawBurst(p *ui.Painter, cx, cy, k float32) {
	if k <= 0 || k >= 1 {
		return
	}
	const n = 28
	e := float32(1 - math.Pow(float64(1-k), 3))
	for i := range n {
		angle := (float64(i) + 0.3*float64(hash(i, 1, 8))) * 2 * math.Pi / n
		r := 11 + (6+12*hash(i, 1, 9))*e
		sin, cos := math.Sincos(angle)
		color := burstColors[i%len(burstColors)]
		size := float32(2)
		x, y := cx+r*float32(cos)*1.05, cy+r*float32(sin)*0.95
		p.Fill(ui.Rect{X: x - size/2, Y: y - size/2, W: size, H: size}, color.Alpha(1-k*k), 0)
	}
}

// drawSparkles draws the pixels glinting on the brain.
func drawSparkles(p *ui.Painter, cx, cy, g float32, secs float64) {
	for i := range 10 {
		x := cx - 10 + 20*hash(i, 2, 10)
		y := cy - 9 + 18*hash(i, 2, 11)
		a := twinkle(i, 99, secs*1.3) * g
		if a < 0.05 {
			continue
		}
		color := ui.Hex("#ffffff")
		if i%3 == 0 {
			color = ui.Hex("#7dd3fc")
		}
		p.Fill(ui.Rect{X: x - 0.75, Y: y - 0.75, W: 1.5, H: 1.5}, color.Alpha(a), 0)
	}
}

// glows are soft round glows, white, at a few opacities: the painter has
// no blur of shapes, so a glow is a bitmap.
var glows = map[int]*ui.Bitmap{}

const glowSteps = 32

// drawGlow draws a soft glow of color c and opacity a, radius r DIPs,
// centered on (cx, cy).
func drawGlow(p *ui.Painter, cx, cy, r float32, c ui.Color, a float32) {
	step := int(a*glowSteps + 0.5)
	if step <= 0 {
		return
	}
	key := step<<24 | int(c.R)<<16 | int(c.G)<<8 | int(c.B)
	bmp := glows[key]
	if bmp == nil {
		const n = 96
		img := image.NewRGBA(image.Rect(0, 0, n, n))
		alpha := float64(step) / glowSteps
		for y := range n {
			for x := range n {
				dx, dy := (float64(x)+0.5)/n*2-1, (float64(y)+0.5)/n*2-1
				d2 := dx*dx + dy*dy
				v := alpha * math.Exp(-d2*4.5) * math.Max(0, 1-d2)
				i := img.PixOffset(x, y)
				img.Pix[i] = uint8(float64(c.R)*v + 0.5)
				img.Pix[i+1] = uint8(float64(c.G)*v + 0.5)
				img.Pix[i+2] = uint8(float64(c.B)*v + 0.5)
				img.Pix[i+3] = uint8(255*v + 0.5)
			}
		}
		bmp = ui.NewBitmap(img)
		glows[key] = bmp
	}
	p.Image(bmp, ui.Rect{X: cx - r, Y: cy - r, W: 2 * r, H: 2 * r}, ui.FillBox)
}

// drawGalaxyBrain draws the brain in Galaxy's colors, g of the way there.
func drawGalaxyBrain(p *ui.Painter, cx, cy, size, g float32) {
	if g <= 0 {
		return
	}
	p.FillPathGradient(icon(brainPaths[:2], cx, cy, size), ui.LinearGradient{From: galaxyFrom.Alpha(g), To: galaxyTo.Alpha(g), Angle: 135, Oklab: true})
	p.StrokePath(icon(brainPaths, cx, cy, size), 2*size/24, galaxyStroke.Alpha(g))
}
