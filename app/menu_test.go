package main

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// menuTemplate 取模板（darwin），失败即 Fatal。
func menuTemplate(t *testing.T, c *controller) []*mygo.MenuItem {
	t.Helper()
	items := appMenuTemplate("darwin", c)
	if items == nil {
		t.Fatal("darwin 应返回原生菜单模板")
	}
	return items
}

// TestAppMenuTemplate 钉住 macOS 原生菜单栏的结构，对齐 Tauri 原版
// （main:src-tauri/src/lib.rs 的 setup_native_menu）：
//
//	[应用菜单] 编辑(剪切/复制/粘贴/全选) 设置(关于应用/快捷键/检查更新/退出应用⌘Q)
//
// 回归点：原生版此前根本没装应用菜单，mygo 给 darwin 装了自带默认菜单
// （App/File/Edit/View/Window，英文），和原版完全对不上。
func TestAppMenuTemplate(t *testing.T) {
	c := &controller{app: &view.App{}}
	items := menuTemplate(t, c)

	if len(items) != 3 {
		t.Fatalf("顶层菜单 %d 项，应为 3（应用菜单 / 编辑 / 设置）", len(items))
	}
	// 第一个是标准应用菜单（Tauri 在 macOS 上也会补这一块）。
	if items[0].Role != mygo.RoleAppMenu {
		t.Errorf("第一项 role = %q，应为 appMenu", items[0].Role)
	}
	// 编辑子菜单：只含原版的四个 PredefinedMenuItem，无分隔线。
	edit := items[1]
	if edit.Label != "编辑" {
		t.Errorf("第二项 = %q，应为「编辑」", edit.Label)
	}
	wantEdit := []struct {
		label string
		role  mygo.MenuRole
	}{
		{"剪切", mygo.RoleCut},
		{"拷贝", mygo.RoleCopy},
		{"粘贴", mygo.RolePaste},
		{"全选", mygo.RoleSelectAll},
	}
	if len(edit.Submenu) != len(wantEdit) {
		t.Fatalf("编辑子菜单 %d 项，应为 %d", len(edit.Submenu), len(wantEdit))
	}
	for i, want := range wantEdit {
		if edit.Submenu[i].Role != want.role {
			t.Errorf("编辑[%d] role = %q，应为 %q", i, edit.Submenu[i].Role, want.role)
		}
		if edit.Submenu[i].Label != want.label {
			t.Errorf("编辑[%d] = %q，应为 %q", i, edit.Submenu[i].Label, want.label)
		}
	}
	// 设置子菜单：前四项紧排、顺序与原版一致、退出应用带 Cmd+Q；
	// 本 fork 在「检查更新」与「退出应用」之间插了分隔线 + 「隐身模式」复选框
	//（入口上移，见 settingsIncognitoRow）。
	settings := items[2]
	if settings.Label != "设置" {
		t.Errorf("第三项 = %q，应为「设置」", settings.Label)
	}
	wantLabels := []string{"关于应用", "快捷键", "检查更新", "退出应用"}
	// 去掉新加的分隔线、播放偏好开关与隐身项，还原原版的四项再逐项校验。
	var plain []*mygo.MenuItem
	var incognito *mygo.MenuItem
	toggles := map[string]*mygo.MenuItem{}
	glass := map[string]*mygo.MenuItem{}
	for _, it := range settings.Submenu {
		if it.Type == mygo.MenuItemSeparator {
			continue
		}
		if it.Label == "隐身模式" {
			incognito = it
			continue
		}
		if it.Type == mygo.MenuItemCheckbox {
			toggles[it.Label] = it
			continue
		}
		if it.Label == "表面材质" || it.Label == "玻璃浓度（明）" || it.Label == "玻璃浓度（暗）" {
			glass[it.Label] = it
			continue
		}
		plain = append(plain, it)
	}
	if len(plain) != len(wantLabels) {
		t.Fatalf("设置子菜单原版条目 %d 项，应为 %d", len(plain), len(wantLabels))
	}
	for i, label := range wantLabels {
		it := plain[i]
		if it.Label != label {
			t.Errorf("设置[%d] = %q，应为 %q", i, it.Label, label)
		}
		if it.Click == nil {
			t.Errorf("设置[%d]（%s）没有点击回调", i, label)
		}
	}
	if got := plain[3].Accelerator; got != "CmdOrCtrl+Q" {
		t.Errorf("退出应用快捷键 = %q，应为 CmdOrCtrl+Q", got)
	}
	_ = toggles
	// macOS 用系统菜单：三个播放偏好必须是原生复选框，且初始状态取自 App。
	for _, label := range []string{"桌面字幕", "弹幕热力", "顺着听"} {
		it := toggles[label]
		if it == nil {
			t.Errorf("macOS 设置菜单缺 %q 复选框", label)
			continue
		}
		if it.Click == nil {
			t.Errorf("%q 没有点击回调", label)
		}
	}
	// 隐身模式复选框（本 fork 入口上移）。
	if incognito == nil {
		t.Fatal("设置子菜单缺「隐身模式」复选框（隐身入口上移）")
	}
	if incognito.Type != mygo.MenuItemCheckbox {
		t.Errorf("隐身模式 Type = %v，应为 Checkbox", incognito.Type)
	}
	if incognito.Click == nil {
		t.Error("隐身模式没有点击回调")
	}
	// 表面材质 / 浓度：macOS 只能走原生菜单，三个材质 + 各三档浓度都是
	// 单选（radio），且点击回调非空。
	for _, label := range []string{"表面材质", "玻璃浓度（明）", "玻璃浓度（暗）"} {
		it := glass[label]
		if it == nil {
			t.Fatalf("macOS 设置菜单缺 %q 子菜单", label)
		}
		want := 3
		if len(it.Submenu) != want {
			t.Errorf("%q 子菜单 %d 项，应为 %d", label, len(it.Submenu), want)
		}
		for i, sub := range it.Submenu {
			if sub.Type != mygo.MenuItemRadio {
				t.Errorf("%q[%d] Type = %v，应为 Radio", label, i, sub.Type)
			}
			if sub.Click == nil {
				t.Errorf("%q[%d] 没有点击回调", label, i)
			}
		}
	}
}

