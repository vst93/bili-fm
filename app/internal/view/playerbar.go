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
	// 栏顶的滚动边缘效果（原版 #player::before 那条 10px 渐变就是在手搓这个）：
	// 内容滚到栏下时淡出/结霜，栏体上沿才不会是一条硬切线。
	// 框架的 ScrollEdge 就是 macOS 26+ 的这个效果；软边缘淡出、硬边缘结霜。
	// 高度按框架的约定 = 栏高 + 10（软边缘多出的那 10 DIP）。
	// Background 必须显式给：主题的 Background 被我们设成了透明。
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(66).Shrink(0).
		PassThrough().Material(glass.ScrollEdge{Background: t.PlayerSurface})
	// 播放栏：原版这里是 backdrop-filter: blur(20px) saturate(1.4) 的一层玻璃。
	// 用框架的 Liquid Glass 而不是自铺半透明白 —— 玻璃自带边缘折射与 1px 描光
	// （原版那套 inset 亮线/暗线是在手搓同一件事）。
	ui.Column(c).FillWidth().Height(56).Shrink(0).
		Material(glass.Glass{}).
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
				playerSlot(c, 34, func() { a.volumeButton(c) })
				playerSlot(c, 40, func() { a.speedButton(c) })
				playerSlot(c, 34, func() { a.eqButton(c) })
				playerSlot(c, 40, func() { a.sponsorButton(c) })
			})
		})

	// 倍速 / 音量弹层：从播放栏上方向上弹。
	if a.ShowSpeed {
		a.speedPopover(c)
	}
	if a.ShowVolume {
		a.volumePopover(c)
	}
}

// glassTint 按「激活 / 悬停 / 常态」给按钮选玻璃的 Tint —— 播放栏里的小按钮
// 统一走框架玻璃，状态差异只体现在 Tint 上（原版是用半透明白底 + 蓝底表达的）。
func glassTint(active, hovered bool, activeTint, hoverTint ui.Color) glass.Glass {
	switch {
	case active:
		return glass.Glass{Tint: activeTint}
	case hovered:
		return glass.Glass{Tint: hoverTint}
	}
	return glass.Glass{}
}

