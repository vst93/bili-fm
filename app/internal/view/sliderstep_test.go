package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（进度条 input step="0.1" 对齐）的测试。
//
// 原版进度条是 `<input type="range" min="0" max={duration} step="0.1">`
//（player.tsx:1324），拖动与方向键都按 0.1 秒吸附。原生版此前没接 `Step`，
// 框架 `snap` 在 step<=0 时**原样返回指针位置**（没有吸附），于是拖动落点是
// 一串任意浮点（如 123.39999675750732），而原版是 0.1 的干净倍数。
// `Step(0.1)` 同时改框架的输入步长与无障碍层的 step。

// seekIsTenth 报告 v 是不是 0.1 的干净倍数（框架 snap 到 0.1 后会把小数位
// 裁到一位；未吸附的原始指针值几乎总有更多位）。
func seekIsTenth(v float64) bool {
	return math.Abs(v*10-math.Round(v*10)) < 1e-6
}

// TestProgressSliderStep 用真实点击钉住主窗进度条的吸附：Dur=200 的曲目，
// 在多个不同位置按下，落点都必须是 0.1 的倍数。未接 Step 时这些点是任意浮点。
func TestProgressSliderStep(t *testing.T) {
	a, tst := playerBarProbe(t, nil)
	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	tr := rangeTrackRect(r, timelineGeom)

	for _, frac := range []float32{0.13, 0.27, 0.502, 0.617, 0.731, 0.183, 0.409, 0.856} {
		a.SeekValue = 100
		tst.ClickAt(tr.X+tr.W*frac, tr.Y+tr.H/2)
		tst.Frame()
		if !seekIsTenth(a.SeekValue) {
			t.Errorf("点 %.3f 处 SeekValue = %v，不是 0.1 的倍数（step=0.1 未接？）",
				frac, a.SeekValue)
		}
	}
}

// TestMiniProgressSliderStep 同一条规则在迷你窗的进度条上：两处共用同一份
// 几何与步长，落点同样必须吸附到 0.1。
func TestMiniProgressSliderStep(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur, a.Pos, a.SeekValue = 200, 100, 100
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()

	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("迷你窗里找不到进度条")
	}
	tr := rangeTrackRect(r, timelineGeom)
	for _, frac := range []float32{0.13, 0.502, 0.617, 0.856} {
		a.SeekValue = 100
		tst.ClickAt(tr.X+tr.W*frac, tr.Y+tr.H/2)
		tst.Frame()
		if !seekIsTenth(a.SeekValue) {
			t.Errorf("迷你窗点 %.3f 处 SeekValue = %v，不是 0.1 的倍数", frac, a.SeekValue)
		}
	}
}
