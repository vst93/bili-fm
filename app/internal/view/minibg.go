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
//
// 生产路径已改成「线性渐变 + 两条框架径向渐变」三层元素（见 miniBackground），
// 不再用这张位图；保留它是为了逐像素核对三层叠加的结果与 CSS 一致。
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

// miniBackground 画迷你窗 #root 背景（145° 线性 + 两条正圆径向光晕）。
//
// 三层各画一次，都是框架的原生渐变（mygo v0.7.4 起有径向渐变）：此前是整块
// 烘成 400×155 的位图，白占 248KB 常驻内存，窗口尺寸一变还要重算。两条光晕
// 都是 `circle at …`（正圆，默认 farthest-corner），正好对上框架的 RadialGradient
// ——Radius 传 0 就是 farthest-corner，Center 是盒子的比例。
func (a *App) miniBackground(c *ui.Context) {
	ui.Box(c).Absolute().Fill().PassThrough().
		LinearGradient(ui.LinearGradient{Angle: 145, Stops: miniBgBaseStops})
	for i := range miniBgBloomGradients {
		ui.Box(c).Absolute().Fill().PassThrough().RadialGradient(miniBgBloomGradients[i])
	}
}

// miniBgBaseStops / miniBgBloomGradients 是那三层的框架停靠点：进程内只转一次，
// 不随帧重建（迷你窗每帧都画）。
var (
	miniBgBaseStops = gradStops([]gradStop{{0, miniBgFrom}, {1, miniBgTo}})
	// miniBgBloomGradients 按 CSS 顺序（列表靠前在上）：后画的压在先画的上面。
	miniBgBloomGradients = func() []ui.RadialGradient {
		out := make([]ui.RadialGradient, len(miniBgBlooms))
		for i, bl := range miniBgBlooms {
			col := ui.RGBA(to8(bl.r/255), to8(bl.g/255), to8(bl.b/255), bl.alpha)
			clear := col.Alpha(0)
			out[i] = ui.RadialGradient{
				Center: ui.Point{X: bl.cx, Y: bl.cy},
				// 0 = farthest-corner，与 CSS `circle at …` 的默认尺寸一致。
				Radius: 0,
				Stops: []ui.GradientStop{
					{Color: col, At: 0},
					{Color: clear, At: bl.stop},
				},
			}
		}
		return out
	}()
)
