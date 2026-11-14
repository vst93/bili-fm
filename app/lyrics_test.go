package main

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

func TestLyricLines(t *testing.T) {
	cues := []view.SubtitleCue{
		{From: 0, To: 1, Content: "第一句"},
		{From: 2, To: 3, Content: "第二句"},
	}
	cases := []struct {
		pos       float64
		cur, next string
	}{
		{-1, "", "第一句"},
		{0.5, "第一句", "第二句"},
		{1.5, "第一句", "第二句"}, // 句间空隙保留上一句
		{2.5, "第二句", ""},
	}
	for _, c := range cases {
		cur, next := lyricLines(cues, c.pos)
		if cur != c.cur || next != c.next {
			t.Errorf("pos=%v: got (%q,%q), want (%q,%q)", c.pos, cur, next, c.cur, c.next)
		}
	}
}

func TestBuildDanmakuHeat(t *testing.T) {
	items := []danmaku.Item{
		{Time: 0.1}, {Time: 1}, {Time: 2},
		{Time: 11}, {Time: 12},
	}
	pts := buildDanmakuHeat(items)
	if len(pts) != 2 {
		t.Fatalf("heat points = %d, want 2: %#v", len(pts), pts)
	}
	if pts[0].At != 0 || pts[0].Value != 1 {
		t.Errorf("首桶 = %#v, want At=0 Value=1", pts[0])
	}
	if pts[1].At != 10 || pts[1].Value <= 0 {
		t.Errorf("第二个桶 = %#v, want At=10 Value>0", pts[1])
	}
}

func TestLyricsConfigPlacementNearBottom(t *testing.T) {
	// 1080 高工作区，预览窗在 y=900（底边 1000，只剩 80px）；配置页要多
	// 330px，下方放不下、上方够，所以应向上展开，预览停在原来的 y=900。
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	bounds := mygo.Rectangle{X: 680, Y: 900, Width: 560, Height: 100}
	above, y := lyricsConfigPlacement(bounds, work, 100, 430, 8)
	if !above {
		t.Fatal("贴近屏幕底部时设置页应向上展开")
	}
	if y != 570 {
		t.Fatalf("向上展开后的窗口上缘 = %d，应为 570", y)
	}
}

func TestLyricsConfigPlacementFitsBelow(t *testing.T) {
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
	bounds := mygo.Rectangle{X: 680, Y: 100, Width: 560, Height: 100}
	above, y := lyricsConfigPlacement(bounds, work, 100, 430, 8)
	if above {
		t.Fatal("下方空间足够时不应向上展开")
	}
	if y != 100 {
		t.Fatalf("下方展开不应移动窗口，上缘 = %d", y)
	}
}

func TestLyricsConfigPlacementClampsIntoScreen(t *testing.T) {
	// 上下都不够时仍向下，但窗口上缘要夹回工作区。
	work := mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 500}
	bounds := mygo.Rectangle{X: 0, Y: 80, Width: 560, Height: 100}
	above, y := lyricsConfigPlacement(bounds, work, 100, 430, 8)
	if above {
		t.Fatal("上下都放不下时不应选择向上（会把顶部切掉）")
	}
	if y < 8 || y+430 > 500-8 {
		t.Fatalf("窗口未夹进工作区：y=%d, bottom=%d", y, y+430)
	}
}

func TestResetLyricsOffsetOnTrackChange(t *testing.T) {
	a := &view.App{LyricsOffset: 1.5}
	c := &controller{app: a}
	c.resetLyricsOffsetOnTrackChange()
	if a.LyricsOffset != 0 {
		t.Fatalf("未勾选记住时切歌应归零，得到 %.1f", a.LyricsOffset)
	}
	a.LyricsOffset = 1.5
	a.LyricsOffsetRemember = true
	c.resetLyricsOffsetOnTrackChange()
	if a.LyricsOffset != 1.5 {
		t.Fatalf("勾选记住后切歌不应归零，得到 %.1f", a.LyricsOffset)
	}
}

