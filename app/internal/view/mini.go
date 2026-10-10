package view

import (
	"github.com/egoist/mygo/ui"
)

// miniWidth / miniHeight 是迷你模式的窗口尺寸，与原版一致（400×155）。
// 原版在 Linux 上禁用迷你模式，因为 webkit2gtk 的无边框窗口无法在运行时
// 调整大小；原生窗口没有这个限制，所以三个平台都能用。
const (
	miniWidth  = 400
	miniHeight = 155

	// miniTitleBar 是迷你模式的标题栏高度；整窗 = 24 + 88 + 43 = 155。
	miniTitleBar = 24
	miniInfoH    = 88
	miniPlayerH  = 43
	miniCover    = 64
)

// MiniShell 是迷你模式的界面（原版 body.mini-mode）：
//
//	24px  标题栏（macOS 是原生红绿灯；Windows/Linux 是原生窗口按钮）
//	88px  #min-video-info：64px 封面 + 标题/选集 + 右侧窗口控制
//	43px  #player：切曲 / 当前时间 / 进度 / 时长 / 音量
//
// 迷你模式的背景是固定的浅色渐变，不跟时段（body.mini-mode #root）。
func (a *App) MiniShell(c *ui.Context) {
	t := a.Theme
	c.SetTheme(t.uiTheme())
	a.drainToasts(c)
	a.shortcuts(c)
	c.Root().Background(ui.Hex("#f7fafd"))

	ui.Column(c).Fill().Children(func() {
		ui.Box(c).FillWidth().Grow(1).LinearGradient(ui.LinearGradient{
			From: ui.Hex("#fbfdff"), To: ui.Hex("#f3f8fb"), Angle: 145,
		})
	})

	ui.Column(c).Absolute().Fill().Children(func() {
		// 标题栏：只用来拖动窗口（原版 mini 下把内容整体淡出，只留窗口按钮）。
		ui.Row(c).FillWidth().Height(miniTitleBar).Shrink(0).DragWindow()

		// 主体：封面 + 标题 + 选集 + 窗口控制。
		// 整行可拖动窗口（原版 #min-video-info 带 data-tauri-drag-region="deep"）。
		ui.Row(c).FillWidth().Height(miniInfoH).Shrink(0).Padding(6, 12).
			Gap(11).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(miniCover, miniCover).Shrink(0).Radius(10).
				Background(ui.Hex("#ffffff").Alpha(0.58)).Clip().Children(func() {
				// 封面缺失时退回 logo（原版 cover || "/logo.png"）。
				if bmp := a.coverBitmap(); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				} else if Logo != nil {
					ui.Image(c, Logo).Fill().Fit(ui.Cover)
				}
				// 高光（原版 .mini-cover-shine）：左上到右下的白渐变。
				ui.Box(c).Fill().PassThrough().LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#ffffff").Alpha(0.30),
					To:    ui.Transparent,
					Angle: 135,
				})
			})

			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Justify(ui.Center).Children(func() {
				ui.Text(c, a.infoTitle()).FontSize(15).Bold().TextColor(t.Ink).MaxLines(2)
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					// 播放列表模式圆点玫红 + 外圈光晕（原版
					// .mini-status-dot-playlist 的 box-shadow ring）。
					inList := a.PlayingPlaylist != ""
					dotInk := pick(inList, t.Rose, t.Blue)
					dot := ui.Box(c).Size(7, 7).Shrink(0).Margin(0, 0, 0, 5).
						Radius(RadiusPill).Background(dotInk)
					dot.Shadow(0, 0, 0, 4, dotInk.Alpha(0.12))
					ui.Text(c, a.partTitle()).FontSize(12).Bold().
						TextColor(ui.Hex("#526174")).SingleLine()
				})
			})

			ui.Column(c).Width(30).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
				a.miniControl(c, "mini-restore", "切换到窗口模式", iconRestore, func() {
					if a.Act.SetMini != nil {
						a.Act.SetMini(false)
					}
				})
				a.miniControl(c, "mini-pin", pick(a.Pinned, "取消窗口置顶", "窗口置顶"),
					iconPin, func() {
						if a.Act.TogglePin != nil {
							a.Act.TogglePin()
						}
					})
			})
		})

		// 播放栏：切曲 / 当前时间 / 进度 / 时长 / 音量。
		a.miniPlayerBar(c)

		// 音量弹层（迷你窗里也要能弹出来，否则音量键点了没反应）。
		if a.ShowVolume {
			a.volumePopoverAt(c, 8, 44)
		}
		a.toastViewport(c, 50)
	})
}

