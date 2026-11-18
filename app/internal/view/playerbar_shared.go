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
//   - 少量开关（这一帧显示哪些键、弹层开着没有、有没有全屏键、热力数据）。
//
// **键的材质只有一套**（playerToolSurface / playerPlaySurface / playerRim）：
// 视频窗控件条压在 Panel 上也照用这套，hover 只换色值、不换层。唯一按底材
// 分叉的是「图标要不要那层白色 drop-shadow 副本」（OnPanel 时不要）——玻璃
// 播放栏背后是应用的背景，面板背后是视频画面；面板上那层白副本会变成一圈
// 白边，且 hover 时图标墨色在过渡、副本不动，看起来就是图标在闪。

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
	// 轨道**布局出来的**矩形（弹幕热力就靠它对齐宽度，见 playerHeatPainter）。
	// 主窗传 &App.progressHandle，视频窗传 &VideoChrome.progressHandle ——
	// 两窗都绑，热力才都能量到自己的轨道。
	Handle *ui.Handle
	// Bubble 是悬停时间气泡（主窗与视频窗都挂同一份）。
	Bubble *seekBubbleState
	// Overlay 在轨道画完之后叠一层（可选钩子，默认不叠）。
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
		box := ui.Box(c.Key("seek-bubble")).
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

// PlayerKeyMetrics 是控件条键的尺寸令牌（原版 #player 的 grid 列：播放 42
// 正圆、工具键 32、均衡/恰饭 34、倍速列 40、时间列 56）。
//
// 只有**一套**，主窗与视频窗共用：视频窗控件条就是这条播放栏，两窗的键不可能
// 再漂开（视频窗旧的 videoPanelKeys 已删）。结构体留着是为了把「原版 grid」
// 收在一处文档化，绘制函数只读它、不写字面量。
type PlayerKeyMetrics struct {
	Key   float32 // 方形图标键的边长
	KeyR  float32 // 方形图标键的圆角
	Wide  float32 // 算法/过滤类键的宽度（主窗 34）
	Play  float32 // 播放键边长（直径）
	PlayR float32 // 播放键圆角（RadiusPill 正圆）
	// TransportR 是传输键（上一集 / 下一集）的圆角：与播放键一样是 RadiusPill
	// 正圆，**不是**工具键那 8px 方角。458c170 把控件条抽到这里时，工具键统一
	// 成 KeyR，把传输键一起带成了方块（原版 #player 的 32×32 换集键是正圆）。
	TransportR float32
	SpeedW     float32 // 倍速（文字值键）的宽度
	// 列宽：键居中放在这一列里（原版 grid 的 34/40）。
	ToolSlot     float32
	SpeedSlot    float32
	SponsorSlot  float32
	TransportGap float32
	TimeW        float32 // 时间码列宽（固定宽度）
	TimeSize     float32 // 时间码字号
}

// playerBarKeyMetrics 是播放栏那套尺寸（原版 #player 的 grid 列）。
//
// 这是**唯一**一套：主窗底部播放栏与视频窗控件条共用（用户 2026-11：
// 「视频窗控件条 == 主窗播放栏，只多一个全屏按钮」）。视频窗不再有自己的
// 键令牌（旧的 videoPanelKeys 已删）—— 两窗的键尺寸、列宽、时间列、图标
// 尺寸因此不可能再漂开。
var playerBarKeyMetrics = PlayerKeyMetrics{
	Key: 32, KeyR: Radius, Wide: 34,
	Play: 42, PlayR: RadiusPill, TransportR: RadiusPill,
	SpeedW: 40, ToolSlot: 34, SpeedSlot: 40, SponsorSlot: 40,
	TransportGap: 6, TimeW: 56, TimeSize: 11,
}

// playerTransportRadius 是传输键（上一集 / 下一集）的圆角：两键同族、同尺寸
// （32×32），形状也只有**一个**来源 —— 键自己的 .Radius 与描边玩家 playerRim
// 都取这里的值，不会出现「键是圆的、描边还是方的」。
//
// 界面这一版传输列只画下一集（上一集是同一族的另一半，iconPrev 保留未上键）；
// 按名字取值是为了让测试能把两键一起钉住，也提醒后来者：加回上一集键时直接用
// 这个函数，不要再自己写圆角。
func playerTransportRadius(key string) float32 {
	if key == "prev" || key == "next" {
		return playerBarKeyMetrics.TransportR
	}
	return playerBarKeyMetrics.KeyR
}

