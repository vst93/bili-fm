package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// 抽屉表头的操作键、关闭键、开关与 tab 共用状态解析。浅色保留中性玻璃，
// 深色使用低强度高光；选中后的悬停与按下仍有独立反馈。尺寸与圆角由调用点
// 决定，这里统一材质、过渡、按压和键盘焦点环。

// headerState 是抽屉表头控件这一帧的交互状态。Disabled 的控件不进来：
// mygo 的 Hovered / Pressed 对禁用元素恒假，headerStateOf 再兜一层。
type headerState struct {
	Selected bool
	Hovered  bool
	Pressed  bool
}

// headerFamily 区分表头里两族控件：动作键（普通玻璃）与 tab（液态玻璃 +
// 选中强调）。两族在浅色下的材质不同（原版是两套 CSS），深色下收敛到同一套
// 深色玻璃。
type headerFamily int

const (
	headerActionFamily headerFamily = iota
	headerTabFamily
)

// headerPaint 是一个控件在一态下解析好的材质。零值颜色表示不画那一层。
type headerPaint struct {
	// Fill / Fill2 是填充：Fill2 非零时铺 145° 渐变（浅色选中态的淡蓝→白）。
	Fill, Fill2 ui.Color
	Border      ui.Color      // 1px 描边
	Inset       ui.Color      // inset 0 1px 1px 顶部内高光
	BotIn       ui.Color      // inset 0 -1px 1px 底部内暗线
	Outer       [2]shadowSpec // 外投影（悬停抬起 / 选中光环）
	Rim         float32       // 聚光边（::after）整圈不透明度，tab 专用
	Sheen       bool          // --glass-sheen-static 洗光，tab 浅色专用
}

// headerAnim 是表头动作键的悬停 / 按下色值过渡：短促的 140ms ease，与播放栏
// 键族同一档（CSS ease = cubic-bezier(.25,.1,.25,1)）。
var headerAnim = ui.Animation{
	Duration: 140 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// headerPressAnim 是按压缩放：原版表头键 `transform 0.1s ease`。
var headerPressAnim = ui.Animation{
	Duration: 100 * time.Millisecond,
	Ease:     ui.CubicBezier(0.25, 0.1, 0.25, 1),
}

// headerPressScale 是表头动作键按下时的缩放（关闭键与并排的动作键一致）。
const headerPressScale float32 = 0.94

// headerStateOf 读出控件这一帧的状态。禁用的控件一律按常态画：不给悬停 /
// 按下反馈（加载中的刷新键、无内容的「播放全部」不能因为指针在上面就亮）。
func headerStateOf(b ui.Element, selected bool) headerState {
	st := headerState{Selected: selected}
	if b.IsDisabled() {
		return st
	}
	st.Hovered, st.Pressed = b.Hovered(), b.Pressed()
	return st
}

// headerPaintFor 解析某一族控件在某一态的材质。
//
// 浅色：常态白 .16 中性玻璃 + 顶部白 .9 内高光；悬停白 .38 + 底部墨 .04 +
// 0 6 16 外投影；按下白 .10（比常态更暗，读作「按进去了」）+ 内高光收暗。
// 深色：t.Glass / t.GlassHover / t.GlassActive 三档，描边走 t.GlassBorder →
// t.GlassBorderBright，内高光是 t.Highlight（白 .18，不做白色镜面）。
// 选中：浅色是淡蓝渐变 + 蓝描边 + 2px 蓝光环，深色是蓝调深底 + 蓝描边；
// 选中 + 悬停都再亮一档（此前深色选中态把悬停整条盖掉了）。
func (t Theme) headerPaintFor(st headerState, family headerFamily) headerPaint {
	if t.Mode == "dark" {
		return t.headerPaintDark(st)
	}
	if family == headerTabFamily {
		return headerPaintLightTab(st)
	}
	return headerPaintLightAction(st)
}

// headerPaintLightAction 是浅色下的动作键（刷新 / 定位 / 清空 / 播放模式 /
// 播放全部 / 全部添加 / 关注 / 滚动键 / 关闭键）。
func headerPaintLightAction(st headerState) headerPaint {
	if st.Selected {
		p := headerPaint{
			Fill:   ui.Hex("#0ea5e9").Alpha(0.22),
			Fill2:  ui.Hex("#ffffff").Alpha(0.50),
			Border: ui.Hex("#0284c7").Alpha(0.45),
			Inset:  ui.Hex("#ffffff").Alpha(0.72),
			Outer:  [2]shadowSpec{{}, {0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.14)}},
		}
		if st.Hovered {
			p.Fill = ui.Hex("#0ea5e9").Alpha(0.32)
			p.Fill2 = ui.Hex("#ffffff").Alpha(0.56)
			p.Border = ui.Hex("#0284c7").Alpha(0.58)
			p.Outer[1] = shadowSpec{0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.22)}
		}
		if st.Pressed {
			p.Fill = ui.Hex("#0ea5e9").Alpha(0.16)
			p.Fill2 = ui.Hex("#ffffff").Alpha(0.40)
			p.Border = ui.Hex("#0284c7").Alpha(0.45)
			p.Inset = ui.Hex("#ffffff").Alpha(0.60)
			p.Outer[1] = shadowSpec{0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.10)}
		}
		return p
	}
	p := headerPaint{
		Fill:   ui.Hex("#ffffff").Alpha(0.16),
		Border: ui.Hex("#ffffff").Alpha(0.45),
		Inset:  ui.Hex("#ffffff").Alpha(0.90),
		BotIn:  ui.Hex("#0f172a").Alpha(0.04),
	}
	if st.Hovered {
		p.Fill = ui.Hex("#ffffff").Alpha(0.38)
		p.Border = ui.Hex("#ffffff").Alpha(0.62)
		p.BotIn = ui.Hex("#0f172a").Alpha(0.05)
		p.Outer[0] = shadowSpec{0, 6, 16, 0, shadowInk.Alpha(0.09)}
	}
	if st.Pressed {
		p.Fill = ui.Hex("#ffffff").Alpha(0.10)
		p.Border = ui.Hex("#ffffff").Alpha(0.50)
		p.Inset = ui.Hex("#ffffff").Alpha(0.55)
		p.BotIn = ui.Hex("#0f172a").Alpha(0.06)
	}
	return p
}

