package view

import (
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/imagecache"
)

// Shell 画出整个主窗口。
//
// 布局与原版一一对应（globals.css 的尺寸标在注释里）：
//
//	标题栏 36px     .app-title-bar（品牌居中，窗口按钮靠右）
//	主区   1fr      .home-stage（上：搜索药丸；下：封面圆盘 + 视频信息）
//	播放栏 56px     #player（单行）
//	抽屉   覆盖层   [data-slot="wrapper"] > section（底部滑入）
func (a *App) Shell(c *ui.Context) {
	// 桌面的两个无障碍偏好叠在时段令牌之上，源序是 reduced-transparency 在前、
	// contrast 在后（见 withPreferences）。只在状态真的变了的那一帧重推整份主题
	//（withPreferences 幂等），避免每帧无谓地写共享状态。
	hc := c.Preferences().HighContrast
	rt := a.ReducedTransparency()
	if hc != a.Theme.HighContrast || rt != a.Theme.ReducedTransparency {
		a.Theme = a.Theme.withPreferences(hc, rt)
	}
	if a.Mini {
		a.MiniShell(c)
		return
	}
	t := a.Theme
	c.SetTheme(t.uiTheme())
	a.drainToasts(c)
	a.shortcuts(c, c.Root())
	c.Root().Background(t.Period.Sample(0.5))

	// 背景：时段渐变 + 方向光晕 / 时段光罩（#root::before/::after 一块烘图）
	// + 当前封面的氛围光（原版 #root 的 --liquid-backdrop、#root::before /
	// ::after、.app-shell::before / ::after）。
	ui.Column(c).Fill().Children(func() {
		a.backdrop(c)
		a.lightField(c)
		a.ambient(c)
	})

	// 前景浮在背景之上。
	ui.Column(c).Absolute().Fill().Children(func() {
		a.titleBar(c)
		a.homeStage(c)
		// 播放栏整条贴底、不占流式布局：原版 #player 是 position: fixed，
		// 主区（homeStage）因此能往下探进它背后。见 homeStage 的高度说明。
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Children(func() {
			a.playerBar(c)
		})
		a.drawer(c)
		a.modal(c)
		a.toastViewport(c)
	})
}

// backdrop 铺窗口背景：原版 `#root` 的 `background: var(--liquid-backdrop)` ——
// 一条 **165° 的多停靠点**线性渐变，整块一次铺满（见 backdrop.go 的说明）。
func (a *App) backdrop(c *ui.Context) {
	a.Theme.Period.Fill(ui.Box(c).FillWidth().Grow(1))
}

