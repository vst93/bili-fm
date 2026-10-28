package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestIconComponentSizes 钉住「图标按钮的尺寸 / 圆角」与原版 globals.css 对齐。
//
// 用户反馈「图标和组件的尺寸…和原版有差异」。原版三组图标钮的尺寸是分开的：
//
//	.top-tool-btn / .search-submit-btn      32×32 圆角 6（搜索栏内腔同心链）
//	.video-content-actions .nav-icon-btn    34×34 圆角 8
//	.video-context-dock .nav-icon-btn       34×34 圆角 7
//
// 之前内容操作行和工具条都按 32×32/圆角 6 画，比原版小一圈。
func TestIconComponentSizes(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	// 已收藏，让内容操作行的收藏键文案变成「取消收藏」，与搜索栏的「收藏」
	// 入口区分开（两者都叫「收藏」时 Find 只能拿到第一个）。
	a.Faved = true
	// 内容操作行只在「有曲目」时存在：真空态（Info 与 Track 都为空）现在
	// 不画这一排占位按钮（见 videoInfo / emptyInfo）。
	a.Track = &Track{Bvid: "BV1", Title: "标题"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)

	// 内容操作行（.video-content-actions）：34×34。
	for _, label := range []string{"浏览器打开", "视频播放", "弹幕、评论、章节与字幕", "点赞", "投币(2个)", "取消收藏"} {
		r, ok := tst.Find(label)
		if !ok {
			t.Fatalf("找不到内容操作按钮 %q", label)
		}
		if r.W != ContentIcon || r.H != ContentIcon {
			t.Errorf("内容操作按钮 %q = %.0f×%.0f, want %d×%d",
				label, r.W, r.H, ContentIcon, ContentIcon)
		}
	}

	// 工具条（.video-context-dock）：34×34。
	for _, label := range []string{"搜索结果", "选集", "合集", "播放列表"} {
		r, ok := tst.Find(label)
		if !ok {
			t.Fatalf("找不到工具条按钮 %q", label)
		}
		if r.W != DockIcon || r.H != DockIcon {
			t.Errorf("工具条按钮 %q = %.0f×%.0f, want %d×%d",
				label, r.W, r.H, DockIcon, DockIcon)
		}
	}

	// 搜索栏入口（.top-tool-btn）：32×32，与内容操作行区分开。
	if r, ok := tst.Find("热门与推荐"); !ok {
		t.Error("找不到搜索栏入口「热门与推荐」")
	} else if r.W != ToolButton || r.H != ToolButton {
		t.Errorf("搜索栏入口 = %.0f×%.0f, want %d×%d", r.W, r.H, ToolButton, ToolButton)
	}
}

// TestPlayerBarButtonSizes 单独渲染播放栏（避免与主区的「播放/暂停」圆盘重名），
// 钉住播放键 42×42、其余工具键 32×32（原版 #player 的 grid 列）。
func TestPlayerBarButtonSizes(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Cid: 1, Title: "t"}
	tst := ui.NewTester(func(c *ui.Context) { a.playerBar(c) }, 800, 200)

	cases := []struct {
		label string
		want  float32
	}{
		{"播放", 42},
		{"下一个视频", 32},
		{"音量", 32},
	}
	for _, c := range cases {
		r, ok := tst.Find(c.label)
		if !ok {
			t.Fatalf("播放栏找不到 %q", c.label)
		}
		if r.W != c.want || r.H != c.want {
			t.Errorf("播放栏 %q = %.0f×%.0f, want %.0f×%.0f",
				c.label, r.W, r.H, c.want, c.want)
		}
	}
}

// TestIconSizeConstants 钉住三组图标钮的常量本身（改动要显式）。
func TestIconSizeConstants(t *testing.T) {
	if ToolButton != 32 || ToolButtonR != 6 {
		t.Errorf("搜索栏按钮常量 = %v/%v, want 32/6", ToolButton, ToolButtonR)
	}
	if ContentIcon != 34 || ContentIconR != 8 {
		t.Errorf("内容操作按钮常量 = %v/%v, want 34/8", ContentIcon, ContentIconR)
	}
	if DockIcon != 34 || DockIconR != 7 {
		t.Errorf("工具条按钮常量 = %v/%v, want 34/7", DockIcon, DockIconR)
	}
}