// headerPaintLightTab 是浅色下的 tab / 收藏夹胶囊（原版 [data-slot="tab"] 的
// 液态玻璃：常态白 .22、悬停白 .30 + 抬起、选中白 .52 + 更高一档抬起）。
func headerPaintLightTab(st headerState) headerPaint {
	p := headerPaint{
		Fill:   ui.Hex("#ffffff").Alpha(0.22),
		Border: ui.Hex("#ffffff").Alpha(0.55),
		Rim:    0.65,
		Sheen:  true,
	}
	switch {
	case st.Selected:
		p.Fill = ui.Hex("#ffffff").Alpha(0.52)
		p.Border = ui.Hex("#ffffff").Alpha(0.72)
		p.Rim = 1
		p.Outer[0] = shadowSpec{0, 16, 38, 0, shadowInk.Alpha(0.11)}
		p.Outer[1] = shadowSpec{0, 2, 8, 0, shadowInk.Alpha(0.04)}
		if st.Hovered {
			// 选中 + 悬停必须看得出变化，同时保持选中：底更实、边更亮。
			p.Fill = ui.Hex("#ffffff").Alpha(0.70)
			p.Border = ui.Hex("#ffffff").Alpha(0.88)
		}
	case st.Hovered:
		p.Fill = ui.Hex("#ffffff").Alpha(0.30)
		p.Border = ui.Hex("#ffffff").Alpha(0.72)
		p.Outer[0] = shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}
		p.Outer[1] = shadowSpec{0, 1, 4, 0, shadowInk.Alpha(0.03)}
	}
	if st.Pressed {
		// 按下压暗一层，但**不抹掉选中**：选中 tab 仍是实底 + 满圈聚光边。
		if st.Selected {
			p.Fill = ui.Hex("#ffffff").Alpha(0.44)
		} else {
			p.Fill = ui.Hex("#ffffff").Alpha(0.14)
			p.Border = ui.Hex("#ffffff").Alpha(0.58)
			p.Rim = 0.55
			p.Outer = [2]shadowSpec{}
		}
	}
	return p
}

// headerPaintDark 是深色主题下两族共用的材质：低强度描边的深色玻璃，
// 选中态用蓝调深底 + 蓝描边（不是亮蓝实心块）。
func (t Theme) headerPaintDark(st headerState) headerPaint {
	p := headerPaint{
		Fill:   t.Glass,
		Border: t.GlassBorder,
		Inset:  t.Highlight,
	}
	if st.Selected {
		p.Fill = t.surface(ui.Transparent, "#1b4058", .48)
		p.Border = t.Blue.Alpha(.60)
		p.Inset = ui.Hex("#ffffff").Alpha(.10)
	}
	if st.Hovered {
		p.Fill = t.GlassHover
		p.Border = t.GlassBorderBright
		if st.Selected {
			p.Fill = t.surface(ui.Transparent, "#25597c", .58)
			p.Border = t.Blue.Alpha(.80)
			p.Inset = ui.Hex("#ffffff").Alpha(.16)
		}
	}
	if st.Pressed {
		// 按下比悬停**更暗**：深色玻璃上的「凹下去」靠填充压深一档读出来，
		// 不再只换一个几乎同亮的填充。
		p.Fill = t.surface(ui.Transparent, "#081120", .55)
		p.Border = t.GlassBorder
		p.Inset = ui.Hex("#ffffff").Alpha(.05)
		if st.Selected {
			p.Fill = t.surface(ui.Transparent, "#0a2231", .62)
			p.Border = t.Blue.Alpha(.55)
			p.Inset = ui.Hex("#ffffff").Alpha(.07)
		}
	}
	return p
}

