package main

import (
	"runtime"

	"github.com/egoist/mygo"

	"github.com/vst93/bili-fm/app/internal/view"
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
		// 原版的「设置」子菜单：四个动作条目 + Cmd+Q 退出。macOS 没有
		// 窗口内自绘设置下拉，所以三个播放偏好必须在这里以原生复选框出现；
		// Windows / Linux 由 settingsmenu.go 的手绘菜单承担。
		{Label: "设置", Submenu: []*mygo.MenuItem{
			{Label: "关于应用", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuAbout() }},
			{Label: "快捷键", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuShortcuts() }},
			{Label: "检查更新", Click: func(*mygo.MenuItem, *mygo.Window) { c.menuCheckUpdate() }},
			mygo.Separator(),
			c.menuCheckbox("桌面字幕", c.app != nil && c.app.LyricsEnabled, c.menuToggleLyrics),
			c.menuCheckbox("弹幕热力", c.app != nil && c.app.HeatOn, c.menuToggleHeat),
			c.menuCheckbox("顺着听", c.app != nil && c.app.RadioOn, c.menuToggleRadio),
			mygo.Separator(),
			// 表面材质 / 浓度：macOS 没有窗口内设置菜单，只能放进系统菜单。
			// 浓度用三档预设（原生菜单放不下连续滑杆），与窗口内滑杆共享同一
			// 份 App 字段与偏好键。
			{Label: "表面材质", Submenu: []*mygo.MenuItem{
				c.menuGlassRadio("经典", view.GlassClassic),
				c.menuGlassRadio("液态玻璃·明", view.GlassLight),
				c.menuGlassRadio("液态玻璃·暗", view.GlassDark),
			}},
			{Label: "玻璃浓度（明）", Submenu: c.menuTintRadios(false)},
			{Label: "玻璃浓度（暗）", Submenu: c.menuTintRadios(true)},
			mygo.Separator(),
			// 隐身入口上移（见 app/internal/view/settingsmenu.go）：它影响所有
			// 列表的云端进度同步，藏在历史抽屉里太深；历史抽屉原开关保留。
			{Label: "隐身模式", Type: mygo.MenuItemCheckbox, Checked: c.app != nil && c.app.Incognito,
				Click: func(it *mygo.MenuItem, _ *mygo.Window) { c.menuToggleIncognito(it) }},
			mygo.Separator(),
			{Label: "退出应用", Accelerator: "CmdOrCtrl+Q",
				Click: func(*mygo.MenuItem, *mygo.Window) { c.quit() }},
		}},
	}
}

// glassTintPresets 是原生菜单能提供的三档浓度（0..1.5 的连续值由窗口内
// 滑杆负责；macOS 没有窗口内设置菜单，用预设覆盖常用档）。
var glassTintPresets = []struct {
	label string
	value float64
}{
	{"淡", 0.6},
	{"标准", 1.0},
	{"浓", 1.4},
}

// menuGlassRadio 建一个表面材质单选项。
func (c *controller) menuGlassRadio(label string, g view.GlassTheme) *mygo.MenuItem {
	it := &mygo.MenuItem{
		Label:   label,
		Type:    mygo.MenuItemRadio,
		Checked: c.app != nil && c.app.GlassTheme == g,
	}
	it.Click = func(*mygo.MenuItem, *mygo.Window) {
		c.setGlassTheme(g)
		c.syncMenuGlass()
	}
	c.menuGlass[int(g)] = it
	return it
}

// menuTintRadios 建一档浓度的三个预设项。
func (c *controller) menuTintRadios(dark bool) []*mygo.MenuItem {
	out := make([]*mygo.MenuItem, 0, len(glassTintPresets))
	for i, p := range glassTintPresets {
		i, p := i, p
		it := &mygo.MenuItem{Label: p.label, Type: mygo.MenuItemRadio}
		it.Checked = c.app != nil && c.tintFor(dark) == p.value
		it.Click = func(*mygo.MenuItem, *mygo.Window) {
			if c.app == nil {
				return
			}
			if dark {
				c.app.GlassTintDark = p.value
			} else {
				c.app.GlassTintLight = p.value
			}
			c.persistGlassTint()
			c.syncMenuGlass()
		}
		if dark {
			c.menuTintDark[i] = it
		} else {
			c.menuTintLight[i] = it
		}
		out = append(out, it)
	}
	return out
}

