package view

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（桌面「减少透明度」偏好 `prefers-reduced-transparency: reduce`）的
// 无头回归。原版 `main:src/styles/globals.css` 末尾（排在高对比块之前，所以
// 高对比的覆写压过它的 --player-border）有一条：
//
//	@media (prefers-reduced-transparency: reduce) {
//	    html:root #player {
//	        --player-surface: #e6edf4;
//	        --player-sheen: none;
//	        --player-border: rgba(71,85,105,.28);
//	        --player-edge-fade: rgba(71,85,105,.08);
//	        --player-shadow: inset 0 1px 0 rgba(255,255,255,.72),
//	                         0 -8px 20px rgba(15,23,42,.04); }
//	    html:root #player .player-controls,
//	    ... .player-volume-popover, ... .player-speed-popover { backdrop-filter: none }
//	    ... .player-volume-popover, ... .player-speed-popover {
//	        background-color: #e6edf4; background-image: none; }
//	    html:root #player .player-controls .player-button::before {
//	        background-color: rgba(248,251,254,.88); background-image: none; }
//	    ... .player-eq-button[data-active]::before,
//	    ... .player-sponsor-button[data-active]::before,
//	    ... .player-volume-button[data-open]::before,
//	    ... .player-speed-button[data-open]::before { background-color: #d9eef9; }
//	    html:root #player .player-speed-option { color: #334155; }
//	}
//
// mygo 的 `ui.Preferences` 没有这一项（它只在根包 `mygo.Theme.Preferences()` 的
// `ReduceTransparency` 里），所以原生版在 main 读一次、`mygo.Theme.OnUpdated`
// 跟进，视图只读 `Theme.ReducedTransparency`。下面钉住主题合成、键面 / 弹层
// 材质分支，并用 backdrop 层数做无头反证：这层偏好下播放栏与弹层都不再走
// 框架 Liquid Glass。

// rtThemes 返回浅色 / 烟熏玻璃两个时段各自叠上减少透明度的主题。
func rtThemes() (light, dark Theme) {
	return ThemeFor(PeriodAt(12)).withReduceTransparency(true),
		ThemeFor(PeriodAt(0)).withReduceTransparency(true)
}

// TestThemeReduceTransparencyTokens 钉住四个令牌的覆写（浅色与烟熏玻璃时段
// 取同一套不透明 slate，因为这条块带 html:root + #player、压过烟熏玻璃那条
// 1,1,0）、只动播放栏、幂等，以及退回普通时按**时段**重新推导。
func TestThemeReduceTransparencyTokens(t *testing.T) {
	for _, hour := range []int{12, 0} {
		base := ThemeFor(PeriodAt(hour))
		rt := base.withReduceTransparency(true)

		if !rt.ReducedTransparency {
			t.Fatalf("%d 点叠减少透明度后 ReducedTransparency 应为真", hour)
		}
		if got := rt.PlayerSurface; got != ui.Hex("#e6edf4") {
			t.Errorf("%d 点 --player-surface = %v，应为 #e6edf4", hour, got)
		}
		if got := rt.PlayerBorder; got != ui.Hex("#475569").Alpha(0.28) {
			t.Errorf("%d 点 --player-border = %v，应为 slate .28", hour, got)
		}
		if got := rt.PlayerEdgeFade; got != ui.Hex("#475569").Alpha(0.08) {
			t.Errorf("%d 点 --player-edge-fade = %v，应为 slate .08", hour, got)
		}
		// 只覆写播放栏令牌：Dark 标志与其它令牌原样保留。
		if rt.Dark != base.Dark {
			t.Errorf("%d 点把 Dark 改成了 %v", hour, rt.Dark)
		}
		if rt.Panel != base.Panel || rt.Glass != base.Glass || rt.PlayerButton != base.PlayerButton {
			t.Errorf("%d 点动了播放栏以外的令牌", hour)
		}
		// 幂等。
		if again := rt.withReduceTransparency(true); !reflect.DeepEqual(again, rt) {
			t.Errorf("%d 点重复叠减少透明度改变了主题", hour)
		}
		// 退回普通 = 时段基态（浅色与烟熏玻璃各自回自己的面）。
		if back := rt.withReduceTransparency(false); !reflect.DeepEqual(back, base) {
			t.Errorf("%d 点退回普通后与时段基态不一致", hour)
		}
	}
	// 两个时段的减少透明度面必须完全一样（都在 ThemeFor 之上被同一套覆写）。
	light, dark := rtThemes()
	if light.PlayerSurface != dark.PlayerSurface || light.PlayerBorder != dark.PlayerBorder ||
		light.PlayerEdgeFade != dark.PlayerEdgeFade {
		t.Error("浅色与烟熏玻璃时段的减少透明度播放栏令牌应一致")
	}
	if light.PlayerSurface == ThemeFor(PeriodAt(12)).PlayerSurface {
		t.Error("浅色的减少透明度面没有把近白玻璃换成不透明 #e6edf4")
	}
}