func TestLoadLyricsOffsetOnlyWhenRemembered(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	_ = kv.SetString(prefLyricsOffset, "1.5")
	_ = kv.SetString(prefLyricsOffsetRemember, "false")
	c := &controller{app: view.NewApp(func() {}), kv: kv}
	c.loadPrefs()
	if c.app.LyricsOffset != 0 {
		t.Fatalf("未勾选记住时不应恢复微调，得到 %.1f", c.app.LyricsOffset)
	}
	_ = kv.SetString(prefLyricsOffsetRemember, "true")
	c = &controller{app: view.NewApp(func() {}), kv: kv}
	c.loadPrefs()
	if c.app.LyricsOffset != 1.5 {
		t.Fatalf("勾选记住时应恢复微调 1.5，得到 %.1f", c.app.LyricsOffset)
	}
}

func TestLyricsWidthForWork(t *testing.T) {
	if got := lyricsWidthForWork(1920, 0.30); got != 576 {
		t.Errorf("1920×30%% = %d，应为 576", got)
	}
	if got := lyricsWidthForWork(1920, 0.80); got != 1536 {
		t.Errorf("1920×80%% = %d，应为 1536", got)
	}
	if got := lyricsWidthForWork(1024, 0.20); got != 320 {
		t.Errorf("1024×20%% 应夹到 320，得到 %d", got)
	}
}

func TestResetLyricsConfigRestoresAllDefaults(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := view.NewApp(func() {})
	a.LyricsColor = "#ff0000"
	a.LyricsBgColor = "#00ff00"
	a.LyricsFontSize = 27
	a.LyricsWidthRatio = 0.7
	a.LyricsTextOpacity = 0.2
	a.LyricsBgOpacity = 0.1
	a.LyricsOffset = 3.5
	a.LyricsOffsetRemember = true
	c := &controller{app: a, kv: kv}
	c.resetLyricsConfig()
	if a.LyricsColor != view.DefaultLyricsColor || a.LyricsBgColor != view.DefaultLyricsBgColor ||
		a.LyricsFontSize != view.DefaultLyricsFontSize || a.LyricsWidthRatio != view.DefaultLyricsWidthRatio ||
		a.LyricsTextOpacity != view.DefaultLyricsTextOpacity || a.LyricsBgOpacity != view.DefaultLyricsBgOpacity ||
		a.LyricsOffset != view.DefaultLyricsOffset || a.LyricsOffsetRemember != view.DefaultLyricsRememberOffset {
		t.Fatalf("恢复默认后配置未全部还原：%+v", a)
	}
	if got := kv.String(prefLyricsWidthRatio); got != "0.3" {
		t.Errorf("恢复默认后宽度未落盘：%q", got)
	}
	if got := kv.String(prefLyricsOffsetRemember); got != "false" {
		t.Errorf("恢复默认后记住微调未关闭：%q", got)
	}
}

func TestPersistFollowPrefs(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := view.NewApp(func() {})
	a.SubtitleFollow = false
	a.DanmakuAutoScroll = true
	c := &controller{app: a, kv: kv}
	c.persistFollowPrefs()
	if got := kv.String(prefSubtitleFollow); got != "false" {
		t.Errorf("字幕跟随落盘 = %q，应为 false", got)
	}
	if got := kv.String(prefDanmakuFollow); got != "true" {
		t.Errorf("弹幕跟随落盘 = %q，应为 true", got)
	}
}

func TestResolvePlaybackCid(t *testing.T) {
	info := &view.Info{Cid: 10, Parts: []view.Part{{Cid: 10}, {Cid: 20}}}
	if got := resolvePlaybackCid(info, 0); got != 10 {
		t.Errorf("cid=0 应用首 P 10，得到 %d", got)
	}
	if got := resolvePlaybackCid(info, 20); got != 20 {
		t.Errorf("属于当前视频的分 P 应保留 20，得到 %d", got)
	}
	if got := resolvePlaybackCid(info, 99); got != 10 {
		t.Errorf("不属于当前视频的过期 cid 应回退 10，得到 %d", got)
	}
	if got := resolvePlaybackCid(nil, 99); got != 99 {
		t.Errorf("没有 Info 时应保留原 cid，得到 %d", got)
	}
}

