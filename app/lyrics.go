package main

// 桌面字幕：一个很小的、无边框、置顶的悬浮窗，只显示当前字幕与下一句。
//
// 它不是常驻窗口：字幕页里打开才创建；没有字幕的视频不创建；关闭视频 /
// 退出应用时销毁。窗口只画文本，不加载图片，不启动 webview。
//
// 窗口右上角的齿轮把同一个窗口切换到设置页（颜色 / 字号 / 屏幕宽度比例 /
// 透明度 / 时间微调），设置存本地 KV，重启后继续生效。

import (
	"runtime"
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view"
)

const (
	// desktopLyricsWidth 是设置页的固定宽度 / 拿不到屏幕信息时的兜底；
	// 显示页宽度按 LyricsWidthRatio 占屏幕工作区比例计算。
	desktopLyricsWidth = 560
)

// lyricsDisplayHeight 返回当前字号下显示页应有的高度。
func (c *controller) lyricsDisplayHeight() int {
	return view.LyricsDisplayHeight(c.app.LyricsFontSize)
}

// lyricsConfigHeight 返回设置页窗口应有的高度：显示页 + 固定面板。
func (c *controller) lyricsConfigHeight() int {
	return c.lyricsDisplayHeight() + view.LyricsConfigPanelHeight
}

// lyricsDisplayRectFrom 从窗口 bounds 还原「显示页」矩形。设置页展开时窗口比
// 显示页宽 / 高：显示页水平居中；设置页在上方时显示页贴在窗口底部。保存位置
// 时必须用这个矩形，不能直接存窗口左上角（否则窄屏上会偏掉半个宽度）。
func lyricsDisplayRectFrom(b mygo.Rectangle, config, above bool, displayW, displayH, panelH int) mygo.Rectangle {
	x, y := b.X, b.Y
	if config {
		x = b.X + (b.Width-displayW)/2
		if above {
			y = b.Y + panelH
		}
	}
	return mygo.Rectangle{X: x, Y: y, Width: displayW, Height: displayH}
}

// toggleDesktopLyrics 切换桌面字幕的**用户开关**。有没有字幕由播放器数据
// 决定：开关打开时，遇到无字幕视频自动隐藏窗口（开关保持开），后续遇到有
// 字幕的视频自动重新显示。
func (c *controller) toggleDesktopLyrics() {
	a := c.app
	a.LyricsEnabled = !a.LyricsEnabled
	_ = c.kv.SetString(prefLyricsEnabled, strconv.FormatBool(a.LyricsEnabled))
	if c.menuLyrics != nil {
		c.menuLyrics.SetChecked(a.LyricsEnabled)
	}
	if !a.LyricsEnabled {
		c.disableDesktopLyrics()
		a.Win.Update(func() {})
		return
	}
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		// 允许先打开开关；真正有播放内容/字幕后再自动显示。
		a.Win.Update(func() {})
		return
	}
	c.ensureExtras()
	c.retrySubtitles()
	c.openDesktopLyricsIfReady()
	a.Win.Update(func() {})
}

