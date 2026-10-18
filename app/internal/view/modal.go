package view

import (
	"math"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// modalIn 是模态框入场（缩放 + 淡入）的时长。
const modalIn = 180 * time.Millisecond

// loginGlassBlur 是 `.login-glass` 的 `backdrop-filter: blur(28px)`
// （globals.css:5363）里的 28；dialogGlassBlur 是 `.liquid-dialog` 的
// `blur(24px)`（globals.css:5654）里的 24。两者都是 CSS blur() 的那个值，
// 也就是 glass.Blur.Radius 的口径。
const (
	loginGlassBlur  float32 = 28
	dialogGlassBlur float32 = 24
)

// loginPanelFill 是 `.login-glass` 的 `background: rgba(255,255,255,0.28)`。
//
// 声明成 var 是为了让无头测试能把它改成透明做反证（见 modalglass_test.go）：
// 改透明后面板的提亮必须消失，才能证明那点亮来自这层底色。
var loginPanelFill = ui.Hex("#ffffff").Alpha(0.28)

// dialogPanelGradient 是 `.liquid-dialog` 的内联 background
// （main:src/components/dialog/DialogProvider.tsx:160）：
//
//	linear-gradient(145deg, rgba(255,255,255,.88), rgba(255,255,255,.75) 55%,
//	                accentForType(type))
//
// 145deg 与 ui.LinearGradient.Angle 同一口径（0 = 朝上、顺时针），停靠点位置
// 也照抄 55%。accent 就是 dialogTypeStyleFor().Tint（与 accentForType 逐值相同）。
func dialogPanelGradient(accent ui.Color) ui.LinearGradient {
	return ui.LinearGradient{Angle: 145, Stops: []ui.GradientStop{
		{Color: ui.Hex("#ffffff").Alpha(0.88), At: 0},
		{Color: ui.Hex("#ffffff").Alpha(0.75), At: 0.55},
		{Color: accent, At: 1},
	}}
}

// paintDialogPanelFill 把上面那条渐变铺在面板边框的内缘（inset 1px：Draw 在
// border 之后跑，不 inset 会盖掉自己的 1px 白边，口径同 paintGlassPanelFill）。
func paintDialogPanelFill(p *ui.Painter, r ui.Rect, radius float32, accent ui.Color) {
	p.FillGradient(ui.Rect{X: r.X + 1, Y: r.Y + 1, W: r.W - 2, H: r.H - 2},
		dialogPanelGradient(accent), max(radius-1, 0))
}

// modal 画当前模态对话框（原版的 DialogProvider + 登录面板）。
//
// v0.4.0 起用官方 `ui.DialogBase`：Esc / 点外部关闭是框架行为（Dismissed），
// 焦点圈、覆盖层、居中都不用自己管——之前手绘的 overlay 把整个窗口的
// 点击都吃掉的教训不会再犯。
//
// 登录框的关闭走 CloseLogin（要停轮询），其余走 CloseModal。
func (a *App) modal(c *ui.Context) {
	m := a.Modal
	if m == nil {
		return
	}
	t := a.Theme
	open := true
	// 缩放入场（原版 .liquid-dialog 的 scale 0.95 → 1 + 淡入，
	// 0.2s transition）。进度由打开时间算，值交给 AnimateTo 平滑。
	if a.modalOpenedAt.IsZero() {
		a.modalOpenedAt = time.Now()
	}

	ui.DialogBase(c, &open, func(backdrop, panel ui.Element) {
		switch m.Kind {
		case "login":
			// 原版 .login-glass：320 宽（w-80）、圆角 16（rounded-2xl）。
			//
			// 材质：原版 background 白 .28 与 backdrop-filter blur(28px) 是**两件
			// 独立的事**。此前写的是 `Material(glass.Glass{})` + `Background(白.28)`——
			// Material 把 fill 置成 material，其后的 Background 是空操作（被静默
			// 忽略），于是面板只剩 Regular 玻璃的「模糊 + 提亮」，白 .28 从来没画
			// 上去，整块比原版亮一档（同 toast / 设置下拉那一族）。
			// 现在：glass.Blur（只模糊）+ Draw 里自铺白 .28（`::before` 光泽层仍由
			// modalLogin 的 loginSheen 画在这层之上、内容之下）。
			panel.Width(loginPanelWidth).Radius(16).
				Padding(32, 24, 24).
				Material(blurLayer(glass.Blur{Radius: loginGlassBlur})).
				Draw(func(p *ui.Painter, r ui.Rect) {
					paintGlassPanelFill(p, r, 16, loginPanelFill)
				}).
				Border(1, ui.Hex("#ffffff").Alpha(0.40)).
				InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.30)).
				Shadow(0, 8, 32, 0, ui.Hex("#0f172a").Alpha(0.20)).
				Shadow(0, 2, 8, 0, ui.Hex("#0f172a").Alpha(0.14))
		default:
			// 原版 .liquid-dialog：min-width 360、padding 28 32 22、圆角 18、
			// 1px 白边 .65 + 两条 inset（顶部白高光 / 底部内影）+ 两条外投影，
			// 底色是 backdrop-filter 玻璃，颜色渐变末尾接的是 accentForType(type)。
			st := dialogTypeStyleFor(m.Type)
			panel.MinWidth(360).MaxWidth(460).Radius(18).
				Padding(28, 32, 22).
				// 原版 .liquid-dialog 的 `backdrop-filter: blur(24px)` 是材质，
				// 真正的底色是 DialogProvider 的内联 `linear-gradient(145deg, …)`：
				// 白 .88 → 白 .75（55%）→ accentForType(type)。
				//
				// 此前是 `Material(glass.Glass{Tint})` + `Background(#f1f5f9 .88)`：
				// 前者把 fill 置成 material，后者是空操作 —— 那条渐变（以及它末尾的
				// 类型强调色）从来没画上去，面板只剩玻璃。现在按同一套写法重排：
				// glass.Blur 只模糊，渐变由 Draw 自铺（inset 1px 让开边框）。
				Material(blurLayer(glass.Blur{Radius: dialogGlassBlur})).
				Draw(func(p *ui.Painter, r ui.Rect) {
					paintDialogPanelFill(p, r, 18, st.Tint)
				}).
				Border(1, ui.Hex("#ffffff").Alpha(0.65)).
				InsetShadow(0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.85)).
				InsetShadow(0, -1, 3, 0, ui.Hex("#0f172a").Alpha(0.05)).
				Shadow(0, 24, 64, 0, ui.Hex("#0f172a").Alpha(0.22)).
				Shadow(0, 4, 16, 0, ui.Hex("#0f172a").Alpha(0.06))
		}
		prog := float32(1)
		if age := time.Since(a.modalOpenedAt); age < modalIn {
			prog = float32(age) / float32(modalIn)
		}
		// 全窗遮罩：原版对话框/登录覆盖层都会把整窗压暗（对话框 slate .2、
		// 登录纯黑 .25），此前 DialogBase 的 backdrop 一直没上色——弹窗浮在
		// 亮背景上、没有任何压暗。alpha 乘入场进度，跟面板一起淡入。
		backdrop.Background(modalScrimInk(m.Kind).Alpha(prog))
		sc := panel.AnimateTo("modal-in", 0.96+0.04*prog, ui.Snappy)
		panel.Scale(sc, sc).
			Opacity(panel.AnimateTo("modal-fade", prog,
				ui.Animation{Duration: modalIn}))
		panel.Children(func() {
			switch m.Kind {
			case "login":
				a.modalLogin(c, m, t)
			default:
				a.modalMessage(c, m, t)
				// ::after 光泽层是伪元素，天然画在内容之上。
				dialogSheen(c)
			}
		})

		// Esc / 点外部关闭由框架报告；登录框要停轮询（CloseLogin 换代）。
		if panel.Dismissed() {
			a.modalOpenedAt = time.Time{}
			if m.Kind == "login" {
				if a.Act.CloseLogin != nil {
					a.Act.CloseLogin()
				}
			} else if a.Act.CloseModal != nil {
				a.Act.CloseModal()
			}
		}
	})
}

