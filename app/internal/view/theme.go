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
	// PlayerEdgeFade 是播放栏上沿那 10px 的淡出色（原版 --player-edge-fade）：
	// 浅色时段是浅蓝 24%，深色时段是灰蓝 11%。用 --player-surface 当淡出色
	// 会得到一条明显的灰带（两者不是一个用处）。
	PlayerEdgeFade ui.Color
	PlayerButton   ui.Color
	// PlayerTime 是播放栏时间标签的墨色（原版 --player-time-color）：
	// 浅色时段**没有**定义该令牌，落到规则里的兜底 #64748b；烟熏玻璃时段
	// 才覆写成 rgba(51,65,85,.72)。之前视图把深色值硬编码进两处时间文本，
	// 浅色时段的时间比原版暗一档。
	PlayerTime ui.Color

	// HighContrast 为真表示桌面要求更高对比（原版 CSS 的
	// `@media (prefers-contrast: more)`，mygo 的 ui.Preferences.HighContrast）。
	// 原版在 globals.css **末尾**有一条 `html:root #player { … }` 覆写块
	//（特异度 1,1,1，压过烟熏玻璃时段那条 1,1,0），只改播放栏的四个令牌。
	// 由 withHighContrast 叠加，**只影响播放栏**。
	HighContrast bool

	// ReducedTransparency 为真表示桌面要求「减少透明度、换成不透明表面」
	//（原版 CSS 的 `@media (prefers-reduced-transparency: reduce)`，对应
	// macOS 的 Reduce Transparency / Windows 关闭透明效果）。它由
	// withReduceTransparency 叠加：把播放栏那层 Liquid Glass 换成原版的
	// 不透明兜底面，并让键面 / 弹层的 `::before` 光泽整条撤掉。**只影响播放栏**。
	ReducedTransparency bool

	// 进度条 / 音量条那一对变量（原版 --player-track-border /
	// --player-track-rest）：亮色时段是白 .68 / 白 .32，烟熏玻璃时段收成
	// 白 .28 / 白 .20——轨道在深色玻璃上再铺原来那么亮就会糊成一条白带。
	PlayerTrackBorder ui.Color
	PlayerTrackRest   ui.Color

	// ---- 液态玻璃材质令牌（原生版）----
	//
	// 原版是 CSS 的 `background-color` + `backdrop-filter` + `box-shadow` +
	// `color` 四条声明；原生版没有 CSS，材质由 `liquidGlass`（liquidglass.go）
	// 用 mygo 的 `glass.Effect` 读 backdrop 绘制。这里只存**状态**：亮色、
	// 烟熏（播放栏深色时段）、亮色压在暗背景上，以及按背景算出的密度。
	//
	// 表面只选状态，不再各写一套色值 —— 与 CSS 令牌同一分工。
	GlassLight    glassState
	GlassDark     glassState
	GlassOverDark glassState
	// GlassDensity 是背景的视觉密度 0..1（亮→暗、平→花、灰→艳），把 tint
	// 从 Thin 推到 Dense。贴壁纸的大面板可再用 glassOpts.denseFloor 抬下限。
	GlassDensity float32
	// GlassBackgroundLum 是时段壁纸去极值后的平均亮度，用来选墨色：低于
	// 0.5 用浅墨，否则用现有深墨。
	GlassBackgroundLum float32
	// GlassInk / GlassInkMuted 是画在玻璃上的两级墨色（标题/正文 vs 次要）。
	// 原版的固定色阶：深墨 #1e293b / 次级 #475569；浅墨 #f5f5fa / #ebebf5。
	GlassInk      ui.Color
	GlassInkMuted ui.Color
	// GlassRowHover / GlassRowSelected 是玻璃上的行高亮：**更亮的一层玻璃**，
	// 不是灰洗。悬停白 .42、选中白 .76。
	GlassRowHover    ui.Color
	GlassRowSelected ui.Color
	// GlassSolid 是减少透明度 / 强制颜色时的不透明兜底面（同一种色调）。
	GlassSolid ui.Color
	// GlassTintScaleLight / Dark 是用户设置（0..1.5，1 为设计值）：分别缩放
	// 亮色表面与烟熏表面的 tint 与提亮。每个状态各记一份。
	GlassTintScaleLight float32
	GlassTintScaleDark  float32
	// GlassTheme 是用户选的表面主题：原有材质（经典）或新的液态玻璃明/暗。
	// 它是「表面」这一层的开关，不改布局、字号、圆角或文案。
	GlassTheme GlassTheme
}

// GlassTheme 是表面材质主题。Classic 保持产品原有的 CSS 复刻材质；
// GlassLight / GlassDark 是新的液态玻璃两档（同一套 glass.Effect，只换状态）。
type GlassTheme uint8

const (
	// GlassClassic 是原有材质：cssGlass（模糊 + 自铺底色）与 glass.Blur。
	GlassClassic GlassTheme = iota
	// GlassLight 是液态玻璃·明：白 6–22% tint，brightness 1.06，
	// saturate 180%，深墨。
	GlassLight
	// GlassDark 是液态玻璃·暗（烟熏）：rgb(20 20 24) 16–38% tint，
	// brightness 0.80，saturate 160%。产品的壁纸仍是亮色，所以墨色仍用
	// 深墨 —— 这与既有烟熏播放栏的取法一致，不是为了暗色场景翻白字。
	GlassDark
)

// String 是设置菜单里显示的名字。
func (g GlassTheme) String() string {
	switch g {
	case GlassLight:
		return "液态玻璃·明"
	case GlassDark:
		return "液态玻璃·暗"
	default:
		return "经典"
	}
}

// Next 循环到下一套主题。
func (g GlassTheme) Next() GlassTheme { return (g + 1) % 3 }

// classicGlass 报告是否走原有材质。
func (t Theme) classicGlass() bool { return t.GlassTheme == GlassClassic }

// glassIsDark 报告新材质是否用烟熏状态。经典主题下仍按原来的 t.Dark
// （只有播放栏在深色时段变烟熏）。
func (t Theme) glassIsDark() bool {
	if t.classicGlass() {
		return t.Dark
	}
	return t.GlassTheme == GlassDark
}

// surfaceDark 是「播放栏/弹层这一族表面该按哪套墨色与阴影画」。经典主题
// 跟时段（t.Dark）；新主题跟用户选的那套（明/暗），这样晚上选「明」不会
// 一半键面又变成烟熏色。
func (t Theme) surfaceDark() bool { return t.glassIsDark() }

