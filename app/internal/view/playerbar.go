package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"

	"github.com/egoist/mygo/ui"
)

// 播放栏（原版 #player）：固定底部、高 56、单行。
//
//	#player .player-controls {
//	    height: 56px; padding: 0 18px;
//	    grid-template-columns: 80px 56px minmax(0,1fr) 56px 34px 40px 34px 40px;
//	    gap: 8px; border-top: 1px solid var(--player-border);
//	    background-color: var(--player-surface);
//	}
//
// 列依次是：传输键（播放 42 + 下一集 32）/ 当前时间 / 进度 / 总时长 /
// 音量 / 倍速 / 均衡 / 跳过赞助。原版没有封面和标题 —— 那些在主区。
func (a *App) playerBar(c *ui.Context) {
	t := a.Theme
	// 后四列里音量 / 倍速两列的句柄：弹层要挂在它们上面（见 playerBar 末尾）。
	var volumeSlot, speedSlot ui.Element
	// 栏顶的滚动边缘效果（原版 #player::before 那条 10px 渐变就是在手搓这个）：
	// 内容滚到栏下时淡出/结霜，栏体上沿才不会是一条硬切线。
	// 框架的 ScrollEdge 就是 macOS 26+ 的这个效果；软边缘淡出、硬边缘结霜。
	// 高度按框架的约定 = 栏高 + 10（软边缘多出的那 10 DIP）。
	// Background 必须显式给：主题的 Background 被我们设成了透明。
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(66).Shrink(0).
		PassThrough(). // 淡出色用 --player-edge-fade（浅蓝 24%），不是 --player-surface ——
		// 后者是个不透明的蓝灰，淡出来是一条明显的灰带。
		// 淡出色用 --player-edge-fade（浅蓝 24%），不是 --player-surface ——
		// 后者是不透明的蓝灰，淡出来是一条明显的灰带。
		// Bottom: true = 「栏在元素的底部、效果好从栏向上生」；
		// 元素是「栏 56 + 上方 10DIP」，方向搞反会变成向下淡出（一条硬边）。
		Material(glass.ScrollEdge{Background: t.PlayerEdgeFade, Bottom: true})
	// 播放栏：原版这里是 backdrop-filter: blur(20px) saturate(1.4) 的一层玻璃。
	// 用框架的 Liquid Glass 而不是自铺半透明白 —— 玻璃自带边缘折射与 1px 描光
	// （原版那套 inset 亮线/暗线是在手搓同一件事）。
	ui.Column(c).FillWidth().Height(56).Shrink(0).
		Material(glassLayer(glass.Glass{})).
		// 原版 --player-shadow 的第三层：向**上**投到内容区的一道淡影
		//（0 -10px 28px rgba(15,23,42,.05)，浅色/深色时段都是这个值）。
		// 前两层内影由玻璃自己的描光承担。
		Shadow(0, -10, 28, 0, ui.Hex("#0f172a").Alpha(0.05)).
		Children(func() {
			// 顶边 1px（原版 border-top）用 Absolute 画，不占布局——
			// 内容区保持完整 56px。
			ui.Box(c).FillWidth().Height(1).Absolute().Top(0).Background(t.PlayerBorder)
			ui.Row(c).FillWidth().Grow(1).Padding(0, 18).Gap(8).
				AlignItems(ui.Center).Children(func() {
				a.transport(c)
				// 拖动中/seek 未落地时，左侧时间显示预览位置
				//（原版 updateSeekPreviewUi 会一起更新时间标签）。
				a.timeText(c, a.shownPos())
				a.progress(c)
				a.timeText(c, a.Dur)
				// 后四列：列宽 34/40/34/40，按钮 32×32（原版 grid 列）。
				// 音量 / 倍速这两列记下句柄：弹层挂在这两列上（原版
				// .player-volume / .player-speed 就是弹层的定位上下文）。
				volumeSlot = playerSlot(c, 34, func() { a.volumeButton(c) })
				speedSlot = playerSlot(c, 40, func() { a.speedButton(c) })
				playerSlot(c, 34, func() { a.eqButton(c) })
				playerSlot(c, 40, func() { a.sponsorButton(c) })
			})
		})

	// 倍速 / 音量弹层：从播放栏上方向上弹，锚点是各自的列容器（见
	// popoverEdgeGap 与 speedPopover / volumePopover 的注释）。
	if a.ShowSpeed {
		a.speedPopover(c, speedSlot)
	}
	if a.ShowVolume {
		a.volumePopover(c, volumeSlot)
	}
}

// buttonSurface 给按钮上「半透明填充」材质：半透明底 + 1px 玻璃边 +
// inset 顶部高光 + 一点点外投影，状态差异只体现在底色（激活 / 悬停用各自的
// Tint，常态用 --liquid-bg）。
//
// **这不是 Backdrop 真玻璃**：原版这些按钮的 CSS（`#player .player-button`、
// HeroUI Tabs、`.up-follow-btn` …）只有 `background` / `border` / `box-shadow`，
// 没有 `backdrop-filter`——按钮浮在播放栏（那层才是有 blur 的真玻璃）之上。
// 之前用 `glass.Glass{}`（Backdrop 后处理）给每个按钮多算一次 backdrop，
// macOS 上每次按钮一个 render pass + 重读目标，一排按钮就是十几次——
// 与卡片同一个坑（见 renderCard 的注释），也是抽屉每帧 backdrop 数目
// 居高不下的另一半。填充分法与原版一致，观感不变。
func buttonSurface(b ui.Element, active, hovered bool, activeTint, hoverTint ui.Color) ui.Element {
	fill := ui.Hex("#ffffff").Alpha(0.22) // --liquid-bg
	switch {
	case active && activeTint.A > 0:
		fill = activeTint
	case hovered && hoverTint.A > 0:
		fill = hoverTint
	}
	return b.Background(fill).
		Border(1, ui.Hex("#ffffff").Alpha(0.55)).
		InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.55)).
		Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(0.045))
}