// modalScrimInk 是模态遮罩的压暗层底色（原版对话框/登录面板铺满全窗的那层）：
//
//	对话框  DialogProvider 覆盖层 `background: rgba(15,23,42,0.2)`
//	登录    index.tsx 覆盖层 `bg-black/25`（纯黑 .25）
//
// 原版两层都还带 `backdrop-filter: blur(...)`；原生版按抽屉遮罩的既定做法只铺
// 纯色压暗（见 drawer.go 的「覆盖层 rgba(15,23,42,.28)，不模糊」）——面板自己
// 已经是真玻璃（会模糊它背后的这一层），再给全窗加一层 backdrop 是每帧最贵的
// 那类开销，观感收益却只是 6px 模糊。返回的颜色已带最终 alpha，调用点用
// `.Alpha(prog)` 让它随入场淡入。
func modalScrimInk(kind string) ui.Color {
	if kind == "login" {
		return ui.Hex("#000000").Alpha(0.25)
	}
	return ui.Hex("#0f172a").Alpha(0.20)
}

// modalLogin 是扫码登录面板（原版 .login-glass + index.tsx 的登录块）。
func (a *App) modalLogin(c *ui.Context, m *Modal, t Theme) {
	// .login-glass::before 的 135° 光泽层：画在最前，落在面板底之上、内容之下
	//（原版 `.login-glass > * { z-index: 1 }` 把内容抬到它上面）。
	loginSheen(c)

	// 关闭键：原版 .login-close-btn 是 32 圆键（半透明白底 + 白描边 +
	// 顶部内高光 + 外投影，见 loginclose.go）。原版定位 `-right-4 top-4`
	// （探出面板 16px），用户反馈那样会掉出背景框；这里完整落在面板内。
	close := ui.ButtonBase(c.Key("login-close")).Size(loginCloseSize, loginCloseSize).
		Radius(RadiusPill).Center().Label("关闭登录").Tooltip("关闭登录")
	close.Absolute().Right(loginCloseInset).Top(loginCloseInset)
	p := loginClosePaintFor(close.Hovered(), close.Pressed())
	close.Background(p.BG).Border(1, p.Border)
	for _, s := range p.Inset {
		close.InsetShadow(s.X, s.Y, s.Blur, s.Spread, s.C)
	}
	applyShadows(close, p.Outer)
	close.AnimatePose(ui.Pose{Scale: p.Scale}, ui.Snappy)
	close.Children(func() {
		ui.Icon(c, iconCloseSmall).Size(loginCloseIconSize, loginCloseIconSize).
			TextColor(ui.Hex("#475569"))
	})
	if close.Clicked() {
		if a.Act.CloseLogin != nil {
			a.Act.CloseLogin()
		}
	}

	// 内容居中（原版 text-center）。
	ui.Box(c).FillWidth().Center().Children(func() {
		// h3：text-lg（18）/ bold / #0f172a，下间距 4。
		ui.Text(c, "使用 B站 App 扫码登录").FontSize(18).FontWeight(700).
			TextColor(ui.Hex("#0f172a"))
	})
	ui.Box(c).FillWidth().Center().Margin(4, 0, 16, 0).Children(func() {
		// p：text-sm（14）/ 500 / #334155，下间距 16。
		ui.Text(c, "打开手机扫一扫").FontSize(14).FontWeight(500).
			TextColor(ui.Hex("#334155"))
	})
	ui.Box(c).FillWidth().Center().Children(func() {
		// .login-qr-card：padding 12、圆角 12；里面是 160 的二维码（w-40 h-40）。
		p := loginQRCardPaint()
		card := ui.Box(c).Size(184, 184).Radius(12).Padding(12).
			Background(p.BG).Border(1, p.Border)
		applyInsetShadows(card, p.Inset)
		applyShadows(card, p.Outer)
		card.Children(func() {
			if m.QR != nil {
				ui.Image(c, m.QR).Fill().Fit(ui.Contain)
			} else {
				ui.Box(c).Fill().Center().Children(func() {
					ui.Text(c, "二维码加载中…").FontSize(12).TextColor(t.Faint)
				})
			}
		})
	})
}