func (c *controller) tintFor(dark bool) float64 {
	if c.app == nil {
		return 1
	}
	if dark {
		return c.app.GlassTintDark
	}
	return c.app.GlassTintLight
}

// syncMenuGlass 把原生单选组的勾选态同步到 App（窗口内菜单/滑杆改动后也调）。
func (c *controller) syncMenuGlass() {
	if c.app == nil {
		return
	}
	for i, it := range c.menuGlass {
		if it != nil {
			it.SetChecked(int(c.app.GlassTheme) == i)
		}
	}
	for i, it := range c.menuTintLight {
		if it != nil {
			it.SetChecked(absF64(c.app.GlassTintLight-glassTintPresets[i].value) < 0.05)
		}
	}
	for i, it := range c.menuTintDark {
		if it != nil {
			it.SetChecked(absF64(c.app.GlassTintDark-glassTintPresets[i].value) < 0.05)
		}
	}
}

func absF64(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// menuCheckbox 建一个原生复选框并记下指针，供其它入口切换后同步勾选态。
func (c *controller) menuCheckbox(label string, checked bool, click func(*mygo.MenuItem)) *mygo.MenuItem {
	it := &mygo.MenuItem{
		Label: label, Type: mygo.MenuItemCheckbox, Checked: checked,
		Click: func(mi *mygo.MenuItem, _ *mygo.Window) { click(mi) },
	}
	switch label {
	case "桌面字幕":
		c.menuLyrics = it
	case "弹幕热力":
		c.menuHeat = it
	case "顺着听":
		c.menuRadio = it
	}
	return it
}

// syncMenuToggles 把 App 的真实状态同步回 macOS 原生菜单。菜单勾选是
// 原生控件，不会自己知道用户在抽屉/标题栏里改了什么。
func (c *controller) syncMenuToggles() {
	if c.app == nil {
		return
	}
	if c.menuLyrics != nil {
		c.menuLyrics.SetChecked(c.app.LyricsEnabled)
	}
	if c.menuHeat != nil {
		c.menuHeat.SetChecked(c.app.HeatOn)
	}
	if c.menuRadio != nil {
		c.menuRadio.SetChecked(c.app.RadioOn)
	}
}

// menuToggleLyrics / menuToggleHeat / menuToggleRadio 是 macOS 原生菜单
// 复选框的点击回调。框架在调用 Click 前已翻转 Checked，所以以 want 为准；
// 动作失败（例如没播放时开电台）再把勾选态拉回真实状态。
func (c *controller) menuToggleLyrics(it *mygo.MenuItem) {
	if c.app == nil || it == nil {
		return
	}
	want := it.IsChecked()
	if c.app.LyricsEnabled != want {
		c.toggleDesktopLyrics()
	}
	it.SetChecked(c.app.LyricsEnabled)
}

func (c *controller) menuToggleHeat(it *mygo.MenuItem) {
	if c.app == nil || it == nil {
		return
	}
	c.setHeatOn(it.IsChecked())
	it.SetChecked(c.app.HeatOn)
}

func (c *controller) menuToggleRadio(it *mygo.MenuItem) {
	if c.app == nil || it == nil {
		return
	}
	want := it.IsChecked()
	if c.app.RadioOn != want {
		c.toggleRadio()
	}
	it.SetChecked(c.app.RadioOn)
}

// menuToggleIncognito 是 macOS 原生菜单里「隐身模式」复选框的点击回调。
// 复选框的 Checked 由框架在调 Click 前自动翻转，所以这里只需校验并落盘。
func (c *controller) menuToggleIncognito(it *mygo.MenuItem) {
	if c.app == nil {
		return
	}
	c.app.Incognito = it != nil && it.IsChecked()
	_ = c.kv.SetString("incognito", map[bool]string{true: "true", false: "false"}[c.app.Incognito])
	c.app.Win.Update(func() {})
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
