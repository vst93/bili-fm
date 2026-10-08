package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Theme 是 bili-FM 的视觉令牌，取自 src/styles/globals.css 的 :root 与
// 各时段覆盖块。
//
// 关键差异：原版没有传统意义上的「亮色/暗色开关」，而是**由时段驱动**。
// 深夜 / 黎明前 / 黄昏 / 傍晚 / 深夜这几个时段的背景更深，玻璃令牌随之切到
// 「烟熏玻璃」（更低的填充不透明度、更亮的描边）。
//
// 注意：**文字颜色不随时段变**。原版的 --studio-ink 从不被时段覆盖，正文
// 始终是深色压在浅色渐变上；深色时段的 color 覆盖全部集中在 #player 里。
type Theme struct {
	Period Period
	// Dark 为真时使用烟熏玻璃令牌（对应 CSS 里
	// html:is([data-time-of-day="midnight"|"predawn"|"dusk"|"evening"|"latenight"]) 的规则）。
	Dark bool

	// 文字
	Ink   ui.Color // --studio-ink
	Muted ui.Color // --studio-muted
	Faint ui.Color

	// 线
	Line ui.Color // --studio-line

	// 玻璃
	Glass       ui.Color // --glass-bg
	GlassHover  ui.Color // --glass-bg-hover
	GlassActive ui.Color // --glass-bg-active
	GlassBorder ui.Color // --glass-border
	Highlight   ui.Color // --glass-highlight

	// 面板（不透明表面，用于内容区）
	Panel ui.Color // --studio-panel
	Soft  ui.Color // --studio-soft

	// 强调色
	Blue ui.Color // --studio-blue
	Mint ui.Color // --studio-mint
	Rose ui.Color // --studio-rose

	// 播放栏：深色时段用烟熏玻璃而不是不透明色带（CSS 的 #player 覆盖块）
	PlayerSurface ui.Color
	PlayerBorder  ui.Color
	PlayerButton  ui.Color
}

// darkPeriods 是背景较深、使用烟熏玻璃令牌的时段。
var darkPeriods = map[string]bool{
	"midnight":  true,
	"predawn":   true,
	"dusk":      true,
	"evening":   true,
	"latenight": true,
}

// ThemeAt 返回某个时刻的视觉令牌。
func ThemeAt(t time.Time) Theme {
	return ThemeFor(PeriodAt(t.Hour()))
}

// ThemeFor 返回某个时段的视觉令牌。
func ThemeFor(p Period) Theme {
	if darkPeriods[p.Name] {
		return Theme{
			Period: p,
			Dark:   true,

			// 文字在任何时段都是深色：原版 --studio-ink 从不被时段覆盖，
			// 深色时段变深的只有背景和玻璃，正文仍压在浅色渐变上。
			Ink:   ui.Hex("#1e293b"),
			Muted: ui.Hex("#475569"),
			Faint: ui.Hex("#64748b"),

			Line: ui.Hex("#475569").Alpha(0.28),

			Glass:       ui.Hex("#ffffff").Alpha(0.10),
			GlassHover:  ui.Hex("#ffffff").Alpha(0.17),
			GlassActive: ui.Hex("#ffffff").Alpha(0.07),
			GlassBorder: ui.Hex("#ffffff").Alpha(0.22),
			Highlight:   ui.Hex("#ffffff").Alpha(0.32),

			Panel: ui.Hex("#1e293b").Alpha(0.16),
			Soft:  ui.Hex("#334155").Alpha(0.30),

			Blue: ui.Hex("#38bdf8"),
			Mint: ui.Hex("#2dd4bf"),
			Rose: ui.Hex("#fb7185"),

			PlayerSurface: ui.Hex("#1e293b").Alpha(0.16),
			PlayerBorder:  ui.Hex("#ffffff").Alpha(0.22),
			PlayerButton:  ui.Hex("#334155").Alpha(0.84),
		}
	}
	return Theme{
		Period: p,
		Dark:   false,

		Ink:   ui.Hex("#1e293b"),
		Muted: ui.Hex("#475569"),
		Faint: ui.Hex("#64748b"),

		Line: ui.Hex("#64748b").Alpha(0.22),

		Glass:       ui.Hex("#ffffff").Alpha(0.24),
		GlassHover:  ui.Hex("#ffffff").Alpha(0.38),
		GlassActive: ui.Hex("#ffffff").Alpha(0.16),
		GlassBorder: ui.Hex("#ffffff").Alpha(0.55),
		Highlight:   ui.Hex("#ffffff").Alpha(0.90),

		Panel: ui.Hex("#ffffff").Alpha(0.88),
		Soft:  ui.Hex("#eef2f7"),

		Blue: ui.Hex("#0284c7"),
		Mint: ui.Hex("#0d9488"),
		Rose: ui.Hex("#e11d48"),

		PlayerSurface: ui.Hex("#ffffff").Alpha(0.42),
		PlayerBorder:  ui.Hex("#ffffff").Alpha(0.62),
		PlayerButton:  ui.Hex("#334155").Alpha(0.72),
	}
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
)