// PlayerBusy 是播放键上的「忙」提示（issue #42）。它只影响播放键自己的
// 图标 / 文案 / accessibility busy 态，不占额外布局、不加遮罩或横幅。
//
// 主窗、迷你窗与原生视频窗共用这一套：主窗/迷你窗的值来自 App.playerBusy，
// 视频窗来自 VideoChrome.playerBusy（直接读播放器）。「加载中」是起播/首帧
// 还没到，「缓冲中」是播放已经建立、声卡/画面在等数据。
type PlayerBusy uint8

const (
	PlayerIdle PlayerBusy = iota
	PlayerLoading
	PlayerBuffering
)

// playerBusyTip 是忙态下播放键的 tooltip / accessibility 名称。加载中点击
// 不会「播放」（起播已经在路上），缓冲中点击仍然可以暂停，所以两条文案不同。
func playerBusyTip(b PlayerBusy) string {
	switch b {
	case PlayerLoading:
		return "加载中…"
	case PlayerBuffering:
		return "缓冲中 · 点击暂停"
	default:
		return ""
	}
}

// playerTip 是播放键这一帧的 tooltip / accessibility 名称：忙态优先，闲时
// 回到普通播放/暂停。主窗、迷你窗、视频窗都从这里取，避免各自再拼一遍。
func playerTip(playing bool, busy PlayerBusy) string {
	if busy != PlayerIdle {
		return playerBusyTip(busy)
	}
	return playTip(playing)
}

// 忙态圆环沿用 ringSpinner（原版 dialog-spin 的细环 + 顶弧，0.7s linear
// infinite），只换尺寸：播放键 42px 里用 18/2，迷你窗 28px 里用 16/2。
// 图标位本来就是固定尺寸的居中盒子，换成圆环不会改变播放键的几何。
const (
	playerBusyRingSize   float32 = 18
	playerBusyRingStroke float32 = 2
	miniBusyRingSize     float32 = 16
)

// PlayerBarBind 是一次播放栏渲染的绑定目标快照。主窗与视频窗每帧各建一份，
// 然后调用同一组 playerTransportItems / playerTimelineItems /
// playerSettingsItems。字段全是值或回调，方便无头测试直接构造。
type PlayerBarBind struct {
	Theme Theme
	Set   *PlayerSettings

	// 传输（各自独立，互不镜像）
	Playing bool
	// Busy 是播放键的忙态（见 PlayerBusy）。PlayerIdle 时播放键与以前完全
	// 一样；忙时只把图标换成细圆环、把文案换成「加载中… / 缓冲中 · 点击暂停」，
	// 点击仍然下发 OnPlay（缓冲中必须能暂停；加载中由控制器忽略，避免去
	// Resume 旧引擎）。
	Busy      PlayerBusy
	OnPlay    func()
	HasSrc    bool
	Pos       float64
	Dur       float64
	Buffered  float64
	SeekValue *float64
	OnFrame   func(PlayerTimelineInput) (commit bool, value float64)
	OnSeek    func(sec float64)
	// TimelineHandle 可选：绑定该窗口的进度条句柄（主窗 App.progressHandle /
	// 视频窗 VideoChrome.progressHandle）。弹幕热力用它取轨道真正的布局矩形，
	// 宽度才能与进度条一致。
	TimelineHandle *ui.Handle
	// Bubble 是悬停时间气泡的状态（每帧复用同一份，不能新建）。
	Bubble *seekBubbleState
	// Timeline 允许调用方换掉材质 / 颜色；零值按主窗播放栏。
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

	// OnPanel 表示这条控件条压在视频窗的 Panel 上（浅色面板，不是播放栏那层
	// 半透明玻璃）。它只影响一件事：**不**在图标下再画一层播放栏的白色
	// drop-shadow 副本 —— 面板上那层白副本会变成一圈白边，hover 时图标墨
	// 色在 140ms 里过渡、白副本不动，看起来就是「图标在闪」。一颗键只画
	// 一层图标（用户 2026-11 报的播放 / 全屏键悬浮闪烁）。
	OnPanel bool

	// Heat 是弹幕热力条（nil 或 On=false 不画）。数据来自**同一个** cid 缓存，
	// 绘制只有 paintHeatBackdrop 一份：主窗与视频窗都传这一项。
	Heat *PlayerHeatBind
	// FullScreen 是控件条最后那颗全屏键（只有视频窗有；nil = 没有这一颗）。
	FullScreen *PlayerFullScreenBind
}

// PlayerFullScreenBind 是「视频窗唯一多出来的那颗键」的绑定：
// 主窗不传，视频窗传自己的全屏状态与切换回调。键本身（材质 / 尺寸 /
// 图标档 / 悬停与按压手感 / 焦点环）全部走播放栏那套，不另画。
type PlayerFullScreenBind struct {
	Full     bool
	OnToggle func()
}

