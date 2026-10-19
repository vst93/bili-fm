package view

import (
	"image"
	"math"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本文件实现原版 #root 的两条常驻伪元素，它们此前在原生版里一直缺失：
//
//	#root::before  方向光晕 —— 三个径向渐变（左上暖光、右下冷光、右上冷光），
//	               整层 opacity 只在四个深色时段下调，其余时段为 1；
//	#root::after   时段光罩 —— 一条按时段换色的 165° 线性渐变，午间完全透明。
//
// 两者都压在 #root 的时段背景（backdrop）之上、内容之下，是「光穿过玻璃」
// 那点方向感的主体。globals.css 里它们的取值见下。
//
// mygo 只有两色的 LinearGradient、没有径向渐变接口，所以整块「光场」在这里
// 按像素算成一张小位图（ui.NewBitmap），画一次、每帧当图显示 —— 只随窗口尺寸 /
// 时段变，不随帧变，每帧只多一个全窗图元。
//
// 为什么逐像素算而不是用 Painter 叠椭圆：软件的 alpha 合成按 8 位预乘存储，
// 用几十层低 alpha 的同心椭圆逼近径向渐变，每层都会丢 RGB 精度（实测会把
// rgba(255,238,218) 的暖光算成偏黄的 [255,255,237]）。逐像素在 float 里合成
// 一次，颜色才准。

// bloom 是一条径向渐变光晕：中心 / 半径是伪元素盒的比例，颜色是中心的
// RGB（0..255）与 alpha，stop 是「到透明」的半径比例（CSS 的 `transparent 62%`）。
type bloom struct {
	cx, cy  float32
	rx, ry  float32
	r, g, b float32
	alpha   float32
	stop    float32
}

// rootBlooms 是原版 #root::before 的三条 radial-gradient，按 CSS 顺序
// （列表靠前的在上层，合成时从后往前叠）。
var rootBlooms = []bloom{
	// radial-gradient(ellipse 70% 55% at 15% 5%, rgba(255,238,218,.45), transparent 62%)
	{cx: 0.15, cy: 0.05, rx: 0.70, ry: 0.55, r: 255, g: 238, b: 218, alpha: 0.45, stop: 0.62},
	// radial-gradient(ellipse 60% 50% at 85% 95%, rgba(168,207,240,.38), transparent 60%)
	{cx: 0.85, cy: 0.95, rx: 0.60, ry: 0.50, r: 168, g: 207, b: 240, alpha: 0.38, stop: 0.60},
	// radial-gradient(ellipse 45% 35% at 92% 15%, rgba(230,240,255,.25), transparent 55%)
	{cx: 0.92, cy: 0.15, rx: 0.45, ry: 0.35, r: 230, g: 240, b: 255, alpha: 0.25, stop: 0.55},
}

// rootInset 是 #root::before 的 inset（-8%）：光晕的几何参照比窗口四周各
// 外扩 8% 的盒子，中心/半径都是相对那个盒子算的。
const rootInset = 0.08

// bloomOpacity 是 #root::before 整层的不透明度。原版只在四个深色时段下调，
// 其余时段是 CSS 默认的 1（`html[data-time-of-day=...] #root::before`）。
func bloomOpacity(period string) float32 {
	switch period {
	case "midnight":
		return 0.22
	case "predawn":
		return 0.20
	case "evening":
		return 0.30
	case "latenight":
		return 0.25
	}
	return 1
}

// bloomAlphaAt 返回窗口坐标 (x,y) 处第 i 条光晕的 alpha（在 [0, bl.alpha]，
// 到 stop 处线性淡到 0）。w,h 是窗口尺寸（DIP）。
func bloomAlphaAt(bl bloom, x, y, w, h float32) float32 {
	bw, bh := w*(1+2*rootInset), h*(1+2*rootInset)
	cx, cy := -w*rootInset+bl.cx*bw, -h*rootInset+bl.cy*bh
	rx, ry := bl.rx*bw, bl.ry*bh
	if rx <= 0 || ry <= 0 {
		return 0
	}
	dx, dy := (x-cx)/rx, (y-cy)/ry
	dist := float32(math.Hypot(float64(dx), float64(dy)))
	if dist >= bl.stop {
		return 0
	}
	return bl.alpha * (1 - dist/bl.stop)
}

// periodOverlayStops 是 #root::after 的多停靠点渐变（按时段），取自
// globals.css；midday 返回 nil（原版那里是 transparent + opacity 0）。
func periodOverlayStops(period string) []gradStop {
	switch period {
	case "midnight":
		return []gradStop{
			{0, ui.RGBA(50, 60, 90, 0.05)},
			{0.5, ui.RGBA(45, 55, 75, 0.02)},
			{0.8, ui.Transparent},
		}
	case "predawn":
		return []gradStop{
			{0, ui.RGBA(200, 180, 200, 0.06)},
			{0.5, ui.RGBA(180, 180, 210, 0.03)},
			{0.8, ui.Transparent},
		}
	case "dawn":
		return []gradStop{
			{0, ui.RGBA(255, 210, 170, 0.14)},
			{0.4, ui.RGBA(255, 200, 150, 0.05)},
			{0.7, ui.Transparent},
		}
	case "morning":
		return []gradStop{
			{0, ui.RGBA(255, 220, 180, 0.10)},
			{0.5, ui.RGBA(240, 200, 200, 0.04)},
			{0.75, ui.Transparent},
		}
	case "noon":
		return []gradStop{
			{0, ui.RGBA(255, 240, 220, 0.06)},
			{0.6, ui.Transparent},
		}
	case "afternoon":
		return []gradStop{
			{0, ui.RGBA(255, 230, 200, 0.10)},
			{0.5, ui.RGBA(240, 210, 190, 0.04)},
			{0.75, ui.Transparent},
		}
	case "golden":
		return []gradStop{
			{0, ui.RGBA(255, 200, 150, 0.14)},
			{0.4, ui.RGBA(255, 180, 140, 0.06)},
			{0.7, ui.Transparent},
		}
	case "dusk":
		return []gradStop{
			{0, ui.RGBA(255, 180, 140, 0.12)},
			{0.5, ui.RGBA(200, 180, 255, 0.05)},
			{0.8, ui.Transparent},
		}
	case "evening":
		return []gradStop{
			{0, ui.RGBA(40, 55, 85, 0.08)},
			{0.5, ui.RGBA(45, 55, 80, 0.04)},
			{0.8, ui.Transparent},
		}
	case "latenight":
		return []gradStop{
			{0, ui.RGBA(50, 60, 90, 0.06)},
			{0.5, ui.RGBA(45, 55, 75, 0.03)},
			{0.8, ui.Transparent},
		}
	}
	return nil
}

// periodOverlayVisible 报告该时段的 #root::after 是否真的画东西（原版
// midday 是 transparent + opacity 0）。
func periodOverlayVisible(period string) bool { return period != "midday" }

// overlayT 返回窗口坐标 (x,y) 在 #root::after 的 165° 线性渐变上的位置
// （0..1），与 CSS 的渐变线定义一致。
func overlayT(x, y, w, h float32) float32 {
	return linearT(x, y, w, h, 165)
}

// overStraight 把 straight-alpha 的源 (sr,sg,sb,sa) 叠到 (dr,dg,db,da) 上，
// 全部在 [0,1] 的 float 里算（不经过 8 位预乘，避免累积丢精度）。
func overStraight(dr, dg, db, da, sr, sg, sb, sa float32) (r, g, b, a float32) {
	a = sa + da*(1-sa)
	if a <= 0 {
		return 0, 0, 0, 0
	}
	inv := da * (1 - sa)
	r = (sr*sa + dr*inv) / a
	g = (sg*sa + dg*inv) / a
	b = (sb*sa + db*inv) / a
	return r, g, b, a
}

// lightFieldPixels 把 #root::before + #root::after 算成一张 straight-alpha 的
// 位图：先按 CSS 顺序合成三条光晕（列表靠前的在上层），整层乘时段不透明度，
// 再叠时段光罩。
func lightFieldPixels(w, h int, period string) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	fw, fh := float32(w), float32(h)
	op := bloomOpacity(period)
	stops := periodOverlayStops(period)
	visible := periodOverlayVisible(period)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// 像素中心。
			px, py := float32(x)+0.5, float32(y)+0.5
			var r, g, b, a float32
			// 从最底层往上层叠（rootBlooms 列表靠前的在上）。
			for i := len(rootBlooms) - 1; i >= 0; i-- {
				bl := rootBlooms[i]
				al := bloomAlphaAt(bl, px, py, fw, fh)
				if al <= 0 {
					continue
				}
				r, g, b, a = overStraight(r, g, b, a,
					bl.r/255, bl.g/255, bl.b/255, al)
			}
			// ::before 的整层 opacity：只压 alpha，颜色（straight）不变。
			a *= op
			if visible && len(stops) > 0 {
				oc := gradAt(stops, overlayT(px, py, fw, fh))
				r, g, b, a = overStraight(r, g, b, a,
					float32(oc.R)/255, float32(oc.G)/255, float32(oc.B)/255, float32(oc.A)/255)
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

// to8 把 [0,1] 的通道值转成 8 位（四舍五入并夹取）。
func to8(v float32) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v*255 + 0.5)
}

