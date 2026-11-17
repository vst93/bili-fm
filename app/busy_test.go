package main

// issue #42：起播初始化与运行中等待是两套状态。这里只钉控制器侧的接线：
//   - 初始化期间点播放不能去 Resume 旧/空引擎；
//   - 运行中等待不挡 seek（只有初始化才排队）；
//   - 起播收尾的 stale 复核真的保护新请求的忙态；
//   - 视频→主窗回传忙态，关窗/失败时清掉。

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/media"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestIssue42TogglePlayDuringInitDoesNotResumeOldEngine 钉住起播初始化期间的
// 播放键：它必须直接返回。旧实现会走 c.mp.Resume()，对已经 Stop/尚未装上的
// oto player 是空操作，却把 app.Playing 手写成 true（界面显示「已暂停」）。
//
// 用零值 media.Player 作替身：如果没有这层守卫，Resume 是空操作但
// app.Playing 仍会被置真，断言能抓住。
func TestIssue42TogglePlayDuringInitDoesNotResumeOldEngine(t *testing.T) {
	c, _ := newVideoCloseController(t, false)
	c.app.VideoOpen = false
	c.app.Buffering = true
	c.app.Waiting = false
	c.app.Playing = false
	c.mp = &media.Player{}

	c.togglePlay()

	if c.app.Playing {
		t.Fatal("起播初始化期间点播放把界面置成了「正在播放」——旧引擎被 Resume 了")
	}
	if !c.app.Buffering {
		t.Fatal("起播初始化状态被播放键改掉了")
	}
}

// TestIssue42SeekGateFollowsOutputOwner 钉住 seek 排队判据：只有音频起播
// 初始化且视频窗没占输出时才排队；视频占着声音时 seek 必须直接发给视频。
func TestIssue42SeekGateFollowsOutputOwner(t *testing.T) {
	cases := []struct {
		name             string
		buffering, video bool
		want             bool
	}{
		{"音频初始化", true, false, true},
		{"音频初始化+视频占输出", true, true, false},
		{"运行中欠载", false, false, false},
		{"视频占输出", false, true, false},
	}
	for _, tc := range cases {
		if got := seekWaitsForInit(tc.buffering, tc.video); got != tc.want {
			t.Errorf("%s: seekWaitsForInit(%v,%v)=%v，想 %v",
				tc.name, tc.buffering, tc.video, got, tc.want)
		}
	}
}

// TestIssue42SeekDuringRuntimeWaitIsNotQueued 钉住 seek 闸门只认初始化：
// 运行中等待（Waiting）不能把用户拖动的目标排进 PendingSeek 永久挂住。
func TestIssue42SeekDuringRuntimeWaitIsNotQueued(t *testing.T) {
	t.Run("初始化排队", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.Buffering = true
		c.app.Waiting = true
		c.app.SeekBusy = false
		c.seek(42)
		if !c.app.HasPendingSeek || c.app.PendingSeek != 42 || c.app.SeekValue != 42 {
			t.Fatalf("初始化期间 seek 应排队：pending=%v target=%v value=%v",
				c.app.HasPendingSeek, c.app.PendingSeek, c.app.SeekValue)
		}
		if c.app.SeekBusy {
			t.Fatal("初始化期间 seek 不该开始执行")
		}
	})
	t.Run("运行中等待立即执行", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.Buffering = false
		c.app.Waiting = true
		c.app.SeekBusy = false
		c.mp = nil // 让 runSeek 走「没有引擎」的收尾，不依赖声卡
		c.seek(42)
		if c.app.HasPendingSeek {
			t.Fatal("运行中等待把 seek 排进了 PendingSeek —— 正常缓冲期间拖进度条会卡住")
		}
		if c.app.SeekBusy {
			t.Fatal("seek 没有走完收尾，SeekBusy 仍为真")
		}
		if c.app.SeekStableAt.IsZero() {
			t.Fatal("seek 没有执行（SeekStableAt 未更新）")
		}
	})
}

// TestIssue42FinishPlaybackInitStale 钉住起播收尾的二次 stale 复核：旧请求
// 的慢闭包不能把新请求刚置上的 Buffering/Waiting 清掉。
func TestIssue42FinishPlaybackInitStale(t *testing.T) {
	a := view.NewApp(func() {})
	a.Buffering, a.Waiting = true, true
	a.PlaybackRequestID = 2
	c := &controller{app: a}

	if c.finishPlaybackInit(func() bool { return true }) {
		t.Fatal("stale 请求的收尾不该成功")
	}
	if !a.Buffering || !a.Waiting {
		t.Fatal("stale 收尾清掉了新请求的起播状态")
	}
	if !c.finishPlaybackInit(func() bool { return false }) {
		t.Fatal("最新请求的收尾应成功")
	}
	if a.Buffering || a.Waiting {
		t.Fatal("最新请求收尾后 Buffering/Waiting 应清零")
	}
}

