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

// backdrop 叠出时段背景渐变。
func (a *App) backdrop(c *ui.Context) {
	const bands = 4
	for _, b := range a.Theme.Period.BackdropBands(bands) {
		ui.Box(c).FillWidth().Grow(1).LinearGradient(ui.LinearGradient{
			From: b[0], To: b[1], Angle: 180,
		})
	}
}

// titleBar 是标题栏（.app-title-bar）：高 36，品牌居中，右侧一个「切换到迷你模式」。
//
// 窗口用原生窗口控件（mygo 的 TitleBarHidden：macOS 是红绿灯、Windows 是
// 最小化/最大化/关闭、Linux 是 GTK 的标题按钮），与旧版一致 —— 旧版在
// macOS 上就是 decorations + TitleBarStyle::Overlay。c.TitleBar() 给出原生
// 控件占的宽高，两边留出来就不会被压住，同时保持品牌落在窗口正中。
//
// 旧版在 Linux 上不提供迷你模式（webkit2gtk 的窗口改不了尺寸），所以那里
// 不画这个按钮；原生窗口没这个限制。
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
				// 原版 text-sm（14px）。
				ui.Text(c, "bili-FM").FontSize(14).FontWeight(600).TextColor(t.Ink)
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
		actions.Opacity(titleBarFade(actions, "tb-child", child))
		actions.Children(func() {
			// 迷你入口按钮（原 switch-window-mode，ZoomInternal 图标）已移除：
			// 浮窗应用不需要在标题栏放一颗看似全屏的按钮，避免误导。
		})
		ui.Box(c).Width(tb.Right).Shrink(0)
	})
}

// titleBarLayer 画 .app-title-bar 的 ::before / ::after 那一层：绝对定位、贴住
// 标题栏左右与顶沿、比标题栏高（原版 170% / 140%），整层乘 opacity 后往下淡出。
//
// 整棵子树（含分带）都要 PassThrough：它们盖在标题栏按钮之上，只要有哪一层
// 参与命中测试，标题栏里按下的就是它（或者它的祖先 —— 带 DragWindow 的那一行），
// 按钮就再也点不到了。
func titleBarLayer(c *ui.Context, key string, stops []gradStop, height, opacity float32) {
	box := ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(height).PassThrough()
	// 整层的不透明度走 0.24s 过渡（原版 `transition: opacity 0.24s ease`），
	// 鼠标进出标题栏时白玻璃与顶沿投影是淡入淡出，不是硬切。
	box.Opacity(titleBarFade(box, key, opacity))
	box.Children(func() {
		ui.Column(c).Fill().PassThrough().Children(func() {
			for _, b := range gradBands(stops, 4) {
				ui.Box(c).FillWidth().Grow(1).PassThrough().LinearGradient(ui.LinearGradient{
					From: b[0], To: b[1], Angle: 180,
				})
			}
		})
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
		ui.Text(c, "设置").FontSize(14).TextColor(t.Ink)
	})
	// 点击开合（原版 toggleMenu）；面板由 PopoverBase 放进 Overlay，画在最上层。
	if b.Clicked() {
		a.ShowSettings = !a.ShowSettings
	}
	a.settingsMenu(c, b)
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
