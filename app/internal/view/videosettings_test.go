package view

// 视频窗内置设置面板（videosettings.go）的无头测试：
//
//   - 齿轮键开合：再点一次 / Esc / 点面板外都能关，Esc 只关面板不关窗口；
//   - 弹幕 5 项：点一下立刻落到 VideoChrome 的弹幕设置（同一帧就影响渲染
//     参数）并回传主窗落盘（OnDanmakuSettings / OnDanmakuToggle）；
//   - 播放 2 项：倍速 / 广告跳过读写的是**共享的 PlayerSettings**（面板改
//     主窗读到、主窗改面板读到）；音量均衡**不在**面板里（原生视频的 PCM 在
//     系统媒体框架内部，点了也不生效，主窗的均衡键与共享值不受影响）；
//   - 播放引擎：原生视频播放开关仍在面板里，翻转回传主窗；
//   - 面板打开时控件条不自动隐藏，关闭后恢复原来的 2.5s 静默淡出；
//   - 320×200 最小窗口：面板不撑破窗口、内部可滚、关闭键始终可达。
//
// 播放器与窗口都是纯逻辑桩（见 videochrome_test.go），不碰媒体框架。

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// settingsProbe 起一个无头视频窗：可指定尺寸，共享设置与弹幕设置都按默认值
// 注入（与 internal/video 的生产路径一致：Set 是主窗那份的同一个指针）。
func videoSettingsProbe(t *testing.T, w, h int) (*VideoChrome, *videoStubPlayer, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 200 * time.Second,
		pos: 30 * time.Second, rate: 1, vol: 1}
	win := newVideoStubWindow()
	set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1}
	set.Ensure()
	ch := &VideoChrome{
		Theme:           ThemeFor(PeriodAt(12)),
		Title:           "测试稿件 · 第一部分",
		Pl:              pl,
		Win:             win,
		Set:             set,
		Danmaku:         true,
		DanmakuSettings: danmaku.DefaultSettings(),
		NativeVideo:     true,
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, w, h)
	tt.Frame()
	return ch, pl, tt
}

// openVideoSettings 点顶栏齿轮键打开面板（失败即 Fatal）。
func openVideoSettings(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click("视频设置"); err != nil {
		t.Fatalf("点不到顶栏设置键：%v", err)
	}
	tt.Frame()
	if _, ok := tt.Find("关闭设置"); !ok {
		t.Fatalf("设置面板没有出现（现有文字：%v）", tt.Texts())
	}
}

// clickVideoSetting 点面板里的一项并推进一帧。
func clickVideoSetting(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := tt.Click(label); err != nil {
		t.Fatalf("点不到 %q：%v", label, err)
	}
	tt.Frame()
}

// TestVideoSettingsOpenClose 钉住开合的四条路：齿轮（开 / 再点关）、Esc、
// 点面板外。Esc 只关面板 —— 视频窗（与播放）不能被这一下带走。
func TestVideoSettingsOpenClose(t *testing.T) {
	ch, pl, tt := videoSettingsProbe(t, 640, 360)
	closed := false
	ch.OnClose = func() { closed = true }

	// 齿轮：开。
	openVideoSettings(t, tt)
	if !ch.showSettings {
		t.Fatal("点齿轮后面板状态应为开")
	}
	if _, ok := tt.Find("视频设置：弹幕总开关"); !ok {
		t.Fatalf("面板里看不到弹幕总开关：%v", tt.Texts())
	}
	// 齿轮：再点一次关。
	clickVideoSetting(t, tt, "视频设置")
	if ch.showSettings {
		t.Fatal("再点齿轮面板没有关")
	}
	if _, ok := tt.Find("关闭设置"); ok {
		t.Fatal("面板关了但内容还在画")
	}

	// Esc：开→关，且**不**关窗口。
	openVideoSettings(t, tt)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ch.showSettings {
		t.Fatal("Esc 没有关掉面板")
	}
	if closed {
		t.Fatal("Esc 关面板时把整个视频窗也关了（应在面板开着时先关面板）")
	}
	if !pl.playing {
		t.Error("关面板不该动播放状态")
	}

	// 点面板外：开→关（点画面右侧，避开标题栏、控件条与面板本身）。
	openVideoSettings(t, tt)
	tt.ClickAt(600, 300)
	tt.Frame()
	if ch.showSettings {
		t.Fatal("点面板外没有关掉面板")
	}
	if closed {
		t.Error("点面板外不该关窗口")
	}
}

