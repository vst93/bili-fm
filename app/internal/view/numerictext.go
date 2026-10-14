package view

import "github.com/egoist/mygo/ui"

// tabularNums 是数字文本要打开的 OpenType 特性，对应原版 CSS 的
// `font-variant-numeric: tabular-nums`：数字等宽，数值变化时文本宽度不跳。
//
// 原版给这些数值文本都写了这一条（globals.css）：
//
//	.c-cover-duration               卡片封面时长角标
//	.nav-icon-btn > .nav-stat-value 赞/币/藏计数
//	#player .player-speed-button    倍速键
//	#player .player-time            播放时间（主窗与 mini 共用）
//	#player .player-seek-bubble     进度条悬停时间气泡
//	#player .player-speed-option    倍速档位
//	.danmaku-time-label             弹幕时间标签
//	.danmaku-repeat-count           弹幕重复数 ×N
//
// mygo 没有 `font-variant-numeric`，对应的 API 是 Element.FontFeatures
// （OpenType feature tag），语义相同。
var tabularNums = []string{"tnum"}

// numericText 建一段数字文本并打开等宽数字特性（原版 tabular-nums）。
// 调用方照常链式设置字号 / 颜色 / 布局。
//
// 做成 var 是为了让测试换桩统计调用点（同 playerTimeInk / drawerFadeColor）。
var numericText = func(c *ui.Context, s string) ui.Element {
	return ui.Text(c, s).FontFeatures(tabularNums...)
}
