package view

import (
	"os"
	"strings"
	"testing"
)

// flatten 读源码并把所有空白去掉，便于逐字匹配一条元素链（同 cardtooltip_test.go /
// mutebutton_test.go 的手法）。无头 ui.Tester 不导出无障碍树（mygo 只把 Role/
// Checked/Expanded 写进内部 tree，没有查询入口），所以这类「状态有没有接上线」的
// 断言只能落在源码链上。
func flatten(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", path, err)
	}
	return strings.Join(strings.Fields(string(src)), "")
}

// TestPlayerBarA11yStatesWired 钉住播放栏各键的无障碍状态接线，逐条对齐原版
// player.tsx：
//
//	音量键   aria-expanded={isVolumeOpen}  → RoleDisclosure + Expanded(a.ShowVolume)
//	倍速键   aria-expanded={isSpeedOpen}   → RoleDisclosure + Expanded(a.ShowSpeed)
//	均衡键   aria-pressed={isLoudnessEq}   → RoleToggleButton + Checked(a.EQ)
//	恰饭键   aria-pressed={sponsorSkip}    → RoleToggleButton + Checked(a.Sponsor)
//	静音键   aria-pressed={volume === 0}   → RoleToggleButton + Checked(muted)
//
// 关键点：aria-pressed 的 button 在无障碍上是 toggle button，只有 RoleToggleButton
// 才让 Checked 在各平台读成「已按下」（Windows 的 IToggleProvider / macOS 的
// AXValue 只对 toggle 角色开放；普通 RoleButton 会把状态丢掉）；aria-expanded 的
// 弹层开关用 RoleDisclosure（「显示/隐藏内容的按钮，打开时 expanded」），Windows
// 也只对这种角色给 ExpandCollapsePattern。上一版静音键只加了 Checked 没给角色，
// 在 Windows/macOS 上等于没接。
func TestPlayerBarA11yStatesWired(t *testing.T) {
	flat := flatten(t, "playerbar.go")
	cases := []struct {
		name  string
		chain string
	}{
		{"音量键（aria-expanded）",
			`ui.ButtonBase(c.Key("volume")).Size(32, 32).Radius(Radius).Center().Role(ui.RoleDisclosure).Label("音量").Tooltip("音量").Expanded(a.ShowVolume)`},
		{"倍速键（aria-expanded）",
			`ui.ButtonBase(c.Key("speed")).Size(40, 32).Radius(Radius).Center().Role(ui.RoleDisclosure).Label(speedTip(a.Speed)).Tooltip(speedTip(a.Speed)).Expanded(a.ShowSpeed)`},
		{"均衡键（aria-pressed）",
			`ui.ButtonBase(c.Key("eq")).Size(34, 32).Radius(Radius).Center().Role(ui.RoleToggleButton).Label(eqTip(a.EQ)).Tooltip(eqTip(a.EQ)).Checked(a.EQ)`},
		{"恰饭键（aria-pressed）",
			`ui.ButtonBase(c.Key("sponsor")).Size(34, 32).Radius(Radius).Center().Role(ui.RoleToggleButton).Label(sponsorTip()).Tooltip(sponsorTip()).Checked(a.Sponsor)`},
		{"静音键（aria-pressed）",
			`Center().Role(ui.RoleToggleButton).Label(pick(muted, "取消静音", "静音")).Tooltip(pick(muted, "取消静音", "静音")).Checked(muted)`},
	}
	for _, c := range cases {
		chain := strings.Join(strings.Fields(c.chain), "")
		if !strings.Contains(flat, chain) {
			t.Errorf("%s 的无障碍状态没接在元素链上：找不到 %q", c.name, chain)
		}
	}
}

// TestPlayerBarA11yRolesNotSwapped 反证角色没接反：音量/倍速是弹层开关（Disclosure
// + Expanded），不是 Checked；均衡/恰饭是保持型切换（ToggleButton + Checked），
// 不是 Expanded。两种角色接反了屏幕阅读器会把「展开」读成「按下」或反之。
func TestPlayerBarA11yRolesNotSwapped(t *testing.T) {
	flat := flatten(t, "playerbar.go")

	// 均衡 / 恰饭键不得带 Expanded（它们不是弹层开关）。
	for _, key := range []string{`ui.ButtonBase(c.Key("eq"))`, `ui.ButtonBase(c.Key("sponsor"))`} {
		i := strings.Index(flat, strings.Join(strings.Fields(key), ""))
		if i < 0 {
			t.Fatalf("playerbar.go 里找不到 %s", key)
		}
		seg := flat[i:min(len(flat), i+320)]
		if strings.Contains(seg, ".Expanded(") {
			t.Errorf("%s 被误接 Expanded：它是 aria-pressed 的保持型切换，不是弹层开关", key)
		}
	}

	// 音量 / 倍速键不得带 Checked（它们不是保持型切换）。
	for _, key := range []string{`ui.ButtonBase(c.Key("volume"))`, `ui.ButtonBase(c.Key("speed"))`} {
		i := strings.Index(flat, strings.Join(strings.Fields(key), ""))
		if i < 0 {
			t.Fatalf("playerbar.go 里找不到 %s", key)
		}
		seg := flat[i:min(len(flat), i+320)]
		if strings.Contains(seg, ".Checked(") {
			t.Errorf("%s 被误接 Checked：它是 aria-expanded 的弹层开关，不是 aria-pressed", key)
		}
	}
}

// TestIncognitoSwitchA11yWired 钉住历史抽屉隐身开关的无障碍开关态。原版这颗键是
// `role="switch"` + `aria-checked={isIncognitoMode}`，读屏用户才知道开关是开是关。
// mygo 对应 RoleSwitch + Checked(a.Incognito)。此前原生版只画了图标/轨道/底色，
// 无障碍层上它是一颗「未按下」的普通按钮，与视觉状态脱节。
//
// 反证：角色必须是 RoleSwitch（不是 RoleToggleButton/普通按钮），否则读屏不会把
// Checked 报成开关的「开/关」。
func TestIncognitoSwitchA11yWired(t *testing.T) {
	flat := flatten(t, "drawer.go")
	chain := strings.Join(strings.Fields(
		`ui.ButtonBase(c.Key("incognito")).MinWidth(minW).Height(32).Padding(0, 8).Radius(Radius).Role(ui.RoleSwitch).Label("隐身").Tooltip(incognitoTip(a.Incognito)).Checked(a.Incognito)`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("隐身开关没接 Role(ui.RoleSwitch) + Checked(a.Incognito)（原版 role=\"switch\" aria-checked）")
	}

	// 反证：a.Incognito 同时驱动视觉与无障碍（filled 图标 / 轨道色 / Checked），
	// 不许用常量把状态写死。
	if strings.Contains(flat, "Checked(true)") || strings.Contains(flat, "Checked(false)") {
		t.Error("隐身开关的 Checked 被写死成常量，应读 a.Incognito")
	}
}
