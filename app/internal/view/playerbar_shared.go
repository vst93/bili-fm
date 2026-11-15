package view

// 播放栏的**共享实现**：主窗底部播放栏与视频窗底部控件条用的是同一份代码，
// 靠 PlayerBarBind 这个「绑定目标」区分控制谁。
//
// 这不是把主窗那套复制一份改吧改吧：键的尺寸 / 圆角 / 材质 / 悬停过渡 /
// 按压手感 / tooltip / 焦点环、时间文本的字号字重、进度条的轨道与滑块绘制、
// 音量与倍速弹层，全部只有这里一份。两个窗口各自只提供：
//
//   - 一个 PlayerSettings 指针（共享设置：音量/静音/倍速/均衡/广告跳过）；
//   - 传输状态（播放中、位置、时长、缓冲、seek 回调）—— 各绑各的引擎；
//   - 少量开关（这一帧显示哪些键、键下是薄玻璃还是面板、弹层开着没有）。
//
// 唯一按底材分叉的是键的**底色**：主窗播放栏是半透明玻璃（浅色下白 .22），
// 视频窗控件条是 Panel（浅色下白 .88）——白压白看不见。所以浅色面板上换成
// 墨色淡底；深色面板与播放栏同一套玻璃值，直接复用。

import (
	"fmt"

	"github.com/egoist/mygo/ui"
)

// PlayerTimelineInput 是进度条这一帧的输入结论。
type PlayerTimelineInput struct {
	Held    bool // 指针按住（拖动 / 按下）
	Changed bool // 框架本帧改了值（拖动或方向键）
	Cancel  bool // 按下被系统取消
}

// PlayerTimelineBind 是一条播放进度条的绑定。主窗与视频窗共用 playerTimeline
// 的输入接线与绘制，只在 OnFrame 里跑各自的状态机：
//
//   - 主窗：progressPlan（拖动中停在目标值、seek 未落地时等、松手后回落）；
//   - 视频窗：直通播放器（拖动即 seek，松手跟随播放位置）。
type PlayerTimelineBind struct {
	Value    *float64
	Dark     bool
	Dur      float64
	Buffered float64
	Segments []Segment
	Chapters []float64
	Colors   rangeColors
	// SkinFor 是这一帧的轨道/滑块材质（按悬停态取）；nil 时按主题取
	// （主窗播放栏）。
	SkinFor func(hovered bool) rangeSkin
	// OnFrame 在框架应用输入之后调用；返回是否提交 seek 与滑块该停的值。
	OnFrame func(PlayerTimelineInput) (commit bool, value float64)
	// OnSeek 是提交 seek 的出口（主窗走 Act.Seek，视频窗直通播放器）。
	OnSeek func(sec float64)
	// Handle 可选：把这条滑块绑到一个 ui.Handle，调用方以后能用它查到
	// 轨道**布局出来的**矩形（弹幕热力就靠它对齐宽度，见 heatBarPainter）。
	// 视频窗不传，主窗传 &App.progressHandle。
	Handle *ui.Handle
	// TrackH 是轨道的视觉高度（0 → timelineTrackH）。视频窗要细轨道（4）。
	TrackH float32
	// BufferedFrom / BufferedTo 是缓冲层的颜色（零值 → 原版那对青→天蓝）。
	BufferedFrom ui.Color
	BufferedTo   ui.Color
	// HideIdleThumb 让滑块只在悬停 / 拖动中出现（视频窗）。
	HideIdleThumb bool
	// Bubble 是悬停时间气泡（主窗与视频窗都挂同一份）。
	Bubble *seekBubbleState
	// Overlay 在轨道画完之后叠一层（视频窗的缓冲呼吸描边）。
	Overlay func(p *ui.Painter, r ui.Rect, rp rangePaint)
}

