package mediactl

// MPNowPlayingPlaybackState 的取值（MediaPlayer 框架，macOS 10.12.2+）：
// 0 unknown / 1 playing / 2 paused / 3 stopped / 4 interrupted。
const (
	nowPlayingUnknown = 0
	nowPlayingPlaying = 1
	nowPlayingPaused  = 2
	nowPlayingStopped = 3
)

// nowPlayingState 把播放状态映射成 MPNowPlayingPlaybackState。抽成纯函数，
// 便于在没有 macOS 的机器上单测（darwin 实现直接调它）。
func nowPlayingState(s Status) uint64 {
	switch s {
	case Playing:
		return nowPlayingPlaying
	case Paused:
		return nowPlayingPaused
	case Stopped:
		return nowPlayingStopped
	}
	return nowPlayingUnknown
}
