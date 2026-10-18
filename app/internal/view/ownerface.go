package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// UP 主头像（主区右栏 .video-owner-wrap / #video-owner-face）在 main 分支
// globals.css 里的原样：
//
//	.video-owner-wrap         flex + gap 11，position: relative
//	.video-owner-wrap::before 48×48 圆，锚在 wrap 左上，盖在头像上：
//	                          180° 渐变（白 .22 → 45% 白 .06 → 100% 蓝 .06）
//	                          + 两道 inset（上白 .6 / 下墨 .08）
//	                          常态 opacity 1，:hover 降到 .7（pointer-events:none）
//	#video-owner-face         44×44（!important，压过 JSX 的 40）、margin 2px、
//	                          1.5px 白 .7 描边、box-shadow 蓝环 .18 + 墨影 .1；
//	                          :hover 换蓝环 .3 + 蓝影 .18，并 scale(1.06)
//
// 原生版此前把头像画成 40×40、没有描边/投影/玻璃罩，悬停也只放大 1.05 ——
// 比原版小一圈、扁一档。这里把尺寸与三态（常态 / 悬停）集中一处，
// 沿用 topToolStyleFor / playlistRowPaint 的写法。

// ownerFaceSize 是头像本体的边长（原版 #video-owner-face 的 44×44）。
const ownerFaceSize = 44

// ownerFaceMargin 是头像四周的 2px margin；加上它头像外框正好是玻璃罩的
// 48×48（原版 ::before 就按这个尺寸对齐）。
const ownerFaceMargin = 2

// ownerWrapGap 是头像与名字列之间的间距（原版 .video-owner-wrap 的 gap 11）。
const ownerWrapGap = 11

// ownerGlassSize 是玻璃罩的边长（原版 ::before 的 48×48 = 44 + 2×2）。
const ownerGlassSize = ownerFaceSize + 2*ownerFaceMargin

// ownerFacePaint 是头像在某一状态下解析好的样式。
type ownerFacePaint struct {
	// Scale 是 hover:scale 的倍率（原版常态 1、悬停 1.06）。
	Scale float32
	// Shadows 是这一态的 box-shadow（原版是二选一，不是叠加）。
	Shadows []shadowSpec
}

// ownerFacePaintFor 解析头像的两态（原版 #video-owner-face 与 :hover）。
// mygo 的 Shadow 是累加的，所以两态必须整组二选一，不能「先铺常态再叠悬停」。
func ownerFacePaintFor(hovered bool) ownerFacePaint {
	if hovered {
		return ownerFacePaint{
			Scale: 1.06,
			Shadows: []shadowSpec{
				{0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.30)}, // 蓝环 .3
				{0, 8, 18, 0, ui.Hex("#0ea5e9").Alpha(0.18)}, // 蓝影 .18
			},
		}
	}
	return ownerFacePaint{
		Scale: 1,
		Shadows: []shadowSpec{
			{0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.18)}, // 蓝环 .18
			{0, 4, 12, 0, shadowInk.Alpha(0.10)},        // 墨影 .1
		},
	}
}

// ownerFaceBorder 是头像的 1.5px 白 .7 描边（原版 #video-owner-face）。
func ownerFaceBorder() ui.Color { return ui.Hex("#ffffff").Alpha(0.70) }

// ownerFaceFocusShadows 是头像拿到键盘焦点时的 box-shadow（原版
// `#video-owner-face:focus-visible`）：2px 蓝环 + 淡墨影，整组替换悬停那两道。
func ownerFaceFocusShadows() []shadowSpec {
	return []shadowSpec{
		{0, 0, 0, 2, ui.Hex("#0284c7")},      // 0 0 0 2px var(--studio-blue)
		{0, 4, 12, 0, shadowInk.Alpha(0.10)}, // 0 4px 12px rgba(15,23,42,.1)
	}
}

// ownerGlassFadeAnim 是玻璃罩悬停淡出的过渡：原版
// `.video-owner-wrap::before { transition: opacity 0.2s ease }`（CSS 的 ease =
// cubic-bezier(.25,.1,.25,1)）。原生版此前直接 set 目标不透明度，悬停/移开是
// 硬切；这里走 AnimateTo，与播放栏/标题栏的悬停过渡同一条路子。
var ownerGlassFadeAnim = ui.Animation{
	Duration: 200 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// ownerGlassOpacity 是玻璃罩的透明度：常态 1，悬停 .7
// （原版 .video-owner-wrap:hover::before）。
func ownerGlassOpacity(hovered bool) float32 {
	if hovered {
		return 0.7
	}
	return 1
}

// ownerGlassGradient 是玻璃罩的整条渐变（原版 `.video-owner-wrap::before` 的
// 180° 三段渐变）：
//
//	linear-gradient(180deg,
//	    rgba(255,255,255,.22) 0%,
//	    rgba(255,255,255,.06) 45%,
//	    rgba(14,165,233,.06) 100%)
//
// v0.5.5 起 `ui.LinearGradient.Stops` 支持任意停靠点，于是一条渐变即可。
// 此前框架只有两端，这里拆成两层绝对定位的圆（上半段 0→45% 白 .22→.06、
// 下半段 45%→100% 透明→蓝 .06）—— 下半段那条透明的起点让**上半层的白 .06
// 一直垫在蓝下面**，底部 55% 多出一层 6% 白，深色封面上是一层可见的奶雾。
// 单层三段渐变与 CSS 逐值一致（见 ownerface_test.go 的底部像素反证）。
func ownerGlassGradient() ui.LinearGradient {
	return ui.LinearGradient{
		Angle: 180,
		Stops: []ui.GradientStop{
			{Color: ui.Hex("#ffffff").Alpha(0.22), At: 0},
			{Color: ui.Hex("#ffffff").Alpha(0.06), At: 0.45},
			{Color: ui.Hex("#0ea5e9").Alpha(0.06), At: 1},
		},
	}
}

// ownerGlassOverlay 画 UP 主头像上的磨砂玻璃罩（原版 .video-owner-wrap::before）：
// 48×48 圆、锚在 wrap 左上、盖在头像上、不吃指针；常态 opacity 1、悬停 .7，
// 外加原版两道 inset（上缘白高光、下缘墨影，画在渐变之上）。
//
// 声明成 var 便于测试换桩退回旧的两层画法做反证。
var ownerGlassOverlay = func(c *ui.Context, hovered bool) {
	glass := ui.Box(c).Absolute().Left(0).Top(0).
		Size(ownerGlassSize, ownerGlassSize).Radius(RadiusPill).PassThrough()
	glass.Opacity(glass.AnimateTo("owner-glass-fade",
		ownerGlassOpacity(hovered), ownerGlassFadeAnim))
	glass.LinearGradient(ownerGlassGradient())
	glass.InsetShadow(0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.60))
	glass.InsetShadow(0, -1, 2, 0, shadowInk.Alpha(0.08))
}
