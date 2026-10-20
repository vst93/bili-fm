package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
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
