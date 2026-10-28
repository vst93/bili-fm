package main

import (
	"testing"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/bilibili"
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
	items := []bilibili.DanmakuItem{
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