// openDesktopLyricsIfReady 在字幕正文就绪后创建窗口（主线程）。
func (c *controller) openDesktopLyricsIfReady() {
	a := c.app
	if !a.LyricsEnabled || len(a.Subtitles) == 0 {
		return
	}
	// 换集/无字幕时只隐藏窗口，不销毁：Wayland 不允许客户端给顶层窗口
	// 设置绝对位置，重建窗口会让合成器把位置重置；复用同一个 toplevel
	// 才能让用户拖动的位置跨视频保留。
	if c.lyricsWin != nil {
		if !a.LyricsOn {
			a.LyricsConfig = false
			a.LyricsConfigAbove = false
			a.LyricsOn = true
			c.lyricsAutoHide = false
			// 比例/字号可能在隐藏期间改过；重新按当前屏幕和字号算尺寸，
			// 避免沿用设置页/旧窗口留下的像素宽度。
			h := c.lyricsDisplayHeight()
			// 优先用隐藏时落盘的位置：Wayland 的 toplevel 位置由合成器
			// 管理，Position() 不一定是用户最后看到的位置；KV 是本应用
			// 自己的事实源。取不到时才退回窗口当前位置。
			var w int
			x, y, ok := c.savedLyricsPos(h)
			if ok {
				w = c.desktopLyricsWidthFor(x, y)
				x, y = clampLyricsPos(x, y, w, h)
			} else {
				posX, posY := c.lyricsWin.Position()
				w = c.desktopLyricsWidthFor(posX, posY)
				x, y = clampLyricsPos(posX, posY, w, h)
			}
			a.LyricsPixelWidth = w
			c.lyricsWin.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: h})
			c.lyricsWin.ShowInactive()
			c.syncLyricsAt(a.Pos)
			c.lyricsWin.Update(func() {})
		}
		return
	}
	a.LyricsConfig = false
	a.LyricsConfigAbove = false
	h := c.lyricsDisplayHeight()
	x, y, ok := c.savedLyricsPos(h)
	if !ok {
		// 首次打开：按主屏比例算出实际宽度后再底部居中，不能拿设置页的
		// 560 当显示宽度。
		primary := mygo.Screen.PrimaryDisplay()
		w := lyricsWidthForWork(primary.WorkArea.Width, a.LyricsWidthRatio)
		x, y = desktopLyricsDefaultPos(w, h)
	}
	w := c.desktopLyricsWidthFor(x, y)
	x, y = clampLyricsPos(x, y, w, h)
	a.LyricsPixelWidth = w
	opts := mygo.WindowOptions{
		Title:       "桌面字幕",
		Width:       w,
		Height:      h,
		X:           x,
		Y:           y,
		Frameless:   true,
		Transparent: true,
		// 桌面字幕必须钉在所有窗口之上，所以三平台都用「面板」窗口
		// （mygo 的 WindowOptions.Panel）——只有它保证层级，普通窗口的
		// AlwaysOnTop 各平台都只是「尽力」：
		//
		//   Windows  topmost tool window（WS_EX_TOPMOST），不是它就在
		//            普通窗口层，随便一个新窗口都能盖住；顺带还有副作用
		//            —— 普通 native-UI 窗口有重定向位图，根节点透明会被
		//            画成纯黑，Panel 是没有重定向位图的 WS_POPUP，透明
		//            像素才能露出桌面。
		//   macOS    NSPanel + CanJoinAllSpaces/FullScreenAuxiliary：浮动
		//            层级、在所有 Space 上都显示，全屏应用也压得住；普通
		//            窗口只到 floating level，切换 Space / 进全屏就没了。
		//   Linux    X11 是 utility window + keep_above；Wayland 有
		//            gtk-layer-shell 时是 overlay 层（Sway/Hyprland/KDE/
		//            COSMIC），压得住全屏窗；GNOME 没有 layer-shell，
		//            合成器说了算，这一点客户端无能为力。
		//
		// 面板窗不吃键盘：窗口用 ShowInactive 显示，Wayland 下只是
		// on-demand 键盘，不会把主窗的输入抢走。
		Panel: true,
		// Windows 的窗口阴影/描边由 DWM 按**窗口矩形**画，不看内容形状：
		// 背景透明度调到 0（纯文字浮在桌面上）时仍会有一圈方框影 + 1px
		// 描边，非常扎眼。这里先不给，创建后再由 syncLyricsWindowShadow
		// 按背景透明度打开（背景不透明时阴影是加分的）。macOS 的窗口
		// 阴影跟着内容 alpha 走、Linux 透明窗本来就不画，都不受影响。
		DisableShadow:     runtime.GOOS == "windows",
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		// 刻意不设 StateKey：mygo 的窗口状态会在 NewWindow 时覆盖显式
		// 传入的 X/Y/尺寸，而 Linux Wayland 不允许创建后再定位 —— 两者
		// 叠在一起就是「每次开关位置被重置」。位置/尺寸统一由本文件的
		// KV + SetBounds 管理。
		Hidden:  true,
		Content: ui.View(a.LyricsShell),
	}
	win := mygo.NewWindow(opts)
	if win == nil {
		a.LyricsOn = false
		a.Win.Update(func() {})
		return
	}
	c.lyricsWin = win
	a.LyricsOn = true
	c.lyricsAutoHide = false
	// 拖动后立刻落盘。只在隐藏 / 退出时保存的话，系统注销、断电或进程
	// 被结束时用户最后一次拖动就丢了 —— 隔天重开看起来像「位置被重置」。
	// mygo 的 OnMove 在用户拖动和程序化 SetBounds 后都会触发；这里只存
	// 可见窗口，避免初始化 / 隐藏收尺寸时的中间坐标覆盖已保存的位置。
	win.OnMove(func() {
		if c.lyricsWin == win && win.IsVisible() {
			c.saveLyricsPos(win)
		}
	})
	// 显式写回显示页尺寸：不设 StateKey，窗口状态不会用上次设置页的
	// 高度把显示页撑开；位置也由本文件的 KV 管理。
	win.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: h})
	c.syncLyricsWindowShadow()
	win.ShowInactive() // 不抢当前应用焦点，桌面字幕只是被动显示
	// Alt+F4 等用户关闭路径：不销毁窗口，只隐藏并关掉开关。Wayland 上
	// 销毁 toplevel 会丢掉合成器记住的位置，复用同一个 toplevel 才能让
	// 下次开关回到用户拖到的地方。真正退出应用时 c.quitting 为真，放行。
	win.OnClose(func(e *mygo.CloseEvent) {
		if c.quitting {
			// 系统注销 / 关机等路径可能绕过 c.quit() 的 hideDesktopLyrics；
			// 这里在 native 窗口还在时补存一次（OnClosed 时 Bounds 已是零值）。
			c.saveLyricsPos(win)
			return
		}
		e.PreventDefault()
		c.disableDesktopLyrics()
	})
	win.OnClosed(func() {
		// 只作为「窗口被系统销毁 / 应用退出」的兜底；正常关闭路径已被
		// OnClose 拦成隐藏。应用退出时不要把用户的桌面字幕开关关掉。
		// 位置已经在 OnMove / OnClose / 隐藏路径保存过：OnClosed 触发时
		// mygo 已把 native 置空，再读 Bounds() 只会拿到零值。
		if c.lyricsWin == win {
			c.lyricsWin = nil
		}
		a.LyricsOn = false
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		if !c.lyricsAutoHide && !c.quitting {
			a.LyricsEnabled = false
			_ = c.kv.SetString(prefLyricsEnabled, "false")
			c.syncMenuToggles()
		}
		c.lyricsAutoHide = false
		a.Win.Update(func() {})
	})
	c.syncLyricsAt(a.Pos)
}

