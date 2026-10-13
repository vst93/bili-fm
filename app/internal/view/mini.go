package view

import (
	"github.com/egoist/mygo/plugins/glass"
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
	a.shortcuts(c, c.Root())
	c.Root().Background(ui.Hex("#f7fafd"))

	// 背景：145° 线性渐变 + 左上偏蓝 / 右下偏绿两条径向光晕
	//（原版 body.mini-mode #root 的三层 background，见 minibg.go）。
	ui.Column(c).Fill().Children(func() {
		a.miniBackground(c)
	})

	ui.Column(c).Absolute().Fill().Children(func() {
		// 标题栏：只用来拖动窗口（原版 mini 下把内容整体淡出，只留窗口按钮）。
		ui.Row(c).FillWidth().Height(miniTitleBar).Shrink(0).DragWindow()

		// 主体：封面 + 标题 + 选集 + 窗口控制。
		// 整行可拖动窗口（原版 #min-video-info 带 data-tauri-drag-region="deep"）。
		ui.Row(c).FillWidth().Height(miniInfoH).Shrink(0).Padding(6, 12).
			Gap(11).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(miniCover, miniCover).Shrink(0).Radius(10).
				Background(ui.Hex("#ffffff").Alpha(0.58)).
				// 原版 #min-video-info-cover：box-shadow 0 10px 22px rgba(15,23,42,.16)，
				// 封面浮在迷你窗背景之上。
				Shadow(0, 10, 22, 0, shadowInk.Alpha(0.16)).
				Clip().Children(func() {
				// 封面缺失时退回 logo（原版 cover || "/logo.png"）。
				if bmp := a.coverBitmap(); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				} else if Logo != nil {
					ui.Image(c, Logo).Fill().Fit(ui.Cover)
				}
				// 高光（原版 .mini-cover-shine）：左上到右下的白渐变。
				// 必须 Absolute：原版是 position:absolute; inset:0，不参与布局。
				// 否则它作为封面的第二个兄弟参与列布局，把封面挤回自然高度
				// （64 宽的 16:9 封面只剩 ~36 高），下半张就是空的。
				ui.Box(c).Absolute().Fill().PassThrough().LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#ffffff").Alpha(0.28),
					To:    ui.Transparent,
					Angle: 135,
				})
			})

			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Justify(ui.Center).Children(func() {
				// 原版 .mini-title：15px / 800。
				ui.Text(c, a.infoTitle()).FontSize(15).FontWeight(800).TextColor(t.Ink).MaxLines(2)
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					// 播放列表模式圆点玫红 + 外圈光晕（原版
					// .mini-status-dot-playlist 的 box-shadow ring）。
					inList := a.PlayingPlaylist != ""
					dotInk := pick(inList, t.Rose, t.Blue)
					dot := ui.Box(c).Size(7, 7).Shrink(0).Margin(0, 0, 0, 5).
						Radius(RadiusPill).Background(dotInk)
					dot.Shadow(0, 0, 0, 4, dotInk.Alpha(0.12))
					// 原版 .mini-part：12px / 650 / #526174。
					ui.Text(c, a.partTitle()).FontSize(12).FontWeight(650).
						TextColor(ui.Hex("#526174")).SingleLine()
				})
			})

			ui.Column(c).Width(30).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
				a.miniControl(c, "mini-restore", "切换到窗口模式", iconRestore, false, func() {
					if a.Act.SetMini != nil {
						a.Act.SetMini(false)
					}
				})
				a.miniControl(c, "mini-pin", pick(a.Pinned, "取消窗口置顶", "窗口置顶"),
					iconPin, a.Pinned, func() {
						if a.Act.TogglePin != nil {
							a.Act.TogglePin()
						}
					})
			})
		})

		// 播放栏：切曲 / 当前时间 / 进度 / 时长 / 音量。
		volAnchor := a.miniPlayerBar(c)

		// 音量弹层（迷你窗里也要能弹出来，否则音量键点了没反应）。
		if a.ShowVolume {
			// 原版 mini：158×38、圆角 12，挂在音量列左侧 8px、与栏垂直居中
			//（body.mini-mode #player .player-volume-popover）。
			a.volumePopoverAt(c, volAnchor, miniVolumePopoverEdgeGap, 158, 38, 12)
		}
		a.toastViewport(c)
	})
}

