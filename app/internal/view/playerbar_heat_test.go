package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// heatProbe 渲染主窗（1000×700）并返回图像；heat 控制热力开关，
// chapters 控制章节分界。播放栏贴底，顶边在 700-PlayerBarHeight。
func heatProbe(t *testing.T, heat, chapters bool) *image.RGBA {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	a.HeatOn = heat
	a.DanmakuHeat = []HeatPoint{
		{At: 0, Value: 1}, {At: 50, Value: 1}, {At: 100, Value: 1},
		{At: 150, Value: 1}, {At: 200, Value: 1},
	}
	if chapters {
		a.Chapters = []Chapter{{From: 40}, {From: 120}}
	}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	return tst.Image()
}

// diffBounds 返回两张图像不同像素的包围盒与数量。
func diffBounds(a, b *image.RGBA) (minX, minY, maxX, maxY, n int) {
	minX, minY, maxX, maxY = 1<<30, 1<<30, -1, -1
	for y := 0; y < a.Rect.Dy(); y++ {
		for x := 0; x < a.Rect.Dx(); x++ {
			i := y*a.Stride + x*4
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				n++
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	return
}

// TestHeatBackdropStaysBelowTopInset 钉住热力面积的最高点：从播放栏
// 底边升起，但顶部至少留出 heatBarTopInset，不再顶到栏的描边/时间胶囊。
func TestHeatBackdropStaysBelowTopInset(t *testing.T) {
	off := heatProbe(t, false, false)
	on := heatProbe(t, true, false)
	_, minY, _, maxY, n := diffBounds(off, on)
	if n == 0 {
		t.Fatal("打开热力后画面没有变化（热力层没接上）")
	}
	barTop := 700 - PlayerBarHeight
	if minY < barTop+int(heatBarTopInset)-1 {
		t.Errorf("热力最高点 y=%d，越过了顶部留白：barTop=%d inset=%v", minY, barTop, heatBarTopInset)
	}
	if maxY > barTop+PlayerBarHeight-1 {
		t.Errorf("热力最低点 y=%d 超出播放栏底边 %d", maxY, barTop+PlayerBarHeight-1)
	}
	if maxY < barTop+PlayerBarHeight-4 {
		t.Errorf("热力没有贴到播放栏底边：maxY=%d", maxY)
	}
}

// TestChapterMarksAreTicks 钉住章节分界是 2px 短刻线，不是画满轨道的
// 竖条 —— 后者会和弹幕热力混成“柱状图”。
func TestChapterMarksAreTicks(t *testing.T) {
	off := heatProbe(t, false, false)
	on := heatProbe(t, false, true)
	_, minY, _, maxY, n := diffBounds(off, on)
	if n == 0 {
		t.Fatal("章节分界没有画出来")
	}
	if h := maxY - minY + 1; h > 3 {
		t.Errorf("章节刻线高 %dpx，want ≤3px（画满 6px 轨道会被误读成柱状图）", h)
	}
}