// glassOn 给「常态无框」的小图标按钮上材质：只有激活/悬停才铺玻璃面，
// 常态保持透明 —— 原版 .nav-icon-btn 常态就是透明 + 一点径向高光，
// 每颗都铺一块玻璃会显得"每个图标都套了个盒子"。
func glassOn(b ui.Element, active, hovered bool, activeTint, hoverTint ui.Color) ui.Element {
	switch {
	case active:
		return b.Material(glass.Glass{Tint: activeTint})
	case hovered:
		return b.Material(glass.Glass{Tint: hoverTint})
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

// transport 是第 1 列（80px）：播放/暂停（42） + 下一集（32）。
func (a *App) transport(c *ui.Context) {
	t := a.Theme
	ui.Row(c).Width(80).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
		// 原版 .player-play-button：42×42 正圆，播放中 #0369a1、暂停态 #475569，
		// 内亮线（inset 0 1px 1px rgba(255,255,255,.82)）。
		// 播放键用框架的 Interactive 玻璃：按下时玻璃会长大一点
		//（macOS 27 的 NSGlassEffectView 行为），这正是原版那套手搓
		// inset 阴影想表达而做不到的"液态"感。播放中用 Tint 上蓝。
		play := ui.ButtonBase(c.Key("play")).Size(42, 42).Radius(RadiusPill).Center().
			Label(pick(a.Playing, "暂停", "播放")).Tooltip(pick(a.Playing, "暂停", "播放"))
		pg := glass.Glass{Interactive: true}
		if a.Playing {
			pg.Tint = pick(play.Hovered(), t.Blue.Alpha(0.34), t.Blue.Alpha(0.22))
		} else if play.Hovered() {
			pg.Tint = ui.Hex("#ffffff").Alpha(0.22)
		}
		play.Material(pg)
		play.Children(func() {
			ink := pick(a.Playing, ui.Hex("#0369a1"), ui.Hex("#475569"))
			ui.Icon(c, pick(a.Playing, iconPause, iconPlay)).Size(18, 18).TextColor(ink)
		})
		if play.Clicked() && a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}

		next := ui.ButtonBase(c.Key("next")).Size(32, 32).Radius(Radius).Center().
			Label("下一集").Tooltip("下一集")
		next.Disabled(!a.canNavigate())
		if !a.canNavigate() {
			// 原版 .player-button:disabled 是 opacity .42（mygo 已乘 .5）。
			next.Opacity(0.84)
		}
		next.Material(glassTint(false, next.Hovered(),
			ui.Color{}, ui.Hex("#ffffff").Alpha(0.18)))
		next.Children(func() { ui.Icon(c, iconNext).Size(15, 15).TextColor(ui.Hex("#334155")) })
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

// timeText 是第 2 / 第 4 列（56px）的时间文本。
func (a *App) timeText(c *ui.Context, sec float64) {
	ui.Text(c, fmtTime(sec)).FontSize(11).TextColor(ui.Hex("#334155").Alpha(0.72)).
		Width(56).Shrink(0).Center()
}

// progress 是第 3 列（1fr）的可拖动进度条，上面叠广告段标记。
// seekSettle 是松手/seek 完成后滑块停在目标值、不跟随播放位置的时长。
const seekSettle = 250 * time.Millisecond

// 进度条（原版 #player .player-timeline）的视觉参数。
const (
	timelineH      = 24 // 外层高（可点击区）
	timelineTrackH = 6  // 轨道高
	timelineInset  = 7  // 轨道左右内缩 = 滑块半径
	timelineThumb  = 14 // 滑块直径
)

// paintTimeline 画进度条的轨道、已播进度与滑块。原版这套外观是纯 CSS
// （6px 轨道 + 1px 白边 + 青→蓝渐变填充 + 14px 玻璃滑块），框架默认的
// Slider 只有一条素色轨道，所以这里用 SliderBase + 自绘。
func paintTimeline(p *ui.Painter, r ui.Rect, frac float32) {
	if frac < 0 {
		frac = 0
	} else if frac > 1 {
		frac = 1
	}
	cy := r.Y + r.H/2
	track := ui.Rect{
		X: r.X + timelineInset,
		Y: cy - timelineTrackH/2,
		W: r.W - timelineInset*2,
		H: timelineTrackH,
	}
	// 未播部分：半透明白（原版 --player-track-rest）。
	p.Fill(track, ui.Hex("#ffffff").Alpha(0.32), timelineTrackH/2)
	if frac > 0.001 {
		// 已播部分：90° 青 → 蓝（原版渐变在 --player-progress 处硬停）。
		fill := ui.Rect{X: track.X, Y: track.Y, W: track.W * frac, H: track.H}
		p.FillGradient(fill, ui.LinearGradient{
			From:  ui.Hex("#2dd4bf").Alpha(0.78),
			To:    ui.Hex("#0284c7").Alpha(0.82),
			Angle: 90,
		}, timelineTrackH/2)
	}
	p.Stroke(track, ui.Hex("#ffffff").Alpha(0.68), timelineTrackH/2, 1)

	// 滑块：14 圆、1px 白边、淡蓝底 + 蓝色投影。
	cx := r.X + timelineInset + (r.W-timelineThumb)*frac
	k := ui.Rect{
		X: cx - timelineThumb/2,
		Y: cy - timelineThumb/2,
		W: timelineThumb,
		H: timelineThumb,
	}
	p.Shadow(k, timelineThumb/2, 0, 2, 7, 0, ui.Hex("#0284c7").Alpha(0.24))
	p.Fill(k, ui.Hex("#e0f2fe").Alpha(0.62), timelineThumb/2)
	p.Stroke(k, ui.Hex("#ffffff").Alpha(0.90), timelineThumb/2, 1)
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
	s.Draw(func(p *ui.Painter, r ui.Rect) { paintTimeline(p, r, frac) })
}

// playerSlot 是播放栏的定宽列——原版 #player .player-controls 是
// grid-template-columns: 80px 56px 1fr 56px 34px 40px 34px 40px，
// 列宽由列容器决定，**里面按钮一律 32×32**（.player-button）。
func playerSlot(c *ui.Context, width float32, build func()) {
	ui.Box(c).Width(width).Height(32).Shrink(0).Children(build)
}

// volumeButton 是第 5 列（34px）：点开音量弹层。
func (a *App) volumeButton(c *ui.Context) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("volume")).Size(32, 32).Radius(Radius).Center().
		Label("音量").Tooltip("音量")
	b.Material(glassTint(a.ShowVolume, b.Hovered(),
		ui.Hex("#ffffff").Alpha(0.30), ui.Hex("#ffffff").Alpha(0.18)))
	// 原版：音量键图标随音量变化（0 视为静音）。
	muted := a.Muted || a.Volume <= 0
	b.Children(func() {
		ui.Icon(c, pick(muted, iconMute, iconVolume)).Size(15, 15).
			TextColor(pick(muted, t.Rose, ui.Hex("#334155")))
	})
	if b.Clicked() {
		a.ShowVolume = !a.ShowVolume
		a.ShowSpeed = false
	}
}