func TestWaylandSessionFor(t *testing.T) {
	if !waylandSessionFor("linux", "wayland-0", "") {
		t.Error("Linux + WAYLAND_DISPLAY 应识别为 Wayland")
	}
	if !waylandSessionFor("linux", "", "wayland") {
		t.Error("Linux + XDG_SESSION_TYPE=wayland 应识别为 Wayland")
	}
	if waylandSessionFor("linux", "", "x11") {
		t.Error("X11 不应识别为 Wayland")
	}
	if waylandSessionFor("windows", "wayland-0", "wayland") ||
		waylandSessionFor("darwin", "wayland-0", "wayland") {
		t.Error("非 Linux 平台不应受 Wayland 环境变量影响")
	}
}

// TestLyricsConfigRoundTrip 钉住「配置在本地记住，下次打开自动应用」：
// updateLyricsConfig 写盘后，新的 controller.loadPrefs 必须逐项读回。
func TestLyricsConfigRoundTrip(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := view.NewApp(func() {})
	a.LyricsColor = "#38bdf8"
	a.LyricsBgColor = "#000000"
	a.LyricsFontSize = 26
	a.LyricsWidthRatio = 0.45
	a.LyricsTextOpacity = 0.8
	a.LyricsBgOpacity = 0.3
	a.LyricsOffset = 1.2
	a.LyricsOffsetRemember = true
	a.LyricsEnabled = true
	c := &controller{app: a, kv: kv}
	c.updateLyricsConfig()
	_ = kv.SetString(prefLyricsEnabled, "true")

	a2 := view.NewApp(func() {})
	c2 := &controller{app: a2, kv: kv}
	c2.loadPrefs()
	if a2.LyricsColor != "#38bdf8" || a2.LyricsBgColor != "#000000" ||
		a2.LyricsFontSize != 26 || a2.LyricsWidthRatio != 0.45 ||
		a2.LyricsTextOpacity != 0.8 || a2.LyricsBgOpacity != 0.3 ||
		a2.LyricsOffset != 1.2 || !a2.LyricsOffsetRemember || !a2.LyricsEnabled {
		t.Fatalf("重启后配置未完整恢复：%+v", a2)
	}
}

// TestLyricsDisplayRectFrom 钉住位置记忆的坐标系：保存的必须是**显示页**
// 左上角。设置页比显示页宽时显示页在窗口里居中；设置页在上方时显示页在
// 窗口底部。直接存设置页窗口坐标会让窄屏上的位置横向/纵向偏掉。
func TestLyricsDisplayRectFrom(t *testing.T) {
	b := mygo.Rectangle{X: 100, Y: 200, Width: 560, Height: 540}
	// 显示页 320 宽、140 高；设置面板 400 高（窗口 540 = 140 + 400）。
	below := lyricsDisplayRectFrom(b, true, false, 320, 140, 400)
	if below.X != 220 || below.Y != 200 || below.Width != 320 || below.Height != 140 {
		t.Errorf("设置页在下方时显示页矩形 = %+v，want {220 200 320 140}", below)
	}
	above := lyricsDisplayRectFrom(b, true, true, 320, 140, 400)
	if above.X != 220 || above.Y != 600 || above.Width != 320 || above.Height != 140 {
		t.Errorf("设置页在上方时显示页矩形 = %+v，want {220 600 320 140}", above)
	}
	plain := lyricsDisplayRectFrom(b, false, false, 320, 140, 400)
	if plain.X != 100 || plain.Y != 200 || plain.Width != 320 || plain.Height != 140 {
		t.Errorf("显示模式矩形 = %+v，want {100 200 320 140}", plain)
	}
}

