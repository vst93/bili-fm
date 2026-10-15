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

	// 信息行（原版 #min-video-info：padding 6 12、gap 11、
	// grid-template-columns 64px minmax(0,1fr) 30px）的横向几何。
	miniInfoPadH  = 12
	miniInfoGap   = 11
	miniControlsW = 30

	// miniTitleWidth 是标题列的可用宽度：信息行宽 − 左右内边距 − 封面 −
	// 两处间距 − 窗口控制列。迷你窗定宽 400，所以这是个常数 —— 原版
	// .mini-title 的盒高由内容（折行数）决定，而自绘标题必须在**布局期**就知道
	// 折行数（折行数决定盒高），只能按这个常数预折。
	miniTitleWidth = miniWidth - 2*miniInfoPadH - miniCover - 2*miniInfoGap - miniControlsW

	// 原版 .mini-title：15px / 800 / line-height 1.25 / 两行截断。
	miniTitleSize       = 15
	miniTitleWeight     = 800
	miniTitleLineHeight = 1.25
)

var (
	miniTitleFont = ui.Font{Size: miniTitleSize, Weight: miniTitleWeight}
	// miniTitlePitch 由 textLineBox 算出（= 1.25 × 15 = 18.75），声明成 var
	// 便于无头测试换桩做反证（同 infoTitlePitch）。
	miniTitlePitch = func() float32 { h, _ := textLineBox(miniTitleSize, miniTitleLineHeight); return h }()
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
		// 标题栏：只用来拖动窗口。原版 mini 下 `.app-title-bar > *` 整条淡到
		// opacity 0（只剩 Windows/Linux 由前端自绘的窗口按钮）——但标题栏那两层
		// hover-to-reveal 贴边**照旧**：`::before` 顶沿投影 .52 → hover .78、
		// `::after` 白玻璃洗层 0 → hover 1（都是 `transition: opacity .24s ease`）。
		// mygo 的迷你窗标题栏没有自绘子元素（窗口按钮是 OS 绘制的原生控件，
		// 应用没法淡入它们），所以这两层贴边就是迷你窗标题栏的 hover 反馈，
		// 与主窗同一手法（titleBarLayer + titleBarReveal）。
		bar := ui.Row(c).FillWidth().Height(miniTitleBar).Shrink(0).DragWindow()
		_, _, edge, wash := titleBarReveal(bar.Hovered() || bar.FocusWithin())
		bar.Children(func() {
			// 两层**无条件**建出来（静息时白玻璃那层 opacity 0、整棵子树不绘制）：
			// mygo 的元素 id 由「父 id + 已在建的子元素个数」决定，少建一个兄弟会让
			// 它后面所有兄弟换 id（悬停/动画状态会跟着丢）—— 同主窗 titleBar 的注释。
			// 两层整棵子树都 PassThrough：它们盖在信息行之上，不能挡拖动与命中。
			titleBarLayer(c, "mini-tb-edge", titleBarEdgeStops, miniTitleBar*1.7, edge)
			titleBarLayer(c, "mini-tb-wash", titleBarWashStops, miniTitleBar*1.4, wash)
		})

		// 主体：封面 + 标题 + 选集 + 窗口控制。
		// 整行可拖动窗口（原版 #min-video-info 带 data-tauri-drag-region="deep"）。
		ui.Row(c).FillWidth().Height(miniInfoH).Shrink(0).Padding(6, miniInfoPadH).
			Gap(miniInfoGap).AlignItems(ui.Center).DragWindow().Children(func() {
			ui.Box(c).Size(miniCover, miniCover).Shrink(0).Radius(10).
				Background(ui.Hex("#ffffff").Alpha(0.58)).
				// 原版 #min-video-info-cover：box-shadow 0 10px 22px rgba(15,23,42,.16)，
				// 封面浮在迷你窗背景之上。
				Shadow(0, 10, 22, 0, shadowInk.Alpha(0.16)).
				Clip().Children(func() {
				// 封面缺失时退回 logo（原版 cover || "/logo.png"）——用白底那张。
				if bmp := a.coverBitmap(); bmp != nil {
					ui.Image(c, bmp).Fill().Fit(ui.Cover)
				} else if lg := coverLogo(); lg != nil {
					ui.Image(c, lg).Fill().Fit(ui.Cover)
				}
				// 高光（原版 .mini-cover-shine）：左上到右下的白渐变。
				// 必须 Absolute：原版是 position:absolute; inset:0，不参与布局。
				// 否则它作为封面的第二个兄弟参与列布局，把封面挤回自然高度
				// （64 宽的 16:9 封面只剩 ~36 高），下半张就是空的。
				ui.Box(c).Absolute().Fill().PassThrough().LinearGradient(miniCoverShine())
			})

			ui.Column(c).Grow(1).MinWidth(0).Gap(7).Justify(ui.Center).Children(func() {
				// 原版 .mini-title：15px / 800 / line-height 1.25 / 两行截断。
				// 行距 1.25 × 15 = 18.75 比框架自然行盒（15px 实测 23）矮，框架
				// 会把行距顶到 23 —— 第二行比原版低 4.25px，单行盒高也多 4.25，
				// 把信息列（Justify Center）整体推偏。与宽窗标题同法自绘逐行落点
				// （断行仍交给框架，见 multilinetext.go）：折行数决定盒高、又只能
				// 在布局期知道，而标题列宽是常数（miniTitleWidth），按它预折。
				title := a.infoTitle()
				lines := wrapTextLines(title, miniTitleFont, miniTitleWidth, 2)
				if len(lines) == 0 {
					lines = []string{title}
				}
				tb := ui.Box(c).Width(miniTitleWidth).
					Height(float32(len(lines)) * miniTitlePitch).Label(title)
				tb.Draw(func(p *ui.Painter, r ui.Rect) {
					paintTextLines(p, r, lines, miniTitleFont, miniTitlePitch, t.Ink)
				})
				ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
					// 播放列表模式圆点玫红 + 外圈光晕（原版
					// .mini-status-dot-playlist 的 box-shadow ring）。
					inList := a.PlayingPlaylist != ""
					dotInk, dotRing := miniStatusDot(t, inList)
					dot := ui.Box(c).Size(7, 7).Shrink(0).Margin(0, 0, 0, 5).
						Radius(RadiusPill).Background(dotInk)
					dot.Shadow(0, 0, 0, 4, dotRing.Alpha(0.12))
					// 原版 .mini-part：12px / 650 / #526174 / line-height 1.2。
					// 框架自然行盒（12px ≈ 18）下不去，这一行会高约 3.6px、把
					// 信息列（Justify Center）整体推偏；用显式盒高 + 字形上移
					// 复现（见 linebox.go）。
					mh, msh := textLineBox(12, 1.2)
					ui.Text(c, a.partTitle()).FontSize(12).FontWeight(650).
						TextColor(ui.Hex("#526174")).SingleLine().Height(mh).Translate(0, msh)
				})
			})

			ui.Column(c).Width(miniControlsW).Shrink(0).Gap(4).AlignItems(ui.Center).
				// 原版 .mini-window-controls 是 `role="group" aria-label="窗口控制"`。
				Role(ui.RoleGroup).Label("窗口控制").Children(func() {
				a.miniControl(c, "mini-restore", "切换到窗口模式", iconRestore, false, false,
					a.WindowControlPending, func() {
						if a.Act.SetMini != nil {
							a.Act.SetMini(false)
						}
					})
				a.miniControl(c, "mini-pin", pick(a.Pinned, "取消窗口置顶", "窗口置顶"),
					miniPinIcon(a.Pinned), a.Pinned, true, a.WindowControlPending, func() {
						if a.Act.TogglePin != nil {
							a.Act.TogglePin()
						}
					})
			})
		})

		// 播放栏上沿那 10px 的淡出（原版 `body.mini-mode #player::before`）：
		// 主窗早就用框架 ScrollEdge 画了这一层，mini 一直漏着 —— 栏顶于是是
		// 一条硬切线（1px 亮描边直接切在信息区上）。盒子与主窗同一手法：
		// 「栏高 + 10」的 Absolute 层、Bottom: true（效果自栏向上生），
		// 必须画在播放栏**之前**（在下层），并且 PassThrough 不挡指针。
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(miniPlayerH + 10).
			PassThrough().Material(glass.ScrollEdge{Background: miniPlayerEdgeFade(t), Bottom: true})

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

