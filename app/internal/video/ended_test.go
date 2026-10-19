package video

import (
	"testing"
	"time"
)

// TestManagerTracksEnded 钉住弹窗的 ended 事实源：页面 ended 事件后不再每秒
// 回传状态，单看 Paused/Time 会误判；主窗再点播放时靠这个标志走 Replay。
func TestManagerTracksEnded(t *testing.T) {
	m := New()
	if m.Ended() {
		t.Fatal("新 Manager 不应是已结束")
	}
	done := make(chan struct{}, 1)
	m.OnEnded(func() { done <- struct{}{} })
	NewService(m).Ended()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Ended 没回调 onEnded")
	}
	if !m.Ended() {
		t.Fatal("Ended 后 Manager.Ended 应为真")
	}
	m.Close()
	if m.Ended() {
		t.Fatal("Close 应清掉 ended，下一次打开从新状态开始")
	}
}
