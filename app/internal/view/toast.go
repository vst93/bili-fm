package view

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// toastViewport 是自定义的 toast 外观（原版 .toast-glass）：右上角（fixed
// top-4 right-4）、玻璃卡片 + 左侧类型图标 + 文案，自动消失（2s）。
func (a *App) toastViewport(c *ui.Context) {
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		// 原版 toast 在右上角（fixed top-4 right-4），彼此 gap 8。
		// 注意 viewport 由框架固定成 Absolute().Left(0).Top(0).Right(0).Bottom(0)
		// 的全窗容器（Justify(End) 即贴底），所以不能靠 Top/Right 改位置——
		// 要用主轴/交叉轴对齐把它挪到顶部靠右。
		viewport.Justify(ui.Start).AlignItems(ui.End).Padding(16).Gap(8)
		for _, t := range toasts {
			p := ui.ToastBase(c, t)
			accent, ic := toastAccent(t.Type)
			// 原版底色：linear-gradient(135deg, rgba(255,255,255,.5), 类型色 8%)。
			p.Root.Row().Gap(8).Padding(8, 16).Radius(13).
				Material(glass.Glass{}).
				LinearGradient(ui.LinearGradient{
					From:  ui.Hex("#ffffff").Alpha(0.50),
					To:    accent.Alpha(0.08),
					Angle: 135,
				}).
				Border(1, ui.Hex("#ffffff").Alpha(0.55)).
				// 原版 --glass-specular + --glass-shadow-lifted。
				InsetShadow(0, 1, 1, 0.5, ui.Hex("#ffffff").Alpha(0.90)).
				InsetShadow(0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.06)).
				Shadow(0, 14, 40, 0, ui.Hex("#0f172a").Alpha(0.10)).
				AlignItems(ui.Center)
			p.Root.Children(func() {
				ui.Icon(c, ic).Size(20, 20).Shrink(0).TextColor(accent)
				// 原版：text-sm（14）font-medium text-slate-800。
				ui.Text(c, t.Title).FontSize(14).FontWeight(500).
					TextColor(ui.Hex("#1e293b"))
			})
		}
	})
}

// toastAccent 把 toast 类型映射成原版的图标与颜色
// （success 勾 / error 叉 / warning 感叹 / 默认信息）。
func toastAccent(typ string) (ui.Color, *ui.SVG) {
	switch typ {
	case "error":
		return ui.Hex("#f43f5e"), iconClose
	case "warning":
		return ui.Hex("#f59e0b"), iconWarn
	case "success":
		return ui.Hex("#22c55e"), iconCheck
	default:
		return ui.Hex("#0ea5e9"), iconInfo
	}
}
