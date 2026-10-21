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