// TestLyricsConfigHeightFollowsFontSize 钉住设置窗高度 = 显示页 + 固定面板。
// 字号变大时设置窗必须跟着变高，否则预览长高会把下面的控件挤掉。
func TestLyricsConfigHeightFollowsFontSize(t *testing.T) {
	a := view.NewApp(func() {})
	c := &controller{app: a}
	h20 := c.lyricsConfigHeight()
	if want := view.LyricsDisplayHeight(20) + view.LyricsConfigPanelHeight; h20 != want {
		t.Errorf("20px 设置窗高度 = %d，want %d", h20, want)
	}
	a.LyricsFontSize = 30
	h30 := c.lyricsConfigHeight()
	if h30 <= h20 {
		t.Fatalf("字号变大后设置窗应变高：20→%d, 30→%d", h20, h30)
	}
	if want := view.LyricsDisplayHeight(30) + view.LyricsConfigPanelHeight; h30 != want {
		t.Errorf("30px 设置窗高度 = %d，want %d", h30, want)
	}
}

// TestDesktopLyricsDefaultPosLowerMiddle 钉住首次打开的位置：水平居中，
// 垂直方向在可用高度的 2/3（中间靠下），不是贴底、也不是正中。
func TestDesktopLyricsDefaultPosLowerMiddle(t *testing.T) {
	work := mygo.Rectangle{X: 100, Y: 50, Width: 1000, Height: 800}
	x, y := desktopLyricsDefaultPosIn(work, 400, 100)
	if wantX := 100 + (1000-400)/2; x != wantX {
		t.Errorf("默认 x = %d，want %d（水平居中）", x, wantX)
	}
	if wantY := work.Y + int(float64(work.Height-100)*2.0/3.0); y != wantY {
		t.Errorf("默认 y = %d，want %d（可用高度 2/3）", y, wantY)
	}
	center := work.Y + work.Height/2
	bottom := work.Y + work.Height - 100
	if !(y > center && y < bottom) {
		t.Errorf("默认 y=%d 应落在中间靠下：center=%d bottom=%d", y, center, bottom)
	}
}

// TestDesktopLyricsDefaultPosClamps 钉住小工作区/大窗口时不会把窗口顶边
// 推到工作区外。
func TestDesktopLyricsDefaultPosClamps(t *testing.T) {
	work := mygo.Rectangle{X: 10, Y: 20, Width: 300, Height: 200}
	x, y := desktopLyricsDefaultPosIn(work, 400, 100)
	if x != work.X {
		t.Errorf("窗口比工作区宽时 x = %d，want %d", x, work.X)
	}
	if y < work.Y || y+100 > work.Y+work.Height {
		t.Errorf("窗口顶边/底边越界：y=%d work=%+v", y, work)
	}
}

// TestSavedLyricsPosIgnoresLegacyZero 钉住旧版 0,0 脏数据的迁移：旧版
// OnClosed 在 native 置空后把 (0,0) 写进 KV，修复后不能把它当成用户真的
// 把字幕拖到了左上角，否则第一次启动仍然看起来位置被重置。没有新版本
// 标记时 (0,0) 视为无记录，回退到默认位置。
func TestSavedLyricsPosIgnoresLegacyZero(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	_ = kv.SetString(prefLyricsX, "0")
	_ = kv.SetString(prefLyricsY, "0")
	c := &controller{app: view.NewApp(func() {}), kv: kv}
	if _, _, ok := c.savedLyricsPos(100); ok {
		t.Fatal("旧版写入的 (0,0) 应视为无记录")
	}
}