// TestMenuToggleIncognito 钉住原生菜单的隐身复选框：框架在调 Click 前
// 已翻转 Checked，回调要把它读成真值并落盘（菜单打开时的初始 Checked
// 取自 app.Incognito）。
func TestMenuToggleIncognito(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{}
	c := &controller{app: a, kv: kv}

	it := &mygo.MenuItem{Type: mygo.MenuItemCheckbox, Label: "隐身模式", Checked: true}
	c.menuToggleIncognito(it)
	if !a.Incognito {
		t.Error("勾选后 app.Incognito 应为 true")
	}
	if got := kv.String("incognito"); got != "true" {
		t.Errorf("落盘 incognito = %q，应为 true", got)
	}
	it.SetChecked(false)
	c.menuToggleIncognito(it)
	if a.Incognito {
		t.Error("取消勾选后 app.Incognito 应为 false")
	}
	if got := kv.String("incognito"); got != "false" {
		t.Errorf("落盘 incognito = %q，应为 false", got)
	}
	// 菜单初始 Checked 与 app.Incognito 同源。
	a.Incognito = true
	settings := menuTemplate(t, c)[2].Submenu
	for _, it := range settings {
		if it.Label == "隐身模式" && !it.Checked {
			t.Error("菜单初始 Checked 应取自 app.Incognito=true")
		}
	}
}

// TestAppMenuNonDarwin 钉住「非 macOS 不装原生菜单」：原版那里是 no-op，
// 菜单由窗口内自绘的标题栏「设置」下拉承担。
func TestAppMenuNonDarwin(t *testing.T) {
	c := &controller{app: &view.App{}}
	for _, goos := range []string{"windows", "linux", "freebsd"} {
		if items := appMenuTemplate(goos, c); items != nil {
			t.Errorf("%s 不应有原生应用菜单，得到 %d 项", goos, len(items))
		}
	}
}