// miniPlayerBar 是迷你模式的播放栏（原版 body.mini-mode #player）：
// 高 43、内边距 0 10，列宽 59 / 44 / 1fr / 44 / 34，间距 4。
// 返回音量列的句柄：音量弹层挂在它上面当锚点。
func (a *App) miniPlayerBar(c *ui.Context) ui.Element {
	t := a.Theme
	var volSlot ui.Element
	ui.Column(c).FillWidth().Height(miniPlayerH).Shrink(0).
		// 与主窗同一层框架玻璃 + 同一道向上投影。
		Material(glassLayer(glass.Glass{})).
		Shadow(0, -10, 28, 0, ui.Hex("#0f172a").Alpha(0.05)).
		Children(func() {
			ui.Box(c).FillWidth().Height(1).Shrink(0).Background(t.PlayerBorder)
			ui.Row(c).FillWidth().Grow(1).Padding(0, 10).Gap(4).
				AlignItems(ui.Center).Children(func() {
				// 59px：播放 28 + 3 + 下一集 28。
				ui.Row(c).Width(59).Shrink(0).Gap(3).AlignItems(ui.Center).Children(func() {
					// mini 下按钮一律 28×28 正圆（原版 body.mini-mode .player-button）。
					// 禁用态与文案与主窗同一套（原版 mini 用的是同一个 Player 组件）。
					play := ui.ButtonBase(c.Key("mini-play")).Size(28, 28).Radius(RadiusPill).Center().
						Label(playTip(a.Playing)).Tooltip(playTip(a.Playing))
					disablePlayerButton(play, !a.hasSource())
					// 原版 body.mini-mode #player .player-play-button：浅蓝填充
					// #bae6fd .35 + 蓝描边 + 蓝调内外投影，**与是否播放无关**
					//（mini 规则 (1,2,1) 的特异度盖过 [data-playing] 的 (1,2,0)），
					// 只有图标在 play/pause 间切换；悬停时填充/描边略加深
					//（#player .player-play-button:hover 的 (1,3,0) 盖过 mini 基态）。
					// 底色/描边/墨色走与主窗同一条 140ms ease 过渡（原版 mini 的
					// transition 没有覆盖 #player .player-button 的那条）。
					playFillRaw, playBorderRaw, playInkRaw := miniPlayStyle(play.Hovered())
					playFill := playerFadeColor(play, "mini-play-bg", playFillRaw)
					playBorder := playerFadeColor(play, "mini-play-border", playBorderRaw)
					playInk := playerFadeColor(play, "mini-play-ink", playInkRaw)
					// 按下（:active）时原版把 ::before 底色换成 --liquid-bg-active
					// （白 .16）、box-shadow 换成更深的 `inset 0 1px 3px 墨 .1`
					//（:active 规则特异度 (1,3,0) 盖过 mini 基态与 [data-playing]）。
					if play.Pressed() {
						play.Background(playerPressFill()).
							Border(1, playBorder)
						applyInsetShadows(play, playerPressShadowSpecs())
					} else {
						play.Background(playFill).
							Border(1, playBorder).
							InsetShadow(0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.90)).
							InsetShadow(0, -2, 4, 0, ui.Hex("#0284c7").Alpha(0.08)).
							Shadow(0, 2, 6, 0, ui.Hex("#0284c7").Alpha(0.12))
					}
					playerPressPose(play)
					play.Children(func() {
						ui.Icon(c, pick(a.Playing, iconPause, iconPlay)).Size(16, 16).
							TextColor(playInk).
							Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
					})
					if play.Clicked() && a.Act.TogglePlay != nil {
						a.Act.TogglePlay()
					}

					next := ui.ButtonBase(c.Key("mini-next")).Size(28, 28).Radius(RadiusPill).Center().
						Label(nextTip()).Tooltip(nextTip())
					disablePlayerButton(next, !a.canNavigate())
					// 工具键材质与主窗同一套（mini 只把 32→28、圆角改 50%）。
					playerToolSurface(next, false, next.Hovered(), next.Pressed())
					playerPressPose(next)
					next.Children(func() {
						ui.Icon(c, iconNext).Size(18, 18).
							TextColor(playerFadeColor(next, "mini-next-ink",
								pick(next.Hovered(), ui.Hex("#0284c7"), ui.Hex("#334155")))).
							Opacity(playerDisabledIconOpacityFor(!a.canNavigate()))
					})
					if next.Clicked() && a.Act.Next != nil {
						a.Act.Next()
					}
				})

				// mini 的时间标签：9px / 44 列宽（原版 body.mini-mode
				// #player .player-time 只改字号与 padding，字重/颜色沿用主窗）。
				a.timeTextAt(c, a.Pos, 9, 44)

				// 与主窗同一套 seek 状态机（见 progress）：拖动中停在目标值，
				// 松手发 seek、等引擎追上才恢复跟随——否则滑块弹回旧位置。
				a.miniProgress(c)

				a.timeTextAt(c, a.Dur, 9, 44)

				// 列宽 34（原版 mini grid 最后一列），按钮本体 28×28 正圆。
				volSlot = ui.Box(c).Width(34).Shrink(0).Children(func() {
					vol := ui.ButtonBase(c.Key("mini-volume")).Size(28, 28).Radius(RadiusPill).Center().
						Label("音量").Tooltip("音量")
					// 原版 disabled={!src}。
					disablePlayerButton(vol, !a.hasSource())
					playerToolSurface(vol, false, vol.Hovered(), vol.Pressed())
					playerPressPose(vol)
					vol.Children(func() {
						ui.Icon(c, iconVolume).Size(18, 18).
							TextColor(playerFadeColor(vol, "mini-volume-ink", playerToolInk(false, vol.Hovered()))).
							Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
					})
					if vol.Clicked() {
						a.ShowVolume = !a.ShowVolume
					}
				})
			})
		})
	return volSlot
}

