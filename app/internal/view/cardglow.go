package view

import (
	"math"

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
// mygo 没有径向渐变，这里用「一串同心实心圆」叠出来：由外向内画，每一层只补
// 该半径处还差的那点不透明度（相对已铺外层做 alpha 补偿），叠完每一点的累计
// 不透明度就落在原版渐变曲线上（见 glowLayerAlpha）。
var cardGlowStops = []gradStop{
	{0, ui.RGBA(255, 255, 255, 0.6)},
	{0.4, ui.RGBA(125, 211, 252, 0.2)},
	{0.7, ui.RGBA(125, 211, 252, 0)},
}

const (
	// cardGlowRadius 是渐变圆的半径（原版 radial-gradient 的 200px）。
	cardGlowRadius = float32(200)
	// cardGlowSteps 是同心圆层数：层数越多越接近连续渐变，也越贵。16 层在
	// 200px 半径上约 12.5px 一环，肉眼看不出台阶。
	cardGlowSteps = 16
	// cardGlowOpacity 是整层不透明度（原版 .is-pointer-hovered::before 的 .9）。
	cardGlowOpacity = float32(0.9)
)

// cardGlowLayerAlpha 返回在外层累计不透明度为 prev 时，要把该点补到 want
// 这一环需要的额外不透明度（普通 alpha 叠加：a ∘ b = a + b(1-a)）。
func cardGlowLayerAlpha(prev, want float32) float32 {
	if want <= prev {
		return 0
	}
	if d := 1 - prev; d > 0 {
		return (want - prev) / d
	}
	return 0
}

// paintRadialStops 用一串同心圆叠出一条径向渐变（mygo 没有径向渐变接口）。
// stops 的 At 是「到 radius 的比例」，opacity 是整层不透明度；每圈只补该半径
// 处还差的 alpha（相对已铺外层做补偿），叠完每点的累计不透明度落在曲线上。
func paintRadialStops(p *ui.Painter, cx, cy, radius float32, stops []gradStop, steps int, opacity float32) {
	if steps < 2 || radius <= 0 {
		return
	}
	prev := float32(0)
	for i := 0; i < steps; i++ {
		// i=0 是最外层，最后一个是圆心。
		rr := radius * float32(steps-1-i) / float32(steps-1)
		stop := gradAt(stops, rr/radius)
		want := opacity * float32(stop.A) / 255
		inc := cardGlowLayerAlpha(prev, want)
		if inc <= 0 {
			continue
		}
		var ring ui.Path
		ring.Circle(cx, cy, max(rr, 0.5))
		p.FillPath(&ring, ui.RGB(stop.R, stop.G, stop.B).Alpha(inc))
		prev += inc * (1 - prev)
	}
}

// paintCardGlow 把指针跟随的径向光晕画进卡片，裁到圆角矩形（原版的 ::before
// 是 overflow:hidden 卡片里的绝对定位层）。px/py 是指针相对卡片左上角的坐标，
// box 是卡片矩形。
func paintCardGlow(p *ui.Painter, box ui.Rect, px, py float32) {
	p.Clip(box, 12, func() {
		paintRadialStops(p, box.X+px, box.Y+py, cardGlowRadius,
			cardGlowStops, cardGlowSteps, cardGlowOpacity)
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
		paintRadialStops(p, cx, cy, r, cardPressBottomStops, cardRadialSteps(r), 1)
		cx, cy, r = cardRadialAt(box, 0.18, 0.10)
		paintRadialStops(p, cx, cy, r, cardPressTopStops, cardRadialSteps(r), 1)
	})
}

// cardRadialSteps 按半径给同心圆层数（约 4px 一环）：按下态的渐变在中心处
// 亮度陡（白 .7 → 0），16 层在 ~230px 半径上会留下可见的台阶。
func cardRadialSteps(radius float32) int {
	return max(cardGlowSteps, int(radius/4)+1)
}

// cardSurface 在卡片背景之上、子元素之前画它的「背景图」层：按下态的两个
// 固定径向高光（原版 :active 的 background 前两层），外加悬停时指针跟随的
// 光晕（原版 ::before，画在背景之上）。两者必须合成**一个** Draw ——
// Element.Draw 只存一个 paintFn，先后调两次会互相覆盖。
func cardSurface(box ui.Element, pressed bool) {
	px, py, over := box.PointerPosition()
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		if pressed {
			paintCardPressGlow(p, r)
		}
		if over {
			paintCardGlow(p, r, px, py)
		}
	})
}
