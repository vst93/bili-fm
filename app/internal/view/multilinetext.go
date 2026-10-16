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

// wrapHangingFirst 把 body 按 firstW 宽折出**第一行**，返回该行文本与余下部分。
// 供悬挂缩进的段落使用（原版楼中楼是「昵称: 正文」两个 inline span：正文首行
// 接着昵称、可用宽被昵称挤窄，续行却从容器左缘开始、可用宽是整行）。
// 断行仍交给框架（ShapeText + Caret 的 Y 抬高），只是把「第一行的宽」与
// 「续行的宽」分开喂给两次折行。
func wrapHangingFirst(body string, font ui.Font, firstW float32) (first, rest string) {
	if body == "" || firstW <= 0 {
		return "", body
	}
	layout := ui.ShapeText(body, font, firstW)
	runes := []rune(body)
	units := make([]int, len(runes)+1)
	for i, r := range runes {
		units[i+1] = units[i] + utf16.RuneLen(r)
	}
	prevY := layout.Caret(units[0]).Y
	for i := 1; i <= len(runes); i++ {
		if layout.Caret(units[i]).Y != prevY {
			return string(runes[:i]), string(runes[i:])
		}
	}
	return body, ""
}

// wrapHangingLines 折一个**悬挂缩进**的段落：第一行限宽 firstW，续行限宽
// restW（≥ firstW）。原版楼中楼正文（danmakuList.tsx:379）是「用户:正文」两个
// inline span 组成的段落，续行没有悬挂缩进 —— 首行接着昵称、被昵称的宽挤窄，
// 续行从容器左缘起、用整行宽。这里把两段宽分别折行后拼起来，复现这套 inline 流。
func wrapHangingLines(body string, font ui.Font, firstW, restW float32, maxLines int) []string {
	if maxLines <= 0 {
		return nil
	}
	if body == "" {
		return []string{""}
	}
	first, rest := wrapHangingFirst(body, font, firstW)
	lines := make([]string, 0, 4)
	if first != "" {
		lines = append(lines, first)
	}
	if rest != "" {
		lines = append(lines, wrapTextLines(rest, font, restW, maxLines)...)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		last := []rune(lines[maxLines-1])
		for len(last) > 0 && textWidth(string(last)+"…", font) > restW {
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

// paintHangingLines 同 paintTextLines，但首行与续行的**左缘不同**：首行在盒
// 左缘（盒左缘就是容器左缘 + 昵称宽 + 4），续行向左回退 indent（= 昵称宽 + 4）
// 到容器左缘 —— 复现原版楼中楼「昵称: 正文」两个 inline span 的悬挂缩进。
func paintHangingLines(p *ui.Painter, r ui.Rect, lines []string, font ui.Font, linePitch, indent float32, c ui.Color) {
	if linePitch <= 0 {
		return
	}
	_, shift := pxLineBox(font.Size, linePitch)
	for i, s := range lines {
		if s == "" {
			continue
		}
		x := r.X
		if i > 0 {
			x = r.X - indent
		}
		p.TextLayout(ui.ShapeText(s, font, 0), x, r.Y+float32(i)*linePitch+shift, c)
	}
}
