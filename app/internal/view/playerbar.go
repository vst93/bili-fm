package view

import (
	"fmt"
	"image"
	"math"
	"sync"
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

// playerFadeAnim 是原版 `#player .player-button` 的
// `transition: color/background-color/border-color 140ms ease`：底色 / 描边 /
// 墨色此前都是瞬时切换（悬停时键面「啪」地换色），这里按原版补上 140ms
// 过渡。box-shadow 也带 140ms，但 mygo 的阴影没有逐通道动画通道，这轮只补
// 三条色值（内高光/投影随底色当帧切换，过渡期里差一帧级，远小于此前整面
// 跳色）。CSS `ease` = CubicBezier(0.25, 0.1, 0.25, 1)，与 navIconBgAnim 同源
// （时长不同：工具键族是 140ms，不是导航族的 180ms）。
var playerFadeAnim = ui.Animation{
	Duration: 140 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// playerFadeColor 返回这颗键这一帧该用的色值（带 140ms ease 过渡）。拆成
// var 便于测试：AnimateColor 在目标变化的那一帧返回**上一帧的颜色**
// （曲线 t=0），直接铺目标色会瞬时跳变——测试可换成「瞬时铺色」的桩做
// 反证。key 在同一颗键内须唯一。
var playerFadeColor = func(b ui.Element, key string, target ui.Color) ui.Color {
	return b.AnimateColor(key, target, playerFadeAnim)
}

// 烟熏玻璃（深色）时段对播放栏按钮的调参，逐值取自原版 globals.css 的
// `html:is([data-time-of-day="midnight"|"predawn"|"dusk"|"evening"|"latenight"])`
// 块。深色块只覆写**基态**：hover 规则特异度 (1,3,0)、:active (1,3,0)、
// [data-active]:hover (1,4,0) 都高过它的 (1,2,1)，所以在深色时段悬停 / 按下
// 仍按浅色值走；只有激活态（EQ / 恰饭 / 音量·倍速展开）原版另给了深色值。
const (
	playerDarkFillA         float32 = 0.085 // .player-button::before 白 .085
	playerDarkBorderA       float32 = 0.15  // border-color 白 .15
	playerDarkInsetA        float32 = 0.22  // inset 0 1px 0 白 .22
	playerDarkShadowA       float32 = 0.05  // 0 2px 7px rgba(15,23,42,.05)
	playerDarkActiveFillA   float32 = 0.16  // [data-active]/[data-open]::before rgba(125,211,252,.16)
	playerDarkActiveBorderA float32 = 0.42  // border-color rgba(125,211,252,.42)
	playerDarkActiveInsetA  float32 = 0.32  // inset 0 1px 0 白 .32
	playerDarkActiveGlowA   float32 = 0.12  // 0 2px 9px rgba(2,132,199,.12)
	playerPlayDarkFillA     float32 = 0.24  // .player-play-button::before rgba(224,242,254,.24)
	playerPlayDarkBorderA   float32 = 0.48  // border-color rgba(224,242,254,.48)
	playerPlayDarkInsetA    float32 = 0.46  // inset 0 1px 1px 白 .46
)

// playerToolSurface 是播放栏工具键（下一集/音量/倍速/均衡/恰饭）的材质，
// 对齐原版那条 tool 专用规则（`#player .player-next-button,
// #player .player-volume-button, #player .player-speed-button,
// #player .player-eq-button`）：这些键的玻璃边比通用 .player-button 更收敛
// （`rgba(148,163,184,.2)` 而不是 `var(--glass-border)` 白 .55），内高光也更淡
// （白 .44），否则一排工具键会亮成一圈白描边。三态颜色由 playerToolFill /
// playerToolBorder / playerToolInset 给出（纯函数，便于测试钉住）。
//
// 深色（烟熏玻璃）时段这条 tool 规则 (1,1,0) 反而输给深色基态 (1,2,1)，
// 所以基态换成「白 .15 边 + slate .84 墨 + 白 .085 底 + 白 .22 内高光」；
// 悬停 / 按下不受影响（特异度更高）。激活态另有一套深色值。
//
// 按下（pressed）时原版 :active 把整条 box-shadow 换成更深的内压
// （`inset 0 1px 3px rgba(15,23,42,.1)`）、底色转 `--liquid-bg-active`（白 .16），
// 描边沿用悬停/激活态（:active 不改 border-color）。
func playerToolSurface(t Theme, b ui.Element, active, hovered, pressed bool) ui.Element {
	if pressed {
		return applyInsetShadows(
			b.Background(playerPressFill()).Border(1, playerToolBorder(t, active, hovered)),
			playerPressShadowSpecs())
	}
	b = b.Background(playerFadeColor(b, "player-tool-bg", playerToolFill(t, active, hovered))).
		Border(1, playerFadeColor(b, "player-tool-border", playerToolBorder(t, active, hovered))).
		InsetShadow(0, 1, 1, 0, playerToolInset(t, active, hovered))
	if active {
		// 深色激活态原版整条 box-shadow 只有「顶内高光白 .32 + 外蓝 .12」，
		// 没有浅色那条 `inset 0 0 0 1px #bae6fd` 的蓝环。
		if t.Dark && !hovered {
			return b.Shadow(0, 2, 9, 0, ui.Hex("#0284c7").Alpha(playerDarkActiveGlowA))
		}
		return b.InsetShadow(0, 0, 0, 1, ui.Hex("#bae6fd").Alpha(0.70)).
			Shadow(0, 2, 8, 0, ui.Hex("#0284c7").Alpha(0.14))
	}
	if hovered {
		return b.Shadow(0, 3, 9, 0, ui.Hex("#0f172a").Alpha(0.06))
	}
	return b.Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(pick(t.Dark, playerDarkShadowA, 0.045)))
}

// playerToolFill 是工具键三态的底色：常态 --liquid-bg（白 .22）、悬停白 .34、
// 激活 rgba(186,230,253,.30)（原版 ::before 的 background-color）；深色时段
// 常态收成白 .085、激活转 rgba(125,211,252,.16)，悬停不变（hover 规则更特异）。
func playerToolFill(t Theme, active, hovered bool) ui.Color {
	switch {
	case active:
		if t.Dark {
			return ui.Hex("#7dd3fc").Alpha(playerDarkActiveFillA)
		}
		return ui.Hex("#bae6fd").Alpha(0.30)
	case hovered:
		return ui.Hex("#ffffff").Alpha(0.34)
	case t.Dark:
		return ui.Hex("#ffffff").Alpha(playerDarkFillA)
	}
	return ui.Hex("#ffffff").Alpha(0.22)
}

// playerToolBorder 是工具键三态的描边：常态 slate .2、悬停 slate .32、
// 激活 rgba(2,132,199,.28)；深色时段常态转白 .15、激活转 rgba(125,211,252,.42)。
func playerToolBorder(t Theme, active, hovered bool) ui.Color {
	switch {
	case active:
		if t.Dark {
			return ui.Hex("#7dd3fc").Alpha(playerDarkActiveBorderA)
		}
		return ui.Hex("#0284c7").Alpha(0.28)
	case hovered:
		return ui.Hex("#94a3b8").Alpha(0.32)
	case t.Dark:
		return ui.Hex("#ffffff").Alpha(playerDarkBorderA)
	}
	return ui.Hex("#94a3b8").Alpha(0.20)
}

// playerToolInset 是工具键的内高光（原版 tool 规则的白 .44，悬停抬到 .56）；
// 深色常态收成白 .22、深色激活抬到白 .32。
func playerToolInset(t Theme, active, hovered bool) ui.Color {
	switch {
	case hovered:
		return ui.Hex("#ffffff").Alpha(0.56)
	case t.Dark && active:
		return ui.Hex("#ffffff").Alpha(playerDarkActiveInsetA)
	case t.Dark:
		return ui.Hex("#ffffff").Alpha(playerDarkInsetA)
	}
	return ui.Hex("#ffffff").Alpha(0.44)
}

// playerToolInk 是工具键图标 / 文字的颜色：原版常态 #526174（下一集例外，
// 沿用 .player-button 的 #334155，因为它没被那条 tool 规则覆盖 color），
// 悬停与激活都是 --studio-blue #0284c7，激活且悬停时加深到 #0369a1。
// 深色时段常态换成 Theme.PlayerButton（rgba(51,65,85,.84)，那份令牌此前
// 没有任何使用点），悬停/激活不变（规则更特异）。
func playerToolInk(t Theme, active, hovered bool) ui.Color {
	switch {
	case active && hovered:
		return ui.Hex("#0369a1")
	case active || hovered:
		return ui.Hex("#0284c7")
	case t.Dark:
		return t.PlayerButton
	}
	return ui.Hex("#526174")
}

// playerButtonInk 是通用 `.player-button` 的墨色（下一集键用；它没被 tool 规则
// 覆盖 color）：常态 #334155、悬停 --studio-blue；深色常态走 Theme.PlayerButton。
func playerButtonInk(t Theme, hovered bool) ui.Color {
	switch {
	case hovered:
		return ui.Hex("#0284c7")
	case t.Dark:
		return t.PlayerButton
	}
	return ui.Hex("#334155")
}

// playerMuteSurface 是音量弹层里静音键（原版 `#player .player-volume-mute-button`）
// 的材质。这颗键虽然也叫 `.player-button`，原版却为它另开了一条规则：尺寸 26、
// 圆角 8、描边 slate .2、内高光白 .44（比通用 `:hover`/常态的白 .7 收敛）、
// 墨色 #526174。两处此前踩错：
//
//   - 它**不在**工具键的悬停名单（next / volume / speed / eq）里，所以原版
//     `:hover` 不换面——之前原生版误铺了 GlassHover，静音键一悬停就亮一块；
//   - `[data-active]`（已静音）只改墨色与描边为蓝调（studio-blue / 蓝 .28），
//     **不动底色**：底色始终是 `--liquid-bg` 白 .22（之前用了 Rose 底/墨，整键泛红）。
//
// `:active` 只吃通用 `.player-button:active { transform: scale(.93) }`（缩放由
// volumePopover 里的 playerPressPose 给），不换阴影也不换底——它不在 `:active`
// 改内压/底色的名单里。
// 改写成 var：测试可换成记录参数的桩，无头反证视图真的把这份材质接在静音键上
// （同 topToolInk 的手法）。
//
// 深色时段静音键也是 `.player-button`，基态换成烟熏玻璃（白 .085 底 / 白 .15 边 /
// 白 .22 内高光），而且 `[data-active]` (1,2,0) 输给深色基态 (1,2,1)——原版在
// 深色下已静音也**不转蓝**，只靠图标形状区分。
var playerMuteSurface = func(t Theme, b ui.Element, muted bool) ui.Element {
	if t.Dark {
		return b.Background(ui.Hex("#ffffff").Alpha(playerDarkFillA)).
			Border(1, ui.Hex("#ffffff").Alpha(playerDarkBorderA)).
			InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(playerDarkInsetA)).
			Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(playerDarkShadowA))
	}
	return b.Background(ui.Hex("#ffffff").Alpha(0.22)).
		Border(1, playerMuteBorder(t, muted)).
		InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.44)).
		Shadow(0, 2, 7, 0, ui.Hex("#0f172a").Alpha(0.045))
}

