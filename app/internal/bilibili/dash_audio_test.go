package bilibili

import "testing"

// TestPickDashAudioURLPrefersHighestLC 钉住挑轨规则：只认 AAC-LC
// （mp4a.40.2），同一 codec 取码率最高的一条。
//
// 现场用例来自 BV1k3411f7mX（许嵩歌曲精选）：360P 合流是 HE-AAC，
// DASH 里的 30216 也是 HE-AAC，只有 30232/30280 是 AAC-LC。
func TestPickDashAudioURLPrefersHighestLC(t *testing.T) {
	list := []DashAudio{
		{ID: 30216, Codecs: "mp4a.40.5", Bandwidth: 50868, BaseURL: "he-aac-30216"},
		{ID: 30232, Codecs: "mp4a.40.2", Bandwidth: 117213, BaseURL: "lc-30232"},
		{ID: 30280, Codecs: "mp4a.40.2", Bandwidth: 185206, BaseURL: "lc-30280"},
		{ID: 30250, Codecs: "ec-3", Bandwidth: 448000, BaseURL: "dolby"},
		{ID: 30251, Codecs: "fLaC", Bandwidth: 900000, BaseURL: "flac"},
	}
	if got := pickDashAudioURL(list); got != "lc-30280" {
		t.Errorf("挑中的音轨 = %q，应为码率最高的 AAC-LC（lc-30280）", got)
	}
}

// TestPickDashAudioURLRejectsUndecodable 没有 AAC-LC 时必须返回空，
// 让调用方退回合流地址（HE-AAC/Dolby/FLAC 解码器都解不了）。
func TestPickDashAudioURLRejectsUndecodable(t *testing.T) {
	list := []DashAudio{
		{ID: 30216, Codecs: "mp4a.40.5", Bandwidth: 50868, BaseURL: "he-aac"},
		{ID: 30250, Codecs: "ec-3", Bandwidth: 448000, BaseURL: "dolby"},
	}
	if got := pickDashAudioURL(list); got != "" {
		t.Errorf("没有 AAC-LC 时应返回空，得到 %q", got)
	}
	if got := pickDashAudioURL(nil); got != "" {
		t.Errorf("空列表应返回空，得到 %q", got)
	}
}

// TestPickDashAudioURLSkipsEmptyBaseURL 只有地址非空的候选才算数。
func TestPickDashAudioURLSkipsEmptyBaseURL(t *testing.T) {
	list := []DashAudio{
		{ID: 30280, Codecs: "mp4a.40.2", Bandwidth: 185206, BaseURL: ""},
		{ID: 30232, Codecs: "mp4a.40.2", Bandwidth: 117213, BaseURL: "lc-30232"},
	}
	if got := pickDashAudioURL(list); got != "lc-30232" {
		t.Errorf("挑中的音轨 = %q，应跳过空地址", got)
	}
}

// --- GetDashAVByCid 的挑轨规则（issue #42 视频提速） ---

func track(id, codecid int, codecs string, bw, w, h int, base, init, idx string) DashTrack {
	t := DashTrack{ID: id, CodecID: codecid, Codecs: codecs, Bandwidth: bw,
		Width: w, Height: h, BaseURL: base}
	t.SegmentBase.Initialization = init
	t.SegmentBase.IndexRange = idx
	return t
}

// TestPickDashVideoTrackPrefersAVC360P 钉住视频挑轨：只认 AVC，360P 优先。
//
// 现场用例来自 BV1BFsA6TERt：同一条稿件同时给了 480P/360P 的 AVC 与 HEVC。
// 选 HEVC 会让 Windows/macOS/Linux 三条解码栈里至少一条播不了（未实测），
// 而 360P 正是现在 platform=html5 合流的清晰度，换轨不该顺手改画质。
func TestPickDashVideoTrackPrefersAVC360P(t *testing.T) {
	list := []DashTrack{
		track(32, 7, "avc1.640033", 289636, 852, 480, "u", "0-937", "938-13589"),
		track(32, 12, "hvc1.1.6.L120.90", 213893, 852, 480, "u", "0-1103", "1104-13755"),
		track(16, 12, "hvc1.1.6.L120.90", 150160, 640, 360, "u", "0-1099", "1100-13751"),
		track(16, 7, "avc1.640033", 202553, 640, 360, "u", "0-933", "934-13585"),
	}
	got := pickDashVideoTrack(list)
	if got.ID != 16 || got.CodecID != 7 {
		t.Fatalf("挑到 id=%d codecid=%d，想要 AVC 360P（id=16 codecid=7）", got.ID, got.CodecID)
	}
}

