package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Theme 是 bili-FM 的视觉令牌，取自 src/styles/globals.css。
//
// 关键：原版没有「亮色/暗色开关」，而由时段驱动 —— 但**跟着时段走的只有
// 窗口背景和播放栏**：
//
//   - 背景：--liquid-backdrop，11 个时段各一条渐变（见 backdrop.go）
//   - 播放栏：#player 的 --player-* 令牌，深夜/黎明前/黄昏/傍晚/深夜
//     换成「烟熏玻璃」
//
// 其余令牌（--studio-*、--glass-*、--liquid-*）在 CSS 里只定义在 :root，
// 任何时段都不覆写。所以正文永远深色压在浅色渐变上，玻璃永远是同一种白。
type Theme struct {
	Period Period
	// Dark 为真表示处于烟熏玻璃时段。**只影响播放栏**。
	Dark bool

	// 文字（--studio-ink / muted / faint，时段无关）
	Ink   ui.Color
	Muted ui.Color
	Faint ui.Color

	// 线（--studio-line，时段无关）
	Line ui.Color

	// 玻璃（--glass-*，时段无关）
	Glass             ui.Color // --glass-bg
	GlassHover        ui.Color // --glass-bg-hover
	GlassActive       ui.Color // --glass-bg-active
	GlassBorder       ui.Color // --glass-border
	GlassBorderBright ui.Color // --glass-border-bright（hover 时）
	Highlight         ui.Color // --glass-highlight

	// 液态玻璃填充（--liquid-*，时段无关）
	LiquidBg       ui.Color // --liquid-bg
	LiquidBgHover  ui.Color // --liquid-bg-hover
	LiquidBgActive ui.Color // --liquid-bg-active

	// 面板（不透明表面，用于内容区）
	Panel ui.Color // --studio-panel
	Soft  ui.Color // --studio-soft

	// 强调色（时段无关）
	Blue ui.Color // --studio-blue
	Mint ui.Color // --studio-mint
	Rose ui.Color // --studio-rose

	// 播放栏：只有这一组按时段切（CSS 的 #player 覆盖块）。
	PlayerSurface ui.Color
	PlayerBorder  ui.Color
	PlayerButton  ui.Color
}

// darkPeriods 是背景较深、播放栏换烟熏玻璃的时段。
var darkPeriods = map[string]bool{
	"midnight":  true,
	"predawn":   true,
	"dusk":      true,
	"evening":   true,
	"latenight": true,
}

// shadowInk 是 CSS 里所有投影用的墨色（rgba(15, 23, 42, ...)）。
var shadowInk = ui.Hex("#0f172a")

// ThemeAt 返回某个时刻的视觉令牌。
func ThemeAt(t time.Time) Theme {
	return ThemeFor(PeriodAt(t.Hour()))
}

// ThemeFor 返回某个时段的视觉令牌。
func ThemeFor(p Period) Theme {
	t := Theme{
		Period: p,

		Ink:   ui.Hex("#1e293b"),
		Muted: ui.Hex("#475569"),
		Faint: ui.Hex("#64748b"),

		Line: ui.Hex("#64748b").Alpha(0.22),

		Glass:             ui.Hex("#ffffff").Alpha(0.24),
		GlassHover:        ui.Hex("#ffffff").Alpha(0.38),
		GlassActive:       ui.Hex("#ffffff").Alpha(0.16),
		GlassBorder:       ui.Hex("#ffffff").Alpha(0.55),
		GlassBorderBright: ui.Hex("#ffffff").Alpha(0.72),
		Highlight:         ui.Hex("#ffffff").Alpha(0.90),

		LiquidBg:       ui.Hex("#ffffff").Alpha(0.22),
		LiquidBgHover:  ui.Hex("#ffffff").Alpha(0.30),
		LiquidBgActive: ui.Hex("#ffffff").Alpha(0.16),

		Panel: ui.Hex("#ffffff").Alpha(0.88),
		Soft:  ui.Hex("#eef2f7"),

		Blue: ui.Hex("#0284c7"),
		Mint: ui.Hex("#0d9488"),
		Rose: ui.Hex("#e11d48"),

		// 浅色时段的播放栏就是 CSS 里 var(--player-*) 的兜底值。
		PlayerSurface: ui.Hex("#f1f8fd").Alpha(0.36),
		PlayerBorder:  ui.Hex("#ffffff").Alpha(0.72),
		PlayerButton:  ui.Hex("#334155").Alpha(0.72),
	}

	if darkPeriods[p.Name] {
		t.Dark = true
		// 只有 #player 的令牌变深（globals.css 的 smoked glass 块）。
		t.PlayerSurface = ui.Hex("#1e293b").Alpha(0.16)
		t.PlayerBorder = ui.Hex("#ffffff").Alpha(0.22)
		t.PlayerButton = ui.Hex("#334155").Alpha(0.84)
	}
	return t
}