// titleBar 是标题栏（.app-title-bar）：高 36，品牌居中，右侧依次是
// 「切换到迷你模式」入口、隐身开关，以及（非 macOS）自绘的最小化/关闭键。
//
// 窗口控件：macOS 走原生红绿灯（mygo 的 TitleBarHidden，与旧版的
// decorations + Overlay 一致），`c.TitleBar()` 给出它们占的宽高，两边留出来
// 就不会被压住，同时保持品牌落在窗口正中；Windows / Linux 主窗是 Frameless
// （见 main.go 的 WindowOptions），没有原生 caption，`tb.Right == 0`，下面
// 那个分支画出与原版 `.app-title-bar-btn` 逐值对齐的两颗键。
//
// 与原版的 Linux 差异：原版 Tauri 在 Linux 上不提供迷你模式（webkit2gtk 的
// 无边框窗口改不了尺寸），所以在那里不画迷你入口；mygo 的 Linux 后端直接驱动
// GTK（gtkWindowResize），运行时改尺寸正常，所以三平台都画（见 miniEntryButton）。
func (a *App) titleBar(c *ui.Context) {
	t := a.Theme
	tb := c.TitleBar()
	bar := ui.Row(c).FillWidth().Height(TitleBarHeight).Shrink(0).
		Padding(0, 16).AlignItems(ui.Center).DragWindow()
	// hover-to-reveal（原版 .app-title-bar 的 :hover / :focus-within）：鼠标落在
	// 标题栏上、或里面的控件拿到焦点，整条才从 .32/.7 亮到 .82/1，并浮出顶沿投影
	// 与一层白玻璃。
	child, brand, edge, wash := titleBarReveal(bar.Hovered() || bar.FocusWithin())
	// 品牌在窄窗隐藏（原版 titleBar.tsx：windowWidth <= 600 时 display:none）。
	// 元素本身仍建出来（只不填子节点）——mygo 的元素 id 由「父 id + 已建子元素
	// 个数」决定，少建一个兄弟会让后面的迷你入口换 id、丢掉悬停/焦点状态。
	w, _ := c.Size()
	showBrand := titleBarShowsBrand(w)
	bar.Children(func() {
		// ::before / ::after 是绝对定位的两层贴边渐变，画在子元素之下
		//（原版 z-index 0 < 子元素 1），高度是标题栏的 170% / 140%，往下淡出。
		//
		// 两层都**无条件**建出来（静息时白玻璃那层 opacity 0、整棵子树不绘制）：
		// mygo 的元素 id 由「父 id + 已在建的子元素个数」决定，少建一个兄弟会
		// 让它后面所有兄弟换 id —— 焦点/悬停/动画状态会跟着丢。原版 CSS 里这两层
		// 也是常驻的（只是 opacity 0）。
		titleBarLayer(c, "tb-edge", titleBarEdgeStops, TitleBarHeight*1.7, edge)
		titleBarLayer(c, "tb-wash", titleBarWashStops, TitleBarHeight*1.4, wash)
		mac := viewGOOS == "darwin"
		// 品牌：macOS 居中（红绿灯在左），Windows/Linux 靠左 + 设置在旁
		// （原版 .title-bar-brand 的 isMac 分支）。≤600px 时隐藏（原版如此）。
		//
		// 整块走 .title-bar-brand 的 .7/1，而不是外层子元素的 .32/.82；
		// 设置按钮在品牌里面（原版 DOM 如此），所以也跟着 .7/1。
		brandBox := func(grow, settings bool) {
			box := ui.Row(c).AlignItems(ui.Center)
			// `.title-bar-brand` 的不透明度（.70 ↔ 1）跟着 `> *` 那条 transition
			// 一起走 0.24s。
			box.Opacity(titleBarFade(box, "tb-brand", brand))
			if grow {
				box.Grow(1)
			} else {
				box.Shrink(0)
			}
			box.Children(func() {
				if !showBrand {
					return
				}
				if Logo != nil {
					ui.Image(c, Logo).Size(24, 24)
				}
				// 原版 `text-sm`（14px），字重继承 `.app-title-bar { font-weight: 500 }`
				// （品牌 <span> 与「设置」<button> 都在这条规则下，两边同一个 500）。
				// 此前品牌写 600、设置钮默认 400：`#settings-entry` 与品牌同字号却差
				// 两级字重，CJK 字形的墨迹落点被拉低 1~1.5px，摆在同一行里看上去
				// 「设置钮往下沉」（用户报的 Linux 上标题栏没对齐）。两边统一 500。
				ui.Text(c, "bili-FM").FontSize(14).FontWeight(titleBarFontWeight).TextColor(t.Ink)
				if settings {
					a.settingsButton(c)
				}
			})
		}
		if mac {
			ui.Box(c).Width(tb.Left).Shrink(0)
			ui.Box(c).Grow(1)
			brandBox(false, false)
			ui.Box(c).Grow(1)
		} else {
			ui.Box(c).Width(tb.Left).Shrink(0)
			brandBox(false, true)
			ui.Box(c).Grow(1)
		}
		actions := ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center)
		// `.app-title-bar > *` 的不透明度（.32 ↔ .82）走 0.24s 过渡。
		// 隐身键开启时给本行的静息不透明度加一个下限（见 tbIncognitoRestInk）：
		// 原版 `.app-title-bar > *` 静息只到 .32，而这一行现在**只装隐身键**，
		// .32 会把「开着」的状态压得几乎看不出（用户明确要求状态一眼可见）。
		// 关闭态不设下限——关着的隐身键本就该安静地退到背景里。
		childTarget := child
		if a.Incognito && childTarget < tbIncognitoRestInk {
			childTarget = tbIncognitoRestInk
		}
		actions.Opacity(titleBarFade(actions, "tb-child", childTarget))
		actions.Children(func() {
			// 迷你模式入口（原 #switch-window-mode，ZoomInternal 图标）：用户
			// 2026-10-18 要求「看看 Linux 下能不能恢复迷你模式」。原版 Tauri 在
			// Linux 禁用迷你模式，因为 webkit2gtk 的无边框窗口**无法在运行时改尺寸**；
			// mygo 的 Linux 后端是直接驱动 GTK（internal/linux 的 gtkWindowResize /
			// gtkWindowSetDefaultSize），运行时 SetSize 正常生效，所以这个限制在
			// 原生版不存在——三平台都能进迷你模式。
			//
			// 形态对齐原版：30×30、图标 15px、常态透明 + 图标 72%，悬停玻璃底 +
			// 图标转蓝，按下 scale(.94)（`.app-title-bar-btn` 那套）。
			a.miniEntryButton(c)
			// 隐身模式开关放在这里（右侧动作区，**一切平台、一切窗宽都常驻**）：
			// 它会影响所有列表/播放的云端进度同步，用户明确否掉了「藏在设置菜单/
			// 历史抽屉二级入口」的方案，要求常驻可见、一次点击可达、状态一眼可见。
			// actions 行每帧无条件建出（不像品牌行会在 ≤600px 隐藏），且它就在
			// 原生窗口控件左侧，位置稳定、不易误触。
			a.incognitoToggle(c)
		})
		// 自绘窗口控件（最小化 / 关闭）：原版 titleBar.tsx 在**非 macOS** 上始终
		// 自绘这两颗键（Tauri 的 decorations:false，前端 .app-title-bar-actions
		// 里的 #wds-handle-group）。主窗在 Windows / Linux 上是 Frameless，
		// `tb.Right` 恒为 0，所以两颗键总会画出来 —— 既修掉了「Wayland 下合成器
		// 不给原生按钮 → 关闭/最小化消失」（用户 2026-10-17 报告 ③），也修掉了
		// 「Windows 的 DWM caption 键样式与原版对不上」（用户 2026-10-18 晚间 ①）。
		// 判据保留 `tb.Right <= 0`：万一某个平台仍有原生控件占位（macOS 以外的
		// 隐藏标题栏窗口），也不重复画。
		if !mac && tb.Right <= 0 {
			// 间距/内边距逐值对齐原版 #wds-handle-group（titleBar.tsx:411）：
			// `flex items-center gap-1 px-1 py-0.5` —— Tailwind gap-1 = 4px、
			// px-1 = 左右各 4px、py-0.5 = 上下各 2px。此前写的是 Gap(1)（1px）
			// 且没有 px-1，两颗键几乎贴在一起、右侧也比原版少 4px。
			wc := ui.Row(c).Gap(tbWinCtrlGap).Padding(tbWinCtrlPadY, tbWinCtrlPadX).Shrink(0).AlignItems(ui.Center)
			// 组高 = 键 30 + py-0.5 4 = 34（下边框 1px 由框架画在高度之外，
			// 合计 35 —— 与原版内容撑高同值）。36px 的栏里居中后上下各留 0.5。
			wc.Height(tbWinCtrlSize + 2*tbWinCtrlPadY)
			titleBarCtrlTray(wc, tbTrayRadius())
			// 与 actions 行同一层探用 titleBarFade 走 0.32 ↔ 0.82 的
			// hover-to-reveal（原版 `.app-title-bar > *`）；按钮自身那层
			// 0.72 ↔ 1 由 titleBarButtonStyle 给（`.app-title-bar-btn`）。
			wc.Opacity(titleBarFade(wc, "tb-wc", child))
			wc.Children(func() {
				a.windowControlButton(c, "tb-min", "最小化", iconMinus, false, func() {
					if a.Act.Minimize != nil {
						a.Act.Minimize()
					}
				})
				a.windowControlButton(c, "tb-close", "隐藏窗口", iconClose, true, func() {
					// 原版关闭键调 hide_window（隐藏到托盘），不是退出应用——
					// 与 mygo 的 OnClose（关窗=隐藏到托盘）同一条路径。
					if a.Act.HideWindow != nil {
						a.Act.HideWindow()
					}
				})
			})
		} else {
			ui.Box(c).Width(tb.Right).Shrink(0)
		}
	})
}

