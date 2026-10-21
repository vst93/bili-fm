package bilibili

import (
	"strings"
	"testing"
)

func TestPlayerExtrasURLPrefersBvid(t *testing.T) {
	// 多 P 视频必须带 aid + cid 定位分 P；bvid 只作为稿件身份的辅助校验。
	got := playerExtrasURL(123, 456, "BV1xx411c7mD")
	if !strings.Contains(got, "aid=123") {
		t.Fatalf("缺少 aid: %s", got)
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

func TestPlayerExtrasIdentityRejectsOtherVideo(t *testing.T) {
	if err := playerExtrasIdentityError(map[string]any{"aid": float64(2), "cid": float64(456), "bvid": "BV2"}, 1, 456, "BV1"); err == nil {
		t.Fatal("aid/bvid 不匹配时应拒绝")
	}
	if err := playerExtrasIdentityError(map[string]any{"aid": float64(1), "cid": float64(999), "bvid": "BV1"}, 1, 456, "BV1"); err == nil {
		t.Fatal("cid 不匹配时应拒绝")
	}
	if err := playerExtrasIdentityError(map[string]any{"aid": float64(1), "cid": float64(456), "bvid": "BV1"}, 1, 456, "BV1"); err != nil {
		t.Fatalf("身份一致时不应报错：%v", err)
	}
	// 旧接口可能不回身份字段，保持兼容。
	if err := playerExtrasIdentityError(map[string]any{}, 1, 456, "BV1"); err != nil {
		t.Fatalf("缺字段时应兼容：%v", err)
	}
}
