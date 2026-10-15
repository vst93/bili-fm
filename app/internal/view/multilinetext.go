package view

import (
	"math"
	"unicode/utf16"

	"github.com/egoist/mygo/ui"
)

// 框架把每一行文本的行盒高度夹在 `max(指定行高, 字体 ascent+descent)` 之上
// （mygo 的 internal/text/layout.go：`line.Height = max(m.lineHeight,
// line.Ascent+line.Descent)`），而 line.Ascent/Descent 至少是字体度量的
// ascent/descent —— 于是 `LineHeight()` 只能把行距**放大**、压不下去（见
// linebox.go 的说明）。单行元素可以靠「显式盒高 + 负 half-leading 位移」复现
// （textLineBox / pxLineBox），但多行文本没有逐行的 API：像 `#video-title h2`
// 这样 `line-height: 1.18`（22px → 25.96）比自然行盒（22px 中文实测 33 =
// 1.5em）矮两行的文本，框架会把行距顶到 33，第二行比原版低 7px。
//
// 复现办法：自己按框架的断行结果把每一行单独成型，再用 `Painter.TextLayout`
// 按原版行距逐行落点绘制 —— 断行规则、字体回退、双向文本都仍交给框架，只把
// 「行距」这一个被夹住的量拿回来。

// wrapTextLines 用框架自身的断行规则（ui.ShapeText）把 s 折到 width 宽
// （DIP），最多返回 maxLines 行。超过 maxLines 时末行截断并补「…」，复现原版
// `-webkit-line-clamp: 2`。width ≤ 0 或 maxLines ≤ 0 时退回单行 / 空。
func wrapTextLines(s string, font ui.Font, width float32, maxLines int) []string {
	if s == "" || maxLines <= 0 {
		return nil
	}
	if width <= 0 {
		return []string{s}
	}
	layout := ui.ShapeText(s, font, width)
	runes := []rune(s)
	// ShapeText 的 Caret 用 UTF-16 索引，先把每个 rune 边界映射过去，逐 rune
	// 观察 Caret 的 Y：Y 一旦抬高，就是框架在此处软换行了。
	units := make([]int, len(runes)+1)
	for i, r := range runes {
		units[i+1] = units[i] + utf16.RuneLen(r)
	}
	var lines []string
	start := 0
	prevY := float32(math.Inf(-1))
	for i := 0; i <= len(runes); i++ {
		y := layout.Caret(units[i]).Y
		if i == 0 {
			prevY = y
			continue
		}
		if y != prevY {
			lines = append(lines, string(runes[start:i]))
			start = i
			prevY = y
		}
	}
	lines = append(lines, string(runes[start:]))

	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := []rune(lines[maxLines-1])
		// 逐步砍到「末行 + …」放得下（原版行尾省略号）。
		for len(last) > 0 && textWidth(string(last)+"…", font) > width {
			last = last[:len(last)-1]
		}
		lines[maxLines-1] = string(last) + "…"
	}
	return lines
}

// textWidth 是单行文本的实测宽度（DIP），用于省略号截断判定。
func textWidth(s string, font ui.Font) float32 {
	if s == "" {
		return 0
	}
	w, _ := ui.ShapeText(s, font, 0).Size()
	return w
}

// paintTextLines 把 lines 逐行画在盒 r 里，行距固定为原版 CSS 的
// linePitch（px）。每一行都单独成型成单行布局，再套上负 half-leading 位移
// （与 textLineBox 同一套换算），字形落点因此由 linePitch 决定，而不是框架的
// 自然行盒 —— 这正是多行文本行距压不下去的绕过办法。
func paintTextLines(p *ui.Painter, r ui.Rect, lines []string, font ui.Font, linePitch float32, c ui.Color) {
	if linePitch <= 0 {
		return
	}
	_, shift := pxLineBox(font.Size, linePitch)
	for i, s := range lines {
		if s == "" {
			continue
		}
		p.TextLayout(ui.ShapeText(s, font, 0), r.X, r.Y+float32(i)*linePitch+shift, c)
	}
}
