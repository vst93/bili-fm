package main

// 桌面字幕：一个很小的、无边框、置顶的悬浮窗，只显示当前字幕与下一句。
//
// 它不是常驻窗口：字幕页里打开才创建；没有字幕的视频不创建；关闭视频 /
// 退出应用时销毁。窗口只画文本，不加载图片，不启动 webview。
//
// 窗口右上角的齿轮把同一个窗口切换到设置页（颜色 / 字号 / 边距 / 透明度），
// 设置存本地 KV，重启后继续生效。

import (
	"strconv"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view"
)

const (
	desktopLyricsWidth        = 560
	desktopLyricsHeight       = 100
	desktopLyricsConfigHeight = 430
)

// toggleDesktopLyrics 切换桌面字幕的**用户开关**。有没有字幕由播放器数据
// 决定：开关打开时，遇到无字幕视频自动隐藏窗口（开关保持开），后续遇到有
// 字幕的视频自动重新显示。
func (c *controller) toggleDesktopLyrics() {
	a := c.app
	a.LyricsEnabled = !a.LyricsEnabled
	_ = c.kv.SetString(prefLyricsEnabled, strconv.FormatBool(a.LyricsEnabled))
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
	c.ensureSubtitles()
	c.openDesktopLyricsIfReady()
	a.Win.Update(func() {})
}

// openDesktopLyricsIfReady 在字幕正文就绪后创建窗口（主线程）。
func (c *controller) openDesktopLyricsIfReady() {
	a := c.app
	if !a.LyricsEnabled || c.lyricsWin != nil || len(a.Subtitles) == 0 {
		return
	}
	a.LyricsConfig = false
	w := desktopLyricsWidth
	h := desktopLyricsHeight
	x, y := desktopLyricsDefaultPos(w, h)
	opts := mygo.WindowOptions{
		Title:             "桌面字幕",
		Width:             w,
		Height:            h,
		X:                 x,
		Y:                 y,
		Frameless:         true,
		Transparent:       true,
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
	// StateKey 会连窗口尺寸一起恢复；如果上次是在设置页关掉的，恢复成
	// 316 高会把显示页撑开，所以创建后强制回显示页高度。
	win.SetSize(desktopLyricsWidth, desktopLyricsHeight)
	win.ShowInactive() // 不抢当前应用焦点，桌面字幕只是被动显示
	win.OnClosed(func() {
		if c.lyricsWin == win {
			c.lyricsWin = nil
		}
		a.LyricsOn = false
		a.LyricsConfig = false
		// 用户用 Alt+F4 等方式关闭窗口 = 关掉开关；自动隐藏不算。
		if !c.lyricsAutoHide {
			a.LyricsEnabled = false
			_ = c.kv.SetString(prefLyricsEnabled, "false")
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

// closeDesktopLyrics 是用户主动关闭：窗口和开关一起关。
func (c *controller) closeDesktopLyrics() {
	a := c.app
	a.LyricsEnabled = false
	_ = c.kv.SetString(prefLyricsEnabled, "false")
	c.hideDesktopLyrics()
}

// hideDesktopLyrics 只关闭窗口，保留用户开关。用于当前视频没有字幕时
// 自动隐藏；下一条有字幕的视频会由 openDesktopLyricsIfReady 自动重新显示。
func (c *controller) hideDesktopLyrics() {
	a := c.app
	a.LyricsOn = false
	a.LyricsConfig = false
	a.LyricCurrent, a.LyricNext = "", ""
	if c.lyricsWin != nil {
		w := c.lyricsWin
		c.lyricsWin = nil
		c.lyricsAutoHide = true
		w.Close()
	}
	if a.Win != nil {
		a.Win.Update(func() {})
	}
	trimMemory()
}

// toggleLyricsConfig 在字幕显示页与设置页之间切换，并调整窗口高度。
func (c *controller) toggleLyricsConfig() {
	a := c.app
	if c.lyricsWin == nil {
		return
	}
	a.LyricsConfig = !a.LyricsConfig
	h := desktopLyricsHeight
	if a.LyricsConfig {
		h = desktopLyricsConfigHeight
	}
	c.lyricsWin.SetSize(desktopLyricsWidth, h)
	c.lyricsWin.Update(func() {})
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
	if a.LyricsMargin < 4 {
		a.LyricsMargin = 4
	}
	if a.LyricsMargin > 40 {
		a.LyricsMargin = 40
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
	_ = c.kv.SetString(prefLyricsColor, a.LyricsColor)
	_ = c.kv.SetString(prefLyricsBgColor, a.LyricsBgColor)
	_ = c.kv.SetString(prefLyricsSize, formatPrefFloat(a.LyricsFontSize))
	_ = c.kv.SetString(prefLyricsMargin, formatPrefFloat(a.LyricsMargin))
	_ = c.kv.SetString(prefLyricsBgOpacity, formatPrefFloat(a.LyricsBgOpacity))
	_ = c.kv.SetString(prefLyricsTextOpacity, formatPrefFloat(a.LyricsTextOpacity))
	if c.lyricsWin != nil {
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
	cur, next := lyricLines(a.Subtitles, pos)
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