// loginQRCardPaint 是登录二维码卡片（原版 `.login-qr-card`）的底 / 边 / 影。
//
//	.login-qr-card {
//	    background: rgba(255,255,255,.95);
//	    border: 1px solid rgba(255,255,255,.6);
//	    box-shadow: inset 0 1px 0 rgba(255,255,255,1),
//	                0 8px 24px rgba(15,23,42,.1),
//	                0 1px 2px rgba(15,23,42,.06);
//	}
//
// 原生版此前画成了**纯白实底 + 单条 Tailwind `shadow-lg` 的近似**（0 10 15 墨
// .12），少了原版的 1px 白描边、顶部内高光与第二条贴身暗影。卡片就贴在同样
// 近白的登录玻璃面板上，没有那条 1px 白边和顶部内高光时，卡与面板糊成一片、
// 看不出卡片的边界（用户可感知）。这里逐值对齐原版，抽成纯函数便于单测。
type loginQRCardPaintSpec struct {
	BG     ui.Color
	Border ui.Color
	Inset  []shadowSpec
	Outer  []shadowSpec
}

func loginQRCardPaint() loginQRCardPaintSpec {
	return loginQRCardPaintSpec{
		BG:     ui.Hex("#ffffff").Alpha(0.95),
		Border: ui.Hex("#ffffff").Alpha(0.60),
		Inset:  []shadowSpec{{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(1)}},
		Outer: []shadowSpec{
			{0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.10)},
			{0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.06)},
		},
	}
}