// playerMuteBorder 是静音键的描边：常态 slate .2、已静音 studio-blue .28；
// 深色常态转白 .15（`[data-active]` 在深色下不生效）。
func playerMuteBorder(t Theme, muted bool) ui.Color {
	if t.Dark {
		return ui.Hex("#ffffff").Alpha(playerDarkBorderA)
	}
	if muted {
		return ui.Hex("#0284c7").Alpha(0.28)
	}
	return ui.Hex("#94a3b8").Alpha(0.20)
}

// playerMuteInk 是静音键图标的墨色：常态 #526174、已静音 studio-blue；
// 深色常态走 Theme.PlayerButton（已静音不转蓝）。
func playerMuteInk(t Theme, muted bool) ui.Color {
	if t.Dark {
		return t.PlayerButton
	}
	if muted {
		return ui.Hex("#0284c7")
	}
	return ui.Hex("#526174")
}

// playerButtonPressAnim 是 `#player .player-button` 的按压位移参数（原版
// globals.css）：
//
//	transition: ... transform 120ms cubic-bezier(0.23, 1, 0.32, 1);
//
// 曲线与时长都与 `.nav-icon-btn` 不同（后者 160ms ease），也不是
// pressPose（120ms/EaseOut 的立方缓出）。抽成 var 便于测试逐值钉住。
var playerButtonPressAnim = ui.Animation{
	Duration: 120 * time.Millisecond,
	Ease:     ui.CubicBezier(0.23, 1, 0.32, 1),
}

