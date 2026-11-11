package view

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
)

var transparencyPresets = []struct {
	Value float64
	Label string
}{{0, "不透明"}, {0.15, "轻透"}, {0.25, "通透"}, {0.40, "轻盈"}}

// Only pointer input snaps; keyboard input remains fine-grained and can leave
// a detent without being pulled back to it on each key press.
func snapTransparency(v float64) float64 {
	v = clampAppearanceTransparency(v)
	for _, preset := range transparencyPresets {
		if math.Abs(v-preset.Value) <= 0.012 {
			return preset.Value
		}
	}
	return v
}

func (t Theme) backgroundOpacity() float32 {
	if t.ReducedTransparency {
		return 1
	}
	return float32(1 - t.Transparency)
}

// Stronger backing on floating controls compensates for a more transparent
// window. Foreground content never inherits the background opacity.
func (t Theme) surface(light ui.Color, dark string, minimum float32) ui.Color {
	if t.Mode == "dark" {
		light = ui.Hex(dark).Alpha(minimum)
	}
	if t.ReducedTransparency {
		light.A = 255
		return light
	}
	a := float32(light.A) / 255
	light.A = uint8(math.Round(float64((a + (1-a)*float32(t.Transparency)*0.75) * 255)))
	return light
}

// darkControl keeps small controls legible without the bright light-theme sheen.
func (t Theme) darkControl(b ui.Element, active, hovered bool) ui.Element {
	fill, border := t.Glass, t.GlassBorder
	if hovered {
		fill = t.GlassHover
	}
	if active {
		fill, border = t.surface(ui.Transparent, "#1b4058", .48), t.Blue.Alpha(.6)
	}
	if b.Pressed() {
		fill = t.GlassActive
	}
	return b.Background(fill).Border(1, border).InsetShadow(0, 1, 0, 0, t.Highlight)
}

// glassPaneVeil 是搜索药丸 / 播放栏这两块**大面积**玻璃保留原版 CSS
// 底色令牌的比例。原版是白 .22 / --player-surface .36；整份照铺时用户仍觉得
// “不够透”，这里只留四成，让背后的渐变与封面真正读得出来。小的控件面
// （按钮、抽屉、弹层）不走这两个值，仍按各自令牌给足对比。
// 深色那一档比浅色**再透一大截**：用户 2026-11-11 反馈「深色模式下搜索栏和
// 底部播放栏影响通透性」，浅色还好。深色底本身够暗，veil 薄一点不会伤可读性，
// 而厚一点就会把背后的渐变整块盖成一块深色板。
const (
	glassPaneVeil     float32 = 0.40
	glassPaneVeilDark float32 = 0.06
)

// glassPaneSheenBase / glassPaneSheenBaseDark 是两块大面板在**零窗口
// 透明度**时也保留的洗光比例。veil 减薄之后，烘焙的白色洗光已经是面上一层
// 最大的不透明层；用户第二轮仍要求“再透些”，所以浅色收到 .72，深色再
// 收到 .05——深色底上的白洗光最显“蒙了一层灰”，必须比浅色更克制。
// 抽屉面板不走这个值（见 glassSheenFade），保持原有玻璃质感。
const (
	glassPaneSheenBase     float32 = 0.72
	glassPaneSheenBaseDark float32 = 0.05
)

// glassPaneOpacity 是两块大面板跟随窗口背景透明度的系数。窗口越透，它们
// 也越透，且用平方曲线：前段（0–15%）变化温和，后段（25–40%）明显让出
// 背后的桌面，避免滑块拉满时面板还像一层固定白纱。
//
// 抽屉不跟这条曲线（它的内容更密，仍走线性的 backgroundOpacity）。
func (t Theme) glassPaneOpacity() float32 {
	if t.ReducedTransparency {
		return 1
	}
	b := t.backgroundOpacity()
	return b * b
}

// glassVeil keeps broad panes light: higher window transparency also thins
// their tint, while blur and local control fills preserve content contrast.
func (t Theme) glassVeil(light ui.Color, dark string, darkAlpha float32) ui.Color {
	if t.Mode == "dark" {
		light = ui.Hex(dark).Alpha(darkAlpha)
	}
	if t.ReducedTransparency {
		light.A = 255
		return light
	}
	return light.Alpha(t.backgroundOpacity())
}