// modalMessage 是通用提示（关于 / 快捷键 / 消息）。
func (a *App) modalMessage(c *ui.Context, m *Modal, t Theme) {
	st := dialogTypeStyleFor(m.Type)
	// 原版 DialogProvider 的内容块：`flex items-start gap-4 mb-5` —— 左侧 28px
	// 类型图标（mt-0.5），右侧标题 + 正文。图标不是装饰：原版每种对话框（关于 /
	// 快捷键 / 更新成功 / 更新失败 / 发现新版本 / 检查更新中）都带它。
	ui.Row(c).Gap(16).AlignItems(ui.Start).Margin(0, 0, 20, 0).Children(func() {
		if st.Spinner {
			a.dialogSpinner(c, st.Ink)
		} else {
			ui.Icon(c, st.Icon).Size(28, 28).Shrink(0).TextColor(st.Ink).
				Margin(2, 0, 0, 0)
		}
		ui.Column(c).Children(func() {
			// .dialog-title：16px / 600 / #1e293b，下间距 8。
			ui.Text(c, m.Title).FontSize(16).FontWeight(600).
				TextColor(ui.Hex("#1e293b")).Margin(0, 0, 8, 0)
			if m.Message != "" {
				// .dialog-message：13.5px / #475569 / `line-height:1.7`
				//（globals.css:5679，即 13.5×1.7=22.95px）；多行文本按行拆开逐行画
				//（原生 Text 不处理 \n，空行保留成一条只占行高的空 Text）。
				// 行高必须显式给：框架默认取字体自然行盒（13.5px 时约 20px），
				// 比原版每行少约 3px，多行/空行会明显更挤（`\n\n` 的段间距尤其）。
				ui.Column(c).Children(func() {
					for _, line := range splitLines(m.Message) {
						ui.Text(c, line).FontSize(13.5).LineHeight(1.7).
							TextColor(ui.Hex("#475569"))
					}
				})
			}
			if m.LinkURL != "" {
				ui.Row(c).Margin(8, 0, 0, 0).Children(func() {
					a.dialogLink(c, m.LinkLabel, m.LinkURL)
				})
			}
		})
	})

	// 底部按钮：原版默认是单个主按钮「确定」，调用点可以换成自己的文案 / 编号。
	btns := m.Buttons
	if len(btns) == 0 {
		btns = []ModalButton{{Label: "确定", Primary: true}}
	}
	ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
		for _, btn := range btns {
			b := btn
			a.modalButton(c, b, func() {
				if b.OnClick != nil {
					b.OnClick()
					return
				}
				if a.Act.CloseModal != nil {
					a.Act.CloseModal()
				}
			})
		}
	})
}