// glassState 是一种玻璃状态的材质参数。
//
// Tint 是不带 alpha 的染色（白 / rgb(20 20 24) / rgb(16 18 22)），Thin 与
// Dense 是它在「背景稀」和「背景密」两端的 alpha；Brightness 与 Saturation
// 是 `backdrop-filter` 的 brightness()/saturate()，在 `glass.Effect` 的色调
// 曲线里精确等价（low=0, high=brightness, curve=1）。
type glassState struct {
	Tint        ui.Color
	Thin, Dense float32
	Brightness  float32
	Saturation  float32
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

// 标题栏窗口控制键组的「内凹玻璃托盘」（原版 #wds-handle-group，
// globals.css:1913-1922）：
//
//	background: rgba(255, 255, 255, 0.06);
//	border: 1px solid rgba(255, 255, 255, 0.18); border-top: 0;
//	box-shadow: inset 0 2px 4px rgba(15, 23, 42, 0.08),
//	            inset 0 -1px 0 rgba(255, 255, 255, 0.12);
//
// 注释写明「no top highlight line」：托盘是从窗顶挂下来的一个凹槽（上边无边框、
// 只有下面两个角跟着窗口圆角），不是一颗浮起的玻璃键。此前原生版只对齐了
// 组内几何（gap / px-1），**这层托盘整个没画** —— Windows / Linux 上两颗窗口键
// 就是直接压在标题栏白玻璃上，比原版少一层「压进表面」的深度。
var (
	tbTrayBg     = ui.Hex("#ffffff").Alpha(0.06)
	tbTrayBorder = ui.Hex("#ffffff").Alpha(0.18)
	tbTrayInk    = shadowInk.Alpha(0.08)
	tbTrayRim    = ui.Hex("#ffffff").Alpha(0.12)
)

// tbTrayRadius 返回托盘下沿的圆角：原版是 `border-radius: 0 0
// var(--app-window-radius) var(--app-window-radius)`，而 --app-window-radius
// 跟着平台走 —— Windows 对齐 DWM 的 8px，Linux 由 `body.platform-linux #root`
// 置 0（窗口管理器画不出外层圆角）。macOS 走原生红绿灯，不画这个托盘。
//
// 它是变量是为了让测试能换桩（测试进程的 runtime.GOOS 改不了）。
var tbTrayRadius = func() float32 {
	if viewGOOS == "windows" {
		return 8
	}
	return 0
}

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
		PlayerSurface:     ui.Hex("#f1f8fd").Alpha(0.36),
		PlayerBorder:      ui.Hex("#ffffff").Alpha(0.72),
		PlayerEdgeFade:    ui.Hex("#bae6fd").Alpha(0.24),
		PlayerButton:      ui.Hex("#334155").Alpha(0.72),
		PlayerTime:        ui.Hex("#64748b"),
		PlayerTrackBorder: ui.Hex("#ffffff").Alpha(0.68),
		PlayerTrackRest:   ui.Hex("#ffffff").Alpha(0.32),
	}

	// 液态玻璃：一次算出背景密度/亮度，三种状态共用。时段渐变是产品已知的
	// 背景，所以这里采样它（5×3，去两端极值），而不是每帧回读像素；同时段
	// 的所有表面共享同一份密度与墨色决定（原版「相邻表面共享一次判断」）。
	stats := glassStatsFor(p, t.Ink)
	t.GlassDensity = stats.density
	t.GlassBackgroundLum = stats.lum
	t.GlassLight = glassState{
		Tint: ui.Hex("#ffffff"), Thin: 0.06, Dense: 0.22,
		Brightness: 1.06, Saturation: 1.8 * stats.saturationKept,
	}
	// 烟熏档的上端收到 .26（规格的 38% 是给暗色场景的）：产品的壁纸
	// 最暗仍是中亮灰蓝，38% 会把 #1e293b 正文压到 3:1 以下。16% 保留
	// 既有烟熏播放栏的观感，.26 是「最暗时段仍过 3.5:1」的上限。
	t.GlassDark = glassState{
		Tint: ui.Hex("#141418"), Thin: 0.16, Dense: 0.22,
		Brightness: 0.80, Saturation: 1.6 * stats.saturationKept,
	}
	t.GlassOverDark = glassState{
		Tint: ui.Hex("#101216"), Thin: 0.10, Dense: 0.30,
		Brightness: 0.85, Saturation: 1.7 * stats.saturationKept,
	}
	// 玻璃墨色取原版规格的「黑 86%」一档。原生没有 mix-blend-mode，所以
	// 用不透明的近黑而不是 alpha，避免正文被背后的烟熏色抬亮。
	t.GlassInk = ui.Hex("#0a0a0a")
	t.GlassInkMuted = ui.Hex("#3c3c43")
	if stats.lum < 0.5 {
		// 原版只有「浅墨压在暗玻璃上」这一支；产品的时段壁纸都亮于 0.5，
		// 所以实际走不到这里，但保留正确的判据。
		t.GlassInk = ui.Hex("#f5f5fa")
		t.GlassInkMuted = ui.Hex("#ebebf5").Alpha(0.64)
	}
	t.GlassRowHover = ui.Hex("#ffffff").Alpha(0.42)
	t.GlassRowSelected = ui.Hex("#ffffff").Alpha(0.76)
	t.GlassSolid = ui.Hex("#f2f6fa")
	t.GlassTintScaleLight, t.GlassTintScaleDark = 1, 1
	t.GlassTheme = GlassClassic

	if darkPeriods[p.Name] {
		t.Dark = true
		// 只有 #player 的令牌变深（globals.css 的 smoked glass 块）。
		t.PlayerSurface = ui.Hex("#1e293b").Alpha(0.16)
		t.PlayerBorder = ui.Hex("#ffffff").Alpha(0.22)
		t.PlayerEdgeFade = ui.Hex("#475569").Alpha(0.11)
		t.PlayerButton = ui.Hex("#334155").Alpha(0.84)
		t.PlayerTime = ui.Hex("#334155").Alpha(0.72)
		t.PlayerTrackBorder = ui.Hex("#ffffff").Alpha(0.28)
		t.PlayerTrackRest = ui.Hex("#ffffff").Alpha(0.20)
	}
	return t
}