// headerFade / headerLift / headerRimFade 把色值、投影几何与聚光边交给过渡层。
// 桌面要求减少动态效果时当帧到位（原版对应的 prefers-reduced-motion 块写的是
// `transition: none`）。声明成 var 便于测试冻结动画相位。
var (
	headerFade = func(c *ui.Context, b ui.Element, key string, target ui.Color, a ui.Animation) ui.Color {
		if reduceMotion(c) {
			return target
		}
		return b.AnimateColor(key, target, a)
	}
	headerLift = func(c *ui.Context, b ui.Element, key string, s shadowSpec, a ui.Animation) shadowSpec {
		if reduceMotion(c) {
			return s
		}
		return shadowSpec{
			X:      b.AnimateTo(key+"-x", s.X, a),
			Y:      b.AnimateTo(key+"-y", s.Y, a),
			Blur:   b.AnimateTo(key+"-blur", s.Blur, a),
			Spread: b.AnimateTo(key+"-spread", s.Spread, a),
			C:      b.AnimateColor(key+"-ink", s.C, a),
		}
	}
	headerRimFade = func(c *ui.Context, b ui.Element, target float32, a ui.Animation) float32 {
		if reduceMotion(c) {
			return target
		}
		return b.AnimateTo("header-rim", target, a)
	}
)

// headerFocusRing 是表头控件的键盘焦点环（` :focus-visible `）：浅色是
// --studio-blue，深色换成主题强调色，几何统一为 2px / offset 2。焦点环只在
// 键盘焦点下画（focusOutline 判 FocusVisible），鼠标点击不会留下常驻环。
func (t Theme) headerFocusRing() focusRing {
	if t.Mode == "dark" {
		return focusRing{t.Blue.Alpha(0.80), 2, 2}
	}
	return focusStudio
}

// headerAction 给表头动作键铺材质 + 按压 + 焦点环。selected 为真表示这颗键
// 是「开」着的开关（弹幕热力 / 跟随 / 桌面字幕等）；隐身开关、关注、刷新
// 这类动作传 false —— 它们只改图标墨色，不铺选中底色。
func (t Theme) headerAction(c *ui.Context, b ui.Element, selected bool, radius float32) ui.Element {
	st := headerStateOf(b, selected)
	return t.headerPaintInto(c, b, st, t.headerPaintFor(st, headerActionFamily), headerAnim, radius, true)
}

// headerTab 给表头 tab 铺材质。tab 与动作键共用状态解析与过渡，只是浅色下
// 保留原版的液态玻璃（洗光 + 聚光边 + 抬起），按下不缩放（原版
// `header [data-slot="tab"]:active { transform: none }`）。
func (t Theme) headerTab(c *ui.Context, b ui.Element, active bool) ui.Element {
	st := headerStateOf(b, active)
	p := t.headerPaintFor(st, headerTabFamily)
	b = t.headerPaintInto(c, b, st, p, drawerTabAnim, Radius, false)
	if !p.Sheen {
		return b
	}
	rim := headerRimFade(c, b, p.Rim, drawerTabAnim)
	// ::after 上那组不随状态变的内影与色散边（原版 inset 0 1px 1px .5px
	// --glass-highlight、inset 0 -1px 1px 墨 .06、--liquid-chromatic 两条
	// 0.5px 亚像素边），整圈不透明度由 rim 统一缩放。
	b.InsetShadow(0, 1, 1, 0.5, ui.Hex("#ffffff").Alpha(0.90*rim))
	b.InsetShadow(0, -1, 1, 0, shadowInk.Alpha(0.06*rim))
	b.Shadow(0, 0.5, 0, 0, ui.Hex("#fffaf0").Alpha(0.50*rim))
	b.Shadow(0, -0.5, 0, 0, ui.Hex("#c8dcf0").Alpha(0.20*rim))
	return drawerTabGlass(b, rim)
}

// headerPaintInto 把解析好的材质落到元素上。press 为真时加按压缩放（悬停
// **不上浮**：原版表头键只有 `:active { transform }`）；tab 传 false。
func (t Theme) headerPaintInto(c *ui.Context, b ui.Element, st headerState, p headerPaint, a ui.Animation, radius float32, press bool) ui.Element {
	if p.Fill2.A > 0 {
		b.Gradient(headerFade(c, b, "header-fill", p.Fill, a), headerFade(c, b, "header-fill2", p.Fill2, a), 145)
	} else {
		b.Background(headerFade(c, b, "header-fill", p.Fill, a))
	}
	b.Border(1, headerFade(c, b, "header-border", p.Border, a))
	if inset := headerFade(c, b, "header-inset", p.Inset, a); inset.A > 0 {
		b.InsetShadow(0, 1, 1, 0, inset)
	}
	if bot := headerFade(c, b, "header-botin", p.BotIn, a); bot.A > 0 {
		b.InsetShadow(0, -1, 1, 0, bot)
	}
	if s := headerLift(c, b, "header-outer0", p.Outer[0], a); s.C.A > 0 {
		b.Shadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	if s := headerLift(c, b, "header-outer1", p.Outer[1], a); s.C.A > 0 {
		b.Shadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	if press {
		pose := ui.Pose{}
		if st.Pressed {
			pose = ui.Pose{Scale: headerPressScale}
		}
		b.AnimatePose(pose, headerPressAnim)
	}
	return focusOutline(b, radius, t.headerFocusRing())
}
