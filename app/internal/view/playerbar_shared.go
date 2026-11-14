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
		Frac:         frac,
		Dark:         b.Dark,
		Buffered:     bufferedFrac(b.Buffered, b.Dur),
		Segs:         b.Segments,
		Dur:          b.Dur,
		ChapterMarks: b.Chapters,
		Hovered:      s.Hovered(),
		Held:         held,
		Colors:       b.Colors,
		Geom:         timelineGeom,
		ThumbScale:   s.AnimateTo("player-thumb-scale", rangeThumbScale(s.Hovered(), held), thumbScaleAnim),
		Skin:         skin,
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
	// Bubble 是悬停时间气泡的状态（每帧复用同一份，不能新建）。
	Bubble *seekBubbleState
	// Timeline 允许调用方换掉材质 / 颜色 / 叠缓冲描边；零值按主窗播放栏。
	TimelineSkinFor func(hovered bool) rangeSkin
	TimelineColors  *rangeColors
	TimelineOverlay func(p *ui.Painter, r ui.Rect, rp rangePaint)

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

	// OnPanel 表示键压在视频窗的 Panel 上（浅色下换墨色淡底）。
	OnPanel bool
}

// playerTimeText 画一枚播放栏时间标签（主窗与视频窗同一份字号/字重/颜色）。
func playerTimeText(c *ui.Context, t Theme, sec float64, size, width float32) {
	numericText(c, fmtTime(sec)).FontSize(size).FontWeight(playerTimeWeight).
		TextColor(playerTimeInk(t)).Width(width).Shrink(0).
		LineHeight(18 / size).
		TextAlign(ui.Center).Center()
}

// playerTransportItems 画传输键：播放（42 正圆）+ 下一集（32）。视频窗的
// 下一集没有目标（没有换集能力）时由 CanNext=false 置灰并给说明。
func playerTransportItems(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	ui.Row(c).Width(80).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
		play := ui.ButtonBase(c.Key("play")).Size(42, 42).Radius(RadiusPill).Center().
			Label(playTip(b.Playing)).Tooltip(playTip(b.Playing))
		disablePlayerButton(play, !b.HasSrc)
		if b.OnPanel {
			playerPanelPlaySurface(t, play, b.Playing, play.Hovered(), play.Pressed())
		} else {
			playerPlaySurface(t, play, b.Playing, play.Hovered(), play.Pressed())
		}
		playerPressPose(play)
		if !b.OnPanel || t.Dark {
			playerRim(t, play, RadiusPill, false, sheenKindPlay, b.Playing, play.Hovered())
		}
		play.Children(func() {
			ic := pick(b.Playing, iconPause, iconPlayOne)
			ink := playerPlayInk(t, b.Playing, play.Hovered())
			if b.OnPanel {
				ink = playerPanelPlayInk(t, b.Playing, play.Hovered())
			}
			icon := ui.Icon(c, ic).Size(playIconSize, playIconSize).
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
		next := ui.ButtonBase(c.Key("next")).Size(32, 32).Radius(RadiusPill).Center().
			Label(tip).Tooltip(tip)
		disablePlayerButton(next, !b.CanNext)
		if b.OnPanel {
			playerPanelToolSurface(c, t, toolNext, next, false, next.Hovered(), next.Pressed())
		} else {
			playerToolSurface(c, t, toolNext, next, false, next.Hovered(), next.Pressed())
		}
		playerPressPose(next)
		if !b.OnPanel || t.Dark {
			playerRim(t, next, RadiusPill, true, sheenKindStatic, false, false)
		}
		next.Children(func() {
			icon := ui.Icon(c, iconNext).Size(nextIconSize, nextIconSize).
				TextColor(playerFadeColor(c, next, "player-next-ink", playerButtonInk(t, next.Hovered()))).
				Opacity(playerDisabledIconOpacityFor(!b.CanNext))
			playerIconShadowFn(icon, iconNext, 0, 1, playerIconShadowInk)
		})
		if next.Clicked() && b.CanNext && b.OnNext != nil {
			b.OnNext()
		}
	})
}

