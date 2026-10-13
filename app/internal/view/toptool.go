package view

import (
	"github.com/egoist/mygo/ui"
)

// 搜索药丸里的两类小键共用原版 globals.css 的一套状态：
//
//	.top-tool-btn       四个内容入口（动态 / 热门与推荐 / 收藏 / 历史）
//	.search-submit-btn  输入框右端的放大镜提交键
//
// 原版给两者的基态 / 悬停 / 按下逐字相同，只有**悬停底色**不同
// （.top-tool-btn 是 rgba(255,255,255,.65)，.search-submit-btn 是 .50）：
//
//	基态  背景透明、opacity .82、图标 #64748b
//	悬停  铺白底、opacity 1、图标转蓝 #0284c7、translateY(-1px)
//	按下  缩到 scale(.92)、底换 --glass-bg-active（白 .16），
//	      color / opacity 沿用悬停（:active 不重置它们）
//
// 之前原生版把这四颗入口键当成 .nav-icon-btn 画（图标用 --studio-muted
// #475569、悬停底色白 .30、按下缩 .94），于是悬停时图标不变蓝、常态也比
// 原版深一档。这里把状态集中一处（同 playlistRowPaint / settingsItemPaint）。
type topToolKind int

const (
	topToolEntry  topToolKind = iota // .top-tool-btn
	topToolSubmit                    // .search-submit-btn
)

// topToolStyle 是某个状态下解析好的样式。
type topToolStyle struct {
	Bg      ui.Color
	Ink     ui.Color
	Opacity float32
}

// topToolHoverBg 是两类键悬停底色的唯一差别（原版 .top-tool-btn 白 .65、
// .search-submit-btn 白 .50）。
func topToolHoverBg(k topToolKind) ui.Color {
	if k == topToolSubmit {
		return ui.Hex("#ffffff").Alpha(0.50)
	}
	return ui.Hex("#ffffff").Alpha(0.65)
}

// topToolStyleFor 解析三态。按下只覆盖底色，图标色与不透明度沿用悬停。
func topToolStyleFor(k topToolKind, hovered, pressed bool) topToolStyle {
	s := topToolStyle{
		Bg:      ui.Transparent,
		Ink:     ui.Hex("#64748b"), // 原版 .top-tool-btn / .search-submit-btn 的 color
		Opacity: 0.82,
	}
	if hovered || pressed {
		s.Bg = topToolHoverBg(k)
		s.Ink = ui.Hex("#0284c7")
		s.Opacity = 1
	}
	if pressed {
		s.Bg = ui.Hex("#ffffff").Alpha(0.16) // --glass-bg-active
	}
	return s
}

// topToolPoseFor / topToolPose 给搜索药丸小键加原版的手感：悬停上浮 1px、
// 按下缩到 .92。与 navIconPoseFor（.nav-icon-btn，缩 .94）同形、只差 :active
// 缩放（原版 .top-tool-btn / .search-submit-btn 的 :active 就是 .92）。过渡参数
// 与 .nav-icon-btn 逐字相同（`transform 0.16s ease`），所以共用 navIconAnim ——
// 此前这里走 pressPoseScale 的 120ms/EaseOut，上浮/缩放比原版更快、更硬。
func topToolPoseFor(pressed, hovered bool) ui.Pose {
	switch {
	case pressed:
		return ui.Pose{Scale: 0.92}
	case hovered:
		return ui.Pose{Y: -1}
	}
	return ui.Pose{}
}

func topToolPose(b ui.Element) {
	b.AnimatePose(topToolPoseFor(b.Pressed(), b.Hovered()), navIconAnim)
}

// topToolBgColor / topToolFadeValue / topToolBgTo / topToolFade 是
// `.top-tool-btn` / `.search-submit-btn` 的**底色 / 不透明度过渡**。原版 transition
// 与 `.nav-icon-btn` 逐字相同：
//
//	transition: color 0.18s ease, background-color 0.18s ease,
//	            opacity 0.16s ease, transform 0.16s ease;
//
// 底色 180ms、不透明度 160ms，都是 CSS 默认缓动曲线。此前原生版两条都瞬时写入
// （悬停时白底与不透明度「啪」地跳出），这里按原版补上。纯函数拆出来便于测试
// 钉住「走的是过渡而非瞬时」；两个 var 便于测试独立冻结 / 反证（同 navIconBgTo /
// navIconFade）。
func topToolBgColor(b ui.Element, target ui.Color) ui.Color {
	return b.AnimateColor("top-tool-bg", target, navIconBgAnim)
}

