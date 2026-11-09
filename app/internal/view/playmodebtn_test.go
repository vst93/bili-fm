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
// 也就是材质必须逐像素一致 —— 浅色与深色、常态 / 悬停 / 按下都一样。此前播放
// 模式键直接铺浅色的 drawerHeaderSurface，深色主题下就是一块亮白的键。

// playmodeHeaderProbe 渲染播放列表抽屉的表头（让定位 / 播放模式 / 清空三颗键
// 同时出现），返回 tester 与三颗键的矩形。
func playmodeHeaderProbe(t *testing.T, mode string) (*ui.Tester, ui.Rect, ui.Rect, ui.Rect) {
	t.Helper()
	a := headerApp(mode)
	a.Playlist = []PlayItem{
		{ID: "BV1-1", Bvid: "BV1", Cid: 1, Title: "歌一"},
		{ID: "BV2-2", Bvid: "BV2", Cid: 2, Title: "歌二"},
	}
	a.PlaylistTab = ListUser
	a.PlayingPlaylist = ListUser // 让「定位」键出现，三颗键才是同一排
	a.Index = 0
	a.Drawer = DrawerPlaylist

	tst := headerProbe(t, a)
	pm, ok1 := tst.Find("当前：顺序播放")
	loc, ok2 := tst.Find("定位到当前播放的记录")
	clr, ok3 := tst.Find("清空播放列表")
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("表头三颗键没都出现：play-mode=%v locate=%v clear=%v", ok1, ok2, ok3)
	}
	return tst, pm, loc, clr
}

// playModeBands 是避开图标的四条采样带（图标 18px 居中，约占高度的
// 22%–78%）：顶 / 底描边内高光，以及图标上下两条纯填充带。播放模式键的图标
// 墨色与旁边两颗不同，中间带会把「图标颜色」误读成「键面材质」。
var playModeBands = [][2]float64{{0.02, 0.09}, {0.10, 0.20}, {0.80, 0.90}, {0.93, 0.98}}

// TestPlayModeButtonMatchesHeaderButtons 比播放模式键与旁边两颗键（定位 / 清空）
// 的键面材质：常态三颗键亮度一致；悬停 / 按下的**变化量**一致（同一套材质与
// 过渡）。比变化量而不是绝对值，是因为三颗键的 x 不同、底下的表头渐变略有差异；
// 半透明填充上的变化量本身也随底层颜色浮动（实测最多差 2.6），所以容差取 3。
func TestPlayModeButtonMatchesHeaderButtons(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			tst, _, _, _ := playmodeHeaderProbe(t, mode)
			rest := map[string]headerShot{}
			for _, label := range []string{"当前：顺序播放", "定位到当前播放的记录", "清空播放列表"} {
				rest[label] = headerRender(t, tst, label, "rest")
			}
			pm := rest["当前：顺序播放"]
			for _, other := range []string{"定位到当前播放的记录", "清空播放列表"} {
				for _, band := range playModeBands {
					if d := absF(float32(headerBand(t, pm, band[0], band[1]) - headerBand(t, rest[other], band[0], band[1]))); d > 2.0 {
						t.Errorf("%s 常态：播放模式键亮度 %.2f ≠ %s %.2f（%.2f–%.2f 带）",
							mode, headerBand(t, pm, band[0], band[1]), other,
							headerBand(t, rest[other], band[0], band[1]), band[0], band[1])
					}
				}
				for _, state := range []string{"hover", "press"} {
					pms := headerRender(t, tst, "当前：顺序播放", state)
					os := headerRender(t, tst, other, state)
					for _, band := range playModeBands {
						dpm := headerBand(t, pms, band[0], band[1]) - headerBand(t, pm, band[0], band[1])
						dother := headerBand(t, os, band[0], band[1]) - headerBand(t, rest[other], band[0], band[1])
						if d := absF(float32(dpm - dother)); d > 3.0 {
							t.Errorf("%s %s：播放模式键变化 %.2f ≠ %s 变化 %.2f（%.2f–%.2f 带，差 %.2f）",
								mode, state, dpm, other, dother, band[0], band[1], d)
						}
					}
				}
			}
		})
	}
}

// TestPlayModeButtonPressedScales 三颗键按下缩到同一档（.94），松手回位。
func TestPlayModeButtonPressedScales(t *testing.T) {
	tst, _, _, _ := playmodeHeaderProbe(t, "light")
	rest := headerRender(t, tst, "当前：顺序播放", "rest")
	press := headerRender(t, tst, "当前：顺序播放", "press")
	if ratio := press.r.W / rest.r.W; absF(ratio-headerPressScale) > 0.02 {
		t.Errorf("播放模式键按下缩放 = %.3f，应为 %.2f", ratio, headerPressScale)
	}
}