// TestVideoSettingsEscInFullScreen 钉住全屏下的 Esc 分层：面板开着时第一下
// 只关面板（既不退全屏也不关窗），之后才是原来的「先退全屏、再关窗」。
func TestVideoSettingsEscInFullScreen(t *testing.T) {
	ch, _, tt := videoSettingsProbe(t, 1280, 720)
	closed := false
	ch.OnClose = func() { closed = true }
	tt.Key(0, ui.KeyF)
	tt.Frame()
	if !ch.fs.full {
		t.Fatal("F 没有进入全屏")
	}
	openVideoSettings(t, tt)
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ch.showSettings {
		t.Fatal("全屏下 Esc 没有先关面板")
	}
	if !ch.fs.full || closed {
		t.Fatalf("关面板那一下动了全屏/窗口：full=%v closed=%v", ch.fs.full, closed)
	}
	// 第二下：退全屏（不关窗）。
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ch.fs.full || closed {
		t.Fatalf("第二下 Esc 应退出全屏：full=%v closed=%v", ch.fs.full, closed)
	}
	// 第三下：窗口态 Esc，关窗。
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !closed {
		t.Error("第三下 Esc 应关视频窗")
	}
}

// TestVideoSettingsDanmakuItems 钉住弹幕那 5 项「点一下立即生效 + 回传主窗」：
//
//   - 总开关翻 VideoChrome.Danmaku 并走 OnDanmakuToggle（主窗落盘 + 同步）；
//   - 其余 4 项改 VideoChrome.DanmakuSettings 并走 OnDanmakuSettings；
//   - 落盘之后渲染参数真的变了（渲染点为 0 / 同屏上限收紧）。
//
// 窗口放大到 700×900：面板要能一次展开全部行（小窗口的滚动另有测试）。
func TestVideoSettingsDanmakuItems(t *testing.T) {
	// 200 条密集弹幕：上限收到 30 之后同屏条数必须跟着降。
	items := make([]danmaku.Item, 0, 200)
	for i := 0; i < 200; i++ {
		items = append(items, danmaku.Item{Time: 29.5 + float64(i)*0.01, Text: "密集弹幕"})
	}
	ch, _, tt := videoSettingsProbe(t, 700, 900)
	o := &danmaku.Overlay{}
	o.SetItems(items)
	ch.DanmakuOverlay = o
	tt.Frame()

	var toggles []bool
	var styles []danmaku.Settings
	ch.OnDanmakuToggle = func(on bool) { toggles = append(toggles, on) }
	ch.OnDanmakuSettings = func(s danmaku.Settings) { styles = append(styles, s) }

	if o.Active() > ch.DanmakuSettings.Max {
		t.Fatalf("同屏 %d 条已超过默认上限 %d", o.Active(), ch.DanmakuSettings.Max)
	}
	openVideoSettings(t, tt)

	clickVideoSetting(t, tt, "视频设置：不透明度 50%")
	if got := ch.DanmakuSettings.Opacity; got != 0.5 {
		t.Errorf("不透明度 = %v，想要 0.5", got)
	}
	clickVideoSetting(t, tt, "视频设置：字号 大")
	if got := ch.DanmakuSettings.Size; got != 2 {
		t.Errorf("字号档 = %d，想要 2（大）", got)
	}
	clickVideoSetting(t, tt, "视频设置：显示区域 上方 1/2")
	if got := ch.DanmakuSettings.Area; got != 0.5 {
		t.Errorf("显示区域 = %v，想要 0.5", got)
	}
	clickVideoSetting(t, tt, "视频设置：同屏上限 30")
	if got := ch.DanmakuSettings.Max; got != danmaku.MaxOnScreen(0) {
		t.Errorf("同屏上限 = %d，想要 30", got)
	}
	// 「立即生效」：同一帧之后重排，同屏条数落在新上限以内。
	if o.Active() > ch.DanmakuSettings.Max {
		t.Errorf("上限收到 %d 之后同屏仍有 %d 条", ch.DanmakuSettings.Max, o.Active())
	}
	if ch.DanmakuDrawn() > ch.DanmakuSettings.Max {
		t.Errorf("渲染点 %d 超过新上限 %d", ch.DanmakuDrawn(), ch.DanmakuSettings.Max)
	}

	if len(styles) != 4 {
		t.Fatalf("主窗收到 %d 次弹幕设置更新，想要 4（每次点击一次）", len(styles))
	}
	last := styles[len(styles)-1]
	if !last.On || last.Opacity != 0.5 || last.Size != 2 || last.Area != 0.5 ||
		last.Max != danmaku.MaxOnScreen(0) {
		t.Errorf("最后一次回传的设置不对：%+v", last)
	}
	if !ch.showSettings {
		t.Error("调弹幕设置不该收起面板")
	}

	// 总开关：关掉之后一个字都不画。
	clickVideoSetting(t, tt, "视频设置：弹幕总开关")
	if ch.Danmaku {
		t.Error("弹幕总开关没有关掉")
	}
	if len(toggles) != 1 || toggles[0] {
		t.Errorf("总开关应回传一次 false，实际 %v", toggles)
	}
	if ch.DanmakuDrawn() != 0 || o.Active() != 0 {
		t.Errorf("关掉后仍有弹幕：drawn=%d active=%d", ch.DanmakuDrawn(), o.Active())
	}
	// 再点回来：恢复渲染。
	clickVideoSetting(t, tt, "视频设置：弹幕总开关")
	if !ch.Danmaku || ch.DanmakuDrawn() == 0 {
		t.Errorf("重新打开后没有恢复渲染：on=%v drawn=%d", ch.Danmaku, ch.DanmakuDrawn())
	}
}