// speedButton 是第 6 列（40px）：显示当前倍速，点开档位菜单。
func (a *App) speedButton(c *ui.Context) {
	// 原版 .player-speed-button：40×32（列宽同为 40）。
	b := ui.ButtonBase(c.Key("speed")).Size(40, 32).Radius(Radius).Center().
		Label("倍速").Tooltip("倍速")
	b.Material(glassTint(a.ShowSpeed, b.Hovered(),
		ui.Hex("#ffffff").Alpha(0.30), ui.Hex("#ffffff").Alpha(0.18)))
	b.Children(func() {
		ui.Text(c, speedLabel(a.Speed)).FontSize(11).TextColor(ui.Hex("#334155"))
	})
	if b.Clicked() {
		a.ShowSpeed = !a.ShowSpeed
		a.ShowVolume = false
	}
}

// eqButton 是第 7 列（34px）：均衡开关。
func (a *App) eqButton(c *ui.Context) {
	t := a.Theme
	// 原版 .player-eq-button：34×32（列宽 34）。
	b := ui.ButtonBase(c.Key("eq")).Size(34, 32).Radius(Radius).Center().
		Label("均衡").Tooltip("均衡")
	b.Material(glassTint(a.EQ, b.Hovered(),
		t.Blue.Alpha(0.34), ui.Hex("#ffffff").Alpha(0.18)))
	b.Children(func() {
		ui.Icon(c, iconEQ).Size(15, 15).TextColor(pick(a.EQ, ui.Hex("#0369a1"), ui.Hex("#334155")))
	})
	if b.Clicked() && a.Act.ToggleEQ != nil {
		a.Act.ToggleEQ()
	}
}

