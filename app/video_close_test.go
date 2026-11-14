package main

import (
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

// stubAudio 是音频引擎的桩替身：只记「暂停 / 继续 / 是否在放」。
// 关视频窗的每条路径都要断言 app.Playing 与它一致（真引擎要声卡，无头跑不了）。
type stubAudio struct {
	playing bool
	pauses  int
	resumes int
}

func (s *stubAudio) Pause()        { s.pauses++; s.playing = false }
func (s *stubAudio) Resume()       { s.resumes++; s.playing = true }
func (s *stubAudio) Playing() bool { return s.playing }

// newVideoCloseController 建一个只够跑「关视频窗」的最小 controller：
// 一条已选中的曲目、一个可写的 KV（断点会落盘）、一个引擎桩。
//
// SeekBusy=true 让 c.seek 只记 PendingSeek、不碰真 mp / 窗口（仓库既有的
// 桩替身手法，见 newTestController）。
func newVideoCloseController(t *testing.T, enginePlaying bool) (*controller, *stubAudio) {
	t.Helper()
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := view.NewApp(func() {})
	a.SeekBusy = true
	a.Queue = []view.Track{{Aid: 1, Bvid: "BV1test", Cid: 2, Title: "测试稿件"}}
	a.Index = 0
	a.Dur = 300
	a.VideoOpen = true
	st := &stubAudio{playing: enginePlaying}
	return &controller{app: a, kv: kv, audioStub: st}, st
}

// TestVideoCloseLeavesStateMatchingEngine 定死不变式：任何「离开视频窗」的
// 路径执行完之后，主窗的播放状态必须等于音频引擎的实际状态。
//
// 用户报的症状是「视频暂停时关窗 → 声音出来了，主窗还显示暂停」：旧实现只在
// !st.Paused 时 Resume、并且手写 app.Playing，于是引擎（可能因为起播竞态已经在
// 放）和界面各说各话。这里逐条路径 × 播放/暂停都断言两者一致。
func TestVideoCloseLeavesStateMatchingEngine(t *testing.T) {
	cases := []struct {
		name string
		// st 是弹窗关窗前最后一次回传的状态。
		st video.State
		// enginePlaying 是「关窗那一刻音频引擎其实在放」（起播竞态残留 / 上一次
		// 接力没落地等，正是要清掉的那种残留）。
		enginePlaying bool
		quit          bool
		silent        bool
		noTrack       bool
		wantPlaying   bool
		wantResumes   int
	}{
		{
			name:        "暂停时关闭：不续播",
			st:          video.State{Time: 42, Duration: 300, Paused: true, Ready: 4},
			wantPlaying: false,
		},
		{
			name:          "暂停时关闭：引擎有残留也必须停",
			st:            video.State{Time: 42, Duration: 300, Paused: true, Ready: 4},
			enginePlaying: true,
			wantPlaying:   false,
		},
		{
			name:        "播放中关闭：接力（webview 既有行为）",
			st:          video.State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			wantPlaying: true,
			wantResumes: 1,
		},
		{
			name:        "自然播完：片尾不续播",
			st:          video.State{Time: 299.9, Duration: 300, Paused: true, Ready: 4},
			wantPlaying: false,
		},
		{
			name:        "自然播完但缓存的 Paused 还是旧的：靠片尾位置兜住",
			st:          video.State{Time: 300, Duration: 300, Paused: false, Ready: 4},
			wantPlaying: false,
		},
		{
			name:          "弹窗里 Ctrl/Cmd+Q 退出：引擎停、状态落地",
			st:            video.State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			enginePlaying: true,
			quit:          true,
			wantPlaying:   false,
		},
		{
			name:          "切歌静默关闭：新一集已在放，不能把它按停",
			st:            video.State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			enginePlaying: true,
			silent:        true,
			wantPlaying:   true,
		},
		{
			name:        "切歌静默关闭：新一集还没起来，状态跟着引擎停",
			st:          video.State{Time: 42, Duration: 300, Paused: true, Ready: 4},
			silent:      true,
			wantPlaying: false,
		},
		{
			name:          "没有当前曲目：引擎停",
			st:            video.State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			enginePlaying: true,
			noTrack:       true,
			wantPlaying:   false,
		},
		{
			name:        "起播没就绪就关窗：不接力",
			st:          video.State{Time: 0, Duration: 0, Paused: false, Ready: 0},
			wantPlaying: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, eng := newVideoCloseController(t, tc.enginePlaying)
			c.quitting = tc.quit
			c.videoSilent = tc.silent
			if tc.noTrack {
				c.app.Queue, c.app.Index = nil, 0
			}

			c.applyVideoClose(tc.st)

			if c.app.Playing != eng.Playing() {
				t.Fatalf("主窗 Playing=%v，引擎 Playing=%v：两边分叉（用户报的「有声音、界面显示暂停」）",
					c.app.Playing, eng.Playing())
			}
			if c.app.Playing != tc.wantPlaying {
				t.Errorf("主窗 Playing=%v，想要 %v", c.app.Playing, tc.wantPlaying)
			}
			if c.app.VideoOpen {
				t.Error("关窗后 VideoOpen 仍为真")
			}
			if eng.resumes != tc.wantResumes {
				t.Errorf("Resume 调用 %d 次，想要 %d（暂停/播完/退出都不该续播）",
					eng.resumes, tc.wantResumes)
			}
			if tc.silent && c.videoSilent {
				t.Error("静默关窗之后 videoSilent 没清掉（下一次正常关窗会被误当切歌）")
			}
			// 暂停时关窗仍要把位置接回来：再点播放应该从视频停下的地方继续。
			if !tc.quit && !tc.silent && !tc.noTrack && tc.st.Time > 0 {
				if c.app.PendingSeek != tc.st.Time || !c.app.HasPendingSeek {
					t.Errorf("位置没接回来：PendingSeek=%v HasPendingSeek=%v，想要 %v",
						c.app.PendingSeek, c.app.HasPendingSeek, tc.st.Time)
				}
			}
		})
	}
}

