package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestHomeIdleEmptyState 钉住主区空态：没有详情也没有当前曲目时，右栏只给
// 一句标题 + 一句可执行的引导，不再把占位当内容画（「神秘的UP主」「无描述」
// 「当前选集 无选集标题」以及一排全禁用的计数键）。
//
// 工具条（搜索结果 / 选集 / 合集 / 播放列表）必须保留：播放列表往往已经有歌，
// 它是空态里唯一真正能点的入口。
func TestHomeIdleEmptyState(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Playlist = []PlayItem{
		{ID: "1", Bvid: "BV1", Title: "第一首"},
		{ID: "2", Bvid: "BV2", Title: "第二首"},
	}
	if !a.idle() {
		t.Fatal("NewApp 默认应为空态")
	}
	tt := shellTesterSize(t, a, 800, 600)
	for _, want := range []string{
		"还没有正在听的内容",
		// 播放列表里有歌时，引导要把「从播放列表继续」也带上。
		"搜索关键词、粘贴 B 站链接，或从下方播放列表继续。",
		"播放列表", // 工具条保留
	} {
		if !tt.HasText(want) {
			t.Errorf("空态缺少 %q", want)
		}
	}
	for _, gone := range []string{
		"神秘的UP主", "无描述", "当前选集", "无选集标题", "点赞",
	} {
		if tt.HasText(gone) {
			t.Errorf("空态不该再出现占位内容 %q", gone)
		}
	}
}

// TestHomeIdleHintWithoutPlaylist 是上一条的另一半：播放列表为空时不给
// 「从下方播放列表继续」这条走不通的建议。
func TestHomeIdleHintWithoutPlaylist(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTesterSize(t, a, 800, 600)
	if !tt.HasText("搜索关键词，或把 B 站视频链接粘贴到上面的搜索框。") {
		t.Error("没有播放列表时缺搜索/粘贴引导")
	}
	if tt.HasText("或从下方播放列表继续") {
		t.Error("播放列表为空时不该建议从播放列表继续")
	}
}

// TestIdleOnlyWhenNothingToPlay 钉住「空态」与「有曲目、详情还在路上」的分界：
// 只要有一条 Track，就照常画 UP 主行 / 简介 / 操作行（哪怕详情还没回来）。
func TestIdleOnlyWhenNothingToPlay(t *testing.T) {
	a := NewApp(func() {})
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	if a.idle() {
		t.Fatal("有 Track 时不该算空态")
	}
	a.Track = nil
	a.Info = &Info{Bvid: "BV1", Title: "标题"}
	if a.idle() {
		t.Fatal("有 Info 时不该算空态")
	}
	a.Info = nil
	if !a.idle() {
		t.Fatal("两者都为空时应算空态")
	}
}

// TestPartPillHiddenWithoutPartTitle 钉住「没有分集标题就不画胶囊」：
// 单 P 视频的胶囊此前固定写「当前选集 无选集标题」，等于占一行告诉用户
// 「这里本该有东西」。从播放列表播入时仍保留（来源本身是有效信息）。
func TestPartPillHiddenWithoutPartTitle(t *testing.T) {
	app := func() *App {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Info = &Info{Bvid: "BV1", Title: "标题"}
		a.Track = &Track{Bvid: "BV1", Title: "标题"} // 没有 Part
		a.Queue = []Track{*a.Track}
		return a
	}
	tt := shellTesterSize(t, app(), 800, 600)
	if tt.HasText("当前选集") {
		t.Error("没有分集标题时不该画「当前选集」胶囊")
	}
	inList := app()
	inList.PlayingPlaylist = ListUser
	inList.Playlist = []PlayItem{{ID: "1", Bvid: "BV1", Title: "标题"}}
	tt2 := shellTesterSize(t, inList, 800, 600)
	if !tt2.HasText("播放列表") {
		t.Error("从播放列表播入时应保留胶囊（来源是有效信息）")
	}
	if tt2.HasText("当前选集") {
		t.Error("播放列表模式的前缀应为「播放列表」，不是「当前选集」")
	}
}

// TestMiniIdleStatus 钉住迷你窗空态的第二行：有播放列表就报条数，没有就给
// 一句回主窗的动作提示；不再显示「无选集标题」。
func TestMiniIdleStatus(t *testing.T) {
	render := func(items int) *ui.Tester {
		a := NewApp(func() {})
		a.Mini = true
		a.Theme = ThemeFor(PeriodAt(12))
		for i := 0; i < items; i++ {
			a.Playlist = append(a.Playlist, PlayItem{ID: string(rune('a' + i)), Bvid: "BV1", Title: "t"})
		}
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tst.Frame()
		return tst
	}
	withList := render(8)
	if !withList.HasText("还没有正在听的内容") {
		t.Error("迷你窗空态缺标题")
	}
	if !withList.HasText("播放列表 8 首") {
		t.Error("迷你窗空态第二行应为播放列表条数")
	}
	if withList.HasText("无选集标题") {
		t.Error("迷你窗不该再显示「无选集标题」")
	}
	empty := render(0)
	if !empty.HasText("回到主窗开始搜索") {
		t.Error("播放列表为空时应提示回主窗搜索")
	}
}

// TestSearchDrawerTitle 钉住搜索抽屉表头的关键词：抽屉会盖住主窗的搜索药丸，
// 标题里不带关键词就不知道自己在看哪次搜索。长词按 14 个字符截断。
func TestSearchDrawerTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "搜索"},
		{"   ", "搜索"},
		{"许嵩", "搜索「许嵩」"},
		{" 许嵩 ", "搜索「许嵩」"},
		{"一二三四五六七八九十一二三四五六七八九十", "搜索「一二三四五六七八九十…」"},
	}
	for _, tc := range cases {
		if got := searchDrawerTitle(tc.in); got != tc.want {
			t.Errorf("searchDrawerTitle(%q) = %q，want %q", tc.in, got, tc.want)
		}
	}
	if got := searchDrawerTitle(strings.Repeat("x", 40)); !strings.HasSuffix(got, "」") ||
		!strings.Contains(got, "…") || len([]rune(got)) != len([]rune("搜索「"))+10+len([]rune("…」")) {
		t.Errorf("长关键词截断不符合预期：%q", got)
	}
}
