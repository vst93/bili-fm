package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏向上投影随时段 / mini 变化）的无头测试。原版播放栏 box-shadow
// 的第三层（向上投到内容区的那道淡影）四个组合各不同：
//
//	主窗浅色  0 -10px 28px rgba(15,23,42,.05)
//	主窗深色  0 -10px 28px rgba(15,23,42,.045)  （烟熏玻璃 --player-shadow）
//	mini 浅色 0 -10px 22px rgba(15,23,42,.05)   （mini 规则里的兜底值）
//	mini 深色 0 -10px 28px rgba(15,23,42,.045)  （--player-shadow 盖过兜底）
//
// 原生版此前两层播放栏都写死 `0 -10px 28px .05`。

// TestPlayerBarShadowTokens 钉住四个组合的位置 / 模糊 / 墨色。
func TestPlayerBarShadowTokens(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	dark := ThemeFor(PeriodAt(0))
	cases := []struct {
		name  string
		theme Theme
		mini  bool
		blur  float32
		alpha float32
	}{
		{"主窗浅色", light, false, 28, 0.05},
		{"主窗深色", dark, false, 28, 0.045},
		{"mini 浅色", light, true, 22, 0.05},
		{"mini 深色", dark, true, 28, 0.045},
	}
	for _, c := range cases {
		s := playerBarShadowSpec(c.theme, c.mini)
		if s.X != 0 || s.Y != -10 || s.Spread != 0 {
			t.Errorf("%s: 位置应为 (0,-10) spread 0，得到 %+v", c.name, s)
		}
		if s.Blur != c.blur {
			t.Errorf("%s: blur = %v，应为 %v", c.name, s.Blur, c.blur)
		}
		if want := shadowInk.Alpha(c.alpha); s.C != want {
			t.Errorf("%s: 墨色 = %v，应为 %v", c.name, s.C, want)
		}
	}
	// 三个差异必须真的存在，否则这轮白改。
	if playerBarShadowSpec(light, false).Blur == playerBarShadowSpec(light, true).Blur {
		t.Error("mini 浅色投影应比主窗浅色更紧（22 vs 28）")
	}
	if playerBarShadowSpec(light, false).C == playerBarShadowSpec(dark, false).C {
		t.Error("深色时段投影墨色应比浅色淡（.045 vs .05）")
	}
	if playerBarShadowSpec(dark, true) != playerBarShadowSpec(dark, false) {
		t.Error("mini 深色应吃 --player-shadow，与主窗深色同值")
	}
}

// TestPlayerBarShadowWired 无头反证：把投影换成一个不透明黑影，播放栏**上沿
// 上方**那条横带应明显变暗 —— 证明 playerBarShadowSpec 的返回值真的进了
// `Shadow(...)`。主窗与 mini 两层都要验（同一函数、两处调用点）。
func TestPlayerBarShadowWired(t *testing.T) {
	orig := playerBarShadowSpec
	defer func() { playerBarShadowSpec = orig }()

	stub := func(s shadowSpec) {
		playerBarShadowSpec = func(Theme, bool) shadowSpec { return s }
	}

	// 主窗 1000×700：栏高 56 → 栏顶 y = 644，取上方 2px 那条横带。
	main := func(s shadowSpec) float64 {
		stub(s)
		_, tst := playerBarProbe(t, nil)
		return barStripLuma(tst, 700-56-2, 20, 980)
	}
	mainNone := main(shadowSpec{0, -10, 28, 0, ui.Transparent})
	mainBlack := main(shadowSpec{0, -10, 28, 24, ui.Hex("#000000")})
	if mainBlack >= mainNone {
		t.Errorf("主窗换黑影后栏顶上方应变暗：无影=%v 黑影=%v", mainNone, mainBlack)
	}

	// mini 400×155：栏高 43 → 栏顶 y = 112，取上方 2px 那条横带。
	mini := func(s shadowSpec) float64 {
		stub(s)
		tst := miniProbe(t, false)
		return barStripLuma(tst, miniHeight-miniPlayerH-2, 5, miniWidth-5)
	}
	miniNone := mini(shadowSpec{0, -10, 28, 0, ui.Transparent})
	miniBlack := mini(shadowSpec{0, -10, 28, 24, ui.Hex("#000000")})
	if miniBlack >= miniNone {
		t.Errorf("mini 换黑影后栏顶上方应变暗：无影=%v 黑影=%v", miniNone, miniBlack)
	}
}