// playerButtonPressPose 给播放栏按钮（`.player-button`，含迷你窗）算按下位移：
//
//	#player .player-button:active              { transform: scale(0.93) }
//	:active:not(:disabled) 的 play/next/volume/speed/eq 同上（只多一条内压阴影）
//
// 注意两点与 `.nav-icon-btn` 的 pressPose 不同：① 缩放是 **.93**（不是 .94）；
// ② 原版 `#player .player-button:hover` **没有 transform** —— 悬停不上浮，只有
// 按下才缩放。此前播放栏六颗键都错用了 pressPose，悬停凭空上浮 1px、按下缩放
// 也差一档。拆成纯函数便于测试直接钉住。
func playerButtonPressPose(pressed bool) ui.Pose {
	if pressed {
		return ui.Pose{Scale: 0.93}
	}
	return ui.Pose{}
}

// playerPressPose 把播放栏按钮的按压手感落到元素上（120ms + 原版贝塞尔曲线）。
// 与 navIconPose 同形（只读 Pressed/Hovered），但播放栏悬停不上浮。
func playerPressPose(b ui.Element) {
	b.AnimatePose(playerButtonPressPose(b.Pressed()), playerButtonPressAnim)
}

// playerPressFill 是 `#player .player-button:active::before` 的底色
// `--liquid-bg-active`（白 .16，逐值）。
func playerPressFill() ui.Color { return ui.Hex("#ffffff").Alpha(0.16) }

// playerPressShadowSpecs 是 `#player .player-button:active` 的那条内压阴影
// `inset 0 1px 3px rgba(15,23,42,.1)`（play/next/volume/speed/eq 逐字相同）。
// 抽成纯函数便于把值钉死（同 pressShadowSpecs）。
func playerPressShadowSpecs() []shadowSpec {
	return []shadowSpec{
		{0, 1, 3, 0, ui.Hex("#0f172a").Alpha(0.10)},
	}
}