// TestThemePreferencesCompose 钉住两个无障碍偏好的层叠：源序是
// reduced-transparency 在前、contrast 在后，重叠的 --player-border 由高对比
// 胜出，而高对比不管的 --player-surface / --player-edge-fade 仍取减少透明度值。
func TestThemePreferencesCompose(t *testing.T) {
	rt := ThemeFor(PeriodAt(12)).withPreferences(false, true)
	if !rt.ReducedTransparency || rt.HighContrast {
		t.Fatalf("只开减少透明度时标志不对：rt=%v hc=%v", rt.ReducedTransparency, rt.HighContrast)
	}
	if got := rt.PlayerBorder; got != ui.Hex("#475569").Alpha(0.28) {
		t.Errorf("只开减少透明度的 --player-border = %v，应为 slate .28", got)
	}

	both := ThemeFor(PeriodAt(12)).withPreferences(true, true)
	if !both.ReducedTransparency || !both.HighContrast {
		t.Fatalf("两个偏好都应置位：rt=%v hc=%v", both.ReducedTransparency, both.HighContrast)
	}
	// 重叠令牌：高对比 slate .52 压过减少透明度 slate .28。
	if got := both.PlayerBorder; got != ui.Hex("#334155").Alpha(0.52) {
		t.Errorf("两个偏好同开时 --player-border = %v，应为高对比 slate .52", got)
	}
	if got := both.PlayerSurface; got != ui.Hex("#e6edf4") {
		t.Errorf("两个偏好同开时 --player-surface = %v，应为减少透明度 #e6edf4", got)
	}
	if got := both.PlayerEdgeFade; got != ui.Hex("#475569").Alpha(0.08) {
		t.Errorf("两个偏好同开时 --player-edge-fade = %v，应为减少透明度 slate .08", got)
	}
	if got := both.PlayerTime; got != ui.Hex("#334155") {
		t.Errorf("两个偏好同开时 --player-time-color = %v，应为高对比 #334155", got)
	}
	// 从两个偏好退回只剩高对比：时段基态 + 高对比。
	hcOnly := both.withPreferences(true, false)
	if hcOnly.ReducedTransparency || !hcOnly.HighContrast {
		t.Fatalf("退回只剩高对比时标志不对：rt=%v hc=%v", hcOnly.ReducedTransparency, hcOnly.HighContrast)
	}
	if !reflect.DeepEqual(hcOnly, ThemeFor(PeriodAt(12)).withHighContrast(true)) {
		t.Error("退回只剩高对比后与「时段基态 + 高对比」不一致")
	}
}