// TestVideoSettingsSharedPlayback 钉住「播放 3 项与主窗同一份状态、双向同步」：
//
//   - 面板 → 主窗：点档位 / 开关后共享的 PlayerSettings 立刻是新值（生产路径
//     里它就是主窗 &App.PlayerSettings 的同一个指针），并已下发到播放器；
//   - 主窗 → 面板：主窗那边改同一个指针，面板下一帧就显示新状态（Checked）。
func TestVideoSettingsSharedPlayback(t *testing.T) {
	ch, pl, tt := videoSettingsProbe(t, 700, 900)
	set := ch.Set // 主窗持的是同一个指针
	openVideoSettings(t, tt)

	// ---- 面板 → 主窗
	clickVideoSetting(t, tt, "视频设置：倍速 1.5x")
	if set.Speed != 1.5 {
		t.Errorf("共享倍速 = %v，想要 1.5（主窗应读到同一个值）", set.Speed)
	}
	if pl.rate != 1.5 {
		t.Errorf("播放器倍速 = %v，想要 1.5", pl.rate)
	}
	// 音量均衡：视频窗的声音不经过本仓库的 DSP 管线，点了也不生效 —— 面板里
	// **没有这一项**（不是禁用行，是不画）。共享 EQ 值不因视频窗的改动而变。
	if _, ok := tt.Find("视频设置：音量均衡"); ok {
		t.Errorf("面板里仍有「音量均衡」项：%v", tt.Texts())
	}
	if tt.HasText("音量均衡") {
		t.Errorf("面板里仍出现「音量均衡」文案：%v", tt.Texts())
	}
	if set.EQ {
		t.Error("视频设置面板不该改共享 EQ")
	}
	// 播放组剩下的两项保持原样（倍速在上面点过、广告跳过在这里点）。
	clickVideoSetting(t, tt, "视频设置：广告跳过")
	if !set.Sponsor {
		t.Error("广告跳过没有打开（共享设置）")
	}
	// 面板里的按压态 / 选中态跟着共享状态走（音量均衡不在其中：它没有
	// 可按压的开关，主窗当前值以 tooltip 文字报出）。
	for label, want := range map[string]bool{
		"视频设置：广告跳过":    true,
		"视频设置：倍速 1.5x": true,
	} {
		if info, ok := tt.AccessByRole(ui.RoleSwitch, label); ok && info.Checked != want {
			t.Errorf("%q 的按压态 = %v，想要 %v", label, info.Checked, want)
		}
		if info, ok := tt.AccessByRole(ui.RoleToggleButton, label); ok && info.Checked != want {
			t.Errorf("%q 的按压态 = %v，想要 %v", label, info.Checked, want)
		}
	}

	// ---- 主窗 → 面板
	set.SetSpeed(2)
	set.SetEQ(true)
	set.SetSponsor(false)
	tt.Frame()
	if pl.rate != 2 {
		t.Errorf("主窗改倍速后播放器 = %v，想要 2", pl.rate)
	}
	if info, ok := tt.AccessByRole(ui.RoleToggleButton, "视频设置：倍速 2.0x"); !ok || !info.Checked {
		t.Errorf("主窗改成 2x 后面板没有选中 2.0x（ok=%v info=%+v）", ok, info)
	}
	if info, ok := tt.AccessByRole(ui.RoleToggleButton, "视频设置：倍速 1.5x"); !ok || info.Checked {
		t.Errorf("主窗改成 2x 后面板仍勾着 1.5x（ok=%v）", ok)
	}
	if info, ok := tt.AccessByRole(ui.RoleSwitch, "视频设置：广告跳过"); !ok || info.Checked {
		t.Errorf("主窗关掉「视频设置：广告跳过」后面板的按压态没跟上（ok=%v info=%+v）", ok, info)
	}
	// 音量均衡：主窗打开 / 关闭时，视频窗的面板既不出现这一项、也不把共享
	// EQ 画成这个窗口的「开」—— 共享值本身仍由主窗照常读写。
	if set.EQ != true {
		t.Fatal("主窗打开音量均衡后共享 EQ 没变")
	}
	if _, ok := tt.Find("视频设置：音量均衡"); ok {
		t.Error("主窗打开音量均衡后，视频窗面板冒出了这一项")
	}
	set.SetEQ(false)
}

