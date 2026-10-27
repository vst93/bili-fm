package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestSubtitleTabAlwaysShown 钉住字幕 tab 的稳定性：当前视频没有字幕时
// tab 仍然在表头里，点进去正文提示「当前视频没有字幕」，而不是 tab 消失。
func TestSubtitleTabAlwaysShown(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabDanmaku
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()

	if _, ok := tt.Find("字幕"); !ok {
		t.Fatal("没有字幕时「字幕」tab 也不应消失")
	}
	if err := tt.Click("字幕"); err != nil {
		t.Fatalf("点字幕 tab 失败：%v", err)
	}
	tt.Frame()
	if !tt.HasText("当前视频没有字幕") {
		t.Fatal("字幕 tab 的空态应提示「当前视频没有字幕」")
	}
}

// TestPlaylistSeriesTabAlwaysShown 钉住播放列表「合集列表」tab 的稳定性：
// 合集列表为空时 tab 仍在，点进去正文提示合集列表为空。
func TestPlaylistSeriesTabAlwaysShown(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()

	if _, ok := tt.Find("合集列表(0)"); !ok {
		t.Fatal("合集列表为空时 tab 也不应消失")
	}
	if err := tt.Click("合集列表(0)"); err != nil {
		t.Fatalf("点合集列表 tab 失败：%v", err)
	}
	tt.Frame()
	if !tt.HasText("合集列表为空（在合集里点「播放全部」加载）") {
		t.Fatal("合集列表 tab 的空态没有显示")
	}
}