// miniPlayerEdgeFade 是迷你播放栏上沿那 10px 淡出的颜色，对应原版
// `body.mini-mode #player::before`：
//
//	background: linear-gradient(to bottom, transparent,
//	    var(--player-edge-fade, rgba(244,248,252,.52)));
//
// 浅色时段**没有**定义 --player-edge-fade，所以 mini 落到规则里的兜底
// rgba(244,248,252,.52)——比主窗那条 rgba(186,230,253,.24) 更白更实（mini
// 的背景是固定的浅色渐变）。五个烟熏玻璃时段把 --player-edge-fade 覆写成
// rgba(71,85,105,.11)，mini 与主窗就都吃这个值。声明成 var 便于测试换桩。
var miniPlayerEdgeFade = func(t Theme) ui.Color {
	if t.Dark {
		return t.PlayerEdgeFade
	}
	return ui.Hex("#f4f8fc").Alpha(0.52)
}

// miniPlayerBar 是迷你模式的播放栏（原版 body.mini-mode #player）：
// 高 43、内边距 0 10，列宽 59 / 44 / 1fr / 44 / 34，间距 4。
// 返回音量列的句柄：音量弹层挂在它上面当锚点。
func (a *App) miniPlayerBar(c *ui.Context) ui.Element {
	t := a.Theme
	var volSlot ui.Element
	applyShadows(
		applyInsetShadows(
			playerBarMaterial(t,
				ui.Column(c).FillWidth().Height(miniPlayerH).Shrink(0)),
			// 原版 body.mini-mode --player-shadow 的前两层 inset（浅色是 mini
			// 专属的白 .9 顶 + 墨 .04 底，深色吃 --player-shadow 那条），
			// 见 playerBarShadowInsets。
			playerBarShadowInsets(t, true)),
		// 向上投影与主窗同一套令牌，但 mini 浅色的兜底更紧（22px，
		// 见 playerBarShadowSpec）。
		[]shadowSpec{playerBarShadowSpec(t, true)}).
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
					playFillRaw, playBorderRaw, playInkRaw := miniPlayStyle(t, a.Playing, play.Hovered())
					playFill := playerFadeColor(c, play, "mini-play-bg", playFillRaw)
					playBorder := playerFadeColor(c, play, "mini-play-border", playBorderRaw)
					playInk := playerFadeColor(c, play, "mini-play-ink", playInkRaw)
					// 基态三色 + 按下/悬停/深色的 box-shadow（原版 mini 专属规则只改
					// 基态，悬停与按下仍由主窗那几条 (1,3,x) 规则胜出，见 miniPlayShadow）。
					play.Background(playFill).Border(1, playBorder)
					miniPlayShadow(t, play, a.Playing, play.Hovered(), play.Pressed())
					playerPressPose(play)
					// mini 也是 .player-button：补聚光描边（播放键 1.0）+ 图标压印。
					// ::before 光泽：浅色走 mini 专属渐变（悬停转主窗 hover 渐变），
					// 深色统一 --player-sheen（见 playerSheenFor）。
					playerRim(t, play, RadiusPill, false, sheenKindMiniPlay, a.Playing, play.Hovered())
					play.Children(func() {
						ic := pick(a.Playing, iconPause, iconPlayOne)
						icon := ui.Icon(c, ic).Size(16, 16).
							TextColor(playInk).
							Opacity(playerDisabledIconOpacityFor(!a.hasSource())).
							Translate(0.5, 0)
						playerIconShadowFn(icon, ic, 0, 1, playerPlayIconShadowInk)
					})
					if play.Clicked() && a.Act.TogglePlay != nil {
						a.Act.TogglePlay()
					}

					next := ui.ButtonBase(c.Key("mini-next")).Size(28, 28).Radius(RadiusPill).Center().
						Label(nextTip()).Tooltip(nextTip())
					disablePlayerButton(next, !a.canNavigate())
					// 工具键材质与主窗同一套（mini 只把 32→28、圆角改 50%）。
					playerToolSurface(c, t, toolNext, next, false, next.Hovered(), next.Pressed())
					playerPressPose(next)
					playerRim(t, next, RadiusPill, true, sheenKindStatic, false, false)
					next.Children(func() {
						icon := ui.Icon(c, iconNext).Size(18, 18).
							TextColor(playerFadeColor(c, next, "mini-next-ink",
								playerButtonInk(t, next.Hovered()))).
							Opacity(playerDisabledIconOpacityFor(!a.canNavigate()))
						playerIconShadowFn(icon, iconNext, 0, 1, playerIconShadowInk)
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
					playerToolSurface(c, t, toolVolume, vol, false, vol.Hovered(), vol.Pressed())
					playerPressPose(vol)
					playerRim(t, vol, RadiusPill, true, sheenKindStatic, false, false)
					vol.Children(func() {
						// 原版 mini 与主窗共用同一个 Player，所以音量键图标同样随音量切换
						// （`volume > 0 ? VolumeNotice : VolumeMute`）—— 音量 0（静音）时
						// mini 的那颗键也该是静音图标。之前这里恒画喇叭。
						ic := volumeIconFor(a.Muted || a.Volume <= 0)
						icon := ui.Icon(c, ic).Size(18, 18).
							TextColor(playerFadeColor(c, vol, "mini-volume-ink", playerToolInk(t, toolVolume, false, vol.Hovered()))).
							Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
						playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
					})
					if vol.Clicked() {
						a.ShowVolume = !a.ShowVolume
					}
				})
			})
		})
	return volSlot
}