// desktopLyricsDefaultYRatio 是首次打开时窗口顶边在工作区可用高度中的
// 位置：2/3，也就是「中间靠下」。字幕/歌词贴着屏幕最底部会离视线太远，
// 居中又挡住主内容，2/3 更接近播放器歌词条的习惯位置。
const desktopLyricsDefaultYRatio = 2.0 / 3.0

// desktopLyricsDefaultPos 把首次打开的窗口放在主屏工作区中间靠下。
// 之后位置由 KV（prefLyricsX/Y）恢复。
func desktopLyricsDefaultPos(w, h int) (int, int) {
	return desktopLyricsDefaultPosIn(mygo.Screen.PrimaryDisplay().WorkArea, w, h)
}

// desktopLyricsDefaultPosIn 是默认位置的纯计算版，便于无头测试。
func desktopLyricsDefaultPosIn(work mygo.Rectangle, w, h int) (int, int) {
	x := work.X + (work.Width-w)/2
	y := work.Y + int(float64(work.Height-h)*desktopLyricsDefaultYRatio)
	return clampToWorkArea(x, y, w, h, work)
}

// desktopLyricsWidthFor 按当前屏幕工作区和宽度比例算字幕窗宽度。给一个
// 下限，避免缩得太窄后设置页控件挤在一起；窗口不能超过工作区。
func (c *controller) desktopLyricsWidthFor(x, y int) int {
	a := c.app
	ratio := a.LyricsWidthRatio
	if ratio < 0.2 {
		ratio = 0.2
	}
	if ratio > 0.8 {
		ratio = 0.8
	}
	d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y})
	work := d.WorkArea
	if work.Width <= 0 {
		return desktopLyricsWidth
	}
	return lyricsWidthForWork(work.Width, ratio)
}

