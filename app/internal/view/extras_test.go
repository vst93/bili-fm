package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestChapterAt(t *testing.T) {
	chs := []Chapter{{From: 0, To: 10, Title: "a"}, {From: 10, To: 20, Title: "b"}}
	if got := chapterAt(chs, -1); got != -1 {
		t.Fatalf("before = %d", got)
	}
	if got := chapterAt(chs, 0); got != 0 {
		t.Fatalf("start = %d", got)
	}
	if got := chapterAt(chs, 10); got != 1 {
		t.Fatalf("boundary = %d", got)
	}
	if got := chapterAt(chs, 20); got != -1 {
		t.Fatalf("after = %d", got)
	}
}

func TestSubtitleIndexAt(t *testing.T) {
	cues := []SubtitleCue{{From: 0, To: 1, Content: "a"}, {From: 2, To: 3, Content: "b"}, {From: 4, To: 5, Content: "c"}}
	cases := []struct {
		pos  float64
		want int
	}{
		{-1, -1},
		{0, 0},
		{1.5, 0}, // 句间空隙保留上一句
		{2, 1},
		{4.5, 2},
		{99, 2},
	}
	for _, c := range cases {
		if got := subtitleIndexAt(cues, c.pos); got != c.want {
			t.Errorf("pos=%v: got %d, want %d", c.pos, got, c.want)
		}
	}
}

// TestChaptersAndSubtitlesRender 钉住章节 / 字幕页的渲染与点击跳转事件：
// 两个新 tab 都复用现有抽屉，不能因为空数据或 List 状态缺失而 panic。
func TestChaptersAndSubtitlesRender(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerDanmaku
	a.Chapters = []Chapter{{From: 0, To: 10, Title: "开场"}, {From: 10, To: 20, Title: "正题"}}
	a.DanmakuTab = TabChapters
	var seeks []float64
	a.Act.Seek = func(v float64) { seeks = append(seeks, v) }
	tt := shellTester(t, a)
	tt.Frame()
	if err := tt.Click("开场"); err != nil {
		t.Fatalf("章节行不可点击：%v", err)
	}
	if len(seeks) != 1 || seeks[0] != 0 {
		t.Fatalf("章节跳转 = %v，want [0]", seeks)
	}

	a.DanmakuTab = TabSubtitle
	a.Subtitles = []SubtitleCue{{From: 0, To: 1, Content: "第一句"}, {From: 1, To: 2, Content: "第二句"}}
	a.SubtitleTracks = []SubtitleTrack{{Lan: "zh-CN", Label: "中文"}, {Lan: "en-US", Label: "English"}}
	a.SubtitleLang = "zh-CN"
	selected := -1
	a.Act.SelectSubtitle = func(i int) { selected = i }
	a.Pos = 1.2
	tt.Frame()
	if err := tt.Click("第二句"); err != nil {
		t.Fatalf("字幕行不可点击：%v", err)
	}
	if len(seeks) != 2 || seeks[1] != 1 {
		t.Fatalf("字幕跳转 = %v，want 第二次为 1", seeks)
	}
	if err := tt.Click("English"); err != nil {
		t.Fatalf("字幕版本不可选：%v", err)
	}
	if selected != 1 {
		t.Fatalf("字幕版本选择 index=%d, want 1", selected)
	}
}

