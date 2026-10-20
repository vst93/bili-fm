package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestDrawerChangeNotifiesOncePerChange 钉住抽屉回收回调的语义：只在 Drawer
// 的 key 真的变化时通知一次，不随每帧重建重复触发。
//
// 这条回调替代了旧 memoryWatch 的每秒 Win.Update 轮询 —— 轮询本身会请求
// 一帧，让渲染器永远等不到 idle trim（macOS 上那份 GPU 工作集有 45MB 级）。
func TestDrawerChangeNotifiesOncePerChange(t *testing.T) {
	a := NewApp(func() {})
	var n int
	a.OnDrawerChange = func() { n++ }
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)

	tst.Frame()
	if n != 0 {
		t.Fatalf("首帧（Drawer 没变）就通知了 %d 次", n)
	}
	a.Drawer = "feed"
	tst.Frame()
	if n != 1 {
		t.Fatalf("打开抽屉后通知了 %d 次，want 1", n)
	}
	// 同一抽屉的后续帧不应重复通知（否则等于又回到每帧 trim）。
	tst.Frame()
	tst.Frame()
	if n != 1 {
		t.Fatalf("同一抽屉的后续帧又通知了：%d 次", n)
	}
	a.Drawer = ""
	tst.Frame()
	if n != 2 {
		t.Fatalf("关闭抽屉后共通知 %d 次，want 2", n)
	}
}