// lyricsWidthForWork 是宽度比例的纯计算：工作区宽 × 比例，夹在 320 与
// 工作区宽减 16 之间。拆出来便于无头测试。
func lyricsWidthForWork(workWidth int, ratio float64) int {
	if workWidth <= 0 {
		return desktopLyricsWidth
	}
	if ratio < 0.2 {
		ratio = 0.2
	}
	if ratio > 0.8 {
		ratio = 0.8
	}
	w := int(float64(workWidth) * ratio)
	if w < 320 {
		w = 320
	}
	if max := workWidth - 16; w > max {
		w = max
	}
	return w
}

// resizeDesktopLyrics 在字号 / 宽度变化后同步窗口尺寸。显示页保持底边不动，
// 设置页保持窗口左上角不动（预览高度变化时面板往下长），避免拖字号时整块
// 窗口乱跳；超出工作区时再由 clampLyricsPos 夹回来。
func (c *controller) resizeDesktopLyrics() {
	if c.lyricsWin == nil {
		return
	}
	a := c.app
	b := c.lyricsWin.Bounds()
	displayH := c.lyricsDisplayHeight()
	if a.LyricsConfig {
		// 设置页宽度固定，只更新预览用的实际字幕宽度与窗口高度；
		// 等用户点「完成」回到显示页时再按比例应用显示宽度。
		a.LyricsPixelWidth = c.desktopLyricsWidthFor(b.X, b.Y)
		configH := displayH + view.LyricsConfigPanelHeight
		// 设置页在下方时窗口顶边就是预览顶边，保持不动；在上方时预览
		// 贴在窗口底部，改为保持底边不动，预览不会跟着字号上下跳。
		newY := b.Y
		if a.LyricsConfigAbove {
			newY = b.Y + b.Height - configH
		}
		x, y := clampLyricsPos(b.X, newY, desktopLyricsWidth, configH)
		if b.X != x || b.Y != y || b.Width != desktopLyricsWidth || b.Height != configH {
			c.lyricsWin.SetBounds(mygo.Rectangle{X: x, Y: y, Width: desktopLyricsWidth, Height: configH})
		}
		return
	}
	w := c.desktopLyricsWidthFor(b.X, b.Y)
	a.LyricsPixelWidth = w
	// 显示页保持底边不动：字号变大时向上长，不会突然往屏幕外探。
	newY := b.Y + b.Height - displayH
	x, y := clampLyricsPos(b.X, newY, w, displayH)
	if b.X != x || b.Y != y || b.Width != w || b.Height != displayH {
		c.lyricsWin.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: displayH})
	}
}

// savedLyricsPos 读取上次关闭时记住的窗口位置，并夹到当前屏幕工作区内。
// 显示器被拔掉/分辨率变化时也不能把窗口留在看不见的地方。
func (c *controller) savedLyricsPos(h int) (int, int, bool) {
	if c.kv == nil {
		return 0, 0, false
	}
	xs, ys := c.kv.String(prefLyricsX), c.kv.String(prefLyricsY)
	if xs == "" || ys == "" {
		return 0, 0, false
	}
	x, errX := strconv.Atoi(xs)
	y, errY := strconv.Atoi(ys)
	if errX != nil || errY != nil {
		return 0, 0, false
	}
	// 旧版 OnClosed 在 native 置空后会把 0,0 写进 KV；没有新版本标记时
	// 不能把它当成用户真的把窗口拖到了左上角，否则修复后第一次启动仍然
	// 看起来「位置被重置」。用户之后真的拖到 (0,0) 时 saveLyricsPos 会
	// 补上标记，下次就能正常恢复。
	if x == 0 && y == 0 && c.kv.String(prefLyricsPosValid) != "true" {
		return 0, 0, false
	}
	// 夹取时必须用**显示页实际宽度**，不能拿设置页的 560 去夹 320 宽的
	// 字幕：窄屏上 560 放不下会把 X 一路推到工作区左缘，重启后看起来就像
	// 位置被重置。宽度按保存点所在显示器与当前比例算，与打开窗口时同源。
	w := c.desktopLyricsWidthFor(x, y)
	cx, cy := clampLyricsPos(x, y, w, h)
	return cx, cy, true
}

// lyricsDisplayRect 返回当前窗口里「显示页」的矩形。设置页展开时窗口比显示页
// 宽 / 高：显示页水平居中；设置页在上方时显示页贴在窗口底部。
func (c *controller) lyricsDisplayRect(w *mygo.Window) mygo.Rectangle {
	if w == nil {
		return mygo.Rectangle{}
	}
	return c.lyricsDisplayRectOf(w.Bounds())
}

