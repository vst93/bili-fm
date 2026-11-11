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

// TestShellSkipsBuildWhenWindowHidden 钉住「看不见就别建树」：窗口被别的窗口挡住 /
// 切到别的桌面（框架的 WindowHidden，而不是 OnHide 那条 Background）时，Shell
// 必须整棵早退。
//
// 为什么值得测：框架会把不可见窗口的**绘制**按下去（ui/runtime.go 的 holdFrame），
// 但**构建**照跑 —— 实测不可见时一帧 1.02ms（可见 21.97ms）。那 1ms 是纯浪费，
// 而这只有我们自己早退才省得掉（用户 2026-11-11 要求「看不见就别渲染」）。
func TestShellSkipsBuildWhenWindowHidden(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tst.Frame()
	if _, ok := tst.Find("设置"); !ok {
		t.Fatal("窗口可见时应画出标题栏设置键")
	}

	// 只让窗口「不可见」，**不动** Background —— 正是「被盖住 / 在别的桌面」那种。
	tst.SetWindowHidden(true)
	BackdropLayers() // 丢掉轮询期的计数
	tst.Frame()
	if !a.WindowHidden {
		t.Fatal("App.WindowHidden 没跟上（早退位置在读取之前？）")
	}
	if got := BackdropLayers(); got != 0 {
		t.Errorf("不可见时仍在建树：backdrop 层数 = %d，应为 0（Shell 应整棵早退）", got)
	}
	for _, label := range []string{"设置", "标题", "播放"} {
		if _, ok := tst.Find(label); ok {
			t.Errorf("不可见时仍在建树：找得到 %q", label)
		}
	}

	// 恢复显示：框架会再构建一次，树必须回来（不能停在空界面上）。
	tst.SetWindowHidden(false)
	tst.Frame()
	if _, ok := tst.Find("设置"); !ok {
		t.Fatal("重新显示后 Shell 没有重新建树")
	}
}

// TestHiddenAtomicMirror 钉住 WindowHidden 的原子镜像：Shell（UI 线程）写它，
// 播放器的上报 goroutine 读它来决定「这次更新要不要顺带请求一帧」。
//
// 读裸字段会是数据竞争，所以 App 里另存一份 atomic.Bool，由 noteHidden 一起写。
func TestHiddenAtomicMirror(t *testing.T) {
	a := NewApp(func() {})
	if a.Hidden() {
		t.Error("默认应可见")
	}
	a.noteHidden(true)
	if !a.WindowHidden || !a.Hidden() {
		t.Error("noteHidden(true) 应同时写字段与原子量")
	}
	a.noteHidden(false)
	if a.WindowHidden || a.Hidden() {
		t.Error("noteHidden(false) 应同时清字段与原子量")
	}
}
