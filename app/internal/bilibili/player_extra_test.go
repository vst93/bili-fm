package bilibili

import (
	"strings"
	"testing"
)

func TestPlayerExtrasURLPrefersBvid(t *testing.T) {
	// 搜索卡片上的 aid 可能过期；有 bvid 时不能再把 aid 传给上游，否则
	// player/v2 可能按 aid 返回另一条稿件的字幕。
	got := playerExtrasURL(123, 456, "BV1xx411c7mD")
	if strings.Contains(got, "aid=") {
		t.Fatalf("有 bvid 时不应带 aid: %s", got)
	}
	if !strings.Contains(got, "bvid=BV1xx411c7mD") || !strings.Contains(got, "cid=456") {
		t.Fatalf("bvid/cid 参数缺失: %s", got)
	}
}

func TestPlayerExtrasURLFallsBackToAid(t *testing.T) {
	got := playerExtrasURL(123, 456, "")
	if !strings.Contains(got, "aid=123") || !strings.Contains(got, "cid=456") {
		t.Fatalf("无 bvid 时应退回 aid: %s", got)
	}
}

func TestSubtitleCuesFitRejectsWrongVideo(t *testing.T) {
	// 10 分钟的视频，字幕却跑到 40 分钟：明显是别的稿件的轨。
	cues := []SubtitleCue{{From: 0, To: 5, Content: "a"}, {From: 39 * 60, To: 40 * 60, Content: "b"}}
	if SubtitleCuesFit(cues, 600) {
		t.Fatal("明显超长的字幕不应通过时长校验")
	}
}

func TestSubtitleCuesFitAcceptsNormalTrack(t *testing.T) {
	cues := []SubtitleCue{{From: 0, To: 5, Content: "a"}, {From: 590, To: 610, Content: "b"}}
	if !SubtitleCuesFit(cues, 600) {
		t.Fatal("正常字幕不应被时长校验误杀")
	}
}
