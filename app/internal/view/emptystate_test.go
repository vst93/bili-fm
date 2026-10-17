package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestEmptyStyleFor 钉住原版三种空态写法的取值（对照 main:src 与
// globals.css 的 .history-empty-tip / h-40 text-gray-400 / py-8）。
func TestEmptyStyleFor(t *testing.T) {
	tip := emptyStyleFor(emptyTipVariant)
	if tip.MinHeight != 160 || tip.PadY != 24 || tip.PadX != 12 || tip.FontSize != 13 {
		t.Fatalf(".history-empty-tip = %+v，应为 min-h 160 / pad 24 12 / 13px", tip)
	}
	if tip.Ink != ui.Hex("#94a3b8") {
		t.Errorf(".history-empty-tip 颜色 = %v，应为 #94a3b8", tip.Ink)
	}

	center := emptyStyleFor(emptyCenterVariant)
	if center.MinHeight != 160 || center.PadY != 0 || center.PadX != 0 || center.FontSize != 14 {
		t.Fatalf("h-40 空态 = %+v，应为 min-h 160 / 无内边距 / 14px", center)
	}
	// text-gray-400 在抽屉里被 `[data-slot="wrapper"] .text-gray-400` 覆写成
	// #64748b —— 比其它空态深一档，之前原生版用的是 #94a3b8。
	if center.Ink != ui.Hex("#64748b") {
		t.Errorf("热门/推荐空态颜色 = %v，应为覆写后的 #64748b", center.Ink)
	}

	note := emptyStyleFor(emptyNoteVariant)
	if note.MinHeight != 0 || note.PadY != 32 || note.FontSize != 14 {
		t.Fatalf("py-8 空态 = %+v，应为无最小高度 / pad 32 / 14px", note)
	}
	if note.Ink != ui.Hex("#94a3b8") {
		t.Errorf("弹幕/评论空态颜色 = %v，应为 #94a3b8", note.Ink)
	}
}

// TestEmptyVariantFor 钉住抽屉 → 空态写法的映射：只有热门与推荐是 h-40 +
// text-gray-400，弹幕/评论是 py-8，其余都是 .history-empty-tip。
func TestEmptyVariantFor(t *testing.T) {
	cases := []struct {
		drawer string
		want   emptyVariant
	}{
		{"popular", emptyCenterVariant},
		{DrawerDanmaku, emptyNoteVariant},
		{"favorite", emptyTipVariant},
		{"history", emptyTipVariant},
		{DrawerSeries, emptyTipVariant},
		{DrawerUp, emptyTipVariant},
		{DrawerSearch, emptyTipVariant},
		{"feed", emptyTipVariant},
		{DrawerParts, emptyTipVariant},
		{DrawerPlaylist, emptyTipVariant},
	}
	for _, c := range cases {
		if got := emptyVariantFor(c.drawer); got != c.want {
			t.Errorf("emptyVariantFor(%q) = %v，应为 %v", c.drawer, got, c.want)
		}
	}
}

// emptyProbe 打开某个抽屉（列表为空、不在加载中），返回 App 与无头窗口。
func emptyProbe(t *testing.T, setup func(a *App)) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = true
	setup(a)
	return a, settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
}

// emptyOffset 返回空态文案中心相对表头文案中心的竖直偏移（两者都随抽屉
// 面板一起动，所以偏移与入场动画进度无关）。
func emptyOffset(t *testing.T, tt *ui.Tester, header, empty string) float32 {
	t.Helper()
	h, ok := tt.Find(header)
	if !ok {
		t.Fatalf("表头找不到 %q", header)
	}
	e, ok := tt.Find(empty)
	if !ok {
		t.Fatalf("找不到空态文案 %q", empty)
	}
	return (e.Y + e.H/2) - (h.Y + h.H/2)
}

// TestEmptyStateVerticallyCentered 是整窗的无头实测：.history-empty-tip 与
// h-40 两种空态都是「160 高的块里居中」，文案中心落在正文顶 + 80px（表头
// 中心 + 50 + 8 + 80 - 24 = +114）；而弹幕/评论的 py-8 是贴顶的
// （表头中心 + 50 + 32 + 半行 ≈ +67）。
//
// **反证**：把 emptyBlock 改回旧的 `Padding(32, 0)`（贴顶）→ 前两条掉到
// +67 附近、测试红。
func TestEmptyStateVerticallyCentered(t *testing.T) {
	const tipWant float32 = 114 // 160/2 + 正文顶(58) - 表头中心(24)
	const noteWant float32 = 67 // 32 + 半行(~9.5) + 50 - 24

	// 稍后再看：.history-empty-tip。
	_, tt := emptyProbe(t, func(a *App) {
		a.Section = 3 // history
		a.Drawer = "history"
		a.HistTab = HistWatchLater
	})
	tip := emptyOffset(t, tt, "观看历史", "暂无稍后再看视频")
	t.Logf("稍后再看空态偏移 = %.1f（目标 %.0f）", tip, tipWant)
	if tip < tipWant-10 || tip > tipWant+10 {
		t.Errorf("稍后再看空态偏移 = %.1f，应约 %.0f（.history-empty-tip 在 160 高的块里居中）", tip, tipWant)
	}

	// 热门：h-40 居中，位置与上面一致。
	_, tt2 := emptyProbe(t, func(a *App) {
		a.Section = 1 // popular
		a.Drawer = "popular"
		a.RecTab = RecHot
	})
	center := emptyOffset(t, tt2, "热门", "暂无热门内容")
	t.Logf("热门空态偏移 = %.1f（目标 %.0f）", center, tipWant)
	if center < tipWant-10 || center > tipWant+10 {
		t.Errorf("热门空态偏移 = %.1f，应约 %.0f（h-40 居中）", center, tipWant)
	}

	// 弹幕：py-8 贴顶，明显比上面两种高。
	_, tt3 := emptyProbe(t, func(a *App) {
		a.Drawer = DrawerDanmaku
		a.DanmakuTab = TabDanmaku
	})
	note := emptyOffset(t, tt3, "弹幕 (0)", "暂无弹幕")
	t.Logf("弹幕空态偏移 = %.1f（目标 %.0f）", note, noteWant)
	if note < noteWant-10 || note > noteWant+10 {
		t.Errorf("弹幕空态偏移 = %.1f，应约 %.0f（py-8 贴顶）", note, noteWant)
	}
	if note >= tip-20 {
		t.Errorf("弹幕空态（%.1f）不该和 160 高块里的空态（%.1f）一样低", note, tip)
	}
}

