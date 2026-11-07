package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 用户 2026-11-07 报：「当前播放列表的 播放顺序 按钮没有和其他按钮一样的按钮
// 背景样式，要保持一致」。
//
// 原版（tauri 分支 src/components/playlist.tsx）里这三颗键是**同一个**组件：
//
//	<Button isIconOnly size="sm" variant="light" title="定位到当前播放的记录">
//	<Button isIconOnly size="sm" variant="light" title="当前：顺序播放">
//	<Button isIconOnly size="sm" variant="light" title="清空播放列表">
//
// 也就是材质必须逐像素一致。原生版此前把播放模式键单独铺了 GlassActive /
// GlassHover，底色 alpha 虽然一样（白 .16 / .38），但少了表头小键那层
// `inset 0 1px 1px 白 .9` 顶部高光与悬停外投影，也没有按压缩放 —— 在一排键里
// 看着就是「平」的、像没有底色的裸图标。

// playmodeHeaderProbe 渲染播放列表抽屉的表头（让定位 / 播放模式 / 清空三颗键
// 同时出现），返回 tester 与三颗键的矩形。
func playmodeHeaderProbe(t *testing.T) (*ui.Tester, ui.Rect, ui.Rect, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Playlist = []PlayItem{
		{ID: "BV1-1", Bvid: "BV1", Cid: 1, Title: "歌一"},
		{ID: "BV2-2", Bvid: "BV2", Cid: 2, Title: "歌二"},
	}
	a.PlaylistTab = ListUser
	a.PlayingPlaylist = ListUser // 让「定位」键出现，三颗键才是同一排
	a.Index = 0
	a.Drawer = DrawerPlaylist

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	pm, ok1 := tst.Find("当前：顺序播放")
	loc, ok2 := tst.Find("定位到当前播放的记录")
	clr, ok3 := tst.Find("清空播放列表")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("表头三颗键没都出现：play-mode=%v locate=%v clear=%v", ok1, ok2, ok3)
	}
	return tst, pm, loc, clr
}

// TestPlayModeButtonMatchesHeaderButtons 比播放模式键与旁边定位键的**背景带均值**：
// 顶部两行（`inset 0 1px 1px 白 .9` 的落点）与底部两行必须基本一致。
//
// 为什么比均值而不是逐像素：两颗键的 x 不同（中间还隔着 tab），亚像素落点与圆角
// 抗锯齿会带来 ±1 的抖动，逐像素比会假红。均值能把「缺了那层 .9 白高光」这种
// 系统性差异放大出来：实测缺高光时顶部两行均值 250.4 vs 252.6（差 2.2），
// 补齐后是 252.65 vs 252.60（差 0.05）。
func TestPlayModeButtonMatchesHeaderButtons(t *testing.T) {
	tst, pm, loc, _ := playmodeHeaderProbe(t)
	img := tst.Image()

	bandMean := func(r ui.Rect, y0, y1 int) float64 {
		sum, n := 0, 0
		for y := y0; y < y1; y++ {
			for x := 4; x < 28; x++ {
				sum += lum(pixelAt(t, img, int(r.X)+x, int(r.Y)+y))
				n++
			}
		}
		return float64(sum) / float64(n)
	}

	for _, b := range []struct {
		name   string
		y0, y1 int
	}{
		{"顶部两行（内高光）", 0, 2},
		{"底部两行", 30, 32},
	} {
		got := bandMean(pm, b.y0, b.y1)
		want := bandMean(loc, b.y0, b.y1)
		if abs(got-want) > 1.0 {
			t.Errorf("%s：播放模式键均值 %.2f ≠ 定位键 %.2f（差 %.2f）—— "+
				"原版三颗键是同一个 Button，材质必须一致；缺 inset 顶部高光会差约 2.2",
				b.name, got, want, abs(got-want))
		}
	}
}

// TestPlayModeButtonSharesHeaderMaterial 是**结构**断言：播放模式键也必须经过
// 表头小键的共享材质与按压手感（drawerHeaderSurface / drawerHeaderPress）。
//
// 像素测试盯的是「现在长得一样」，这条盯的是「走的是同一套实现」—— 有人以后把
// 它改回手写底色，像素那条会红，但这条能直接指出「绕过了共享材质」。
func TestPlayModeButtonSharesHeaderMaterial(t *testing.T) {
	origSurface, origPress := drawerHeaderSurface, drawerHeaderPress
	t.Cleanup(func() { drawerHeaderSurface, drawerHeaderPress = origSurface, origPress })

	surfaceCalls, pressCalls := 0, 0
	drawerHeaderSurface = func(b ui.Element, hovered bool) ui.Element {
		surfaceCalls++
		return origSurface(b, hovered)
	}
	drawerHeaderPress = func(b ui.Element) {
		pressCalls++
		origPress(b)
	}

	playmodeHeaderProbe(t)
	// 表头一排三颗键（定位 / 播放模式 / 清空）每帧各走一次。
	if surfaceCalls < 3 {
		t.Errorf("drawerHeaderSurface 只被调了 %d 次，表头三颗键应各走一次"+
			"（播放模式键绕过了共享材质？）", surfaceCalls)
	}
	if pressCalls < 3 {
		t.Errorf("drawerHeaderPress 只被调了 %d 次，表头三颗键应各走一次"+
			"（播放模式键没有按压缩放？）", pressCalls)
	}
}