// playerTimeline 画一条播放进度条（外观与交互同主窗播放栏）。
func playerTimeline(c *ui.Context, b PlayerTimelineBind) ui.Element {
	if b.Value == nil {
		b.Value = new(float64)
	}
	hi := maxF64(b.Dur, 1)
	s := ui.SliderBase(c, b.Value, 0, hi).FillWidth().Height(timelineH).
		PaddingX(timelineInset).Label("播放进度").Step(0.1)
	if b.Handle != nil {
		s = s.Bind(b.Handle)
	}
	s.Disabled(b.Dur <= 0).DimDisabled(false)
	if b.Dur <= 0 {
		s.Opacity(progressDisabledOpacity)
	}
	focusOutline(s, 0, focusStudio)

	_, _, held := s.Dragged()
	canceled := s.PressCanceled()
	changed := s.Changed()

	commit, next := false, *b.Value
	if b.OnFrame != nil {
		commit, next = b.OnFrame(PlayerTimelineInput{Held: held, Changed: changed, Cancel: canceled})
	}
	*b.Value = next
	if commit && b.OnSeek != nil {
		// 用用户放下的值提交（主窗的 progressPlan 已经把 next 夹好）。
		b.OnSeek(next)
	}

	frac := float32(0)
	if b.Dur > 0 {
		frac = float32(clampF(*b.Value/b.Dur, 0, 1))
	}
	var skin *rangeSkin
	if b.SkinFor != nil {
		sk := b.SkinFor(s.Hovered())
		skin = &sk
	}
	rp := rangePaint{
		Frac:          frac,
		Dark:          b.Dark,
		Buffered:      bufferedFrac(b.Buffered, b.Dur),
		Segs:          b.Segments,
		Dur:           b.Dur,
		ChapterMarks:  b.Chapters,
		Hovered:       s.Hovered(),
		Held:          held,
		Colors:        b.Colors,
		Geom:          timelineGeomWith(b.TrackH),
		ThumbScale:    s.AnimateTo("player-thumb-scale", rangeThumbScale(s.Hovered(), held), thumbScaleAnim),
		Skin:          skin,
		BufferedFrom:  b.BufferedFrom,
		BufferedTo:    b.BufferedTo,
		HideIdleThumb: b.HideIdleThumb,
	}
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		paintTimeline(p, r, rp)
		if b.Overlay != nil {
			b.Overlay(p, r, rp)
		}
	})
	if b.Bubble != nil {
		b.Bubble.draw(c, s, b.Dur)
	}
	return s
}

// seekBubbleState 是悬停时间气泡的文案与位置。主窗与视频窗各持一份，
// 绘制代码只有一份（drawSeekBubble）。
type seekBubbleState struct {
	seekBubbleLabel string
	seekBubbleWidth float32
	seekBubbleLeft  float32
}

// draw 在进度条上方画 / 淡出时间气泡。语义见原 seekBubble：
// 文案与位置只在悬停时更新，移开后停在最后位置淡出。
func (st *seekBubbleState) draw(c *ui.Context, s ui.Element, dur float64) {
	px, _, _ := s.PointerPosition()
	over := s.Hovered()
	show := over && dur > 0
	w := s.Bounds().W
	span := w - 2*timelineInset
	measure := seekBubbleMeasure(c)
	if show && span > 0 {
		ratio := clampF(float64((px-timelineInset)/span), 0, 1)
		st.seekBubbleLabel = fmtTime(dur * ratio)
		st.seekBubbleWidth = seekBubbleWidth(measure, st.seekBubbleLabel)
		left := px - st.seekBubbleWidth/2
		if left < 0 {
			left = 0
		}
		if maxLeft := w - st.seekBubbleWidth; left > maxLeft {
			left = maxLeft
		}
		st.seekBubbleLeft = left
	}
	label := st.seekBubbleLabel
	bw := st.seekBubbleWidth
	left := st.seekBubbleLeft
	th, tsh := textLineBox(seekBubbleFont, seekBubbleLH)
	s.Children(func() {
		box := ui.Box(c).Key("seek-bubble").
			Absolute().Left(left).Bottom(timelineH+6).
			Width(bw).Height(seekBubbleH).Center().PassThrough().
			Padding(seekBubblePadY, seekBubblePadX).Radius(seekBubbleRadius).
			Background(ui.Hex("#ffffff").Alpha(0.86)).
			Border(seekBubbleBorder, ui.Hex("#ffffff").Alpha(0.72)).
			InsetShadow(0, 1, 1, 0, seekBubbleTopHighlight()).
			Shadow(0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.14))
		target := float32(0)
		if show {
			target = 1
		}
		box.Opacity(box.AnimateTo("seek-bubble-fade", target, seekBubbleFadeAnim))
		if label != "" {
			box.Children(func() {
				numericText(c, label).FontSize(seekBubbleFont).FontWeight(650).
					Height(th).Translate(0, tsh).
					TextColor(ui.Hex("#0f172a"))
			})
		}
	})
}

