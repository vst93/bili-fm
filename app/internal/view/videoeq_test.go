package view

// 视频窗去掉「音量均衡」入口的回归断言（用户 2026-11 裁决：「先不动吧，
// 视频窗口去掉 音量均衡 按钮。」）。
//
// 背景（已查清，不再论证）：EQ 挂在 Go 的 PCM 管线（internal/media/eq.go），
// 原生视频的 PCM 由系统媒体框架解码后直接送去声卡，ui/video 没有任何音频
// tap 接口 —— 视频窗里那颗键永远不生效。此前的做法是「置灰 + 不适用说明」，
// 但一个点了也不会生效的入口仍然是死入口，所以这一轮直接删掉：
//
//  1. 控件条：视频窗**没有**均衡键（不是置灰），控件集合 =
//     主窗控件集合 + 全屏键 − 均衡键；
//  2. 设置面板：播放组里**没有**「音量均衡」这一项，剩下的倍速 / 广告跳过
//     保持原样；
//  3. 共享的 PlayerSettings 与主窗的均衡键一律不动：主窗仍可切换 EQ，
//     主窗改音量 / 倍速仍与视频窗双向同步（同一个指针 + OnChange 监听）。
//
// 反证：把 ShowEQ 给视频窗打开、或把均衡行加回 settingsBody，对应断言立即
// 变红；把共享同步改成视频窗自己存一份，最后一条变红。

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestVideoEQKeyGone 钉住「视频窗控件条里没有均衡键」：不是禁用，是根本
// 不在无障碍树里，也没有「音量均衡」文案。
func TestVideoEQKeyGone(t *testing.T) {
	set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, EQ: true}
	set.Ensure()
	_, _, tt := playerBindVideo(t, set)
	tt.Frame()

	for _, label := range []string{eqTip(false), eqTip(true)} {
		if info, ok := tt.AccessByRole(ui.RoleToggleButton, label); ok {
			t.Errorf("视频窗仍画着均衡键（%q）：%+v", label, info)
		}
	}
	if tt.HasText("音量均衡") {
		t.Errorf("视频窗控件条仍出现「音量均衡」文案：%v", tt.Texts())
	}
	// 共享值没有被视频窗的渲染改写。
	if !set.EQ {
		t.Error("渲染视频窗把共享 EQ 改掉了")
	}
}

// TestVideoBarEqualsMainPlusFullscreenMinusEQ 是交付 A 的总断言：
// 视频窗控件集合 == 主窗控件集合 + 全屏键 − 均衡键。
//
// 这里不复用 videobar_sync_test 的像素采样，只按角色 + 文案比较集合，
// 这样「删掉均衡」与「多出全屏」两件事同时被钉住。
func TestVideoBarEqualsMainPlusFullscreenMinusEQ(t *testing.T) {
	a, mainTT := playerBindApp(t)
	_, _, vidTT := playerBindVideo(t, &a.PlayerSettings)
	mainTT.Frame()
	vidTT.Frame()

	mainKeys := videoKeysIn(mainTT, float32(700-PlayerBarHeight), 700)
	vidKeys := videoKeysIn(vidTT, videoBarYFor(900), 500)

	mainNames := keyNames(mainKeys)
	vidNames := keyNames(vidKeys)
	var want []string
	removed := 0
	for _, n := range mainNames {
		if n == eqTip(false) || n == eqTip(true) {
			removed++
			continue
		}
		want = append(want, n)
	}
	if removed != 1 {
		t.Fatalf("主窗控件集合里应有且只有一颗均衡键，实际 %d：%v", removed, mainNames)
	}
	want = append(want, "全屏")
	if strings.Join(vidNames, " | ") != strings.Join(want, " | ") {
		t.Fatalf("视频窗控件集合 != 主窗 + 全屏 − 均衡：\n 视频 = %v\n 想要 = %v", vidNames, want)
	}
	// 再明确一次：视频窗里没有任何名字像均衡的键。
	for _, n := range vidNames {
		if n == eqTip(false) || n == eqTip(true) || strings.Contains(n, "均衡") {
			t.Errorf("视频窗出现了均衡键 %q", n)
		}
	}
}

// TestMainEQKeyStillWorks 是上一条的反证：主窗仍有均衡键，且点击真的切换
// 共享的 PlayerSettings.EQ（视频窗的删改没有把主窗的入口一起带走）。
func TestMainEQKeyStillWorks(t *testing.T) {
	a, mainTT := playerBindApp(t)
	if a.EQ {
		t.Fatal("初始 EQ 应为关")
	}
	if info, ok := mainTT.AccessByRole(ui.RoleToggleButton, eqTip(false)); !ok {
		t.Fatalf("主窗找不到均衡键：%v", mainTT.Texts())
	} else if info.Checked {
		t.Error("初始状态均衡键不应按下")
	}
	if err := mainTT.Click(eqTip(false)); err != nil {
		t.Fatalf("点主窗均衡键失败：%v", err)
	}
	mainTT.Frame()
	if !a.EQ {
		t.Fatal("主窗点均衡键没有打开共享 EQ")
	}
	if info, ok := mainTT.AccessByRole(ui.RoleToggleButton, eqTip(true)); !ok || !info.Checked {
		t.Errorf("主窗均衡键没有变成按下态：ok=%v info=%+v", ok, info)
	}
	if err := mainTT.Click(eqTip(true)); err != nil {
		t.Fatalf("再点主窗均衡键失败：%v", err)
	}
	mainTT.Frame()
	if a.EQ {
		t.Error("主窗再点均衡键没有关闭共享 EQ")
	}
}

