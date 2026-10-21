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
	// desktopLyricsWidth 是设置页的最小宽度 / 拿不到屏幕信息时的兜底；
	// 显示页宽度按 LyricsWidthRatio 占屏幕工作区比例计算。
	desktopLyricsWidth        = 560
	desktopLyricsHeight       = 100
	desktopLyricsConfigHeight = 500
)

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
		c.closeDesktopLyrics()
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
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		if !a.LyricsOn {
			a.LyricsOn = true
			c.lyricsAutoHide = false
			c.lyricsWin.SetSize(a.LyricsPixelWidth, desktopLyricsHeight)
			c.lyricsWin.ShowInactive()
			c.syncLyricsAt(a.Pos)
			c.lyricsWin.Update(func() {})
		}
		return
	}
	a.LyricsConfig = false
	a.LyricsConfigAbove = false
	h := desktopLyricsHeight
	x, y, ok := c.savedLyricsPos(desktopLyricsWidth, h)
	if !ok {
		x, y = desktopLyricsDefaultPos(desktopLyricsWidth, h)
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
		// Windows 上普通 native-UI 窗口有重定向位图，根节点透明会被画成
		// 纯黑；Panel 是无重定向位图的 WS_POPUP，透明像素才能露出桌面。
		// macOS/Linux 仍用普通窗口，避免 Wayland 的 popup 定位/焦点差异。
		Panel:             runtime.GOOS == "windows",
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		StateKey:          "desktop-lyrics",
		Hidden:            true,
		Content:           ui.View(a.LyricsShell),
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
	// StateKey 会连窗口尺寸一起恢复；如果上次是在设置页关掉的，恢复成
	// 316 高会把显示页撑开，所以创建后强制回显示页高度。位置也显式写回：
	// 不依赖 StateKey 是否保存位置，保证切换视频/重启后落在用户拖到的地方。
	win.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: h})
	win.ShowInactive() // 不抢当前应用焦点，桌面字幕只是被动显示
	win.OnClosed(func() {
		// Alt+F4 等用户关闭路径也要记住位置；hideDesktopLyrics 里还会先存
		// 一次（因为那里会提前清 LyricsConfig，需保留设置页展开方向）。
		if c.lyricsWin == win {
			c.saveLyricsPos(win)
			c.lyricsWin = nil
		}
		a.LyricsOn = false
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		// 用户用 Alt+F4 等方式关闭窗口 = 关掉开关；自动隐藏不算。
		if !c.lyricsAutoHide {
			a.LyricsEnabled = false
			_ = c.kv.SetString(prefLyricsEnabled, "false")
			c.syncMenuToggles()
		}
		c.lyricsAutoHide = false
		a.Win.Update(func() {})
	})
	c.syncLyricsAt(a.Pos)
}