// glassSheenFade thins the baked static sheen together with the window
// transparency. The sheen is a fixed bitmap: without this, thinning only the
// veil leaves the white reflection as the largest opaque layer at the top of
// the slider, and the pane stops getting clearer.
func (t Theme) glassSheenFade() float32 {
	if t.ReducedTransparency {
		return 1
	}
	return t.backgroundOpacity()
}

// glassPaneSheenFade is glassSheenFade for the two large panes, which start
// from a thinner base (glassPaneSheenBase) and follow the pane opacity curve
// (glassPaneOpacity) as the window clears. The drawer keeps the full sheen:
// its larger, darker panel needs the glass depth more than these panes need
// to hide the backdrop.
func (t Theme) glassPaneSheenFade() float32 {
	if t.ReducedTransparency {
		return 1
	}
	base := glassPaneSheenBase
	if t.Mode == "dark" {
		base = glassPaneSheenBaseDark
	}
	return base * t.glassPaneOpacity()
}

// glassPaneTint is glassVeil for the two large panes: the original CSS token
// keeps glassPaneVeil of its alpha, then scales with the window transparency
// through glassPaneOpacity. The small controls and the drawer keep glassVeil's
// gentler linear fade.
func (t Theme) glassPaneTint(light ui.Color, dark string, darkAlpha float32) ui.Color {
	if t.Mode == "dark" {
		light = ui.Hex(dark).Alpha(darkAlpha)
	}
	if t.ReducedTransparency {
		light.A = 255
		return light
	}
	return light.Alpha(t.glassPaneOpacity())
}

func (t Theme) panelFill() ui.Color {
	return t.glassVeil(drawerPanelFill(), "#142238", .50)
}

func (t Theme) floatingFill() ui.Color {
	return t.surface(ui.Hex("#f5f8fc").Alpha(0.96), "#18263a", 0.97)
}

func (t Theme) glassSheen() *sheenBaked {
	if t.ReducedTransparency {
		return nil
	}
	if t.Mode == "dark" {
		return sheenDarkBaked
	}
	return sheenGlassBaked
}

func (t Theme) topTool(s topToolStyle, selected, hovered, pressed bool) topToolStyle {
	if t.Mode != "dark" {
		return s
	}
	s.Bg, s.Ink = ui.Transparent, t.Muted
	if selected {
		s.Bg, s.Ink, s.Opacity = t.Blue.Alpha(.14), t.Blue, 1
	}
	if hovered || pressed {
		s.Bg, s.Ink, s.Opacity = t.Blue.Alpha(pick[float32](selected, .20, .10)), t.Blue, 1
	}
	if pressed {
		s.Bg = t.GlassActive
	}
	return s
}

// Search tools share one lighting layer. Dark appearance uses a broad, fading
// blue reflection so hover and selection never introduce a white top edge.
func (t Theme) topToolLighting(b ui.Element, kind topToolKind, selected, pressed, interactive bool) {
	if t.Mode != "dark" {
		if pressed {
			btnLightPress.draw(b, ToolButtonR)
		} else {
			btnLightTool.draw(b, ToolButtonR)
		}
		if !interactive {
			return
		}
		if pressed {
			navPressShadow(b)
		} else if b.Hovered() {
			topToolHoverHighlight(b, kind)
		}
		return
	}
	target := float32(.025)
	if selected {
		target = .065
	}
	if interactive && b.Hovered() {
		target = .10
	}
	if pressed {
		target = .025
	}
	strength := b.AnimateTo("dark-tool-reflection", target, navIconBgAnim)
	b.Draw(func(p *ui.Painter, r ui.Rect) {
		p.FillGradient(r, ui.LinearGradient{Angle: 155, Stops: []ui.GradientStop{
			{At: 0, Color: ui.Hex("#a9d9f4").Alpha(strength)},
			{At: .55, Color: ui.Transparent},
			{At: 1, Color: ui.Transparent},
		}}, ToolButtonR)
	})
	if pressed {
		b.InsetShadow(0, 2, 4, 0, ui.Hex("#020914").Alpha(.18))
	}
}

