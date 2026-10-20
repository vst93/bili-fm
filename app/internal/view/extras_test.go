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
	a.Pos = 1.2
	tt.Frame()
	if err := tt.Click("第二句"); err != nil {
		t.Fatalf("字幕行不可点击：%v", err)
	}
	if len(seeks) != 2 || seeks[1] != 1 {
		t.Fatalf("字幕跳转 = %v，want 第二次为 1", seeks)
	}
}

func TestLyricsShellRenders(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LyricCurrent = "第一句"
	a.LyricNext = "第二句"
	lt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, 92)
	lt.Frame()
}