func topToolFadeValue(b ui.Element, target float32) float32 {
	return b.AnimateTo("top-tool-fade", target, navIconFadeAnim)
}

// topToolInkColor / topToolInk 是第三条非位移动画：`color 0.18s ease`
// （图标 #64748b → #0284c7 的淡变）。原版 transition 里这条比底色还靠前，
// 但此前原生版一直瞬时写入 —— 悬停时图标「啪」地变蓝。时长与底色同为 180ms、
// 同一条 CSS 默认缓动，所以复用 navIconBgAnim；拆纯函数便于测试钉住「走的是过渡
// 而非瞬时」，var 便于无头测试冻结 / 反证。
func topToolInkColor(b ui.Element, target ui.Color) ui.Color {
	return b.AnimateColor("top-tool-ink", target, navIconBgAnim)
}

var (
	topToolBgTo = func(b ui.Element, target ui.Color) ui.Element {
		return b.Background(topToolBgColor(b, target))
	}
	topToolFade = func(b ui.Element, target float32) ui.Element {
		return b.Opacity(topToolFadeValue(b, target))
	}
	topToolInk = func(b ui.Element, target ui.Color) ui.Element {
		return b.TextColor(topToolInkColor(b, target))
	}
)

// shadowSpec 是一条 box-shadow（原版 CSS 里的 x y blur spread color）。
type shadowSpec struct {
	X, Y, Blur, Spread float32
	C                  ui.Color
}

// searchPillShadows 返回搜索药丸的两态投影。原版 box-shadow 走
// --liquid-shadow / --liquid-shadow-lifted，各两条、**二选一**；mygo 的
// Shadow 是累加的，所以集中在这里给出，调用点不再「先铺常态再叠悬停」
// （那会同时挂上四道投影）。
func searchPillShadows(hovered bool) []shadowSpec {
	if hovered {
		return []shadowSpec{
			{0, 16, 38, 0, ui.Hex("#0f172a").Alpha(0.11)},
			{0, 2, 8, 0, ui.Hex("#0f172a").Alpha(0.04)},
		}
	}
	return []shadowSpec{
		{0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.07)},
		{0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03)},
	}
}

