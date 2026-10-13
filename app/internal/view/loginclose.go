package view

import "github.com/egoist/mygo/ui"

// 登录面板右上角的关闭键（原版 index.tsx 的 `.login-close-btn`，样式在
// globals.css）。原样：
//
//	.login-close-btn         32×32 圆（w-8 h-8 rounded-full）、绝对定位
//	                         right:-16px top:16px、图标 CloseSmall 18 #475569；
//	                         底 rgba(255,255,255,.2) + 1px 白 .3 边 +
//	                         inset 0 1px 1px 白 .35 + 0 4px 12px 墨 .12
//	.login-close-btn:hover   底 白 .28、边 白 .4、投影抬到 0 6px 16px 墨 .15
//	.login-close-btn:active  transform: scale(.92)
//
// 原生版此前把这个键画成**裸图标**（无底 / 无边 / 无投影），悬停还铺一层
// 深色底（#0f172a .08）——方向反了：它坐在浅色玻璃面板上，原版是一枚半透明
// 白色磨砂圆。这里沿用 ownerFacePaintFor / settingsItemPaint 的「状态集中一处」
// 写法。
const (
	// loginCloseSize 是关闭键的边长（原版 w-8 h-8）。
	loginCloseSize = 32
	// loginCloseIconSize 是键内图标尺寸（原版 CloseSmall size 18）。
	loginCloseIconSize = 18
)

// loginClosePaint 是关闭键在某一状态下解析好的样式。mygo 的 Shadow 是累加的，
// 所以三态各自整组给出，不能「先铺常态再叠悬停」；内影（inset）与外投影分开
// 两组，因为两者走的是不同的原语（InsetShadow / Shadow）。
type loginClosePaint struct {
	BG     ui.Color
	Border ui.Color
	Inset  []shadowSpec
	Outer  []shadowSpec
	// Scale 是按下时的缩放（原版 :active 的 scale(.92)），其余状态为 1。
	Scale float32
}

// loginClosePaintFor 解析关闭键的三态。原版 :active 只加 transform: scale(.92)，
// 底 / 边 / 投影沿用 :hover 的那套（`:active` 规则里没有 background / border /
// box-shadow），所以这里按下态只在悬停态之上改 Scale。
func loginClosePaintFor(hovered, pressed bool) loginClosePaint {
	p := loginClosePaint{
		BG:     ui.Hex("#ffffff").Alpha(0.20),
		Border: ui.Hex("#ffffff").Alpha(0.30),
		Inset: []shadowSpec{
			{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.35)},
		},
		Outer: []shadowSpec{
			{0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.12)},
		},
		Scale: 1,
	}
	if hovered {
		p.BG = ui.Hex("#ffffff").Alpha(0.28)
		p.Border = ui.Hex("#ffffff").Alpha(0.40)
		p.Inset = []shadowSpec{
			{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.40)},
		}
		p.Outer = []shadowSpec{
			{0, 6, 16, 0, ui.Hex("#0f172a").Alpha(0.15)},
		}
	}
	if pressed {
		p.Scale = 0.92
	}
	return p
}