// playerToolSurface 是播放栏工具键（下一集/音量/倍速/均衡/恰饭）的材质，
// 对齐原版那条 tool 专用规则（`#player .player-next-button,
// #player .player-volume-button, #player .player-speed-button,
// #player .player-eq-button`）：这些键的玻璃边比通用 .player-button 更收敛
//（`rgba(148,163,184,.2)` 而不是 `var(--glass-border)` 白 .55），内高光也更淡
//（白 .44），否则一排工具键会亮成一圈白描边。三态颜色由 playerToolFill /
// playerToolBorder / playerToolInset 给出（纯函数，便于测试钉住）。
func playerToolSurface(b ui.Element, active, hovered bool) ui.Element {
	b = b.Background(playerToolFill(active, hovered)).
		Border(1, playerToolBorder(active, hovered)).
		InsetShadow(0, 1, 1, 0, playerToolInset(hovered))
	if active {
		return b.InsetShadow(0, 0, 0, 1, ui.Hex("#bae6fd").Alpha(0.70)).
			Shadow(0, 2, 8, 0, ui.Hex("#0284c7").Alpha(0.14))
	}
	if hovered {
		return b.Shadow(0, 3, 9, 0, ui.Hex("#0f172a").Alpha(0.06))
	}
	return b.Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(0.045))
}

// playerToolFill 是工具键三态的底色：常态 --liquid-bg（白 .22）、悬停白 .34、
// 激活 rgba(186,230,253,.30)（原版 ::before 的 background-color）。
func playerToolFill(active, hovered bool) ui.Color {
	switch {
	case active:
		return ui.Hex("#bae6fd").Alpha(0.30)
	case hovered:
		return ui.Hex("#ffffff").Alpha(0.34)
	}
	return ui.Hex("#ffffff").Alpha(0.22)
}

// playerToolBorder 是工具键三态的描边：常态 slate .2、悬停 slate .32、
// 激活 rgba(2,132,199,.28)。
func playerToolBorder(active, hovered bool) ui.Color {
	switch {
	case active:
		return ui.Hex("#0284c7").Alpha(0.28)
	case hovered:
		return ui.Hex("#94a3b8").Alpha(0.32)
	}
	return ui.Hex("#94a3b8").Alpha(0.20)
}

// playerToolInset 是工具键的内高光（原版 tool 规则的白 .44，悬停抬到 .56）。
func playerToolInset(hovered bool) ui.Color {
	if hovered {
		return ui.Hex("#ffffff").Alpha(0.56)
	}
	return ui.Hex("#ffffff").Alpha(0.44)
}

// playerToolInk 是工具键图标 / 文字的颜色：原版常态 #526174（下一集例外，
// 沿用 .player-button 的 #334155，因为它没被那条 tool 规则覆盖 color），
// 悬停与激活都是 --studio-blue #0284c7，激活且悬停时加深到 #0369a1。
func playerToolInk(active, hovered bool) ui.Color {
	switch {
	case active && hovered:
		return ui.Hex("#0369a1")
	case active || hovered:
		return ui.Hex("#0284c7")
	}
	return ui.Hex("#526174")
}

// playerPlaySurface 是播放/暂停键的材质（原版 .player-play-button[data-playing]
// 与 :not([data-playing]) 两条 + 各自的 hover）。键是 42 正圆，播放中走蓝调、
// 暂停走中性灰蓝；三态颜色由 playerPlayFill / playerPlayBorder / playerPlayInk 给。
func playerPlaySurface(b ui.Element, playing, hovered bool) ui.Element {
	b = b.Background(playerPlayFill(playing, hovered)).
		Border(1, playerPlayBorder(playing, hovered))
	switch {
	case playing && hovered:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.88)).
			InsetShadow(0, -2, 5, 0, ui.Hex("#0284c7").Alpha(0.12)).
			Shadow(0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.06)).
			Shadow(0, 5, 14, 0, ui.Hex("#0284c7").Alpha(0.14))
	case playing:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.82)).
			InsetShadow(0, -2, 4, 0, ui.Hex("#0284c7").Alpha(0.10)).
			Shadow(0, 4, 12, 0, ui.Hex("#0284c7").Alpha(0.10))
	case hovered:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)).
			Shadow(0, 6, 16, 0, ui.Hex("#0284c7").Alpha(0.18))
	default:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.76)).
			Shadow(0, 3, 10, 0, ui.Hex("#0f172a").Alpha(0.06))
	}
}

// playerPlayFill 是播放键三态的底色（原版 ::before 的 background-color）。
func playerPlayFill(playing, hovered bool) ui.Color {
	switch {
	case playing && hovered:
		return ui.Hex("#7dd3fc").Alpha(0.22)
	case playing:
		return ui.Hex("#7dd3fc").Alpha(0.16)
	case hovered:
		return ui.Hex("#bae6fd").Alpha(0.50)
	}
	return ui.Hex("#ffffff").Alpha(0.22)
}