// PlayerKeyMetrics 是一帧里控件条键的尺寸令牌。零值 = 主窗播放栏（原版 #player
// 的 grid 列：播放 42 正圆、工具键 32、均衡 34、倍速列 40）；视频窗传
// videoKeyMetrics()，于是顶栏与控件条的全部键同尺寸同圆角（30×30 / 圆角 6）。
//
// 这是「两个窗口共用一份控件代码」与「各自一套视觉令牌」之间的接缝：共享代码
// 只读令牌，不再写字面量；哪个窗口用哪套由绑定目标决定。
type PlayerKeyMetrics struct {
	Key      float32 // 方形图标键的边长
	KeyR     float32 // 方形图标键的圆角
	Wide     float32 // 算法/过滤类键的宽度（主窗 34）
	Play     float32 // 播放键边长（直径）
	PlayR    float32 // 播放键圆角（主窗 RadiusPill 正圆；视频窗与图标键同档）
	Icon     float32 // 图标的光学尺寸（0 → 各键自己的原版值）
	PlayIcon float32 // 播放图标的尺寸
	SpeedW   float32 // 倍速（文字值键）的宽度
	// 列宽：键居中放在这一列里。视频窗把列宽收成键宽，键与键之间因此正好
	// 是行间距；主窗保留原版 grid 的 34/40。
	ToolSlot     float32
	SpeedSlot    float32
	SponsorSlot  float32
	TransportGap float32
	TimeW        float32 // 时间码列宽（固定宽度）
	TimeSize     float32 // 时间码字号
	TrackH       float32 // 进度条视觉轨道高度（0 → timelineTrackH）
}

// playerKeyMetricsMain 是主窗播放栏那套尺寸（原版 #player 的 grid 列）。
func playerKeyMetricsMain() PlayerKeyMetrics {
	return PlayerKeyMetrics{
		Key: 32, KeyR: Radius, Wide: 34,
		Play: 42, PlayR: RadiusPill, PlayIcon: playIconSize,
		SpeedW: 40, ToolSlot: 34, SpeedSlot: 40, SponsorSlot: 40,
		TransportGap: 6, TimeW: 56, TimeSize: 11, TrackH: timelineTrackH,
	}
}

// keyMetrics 返回这一帧的键尺寸令牌。
func (b PlayerBarBind) keyMetrics() PlayerKeyMetrics {
	if b.Metrics != nil {
		return *b.Metrics
	}
	return playerKeyMetricsMain()
}

// icon 返回某一枚图标的光学尺寸：令牌给了统一值就用它，否则沿用各键自己的
// 原版取值（主窗播放栏的图标本来就不是同一个尺寸）。
func (m PlayerKeyMetrics) icon(def float32) float32 {
	if m.Icon > 0 {
		return m.Icon
	}
	return def
}

// PlayerBarBind 是一次播放栏渲染的绑定目标快照。主窗与视频窗每帧各建一份，
// 然后调用同一组 playerTransportItems / playerTimelineItems /
// playerSettingsItems。字段全是值或回调，方便无头测试直接构造。
type PlayerBarBind struct {
	Theme Theme
	Set   *PlayerSettings

	// 传输（各自独立，互不镜像）
	Playing   bool
	OnPlay    func()
	HasSrc    bool
	Pos       float64
	Dur       float64
	Buffered  float64
	SeekValue *float64
	OnFrame   func(PlayerTimelineInput) (commit bool, value float64)
	OnSeek    func(sec float64)
	// TimelineHandle 可选：绑定主窗的进度条句柄（App.progressHandle）。
	// 弹幕热力用它取轨道真正的布局矩形，宽度才能与进度条一致。
	TimelineHandle *ui.Handle
	// Bubble 是悬停时间气泡的状态（每帧复用同一份，不能新建）。
	Bubble *seekBubbleState
	// Timeline 允许调用方换掉材质 / 颜色 / 叠缓冲描边；零值按主窗播放栏。
	TimelineSkinFor func(hovered bool) rangeSkin
	TimelineColors  *rangeColors
	TimelineOverlay func(p *ui.Painter, r ui.Rect, rp rangePaint)
	// TimelineBufferedFrom / To 是缓冲层的颜色（零值 → 播放栏那对青→天蓝）。
	// 视频窗换成灰蓝的中性带，与青→蓝的播放层一眼分色。
	TimelineBufferedFrom ui.Color
	TimelineBufferedTo   ui.Color
	// TimelineThumbOnHover 让滑块只在悬停 / 拖动时出现（视频窗）。
	TimelineThumbOnHover bool

	// 进度条上的数据。主窗有章节与预读；视频窗只有恰饭段。
	Segments []Segment
	Chapters []float64

	// 上一集 / 下一集。视频窗没有换集能力时 CanNext=false 并给出说明。
	NextTip string
	CanNext bool
	OnNext  func()

	// 弹层开关（各窗口自己的界面状态）
	ShowVolume  bool
	ShowSpeed   bool
	OnVolumeKey func()
	OnSpeedKey  func()

	// 这一帧显示哪些控件（窄窗渐进收纳）。
	ShowPos     bool
	ShowDur     bool
	ShowNext    bool
	ShowEQ      bool
	ShowSponsor bool

	// EQUnavailable 表示这个窗口的声音不经过本仓库的 DSP 管线（原生视频由
	// 系统媒体框架直接解码出声）：音量均衡键置灰、不可点，tooltip 说明原因
	// 并报出主窗那条链路的当前值。**不**把共享的 EQ 当成本窗口的「开」态
	// 显示 —— 那会让人以为视频的声音真被均衡了（见 eqVideoUnavailableTip）。
	EQUnavailable bool

	// OnPanel 表示键压在视频窗的 Panel 上（浅色下换墨色淡底）。
	OnPanel bool
	// Metrics 是这一帧键的尺寸令牌（nil → 主窗播放栏那套）。
	Metrics *PlayerKeyMetrics
}

