package main

// 桌面歌词：一个很小的、无边框、置顶的悬浮窗，只显示当前字幕与下一句。
//
// 它不是常驻窗口：字幕页里打开才创建；没有字幕的视频不创建；关闭视频 /
// 退出应用时销毁。窗口只画文本，不加载图片，不启动 webview。

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/view"
)

const (
	desktopLyricsWidth  = 560
	desktopLyricsHeight = 92
)

// toggleDesktopLyrics 切换桌面歌词窗。字幕还没下载时先记下开启意图，等
// 字幕正文到达后由 openDesktopLyricsIfReady 创建窗口。
func (c *controller) toggleDesktopLyrics() {
	a := c.app
	if c.lyricsWin != nil {
		c.closeDesktopLyrics()
		return
	}
	if a.LyricsOn {
		// 开头点过一次、还在等字幕下载：再点一次视为取消，不要重复挂起。
		a.LyricsOn = false
		a.Win.Update(func() {})
		return
	}
	t := currentExtrasTrack(a)
	if t == nil || t.Cid == 0 {
		a.Notify("请先播放一个视频")
		return
	}
	if len(a.SubtitleTracks) == 0 && !a.ExtrasLoading {
		// 没有字幕轨：不创建一个空窗口，也不把状态留成开。
		a.LyricsOn = false
		a.Notify("当前视频没有字幕，无法显示桌面歌词")
		a.Win.Update(func() {})
		return
	}
	a.LyricsOn = true
	a.Win.Update(func() {})
	c.ensureSubtitles()
	c.openDesktopLyricsIfReady()
}

// openDesktopLyricsIfReady 在字幕正文就绪后创建窗口（主线程）。
func (c *controller) openDesktopLyricsIfReady() {
	a := c.app
	if !a.LyricsOn || c.lyricsWin != nil || len(a.Subtitles) == 0 {
		return
	}
	w := desktopLyricsWidth
	h := desktopLyricsHeight
	x, y := desktopLyricsDefaultPos(w, h)
	opts := mygo.WindowOptions{
		Title:             "桌面歌词",
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
	win.ShowInactive() // 不抢当前应用焦点，桌面歌词只是被动显示
	win.OnClosed(func() {
		if c.lyricsWin == win {
			c.lyricsWin = nil
		}
		a.LyricsOn = false
		a.Win.Update(func() {})
	})
	c.syncLyricsAt(a.Pos)
}

// desktopLyricsDefaultPos 把首次打开的歌词窗放在主屏工作区底部居中；之后
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

// closeDesktopLyrics 关闭悬浮窗并清掉界面状态。
func (c *controller) closeDesktopLyrics() {
	a := c.app
	a.LyricsOn = false
	a.LyricCurrent, a.LyricNext = "", ""
	if c.lyricsWin != nil {
		w := c.lyricsWin
		c.lyricsWin = nil
		w.Close()
	}
	if a.Win != nil {
		a.Win.Update(func() {})
	}
}

// syncLyricsAt 根据播放位置更新歌词窗。只在字幕句变化时重建窗口内容，
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
		c.app.LyricCurrent, c.app.LyricNext = "", ""
		c.lyricsWin.Update(func() {})
	}
}