// navIconLighting 给内容操作行 / 工具条那批 .nav-icon-btn 铺常态的径向聚光。
// 浅色沿用原版白 .38 的左上光；深色换低强度冷色反射（btnLightNavDark），
// 不再在深底上压一块白斑。
func (t Theme) navIconLighting(b ui.Element, radius float32) ui.Element {
	if t.Mode == "dark" {
		btnLightNavDark.draw(b, radius)
		return b
	}
	btnLightNav.draw(b, radius)
	return b
}

// navIconHover 是 .nav-icon-btn 悬停时 ::after 的内高光。深色下原版那条
// 白 .90 顶缘会成为一条扎眼的亮线，改成与抽屉表头深色材质一致的
// t.Highlight（白 .18）再加一道暗底缘与暗投影。
func (t Theme) navIconHover(b ui.Element) ui.Element {
	if t.Mode != "dark" {
		return navHoverHighlight(b)
	}
	b.InsetShadow(0, 1, 1, 0, t.Highlight)
	b.InsetShadow(0, -1, 1, 0, ui.Hex("#020914").Alpha(.28))
	b.Shadow(0, 2, 10, 0, ui.Hex("#020914").Alpha(.22))
	return b
}

// navIconPress 是 .nav-icon-btn:active 的内压阴影。浅色照原版墨 .08；
// 深色背景上那条几乎看不见，换成更深的冷墨，按下仍有“压进去”的深度。
func (t Theme) navIconPress(b ui.Element) ui.Element {
	if t.Mode != "dark" {
		return navPressShadow(b)
	}
	return b.InsetShadow(0, 2, 4, 0, ui.Hex("#020914").Alpha(.38))
}

// Existing detailed component palettes keep their light values. In dark mode
// neutral inks and saturated action colors are mapped to their readable peers.
func (t Theme) text(light ui.Color) ui.Color {
	if t.Mode != "dark" {
		return light
	}
	alpha := float32(light.A) / 255
	switch {
	case int(max(light.R, light.G, light.B))-int(min(light.R, light.G, light.B)) <= 70:
		if light.R < 70 && light.G < 90 && light.B < 120 {
			return t.Ink.Alpha(alpha)
		}
		if light.R < 175 && light.G < 185 && light.B < 200 {
			return t.Muted.Alpha(alpha)
		}
	case int(light.R) > int(light.G)+30 && int(light.B) > int(light.G)+30:
		return ui.Hex("#c4b5fd").Alpha(alpha)
	case int(light.B) > int(light.R)+55 && light.B > light.G:
		return t.Blue.Alpha(alpha)
	case int(light.R) > int(light.G)+55 && light.R > light.B:
		return t.Rose.Alpha(alpha)
	case int(light.G) > int(light.R)+55:
		return t.Mint.Alpha(alpha)
	}
	return light
}

var iconAppearance = icon(`<circle cx="8" cy="8" r="6"/><path d="M8 2a6 6 0 0 1 0 12Z" fill="currentColor" stroke="none"/>`)

func (a *App) appearanceButton(c *ui.Context) ui.Element {
	t := a.Theme
	b := ui.ButtonBase(c.Key("cover-appearance")).Size(28, 28).Radius(RadiusPill).
		Center().Label("外观").Tooltip("外观 · 主题与透明度").
		Expanded(a.ShowAppearance)
	bg, border, ink := coverToggleStyle(a.ShowAppearance, b.Hovered())
	if t.Mode == "dark" {
		bg, border, ink = t.Glass, t.GlassBorder, t.Muted
		if b.Hovered() || a.ShowAppearance {
			bg, ink = t.GlassHover, t.Blue
		}
	}
	// 与边上的封面开关同一套：底色 200ms 过渡，按下不缩放。此前这颗键
	// 瞬时换底、按下直接 Scale(.97)，既和邻居不一致，缩放还会在亚像素
	// 落点上让图标“往下跳一下”。
	b.Background(b.AnimateColor("cover-appearance-bg", bg, coverToggleAnim)).Border(1, border)
	focusOutline(b, RadiusPill, focusStudio)
	b.Children(func() { ui.Icon(c, iconAppearance).Size(14, 14).TextColor(ink) })
	if b.Clicked() {
		a.ShowAppearance = !a.ShowAppearance
	}
	return b
}