// playerPlaySurface 是播放/暂停键的材质（原版 .player-play-button[data-playing]
// 与 :not([data-playing]) 两条 + 各自的 hover）。键是 42 正圆，播放中走蓝调、
// 暂停走中性灰蓝；三态颜色由 playerPlayFill / playerPlayBorder / playerPlayInk 给。
//
// 按下（pressed）时原版 :active 把整条 box-shadow 换成更深的内压
// （`inset 0 1px 3px rgba(15,23,42,.1)`）、底色转 `--liquid-bg-active`（白 .16）。
//
// 深色（烟熏玻璃）时段原版另给了一条 `#player .player-play-button` 深色基态
// （(1,2,1)，源序在 `[data-playing]` 的 (1,2,0) 之后）——播放与否在深色下
// **同一个面**（浅蓝填充 #e0f2fe .24 + 浅蓝边 .48 + #0369a1 墨），只有悬停
// （hover 规则 (1,3,0)）仍按浅色值走。
func playerPlaySurface(t Theme, b ui.Element, playing, hovered, pressed bool) ui.Element {
	if pressed {
		return applyInsetShadows(
			b.Background(playerPressFill()).Border(1, playerPlayBorder(t, playing, hovered)),
			playerPressShadowSpecs())
	}
	b = b.Background(playerPlayFill(t, playing, hovered)).
		Border(1, playerPlayBorder(t, playing, hovered))
	switch {
	case playing && hovered:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.88)).
			InsetShadow(0, -2, 5, 0, ui.Hex("#0284c7").Alpha(0.12)).
			Shadow(0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.06)).
			Shadow(0, 5, 14, 0, ui.Hex("#0284c7").Alpha(0.14))
	case hovered:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)).
			Shadow(0, 6, 16, 0, ui.Hex("#0284c7").Alpha(0.18))
	case t.Dark:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(playerPlayDarkInsetA)).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0284c7").Alpha(0.08)).
			Shadow(0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.10))
	case playing:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.82)).
			InsetShadow(0, -2, 4, 0, ui.Hex("#0284c7").Alpha(0.10)).
			Shadow(0, 4, 12, 0, ui.Hex("#0284c7").Alpha(0.10))
	default:
		return b.InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.76)).
			Shadow(0, 3, 10, 0, ui.Hex("#0f172a").Alpha(0.06))
	}
}

// playerPlayFill 是播放键三态的底色（原版 ::before 的 background-color）；
// 深色基态（无论是否播放）是 rgba(224,242,254,.24)。
func playerPlayFill(t Theme, playing, hovered bool) ui.Color {
	switch {
	case playing && hovered:
		return ui.Hex("#7dd3fc").Alpha(0.22)
	case hovered:
		return ui.Hex("#bae6fd").Alpha(0.50)
	case t.Dark:
		return ui.Hex("#e0f2fe").Alpha(playerPlayDarkFillA)
	case playing:
		return ui.Hex("#7dd3fc").Alpha(0.16)
	}
	return ui.Hex("#ffffff").Alpha(0.22)
}

// playerPlayBorder 是播放键三态的描边；深色基态是 rgba(224,242,254,.48)。
func playerPlayBorder(t Theme, playing, hovered bool) ui.Color {
	switch {
	case playing && hovered:
		return ui.Hex("#0284c7").Alpha(0.40)
	case hovered:
		return ui.Hex("#0284c7").Alpha(0.28)
	case t.Dark:
		return ui.Hex("#e0f2fe").Alpha(playerPlayDarkBorderA)
	case playing:
		return ui.Hex("#0284c7").Alpha(0.30)
	}
	return ui.Hex("#94a3b8").Alpha(0.22)
}

