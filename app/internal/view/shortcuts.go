package view

import "github.com/egoist/mygo/ui"

// shortcuts 注册全局快捷键，与原版一致：
//
//	空格      暂停 / 播放
//	← →       上一集 / 下一集
//	↑ ↓       音量 ±5%
//	Esc       关闭面板
//	Ctrl/Cmd+F 聚焦搜索（由 SearchField 自己处理 Tab 顺序，这里只做提示）
func (a *App) shortcuts(c *ui.Context) {
	c.OnShortcut(0, ui.KeySpace, func() {
		if a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}
	})
	c.OnShortcut(0, ui.KeyLeft, func() {
		if a.Act.Prev != nil {
			a.Act.Prev()
		}
	})
	c.OnShortcut(0, ui.KeyRight, func() {
		if a.Act.Next != nil {
			a.Act.Next()
		}
	})
	c.OnShortcut(0, ui.KeyUp, func() { a.nudgeVolume(0.05) })
	c.OnShortcut(0, ui.KeyDown, func() { a.nudgeVolume(-0.05) })
	c.OnShortcut(0, ui.KeyEscape, func() {
		switch {
		case a.ShowParts:
			a.ShowParts = false
		case a.ShowDanmaku:
			a.ShowDanmaku = false
		case a.ShowSpeed:
			a.ShowSpeed = false
		}
	})
}

// nudgeVolume 调整音量，步进与原版一致（5%）。
func (a *App) nudgeVolume(delta float64) {
	v := a.Volume + delta
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	a.Volume = v
	if a.Act.SetVolume != nil {
		a.Act.SetVolume(v)
	}
}
