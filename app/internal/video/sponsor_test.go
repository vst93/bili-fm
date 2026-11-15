package video

// 原生视频窗的「广告跳过」接线测试。
//
// 广告分段的来源与主窗完全相同：SponsorBlock 查询（internal/bilibili 的
// FetchSponsorSegments）→ controller.sponsor → toViewSegments → Options.
// SponsorSegments / Manager.SetSponsorSegments。跳过的判据也只写在一处
// （view.VideoChrome.skipSponsor，与主窗 maybeSkipSponsor 同值：lead 0.25s、
// 落点 end+0.05s、时长容差 2s）。
//
// 这里钉住的是**最后一步真的发生了**：外壳到点必须对 ui/video 播放器
// （nativeWindow.pl，生产环境是 uiPlayer{*ui.VideoPlayer}）发起 Seek(end+0.05)，
// 而不是只画进度条上的标记。开关关掉时一次都不许 seek。

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view"
)

// nativeSponsorProbe 起一个原生窗视图：300s 时长、位置 42s、恰饭段 40–45s。
func nativeSponsorProbe(t *testing.T, sponsor bool, pos time.Duration) (*nativeWindow, *stubPlayer, *ui.Tester) {
	t.Helper()
	pl := &stubPlayer{playing: true, ready: true, dur: 300 * time.Second, pos: pos, rate: 1, vol: 1}
	set := &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: sponsor}
	set.Ensure()
	n := &nativeWindow{
		pl:       pl,
		title:    "测试稿件",
		settings: set,
		segments: []view.Segment{{Start: 40, End: 45}},
	}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	return n, pl, tt
}

// TestNativeSkipsSponsorSegment 是「原生路径真的跳过」的那条断言：
// 位置进入 [start-lead, end) 后，播放器收到 Seek(end+0.05)。
func TestNativeSkipsSponsorSegment(t *testing.T) {
	_, pl, _ := nativeSponsorProbe(t, true, 42*time.Second)
	if len(pl.seeks) != 1 {
		t.Fatalf("到点后应 seek 一次，实际 %v（seeks=%v）", len(pl.seeks), pl.seeks)
	}
	want := 45*time.Second + 50*time.Millisecond
	if d := pl.seeks[0] - want; d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("seek 到 %v，想要 %v（分段末尾 + 0.05s）", pl.seeks[0], want)
	}
}

// TestNativeSponsorOffNoSeek 钉住共享开关真的控制这条路径：关掉后即便位置
// 落在分段里也不 seek（不能出现「开关只改状态点」）。
func TestNativeSponsorOffNoSeek(t *testing.T) {
	_, pl, _ := nativeSponsorProbe(t, false, 42*time.Second)
	if len(pl.seeks) != 0 {
		t.Errorf("广告跳过关闭时不该 seek，实际 %v", pl.seeks)
	}
}

// TestNativeSponsorLeadAndTolerance 钉住与主窗同一套判据：提前 0.25s 触发、
// 时长偏差 >2s 的段丢弃（那是另一版视频的标记）。
func TestNativeSponsorLeadAndTolerance(t *testing.T) {
	// 39.75s（start-0.25）已在窗口内。
	_, pl, _ := nativeSponsorProbe(t, true, 39750*time.Millisecond)
	if len(pl.seeks) != 1 {
		t.Errorf("pos=start-0.25 应触发跳过，实际 seeks=%v", pl.seeks)
	}

	// 39.74s 还差一点：不触发。
	_, pl, _ = nativeSponsorProbe(t, true, 39740*time.Millisecond)
	if len(pl.seeks) != 0 {
		t.Errorf("pos=start-0.26 不该触发，实际 seeks=%v", pl.seeks)
	}

	// 时长对不上（VideoDuration 297 vs 300，差 3s > 2s）：标记不属于这一版。
	pl2 := &stubPlayer{playing: true, ready: true, dur: 300 * time.Second, pos: 42 * time.Second, rate: 1, vol: 1}
	set := &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: true}
	set.Ensure()
	n := &nativeWindow{pl: pl2, title: "x", settings: set,
		segments: []view.Segment{{Start: 40, End: 45, VideoDuration: 297}}}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if len(pl2.seeks) != 0 {
		t.Errorf("时长偏差 >2s 的分段不该跳过，实际 seeks=%v", pl2.seeks)
	}
}

// TestNativeSponsorPausedNoSeek 钉住「只在播放中判」：暂停停在恰饭段上不该
// 自己跳走（与主窗同一语义，见 VideoChrome.View）。
func TestNativeSponsorPausedNoSeek(t *testing.T) {
	pl := &stubPlayer{playing: false, ready: true, dur: 300 * time.Second, pos: 42 * time.Second, rate: 1, vol: 1}
	set := &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: true}
	set.Ensure()
	n := &nativeWindow{pl: pl, title: "x", settings: set,
		segments: []view.Segment{{Start: 40, End: 45}}}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if len(pl.seeks) != 0 {
		t.Errorf("暂停时不该自动跳过，实际 seeks=%v", pl.seeks)
	}
}

// TestNativeSponsorSegmentsSyncedLater 钉住「主窗查完再同步」这条路：
// 开窗时还没有分段（Options 为空），随后 Manager.SetSponsorSegments 送来，
// 下一帧到点照样 seek —— SetSponsorSegments 写的是同一份 n.segments。
func TestNativeSponsorSegmentsSyncedLater(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 300 * time.Second, pos: 42 * time.Second, rate: 1, vol: 1}
	set := &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1, Sponsor: true}
	set.Ensure()
	n := &nativeWindow{pl: pl, title: "x", settings: set}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if len(pl.seeks) != 0 {
		t.Fatalf("还没有分段时不该 seek，实际 %v", pl.seeks)
	}
	n.segments = []view.Segment{{Start: 40, End: 45}}
	tt.Frame()
	if len(pl.seeks) != 1 {
		t.Errorf("分段同步进来后应跳过，实际 seeks=%v", pl.seeks)
	}
}