// playerTimeText 画一枚播放栏时间标签（主窗与视频窗同一份字号/字重/颜色）。
func playerTimeText(c *ui.Context, t Theme, sec float64, size, width float32) {
	numericText(c, fmtTime(sec)).FontSize(size).FontWeight(playerTimeWeight).
		TextColor(playerTimeInk(t)).Width(width).Shrink(0).
		LineHeight(18 / size).
		TextAlign(ui.Center).Center()
}

// playerTransportItems 画传输键：播放 + 下一集。视频窗的下一集没有目标
// （没有换集能力）时由 CanNext=false 置灰并给说明。
//
// 尺寸全部来自绑定目标的键令牌（b.keyMetrics）：主窗是 42 正圆 + 32，视频窗
// 是两颗同尺寸的 30×30 圆角 6 —— 同一份接线，两套令牌。
func playerTransportItems(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := b.keyMetrics()
	width := m.Play
	if b.ShowNext {
		width += m.TransportGap + m.Key
	}
	ui.Row(c).Width(width).Shrink(0).Gap(m.TransportGap).AlignItems(ui.Center).Children(func() {
		play := ui.ButtonBase(c.Key("play")).Size(m.Play, m.Play).Radius(m.PlayR).Center().
			Label(playTip(b.Playing)).Tooltip(playTip(b.Playing))
		disablePlayerButton(play, !b.HasSrc)
		if b.OnPanel {
			videoPlaySurface(t, play, play.Hovered(), play.Pressed())
		} else {
			playerPlaySurface(t, play, b.Playing, play.Hovered(), play.Pressed())
		}
		playerPressPose(play)
		if !b.OnPanel {
			playerRim(t, play, m.PlayR, false, sheenKindPlay, b.Playing, play.Hovered())
		}
		play.Children(func() {
			ic := pick(b.Playing, iconPause, iconPlayOne)
			ink := playerPlayInk(t, b.Playing, play.Hovered())
			if b.OnPanel {
				ink = videoPlayInk(t, play.Hovered())
			}
			icon := ui.Icon(c, ic).Size(m.PlayIcon, m.PlayIcon).
				TextColor(playerFadeColor(c, play, "player-play-ink", ink)).
				Opacity(playerDisabledIconOpacityFor(!b.HasSrc)).
				Translate(0.5, 0)
			playerIconShadowFn(icon, ic, 0, 1, playerPlayIconShadowInk)
		})
		if play.Clicked() && b.OnPlay != nil {
			b.OnPlay()
		}

		if !b.ShowNext {
			return
		}
		tip := b.NextTip
		if tip == "" {
			tip = nextTip()
		}
		next := ui.ButtonBase(c.Key("next")).Size(m.Key, m.Key).Radius(m.KeyR).Center().
			Label(tip).Tooltip(tip)
		disablePlayerButton(next, !b.CanNext)
		if b.OnPanel {
			videoKeyShape(c, t, next, headerState{Hovered: next.Hovered(), Pressed: next.Pressed()})
		} else {
			playerToolSurface(c, t, toolNext, next, false, next.Hovered(), next.Pressed())
		}
		playerPressPose(next)
		if !b.OnPanel {
			playerRim(t, next, m.KeyR, true, sheenKindStatic, false, false)
		}
		next.Children(func() {
			ink := playerButtonInk(t, next.Hovered())
			if b.OnPanel {
				ink = videoKeyInk(t, false, next.Hovered())
			}
			icon := ui.Icon(c, iconNext).Size(m.icon(nextIconSize), m.icon(nextIconSize)).
				TextColor(playerFadeColor(c, next, "player-next-ink", ink)).
				Opacity(playerDisabledIconOpacityFor(!b.CanNext))
			playerIconShadowFn(icon, iconNext, 0, 1, playerIconShadowInk)
		})
		if next.Clicked() && b.CanNext && b.OnNext != nil {
			b.OnNext()
		}
	})
}