// playerPlayBorder 是播放键三态的描边。
func playerPlayBorder(playing, hovered bool) ui.Color {
	switch {
	case playing && hovered:
		return ui.Hex("#0284c7").Alpha(0.40)
	case playing:
		return ui.Hex("#0284c7").Alpha(0.30)
	case hovered:
		return ui.Hex("#0284c7").Alpha(0.28)
	}
	return ui.Hex("#94a3b8").Alpha(0.22)
}

// playerPlayInk 是播放键图标的颜色（原版播放中 #0369a1、暂停 #475569、
// 暂停悬停转 #0284c7）。
func playerPlayInk(playing, hovered bool) ui.Color {
	switch {
	case playing:
		return ui.Hex("#0369a1")
	case hovered:
		return ui.Hex("#0284c7")
	}
	return ui.Hex("#475569")
}

// glassOn 给「常态无框」的小图标按钮上材质：只有激活/悬停才铺半透明面，
// 常态保持透明 —— 原版 .nav-icon-btn 常态就是透明 + 一点径向高光，
// 每颗都铺一块面会显得"每个图标都套了个盒子"。激活/悬停同样是填充而非
// Backdrop 真玻璃（原版没有 backdrop-filter，且每次 backdrop 都是一次
// render pass）。
func glassOn(b ui.Element, active, hovered bool, activeTint, hoverTint ui.Color) ui.Element {
	switch {
	case active && activeTint.A > 0:
		return buttonSurface(b, true, hovered, activeTint, hoverTint)
	case hovered && hoverTint.A > 0:
		return buttonSurface(b, false, true, activeTint, hoverTint)
	}
	return b.Background(ui.Transparent)
}

// disablePlayerButton 把播放栏按钮设成原版 `.player-button:disabled` 的样子：
// 禁用 + opacity .42（mygo 的禁用态自己乘 .5，这里再乘 .84）。
func disablePlayerButton(b ui.Element, off bool) ui.Element {
	b.Disabled(off)
	if off {
		b.Opacity(0.84)
	}
	return b
}

// playerDisabledIconOpacityFor 是 `#player .player-button:disabled svg` 的**额外**
// 压暗（原版 .34）。按钮自身已由 disablePlayerButton 压到 .42，图标再叠一层
// 即 .42×.34≈.14——没有媒体时整排控件应该几乎看不见（倍速键是文字不是 svg，
// 不吃这层，保持 .42）。
var playerDisabledIconOpacityFor = func(off bool) float32 {
	if off {
		return 0.34
	}
	return 1
}

// 播放栏图标尺寸（原版 player.tsx 的 icon-park size 属性）：播放/暂停 24、
// 下一集 18、音量 20、均衡 17、恰饭 17。集中一处便于测试钉住。
const (
	playIconSize   = 24
	nextIconSize   = 18
	volumeIconSize = 20
	eqIconSize     = 17
)

// transport 是第 1 列（80px）：播放/暂停（42） + 下一集（32）。
func (a *App) transport(c *ui.Context) {
	src := a.hasSource()
	ui.Row(c).Width(80).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
		// 原版 .player-play-button：42×42 正圆，播放中 #0369a1、暂停态 #475569，
		// 图标 size 24（见 playerPlaySurface 的三态材质）。
		play := ui.ButtonBase(c.Key("play")).Size(42, 42).Radius(RadiusPill).Center().
			Label(playTip(a.Playing)).Tooltip(playTip(a.Playing))
		// 原版 disabled={!src}：还没有曲目时整个播放栏是禁用态。
		disablePlayerButton(play, !src)
		// 原版 .player-play-button::before 只有 background-color/background-image，
		// **没有** backdrop-filter——按键浮在已有 blur 的播放栏之上。之前误用了
		// Interactive 真玻璃，于是每个播放键每帧多一个 backdrop。这里改回填充；
		// 按下手感交给 pressPose（原版 :active 就是 scale(.93)）。
		// 三态（播放/悬停/常态）见 playerPlaySurface。
		playerPlaySurface(play, a.Playing, play.Hovered())
		pressPose(play)
		play.Children(func() {
			// 原版播放键图标 size 24（暂停态 #0369a1、暂停图 #475569，悬停转 #0284c7）。
			// 禁用时 svg 再乘 .34（原版 `:disabled svg`）。
			ui.Icon(c, pick(a.Playing, iconPause, iconPlay)).Size(playIconSize, playIconSize).
				TextColor(playerPlayInk(a.Playing, play.Hovered())).
				Opacity(playerDisabledIconOpacityFor(!src))
		})
		if play.Clicked() && a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}

		next := ui.ButtonBase(c.Key("next")).Size(32, 32).Radius(RadiusPill).Center().
			Label(nextTip()).Tooltip(nextTip())
		next.Disabled(!a.canNavigate())
		if !a.canNavigate() {
			// 原版 .player-button:disabled 是 opacity .42（mygo 已乘 .5）。
			next.Opacity(0.84)
		}
		playerToolSurface(next, false, next.Hovered())
		pressPose(next)
		next.Children(func() {
			// 原版下一集图标 size 18；颜色沿用 .player-button 的 #334155（未被
			// tool 规则覆盖），悬停转蓝。
			ink := ui.Hex("#334155")
			if next.Hovered() {
				ink = ui.Hex("#0284c7")
			}
			ui.Icon(c, iconNext).Size(nextIconSize, nextIconSize).TextColor(ink).
				Opacity(playerDisabledIconOpacityFor(!a.canNavigate()))
		})
		if next.Clicked() && a.Act.Next != nil {
			a.Act.Next()
		}
	})
}

