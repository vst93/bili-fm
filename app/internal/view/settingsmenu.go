package view

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 设置菜单
//
// 面板材质 = 玻璃 + `--glass-sheen-static` 洗光（见 settingsMenu 的 Draw），
// 与条目自己的三态底色一起构成原版 #settings-menu 的那块液态玻璃。

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
	settingsMenuMinW   = 208 // 设置菜单含开关行后的宽度（原版 120 只够纯文字条目）
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
	t := a.Theme
	ui.PopoverBase(c, anchor, &a.ShowSettings, func(panel ui.Element) {
		// 原版 #settings-menu：min-w 120、圆角 8、py-1、玻璃底 + 白边 + 两层投影。
		// 材质用框架玻璃（与原版同一种液态玻璃，自带边缘聚光），外投影按原版
		// 的 --glass-specular + 0 18px 44px 补上。
		panel.MinWidth(settingsMenuMinW).Padding(settingsMenuPadY, 0).
			Radius(settingsMenuRadius).Margin(settingsMenuGap, 0, 0, 0).
			// 材质：原版 #settings-menu 是 `background-color: var(--glass-bg)`
			// （白 .24）+ `backdrop-filter: var(--liquid-blur)`（blur 20px）。
			// 用 glass.Blur（只模糊）+ 自铺白 .24 才对得上；此前用 Regular
			// 玻璃（glass.Glass{}）会把面板提亮 —— 实测面板右中比背景亮 +20.9
			// 灰阶，原版白 .24 只值 +6，整块比原版白一档。
			Material(blurLayer(glass.Blur{Radius: glassBlurSigma})).
			Border(1, t.GlassBorder).
			// 原版 `#settings-menu { background-image: var(--glass-sheen-static)
			// !important }`（globals.css:1928）：在玻璃底之上、条目之下再铺一层
			// 「左上径向热点 + 170° 斜向洗光」。此前只画了玻璃材质，面板顶左比
			// 原版平一档（同为玻璃浮层的抽屉 tab / 搜索框 / 播放栏都已铺这条洗光，
			// 只有设置菜单与合作 UP 主下拉漏着）。
			Draw(func(p *ui.Painter, r ui.Rect) {
				paintGlassPanelFill(p, r, settingsMenuRadius, t.Glass)
				paintBakedSheen(p, r, settingsMenuRadius, sheenGlassBaked)
			}).
			InsetShadow(0, 1, 1, 0, a.Theme.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.06)).
			Shadow(0, 18, 44, 0, ui.Hex("#0f172a").Alpha(0.10))
		panel.Children(func() {
			// 动作条目保持原版顺序；三个播放偏好接在下面，用一条淡线隔开。
			// 开关行不关闭菜单，方便连续调整。
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
			ui.Box(c).FillWidth().Height(1).Margin(4, 0).Shrink(0).
				Background(ui.Hex("#64748b").Alpha(0.16))
			a.settingsToggle(c, "桌面字幕", a.LyricsEnabled, func() {
				if a.Act.ToggleDesktopLyrics != nil {
					a.Act.ToggleDesktopLyrics()
				}
			})
			a.settingsToggle(c, "弹幕热力", a.HeatOn, func() {
				if a.Act.ToggleHeat != nil {
					a.Act.ToggleHeat()
				}
			})
			// 一句话说明播放栏里的两种图形：底部面积是弹幕密度，进度条
			// 上的短竖线是章节分界，不是第二组热力数据。
			ui.Text(c, "面积=弹幕密度；短竖线=章节分界").
				FontSize(11).TextColor(settingsItemInk.Alpha(0.65)).
				Padding(0, settingsItemPadX).Margin(0, 0, 6, 0)
			a.settingsToggle(c, "顺着听", a.RadioOn, func() {
				if a.Act.ToggleRadio != nil {
					a.Act.ToggleRadio()
				}
			})
		})
	})
}

