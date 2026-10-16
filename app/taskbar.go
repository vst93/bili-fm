package main

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
	if c.app.Win == nil {
		return
	}
	hwnd := c.app.Win.NativeHandle()
	taskbarInit(hwnd)
	c.app.Win.OnShow(func() { taskbarInit(hwnd) })
}

// syncTaskbar 把播放状态与可上一首/下一首推给任务栏按钮（换图标 / 禁用态）。
// Windows 之外是空实现。与系统媒体中心同一触发点（syncMediaTrack）。
func (c *controller) syncTaskbar() {
	a := c.app
	nav := a.CanNavigate()
	taskbarSetState(a.Playing, nav, nav)
}
