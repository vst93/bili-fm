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

// TestCoverToggleA11yStatesWired 钉住封面右下角小开关的无障碍按压态。原版
// videoCover.tsx 只给「封面背景 / 高级质感」两颗键写了
// `aria-pressed={ambientBackgroundEnabled}` / `aria-pressed={premiumTexture}`；
// 模式切换键（Refresh）没有 aria-pressed，是纯动作键。mygo 对应
// RoleToggleButton + Checked(active)，且角色与状态必须在同一条元素链上。
func TestCoverToggleA11yStatesWired(t *testing.T) {
	flat := flatten(t, "home.go")
	chain := strings.Join(strings.Fields(
		`if coverToggleStateful(key) { b.Role(ui.RoleToggleButton).Checked(active) }`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("封面小开关没按状态接 Role(ui.RoleToggleButton) + Checked(active)")
	}
	// 状态读的是 coverToggleActive（= a.Ambient / a.Premium），不是写死的常量。
	if !strings.Contains(flat, `active:=coverToggleActive(key,a.Ambient,a.Premium)`) {
		t.Error("封面小开关的 active 没读 coverToggleActive(a.Ambient, a.Premium)")
	}
}

// TestCoverToggleStateful 反证「只有封面背景 / 高级质感是保持型开关」：模式切换键
// 不能被写成 toggle（原版它没有 aria-pressed），否则读屏会把它当成有开/关状态的
// 切换键。
func TestCoverToggleStateful(t *testing.T) {
	for _, key := range []string{"ambient", "premium"} {
		if !coverToggleStateful(key) {
			t.Errorf("%q 应是保持型开关（原版有 aria-pressed）", key)
		}
	}
	if coverToggleStateful("cover-mode") {
		t.Error("模式切换键不应是保持型开关（原版没有 aria-pressed）")
	}
}

// TestFavButtonA11yWired 钉住收藏键的无障碍按压态。原版 videoInfo.tsx 只有收藏键写
// 了 `aria-pressed={isFavorite}`（点赞 / 投币没有），mygo 对应
// RoleToggleButton + Checked(spec.Active)，角色与状态必须同一条链。
func TestFavButtonA11yWired(t *testing.T) {
	flat := flatten(t, "home.go")
	chain := strings.Join(strings.Fields(
		`if spec.Toggle { b.Role(ui.RoleToggleButton).Checked(spec.Active) }`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("互动按钮没按 Toggle 接 Role(ui.RoleToggleButton) + Checked(spec.Active)")
	}
	// 只有收藏键把 Toggle 置真。
	if !strings.Contains(flat, `Toggle:true,`) {
		t.Error("收藏键没设 Toggle: true（原版 aria-pressed={isFavorite}）")
	}
	if n := strings.Count(flat, "Toggle:true,"); n != 1 {
		t.Errorf("statSpec 里 Toggle: true 出现 %d 次，应只在收藏键一处", n)
	}
}

// TestMiniPinA11yWired 钉住迷你窗「窗口置顶」键的无障碍按压态。原版
// miniVideoInfo.tsx 的置顶键有 `aria-pressed={isPinned}`（切回窗口模式的键没有），
// mygo 对应 RoleToggleButton + Checked(active)，角色与状态同一条链。
func TestMiniPinA11yWired(t *testing.T) {
	flat := flatten(t, "mini.go")
	chain := strings.Join(strings.Fields(
		`if toggle { b.Role(ui.RoleToggleButton).Checked(active) }`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("迷你窗控制键没按 toggle 接 Role(ui.RoleToggleButton) + Checked(active)")
	}
	// 置顶键把 toggle 置真、状态读 a.Pinned；切回窗口模式的键 toggle=false。
	// miniControl 的签名是 (…, active, toggle, pending bool, fn func())，
	// 两颗键都把 a.WindowControlPending 当 pending 传（原版
	// `disabled={isWindowControlPending}`）。
	pin := strings.Join(strings.Fields(
		`miniPinIcon(a.Pinned), a.Pinned, true, a.WindowControlPending, func() {`), "")
	if !strings.Contains(flat, pin) {
		t.Error("置顶键没把 a.Pinned / toggle=true / pending 传给 miniControl")
	}
	restore := strings.Join(strings.Fields(
		`a.miniControl(c, "mini-restore", "切换到窗口模式", iconRestore, false, false, a.WindowControlPending, func() {`), "")
	if !strings.Contains(flat, restore) {
		t.Error("切回窗口模式的键不应是保持型开关（原版没有 aria-pressed）")
	}
}

// TestSettingsMenuA11yWired 钉住标题栏「设置」键的无障碍展开态。原版 titleBar.tsx 的
// #settings-entry 是 `aria-haspopup="menu" aria-expanded={showMenu}` 的 button，
// mygo 对应 RoleMenuButton + Expanded(a.ShowSettings)。
func TestSettingsMenuA11yWired(t *testing.T) {
	flat := flatten(t, "shell.go")
	chain := strings.Join(strings.Fields(
		`b.Role(ui.RoleMenuButton).Expanded(a.ShowSettings)`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("设置键没接 Role(ui.RoleMenuButton) + Expanded(a.ShowSettings)")
	}
}

// flattenNoComments 读源码并「剥注释 + 压空白」。flatten 会把换行折掉，
// 若注释里恰好出现待匹配的代码片段就会误判，所以跨行的契约断言用它。
func flattenNoComments(t *testing.T, path string) string {
	t.Helper()
	return strings.Join(strings.Fields(stripGoComments(flattenRaw(t, path))), "")
}

func flattenRaw(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", path, err)
	}
	return string(src)
}

// stripGoComments 删掉 Go 的行注释（// 到行尾）与块注释（/* */）。
func stripGoComments(src string) string {
	var out []byte
	for i := 0; i < len(src); {
		switch {
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i += 2
		default:
			out = append(out, src[i])
			i++
		}
	}
	return string(out)
}

// srcHas 是压空白后的子串判定。
func srcHas(haystack, needle string) bool {
	return needle != "" && strings.Contains(haystack, needle)
}
