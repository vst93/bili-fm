package view

import (
	"math"
	"testing"

	ui "github.com/egoist/mygo/ui"
)

// 本轮（迷你窗对照 main 原版的对齐轮）的测试。
//
// 对照 main:src/styles/globals.css 的 body.mini-mode 段与 miniVideoInfo.tsx，
// 修 5 处差异：
//
//	① .mini-cover-shine       渐变第二 stop 停在 42%（End 缺省 1 会拉到整条对角线）
//	② .mini-status-dot        蓝点外圈光晕是 sky-500 #0ea5e9（≠ 点色 --studio-blue）
//	③ #player .player-play-button 悬停态（播放中一套 / 暂停一套）此前整支漏掉
//	④ .mini-title             line-height 1.25 的多行行距（框架顶到自然行盒 23）
//	⑤ .mini-window-controls   role="group" aria-label="窗口控制"

// miniTitleProbe 建一个 400×155 的迷你窗，标题为 title（用于量标题行距 / 盒高）。
func miniTitleProbe(t *testing.T, title string) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: title, Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	return tst
}

// titleInkRunsMini 在迷你窗里量标题盒内「有墨迹的行」（灰度 < 140）的起止 y。
// x 限定在标题盒内，避开左边的封面与右边的窗口控制列。
func titleInkRunsMini(t *testing.T, title string) ([][2]int, ui.Rect) {
	t.Helper()
	tt := miniTitleProbe(t, title)
	tr, ok := tt.Find(title)
	if !ok {
		t.Fatalf("找不到迷你窗标题 %q", title)
	}
	img := tt.Image()
	x0, x1 := int(tr.X), int(tr.X+tr.W)
	var runs [][2]int
	start := -1
	// 从盒顶上方 6px 扫到盒底下方 4px：自绘的行盒带负 half-leading，首行墨迹
	// 会略微高出盒顶。
	for y := int(tr.Y) - 6; y <= int(tr.Y+tr.H)+4; y++ {
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
	return runs, tr
}

// TestMiniTitleLinePitchMatchesOriginal 是像素实测：两行标题的墨迹行起点差应等于
// 原版行距 `1.25 × 15 = 18.75px`，而不是框架自然行盒的 23px。
//
// 反证（同测试内）：把 miniTitlePitch 换成自然行盒（23，即框架原来的行为），
// 第二行墨迹会整体下移 —— 起点差随之变大。
func TestMiniTitleLinePitchMatchesOriginal(t *testing.T) {
	title := "这是一个相当长的视频标题用来测试两行截断的效果另一个很长的标题"

	measure := func() float32 {
		runs, tr := titleInkRunsMini(t, title)
		if len(runs) < 2 {
			t.Fatalf("只看得到 %d 段墨迹（%v），拿不到两行标题", len(runs), runs)
		}
		if runs[0][0] < int(tr.Y)-6 || runs[0][0] > int(tr.Y)+8 {
			t.Fatalf("第一行墨迹顶 %d 不在标题盒顶 %v 附近", runs[0][0], tr.Y)
		}
		return float32(runs[1][0] - runs[0][0])
	}

	got := measure()
	if math.Abs(float64(got-miniTitlePitch)) > 1 {
		t.Errorf("两行迷你标题行距 %v，want %v（原版 line-height 1.25 × 15）", got, miniTitlePitch)
	}
	if got >= 21 {
		t.Errorf("行距 %v 像框架的自然行盒（23），不是原版 1.25（18.75）", got)
	}

	// 反证：换回自然行盒（框架原行为）应把第二行推低。
	old := miniTitlePitch
	miniTitlePitch = naturalLineBox(miniTitleSize)
	defer func() { miniTitlePitch = old }()
	if back := measure(); math.Abs(float64(back-23)) > 1.5 {
		t.Errorf("换回自然行盒后行距 %v，want ≈23（证明行距确实由 miniTitlePitch 驱动）", back)
	}
}

// TestMiniTitleSingleLineBoxHeight 钉住单行迷你标题的盒高：原版 .mini-title 的
// line-height 1.25 ⇒ 单行盒高 18.75（框架自然行盒是 23）。盒高决定信息列
// （Justify Center）的居中位置，多出 4.25 会把标题/分集行整体推偏。
func TestMiniTitleSingleLineBoxHeight(t *testing.T) {
	tt := miniTitleProbe(t, "短标题")
	tr, ok := tt.Find("短标题")
	if !ok {
		t.Fatal("找不到迷你窗标题")
	}
	if math.Abs(float64(tr.H-miniTitlePitch)) > 0.5 {
		t.Errorf("单行标题盒高 %v，want %v（原版 line-height 1.25 × 15）", tr.H, miniTitlePitch)
	}
	// 宽度应是标题列宽（信息行减去封面/间距/窗口控制列），证明确实按常数预折。
	if math.Abs(float64(tr.W-miniTitleWidth)) > 1 {
		t.Errorf("标题盒宽 %v，want %v（标题列可用宽）", tr.W, miniTitleWidth)
	}
}

// TestMiniCoverShineStopsAt42 钉住封面高光层（原版 .mini-cover-shine）的渐变：
// 第二 stop 停在 42%，不是整条对角线。整块封面铺纯黑，量渐变线 70% 处：
// End=0.42 时那里已全透明（黑），End 缺省 1 会留一层白（更亮）。
func TestMiniCoverShineStopsAt42(t *testing.T) {
	g := miniCoverShine()
	if g.Angle != 135 {
		t.Errorf("高光角度 = %v，want 135（原版 135deg）", g.Angle)
	}
	if math.Abs(float64(g.End-0.42)) > 1e-6 {
		t.Errorf("高光渐变 End = %v，want 0.42（原版 transparent 42%%）", g.End)
	}
	if g.From != ui.Hex("#ffffff").Alpha(0.28) {
		t.Errorf("高光起色 = %v，want 白 .28", g.From)
	}

	// 无头实测：封面铺 64×64 纯黑（覆盖 box 的背景色），高光叠上去后，
	// 渐变线 70% 处（盒坐标 ≈ (44.8,44.8) → 窗口 (57,81)）应回到纯黑。
	sample := func() int {
		black := solidRGBA(64, 64, 0, 0, 0, 255)
		a := NewApp(func() {})
		a.Mini = true
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP", Cover: "cover"}
		a.Images = &miniCoverCache{bmp: black}
		tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tt.Frame()
		img := tt.Image()
		// 3×3 平均，抗单像素噪点。
		sum := 0
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				sum += grayAt(img, 57+dx, 81+dy)
			}
		}
		return sum / 9
	}

	got := sample()
	if got > 8 {
		t.Errorf("高光 42%% 之外（渐变线 70%% 处）仍偏亮：灰 %d，应为纯黑（End 没停在 42%%？）", got)
	}

	// 反证：把 End 换回缺省的 1，同一点应重新变亮。
	old := miniCoverShine
	miniCoverShine = func() ui.LinearGradient {
		return ui.LinearGradient{From: ui.Hex("#ffffff").Alpha(0.28), To: ui.Transparent, Angle: 135}
	}
	defer func() { miniCoverShine = old }()
	if back := sample(); back <= got+6 {
		t.Errorf("End 换回 1 后同一点灰 %d，应与 %d 拉开（证明该点亮度由 End 驱动）", back, got)
	}
}

