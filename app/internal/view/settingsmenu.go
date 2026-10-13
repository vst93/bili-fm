package view

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 设置菜单

// 设置菜单是非 macOS 标题栏里「设置」按钮的下拉（原版 titleBar.tsx 的
// #settings-menu）。原版是一块**手写的液态玻璃面板**，不是系统菜单：
//
//	定位   fixed，top = 设置键 rect.bottom + 4、left = 设置键 rect.left（左对齐）
//	盒子   min-w 120、圆角 8（rounded-lg）、上下内边距 4（py-1）、
//	       1px 玻璃边 + --glass-bg 底 + --glass-sheen-static + blur(20)，
//	       box-shadow = --glass-specular + 0 18px 44px rgba(15,23,42,.1)
//	条目   四个整宽按钮（关于应用 / 快捷键 / 检查更新 / 退出应用，**无分隔线**）：
//	       px-4 py-2（16/8）、text-sm（14）、色 #334155；
//	       悬停 --glass-bg-hover（白 .38）+ inset 0 1px 1px --glass-highlight；
//	       按下 scale(.97) + --glass-bg-active（白 .16）
//
// 原生版此前用的是框架 Element.Menu —— 那是**系统原生弹出菜单**（GTK / Windows
// 的平台菜单），既没有这块玻璃面板的底/描边/投影，条目是系统观感，还多画了一条
// 原版没有的分隔线（退出应用前）。本轮换成照原版手搓的 PopoverBase 面板：
// 面板由框架放进 Overlay（画在最上层），并自带「点外部关 / Esc 关 / 焦点归还」。
const (
	settingsMenuMinW   = 120 // min-w-[120px]
	settingsMenuPadY   = 4   // py-1
	settingsMenuRadius = 8   // rounded-lg
	settingsMenuGap    = 4   // 面板距设置键 4px（原版 menuPos.top = rect.bottom + 4）
	settingsItemH      = 36  // py-2 + text-sm 行高
	settingsItemPadX   = 16  // px-4
)

// settingsItemInk 是菜单条目的文案色（原版 #settings-menu button 的 #334155）。
var settingsItemInk = ui.Hex("#334155")

// settingsItems 是设置菜单的条目，顺序与原版一一对应（原版没有分隔线）。
func settingsItems() []string {
	return []string{"关于应用", "快捷键", "检查更新", "退出应用"}
}

// settingsItemPaint 是 #settings-menu button 的三态取值：
//
//	常态  background transparent、无内高光、scale 1
//	悬停  background --glass-bg-hover（白 .38）+ inset 0 1px 1px --glass-highlight
//	按下  background --glass-bg-active（白 .16）+ scale .97
//	      （内高光仍在：原版 :active 只覆盖 background，不重置 :hover 的 box-shadow）
func settingsItemPaint(t Theme, hovered, pressed bool) (bg ui.Color, inset bool, scale float32) {
	scale = 1
	switch {
	case pressed:
		bg, inset, scale = t.GlassActive, true, 0.97
	case hovered:
		bg, inset = t.GlassHover, true
	default:
		bg = ui.Transparent
	}
	return bg, inset, scale
}

// settingsMenu 是设置键的下拉面板。用 PopoverBase：框架负责开合、点外部关、
// Esc 关与焦点归还，面板本身由这里照原版上材质。
func (a *App) settingsMenu(c *ui.Context, anchor ui.Element) {
	ui.PopoverBase(c, anchor, &a.ShowSettings, func(panel ui.Element) {
		// 原版 #settings-menu：min-w 120、圆角 8、py-1、玻璃底 + 白边 + 两层投影。
		// 材质用框架玻璃（与原版同一种液态玻璃，自带边缘聚光），外投影按原版
		// 的 --glass-specular + 0 18px 44px 补上。
		panel.MinWidth(settingsMenuMinW).Padding(settingsMenuPadY, 0).
			Radius(settingsMenuRadius).Margin(settingsMenuGap, 0, 0, 0).
			Material(glassLayer(glass.Glass{})).
			InsetShadow(0, 1, 1, 0, a.Theme.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.06)).
			Shadow(0, 18, 44, 0, ui.Hex("#0f172a").Alpha(0.10))
		panel.Children(func() {
			for _, label := range settingsItems() {
				label := label
				item := ui.ButtonBase(c.Key("settings-"+label)).
					FillWidth().Height(settingsItemH).Padding(0, settingsItemPadX).
					Justify(ui.Start).Radius(0).Label(label)
				bg, inset, scale := settingsItemPaint(a.Theme, item.Hovered(), item.Pressed())
				item.Background(bg)
				if inset {
					item.InsetShadow(0, 1, 1, 0, a.Theme.Highlight)
				}
				// 按下缩到 .97（原版 #settings-menu button:active scale(.97)）。
				p := ui.Pose{}
				if scale != 1 {
					p.Scale = scale
				}
				item.AnimatePose(p, ui.Animation{Duration: 100 * time.Millisecond, Ease: ui.EaseOut})
				item.Children(func() {
					ui.Text(c, label).FontSize(14).TextColor(settingsItemInk)
				})
				if item.Clicked() {
					a.ShowSettings = false
					a.chooseSettingsItem(label)
				}
			}
		})
	})
}

// chooseSettingsItem 执行设置菜单里选中的条目（原版 titleBar.tsx 的
// handleShowAbout / handleShowKeyboardShortcuts / handleCheckUpdate / handleExit）。
func (a *App) chooseSettingsItem(label string) {
	switch label {
	case "关于应用":
		if a.Act.ShowAbout != nil {
			a.Act.ShowAbout()
		}
	case "快捷键":
		if a.Act.ShowShortcuts != nil {
			a.Act.ShowShortcuts()
		}
	case "检查更新":
		if a.Act.CheckUpdate != nil {
			a.Act.CheckUpdate()
		}
	case "退出应用":
		if a.Act.Quit != nil {
			a.Act.Quit()
		}
	}
}