// withHighContrast 把桌面的高对比偏好叠在时段令牌之上（原版
// `@media (prefers-contrast: more)` 里的 `html:root #player` 块）：
//
//	html:root #player {
//	    --player-time-color: #334155;
//	    --player-border: rgba(51, 65, 85, 0.52);
//	    --player-track-border: rgba(51, 65, 85, 0.58);
//	    --player-track-rest: rgba(51, 65, 85, 0.3);
//	}
//
// 幂等：状态没变时原样返回；从高对比退回普通时按**时段**重新推导（HC 只
// 覆写播放栏那四个令牌，其余令牌与时段一一对应，所以 ThemeFor(Period) 就是
// 基态）。这条块在 CSS 里排在最末、又是 html:root + #player，所以浅色与
// 烟熏玻璃时段的高对比值完全一样（都覆盖成这一套不透明的 slate）。
func (t Theme) withHighContrast(hc bool) Theme {
	if t.HighContrast == hc {
		return t
	}
	if !hc {
		base := ThemeFor(t.Period)
		if t.ReducedTransparency {
			base = base.withReduceTransparency(true)
		}
		return base
	}
	t.HighContrast = true
	t.PlayerTime = ui.Hex("#334155")
	t.PlayerBorder = ui.Hex("#334155").Alpha(0.52)
	t.PlayerTrackBorder = ui.Hex("#334155").Alpha(0.58)
	t.PlayerTrackRest = ui.Hex("#334155").Alpha(0.30)
	return t
}

// withReduceTransparency 把桌面的「减少透明度」偏好叠在时段令牌之上（原版
// `@media (prefers-reduced-transparency: reduce)` 里的 `html:root #player` 块）：
//
//	html:root #player {
//	    --player-surface: #e6edf4;
//	    --player-sheen: none;
//	    --player-border: rgba(71, 85, 105, 0.28);
//	    --player-edge-fade: rgba(71, 85, 105, 0.08);
//	    --player-shadow:
//	        inset 0 1px 0 rgba(255, 255, 255, 0.72),
//	        0 -8px 20px rgba(15, 23, 42, 0.04);
//	}
//
// 这几条把播放栏从半透明玻璃换成一块不透明的 #e6edf4 面（`backdrop-filter:
// none`，见 playerBarMaterial），并把向上投影收紧成 8/20（见
// playerBarShadowSpec）。除这四个令牌外，键面填充（rgba(248,251,254,.88) / 激活
// #d9eef9）、弹层不透明底与 `background-image: none` 由渲染层读
// Theme.ReducedTransparency 分支给出。
//
// 幂等；退回普通时按**时段**重新推导、并保留高对比（两块的源序是 reduced-
// transparency 在前、contrast 在后，见 withPreferences）。
func (t Theme) withReduceTransparency(rt bool) Theme {
	if t.ReducedTransparency == rt {
		return t
	}
	if !rt {
		base := ThemeFor(t.Period)
		if t.HighContrast {
			base = base.withHighContrast(true)
		}
		return base
	}
	t.ReducedTransparency = true
	t.PlayerSurface = ui.Hex("#e6edf4")
	t.PlayerBorder = ui.Hex("#475569").Alpha(0.28)
	t.PlayerEdgeFade = ui.Hex("#475569").Alpha(0.08)
	return t
}

// withPreferences 从**时段基态**重新推导两个无障碍偏好（减少透明度 + 高对比）。
// 原版两条媒体查询的源序是 `prefers-reduced-transparency` 在前、
// `prefers-contrast` 在后，且都写 `html:root #player`：重叠的令牌（--player-border）
// 由高对比那条胜出。所以这里也先叠减少透明度、再叠高对比。
//
// 每次都从 ThemeFor(Period) 重建，所以调用方不必先单独撤掉某一个偏好，
// 两个布尔一起传就得到该时段下正确的合成主题。
func (t Theme) withPreferences(hc, rt bool) Theme {
	base := ThemeFor(t.Period)
	if rt {
		base = base.withReduceTransparency(true)
	}
	if hc {
		base = base.withHighContrast(true)
	}
	// withPreferences 从时段基态重建，会丢掉用户 tint / 表面主题；调用方
	// 紧接着用 withGlassTint / withGlassTheme 叠回（见 Shell / RefreshTheme）。
	base.GlassTintScaleLight, base.GlassTintScaleDark = t.GlassTintScaleLight, t.GlassTintScaleDark
	base.GlassTheme = t.GlassTheme
	return base
}

// withGlassTint 叠上用户设置的两个 tint 缩放（0..1.5，1 为设计值）。
// 亮色表面与烟熏表面各记一份；时段切换时同一份值继续生效。
func (t Theme) withGlassTint(light, dark float32) Theme {
	t.GlassTintScaleLight = min(max(light, 0), 1.5)
	t.GlassTintScaleDark = min(max(dark, 0), 1.5)
	return t
}

// withGlassTheme 叠上用户选的表面材质主题。
//
// 明/暗两套主题各自带一套播放栏墨色与轨道令牌：新主题不跟时段走（否则
// 晚上选「明」会一半键面又变回烟熏色）。经典主题不经过这段，行为不变。
func (t Theme) withGlassTheme(g GlassTheme) Theme {
	t.GlassTheme = g
	switch g {
	case GlassLight:
		if !t.ReducedTransparency {
			t.PlayerSurface = ui.Hex("#f1f8fd").Alpha(0.36)
		}
		t.PlayerBorder = ui.Hex("#ffffff").Alpha(0.72)
		t.PlayerEdgeFade = ui.Hex("#bae6fd").Alpha(0.24)
		t.PlayerButton = ui.Hex("#334155").Alpha(0.72)
		t.PlayerTime = ui.Hex("#64748b")
		t.PlayerTrackBorder = ui.Hex("#ffffff").Alpha(0.68)
		t.PlayerTrackRest = ui.Hex("#ffffff").Alpha(0.32)
		t.GlassInk = ui.Hex("#1e293b")
		t.GlassInkMuted = ui.Hex("#475569")
	case GlassDark:
		// 注意：产品的「暗」是烟熏玻璃压在**亮色时段壁纸**上，不是暗色
		// 场景。所以墨色仍是深墨（与既有烟熏播放栏一致）；把文字翻白会
		// 让浅壁纸上的对比掉到 2:1。真正的暗色场景需要连同壁纸一起换，
		// 不在这次「只换表面」的范围里。
		if !t.ReducedTransparency {
			t.PlayerSurface = ui.Hex("#141418").Alpha(0.22)
		}
		t.PlayerBorder = ui.Hex("#ffffff").Alpha(0.22)
		t.PlayerEdgeFade = ui.Hex("#475569").Alpha(0.11)
		t.PlayerButton = ui.Hex("#334155").Alpha(0.84)
		t.PlayerTime = ui.Hex("#334155").Alpha(0.72)
		t.PlayerTrackBorder = ui.Hex("#ffffff").Alpha(0.28)
		t.PlayerTrackRest = ui.Hex("#ffffff").Alpha(0.20)
		t.GlassRowHover = ui.Hex("#ffffff").Alpha(0.18)
		t.GlassRowSelected = ui.Hex("#ffffff").Alpha(0.28)
	}
	return t
}