// playerPlayInk 是播放键图标的颜色（原版播放中 #0369a1、暂停 #475569、
// 暂停悬停转 #0284c7）；深色基态是 #0369a1（与播放中同色）。
func playerPlayInk(t Theme, playing, hovered bool) ui.Color {
	switch {
	case playing:
		return ui.Hex("#0369a1")
	case hovered:
		return ui.Hex("#0284c7")
	case t.Dark:
		return ui.Hex("#0369a1")
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
// 禁用 + opacity .42。mygo 的禁用节点会自己再乘 .5（ui/paint.go），所以元素值要写
// disabledDim(.42)=.84 抵消掉（见 disabled.go 的总说明）。
func disablePlayerButton(b ui.Element, off bool) ui.Element {
	b.Disabled(off)
	if off {
		b.Opacity(disabledDim(playerButtonDisabledOpacity))
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

// ---------------------------------------------------------------- 聚光描边 / 图标压印
//
// 原版每颗 `#player .player-button` 上还有两层此前原生版没画：
//
//	::after  1px 聚光描边（--liquid-rim 的白色 conic-gradient 蒙在 1px 边框上，
//	         光聚在上边、往下渐淡）+ --liquid-chromatic 色散边；
//	svg      filter: drop-shadow(0 1px 0 rgba(255,255,255,.7))（图标下方 1px 的
//	         白色压印；play 键是 drop-shadow(0 1px 1px rgba(255,255,255,.6))
//	         + transform: translateX(.5px)）。
//
// mygo 画不了 conic-gradient，也没有 svg 滤镜，这里各取一个等价近似：描边用
// 「上边最亮、左右次之、下边最淡」四条边带（圆角裁剪收尾）；图标压印用 paintFn
// 在下方 1px 先画一份白图标（paintFn 跑在真图标之前，白色副本被真图标盖住，
// 只从下缘露出约 1px 的白边，正是 drop-shadow 的效果）。

// playerRimOpacity 是聚光描边整圈的不透明度：工具键（下一集/音量/倍速/均衡/
// 恰饭）原版 `::after { opacity: .52 }`；播放键与静音键不吃这条覆写（1.0）；
// 烟熏玻璃（深色）时段原版把所有 `#player .player-button::after` 统一压到 .3
// （那条规则特异度 (1,2,2) 盖过 .52 的 (1,2,1)）。
func playerRimOpacity(dark, tool bool) float32 {
	switch {
	case dark:
		return 0.30
	case tool:
		return 0.52
	}
	return 1.0
}

// playerRimPaint 是一圈聚光描边的上/侧/下颜色与宽度。
//
// 原版 --liquid-rim 是 conic-gradient（最亮点落在按钮上缘），mygo 只有线性
// 渐变，用「上边最亮、左右次之、下边最淡」三段近似：上白 .85、侧白 .40→.10
// （竖直渐变）、下白 .08，再乘整圈 opacity。
type playerRimPaint struct {
	Top, SideTop, SideBottom, Bottom ui.Color
	Width                            float32
}

// playerRimPaintFor 由整圈 opacity 算出描边四边色。抽成纯函数便于测试钉值。
func playerRimPaintFor(opacity float32) playerRimPaint {
	return playerRimPaint{
		Top:        ui.Hex("#ffffff").Alpha(0.85 * opacity),
		SideTop:    ui.Hex("#ffffff").Alpha(0.40 * opacity),
		SideBottom: ui.Hex("#ffffff").Alpha(0.10 * opacity),
		Bottom:     ui.Hex("#ffffff").Alpha(0.08 * opacity),
		Width:      1,
	}
}

// paintPlayerRim 画一圈聚光描边：在 1px 边框之上，上边最亮、左右次之、
// 下边最淡。用四条实心/渐变边带（框架的 Fill/FillGradient 会吸附到设备像素，
// 覆盖完整，不像路径描边在半像素位置上被摊薄），四角由圆角裁剪自然收尾。
func paintPlayerRim(p *ui.Painter, r ui.Rect, radius, opacity float32) {
	if r.W < 2 || r.H < 2 {
		return
	}
	paint := playerRimPaintFor(opacity)
	w := paint.Width
	p.Clip(r, radius, func() {
		// 上边：光最聚处（原版 conic 的 0deg/360deg 白 1）。
		p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: w}, paint.Top, 0)
		// 左右：从靠近上边的 .40 竖直淡到下边的 .10。
		for _, x := range [2]float32{r.X, r.X + r.W - w} {
			p.FillGradient(ui.Rect{X: x, Y: r.Y, W: w, H: r.H}, ui.LinearGradient{
				From:  paint.SideTop,
				To:    paint.SideBottom,
				Angle: 180,
			}, 0)
		}
		// 下边：最淡。
		p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - w, W: r.W, H: w}, paint.Bottom, 0)
	})
}

// playerRimOpacityFn 是视图取整圈 opacity 的入口（测试可换桩关掉描边做反证）。
var playerRimOpacityFn = playerRimOpacity

// ---------------------------------------------------------------- 深色 ::before 的 --player-sheen
//
// 深色（烟熏玻璃）时段原版 `#player .player-button::before` 除了
// `background-color: rgba(255,255,255,.085)` 之外，还铺了一层
// `background-image: var(--player-sheen)`：
//
//	radial-gradient(120% 90% at 18% -20%, rgba(255,255,255,.2), transparent 48%),
//	linear-gradient(170deg, rgba(255,255,255,.055), transparent 56%)
//
// （`--player-sheen` 定义在深色 `#player` 令牌块里。浅色时段 `::before` 用的是
// 另一套 `--glass-sheen-static`，不在本轮范围。）原生版此前只画了那层纯色
// 白 .085，深色键面比原版平一档——本轮把这条光泽补上。
//
// mygo 没有径向渐变接口（见 bloom.go），所以整条 sheen 在这里按像素算成一张
// 白-透明位图，进程内只烘焙一次（渐变按盒比例定义，与按钮尺寸无关，画时拉伸
// 即可）。两条渐变都是白色、列表靠前的径向在上层，合成后仍是白色，只差 alpha。
const playerSheenSide = 128