// TestDesktopLyricsWindowPinnedAbove 钉住桌面字幕窗的层级与阴影设置。
//
// 层级必须是 Panel：三平台里只有面板窗口保证「压在所有窗口之上、跨 Space /
// 虚拟桌面、连全屏应用也压得住」。普通窗口的 AlwaysOnTop 各平台都只是尽力
// （macOS 只到 floating level、切 Space 就没了；Wayland 上 keep_above 是空
// 操作；Windows 上非 topmost 的新窗口随便盖住它）。
//
// 阴影：Windows 的 DWM 阴影按窗口矩形画，背景透明度 0（纯文字浮在桌面）时
// 会留一圈方框影 + 1px 描边，所以创建时先 DisableShadow，再由
// syncLyricsWindowShadow 按背景透明度打开；DWM 那圈**边框**和圆角裁剪则由
// tuneWindowFrame 关掉（DwmSetWindowAttribute 的 DWMWA_BORDER_COLOR =
// DWMWA_COLOR_NONE、DWMWA_WINDOW_CORNER_PREFERENCE = DoNotRound）。
func TestDesktopLyricsWindowPinnedAbove(t *testing.T) {
	src, err := os.ReadFile("lyrics.go")
	if err != nil {
		t.Fatalf("读 lyrics.go 失败：%v", err)
	}
	s := string(src)
	for _, want := range []string{
		"Panel: true,",
		"DisableShadow:     runtime.GOOS == \"windows\"",
		"c.syncLyricsWindowShadow()",
		"flatWindowFrame(win.NativeHandle())",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("桌面字幕窗缺少 %q（层级/阴影设置被改回去了）", want)
		}
	}
	if strings.Contains(s, "Panel: runtime.GOOS") {
		t.Error("桌面字幕窗又按平台分支 Panel 了：macOS 会退回普通窗口，层级压不住其他应用")
	}
}

