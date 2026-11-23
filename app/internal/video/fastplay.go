package video

import "time"

const maxNormalHLSRate = 2

// PreferProgressive 报告「这一集在 >2 倍速时要不要改用合流 MP4」。
//
// 两条平台各有各的理由，但结论一样：本地 HLS 分片在 >2 倍速下不可靠。
//
//   - macOS：AVFoundation 在 rate > 2.0 时**根本不给** AVPlayerItemVideoOutput
//     画面（时钟与声音照走），所以 >2 倍速必须换成渐进式源 + 压缩时间轴
//     （见 mygo 的 timelineFor，2026-11-19 实测）。
//   - Windows：IMFMediaEngine 的 SetPlaybackRate 对**文件源**支持到 16 倍速
//     （超过 2 倍速用抽帧），但流式源（HLS / HTTP）不保证；Media Foundation
//     的 HLS 源在 >2 倍速下常常直接拒绝这次 SetPlaybackRate，而 mygo 的
//     mfEngine.SetRate 不检查 HRESULT —— 用户看到的就是「3 倍速点了没反应 /
//     画面不动」。合流 MP4 有确定时长与文件语义，抽帧那条路才成立。
//
// Linux 保持分片源：GStreamer 的 hlsdemux + flushing accurate seek 在 3 倍速
// 下实测正常（见 .pending-bugs.md 的 dashhls 实测一节），换成合流反而要先把
// 几 MB 的 moov 读完，起播更慢。
func PreferProgressive(platform string, rate float64) bool {
	return rate > maxNormalHLSRate && progressivePlatform(platform)
}

// progressivePlatform 报告这个平台在 >2 倍速时是否需要合流 MP4（理由见
// PreferProgressive）。开窗选源与播放中提速共用它，两边不会分叉。
func progressivePlatform(platform string) bool {
	return platform == "darwin" || platform == "windows"
}

func (m *Manager) wireNativeSource(n *nativeWindow, o Options, platform string) {
	m.wireNativeFail(n, o)
	n.openProgressive = nil
	// 播放中提速到 >2 倍速时原地换源：与开窗时的选源同一条判据，两边不能
	// 分叉（否则「1 倍速打开、再点 3 倍速」会走到没人处理的那条路）。
	if progressivePlatform(platform) && o.SrcFallback != "" {
		n.openProgressive = func() (playerAPI, error) { return m.openNativePlayer(o.SrcFallback) }
	}
}

func (n *nativeWindow) prepareFastPlayback() {
	if n.openProgressive == nil || n.settings.Speed <= maxNormalHLSRate || n.pl.Ended() || n.pl.Err() != nil {
		return
	}
	open := n.openProgressive
	n.openProgressive = nil
	n.fallbackTried = true
	next, err := open()
	if err != nil {
		if failed := n.onFailed; failed != nil {
			n.onFailed = nil
			failed(err)
		}
		return
	}
	old := n.pl
	if !old.Playing() {
		next.Pause()
	}
	next.SetMuted(n.settings.Muted)
	next.SetVolume(n.settings.Volume)
	next.SetRate(n.settings.Speed)
	pos := old.Position()
	if !old.Ready() && pos == 0 {
		pos = time.Duration(n.startAt * float64(time.Second))
	}
	if pos > 0 {
		next.Seek(pos)
	}
	n.pl = next
	n.haveState = false
	old.Close()
}