var (
	playerSheenOnce sync.Once
	playerSheenBmp  *ui.Bitmap
)

// playerSheenBitmap 返回烘焙好的 sheen 位图（懒加载、进程内共用一张）。
func playerSheenBitmap() *ui.Bitmap {
	playerSheenOnce.Do(func() {
		playerSheenBmp = ui.NewBitmap(playerSheenPixels(playerSheenSide, playerSheenSide))
	})
	return playerSheenBmp
}

// playerSheenPixels 把 `--player-sheen` 算成一张 white-alpha 的 NRGBA 位图。
// 几何与 CSS 一致：径向是「椭圆尺寸 120%×90%、中心 (18%, -20%)、48% 处淡到
// 透明」；线性 170°（几乎朝下、略偏右）、0% 白 .055 → 56% 透明。
func playerSheenPixels(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return img
	}
	fw, fh := float32(w), float32(h)
	// 径向层。
	rcx, rcy := 0.18*fw, -0.20*fh
	rrx, rry := 1.20*fw, 0.90*fh
	// 线性层：方向向量与 mygo 的 LinearGradient 同约定（0° 朝上、90° 向右）。
	la := 170 * math.Pi / 180
	ldx, ldy := float32(math.Sin(la)), float32(-math.Cos(la))
	lhalf := (abs32(fw*ldx) + abs32(fh*ldy)) / 2
	lx0, ly0 := fw/2-ldx*lhalf, fh/2-ldy*lhalf
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			px, py := float32(x)+0.5, float32(y)+0.5
			// 线性层 alpha。
			lt := ((px-lx0)*ldx + (py-ly0)*ldy) / (2 * lhalf)
			lAlpha := float32(0)
			if lt < 0.56 {
				lAlpha = 0.055 * (1 - max(lt, 0)/0.56)
			}
			// 径向层 alpha。
			rdx, rdy := (px-rcx)/rrx, (py-rcy)/rry
			rd := float32(math.Hypot(float64(rdx), float64(rdy)))
			rAlpha := float32(0)
			if rd < 0.48 {
				rAlpha = 0.2 * (1 - rd/0.48)
			}
			// 径向在上、线性在下，两层都是白。
			a := rAlpha + lAlpha*(1-rAlpha)
			i := img.PixOffset(x, y)
			img.Pix[i+0], img.Pix[i+1], img.Pix[i+2] = 255, 255, 255
			img.Pix[i+3] = to8(a)
		}
	}
	return img
}

// playerSheenPaint 是视图画 sheen 的入口（测试可换桩关掉做反证）。
var playerSheenPaint = paintPlayerSheen

// paintPlayerSheen 把 sheen 位图裁到按钮圆角内、拉满整个矩形。
func paintPlayerSheen(p *ui.Painter, r ui.Rect, radius float32) {
	if r.W < 2 || r.H < 2 {
		return
	}
	p.Clip(r, radius, func() {
		p.Image(playerSheenBitmap(), r, ui.FillBox)
	})
}

// playerRim 把聚光描边（以及深色时段 `::before` 的 --player-sheen 光泽）接在
// 按钮上，画在 1px 边框之上。radius 必须与按钮自己的圆角一致，否则描边会离
// 边框错位。深色时先铺 sheen（纯色底之上、描边与图标之下），浅色时只画描边。
// 整圈 opacity 由 playerRimOpacityFn 按（dark, tool）给出。
func playerRim(b ui.Element, radius float32, dark, tool bool) ui.Element {
	opacity := playerRimOpacityFn(dark, tool)
	if opacity <= 0 && !dark {
		return b
	}
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		if dark {
			playerSheenPaint(p, r, radius)
		}
		if opacity > 0 {
			paintPlayerRim(p, r, radius, opacity)
		}
	})
}

// 图标压印的白色不透明度（原版 svg 的 drop-shadow 颜色）。
const (
	playerIconShadowInk     = 0.70 // #player .player-button svg
	playerPlayIconShadowInk = 0.60 // #player .player-play-button svg
)

// playerIconShadowFn 是视图给图标挂压印的入口（测试可换桩做反证）。
var playerIconShadowFn = playerIconShadow

