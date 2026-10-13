package main

import (
	"runtime"

	"github.com/egoist/mygo"
)

// setupAppMenu 装 macOS 顶端的**原生菜单栏**，对齐 Tauri 原版
// （main:src-tauri/src/lib.rs 的 setup_native_menu）。
//
// 原版在 macOS 上装两个子菜单：
//
//	编辑  剪切 / 复制 / 粘贴 / 全选（Tauri 的四个 PredefinedMenuItem）
//	设置  关于应用 / 快捷键 / 检查更新 / 退出应用（Cmd+Q，四项紧排、无分隔线）
//
// 其中「关于应用 / 快捷键 / 检查更新」点击后通知前端弹对应对话框，「退出应用」
// 走 Cmd+Q。Tauri 在 macOS 上会再补一个标准的应用菜单（App 名 / 隐藏 / 退出），
// 这里用 mygo 的 RoleAppMenu 同款。
//
// 不装的话，mygo 在 darwin 上会装它自带的默认菜单（App / File / Edit / View /
// Window，全是英文），和原版对不上 —— 这正是用户报的「菜单栏不一样」。
//
// 非 macOS 不装：原版那里就是 no-op，菜单由窗口内自绘的标题栏「设置」下拉承担；
// mygo 在非 darwin 的默认菜单本来就是 nil，什么都不做即可。
func (c *controller) setupAppMenu() {
	items := appMenuTemplate(runtime.GOOS, c)
	if items == nil {
		return
	}
	mygo.App.SetMenu(mygo.NewMenu(items))
}

// appMenuTemplate 返回原生应用菜单的模板；非 macOS 返回 nil（不装）。
//
// 拆成纯函数便于跨平台断言结构（菜单内容不依赖运行时窗口，Click 也只在
// 真正点击时才用到 c）。
func appMenuTemplate(goos string, c *controller) []*mygo.MenuItem {
	if goos != "darwin" {
		return nil
	}
	return []*mygo.MenuItem{
		// 标准应用菜单（About / 服务 / 隐藏 / 退出），与 macOS 惯例一致，
		// 也是 Tauri 在 macOS 上自动补的那一块。
		{Role: mygo.RoleAppMenu},
		{Label: "编辑", Submenu: []*mygo.MenuItem{
			// 角色给出的是原生剪切/复制/粘贴/全选动作；标题按 macOS 中文
			// 菜单的惯用译法写（AppKit 会把原版的 PredefinedMenuItem 本地
			// 化成这几个词，mygo 的角色标题固定英文，这里显式对齐）。
			{Label: "剪切", Role: mygo.RoleCut},
			{Label: "拷贝", Role: mygo.RoleCopy},
			{Label: "粘贴", Role: mygo.RolePaste},
			{Label: "全选", Role: mygo.RoleSelectAll},
		}},
		// 原版的「设置」子菜单：四个条目紧排，没有分隔线；「退出应用」带 Cmd+Q。
		{Label: "设置", Submenu: []*mygo.MenuItem{
			{Label: "关于应用", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuAbout() }},
			{Label: "快捷键", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuShortcuts() }},
			{Label: "检查更新", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuCheckUpdate() }},
			{Label: "退出应用", Accelerator: "CmdOrCtrl+Q",
				Click: func(*mygo.MenuItem, *mygo.Window) { c.quit() }},
		}},
	}
}

// 下面三个小包装把原生菜单动作接到 App 的 Actions 上（与窗口内「设置」下拉
// chooseSettingsItem 走的是同一组入口，保证两条路径行为一致）。
func (c *controller) menuAbout() {
	if c.app != nil && c.app.Act.ShowAbout != nil {
		c.app.Act.ShowAbout()
	}
}

func (c *controller) menuShortcuts() {
	if c.app != nil && c.app.Act.ShowShortcuts != nil {
		c.app.Act.ShowShortcuts()
	}
}

func (c *controller) menuCheckUpdate() {
	if c.app != nil && c.app.Act.CheckUpdate != nil {
		c.app.Act.CheckUpdate()
	}
}