// playerTimelineItems 画「当前时间 + 进度条 + 总时长」三列。主窗一行布局与
// 视频窗两行布局的第一行都调它；窄窗只收时间列，进度条始终在。
func playerTimelineItems(c *ui.Context, b PlayerBarBind) {
	if b.ShowPos {
		playerTimeText(c, b.Theme, b.Pos, 11, 56)
	}
	colors := rangeColorsFor(b.Theme)
	if b.TimelineColors != nil {
		colors = *b.TimelineColors
	}
	playerTimeline(c, PlayerTimelineBind{
		Value:    b.SeekValue,
		Dark:     b.Theme.Dark,
		Dur:      b.Dur,
		Buffered: b.Buffered,
		Segments: b.Segments,
		Chapters: b.Chapters,
		Colors:   colors,
		SkinFor:  b.TimelineSkinFor,
		OnFrame:  b.OnFrame,
		OnSeek:   b.OnSeek,
		Overlay:  b.TimelineOverlay,
		Bubble:   b.Bubble,
	})
	if b.ShowDur {
		playerTimeText(c, b.Theme, b.Dur, 11, 56)
	}
}

// playerSettingsItems 画四颗「设置类」键：音量 / 倍速 / 均衡 / 跳过广告。
// 音量与倍速两列的句柄回填给调用方（弹层挂在列上）。四颗键都只改
// PlayerSettings，由各自窗口的监听把值下发到引擎 —— 这就是「改一边另一边
// 立刻反映」的接线点。
func playerSettingsItems(c *ui.Context, b PlayerBarBind, volumeSlot, speedSlot *ui.Element) {
	*volumeSlot = playerSlot(c, 34, func() { playerVolumeKey(c, b) })
	*speedSlot = playerSlot(c, 40, func() { playerSpeedKey(c, b) })
	if b.ShowEQ {
		playerSlot(c, 34, func() { playerEQKey(c, b) })
	}
	if b.ShowSponsor {
		playerSlot(c, 40, func() { playerSponsorKey(c, b) })
	}
}