// TestRTButtonFill 钉住键面实底：普通键白 .88、四个激活键 #d9eef9。
func TestRTButtonFill(t *testing.T) {
	if got := rtButtonFill(false); got != ui.Hex("#f8fbfe").Alpha(0.88) {
		t.Errorf("普通键面 = %v，应为白 .88", got)
	}
	if got := rtButtonFill(true); got != ui.Hex("#d9eef9") {
		t.Errorf("激活键面 = %v，应为 #d9eef9", got)
	}
	if rtButtonFill(false) == rtButtonFill(true) {
		t.Error("普通键与激活键的实底不应相同")
	}
}

// TestPlayerFillReducedTransparency 钉住减少透明度下键面填充：通用规则
// (1,3,2) 压过悬停 (1,3,1) 与 :active::before，所以悬停 / 按下都**不改底色** ——
// 工具键无论悬停与否都是白 .88（激活键 #d9eef9），播放键是白 .88，按下也不回
// 到 --liquid-bg-active 的白 .16。
func TestPlayerFillReducedTransparency(t *testing.T) {
	light, dark := rtThemes()

	for _, th := range []Theme{light, dark} {
		for _, hovered := range []bool{false, true} {
			if got := playerToolFill(th, false, hovered); got != rtButtonFill(false) {
				t.Errorf("减少透明度普通工具键(h=%v)底 = %v，应为白 .88", hovered, got)
			}
			if got := playerToolFill(th, true, hovered); got != rtButtonFill(true) {
				t.Errorf("减少透明度激活工具键(h=%v)底 = %v，应为 #d9eef9", hovered, got)
			}
		}
		for _, playing := range []bool{false, true} {
			for _, hovered := range []bool{false, true} {
				if got := playerPlayFill(th, playing, hovered); got != rtButtonFill(false) {
					t.Errorf("减少透明度播放键(p=%v,h=%v)底 = %v，应为白 .88", playing, hovered, got)
				}
			}
		}
	}
	// 按下：普通键白 .88、激活键 #d9eef9，都不走 --liquid-bg-active。
	if got := playerPressFillFor(light, false); got != rtButtonFill(false) {
		t.Errorf("减少透明度按下普通键 = %v，应为白 .88", got)
	}
	if got := playerPressFillFor(light, true); got != rtButtonFill(true) {
		t.Errorf("减少透明度按下激活键 = %v，应为 #d9eef9", got)
	}
	// 对照：常态（无偏好）仍是白 .16。
	if got := playerPressFillFor(ThemeFor(PeriodAt(12)), false); got != playerPressFill() {
		t.Errorf("无偏好按下底色 = %v，应为 --liquid-bg-active 白 .16", got)
	}
}

// TestPlayerSheenReducedTransparency 钉住键面 ::before 光泽整条撤掉：原版在
// 这条偏好下写 `background-image: none`（通用规则盖过所有键自己的光泽），所以
// playerSheenForTheme 恒返回 nil；对照常态仍按原层叠选层。
func TestPlayerSheenReducedTransparency(t *testing.T) {
	light, dark := rtThemes()
	kinds := []playerSheenKind{sheenKindStatic, sheenKindPlay, sheenKindMiniPlay}
	for _, th := range []Theme{light, dark} {
		for _, kind := range kinds {
			for _, playing := range []bool{false, true} {
				for _, hovered := range []bool{false, true} {
					if got := playerSheenForTheme(th, kind, playing, hovered); got != nil {
						t.Errorf("减少透明度(kind=%d,p=%v,h=%v)仍铺了 ::before 光泽 %p", kind, playing, hovered, got)
					}
				}
			}
		}
	}
	// 对照：无偏好时层叠照旧（浅色工具键走 --glass-sheen-static）。
	if got := playerSheenForTheme(ThemeFor(PeriodAt(12)), sheenKindStatic, false, false); got != sheenGlassBaked {
		t.Errorf("无偏好浅色工具键光泽 = %p，应为 --glass-sheen-static", got)
	}
	// 弹层洗光同样撤掉。
	if got := volumePopoverSheen(light); got != nil {
		t.Errorf("减少透明度音量弹层洗光 = %p，应为 nil", got)
	}
	if got := speedPopoverSheen(dark); got != nil {
		t.Errorf("减少透明度倍速弹层洗光 = %p，应为 nil", got)
	}
}