// TestAppMenuWiring 钉住原生菜单的三个对话框项接到 view.Actions 的同一组入口
// （与窗口内「设置」下拉 chooseSettingsItem 一致）。
func TestAppMenuWiring(t *testing.T) {
	a := &view.App{}
	c := &controller{app: a}
	var got string
	a.Act.ShowAbout = func() { got = "about" }
	a.Act.ShowShortcuts = func() { got = "shortcuts" }
	a.Act.CheckUpdate = func() { got = "update" }

	settings := menuTemplate(t, c)[2].Submenu
	for i, want := range []string{"about", "shortcuts", "update"} {
		got = ""
		settings[i].Click(nil, nil)
		if got != want {
			t.Errorf("设置[%d]（%s）触发 %q，应为 %q", i, settings[i].Label, got, want)
		}
	}
}

// TestMenuPlaybackTogglesInitialState 钉住 macOS 原生菜单三个播放偏好复选框
// 的初始勾选态取自 App；syncMenuToggles 负责在其它入口切换后回写。
func TestMenuPlaybackTogglesInitialState(t *testing.T) {
	a := &view.App{LyricsEnabled: true, HeatOn: true}
	c := &controller{app: a}
	settings := menuTemplate(t, c)[2].Submenu
	want := map[string]bool{"桌面字幕": true, "弹幕热力": true, "顺着听": false}
	for _, it := range settings {
		if it.Type != mygo.MenuItemCheckbox || it.Label == "隐身模式" {
			continue
		}
		if got, ok := want[it.Label]; ok && it.Checked != got {
			t.Errorf("%s Checked = %v，应为 %v", it.Label, it.Checked, got)
		}
	}
	// 其它入口改状态后，菜单也要跟着变。
	a.LyricsEnabled = false
	a.HeatOn = false
	a.RadioOn = true
	c.syncMenuToggles()
	for _, it := range settings {
		switch it.Label {
		case "桌面字幕":
			if it.IsChecked() {
				t.Error("桌面字幕菜单勾选未同步为 false")
			}
		case "弹幕热力":
			if it.IsChecked() {
				t.Error("弹幕热力菜单勾选未同步为 false")
			}
		case "顺着听":
			if !it.IsChecked() {
				t.Error("顺着听菜单勾选未同步为 true")
			}
		}
	}
}

// TestMenuGlassRadioClick 实测 macOS 原生菜单的材质单选：点「液态玻璃·明」
// 后 App 字段、偏好落盘与单选勾选态都跟上。
func TestMenuGlassRadioClick(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{GlassTheme: view.GlassClassic}
	c := &controller{app: a, kv: kv}
	items := menuTemplate(t, c)
	// 取「设置」子菜单里的「表面材质」。
	var themeItem *mygo.MenuItem
	for _, it := range items[2].Submenu {
		if it.Label == "表面材质" {
			themeItem = it
			break
		}
	}
	if themeItem == nil {
		t.Fatal("缺表面材质子菜单")
	}
	// 点击「液态玻璃·明」单选项。
	for _, it := range themeItem.Submenu {
		if it.Label == "液态玻璃·明" {
			it.Click(it, nil)
			break
		}
	}
	if a.GlassTheme != view.GlassLight {
		t.Errorf("点击后 GlassTheme = %v，应为 GlassLight", a.GlassTheme)
	}
	if got := kv.String(prefGlassTheme); got != "light" {
		t.Errorf("surfaceTheme = %q，应为 light", got)
	}
	if themeItem.Submenu[1].IsChecked() == false {
		t.Error("液态玻璃·明没有勾上")
	}
}

// TestLoadPrefsGlassDefault 钉住首次运行直接进液态玻璃·明（旧配置没有
// surfaceTheme 键），而显式 classic 仍能保持经典。
func TestLoadPrefsGlassDefault(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("建 KV 失败：%v", err)
	}
	a := &view.App{}
	c := &controller{app: a, kv: kv}
	c.loadPrefs()
	if a.GlassTheme != view.GlassLight {
		t.Errorf("空配置默认主题 = %v，应为 GlassLight", a.GlassTheme)
	}
	if a.GlassTintLight != 1 || a.GlassTintDark != 1 {
		t.Errorf("空配置 tint = %v/%v，应为 1/1", a.GlassTintLight, a.GlassTintDark)
	}
	_ = kv.SetString(prefGlassTheme, "classic")
	a2 := &view.App{}
	c2 := &controller{app: a2, kv: kv}
	c2.loadPrefs()
	if a2.GlassTheme != view.GlassClassic {
		t.Errorf("显式 classic = %v，应为 GlassClassic", a2.GlassTheme)
	}
}
