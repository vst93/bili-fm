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