// glassBodyInk / glassMenuInk 是玻璃上正文/菜单文字的墨色：经典主题保持
// 原有色值，新主题用 GlassInk（暗主题已在上面的 switch 里翻成浅墨）。
func (t Theme) glassBodyInk() ui.Color {
	if t.classicGlass() {
		return ui.Hex("#1e293b")
	}
	return t.GlassInk
}

func (t Theme) glassMenuInk() ui.Color {
	if t.classicGlass() {
		return ui.Hex("#334155")
	}
	return t.GlassInk
}

// glassStateFor 选一种材质状态：烟熏表面用 Dark；亮色表面看背景亮度，
// 低于 0.5 用「亮玻璃压在暗背景」那一支（浅墨）。
func (t Theme) glassStateFor(dark bool) glassState {
	if dark {
		return t.GlassDark
	}
	if t.GlassBackgroundLum < 0.5 {
		return t.GlassOverDark
	}
	return t.GlassLight
}

// glassTintScale 是某个状态此刻的用户缩放。
func (t Theme) glassTintScale(dark bool) float32 {
	if dark {
		return t.GlassTintScaleDark
	}
	return t.GlassTintScaleLight
}

// CoverPlaceholder 是封面还没加载出来时的底色。
func (t Theme) CoverPlaceholder() ui.Color {
	if t.Dark {
		return ui.Hex("#ffffff").Alpha(0.08)
	}
	return ui.Hex("#0f172a").Alpha(0.08)
}

// drawerPanelFill 是抽屉面板（`[data-slot="wrapper"] > section::before`）的底色。
//
// 经典（旧）主题的面板近似：原版是 --liquid-bg(白 .22) + blur(14px)。
// 早先为了阅读性用了 0.9 的兜底值，太实太白；后来收到 0.70。
// 新的液态玻璃主题不再用这块填充，改由 liquidGlass 的 tint + 边缘折射绘制
// （见 liquidglass.go），这里的值只服务经典主题。
func drawerPanelFill() ui.Color {
	return ui.Hex("#ffffff").Alpha(0.70)
}

// uiTheme 把我们的令牌装进 mygo 的控件主题，让输入框、滚动条、tooltip
// 这些现成控件也用同一套颜色。原版没有暗色主题（正文永远是深色），所以
// 这里固定 Dark = false。
func (t Theme) uiTheme() *ui.Theme {
	th := ui.LightTheme()
	th.Background = ui.Transparent // 背景由 Shell 自己画时段渐变
	// 三平台统一字体（见 font.go）：拉丁用内置 Inter，中文按 PingFang →
	// YaHei → Noto Sans CJK 的顺序回退。写成主题级，所有元素继承。
	th.Font = fontStack
	th.Surface = ui.Hex("#ffffff").Alpha(0.80)
	th.SurfaceHover = ui.Hex("#ffffff").Alpha(0.92)
	th.SurfacePressed = t.LiquidBgActive
	th.Border = ui.Hex("#94a3b8").Alpha(0.35)
	th.Text = ui.Hex("#334155") // 原版输入框是 text-slate-700
	th.TextMuted = ui.Hex("#94a3b8")
	if t.GlassTheme == GlassDark {
		// 暗主题的控件表面换成更薄的白层，但文字仍是深墨（同上：壁纸是亮的）。
		th.Surface = ui.Hex("#ffffff").Alpha(0.10)
		th.SurfaceHover = ui.Hex("#ffffff").Alpha(0.18)
	}
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
	// 原版滚动条：6px 宽、拇指 rgba(100,116,139,.3)。
	//
	// 原版 globals.css 里有**两条**声明：
	//
	//	* { scrollbar-color: rgba(100,116,139,.35) transparent }   ← Firefox 专用
	//	*::-webkit-scrollbar-thumb { background: rgba(100,116,139,.3) }  ← 真正生效
	//
	// Tauri 两端都是 WebView（Windows = WebView2/Chromium，Linux = WebKitGTK），
	// 都走 ::-webkit-scrollbar，所以用户看到的拇指是 .3。此前抄的是 Firefox 那条
	// .35，比原版实际渲染深 5% alpha。
	th.Scrollbar = ui.Hex("#64748b").Alpha(0.30)
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
	// titleBarFontWeight 是标题栏文字的字重。原版 `.app-title-bar` 自己声明
	// `font-weight: 500`，品牌 <span class="text-sm"> 与「设置」<button> 都
	// 继承它；原生版此前品牌写 600、设置钮默认 400，同字号下两边墨迹落点差
	// 1~1.5px（CJK 字形对字重更敏感），看上去就是「设置钮往下沉了」。
	// 见 shell.go 的 titleBar / settingsButton。
	titleBarFontWeight = 500
	// tbWinCtrlGap / tbWinCtrlPadX / tbWinCtrlPadY 是标题栏窗口控制键组（原版
	// #wds-handle-group）的间距与内边距：`flex items-center gap-1 px-1 py-0.5`
	// （main:src/components/titleBar.tsx:411）——gap-1 = 4px、px-1 = 4px、
	// py-0.5 = 2px（globals.css:1913 的 `padding: 2px 4px !important` 同值）。
	//
	// 托盘还有一条 1px 边框（上边为 0，见 tbTrayBorder），所以关闭键右缘离窗口
	// 右边 = 栏内边距 16 + 组边框 1 + px-1 4 = **21**（box-sizing: border-box：
	// 边框在组宽之内，把内容整体往左推 1px）。此前只算了 padding，写的是 20。
	tbWinCtrlGap  = 4
	tbWinCtrlPadX = 4
	tbWinCtrlPadY = 2
	// tbWinCtrlSize 是主窗窗口键的边长（原版 `.app-title-bar-btn` 30×30，
	// globals.css:1493；迷你模式收到 20，见 mini.go 的 miniWinCtrlSize）。
	tbWinCtrlSize = 30
	// PlayerBarHeight 是播放栏（#player）的高度。原版 .home-stage 是
	// `height: calc(100% - 56px)`，而 100% 基准 .app-main 会被 flex-grow 撑满
	// 整窗（见 homeStage），所以主区**只**减播放栏高度、不减标题栏 ——
	// 标题栏那 36px 实际由主区探到播放栏底下消化。见 homeStage 的说明。
	PlayerBarHeight = 56
	Radius          = 8
	RadiusSmall     = 6
	RadiusPill      = 999

	// 搜索药丸（.home-searchbar）：max-width 520、高 46、圆角 13、
	// 内边距 4；内腔按钮圆角 = 13 - 7 = 6。
	SearchBarWidth  = 520
	SearchBarHeight = 46
	SearchRadius    = 13
	SearchPadding   = 4
	// 内腔高 36 = 46 - 2(外框边框) - 8(外框 padding)；圆角 = 13 - 5 = 8。
	SearchSlotH = 36
	SearchSlotR = 8
	ToolButton  = 32
	ToolButtonR = 6
	AvatarSize  = 36

	// 搜索键内联 spinner（原版 .search-submit-spinner）：14×14、2px 描边，
	// 复用 dialog-spin 关键帧（0.7s linear infinite）。原生版此前用框架的
	// ui.Spinner（辐条式）且画成 18px，与原版「细环 + 顶弧」的 14px 不对。
	searchSubmitSpinnerSize = 14
	searchSubmitSpinnerRing = 2

	// 原版两组图标按钮（globals.css）：
	//   .video-content-actions .nav-icon-btn  34×34 圆角 8
	//   .video-context-dock .nav-icon-btn     34×34 圆角 7
	// 内容操作行（点赞/投币/收藏/浏览器/视频/弹幕）与工具条（搜索/选集/合集/
	// 播放列表）都比搜索栏里的 .top-tool-btn（32/6）大一圈。
	ContentIcon  = 34
	ContentIconR = 8
	DockIcon     = 34
	DockIconR    = 7

	// titleBarBrandWidth 是标题栏品牌的隐藏断点（原版 titleBar.tsx：windowWidth <= 600
	// 时整个 .title-bar-brand 不显示）。
	titleBarBrandWidth = 600
)

