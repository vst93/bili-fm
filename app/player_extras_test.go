package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
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

func TestExtrasAidUsesCurrentInfoOnlyWhenBvidMatches(t *testing.T) {
	a := &view.App{}
	track := &view.Track{Aid: 10, Bvid: "BV1", Cid: 100}
	if got := extrasAid(a, track); got != 10 {
		t.Fatalf("无 Info 时 extrasAid = %d，应为 track.Aid 10", got)
	}
	a.Info = &view.Info{Aid: 20, Bvid: "BV1"}
	if got := extrasAid(a, track); got != 20 {
		t.Fatalf("bvid 匹配时应使用 Info.Aid 20，得到 %d", got)
	}
	a.Info.Bvid = "BV2"
	if got := extrasAid(a, track); got != 10 {
		t.Fatalf("bvid 不匹配时不能借用另一条视频的 aid，得到 %d", got)
	}
}

func TestDropFutureRadio(t *testing.T) {
	a := &view.App{
		Index: 0,
		Queue: []view.Track{
			{Bvid: "origin"},
			{Bvid: "radio1", FromRadio: true},
			{Bvid: "origin2"},
			{Bvid: "radio2", FromRadio: true},
		},
	}
	dropFutureRadio(a)
	if len(a.Queue) != 2 || a.Queue[0].Bvid != "origin" || a.Queue[1].Bvid != "origin2" {
		t.Fatalf("关闭电台后应只保留原队列，得到 %#v", a.Queue)
	}
}

func TestPickSubtitlePrefersHumanTrack(t *testing.T) {
	tracks := []view.SubtitleTrack{
		{Lan: "zh-CN", Label: "AI", URL: "ai", AIType: 1},
		{Lan: "zh-CN", Label: "人工", URL: "human"},
	}
	got, ok := pickSubtitleTrackPref(tracks, "")
	if !ok || got.URL != "human" {
		t.Fatalf("同语言应优先人工字幕，得到 %#v", got)
	}
}

func TestSubtitleCuesFit(t *testing.T) {
	// 10 分钟视频，字幕只到 2 分钟：明显是别的短轨/截断轨。
	short := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 95, To: 100}}
	if subtitleCuesFit(short, 600) {
		t.Fatal("覆盖率不足的字幕轨不应通过")
	}
	// 正常覆盖，尾句略有超出（片尾字幕）不应误杀。
	normal := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 580, To: 610}}
	if !subtitleCuesFit(normal, 600) {
		t.Fatal("正常覆盖的字幕轨被误杀")
	}
	// 字幕明显跑到另一条更长的视频里。
	long := []bilibili.SubtitleCue{{From: 0, To: 5}, {From: 1200, To: 1300}}
	if subtitleCuesFit(long, 600) {
		t.Fatal("超出时长容差的字幕轨不应通过")
	}
}