// lyricsDisplayRectOf 从窗口 bounds 还原显示页矩形，供 saveLyricsPos 复用
// （它已经读过一次 Bounds，不必再跨线程取一遍）。
func (c *controller) lyricsDisplayRectOf(b mygo.Rectangle) mygo.Rectangle {
	// 宽度始终按当前屏幕 / 比例重算，不信任缓存值：用户可能在设置页
	// 打开期间把窗口拖到另一块显示器。
	displayW := c.desktopLyricsWidthFor(b.X, b.Y)
	return lyricsDisplayRectFrom(b, c.app.LyricsConfig, c.app.LyricsConfigAbove,
		displayW, c.lyricsDisplayHeight(), view.LyricsConfigPanelHeight)
}

// restoreDisplayBounds 在隐藏窗口前把窗口收回到显示页尺寸。这样下次
// ShowInactive 复用同一个 toplevel 时，窗口位置就是字幕位置；Wayland 尤其
// 需要（合成器不让客户端重新指定位置）。
func (c *controller) restoreDisplayBounds() {
	if c.lyricsWin == nil {
		return
	}
	r := c.lyricsDisplayRect(c.lyricsWin)
	c.app.LyricsPixelWidth = r.Width
	c.lyricsWin.SetBounds(r)
}

// saveLyricsPos 保存的是**显示页**的左上角，不是设置页窗口的左上角。设置页
// 比显示页宽时（窄屏上很常见）窗口 X 会偏出去半个宽度，直接存窗口坐标会让
// 下次打开的位置横向偏移。
func (c *controller) saveLyricsPos(w *mygo.Window) {
	if w == nil || c.kv == nil {
		return
	}
	b := w.Bounds()
	// 窗口已经销毁 / 尚未布局时 Bounds 返回零值。mygo 的 OnClosed 在 native
	// 置空之后才触发，若这时把 0,0 写进去，下次启动就真的回到左上角了。
	if b.Width <= 1 || b.Height <= 1 {
		return
	}
	// Wayland 的顶层位置由合成器管理，GTK 对顶层窗口常常恒返回 (0,0)。
	// 不能把「读不到位置」当成用户把窗口拖到了左上角 —— 那会在每次退出时
	// 把有效的旧坐标覆盖掉。
	if waylandSession() && b.X == 0 && b.Y == 0 {
		return
	}
	r := c.lyricsDisplayRectOf(b)
	_ = c.kv.SetString(prefLyricsX, strconv.Itoa(r.X))
	_ = c.kv.SetString(prefLyricsY, strconv.Itoa(r.Y))
	// 标记这条坐标是本版本写下的有效记录（见 savedLyricsPos 的 0,0 迁移）。
	_ = c.kv.SetString(prefLyricsPosValid, "true")
}

// clampLyricsPos 把窗口夹进离它最近的显示器工作区。窗口比工作区还大时
// 顶边对齐工作区（接受溢出），不把顶边推到屏幕外。
func clampLyricsPos(x, y, w, h int) (int, int) {
	d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y})
	return clampToWorkArea(x, y, w, h, d.WorkArea)
}

// clampToWorkArea 把窗口夹进给定工作区；窗口比工作区还大时顶边对齐
// 工作区（接受溢出），不把顶边推到屏幕外。抽成纯函数便于默认位置测试。
func clampToWorkArea(x, y, w, h int, wa mygo.Rectangle) (int, int) {
	if wa.Width <= 0 || wa.Height <= 0 {
		return x, y
	}
	if w > wa.Width {
		x = wa.X
	} else {
		if x < wa.X {
			x = wa.X
		}
		if x+w > wa.X+wa.Width {
			x = wa.X + wa.Width - w
		}
	}
	if h > wa.Height {
		y = wa.Y
	} else {
		if y < wa.Y {
			y = wa.Y
		}
		if y+h > wa.Y+wa.Height {
			y = wa.Y + wa.Height - h
		}
	}
	return x, y
}

