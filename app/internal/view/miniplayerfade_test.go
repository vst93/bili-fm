package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（迷你播放栏上沿淡出）的无头测试。原版 `body.mini-mode #player::before`
// 给迷你播放栏上沿铺一条 10px 的渐变（transparent → --player-edge-fade）：
// 主窗早就用框架 `glass.ScrollEdge` 画了这一层，迷你窗一直漏着——栏顶于是
// 是一条硬切线。

// TestMiniPlayerEdgeFadeTokens 钉住迷你淡出色：浅色 = 规则兜底
// rgba(244,248,252,.52)（--player-edge-fade 在浅色时段未定义）；五个烟熏玻璃
// 时段 = --player-edge-fade（rgba(71,85,105,.11)），与主窗同源。
func TestMiniPlayerEdgeFadeTokens(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	if got, want := miniPlayerEdgeFade(light), ui.Hex("#f4f8fc").Alpha(0.52); got != want {
		t.Errorf("浅色 mini 淡出色 = %v，应为 %v", got, want)
	}
	dark := ThemeFor(PeriodAt(0))
	if got := miniPlayerEdgeFade(dark); got != dark.PlayerEdgeFade {
		t.Errorf("烟熏玻璃 mini 淡出色 = %v，应为 --player-edge-fade %v", got, dark.PlayerEdgeFade)
	}
	// mini 的浅色兜底与主窗的兜底不是一个值：原版在 body.mini-mode 规则里
	// 另写了一条更白更实的 rgba(244,248,252,.52)。
	if miniPlayerEdgeFade(light) == light.PlayerEdgeFade {
		t.Error("mini 浅色淡出色不应等于主窗的 rgba(186,230,253,.24)")
	}
}

// TestMiniPlayerEdgeFadeWired 无头反证：把淡出色换成不透明黑，迷你播放栏
// **上沿那条 10px** 应明显变暗 —— 证明 miniPlayerEdgeFade 的返回值真的进了
// `glass.ScrollEdge{Background: …}`。之前 mini 完全没有这一层。
func TestMiniPlayerEdgeFadeWired(t *testing.T) {
	orig := miniPlayerEdgeFade
	defer func() { miniPlayerEdgeFade = orig }()
	realColor := orig(ThemeFor(PeriodAt(12)))

	strip := func(fade ui.Color) float64 {
		miniPlayerEdgeFade = func(Theme) ui.Color { return fade }
		tst := miniProbe(t, false)
		// 栏高 43 → 栏顶 y = 155 − 43 = 112。取栏上方 2px 那条横带。
		return barStripLuma(tst, miniHeight-miniPlayerH-2, 5, miniWidth-5)
	}
	clear, black := strip(ui.Transparent), strip(ui.Hex("#000000"))
	if black >= clear {
		t.Errorf("淡出色换成黑色后迷你栏上沿应变暗：透明=%v 黑=%v", clear, black)
	}
	// 默认淡出色是近白（#f4f8fc .52），叠在略暗的背景上会把这条横带**提亮**
	// 一点（实测 ≈ +0.8 亮度）——所以判据是「与完全不画不同」，而不是更暗。
	if real := strip(realColor); real == clear {
		t.Errorf("默认淡出色未画上（与完全不画一致）：%v", clear)
	}
}