// CoverPlaceholder 是封面还没加载出来时的底色。
func (t Theme) CoverPlaceholder() ui.Color {
	if t.Dark {
		return ui.Hex("#ffffff").Alpha(0.08)
	}
	return ui.Hex("#0f172a").Alpha(0.08)
}

// liquidGlass 给元素套上液态玻璃材质。
//
// 原版是 .liquid-glass：::before 半透明填充 + backdrop-filter 模糊，
// ::after 1px 聚光描边（conic-gradient 遮罩）。原生界面画不了
// backdrop-filter（见 PLAN.md 的取舍），这里用「半透明填充 + 1px 亮边 +
// 两层投影（--liquid-shadow）」近似。
func (t Theme) liquidGlass(e ui.Element) ui.Element {
	return e.
		Background(t.LiquidBg).
		Border(1, t.GlassBorder).
		Shadow(0, 8, 24, 0, shadowInk.Alpha(0.07)).
		Shadow(0, 1, 4, 0, shadowInk.Alpha(0.03))
}

// liquidGlassHover 是 hover 态的液态玻璃（背景变亮、描边变亮、投影抬高）。
func (t Theme) liquidGlassHover(e ui.Element) ui.Element {
	return e.
		Background(t.LiquidBgHover).
		Border(1, t.GlassBorderBright).
		Shadow(0, 16, 38, 0, shadowInk.Alpha(0.11)).
		Shadow(0, 2, 8, 0, shadowInk.Alpha(0.04))
}

// uiTheme 把我们的令牌装进 mygo 的控件主题，让输入框、滚动条、tooltip
// 这些现成控件也用同一套颜色。原版没有暗色主题（正文永远是深色），所以
// 这里固定 Dark = false。
func (t Theme) uiTheme() *ui.Theme {
	th := ui.LightTheme()
	th.Background = ui.Transparent // 背景由 Shell 自己画时段渐变
	th.Surface = ui.Hex("#ffffff").Alpha(0.80)
	th.SurfaceHover = ui.Hex("#ffffff").Alpha(0.92)
	th.SurfacePressed = t.LiquidBgActive
	th.Border = ui.Hex("#94a3b8").Alpha(0.35)
	th.Text = ui.Hex("#334155") // 原版输入框是 text-slate-700
	th.TextMuted = ui.Hex("#94a3b8")
	th.Accent = t.Blue
	th.AccentHover = ui.Hex("#0369a1")
	th.AccentPressed = ui.Hex("#075985")
	th.AccentText = ui.Hex("#ffffff")
	th.Danger = t.Rose
	th.Warning = ui.Hex("#d97706")
	th.Success = ui.Hex("#16a34a")
	th.Selection = t.Blue.Alpha(0.25)
	th.Focus = t.Blue.Alpha(0.55)
	th.Inverse = t.Ink
	th.InverseText = ui.Hex("#ffffff")
	// 原版滚动条：6px、rgba(100,116,139,0.35)。
	th.Scrollbar = ui.Hex("#64748b").Alpha(0.35)
	th.ScrollbarWidth = 6
	th.Radius = RadiusSmall
	return th
}

// 尺寸常量，取自原版的布局（titleBar 36px、mini 标题栏 24px、
// #video-info 固定 366px 等）。
const (
	TitleBarHeight = 36
	MiniTitleBar   = 24
	PlayerBarPad   = 10
	Radius         = 8
	RadiusSmall    = 6
	RadiusPill     = 999

	// 搜索药丸（.home-searchbar）：max-width 520、高 46、圆角 13、
	// 内边距 4；内腔按钮圆角 = 13 - 7 = 6。
	SearchBarWidth  = 520
	SearchBarHeight = 46
	SearchRadius    = 13
	SearchPadding   = 4
	SearchSlotH     = 38
	SearchSlotR     = 8
	ToolButton      = 32
	ToolButtonR     = 6
	AvatarSize      = 36
)