// dialogSpinner 是原版 `.dialog-loading-spinner`（globals.css:5634）的自绘：
//
//	width/height 28px; border: 3px solid rgba(14,165,233,.15);
//	border-top-color: #0ea5e9; border-radius: 50%;
//	animation: dialog-spin .7s linear infinite
//
// 即「3px 浅蓝整圈 + 深蓝顶弧」的圆环，0.7s 线性转一圈。mygo 的 ui.Spinner 画的是
// 12 根渐隐辐条（中心密、边缘疏），与原版不是一回事；这里按原版自绘。
//
// 原版 border-top-color 上色的「顶弧」是圆环的上 1/4（方框顶边圆角后落在
// 225°→315° 那段）。圆环路径从右侧（0）起、顺时针一圈为 1，顶弧即
// [0.625, 0.875)；旋转就是把这个区间整体沿圆周平移。
func (a *App) dialogSpinner(c *ui.Context, ink ui.Color) {
	ringSpinner(c, ink, 28, 3, "dialog-spin", 2, 0, 0, 0)
}

// ringSpinner 是「细环 + 顶弧」转圈指示器的共用实现（原版 dialog-spin 关键帧
// 之下两处用它：对话框 `.dialog-loading-spinner` 28/3，搜索键内联 spinner
// `.search-submit-spinner` 14/2，都是 0.7s linear infinite）。参数化尺寸与描边
// 宽度，几何按原版 border 模型算（描边全在盒内、中心线半径 = (边长 − 描边)/2）。
// margin 是原版各处不同的外边距（对话框 mt-0.5 = 2，搜索键 0），key 前缀让
// 多处同时转圈时各自一条 Loop。
func ringSpinner(c *ui.Context, ink ui.Color, size, ring float32, key string,
	marginTop, marginRight, marginBottom, marginLeft float32) ui.Element {
	el := ui.Box(c).Size(size, size).Shrink(0)
	if marginTop != 0 || marginRight != 0 || marginBottom != 0 || marginLeft != 0 {
		el.Margin(marginTop, marginRight, marginBottom, marginLeft)
	}
	// 原版 dialog-spin 0.7s linear infinite：Loop 给 0→1 的线性进度，逐帧算角度。
	ph := float64(el.Loop(key, 700*time.Millisecond, ui.Linear))
	el.Draw(func(p *ui.Painter, r ui.Rect) {
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		// border 在盒内：圆环中心线半径 = (边长 − 描边) / 2。
		rad := (min(r.W, r.H) - ring) / 2
		var circle ui.Path
		circle.Circle(cx, cy, rad)
		p.StrokePath(&circle, ring, ui.Hex("#0ea5e9").Alpha(0.15))
		start, end, wrap := dialogSpinnerArc(ph)
		if !wrap {
			var arc ui.Path
			arc.Circle(cx, cy, rad).Trim(start, end)
			p.StrokePath(&arc, ring, ink)
			return
		}
		// 旋转越过起点（0）时拆成两段：start→1 与 0→end。
		var tail ui.Path
		tail.Circle(cx, cy, rad).Trim(start, 1)
		p.StrokePath(&tail, ring, ink)
		var head ui.Path
		head.Circle(cx, cy, rad).Trim(0, end)
		p.StrokePath(&head, ring, ink)
	})
	return el
}

// dialogSpinnerArc 返回顶弧在圆环路径上的 [start,end] 比例区间（0 = 右侧、
// 顺时针一圈为 1；顶弧占上 1/4，即基准 [0.625, 0.875]）。ph 是 0→1 的旋转
// 进度，整体顺时针平移；平移后越过 1 时返回 wrap = true，并已把区间归一到
// [0,1)（end 是拆出来的前半段上界）。抽成纯函数便于单测。
func dialogSpinnerArc(ph float64) (start, end float32, wrap bool) {
	s := math.Mod(0.625+ph, 1)
	if s < 0 {
		s += 1
	}
	e := s + 0.25
	if e > 1 {
		return float32(s), float32(e - 1), true
	}
	return float32(s), float32(e), false
}