// iconOpticalSize 是内容操作行 / 工具条里图标的**光学尺寸**，逐值对应原版
// globals.css:2556 的一条统一覆写：
//
//	.video-content-actions .nav-icon-btn > svg,
//	.video-context-dock .nav-icon-btn > svg { width: 20px; height: 20px }
//
// JSX 里 source 写的是 18 / 20 两种 size，但 `size` 落到 SVG 的 width/height
// **表现属性**上，而 CSS 里表现属性的优先级最低 —— 原版 DOM 里这两组的 svg
// 实际一律 20px（原版注释：「统一图标光学尺寸：源码里混着 18/20 两种规格，
// 同排看有微妙的参差」）。cd94979 换 icon-park path 时按 JSX 把浏览器 / 视频 /
// 弹幕三枚改成了 18，把这条覆写漏掉了。
//
// 声明成变量便于测试换桩反证接线。
var iconOpticalSize float32 = 20

// navIconKind 区分三族 .nav-icon-btn 的**基础透明度**（原版 globals.css 各写一条）：
//
//	.nav-icon-btn                                opacity .92
//	.video-content-actions .nav-icon-btn         opacity .82
//	.video-context-dock .nav-icon-btn            opacity .78
//
// 内容行 / 工具条的覆盖规则与 `.nav-icon-btn:hover { opacity: 1 }` 同特异度
// （0,2,0）但源序在后，所以这两族的按钮**悬停时也不会回到 1**——是个常量；
// 禁用走 `.nav-icon-btn:disabled { opacity: .3 }`（源序最末、同特异度，赢），
// 由 disableNav 负责。
type navIconKind int

const (
	navIconGeneric navIconKind = iota // 通用 .nav-icon-btn（如合作 UP 主选择键）
	navIconContent                    // .video-content-actions .nav-icon-btn
	navIconDock                       // .video-context-dock .nav-icon-btn
)

// navIconRestOpacityFor 给出某族 `.nav-icon-btn` 的常态度。声明成变量便于测试
// 换成直通（1.0）做反证。
var navIconRestOpacityFor = func(k navIconKind) float32 {
	switch k {
	case navIconContent:
		return 0.82
	case navIconDock:
		return 0.78
	}
	return 0.92
}

// navIconStatValueOpacity 是 `.nav-stat-value`（按钮下方的计数）的透明度。原版
// 数值是 `.nav-icon-btn` 的子元素，随按钮一起吃到 `.video-content-actions
// .nav-icon-btn` 的 .82 与 `:disabled` 的 .34；原生版把数值画在按钮**之外**
// （34×(34+13) 的列里），所以要显式补上同一层。数值自己不是禁用节点，不吃框架
// 那层减半，所以直接给原版的绝对值 .34（按钮那边由 disabledDim 补到 .68）。
var navIconStatValueOpacity = func(off bool) float32 {
	if off {
		return navIconDisabledOpacity
	}
	return navIconRestOpacityFor(navIconContent)
}

// titleBarShowsBrand 报告某窗口宽度下标题栏是否显示品牌（logo + bili-FM + 设置）。
func titleBarShowsBrand(width float32) bool { return width > titleBarBrandWidth }