// PlayerHeatBind 是一条弹幕热力条的绑定。
//
// 主窗与视频窗用的是同一个 painter（playerHeatPainter → paintHeatBackdrop），
// 几何也只量一次：热力的横向范围 = 进度条**轨道**矩形，不是整条栏。
type PlayerHeatBind struct {
	On     bool
	Points []HeatPoint
	Dur    float64
	Dark   bool
	// Track 是这一帧进度条滑块的句柄：热力从它量出轨道矩形。
	Track *ui.Handle
	// Painted 在绘制回调里写入实际画出的矩形（测试拿它断言几何）。
	Painted *ui.Rect
	// Version 变化时请一次整窗重绘（0.7.0 的局部重绘可能漏掉 Draw 层）。
	// Seen 是上一次请求对应的版本。
	Version uint64
	Seen    *uint64
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
// 尺寸全部来自 playerBarKeyMetrics：主窗与视频窗是同一套（42 正圆 + 32）。
func playerTransportItems(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	width := m.Play
	if b.ShowNext {
		width += m.TransportGap + m.Key
	}
	ui.Row(c).Width(width).Shrink(0).Gap(m.TransportGap).AlignItems(ui.Center).Children(func() {
		tip := playerTip(b.Playing, b.Busy)
		play := ui.ButtonBase(c.Key("play")).Size(m.Play, m.Play).Radius(m.PlayR).Center().
			Label(tip).Tooltip(tip).Busy(b.Busy != PlayerIdle)
		disablePlayerButton(play, !b.HasSrc)
		playerPlaySurface(t, play, b.Playing, play.Hovered(), play.Pressed())
		playerPressPose(play)
		playerRim(t, play, m.PlayR, false, sheenKindPlay, b.Playing, play.Hovered())
		play.Children(func() {
			if b.Busy != PlayerIdle {
				// 一颗键里只放一个指示器：忙时是细圆环，闲时才是播放/暂停图标。
				// ringSpinner 的 Loop 在「减少动态效果」下照常转（框架的既有
				// 策略：还在转＝还在忙，见 reduce.go）。
				ringSpinner(c, t.Blue, playerBusyRingSize, playerBusyRingStroke,
					"player-busy", 0, 0, 0, 0)
				return
			}
			ic := pick(b.Playing, iconPause, iconPlayOne)
			ink := playerPlayInk(t, b.Playing, play.Hovered())
			icon := ui.Icon(c, ic).Size(playIconSize, playIconSize).
				TextColor(playerFadeColor(c, play, "player-play-ink", ink)).
				Opacity(playerDisabledIconOpacityFor(!b.HasSrc)).
				Translate(0.5, 0)
			if !b.OnPanel {
				// 玻璃播放栏才有这层白色压印；面板上它会让一颗键出现两层图标。
				playerIconShadowFn(icon, ic, 0, 1, playerPlayIconShadowInk)
			}
		})
		if play.Clicked() && b.OnPlay != nil {
			b.OnPlay()
		}

		if !b.ShowNext {
			return
		}
		nextText := b.NextTip
		if nextText == "" {
			nextText = nextTip()
		}
		// 传输键是正圆（原版 32×32 + border-radius: 999px）：圆角与描边同取
		// playerTransportRadius，键面与 rim 不会是两种形状。
		r := playerTransportRadius("next")
		next := ui.ButtonBase(c.Key("next")).Size(m.Key, m.Key).Radius(r).Center().
			Label(nextText).Tooltip(nextText)
		disablePlayerButton(next, !b.CanNext)
		playerToolSurface(c, t, toolNext, next, false, next.Hovered(), next.Pressed())
		playerPressPose(next)
		playerRim(t, next, r, true, sheenKindStatic, false, false)
		next.Children(func() {
			ink := playerButtonInk(t, next.Hovered())
			icon := ui.Icon(c, iconNext).Size(nextIconSize, nextIconSize).
				TextColor(playerFadeColor(c, next, "player-next-ink", ink)).
				Opacity(playerDisabledIconOpacityFor(!b.CanNext))
			if !b.OnPanel {
				playerIconShadowFn(icon, iconNext, 0, 1, playerIconShadowInk)
			}
		})
		if next.Clicked() && b.CanNext && b.OnNext != nil {
			b.OnNext()
		}
	})
}

// playerTimelineItems 画「当前时间 + 进度条 + 总时长」三列。主窗一行布局与
// 视频窗两行布局的第一行都调它；窄窗只收时间列，进度条始终在。时间列宽度与
// 字号由 playerBarKeyMetrics 给（56 / 11）—— 固定宽度 + 等宽数字，播放时
// 不左右跳。
func playerTimelineItems(c *ui.Context, b PlayerBarBind) {
	m := playerBarKeyMetrics
	if b.ShowPos {
		playerTimeText(c, b.Theme, b.Pos, m.TimeSize, m.TimeW)
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
		Handle:   b.TimelineHandle,
	})
	if b.ShowDur {
		playerTimeText(c, b.Theme, b.Dur, m.TimeSize, m.TimeW)
	}
}