// shownPos 是进度条左侧时间该显示的位置：操作中显示滑块停的值，
// 其余时候显示真实播放位置。
func (a *App) shownPos() float64 {
	if a.wasHeld || a.SeekBusy {
		return a.SeekValue
	}
	return a.Pos
}

// playerTimeWeight 是原版 `#player .player-time` 的字重（600）。时间列要等宽
// 且比正文实一点：普通字重压在浅色玻璃上看着发虚。
const playerTimeWeight = 600

// timeText 是第 2 / 第 4 列（56px）的时间文本。
func (a *App) timeText(c *ui.Context, sec float64) { a.timeTextAt(c, sec, 11, 56) }

// timeTextAt 画一个时间标签：原版 .player-time 是 11px / 600 / 居中 /
// color var(--player-time-color)，mini 下字号收成 9px、列宽 44（其余不变）。
// 主窗与迷你窗共用，字重与颜色只有一处。
func (a *App) timeTextAt(c *ui.Context, sec float64, size, width float32) {
	ui.Text(c, fmtTime(sec)).FontSize(size).FontWeight(playerTimeWeight).
		TextColor(ui.Hex("#334155").Alpha(0.72)).Width(width).Shrink(0).Center()
}

// progress 是第 3 列（1fr）的可拖动进度条，上面叠广告段标记。
// seekSettle 是松手/seek 完成后滑块停在目标值、不跟随播放位置的时长。
const seekSettle = 250 * time.Millisecond

// sponsorTolerance 是广告段标记与实际媒体时长的允许差（原版同名常量）。
const sponsorTolerance = 2.0

// paintTimeline 画进度条的轨道、已播进度与滑块。轨道与滑块的画法在
// rangeslider.go（与音量条共用），这里只多出缓冲区与广告段两层。
//
// 原版这套外观是纯 CSS（6px 轨道 + 1px 白边 + 青→蓝渐变填充 + 14px 玻璃
// 滑块），框架默认的 Slider 只有一条素色轨道，所以这里用 SliderBase + 自绘。
//
// 层序与原版的 z-index 一致：外投影 → 缓冲（z-index 0）→ 广告段（z-index 0，
// 在缓冲之上）→ 轨道本体（z-index 1）→ 滑块。缓冲与广告段都**不在**轨道底色
// 之上，而是透过半透明底色显出来（原版就是这么叠的）。
func paintTimeline(p *ui.Painter, r ui.Rect, rp rangePaint) {
	track := rangeTrackRect(r, rp.Geom)
	paintRangeShadow(p, track, rp.Hovered)

	buffered := clamp01(rp.Buffered)
	// 缓冲层最先画：原版这一层 z-index 0，在轨道（z-index 1）之下，
	// 所以是透过 32% 白的轨道底色显出来的青→天蓝，而不是实色。
	if buffered > 0.001 {
		bw := track.W * buffered
		p.FillGradient(ui.Rect{X: track.X, Y: track.Y, W: bw, H: track.H}, ui.LinearGradient{
			From:  ui.Hex("#5eead4").Alpha(0.50),
			To:    ui.Hex("#38bdf8").Alpha(0.46),
			Angle: 90,
		}, timelineTrackH/2)
		// 缓冲末端的 1px 收尾标记（原版 .player-timeline-buffered::after）。
		p.Fill(ui.Rect{X: track.X + bw - 0.5, Y: track.Y, W: 1, H: track.H},
			ui.Hex("#7dd3fc").Alpha(0.70), 0)
	}
	// 广告段标记：原版这一层是 z-index 0（在轨道 z-index 1 之下，但在缓冲层
	// 之上），所以红色是透过半透明轨道透出来的淡红，而不是压在轨道上的实红。
	if rp.Dur > 0 {
		for _, seg := range rp.Segs {
			if seg.VideoDuration > 0 && abs(seg.VideoDuration-rp.Dur) > sponsorTolerance {
				continue // 分段属于另一版视频，位置对不上，丢掉
			}
			lo := clampF(float64(seg.Start)/rp.Dur, 0, 1)
			hi := clampF(float64(seg.End)/rp.Dur, 0, 1)
			if hi <= lo {
				continue
			}
			x0 := track.X + track.W*float32(lo)
			x1 := track.X + track.W*float32(hi)
			p.Fill(ui.Rect{X: x0, Y: track.Y, W: x1 - x0, H: track.H},
				ui.Hex("#f87171").Alpha(0.35), 0)
		}
	}

	paintRangeFill(p, track, rp.Frac, rp.Hovered, rp.Colors)
	paintRangeThumb(p, rangeThumbX(track, rp.Geom, rp.Frac), r.Y+r.H/2, rp.Hovered, rp.Held)
}

