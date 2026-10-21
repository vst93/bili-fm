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
