package view

import (
	"image"
	"math"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

// 列表卡片的「指针跟随光晕」（原版 globals.css 的
// `[data-slot="wrapper"] [role="button"][class*="bg-content"]::before`）：
//
//	background: radial-gradient(200px circle at var(--hover-x) var(--hover-y),
//	    rgba(255,255,255,.6), rgba(125,211,252,.2) 40%, transparent 70%);
//	opacity: calc(var(--hover-strength) * .85);          /* 悬停时 .9 */
//
// 原版在 App.tsx 里给悬停中的那张卡挂 pointerover，逐帧把指针相对卡片的坐标
// 写进 `--hover-x/--hover-y`，于是卡上有一团跟着鼠标走的高光。原生版此前只
// 对齐了 `.is-pointer-hovered` 的边框/投影/上浮，这团光晕从来没画过。
//
// mygo 没有 radial-gradient 接口，此前用「一串同心实心圆」叠出来（由外向内，
// 每层只补该半径处还差的那点 alpha）。那是**阶梯近似**：环内 alpha 恒定，200px
// 半径上每环宽 13.3px、相邻环差 ≈17/255，浅色卡面上就是一圈圈可见的台阶。
// 本轮改为**逐像素烘一张位图**（radialGradientBitmap）：渐变径向对称，烘一张
// 单位圆、画的时候按目标半径拉伸，与 CSS 的 radial-gradient 逐像素等价。
var cardGlowStops = []gradStop{
	{0, ui.RGBA(255, 255, 255, 0.6)},
	{0.4, ui.RGBA(125, 211, 252, 0.2)},
	{0.7, ui.RGBA(125, 211, 252, 0)},
}

const (
	// cardGlowRadius 是渐变圆的半径（原版 radial-gradient 的 200px）。
	cardGlowRadius = float32(200)
	// cardGlowOpacity 是整层不透明度（原版 .is-pointer-hovered::before 的 .9）。
	cardGlowOpacity = float32(0.9)
)

// ---------------------------------------------------------------- 真径向渐变

// radialBakeSize 是径向渐变位图的烘焙边长（384² = 576KB/张，懒烘：按下态
// 那两张只在第一次按下时才烘）。渐变径向对称，画的时候按目标半径拉伸即可，
// 所以一张小图能铺任意大小的圆：384² 的图在半径 192px 上跨 192 个像素，
// 最陡那段 alpha 每像素只差 1~2/255，双线性拉伸后与解析曲线差 ≤2 灰阶
// （实测 1.9，其中大部分是 8 位合成取整；换成 256² 或 512² 都无改善）。
const radialBakeSize = 384

// radialGradientBitmap 把一条「圆心居中、半径 = 边长/2」的径向渐变烘成位图。
// stops 的 At 是到半径的比例（与 CSS 的 `<color> <pct>` 同义）；边角处 t>1，
// gradAt 夹到最后一个停靠点（也就是 CSS 的 transparent 收尾）。
func radialGradientBitmap(stops []gradStop) *ui.Bitmap {
	return radialGradientBitmapSized(stops, radialBakeSize)
}

// radialGradientBitmapSized 同 radialGradientBitmap，但烘焙边长由调用方给：
// 面积大而频率低的渐变（如氛围光的径向暗角，见 shell.go 的
// ambientVignetteBake）用不着 384² 的精度，小图能省下几百 KB 常驻内存。
func radialGradientBitmapSized(stops []gradStop, n int) *ui.Bitmap {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float32(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx := float32(x) + 0.5 - c
			dy := float32(y) + 0.5 - c
			col := gradAt(stops, float32(math.Sqrt(float64(dx*dx+dy*dy)))/c)
			i := img.PixOffset(x, y)
			img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = col.R, col.G, col.B, col.A
		}
	}
	return ui.NewBitmap(img)
}

// 三条径向渐变各烘一张（包级变量，进程内只烘一次）。
var (
	cardGlowBmp        = sync.OnceValue(func() *ui.Bitmap { return radialGradientBitmap(cardGlowStops) })
	cardPressTopBmp    = sync.OnceValue(func() *ui.Bitmap { return radialGradientBitmap(cardPressTopStops) })
	cardPressBottomBmp = sync.OnceValue(func() *ui.Bitmap { return radialGradientBitmap(cardPressBottomStops) })
)