func TestLyricsShellRenders(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LyricCurrent = "第一句"
	a.LyricNext = "第二句"
	toggled := 0
	reset := 0
	a.Act.ToggleLyricsConfig = func() { toggled++ }
	a.Act.ResetLyricsConfig = func() { reset++ }
	lt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, LyricsDisplayHeight(a.LyricsFontSize))
	lt.Frame()
	lt.Move(280, 50)
	lt.Frame()
	if err := lt.Click("桌面字幕设置"); err != nil {
		t.Fatalf("桌面字幕设置按钮不可点：%v", err)
	}
	if toggled != 1 {
		t.Fatalf("设置按钮 toggled=%d, want 1", toggled)
	}

	a.LyricsConfig = true
	cfgH := LyricsDisplayHeight(a.LyricsFontSize) + LyricsConfigPanelHeight
	cfg := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, cfgH)
	cfg.Frame()
	if _, ok := cfg.Find("记住微调"); !ok {
		t.Fatal("设置页看不到「记住微调」")
	}
	// 鼠标穿透开关也在这个页面里（用户 2026-11-11：穿透设置在它的设置页面也要有）
	if _, ok := cfg.Find("鼠标穿透"); !ok {
		t.Fatal("设置页看不到「鼠标穿透」开关")
	}
	if err := cfg.Click("恢复默认"); err != nil {
		t.Fatalf("设置页恢复默认按钮不可点：%v", err)
	}
	if reset != 1 {
		t.Fatalf("恢复默认按钮调用 %d 次，应为 1", reset)
	}
	if err := cfg.Click("完成"); err != nil {
		t.Fatalf("设置页完成按钮不可点：%v", err)
	}
	if toggled != 2 {
		t.Fatalf("完成按钮 toggled=%d, want 2", toggled)
	}

	// 贴近屏幕底部时控制器把设置页放到预览上方；视图必须按标志换序。
	a.LyricsConfig = true
	a.LyricsConfigAbove = true
	above := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, cfgH)
	above.Frame()
	title, ok := above.Find("桌面字幕设置")
	if !ok {
		t.Fatal("上方设置页找不到标题")
	}
	preview, ok := above.Find("第一句")
	if !ok {
		t.Fatal("上方设置页找不到预览字幕")
	}
	if title.Y >= preview.Y {
		t.Fatalf("设置页在预览上方时标题 Y=%.1f，预览 Y=%.1f", title.Y, preview.Y)
	}
}

// TestLyricsDragExceeded 钉住「设置键按住拖动」的起步阈值：轻微抖动不算拖动
// （否则点一下设置键会把窗口挪走），超过阈值才算。设置键既是按钮又是拖动手柄，
// 这个阈值是两者唯一的分界。
func TestLyricsDragExceeded(t *testing.T) {
	cases := []struct {
		dx, dy float32
		want   bool
	}{
		{0, 0, false},
		{1, 1, false},
		{3, 0, false}, // 阈值内：仍是点击
		{0, -3, false},
		{4, 0, true}, // 恰好到阈值
		{0, 5, true},
		{-6, 0, true}, // 方向无关
		{3, 3, true},  // 欧氏距离 4.24 > 4
	}
	for _, c := range cases {
		if got := lyricsDragExceeded(c.dx, c.dy); got != c.want {
			t.Errorf("lyricsDragExceeded(%v,%v) = %v，应为 %v", c.dx, c.dy, got, c.want)
		}
	}
}

// TestLyricsGearHiddenUnlessHot 钉住设置键的显隐：**只在指针压在上面（或指针在
// 窗内）时出现**，不因为「穿透打开」就常显 —— 用户 2026-11-11 报「开启穿透后
// 设置键不再自动隐藏」。穿透时窗口收不到 over，改由 LyricsGearHot 判定。
func TestLyricsGearHiddenUnlessHot(t *testing.T) {
	build := func(clickThrough, hot bool) *ui.Tester {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.LyricCurrent, a.LyricNext = "第一句", "第二句"
		a.LyricsClickThrough = clickThrough
		a.LyricsGearHot = hot
		return ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, LyricsDisplayHeight(a.LyricsFontSize))
	}
	// 穿透打开、指针不在设置键上：键必须**不可见**（这是那条回归的反证）。
	tst := build(true, false)
	tst.Frame()
	if _, ok := tst.Find("桌面字幕设置"); ok {
		t.Error("穿透打开且指针不在设置键上时，设置键不该显示（应自动隐藏）")
	}
	// 指针压在上面：出现。
	tst = build(true, true)
	tst.Frame()
	if _, ok := tst.Find("桌面字幕设置"); !ok {
		t.Error("指针压在设置键热区上时应显示设置键")
	}
	// 未穿透：指针在窗内（测试里默认 hover 视口中心）就显示。
	tst = build(false, false)
	tst.Move(280, 40)
	tst.Frame()
	if _, ok := tst.Find("桌面字幕设置"); !ok {
		t.Error("未穿透时指针在窗内应显示设置键")
	}
}
