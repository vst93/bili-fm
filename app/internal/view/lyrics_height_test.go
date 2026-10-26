package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLyricsDisplayHeightScalesWithFont 钉住「显示高度随字号自适应」：
// 字号越大窗口越高，且默认 20px 时仍是约 100px（不会突然变大变小）。
// 自然行盒换成 1.5×字号的桩，纯算术测试不依赖平台字体度量。
func TestLyricsDisplayHeightScalesWithFont(t *testing.T) {
	defer stubNaturalLineBox(func(size float32) float32 { return 1.5 * size })()

	h14 := LyricsDisplayHeight(14)
	h20 := LyricsDisplayHeight(20)
	h30 := LyricsDisplayHeight(30)
	if !(h14 < h20 && h20 < h30) {
		t.Fatalf("显示高度应随字号递增：14→%d, 20→%d, 30→%d", h14, h20, h30)
	}
	// 20 + 2×30 + 2 + 19.5 = 101.5，向上取整 102。
	if h20 != 102 {
		t.Errorf("20px 字号显示高度 = %d，want 102", h20)
	}
	// 14 + 30 的端点也要覆盖：14px 时下一句夹到 11px（16.5 行盒）。
	// 20 + 2×21 + 2 + 16.5 = 80.5，向上取整 81。
	if h14 != 81 {
		t.Errorf("14px 字号显示高度 = %d，want 81", h14)
	}
	// 30px 时下一句 19.5px（29.25 行盒）：20 + 90 + 2 + 29.25 = 141.25 → 142。
	if h30 != 142 {
		t.Errorf("30px 字号显示高度 = %d，want 142", h30)
	}
}

// TestLyricsDisplayHeightClampsFont 钉住字号兜底：0 / 过小按默认 20，
// 过大按上限 30。窗口高度不能因为脏配置变成负数或无限高。
func TestLyricsDisplayHeightClampsFont(t *testing.T) {
	defer stubNaturalLineBox(func(size float32) float32 { return 1.5 * size })()

	if got, want := LyricsDisplayHeight(0), LyricsDisplayHeight(20); got != want {
		t.Errorf("未设置字号高度 = %d，want 默认 20px 的 %d", got, want)
	}
	if got, want := LyricsDisplayHeight(13), LyricsDisplayHeight(20); got != want {
		t.Errorf("过小字号高度 = %d，want 默认 20px 的 %d", got, want)
	}
	if got, want := LyricsDisplayHeight(80), LyricsDisplayHeight(30); got != want {
		t.Errorf("过大字号高度 = %d，want 上限 30px 的 %d", got, want)
	}
}

// TestLyricsNextFontSizeFloor 钉住下一句字号的 65% 与 11px 下限，
// 避免小字号时下一句缩成看不清的 9px。
func TestLyricsNextFontSizeFloor(t *testing.T) {
	if got := lyricsNextFontSize(20); got != 13 {
		t.Errorf("20px 的下一句字号 = %v，want 13", got)
	}
	if got := lyricsNextFontSize(14); got != 11 {
		t.Errorf("14px 的下一句字号应夹到 11，得到 %v", got)
	}
}

// TestLyricsConfigPanelFitsAtMaxFontSize 钉住设置页在最大字号下仍完整：
// 预览随字号长高后，设置窗也要跟着长高（= 显示页 + 固定面板），否则最下面
// 的说明文字会被窗口裁掉。上下两种展开方向都验一遍。
func TestLyricsConfigPanelFitsAtMaxFontSize(t *testing.T) {
	for _, above := range []bool{false, true} {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.LyricsFontSize = 30
		a.LyricsConfig = true
		a.LyricsConfigAbove = above
		a.LyricCurrent = "当前字幕"
		a.LyricNext = "下一句"
		h := LyricsDisplayHeight(30) + LyricsConfigPanelHeight
		tt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, h)
		tt.Frame()
		hint, ok := tt.Find("正数提前、负数延后；上方预览会实时更新。")
		if !ok {
			t.Fatalf("above=%v：找不到设置页说明文字", above)
		}
		if hint.Y+hint.H > float32(h) {
			t.Errorf("above=%v：说明文字底部 %v 超出窗口高 %d（被裁掉）", above, hint.Y+hint.H, h)
		}
		done, ok := tt.Find("完成")
		if !ok {
			t.Fatalf("above=%v：找不到「完成」键", above)
		}
		if done.Y < 0 || done.Y+done.H > float32(h) {
			t.Errorf("above=%v：「完成」键 %+v 不在窗口内", above, done)
		}
	}
}

// TestLyricsDisplayHeightFitsActualText 用真实渲染验证高度算式：当前句故意
// 长到换行（两行），下一句也画出来，量到的内容高 + 上下内边距必须正好等于
// LyricsDisplayHeight。矮了会裁掉第二行，高了会留一圈难看的空白。
func TestLyricsDisplayHeightFitsActualText(t *testing.T) {
	for _, size := range []float64{14, 20, 30} {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.LyricsFontSize = size
		a.LyricCurrent = strings.Repeat("很长的字幕内容", 20) // 一定换行
		a.LyricNext = "下一句字幕"
		h := LyricsDisplayHeight(size)
		tt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, h)
		tt.Frame()
		cur, ok := tt.Find(a.LyricCurrent)
		if !ok {
			t.Fatalf("%vpx：找不到当前句", size)
		}
		next, ok := tt.Find(a.LyricNext)
		if !ok {
			t.Fatalf("%vpx：找不到下一句", size)
		}
		need := cur.H + lyricsLineGap + next.H + 2*lyricsPanelPadY
		if need > float32(h)+0.01 {
			t.Errorf("%vpx：内容高 %v 超过窗口高 %d（会裁字）", size, need, h)
		}
		if float32(h)-need > 1.01 {
			t.Errorf("%vpx：窗口高 %d 比内容 %v 多出太多空白", size, h, need)
		}
	}
}
