package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（内容操作行 / 工具条图标的**光学尺寸**）的测试。
//
// 原版 globals.css:2556 有一条统一覆写：
//
//	.video-content-actions .nav-icon-btn > svg,
//	.video-context-dock .nav-icon-btn > svg { width: 20px; height: 20px }
//
// JSX 里 source 写的是 18 / 20 两种 size，但 `size` 落到 SVG 的 width/height
// **表现属性**上，而 CSS 里表现属性的优先级最低 —— 原版 DOM 里这两组的 svg
// 实际一律 20px（原版注释：「统一图标光学尺寸：源码里混着 18/20 两种规格，
// 同排看有微妙的参差」）。cd94979 换 icon-park path 时按 JSX 把浏览器 / 视频 /
// 弹幕三枚改成了 18，把这条覆写漏掉了。

// iconInkBox 在按钮矩形里量图标墨迹（暗于底色的像素）的包围盒。内缩 4px
// 是为了避开按钮的圆角 / 描边（34 − 8 = 26 > 20，不会把 20px 的字形切掉）。
func iconInkBox(img *image.RGBA, r ui.Rect) image.Rectangle {
	x0, x1 := int(r.X)+8, int(r.X+r.W)-8
	y0, y1 := int(r.Y)+8, int(r.Y+r.H)-8
	// 空包围盒：image.Rect 会归一化（小值当 Min），所以不能用
	// image.Rect(x1, y1, x0, y0) 当「空」—— 那样起点就是整块采样区。
	bb := image.Rectangle{Min: image.Pt(x1, y1), Max: image.Pt(x0, y0)}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			o := img.PixOffset(x, y)
			if (int(img.Pix[o])+int(img.Pix[o+1])+int(img.Pix[o+2]))/3 < 200 {
				if x < bb.Min.X {
					bb.Min.X = x
				}
				if x >= bb.Max.X {
					bb.Max.X = x + 1
				}
				if y < bb.Min.Y {
					bb.Min.Y = y
				}
				if y >= bb.Max.Y {
					bb.Max.Y = y + 1
				}
			}
		}
	}
	return bb
}

// iconForLabel 把按钮的无障碍标签映射到它画的那枚图标（测试里逐枚量基线用）。
func iconForLabel(t *testing.T, label string) *ui.SVG {
	t.Helper()
	switch label {
	case "浏览器打开":
		return iconBrowser
	case "视频播放":
		return iconVideo
	case "弹幕、评论、章节与字幕":
		return iconText
	case "搜索结果":
		return iconSearch
	case "选集":
		return iconList
	case "合集":
		return iconSeries
	case "播放列表":
		return iconMusicList
	}
	t.Fatalf("没有 %q 对应的图标", label)
	return nil
}

// iconRenderedInk 在一个受控白底上单渲染一枚图标，返回墨迹的宽高 —— 用来对照
// 「元素写多少 px」与「字形实际画多大」。
func iconRenderedInk(ic *ui.SVG, size float32) (int, int) {
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Icon(c, ic).Size(size, size).TextColor(ui.Hex("#475569")).
			Absolute().Left(20).Top(20)
	}, 64, 64, 1)
	x0, x1 := 999, -1
	y0, y1 := 999, -1
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			if img.Pix[img.PixOffset(x, y)] < 200 {
				if x < x0 {
					x0 = x
				}
				if x > x1 {
					x1 = x
				}
				if y < y0 {
					y0 = y
				}
				if y > y1 {
					y1 = y
				}
			}
		}
	}
	return x1 - x0 + 1, y1 - y0 + 1
}

// TestIconOpticalSizeToken 钉住光学尺寸常量：原版 CSS 覆写是 20px。
func TestIconOpticalSizeToken(t *testing.T) {
	if iconOpticalSize != 20 {
		t.Fatalf("iconOpticalSize = %v，应为原版 %s 覆写的 20",
			iconOpticalSize, `svg { width/height: 20px }`)
	}
}

// TestContentActionIconsRenderedAt20 无头实测：内容操作行（浏览器 / 视频 /
// 弹幕）与工具条（搜索 / 选集 / 合集 / 播放列表）的图标墨迹边长必须一致地
// 落在 20px 那一档（比 18px 那一档宽 2px）。**反证**：把 iconOpticalSize
// 换回 18，前三枚会缩小 2px。
func TestContentActionIconsRenderedAt20(t *testing.T) {
	// 先量两个档位在同一枚图标上的字形尺寸，作为判据的基线。
	w20, _ := iconRenderedInk(iconBrowser, 20)
	w18, _ := iconRenderedInk(iconBrowser, 18)
	t.Logf("iconBrowser 字形：20px → %d，18px → %d", w20, w18)
	if w20 == w18 {
		t.Fatalf("20px 与 18px 渲染出的字形一样宽（%d），测试无法区分两档", w20)
	}

	tt := shellTesterSize(t, statCountApp(), 1000, 700)
	img := tt.Image()
	for _, label := range []string{"浏览器打开", "视频播放", "弹幕、评论、章节与字幕", "搜索结果", "选集", "合集", "播放列表"} {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("找不到按钮 %q", label)
		}
		bb := iconInkBox(img, r)
		gotW, gotH := bb.Dx(), bb.Dy()
		t.Logf("%s 图标墨迹 %d×%d", label, gotW, gotH)
		// 判据不按绝对像素：各枚 icon-park path 的字形内边距不同（选集 12×12、
		// 播放列表 16×14 都是原图自带的空白），所以逐枚拿它自己在 20px 档下的
		// 基线比。核心是「不能落在 18px 那一档」。
		base, _ := iconRenderedInk(iconForLabel(t, label), iconOpticalSize)
		if gotW != base {
			t.Errorf("%s 图标墨迹宽 %d，应与 %vpx 档基线 %d 一致",
				label, gotW, iconOpticalSize, base)
		}
		if gotW > 22 || gotH > 22 {
			t.Errorf("%s 图标墨迹 %d×%d 超出按钮（34）该有的量", label, gotW, gotH)
		}
	}
}

// TestContentActionIconsOpticalWired 是接线反证：把 iconOpticalSize 换回 18，
// 内容操作行前三枚的墨迹必须缩小。（同为 20 的后四枚在工具条上，不受影响。）
func TestContentActionIconsOpticalWired(t *testing.T) {
	measure := func() map[string]int {
		tt := shellTesterSize(t, statCountApp(), 1000, 700)
		img := tt.Image()
		out := map[string]int{}
		for _, label := range []string{"浏览器打开", "视频播放", "弹幕、评论、章节与字幕"} {
			r, ok := tt.Find(label)
			if !ok {
				t.Fatalf("找不到按钮 %q", label)
			}
			out[label] = iconInkBox(img, r).Dx()
		}
		return out
	}
	full := measure()
	old := iconOpticalSize
	iconOpticalSize = 18
	defer func() { iconOpticalSize = old }()
	small := measure()
	for label, w := range full {
		t.Logf("%s 墨迹宽：20px → %d，18px → %d", label, w, small[label])
		if small[label] >= w {
			t.Errorf("%s 缩到 18px 后墨迹宽 %d 没有变小（原 %d）——iconOpticalSize 没接线",
				label, small[label], w)
		}
	}
}