// TestVideoSettingsNativeToggle 钉住「原生视频播放」仍然可选，且入口在面板里：
// 翻转本地状态并回传主窗（主窗落盘 nativeVideoPlayer + Manager 生效）。
func TestVideoSettingsNativeToggle(t *testing.T) {
	ch, _, tt := videoSettingsProbe(t, 700, 900)
	var got []bool
	ch.OnNativeToggle = func(on bool) { got = append(got, on) }
	openVideoSettings(t, tt)

	if _, ok := tt.Find("视频设置：原生视频播放"); !ok {
		t.Fatalf("面板里找不到「原生视频播放」开关：%v", tt.Texts())
	}
	// 旁边必须写清「不可用时会先问」这条回退语义（上一轮的询问式回退）。
	if !tt.HasText("起播会先询问") {
		t.Error("「原生视频播放」旁边缺少「不可用时起播会询问」的说明")
	}
	clickVideoSetting(t, tt, "视频设置：原生视频播放")
	if ch.NativeVideo {
		t.Error("点一下后本地开关应为关")
	}
	if len(got) != 1 || got[0] {
		t.Fatalf("回传应是一次 false，实际 %v", got)
	}
	clickVideoSetting(t, tt, "视频设置：原生视频播放")
	if !ch.NativeVideo || len(got) != 2 || !got[1] {
		t.Fatalf("再点一次应回到开：native=%v got=%v", ch.NativeVideo, got)
	}
}

