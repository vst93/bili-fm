package mediactl

import "testing"

// TestNowPlayingState 钉住 macOS Now Playing 的状态映射（MPNowPlayingPlaybackState）。
// 抽成纯函数后，这条在 Linux/Windows 上也能跑；darwin 实现直接调它。
func TestNowPlayingState(t *testing.T) {
	cases := []struct {
		in   Status
		want uint64
	}{
		{Playing, 1},
		{Paused, 2},
		{Stopped, 3},
		{Status("weird"), 0},
	}
	for _, c := range cases {
		if got := nowPlayingState(c.in); got != c.want {
			t.Errorf("nowPlayingState(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
