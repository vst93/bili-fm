package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏 `--player-shadow` 的两层 inset 内影）的无头回归。原版
// `main:src/styles/globals.css` 的 `#player .player-controls`（2634）与
// `body.mini-mode #player .player-controls`（4913）box-shadow 的前两条 `inset`：
//
//	主窗浅色  inset 0 1px 1px 白 .88 / inset 0 -1px 1px 墨 .05
//	主窗深色  inset 0 1px 0   白 .24 / inset 0 -1px 0   墨 .08
//	mini 浅色 inset 0 1px 1px var(--glass-highlight)（白 .9） / inset 0 -1px 1px 墨 .04
//	mini 深色 与主窗深色同（--player-shadow 盖过 mini 兜底）
//
// 这三层（含减少透明度）此前只有减少透明度分支画了顶层白、且内联在
// playerBarMaterial 里；常态播放栏完全没接，只靠框架 Liquid Glass 那条不论
// 上下缘的 1px 折光顶着 —— 顶不到 .88 / .24 / .9 的高光，也**没有**原版那条
// 下缘内暗线。现在拆成 playerBarShadowInsets（按 4 组合逐值）+ 两个调用点。

// TestPlayerBarShadowInsetsTokens 钉住四个组合的逐值内影（层数、偏移、模糊、
// 墨色），并断言四条差异真的存在（否则本轮白改）。
func TestPlayerBarShadowInsetsTokens(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	dark := ThemeFor(PeriodAt(0))

	cases := []struct {
		name  string
		theme Theme
		mini  bool
		want  []shadowSpec
	}{
		{"主窗浅色", light, false, []shadowSpec{
			{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.88)},
			{0, -1, 1, 0, shadowInk.Alpha(0.05)},
		}},
		{"主窗深色", dark, false, []shadowSpec{
			{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.24)},
			{0, -1, 0, 0, shadowInk.Alpha(0.08)},
		}},
		{"mini 浅色", light, true, []shadowSpec{
			{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)},
			{0, -1, 1, 0, shadowInk.Alpha(0.04)},
		}},
		{"mini 深色", dark, true, []shadowSpec{
			{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.24)},
			{0, -1, 0, 0, shadowInk.Alpha(0.08)},
		}},
	}
	for _, c := range cases {
		got := playerBarShadowInsets(c.theme, c.mini)
		if len(got) != len(c.want) {
			t.Errorf("%s：内影层数 = %d，应为 %d（%+v）", c.name, len(got), len(c.want), got)
			continue
		}
		for i := range c.want {
			if got[i] != c.want[i] {
				t.Errorf("%s 第 %d 层 = %+v，应为 %+v", c.name, i, got[i], c.want[i])
			}
		}
	}
	// 四条差异真的存在。
	if playerBarShadowInsets(light, false)[0] == playerBarShadowInsets(light, true)[0] {
		t.Error("mini 浅色顶缘高光应比主窗浅色亮（白 .9 vs .88）")
	}
	if playerBarShadowInsets(light, false)[0].C == playerBarShadowInsets(dark, false)[0].C {
		t.Error("深色时段顶缘高光应比浅色淡（白 .24 vs .88）")
	}
	if playerBarShadowInsets(dark, true)[0] != playerBarShadowInsets(dark, false)[0] {
		t.Error("mini 深色应吃 --player-shadow，与主窗深色同值")
	}
	// 每条组合的第二层是下缘暗线（y = -1），与顶层的 y = +1 区分。
	for _, c := range cases {
		got := playerBarShadowInsets(c.theme, c.mini)
		if got[0].Y != 1 || got[1].Y != -1 {
			t.Errorf("%s：应为「顶缘 y=+1 / 下缘 y=-1」，得到 %+v", c.name, got)
		}
	}
}

// TestPlayerBarShadowInsetsReducedTransparency 钉住减少透明度下的单层：原版
// 该块只声明 `inset 0 1px 0 白 .72`，没有下缘暗线 —— 所以这里只有一层，且
// 与常态两层不同。
func TestPlayerBarShadowInsetsReducedTransparency(t *testing.T) {
	rt := ThemeFor(PeriodAt(12)).withReduceTransparency(true)
	got := playerBarShadowInsets(rt, false)
	if len(got) != 1 || got[0] != (shadowSpec{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.72)}) {
		t.Errorf("减少透明度内影 = %+v，应为单层 inset 0 1 0 白 .72", got)
	}
	// 与常态（两层）不同，且与 mini 无关。
	if len(playerBarShadowInsets(rt, true)) != 1 {
		t.Error("减少透明度下 mini 也应是同一单层")
	}
}

// TestPlayerBarShadowInsetsWired 无头反证：把内影换成一个不透明黑影（罩满
// 整条播放栏），播放栏**下缘内侧 1px** 那条横带应明显变暗 —— 证明
// playerBarShadowInsets 的返回值真的进了 `InsetShadow(...)`。主窗与 mini 两层
// 都要验（同一函数、两处调用点）。
func TestPlayerBarShadowInsetsWired(t *testing.T) {
	orig := playerBarShadowInsets
	defer func() { playerBarShadowInsets = orig }()

	stub := func(ss []shadowSpec) {
		playerBarShadowInsets = func(Theme, bool) []shadowSpec { return ss }
	}

	// 主窗 1000×700：栏高 56 → 栏顶 y = 644，栏内下缘取 y = 700-2。
	main := func(ss []shadowSpec) float64 {
		stub(ss)
		_, tst := playerBarProbe(t, nil)
		return barStripLuma(tst, 700-2, 20, 980)
	}
	mainNone := main(nil)
	mainBlack := main([]shadowSpec{{0, -1, 4, 0, ui.Hex("#000000")}})
	if mainBlack >= mainNone {
		t.Errorf("主窗加内影后栏内下缘应变暗：无影=%v 黑影=%v", mainNone, mainBlack)
	}

	// mini 400×155：栏高 43 → 栏顶 y = 112，栏内下缘取 y = 155-2。
	mini := func(ss []shadowSpec) float64 {
		stub(ss)
		tst := miniProbe(t, false)
		return barStripLuma(tst, miniHeight-2, 5, miniWidth-5)
	}
	miniNone := mini(nil)
	miniBlack := mini([]shadowSpec{{0, -1, 4, 0, ui.Hex("#000000")}})
	if miniBlack >= miniNone {
		t.Errorf("mini 加内影后栏内下缘应变暗：无影=%v 黑影=%v", miniNone, miniBlack)
	}
}