// disableDesktopLyrics 是用户主动关闭开关：只隐藏窗口，不销毁。
//
// 位置记忆不能靠「关掉时销毁、打开时重建」：Linux Wayland 的 toplevel
// 位置由合成器管理，重建的窗口不会接受客户端给的 X/Y，看起来就是每次
// 开关都回到默认位置。复用同一个 toplevel，位置就一直是用户拖到的
// 地方；KV 只负责跨重启的记忆。
func (c *controller) disableDesktopLyrics() {
	a := c.app
	a.LyricsEnabled = false
	_ = c.kv.SetString(prefLyricsEnabled, "false")
	if c.menuLyrics != nil {
		c.menuLyrics.SetChecked(false)
	}
	if c.lyricsWin != nil {
		// 先保存显示页坐标，再收回显示页尺寸；设置页展开时直接存窗口
		// 左上角会偏掉半个宽度。
		c.saveLyricsPos(c.lyricsWin)
		c.restoreDisplayBounds()
		c.lyricsAutoHide = true
		c.lyricsWin.Hide()
	}
	a.LyricsOn = false
	a.LyricsConfig = false
	a.LyricsConfigAbove = false
	a.LyricCurrent, a.LyricNext = "", ""
	if a.Win != nil {
		a.Win.Update(func() {})
	}
	trimMemory()
}

// hideDesktopLyrics 只关闭窗口，保留用户开关。用于当前视频没有字幕时
// 自动隐藏；下一条有字幕的视频会由 openDesktopLyricsIfReady 自动重新显示。
func (c *controller) hideDesktopLyrics() {
	a := c.app
	// 先保存位置，再清 LyricsConfig：saveLyricsPos 需要知道设置页是向上
	// 还是向下展开，才能把显示页的左上角还原出来。
	if c.lyricsWin != nil {
		c.saveLyricsPos(c.lyricsWin)
		// 把窗口收回到显示页尺寸再隐藏：下次 ShowInactive 复用同一个
		// toplevel 时，窗口位置就是字幕位置（Wayland 不能重新定位）。
		c.restoreDisplayBounds()
	}
	a.LyricsOn = false
	a.LyricsConfig = false
	a.LyricsConfigAbove = false
	a.LyricCurrent, a.LyricNext = "", ""
	if c.lyricsWin != nil {
		// 不销毁窗口：Linux Wayland 的 toplevel 位置由合成器管理，重建会
		// 回到默认位置。隐藏后下一次有字幕再 ShowInactive 即可保留位置。
		c.lyricsAutoHide = true
		c.lyricsWin.Hide()
	}
	if a.Win != nil {
		a.Win.Update(func() {})
	}
	trimMemory()
}

// lyricsConfigPlacement 决定设置页向哪边展开。默认向下；当窗口下方放不下
// 配置页、而上方放得下时改为向上，并让预览停在原来的位置。两边都放不下时
// 仍向下，但把窗口上缘往上夹到工作区内。
func lyricsConfigPlacement(bounds, work mygo.Rectangle, displayH, configH, margin int) (above bool, y int) {
	extra := configH - displayH
	if extra < 0 {
		extra = 0
	}
	below := work.Y + work.Height - (bounds.Y + bounds.Height)
	aboveRoom := bounds.Y - work.Y
	above = below < extra+margin && aboveRoom >= extra+margin
	if above {
		y = bounds.Y - extra
	} else {
		y = bounds.Y
		if y+configH > work.Y+work.Height-margin {
			y = work.Y + work.Height - margin - configH
		}
		if y < work.Y+margin {
			y = work.Y + margin
		}
	}
	return above, y
}