func (a *App) persistAppearance() {
	if a.Act.PersistAppearance != nil {
		a.Act.PersistAppearance()
	}
}

func (a *App) appearancePopover(c *ui.Context, anchor ui.Element) {
	t := a.Theme
	wasOpen := a.ShowAppearance
	ui.PopoverBase(c, anchor, &a.ShowAppearance, func(panel ui.Element) {
		panel.Width(280).Padding(16).Gap(12).Radius(14).Margin(8, 0, 0, 0).
			Background(t.floatingFill()).Border(1, t.GlassBorder).
			Shadow(0, 12, 32, 0, shadowInk.Alpha(0.20)).
			Label("外观设置").Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "外观").FontSize(14).FontWeight(600).TextColor(t.Ink).Grow(1)
				b := ui.ButtonBase(c.Key("appearance-close")).Size(24, 24).Radius(6).
					Center().Label("关闭外观设置").Tooltip("关闭")
				if b.Hovered() {
					b.Background(t.GlassHover)
				}
				b.Children(func() { ui.Icon(c, iconClose).Size(14, 14).TextColor(t.Muted) })
				if b.Clicked() {
					a.ShowAppearance = false
				}
			})
			pref := a.AppearancePref()
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				for _, mode := range []string{"light", "dark", "auto"} {
					label := pick(mode == "light", "浅色", pick(mode == "auto", "自动", "深色"))
					selected := pref == mode
					b := ui.ButtonBase(c.Key("appearance-" + mode)).Grow(1).Basis(0).Height(36).
						Radius(8).Center().Label(label + "主题").Role(ui.RoleToggleButton).Checked(selected)
					if mode == "auto" {
						b.Tooltip("跟随系统时间自动切换（18:00–06:00 深色，06:00–18:00 浅色）")
					}
					b.Background(pick(selected, t.Blue.Alpha(0.14), t.Glass)).
						Border(1, pick(selected, t.Blue.Alpha(0.6), t.Line))
					b.Children(func() { ui.Text(c, label).FontSize(13).TextColor(pick(selected, t.Blue, t.Muted)) })
					focusOutline(b, 8, focusStudio)
					if b.Clicked() {
						a.SetAppearance(mode, a.Theme.Transparency)
						a.persistAppearance()
					}
				}
			})
			ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
				ui.Text(c, "背景透明度").FontSize(12).TextColor(t.Muted).Grow(1)
				ui.Text(c, fmt.Sprintf("%.0f%%", a.Theme.Transparency*100)).FontSize(12).FontWeight(600).TextColor(t.Ink)
			})
			a.transparencySlider(c)
			ui.Row(c).FillWidth().Gap(4).Children(func() {
				for _, preset := range transparencyPresets {
					selected := math.Abs(a.Theme.Transparency-preset.Value) < 0.00001
					b := ui.ButtonBase(c.Key(fmt.Sprintf("opacity-%g", preset.Value))).Grow(1).Basis(0).
						Height(42).Radius(7).Column().Center().Label(fmt.Sprintf("透明度 %.0f%%", preset.Value*100)).
						Role(ui.RoleToggleButton).Checked(selected).Disabled(t.ReducedTransparency)
					if selected || b.Hovered() {
						b.Background(t.Blue.Alpha(pick[float32](selected, 0.14, 0.07)))
					}
					b.Children(func() {
						ui.Text(c, fmt.Sprintf("%.0f%%", preset.Value*100)).FontSize(12).TextColor(pick(selected, t.Blue, t.Ink))
						ui.Text(c, preset.Label).FontSize(10).TextColor(t.Faint)
					})
					if b.Clicked() {
						a.SetAppearance(a.AppearancePref(), preset.Value)
						a.persistAppearance()
					}
				}
			})
			hint := "拖动自由调整 · 靠近推荐刻度自动吸附"
			if t.ReducedTransparency {
				hint = "系统已开启减少透明度，当前使用实色背景"
			} else if pref == "auto" {
				hint = fmt.Sprintf("自动 · 当前%s（%s）· 拖动调整",
					pick(t.Mode == "dark", "深色", "浅色"),
					pick(t.Mode == "dark", "18:00–06:00", "06:00–18:00"))
			}
			// 这一行始终只有一个固定位置：自动提示替换原有拖动提示，
			// 不再额外加一行，切换自动时弹层高度不会跳。
			ui.Text(c, hint).FontSize(10).TextColor(t.Faint).SingleLine()
		})
	})
	if wasOpen && !a.ShowAppearance && a.appearanceHeld {
		a.appearanceHeld = false
		a.persistAppearance()
	}
}

