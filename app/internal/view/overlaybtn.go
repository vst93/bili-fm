package view

import "github.com/egoist/mygo/ui"

// 封面叠加小键：列表卡片在封面上压的一颗小键。原版只有两处，而且样式来源
// 完全不同，不能混用一套取值：
//
//	选集「添加到播放列表」  pageList.tsx 的行内 Tailwind 类：
//	    absolute top-1 right-1 z-10 min-w-6 w-6 h-6 rounded-full
//	    bg-black/30 backdrop-blur-sm border-0
//	    → 24×24 正圆、贴右上 4px、纯黑 30% 填充、**无边框**、图标 14px
//	稍后再看「移除」        globals.css 的 .watchlater-remove-btn：
//	    top/right 6、26×26、圆角 7、1px 白 .48 描边、
//	    rgba(15,23,42,.62) 底、#f8fafc 图标；
//	    :hover 底 .76 + 图标 #fff + scale(1.04)；:disabled opacity .55
//
// 原生版此前两处都写成同一套「#0f172a 半透明 + 白 .2 描边」的近似值：选集
// 添加键的底/边框/图标尺寸都对不上，移除键则整体小一圈（22 vs 26）、圆角
// 偏方（4 vs 7）、贴角位置也偏内（4 vs 6）。这里把两处的取值集中一处，
// 沿用 topToolStyleFor / ownerFacePaintFor 的写法。

// ------------------------------------------------------------ 选集「添加」键

const (
	// partAddSize 是添加键的边长（原版 w-6 h-6 / min-w-6 = 24）。
	partAddSize = 24
	// partAddInset 是添加键贴封面右上角的距离（原版 top-1 right-1 = 4）。
	partAddInset = 4
	// partAddIconSize 是添加键图标尺寸（原版 <AddOne size={14} /> / <Check size={14} />）。
	partAddIconSize = 14
)

// partAddFill 是添加键的填充（原版 bg-black/30 = rgba(0,0,0,.30)）。
//
// 原版只给了基态一个类、没有 :hover 兄弟规则；这里悬停时在同色系里略加深，
// 给一点按压反馈（不引入原版 HeroUI variant 的灰底）。
func partAddFill(hovered bool) ui.Color {
	if hovered {
		return ui.Hex("#000000").Alpha(0.42)
	}
	return ui.Hex("#000000").Alpha(0.30)
}

// partAddInk 是添加键图标色：已在播放列表里是绿勾（#4ade80），否则白加号。
func partAddInk(inList bool) ui.Color {
	return pick(inList, ui.Hex("#4ade80"), ui.Hex("#ffffff"))
}

// ------------------------------------------------------- 稍后再看「移除」键

const (
	// wlRemoveSize 是移除键的边长（原版 .watchlater-remove-btn width/height 26）。
	wlRemoveSize = 26
	// wlRemoveInset 是移除键贴封面右上角的距离（原版 top/right 6px）。
	wlRemoveInset = 6
	// wlRemoveRadius 是移除键圆角（原版 border-radius 7px）。
	wlRemoveRadius = 7
	// wlRemoveIconSize 是移除键图标尺寸（原版 <Close size="12" />）。
	wlRemoveIconSize = 12
)

// wlRemoveFill 是移除键的底色（原版 .watchlater-remove-btn / :hover）：
// 常态 rgba(15,23,42,.62)，悬停 .76。
func wlRemoveFill(hovered bool) ui.Color {
	return ui.Hex("#0f172a").Alpha(pick(hovered, float32(0.76), float32(0.62)))
}

// wlRemoveInk 是移除键图标色（原版常态 #f8fafc、:hover #fff）。
func wlRemoveInk(hovered bool) ui.Color {
	return pick(hovered, ui.Hex("#ffffff"), ui.Hex("#f8fafc"))
}

// wlRemoveBorder 是移除键的 1px 白 .48 描边（原版 .watchlater-remove-btn）。
func wlRemoveBorder() ui.Color { return ui.Hex("#ffffff").Alpha(0.48) }

// wlRemoveScale 是移除键的悬停缩放（原版 :hover transform: scale(1.04)）。
func wlRemoveScale(hovered bool) float32 {
	return pick(hovered, float32(1.04), float32(1))
}

// wlRemoveOpacity 是移除键请求中的整体透明度（原版 :disabled opacity .55；
// 按钮置灰但保留位置，避免布局跳动）。
func wlRemoveOpacity(pending bool) float32 {
	return pick(pending, float32(0.55), float32(1))
}