// playerTimelineItems 画「当前时间 + 进度条 + 总时长」三列。主窗一行布局与
// 视频窗两行布局的第一行都调它；窄窗只收时间列，进度条始终在。时间列宽度由
// 令牌给（主窗 56、视频窗 52）—— 固定宽度 + 等宽数字，播放时不左右跳。
func playerTimelineItems(c *ui.Context, b PlayerBarBind) {
	m := b.keyMetrics()
	if b.ShowPos {
		playerTimeText(c, b.Theme, b.Pos, m.TimeSize, m.TimeW)
	}
	colors := rangeColorsFor(b.Theme)
	if b.TimelineColors != nil {
		colors = *b.TimelineColors
	}
	playerTimeline(c, PlayerTimelineBind{
		Value:         b.SeekValue,
		Dark:          b.Theme.Dark,
		Dur:           b.Dur,
		Buffered:      b.Buffered,
		Segments:      b.Segments,
		Chapters:      b.Chapters,
		Colors:        colors,
		SkinFor:       b.TimelineSkinFor,
		OnFrame:       b.OnFrame,
		OnSeek:        b.OnSeek,
		Overlay:       b.TimelineOverlay,
		Bubble:        b.Bubble,
		Handle:        b.TimelineHandle,
		TrackH:        m.TrackH,
		BufferedFrom:  b.TimelineBufferedFrom,
		BufferedTo:    b.TimelineBufferedTo,
		HideIdleThumb: b.TimelineThumbOnHover,
	})
	if b.ShowDur {
		playerTimeText(c, b.Theme, b.Dur, m.TimeSize, m.TimeW)
	}
}

// playerSettingsItems 画四颗「设置类」键：音量 / 倍速 / 均衡 / 跳过广告。
// 音量与倍速两列的句柄回填给调用方（弹层挂在列上）。四颗键都读写
// PlayerSettings，由各自窗口的监听把值下发到引擎 —— 这就是「改一边另一边
// 立刻反映」的接线点。
//
// 例外：EQUnavailable（原生视频窗）时均衡键置灰不可点 —— 那一窗口的声音
// 不经过本仓库的 DSP 管线，eqVideoUnavailableTip 写明了原因与主窗当前值。
func playerSettingsItems(c *ui.Context, b PlayerBarBind, volumeSlot, speedSlot *ui.Element) {
	m := b.keyMetrics()
	*volumeSlot = playerSlot(c, m.ToolSlot, m.Key, func() { playerVolumeKey(c, b) })
	*speedSlot = playerSlot(c, m.SpeedSlot, m.Key, func() { playerSpeedKey(c, b) })
	if b.ShowEQ {
		playerSlot(c, m.ToolSlot, m.Key, func() { playerEQKey(c, b) })
	}
	if b.ShowSponsor {
		playerSlot(c, m.SponsorSlot, m.Key, func() { playerSponsorKey(c, b) })
	}
}

