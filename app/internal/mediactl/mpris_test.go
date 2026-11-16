//go:build linux

package mediactl

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestTrackSlugIsValidObjectPath(t *testing.T) {
	titles := []string{
		"",
		"【Hi-Res】 2026破亿热门歌曲合集🔥",
		"a/b\\c d?e",
		"纯中文标题",
		"plain",
	}
	for _, title := range titles {
		p := dbus.ObjectPath("/org/bilifm/track/" + trackSlug(title))
		if !p.IsValid() {
			t.Errorf("trackSlug(%q) → 无效对象路径 %q", title, p)
		}
	}
}

// TestCanRaise 钉住 CanRaise 与 Raise 回调同源：没有回调就必须报 false，
// 否则 GNOME / KDE 的媒体控制卡片会给出一个按了没反应的「打开播放器」键
// （mygo 0.9.2 的 NowPlaying 在 MPRIS 上也做同一件事，见 #1924）。
func TestCanRaise(t *testing.T) {
	if canRaise(Callbacks{}) {
		t.Error("没有 Raise 回调时 CanRaise 应为 false")
	}
	if !canRaise(Callbacks{Raise: func() {}}) {
		t.Error("有 Raise 回调时 CanRaise 应为 true")
	}
}

// TestRootRaiseCallsCallback 钉住 D-Bus 的 Raise 真的转给回调，且没有回调时
// 不 panic（各平台的 Controller 都会收到这个请求，只有 MPRIS 有 Raise）。
func TestRootRaiseCallsCallback(t *testing.T) {
	calls := 0
	r := &rootIface{m: &mpris{cb: Callbacks{Raise: func() { calls++ }}}}
	if err := r.Raise(); err != nil {
		t.Fatalf("Raise 返回错误：%v", err)
	}
	if calls != 1 {
		t.Fatalf("Raise 调了 %d 次回调，应为 1", calls)
	}
	if err := (&rootIface{m: &mpris{}}).Raise(); err != nil {
		t.Fatalf("没有回调时 Raise 不该报错：%v", err)
	}
}
