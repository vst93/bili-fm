package view

import "testing"

func TestLyricsOffsetLabel(t *testing.T) {
	cases := map[float64]string{0: "0.0s", 1.2: "+1.2s", -1.2: "-1.2s", 10: "+10.0s"}
	for in, want := range cases {
		if got := lyricsOffsetLabel(in); got != want {
			t.Errorf("offset %.1f label = %q, want %q", in, got, want)
		}
	}
}

func TestInfoHasPartCid(t *testing.T) {
	info := &Info{Cid: 10, Parts: []Part{{Cid: 10}, {Cid: 20}}}
	if !info.HasPartCid(20) {
		t.Error("应识别属于当前视频的分 P")
	}
	if info.HasPartCid(99) || info.HasPartCid(0) {
		t.Error("不属于当前视频的 cid 不应通过")
	}
}
