package view

import (
	"image"
	"math"

	"github.com/egoist/mygo/ui"
)

// 本文件实现迷你窗 #root 的背景（globals.css 的 body.mini-mode #root）：
//
//	radial-gradient(circle at 12% 18%, rgba(14,165,233,.12), transparent 36%),
//	radial-gradient(circle at 92% 72%, rgba(79,185,159,.14), transparent 38%),
//	linear-gradient(145deg, #fbfdff 0%, #f3f8fb 100%);
//
// 原生版此前只画了最底那条 145° 线性渐变，左上偏蓝、右下偏绿的两团径向
// 光晕（迷你窗那点「清晨窗玻璃」的色偏）一直缺失。mygo 没有径向渐变接口，
// 这里和 #root::before 的光场（bloom.go）同一手法：整块背景按像素在 float 里
// 合成一张小位图，画一次、每帧当图显示；mini 窗口固定 400×155，位图很小。

// miniBgBloom 是迷你窗背景里的一条径向光晕。cx/cy 是圆心（盒比例），
// r/g/b/alpha 是圆心颜色（CSS 的 `radial-gradient(circle at ...)` 默认
// farthest-corner，半径 = 圆心到四角的最大距离），stop 是「到透明」的半径
// 比例（CSS 的 `transparent 36%`）。
type miniBgBloom struct {
	cx, cy  float32
	r, g, b float32
	alpha   float32
	stop    float32
}

// miniBgBlooms 是 body.mini-mode #root 的两条径向光晕，按 CSS 顺序
// （列表靠前的在上层，合成时从后往前叠）。
var miniBgBlooms = []miniBgBloom{
	// radial-gradient(circle at 12% 18%, rgba(14,165,233,.12), transparent 36%)
	{cx: 0.12, cy: 0.18, r: 14, g: 165, b: 233, alpha: 0.12, stop: 0.36},
	// radial-gradient(circle at 92% 72%, rgba(79,185,159,.14), transparent 38%)
	{cx: 0.92, cy: 0.72, r: 79, g: 185, b: 159, alpha: 0.14, stop: 0.38},
}

// 背景最底层的 linear-gradient(145deg, #fbfdff 0%, #f3f8fb 100%)。
var (
	miniBgFrom = ui.Hex("#fbfdff")
	miniBgTo   = ui.Hex("#f3f8fb")
)

// linearT 返回盒坐标 (x,y) 在 w×h 盒内 CSS `linear-gradient(<deg>deg, …)`
// 渐变线上的位置（0..1）。方向与 mygo 的 LinearGradient 一致（0° 朝上、
// 90° 朝右），线长 = |w·dx| + |h·dy|，过盒子中心。
func linearT(x, y, w, h, deg float32) float32 {
	rad := float64(deg) * math.Pi / 180
	dx, dy := float32(math.Sin(rad)), float32(-math.Cos(rad))
	length := abs32(w*dx) + abs32(h*dy)
	if length <= 0 {
		return 0
	}
	t := ((x-w/2)*dx+(y-h/2)*dy)/length + 0.5
	return max(0, min(t, 1))
}

// circleFarthest 返回 CSS `radial-gradient(circle at (cx,cy))`（默认尺寸
// farthest-corner）的半径：圆心到四角的最大距离。
func circleFarthest(cx, cy, w, h float32) float32 {
	return max(
		float32(math.Hypot(float64(cx), float64(cy))),
		max(float32(math.Hypot(float64(w-cx), float64(cy))),
			max(float32(math.Hypot(float64(cx), float64(h-cy))),
				float32(math.Hypot(float64(w-cx), float64(h-cy))))))
}

// miniBgBloomAlpha 返回盒坐标 (x,y) 处一条光晕的 alpha（在 [0, bl.alpha]，
// 到 stop 处线性淡到 0）。
func miniBgBloomAlpha(bl miniBgBloom, x, y, w, h float32) float32 {
	cx, cy := bl.cx*w, bl.cy*h
	rad := circleFarthest(cx, cy, w, h) * bl.stop
	if rad <= 0 {
		return 0
	}
	d := float32(math.Hypot(float64(x-cx), float64(y-cy)))
	if d >= rad {
		return 0
	}
	return bl.alpha * (1 - d/rad)
}

// miniBgPixels 把迷你窗背景算成一张不透明的 straight-alpha 位图：底层 145°
// 线性渐变，再按 CSS 顺序（列表靠前在上）叠两条径向光晕。
func miniBgPixels(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	fw, fh := float32(w), float32(h)
	base := []gradStop{{0, miniBgFrom}, {1, miniBgTo}}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float32(x)+0.5, float32(y)+0.5
			c := gradAt(base, linearT(px, py, fw, fh, 145))
			r := float32(c.R) / 255
			g := float32(c.G) / 255
			b := float32(c.B) / 255
			a := float32(c.A) / 255
			// CSS 背景列表靠前的在上层：从后往前叠。
			for i := len(miniBgBlooms) - 1; i >= 0; i-- {
				bl := miniBgBlooms[i]
				al := miniBgBloomAlpha(bl, px, py, fw, fh)
				if al <= 0 {
					continue
				}
				r, g, b, a = overStraight(r, g, b, a, bl.r/255, bl.g/255, bl.b/255, al)
			}
			i := img.PixOffset(x, y)
			img.Pix[i+0] = to8(r)
			img.Pix[i+1] = to8(g)
			img.Pix[i+2] = to8(b)
			img.Pix[i+3] = to8(a)
		}
	}
	return img
}

// miniBackground 画迷你窗 #root 背景（线性渐变 + 两条径向光晕）。整块算成
// 一张位图、每帧只多一个全窗图元；尺寸变了才重算（迷你窗固定 400×155，
// 实际只烘一次）。
func (a *App) miniBackground(c *ui.Context) {
	ui.Box(c).Absolute().Fill().PassThrough().Draw(func(p *ui.Painter, r ui.Rect) {
		w, h := int(r.W+0.5), int(r.H+0.5)
		if w < 1 || h < 1 {
			return
		}
		if a.miniBgBmp == nil || a.miniBgSize != [2]int{w, h} {
			a.miniBgBmp = ui.NewBitmap(miniBgPixels(w, h))
			a.miniBgSize = [2]int{w, h}
		}
		p.Image(a.miniBgBmp, r, ui.FillBox)
	})
}