// TestVideoOpenedPinsAudioPaused 钉住「视频窗开着」这段时间的不变式：引擎
// 暂停、主窗 Playing 从引擎读回。原生窗、原生回退到 webview（ErrUnsupported
// / 起播失败 / 地址不合法都落在这里）共用同一个落点。
func TestVideoOpenedPinsAudioPaused(t *testing.T) {
	c, eng := newVideoCloseController(t, true) // 起播竞态：引擎已经在放
	c.app.VideoOpen = false
	c.app.Playing = true

	c.videoOpened()

	if !c.app.VideoOpen {
		t.Error("videoOpened 应把 VideoOpen 置真")
	}
	if eng.Playing() {
		t.Error("视频窗开着时音频引擎必须暂停（否则双声）")
	}
	if c.app.Playing != eng.Playing() {
		t.Errorf("主窗 Playing=%v 与引擎 %v 不一致", c.app.Playing, eng.Playing())
	}
}

// TestVideoOpenedIgnoredWhenAlreadyClosed 钉住一个残留态：开窗结果（可能来自
// 回退链路）落定之前用户已经把窗口关掉了 —— 这时不能再把 VideoOpen 置真，
// 否则主窗永远以为视频还开着（空格 / 进度条一直发给不存在的弹窗）。
func TestVideoOpenedIgnoredWhenAlreadyClosed(t *testing.T) {
	c, eng := newVideoCloseController(t, false)
	c.vid = video.New() // 没开过任何弹窗
	c.app.VideoOpen = false

	c.videoOpened()

	if c.app.VideoOpen {
		t.Error("弹窗已经关了，VideoOpen 不该被置真")
	}
	if c.app.Playing != eng.Playing() {
		t.Error("主窗状态仍要与引擎一致")
	}
}