func (a *App) progress(c *ui.Context) {
	hi := a.Dur
	if hi <= 0 {
		hi = 1
	}
	// 进度条 = 原版 <input type=range>，既显示也是输入。
	//
	// *value 有两个写入方，靠「指针是否按住」分开：
	//   框架（sliderInput，在每帧构建之后的 applyInputs 里跑）——
	//         指针按住时按指针位置写（点击与拖动是同一件事）；
	//   应用——无人操作时写播放位置（滑块跟随播放）。
	//
	// held 用 Dragged() 的 ok：它就是 st.pressed，**与元素矩形无关**。
	// 不能用 Pressed()——指针一旦移出细轨道它就返回 false，拖动会被误判
	// 成松手（这是「拖到一半失效」的原因）。
	//
	// PaddingX(7) 让取值区间两端各缩进一个滑块半径，滑块的圆心才能走到
	// 轨道的两个端点（原生 range 的行为）。
	s := ui.SliderBase(c, &a.SeekValue, 0, hi).FillWidth().Height(timelineH).
		PaddingX(timelineInset).Label("播放进度")
	_, _, held := s.Dragged()
	changed := s.Changed() // 应用框架本帧的输入（键盘改值也走这里）

	if held != a.wasHeld {
		a.SeekStableAt = time.Now() // 交接点：按下 / 松手
	}
	seek, next := progressPlan(progressInput{
		held:    held,
		changed: changed,
		busy:    a.SeekBusy,
		settle:  time.Since(a.SeekStableAt),
		pos:     a.Pos,
	}, a.SeekValue, a.wasHeld, seekSettle)
	if seek && a.Act.Seek != nil {
		a.Act.Seek(a.SeekValue) // 用用户放下的值提交
	}
	a.SeekValue = next
	a.wasHeld = held

	frac := float32(a.SeekValue / hi)
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		paintTimeline(p, r, a.timelinePaint(s, frac, held))
	})
	a.seekBubble(c, s, a.Dur)
}

// bufferedFrac 把「已预读的媒体秒数」换算成进度轨道的 0..1 比例。
func bufferedFrac(buffered, dur float64) float32 {
	if dur <= 0 || buffered <= 0 {
		return 0
	}
	return float32(clampF(buffered/dur, 0, 1))
}

// timelinePaint 收集进度条这一帧的绘制输入：悬停 / 拖动态取自框架的指针
// 状态（主窗与迷你窗各有一条进度条，两条共用同一套外观与状态机），
// 缓冲区比例与广告段数据取自 App。
func (a *App) timelinePaint(s ui.Element, frac float32, held bool) rangePaint {
	return rangePaint{
		Frac:     frac,
		Buffered: bufferedFrac(a.Buffered, a.Dur),
		Segs:     a.SponsorSegments,
		Dur:      a.Dur,
		Hovered:  s.Hovered(),
		Held:     held,
		Colors:   rangeColorsFor(a.Theme),
		Geom:     timelineGeom,
	}
}

// playerSlot 是播放栏的定宽列——原版 #player .player-controls 是
// grid-template-columns: 80px 56px 1fr 56px 34px 40px 34px 40px，
// 列宽由列容器决定，**里面按钮一律 32×32**（.player-button）。
// 返回列容器本身：音量 / 倍速弹层要挂在它上面当锚点。
func playerSlot(c *ui.Context, width float32, build func()) ui.Element {
	return ui.Box(c).Width(width).Height(32).Shrink(0).Children(build)
}

// volumeButton 是第 5 列（34px）：点开音量弹层。
func (a *App) volumeButton(c *ui.Context) {
	b := ui.ButtonBase(c.Key("volume")).Size(32, 32).Radius(Radius).Center().
		Label("音量").Tooltip("音量")
	// 原版 disabled={!src}。
	disablePlayerButton(b, !a.hasSource())
	playerToolSurface(b, a.ShowVolume, b.Hovered())
	// 原版：音量键图标随音量变化（0 视为静音）；颜色同工具键（#526174）。
	muted := a.Muted || a.Volume <= 0
	b.Children(func() {
		ui.Icon(c, pick(muted, iconMute, iconVolume)).Size(volumeIconSize, volumeIconSize).
			TextColor(playerToolInk(a.ShowVolume, b.Hovered())).
			Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
	})
	if b.Clicked() {
		a.ShowVolume = !a.ShowVolume
		a.ShowSpeed = false
	}
}

// speedButton 是第 6 列（40px）：显示当前倍速，点开档位菜单。
func (a *App) speedButton(c *ui.Context) {
	// 原版 .player-speed-button：40×32（列宽同为 40）。
	// tooltip 是「播放速度 X 倍」（用原始数值，不是 1.0x 的标签）。
	b := ui.ButtonBase(c.Key("speed")).Size(40, 32).Radius(Radius).Center().
		Label(speedTip(a.Speed)).Tooltip(speedTip(a.Speed))
	// 原版 disabled={!src}。
	disablePlayerButton(b, !a.hasSource())
	playerToolSurface(b, a.ShowSpeed, b.Hovered())
	pressPose(b)
	b.Children(func() {
		// 原版 .player-speed-button：11px / 700 / tabular-nums / #526174。
		ui.Text(c, speedLabel(a.Speed)).FontSize(11).FontWeight(700).
			TextColor(playerToolInk(a.ShowSpeed, b.Hovered()))
	})
	if b.Clicked() {
		a.ShowSpeed = !a.ShowSpeed
		a.ShowVolume = false
	}
}

// eqButton 是第 7 列（34px）：均衡开关。
func (a *App) eqButton(c *ui.Context) {
	// 原版 .player-eq-button：34×32（列宽 34），tooltip「音量均衡: 开/关」。
	b := ui.ButtonBase(c.Key("eq")).Size(34, 32).Radius(Radius).Center().
		Label(eqTip(a.EQ)).Tooltip(eqTip(a.EQ))
	// 原版 disabled={!src}。
	disablePlayerButton(b, !a.hasSource())
	playerToolSurface(b, a.EQ, b.Hovered())
	pressPose(b)
	b.Children(func() {
		ui.Icon(c, iconEQ).Size(eqIconSize, eqIconSize).TextColor(playerToolInk(a.EQ, b.Hovered())).
			Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
	})
	if b.Clicked() && a.Act.ToggleEQ != nil {
		a.Act.ToggleEQ()
	}
}