// miniCoverShine 是封面高光层（原版 .mini-cover-shine）的渐变：
//
//	linear-gradient(135deg, rgba(255,255,255,0.28), transparent 42%)
//
// 第二个 stop 停在 **42%**（不是整条对角线）。End 缺省 1 会把高光一直拉到右下角
// ——整张封面蒙上一层白，左上那道高光的落点也偏长。声明成 var 便于无头测试换桩
// 做反证（同 miniPlayerEdgeFade）。
var miniCoverShine = func() ui.LinearGradient {
	return ui.LinearGradient{
		From:  ui.Hex("#ffffff").Alpha(0.28),
		To:    ui.Transparent,
		Angle: 135,
		End:   0.42,
	}
}

// miniStatusDot 返回迷你窗分集行状态点的（点色, 外圈光晕色），对齐原版
// body.mini-mode 的 .mini-status-dot / .mini-status-dot-playlist：
//
//	非播放列表  background --studio-blue #0284c7  ring rgba(14,165,233,.12)
//	播放列表    background --studio-rose #e11d48  ring rgba(225,29,72,.12)
//
// 两态的光晕色**不同源**：蓝点的 ring 是 sky-500（#0ea5e9）而不是点色
// --studio-blue —— 此前直接拿点色乘 .12 画蓝点光晕，比原版偏深一档。
func miniStatusDot(t Theme, inList bool) (dot, ring ui.Color) {
	if inList {
		return t.Rose, t.Rose
	}
	return t.Blue, ui.Hex("#0ea5e9")
}