// TestEmptyStateInk 钉住两种空态的墨色：热门/推荐是覆写后的 #64748b
// （= [100 116 139]），稍后再看是 #94a3b8（= [148 163 184]）。
//
// **反证**：把 emptyCenterVariant 的颜色也写成 #94a3b8 → 热门那条通道差
// ~48、测试红。
func TestEmptyStateInk(t *testing.T) {
	check := func(name string, img *ui.Tester, text string, want [3]int) {
		t.Helper()
		r, ok := img.Find(text)
		if !ok {
			t.Fatalf("找不到 %q", text)
		}
		got := darkestIn(img.Image(), int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
		t.Logf("%s 最暗像素 = %v，目标 %v", name, got, want)
		for i := range want {
			if d := got[i] - want[i]; d < -25 || d > 25 {
				t.Errorf("%s 通道%d = %d，应约 %d", name, i, got[i], want[i])
			}
		}
	}

	_, tt := emptyProbe(t, func(a *App) {
		a.Section = 3
		a.Drawer = "history"
		a.HistTab = HistWatchLater
	})
	check("稍后再看（#94a3b8）", tt, "暂无稍后再看视频", [3]int{148, 163, 184})

	_, tt2 := emptyProbe(t, func(a *App) {
		a.Section = 1
		a.Drawer = "popular"
		a.RecTab = RecHot
	})
	check("热门（覆写后 #64748b）", tt2, "暂无热门内容", [3]int{100, 116, 139})
}

// TestCollectHeaderKeepsScrollButtons 钉住收藏抽屉表头不再「判空换文案」：
// 收藏夹还没回来时表头照样画左右滚动键，且不出现和正文重复的空态文案
// （原版 collectList 的表头从不判空，空态只在正文里）。
//
// **反证**：加回 `if len(a.Folders) == 0 { emptyNote(...) ; break }` →
// 两个滚动键都点不到、测试红。
func TestCollectHeaderKeepsScrollButtons(t *testing.T) {
	a, tt := emptyProbe(t, func(a *App) {
		a.Section = 2 // favorite
		a.Drawer = "favorite"
	})
	if len(a.Folders) != 0 {
		t.Fatalf("前提：收藏夹为空，实际 %d 个", len(a.Folders))
	}
	if _, ok := tt.Find("向左滚动"); !ok {
		t.Error("收藏夹为空时表头少了左滚动键")
	}
	if _, ok := tt.Find("向右滚动"); !ok {
		t.Error("收藏夹为空时表头少了右滚动键")
	}
	// 正文里的空态还在（只该有一份）。
	if _, ok := tt.Find("暂无收藏内容"); !ok {
		t.Error("正文里应有「暂无收藏内容」空态")
	}
	n := 0
	for _, s := range tt.Texts() {
		if s == "暂无收藏内容" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("「暂无收藏内容」出现 %d 次，应恰好 1 次（正文），表头不该再放一份", n)
	}
}

// TestSeriesDrawerEmptyHint 钉住「点进某个合集后没有视频」的空态文案：原版
// seriesList.tsx 的 role="status" 写死「暂无合集视频」，而 emptyHint 此前漏了
// DrawerSeries 这一支，回落到通用「没有内容」。
//
// **反证**：去掉 emptyHint 里 `if a.Drawer == DrawerSeries` 那段 → 正文变成
// 「没有内容」，测试红。
func TestSeriesDrawerEmptyHint(t *testing.T) {
	a, tt := emptyProbe(t, func(a *App) {
		a.Drawer = DrawerSeries
		a.SeriesName = "被清空的合集"
	})
	if len(a.ListFor(DrawerSeries).Cards) != 0 {
		t.Fatalf("前提：合集列表为空，实际 %d 条", len(a.ListFor(DrawerSeries).Cards))
	}
	if _, ok := tt.Find("暂无合集视频"); !ok {
		t.Error("合集抽屉空态应为「暂无合集视频」")
	}
	for _, s := range tt.Texts() {
		if s == "没有内容" {
			t.Error("合集抽屉空态不该是通用的「没有内容」")
		}
	}
}