// TestPlayerBarShadowReducedTransparency 钉住向上投影换成
// `0 -8px 20px rgba(15,23,42,.04)`（更紧、更淡），且与主窗 / mini 无关。
func TestPlayerBarShadowReducedTransparency(t *testing.T) {
	light, dark := rtThemes()
	for name, th := range map[string]Theme{"浅色": light, "深色": dark} {
		for _, mini := range []bool{false, true} {
			s := playerBarShadowSpec(th, mini)
			if s.X != 0 || s.Y != -8 || s.Spread != 0 || s.Blur != 20 || s.C != shadowInk.Alpha(0.04) {
				t.Errorf("%s(mini=%v)向上投影 = %+v，应为 0 -8 20 墨 .04", name, mini, s)
			}
		}
	}
	// 与常态的 (0,-10,28/.05) 必须不同，否则这条偏好没生效。
	if playerBarShadowSpec(light, false) == playerBarShadowSpec(ThemeFor(PeriodAt(12)), false) {
		t.Error("减少透明度向上投影与常态相同了")
	}
}

// TestPlayerSpeedOptionInkReducedTransparency 钉住倍速档位文字换成实底上的
// `color: #334155`（浅色 / 深色都一样）。
func TestPlayerSpeedOptionInkReducedTransparency(t *testing.T) {
	light, dark := rtThemes()
	for name, th := range map[string]Theme{"浅色": light, "深色": dark} {
		if got := playerSpeedOptionInk(th); got != ui.Hex("#334155") {
			t.Errorf("%s减少透明度倍速档位墨色 = %v，应为 #334155", name, got)
		}
	}
	// 对照：常态浅色 #475569、深色半透明。
	if got := playerSpeedOptionInk(ThemeFor(PeriodAt(12))); got != ui.Hex("#475569") {
		t.Errorf("常态浅色倍速档位墨色 = %v，应为 #475569", got)
	}
}

// TestPlayerBarMaterialNoGlass 无头反证：减少透明度下播放栏与音量弹层都不再
// 走框架 Liquid Glass（原版把 `backdrop-filter` 整条撤掉）。用 backdrop 层数
// 直接量「这一帧建了几层读 backdrop 的玻璃」：偏好关时播放栏 + 弹层各一层，
// 偏好开时归零。
func TestPlayerBarMaterialNoGlass(t *testing.T) {
	// 关：播放栏玻璃 + 音量弹层玻璃。
	BackdropLayers()
	playerBarOnlyProbe(t, func(a *App) {
		a.Theme = ThemeFor(PeriodAt(12))
		a.ShowVolume = true
	})
	if got := BackdropLayers(); got < 2 {
		t.Fatalf("常态播放栏 + 音量弹层应至少 2 层玻璃，实测 %d", got)
	}

	// 开：播放栏与弹层都换成不透明实底，不再有玻璃层。
	BackdropLayers()
	playerBarOnlyProbe(t, func(a *App) {
		a.Theme = ThemeFor(PeriodAt(12)).withReduceTransparency(true)
		a.ShowVolume = true
	})
	if got := BackdropLayers(); got != 0 {
		t.Errorf("减少透明度下播放栏 / 弹层仍建了 %d 层玻璃，应为 0", got)
	}

	// 倍速弹层同样。
	BackdropLayers()
	playerBarOnlyProbe(t, func(a *App) {
		a.Theme = ThemeFor(PeriodAt(12)).withReduceTransparency(true)
		a.ShowSpeed = true
	})
	if got := BackdropLayers(); got != 0 {
		t.Errorf("减少透明度下倍速弹层仍建了 %d 层玻璃，应为 0", got)
	}
}

