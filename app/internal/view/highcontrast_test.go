package view

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（高对比偏好 `prefers-contrast: more`）的无头回归测试。
//
// 原版 `main:src/styles/globals.css` 末尾（排在烟熏玻璃与 mini 之后，所以
// 任何时段材质都盖不过它）有一条：
//
//	@media (prefers-contrast: more) {
//	    html:root #player { --player-time-color: #334155;
//	        --player-border: rgba(51,65,85,.52);
//	        --player-track-border: rgba(51,65,85,.58);
//	        --player-track-rest: rgba(51,65,85,.3); }
//	    html:root #player .player-controls .player-button {
//	        border-color: rgba(51,65,85,.44); color: #1e293b; }
//	    html:root #player .player-controls .player-eq-button[data-active] {
//	        border-color: #0369a1; color: #075985; }
//	    html:root #player .player-controls .player-sponsor-button[data-active] {
//	        border-color: #0369a1; color: #075985; }
//	}
//
// 关键特异度：通用 `.player-button` 规则是 (1,3,1)，压过基态 (1,1,0)、tool
// 规则 (1,1,0)、`[data-active]`/`[data-open]` (1,2,0)、深色基态 (1,2,1) 与
// 所有 (1,3,0) 悬停；EQ / 恰饭激活态被 (1,4,1) 单独抬成蓝调，而播放键
// 「播放中悬停」(1,4,0) 仍高过通用规则、走浅色悬停值。

// TestThemeHighContrastTokens 钉住高对比对播放栏四个令牌的覆写、只动播放栏、
// 幂等、以及退回普通时按**时段**重新推导（浅色/深色各自回自己的基态）。
func TestThemeHighContrastTokens(t *testing.T) {
	for _, hour := range []int{12, 0} {
		base := ThemeFor(PeriodAt(hour))
		hc := base.withHighContrast(true)

		if !hc.HighContrast {
			t.Fatalf("%d 点高对比后 HighContrast 应为真", hour)
		}
		if got := hc.PlayerTime; got != ui.Hex("#334155") {
			t.Errorf("%d 点高对比 --player-time-color = %v，应为 #334155", hour, got)
		}
		if got := hc.PlayerBorder; got != ui.Hex("#334155").Alpha(0.52) {
			t.Errorf("%d 点高对比 --player-border = %v，应为 slate .52", hour, got)
		}
		if got := hc.PlayerTrackBorder; got != ui.Hex("#334155").Alpha(0.58) {
			t.Errorf("%d 点高对比 --player-track-border = %v，应为 slate .58", hour, got)
		}
		if got := hc.PlayerTrackRest; got != ui.Hex("#334155").Alpha(0.30) {
			t.Errorf("%d 点高对比 --player-track-rest = %v，应为 slate .3", hour, got)
		}
		// HC 只覆写播放栏那四个令牌：其余令牌（含 Dark 标志）原样保留。
		if hc.Dark != base.Dark {
			t.Errorf("%d 点高对比把 Dark 改成了 %v", hour, hc.Dark)
		}
		if hc.Panel != base.Panel || hc.Glass != base.Glass || hc.PlayerButton != base.PlayerButton {
			t.Errorf("%d 点高对比不该动播放栏以外的令牌", hour)
		}
		// 幂等：已经是高对比再叠一次不应变。
		if again := hc.withHighContrast(true); !reflect.DeepEqual(again, hc) {
			t.Errorf("%d 点重复叠高对比改变了主题", hour)
		}
		// 退回普通 = 时段基态。
		if back := hc.withHighContrast(false); !reflect.DeepEqual(back, base) {
			t.Errorf("%d 点退回普通后与时段基态不一致", hour)
		}
	}
	// 浅色与烟熏玻璃的高对比值完全一样（CSS 里 HC 块排最末、又带 html:root
	// 与 #player，两套时段被覆写成同一套不透明 slate）。
	light, dark := ThemeFor(PeriodAt(12)).withHighContrast(true), ThemeFor(PeriodAt(0)).withHighContrast(true)
	if light.PlayerTime != dark.PlayerTime || light.PlayerBorder != dark.PlayerBorder ||
		light.PlayerTrackBorder != dark.PlayerTrackBorder || light.PlayerTrackRest != dark.PlayerTrackRest {
		t.Error("浅色与烟熏玻璃时段的高对比播放栏令牌应一致")
	}
}

