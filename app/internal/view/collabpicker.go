package view

import (
	"fmt"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// 合作视频的「选择合作 UP 主」下拉（原版 videoInfo.tsx 的 Dropdown + globals.css
// 的 `.video-owner-picker-menu`）。
//
// 原版是一块**手写的液态玻璃面板**（HeroUI Dropdown，placement=bottom-start），
// 不是系统菜单：
//
//	面板  width min(240px, 100vw - 24px)、max-height 220 + overflow-y auto、
//	      padding 4、圆角 8、1px 玻璃边 + 白 .94 底 + --glass-sheen-static +
//	      blur(20) saturate(1.4)、box-shadow --liquid-shadow
//	条目  min-height 34、padding 7 9、圆角 7、常态透明 + #334155；
//	      hover/focus 铺 --glass-bg-hover + inset 0 1px 1px --glass-highlight
//	      且转蓝字；[aria-checked] 也是蓝字；按下 --glass-bg-active +
//	      inset 0 2px 4px rgba(15,23,42,.08) + scale(.98)
//	文字  name 12px/650；role（description）11px + --studio-muted
//
// 原生版此前用的是框架 Element.Menu —— 那是**系统原生弹出菜单**（GTK /
// Windows 的平台菜单），既没有这块玻璃面板的底/描边/投影，条目也没有
// name/role 两行与那套三态，观感完全对不上。本轮换成照原版手搓的 PopoverBase
// 面板：框架负责开合、点外部关、Esc 关与焦点归还，面板本身由这里上材质。
const (
	collabMenuWidth    = 240 // width: min(240px, calc(100vw - 24px))
	collabMenuMaxH     = 220 // max-height 220
	collabMenuPad      = 4   // padding 4
	collabMenuRadius   = 8   // border-radius 8
	collabMenuGap      = 8   // 与触发键的间距（HeroUI placement=bottom-start 的默认 offset）
	collabItemMinH     = 34  // min-height 34
	collabItemPadX     = 9   // padding 7px 9px
	collabItemPadY     = 7
	collabItemRadius   = 7 // border-radius 7
	collabNameSize     = 12
	collabRoleSize     = 11
	collabNameWeight   = 650
	collabItemInk      = "#334155"
	collabTriggerInk   = "#475569"
	collabPressedInk   = "#0f172a"
	collabPressedAlpha = 0.08
	collabCheckSize    = 14
)

// collabMenuWidthFor 给出菜单宽度：原版 `width: min(240px, calc(100vw - 24px))`
// —— 窗口再窄也不会顶出屏幕。
func collabMenuWidthFor(winW float32) float32 {
	return minF(collabMenuWidth, winW-2*12)
}

// collabItemStyle 是 `.video-owner-picker-item` 这一帧的取值。InsetDy/InsetBlur
// 为 0 表示不画内影（常态）；hover 是 1/1 的玻璃高光，按下是 2/4 的深内影
// （两条 CSS 规则的偏移/模糊不同，所以这里连几何一起带上）。
type collabItemStyle struct {
	Bg        ui.Color
	Ink       ui.Color
	InsetInk  ui.Color
	InsetDy   float32
	InsetBlur float32
	Scale     float32
}

// collabItemStyleFor 按原版 `.video-owner-picker-item` 的几条规则给条目取值：
//
//	常态  transparent + #334155 + scale 1
//	悬停  --glass-bg-hover + inset 0 1px 1px --glass-highlight + 蓝字
//	选中  蓝字（背景仍由悬停决定）
//	按下  --glass-bg-active + inset 0 2px 4px rgba(15,23,42,.08) + scale .98
//
// 按下规则在 CSS 里排在悬停之后且是独立 data 属性，所以按下时底色/内影/缩放
// 全走按下，字色仍按「选中或悬停」决定。
func collabItemStyleFor(t Theme, checked, hovered, pressed bool) collabItemStyle {
	s := collabItemStyle{Ink: ui.Hex(collabItemInk), Scale: 1}
	switch {
	case pressed:
		s.Bg = t.GlassActive
		s.InsetInk = ui.Hex(collabPressedInk).Alpha(collabPressedAlpha)
		s.InsetDy, s.InsetBlur = 2, 4
		s.Scale = 0.98
	case hovered:
		s.Bg = t.GlassHover
		s.InsetInk = t.Highlight
		s.InsetDy, s.InsetBlur = 1, 1
	default:
		s.Bg = ui.Transparent
	}
	if checked || hovered {
		s.Ink = t.Blue
	}
	return s
}

// collabTriggerStyle 是选择键这一帧的取值，对应原版
// `.nav-icon-btn.video-owner-picker-button` 的规则：
//
//	常态  透明底 + 图标 #475569
//	悬停  玻璃底（--glass-bg-hover）+ --glass-highlight 内高光
//	展开  [aria-expanded=true]:not(:active) 同样铺玻璃底 + 内高光，且图标转蓝
//	按下  --glass-bg-active（展开态被 :not(:active) 排除）
func collabTriggerStyleFor(t Theme, expanded, hovered, pressed bool) collabItemStyle {
	s := collabItemStyle{Ink: ui.Hex(collabTriggerInk), Scale: 1}
	switch {
	case pressed:
		s.Bg = t.GlassActive
	case hovered || expanded:
		s.Bg = t.GlassHover
	default:
		s.Bg = ui.Transparent
	}
	if expanded {
		s.Ink = t.Blue
	}
	if hovered || expanded {
		s.InsetInk = t.Highlight
		s.InsetDy, s.InsetBlur = 1, 1
	}
	return s
}

// collabChecker 报告某位参与者是否就是当前面板显示的那位（原版 DropdownMenu
// 的 selectedKeys = displayedOwner.mid）。换视频后 CollabFor 不匹配 → 没有选中项。
func (a *App) collabChecker() func(Collaborator) bool {
	return func(st Collaborator) bool {
		return a.Info != nil && st.Mid == a.CollabMid && a.CollabFor == a.Info.Bvid
	}
}

// collabMenuOpen 报告选择面板此刻是否该展开：展开状态只属于打开它的那条视频，
// 换视频后自动失效（原版 videoInfo 的 useEffect 在 bvid 变化时
// setIsOwnerMenuOpen(false)）。
func (a *App) collabMenuOpen() bool {
	return a.CollabOpen && a.Info != nil && a.CollabOpenFor == a.Info.Bvid
}

// collabPicker 是合作视频的「选择合作 UP 主」键（原版 videoInfo 的
// `.nav-icon-btn.video-owner-picker-button`：32×32 图标钮 + Peoples size 18）。
// 只有 staff > 1 时才画（原版 creators.length > 1）。
func (a *App) collabPicker(c *ui.Context) {
	t := a.Theme
	n := len(a.Info.Staff)
	// 换了视频就收起面板（原版 bvid 变化时 setIsOwnerMenuOpen(false)）。
	// 归一化写在画之前，PopoverBase 拿到的 a.CollabOpen 才是当前视频的展开态。
	if a.CollabOpen && !a.collabMenuOpen() {
		a.CollabOpen = false
	}
	b := ui.ButtonBase(c.Key("owner-picker")).Size(32, 32).Radius(DockIconR).Center().
		Label("选择合作 UP 主").Tooltip(fmt.Sprintf("选择合作 UP 主 (%d)", n))
	st := collabTriggerStyleFor(t, a.CollabOpen, b.Hovered(), b.Pressed())
	b.Background(st.Bg)
	if st.InsetInk.A != 0 {
		b.InsetShadow(0, st.InsetDy, st.InsetBlur, 0, st.InsetInk)
	}
	// .nav-icon-btn:hover 上浮 1px、:active 缩到 .94（pressPose）。
	pressPose(b)
	b.Children(func() {
		ui.Icon(c, iconPeoples).Size(18, 18).TextColor(st.Ink)
	})
	if b.Clicked() {
		a.CollabOpen = !a.CollabOpen
		if a.CollabOpen {
			a.CollabOpenFor = a.Info.Bvid
		}
	}
	a.collabMenu(c, b)
}

// collabMenu 是选择键的下拉面板。用 PopoverBase：框架负责开合、点外部关、
// Esc 关与焦点归还，面板本身照原版 `.video-owner-picker-menu` 上材质。
func (a *App) collabMenu(c *ui.Context, anchor ui.Element) {
	t := a.Theme
	ui.PopoverBase(c, anchor, &a.CollabOpen, func(panel ui.Element) {
		// 原版面板：width min(240,100vw-24)、max-height 220 + 纵向滚动、
		// padding 4、圆角 8、玻璃底 + 白边 + --liquid-shadow（两层）。
		panel.Width(collabMenuWidthFor(windowWidth(c))).
			Padding(collabMenuPad).Radius(collabMenuRadius).
			Margin(collabMenuGap, 0, 0, 0).
			Material(glassLayer(glass.Glass{})).
			InsetShadow(0, 1, 1, 0, t.Highlight).
			InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.04)).
			Shadow(0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.07)).
			Shadow(0, 1, 4, 0, ui.Hex("#0f172a").Alpha(0.03))
		panel.Children(func() {
			// max-height 220 是**含内边距**的盒高，滚动区相应减掉上下 4。
			ui.Scroll(c).FillWidth().MaxHeight(collabMenuMaxH - 2*collabMenuPad).Children(func() {
				ui.Column(c).FillWidth().Gap(0).Children(func() {
					checked := a.collabChecker()
					for _, st := range a.Info.Staff {
						a.collabItem(c, st, checked(st))
					}
				})
			})
		})
	})
}