// miniPlayStyle 返回迷你窗播放键的（填充, 描边, 图标色）。取自原版
// body.mini-mode #player .player-play-button 及其悬停态：
//
//	基态·暂停/播放中  background rgba(186,230,253,.35); border rgba(2,132,199,.20); color #0284c7
//	悬停·暂停         background rgba(186,230,253,.50); border rgba(2,132,199,.28); color #0284c7
//	悬停·播放中       background rgba(125,211,252,.22); border rgba(2,132,199,.40); color #0369a1
//
// 悬停两条来自 `#player .player-play-button:hover:not(:disabled)` (1,3,0) 与
// `#player .player-play-button[data-playing]:hover:not(:disabled)` (1,4,0)，后者
// 在播放中胜出（此前 mini 悬停恒按「暂停」那套，播放中悬停的颜色偏了）。
// 深色时段的主窗深色基态 (1,2,1) 源序在 mini 基态之后，盖过 mini 基态 —— 深色
// 未悬停换成主窗深色面；悬停规则仍 (1,3,x) 胜出，两时段同值。
func miniPlayStyle(t Theme, playing, hovered bool) (fill, border, ink ui.Color) {
	if hovered {
		if playing {
			return ui.Hex("#7dd3fc").Alpha(0.22), ui.Hex("#0284c7").Alpha(0.40), ui.Hex("#0369a1")
		}
		return ui.Hex("#bae6fd").Alpha(0.50), ui.Hex("#0284c7").Alpha(0.28), ui.Hex("#0284c7")
	}
	if t.Dark {
		return ui.Hex("#e0f2fe").Alpha(playerPlayDarkFillA),
			ui.Hex("#e0f2fe").Alpha(playerPlayDarkBorderA), ui.Hex("#0369a1")
	}
	return ui.Hex("#bae6fd").Alpha(0.35), ui.Hex("#0284c7").Alpha(0.20), ui.Hex("#0284c7")
}