// toggleLyricsConfig 在字幕显示页与设置页之间切换，并调整窗口高度。
func (c *controller) toggleLyricsConfig() {
	a := c.app
	if c.lyricsWin == nil {
		return
	}
	now := !a.LyricsConfig
	b := c.lyricsWin.Bounds()
	displayH := c.lyricsDisplayHeight()
	configH := c.lyricsConfigHeight()
	if now {
		// 打开设置页前按当前比例重新算一次实际宽度，避免窗口隐藏期间
		// 缓存/旧设置页留下的 560 像素宽让预览看起来永远被限制。
		a.LyricsPixelWidth = c.desktopLyricsWidthFor(b.X, b.Y)
		work := mygo.Screen.DisplayMatching(b).WorkArea
		if work.Width <= 0 {
			work = mygo.Screen.PrimaryDisplay().WorkArea
		}
		above, y := lyricsConfigPlacement(b, work, displayH, configH, 8)
		a.LyricsConfigAbove = above
		a.LyricsConfig = true
		// 设置页固定 desktopLyricsWidth，不被字幕宽度比例改变；窗口以
		// 原来的显示窗为中心展开，避免打开设置时预览横向跳动。
		configX := b.X + (b.Width-desktopLyricsWidth)/2
		cx, cy := clampLyricsPos(configX, y, desktopLyricsWidth, configH)
		c.lyricsWin.SetBounds(mygo.Rectangle{
			X: cx, Y: cy, Width: desktopLyricsWidth, Height: configH,
		})
	} else {
		y := b.Y
		if a.LyricsConfigAbove {
			y += configH - displayH
		}
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		// 从比例重新算一遍，不能只依赖设置页里缓存的像素宽度；否则某次
		// 滑块/显示器变化会把显示窗永久限制在设置页的 560 宽。
		w := c.desktopLyricsWidthFor(b.X, y)
		a.LyricsPixelWidth = w
		// 仍然以设置页中心为锚点缩回显示页，位置不会横向偏。
		centerX := b.X + b.Width/2 - w/2
		x, y := clampLyricsPos(centerX, y, w, displayH)
		c.lyricsWin.SetBounds(mygo.Rectangle{
			X: x, Y: y, Width: w, Height: displayH,
		})
	}
	c.lyricsWin.Update(func() {})
}

// resetLyricsOffsetOnTrackChange 在切换播放时归零未勾选“记住”的微调。
// 勾选后 offset 跨视频/重启保留；未勾选时只属于当前播放。
func (c *controller) resetLyricsOffsetOnTrackChange() {
	a := c.app
	if !a.LyricsOffsetRemember && a.LyricsOffset != 0 {
		a.LyricsOffset = 0
	}
}

// resetLyricsConfig 恢复桌面字幕的全部样式/时间默认值，并立即落盘、调整
// 窗口宽度和当前句。跟随开关属于播放列表交互偏好，不在「恢复默认」范围。
func (c *controller) resetLyricsConfig() {
	a := c.app
	a.LyricsColor = view.DefaultLyricsColor
	a.LyricsBgColor = view.DefaultLyricsBgColor
	a.LyricsFontSize = view.DefaultLyricsFontSize
	a.LyricsWidthRatio = view.DefaultLyricsWidthRatio
	a.LyricsTextOpacity = view.DefaultLyricsTextOpacity
	a.LyricsBgOpacity = view.DefaultLyricsBgOpacity
	a.LyricsOffset = view.DefaultLyricsOffset
	a.LyricsOffsetRemember = view.DefaultLyricsRememberOffset
	c.updateLyricsConfig()
	if a.Win != nil {
		a.Win.Update(func() {})
	}
}

// updateLyricsConfig 夹取并持久化桌面字幕设置。
func (c *controller) updateLyricsConfig() {
	a := c.app
	if a.LyricsFontSize < 14 {
		a.LyricsFontSize = 14
	}
	if a.LyricsFontSize > 30 {
		a.LyricsFontSize = 30
	}
	if a.LyricsBgOpacity < 0 {
		a.LyricsBgOpacity = 0
	}
	if a.LyricsBgOpacity > 1 {
		a.LyricsBgOpacity = 1
	}
	if a.LyricsTextOpacity < 0 {
		a.LyricsTextOpacity = 0
	}
	if a.LyricsTextOpacity > 1 {
		a.LyricsTextOpacity = 1
	}
	if !validHexColor(a.LyricsColor) {
		a.LyricsColor = "#ffffff"
	}
	if !validHexColor(a.LyricsBgColor) {
		a.LyricsBgColor = "#0f172a"
	}
	if a.LyricsOffset < -10 {
		a.LyricsOffset = -10
	}
	if a.LyricsOffset > 10 {
		a.LyricsOffset = 10
	}
	if a.LyricsWidthRatio < 0.2 {
		a.LyricsWidthRatio = 0.2
	}
	if a.LyricsWidthRatio > 0.8 {
		a.LyricsWidthRatio = 0.8
	}
	_ = c.kv.SetString(prefLyricsColor, a.LyricsColor)
	_ = c.kv.SetString(prefLyricsBgColor, a.LyricsBgColor)
	_ = c.kv.SetString(prefLyricsSize, formatPrefFloat(a.LyricsFontSize))
	_ = c.kv.SetString(prefLyricsWidthRatio, formatPrefFloat(a.LyricsWidthRatio))
	_ = c.kv.SetString(prefLyricsBgOpacity, formatPrefFloat(a.LyricsBgOpacity))
	_ = c.kv.SetString(prefLyricsTextOpacity, formatPrefFloat(a.LyricsTextOpacity))
	_ = c.kv.SetString(prefLyricsOffset, formatPrefFloat(a.LyricsOffset))
	_ = c.kv.SetString(prefLyricsOffsetRemember, strconv.FormatBool(a.LyricsOffsetRemember))
	if c.lyricsWin != nil {
		// 背景透明度可能刚改过：Windows 的窗口阴影要跟着一起开关。
		c.syncLyricsWindowShadow()
		// 宽度 / 字号变化要同步调整窗口；微调变化则立即重算当前句，
		// 否则要等下一次 500ms 进度回调才看到效果。
		c.resizeDesktopLyrics()
		c.syncLyricsAt(a.Pos)
		c.lyricsWin.Update(func() {})
	}
}