// paintRadialGradient 把烘好的单位圆渐变铺成圆心 (cx,cy)、半径 radius 的圆，
// 整层再乘 opacity（对应原版 `opacity: calc(var(--hover-strength) * .85)`）。
func paintRadialGradient(p *ui.Painter, bmp *ui.Bitmap, cx, cy, radius, opacity float32) {
	if bmp == nil || radius <= 0 || opacity <= 0 {
		return
	}
	dst := ui.Rect{X: cx - radius, Y: cy - radius, W: 2 * radius, H: 2 * radius}
	// Tint 乘颜色与 alpha —— 白 + alpha=opacity 就是「整层不透明度」。
	p.ImageRegion(bmp, ui.Rect{}, dst, ui.ImageOptions{Tint: ui.RGBA(255, 255, 255, opacity)})
}

// radialPaint 是「画一条径向渐变」的接线点：生产走位图；测试把它换成旧的
// 同心圆写法做反证（见 cardglow_test.go 的 radialGradientBitmapStepped）。
var radialPaint = paintRadialGradient

// cardGlowFadeAnim 是卡片指针光晕的淡出曲线（原版
// `.is-pointer-hovered::before { transition: opacity .12s ease }`）。此前原生版
// 是「指针在就画、一走就没了」的瞬时切换，指针滑出卡片时那团白光硬跳一下。
var cardGlowFadeAnim = ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut}

// paintCardGlow 把指针跟随的径向光晕画进卡片，裁到圆角矩形（原版的 ::before
// 是 overflow:hidden 卡片里的绝对定位层）。px/py 是指针相对卡片左上角的坐标，
// box 是卡片矩形。
func paintCardGlow(p *ui.Painter, box ui.Rect, px, py float32) {
	paintCardGlowAlpha(p, box, px, py, cardGlowOpacity)
}

// paintCardGlowAlpha 同 paintCardGlow，但整层不透明度由调用方给：淡出期间
// 从 cardGlowOpacity 逐帧降到 0。
func paintCardGlowAlpha(p *ui.Painter, box ui.Rect, px, py, opacity float32) {
	if opacity <= 0 {
		return
	}
	p.Clip(box, 12, func() {
		radialPaint(p, cardGlowBmp(), box.X+px, box.Y+py, cardGlowRadius, opacity)
	})
}

// ---------------------------------------------------------------- 按下态背景

// 原版抽屉卡片按下的 background 是三层（globals.css:4144 的
// `[data-slot="wrapper"] [role="button"][class*="bg-content"]:active`）：
//
//	radial-gradient(circle at 18% 10%, rgba(255,255,255,.7), transparent 34%),
//	radial-gradient(circle at 86% 88%, rgba(14,165,233,.13), transparent 38%),
//	linear-gradient(145deg, rgba(255,255,255,.52), rgba(255,255,255,.32) 54%,
//	                rgba(226,242,251,.42))
//
// 第一层在最上面。mygo 没有径向渐变接口，两个径向用同心圆叠（同 cardGlow），
// 线性层直接走 ui.LinearGradient.Stops。mygo 此前完全没接按下态：卡片按下去
// 只有子元素自己的反馈，整卡不压不亮，和原版差异明显。
var (
	cardPressBaseStops = []ui.GradientStop{
		{Color: ui.Hex("#ffffff").Alpha(0.52), At: 0},
		{Color: ui.Hex("#ffffff").Alpha(0.32), At: 0.54},
		{Color: ui.Hex("#e2f2fb").Alpha(0.42), At: 1},
	}
	// 左上白高光（radial at 18% 10%，34% 处收干净）。
	cardPressTopStops = []gradStop{
		{0, ui.Hex("#ffffff").Alpha(0.70)},
		{0.34, ui.Hex("#ffffff").Alpha(0)},
	}
	// 右下的天蓝反光（radial at 86% 88%，38% 处收干净）。
	cardPressBottomStops = []gradStop{
		{0, ui.Hex("#0ea5e9").Alpha(0.13)},
		{0.38, ui.Hex("#0ea5e9").Alpha(0)},
	}
)

// cardPressScale 是按下时的缩放（原版 `transform: translateY(0) scale(0.985)`）。
const cardPressScale = 0.985

// cardPressPaint 是按下态这一帧的底色与投影（原版同一条 :active 规则）。
type cardPressPaint struct {
	Stops  []ui.GradientStop
	Inset  ui.Color // inset 0 1px 0 白 .84
	Ring   ui.Color // 0 0 0 2px 天蓝 .16
	Shadow ui.Color // 0 4px 12px 墨 .10
}

