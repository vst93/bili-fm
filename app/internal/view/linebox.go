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
//  2. 用**负的上内边距**（`Padding(shift, 0, 0, 0)`，shift < 0）把字形上移原版
//     的**负 half-leading** =（原版行盒 − 自然行盒）/ 2，字形的落点才与原版一致。
//
// 第 2 步早先写的是 `Translate(0, shift)`，2026-10 的内存排查改成了负内边距：
// mygo 的 GPU 构建器（internal/gpu.markEnclosing）会把「带 transform 的内容」
// 所在的每一个圆角 clip 升级成**预烘焙遮罩**的 clip。卡片/弹幕/播放列表里几十个
// 上移半个像素的文本因此让抽屉面板和每张卡片都摊上一张遮罩：稳态实测 103 个
// 遮罩槽、约 52MB 像素（GPU 侧再一份），面积每帧抖 1px 还要重新光栅化。
// 负内边距只是布局（Height 是 border-box，外框尺寸不变），同样的落点、零
// transform、零遮罩。
//
// 只对**单行**文本成立。多行文本的行**间距**由框架逐行决定，盒高压不掉行距 ——
// 要么退回到自然行盒，要么像 `#video-title h2` 那样自绘逐行落点
// （见 multilinetext.go 的 wrapTextLines / paintTextLines，同一套负 half-leading）。

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
//
// 量的是**拉丁**行盒（主题字体栈里的 Inter）：数字、时间、英文标签这类单行
// 文本用它最准。中文文本的行盒由回退字体决定，比它高 3~5px，固定高度容器里
// 放中文要用 cjkLineBox（见下）。
func naturalLineBoxMeasured(size float32) float32 {
	if size <= 0 {
		return 0
	}
	m := fontOf(size, 0).Metrics()
	return m.Ascent + m.Descent + m.LineGap
}

// cjkLineBox 返回一行**中文**文本在框架里的自然行盒。
//
// 主题字体栈（font.go）里 Inter 只有拉丁字形，汉字由回退字体绘制：macOS 的
// PingFang SC、Windows 的 Microsoft YaHei UI、Linux 的 Noto Sans CJK SC。
// 行盒取这些字体的 ascent+descent，比 Inter 的拉丁行盒高（14px 实测：拉丁
// 18px，中文 22px）。固定高度容器里放中文 —— 桌面字幕窗、多行标题的逐行
// 落点 —— 必须按中文行盒算，否则会按拉丁度量算出偏矮的盒子，裁掉半行字。
//
// 用「字」真正成型一次问框架要行盒，而不是写死倍数：三平台的中文回退字体
// 不是同一款，度量本来就不同，量出来才是框架这一帧实际用的值。
var cjkLineBox = cjkLineBoxMeasured

// cjkLineBoxMeasured 是 cjkLineBox 的实现，声明成变量是为了纯算术测试能换桩
// （同 naturalLineBox）。
func cjkLineBoxMeasured(size float32) float32 {
	if size <= 0 {
		return 0
	}
	_, h := ui.ShapeText("字", fontOf(size, 0), 0).Size()
	return h
}

// textLineBoxFor 返回**这段文字**在框架里的自然行盒：拉丁（Inter）与中文
//（回退字体）各自的度量不同，混排在同一行时要用同一个共用盒高 + 每段文字各自
// 的居中位移，盒子和墨迹才对得齐。
//
// 典型：选集胶囊里「当前选集」（中文）、分集标题「P1」（拉丁）、「152 人在看」
//（混排）三者 13px 的行盒是 20 / 17 / 20，不统一的话盒顶各差半个行盒差。
func textLineBoxFor(s string, size float32) float32 {
	if s == "" || size <= 0 {
		return 0
	}
	_, h := ui.ShapeText(s, fontOf(size, 0), 0).Size()
	return h
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
	return halfLeading(naturalLineBox(size), cssLineHeight)
}

// cjkTextLineBox 同 textLineBox，但按**中文**的自然行盒算 half-leading：多行
// 标题（视频标题、迷你标题）逐行落点画的是中文，行盒得按中文回退字体的度量。
func cjkTextLineBox(size, cssLineHeight float32) (height, shift float32) {
	return halfLeading(cjkLineBox(size), cssLineHeight*size)
}

// halfLeading 返回指定行高下元素盒高与字形需要的上移量：原版 CSS 的负
// half-leading = (行高 − 自然行盒)/2。
func halfLeading(natural, cssLineHeight float32) (height, shift float32) {
	return cssLineHeight, (cssLineHeight - natural) / 2
}
