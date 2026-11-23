package main

// issue #42：起播初始化与运行中等待是两套状态。这里只钉控制器侧的接线：
//   - 初始化期间点播放不能去 Resume 旧/空引擎；
//   - 运行中等待不挡 seek（只有初始化才排队）；
//   - 起播收尾的 stale 复核真的保护新请求的忙态；
//   - 视频→主窗回传忙态，关窗/失败时清掉。

import (
	"os"
	"strings"
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

// --- 开窗忙态（点击 → 取地址 → 建窗） ---

// TestIssue42VideoOpeningFlagLifecycle 钉住 VideoOpening 的每条出口都清干净。
//
// 漏清的表现是播放键永远转圈：用户点了播放、窗口开失败（或他提前关掉），
// 界面却一直停在「加载中…」，看起来像卡死。
func TestIssue42VideoOpeningFlagLifecycle(t *testing.T) {
	t.Run("开成时清掉", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.VideoOpening = true
		c.videoOpened()
		if c.app.VideoOpening {
			t.Error("窗口开出来后 VideoOpening 没清")
		}
		if !c.app.VideoOpen {
			t.Error("VideoOpen 没置真")
		}
	})
	t.Run("窗口在结果落定前被关掉时清掉", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.VideoOpening = true
		// vid 为 nil 时 videoOpened 不会走「已经关掉」那一条（那条要求
		// c.vid != nil），所以这里用关窗收尾覆盖「用户提前关窗」：
		// 它同样必须把忙态清掉。
		c.applyVideoClose(video.State{Time: 0, Duration: 300, Paused: true, Ready: 0})
		if c.app.VideoOpening {
			t.Error("用户提前关窗后 VideoOpening 没清")
		}
		if c.app.VideoOpen {
			t.Error("窗口已经关了却把 VideoOpen 置真")
		}
	})
	t.Run("关窗时清掉", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpening = true
		c.applyVideoClose(video.State{Time: 10, Duration: 300, Paused: true, Ready: 4})
		if c.app.VideoOpening {
			t.Error("关窗后 VideoOpening 没清")
		}
	})
	t.Run("切集时清掉", func(t *testing.T) {
		c, _ := newVideoCloseController(t, false)
		c.app.VideoOpening = true
		c.videoOpenID.Add(1)
		c.finishVideoOpen()
		if c.app.VideoOpening {
			t.Error("切集后 VideoOpening 没清")
		}
	})
}

// TestIssue42VideoOpeningStaleOpenCancelled 钉住「切集之后旧的那次开窗作废」。
//
// 取地址 + 取索引要几百毫秒：这期间用户点下一集，旧结果回来时不能把窗口
// 开成上一集（用户看到的是「点了下一集，播放的却是上一集」）。
func TestIssue42VideoOpeningStaleOpenCancelled(t *testing.T) {
	c, _ := newVideoCloseController(t, false)
	c.app.VideoOpen = false

	// 第一次开窗拿到的代号。
	c.videoOpenID.Add(1)
	openID := c.videoOpenID.Load()
	stale := func() bool { return c.videoOpenID.Load() != openID }
	if stale() {
		t.Fatal("刚取的代号不该已经过期")
	}
	// 用户切集：代号往前走一格。
	c.videoOpenID.Add(1)
	if !stale() {
		t.Fatal("切集之后旧代号必须过期（否则会开成上一集）")
	}
}

// TestIssue42VideoOpeningSnapshotsUIFields 钉住 openVideo 的 goroutine 不直接
// 读界面字段。
//
// a.Pos / a.Speed / a.Muted / a.Volume / a.VideoPinned 都是界面线程写的；
// 后台 goroutine 读同一块内存就是数据竞争（用户拖进度条 / 调音量时后台正在
// 读）。这里做源码级契约：仓库既有的做法（见 TestBackdropUpdatesGateOnHidden）。
func TestIssue42VideoOpeningSnapshotsUIFields(t *testing.T) {
	flat := flattenCode(t, "main.go")
	// 起 goroutine 之前必须先取好这几份快照。
	for _, want := range []string{
		"startAt,speed:=c.app.Pos,c.app.Speed",
		"muted:=c.app.Muted||c.app.Volume<=0",
		"pinned:=c.app.VideoPinned",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("openVideo 没有在界面线程取快照：%q", want)
		}
	}
	// goroutine 体内不许再出现这些界面字段（快照之后只准用局部变量）。
	//
	// 锚点不再用「goroutine 的第一句」：openVideo 现在并发取合流地址与 DASH
	// 轨道，goroutine 开头是 var 声明 + 两个内部 goroutine，按第一句切会把
	// 后半段漏掉。这里直接取整个 openVideo 函数体（快照那几句用的是局部变量
	// 赋值，不会命中下面的 bad 模式）。
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	flatFn := strings.Join(strings.Fields(stripComments(
		sourceBlock(t, src, "func (c *controller) openVideo() {"))), "")
	// 并发取地址：两条接口都必须在 fetchWait 的两个 goroutine 里，起播延迟
	// 只等较慢的那一个（串行时是两次 RTT）。
	for _, want := range []string{
		"fetchWait.Add(2)",
		"u=c.bl.GetUrlByCid(aid,cid)",
		"dashV,dashA,dashDur,dashOK=c.bl.GetDashAVByCid(aid,cid)",
	} {
		if !strings.Contains(flatFn, want) {
			t.Errorf("openVideo 没有并发取地址：%q", want)
		}
	}
	// 只检查后台 goroutine 体内（快照那几句在 go 之前，用的是界面字段本身）。
	body := flatFn
	if i := strings.Index(flatFn, "gofunc(){"); i >= 0 {
		body = flatFn[i:]
	} else {
		t.Fatal("找不到 openVideo 的后台 goroutine（接线改了？）")
	}
	for _, bad := range []string{
		"StartAt:c.app.Pos",
		"Speed:c.app.Speed",
		"c.app.Muted",
		"c.app.Volume",
		"VideoPinned:c.app.VideoPinned",
		"DanmakuCid:t.Cid",
	} {
		if strings.Contains(body, bad) {
			t.Errorf("后台 goroutine 仍在直接读界面字段：%q", bad)
		}
	}
}
