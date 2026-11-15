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
	if heat {
		a.HeatVersion++ // 模拟数据到达，触发一次整窗重绘
	}
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

// chapterProbe 只渲染播放栏本身，避免章节 tab / 正文等整窗内容混进
// 章节刻线的包围盒断言。
//
// 底必须是**深色**的：章节刻线是白 .55，且按层序画在半透明轨道**下面**
// （见 paintTimeline 的层序），铺在白底上时白刻线叠在白色的轨道底色里
// 逐像素差为零 —— 测试会报「章节分界没有画出来」（CI 与开发机都这么红过）。
func chapterProbe(t *testing.T, chapters bool) *image.RGBA {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	if chapters {
		a.Chapters = []Chapter{{From: 40}, {From: 120}}
	}
	tst := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.Theme.uiTheme())
		c.Root().Background(ui.Hex("#1e293b"))
		a.playerBar(c)
	}, 1000, PlayerBarHeight)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	return tst.Image()
}

// diffBounds 返回两张图像不同像素的包围盒与数量。
func diffBounds(a, b *image.RGBA) (minX, minY, maxX, maxY, n int) {
	return diffBoundsIn(a, b, 0, a.Rect.Dy())
}

// diffBoundsIn 是 diffBounds 的「只看 [y0, y1) 这几行」版。
//
// 两张图是**两次独立渲染**：主区里随时间/动画变化的像素（封面圆盘、谈话
// 光晕等）会把包围盒拉出被测的那一层，在 CI 那种慢机器上尤其容易。只量
// 被测层所在的行段，噪声就进不来。
func diffBoundsIn(a, b *image.RGBA, y0, y1 int) (minX, minY, maxX, maxY, n int) {
	minX, minY, maxX, maxY = 1<<30, 1<<30, -1, -1
	if y0 < 0 {
		y0 = 0
	}
	if y1 > a.Rect.Dy() {
		y1 = a.Rect.Dy()
	}
	for y := y0; y < y1; y++ {
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
	barTop := 700 - PlayerBarHeight
	// 只量播放栏那几行：主区里随时间变化的像素与热力层无关（两张图是两次
	// 独立渲染，CI 上曾被这类噪声把包围盒拉到 y=198 而假红）。
	_, minY, _, maxY, n := diffBoundsIn(off, on, barTop, 700)
	if n == 0 {
		t.Fatal("打开热力后画面没有变化（热力层没接上）")
	}
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
	off := chapterProbe(t, false)
	on := chapterProbe(t, true)
	_, minY, _, maxY, n := diffBounds(off, on)
	if n == 0 {
		t.Fatal("章节分界没有画出来")
	}
	if h := maxY - minY + 1; h > 3 {
		t.Errorf("章节刻线高 %dpx，want ≤3px（画满 6px 轨道会被误读成柱状图）", h)
	}
}

// heatGeomProbe 渲染主窗并返回 App、渲染用的 Context 与 Tester。窗口宽度
// 可变，用来验证热力与进度条的几何关系在宽窗 / 小窗下都成立。
func heatGeomProbe(t *testing.T, w, h int) (*App, *ui.Context, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	a.HeatOn = true
	a.HeatVersion++ // 模拟热力数据到达（触发一次整窗重绘）
	a.DanmakuHeat = []HeatPoint{
		{At: 0, Value: 0.4}, {At: 50, Value: 1}, {At: 100, Value: 0.6},
		{At: 150, Value: 0.9}, {At: 200, Value: 0.3},
	}
	var ctx *ui.Context
	tst := ui.NewTester(func(c *ui.Context) {
		ctx = c
		a.Shell(c)
	}, w, h)
	// 两帧：进度条句柄在构建时绑定，Bounds 读的是已提交的那一帧。
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	tst.Frame()
	return a, ctx, tst
}

// TestHeatWidthMatchesProgressTrack 钉住「热力宽度 == 进度条轨道宽度」：
// 同一行、同一段，左边界与右边界都对齐（允许 1px 亚像素误差）。
//
// 断言用布局几何而不是像素猜：热力画出的矩形（App.heatTrack，绘制回调里
// 记下的实际入参）对比进度条句柄量出的轨道（rangeTrackRect），再反证它
// 没有铺满整条播放栏。回归背景：458c170 把进度条搬进 playerbar_shared.go
// 时漏了 .Bind(&a.progressHandle)，热力取不到轨道就退回「整条播放栏」宽，
// 用户看到的是热力被拉满整个窗口。
func TestHeatWidthMatchesProgressTrack(t *testing.T) {
	// 1000 / 800 是常用宽度，480 是主窗窄档，320 是**最小宽度**：主窗播放栏
	// 的固定列在 320 已经把 1fr 的进度条挤成 14px 盒（轨道 0），热力此时
	// 不该退回整条栏宽 —— 这一档只断言「不比轨道宽」，其余三档断言完整对齐。
	for _, w := range []int{1000, 800, 480, 320} {
		a, ctx, _ := heatGeomProbe(t, w, 700)
		bar := a.progressHandle.Bounds(ctx)
		if bar.W <= 0 {
			t.Fatalf("窗口 %d 宽：进度条句柄没有绑上（Bounds 为空）—— 热力只能退回整条栏宽", w)
		}
		track := rangeTrackRect(bar, timelineGeom)
		got := a.heatTrack
		if got.W > track.W+1 {
			t.Errorf("窗口 %d 宽：热力宽 %.1f 超过进度条轨道宽 %.1f（回归：被拉成整条播放栏）",
				w, got.W, track.W)
		}
		// 反证：轨道比滑块盒窄（左右各内缩 timelineInset），所以「跟着轨道」
		// 与「跟着整条播放栏」在几何上不可能同时成立。
		if track.W >= bar.W {
			t.Fatalf("窗口 %d 宽：轨道宽 %.1f 不小于滑块盒宽 %.1f，判据失效", w, track.W, bar.W)
		}
		if track.W < 8 {
			continue // 窄到进度条没有轨道：只要求上面那条「不比轨道宽」
		}
		if got.W <= 0 {
			t.Fatalf("窗口 %d 宽：热力没有画出来（heatTrack 为空，轨道 %v）", w, track)
		}
		if d := absF(got.W - track.W); d > 1 {
			t.Errorf("窗口 %d 宽：热力宽 %.1f，进度条轨道宽 %.1f（差 %.1f > 1）", w, got.W, track.W, d)
		}
		if d := absF(got.X - track.X); d > 1 {
			t.Errorf("窗口 %d 宽：热力左边界 %.1f，轨道左边界 %.1f（差 %.1f > 1）", w, got.X, track.X, d)
		}
		if d := absF((got.X + got.W) - (track.X + track.W)); d > 1 {
			t.Errorf("窗口 %d 宽：热力右边界 %.1f，轨道右边界 %.1f（差 %.1f > 1）",
				w, got.X+got.W, track.X+track.W, d)
		}
		t.Logf("窗口 %d 宽：热力 %v，轨道 %v，滑块盒 %v", w, got, track, bar)
	}
}