// TestPlayerBarMaterialOpacity 无头实测（直接因果）：常态播放栏是半透明玻璃、
// 会采样背后的深色，减少透明度下变成不透明 #e6edf4 面。给根铺一块纯黑背景，
// 播放栏纯玻璃行在偏好开时明显比偏好关时亮（玻璃会把黑抬亮，但仍远暗于实底）。
func TestPlayerBarMaterialOpacity(t *testing.T) {
	probe := func(theme Theme) *ui.Tester {
		a := NewApp(func() {})
		a.Theme = theme
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		a.Dur, a.Pos, a.SeekValue, a.Volume = 200, 100, 100, 0.5
		tst := ui.NewTester(func(c *ui.Context) {
			c.Root().Background(ui.Hex("#000000"))
			a.playerBar(c)
		}, 1000, 200)
		tst.Frame()
		return tst
	}

	luma := func(tst *ui.Tester) float64 {
		r, ok := tst.Find("音量")
		if !ok {
			t.Fatal("找不到音量键")
		}
		return barStripLuma(tst, int(r.Y)-12+4, 20, 980)
	}

	off := luma(probe(ThemeFor(PeriodAt(12))))
	on := luma(probe(ThemeFor(PeriodAt(12)).withReduceTransparency(true)))
	t.Logf("黑底播放栏纯玻璃行亮度：常态=%v 减少透明度=%v", off, on)
	if on <= off {
		t.Errorf("减少透明度播放栏应变亮（不透明 #e6edf4 面而非半透明玻璃）：关=%v 开=%v", off, on)
	}
	// #e6edf4 的亮度 ≈ 236；半透明玻璃压黑底后远低于它。
	if on < 200 {
		t.Errorf("减少透明度播放栏亮度 = %v，不像不透明 #e6edf4 面", on)
	}
}

// TestSetReduceTransparencyWired 钉住视图侧的偏好入口：SetReduceTransparency
// 只在真的变化时返回 true，并立刻把整份主题从时段基态重推（保留高对比）；
// RefreshTheme（时段翻页）与 Shell 每帧比对都不会把偏好弄丢。
func TestSetReduceTransparencyWired(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))

	if a.ReducedTransparency() {
		t.Fatal("默认不应是减少透明度")
	}
	if !a.SetReduceTransparency(true) {
		t.Fatal("首次置位应报告变化")
	}
	if !a.ReducedTransparency() || !a.Theme.ReducedTransparency {
		t.Fatal("置位后 Theme 没跟上")
	}
	if got := a.Theme.PlayerSurface; got != ui.Hex("#e6edf4") {
		t.Errorf("置位后 --player-surface = %v，应为 #e6edf4", got)
	}
	if a.SetReduceTransparency(true) {
		t.Error("重复置位不应报告变化")
	}
	if !a.SetReduceTransparency(false) {
		t.Fatal("退回应报告变化")
	}
	if a.Theme.ReducedTransparency || a.Theme.PlayerSurface != ThemeFor(PeriodAt(12)).PlayerSurface {
		t.Error("退回后主题没回时段基态")
	}

	// 高对比 + 减少透明度：SetReduceTransparency 重推时保留高对比。
	a.Theme = a.Theme.withHighContrast(true)
	a.SetReduceTransparency(true)
	if !a.Theme.HighContrast {
		t.Error("SetReduceTransparency 重推时把高对比弄丢了")
	}
	// 时段翻页（RefreshTheme）同样保留两个偏好。
	a.RefreshTheme()
	if !a.Theme.ReducedTransparency || !a.Theme.HighContrast {
		t.Errorf("RefreshTheme 后偏好丢失：rt=%v hc=%v", a.Theme.ReducedTransparency, a.Theme.HighContrast)
	}
	// Shell 每帧只按 a.ReducedTransparency() 比对，不会把外部置位改掉。
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	if !a.Theme.ReducedTransparency {
		t.Error("Shell 渲染一帧后减少透明度偏好被清掉了")
	}
}