// titleBarLayer 画 .app-title-bar 的 ::before / ::after 那一层：绝对定位、贴住
// 标题栏左右与顶沿、比标题栏高（原版 170% / 140%），整层乘 opacity 后往下淡出。
//
// 整棵子树（含分带）都要 PassThrough：它们盖在标题栏按钮之上，只要有哪一层
// 参与命中测试，标题栏里按下的就是它（或者它的祖先 —— 带 DragWindow 的那一行），
// 按钮就再也点不到了。
func titleBarLayer(c *ui.Context, key string, stops []gradStop, height, opacity float32) {
	titleBarLayerFn(c, key, stops, height, opacity)
}

// titleBarLayerFn 是 titleBarLayer 的实际绘制（测试可换成旧的「4 条带拼法」）。
var titleBarLayerFn = func(c *ui.Context, key string, stops []gradStop, height, opacity float32) {
	box := ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(height).PassThrough()
	// 整层的不透明度走 0.24s 过渡（原版 `transition: opacity 0.24s ease`），
	// 鼠标进出标题栏时白玻璃与顶沿投影是淡入淡出，不是硬切。
	box.Opacity(titleBarFade(box, key, opacity))
	// 一条**多停靠点**渐变整块铺满（`ui.LinearGradient.Stops`），不再按 4 条
	// 等高带拼。180° 纯垂直下拼法的误差本来就小（实测最坏 2 灰阶，因为渐变线
	// 与带边垂直，逐带重投影只在带内保留比例），但两处 135° 的同类层误差达
	// **38 灰阶 / 0.68 alpha**（见 dialogSheen / loginSheen），所以统一走真停靠点：
	// 一处写法、一处测试口径，也就不再有「某个角度下才出问题」的暗礁。
	box.Children(func() {
		ui.Box(c).Fill().PassThrough().
			LinearGradient(ui.LinearGradient{Angle: 180, Stops: gradStops(stops)})
	})
}

