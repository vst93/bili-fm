package view

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// modalIn 是模态框入场（缩放 + 淡入）的时长。
const modalIn = 180 * time.Millisecond

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
			panel.Width(320).Radius(16).
				Padding(32, 24, 24).
				Material(glass.Glass{}).
				Background(ui.Hex("#ffffff").Alpha(0.28)).
				Border(1, ui.Hex("#ffffff").Alpha(0.40)).
				InsetShadow(0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.30)).
				Shadow(0, 8, 32, 0, ui.Hex("#0f172a").Alpha(0.20)).
				Shadow(0, 2, 8, 0, ui.Hex("#0f172a").Alpha(0.14))
		default:
			// 原版 .liquid-dialog：min-width 360、padding 28 32 22、圆角 18、
			// 1px 白边 .65 + 四层阴影，底色是 145° 白→浅白的渐变。
			panel.MinWidth(360).Radius(18).
				Padding(28, 32, 22).
				// 原版这里是 backdrop-filter: blur(24px) 的一层玻璃；用框架玻璃后
				// 不再手搓底色与内影（Material 之后调 Background 是空操作，会被
				// 静默忽略；LinearGradient 更会把 Material 整个顶掉）。
				Material(glass.Glass{}).
				Shadow(0, 24, 64, 0, ui.Hex("#0f172a").Alpha(0.22)).
				Shadow(0, 4, 16, 0, ui.Hex("#0f172a").Alpha(0.06))
		}
		prog := float32(1)
		if age := time.Since(a.modalOpenedAt); age < modalIn {
			prog = float32(age) / float32(modalIn)
		}
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

// modalLogin 是扫码登录面板（原版 .login-glass + index.tsx 的登录块）。
func (a *App) modalLogin(c *ui.Context, m *Modal, t Theme) {
	// 关闭键：原版是 absolute -right-4 top-4 的 32 圆键，图标 18px #475569。
	close := ui.ButtonBase(c.Key("login-close")).Size(32, 32).Radius(RadiusPill).
		Center().Label("关闭登录").Tooltip("关闭登录")
	close.Absolute().Right(-16).Top(16)
	if close.Hovered() {
		close.Background(ui.Hex("#0f172a").Alpha(0.08))
	}
	close.Children(func() {
		ui.Icon(c, iconClose).Size(18, 18).TextColor(ui.Hex("#475569"))
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
		// .login-qr-card：padding 12、圆角 12、shadow-lg；里面是 160 的二维码。
		card := ui.Box(c).Size(184, 184).Radius(12).Padding(12).
			Background(ui.Hex("#ffffff")).
			Shadow(0, 10, 15, 0, ui.Hex("#0f172a").Alpha(0.12))
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

// modalMessage 是通用提示（关于 / 快捷键）。
func (a *App) modalMessage(c *ui.Context, m *Modal, t Theme) {
	// .dialog-title：16px / 600 / #1e293b，下间距 8。
	ui.Text(c, m.Title).FontSize(16).FontWeight(600).
		TextColor(ui.Hex("#1e293b")).Margin(0, 0, 8, 0)
	if m.Message != "" {
		// .dialog-message：13.5px / #475569；多行文本按行拆开逐行画
		//（原生 Text 不处理 \n）。
		ui.Column(c).Margin(0, 0, 20, 0).Children(func() {
			for _, line := range splitLines(m.Message) {
				ui.Text(c, line).FontSize(13.5).TextColor(ui.Hex("#475569"))
			}
		})
	}
	if m.LinkURL != "" {
		ui.Row(c).FillWidth().Margin(0, 0, 20, 0).Children(func() {
			ui.Link(c, m.LinkLabel, m.LinkURL)
		})
	}
	ui.Row(c).FillWidth().Justify(ui.End).Gap(8).Children(func() {
		a.modalButton(c, "好的", true, func() {
			if a.Act.CloseModal != nil {
				a.Act.CloseModal()
			}
		})
	})
}

// modalButton 是模态框里的按钮（原版 .liquid-dialog-btn）。
// 主按钮是 135° 蓝色渐变 + 白字；普通按钮是半透明白底 + 1px 白边。
func (a *App) modalButton(c *ui.Context, label string, primary bool, fn func()) {
	t := a.Theme
	// padding 8 22、圆角 10、13px / 500。
	b := ui.ButtonBase(c.Key("modal-"+label)).Height(34).Padding(0, 22).
		Radius(10).Center().Label(label)
	// 材质走框架玻璃：主按钮用较实的蓝色 Tint 表达"强调"，普通按钮是素玻璃。
	// （原版主按钮是 135° 蓝渐变 —— 那是手搓玻璃的做法，换成 Tint 更贴框架材质。）
	if primary {
		// Tint 要给足：玻璃本身会把颜色稀释，0.5 时白字对比度不够。
		b.Material(glass.Glass{Tint: pick(b.Hovered(), t.Blue.Alpha(0.85), t.Blue.Alpha(0.75))}).
			Shadow(0, 4, 12, 0, ui.Hex("#0ea5e9").Alpha(0.22))
	} else {
		buttonSurface(b, false, b.Hovered(),
			ui.Color{}, ui.Hex("#ffffff").Alpha(0.30))
	}
	pressPose(b)
	b.Children(func() {
		ui.Text(c, label).FontSize(13).FontWeight(500).
			TextColor(pick(primary, ui.Hex("#ffffff"), ui.Hex("#475569")))
	})
	if b.Clicked() {
		fn()
	}
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
