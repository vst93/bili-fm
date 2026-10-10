package main

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// The painter draws glyphs sharp; the demo blurs its letters as they come
// and go, as CSS's filter: blur() does. So the letters are rasterized once,
// before the window opens (ui.Render draws a frame of its own, which must
// not happen within the window's), and blurred on the CPU as they move.

// sheetScale is the pixels per DIP the letters are rasterized at, to be
// shrunk to the display's.
const sheetScale = 4

// Each text of the sheet has its pen sheetPad DIPs right of its cell's
// left and its baseline sheetBaseline DIPs from the top.
const sheetPad, sheetBaseline, sheetH = 4, 18, 24

// letterSheet is texts rasterized white on transparent at sheetScale:
// letters, and whole words as Core Text kerns them.
type letterSheet struct {
	img   *image.RGBA
	cells map[string][2]int // the left and width of each text's cell, in DIPs
}

// newLetterSheet rasterizes the letters of words, and the words, in font
// f.
func newLetterSheet(f ui.Font, words ...string) *letterSheet {
	ls := &letterSheet{cells: map[string][2]int{}}
	var texts []string
	var width int
	add := func(t string) {
		if _, ok := ls.cells[t]; ok {
			return
		}
		var adv float32
		for _, g := range ui.Shape(t, f) {
			adv = max(adv, g.X+g.Advance)
		}
		w := int(math.Ceil(float64(adv))) + 2*sheetPad
		ls.cells[t] = [2]int{width, w}
		texts = append(texts, t)
		width += w
	}
	for _, w := range words {
		add(w)
		for _, r := range w {
			add(string(r))
		}
	}
	view := func(c *ui.Context) {
		c.Root().Background(ui.Transparent)
		ui.Box(c).Fill().Draw(func(p *ui.Painter, _ ui.Rect) {
			for _, t := range texts {
				p.Glyphs(ui.Shape(t, f), float32(ls.cells[t][0]+sheetPad), sheetBaseline, ui.Hex("#ffffff"))
			}
		})
	}
	ls.img = ui.Render(view, width, sheetH, sheetScale)
	return ls
}

// blurred returns text t of the sheet blurred by sigma DIPs at scale
// pixels per DIP, in color c, and where its top left corner is from the
// text's pen on its baseline, in DIPs.
func (ls *letterSheet) blurred(t string, scale, sigma float32, c ui.Color) (*ui.Bitmap, float32, float32) {
	cell, ok := ls.cells[t]
	if !ok {
		return nil, 0, 0
	}
	// The cell, shrunk to the display's pixels: each pixel the mean of the
	// sheet's under it.
	k := float64(sheetScale) / float64(scale)
	nw, nh := int(math.Ceil(float64(cell[1])*float64(scale))), int(math.Ceil(sheetH*float64(scale)))
	pad := int(math.Ceil(float64(3 * sigma * scale)))
	w, h := nw+2*pad, nh+2*pad
	cov := make([]float32, w*h)
	x0, sw, sh := cell[0]*sheetScale, cell[1]*sheetScale, sheetH*sheetScale
	for y := range nh {
		for x := range nw {
			sx0, sy0 := float64(x)*k, float64(y)*k
			var sum, area float64
			for sy := int(sy0); float64(sy) < sy0+k && sy < sh; sy++ {
				for sx := int(sx0); float64(sx) < sx0+k && sx < sw; sx++ {
					// The share of the sheet's pixel under the display's.
					fx := min(float64(sx+1), sx0+k) - max(float64(sx), sx0)
					fy := min(float64(sy+1), sy0+k) - max(float64(sy), sy0)
					a := fx * fy
					sum += a * float64(ls.img.Pix[sy*ls.img.Stride+(x0+sx)*4+3])
					area += a
				}
			}
			if area > 0 {
				cov[(y+pad)*w+x+pad] = float32(sum / area / 255)
			}
		}
	}
	gaussian(cov, w, h, sigma*scale)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for j, a := range cov {
		a *= float32(c.A) / 255
		img.Pix[j*4] = uint8(float32(c.R)*a + 0.5)
		img.Pix[j*4+1] = uint8(float32(c.G)*a + 0.5)
		img.Pix[j*4+2] = uint8(float32(c.B)*a + 0.5)
		img.Pix[j*4+3] = uint8(255*a + 0.5)
	}
	return ui.NewBitmap(img), -sheetPad - float32(pad)/scale, -sheetBaseline - float32(pad)/scale
}

// gaussian blurs the w×h coverage in place by sigma pixels, rows then
// columns.
func gaussian(cov []float32, w, h int, sigma float32) {
	if sigma < 0.01 {
		return
	}
	r := int(math.Ceil(float64(3 * sigma)))
	kernel := make([]float32, 2*r+1)
	var sum float32
	for i := range kernel {
		d := float64(i - r)
		kernel[i] = float32(math.Exp(-d * d / (2 * float64(sigma*sigma))))
		sum += kernel[i]
	}
	for i := range kernel {
		kernel[i] /= sum
	}
	tmp := make([]float32, len(cov))
	for y := range h {
		for x := range w {
			var v float32
			for i, kv := range kernel {
				if xx := x + i - r; xx >= 0 && xx < w {
					v += kv * cov[y*w+xx]
				}
			}
			tmp[y*w+x] = v
		}
	}
	for y := range h {
		for x := range w {
			var v float32
			for i, kv := range kernel {
				if yy := y + i - r; yy >= 0 && yy < h {
					v += kv * tmp[yy*w+x]
				}
			}
			cov[y*w+x] = v
		}
	}
}
