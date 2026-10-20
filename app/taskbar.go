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
	// 关键顺序（Microsoft 的 Taskbar Extensions 文档）：
	//   Explorer 创建任务栏按钮后才发 TaskbarButtonCreated，**收到这条消息
	//   之前不能调用任何 ITaskbarList3 方法**（包括 HrInit / AddButtons）。
	//
	// 之前我们在 Show 之前就调了一次 AddButtons：隐藏窗口上它可能返回
	// S_OK 却根本不建工具栏，还会让 added 记录把后续重试全挡住。现在 Show
	// 之前只装子类化钩子，第一次真正 Add 一定发生在窗口显示之后。
	initTaskbar := func(force bool) {
		if c.app.Win != nil {
			hwnd := c.app.Win.NativeHandle()
			// 先装消息钩子，再尝试挂；TaskbarButtonCreated 到达时还会再挂
			// 一次（官方要求的时机）。
			taskbarWatch(hwnd)
			// 窗口还没真正显示时不要碰 ITaskbarList3（-play 等启动参数
			// 可能在 Show 之前就走到 syncTaskbar）。
			if !c.app.Win.IsVisible() {
				return
			}
			if force {
				taskbarReinit(hwnd)
			} else {
				taskbarInit(hwnd)
			}
		}
	}
	// Show 之前只装消息钩子，绝不碰 ITaskbarList3（见上面的关键顺序）。
	// TaskbarButtonCreated 到达时子类化过程会立刻 Add；下面三次 Show 之后的
	// 强制重试兜住「消息没接住 / 任务栏按钮比窗口晚建好」的情况。第一次
	// 调用在 600ms，此时窗口已显示，不会重演隐藏窗口上的假成功。
	if c.app.Win != nil {
		taskbarWatch(c.app.Win.NativeHandle())
	}
	for _, delay := range []time.Duration{600 * time.Millisecond, 2500 * time.Millisecond, 6000 * time.Millisecond} {
		go func(d time.Duration) {
			time.Sleep(d)
			if c.app.Win != nil {
				c.app.Win.Update(func() { initTaskbar(true) })
			}
		}(delay)
	}
	// 窗口再次显示（关闭到托盘后恢复 / 迷你模式切换）时只补装消息钩子：
	// 句柄可能因为窗口重建而变化。真正的 Add 仍交给 TaskbarButtonCreated
	// （Show 之后 Explorer 才会建按钮并发消息）；在 OnShow 里立刻 Add 仍
	// 可能早于那条消息，正是文档禁止的调用时机。
	c.app.Win.OnShow(func() {
		if c.app.Win != nil {
			taskbarWatch(c.app.Win.NativeHandle())
		}
	})
}

// syncTaskbar 把播放状态与可上一首/下一首推给任务栏按钮（换图标 / 禁用态）。
// Windows 之外是空实现。与系统媒体中心同一触发点（syncMediaTrack）。
func (c *controller) syncTaskbar() {
	a := c.app
	// 播放状态每次变化都顺带幂等重挂一次：Explorer 重启/窗口句柄变化后，
	// 不必等下一次 OnShow 才把缩略图工具栏找回来。已经在位时 taskbarInit
	// 第一行就返回，不产生 COM 调用。
	if a.Win != nil && a.Win.IsVisible() {
		hwnd := a.Win.NativeHandle()
		// 没收到 TaskbarButtonCreated 时普通幂等 init 可能被「假 added」
		// 挡住；用强制重挂兜底（播放状态变化本身就是一次新的尝试时机）。
		if taskbarButtonReady(hwnd) {
			taskbarInit(hwnd)
		} else {
			taskbarReinit(hwnd)
		}
	}
	nav := a.CanNavigate()
	taskbarSetState(a.Playing, nav, nav)
}