// syncLyricsWindowShadow 把系统窗口阴影跟背景透明度绑在一起。
//
// Windows 的 DWM 阴影是**窗口矩形**的投影，并在窗口四周画 1px 描边，与内容
// 形状（我们画的是圆角面板）无关。背景全透明时用户要的是「桌面上只剩字」，
// 那圈方框影就像个没内容的边框。所以：背景透明（opacity = 0）关掉系统阴影，
// 有背景色时再打开。改背景透明度是实时拖滑块，必须当场切，不能等重建窗口。
//
// macOS 的阴影跟着窗口内容的 alpha 走，Linux 透明窗根本不画（useCSD 排除
// Transparent/Panel，SetHasShadow 是空实现），都不需要处理。
func (c *controller) syncLyricsWindowShadow() {
	if c.lyricsWin == nil || runtime.GOOS != "windows" {
		return
	}
	c.lyricsWin.SetHasShadow(lyricsWindowShadowOn(c.app.LyricsBgOpacity))
}

// lyricsWindowShadowOn 判断某个背景透明度下要不要系统窗口阴影：完全透明时
// 不要，有背景色时要。抽成纯函数是为了能无头断言这条判据（Windows 分支本身
// 在 Linux 测试机上跑不到）。
func lyricsWindowShadowOn(bgOpacity float64) bool {
	return bgOpacity > 0
}

func formatPrefFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// syncLyricsAt 根据播放位置更新字幕窗。只在字幕句变化时重建窗口内容，
// 500ms 的进度回调不会造成每帧文本重排。
func (c *controller) syncLyricsAt(pos float64) {
	a := c.app
	if c.lyricsWin == nil {
		return
	}
	// 正数表示字幕提前（整体快进），负数表示延后（整体快退）。
	cur, next := lyricLines(a.Subtitles, pos+a.LyricsOffset)
	if cur == a.LyricCurrent && next == a.LyricNext {
		return
	}
	a.LyricCurrent, a.LyricNext = cur, next
	c.lyricsWin.Update(func() {})
}

// lyricLines 返回当前句和下一句。句间空隙保留上一句，避免字幕突然空白。
func lyricLines(cues []view.SubtitleCue, pos float64) (string, string) {
	if len(cues) == 0 {
		return "", ""
	}
	cur := -1
	lo, hi := 0, len(cues)
	for lo < hi {
		mid := (lo + hi) / 2
		if cues[mid].From <= pos {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	cur = lo - 1
	if cur < 0 {
		return "", cues[0].Content
	}
	next := ""
	if cur+1 < len(cues) {
		next = cues[cur+1].Content
	}
	return cues[cur].Content, next
}

// ensureLyricsForTrack 在换集时清空旧文本；窗口保留到新字幕到达。
func (c *controller) ensureLyricsForTrack() {
	if c.lyricsWin != nil {
		// 换集时先隐藏，避免旧字幕继续挂在新视频上；新字幕到达后如果开关
		// 仍然打开，会自动重新显示。
		c.hideDesktopLyrics()
	}
}
