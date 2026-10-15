package view

import (
	"image"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（多行文本行距：宽窗标题自绘逐行落点）的测试。
//
// 框架把每行行盒夹在 `max(指定行高, 字体 ascent+descent)` 之上，多行文本的
// **行距**压不下去（见 multilinetext.go）。原版 `#video-title h2` 的
// line-height 1.18 × 22 ≈ 26，框架实测却按自然行盒 33 排 —— 第二行低 7px。
// wrapTextLines / paintTextLines 拿回行距控制权。

func titleWrapFont() ui.Font { return ui.Font{Size: 22, Weight: 800} }

// TestWrapTextLines 钉住折行：每行不超宽、不截断时拼回原文、越界时末行补「…」。
func TestWrapTextLines(t *testing.T) {
	font := titleWrapFont()
	const width = 200
	// 11 个汉字：每行放得下 9 个（9×22 = 198 ≤ 200），所以正好两行、不截断。
	s := "这是一个很长的视频标题"
	lines := wrapTextLines(s, font, width, 2)
	if len(lines) != 2 {
		t.Fatalf("两行截断得到 %d 行（%q）", len(lines), lines)
	}
	if got := lines[0] + lines[1]; got != s {
		t.Errorf("两行都放得下时不应截断：拼回 %q，原文 %q", got, s)
	}
	for i, l := range lines {
		if w := textWidth(l, font); w > width {
			t.Errorf("第 %d 行宽 %v 超过 %v：%q", i, w, width, l)
		}
	}

	// maxLines=1：末行补省略号，且含省略号仍不超宽。
	long := s + "用于测试两行截断"
	one := wrapTextLines(long, font, width, 1)
	if len(one) != 1 {
		t.Fatalf("单行截断得到 %d 行", len(one))
	}
	if !strings.HasSuffix(one[0], "…") {
		t.Errorf("单行截断末行 %q 应以省略号结尾", one[0])
	}
	if w := textWidth(one[0], font); w > width {
		t.Errorf("截断后行宽 %v 超过 %v：%q", w, width, one[0])
	}
	if !strings.HasPrefix(long, strings.TrimSuffix(one[0], "…")) {
		t.Errorf("截断行 %q 应是原文前缀 + 省略号", one[0])
	}

	// 边界：空串、maxLines ≤ 0、width ≤ 0。
	if lines := wrapTextLines("", font, width, 2); lines != nil {
		t.Errorf("空串应返回 nil，got %q", lines)
	}
	if lines := wrapTextLines(s, font, width, 0); lines != nil {
		t.Errorf("maxLines 0 应返回 nil，got %q", lines)
	}
	if lines := wrapTextLines(s, font, 0, 2); len(lines) != 1 || lines[0] != s {
		t.Errorf("width ≤ 0 应原样单行，got %q", lines)
	}
}

// titleInkRuns 在 800×600 整窗里量标题盒下方每条「有墨迹的行」（灰度 < 140）的
// 起止 y。返回墨迹段、标题盒与整张位图。
func titleInkRuns(t *testing.T, title string) ([][2]int, ui.Rect, *image.RGBA) {
	t.Helper()
	a := descApp()
	a.Info.Title = title
	a.Track.Title = title
	a.Info.Desc = "这是视频简介的第一行文字"
	tt := shellTesterSize(t, a, 800, 600)
	tr, ok := tt.Find(title)
	if !ok {
		t.Fatalf("找不到标题 %q", title)
	}
	img := tt.Image()
	x0, x1 := int(tr.X)-2, int(tr.X+tr.W)+2
	if x0 < 0 {
		x0 = 0
	}
	if x1 > 800 {
		x1 = 800
	}
	var runs [][2]int
	start := -1
	for y := int(tr.Y); y < int(tr.Y)+140; y++ {
		dark := false
		for x := x0; x < x1; x++ {
			if grayAt(img, x, y) < 140 {
				dark = true
				break
			}
		}
		if dark && start < 0 {
			start = y
		}
		if !dark && start >= 0 {
			runs = append(runs, [2]int{start, y - 1})
			start = -1
		}
	}
	return runs, tr, img
}

// TestVideoTitleLinePitchMatchesOriginal 是像素实测：两行标题的墨迹行起点差
// 应等于原版行距 `1.18 × 22 ≈ 26px`，而不是框架自然行盒的 33px。
//
// 反证（同测试内）：把 infoTitlePitch 换成自然行盒（33，即框架原来的行为），
// 第二行墨迹会整体下移 7px —— 起点差随之变大。
func TestVideoTitleLinePitchMatchesOriginal(t *testing.T) {
	title := "这是一个相当长的视频标题用来测试两行截断的效果另一个很长的视频标题"

	measure := func() float32 {
		runs, tr, _ := titleInkRuns(t, title)
		if len(runs) < 2 {
			t.Fatalf("只看得到 %d 段墨迹（%v），拿不到两行标题", len(runs), runs)
		}
		// 前两段墨迹都在标题盒内（第二行墨迹允许溢出盒底一点点，原版也是）。
		if runs[0][0] < int(tr.Y) || runs[0][0] > int(tr.Y)+8 {
			t.Fatalf("第一行墨迹顶 %d 不在标题盒顶 %v 附近", runs[0][0], tr.Y)
		}
		return float32(runs[1][0] - runs[0][0])
	}

	got := measure()
	if math.Abs(float64(got-infoTitlePitch)) > 1 {
		t.Errorf("两行标题行距 %v，want %v（原版 line-height 1.18 × 22）", got, infoTitlePitch)
	}
	if got >= 30 {
		t.Errorf("行距 %v 像框架的自然行盒（33），不是原版 1.18（26）", got)
	}

	// 反证：换回自然行盒（框架原行为）应把第二行推低。
	old := infoTitlePitch
	infoTitlePitch = naturalLineBox(22)
	defer func() { infoTitlePitch = old }()
	if back := measure(); math.Abs(float64(back-33)) > 1 {
		t.Errorf("换回自然行盒后行距 %v，want ≈33（证明行距确实由 infoTitlePitch 驱动）", back)
	}
}

// TestVideoTitleInkWiderThanFrameNatural 证明自绘没有把标题画丢：两行标题都
// 有墨迹、且在标题盒宽度内（横向不跑到窗外）。
func TestVideoTitleInkWiderThanFrameNatural(t *testing.T) {
	title := "这是一个相当长的视频标题用来测试两行截断的效果另一个很长的视频标题"
	runs, tr, img := titleInkRuns(t, title)
	if len(runs) < 2 {
		t.Fatalf("标题墨迹段只有 %d 段（%v）", len(runs), runs)
	}
	// 第二行墨迹应落在标题盒内（顶在盒内、横向在盒宽内）。
	if runs[1][0] < int(tr.Y) || runs[1][0] > int(tr.Y)+int(tr.H) {
		t.Errorf("第二行墨迹顶 %d 不在标题盒 [%v, %v] 内", runs[1][0], tr.Y, tr.Y+tr.H)
	}
	// 标题盒右侧一列（盒右 + 6）在标题行范围内应无墨迹，证明确实折到盒宽内。
	x := int(tr.X+tr.W) + 6
	if x < 800 {
		for y := int(tr.Y); y < int(tr.Y)+int(tr.H); y++ {
			if grayAt(img, x, y) < 140 {
				t.Errorf("标题盒右外侧 x=%d y=%d 有墨迹，说明没折到盒宽内", x, y)
				break
			}
		}
	}
}
