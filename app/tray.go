package main

import (
	_ "embed"
	"log"
	"runtime"

	"github.com/egoist/mygo"
)

// trayIconColored 是 Windows / Linux 用的彩色托盘图（白底圆角方块 + 青色台标）。
//
//go:embed resources/tray.png
var trayIconColored []byte

// trayIconTemplate 是 macOS 用的**模板图**（black + alpha）：整张图是黑色的，
// 形状由 alpha 给出（logo 的电视机轮廓 + 两只眼睛 + 嘴，无背景方块）。macOS
// 会按菜单栏明暗把它染成黑或白。
//
//go:embed resources/tray-template.png
var trayIconTemplate []byte

// trayIcon 选托盘图标与是否当作 macOS 模板图。
//
// 为什么 macOS 不能用那张彩色图：mygo 把它整张发给 NSImage 并设 isTemplate，
// 而模板图 macOS **只看 alpha 通道**。彩色 tray.png 是一块几乎全不透明的白色
// 圆角方块，alpha 下就是一个实心方块 —— 台标全被抹平，用户看到的是「空白」；
// 台标本体只占方块 ~37%，缩到菜单栏高度后更显小。换成本图后 logo 占画布 ~92%
// 宽、轮廓清晰。
func trayIcon() (data []byte, template bool) {
	return trayIconFor(runtime.GOOS)
}

// trayIconFor 是 trayIcon 的纯函数形式，把平台作为参数便于跨平台断言。
func trayIconFor(goos string) (data []byte, template bool) {
	if goos == "darwin" {
		return trayIconTemplate, true
	}
	return trayIconColored, false
}

// setupTray 装系统托盘。关闭窗口是隐藏到托盘而不是退出（与旧版一致），
// 所以托盘是唯一的恢复入口，必须装成功。
func (c *controller) setupTray() {
	icon, template := trayIcon()
	// 菜单先在手里建好，怎么显示按平台分流：mygo 的 Tray 一旦 SetMenu，
	// macOS 的左键就会被 NSStatusItem 的菜单吃掉；AppIndicator（Linux）
	// 更是只能显示菜单、拿不到点击事件。所以：
	//   Windows —— 左键 OnClick 显示主窗口，右键弹菜单（THBN 左键不拦菜单）；
	//   macOS   —— 不常驻菜单，左键 OnClick 显示主窗口，右键临时挂菜单再弹；
	//   Linux   —— 只能常驻菜单（点击即菜单），第一项就是「显示主窗口」。
	// 这样三个平台点 logo 都能回到窗口（Linux 多一步菜单项）。
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "显示主窗口", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("show", c.showWindow) }},
		mygo.Separator(),
		{Label: "播放 / 暂停", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("togglePlay", c.togglePlay) }},
		{Label: "上一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("prev", func() { c.step(-1) }) }},
		{Label: "下一集", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("next", func() { c.step(1) }) }},
		mygo.Separator(),
		// 「检查更新」：与设置菜单 / 原生菜单栏走同一条应用内流程
		//（弹液态玻璃对话框，而不是 updater 插件的原生窗口）。
		// 所有菜单项一律经 trayAction 派发（线程 + 日志一条路），
		// 不要有「这一项直接调」的例外——那正是 Linux 上时好时坏的来源。
		{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("checkUpdate", c.menuCheckUpdate) }},
		mygo.Separator(),
		// 用自己的退出项而不是 RoleQuit：RoleQuit 走 App.Quit()，
		// 而「关窗=隐藏到托盘」的 OnClose 会把它拦掉（Role 那条路
		// 不经过我们的 quit，看不到 quitting 标志）。这里统一走 c.quit。
		{Label: "退出 bili-FM", Click: func(*mygo.MenuItem, *mygo.Window) { c.trayAction("quit", c.quit) }},
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{
		// 稳定的 ID：Windows 上保留用户把图标放进溢出区的选择，macOS 上保留
		// 用户拖动后的菜单栏位置；没有 ID 时每次启动都会被当成新图标。
		ID:             "bili-fm",
		Icon:           icon,
		IconIsTemplate: template, // macOS 会按菜单栏明暗着色
		ToolTip:        "bili-FM",
		// 刻意不在这里传 Menu：Windows/macOS 要留出左键（见上）。
	})
	if err != nil {
		log.Printf("托盘创建失败（关窗后将无法恢复窗口）: %v", err)
		return
	}
	// Windows/macOS：左键（单击或双击）都显示主窗口。双击会回调两次，
	// showWindow 幂等，不会把窗口又藏起来。
	tray.OnClick(func() { c.trayAction("show", c.showWindow) })
	// 右键弹菜单。Windows 原生托盘只在菜单常驻时才自动弹（这里没设），
	// 所以三个平台统一由这条回调临时挂菜单再弹；Linux 没有点击事件，
	// 走下面的常驻菜单。
	tray.OnRightClick(func() {
		c.trayAction("menu", func() {
			tray.SetMenu(menu)
			defer tray.SetMenu(nil)
			tray.PopUpMenu()
		})
	})
	if runtime.GOOS == "linux" {
		// AppIndicator 的点击只能弹菜单：保持常驻，第一项是「显示主窗口」。
		tray.SetMenu(menu)
	}
	// 必须留住托盘本身：Linux（AppIndicator）的原生托盘不持有 mygo 的
	// TrayHandler，而 mygo 的菜单派发（menuItems）只对 MenuItem 持弱引用，
	// Menu 又只被 *Tray 强引用 —— 不存这一笔，*Tray/Menu/每个 MenuItem 都会被
	// GC 回收，托盘菜单点上去没有任何反应（macOS/Windows 的原生托盘持有
	// handler，链子不断，所以只有 Linux 用户报这个）。
	c.tray = tray
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

// trayAction 执行一个托盘菜单动作。
//
// 菜单回调**不保证**跑在 mygo 的事件循环线程上：Linux 的托盘菜单是
// AppIndicator 的 DBusMenu，点击最终由 GTK 的 "activate" 信号派发到
// internal/linux/menu.go 的 cbMenuActivate（GTK 主循环线程），而
// Windows/macOS 的原生托盘各自在自己的消息循环里回调。此前这里直接
// 调用动作，动作里对界面状态的读写（以及 Win.Update 的排队）就依赖
// 「回调线程恰好等于 mygo 主线程」这个假设 —— Linux 上不成立时表现为
// 「菜单点得动、控制按钮没反应」。
//
// 统一用 mygo.RunOnMain 兜住：已经在主线程时它直接执行（零开销），
// 不在时投递到事件循环并等待。日志保留，便于区分「回调没进来」与
// 「动作本身没生效」。
func (c *controller) trayAction(name string, fn func()) {
	log.Printf("托盘菜单：%s", name)
	mygo.RunOnMain(fn)
}

// showWindow 从托盘恢复主窗口。除了 Show/Focus，还要处理「窗口最小化」
// 的状态：只 Show 不会把最小化的窗口恢复出来。
func (c *controller) showWindow() {
	log.Printf("托盘菜单：显示主窗口")
	// 先把「后台」关掉再 Show，避免首帧还走 Shell 的 early-return 而闪一下空白。
	c.setBackground(false)
	w := c.app.Win
	w.Show()
	if w.IsMinimized() {
		w.Restore()
	}
	w.Focus()
}
