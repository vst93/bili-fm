package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWindowHiddenReachesAppState 是**接线实测**：`Context.WindowHidden`
// （mygo 0.8.0）的值真的落到了 `App.WindowHidden`，而且是在 `Background`
// 早退**之前**读的。
//
// 为什么必须测：后台轮询（在线人数）靠这个字段决定要不要打接口。若它永远是
// false（比如读的位置在 `if a.Background { return }` 之后，隐藏时那一帧根本走
// 不到），轮询就停不下来 —— 而编译、vet、其它测试全都不会报错。
//
// 框架的规矩：视图「隐藏时构建一次、重新显示时再构建一次」，从它第一次问这个
// 状态开始。所以这里用 tester 真的把窗口隐藏/显示各走一帧。
func TestWindowHiddenReachesAppState(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tst.Frame()
	if a.WindowHidden {
		t.Fatal("窗口可见时 WindowHidden 不该为 true")
	}

	// **顺序很关键**：生产里先由 OnHide 把 Background 置真、随后框架给一帧
	// 「隐藏帧」（见 main.go 的 app.Win.OnHide）。无头 tester 不跑这些回调，
	// 所以要自己按同样的顺序模拟 —— 而且必须**先**置 Background：
	// `tester.SetWindowHidden` 自己也会触发一次构建，若先调它，那一次构建是在
	// Background 仍为 false 的情况下跑的，读取位置写错也照样能读到值，
	// 这条测试就失去了鉴别力（实测确认过）。
	a.SetBackground(true)
	tst.SetWindowHidden(true)
	tst.Frame()
	if !a.WindowHidden {
		t.Fatal("窗口隐藏后 App.WindowHidden 仍是 false —— Shell 里读取的位置在 " +
			"Background 早退之后？那样后台轮询就永远停不下来")
	}

	// 恢复：先让后台标志归零（OnShow 的职责），再让窗口可见。
	a.SetBackground(false)
	tst.SetWindowHidden(false)
	tst.Frame()
	if a.WindowHidden {
		t.Fatal("窗口重新显示后 App.WindowHidden 应回到 false（框架会再给一帧构建）")
	}
}

// TestWindowHiddenSameInMiniShell 保证迷你窗那条渲染路径也记录它（两条 Shell
// 是独立函数，只修一条是常见疏漏）。
func TestWindowHiddenSameInMiniShell(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Mini = true
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	a.SetBackground(true) // 模拟 OnHide（顺序说明见上一条测试）
	tst.SetWindowHidden(true)
	tst.Frame()
	if !a.WindowHidden {
		t.Fatal("迷你模式隐藏后 App.WindowHidden 仍是 false（MiniShell 漏了读取）")
	}
}