// playerVolumeKey 是音量列：点开音量弹层。与主窗播放栏同一颗键，尺寸由
// 绑定目标的键令牌给（主窗 32、视频窗 30）。
func playerVolumeKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := b.keyMetrics()
	set := b.Set
	btn := ui.ButtonBase(c.Key("volume")).Size(m.Key, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleDisclosure).Label("音量").Tooltip("音量").Expanded(b.ShowVolume)
	disablePlayerButton(btn, !b.HasSrc)
	if b.OnPanel {
		videoKeyShape(c, t, btn, headerState{Selected: b.ShowVolume, Hovered: btn.Hovered(), Pressed: btn.Pressed()})
	} else {
		playerToolSurface(c, t, toolVolume, btn, b.ShowVolume, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel {
		playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	}
	muted := set.Muted || set.Volume <= 0
	btn.Children(func() {
		ic := volumeIconFor(muted)
		ink := playerToolInk(t, toolVolume, b.ShowVolume, btn.Hovered())
		if b.OnPanel {
			ink = videoKeyInk(t, b.ShowVolume, btn.Hovered())
		}
		sz := m.icon(volumeIconSize)
		icon := ui.Icon(c, ic).Size(sz, sz).
			TextColor(playerFadeColor(c, btn, "player-volume-ink", ink)).
			Opacity(playerDisabledIconOpacityFor(!b.HasSrc))
		playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
	})
	if btn.Clicked() && b.OnVolumeKey != nil {
		b.OnVolumeKey()
	}
}

// playerSpeedKey 是倍速列：显示当前倍速，点开档位菜单。宽度取令牌的
// SpeedW（文字值键放得下 1.25x），高度与圆角与图标键同档。
func playerSpeedKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := b.keyMetrics()
	set := b.Set
	tip := speedTip(set.Speed)
	btn := ui.ButtonBase(c.Key("speed")).Size(m.SpeedW, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleDisclosure).Label(tip).Tooltip(tip).Expanded(b.ShowSpeed)
	disablePlayerButton(btn, !b.HasSrc)
	if b.OnPanel {
		videoKeyShape(c, t, btn, headerState{Selected: b.ShowSpeed, Hovered: btn.Hovered(), Pressed: btn.Pressed()})
	} else {
		playerToolSurface(c, t, toolSpeed, btn, b.ShowSpeed, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel {
		playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolSpeed, b.ShowSpeed, btn.Hovered())
		if b.OnPanel {
			ink = videoKeyInk(t, b.ShowSpeed, btn.Hovered())
		}
		numericText(c, speedLabel(set.Speed)).FontSize(11).FontWeight(700).
			TextColor(playerFadeColor(c, btn, "player-speed-ink", ink))
	})
	if btn.Clicked() && b.OnSpeedKey != nil {
		b.OnSpeedKey()
	}
}

// playerEQKey 是第 7 列（34px）：音量均衡开关。视频窗没有自己的 EQ 引擎，
// 这颗键改的是共享设置，由主窗的监听落到音频引擎。
//
// 但「视频窗没有自己的 EQ 引擎」不是「视频的声音会被均衡」：原生视频的
// 音频由系统媒体框架直接输出，本仓库拿不到那路 PCM。所以视频窗传
// EQUnavailable=true —— 键置灰、点击不写共享设置，tooltip 说明不适用并报出
// 主窗当前值。主窗的这颗键行为不变（照常读写共享设置与 internal/media）。
func playerEQKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := b.keyMetrics()
	set := b.Set
	tip := eqTip(set.EQ)
	// 不可用时**不**画成「已开」：共享值只以文字出现在 tooltip 里，避免把
	// 主窗那条链路的开启态误读成「这个窗口的均衡已生效」。
	on := set.EQ
	if b.EQUnavailable {
		tip = eqVideoUnavailableTip(set.EQ)
		on = false
	}
	btn := ui.ButtonBase(c.Key("eq")).Size(m.Wide, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleToggleButton).Label(tip).Tooltip(tip).Checked(on)
	disablePlayerButton(btn, !b.HasSrc || b.EQUnavailable)
	if b.OnPanel {
		videoKeyShape(c, t, btn, headerState{Selected: on, Hovered: btn.Hovered(), Pressed: btn.Pressed()})
	} else {
		playerToolSurface(c, t, toolEq, btn, on, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel {
		playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolEq, on, btn.Hovered())
		if b.OnPanel {
			ink = videoKeyInk(t, on, btn.Hovered())
		}
		sz := m.icon(eqIconSize)
		icon := ui.Icon(c, iconEQ).Size(sz, sz).
			TextColor(playerFadeColor(c, btn, "player-eq-ink", ink)).
			Opacity(playerDisabledIconOpacityFor(!b.HasSrc))
		playerIconShadowFn(icon, iconEQ, 0, 1, playerIconShadowInk)
	})
	if btn.Clicked() {
		set.SetEQ(!set.EQ)
	}
}

