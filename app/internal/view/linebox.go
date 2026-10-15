package view

import "github.com/egoist/mygo/ui"

// 框架把每一行文本的行盒高度夹在 `max(指定行高, 字体 ascent+descent)` 之上
// （mygo 的 internal/text/layout.go：`line.Height = max(m.lineHeight,
// line.Ascent+line.Descent)`）。于是 LineHeight() 只能把行距放大、压不下去：
// 原版 CSS 里那些**比自然行盒矮**的 line-height（1 / 1.2 / 1.25 / 18px…）
// 在原生版里一律退回到自然行盒（见 naturalLineBox），单行元素因此比原版
// 高一截，进而把周围的布局（上下间距、整块居中）也推偏。
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

// naturalLineBox 返回框架在**默认行高**（未指定 line-height）下给一行文本的
// 行盒高度：字体度量的 ascent + descent + lineGap。框架把每行夹在
// `max(指定行高, ascent+descent)` 之上，未指定行高时 m.lineHeight 就是
// `ascent+descent+lineGap`，正是 ui.Font.Metrics() 三个分量之和 —— 无头实测
// 「元素盒高 == Metrics() 之和」逐字号成立。
//
// 此前这里写死 `1.5×字号`（本机 Pango 的近似）。实测该近似并不准：10→15、
// 11→17、12→18、14→22、15→23、22→33（比值 1.5 / 1.545 / 1.5 / 1.571 /
// 1.533 / 1.5）—— 11 / 14 / 15px 都偏 0.5px（14px 的实际行盒是 22 不是 21），
// half-leading 跟着偏，字形落点差四分之一个像素到半个像素；换成 Windows
// （DirectWrite）或 macOS（CoreText）的度量后差得更多。改成直接问框架要度量，
// 换算出的位移就与框架**实际**用的行盒逐值吻合。
//
// 它是变量是为了让纯算术测试能换桩（见 linebox_test.go），生产路径恒为实测。
var naturalLineBox = naturalLineBoxMeasured

// naturalLineBoxMeasured 用框架自己的字体度量量出自然行盒。字号 ≤ 0 时按 0
// （没有文本，不需要位移）。
func naturalLineBoxMeasured(size float32) float32 {
	if size <= 0 {
		return 0
	}
	m := ui.Font{Size: size}.Metrics()
	return m.Ascent + m.Descent + m.LineGap
}

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
	return pxLineBox(size, cssLineHeight*size)
}

// pxLineBox 同 textLineBox，但原版的 line-height 是**写死的像素值**、不随字号
// 缩放。HeroUI 的两个文本 slot 类就是这种：`text-small`（0.875rem / 1.25rem
// ⇒ 行高 20px）与 `text-tiny`（0.75rem / 1rem ⇒ 行高 16px）——组件只覆写
// font-size 时行高仍是那 20px / 16px，不会跟着字号缩。合作 UP 主选择面板的
// 两行（title/description）正是这样（见 collabpicker.go）。
func pxLineBox(size, cssLineHeight float32) (height, shift float32) {
	natural := naturalLineBox(size)
	return cssLineHeight, (cssLineHeight - natural) / 2
}
