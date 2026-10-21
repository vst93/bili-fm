package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
)

func TestTrimRadioQueueKeepsCurrent(t *testing.T) {
	a := &view.App{Queue: make([]view.Track, 60), Index: 50}
	c := &controller{app: a}
	c.trimRadioQueue()
	if len(a.Queue) != 48 {
		t.Fatalf("queue len = %d, want 48", len(a.Queue))
	}
	if a.Index != 38 {
		t.Fatalf("index = %d, want 38（当前项不能被裁掉）", a.Index)
	}
}

func TestTrimRadioQueueNoopWhenSmall(t *testing.T) {
	a := &view.App{Queue: make([]view.Track, 10), Index: 3}
	c := &controller{app: a}
	c.trimRadioQueue()
	if len(a.Queue) != 10 || a.Index != 3 {
		t.Fatalf("small queue changed: len=%d index=%d", len(a.Queue), a.Index)
	}
}

func TestPickSubtitleTrackPrefersChinese(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{Lan: "en-US", Label: "English"},
		{Lan: "ai-zh", Label: "自动字幕"},
		{Lan: "zh-CN", Label: "中文"},
	}
	got, ok := pickSubtitleTrack(tracks)
	if !ok || got.Lan != "zh-CN" {
		t.Fatalf("pick = %+v ok=%v，want zh-CN", got, ok)
	}
	if _, ok := pickSubtitleTrack(nil); ok {
		t.Fatal("空字幕轨应返回 ok=false")
	}
}

func TestPickSubtitleTrackUsesPreference(t *testing.T) {
	tracks := []view.SubtitleTrack{{Lan: "zh-CN", Label: "中文"}, {Lan: "en-US", Label: "English"}}
	got, ok := pickSubtitleTrackPref(tracks, "en-US")
	if !ok || got.Lan != "en-US" {
		t.Fatalf("preferred track = %+v ok=%v，want en-US", got, ok)
	}
	// 新视频没有用户偏好的语言时回落到默认中文。
	got, ok = pickSubtitleTrackPref(tracks, "ja-JP")
	if !ok || got.Lan != "zh-CN" {
		t.Fatalf("fallback = %+v ok=%v，want zh-CN", got, ok)
	}
}