// abs32 是 float32 的绝对值。
func abs32(v float32) float32 { return max(v, -v) }

// bloomMaxSide 是光场烘焙图的长边上限：光晕极软，不需要按窗口像素烘，一块
// 小图双线性拉伸即可（mygo 的 Bitmap 不做 mipmap），省内存。
const bloomMaxSide = 384

// bloomImageSize 返回给定窗口尺寸下光场烘焙图的像素尺寸。
func bloomImageSize(w, h float32) (int, int) {
	if w <= 0 || h <= 0 {
		return 1, 1
	}
	s := float32(bloomMaxSide) / max(w, h)
	if s > 1 {
		s = 1
	}
	return max(1, int(w*s+0.5)), max(1, int(h*s+0.5))
}

// lightFadeKey 是时段光罩交叉淡入的动画通道（同一条通道跨时段复用）。
const lightFadeKey = "light-fade"

// lightFieldFadeAnim 是原版 `#root::after { transition: opacity 1.5s ease,
// background 1.5s ease }`（main globals.css:1139）：时段一变，旧光罩淡出、
// 新光罩淡入，而不是当帧硬切。CSS 的 `ease` = cubic-bezier(.25,.1,.25,1)。
var lightFieldFadeAnim = ui.Animation{
	Duration: 1500 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// lightField 画 #root::before + #root::after。整块算成一张位图，每帧只多一个
// 全窗图元；尺寸或时段变了才重算（时段一天只变几次）。
//
// 时段切换时按原版 `transition: opacity 1.5s ease, background 1.5s ease` 做
// 交叉淡入：旧图留在 bloomPrev 里按 1-k、新图按 k 叠（两张都是低 alpha 柔光，
// 交叉淡出与 CSS 颜色插值肉眼无差），k 到 1 就把旧图放掉 —— 只在过渡的 1.5s
// 内多占一张位图。
func (a *App) lightField(c *ui.Context) {
	period := a.Theme.Period.Name
	box := ui.Box(c).Absolute().Fill().PassThrough()
	// 时段变了：把当前这张挪成旧图，并从 0 起一条新通道。bloomPeriod 为空是
	// 首次烘焙，没有旧图可淡出。
	if a.bloomPeriod != "" && a.bloomPeriod != period {
		a.bloomPrev, a.bloomPrevSize = a.bloomBmp, a.bloomSize
		a.bloomBmp = nil
		box.Jump(lightFadeKey, 0)
	}
	k := box.AnimateTo(lightFadeKey, 1, lightFieldFadeAnim)
	if a.bloomPrev == nil {
		k = 1
	}
	box.DrawOver(func(p *ui.Painter, r ui.Rect) {
		iw, ih := bloomImageSize(r.W, r.H)
		if a.bloomBmp == nil || a.bloomSize != [2]int{iw, ih} {
			a.bloomBmp = ui.NewBitmap(lightFieldPixels(iw, ih, period))
			a.bloomSize = [2]int{iw, ih}
			a.bloomPeriod = period
		}
		if a.bloomPrev != nil {
			// 窗口尺寸同时变了：旧图尺寸对不上，直接换新的（拉伸一张旧光场
			// 没有意义，而且会多留一张不同尺寸的位图）。
			if a.bloomPrevSize != a.bloomSize || k >= 1 {
				a.bloomPrev = nil
			} else {
				p.ImageRegion(a.bloomPrev, ui.Rect{}, r,
					ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, 1-k)})
			}
		}
		if a.bloomPrev != nil {
			p.ImageRegion(a.bloomBmp, ui.Rect{}, r,
				ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, k)})
			return
		}
		p.Image(a.bloomBmp, r, ui.FillBox)
	})
}
