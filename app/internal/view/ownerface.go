package view

import (
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

// ownerGlassOpacity 是玻璃罩的透明度：常态 1，悬停 .7
// （原版 .video-owner-wrap:hover::before）。
func ownerGlassOpacity(hovered bool) float32 {
	if hovered {
		return 0.7
	}
	return 1
}

// ownerGlassTopGradient 是玻璃罩上半段（原版 180° 渐变的 0% → 45%：
// 白 .22 → 白 .06）。
func ownerGlassTopGradient() ui.LinearGradient {
	return ui.LinearGradient{
		From:  ui.Hex("#ffffff").Alpha(0.22),
		To:    ui.Hex("#ffffff").Alpha(0.06),
		Angle: 180,
		Start: 0,
		End:   0.45,
	}
}

// ownerGlassBottomGradient 是玻璃罩下半段（原版 180° 渐变的 45% → 100%：
// 白 .06 → 蓝 .06）。mygo 的 LinearGradient 只有两端，所以和上半段拼出三停靠点：
// 下半段在 45% 之前是透明的，45% 处由上半段的「45% 之后恒为白 .06」兜住。
func ownerGlassBottomGradient() ui.LinearGradient {
	return ui.LinearGradient{
		From:  ui.Transparent,
		To:    ui.Hex("#0ea5e9").Alpha(0.06),
		Angle: 180,
		Start: 0.45,
		End:   1,
	}
}

// ownerGlassOverlay 画 UP 主头像上的磨砂玻璃罩（原版 .video-owner-wrap::before）：
// 48×48 圆、锚在 wrap 左上、盖在头像上、不吃指针。
//
// 原版是一个三停靠点的渐变 + 两道 inset；mygo 的 LinearGradient 只有两端，
// 所以拆成两层绝对定位的圆：上层铺 0→45%，下层铺 45→100% 并带 inset 高光。
// 两层共用同一个 opacity（常态 1 / 悬停 .7）。
func ownerGlassOverlay(c *ui.Context, hovered bool) {
	op := ownerGlassOpacity(hovered)

	top := ui.Box(c).Absolute().Left(0).Top(0).
		Size(ownerGlassSize, ownerGlassSize).Radius(RadiusPill).PassThrough()
	top.Opacity(op)
	top.LinearGradient(ownerGlassTopGradient())

	bottom := ui.Box(c).Absolute().Left(0).Top(0).
		Size(ownerGlassSize, ownerGlassSize).Radius(RadiusPill).PassThrough()
	bottom.Opacity(op)
	bottom.LinearGradient(ownerGlassBottomGradient())
	// 原版的两道 inset（画在渐变之上）：上缘白高光、下缘墨影。
	bottom.InsetShadow(0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.60))
	bottom.InsetShadow(0, -1, 2, 0, shadowInk.Alpha(0.08))
}