// applyShadows 按顺序把一串**外投影** box-shadow 落到元素上。
func applyShadows(e ui.Element, ss []shadowSpec) ui.Element {
	for _, s := range ss {
		e.Shadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	return e
}

// applyInsetShadows 同 applyShadows，但落的是**内影**（原版 CSS 的 `inset`）。
// 两者走不同原语（InsetShadow / Shadow），不能混在一起 —— 之前把
// `.nav-icon-btn::after` 的前两条 `inset` 误当外投影铺了，按钮周围多出一圈原版
// 没有的光晕，而按钮内部的顶/底缘高光反而没画。这里按原版拆成两组。
func applyInsetShadows(e ui.Element, ss []shadowSpec) ui.Element {
	for _, s := range ss {
		e.InsetShadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	return e
}

// hoverHighlight 是原版 `::after` 那组 box-shadow，按 inset / 外投影分成两组
// （前两条 `inset`、第三条是外层）。
type hoverHighlight struct {
	Inset []shadowSpec
	Outer []shadowSpec
}

// applyHighlight 按原版层级铺一组 ::after 阴影：先内影、再外投影。
func applyHighlight(e ui.Element, h hoverHighlight) ui.Element {
	applyInsetShadows(e, h.Inset)
	return applyShadows(e, h.Outer)
}

// navHoverHighlightSpecs 是 .nav-icon-btn 悬停 ::after 的 box-shadow（原版
// globals.css:2511-2518，逐值）：
//
//	inset 0 1px 1px 白 .9 / inset 0 -1px 1px 墨 .03 / 0 2px 10px 墨 .045
func navHoverHighlightSpecs() hoverHighlight {
	return hoverHighlight{
		Inset: []shadowSpec{
			{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}, // inset 0 1px 1px --glass-highlight
			{0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.03)},
		},
		Outer: []shadowSpec{
			{0, 2, 10, 0, ui.Hex("#0f172a").Alpha(0.045)},
		},
	}
}

// topToolHoverHighlightSpecs 是 .top-tool-btn / .search-submit-btn 悬停 ::after。
// 与 nav 族只差顶缘那条内高光的模糊：top-tool 是 `inset 0 1px 0`（blur 0），
// submit 是 `inset 0 1px 1px`（blur 1）。
func topToolHoverHighlightSpecs(k topToolKind) hoverHighlight {
	topBlur := float32(1)
	if k == topToolEntry {
		topBlur = 0
	}
	h := navHoverHighlightSpecs()
	h.Inset[0].Blur = topBlur
	return h
}

// pressShadowSpecs 是 `.nav-icon-btn:active` / `.top-tool-btn:active` /
// `.search-submit-btn:active` 逐字相同的那条内压阴影（原版 globals.css）：
// `inset 0 2px 4px rgba(15,23,42,.08)` —— 按下时 ::after 的内高光归零、换成这条
// 更深的内压。抽成纯函数便于把值钉死。
func pressShadowSpecs() []shadowSpec {
	return []shadowSpec{
		{0, 2, 4, 0, ui.Hex("#0f172a").Alpha(0.08)},
	}
}

// navHoverHighlight 给 .nav-icon-btn 补原版悬停时 ::after 的内高光 + 弱投影
// （原版只描边、不铺底色；::after 在 :hover 时 opacity 0→1、按下/禁用时归 0）。
// 之前原生版对 .nav-icon-btn 悬停只换底色（缺这道内高光），.top-tool-btn 虽有
// 近似（白 .55 边 + inset .55）但值低一档、还多一道边框 —— 上一轮遗留的
// 「下轮候选」。这里按原版逐值补齐三族。
//
// 声明成 var 是为了让无头测试能临时替换成 no-op，反证这层内高光确实来自这里
// （同 btnlight 的作证方式）。
var navHoverHighlight = func(b ui.Element) ui.Element {
	return applyHighlight(b, navHoverHighlightSpecs())
}

// topToolHoverHighlight 同 navHoverHighlight，作用于搜索药丸两类小键。
var topToolHoverHighlight = func(b ui.Element, k topToolKind) ui.Element {
	return applyHighlight(b, topToolHoverHighlightSpecs(k))
}

// navPressShadow 在按钮按下时铺原版 `:active` 的内压阴影（`inset 0 2px 4px`）。
// 三族（.nav-icon-btn / .top-tool-btn / .search-submit-btn）与选择键逐字相同，
// 调用点都用 `Pressed()` 排掉了同帧的悬停内高光（原版 `:active::after { opacity: 0 }`）。
// 声明成 var 便于无头测试换成 no-op 做反证（同 navHoverHighlight）。
var navPressShadow = func(b ui.Element) ui.Element {
	return applyInsetShadows(b, pressShadowSpecs())
}

// avatarShadow 给头像键补原版 .home-user-avatar 的两态投影：
// 常态 0 6px 16px rgba(15,23,42,.08)；悬停 0 12px 28px rgba(14,165,233,.2)。
// mygo 的投影是累加的，所以两态必须二选一（不能用「先铺常态再叠悬停」）。
func avatarShadow(b ui.Element) {
	if b.Hovered() {
		b.Shadow(0, 12, 28, 0, ui.Hex("#0ea5e9").Alpha(0.20))
		return
	}
	b.Shadow(0, 6, 16, 0, shadowInk.Alpha(0.08))
}

// avatarLift 是原版 .home-user-avatar:hover 的 translateY(-1px)，
// 走 `transition: transform 0.18s ease`（与 UP 头像同一时长/曲线）。
// 头像键没有 :active 缩放，所以只上浮。
func avatarLift(b ui.Element) {
	if !b.Hovered() {
		return
	}
	b.AnimatePose(ui.Pose{Y: -1}, faceHoverAnim)
}