// sponsorButton 是第 8 列（40px）：跳过赞助片段开关。
func (a *App) sponsorButton(c *ui.Context) {
	// 原版 .player-sponsor-button：34×32（列宽 40，留 6px 余量）。
	// 这个键**不随「无视频」禁用**（原版没给它 disabled）。
	b := ui.ButtonBase(c.Key("sponsor")).Size(34, 32).Radius(Radius).Center().
		Label(sponsorTip()).Tooltip(sponsorTip())
	playerToolSurface(b, a.Sponsor, b.Hovered())
	pressPose(b)
	b.Children(func() {
		ui.Icon(c, iconSponsor).Size(eqIconSize, eqIconSize).
			TextColor(playerToolInk(a.Sponsor, b.Hovered()))
		// 状态点（原版 .player-sponsor-status）：**常显**（未启用时也是灰点），
		// 6px + 1px 白边 + 右上角 3px，绿=有分段、黄=查询中、红=失败、灰=无/未启用。
		// tooltip 是原版那串状态文案（ok 时带段数、有原因时附在冒号后）。
		dot := ui.Box(c).Size(6, 6).Radius(RadiusPill).Absolute().Right(3).Top(3).
			Background(sponsorDotColor(a.SponsorStatus)).
			Border(1, ui.Hex("#ffffff").Alpha(0.75)).
			Tooltip(sponsorStatusTitle(a.SponsorStatus,
				pick(a.Sponsor, a.SponsorCount, 0), a.SponsorReason))
		// 纯视觉，不能吃掉按钮的点击。
		dot.PassThrough()
	})
	if b.Clicked() && a.Act.ToggleSponsor != nil {
		a.Act.ToggleSponsor()
	}
}

// sponsorDotColor 把 SponsorBlock 状态映射成状态点颜色。
func sponsorDotColor(status string) ui.Color {
	switch status {
	case "loading":
		return ui.Hex("#f59e0b")
	case "ok":
		return ui.Hex("#22c55e")
	case "error":
		return ui.Hex("#ef4444")
	default:
		return ui.Hex("#94a3b8")
	}
}

// ---------------------------------------------------------------- 文案
//
// 播放栏控件的 aria-label / title 都按原版（main:src/components/player.tsx）。
// 文案抽成纯函数：既让界面与测试共用同一个事实源，也避免两边各写一份字符串
// 又慢慢漂开（"下一集" / "跳过赞助片段" 就是这么遗留的）。

// playTip 是播放/暂停键的文案（原版 isPlaying ? "暂停" : "播放"）。
func playTip(playing bool) string { return pick(playing, "暂停", "播放") }

// nextTip 是下一集键的文案。原版是「下一个视频」（不是「下一集」）。
func nextTip() string { return "下一个视频" }

// speedTip 是倍速键的文案（原版 `播放速度 ${playbackRate} 倍`）——
// 用**原始数值**（1 → "播放速度 1 倍"），不是 "1.0x" 那个显示标签。
func speedTip(rate float64) string { return fmt.Sprintf("播放速度 %g 倍", rate) }

// eqTip 是均衡键的文案（原版「音量均衡: 开/关」）。
func eqTip(on bool) string { return pick(on, "音量均衡: 开", "音量均衡: 关") }

// sponsorTip 是恰饭跳过键的文案（原版「自动跳过恰饭片段（SponsorBlock）」）。
func sponsorTip() string { return "自动跳过恰饭片段（SponsorBlock）" }

// sponsorStatusLabel 是状态点 tooltip 的前缀（原版 SPONSOR_STATUS_LABEL）。
// Go 内部把「未启用」叫 off（原版叫 disabled），只差这个名字。
func sponsorStatusLabel(status string) string {
	switch status {
	case "loading":
		return "SponsorBlock 查询中…"
	case "ok":
		return "SponsorBlock 已加载"
	case "empty":
		return "SponsorBlock 无片段"
	case "error":
		return "SponsorBlock 查询失败"
	default:
		return "SponsorBlock 未启用"
	}
}

// sponsorStatusTitle 是状态点的 tooltip，复刻原版那串拼接：
// 状态文案 [+ " (N 段)"（仅 ok）] [+ ": 原因"（有原因时）]。
func sponsorStatusTitle(status string, count int, reason string) string {
	s := sponsorStatusLabel(status)
	if status == "ok" {
		s += fmt.Sprintf(" (%d 段)", count)
	}
	if reason != "" {
		s += ": " + reason
	}
	return s
}

// hasSource 报告播放栏是否有可操作的媒体，对应原版播放键/音量/倍速/均衡
// 的 `disabled={!src}`：队列空（还没选过任何视频）时整排控件是禁用态。
func (a *App) hasSource() bool { return a.Current() != nil }

// ---------------------------------------------------------------- 弹层

// 弹层右边与锚点列左侧的间距（原版 .player-volume-popover /
// .player-speed-popover 都是 `right: calc(100% + 10px)`，挂在音量 / 倍速
// 的**列容器**上；mini 下是 8px）。
const (
	speedPopoverEdgeGap      = 10
	volumePopoverEdgeGap     = 10
	miniVolumePopoverEdgeGap = 8
)

