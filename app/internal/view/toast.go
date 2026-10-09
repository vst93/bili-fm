package view

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// toastViewport 是自定义的 toast 外观（原版 .toast-glass）：白色玻璃卡片 +
// 左侧类型色条 + 关闭键，悬在播放栏上方居中。mygo 默认的 toast 太淡，这里
// 换成和旧版一致的样式。
func (a *App) toastViewport(c *ui.Context, bottom float32) {
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Bottom(bottom).AlignItems(ui.Center).Gap(8)
		for _, t := range toasts {
			p := ui.ToastBase(c, t)
			accent := toastAccent(t.Type)
			root := p.Root
			root.Row().Gap(10).Padding(9, 14).Radius(10).
				Material(glass.Glass{}).
				Border(1, ui.Hex("#ffffff").Alpha(0.8)).
				Shadow(0, 10, 30, 0, shadowInk.Alpha(0.16)).
				AlignItems(ui.Center)
			root.Children(func() {
				ui.Box(c).Width(3).Height(16).Shrink(0).Radius(2).Background(accent)
				ui.Text(c, t.Title).FontSize(12).TextColor(ui.Hex("#1e293b"))
				close := p.CloseButton().Size(18, 18).Radius(RadiusPill).Center().
					Label("关闭通知")
				close.Background(ui.Transparent)
				close.Children(func() {
					ui.Icon(c, iconClose).Size(11, 11).TextColor(ui.Hex("#94a3b8"))
				})
			})
		}
	})
}

// toastAccent 把 toast 类型映射成左侧色条颜色。
func toastAccent(typ string) ui.Color {
	switch typ {
	case "error":
		return ui.Hex("#ef4444")
	case "warning":
		return ui.Hex("#f59e0b")
	case "success":
		return ui.Hex("#22c55e")
	default:
		return ui.Hex("#0ea5e9")
	}
}
