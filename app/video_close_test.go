package main

import (
	"strings"
	"testing"

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