// cardPressGradient 是按下态的线性底（原版 145° 三段渐变）。
func cardPressGradient() ui.LinearGradient {
	return ui.LinearGradient{Angle: 145, Stops: cardPressBaseStops}
}

func cardPressPaintFor() cardPressPaint {
	return cardPressPaint{
		Stops:  cardPressBaseStops,
		Inset:  ui.Hex("#ffffff").Alpha(0.84),
		Ring:   ui.Hex("#0ea5e9").Alpha(0.16),
		Shadow: ui.Hex("#0f172a").Alpha(0.10),
	}
}

// cardRadialAt 返回 CSS `radial-gradient(circle at fx fy, …)` 的圆心与半径：
// 圆心在盒内 (fx,fy) 比例处；circle 的默认尺寸 farthest-corner 取到四个角
// 里最远那个的距离。
func cardRadialAt(box ui.Rect, fx, fy float32) (cx, cy, r float32) {
	cx, cy = box.X+box.W*fx, box.Y+box.H*fy
	dx := max(cx-box.X, box.X+box.W-cx)
	dy := max(cy-box.Y, box.Y+box.H-cy)
	return cx, cy, float32(math.Sqrt(float64(dx*dx + dy*dy)))
}

// paintCardPressGlow 画按下态背景的两个径向高光（右下天蓝在下、左上白在上）。
func paintCardPressGlow(p *ui.Painter, box ui.Rect) {
	p.Clip(box, 12, func() {
		cx, cy, r := cardRadialAt(box, 0.86, 0.88)
		radialPaint(p, cardPressBottomBmp(), cx, cy, r, 1)
		cx, cy, r = cardRadialAt(box, 0.18, 0.10)
		radialPaint(p, cardPressTopBmp(), cx, cy, r, 1)
	})
}

// cardSurface 在卡片背景之上、子元素之前画它的「背景图」层：按下态的两个
// 固定径向高光（原版 :active 的 background 前两层），外加指针跟随的光晕
// （原版 ::before，画在背景之上）。两者必须合成**一个** Draw ——
// Element.Draw 只存一个 paintFn，先后调两次会互相覆盖。
func cardSurface(box ui.Element, pressed bool, px, py, glow float32) {
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if pressed {
			paintCardPressGlow(p, r)
		}
		if glow > 0 {
			paintCardGlowAlpha(p, r, px, py, glow)
		}
	})
}

// cardGlowLayer 是卡片光晕这一层的每帧接线：指针进入这张卡时从 0 渐显到
// cardGlowOpacity，离开后按原版 120ms 渐隐（圆心停在离开处，不再跟着指针走）。
//
// 只给「最后悬停的那一张」挂 Draw/动画通道：原版也只给 activeCard 设
// --hover-x/--hover-y，其余卡片上那层 ::before 的 opacity 恒为 0。
//
// 原版 pointerEffectsEnabled 还要求 `(prefers-reduced-motion: no-preference)`：
// 桌面要求减少动态效果时，App.tsx 直接 clearAll()，光晕整层不画 —— 这里同样
// 在 reduceMotion 下把目标压成 0（悬停的底/边/上浮不受影响，它们是静态状态）。
func (a *App) cardGlowLayer(c *ui.Context, box ui.Element, key string, hovered, pressed, locked bool) {
	// 滚动锁期间不给新卡挂层：只保留进入滚动前最后悬停的那张，让它按
	// 120ms 渐隐；否则卡片在指针下滑过时会逐张亮/灭。
	if locked && a.cardGlowKey != key {
		return
	}
	px, py, over := box.PointerPosition()
	entered := over && !locked && a.cardGlowKey != key
	if over && !locked {
		a.cardGlowKey, a.cardGlowX, a.cardGlowY = key, px, py
	}
	if a.cardGlowKey != key {
		return
	}
	if entered {
		// 指针刚进入这张卡：把通道种到 0，整层从 0 渐显（AnimateTo 第一次
		// 调用会直接返回目标值，不种就会「第一次悬停瞬亮、之后才渐显」）。
		box.Jump("card-glow", 0)
	}
	want := float32(0)
	if hovered && !locked && !reduceMotion(c) {
		want = cardGlowOpacity
	}
	glow := box.AnimateTo("card-glow", want, cardGlowFadeAnim)
	if want == 0 && glow < 0.01 {
		// 淡出结束：放掉这条记录，这张卡不再挂 Draw，也不再请帧。
		a.cardGlowKey = ""
		if !pressed {
			return
		}
		glow = 0
	}
	cardSurface(box, pressed, a.cardGlowX, a.cardGlowY, glow)
}
