package view

import (
	"image"
	"math"
	"strings"
	"testing"
)

// 本轮（弹幕/评论抽屉的楼中楼正文）的测试。
//
// 原版 `main:src/components/danmakuList.tsx:379` 的楼中楼正文是
// `<span className="ml-1 text-default-600 dark:text-default-400">` —— 继承外层
// `div.text-xs` 的 **12px / 16px 行高**，可换行、无行数截断。框架把每行的行盒
// 夹在 `max(指定行高, ascent+descent)` 之上（mygo internal/text/layout.go），
// 12px 的自然行盒是 18，`LineHeight` 只能放大、压不下去，所以正文改成自绘逐行
// 落点（`replyBody`，linePitch = 16，见 multilinetext.go）。
//
// 自绘要在**布局期**知道折行数定盒高，而框架的布局期拿不到「本帧可用宽」
// （Context 只暴露 MeasureText），所以 `replyBodyWidthFor` 用一条与布局同源的
// 算式（窗口宽 − 固定内边距链 − 昵称实测宽）代替实测宽。这两条测试分别钉住
// 「算式 == 实际盒宽」与「逐行落点的行距 == 16」。

// replyBodyContent 是足够长、在常见窗口宽度下都会折成多行的楼中楼正文
// （8 段 × 16 字 ≈ 1536px，800 宽窗口下折 3 行）。
var replyBodyContent = strings.Repeat("楼中楼很长的回复内容重复很多遍。", 8)

// TestReplyBodyWidthMatchesLayout 钉住 replyBodyWidthFor 的算式与真实布局逐值
// 相等（换窗口宽度 / 换昵称长度都成立）。算式一旦与内边距链脱钩（比如
// commentRow 的 4px 6px 外层内边距、正文列 40 缩进改动），自绘折行数就会用错
// 宽度、盒高差一整行 —— 这条会立刻变红。
//
// 这里量的是 body 盒（Grow(1)）的整宽，它也是**首行**可用宽（原版首行接着
// 昵称，可用宽 = 容器 − 昵称宽 − 4）。
func TestReplyBodyWidthMatchesLayout(t *testing.T) {
	for _, winW := range []int{640, 800, 1000, 1400} {
		for _, user := range []string{"乙", "很长的用户名测试"} {
			a := commentRowApp("正文", Comment{User: user, Content: replyBodyContent})
			tt := shellTesterSize(t, a, winW, 1400)
			r, ok := tt.Find(replyBodyContent)
			if !ok {
				t.Fatalf("win=%d user=%q：找不到楼中楼正文", winW, user)
			}
			want := replyBodyWidthFor(float32(winW), user)
			if math.Abs(float64(r.W-want)) > 0.5 {
				t.Errorf("win=%d user=%q：楼中楼正文盒宽 = %v，replyBodyWidthFor = %v（算式与布局脱钩）",
					winW, user, r.W, want)
			}
		}
	}
}

// TestReplyBodyLinePitchPixels 是无头像素实测：楼中楼正文折行后的**行距**是
// 16px（原版 text-xs 的 16px 行高），不是框架的自然行盒 18px。取样正文盒的
// 墨迹行带，量相邻行带起点的间距。
func TestReplyBodyLinePitchPixels(t *testing.T) {
	a := commentRowApp("正文", Comment{User: "乙", Content: replyBodyContent})
	tt := shellTesterSize(t, a, 800, 1400)
	r, ok := tt.Find(replyBodyContent)
	if !ok {
		t.Fatal("找不到楼中楼正文")
	}
	if n := len(replyLines(800, Comment{User: "乙", Content: replyBodyContent})); n < 3 {
		t.Fatalf("测试素材只折了 %d 行，至少要 3 行才能量行距", n)
	}

	img := tt.Image()
	x0, x1 := int(r.X)+1, int(r.X+r.W)-1
	bands := inkBands(img, x0, x1, int(r.Y), int(r.Y+r.H))
	if len(bands) < 3 {
		t.Fatalf("正文盒里只找到 %d 条墨迹行带（%v），至少要 3 条", len(bands), bands)
	}
	for i := 1; i < len(bands); i++ {
		if d := bands[i] - bands[i-1]; d != 16 {
			t.Errorf("第 %d 行与第 %d 行的行距 = %d，应 16（原版 text-xs 12/16；框架自然行盒 18）",
				i+1, i, d)
		}
	}
	// 盒高必须是行数 × 16（不是 × 18）。
	if want := float32(len(bands)) * replyLinePitch; math.Abs(float64(r.H-want)) > 0.5 {
		t.Errorf("正文盒高 = %v，%d 行 × 16 应为 %v", r.H, len(bands), want)
	}
}

// inkBands 返回 [x0,x1) × [y0,y1) 里「有墨迹」的连续行带起点（行号，DIP==像素
// 因为无头默认 scale 1）。相邻行带之间允许 2px 的缝（字形的横向笔画之间）。
func inkBands(img *image.RGBA, x0, x1, y0, y1 int) []int {
	ink := func(y int) bool {
		for x := x0; x <= x1; x++ {
			i := img.PixOffset(x, y)
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) < 3*140 {
				return true
			}
		}
		return false
	}
	var bands []int
	last := -10
	for y := y0; y < y1; y++ {
		if !ink(y) {
			continue
		}
		if y-last > 2 {
			bands = append(bands, y)
		}
		last = y
	}
	return bands
}

