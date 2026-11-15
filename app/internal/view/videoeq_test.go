package view

// 视频窗（view.VideoChrome）这一层的两条断言：
//
//  1. 广告跳过到点**真的 seek 播放器**（不是只画进度条上的标记），关掉共享
//     开关时一次都不 seek；判据与主窗 maybeSkipSponsor 同值（lead 0.25s、
//     落点 end+0.05s、时长容差 2s）。
//  2. 音量均衡在原生视频里**不适用**：键置灰、不可点，不把共享的 EQ 画成
//     这个窗口的「已开」，tooltip 说明原因并报出主窗当前值。
//
// 为什么音量均衡不适用：原生视频的声音由系统媒体框架（AVFoundation /
// Media Foundation / GStreamer）解码后直接送给声卡，PCM 在框架内部；本仓库
// 的均衡器（internal/media.Compressor）挂在 oto 解码后的自己的流水线上，
// 拿不到视频那路采样。ui/video 的导出面只有 Play/Pause/Seek/SetRate/
// SetVolume/SetMuted/Position/Duration/Size，没有任何音频数据或输出钩子
// （见 PLAN.md「原生视频里的广告跳过 / 音量均衡」）。

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestVideoChromeSkipsSponsorSegment 是视频窗路径的「到点 seek」断言。
func TestVideoChromeSkipsSponsorSegment(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 15 * time.Second, rate: 1, vol: 1}
	set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: true}
	set.Ensure()
	ch := &VideoChrome{
		Theme:           ThemeFor(PeriodAt(12)),
		Title:           "测试稿件",
		Pl:              pl,
		Win:             newVideoStubWindow(),
		Set:             set,
		SponsorSegments: []Segment{{Start: 10, End: 20}},
	}
	tt := ui.NewTester(ch.View, 640, 360)
	tt.Frame()
	if len(pl.seeks) != 1 {
		t.Fatalf("到点后应 seek 一次，实际 %v", pl.seeks)
	}
	want := 20*time.Second + 50*time.Millisecond
	if d := pl.seeks[0] - want; d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("seek 到 %v，想要 %v（分段末尾 + 0.05s）", pl.seeks[0], want)
	}
	// 时间闩：seek 还没落地的那几帧不许连发。
	tt.Frame()
	if len(pl.seeks) != 1 {
		t.Errorf("同一段被重复 seek：%v", pl.seeks)
	}
}

// TestVideoChromeSponsorOffNoSeek 是上一条的反证：共享开关关闭时不 seek。
func TestVideoChromeSponsorOffNoSeek(t *testing.T) {
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 15 * time.Second, rate: 1, vol: 1}
	set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: false}
	set.Ensure()
	ch := &VideoChrome{
		Theme:           ThemeFor(PeriodAt(12)),
		Title:           "测试稿件",
		Pl:              pl,
		Win:             newVideoStubWindow(),
		Set:             set,
		SponsorSegments: []Segment{{Start: 10, End: 20}},
	}
	tt := ui.NewTester(ch.View, 640, 360)
	tt.Frame()
	if len(pl.seeks) != 0 {
		t.Errorf("广告跳过关闭时不该 seek，实际 %v", pl.seeks)
	}
}

// TestVideoEQKeyUnavailable 钉住视频窗均衡键的不可用态：
// 共享 EQ 为开时也不画成「已开」，点击不改共享设置，说明里写明原因与主窗值。
func TestVideoEQKeyUnavailable(t *testing.T) {
	set := &PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, EQ: true}
	set.Ensure()
	_, _, tt := playerBindVideo(t, set)

	label := eqVideoUnavailableTip(true)
	info, ok := tt.AccessByRole(ui.RoleToggleButton, label)
	if !ok {
		t.Fatalf("视频窗里找不到音量均衡键（现有：%v）", tt.Texts())
	}
	if !info.Disabled {
		t.Error("视频窗的音量均衡键应禁用（原生视频不适用）")
	}
	if info.Checked {
		t.Error("视频窗把共享 EQ 画成了本窗口「已开」（伪造生效）")
	}
	for _, want := range []string{"不适用于原生视频", "系统媒体框架", "主窗：开"} {
		if !strings.Contains(info.Description, want) {
			t.Errorf("均衡键说明里缺少 %q：%q", want, info.Description)
		}
	}
	// 点一下：不能改共享设置（禁用节点不接收命中）。
	if err := tt.Click(label); err != nil {
		t.Fatalf("点不到均衡键：%v", err)
	}
	tt.Frame()
	if !set.EQ {
		t.Error("视频窗的均衡键不可用，点击却改了共享 EQ（伪造生效）")
	}

	// 主窗关掉后，视频窗的说明跟着报「主窗：关」——共享值仍是同一份，
	// 只是不以可按压的开关形式出现在视频窗。
	set.SetEQ(false)
	tt.Frame()
	if _, ok := tt.AccessByRole(ui.RoleToggleButton, eqVideoUnavailableTip(false)); !ok {
		t.Errorf("主窗关掉均衡后视频窗的说明没跟上：%v", tt.Texts())
	}
}