// settingsButton 是标题栏里的「设置」下拉（原版非 macOS 平台的 #settings-entry）：
// 关于应用 / 快捷键 / 检查更新 / 退出应用。它是 .title-bar-brand 的子元素，所以
// 跟着品牌的不透明度走（由外层 brandBox 施加），自己不再设 opacity。
//
// 原版 Tailwind 类：`hover:bg-gray-200 px-2 py-1 rounded transition-colors
// text-sm ml-2` —— px-2/py-1（8/4）、rounded（4px）、text-sm（14px）、ml-2（8px）；
// 悬停底色被 `.app-title-bar button:hover { … !important }` 顶成 --glass-bg-hover。
//
// 下拉面板不是系统菜单，是原版手搓的液态玻璃面板（见 settingsMenu）。
func (a *App) settingsButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("settings")).Height(28).Padding(0, 8).Radius(4).
		Margin(0, 0, 0, 8).Center().Label("设置").Tooltip("设置")
	// 原版 #settings-entry 是 `aria-haspopup="menu" aria-expanded={showMenu}` 的
	// button：无障碍上是一个「打开菜单、当前是否展开」的菜单键。mygo 对应
	// RoleMenuButton + Expanded(a.ShowSettings)（两者都让 Windows 给
	// ExpandCollapsePattern、macOS 读 AXExpanded）。
	b.Role(ui.RoleMenuButton).Expanded(a.ShowSettings)
	if b.Hovered() || a.ShowSettings {
		// `.app-title-bar button` 的 `background-color 0.18s ease`：悬停底色淡入，
		// 不再「啪」地跳出来（设置键自己不设 opacity，它跟着品牌那层 0.24s 走）。
		b.Background(b.AnimateColor("tb-btn-bg", t.GlassHover, navIconBgAnim))
	} else {
		b.Background(b.AnimateColor("tb-btn-bg", ui.Transparent, navIconBgAnim))
	}
	// #settings-entry 也是 `.app-title-bar button`，同样有 ::after 那层三条
	// 阴影（悬停 0.18s 淡入 / 移开淡出）。此前原生版只换了底色、漏了这层高光。
	titleBarBtnHighlight(b, false, b.Hovered())
	// 原版 .app-title-bar button:hover 的左上角径向聚光（.settings-entry 也是
	// .app-title-bar 里的 button，按下换居中光）。
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, 4)
	}
	b.Children(func() {
		// 同品牌文字：字重 500（见 titleBarFontWeight），并按 titleBarSettingsNudge
		// 光学上移，抵消 CJK 字形相对拉丁品牌的墨迹下沉。
		ui.Text(c, "设置").FontSize(14).FontWeight(titleBarFontWeight).TextColor(t.Ink).
			Translate(0, -titleBarSettingsNudge)
	})
	// 点击开合（原版 toggleMenu）；面板由 PopoverBase 放进 Overlay，画在最上层。
	if b.Clicked() {
		a.ShowSettings = !a.ShowSettings
	}
	a.settingsMenu(c, b)
}