// playerVolumeKey 是第 5 列（34px）：点开音量弹层。与主窗播放栏同一颗键。
func playerVolumeKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	set := b.Set
	btn := ui.ButtonBase(c.Key("volume")).Size(32, 32).Radius(Radius).Center().
		Role(ui.RoleDisclosure).Label("音量").Tooltip("音量").Expanded(b.ShowVolume)
	disablePlayerButton(btn, !b.HasSrc)
	if b.OnPanel {
		playerPanelToolSurface(c, t, toolVolume, btn, b.ShowVolume, btn.Hovered(), btn.Pressed())
	} else {
		playerToolSurface(c, t, toolVolume, btn, b.ShowVolume, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel || t.Dark {
		playerRim(t, btn, Radius, true, sheenKindStatic, false, false)
	}
	muted := set.Muted || set.Volume <= 0
	btn.Children(func() {
		ic := volumeIconFor(muted)
		ink := playerToolInk(t, toolVolume, b.ShowVolume, btn.Hovered())
		if b.OnPanel {
			ink = playerPanelToolInk(t, toolVolume, b.ShowVolume, btn.Hovered())
		}
		icon := ui.Icon(c, ic).Size(volumeIconSize, volumeIconSize).
			TextColor(playerFadeColor(c, btn, "player-volume-ink", ink)).
			Opacity(playerDisabledIconOpacityFor(!b.HasSrc))
		playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
	})
	if btn.Clicked() && b.OnVolumeKey != nil {
		b.OnVolumeKey()
	}
}

// playerSpeedKey 是第 6 列（40px）：显示当前倍速，点开档位菜单。
func playerSpeedKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	set := b.Set
	tip := speedTip(set.Speed)
	btn := ui.ButtonBase(c.Key("speed")).Size(40, 32).Radius(Radius).Center().
		Role(ui.RoleDisclosure).Label(tip).Tooltip(tip).Expanded(b.ShowSpeed)
	disablePlayerButton(btn, !b.HasSrc)
	if b.OnPanel {
		playerPanelToolSurface(c, t, toolSpeed, btn, b.ShowSpeed, btn.Hovered(), btn.Pressed())
	} else {
		playerToolSurface(c, t, toolSpeed, btn, b.ShowSpeed, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel || t.Dark {
		playerRim(t, btn, Radius, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolSpeed, b.ShowSpeed, btn.Hovered())
		if b.OnPanel {
			ink = playerPanelToolInk(t, toolSpeed, b.ShowSpeed, btn.Hovered())
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
func playerEQKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	set := b.Set
	btn := ui.ButtonBase(c.Key("eq")).Size(34, 32).Radius(Radius).Center().
		Role(ui.RoleToggleButton).Label(eqTip(set.EQ)).Tooltip(eqTip(set.EQ)).Checked(set.EQ)
	disablePlayerButton(btn, !b.HasSrc)
	if b.OnPanel {
		playerPanelToolSurface(c, t, toolEq, btn, set.EQ, btn.Hovered(), btn.Pressed())
	} else {
		playerToolSurface(c, t, toolEq, btn, set.EQ, btn.Hovered(), btn.Pressed())
	}
	playerPressPose(btn)
	if !b.OnPanel || t.Dark {
		playerRim(t, btn, Radius, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolEq, set.EQ, btn.Hovered())
		if b.OnPanel {
			ink = playerPanelToolInk(t, toolEq, set.EQ, btn.Hovered())
		}
		icon := ui.Icon(c, iconEQ).Size(eqIconSize, eqIconSize).
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
	set := b.Set
	btn := ui.ButtonBase(c.Key("sponsor")).Size(34, 32).Radius(Radius).Center().
		Role(ui.RoleToggleButton).Label(sponsorTip()).Tooltip(sponsorTip()).Checked(set.Sponsor)
	if b.OnPanel {
		playerPanelToolSurface(c, t, toolSponsor, btn, set.Sponsor, false, false)
	} else {
		playerToolSurface(c, t, toolSponsor, btn, set.Sponsor, false, false)
	}
	playerPressPose(btn)
	if !b.OnPanel || t.Dark {
		playerRim(t, btn, Radius, true, sheenKindStatic, false, false)
	}
	btn.Children(func() {
		ink := playerToolInk(t, toolSponsor, set.Sponsor, false)
		if b.OnPanel {
			ink = playerPanelToolInk(t, toolSponsor, set.Sponsor, false)
		}
		icon := ui.Icon(c, iconSponsor).Size(eqIconSize, eqIconSize).
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

// ---------------------------------------------------------------- 面板键材质

// playerPanelToolSurface 是视频窗 Panel 上的工具键材质：形状 / 过渡 / 按压 /
// 焦点环与主窗完全相同，只有浅色底材那一层换成墨色淡底（白 .06），否则
// 白键压在白面板上等于没有。深色面板直接用播放栏那套玻璃值。
func playerPanelToolSurface(c *ui.Context, t Theme, key playerToolKey, b ui.Element, active, hovered, pressed bool) ui.Element {
	if t.Dark {
		return playerToolSurface(c, t, key, b, active, hovered, pressed)
	}
	fill, border, inset := t.Ink.Alpha(0.06), t.Line, t.Highlight
	switch {
	case pressed:
		fill = t.Ink.Alpha(0.10)
	case active:
		fill, border = t.Blue.Alpha(0.14), t.Blue.Alpha(0.45)
	case hovered:
		fill, border = t.Blue.Alpha(0.10), t.Blue.Alpha(0.32)
	}
	if pressed {
		return applyInsetShadows(b.Background(fill).Border(1, border), playerPressShadowSpecs())
	}
	b = b.Background(playerFadeColor(c, b, "player-tool-bg", fill)).
		Border(1, playerFadeColor(c, b, "player-tool-border", border)).
		InsetShadow(0, 1, 1, 0, inset)
	if active {
		return b.Shadow(0, 2, 8, 0, t.Blue.Alpha(0.10))
	}
	if hovered {
		return b.Shadow(0, 3, 9, 0, shadowInk.Alpha(0.06))
	}
	return b.Shadow(0, 2, 7, 0, shadowInk.Alpha(0.045))
}

// playerPanelToolInk 是 Panel 上工具键的墨色（浅色）；深色复用播放栏那套。
func playerPanelToolInk(t Theme, key playerToolKey, active, hovered bool) ui.Color {
	if t.Dark {
		return playerToolInk(t, key, active, hovered)
	}
	if t.HighContrast {
		if key.hcBlueActive(active) {
			return playerHCActiveInk
		}
		return playerHCInk
	}
	switch {
	case active && hovered:
		return ui.Hex("#0369a1")
	case active || hovered:
		return t.Blue
	}
	return t.Muted
}

// playerPanelPlaySurface 是 Panel 上的播放键材质。浅色下播放键常态就用
// 蓝调淡底（与主窗「播放中」同一档），否则白面板上的白键认不出来。
func playerPanelPlaySurface(t Theme, b ui.Element, playing, hovered, pressed bool) ui.Element {
	if t.Dark {
		return playerPlaySurface(t, b, playing, hovered, pressed)
	}
	fill, border := t.Blue.Alpha(0.16), t.Blue.Alpha(0.42)
	if hovered {
		fill, border = t.Blue.Alpha(0.26), t.Blue.Alpha(0.60)
	}
	if pressed {
		fill = t.Blue.Alpha(0.12)
	}
	b = b.Background(fill).Border(1, border)
	if pressed {
		return applyInsetShadows(b, playerPressShadowSpecs())
	}
	if hovered {
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.70)).
			Shadow(0, 5, 14, 0, t.Blue.Alpha(0.16))
	}
	return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.60)).
		Shadow(0, 3, 10, 0, t.Blue.Alpha(0.10))
}

// playerPanelPlayInk 是 Panel 上播放键的图标色。
func playerPanelPlayInk(t Theme, playing, hovered bool) ui.Color {
	if t.Dark {
		return playerPlayInk(t, playing, hovered)
	}
	if t.HighContrast {
		return playerHCInk
	}
	if hovered {
		return ui.Hex("#0369a1")
	}
	return t.Blue
}

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

// playerBarTitleKey 是视频窗顶栏那三颗键的形状：与主窗标题栏动作键同一套
// （30×30、图标 15、透明底 + 悬停玻璃、按压 .94、焦点环），压在视频面板上
// 时浅色改用墨色淡底，否则白玻璃在白面板上看不见。
func playerBarTitleKey(c *ui.Context, t Theme, key, label, tip string, ic *ui.SVG, active bool) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(30, 30).Radius(RadiusSmall).Center().
		Label(label).Tooltip(tip)
	state := headerStateOf(b, active)
	bg, ink, opacity := ui.Transparent, t.Muted, float32(0.72)
	switch {
	case t.Dark:
		bg, ink, opacity, _ = titleBarButtonStyle(t, state.Selected, state.Hovered, state.Pressed, false)
	case state.Selected:
		bg, ink, opacity = t.Blue.Alpha(0.14), t.Blue, 1
		if state.Hovered {
			bg = t.Blue.Alpha(0.20)
		}
	case state.Pressed:
		bg, ink, opacity = t.Ink.Alpha(0.10), t.Blue, 1
	case state.Hovered:
		bg, ink, opacity = t.Ink.Alpha(0.06), t.Blue, 1
	}
	b.Background(headerFade(c, b, "video-title-fill", bg, headerAnim)).Opacity(opacity)
	if state.Hovered && !state.Pressed {
		b.InsetShadow(0, 1, 1, 0, t.Highlight)
	}
	pressScale(b)
	b.Children(func() {
		ui.Icon(c, ic).Size(15, 15).TextColor(headerFade(c, b, "video-title-ink", ink, headerAnim))
	})
	focusOutline(b, RadiusSmall, t.headerFocusRing())
	return b
}