// TestIssue42PlaybackSettingsUseLatestValues 钉住起播收尾下发的是**当前**
// 共享设置，而不是起播开始时抓的快照：加载中改倍速/均衡/音量不能被覆盖。
func TestIssue42PlaybackSettingsUseLatestValues(t *testing.T) {
	a := view.NewApp(func() {})
	a.Speed, a.EQ, a.Volume = 1.5, true, .35
	c := &controller{app: a, mp: media.NewPlayer()}

	c.applyCurrentPlaybackSettings()

	if got := c.mp.Speed(); got != 1.5 {
		t.Errorf("倍速 = %v，想 1.5（加载中改的值被快照覆盖）", got)
	}
	if !c.mp.EQ() {
		t.Error("均衡 = 关，想开（加载中改的值被快照覆盖）")
	}
	if got := c.mp.Volume(); got != .35 {
		t.Errorf("音量 = %v，想 .35（加载中改的值被快照覆盖）", got)
	}
}

// TestIssue42PlaybackSettingsHonorMuted 钉住静音：记住的音量非零时，起播收尾
// 仍必须下发 0（复用 c.applyVolume），不能直接下发 app.Volume 把声音放出来。
func TestIssue42PlaybackSettingsHonorMuted(t *testing.T) {
	a := view.NewApp(func() {})
	a.Speed, a.EQ = 1.25, true
	a.Volume, a.Muted = .6, true // 记住 .6，但当前是静音
	c := &controller{app: a, mp: media.NewPlayer()}

	c.applyCurrentPlaybackSettings()

	if got := c.mp.Volume(); got != 0 {
		t.Errorf("静音时音量 = %v，想 0（app.Volume 被直接下发）", got)
	}
	if got := c.mp.Speed(); got != 1.25 {
		t.Errorf("静音不应影响倍速：%v，想 1.25", got)
	}
	if !c.mp.EQ() {
		t.Error("静音不应影响均衡")
	}
}

// TestIssue42ApplyVideoStatePropagatesBusy 钉住视频占着声音时主窗播放栏的
// 状态来源：Playing 与忙态都来自 OnState，而不是已经暂停的音频引擎。
func TestIssue42ApplyVideoStatePropagatesBusy(t *testing.T) {
	c, eng := newVideoCloseController(t, false)
	a := c.app
	a.VideoOpen = true
	eng.playing = false // 音频引擎暂停，正是视频占用期间的常态

	c.applyVideoState(video.State{Time: 42, Duration: 300, Paused: false, Ready: 1, Buffering: true})
	if !a.Playing {
		t.Fatal("视频在播时主窗 Playing 应为真")
	}
	if a.VideoBusy != view.PlayerBuffering {
		t.Fatalf("视频缓冲中 VideoBusy=%v，想 PlayerBuffering", a.VideoBusy)
	}

	c.applyVideoState(video.State{Time: 43, Duration: 300, Paused: false, Ready: 0})
	if a.VideoBusy != view.PlayerLoading {
		t.Fatalf("首帧未到 VideoBusy=%v，想 PlayerLoading", a.VideoBusy)
	}

	c.applyVideoState(video.State{Time: 44, Duration: 300, Paused: true, Ready: 1})
	if a.Playing || a.VideoBusy != view.PlayerIdle {
		t.Fatalf("视频暂停时 Playing=%v Busy=%v，想 false/Idle", a.Playing, a.VideoBusy)
	}

	c.applyVideoState(video.State{Time: 44, Duration: 300, Paused: false, Ready: 0, Err: "boom"})
	if a.Playing || a.VideoBusy != view.PlayerIdle {
		t.Fatalf("视频出错时 Playing=%v Busy=%v，想 false/Idle", a.Playing, a.VideoBusy)
	}
}

// TestIssue42VideoCloseClearsBusy 钉住关窗后的忙态复位：视频窗的圆环与
// 音频的运行中等待都不能留在主窗上（Playing 仍按引擎重读）。
func TestIssue42VideoCloseClearsBusy(t *testing.T) {
	c, eng := newVideoCloseController(t, false)
	c.app.VideoOpen = true
	c.app.VideoBusy = view.PlayerBuffering
	c.app.Waiting = true
	c.app.Playing = true

	c.applyVideoClose(video.State{Time: 42, Duration: 300, Paused: true, Ready: 1, Buffering: true})

	if c.app.VideoBusy != view.PlayerIdle {
		t.Fatalf("关窗后 VideoBusy=%v，想 Idle", c.app.VideoBusy)
	}
	if c.app.Waiting {
		t.Fatal("关窗后运行中等待残留")
	}
	if c.app.Playing != eng.Playing() {
		t.Fatalf("关窗后 Playing=%v 与引擎 %v 不一致", c.app.Playing, eng.Playing())
	}
}