// TestVideoSettingsKeepsChrome 钉住「面板打开期间控件条不自动隐藏，关闭后
// 恢复原有自动隐藏」。
//
// 分两段：第一段走真实点击（齿轮会拿到键盘焦点，而标题栏的 hold 条件本来
// 就含 FocusWithin，所以这一段只钉「开着时确实不藏」）；第二段把面板直接
// 打开（没有焦点、指针停在画面上），这时控件条能否淡出只由 showSettings 决定
// —— 开着不藏、关掉照旧 2.5s 后收，两个方向都钉死。
func TestVideoSettingsKeepsChrome(t *testing.T) {
	ch, _, tt := videoSettingsProbe(t, 900, 600)
	openVideoSettings(t, tt)
	// 面板按锚点摆平后靠右；左边这片只有画面。
	tt.Move(4, 300)
	tt.Frame()
	tt.Advance(5 * time.Second)
	if ch.idle.alpha != 1 {
		t.Fatalf("面板开着时控件条淡出了（alpha=%v）", ch.idle.alpha)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ch.showSettings {
		t.Fatal("Esc 没有关面板")
	}

	// 第二段：面板直接打开（不经过点击 → 标题栏没有焦点），指针停在画面上。
	ch2, _, tt2 := videoSettingsProbe(t, 900, 600)
	ch2.showSettings = true
	tt2.Move(4, 300)
	tt2.Frame()
	if ch2.barHover {
		t.Fatalf("指针不在两条栏上却算作悬停（alpha=%v）", ch2.idle.alpha)
	}
	tt2.Advance(5 * time.Second)
	if ch2.idle.alpha != 1 {
		t.Fatalf("面板开着时控件条淡出了（alpha=%v）", ch2.idle.alpha)
	}
	// 关掉面板：同一台状态机、同一个指针位置，恢复 2.5s + 220ms 的淡出。
	ch2.showSettings = false
	tt2.Frame()
	tt2.Advance(5 * time.Second)
	if ch2.idle.alpha != 0 {
		t.Errorf("关面板后控件条没有恢复自动隐藏（alpha=%v）", ch2.idle.alpha)
	}
}

// TestVideoSettingsSmallWindow 钉住 320×200 最小窗口：面板整体在窗口内
// （不撑破、不裁掉关闭键），内容超出时在面板内部滚动。
func TestVideoSettingsSmallWindow(t *testing.T) {
	const w, h = 320, 200
	ch, _, tt := videoSettingsProbe(t, w, h)
	openVideoSettings(t, tt)
	// 面板整体（role=popup）必须在窗口内：既不撑破窗口，也不把它推出边界。
	panel, ok := settingsPanelBounds(tt)
	if !ok {
		t.Fatalf("找不到设置面板的无障碍节点（现有：%v）", tt.Texts())
	}
	if panel.X < 0 || panel.Y < 0 || panel.X+panel.W > w+0.5 || panel.Y+panel.H > h+0.5 {
		t.Errorf("面板超出 %d×%d 窗口：%+v", w, h, panel)
	}
	if panel.H > videoSettingsMaxH(float32(h))+0.5 {
		t.Errorf("面板高 %v 超过上限 %v", panel.H, videoSettingsMaxH(float32(h)))
	}
	inside := func(name string) ui.Rect {
		t.Helper()
		r, ok := tt.Find(name)
		if !ok {
			t.Fatalf("小窗口里看不到 %q：%v", name, tt.Texts())
		}
		if r.X < 0 || r.Y < 0 || r.X+r.W > w+0.5 || r.Y+r.H > h+0.5 {
			t.Errorf("小窗口里 %q 超出窗口：%+v", name, r)
		}
		return r
	}
	// 关闭键在固定头部：面板再矮也够得到。
	closeRect := inside("关闭设置")
	if closeRect.Y+closeRect.H > h {
		t.Errorf("关闭键被挤到窗口外：%+v", closeRect)
	}
	first := inside("视频设置：弹幕总开关")

	// 内部滚动：滚轮往下，第一行跟着上移（面板本身不动）。
	tt.Wheel(float32(w)/2, float32(h)/2+20, 0, 60)
	tt.Advance(300 * time.Millisecond)
	if after, ok := tt.Find("视频设置：弹幕总开关"); ok && after.Y >= first.Y {
		t.Errorf("滚轮没有滚动面板内容：%v → %v", first.Y, after.Y)
	}
	// 滚动之后关闭键仍在原处（不在滚动区里）。
	if again, ok := tt.Find("关闭设置"); !ok || again.Y != closeRect.Y {
		t.Errorf("滚动把关闭键带走了：%v → %v（ok=%v）", closeRect, again, ok)
	}

	// 能关：点面板的关闭键。
	if err := tt.Click("关闭设置"); err != nil {
		t.Fatalf("点关闭键失败：%v", err)
	}
	tt.Frame()
	if ch.showSettings {
		t.Error("点面板关闭键后仍开着")
	}
	if _, ok := tt.Find("视频设置：弹幕总开关"); ok {
		t.Error("面板关了但内容还在画")
	}
}

// settingsPanelBounds 找面板那一块（PopoverBase 给面板的 RolePopup）的矩形。
func settingsPanelBounds(tt *ui.Tester) (ui.Rect, bool) {
	for _, info := range tt.AccessAll() {
		if info.Role == ui.RolePopup {
			return info.Bounds, true
		}
	}
	return ui.Rect{}, false
}