// miniPlayerBar 是迷你模式的播放栏（原版 body.mini-mode #player）：
// 高 43、内边距 0 10，列宽 59 / 44 / 1fr / 44 / 34，间距 4。
func (a *App) miniPlayerBar(c *ui.Context) {
	t := a.Theme
	ui.Column(c).FillWidth().Height(miniPlayerH).Shrink(0).
		Background(t.PlayerSurface).Children(func() {
		ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.PlayerBorder)
		ui.Row(c).FillWidth().Grow(1).Padding(0, 10).Gap(4).
			AlignItems(ui.Center).Children(func() {
			// 59px：播放 28 + 3 + 下一集 28。
			ui.Row(c).Width(59).Shrink(0).Gap(3).AlignItems(ui.Center).Children(func() {
				play := ui.ButtonBase(c.Key("mini-play")).Size(28, 28).Radius(Radius).Center().
					Label(pick(a.Playing, "暂停", "播放"))
				play.Background(t.Blue.Alpha(0.18)).Border(1, t.Blue.Alpha(0.35))
				play.Children(func() {
					ui.Icon(c, pick(a.Playing, iconPause, iconPlay)).Size(13, 13).
						TextColor(ui.Hex("#0369a1"))
				})
				if play.Clicked() && a.Act.TogglePlay != nil {
					a.Act.TogglePlay()
				}

				next := ui.ButtonBase(c.Key("mini-next")).Size(28, 28).Radius(Radius).Center().
					Label("下一集")
				next.Disabled(!a.canNavigate())
				if next.Hovered() {
					next.Background(t.GlassHover)
				} else {
					next.Background(ui.Transparent)
				}
				next.Border(1, t.GlassBorder)
				next.Children(func() { ui.Icon(c, iconNext).Size(13, 13).TextColor(ui.Hex("#334155")) })
				if next.Clicked() && a.Act.Next != nil {
					a.Act.Next()
				}
			})

			ui.Text(c, fmtTime(a.Pos)).FontSize(9).TextColor(ui.Hex("#334155").Alpha(0.72)).
				Width(44).Shrink(0).Center()

			// 与主窗同一套 seek 状态机（见 progress）：拖动中停在目标值，
			// 松手发 seek、等引擎追上才恢复跟随——否则滑块弹回旧位置。
			a.miniProgress(c)

			ui.Text(c, fmtTime(a.Dur)).FontSize(9).TextColor(ui.Hex("#334155").Alpha(0.72)).
				Width(44).Shrink(0).Center()

			vol := ui.ButtonBase(c.Key("mini-volume")).Size(28, 28).Radius(Radius).Center().
				Label("音量")
			if vol.Hovered() {
				vol.Background(t.GlassHover)
			} else {
				vol.Background(ui.Transparent)
			}
			vol.Border(1, t.GlassBorder)
			vol.Children(func() { ui.Icon(c, iconVolume).Size(13, 13).TextColor(ui.Hex("#334155")) })
			if vol.Clicked() {
				a.ShowVolume = !a.ShowVolume
			}
		})
	})
}

// miniControl 是迷你模式右上角的窗口控制键（20×20，原版 .app-title-bar-btn）。
func (a *App) miniControl(c *ui.Context, key, label string, ic *ui.SVG, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(20, 20).Radius(RadiusSmall).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(t.GlassHover)
	} else {
		b.Background(ui.Transparent)
	}
	b.Children(func() { ui.Icon(c, ic).Size(12, 12).TextColor(t.Muted) })
	if b.Clicked() {
		fn()
	}
}

// MiniSize 返回迷你模式的窗口尺寸，供装配层调整窗口用。
func MiniSize() (int, int) { return miniWidth, miniHeight }