// playerSponsorKey 是第 8 列（40px）：跳过赞助片段开关。状态点与 tooltip
// 读共享设置里的 SponsorStatus / Count / Reason —— 视频窗与主窗同一份。
func playerSponsorKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := b.keyMetrics()
	set := b.Set
	btn := ui.ButtonBase(c.Key("sponsor")).Size(m.Wide, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleToggleButton).Label(sponsorTip()).Tooltip(sponsorTip()).Checked(set.Sponsor)
	if b.OnPanel {
		videoKeyShape(c, t, btn, headerState{Selected: set.Sponsor})
	} else {
		playerToolSurface(c, t, toolSponsor, btn, set.Sponsor, false, false)
	}
	playerPressPose(btn)
	if !b.OnPanel {
		playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolSponsor, set.Sponsor, false)
		if b.OnPanel {
			ink = videoKeyInk(t, set.Sponsor, false)
		}
		sz := m.icon(eqIconSize)
		icon := ui.Icon(c, iconSponsor).Size(sz, sz).
			TextColor(playerFadeColor(c, btn, "player-sponsor-ink", ink))
		playerIconShadowFn(icon, iconSponsor, 0, 1, playerIconShadowInk)
		dot := ui.Box(c).Size(6, 6).Radius(RadiusPill).Absolute().Right(3).Top(3).
			Background(sponsorDotColor(set.SponsorStatus)).
			Border(1, ui.Hex("#ffffff").Alpha(0.75)).
			Tooltip(sponsorStatusTitle(set.SponsorStatus,
				pick(set.Sponsor, set.SponsorCount, 0), set.SponsorReason))
		dot.PassThrough()
	})
	if btn.Clicked() {
		set.SetSponsor(!set.Sponsor)
	}
}

// ---------------------------------------------------------------- 视频窗面板键
//
// 视频窗控件条上的工具键（音量 / 倍速 / 均衡 / 跳过）不再各写一套：
// 它们与顶栏五颗键共用 videotokens.go 的 videoKeyShape / videoKeyInk
//（四态只叠一层 accent，不发光、不叠描边+投影）。共享代码里只剩一个
// `b.OnPanel` 分支指向那套令牌。

// ---------------------------------------------------------------- 弹层

// playerSpeedPopover 是倍速弹层（主窗与视频窗共用）。columns > 0 时按网格
// 排（视频窗窄，7 个档位一行放不下）；0 表示主窗那一条横排。
func playerSpeedPopover(c *ui.Context, t Theme, set *PlayerSettings, show *bool, anchor ui.Element, columns int, justOpened bool) ui.Element {
	var panel ui.Element
	if columns > 0 {
		panel = ui.Column(c)
	} else {
		panel = ui.Row(c)
	}
	panel = panel.AttachTo(anchor, ui.AnchorLeft, ui.AnchorRight).
		Margin(0, speedPopoverEdgeGap, 0, 0).
		Padding(4).Gap(2).Radius(speedPopoverRadius).
		Shadow(0, pick[float32](t.Dark, 10, 8), 24, 0, ui.Hex("#0f172a").Alpha(pick[float32](t.Dark, 0.14, 0.12))).
		AlignItems(ui.Center)
	panel = playerPopoverMaterial(t, panel,
		playerPopoverBorder(t, ui.Hex("#ffffff").Alpha(0.72)),
		playerPopoverVeil(t, ui.Hex("#ffffff").Alpha(0.82)), speedPopoverBlur)
	panel = applyInsetShadows(panel, playerPopoverInsets(t))
	panel = popoverGlass(panel, speedPopoverRadius, speedPopoverSheen(t))
	// 打开它的那一次点击会落在弹层之外（弹层这一帧才建出来），不能当成
	// 「点外面关闭」，否则一开就关（justOpened 由调用方按点击前的状态给）。
	if !justOpened && panel.PressedOutside() && show != nil {
		*show = false
	}
	panel.Children(func() {
		build := func(opt float64) {
			playerSpeedOption(c, t, set, opt, func() {
				set.SetSpeed(opt)
				if show != nil {
					*show = false
				}
			})
		}
		if columns <= 0 {
			for _, opt := range speedOptions {
				build(opt)
			}
			return
		}
		// 网格：每行 columns 个，行内仍用主窗那颗档位胶囊。
		for i := 0; i < len(speedOptions); i += columns {
			end := i + columns
			if end > len(speedOptions) {
				end = len(speedOptions)
			}
			row := speedOptions[i:end]
			ui.Row(c).Gap(2).Children(func() {
				for _, opt := range row {
					build(opt)
				}
			})
		}
	})
	return panel
}