// TestRestoreAudioAfterVideoSyncsState 钉住「视频窗没开成」的恢复路径：要恢复
// 引擎，也要把主窗状态从引擎读回；没在播过时不能凭空写成播放中。
func TestRestoreAudioAfterVideoSyncsState(t *testing.T) {
	t.Run("原来在播：恢复且状态在播", func(t *testing.T) {
		c, eng := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.Playing = false

		c.restoreAudioAfterVideo(true)

		if !eng.Playing() || !c.app.Playing {
			t.Fatalf("恢复失败：引擎=%v 主窗=%v", eng.Playing(), c.app.Playing)
		}
		if c.app.Playing != eng.Playing() {
			t.Error("主窗状态与引擎不一致")
		}
	})
	t.Run("原来暂停：不许写成播放中", func(t *testing.T) {
		c, eng := newVideoCloseController(t, false)
		c.app.VideoOpen = false
		c.app.Playing = false

		c.restoreAudioAfterVideo(false)

		if eng.Playing() {
			t.Error("本来没在播，不该 Resume 引擎")
		}
		if c.app.Playing {
			t.Error("本来没在播，主窗却显示播放中")
		}
	})
}

// TestVideoCloseUsesSingleRelayPath 钉住主程序只有一个关窗收尾入口，而且它
// 不手写 Playing：OnClose 回调里再出现 `c.app.Playing = true` 这种写法，就会
// 把「状态 = 引擎实际状态」这条不变式重新撕开（这一轮的用户报障正是它）。
func TestVideoCloseUsesSingleRelayPath(t *testing.T) {
	flat := flattenCode(t, "main.go")
	for _, want := range []string{
		"c.vid.OnClose(func(stvideo.State){",
		"c.applyVideoClose(st)",
		"ifvideo.RelayOnClose(st){",
		// 决定完引擎状态后一律从引擎读回 Playing。
		"a.Playing=c.audioPlaying()",
	} {
		if !strings.Contains(flat, want) {
			t.Errorf("关窗收尾缺少接线：%s", want)
		}
	}
	// 旧的实现（只要没暂停就直接 Resume + 手写 Playing=true）不许回来。
	start := strings.Index(flat, "func(c*controller)applyVideoClose(")
	if start < 0 {
		t.Fatal("找不到 applyVideoClose")
	}
	body := flat[start+1:]
	if next := strings.Index(body, "func(c*controller)"); next >= 0 {
		body = body[:next]
	}
	if strings.Contains(body, "c.mp.Resume()") {
		t.Error("关窗路径仍在直接调 c.mp.Resume()：应走 c.audio() 以便与状态同步")
	}
}

// ---------------------------------------------------------------- 真实点击路径

// stubChromePlayer 是 view.VideoPlayer 的纯逻辑桩：只记「在放 / 暂停 / 位置」。
type stubChromePlayer struct {
	playing bool
	ended   bool
	pos     time.Duration
	dur     time.Duration
	pauses  int
}

func (s *stubChromePlayer) Play()                   { s.playing, s.ended = true, false }
func (s *stubChromePlayer) Pause()                  { s.playing, s.pauses = false, s.pauses+1 }
func (s *stubChromePlayer) Playing() bool           { return s.playing }
func (s *stubChromePlayer) Ended() bool             { return s.ended }
func (s *stubChromePlayer) Ready() bool             { return true }
func (s *stubChromePlayer) Buffering() bool         { return false }
func (s *stubChromePlayer) Duration() time.Duration { return s.dur }
func (s *stubChromePlayer) Position() time.Duration { return s.pos }
func (s *stubChromePlayer) Seek(d time.Duration)    { s.pos = d }
func (s *stubChromePlayer) SetRate(float64)         {}
func (s *stubChromePlayer) SetVolume(float64)       {}
func (s *stubChromePlayer) SetMuted(bool)           {}