// playerSettingsItems 画「设置类」键：音量 / 倍速 / 均衡 / 跳过广告，再在
// 最后接上全屏键（只有视频窗的绑定里有它）。音量与倍速两列的句柄回填给
// 调用方（弹层挂在列上）。这些键都读写 PlayerSettings，由各自窗口的监听把
// 值下发到引擎 —— 这就是「改一边另一边立刻反映」的接线点。
//
// 均衡键由 ShowEQ 控制：主窗传 true；视频窗**不传**（false），因为原生视频的
// PCM 在系统媒体框架内部就送去声卡，本仓库的均衡器拿不到那路采样 —— 一颗
// 永远不生效的键在控件条里就是死入口（用户 2026-11 裁决：直接去掉）。
// 共享的 EQ 值本身不动，主窗照常读写。
//
// 全屏键是**两个窗口控件集合的唯一差别**：视频窗 = 主窗 + 全屏 − 均衡。它
// 在同一个函数里、用同一套键材质画出来（见 playerFullScreenKey），主窗不传
// 绑定就没有它。
//
// volumeSlot / speedSlot 是**回填出口**（列元素，弹层挂在列上），所以是
// `*ui.Element` 而不是值：`go tool mygo migrate-ui` 会建议改成值，但这里要的
// 是「把这一帧建出来的列交回去」，传值拿不到。
func playerSettingsItems(c *ui.Context, b PlayerBarBind, volumeSlot, speedSlot *ui.Element) {
	m := playerBarKeyMetrics
	*volumeSlot = playerSlot(c, m.ToolSlot, m.Key, func() { playerVolumeKey(c, b) })
	*speedSlot = playerSlot(c, m.SpeedSlot, m.Key, func() { playerSpeedKey(c, b) })
	if b.ShowEQ {
		playerSlot(c, m.ToolSlot, m.Key, func() { playerEQKey(c, b) })
	}
	if b.ShowSponsor {
		playerSlot(c, m.SponsorSlot, m.Key, func() { playerSponsorKey(c, b) })
	}
	if b.FullScreen != nil {
		playerSlot(c, m.ToolSlot, m.Key, func() { playerFullScreenKey(c, b) })
	}
}

// playerFullScreenKey 是全屏键：视频窗控件条比主窗播放栏多的**唯一**一颗键。
//
// 形状 / 材质 / 悬停与按压手感 / 焦点环 / 图标档全部与主窗的工具键同一套
// （playerToolSurface + playerPressPose + playerRim）；全屏时换「退出全屏」
// 图标并给出激活态。图标取自应用那套 icon-park 定义（state.go 的
// iconFullScreen / iconExitFullScreen），不在视频窗里另画。
func playerFullScreenKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	fs := b.FullScreen
	ic, tip := iconFullScreen, "全屏"
	if fs.Full {
		ic, tip = iconExitFullScreen, "退出全屏"
	}
	btn := ui.ButtonBase(c.Key("fullscreen")).Size(m.Key, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleToggleButton).Label(tip).Tooltip(tip).Checked(fs.Full)
	playerToolSurface(c, t, toolFullScreen, btn, fs.Full, btn.Hovered(), btn.Pressed())
	playerPressPose(btn)
	playerRim(t, btn, m.KeyR, true, sheenKindStatic, fs.Full, btn.Hovered())
	btn.Children(func() {
		ink := playerToolInk(t, toolFullScreen, fs.Full, btn.Hovered())
		icon := ui.Icon(c, ic).Size(fullScreenIconSize, fullScreenIconSize).
			TextColor(playerFadeColor(c, btn, "player-fullscreen-ink", ink))
		if !b.OnPanel {
			playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
		}
	})
	if btn.Clicked() && fs.OnToggle != nil {
		fs.OnToggle()
	}
}