// miniPlayStyle 返回迷你窗播放键的（填充, 描边, 图标色）。取自原版
// body.mini-mode #player .player-play-button 及其 :hover：
//
//	常态  background rgba(186,230,253,.35); border rgba(2,132,199,.20); color #0284c7
//	悬停  background rgba(186,230,253,.50); border rgba(2,132,199,.28); color #0284c7
//
// 注意 mini 的基态规则特异度 (1,2,1) 盖过主窗 [data-playing] 的 (1,2,0)，
// 所以 mini 播放键与是否播放无关（只有 play/pause 图标变）。
func miniPlayStyle(hovered bool) (fill, border, ink ui.Color) {
	fill, border, ink = ui.Hex("#bae6fd").Alpha(0.35), ui.Hex("#0284c7").Alpha(0.20), ui.Hex("#0284c7")
	if hovered {
		fill, border = ui.Hex("#bae6fd").Alpha(0.50), ui.Hex("#0284c7").Alpha(0.28)
	}
	return
}

// miniControl 是迷你模式信息区右侧的窗口控制键：30×30、16px 图标
// （原版 .mini-window-controls 里的 .app-title-bar-btn + size={16}；
// 两颗纵向 gap 4 → 30+4+30 = 64，正好与封面同高）。
func (a *App) miniControl(c *ui.Context, key, label string, ic *ui.SVG, active bool, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(30, 30).Radius(RadiusSmall).Center().Label(label).Tooltip(label)
	// 原版 .mini-window-controls button 沿用 .app-title-bar-btn 的通用玻璃规则：
	// 常态透明 + 图标 72%；悬停/激活铺 --glass-bg-hover（白 .38）+ 图标转
	// --studio-blue；激活（aria-pressed=true）再叠内高光。没有边框、没有常驻投影。
	bg, ink, opacity, highlight := titleBarButtonStyle(t, active, b.Hovered())
	b.Background(bg).Opacity(opacity)
	if highlight {
		b.InsetShadow(0, 1, 1, 0, t.Highlight).
			Shadow(0, 2, 10, 0, shadowInk.Alpha(0.045))
	}
	// 原版 .mini-window-controls button:hover 的左上角径向聚光（按下换居中光）。
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, RadiusSmall)
	}
	pressScale(b)
	b.Children(func() {
		ui.Icon(c, ic).Size(16, 16).TextColor(ink)
	})
	if b.Clicked() {
		fn()
	}
}

// MiniSize 返回迷你模式的窗口尺寸，供装配层调整窗口用。
func MiniSize() (int, int) { return miniWidth, miniHeight }