// speedPopover 是倍速弹层（原版 .player-speed-popover）：从倍速列
// **左侧横向弹出**的一排档位胶囊（min-width 36、11px/650、圆角 8，
// 白玻璃 0.82 + 14 圆角 + 蓝底 16% 的激活态）。
//
// 位置用 AttachTo 表达「挂在锚点列左侧、与栏垂直居中」（原版 top 50% +
// translateY(-50%)、right calc(100% + 10px)），不再写死窗口坐标。
// 「点弹层外关闭」交给框架的 PressedOutside：它只读这一帧的按下记录
// （原版也是 document 的 pointerdown 判定），不像之前那块全窗捕获层那样
// 参与命中测试 —— 弹层开着时整窗的悬停照常（捕获层会把 hover 全吃掉）。
// 按下锚点（倍速列）不算外部，所以「先开音量再点倍速」不会被当成点外部
// 而关掉刚开的倍速弹层。
func (a *App) speedPopover(c *ui.Context, anchor ui.Element) {
	t := a.Theme
	// 材质：近乎实白 rgba(255,255,255,.82) + 1px 白边 .72 + blur(10)，
	// 与音量弹层的薄玻璃不同（倍速弹层要压住下面的内容）。
	// 原版是近乎实白（rf.82）+ blur(10)；换成框架玻璃，抬一级投影
	// 让它从播放栏上"浮起来"。
	panel := ui.Row(c).AttachTo(anchor, ui.AnchorLeft, ui.AnchorRight).
		Margin(0, speedPopoverEdgeGap, 0, 0).
		Padding(4).Gap(2).Radius(14).
		Material(glassLayer(glass.Glass{})).
		Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.12)).
		AlignItems(ui.Center)
	if panel.PressedOutside() {
		a.ShowSpeed = false
	}
	panel.Children(func() {
		for _, opt := range speedOptions {
			active := abs(opt-a.Speed) < 0.001
			opt := opt
			b := ui.ButtonBase(c.Key(fmt.Sprintf("rate-%v", opt))).
				MinWidth(36).Height(24).Padding(0, 8).Radius(8).
				Center().Label(speedLabel(opt))
			switch {
			case active:
				b.Background(t.Blue.Alpha(0.16))
			case b.Hovered():
				b.Background(t.Blue.Alpha(0.10))
			}
			b.Children(func() {
				ui.Text(c, speedLabel(opt)).FontSize(11).FontWeight(650).
					TextColor(pick(active, t.Blue, ui.Hex("#475569")))
			})
			if b.Clicked() {
				a.Speed = opt
				a.ShowSpeed = false
				if a.Act.SetSpeed != nil {
					a.Act.SetSpeed(opt)
				}
			}
		}
	})
}

// volumePopover 是音量滑块，从播放栏第 5 列上方弹出。
func (a *App) volumePopover(c *ui.Context, anchor ui.Element) {
	// 原版 .player-volume-popover 挂在音量按钮的列容器上（right: calc(100%+10px)、
	// top: 50% + translateY(-50%)）：弹层在列**左侧 10px**、并与播放栏垂直居中。
	a.volumePopoverAt(c, anchor, volumePopoverEdgeGap,
		volumePopoverW, volumePopoverH, volumePopoverRadius)
}

// 音量弹层的几何（原版 .player-volume-popover）：184×42 / 圆角 14，内部
// padding 0 10 0 8、gap 8、静音键 26×26；挂在音量列左侧（间距见
// volumePopoverEdgeGap）并与播放栏垂直居中。抽成常量：视图与测试按同一份
// 几何取样，改一处就够。
const (
	volumePopoverW      = 184
	volumePopoverH      = 42
	volumePopoverRadius = 14
	volumePopoverPadL   = 8
	volumePopoverPadR   = 10
	volumePopoverGap    = 8
	volumeMuteSize      = 26
)

// volumePopoverAt 是音量弹层（原版 .player-volume-popover：从音量列
// **左侧横向弹出**，184×42 一行：静音钮 + 滑杆）。位置与关闭方式同
// speedPopover：AttachTo 挂列 + PressedOutside 点外部关。
func (a *App) volumePopoverAt(c *ui.Context, anchor ui.Element, edgeGap, w, h, radius float32) {
	t := a.Theme
	// 原版宽度 184 / 高 42 / padding 0 10 0 8 / 圆角 14；
	// 底 rgba(255,255,255,.34) + 1px 玻璃边 + specular/lifted 阴影。
	panel := ui.Row(c).AttachTo(anchor, ui.AnchorLeft, ui.AnchorRight).
		Margin(0, edgeGap, 0, 0).Width(w).Height(h).
		Padding(0, volumePopoverPadR, 0, volumePopoverPadL).Radius(radius).
		Material(glassLayer(glass.Glass{})).
		Shadow(0, 14, 40, 0, ui.Hex("#0f172a").Alpha(0.10)).Gap(volumePopoverGap).
		AlignItems(ui.Center)
	if panel.PressedOutside() {
		a.ShowVolume = false
	}
	panel.Children(func() {
		// 静音键：独立的切换（原版 player-volume-mute-button）。
		mute := ui.ButtonBase(c.Key("volume-mute")).Size(volumeMuteSize, volumeMuteSize).
			Radius(RadiusSmall).
			Center().Label(pick(a.Muted, "取消静音", "静音")).Tooltip(pick(a.Muted, "取消静音", "静音"))
		if a.Muted {
			mute.Background(t.Rose.Alpha(0.18))
		} else if mute.Hovered() {
			mute.Background(t.GlassHover)
		} else {
			mute.Background(ui.Transparent)
		}
		mute.Children(func() {
			ui.Icon(c, pick(a.Muted, iconMute, iconVolume)).Size(16, 16).
				TextColor(pick(a.Muted, t.Rose, t.Muted))
		})
		if mute.Clicked() && a.Act.ToggleMute != nil {
			a.Act.ToggleMute()
		}

		v := a.Volume
		// 音量条就是原版 input.player-volume-slider：轨道/滑块的 CSS 与
		// 进度条逐字相同（同一组 thumb 规则 + 同样的 6px 渐变轨道），
		// 所以这里也用 SliderBase + rangeslider.go 的同一份画法。
		// 之前用的是框架默认滑杆（细轨道 + 实心旋钮），与原版完全不同。
		sl := ui.SliderBase(c, &v, 0, 1).Grow(1).Height(timelineH).
			PaddingX(timelineInset).Label("音量")
		_, _, held := sl.Dragged()
		sl.Draw(func(p *ui.Painter, r ui.Rect) {
			paintRangeBar(p, r, rangePaint{
				Frac:    float32(v),
				Hovered: sl.Hovered(),
				Held:    held,
				Colors:  rangeColorsFor(a.Theme),
				Geom:    volumeGeom,
			})
		})
		if sl.Changed() {
			a.Volume = v
			// 拖动中只应用到音频，**不落盘**（原版注释：仅在拖动提交时写。
			// 跟着每次 input 写的话，拖一次要写几十次盘）。
			if a.Act.SetVolume != nil {
				a.Act.SetVolume(v)
			}
		}
		if a.wasVolHeld && !held && a.Act.PersistVolume != nil {
			a.Act.PersistVolume(a.Volume)
		}
		a.wasVolHeld = held
	})
}