// miniPlayShadowSpecs 返回迷你窗播放键这一帧该铺的（内影, 外影）。拆成纯函数
// 便于无头测试逐值断言五支取值（同 playerPlaySurface 的分支口径）。
func miniPlayShadowSpecs(t Theme, playing, hovered, pressed bool) (inset, outer []shadowSpec) {
	switch {
	case pressed:
		// 原版 `:active:not(:disabled)`（1,3,0）：box-shadow 换
		// `inset 0 1px 3px 墨 .1`（与主窗播放键同一支）。
		return playerPressShadowSpecs(), nil
	case hovered && playing:
		// [data-playing]:hover（1,4,0）。
		return []shadowSpec{
				{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.88)},
				{0, -2, 5, 0, ui.Hex("#0284c7").Alpha(0.12)},
			}, []shadowSpec{
				{0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.06)},
				{0, 5, 14, 0, ui.Hex("#0284c7").Alpha(0.14)},
			}
	case hovered:
		// :hover（1,3,0）。
		return []shadowSpec{{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}},
			[]shadowSpec{{0, 6, 16, 0, ui.Hex("#0284c7").Alpha(0.18)}}
	case t.Dark:
		// 深色基态（1,2,1，源序在 mini 之后）。
		return []shadowSpec{
				{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(playerPlayDarkInsetA)},
				{0, -1, 1, 0, ui.Hex("#0284c7").Alpha(0.08)},
			}, []shadowSpec{{0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.10)}}
	default:
		// mini 浅色基态（原版 body.mini-mode #player .player-play-button）。
		return []shadowSpec{
				{0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.90)},
				{0, -2, 4, 0, ui.Hex("#0284c7").Alpha(0.08)},
			}, []shadowSpec{{0, 2, 6, 0, ui.Hex("#0284c7").Alpha(0.12)}}
	}
}

// miniPlayShadow 把 miniPlayShadowSpecs 的结果铺到按钮上。按下时先把底换成
// --liquid-bg-active（原版 `:active::before` 那条），与主窗 playerPlaySurface 同口径。
func miniPlayShadow(t Theme, b ui.Element, playing, hovered, pressed bool) ui.Element {
	if pressed {
		b = b.Background(playerPressFillFor(t, false))
	}
	inset, outer := miniPlayShadowSpecs(t, playing, hovered, pressed)
	return applyShadows(applyInsetShadows(b, inset), outer)
}

// miniPinIcon 返回迷你窗「窗口置顶」键的图标。原版是
// `<Pushpin theme={isPinned ? "filled" : "outline"}>`：置顶时换成 icon-park 的
// **实心**变体（图钉头整块填充 + 实心针杆），未置顶才是描边的图钉。此前原生版
// 两态共用同一枚描边图标，置顶只靠 aria-pressed 的蓝图标 / 白 .38 底区分；
// 形状这层差异一直没画。声明成 var 便于无头测试换桩做反证（同 navIconPose）。
var miniPinIcon = func(pinned bool) *ui.SVG {
	if pinned {
		return iconPinFilled
	}
	return iconPin
}