// videoExitProbe 建一个真的原生视频窗外壳（无头 Tester），并把它的
// 「退出视频播放」接到主窗真正的收尾函数 applyVideoClose 上 —— 与
// internal/video 的 chromeFor 走同一条路（OnClose → 关窗 → 主窗收尾）。
//
// 这样断言的是**用户点那颗键**之后的主窗状态，而不是直接调 applyVideoClose：
// 只测纯函数永远发现不了「按钮接错了动作链」这类错误（用户报的误退应用正是）。
func videoExitProbe(t *testing.T, st video.State, enginePlaying bool) (*controller, *stubAudio, *view.VideoChrome, *ui.Tester, *videoExitCalls) {
	t.Helper()
	c, eng := newVideoCloseController(t, enginePlaying)
	pl := &stubChromePlayer{playing: !st.Paused, pos: time.Duration(st.Time * float64(time.Second)),
		dur: time.Duration(st.Duration * float64(time.Second))}
	ch := &view.VideoChrome{
		Theme:  view.ThemeFor(view.PeriodAt(12)),
		Title:  "测试稿件",
		Pl:     pl,
		Speed:  1,
		Volume: 1,
	}
	calls := &videoExitCalls{}
	ch.OnClose = func() { calls.close.Add(1); c.applyVideoClose(st) }
	ch.OnMinimize = func() { calls.min.Add(1) }
	ch.OnQuit = func() { calls.quit.Add(1) }
	tt := ui.NewTester(func(ctx *ui.Context) { ch.View(ctx) }, 640, 360)
	tt.Frame()
	return c, eng, ch, tt, calls
}

// videoExitCalls 记外壳三条动作链被触发几次（OnQuit 走 goroutine，用原子量）。
type videoExitCalls struct {
	close, min, quit atomic.Int32
}

// TestVideoExitClickLeavesMainWindowMatchingEngine 是本轮的重点不变式，走
// **真实点击**：视频窗任何离开路径执行完之后，主窗播放按钮的状态（= app.Playing，
// 播放栏的图标与 tooltip 都只读它）必须等于音频引擎的实际状态。
//
// 三条路径逐条断言：
//
//	播放中退出 → 接力续播 → 按钮为「播放中」；
//	暂停中退出 → 不续播   → 按钮为「暂停」；
//	自然播完退出 → 不续播 → 按钮为「暂停」。
func TestVideoExitClickLeavesMainWindowMatchingEngine(t *testing.T) {
	cases := []struct {
		name          string
		st            video.State
		enginePlaying bool
		wantPlaying   bool
		wantResumes   int
	}{
		{
			name:        "播放中退出：接力续播，按钮变播放中",
			st:          video.State{Time: 42, Duration: 300, Paused: false, Ready: 4},
			wantPlaying: true,
			wantResumes: 1,
		},
		{
			name:          "暂停中退出：不续播，按钮是暂停",
			st:            video.State{Time: 42, Duration: 300, Paused: true, Ready: 4},
			enginePlaying: true, // 起播竞态的残留
			wantPlaying:   false,
		},
		{
			name:        "自然播完退出：不续播，两边都是暂停",
			st:          video.State{Time: 300, Duration: 300, Paused: false, Ready: 4},
			wantPlaying: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, eng, _, tt, _ := videoExitProbe(t, tc.st, tc.enginePlaying)
			if err := tt.Click("退出视频播放"); err != nil {
				t.Fatalf("点「退出视频播放」失败：%v", err)
			}
			tt.Frame()

			if c.app.Playing != eng.Playing() {
				t.Fatalf("主窗 Playing=%v，引擎 Playing=%v：按钮与实际出声分叉", c.app.Playing, eng.Playing())
			}
			if c.app.Playing != tc.wantPlaying {
				t.Errorf("主窗 Playing=%v，想要 %v", c.app.Playing, tc.wantPlaying)
			}
			if eng.resumes != tc.wantResumes {
				t.Errorf("Resume 调用 %d 次，想要 %d", eng.resumes, tc.wantResumes)
			}
			if c.app.VideoOpen {
				t.Error("退出视频播放之后 VideoOpen 仍为真（主窗以为视频还开着）")
			}
			// 接力那条路上，接下来几次引擎心跳还要继续把状态往引擎上对
			//（Resume 是请求，引擎可能晚几十毫秒才真的出声）。
			if tc.wantPlaying && !time.Now().Before(c.playingSettle) {
				t.Error("接力之后没有记跟随窗口：引擎晚到的那一声没人管，按钮会停在暂停")
			}
		})
	}
}