// TestCommentCardWidthMatchesOriginal 钉住评论卡片的宽度：原版
// `.py-2.px-8.border-b` 被 globals.css 覆写成 `margin: 4px 6px` 的块级 div，
// 宽度 = 滚动区内容宽 − 12。原生版此前把卡片写成 `FillWidth()` + `Margin(4,6)`：
// ui.List 给行根的宽度恒为内容宽（layoutBox(r.e, cw, …)），margin 不参与宽度，
// 于是卡片比原版宽 12px、右缘探进 6px。现在外层盒吃 `Padding(4,6)`、卡片
// FillWidth，宽度回到原版。量卡片正文列的实际盒宽（= 卡片内容宽 − 40 缩进）。
func TestCommentCardWidthMatchesOriginal(t *testing.T) {
	for _, winW := range []int{640, 800, 1200} {
		a := commentRowApp("正文内容")
		tt := shellTesterSize(t, a, winW, 1400)
		r, ok := tt.Find("正文内容")
		if !ok {
			t.Fatalf("win=%d：找不到评论正文", winW)
		}
		// 窗口 − List 内边距 20 − 外层内边距 12 − 卡片边框 2
		// − 卡片内边距 64 − 正文列左缩进 40 = 窗口 − 138。
		// （抽屉面板原满四边 `Border(1)` 会再扣 2px；现改为「只 top 布局边 +
		// 全圈不占布局的聚光描边」，与原版 `border-top` 一致，不再计入。）
		want := float32(winW) - 138
		if math.Abs(float64(r.W-want)) > 0.5 {
			t.Errorf("win=%d：评论正文列盒宽 = %v，应 %v（卡片宽度按原版 margin 4px 6px 扣）",
				winW, r.W, want)
		}
	}
}

// TestReplyBodyHangingIndent 钉住楼中楼正文的**悬挂缩进**：原版 danmakuList.tsx
// 里「昵称:」与正文是两个 inline span，首行接着昵称、可用宽被昵称挤窄，续行从
// 容器左缘起、用整行宽（没有悬挂缩进）。此前原生版把正文与昵称分成 Row 两个
// 子元素，正文盒整列对齐昵称右缘，续行也带缩进。
//
// 无头像素实测：正文盒里第一行墨迹从盒左缘起，第二行墨迹回退「昵称宽 + 4」
// （= 盒左缘 − 昵称宽 − 4 = 容器左缘）。若续行不回退，两行墨迹左缘相同。
func TestReplyBodyHangingIndent(t *testing.T) {
	a := commentRowApp("正文", Comment{User: "乙", Content: replyBodyContent})
	tt := shellTesterSize(t, a, 800, 1400)
	r, ok := tt.Find(replyBodyContent)
	if !ok {
		t.Fatal("找不到楼中楼正文")
	}
	lines := replyLines(800, Comment{User: "乙", Content: replyBodyContent})
	if len(lines) < 3 {
		t.Fatalf("测试素材只折了 %d 行，至少要 3 行", len(lines))
	}
	nameW := textWidth("乙:", replyFont)
	wantIndent := nameW + replyFirstLinePad
	if nameW < 8 {
		t.Fatalf("昵称实测宽只有 %v，测不出悬挂缩进", nameW)
	}

	img := tt.Image()
	firstX := inkLeft(img, int(r.X), int(r.X+r.W), int(r.Y), int(r.Y+r.H))

	// 逐行量墨迹左缘（用行墨带定位每行）。
	bands := inkBands(img, int(r.X)-40, int(r.X+r.W), int(r.Y), int(r.Y+r.H))
	if len(bands) < 3 {
		t.Fatalf("只找到 %d 条墨迹行带，至少要 3 条", len(bands))
	}
	// 续行（第 2、3 行）应回退约 wantIndent（昵称宽 + 4），从容器左缘起。
	for i := 1; i < len(bands); i++ {
		y := bands[i]
		restX := inkLeft(img, int(r.X)-40, int(r.X+r.W), y, y+16)
		if got := float32(restX) - float32(firstX); got > -wantIndent+2 || got < -wantIndent-2 {
			t.Errorf("第 %d 行相对首行回退 %v，应 ≈%v（昵称宽 %v + 4）；两行左缘相同说明续行也带悬挂缩进",
				i+1, got, -wantIndent, nameW)
		}
	}
}

// inkLeft 返回 [x0,x1) × [y0,y1) 里第一个墨迹像素的 x（找不到返回 x1）。
func inkLeft(img *image.RGBA, x0, x1, y0, y1 int) int {
	for x := x0; x < x1; x++ {
		for y := y0; y < y1; y++ {
			i := img.PixOffset(x, y)
			if int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2]) < 3*140 {
				return x
			}
		}
	}
	return x1
}