// windowControlButton 画出标题栏窗口控制区的一颗键（见 titleBar 的窗口控件分支）。
// close 为真时悬停图标转暖红（原版 `.app-title-bar button.app-title-bar-close:hover`
// 的 #e11d48 + 玫红底）。
func (a *App) windowControlButton(c *ui.Context, key, label string, ic *ui.SVG, close bool, fn func()) {
	a.windowControlButtonSized(c, key, label, ic, close, tbWinCtrlSize, 14, fn)
}

// titleBarCtrlTray 给窗口控制键组铺上原版 `#wds-handle-group` 的**内凹玻璃托盘**
// （globals.css:1913-1922，值见 theme.go 的 tbTrayBg / tbTrayBorder / tbTrayInk /
// tbTrayRim）：
//
//	background: rgba(255,255,255,.06);
//	border: 1px solid rgba(255,255,255,.18); border-top: 0;
//	border-radius: 0 0 var(--app-window-radius) var(--app-window-radius);
//	box-shadow: inset 0 2px 4px rgba(15,23,42,.08), inset 0 -1px 0 rgba(255,255,255,.12);
//
// 上边**没有**边框、上面两个角**不**圆：托盘是从窗顶挂下来的凹槽，不是一颗
// 浮起的键。radius 由调用点给（主窗 tbTrayRadius()，迷你窗 0 —— 原生版迷你窗
// 的内容层没有圆角，见 miniwindowradius_test.go）。
func titleBarCtrlTray(el ui.Element, radius float32) {
	el.Background(tbTrayBg).
		BorderWidth(0, 1, 1, 1).BorderColor(tbTrayBorder).
		Radius(0, 0, radius, radius).
		InsetShadow(0, 2, 4, 0, tbTrayInk).
		InsetShadow(0, -1, 0, 0, tbTrayRim)
}

