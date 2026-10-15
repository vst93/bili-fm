package view

// 框架把每一行文本的行盒高度夹在 `max(指定行高, 字体 ascent+descent)` 之上
// （mygo 的 internal/text/layout.go：`line.Height = max(m.lineHeight,
// line.Ascent+line.Descent)`）。于是 LineHeight() 只能把行距放大、压不下去：
// 原版 CSS 里那些**比自然行盒矮**的 line-height（1 / 1.2 / 1.25 / 18px…）
// 在原生版里一律退回到自然行盒（约 1.5×字号），单行元素因此比原版高一截，
// 进而把周围的布局（上下间距、整块居中）也推偏。
//
// 复现办法（弹幕时间标签 / 「N 条」用过，见 drawer.go 的 danmakuLabelHeight /
// danmakuTotalHeight）分两步：
//
//  1. 用 Height() 给元素一个**显式的盒高** = 原版行盒（Height 不参与行高夹取，
//     文字超出盒高的部分照常绘制 —— 原版 CSS 的 overflow 也是可见的）；
//  2. 用 Translate() 把字形上移原版的**负 half-leading** =（原版行盒 − 自然
//     行盒）/ 2，字形的落点才与原版一致。
//
// 只对**单行**文本成立：多行文本的行**间距**由框架逐行决定，盒高压不掉行距，
// 所以标题 / 简介 / 楼中楼这类多行文本不适用（框架硬底）。

// naturalLineRatio 是框架的自然行盒（ascent+descent）与字号的比例。mygo 取
// 字体的垂直度量；本机（Pango）实测 10 / 11 / 12 / 15 / 22px 都是 1.5×
// （ascent 0.8 + descent 0.2 的度量族），取 1.5。
const naturalLineRatio = 1.5

// 原版 CSS 里两个最常用的字号类（Tailwind / HeroUI 默认）的行盒倍数，供
// textLineBox 压回原版行高：text-sm（HeroUI 的 text-small 同值）是
// 14px / 20px 行高，text-xs 是 12px / 16px 行高。卡片页脚、播放列表行等处
// 的单行文本都用这两个。
const (
	textSmLH = 20.0 / 14.0
	textXsLH = 16.0 / 12.0
)

// textLineBox 返回把单行文本压到原版 CSS 的 line-height 时，元素盒高（不含
// 内边距）与字形需要上移的量。size 是字号，cssLineHeight 是原版 line-height
// 的倍数（如 1 / 1.2 / 1.25）。盒高恒为正、shift 恒为非正（自然行盒 ≥ 原版
// 行盒时才需要上移）。
func textLineBox(size, cssLineHeight float32) (height, shift float32) {
	natural := naturalLineRatio * size
	height = cssLineHeight * size
	shift = (height - natural) / 2
	return
}