// TestVideoSettingsNoEQLine 钉住设置面板的枚举里没有均衡项，且播放组剩下
// 的倍速 / 广告跳过还在（不是把整组删了）。
func TestVideoSettingsNoEQLine(t *testing.T) {
	ch, _, tt := videoSettingsProbe(t, 700, 900)
	openVideoSettings(t, tt)

	if _, ok := tt.Find("视频设置：音量均衡"); ok {
		t.Errorf("视频设置面板里仍有「音量均衡」项：%v", tt.Texts())
	}
	if tt.HasText("音量均衡") {
		t.Errorf("视频设置面板里仍出现「音量均衡」文案：%v", tt.Texts())
	}
	for _, want := range []string{"视频设置：倍速 1.0x", "视频设置：广告跳过"} {
		if _, ok := tt.Find(want); !ok {
			t.Errorf("播放组少了 %q（剩下的项应保持原样）：%v", want, tt.Texts())
		}
	}
	// 面板里也没有「不适用」这种残留说明（那是上一版禁用行的措辞）。
	if tt.HasText("不适用") {
		t.Errorf("设置面板里残留「不适用」文案：%v", tt.Texts())
	}
	// 面板打开 / 关闭不影响共享 EQ 值。
	if ch.settings().EQ {
		t.Error("打开设置面板不该改共享 EQ")
	}
}

// TestSharedVolumeSpeedStillSyncAfterEQRemoval 是第三条断言：删掉视频窗的
// 均衡入口后，共享值双向同步逻辑不因这个改动断裂 —— 主窗改音量 / 倍速，
// 视频窗下一帧读到同一个值并下发到自己的播放器；反向也一样。
func TestSharedVolumeSpeedStillSyncAfterEQRemoval(t *testing.T) {
	a, mainTT := playerBindApp(t)
	ch, pl, vidTT := playerBindVideo(t, &a.PlayerSettings)
	if ch.settings() != &a.PlayerSettings {
		t.Fatal("视频窗拿到的不是主窗那份 PlayerSettings")
	}

	// 主窗 → 视频窗：音量。
	mainTT.Frame()
	if err := mainTT.Click("音量"); err != nil {
		t.Fatalf("主窗音量键失败：%v", err)
	}
	mainTT.Frame()
	r, ok := mainTT.Find("音量滑块")
	if !ok {
		t.Fatalf("主窗音量弹层里找不到滑杆：%v", mainTT.Texts())
	}
	mainTT.ClickAt(r.X+r.W*0.6, r.Y+r.H/2)
	mainTT.Frame()
	if a.Volume <= 0 || a.Muted {
		t.Fatalf("主窗拖音量没生效：Volume %v / Muted %v", a.Volume, a.Muted)
	}
	vidTT.Frame()
	if want := a.Volume; pl.vol != want || pl.muted {
		t.Errorf("主窗音量 %v 没同步到视频：vol=%v muted=%v", want, pl.vol, pl.muted)
	}

	// 主窗 → 视频窗：倍速。
	a.SetSpeed(1.5)
	vidTT.Frame()
	if pl.rate != 1.5 {
		t.Errorf("主窗倍速 1.5 没同步到视频：rate=%v", pl.rate)
	}
	if _, ok := vidTT.Find("1.5x"); !ok {
		t.Errorf("视频窗倍速键没有显示 1.5x：%v", vidTT.Texts())
	}

	// 视频窗 → 主窗：音量（静音键）与倍速（弹层）。
	if err := vidTT.Click("音量"); err != nil {
		t.Fatalf("视频音量键失败：%v", err)
	}
	vidTT.Frame()
	if err := vidTT.Click("静音"); err != nil {
		t.Fatalf("视频静音键失败：%v", err)
	}
	vidTT.Frame()
	if a.Volume != 0 || !a.Muted {
		t.Errorf("视频窗静音后主窗 = Volume %v / Muted %v", a.Volume, a.Muted)
	}
	if !pl.muted || pl.vol != 0 {
		t.Errorf("视频窗静音没有落到自己的播放器：muted=%v vol=%v", pl.muted, pl.vol)
	}
	if err := vidTT.Click("1.5x"); err != nil {
		t.Fatalf("视频倍速键失败：%v", err)
	}
	vidTT.Frame()
	if err := vidTT.Click("2.0x"); err != nil {
		t.Fatalf("视频选 2.0x 失败：%v", err)
	}
	vidTT.Frame()
	if a.Speed != 2.0 {
		t.Errorf("视频窗选 2.0x 后主窗 Speed = %v", a.Speed)
	}
	if pl.rate != 2.0 {
		t.Errorf("视频窗选 2.0x 后自己的播放器 rate = %v", pl.rate)
	}
}
