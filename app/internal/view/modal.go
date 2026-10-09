package view

import (
	"fmt"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// modal 画当前模态对话框（原版的 DialogProvider + 登录面板）。
//
// 覆盖层 rgba(15,23,42,0.28) 不模糊，点空白处关闭；面板是白色玻璃卡片、
// 圆角 16、投影 0 24 60。
func (a *App) modal(c *ui.Context) {
	m := a.Modal
	if m == nil {
		return
	}
	t := a.Theme

	overlay := ui.Box(c).Absolute().Fill().Background(ui.Hex("#0f172a").Alpha(0.28))
	overlay.Children(func() {
		ui.Column(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).
			Center().Children(func() {
			panel := ui.Column(c).Width(320).Padding(20).
				Radius(16).Material(glass.Glass{}).
				Border(1, ui.Hex("#ffffff").Alpha(0.70)).
				Shadow(0, 24, 60, 0, shadowInk.Alpha(0.20))
			if m.Kind == "login" {
				panel.Width(340)
			}
			panel.Children(func() {
				switch m.Kind {
				case "login":
					a.modalLogin(c, m, t)
				default:
					a.modalMessage(c, m, t)
				}
			})
		})
	})
	if overlay.Clicked() {
		// 登录框要停掉轮询（closeLogin 会换代），其他框直接关。
		if m.Kind == "login" {
			if a.Act.CloseLogin != nil {
				a.Act.CloseLogin()
			}
		} else if a.Act.CloseModal != nil {
			a.Act.CloseModal()
		}
	}
}

// modalLogin 是扫码登录面板（原版 .login-glass）。
func (a *App) modalLogin(c *ui.Context, m *Modal, t Theme) {
	ui.Box(c).FillWidth().Center().Children(func() {
		ui.Text(c, "使用 B站 App 扫码登录").FontSize(17).Bold().
			TextColor(ui.Hex("#0f172a"))
	})
	ui.Box(c).FillWidth().Center().Margin(4, 0, 14, 0).Children(func() {
		ui.Text(c, "打开手机扫一扫").FontSize(13).TextColor(ui.Hex("#334155"))
	})
	ui.Box(c).FillWidth().Center().Children(func() {
		card := ui.Box(c).Size(184, 184).Radius(12).Padding(12).
			Background(ui.Hex("#ffffff")).
			Border(1, ui.Hex("#94a3b8").Alpha(0.24)).
			Shadow(0, 10, 26, 0, shadowInk.Alpha(0.12))
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
	ui.Box(c).FillWidth().Center().Margin(12, 0, 0, 0).Children(func() {
		a.modalButton(c, "关闭", true, func() {
			if a.Act.CloseLogin != nil {
				a.Act.CloseLogin()
			}
		})
	})
}

// modalMessage 是通用提示（关于 / 快捷键）。
func (a *App) modalMessage(c *ui.Context, m *Modal, t Theme) {
	ui.Text(c, m.Title).FontSize(16).Bold().TextColor(ui.Hex("#0f172a"))
	if m.Message != "" {
		// 多行文本按行拆开，逐行画（原生 Text 不处理 \n）。
		ui.Column(c).Gap(5).Margin(10, 0, 0, 0).Children(func() {
			for _, line := range splitLines(m.Message) {
				ui.Text(c, line).FontSize(13).TextColor(t.Muted)
			}
		})
	}
	ui.Row(c).FillWidth().Justify(ui.End).Margin(16, 0, 0, 0).Children(func() {
		a.modalButton(c, "好的", true, func() {
			if a.Act.CloseModal != nil {
				a.Act.CloseModal()
			}
		})
	})
}

// modalButton 是模态框里的按钮。
func (a *App) modalButton(c *ui.Context, label string, primary bool, fn func()) {
	t := a.Theme
	b := ui.ButtonBase(c.Key("modal-"+label)).Height(34).Padding(0, 18).
		Radius(Radius).Center().Label(label)
	switch {
	case primary:
		b.Background(t.Blue)
		if b.Hovered() {
			b.Background(ui.Hex("#0369a1"))
		}
	default:
		if b.Hovered() {
			b.Background(t.GlassHover)
		} else {
			b.Background(t.GlassActive)
		}
		b.Border(1, t.GlassBorder)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(13).Bold().
			TextColor(pick(primary, ui.Hex("#ffffff"), t.Muted))
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

// fmtBytes 把字节数格式化成 B / KB / MB（原版 formatBytes）。
func fmtBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