// windowControlButtonSized 是 windowControlButton 的可调尺寸版：迷你模式的
// 标题栏只有 24px 高，原版 `body.mini-mode .app-title-bar #wds-handle-group
// .app-title-bar-btn` 把两颗键收到 20×20、图标 12（主窗是 30×30 / 14）。
// 除了尺寸，其余样式（悬停玻璃底 + 蓝图标、关闭键暖红、按压缩放、::after 三条
// 阴影、左上角聚光、焦点环）与主窗完全同一套。
func (a *App) windowControlButtonSized(c *ui.Context, key, label string, ic *ui.SVG, close bool, size, iconSize float32, fn func()) {
	t := a.Theme
	b := titleBarIconButton(c, key, label).Size(size, size)
	// 两颗键都是非开关的动作键，所以「激活」态传给 false；悬停/按下/聚焦由
	// titleBarButtonStyle + 下面两条覆写处理。
	bg, ink, opacity, _ := titleBarButtonStyle(t, false, b.Hovered(), b.Pressed(), false)
	focus := b.FocusVisible()
	if focus {
		ink, opacity = t.Blue, 1
		if !b.Hovered() {
			bg = ui.Transparent
		}
	}
	// 关闭键暖红悬停（原版特异度 (2,2,1) 且 !important，压过通用的悬停蓝）：
	// 图标 #e11d48、底色 rgba(225,29,72,.10)。
	if close && (b.Hovered() || b.Pressed()) {
		ink = ui.Hex("#e11d48")
		bg = ui.Hex("#e11d48").Alpha(0.10)
	}
	b.Background(bg).Opacity(opacity)
	if focus {
		b.InsetShadow(0, 1, 1, 0, t.Highlight)
	}
	titleBarBtnHighlight(b, false, b.Hovered())
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, RadiusSmall)
	}
	pressScale(b)
	b.Children(func() {
		ui.Icon(c, ic).Size(iconSize, iconSize).TextColor(ink)
	})
	focusOutline(b, RadiusSmall, focusGlow)
	if b.Clicked() {
		fn()
	}
}

// miniEntryButton 是标题栏右侧的「切换到迷你模式」入口（原 #switch-window-mode）。
//
// 它曾被移除（`53f7cb5`，理由「浮窗应用不需要看似全屏的按钮」），但用户 2026-10-18
// 明确要求恢复 Linux 的迷你模式；且移除后迷你模式**完全无法从界面进入**
// （只剩 `--mini` 启动参数），所以本体恢复。
//
// 形态：30×30、图标 15px（原版 `<ZoomInternal size="15">`）、常态透明 + 图标 72%，
// 悬停玻璃底 + 图标转蓝，按下 scale(.94)。
func (a *App) miniEntryButton(c *ui.Context) {
	t := a.Theme
	b := titleBarIconButton(c, "switch-mode", "切换到迷你模式")
	bg, ink, opacity, _ := titleBarButtonStyle(t, false, b.Hovered(), b.Pressed(), false)
	focus := b.FocusVisible()
	if focus {
		ink, opacity = t.Blue, 1
		if !b.Hovered() {
			bg = ui.Transparent
		}
	}
	b.Background(bg).Opacity(opacity)
	if focus {
		b.InsetShadow(0, 1, 1, 0, t.Highlight)
	}
	titleBarBtnHighlight(b, false, b.Hovered())
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, RadiusSmall)
	}
	pressScale(b)
	b.Children(func() { ui.Icon(c, iconMini).Size(15, 15).TextColor(ink) })
	focusOutline(b, RadiusSmall, focusGlow)
	if b.Clicked() && a.Act.SetMini != nil {
		a.Act.SetMini(true)
	}
}

// titleBarIconButton 是标题栏右侧动作区里图标键的公共壳（尺寸/圆角/无障碍）。
func titleBarIconButton(c *ui.Context, key, label string) ui.Element {
	return ui.ButtonBase(c.Key(key)).Size(30, 30).Radius(RadiusSmall).Center().
		Label(label).Tooltip(label)
}

