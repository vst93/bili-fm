package video

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// TestManagerDanmakuState 钉住视频管理器这一层的状态：默认开、条目按 cid
// 记录、设置原样下发（窗口不存在时也不丢，等窗口起来再套用）。
func TestManagerDanmakuState(t *testing.T) {
	m := New()
	if _, _, on := m.DanmakuState(); !on {
		t.Fatal("默认应开着弹幕")
	}
	items := []danmaku.Item{{Time: 1, Text: "一"}, {Time: 2, Text: "二"}}
	m.SetDanmaku(42, items)
	cid, n, _ := m.DanmakuState()
	if cid != 42 || n != 2 {
		t.Errorf("弹幕状态 = (cid=%d, n=%d)，想要 (42, 2)", cid, n)
	}
	m.SetDanmakuOn(false)
	if _, _, on := m.DanmakuState(); on {
		t.Error("关掉后仍报告开着")
	}
	s := danmaku.Settings{On: false, Opacity: 0.5, Size: 2, Area: 0.5, Max: 30}
	m.SetDanmakuSettings(s)
	if got := m.DanmakuSettings(); got != s {
		t.Errorf("设置 = %+v，想要 %+v", got, s)
	}
	// 脏值经 Normalize 后下发（0 不透明度 / 负上限不能原样进排版）。
	m.SetDanmakuSettings(danmaku.Settings{Opacity: 0, Max: -1})
	got := m.DanmakuSettings()
	if got.Opacity < 0.3 || got.Max <= 0 {
		t.Errorf("脏设置没有归一：%+v", got)
	}
}
