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