// TestHighContrastToolButtons 钉住高对比下工具键的描边 / 墨色层叠：
// 普通键（下一集/音量/倍速）全部收成 slate .44 / #1e293b；EQ / 恰饭激活态
// 转蓝 #0369a1 / #075985，且**悬停与否都一样**（HC 那条 (1,4,1) 压过 EQ 激活
// 悬停 (1,4,0)）；未激活的 EQ / 恰饭仍是灰。
func TestHighContrastToolButtons(t *testing.T) {
	hc := ThemeFor(PeriodAt(12)).withHighContrast(true)

	for _, key := range []playerToolKey{toolNext, toolVolume, toolSpeed} {
		for _, active := range []bool{false, true} {
			for _, hovered := range []bool{false, true} {
				if got := playerToolBorder(hc, key, active, hovered); got != playerHCBorder {
					t.Errorf("高对比普通工具键(%d,a=%v,h=%v)边 = %v，应为 slate .44", key, active, hovered, got)
				}
				if got := playerToolInk(hc, key, active, hovered); got != playerHCInk {
					t.Errorf("高对比普通工具键(%d,a=%v,h=%v)墨 = %v，应为 #1e293b", key, active, hovered, got)
				}
			}
		}
	}

	for _, key := range []playerToolKey{toolEq, toolSponsor} {
		for _, hovered := range []bool{false, true} {
			if got := playerToolBorder(hc, key, true, hovered); got != playerHCActiveBorder {
				t.Errorf("高对比激活键(%d,h=%v)边 = %v，应为 #0369a1", key, hovered, got)
			}
			if got := playerToolInk(hc, key, true, hovered); got != playerHCActiveInk {
				t.Errorf("高对比激活键(%d,h=%v)墨 = %v，应为 #075985", key, hovered, got)
			}
		}
		if got := playerToolBorder(hc, key, false, true); got != playerHCBorder {
			t.Errorf("高对比未激活键(%d)边 = %v，应为 slate .44", key, got)
		}
		if got := playerToolInk(hc, key, false, true); got != playerHCInk {
			t.Errorf("高对比未激活键(%d)墨 = %v，应为 #1e293b", key, got)
		}
	}
}

// TestHighContrastPlayButton 钉住播放键在高对比下的层叠：通用规则 (1,3,1)
// 盖过暂停悬停 (1,3,0)、播放中 (1,2,0) 与暂停基态；唯独「播放中悬停」那条
// (1,4,0) 更高，仍走浅色悬停的蓝 .40 / #0369a1。
func TestHighContrastPlayButton(t *testing.T) {
	hc := ThemeFor(PeriodAt(12)).withHighContrast(true)

	// 暂停：悬停与否都收成 HC。
	if got := playerPlayBorder(hc, false, false); got != playerHCBorder {
		t.Errorf("高对比暂停键常态边 = %v，应为 slate .44", got)
	}
	if got := playerPlayBorder(hc, false, true); got != playerHCBorder {
		t.Errorf("高对比暂停键悬停边 = %v，应为 slate .44（通用规则压过悬停）", got)
	}
	if got := playerPlayInk(hc, false, false); got != playerHCInk {
		t.Errorf("高对比暂停键常态墨 = %v，应为 #1e293b", got)
	}
	if got := playerPlayInk(hc, false, true); got != playerHCInk {
		t.Errorf("高对比暂停键悬停墨 = %v，应为 #1e293b", got)
	}
	// 播放中未悬停：HC。
	if got := playerPlayBorder(hc, true, false); got != playerHCBorder {
		t.Errorf("高对比播放键常态边 = %v，应为 slate .44", got)
	}
	if got := playerPlayInk(hc, true, false); got != playerHCInk {
		t.Errorf("高对比播放键常态墨 = %v，应为 #1e293b", got)
	}
	// 播放中悬停：(1,4,0) 高过 HC，走浅色悬停值。
	if got := playerPlayBorder(hc, true, true); got != ui.Hex("#0284c7").Alpha(0.40) {
		t.Errorf("高对比播放键悬停边 = %v，应为蓝 .40", got)
	}
	if got := playerPlayInk(hc, true, true); got != ui.Hex("#0369a1") {
		t.Errorf("高对比播放键悬停墨 = %v，应为 #0369a1", got)
	}
}

// TestHighContrastMuteButton 钉住静音键：`[data-active]` (1,2,0) 输给 HC 通用
// 规则 (1,3,1)，所以已静音也不转蓝，统一 slate .44 / #1e293b。
func TestHighContrastMuteButton(t *testing.T) {
	hc := ThemeFor(PeriodAt(12)).withHighContrast(true)
	for _, muted := range []bool{false, true} {
		if got := playerMuteBorder(hc, muted); got != playerHCBorder {
			t.Errorf("高对比静音键(muted=%v)边 = %v，应为 slate .44", muted, got)
		}
		if got := playerMuteInk(hc, muted); got != playerHCInk {
			t.Errorf("高对比静音键(muted=%v)墨 = %v，应为 #1e293b", muted, got)
		}
	}
}

// TestHighContrastWiredFromPreferences 无头反证：桌面的高对比偏好经
// `ui.Preferences` 流进 `a.Theme`（Shell 每帧比对，只在变化时写回）。
func TestHighContrastWiredFromPreferences(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur, a.Pos, a.SeekValue, a.Volume = 200, 100, 100, 0.5

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.SetPreferences(ui.Preferences{HighContrast: true, TextScale: 1})

	if !a.Theme.HighContrast {
		t.Fatal("高对比偏好没流进 Theme.HighContrast")
	}
	if got := a.Theme.PlayerTime; got != ui.Hex("#334155") {
		t.Errorf("高对比渲染后 Theme.PlayerTime = %v，应为 #334155", got)
	}

	// 关掉偏好 → 退回浅色时段基态。
	tst.SetPreferences(ui.Preferences{HighContrast: false, TextScale: 1})
	if a.Theme.HighContrast {
		t.Fatal("关掉高对比后 Theme.HighContrast 仍为真")
	}
	if got := a.Theme.PlayerTime; got != ui.Hex("#64748b") {
		t.Errorf("关掉高对比后 Theme.PlayerTime = %v，应为浅色兜底 #64748b", got)
	}
}