// minF 是 float32 的两值取小。
func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// titleBarButtonStyle 返回标题栏图标键（.app-title-bar-btn / #switch-window-mode）
// titleBarButtonStyle 返回标题栏图标键 / 迷你窗窗口控制键这一帧的视觉。
// 对应原版 CSS（globals.css）：
//
//	常态  background transparent; color --studio-muted; opacity .72
//	悬停  background-color --glass-bg-hover（白 .38）; color #0284c7; opacity 1
//	激活  （迷你窗图钉 aria-pressed=true）background-color --glass-bg-hover;
//	      color --studio-blue; opacity 1; box-shadow inset 0 1px 1px --glass-highlight
//	按下  background-color --glass-bg-active（白 .16）; transform scale(.94)
//
// 悬停与激活的底色都是白 .38、图标色都是 --studio-blue（#0284c7），所以两者
// 共用一套；激活额外叠内高光（原版 ::after 的 inset 高光）。
// 按下时底色换 --glass-bg-active（白 .16）：原版 `.mini-window-controls button:
// active:not(:disabled)` 用 `!important` 把背景从 hover 的 --glass-bg-hover 覆盖成
// --glass-bg-active，且 `[aria-pressed="true"]` 那条规则带 `:not(:active)` 排除。
func titleBarButtonStyle(t Theme, active, hovered, pressed, disabled bool) (bg, ink ui.Color, opacity float32, highlight bool) {
	if disabled {
		return ui.Transparent, t.Muted, 0.34, false
	}
	if active || hovered {
		if pressed {
			return t.GlassActive, t.Blue, 1, true
		}
		return t.GlassHover, t.Blue, 1, true
	}
	if pressed {
		return t.GlassActive, t.Muted, 0.72, true
	}
	return ui.Transparent, t.Muted, 0.72, false
}

// titleBarBtnPinSpecs 是「置顶」时**直接挂在按钮上**的单层内高光。原版：
//
//	body.mini-mode .mini-window-controls button[aria-pressed="true"]
//	    :not(:hover):not(:active):not(:focus-visible) {
//	    box-shadow: inset 0 1px 1px --glass-highlight;
//	}
//
// 这条 box-shadow 不在按钮的 `transition: color…, opacity…, transform…` 列表里，
// 所以它当帧到位（换目标不淡入淡出）。
func titleBarBtnPinSpecs() hoverHighlight {
	return hoverHighlight{Inset: []shadowSpec{{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}}}
}

// titleBarBtnAfterSpecs 是 ::after 那层三条阴影：
//
//	.app-title-bar button::after / .mini-window-controls button::after /
//	#switch-window-mode::after {
//	    box-shadow: inset 0 1px 1px --glass-highlight /
//	                inset 0 -1px 1px 墨 .03 / 0 2px 10px 墨 .045;
//	    opacity: 0; transition: opacity 0.18s ease;
//	}
//
// 与 .nav-icon-btn::after 逐字相同，直接复用 navHoverHighlightSpecs，不重抄。
func titleBarBtnAfterSpecs() hoverHighlight { return navHoverHighlightSpecs() }

// titleBarBtnHighlightAnim 是 ::after 的 `transition: opacity 0.18s ease`
// （CSS ease = cubic-bezier(.25,.1,.25,1)）。此前原生版把高光当帧铺上/撤下，
// 鼠标进出标题栏小键时那层内高光是「啪」地硬切，原版是 0.18s 淡入淡出。
var titleBarBtnHighlightAnim = ui.Animation{
	Duration: 180 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// titleBarBtnHighlightTarget 是 ::after 这一帧该达到的不透明度：
//
//	.app-title-bar button:hover::after { opacity: 1 }
//	.app-title-bar button:active::after { opacity: 0 }   ← 源里更靠后，按下且悬停时以 0 为准
//
// 其余状态（aria-pressed 的置顶、:focus-visible）都不改 ::after 的 opacity，
// 保持 0（置顶那层高光是直接阴影，见 titleBarBtnPinSpecs）。
func titleBarBtnHighlightTarget(pressed, hovered bool) float32 {
	if hovered && !pressed {
		return 1
	}
	return 0
}

// titleBarBtnHighlightFade 返回 ::after 这一帧的不透明度（带 0.18s 过渡）。
// 声明成 var 便于测试换桩（同 titleBarFade / titleBarBtnFade）。
var titleBarBtnHighlightFade = func(b ui.Element, target float32) float32 {
	return b.AnimateTo("tb-btn-hi", target, titleBarBtnHighlightAnim)
}

// scaleHighlight 把一组阴影的 alpha 统一乘以 op，用来模拟 ::after 整层的
// `opacity`（mygo 没有伪元素层级的不透明度，只能落到每条阴影的 alpha 上）。
func scaleHighlight(h hoverHighlight, op float32) hoverHighlight {
	if op >= 1 {
		return h
	}
	out := hoverHighlight{
		Inset: make([]shadowSpec, len(h.Inset)),
		Outer: make([]shadowSpec, len(h.Outer)),
	}
	for i, s := range h.Inset {
		s.C = s.C.Alpha(op)
		out.Inset[i] = s
	}
	for i, s := range h.Outer {
		s.C = s.C.Alpha(op)
		out.Outer[i] = s
	}
	return out
}

// titleBarBtnHighlight 铺标题栏图标键 / 迷你窗窗口控制键这一帧的高光，分两层：
//
//	置顶且不在悬停/按下/聚焦   按钮上的单层内高光（当帧到位）
//	::after                    三条阴影，整层不透明度走 0.18s 过渡
//
// var 便于无头测试换成 no-op 做反证（同 navPressShadow）。
var titleBarBtnHighlight = func(b ui.Element, active, hovered bool) ui.Element {
	pressed, focus := b.Pressed(), b.FocusVisible()
	// 置顶那条 `:not(:hover):not(:active):not(:focus-visible)` 例外：直接阴影。
	if active && !hovered && !pressed && !focus {
		b = applyHighlight(b, titleBarBtnPinSpecs())
	}
	// ::after 整层：三条阴影 × 过渡中的不透明度。op 归零后不再画任何东西。
	if op := titleBarBtnHighlightFade(b, titleBarBtnHighlightTarget(pressed, hovered)); op > 0 {
		b = applyHighlight(b, scaleHighlight(titleBarBtnAfterSpecs(), op))
	}
	return b
}

// tbIncognitoRestInk 是隐身键**开启**时其所在行（标题栏 actions 行）在静息态
// 该有的不透明度下限。
//
// 背景：原版 `.app-title-bar > *` 静息只到 .32、唤出才到 .82（见 titleBarReveal），
// 原生版一字不差照做。但这一行现在**只装常驻隐身键**（迷你入口已删除），于是
// `.32` 唯一的可见后果就是把「隐身开着」的状态压得几乎融进标题栏——无头实测
// 键内最暗像素亮度和：关闭态 592/720、开启态 560/720（只差 32），键内仅 5.6%
// 的像素在两态间可辨。用户明确要求这颗键的状态「一眼可见」，.32 达不到。
//
// 取舍：只对**开启**态加下限（关闭态照旧 .32/退到背景里是多少就是多少），
// 行本身仍走 titleBarFade 的 0.24s 过渡（动效不变，只是静息目标从 .32 抬到
// tbIncognitoRestInk）；唤出态是 .82，本来就高于这个下限，不受影响。
const tbIncognitoRestInk = float32(0.62)

// glassBlurSigma 是 `--glass-blur: blur(20px) saturate(1.45)`（globals.css:959）
// 里的 20，交给 glass.Blur.Radius（它的文档说明这就是 CSS blur() 的那个值，
// 即高斯标准差）。框架的玻璃材质没有 saturate 这一项，其余按原版照抄。
const glassBlurSigma float32 = 20

// paintGlassPanelFill 把 `--glass-bg`（白 .24）铺在面板边框的内缘。
//
// 用在「原版 background 是 --glass-bg、backdrop-filter 是 --glass-blur」的面板
// （toast / 设置下拉 / 合作 UP 主下拉）：它们的材质是 glass.Blur（只模糊、不
// 染色），底色必须自己补 —— 用 glass.Glass{}（Regular）会让面板比原版白一档
// （实测 +21 灰阶，原版白 .24 只值 +6）。inset 1px 是为了不盖住面板自己的
// `1px var(--glass-border)`：Draw 在 background/border 之后跑。
func paintGlassPanelFill(p *ui.Painter, r ui.Rect, radius float32, c ui.Color) {
	if c.A == 0 {
		return
	}
	p.Fill(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2}, c, max(radius-1, 0))
}

