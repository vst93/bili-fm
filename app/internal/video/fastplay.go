package video

import "time"

const maxNormalHLSRate = 2

func PreferProgressive(platform string, rate float64) bool {
	return platform == "darwin" && rate > maxNormalHLSRate
}

func (m *Manager) wireNativeSource(n *nativeWindow, o Options, platform string) {
	m.wireNativeFail(n, o)
	n.openProgressive = nil
	if platform == "darwin" && o.SrcFallback != "" {
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
