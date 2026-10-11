package imagecache

import (
	"image"

	"github.com/egoist/mygo/ui"
)

// 氛围光（原版 .app-shell::before 里的 --ambient-cover）用的小图边长上限。
//
// 原版这层要 blur(20px)、再按 178% × 156% 拉满窗口，用小图足够；原版注释
// 也提到用小图是为了「避免放大后细节发脏」。缓存里为每张封面留一张这么大的
// 源图（64×64×4 ≈ 16KB），才能在换曲时立刻切氛围光。
const ambientSource = 64

// downscale 用盒式平均把 img 缩到最长边不超过 max（已够小就只做转换）。
func downscale(img image.Image, max int) *image.RGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	tw, th := w, h
	if w > max || h > max {
		scale := float64(max) / float64(w)
		if s := float64(max) / float64(h); s < scale {
			scale = s
		}
		tw, th = int(float64(w)*scale), int(float64(h)*scale)
		if tw < 1 {
			tw = 1
		}
		if th < 1 {
			th = 1
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		y0 := b.Min.Y + y*h/th
		y1 := b.Min.Y + (y+1)*h/th
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < tw; x++ {
			x0 := b.Min.X + x*w/tw
			x1 := b.Min.X + (x+1)*w/tw
			if x1 <= x0 {
				x1 = x0 + 1
			}
			var sr, sg, sb, sa, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bl, a := img.At(sx, sy).RGBA()
					sr += uint64(r >> 8)
					sg += uint64(g >> 8)
					sb += uint64(bl >> 8)
					sa += uint64(a >> 8)
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			i := dst.PixOffset(x, y)
			dst.Pix[i] = uint8(sr / n)
			dst.Pix[i+1] = uint8(sg / n)
			dst.Pix[i+2] = uint8(sb / n)
			dst.Pix[i+3] = uint8(sa / n)
		}
	}
	return dst
}

// gradeAmbient 按原版 .app-shell::before 的 CSS 滤镜调色：
//
//	filter: blur(20px) saturate(0.78) brightness(1.14) contrast(0.76)
//
// 模糊交给 mygo 的 glass.Blur（空间操作），这里做的是后三步逐像素的
// 色彩调整——mygo 的玻璃材质只有模糊，没有 saturate/brightness/contrast，
// 所以预先算进像素里。顺序与 CSS 一致。
func gradeAmbient(src *image.RGBA) *image.RGBA {
	out := image.NewRGBA(src.Bounds())
	for i := 0; i < len(src.Pix); i += 4 {
		r := float64(src.Pix[i]) / 255
		g := float64(src.Pix[i+1]) / 255
		b := float64(src.Pix[i+2]) / 255
		// saturate(0.78)：按亮度往中间插值（CSS 用 Rec.709 系数）。
		const sat = 0.78
		lum := 0.2126*r + 0.7152*g + 0.0722*b
		r = lum + (r-lum)*sat
		g = lum + (g-lum)*sat
		b = lum + (b-lum)*sat
		// brightness(1.14)，再 contrast(0.76)。
		const bri, con = 1.14, 0.76
		r = (r*bri-0.5)*con + 0.5
		g = (g*bri-0.5)*con + 0.5
		b = (b*bri-0.5)*con + 0.5
		out.Pix[i] = clamp255(r)
		out.Pix[i+1] = clamp255(g)
		out.Pix[i+2] = clamp255(b)
		out.Pix[i+3] = src.Pix[i+3]
	}
	return out
}

func clamp255(v float64) uint8 {
	switch {
	case v <= 0:
		return 0
	case v >= 1:
		return 255
	}
	return uint8(v*255 + 0.5)
}

// Ambient 返回氛围光用的封面位图：原图经过原版那套滤镜调色后的版本。
//
// 与原图分开缓存（key 加后缀），只在真正要当氛围光时才算——按需生成，
// 不会为列表里每张封面都建一份调色副本。
func (c *Cache) Ambient(rawURL string) *ui.Bitmap {
	if rawURL == "" {
		return nil
	}
	key := rawURL + "\x00ambient"
	c.mu.Lock()
	if e := c.items[key]; e != nil && e.bitmap != nil {
		c.touchLocked(key)
		b := e.bitmap
		c.mu.Unlock()
		return b
	}
	src := c.items[rawURL]
	if src == nil || src.small == nil {
		c.mu.Unlock()
		return nil
	}
	small := src.small
	c.mu.Unlock()

	bmp := ui.NewBitmap(gradeAmbient(small))
	c.mu.Lock()
	if e := c.items[key]; e != nil {
		// 另一个 goroutine 先算好了，用它的。
		bmp = e.bitmap
	} else {
		c.items[key] = &entry{bitmap: bmp}
		c.order = append(c.order, key)
	}
	c.touchLocked(key)
	c.mu.Unlock()
	return bmp
}