// titleBarReveal 是标题栏的 hover-to-reveal 两态（原版 .app-title-bar 的
// `:hover` / `:focus-within`），返回四层的不透明度：
//
//	child  直接子元素（`.app-title-bar > *`）        常态 .32  → 唤出 .82
//	brand  `.title-bar-brand`                        常态 .70  → 唤出 1
//	edge   `.app-title-bar::before` 顶沿投影         常态 .52  → 唤出 .78
//	wash   `.app-title-bar::after` 白玻璃渐隐        常态 0    → 唤出 1
//
// 原版把标题栏做成「静息时退到背景里、鼠标进来才浮现」：不碰它的时候整条只剩
// 三成不透明度（品牌七成），鼠标落上去、或里面的控件拿到焦点，才亮起并浮出一层
// 白玻璃与顶沿投影。
func titleBarReveal(active bool) (child, brand, edge, wash float32) {
	if active {
		return 0.82, 1, 0.78, 1
	}
	return 0.32, 0.7, 0.52, 0
}

// titleBarRevealAnim 是 hover-to-reveal 的过渡参数：原版 `.app-title-bar::after`
// 与 `.app-title-bar > *`（含品牌）都是 `transition: opacity 0.24s ease`，
// 240ms + CSS 默认缓动曲线。此前原生版把四层的不透明度当帧直接铺上/撤下，
// 鼠标进出标题栏是「啪」地硬切，原版是 0.24s 淡入淡出。
var titleBarRevealAnim = ui.Animation{
	Duration: 240 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// titleBarFade 返回标题栏某一层这一帧该用的不透明度（带 0.24s 过渡）。
// 声明成 var 便于测试换桩，统计四层是否都真的走了过渡。
var titleBarFade = func(el ui.Element, key string, target float32) float32 {
	return el.AnimateTo(key, target, titleBarRevealAnim)
}

// titleBarBtnFade 把标题栏 / 迷你窗小键的底色、图标色与不透明度交给框架过渡：
//
//	.app-title-bar button, .mini-window-controls button {
//	    transition: color 0.18s ease, background-color 0.18s ease,
//	                opacity 0.16s ease, transform 0.16s ease;
//	}
//
// 与 nav 小键同一组时长（navIconBgAnim / navIconFadeAnim）。reduce 为真时当帧
// 到位——原版那条 `transition: none` 只写在 `.mini-window-controls button` 上
// （迷你窗里的两个键），标题栏里的「设置」「切换到迷你模式」仍走过渡。
// 声明成 var 便于测试换桩，统计「哪几处把 reduce 传成了真」。
var titleBarBtnFade = func(b ui.Element, bg, ink ui.Color, opacity float32, reduce bool) (ui.Color, ui.Color, float32) {
	if reduce {
		return bg, ink, opacity
	}
	return b.AnimateColor("tb-btn-bg", bg, navIconBgAnim),
		b.AnimateColor("tb-btn-ink", ink, navIconBgAnim),
		b.AnimateTo("tb-btn-fade", opacity, navIconFadeAnim)
}

// gradStop 是一条多停靠点线性渐变的停靠点（位置 0..1 + 颜色）。原版 CSS 里这类
// 渐变有 4~9 个停靠点；v0.5.5 起 `ui.LinearGradient.Stops` 原生支持（一条渐变
// 最多 32 个停靠点），所以**直接用真停靠点**：转成 `[]ui.GradientStop` 即可，
// 与 CSS 的 `linear-gradient(Adeg, c1 p1%, …)` 逐值等价。
//
// 不要退回「按 n 条等高盒子拼」的旧写法（`gradBands`）：那等于把渐变线当成水平的、
// 逐带重新投影，斜向（135°）渐变的误差实测最坏 **38 灰阶**（见 loginsheen_test.go
// 的 TestLoginSheenMatchesCSS）。`gradBands` 已挪进 `gradbands_test.go`（只剩反证
// 在用），生产代码只留 `gradStops` 与 `gradAt`（按列采样的调用点仍用后者）。
type gradStop struct {
	At float32
	C  ui.Color
}

// gradStops 把 []gradStop 转成框架的多停靠点渐变停靠点（CSS 的 `c p%`）。
func gradStops(stops []gradStop) []ui.GradientStop {
	out := make([]ui.GradientStop, len(stops))
	for i, s := range stops {
		out[i] = ui.GradientStop{Color: s.C, At: s.At}
	}
	return out
}

// gradAt 在停靠点之间线性插值采样，与 CSS 的多停靠点线性渐变一致。
func gradAt(stops []gradStop, t float32) ui.Color {
	if len(stops) == 0 {
		return ui.Transparent
	}
	if t <= stops[0].At {
		return stops[0].C
	}
	last := stops[len(stops)-1]
	if t >= last.At {
		return last.C
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].At {
			a, b := stops[i-1], stops[i]
			if span := b.At - a.At; span > 0 {
				return a.C.Mix(b.C, (t-a.At)/span)
			}
			return b.C
		}
	}
	return last.C
}