// modalButton 是模态框里的按钮（原版 .liquid-dialog-btn / -primary）。
// padding 8 22、圆角 10、13px / 500，悬停上浮 1px、按下缩到 .94（pressPose）。
func (a *App) modalButton(c *ui.Context, btn ModalButton, fn func()) {
	label := btn.Label
	b := ui.ButtonBase(c.Key("modal-" + label)).Height(34).Padding(0, 22).
		Radius(10).Center().Label(label)
	hovered := b.Hovered()
	if btn.Primary {
		// 原版主按钮是纯 135° 蓝渐变（没有任何 backdrop-filter）——用渐变填充
		// 既贴合原版，也少一层 backdrop（这里不需要玻璃）。
		from, to := dialogPrimaryStops(hovered)
		shY, shBlur, shInk := float32(4), float32(12), float32(0.22)
		if hovered {
			shY, shBlur, shInk = 6, 18, 0.30
		}
		b.LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 135}).
			Border(1, ui.Hex("#0ea5e9").Alpha(0.40)).
			InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.40)).
			Shadow(0, shY, shBlur, 0, ui.Hex("#0ea5e9").Alpha(shInk))
	} else {
		st := dialogSecondaryStyleFor(hovered)
		b.Background(st.Fill).Border(1, st.Border).
			InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.55))
		if st.Shadow.A > 0 {
			b.Shadow(0, 4, 12, 0, st.Shadow)
		}
	}
	pressPose(b)
	b.Children(func() {
		ui.Text(c, label).FontSize(13).FontWeight(500).
			TextColor(pick(btn.Primary, ui.Hex("#ffffff"), dialogSecondaryStyleFor(hovered).Ink))
	})
	if b.Clicked() {
		fn()
	}
}

// dialogSheen 画 .liquid-dialog::after 那层 135° 光泽：
//
//	linear-gradient(135deg, rgba(255,255,255,.35) 0%, transparent 35%,
//	                 transparent 65%, rgba(255,255,255,.1) 100%)
//
// 原版是 `position:absolute; inset:0; pointer-events:none` 的伪元素，盖在内容
// 之上（圆角由父级的 overflow:hidden 裁）。这里同样 Absolute 到父级内边距盒、
// 用 Radius + Clip 裁圆角，整棵子树 PassThrough —— 它盖在正文与按钮上方，
// 只要参与命中测试，面板里点任何东西都会落到它身上。
//
// 一条**多停靠点**渐变整块铺满，不再按 4 条等高带拼（拼法的斜向误差实测最坏
// **38 灰阶**，见 dialog_test.go 的 TestDialogSheenMatchesCSS）。
func dialogSheen(c *ui.Context) { dialogSheenFn(c) }

// dialogSheenFn 是 dialogSheen 的实际绘制（测试可换成旧的「4 条带拼法」做反证）。
var dialogSheenFn = func(c *ui.Context) {
	ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
		Radius(18).Clip().PassThrough().
		LinearGradient(ui.LinearGradient{Angle: 135, Stops: gradStops(dialogSheenStops)})
}

// dialogTypeStyle 是对话框类型的视觉参数（原版 DialogProvider 的 iconForType /
// accentForType）：左侧 28px 图标的字形与颜色，以及面板玻璃的强调色。
type dialogTypeStyle struct {
	// Icon 是图标字形（Spinner 为真时不用）。
	Icon *ui.SVG
	// Ink 是图标颜色。
	Ink ui.Color
	// Tint 是面板玻璃的强调色（原版 145° 渐变末尾那个 stop）。
	Tint ui.Color
	// Spinner 为真时画加载转圈，而不是静态图标（原版 type: "loading"）。
	Spinner bool
}

// dialogTypeStyleFor 按原版 iconForType / accentForType 取类型样式。
// 空值与未知值都按 info（原版 default 分支）。
func dialogTypeStyleFor(typ string) dialogTypeStyle {
	switch typ {
	case "success":
		return dialogTypeStyle{Icon: iconCheckOne, Ink: ui.Hex("#22c55e"),
			Tint: ui.Hex("#22c55e").Alpha(0.10)}
	case "error":
		return dialogTypeStyle{Icon: iconCloseOne, Ink: ui.Hex("#ef4444"),
			Tint: ui.Hex("#ef4444").Alpha(0.10)}
	case "warning":
		return dialogTypeStyle{Icon: iconWarn, Ink: ui.Hex("#f59e0b"),
			Tint: ui.Hex("#f59e0b").Alpha(0.10)}
	case "question":
		return dialogTypeStyle{Icon: iconRefresh, Ink: ui.Hex("#0ea5e9"),
			Tint: ui.Hex("#0ea5e9").Alpha(0.10)}
	case "loading":
		return dialogTypeStyle{Spinner: true, Ink: ui.Hex("#0ea5e9"),
			Tint: ui.Hex("#0ea5e9").Alpha(0.06)}
	default: // info
		return dialogTypeStyle{Icon: iconInfo, Ink: ui.Hex("#0ea5e9"),
			Tint: ui.Hex("#0ea5e9").Alpha(0.06)}
	}
}