// TestVideoExitClickNeverQuitsApp 用桩钉死用户报的最严重那条：点视频窗的
// 关闭**绝不能**退应用 —— quit / OnQuit / 主窗关闭回调一条都不许被踩到。
func TestVideoExitClickNeverQuitsApp(t *testing.T) {
	c, _, _, tt, calls := videoExitProbe(t, video.State{Time: 42, Duration: 300, Paused: false, Ready: 4}, false)
	if err := tt.Click("退出视频播放"); err != nil {
		t.Fatalf("点「退出视频播放」失败：%v", err)
	}
	tt.Frame()
	if got := calls.quit.Load(); got != 0 {
		t.Errorf("点视频窗的关闭触发了 OnQuit %d 次：视频窗里任何按钮都不许退应用", got)
	}
	if got := calls.close.Load(); got != 1 {
		t.Errorf("OnClose 调用 %d 次，想要 1", got)
	}
	if got := calls.min.Load(); got != 0 {
		t.Errorf("退出视频播放误触了最小化 %d 次", got)
	}
	if c.quitting {
		t.Error("点视频窗的关闭把 quitting 置位了（等于走上退出流程）")
	}
}

// TestVideoMinimizeKeepsStateAndEngine 钉住最小化：窗口最小化，播放继续、
// 声音继续，主窗状态不变（最小化 ≠ 暂停，也不是「离开视频窗」）。
func TestVideoMinimizeKeepsStateAndEngine(t *testing.T) {
	c, eng, ch, tt, calls := videoExitProbe(t, video.State{Time: 42, Duration: 300, Paused: false, Ready: 4}, false)
	if err := tt.Click("最小化"); err != nil {
		t.Fatalf("点最小化失败：%v", err)
	}
	tt.Frame()
	if !c.app.VideoOpen {
		t.Error("最小化被当成了关闭：VideoOpen 变成假")
	}
	if c.app.Playing != eng.Playing() {
		t.Errorf("最小化之后状态分叉：主窗=%v 引擎=%v", c.app.Playing, eng.Playing())
	}
	if eng.resumes != 0 || eng.pauses != 0 {
		t.Errorf("最小化动了音频引擎：resume=%d pause=%d", eng.resumes, eng.pauses)
	}
	if got := calls.min.Load(); got != 1 {
		t.Errorf("最小化：OnMinimize 调用 %d 次，想要 1", got)
	}
	if got := calls.quit.Load(); got != 0 {
		t.Errorf("最小化触发了 OnQuit %d 次", got)
	}
	if got := calls.close.Load(); got != 0 {
		t.Errorf("最小化被当成了退出视频播放（OnClose %d 次）", got)
	}
	// 「恢复」：再建几帧，画面与进度接续 —— 播放器状态与外壳状态都不许变。
	for i := 0; i < 3; i++ {
		tt.Advance(16 * time.Millisecond)
	}
	if ch.Pl.Playing() == false {
		t.Error("恢复后视频播放器停在暂停")
	}
	if c.app.Playing != eng.Playing() {
		t.Error("恢复之后主窗状态与引擎分叉")
	}
}