// fmtTime 把秒格式化成 m:ss / h:mm:ss。
func fmtTime(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	s := int(sec)
	h, m, ss := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, ss)
	}
	return fmt.Sprintf("%d:%02d", m, ss)
}

// speedLabel 是倍速的显示文案（原版 rate===1/2/3 ? toFixed(1) : rate + "x"）：
// 整数档显示一位小数（1.0x/2.0x/3.0x），非整数档原样（0.5x/0.75x/1.25x/1.5x）。
func speedLabel(v float64) string {
	if v == 1 || v == 2 || v == 3 {
		return fmt.Sprintf("%.1fx", v)
	}
	return fmt.Sprintf("%gx", v)
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

// speedOptions 是倍速可选项，与旧版的档位一致。
var speedOptions = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0}

// miniProgress 是迷你窗的进度条（与主窗的 progress 共用同一套 seek
// 状态机，只是尺寸更小、没有广告段标记）。Slider 同样每帧只构建一次。
func (a *App) miniProgress(c *ui.Context) {
	hi := a.Dur
	if hi <= 0 {
		hi = 1
	}
	// 与主窗同一套规则与外观（见 progress / paintTimeline）。
	s := ui.SliderBase(c, &a.SeekValue, 0, hi).Grow(1).Height(timelineH).
		PaddingX(timelineInset).Label("播放进度")
	_, _, held := s.Dragged()
	changed := s.Changed()
	if held != a.wasHeld {
		a.SeekStableAt = time.Now()
	}
	seek, next := progressPlan(progressInput{
		held:    held,
		changed: changed,
		busy:    a.SeekBusy,
		settle:  time.Since(a.SeekStableAt),
		pos:     a.Pos,
	}, a.SeekValue, a.wasHeld, seekSettle)
	if seek && a.Act.Seek != nil {
		a.Act.Seek(a.SeekValue)
	}
	a.SeekValue = next
	a.wasHeld = held

	frac := float32(a.SeekValue / hi)
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		paintTimeline(p, r, a.timelinePaint(s, frac, held))
	})
	a.seekBubble(c, s, a.Dur)
}

// seekBubble 是悬停时间气泡（原版 .player-seek-bubble）：指针停在轨道上时，
// 在对应位置的上方 6px 显示那个点的时间。原版只在精确指针下启用（触摸不显示），
// mygo 拿不到指针类型，就一视同仁。
func (a *App) seekBubble(c *ui.Context, s ui.Element, dur float64) {
	px, _, over := s.PointerPosition()
	if !over || dur <= 0 {
		return
	}
	w := s.Bounds().W
	span := w - 2*timelineInset
	if span <= 0 {
		return
	}
	ratio := clampF(float64((px-timelineInset)/span), 0, 1)
	label := fmtTime(dur * ratio)
	// 气泡宽度按字符数估（原版靠 CSS 自适应，mygo 要自己算才能居中）。
	bw := float32(len(label))*6.2 + 14
	left := px - bw/2
	if left < 0 {
		left = 0
	}
	if max := w - bw; left > max {
		left = max
	}
	// 作为进度条的**子元素**挂上去：Absolute 的定位基准是父级的 padding box，
	// 直接建在这里会以「播放栏那一行」为父级，整块位置会偏掉。
	// PassThrough：气泡只是显示，不能把进度条的指针事件吃掉。
	s.Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(650).
			TextColor(ui.Hex("#0f172a")).
			Absolute().Left(left).Bottom(timelineH+6).
			Width(bw).Center().PassThrough().
			Padding(2, 6).Radius(7).
			Background(ui.Hex("#ffffff").Alpha(0.86)).
			Border(1, ui.Hex("#ffffff").Alpha(0.72)).
			Shadow(0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.14))
	})
}

// clampF 把 v 夹到 [lo, hi]。
func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