// dialogSheenStops 是 .liquid-dialog::after 的四个停靠点。
var dialogSheenStops = []gradStop{
	{0, ui.Hex("#ffffff").Alpha(0.35)},
	{0.35, ui.Hex("#ffffff").Alpha(0)},
	{0.65, ui.Hex("#ffffff").Alpha(0)},
	{1, ui.Hex("#ffffff").Alpha(0.10)},
}

// dialogPrimaryStops 是 .liquid-dialog-btn-primary 的 135° 蓝渐变两态。
func dialogPrimaryStops(hovered bool) (ui.Color, ui.Color) {
	if hovered {
		return ui.Hex("#0ea5e9").Alpha(0.95), ui.Hex("#38bdf8").Alpha(0.90)
	}
	return ui.Hex("#0ea5e9").Alpha(0.88), ui.Hex("#38bdf8").Alpha(0.82)
}

// dialogSecondaryStyle 是 .liquid-dialog-btn（非主按钮）的两态填充。
type dialogSecondaryStyle struct {
	Fill, Border, Ink ui.Color
	// Shadow 是悬停时那点外投影（原版 .liquid-dialog-btn:hover 的
	// `0 4px 12px rgba(15,23,42,.08)`）；常态无投影（A == 0）。
	Shadow ui.Color
}

func dialogSecondaryStyleFor(hovered bool) dialogSecondaryStyle {
	if hovered {
		return dialogSecondaryStyle{
			Fill:   ui.Hex("#ffffff").Alpha(0.65),
			Border: ui.Hex("#ffffff").Alpha(0.85),
			Ink:    ui.Hex("#1e293b"),
			Shadow: ui.Hex("#0f172a").Alpha(0.08),
		}
	}
	return dialogSecondaryStyle{
		Fill:   ui.Hex("#ffffff").Alpha(0.45),
		Border: ui.Hex("#ffffff").Alpha(0.50),
		Ink:    ui.Hex("#475569"),
	}
}

// dialogLink 画对话框正文里的可点链接（原版 .dialog-link）。
//
// 原版是 `.dialog-message` 里内联的 <a>：字号继承正文 13.5px，只覆写颜色与底边
// —— 常态 #0284c7 + 1px rgba(2,132,199,.3) 底边、且无 text-decoration；:hover
// 转 #0ea5e9 且底边加深到 rgba(14,165,233,.6)。此前原生版直接用了框架的
// ui.Link：accent 蓝 + 悬停画的是 text-decoration 下划线，颜色与装饰两处都对不上。
//
// 自绘成「文字 + 1px 底边」的 Row，靠 OnClick 走 ui.Context.OpenURL（与 ui.Link
// 同一条打开路径，键盘聚焦后回车/空格也能触发）。
func (a *App) dialogLink(c *ui.Context, label, url string) {
	if url == "" {
		return
	}
	link := ui.Row(c).AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Focusable().BorderWidth(0, 0, 1, 0)
	// 悬停或键盘聚焦（:focus-visible）都算激活态，给键盘用户同样的提示。
	active := link.Hovered() || link.FocusVisible()
	link.BorderColor(dialogLinkEdge(active))
	link.Children(func() {
		ui.Text(c, label).FontSize(13.5).TextColor(dialogLinkInk(active))
	})
	link.OnClick(func() { c.OpenURL(url) })
}

// dialogLinkInk 是链接文字色（原版 .dialog-link / :hover）。
func dialogLinkInk(hovered bool) ui.Color {
	return pick(hovered, ui.Hex("#0ea5e9"), ui.Hex("#0284c7"))
}

// dialogLinkEdge 是链接 1px 底边的颜色（原版 .dialog-link / :hover）：
// 常态 rgba(2,132,199,.30)，悬停 rgba(14,165,233,.60)。
func dialogLinkEdge(hovered bool) ui.Color {
	if hovered {
		return ui.Hex("#0ea5e9").Alpha(0.60)
	}
	return ui.Hex("#0284c7").Alpha(0.30)
}

// splitLines 按换行拆文本，空行保留（用来画段落间距）。
func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}