// 原版 .app-title-bar::before / ::after 的多停靠点渐变（globals.css）。两层都
// 比标题栏本身高（170% / 140%），往下淡出到全透明；整层的 opacity 由
// titleBarReveal 给。
var (
	titleBarEdgeStops = []gradStop{
		{0, ui.Hex("#0f172a").Alpha(0.10)},
		{0.42, ui.Hex("#0f172a").Alpha(0.045)},
		{0.72, ui.Hex("#0f172a").Alpha(0.012)},
		{1, ui.Hex("#0f172a").Alpha(0)},
	}
	titleBarWashStops = []gradStop{
		{0, ui.Hex("#ffffff").Alpha(0.35)},
		{0.30, ui.Hex("#ffffff").Alpha(0.22)},
		{0.60, ui.Hex("#ffffff").Alpha(0.10)},
		{0.85, ui.Hex("#ffffff").Alpha(0.02)},
		{1, ui.Hex("#ffffff").Alpha(0)},
	}
)

// pressScale 是 .app-title-bar-btn / #switch-window-mode 的按压手感：只有按下
// 缩到 0.94，悬停不上浮（与 .nav-icon-btn 的 pressPose 不同，后者悬停会上浮 1px）。
func pressScale(b ui.Element) {
	if b.Pressed() {
		b.AnimatePose(ui.Pose{Scale: 0.94}, ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut})
	}
}

// pressPose 给按钮加上原版那套按压手感（.nav-icon-btn / .top-tool-btn）：
// 悬停上浮 1px、按下缩到 0.94。用 AnimatePose —— 只重绘不重建视图，
// 比在视图里算值便宜得多，所以这类微动效可以铺开用。
func pressPose(b ui.Element) {
	pressPoseScale(b, 0.94)
}

// pressPoseScale 同 pressPose，但按下缩放可指定。原版各按钮的 :active 缩放
// 并不统一（.nav-icon-btn / .nav-stat-btn 是 .94，.top-tool-btn /
// .search-submit-btn 是 .92）。
func pressPoseScale(b ui.Element, scale float32) {
	pose := ui.Pose{}
	switch {
	case b.Pressed():
		pose = ui.Pose{Scale: scale}
	case b.Hovered():
		pose = ui.Pose{Y: -1}
	}
	b.AnimatePose(pose, ui.Animation{Duration: 120 * time.Millisecond, Ease: ui.EaseOut})
}

// navIconAnim 是 `.nav-icon-btn` 的**自身过渡参数**：
//
//	transition: background-color 0.18s ease, transform 0.16s ease, opacity 0.16s ease
//
// 位移这一路是 160ms、CSS 默认缓动曲线 cubic-bezier(.25,.1,.25,1)。声明成
// var 便于测试逐值钉住（与 pressPose 的 120ms/EaseOut 不同 —— 那是别的按钮族）。
var navIconAnim = ui.Animation{
	Duration: 160 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// navIconBgAnim / navIconFadeAnim 是 `.nav-icon-btn` 另外两条过渡：底色 180ms、
// 不透明度 160ms，都是 CSS 默认缓动。此前原生版底色与不透明度都是瞬时切换
// （悬停时底色「啪」地跳出来），这里按原版补上。
var (
	navIconBgAnim = ui.Animation{
		Duration: 180 * time.Millisecond,
		Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
	}
	navIconFadeAnim = ui.Animation{
		Duration: 160 * time.Millisecond,
		Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
	}
)

// navIconBgColor 返回按钮这一帧该铺的底色（带 `background-color 0.18s ease`
// 过渡）。拆成纯函数便于测试：AnimateColor 在目标变化的那一帧返回**上一帧的
// 颜色**（曲线 t=0），而直接铺目标色会瞬时跳变。
func navIconBgColor(b ui.Element, target ui.Color) ui.Color {
	return b.AnimateColor("nav-icon-bg", target, navIconBgAnim)
}

// navIconBgTo 把 `.nav-icon-btn` 的底色过渡落到按钮上。target 是本帧该走的
// 底色（常/悬停/按下由调用方用 pressFeedback 算好）。声明成 var 便于测试换成
// 「直接铺色」的桩做反证。
var navIconBgTo = func(b ui.Element, target ui.Color) ui.Element {
	return b.Background(navIconBgColor(b, target))
}

// navIconFadeValue 返回按钮这一帧该用的不透明度（带 `opacity 0.16s ease`
// 过渡），与底色同理拆出纯函数便于测试。
func navIconFadeValue(b ui.Element, target float32) float32 {
	return b.AnimateTo("nav-icon-opacity", target, navIconFadeAnim)
}

// navIconFade 是 `.nav-icon-btn` 的不透明度过渡：目标由常态度与是否禁用决定
// （禁用的 .34 经 disabledDim 抵消框架对禁用节点的减半，见 disabled.go）。
// 必须在 `disableNav` 之后调用，末次写入生效。
var navIconFade = func(b ui.Element, off bool, rest float32) ui.Element {
	return b.Opacity(navIconFadeValue(b, navIconOpacityTarget(off, rest)))
}

// navIconPoseFor 是 `.nav-icon-btn` 三态位移的**纯函数**（原版 globals.css）：
//
//	.nav-icon-btn:hover          transform: translateY(-1px)
//	.nav-icon-btn:active         transform: scale(.94)
//	.nav-icon-btn:disabled:hover transform: none
//
// 按下优先：原版 `:active` 规则源序在 `:hover` 之后，transform 被整条替换，
// 所以按下**只有缩放、没有上浮**。禁用态的「不上浮」不在这里判 —— mygo 的
// Hovered/Pressed 对禁用元素本就返回 false（等价 `:disabled:hover`），只要
// 调用方在 Disabled 之后（或同帧稍后）调用即可。
func navIconPoseFor(pressed, hovered bool) ui.Pose {
	switch {
	case pressed:
		return ui.Pose{Scale: 0.94}
	case hovered:
		return ui.Pose{Y: -1}
	}
	return ui.Pose{}
}

// navIconPose 给三族 `.nav-icon-btn` 补上原版的位移手感（此前原生版只画了
// 底色/透明度，位移一直没画）。声明成 var 便于测试换成 no-op 做反证。
var navIconPose = func(b ui.Element) {
	b.AnimatePose(navIconPoseFor(b.Pressed(), b.Hovered()), navIconAnim)
}