// desktopLyricsDefaultPos 把首次打开的窗口放在主屏工作区底部居中；之后
// StateKey 会恢复用户拖到的位置。
func desktopLyricsDefaultPos(w, h int) (int, int) {
	d := mygo.Screen.PrimaryDisplay().WorkArea
	x := d.X + (d.Width-w)/2
	y := d.Y + d.Height - h - 72
	if x < d.X {
		x = d.X
	}
	if y < d.Y {
		y = d.Y
	}
	return x, y
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

// resizeDesktopLyricsWidth 在设置宽度比例后同步窗口宽度；设置页至少保留
// desktopLyricsWidth 宽，避免颜色/滑块行被挤坏。
func (c *controller) resizeDesktopLyricsWidth() {
	if c.lyricsWin == nil {
		return
	}
	b := c.lyricsWin.Bounds()
	w := c.desktopLyricsWidthFor(b.X, b.Y)
	c.app.LyricsPixelWidth = w
	// 设置页本身保持固定宽度，只更新预览用的实际字幕宽度；等用户点
	// 「完成」回到显示页时再应用，避免调宽度时设置窗口跟着变、控件乱跳。
	if c.app.LyricsConfig {
		return
	}
	x, y := clampLyricsPos(b.X, b.Y, w, b.Height)
	c.lyricsWin.SetBounds(mygo.Rectangle{X: x, Y: y, Width: w, Height: b.Height})
}

// savedLyricsPos 读取上次关闭时记住的窗口位置，并夹到当前屏幕工作区内。
// 显示器被拔掉/分辨率变化时也不能把窗口留在看不见的地方。
func (c *controller) savedLyricsPos(w, h int) (int, int, bool) {
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
	cx, cy := clampLyricsPos(x, y, w, h)
	return cx, cy, true
}

// saveLyricsPos 保存的是**显示页**的左上角。设置页在下方时窗口左上就是
// 预览左上；设置页在上方时预览在窗口底部，要补回配置页多出来的高度。
func (c *controller) saveLyricsPos(w *mygo.Window) {
	if w == nil || c.kv == nil {
		return
	}
	b := w.Bounds()
	x, y := b.X, b.Y
	if c.app.LyricsConfig && c.app.LyricsConfigAbove {
		y += desktopLyricsConfigHeight - desktopLyricsHeight
	}
	_ = c.kv.SetString(prefLyricsX, strconv.Itoa(x))
	_ = c.kv.SetString(prefLyricsY, strconv.Itoa(y))
}

// clampLyricsPos 把窗口夹进离它最近的显示器工作区。
func clampLyricsPos(x, y, w, h int) (int, int) {
	d := mygo.Screen.DisplayNearestPoint(mygo.Point{X: x, Y: y})
	wa := d.WorkArea
	if wa.Width <= 0 || wa.Height <= 0 {
		return x, y
	}
	if x < wa.X {
		x = wa.X
	}
	if x+w > wa.X+wa.Width {
		x = wa.X + wa.Width - w
	}
	if y < wa.Y {
		y = wa.Y
	}
	if y+h > wa.Y+wa.Height {
		y = wa.Y + wa.Height - h
	}
	return x, y
}

// closeDesktopLyrics 是用户主动关闭：窗口和开关一起关。
func (c *controller) closeDesktopLyrics() {
	a := c.app
	a.LyricsEnabled = false
	_ = c.kv.SetString(prefLyricsEnabled, "false")
	if c.menuLyrics != nil {
		c.menuLyrics.SetChecked(false)
	}
	if a.LyricsOn || c.lyricsWin != nil {
		if c.lyricsWin != nil {
			c.saveLyricsPos(c.lyricsWin)
			w := c.lyricsWin
			c.lyricsWin = nil
			// 用户主动关闭（不是无字幕自动隐藏）：OnClosed 不能再把开关
			// 翻一次，这里已经把它关掉了。
			c.lyricsAutoHide = true
			w.Close()
		}
		a.LyricsOn = false
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		a.LyricCurrent, a.LyricNext = "", ""
		if a.Win != nil {
			a.Win.Update(func() {})
		}
	}
}

// hideDesktopLyrics 只关闭窗口，保留用户开关。用于当前视频没有字幕时
// 自动隐藏；下一条有字幕的视频会由 openDesktopLyricsIfReady 自动重新显示。
func (c *controller) hideDesktopLyrics() {
	a := c.app
	// 先保存位置，再清 LyricsConfig：saveLyricsPos 需要知道设置页是向上
	// 还是向下展开，才能把预览的左上角还原出来。
	if c.lyricsWin != nil {
		c.saveLyricsPos(c.lyricsWin)
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
	if now {
		work := mygo.Screen.DisplayMatching(b).WorkArea
		if work.Width <= 0 {
			work = mygo.Screen.PrimaryDisplay().WorkArea
		}
		above, y := lyricsConfigPlacement(b, work, desktopLyricsHeight, desktopLyricsConfigHeight, 8)
		a.LyricsConfigAbove = above
		a.LyricsConfig = true
		// 设置页固定 desktopLyricsWidth，不被字幕宽度比例改变；预览在
		// 里面按实际宽度居中。
		cx, cy := clampLyricsPos(b.X, y, desktopLyricsWidth, desktopLyricsConfigHeight)
		c.lyricsWin.SetBounds(mygo.Rectangle{
			X: cx, Y: cy, Width: desktopLyricsWidth, Height: desktopLyricsConfigHeight,
		})
	} else {
		y := b.Y
		if a.LyricsConfigAbove {
			y += desktopLyricsConfigHeight - desktopLyricsHeight
		}
		a.LyricsConfig = false
		a.LyricsConfigAbove = false
		w := a.LyricsPixelWidth
		if w <= 0 {
			w = desktopLyricsWidth
		}
		x, y := clampLyricsPos(b.X, y, w, desktopLyricsHeight)
		c.lyricsWin.SetBounds(mygo.Rectangle{
			X: x, Y: y, Width: w, Height: desktopLyricsHeight,
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
		// 宽度变化要同步调整窗口；微调变化则立即重算当前句，否则要等
		// 下一次 500ms 进度回调才看到效果。
		c.resizeDesktopLyricsWidth()
		c.syncLyricsAt(a.Pos)
		c.lyricsWin.Update(func() {})
	}
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
