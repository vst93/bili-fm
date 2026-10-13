package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// 搜索药丸里的两类小键共用原版 globals.css 的一套状态：
//
//	.top-tool-btn       四个内容入口（动态 / 热门与推荐 / 收藏 / 历史）
//	.search-submit-btn  输入框右端的放大镜提交键
//
// 原版给两者的基态 / 悬停 / 按下逐字相同，只有**悬停底色**不同
// （.top-tool-btn 是 rgba(255,255,255,.65)，.search-submit-btn 是 .50）：
//
//	基态  背景透明、opacity .82、图标 #64748b
//	悬停  铺白底、opacity 1、图标转蓝 #0284c7、translateY(-1px)
//	按下  缩到 scale(.92)、底换 --glass-bg-active（白 .16），
//	      color / opacity 沿用悬停（:active 不重置它们）
//
// 之前原生版把这四颗入口键当成 .nav-icon-btn 画（图标用 --studio-muted
// #475569、悬停底色白 .30、按下缩 .94），于是悬停时图标不变蓝、常态也比
// 原版深一档。这里把状态集中一处（同 playlistRowPaint / settingsItemPaint）。
type topToolKind int

const (
	topToolEntry  topToolKind = iota // .top-tool-btn
	topToolSubmit                    // .search-submit-btn
)

// topToolStyle 是某个状态下解析好的样式。
type topToolStyle struct {
	Bg      ui.Color
	Ink     ui.Color
	Opacity float32
}

// topToolHoverBg 是两类键悬停底色的唯一差别（原版 .top-tool-btn 白 .65、
// .search-submit-btn 白 .50）。
func topToolHoverBg(k topToolKind) ui.Color {
	if k == topToolSubmit {
		return ui.Hex("#ffffff").Alpha(0.50)
	}
	return ui.Hex("#ffffff").Alpha(0.65)
}

// topToolStyleFor 解析三态。按下只覆盖底色，图标色与不透明度沿用悬停。
func topToolStyleFor(k topToolKind, hovered, pressed bool) topToolStyle {
	s := topToolStyle{
		Bg:      ui.Transparent,
		Ink:     ui.Hex("#64748b"), // 原版 .top-tool-btn / .search-submit-btn 的 color
		Opacity: 0.82,
	}
	if hovered || pressed {
		s.Bg = topToolHoverBg(k)
		s.Ink = ui.Hex("#0284c7")
		s.Opacity = 1
	}
	if pressed {
		s.Bg = ui.Hex("#ffffff").Alpha(0.16) // --glass-bg-active
	}
	return s
}

// topToolPose 给搜索药丸小键加原版的手感：悬停上浮 1px、按下缩到 .92。
// 与 pressPose（.nav-icon-btn，缩 .94）分开 —— 原版 .top-tool-btn /
// .search-submit-btn 的 :active 就是 .92。
func topToolPose(b ui.Element) {
	pressPoseScale(b, 0.92)
}

// shadowSpec 是一条 box-shadow（原版 CSS 里的 x y blur spread color）。
type shadowSpec struct {
	X, Y, Blur, Spread float32
	C                  ui.Color
}

// searchPillShadows 返回搜索药丸的两态投影。原版 box-shadow 走
// --liquid-shadow / --liquid-shadow-lifted，各两条、**二选一**；mygo 的
// Shadow 是累加的，所以集中在这里给出，调用点不再「先铺常态再叠悬停」
// （那会同时挂上四道投影）。
func searchPillShadows(hovered bool) []shadowSpec {
	if hovered {
		return []shadowSpec{
			{0, 16, 38, 0, ui.Hex("#0f172a").Alpha(0.11)},
			{0, 2, 8, 0, ui.Hex("#0f172a").Alpha(0.04)},
		}
	}
	return []shadowSpec{
		{0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.07)},
		{0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03)},
	}
}

// applyShadows 按顺序把一串 box-shadow 落到元素上。
func applyShadows(e ui.Element, ss []shadowSpec) ui.Element {
	for _, s := range ss {
		e.Shadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	return e
}

// avatarShadow 给头像键补原版 .home-user-avatar 的两态投影：
// 常态 0 6px 16px rgba(15,23,42,.08)；悬停 0 12px 28px rgba(14,165,233,.2)。
// mygo 的投影是累加的，所以两态必须二选一（不能用「先铺常态再叠悬停」）。
func avatarShadow(b ui.Element) {
	if b.Hovered() {
		b.Shadow(0, 12, 28, 0, ui.Hex("#0ea5e9").Alpha(0.20))
		return
	}
	b.Shadow(0, 6, 16, 0, shadowInk.Alpha(0.08))
}

// avatarLift 是原版 .home-user-avatar:hover 的 translateY(-1px)。
// 头像键没有 :active 缩放，所以只上浮。
func avatarLift(b ui.Element) {
	if !b.Hovered() {
		return
	}
	b.AnimatePose(ui.Pose{Y: -1}, ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut})
}
