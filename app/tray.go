package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

//go:embed resources/tray.png
var trayIcon []byte

// setupTray 装系统托盘。关闭窗口是隐藏到托盘而不是退出（与旧版一致），
// 所以托盘是唯一的恢复入口，必须装成功。
func (c *controller) setupTray() {
	tray, err := mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon,
		IconIsTemplate: true, // macOS 会按菜单栏明暗着色
		ToolTip:        "bili-FM",
		Menu: mygo.NewMenu([]*mygo.MenuItem{
			{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { c.showWindow() }},
			mygo.Separator(),
			{Label: "播放 / 暂停", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("togglePlay", c.togglePlay) }},
			{Label: "上一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("prev", func() { c.step(-1) }) }},
			{Label: "下一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("next", func() { c.step(1) }) }},
			mygo.Separator(),
			// 更新插件的菜单项：点开是 Sparkle 风格的更新窗口。
			// 它自己也做每日后台检查，这里只是给用户一个手动入口。
			updater.MenuItem(),
			mygo.Separator(),
			// 用自己的退出项而不是 RoleQuit：RoleQuit 走 App.Quit()，
			// 而「关窗=隐藏到托盘」的 OnClose 会把它拦掉（Role 那条路
			// 不经过我们的 quit，看不到 quitting 标志）。这里统一走 c.quit。
			{Label: "退出 bili-FM", Click: func(*mygo.MenuItem, *mygo.Window) { c.quit() }},
		}),
	})
	if err != nil {
		log.Printf("托盘创建失败（关窗后将无法恢复窗口）: %v", err)
		return
	}
	_ = tray
}

// setupMediaKeys 注册全局媒体键。
//
// macOS 上 mygo 的全局快捷键走 Carbon RegisterEventHotKey，没有媒体键的
// 键码，注册会失败——那里静默降级，靠 WebKit 自带的 Media Session（系统
// 「正在播放」）接管媒体键。Windows / Linux 正常。
func (c *controller) setupMediaKeys() {
	register := func(acc string, fn func()) {
		if err := mygo.GlobalShortcut.Register(acc, fn); err != nil {
			log.Printf("媒体键 %s 注册失败（可能被其他程序占用）: %v", acc, err)
		}
	}
	register("MediaPlayPause", c.togglePlay)
	register("MediaNextTrack", func() { c.step(1) })
	register("MediaPreviousTrack", func() { c.step(-1) })
}

// trayAction 执行一个托盘菜单动作。菜单回调不一定是主线程上的
// mygo 事件循环里，所以这里打印一行日志便于排查「点了没反应」——
// 有日志说明回调确实进来了，问题在动作本身；没日志就是菜单没接上。
func (c *controller) trayAction(name string, fn func()) {
	log.Printf("托盘菜单：%s", name)
	fn()
}

// showWindow 从托盘恢复主窗口。除了 Show/Focus，还要处理「窗口最小化」
// 的状态：只 Show 不会把最小化的窗口恢复出来。
func (c *controller) showWindow() {
	log.Printf("托盘菜单：显示主窗口")
	w := c.app.Win
	w.Show()
	if w.IsMinimized() {
		w.Restore()
	}
	w.Focus()
}
