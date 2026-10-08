package main

import (
	_ "embed"
	"log"

	"github.com/egoist/mygo"
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
			{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) {
				c.app.Win.Show()
				c.app.Win.Focus()
			}},
			mygo.Separator(),
			{Label: "播放 / 暂停", Click: func(*mygo.MenuItem, *mygo.Window) { c.togglePlay() }},
			{Label: "上一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.step(-1) }},
			{Label: "下一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.step(1) }},
			mygo.Separator(),
			{Role: mygo.RoleQuit},
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