// collabItem 是面板里的一位参与者（原版 DropdownItem：title=名字、description=角色）。
func (a *App) collabItem(c *ui.Context, st Collaborator, checked bool) {
	t := a.Theme
	it := ui.ButtonBase(c.Key(fmt.Sprintf("collab-%d", st.Mid))).
		FillWidth().MinHeight(collabItemMinH).
		Padding(collabItemPadY, collabItemPadX).Radius(collabItemRadius).
		Justify(ui.Start).Label(st.Name)
	// 原版 aria-label 用名字（textValue = name），tooltip 也给它，长名字截断时
	// 悬停还能看全。
	it.Tooltip(st.Name)
	stl := collabItemStyleFor(t, checked, it.Hovered(), it.Pressed())
	it.Background(stl.Bg)
	if stl.InsetInk.A != 0 {
		it.InsetShadow(0, stl.InsetDy, stl.InsetBlur, 0, stl.InsetInk)
	}
	if stl.Scale != 1 {
		it.AnimatePose(ui.Pose{Scale: stl.Scale}, ui.Animation{Duration: 160 * time.Millisecond, Ease: ui.EaseOut})
	}
	it.Children(func() {
		ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				ui.Text(c, st.Name).FontSize(collabNameSize).
					FontWeight(collabNameWeight).TextColor(stl.Ink).SingleLine()
				if st.Title != "" {
					ui.Text(c, st.Title).FontSize(collabRoleSize).
						TextColor(t.Muted).SingleLine()
				}
			})
			// 选中项右侧一个对勾（原版 HeroUI single-select 的选中指示），
			// 与文案同色（蓝）。
			if checked {
				ui.Icon(c, iconCheck).Size(collabCheckSize, collabCheckSize).
					TextColor(stl.Ink).Shrink(0)
			}
		})
	})
	if it.Clicked() {
		// 选中后显示的是 TA（原版 setSelectedOwner），并立刻打开空间
		// （原版 onAction 里 setIsOwnerMenuOpen(false) + onOwnerClick）。
		a.CollabMid, a.CollabFor = st.Mid, a.Info.Bvid
		a.CollabOpen = false
		if a.Act.OpenUp != nil {
			a.Act.OpenUp(st.Mid, st.Name)
		}
	}
}