func (a *App) transparencySlider(c *ui.Context) {
	t := a.Theme
	value := t.Transparency
	s := ui.SliderBase(c.Key("appearance-transparency"), &value, 0, appearanceTransparencyMax).
		FillWidth().Height(24).PaddingX(8).Label("调整背景透明度").Disabled(t.ReducedTransparency)
	_, _, held := s.Dragged()
	held = held && !t.ReducedTransparency
	if !t.ReducedTransparency && s.Changed() {
		if held || s.Pressed() {
			value = snapTransparency(value)
		}
		a.SetAppearance(a.AppearancePref(), value)
		if !held {
			a.persistAppearance()
		}
	}
	if a.appearanceHeld && !held {
		a.persistAppearance()
	}
	a.appearanceHeld = held
	focusOutline(s, 6, focusStudio)
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		x, width, y := r.X+8, r.W-16, r.Y+r.H/2
		frac := float32(a.Theme.Transparency / appearanceTransparencyMax)
		p.Fill(ui.Rect{X: x, Y: y - 2, W: width, H: 4}, t.Line, 2)
		p.Fill(ui.Rect{X: x, Y: y - 2, W: width * frac, H: 4}, t.Blue, 2)
		for _, preset := range transparencyPresets {
			tickX := x + width*float32(preset.Value/appearanceTransparencyMax)
			p.Fill(ui.Rect{X: tickX - 1, Y: y + 5, W: 2, H: 3}, t.Faint.Alpha(0.6), 1)
		}
		p.Shadow(ui.Rect{X: x + width*frac - 7, Y: y - 7, W: 14, H: 14}, 7, 0, 1, 4, 0, shadowInk.Alpha(0.2))
		p.Fill(ui.Rect{X: x + width*frac - 7, Y: y - 7, W: 14, H: 14}, t.Blue, 7)
		p.Fill(ui.Rect{X: x + width*frac - 3, Y: y - 3, W: 6, H: 6}, ui.Hex("#ffffff"), 3)
	})
}

// Dark glass reflects light inside its curved edge. An external white fringe
// reads as a bright underline against navy, so this rim fades toward the bottom.
func paintDarkGlassRim(p *ui.Painter, r ui.Rect, radius, opacity float32) {
	if opacity <= 0 {
		return
	}
	strength := min(opacity/.3, float32(1))
	if p.Preferences().HighContrast {
		strength *= 1.6
	}
	ink := ui.Hex("#bad8ee")
	stops := []ui.GradientStop{
		{At: 0, Color: ink.Alpha(.22 * strength)},
		{At: .14, Color: ink.Alpha(.12 * strength)},
		{At: .30, Color: ink.Alpha(.04 * strength)},
		{At: .50, Color: ink.Alpha(.015 * strength)},
		{At: .70, Color: ink.Alpha(.04 * strength)},
		{At: .86, Color: ink.Alpha(.12 * strength)},
		{At: 1, Color: ink.Alpha(.22 * strength)},
	}
	paint := func(path ui.Path) {
		b := path.Bounds()
		if b.W <= 0 || b.H <= 0 {
			return
		}
		p.FillPathConicGradient(&path, ui.ConicGradient{
			Stops: stops, Center: ui.Point{X: (r.X + r.W/2 - b.X) / b.W, Y: (r.Y + r.H/2 - b.Y) / b.H},
		})
	}
	if r.W*r.H > playerRimConicMaxArea {
		parts := rimRingParts(r, radius, 1)
		if len(parts) > 0 {
			for _, part := range parts {
				paint(part)
			}
			return
		}
	}
	paint(rimRingPath(r, radius, 1))
}