// miniControl 是迷你模式信息区右侧的窗口控制键：30×30、16px 图标
// （原版 .mini-window-controls 里的 .app-title-bar-btn + size={16}；
// 两颗纵向 gap 4 → 30+4+30 = 64，正好与封面同高）。
//
// toggle 为真时这颗键是保持型开关：原版置顶键有 `aria-pressed={isPinned}`
// （切回窗口模式的键没有），mygo 对应 RoleToggleButton + Checked(active)。
//
// pending 为真（一次窗口操作在途）时这颗键按原版 `button:disabled` 渲染且
// 完全不响应点击：opacity .34、光标默认、没有按压缩放 / 悬停高光 / 聚光 /
// 焦点环，但仍照旧读回 aria-pressed 的按压态。
func (a *App) miniControl(c *ui.Context, key, label string, ic *ui.SVG, active, toggle, pending bool, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key(key)).Size(30, 30).Radius(RadiusSmall).Center().Label(label).Tooltip(label)
	// 原版两颗键都带 `disabled={isWindowControlPending}`。用框架的 Disabled 而不是
	// 只在回调里挡：Disabled 才让 Hovered/Pressed/FocusVisible/Clicked 全部返回假
	// ——禁用期间没有按压缩放（`transform: none`）、悬停高光、聚光与焦点环。
	b.Disabled(pending)
	// 原版置顶键是 `aria-pressed={isPinned}` 的 button（无障碍上是 toggle button），
	// 把 isPinned 作为按压态交给读屏；「切换到窗口模式」是动作键，不接。
	if toggle {
		b.Role(ui.RoleToggleButton).Checked(active)
	}
	// 原版 .mini-window-controls button 沿用 .app-title-bar-btn 的通用玻璃规则：
	// 常态透明 + 图标 72%；悬停/激活铺 --glass-bg-hover（白 .38）+ 图标转
	// --studio-blue；激活（aria-pressed=true）再叠内高光。没有边框、没有常驻投影。
	bg, ink, opacity, _ := titleBarButtonStyle(t, active, b.Hovered(), b.Pressed(), b.IsDisabled())
	// 原版 `.mini-window-controls button:disabled { opacity: 0.34 }` 的 .34 是
	// **绝对值**（不是「在常态 .72 上再减半」）：框架对 disabled 节点会再乘 0.5
	// （见 disabled.go），disabledDim 抵消那一次减半。
	if b.IsDisabled() {
		opacity = disabledDim(opacity)
	}
	// 原版 `:is(.app-title-bar, .mini-window-controls) .app-title-bar-btn:focus-visible`：
	// opacity 1、图标转 --studio-blue，高光换成 `inset 0 1px 1px
	// --glass-highlight` ＋ 2px 蓝环。另：置顶那条 `[aria-pressed=true]:not(…)`
	// 带 `:not(:focus-visible)` —— 聚焦且不悬停时它整条失效，底色回透明。
	focus := b.FocusVisible()
	if focus {
		ink, opacity = t.Blue, 1
		if !b.Hovered() {
			bg = ui.Transparent
		}
	}
	// `.mini-window-controls button` 的 color/background 0.18s、opacity 0.16s；
	// 桌面要求减少动态效果时原版对它写了 `transition: none`，色值当帧到位。
	bg, ink, opacity = titleBarBtnFade(b, bg, ink, opacity, reduceMotion(c))
	b.Background(bg).Opacity(opacity)
	if focus {
		b.InsetShadow(0, 1, 1, 0, t.Highlight)
	}
	// ::after 那层三条阴影：悬停淡入、移开淡出（0.18s），按下归零。每帧都要调，
	// 否则移开鼠标那一帧不透明度会直接跳到 0（没有淡出）。置顶且不在悬停/按下/
	// 聚焦时的单层内高光也在这里铺（当帧到位）。
	titleBarBtnHighlight(b, active, b.Hovered())
	// 原版 .mini-window-controls button:hover 的左上角径向聚光（按下换居中光）。
	if l, ok := titleBarBtnLight(b.Pressed(), b.Hovered()); ok {
		l.draw(b, RadiusSmall)
	}
	pressScale(b)
	b.Children(func() {
		ui.Icon(c, ic).Size(16, 16).TextColor(ink)
	})
	focusOutline(b, RadiusSmall, focusGlow)
	if b.Clicked() {
		fn()
	}
}

// MiniSize 返回迷你模式的窗口尺寸，供装配层调整窗口用。
func MiniSize() (int, int) { return miniWidth, miniHeight }