// TestWindowFrameTuningWired 钉住 Windows 上「去掉 DWM 边框与圆角裁剪」
// 这一步真的接上了：属性常量与调用点都在（Win11 给每个顶层窗口画 1px 边框，
// 而本应用所有窗口都是透明无边框窗，那 1px 正好压在内容半透明的边缘上）。
//
// 这两个属性 mygo 没有公开接口，只能走 NativeHandle() + DwmSetWindowAttribute；
// 常量写错（例如把 DWMWA_COLOR_NONE 写成 0）会静默变成「用黑色画边框」。
func TestWindowFrameTuningWired(t *testing.T) {
	src, err := os.ReadFile("windowframe_windows.go")
	if err != nil {
		t.Fatalf("读 windowframe_windows.go 失败：%v", err)
	}
	s := string(src)
	for _, want := range []string{
		"dwmwaWindowCornerPreference = 33",
		"dwmwaBorderColor            = 34",
		"dwmColorNone = 0xFFFFFFFE", // DWMWA_COLOR_NONE，不是 0（0 是黑色）
		"dwmCornerDoNotRound = 1",
		`"DwmSetWindowAttribute"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("windowframe_windows.go 缺少 %q", want)
		}
	}
	if strings.Contains(s, "dwmColorNone = 0x") && !strings.Contains(s, "0xFFFFFFFE") {
		t.Error("DWMWA_COLOR_NONE 必须是 0xFFFFFFFE（0 会被 DWM 当成黑色边框）")
	}
}

// TestMainWindowFrameTuned 钉住**主窗**也接上了窗口框调整。
//
// 用户 2026-11-11：「窗口最外层有一圈深色的边」，深色模式尤其明显。
// 之前只给桌面字幕窗调了（那边先暴露出来），主窗与迷你窗漏了 —— 而主窗是
// 透明窗，DWM 那 1px 正好压在内容半透明的边缘上。三个窗口走同一个函数。
func TestMainWindowFrameTuned(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	flat := strings.Join(strings.Fields(string(src)), "")
	if !strings.Contains(flat, "tuneWindowFrame(app.Win.NativeHandle())") {
		t.Error("主窗没有调 tuneWindowFrame：DWM 会给透明无边框窗画一圈 1px 边框，" +
			"深色主题下就是用户报的「最外层深色边」")
	}
	// 调用必须紧跟在 NewWindow 之后：再晚一点首帧就已经带着那圈边框显示了。
	newIdx := strings.Index(flat, "app.Win=mygo.NewWindow(opts)")
	tuneIdx := strings.Index(flat, "tuneWindowFrame(app.Win.NativeHandle())")
	if newIdx >= 0 && tuneIdx >= 0 && tuneIdx < newIdx {
		t.Error("tuneWindowFrame 写在了 NewWindow 之前：那时还没有窗口句柄")
	}
}

// TestLyricsWindowShadowFollowsBackground 钉住「背景全透明时不要系统阴影」的
// 判据（Windows 生效，其它平台 syncLyricsWindowShadow 直接返回）。
func TestLyricsWindowShadowFollowsBackground(t *testing.T) {
	cases := []struct {
		opacity float64
		want    bool
	}{
		{0, false},
		{0.01, true},
		{0.55, true},
		{1, true},
	}
	for _, c := range cases {
		if got := lyricsWindowShadowOn(c.opacity); got != c.want {
			t.Errorf("背景透明度 %v：阴影 %v，want %v", c.opacity, got, c.want)
		}
	}
}

// TestLyricsIgnoreMouse 钉住「穿透 vs 设置页」的优先级：设置页永远不穿透。
// 设置页是字幕窗里唯一需要点的地方（滑块 / 颜色 / 完成），一旦穿透就只能回
// 主窗菜单才能关掉 —— 那正是我们把开关入口放在主窗的原因，但仍要保证设置页
// 本身可交互。
func TestLyricsIgnoreMouse(t *testing.T) {
	cases := []struct {
		click, config, want bool
	}{
		{false, false, false},
		{false, true, false},
		{true, false, true}, // 显示页按开关穿透
		{true, true, false}, // 设置页必须可点
	}
	for _, c := range cases {
		if got := lyricsIgnoreMouse(c.click, c.config); got != c.want {
			t.Errorf("clickThrough=%v config=%v: got %v, want %v", c.click, c.config, got, c.want)
		}
	}
}

// TestLyricsClickThroughPrefRoundTrip 钉住「字幕鼠标穿透」跨重启保留。切换
// 只取反 + 落盘 + 同步窗口；测试里没有窗口，同步是 no-op（见 singer 里的守卫）。
func TestLyricsClickThroughPrefRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}
	c.toggleLyricsClickThrough()
	if !c.app.LyricsClickThrough {
		t.Fatal("切换一次后 LyricsClickThrough 应为 true")
	}
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if !c.app.LyricsClickThrough {
		t.Fatal("穿透开关没有跨重启保留")
	}
	c.toggleLyricsClickThrough()
	if c.app.LyricsClickThrough {
		t.Fatal("再切换一次应回到 false")
	}
}

// TestLyricsGearHot 钉住「指针在不在齿轮上」的判定：穿透打开时这一小块永远
// 不穿透（用户 2026-11-11：把设置按钮固定为不穿透），所以它的边界直接决定
// 齿轮点不点得到。
//
// 几何：窗口 560×100 摆在 (100, 200)；齿轮贴右上角、内边距 4、边长 26
// （view.LyricsGearInset / LyricsGearSize）⇒ 热区是以 (660-4, 204) 为右上角、
// 外扩 6px 的一块。
func TestLyricsGearHot(t *testing.T) {
	b := mygo.Rectangle{X: 100, Y: 200, Width: 560, Height: 100}
	cases := []struct {
		name string
		at   mygo.Point
		want bool
	}{
		{"齿轮正中", mygo.Point{X: 643, Y: 217}, true},
		{"齿轮左上角", mygo.Point{X: 630, Y: 204}, true},
		{"热区外扩：窗口右缘内 2px", mygo.Point{X: 658, Y: 202}, true},
		{"齿轮左下角", mygo.Point{X: 630, Y: 236}, true},
		{"窗口中部（字幕文字上）", mygo.Point{X: 300, Y: 240}, false},
		{"齿轮左边 10px", mygo.Point{X: 619, Y: 217}, false},
		{"齿轮下方 10px", mygo.Point{X: 643, Y: 247}, false},
		{"窗口外（右下）", mygo.Point{X: 663, Y: 301}, false},
		// 热区夹在窗口内：窗口右缘之外不再算「在齿轮上」—— 那里指针本来也
		// 进不了我们的窗口，判 true 只会把穿透白关掉。
		{"窗口右缘外 2px", mygo.Point{X: 662, Y: 217}, false},
	}
	for _, c := range cases {
		if got := lyricsGearHot(c.at, b); got != c.want {
			t.Errorf("%s：lyricsGearHot(%v) = %v，应为 %v", c.name, c.at, got, c.want)
		}
	}
	// 窗口还没布局（宽高为 0）：不认作「在齿轮上」，否则穿透会一直关着。
	if lyricsGearHot(mygo.Point{X: 100, Y: 200}, mygo.Rectangle{}) {
		t.Error("空边界不该判定为在齿轮上")
	}
}

// TestLyricsHoverAt 钉住「指针在不在字幕窗范围内」的判定。
//
// 这是穿透体验的核心：穿透时窗口收不到 motion 事件，齿轮的显隐全靠这份包围盒
// 判定（用户 2026-11-11：「关闭穿透效果后鼠标悬浮到字幕区不会有任何选中动画，
// 甚至连设置按钮都不会出来」）。半开区间与窗口坐标系的边界都要钉住。
func TestLyricsHoverAt(t *testing.T) {
	b := mygo.Rectangle{X: 100, Y: 200, Width: 560, Height: 100}
	cases := []struct {
		name string
		at   mygo.Point
		want bool
	}{
		{"窗口正中", mygo.Point{X: 380, Y: 250}, true},
		{"左上角（含）", mygo.Point{X: 100, Y: 200}, true},
		{"右下角前一像素", mygo.Point{X: 659, Y: 299}, true},
		{"右缘（不含）", mygo.Point{X: 660, Y: 250}, false},
		{"下缘（不含）", mygo.Point{X: 380, Y: 300}, false},
		{"左外 1px", mygo.Point{X: 99, Y: 250}, false},
		{"上外 1px", mygo.Point{X: 380, Y: 199}, false},
	}
	for _, c := range cases {
		if got := lyricsHoverAt(c.at, b); got != c.want {
			t.Errorf("%s：lyricsHoverAt(%v) = %v，应为 %v", c.name, c.at, got, c.want)
		}
	}
	// 窗口还没布局（宽高为 0）：不认作「在窗内」，否则齿轮会一直显示。
	if lyricsHoverAt(mygo.Point{X: 100, Y: 200}, mygo.Rectangle{}) {
		t.Error("空边界不该判定为在窗内")
	}
}

// TestLyricsClickThroughKeepsGearClickable 钉住串起来的那条判据：
// 「打开穿透 + 显示页」时，指针在齿轮上就不穿透、移开才穿透。
func TestLyricsClickThroughKeepsGearClickable(t *testing.T) {
	b := mygo.Rectangle{X: 0, Y: 0, Width: 560, Height: 100}
	onGear := mygo.Point{X: 560 - 4 - 13, Y: 4 + 13}
	away := mygo.Point{X: 280, Y: 60}
	// 基准：穿透开关与设置页的优先级（设置页永不穿透）。
	if lyricsIgnoreMouse(false, false) || lyricsIgnoreMouse(false, true) {
		t.Error("穿透关闭时任何情况都不该穿透")
	}
	if !lyricsIgnoreMouse(true, false) {
		t.Error("穿透 + 显示页应穿透")
	}
	if lyricsIgnoreMouse(true, true) {
		t.Error("设置页永不穿透")
	}
	// 齿轮上：即使穿透打开也不穿透。
	if !lyricsGearHot(onGear, b) {
		t.Fatal("前置：测试点应在齿轮热区内")
	}
	if lyricsGearHot(away, b) {
		t.Fatal("前置：测试点应在齿轮热区外")
	}
}

// TestLyricsGearShapeRect 钉住「穿透时仍可交互」的矩形本身（窗口坐标）：
// 右上角齿轮 + slop，并夹进窗口内。Linux 直接拿它当窗口输入区域。
func TestLyricsGearShapeRect(t *testing.T) {
	// 560×100：x = 560-4-26-6 = 524，宽 38 → 524+38 = 562 > 560，夹到 36；
	// y = 4-6 = -2 → 夹到 0，高 38-2 = 36。
	x, y, w, h := lyricsGearShapeRect(560, 100)
	if x != 524 || y != 0 || w != 36 || h != 36 {
		t.Errorf("560×100 的热区 = (%v,%v,%v,%v)，应为 (524,0,36,36)", x, y, w, h)
	}
	// 宽窗：上边与右边的 slop 都落在窗口外（齿轮贴上缘 4px、离右缘 4px），
	// 夹掉后是 36×36。
	x, y, w, h = lyricsGearShapeRect(900, 200)
	if x != 900-4-26-6 || y != 0 || w != 36 || h != 36 {
		t.Errorf("900×200 的热区 = (%v,%v,%v,%v)，应为 (864,0,36,36)", x, y, w, h)
	}
	// 比齿轮还小的窗口：不产生负宽高。
	if _, _, w, h := lyricsGearShapeRect(20, 10); w <= 0 || h <= 0 {
		t.Errorf("小窗口的热区 = %v×%v，应为正数", w, h)
	}
	// 还没布局：全零。
	if x, y, w, h := lyricsGearShapeRect(0, 0); x != 0 || y != 0 || w != 0 || h != 0 {
		t.Errorf("零尺寸的热区 = (%v,%v,%v,%v)，应全为 0", x, y, w, h)
	}
}

// TestExpireLogin 钉住「登录过期」的收敛：接口回 -101 时把本地登录态、已失效的
// cookie 与 KV 里的用户资料一起清掉 —— 只把 LoggedIn 置假不够，下次启动还会因为
// KV 里有 uname 而认为已登录（用户 2026-11-11 报「登录了也还是报错」）。
func TestExpireLogin(t *testing.T) {
	probe := bilibili.NewBL()
	savedStatus, savedCookie := bilibili.LoginStatus, probe.GetSESSDATA()
	defer func() {
		bilibili.LoginStatus = savedStatus
		probe.SetSESSDATA(savedCookie)
	}()

	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	a := view.NewApp(func() {})
	a.LoggedIn = true
	a.UName, a.Face = "阿绫", "face.png"
	c := &controller{app: a, kv: kv, bl: bilibili.NewBL()}
	c.bl.SetSESSDATA("SESSDATA=expired")
	c.bl.SetLoginStatus(true)
	_ = kv.SetString("uname", "阿绫")
	_ = kv.SetString("face", "face.png")

	c.expireLogin()

	if a.LoggedIn {
		t.Error("登录过期后 app.LoggedIn 应为 false")
	}
	if a.UName != "" || a.Face != "" {
		t.Errorf("用户资料应清空，得到 %q / %q", a.UName, a.Face)
	}
	if got := c.bl.GetSESSDATA(); got != "" {
		t.Errorf("失效的 cookie 应清掉，得到 %q", got)
	}
	if c.bl.GetLoginStatus() {
		t.Error("bilibili.LoginStatus 应为 false")
	}
	for _, k := range []string{"uname", "face", "mid"} {
		if got := kv.String(k); got != "" {
			t.Errorf("KV %q 应清空，得到 %q", k, got)
		}
	}
	// 幂等：已登出时再调一次不 panic、不重复清。
	c.expireLogin()
}