// incognitoToggle 是标题栏右侧的常驻「隐身模式」开关（用户 2026-10-17 报告）。
//
// 背景：隐身模式影响 **所有**列表/播放的云端进度读写，此前只在历史抽屉表头
// （原版 `.history-incognito-switch`）和设置下拉底部各有一个入口。用户原话：
// 「隐身模式放设置菜单中太蠢了，一般用户怎么会去点他」，明确要求入口**常驻
// 可见、一次点击可达、状态一眼可见**，不得藏进任何二级菜单。
//
// 选位：标题栏右侧动作区（`actions` 行）。它是全平台、全窗宽都每帧无条件建出的
// 一行（品牌行在 ≤600px 会被隐藏，设置下拉要展开才看得到），且紧邻原生窗口
// 控件，位置固定、一眼可见、一下即点，不用先展开任何菜单。
//
// 形态：复用标题栏小键（`.app-title-bar-btn`）的 30×30 / 圆角 6 / 悬停玻璃底 +
// 聚光 + 三条土阴影 + 按压 scale(.94)，与最小化/关闭键同一套观感；图标换成
// mask（开启）与 eye-off 轮廓（关闭），墨色开启时转主题青（`incognitoInk`），
// 与历史抽屉那颗开关**同一事实源**，状态一眼可辨。
//
// 无障碍：`role="switch"` + `aria-checked`（mygo 的 RoleSwitch + Checked），
// 与另两个入口同语义；tooltip 用 `incognitoTip`（随开关态切换）。
func (a *App) incognitoToggle(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("tb-incognito")).Size(30, 30).Radius(RadiusSmall).
		Center().Label("隐身模式").Tooltip(incognitoTip(a.Incognito)).
		Role(ui.RoleSwitch).Checked(a.Incognito)
	// 与标题栏小键同一套三态底色/墨色/不透明度（titleBarButtonStyle）。
	//
	// 用户 2026-10-18 晚间 ②：「激活态底色突兀」。此前把 a.Incognito 传给
	// titleBarButtonStyle 的 active 参数，开启后常态就常驻一块 GlassHover 玻璃底；
	// 这里 active 固定 false —— 底色只由 hover / pressed 决定（与相邻的迷你窗键
	// 完全一致），开启状态只改图标墨色（incognitoInk(true)）与图标 filled。
	bg, ink, opacity, _ := titleBarButtonStyle(t, false, b.Hovered(), b.Pressed(), false)
	// 隐身开启时用主题青（incognitoInk）取代标题栏通用的 --studio-blue，
	// 与历史抽屉/设置菜单两颗开关的墨色一致（同一事实源）。
	if a.Incognito {
		ink = incognitoInk(true)
	}
	bg, ink, opacity = titleBarBtnFade(b, bg, ink, opacity, reduceMotion(c))
	b.Background(bg).Opacity(opacity)
	// 三条土阴影（::after）与悬停聚光（::before）同标题栏小键；active 同样传
	// false，避免开启态常驻一层高光（只保留悬停时的聚光）。
	titleBarBtnHighlight(b, false, b.Hovered())
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, RadiusSmall)
	}
	pressScale(b)
	focusOutline(b, RadiusSmall, focusGlow)
	b.Children(func() {
		ui.Icon(c, pick(a.Incognito, iconMaskFilled, iconMask)).Size(16, 16).TextColor(ink)
	})
	if b.Clicked() && a.Act.ToggleIncognito != nil {
		a.Act.ToggleIncognito()
	}
}

// settingsToggle 是设置菜单里的开关行：左侧标签，右侧和隐身开关同款的小轨道。
// 点击整行切换，菜单保持展开，方便连续调三个播放偏好。
func (a *App) settingsToggle(c *ui.Context, label string, on bool, toggle func()) {
	item := ui.ButtonBase(c.Key("settings-toggle-"+label)).
		FillWidth().Height(34).Padding(0, settingsItemPadX).
		Justify(ui.Start).Radius(0).Label("设置：" + label).
		Tooltip("设置：" + label).
		Role(ui.RoleSwitch).Checked(on)
	bg, inset, scale := settingsItemPaint(a.Theme, item.Hovered(), item.Pressed())
	item.Background(bg)
	if inset {
		item.InsetShadow(0, 1, 1, 0, a.Theme.Highlight)
	}
	p := ui.Pose{}
	if scale != 1 {
		p.Scale = scale
	}
	item.AnimatePose(p, ui.Animation{Duration: 100 * time.Millisecond, Ease: ui.EaseOut})
	item.Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Text(c, label).FontSize(13).TextColor(settingsItemInk).Grow(1)
			incognitoTrack(c, on)
		})
	})
	if item.Clicked() {
		toggle()
	}
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