// TestMiniStatusDotColors 钉住迷你窗分集行状态点的（点色, 光晕色）：原版两态的
// 光晕**不同源** —— 蓝点 ring 是 sky-500 #0ea5e9，不是点色 --studio-blue。
func TestMiniStatusDotColors(t *testing.T) {
	lt := ThemeFor(PeriodAt(12))

	dot, ring := miniStatusDot(lt, false)
	if dot != lt.Blue {
		t.Errorf("非播放列表点色 = %v，want --studio-blue %v", dot, lt.Blue)
	}
	if ring != ui.Hex("#0ea5e9") {
		t.Errorf("非播放列表光晕 = %v，want sky-500 #0ea5e9（≠ 点色）", ring)
	}
	if ring == dot {
		t.Error("蓝点光晕不应等于点色（原版是 sky-500，不是 --studio-blue）")
	}

	dot, ring = miniStatusDot(lt, true)
	if dot != lt.Rose || ring != lt.Rose {
		t.Errorf("播放列表点/光晕 = %v / %v，want 同为 --studio-rose %v", dot, ring, lt.Rose)
	}
}

// TestMiniWindowControlsGroupLabel 钉住窗口控制列的 role/aria-label：原版
// .mini-window-controls 是 `role="group" aria-label="窗口控制"`。
func TestMiniWindowControlsGroupLabel(t *testing.T) {
	tt := miniProbe(t, false)
	r, ok := tt.Find("窗口控制")
	if !ok {
		t.Fatal("找不到带 aria-label「窗口控制」的窗口控制列（group 标签没接上）")
	}
	if math.Abs(float64(r.W-miniControlsW)) > 1 {
		t.Errorf("窗口控制列宽 %v，want %v", r.W, miniControlsW)
	}
	// 列应贴右：窗口 400，右边距 = 信息行内边距 12，列宽 30。
	if want := float32(miniWidth - miniInfoPadH - miniControlsW); math.Abs(float64(r.X-want)) > 1 {
		t.Errorf("窗口控制列 X = %v，want %v", r.X, want)
	}
}
