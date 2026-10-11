package view

import (
	"time"

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
			// 淡入淡出：框架自带的那套只在内置外观里做，我们用了自定义外观
			// 就得自己来。ToastParts 只公开了 Left（剩余时间），首现时间自己记。
			op := p.Root.AnimateTo("toast-in", toastFadeTarget(a, p), ui.Animation{Duration: toastFade})
			// 材质用框架玻璃，类型色只作为 Tint 透出来 —— 原版那层
			// 135° 渐变（白 50% → 类型色 8%）是想手搓玻璃，而
			// LinearGradient 会直接把 Material 顶掉（fill 只有一个）。
			// 注意：Material 之后不能再调 Background/LinearGradient。
			p.Root.Row().Gap(8).Padding(8, 16).Radius(13).
				Material(glass.Glass{Tint: accent.Alpha(0.10)}).
				Shadow(0, 14, 40, 0, ui.Hex("#0f172a").Alpha(0.10)).
				AlignItems(ui.Center)
			p.Root.Opacity(op)
			p.Root.Children(func() {
				ui.Icon(c, ic).Size(20, 20).Shrink(0).TextColor(accent)
				// 原版：text-sm（14）font-medium text-slate-800。
				ui.Text(c, t.Title).FontSize(14).FontWeight(500).
					TextColor(ui.Hex("#1e293b"))
			})
		}
	})
}

// toastFade 是 toast 淡入淡出的时长（框架内置外观用的是 200ms）。
const toastFade = 200 * time.Millisecond

// toastFadeTarget 算出某条 toast 这一帧该有的不透明度：
// 前 200ms 由 0 升到 1，最后 200ms 再由 1 落回 0。
// 交给 AnimateTo 平滑，框架会自己把中间的帧排上。
func toastFadeTarget(a *App, p ui.ToastParts) float32 {
	id := p.Root.ID()
	now := time.Now()
	a.toastSeenMu.Lock()
	if a.toastSeen == nil {
		a.toastSeen = map[uint64]time.Time{}
	}
	first, ok := a.toastSeen[id]
	if !ok {
		first = now
		a.toastSeen[id] = first
	}
	a.toastSeenMu.Unlock()

	if age := now.Sub(first); age < toastFade {
		return float32(age) / float32(toastFade)
	}
	if left := p.Left(); left > 0 && left < toastFade {
		return float32(left) / float32(toastFade)
	}
	return 1
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