// TestPickDashVideoTrackNeverPicksNonAVC 钉住「HEVC / AV1 一律不选」。
func TestPickDashVideoTrackNeverPicksNonAVC(t *testing.T) {
	list := []DashTrack{
		track(32, 12, "hvc1.1.6.L120.90", 213893, 852, 480, "u", "0-1103", "1104-13755"),
		track(16, 13, "av01.0.08M.08.0.110.01.01.01.0", 132080, 640, 360, "u", "0-1", "2-3"),
	}
	if got := pickDashVideoTrack(list); got.ID != 0 {
		t.Fatalf("只有 HEVC/AV1 时应当返回空轨道，得到 %+v", got)
	}
}

// TestPickDashVideoTrackFallsBackToSmallestAVC 钉住「没有 360P 时取分辨率最小
// 的 AVC」：同一画质档位内少下一点是一点。
func TestPickDashVideoTrackFallsBackToSmallestAVC(t *testing.T) {
	list := []DashTrack{
		track(80, 7, "avc1.640033", 900000, 1920, 1080, "u", "0-1", "2-3"),
		track(64, 7, "avc1.640033", 400000, 1280, 720, "u", "0-1", "2-3"),
	}
	if got := pickDashVideoTrack(list); got.ID != 64 {
		t.Fatalf("挑到 id=%d，想要分辨率最小的 64", got.ID)
	}
}

// TestPickDashVideoTrackRequiresSegmentBase 钉住：没有 SegmentBase 的轨道不能选
// （没有索引就没法做 HLS，硬用会退化成整条下载）。
func TestPickDashVideoTrackRequiresSegmentBase(t *testing.T) {
	list := []DashTrack{
		track(16, 7, "avc1.640033", 202553, 640, 360, "u", "", ""),
	}
	if got := pickDashVideoTrack(list); got.ID != 0 {
		t.Fatalf("缺 SegmentBase 不该被选，得到 %+v", got)
	}
}

// TestPickDashAudioTrackRejectsHEAAC 钉住音轨判据与 pickDashAudioURL 一致。
func TestPickDashAudioTrackRejectsHEAAC(t *testing.T) {
	list := []DashTrack{
		track(30216, 0, "mp4a.40.5", 66147, 0, 0, "u", "0-1", "2-3"),
		track(30232, 0, "mp4a.40.2", 68855, 0, 0, "u", "0-1", "2-3"),
		track(30280, 0, "mp4a.40.2", 82828, 0, 0, "u", "0-1", "2-3"),
	}
	got := pickDashAudioTrack(list)
	if got.ID != 30280 {
		t.Fatalf("挑到 id=%d，想要码率最高的 AAC-LC 30280", got.ID)
	}
}

// TestDashTrackUsable 钉住 Usable 的三个条件。
func TestDashTrackUsable(t *testing.T) {
	cases := []struct {
		name string
		t    DashTrack
		want bool
	}{
		{"齐全", track(1, 7, "avc1.1", 1, 1, 1, "u", "0-1", "2-3"), true},
		{"没有地址", track(1, 7, "avc1.1", 1, 1, 1, "", "0-1", "2-3"), false},
		{"没有 init", track(1, 7, "avc1.1", 1, 1, 1, "u", "", "2-3"), false},
		{"没有索引", track(1, 7, "avc1.1", 1, 1, 1, "u", "0-1", ""), false},
	}
	for _, c := range cases {
		if got := c.t.Usable(); got != c.want {
			t.Errorf("%s: Usable()=%v，想 %v", c.name, got, c.want)
		}
	}
}