// ambient 是「氛围光」：把当前封面铺满整窗、压到 31% 不透明度，再叠一层
// 白渐变 —— 原版 .app-shell::before / ::after。这是 bili-FM 的视觉签名，所有
// 玻璃都浮在它上面。
//
// 原版还有 saturate(0.78) brightness(1.14) contrast(0.76) 与 filter: blur(20px)。
// 注意原版用的是 **filter**（作用于元素自身）而不是 backdrop-filter，所以原生版
// 没有理由为它留一个全窗 backdrop；这四步现在全部在 imagecache.gradeAmbient
// 里随封面烤进 64px 源图（blur 由「高级质感」开关决定），每帧只画一张图。
//
// 封面没加载好就什么都不画（imagecache 未命中返回 nil），所以切歌时不会先闪
// 一下空白 —— 旧版为此专门做了「先预载、后换源」。
func (a *App) ambient(c *ui.Context) {
	if !a.Ambient {
		return
	}
	// 氛围光要用**调过色**的那份（原版这一层带 saturate/brightness/contrast
	// 滤镜，见 imagecache.Ambient），拿不到就退回原图。
	bmp := a.Images.Ambient(a.coverURL(), imagecache.WidthAmbient, a.Premium)
	if bmp == nil {
		bmp = a.coverBitmap()
	}
	if bmp == nil {
		return
	}
	ui.Box(c).Absolute().Fill().PassThrough().Children(func() {
		// 调色 + 模糊已经烤进位图（imagecache.gradeAmbient），所以这里直接铺图：
		// 不再为氛围光每帧算一个全窗 backdrop（那是面积最大的一层）。
		ui.Image(c, bmp).Fill().Fit(ui.Cover).Opacity(0.31)
		// ::after：一层白渐变，把氛围光压得更淡更匀。
		ui.Box(c).Absolute().Fill().PassThrough().LinearGradient(ui.LinearGradient{
			From:  ui.Hex("#f8fbff").Alpha(0.22),
			To:    ui.Hex("#f8fbff").Alpha(0.14),
			Angle: 180,
		})
	})
}

// homeStage 是标题栏与播放栏之间的主区（.home-stage）：
// 宽窗内边距 20 28 0 28；窄窗（≤680）12 16 0 16
// （原版 @media (max-width: 680px)）。
//
// **高度是「整窗 - 播放栏」，不是「整窗 - 标题栏 - 播放栏」**：原版
// `.app-main { height: calc(100vh - 36px) }` 同时带 flex-grow: 1，作为
// `.app-shell`（h-screen 的 flex 列）唯一子元素会被撑到整窗 —— 100% 于是等于
// 100vh，`.home-stage { height: calc(100% - 56px) }` = 100vh - 56。主区因此在
// 标题栏之下、又往播放栏底下探进标题栏那 36px。播放栏是 `position: fixed`，
// 本来就不占文档流，探进去的部分是主区自己的留白（now-playing 的 32px 底距），
// 不会真的被压住。
//
// 这一点决定右栏整块的落点：视频信息盒 366 固定高、在外层纵向居中，
// 主区高 544（600 窗）时盒顶 = 142，正好是原版截图里 UP 主行的位置；
// 少算标题栏那 36px（主区 508）会把整块顶上去 18px —— 用户实测反馈的
// 「UP 主离搜索框太近、右栏组件位置不对」就是这个。
// 对应地播放栏不再参与流式布局（Shell 把它整条套进贴底的 absolute 盒里）。
func (a *App) homeStage(c *ui.Context) {
	w, h := c.Size()
	l := homeLayoutFor(w)
	ui.Column(c).FillWidth().Height(h-PlayerBarHeight).Shrink(0).
		Padding(l.StagePadY, l.StagePadX, 0, l.StagePadX).Children(func() {
		a.searchBar(c, l)
		a.nowPlaying(c, l)
	})
}
