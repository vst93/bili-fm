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
	a.Act.ToggleLyricsConfig = func() { toggled++ }
	lt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, 100)
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
	cfg := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, 500)
	cfg.Frame()
	if _, ok := cfg.Find("记住微调"); !ok {
		t.Fatal("设置页看不到「记住微调」")
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
	above := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, 500)
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