// lateAudio 模拟「Resume 只是请求」：引擎在流水线重建期间先报一次「还没在放」，
// 下一次心跳才报在放（用户听到声音时按钮还停在暂停，就是这一格）。
type lateAudio struct {
	playing bool
	late    int
	pauses  int
	resumes int
}

func (s *lateAudio) Pause()  { s.pauses++; s.playing, s.late = false, 0 }
func (s *lateAudio) Resume() { s.resumes++; s.playing, s.late = true, 1 }
func (s *lateAudio) Playing() bool {
	if s.late > 0 {
		s.late--
		return false
	}
	return s.playing
}

// TestVideoCloseFollowsLateEngineStart 钉住接力的**落地**：Resume 之后引擎
// 晚一步才真的出声时，主窗按钮不能永远停在暂停 —— 跟随窗口里下一次引擎
// 心跳（wirePlayer 的 OnProgress → syncPlayingFromEngine）要把它对平。
func TestVideoCloseFollowsLateEngineStart(t *testing.T) {
	c, _ := newVideoCloseController(t, false)
	late := &lateAudio{}
	c.audioStub = late

	c.applyVideoClose(video.State{Time: 42, Duration: 300, Paused: false, Ready: 4})
	if late.resumes != 1 {
		t.Fatalf("接力没发生：Resume %d 次", late.resumes)
	}
	// 采样落在过渡态：这一刻按钮还是暂停（旧实现的最终状态，用户报的就是它）。
	if c.app.Playing {
		t.Fatal("过渡态采样不该直接写成播放中（应当从引擎读回）")
	}
	if !time.Now().Before(c.playingSettle) {
		t.Fatal("没有记跟随窗口，晚到的引擎状态没人管")
	}
	// 下一次引擎心跳。
	c.syncPlayingFromEngine()
	if !c.app.Playing || c.app.Playing != late.Playing() {
		t.Fatalf("跟随之后主窗 Playing=%v，引擎=%v：按钮与出声仍不一致", c.app.Playing, late.Playing())
	}
	// 窗口期过了就不再插手（平时 Playing 归播放键 / 起播收尾管）。
	c.playingSettle = time.Now().Add(-time.Millisecond)
	c.app.Playing = true
	late.Pause()
	c.syncPlayingFromEngine()
	if !c.app.Playing {
		t.Error("跟随窗口过期后仍在改写 Playing（会和播放键抢状态）")
	}
}

// TestVideoCloseSyncWiredToEngineHeartbeat 是源码契约：跟随只能挂在音频引擎
// 的心跳上（reportLoop 每 500ms 一次），不能另起定时器（测试里会留后台
// goroutine，界面线程上也会和播放键抢）。
func TestVideoCloseSyncWiredToEngineHeartbeat(t *testing.T) {
	flat := flattenCode(t, "main.go")
	if !strings.Contains(flat, "c.mp.OnProgress(func(pos,durfloat64){runOnUI(c.app,!c.app.Hidden(),func(){") {
		t.Error("wirePlayer 的 OnProgress 形状变了：跟随窗口的挂点没了")
	}
	if !strings.Contains(flat, "c.syncPlayingFromEngine()") {
		t.Error("OnProgress 里没有 syncPlayingFromEngine：引擎晚到的那一声没人管")
	}
	// 跟随窗口自己不许起定时器：只查这两个函数的函数体（main.go 别处
	// 用 AfterFunc 是别的事）。
	for _, fn := range []string{
		"func(c*controller)applyVideoClose(",
		"func(c*controller)syncPlayingFromEngine(",
	} {
		start := strings.Index(flat, fn)
		if start < 0 {
			t.Fatalf("找不到 %s", fn)
		}
		body := flat[start+1:]
		if next := strings.Index(body, "func(c*controller)"); next >= 0 {
			body = body[:next]
		}
		if strings.Contains(body, "time.AfterFunc") || strings.Contains(body, "go func()") {
			t.Errorf("%s 里起了 goroutine / 定时器：跟随应挂在引擎心跳上", fn)
		}
	}
}