// playerIconShadow 给图标补原版 `filter: drop-shadow(...)` 的白色压印：在真图标
// 画出来之前，先在 (dx, dy) 处画一份白图标（paintFn 跑在图标之前）。白色副本被
// 真图标盖住，只从下缘露出约 1px 的白边。paintFn 在元素自己的 world 变换下绘制，
// 所以图标自身的 Translate（play 键的 translateX(.5px)）会一并作用到副本上，
// dx/dy 只需表达 drop-shadow 的偏移（play 与其余键都是 (0, 1)）。
func playerIconShadow(b ui.Element, svg *ui.SVG, dx, dy, opacity float32) ui.Element {
	if opacity <= 0 || svg == nil {
		return b
	}
	return b.Draw(func(p *ui.Painter, r ui.Rect) {
		shadow := ui.Rect{X: r.X + dx, Y: r.Y + dy, W: r.W, H: r.H}
		p.Icon(svg, shadow, ui.Hex("#ffffff").Alpha(opacity))
	})
}

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
		// 按下手感交给 playerPressPose（原版 :active 是 scale(.93)、悬停不上浮）。
		// 三态（播放/悬停/常态）见 playerPlaySurface。
		playerPlaySurface(a.Theme, play, a.Playing, play.Hovered(), play.Pressed())
		playerPressPose(play)
		// 原版播放键的 ::after 不吃 .52 覆写（整圈 opacity 1）；深色时段压 .3。
		playerRim(play, RadiusPill, a.Theme.Dark, false)
		play.Children(func() {
			// 原版播放键图标 size 24（暂停态 #0369a1、暂停图 #475569，悬停转 #0284c7）。
			// 禁用时 svg 再乘 .34（原版 `:disabled svg`）。
			ic := pick(a.Playing, iconPause, iconPlay)
			icon := ui.Icon(c, ic).Size(playIconSize, playIconSize).
				TextColor(playerFadeColor(play, "player-play-ink", playerPlayInk(a.Theme, a.Playing, play.Hovered()))).
				Opacity(playerDisabledIconOpacityFor(!src)).
				// 原版 .player-play-button svg { transform: translateX(.5px) }。
				Translate(0.5, 0)
			playerIconShadowFn(icon, ic, 0, 1, playerPlayIconShadowInk)
		})
		if play.Clicked() && a.Act.TogglePlay != nil {
			a.Act.TogglePlay()
		}

		next := ui.ButtonBase(c.Key("next")).Size(32, 32).Radius(RadiusPill).Center().
			Label(nextTip()).Tooltip(nextTip())
		// 原版 .player-button:disabled 是 opacity .42（disablePlayerButton 会补上
		// mygo 禁用节点的减半）。
		disablePlayerButton(next, !a.canNavigate())
		playerToolSurface(a.Theme, next, false, next.Hovered(), next.Pressed())
		playerPressPose(next)
		// 下一集是工具键：原版 ::after opacity .52。
		playerRim(next, RadiusPill, a.Theme.Dark, true)
		next.Children(func() {
			// 原版下一集图标 size 18；颜色沿用 .player-button 的 #334155（未被
			// tool 规则覆盖），悬停转蓝；深色常态转 Theme.PlayerButton。
			icon := ui.Icon(c, iconNext).Size(nextIconSize, nextIconSize).
				TextColor(playerFadeColor(next, "player-next-ink", playerButtonInk(a.Theme, next.Hovered()))).
				Opacity(playerDisabledIconOpacityFor(!a.canNavigate()))
			playerIconShadowFn(icon, iconNext, 0, 1, playerIconShadowInk)
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
		TextColor(playerTimeInk(a.Theme)).Width(width).Shrink(0).Center()
}

// playerTimeInk 是 #player .player-time 的墨色，即原版
// `color: var(--player-time-color, #64748b)`：浅色时段该令牌未定义，落到兜底
// #64748b；烟熏玻璃时段定义为 rgba(51,65,85,.72)。拆成 var 便于测试换桩，
// 无头反证视图真的把按时段取的令牌送到两处时间文本（主窗 11px + mini 9px）。
var playerTimeInk = func(t Theme) ui.Color { return t.PlayerTime }

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
	s := rangeSkinFor(rp.Dark, rp.Hovered)
	paintRangeShadow(p, track, rp.Hovered, s)

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

	paintRangeFill(p, track, rp.Frac, rp.Colors, s)
	paintRangeThumb(p, rangeThumbX(track, rp.Geom, rp.Frac), r.Y+r.H/2, rp.Hovered, rp.Held, s)
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
		Dark:     a.Theme.Dark,
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
	playerToolSurface(a.Theme, b, a.ShowVolume, b.Hovered(), b.Pressed())
	playerPressPose(b)
	playerRim(b, Radius, a.Theme.Dark, true)
	// 原版：音量键图标随音量变化（0 视为静音）；颜色同工具键（#526174）。
	muted := a.Muted || a.Volume <= 0
	b.Children(func() {
		ic := pick(muted, iconMute, iconVolume)
		icon := ui.Icon(c, ic).Size(volumeIconSize, volumeIconSize).
			TextColor(playerFadeColor(b, "player-volume-ink", playerToolInk(a.Theme, a.ShowVolume, b.Hovered()))).
			Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
		playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
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
	playerToolSurface(a.Theme, b, a.ShowSpeed, b.Hovered(), b.Pressed())
	playerPressPose(b)
	// 倍速键是文字不是 svg，只吃聚光描边（不吃图标压印）。
	playerRim(b, Radius, a.Theme.Dark, true)
	b.Children(func() {
		// 原版 .player-speed-button：11px / 700 / tabular-nums / #526174。
		ui.Text(c, speedLabel(a.Speed)).FontSize(11).FontWeight(700).
			TextColor(playerFadeColor(b, "player-speed-ink", playerToolInk(a.Theme, a.ShowSpeed, b.Hovered())))
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
	playerToolSurface(a.Theme, b, a.EQ, b.Hovered(), b.Pressed())
	playerPressPose(b)
	playerRim(b, Radius, a.Theme.Dark, true)
	b.Children(func() {
		icon := ui.Icon(c, iconEQ).Size(eqIconSize, eqIconSize).
			TextColor(playerFadeColor(b, "player-eq-ink", playerToolInk(a.Theme, a.EQ, b.Hovered()))).
			Opacity(playerDisabledIconOpacityFor(!a.hasSource()))
		playerIconShadowFn(icon, iconEQ, 0, 1, playerIconShadowInk)
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
	playerToolSurface(a.Theme, b, a.Sponsor, b.Hovered(), b.Pressed())
	playerPressPose(b)
	// 恰饭开关是工具键：原版 ::after opacity .52。
	playerRim(b, Radius, a.Theme.Dark, true)
	b.Children(func() {
		icon := ui.Icon(c, iconSponsor).Size(eqIconSize, eqIconSize).
			TextColor(playerFadeColor(b, "player-sponsor-ink", playerToolInk(a.Theme, a.Sponsor, b.Hovered())))
		playerIconShadowFn(icon, iconSponsor, 0, 1, playerIconShadowInk)
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

// playerPopoverTint 是音量 / 倍速弹层的玻璃色（原版深色时段把两层弹层都从
// 浅白玻璃换成烟熏玻璃：`background-color: rgba(71,85,105,.64)`，边白 .2，
// `inset 0 1px 0 白 .26` + `0 10px 24px rgba(15,23,42,.14)`）。原生版用
// glass.Glass 的 Tint 染色（玻璃自己带 rim 高光），浅色时段不着色。
func playerPopoverTint(t Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#475569").Alpha(0.64)
	}
	return ui.Transparent
}

// playerSpeedOptionInk 是倍速弹层里档位文字的常态色：浅色 #475569，
// 深色时段 rgba(241,245,249,.78)（激活/悬停仍走 --studio-blue）。
func playerSpeedOptionInk(t Theme) ui.Color {
	if t.Dark {
		return ui.Hex("#f1f5f9").Alpha(0.78)
	}
	return ui.Hex("#475569")
}

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
		Material(glassLayer(glass.Glass{Tint: playerPopoverTint(t)})).
		Shadow(0, pick[float32](t.Dark, 10, 8), 24, 0, ui.Hex("#0f172a").Alpha(pick[float32](t.Dark, 0.14, 0.12))).
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
					TextColor(pick(active, t.Blue, playerSpeedOptionInk(t)))
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
		Material(glassLayer(glass.Glass{Tint: playerPopoverTint(t)})).
		Shadow(0, pick[float32](t.Dark, 10, 14), 40, 0, ui.Hex("#0f172a").Alpha(pick[float32](t.Dark, 0.14, 0.10))).Gap(volumePopoverGap).
		AlignItems(ui.Center)
	if panel.PressedOutside() {
		a.ShowVolume = false
	}
	panel.Children(func() {
		// 静音键：独立的切换（原版 player-volume-mute-button）。
		mute := ui.ButtonBase(c.Key("volume-mute")).Size(volumeMuteSize, volumeMuteSize).
			Radius(RadiusSmall).
			Center().Label(pick(a.Muted, "取消静音", "静音")).Tooltip(pick(a.Muted, "取消静音", "静音"))
		// 原版静音键材质：常态白 .22 底 + slate .2 描边 + 白 .44 内高光（悬停不换面），
		// 已静音只转蓝调墨色/描边（见 playerMuteSurface）。
		playerMuteSurface(a.Theme, mute, a.Muted)
		// 静音键也是 .player-button，原版通用 `:active { transform: scale(.93) }`
		// 对它生效（它不在 :active 改内压/底色的名单里，所以只缩放）。
		playerPressPose(mute)
		// 静音键不在 .52 名单里（继承 .player-button::after 的 1.0）。
		playerRim(mute, RadiusSmall, a.Theme.Dark, false)
		mute.Children(func() {
			ic := pick(a.Muted, iconMute, iconVolume)
			icon := ui.Icon(c, ic).Size(16, 16).
				TextColor(playerMuteInk(a.Theme, a.Muted))
			playerIconShadowFn(icon, ic, 0, 1, playerIconShadowInk)
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
				Dark:    a.Theme.Dark,
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
