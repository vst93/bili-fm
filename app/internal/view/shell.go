package view

import (
	"runtime"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
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
	if a.Mini {
		a.MiniShell(c)
		return
	}
	t := a.Theme
	c.SetTheme(t.uiTheme())
	a.drainToasts(c)
	a.shortcuts(c)
	c.Root().Background(t.Period.Sample(0.5))

	// 背景：时段渐变 + 当前封面的氛围光（原版 .app-shell::before / ::after）。
	ui.Column(c).Fill().Children(func() {
		a.backdrop(c)
		a.ambient(c)
	})

	// 前景浮在背景之上。
	ui.Column(c).Absolute().Fill().Children(func() {
		a.titleBar(c)
		a.homeStage(c)
		a.playerBar(c)
		a.drawer(c)
		a.modal(c)
	})
}

// backdrop 叠出时段背景渐变。
func (a *App) backdrop(c *ui.Context) {
	const bands = 12
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
	ui.Row(c).FillWidth().Height(TitleBarHeight).Shrink(0).
		Padding(0, 16).AlignItems(ui.Center).DragWindow().Children(func() {
		ui.Box(c).Width(tb.Left).Shrink(0)
		ui.Box(c).Grow(1)
		ui.Row(c).Gap(6).Shrink(0).AlignItems(ui.Center).Children(func() {
			if Logo != nil {
				ui.Image(c, Logo).Size(24, 24)
			}
			ui.Text(c, "bili-FM").FontSize(13).Bold().TextColor(t.Ink)
		})
		ui.Box(c).Grow(1)
		ui.Row(c).Gap(4).Shrink(0).AlignItems(ui.Center).Children(func() {
			a.settingsButton(c)
			if runtime.GOOS != "linux" {
				b := ui.ButtonBase(c.Key("switch-mode")).Size(30, 30).Radius(RadiusSmall).
					Center().Label("切换到迷你模式").Tooltip("切换到迷你模式")
				if b.Hovered() {
					b.Background(t.GlassHover)
				} else {
					b.Background(ui.Transparent)
				}
				b.Children(func() { ui.Icon(c, iconMini).Size(15, 15).TextColor(t.Muted) })
				if b.Clicked() && a.Act.SetMini != nil {
					a.Act.SetMini(true)
				}
			}
		})
		ui.Box(c).Width(tb.Right).Shrink(0)
	})
}

// settingsButton 是标题栏里的「设置」下拉（原版非 macOS 平台的 #settings-entry）：
// 关于应用 / 快捷键 / 检查更新 / 退出应用。
func (a *App) settingsButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("settings")).Height(24).Padding(0, 9).Radius(RadiusSmall).
		Center().Label("设置").Tooltip("设置")
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() {
		ui.Text(c, "设置").FontSize(13).TextColor(t.Ink)
	})
	b.Menu(func(m *ui.Menu) {
		if m.Item("关于应用").Chosen() && a.Act.ShowAbout != nil {
			a.Act.ShowAbout()
		}
		if m.Item("快捷键").Chosen() && a.Act.ShowShortcuts != nil {
			a.Act.ShowShortcuts()
		}
		if m.Item("检查更新").Chosen() && a.Act.CheckUpdate != nil {
			a.Act.CheckUpdate()
		}
		m.Separator()
		if m.Item("退出应用").Chosen() && a.Act.Quit != nil {
			a.Act.Quit()
		}
	})
}

// AmbientBlur 是氛围光的模糊半径（原版 .app-shell::before 的 filter: blur(20px)）。
const AmbientBlur = 20

// ambient 是「氛围光」：把当前封面铺满整窗、模糊、压到 31% 不透明度，再叠一层
// 白渐变 —— 原版 .app-shell::before / ::after。这是 bili-FM 的视觉签名，所有
// 玻璃都浮在它上面。
//
// 原版还有 saturate(0.78) brightness(1.14) contrast(0.76)，glass.Blur 只做高斯
// 模糊，这三个滤镜没有对应能力（想要更淡可以调 Opacity）。
//
// 封面没加载好就什么都不画（imagecache 未命中返回 nil），所以切歌时不会先闪
// 一下空白 —— 旧版为此专门做了「先预载、后换源」。
func (a *App) ambient(c *ui.Context) {
	if !a.Ambient {
		return
	}
	uri := a.coverURL()
	if uri == "" {
		return
	}
	bmp := a.Images.Bitmap(uri)
	if bmp == nil {
		return
	}
	ui.Box(c).Absolute().Fill().PassThrough().Children(func() {
		ui.Image(c, bmp).Fill().Fit(ui.Cover).Opacity(0.31)
		// 模糊它下面画过的东西（渐变 + 封面）。关掉「高级质感」时不模糊：
		// 少一个离屏合成，代价是背景更锐利一些（原版 premiumTexture 的用意）。
		if a.Premium {
			ui.Box(c).Absolute().Fill().PassThrough().
				Material(glass.Blur{Radius: AmbientBlur})
		}
		// ::after：一层白渐变，把氛围光压得更淡更匀。
		ui.Box(c).Absolute().Fill().PassThrough().LinearGradient(ui.LinearGradient{
			From:  ui.Hex("#f8fbff").Alpha(0.22),
			To:    ui.Hex("#f8fbff").Alpha(0.14),
			Angle: 180,
		})
	})
}

// homeStage 是标题栏与播放栏之间的主区（.home-stage）：
// 高 = 100% - 56px，内边距 20 28 0 28，纵向排「搜索药丸 + 正在播放」。
func (a *App) homeStage(c *ui.Context) {
	ui.Column(c).FillWidth().Grow(1).Shrink(1).Padding(20, 28, 0, 28).Children(func() {
		a.searchBar(c)
		a.nowPlaying(c)
	})
}
