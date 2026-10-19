package main

import (
	"runtime"
	"time"
)

// Windows 任务栏缩略图工具栏（悬停任务栏图标时预览浮层里的 上一首 /
// 播放暂停 / 下一首）。三个按钮的动作与状态同步在这里接线，具体 Win32
// 调用在 taskbar_windows.go（其它平台 taskbar_other.go 是 no-op）。
//
// 为什么原生版要自己画：mygo/WebView2 那套只包含 Blink 的 mediaSession，
// 不含 Chrome 浏览器层对 Windows SMTC 的集成，所以 Windows 上悬停任务栏
// 图标看不到任何媒体控件。Tauri 原版用 taskbar.rs 补了 ITaskbarList3::
// ThumbBarAddButtons，这里把同一件事用 syscall 重做。

// taskbarMediaActions 是任务栏按钮点击时执行的动作。声明成普通闭包，
// 让 Windows 的子类化回调不必知道 controller 的存在。
var taskbarMediaActions struct {
	PlayPause func()
	Prev      func()
	Next      func()
}

// setupTaskbar 装 Windows 任务栏缩略图工具栏；其它平台 no-op。
//
// 除了创建时调一次，还在窗口每次显示时重试：Explorer 的缩略图工具栏要等
// 任务栏按钮存在后才能挂（窗口首次显示前 ThumbBarAddButtons 会失败），
// Explorer 重启后也要重挂。taskbarInit 幂等。
func (c *controller) setupTaskbar() {
	// 动作直接调播放控制：子类化回调跑在窗口线程（就是 mygo 的事件循环
	// 线程），与托盘菜单回调同一条路径，不需要再投递主线程。
	taskbarMediaActions.PlayPause = c.togglePlay
	taskbarMediaActions.Prev = func() { c.step(-1) }
	taskbarMediaActions.Next = func() { c.step(1) }
	if runtime.GOOS != "windows" || c.app.Win == nil {
		return
	}
	// 窗口在 WhenReady 里 NewWindow 时就已 Show（WindowOptions.Hidden 为假），
	// 而 setupTaskbar 是在那之后才调用的 —— 只注册 OnShow 的话，这次显示
	// 事件早就过去了，taskbarInit 永远不会跑。用户看到的「悬停任务栏图标
	// 没有上一首/播放暂停/下一首」根因就在这里（Tauri 版是在 setup 里直接
	// 调 init 的）。
	//
	// 所以这里先立刻挂一次；再留两次短延迟重试，兜住「任务栏按钮还没建好 /
	// Explorer 正在重启」时 ThumbBarAddButtons 失败的情况（taskbarInit 幂等）。
	initTaskbar := func() {
		if c.app.Win != nil {
			taskbarInit(c.app.Win.NativeHandle())
		}
	}
	initTaskbar()
	for _, delay := range []time.Duration{600 * time.Millisecond, 2500 * time.Millisecond, 6000 * time.Millisecond} {
		go func(d time.Duration) {
			time.Sleep(d)
			if c.app.Win != nil {
				c.app.Win.Update(initTaskbar)
			}
		}(delay)
	}
	// 窗口再次显示（关闭到托盘后恢复 / 迷你模式切换）时再挂一次：句柄可能
	// 因为窗口重建而变化，Explorer 重启后缩略图工具栏也需要重挂。窗口句柄
	// 在每次回调里重新取，而不是在闭包里存第一次的 hwnd。
	c.app.Win.OnShow(initTaskbar)
}

// syncTaskbar 把播放状态与可上一首/下一首推给任务栏按钮（换图标 / 禁用态）。
// Windows 之外是空实现。与系统媒体中心同一触发点（syncMediaTrack）。
func (c *controller) syncTaskbar() {
	a := c.app
	// 播放状态每次变化都顺带幂等重挂一次：Explorer 重启/窗口句柄变化后，
	// 不必等下一次 OnShow 才把缩略图工具栏找回来。已经在位时 taskbarInit
	// 第一行就返回，不产生 COM 调用。
	if a.Win != nil {
		taskbarInit(a.Win.NativeHandle())
	}
	nav := a.CanNavigate()
	taskbarSetState(a.Playing, nav, nav)
}
