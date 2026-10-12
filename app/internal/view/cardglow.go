package view

import "github.com/egoist/mygo/ui"

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

// paintCardGlow 把指针跟随的径向光晕画进卡片，裁到圆角矩形（原版的 ::before
// 是 overflow:hidden 卡片里的绝对定位层）。px/py 是指针相对卡片左上角的坐标，
// box 是卡片矩形。
func paintCardGlow(p *ui.Painter, box ui.Rect, px, py float32) {
	cx, cy := box.X+px, box.Y+py
	p.Clip(box, 12, func() {
		prev := float32(0)
		for i := 0; i < cardGlowSteps; i++ {
			// i=0 是最外层（半径 200），最后一个是圆心。
			rr := cardGlowRadius * float32(cardGlowSteps-1-i) / float32(cardGlowSteps-1)
			stop := gradAt(cardGlowStops, rr/cardGlowRadius)
			want := cardGlowOpacity * float32(stop.A) / 255
			inc := cardGlowLayerAlpha(prev, want)
			if inc <= 0 {
				continue
			}
			var ring ui.Path
			ring.Circle(cx, cy, max(rr, 0.5))
			p.FillPath(&ring, ui.RGB(stop.R, stop.G, stop.B).Alpha(inc))
			prev += inc * (1 - prev)
		}
	})
}

// cardGlow 给卡片挂上指针跟随光晕：只在该卡真正被指针悬停时画，位置取指针
// 相对卡片的坐标。PointerPosition 会让引擎在指针移动时补帧，所以高光能跟上。
func cardGlow(box ui.Element) {
	px, py, over := box.PointerPosition()
	if !over {
		return
	}
	box.Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, px, py) })
}
