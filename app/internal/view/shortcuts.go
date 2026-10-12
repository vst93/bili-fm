package view

import "github.com/egoist/mygo/ui"

// shortcuts 注册全局快捷键，与原版一致：
//
//	空格       暂停 / 播放
//	← →        上一集 / 下一集
//	↑ ↓        音量 ±5%
//	Esc        关闭面板
//	Ctrl/Cmd+W 最小化（原版 minimize_window）
//	Ctrl/Cmd+Q 退出（原版 quit_app）
//
// root 是窗口根元素：空格要额外注册在它上面，原因见 bindSpace。
func (a *App) shortcuts(c *ui.Context, root ui.Element) {
	bindSpace(c, root, func() {
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
	c.OnShortcut(0, ui.KeyUp, func() {
		a.nudgeVolume(0.05)
	})
	c.OnShortcut(0, ui.KeyDown, func() {
		a.nudgeVolume(-0.05)
	})
	// 原版 lib 里的窗口快捷键（titleBar.tsx 的 keydown）：
	// Ctrl/Cmd+W 最小化、Ctrl/Cmd+Q 退出。「快捷键」对话框里一直写着这两条，
	// 但原生版此前只有托盘 / 菜单入口 —— 对齐上。
	c.OnShortcut(ui.Cmd, ui.KeyW, func() {
		if a.Act.Minimize != nil {
			a.Act.Minimize()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyQ, func() {
		if a.Act.Quit != nil {
			a.Act.Quit()
		}
	})
	c.OnShortcut(0, ui.KeyEscape, func() {
		// 原版视频弹窗打开时 Esc 关弹窗（其余快捷键屏蔽）。
		if a.VideoOpen {
			if a.Act.CloseVideo != nil {
				a.Act.CloseVideo()
			}
			return
		}
		if a.Modal != nil {
			// 模态框开着时 Esc 等价点关闭。
			if a.Modal.Kind == "login" {
				if a.Act.CloseLogin != nil {
					a.Act.CloseLogin()
				}
			} else if a.Act.CloseModal != nil {
				a.Act.CloseModal()
			}
			return
		}
		switch {
		case a.Drawer != "":
			a.Drawer = ""
		case a.ShowSpeed:
			a.ShowSpeed = false
		case a.ShowVolume:
			a.ShowVolume = false
		}
	})
}

// bindSpace 把空格（播放 / 暂停）同时注册在**窗口**和**窗口根元素**上。
//
// mygo 的规矩是「聚焦的按钮 / 开关保留 Enter 与空格」（docs/ui/input.md）。
// 于是点过任意一个按钮之后，空格会被那个按钮吃掉 —— 等于再点一次那个按钮：
// 点完收藏按空格 = 又收藏一次、点完下一集按空格 = 又切一首、点完搜索按空格
// = 又搜一次，而播放/暂停根本不动。原版应用是反过来的：它在 keydown 的捕获
// 阶段对按钮 preventDefault + stopPropagation，专门把空格从按钮手里抢回来
// 交给播放 / 暂停（src/pages/index.tsx 的 handleSpaceKeyIntercept）。
//
// 怎么抢：mygo 里按钮抢空格看的是 claimedBy(key, false)，它只算**元素级**的
// 登录（窗口级不算）；而按键最终交给谁由 routeKeys 顺着焦点链找最内层的注册，
// 找不到元素级才退回窗口级。所以在根元素上也注册一份空格：聚焦的按钮就抢不
// 到它了，键沿焦点链上浮到根元素执行；窗口级那份保留给「什么都没聚焦」的常态
// （此时焦点链是空的，根元素注册根本不会被找到）。
//
// 文本框不受影响：编辑器在按键路由之前就吃掉了可打印字符（input.go 的
// editor.wants 分支直接 return），所以在搜索框里按空格仍然只是打一个空格。
func bindSpace(c *ui.Context, root ui.Element, fn func()) {
	c.OnShortcut(0, ui.KeySpace, fn)
	root.OnShortcut(0, ui.KeySpace, fn)
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