// playerSpeedOption 画一颗倍速档位胶囊（原版 .player-speed-option）。
func playerSpeedOption(c *ui.Context, t Theme, set *PlayerSettings, opt float64, onPick func()) {
	active := abs(opt-set.Speed) < 0.001
	b := ui.ButtonBase(c.Key(fmt.Sprintf("rate-%v", opt))).
		MinWidth(36).Padding(4, 8).Radius(8).
		Center().Label(speedLabel(opt))
	switch {
	case active:
		b.Background(t.Blue.Alpha(0.16))
	case b.Hovered():
		b.Background(t.Blue.Alpha(0.10))
	}
	b.Children(func() {
		numericText(c, speedLabel(opt)).FontSize(11).FontWeight(650).
			TextColor(pick(active, t.Blue, playerSpeedOptionInk(t)))
	})
	if b.Clicked() {
		onPick()
	}
}

// playerVolumePopover 是音量弹层（主窗与视频窗共用）：静音键 + 音量条。
// onRelease 在拖动提交时调用一次（落盘），held 是各自的「上一帧是否按住」。
func playerVolumePopover(c *ui.Context, t Theme, set *PlayerSettings, show *bool, anchor ui.Element, held *bool, justOpened bool, onRelease func()) ui.Element {
	lift := playerPopoverLiftSpec(t)
	panel := ui.Row(c).AttachTo(anchor, ui.AnchorLeft, ui.AnchorRight).
		Margin(0, volumePopoverEdgeGap, 0, 0).Width(volumePopoverW).Height(volumePopoverH).
		Padding(0, volumePopoverPadR, 0, volumePopoverPadL).Radius(volumePopoverRadius).
		Shadow(lift.X, lift.Y, lift.Blur, lift.Spread, lift.C).Gap(volumePopoverGap).
		AlignItems(ui.Center)
	panel = playerPopoverMaterial(t, panel,
		playerPopoverBorder(t, t.GlassBorder),
		playerPopoverVeil(t, ui.Hex("#ffffff").Alpha(0.34)), volumePopoverBlur)
	panel = applyInsetShadows(panel, playerPopoverInsets(t))
	panel = popoverGlass(panel, volumePopoverRadius, volumePopoverSheen(t))
	panel.TransformOrigin(1, 0.5)
	if tr, ok := volumePopoverEnter(c); ok {
		panel.Transition(tr)
	}
	if !justOpened && panel.PressedOutside() && show != nil {
		*show = false
	}
	panel.Children(func() {
		muted := set.Muted || set.Volume <= 0
		mute := ui.ButtonBase(c.Key("volume-mute")).Size(volumeMuteSize, volumeMuteSize).
			Radius(RadiusSmall).
			Center().Role(ui.RoleToggleButton).Label(pick(muted, "取消静音", "静音")).
			Tooltip(pick(muted, "取消静音", "静音")).Checked(muted)
		playerMuteSurface(t, mute, muted)
		playerPressPose(mute)
		playerRim(t, mute, RadiusSmall, false, sheenKindStatic, false, false)
		mute.Children(func() {
			ic := volumeIconFor(muted)
			icon := ui.Icon(c, ic).Size(16, 16).
				TextColor(playerMuteInk(t, muted))
			playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
		})
		if mute.Clicked() {
			set.ToggleMute()
		}

		v := set.Volume
		sl := ui.SliderBase(c, &v, 0, 1).Grow(1).Height(timelineH).
			PaddingX(timelineInset).Label("音量滑块")
		focusOutline(sl, 0, focusStudio)
		_, _, drag := sl.Dragged()
		sl.Draw(func(p *ui.Painter, r ui.Rect) {
			paintRangeBar(p, r, rangePaint{
				Frac:    float32(v),
				Hovered: sl.Hovered(),
				Held:    drag,
				Dark:    t.Dark,
				Colors:  rangeColorsFor(t),
				Geom:    volumeGeom,
			})
		})
		if sl.Changed() {
			set.SetVolume(v)
		}
		if held != nil {
			if *held && !drag && onRelease != nil {
				onRelease()
			}
			*held = drag
		}
	})
	return panel
}

// ---------------------------------------------------------------- 布局参数