// sponsorButton 是第 8 列（40px）：跳过赞助片段开关。
func (a *App) sponsorButton(c *ui.Context) {
	t := a.Theme
	// 原版 .player-sponsor-button：34×32（列宽 40，留 6px 余量）。
	b := ui.ButtonBase(c.Key("sponsor")).Size(34, 32).Radius(Radius).Center().
		Label("跳过赞助").Tooltip("跳过赞助片段")
	b.Material(glassTint(a.Sponsor, b.Hovered(),
		t.Blue.Alpha(0.34), ui.Hex("#ffffff").Alpha(0.18)))
	b.Children(func() {
		ui.Icon(c, iconSponsor).Size(15, 15).
			TextColor(pick(a.Sponsor, ui.Hex("#0369a1"), ui.Hex("#334155")))
		// 状态点（原版按钮右上角的小圆）：绿=有分段，黄=查询中，红=失败，灰=无/未启用。
		if a.Sponsor {
			ui.Box(c).Size(6, 6).Radius(RadiusPill).Absolute().Right(4).Top(4).
				Background(sponsorDotColor(a.SponsorStatus))
		}
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

// ---------------------------------------------------------------- 弹层

// speedPopover 是倍速档位菜单，从播放栏第 6 列上方弹出。
// speedPopover 是倍速弹层（原版 .player-speed-popover）：从倍速按钮
// **左侧横向弹出**的一排档位胶囊（min-width 36、11px/650、圆角 8，
// 白玻璃 0.82 + 14 圆角 + 蓝底 16% 的激活态）。
func (a *App) speedPopover(c *ui.Context) {
	t := a.Theme
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Top(0).PassThrough().Children(func() {
		// 原版 .player-speed-popover：速度列（右起 108..148）左侧 10px → 右边 158；
		// 高 32（4 padding + 24 选项）居中于 56 的栏 → 距底 12。
		// 材质：近乎实白 rgba(255,255,255,.82) + 1px 白边 .72 + blur(10)，
		// 与音量弹层的薄玻璃不同（倍速弹层要压住下面的内容）。
		ui.Row(c).Absolute().Right(158).Bottom(12).Padding(4).Gap(2).
			Radius(14).
			// 原版是近乎实白（rf.82）+ blur(10)；换成框架玻璃，抬一级投影
			// 让它从播放栏上"浮起来"。
			Material(glass.Glass{}).
			Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.12)).
			AlignItems(ui.Center).
			Children(func() {
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
	})
}

// volumePopover 是音量滑块，从播放栏第 5 列上方弹出。
func (a *App) volumePopover(c *ui.Context) {
	// 原版 .player-volume-popover 挂在音量按钮的列容器上（right: calc(100%+10px)、
	// top: 50% + translateY(-50%)）：弹层在按钮**左侧 10px**、并与播放栏垂直居中。
	// 主窗右侧列宽 18(pad)+40+8+34+8+40+8+34 = 190 → 弹层右边 = 200；
	// 高 42 居中于 56 的栏 → 距底 (56-42)/2 = 7。
	a.volumePopoverAt(c, 200, 7, 184, 42, 14)
}

// volumePopoverAt 是音量弹层（原版 .player-volume-popover：从音量按钮
// **左侧横向弹出**，184×42 一行：静音钮 + 滑杆）。
func (a *App) volumePopoverAt(c *ui.Context, right, bottom, w, h, radius float32) {
	t := a.Theme
	ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Top(0).PassThrough().Children(func() {
		// 原版宽度 184 / 高 42 / padding 0 10 0 8 / 圆角 14；
		// 底 rgba(255,255,255,.34) + 1px 玻璃边 + specular/lifted 阴影。
		ui.Row(c).Absolute().Right(right).Bottom(bottom).Width(w).Height(h).
			Padding(0, 10, 0, 8).Radius(radius).
			Material(glass.Glass{}).
			Shadow(0, 14, 40, 0, ui.Hex("#0f172a").Alpha(0.10)).Gap(8).
			AlignItems(ui.Center).Children(func() {
			// 静音键：独立的切换（原版 player-volume-mute-button）。
			mute := ui.ButtonBase(c.Key("volume-mute")).Size(26, 26).Radius(RadiusSmall).
				Center().Label(pick(a.Muted, "取消静音", "静音")).Tooltip(pick(a.Muted, "取消静音", "静音"))
			if a.Muted {
				mute.Background(t.Rose.Alpha(0.18))
			} else if mute.Hovered() {
				mute.Background(t.GlassHover)
			} else {
				mute.Background(ui.Transparent)
			}
			mute.Children(func() {
				ui.Icon(c, pick(a.Muted, iconMute, iconVolume)).Size(14, 14).
					TextColor(pick(a.Muted, t.Rose, t.Muted))
			})
			if mute.Clicked() && a.Act.ToggleMute != nil {
				a.Act.ToggleMute()
			}

			v := a.Volume
			sl := ui.Slider(c, &v, 0, 1).Grow(1).Height(24).Label("音量")
			if sl.Changed() {
				a.Volume = v
				if a.Act.SetVolume != nil {
					a.Act.SetVolume(v)
				}
			}
		})
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
	s.Draw(func(p *ui.Painter, r ui.Rect) { paintTimeline(p, r, frac) })
}