// playerVolumeKey 是音量列：点开音量弹层。与主窗播放栏同一颗键（尺寸 32、
// 圆角 Radius、图标 20）。
func playerVolumeKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	set := b.Set
	btn := ui.ButtonBase(c.Key("volume")).Size(m.Key, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleDisclosure).Label("音量").Tooltip("音量").Expanded(b.ShowVolume)
	disablePlayerButton(btn, !b.HasSrc)
	playerToolSurface(c, t, toolVolume, btn, b.ShowVolume, btn.Hovered(), btn.Pressed())
	playerPressPose(btn)
	playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	muted := set.Muted || set.Volume <= 0
	btn.Children(func() {
		ic := volumeIconFor(muted)
		ink := playerToolInk(t, toolVolume, b.ShowVolume, btn.Hovered())
		icon := ui.Icon(c, ic).Size(volumeIconSize, volumeIconSize).
			TextColor(playerFadeColor(c, btn, "player-volume-ink", ink)).
			Opacity(playerDisabledIconOpacityFor(!b.HasSrc))
		if !b.OnPanel {
			playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
		}
	})
	if btn.Clicked() && b.OnVolumeKey != nil {
		b.OnVolumeKey()
	}
}

// playerSpeedKey 是倍速列：显示当前倍速，点开档位菜单。宽度取令牌的
// SpeedW（文字值键放得下 1.25x），高度与圆角与图标键同档。
func playerSpeedKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	set := b.Set
	tip := speedTip(set.Speed)
	btn := ui.ButtonBase(c.Key("speed")).Size(m.SpeedW, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleDisclosure).Label(tip).Tooltip(tip).Expanded(b.ShowSpeed)
	disablePlayerButton(btn, !b.HasSrc)
	playerToolSurface(c, t, toolSpeed, btn, b.ShowSpeed, btn.Hovered(), btn.Pressed())
	playerPressPose(btn)
	playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	btn.Children(func() {
		ink := playerToolInk(t, toolSpeed, b.ShowSpeed, btn.Hovered())
		numericText(c, speedLabel(set.Speed)).FontSize(11).FontWeight(700).
			TextColor(playerFadeColor(c, btn, "player-speed-ink", ink))
	})
	if btn.Clicked() && b.OnSpeedKey != nil {
		b.OnSpeedKey()
	}
}

// playerEQKey 是第 7 列（34px）：音量均衡开关。只在主窗出现（视频窗的
// ShowEQ 为 false，见 playerSettingsItems）：这颗键改的是共享设置，由主窗的
// 监听落到 internal/media 的音频引擎。
func playerEQKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	set := b.Set
	tip := eqTip(set.EQ)
	on := set.EQ
	btn := ui.ButtonBase(c.Key("eq")).Size(m.Wide, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleToggleButton).Label(tip).Tooltip(tip).Checked(on)
	disablePlayerButton(btn, !b.HasSrc)
	playerToolSurface(c, t, toolEq, btn, on, btn.Hovered(), btn.Pressed())
	playerPressPose(btn)
	playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	btn.Children(func() {
		ink := playerToolInk(t, toolEq, on, btn.Hovered())
		icon := ui.Icon(c, iconEQ).Size(eqIconSize, eqIconSize).
			TextColor(playerFadeColor(c, btn, "player-eq-ink", ink)).
			Opacity(playerDisabledIconOpacityFor(!b.HasSrc))
		if !b.OnPanel {
			playerIconShadowFn(icon, iconEQ, 0, 1, playerIconShadowInk)
		}
	})
	if btn.Clicked() {
		set.SetEQ(!set.EQ)
	}
}

// playerSponsorKey 是第 8 列（40px）：跳过赞助片段开关。状态点与 tooltip
// 读共享设置里的 SponsorStatus / Count / Reason —— 视频窗与主窗同一份。
func playerSponsorKey(c *ui.Context, b PlayerBarBind) {
	t := b.Theme
	m := playerBarKeyMetrics
	set := b.Set
	btn := ui.ButtonBase(c.Key("sponsor")).Size(m.Wide, m.Key).Radius(m.KeyR).Center().
		Role(ui.RoleToggleButton).Label(sponsorTip()).Tooltip(sponsorTip()).Checked(set.Sponsor)
	playerToolSurface(c, t, toolSponsor, btn, set.Sponsor, false, false)
	playerPressPose(btn)
	playerRim(t, btn, m.KeyR, true, sheenKindStatic, false, false)
	btn.Children(func() {
		ink := playerToolInk(t, toolSponsor, set.Sponsor, false)
		icon := ui.Icon(c, iconSponsor).Size(eqIconSize, eqIconSize).
			TextColor(playerFadeColor(c, btn, "player-sponsor-ink", ink))
		if !b.OnPanel {
			playerIconShadowFn(icon, iconSponsor, 0, 1, playerIconShadowInk)
		}
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
