package view

import (
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// toastDuration 是每条 toast 的停留时长：原版 src/utils/toast.ts 的
// `duration = 2000`（框架默认是 4s，所以这里显式覆盖）。
const toastDuration = 2 * time.Second

// toastEnterDuration 是 toast 入场过渡的时长（原版 `duration-300`）。声明成
// var 便于无头测试把它放长：无头测试拿不到 mygo 的测试时钟，固定 300ms 会在
// 一次 settle（最多 20 帧）里被墙钟跑完，导致「首帧仍透明」的采样偶发假红
// （负载抖动）。放长到远大于一次 settle 的墙钟耗时，首帧就稳定停在 Enter 的
// 起点（见 TestToastFadesIn）。
var toastEnterDuration = 300 * time.Millisecond

// toastTransition 是原版根的 `transition-all duration-300` + `-translate-y-2`
// （Tailwind 0.5rem = 8px，向上）：入场从上方 8px 淡入、退场向上一路淡出。
// 声明成 var 便于测试换桩做反证（去掉 Enter = 不再淡入，首帧即显形）。
var toastTransition = func() ui.ElementTransition {
	return ui.ElementTransition{
		Duration: toastEnterDuration,
		Enter:    &ui.Motion{Y: -8, Opacity: 0},
		Exit:     &ui.Motion{Y: -8, Opacity: 0},
	}
}

// toastRadius 是 .toast-glass 的 `border-radius: 13px`。
const toastRadius = 13

// 注意 toast 的原版**没有**玻璃底色：Toast.tsx 的内联 `style.background` 是
// 简写，把 `.toast-glass` 的 `background: var(--glass-bg)`（白 .24）一起顶掉了，
// 只剩 `backdrop-filter: var(--glass-blur)`（blur 20px）+ 内联渐变。所以这里
// 必须用 glass.Blur（只模糊、不染色/不提亮）+ 内联渐变，不能用 glass.Glass{}——
// 后者是 Regular 材质，会把玻璃下的东西「frosts and lightens」，整块比原版白
// 一档（实测右下角比背景亮 +18.5 灰阶，改后 +2.1）。

// toastSpecularInsets 是 .toast-glass 的 `box-shadow: var(--glass-specular)` 那两层
// 内影（globals.css:964）：顶缘 1px 白 .9 的聚光 + 下缘 1px 深 slate .06 的暗线。
// 框架玻璃只在 1px 环上做边缘折光，顶不到 .9 的高光，也没有那条下缘暗线，
// 此前 toast 只接了 --glass-shadow-lifted 的外投影，整块比原版平一档。
func toastSpecularInsets() [2]ui.Color {
	return [2]ui.Color{
		ui.Hex("#ffffff").Alpha(0.9),  // inset 0 1px 1px .5px var(--glass-highlight)
		ui.Hex("#0f172a").Alpha(0.06), // inset 0 -1px 1px rgba(15,23,42,.06)
	}
}

// toastFillStops 是原版根元素的内联背景
// `linear-gradient(135deg, rgba(255,255,255,0.5), accentForType(type))`，
// 其中 accentForType 是类型色 8%。135°：左上白、右下透出类型色。
func toastFillStops(tint ui.Color) (ui.Color, ui.Color) {
	return ui.Hex("#ffffff").Alpha(0.5), tint.Alpha(0.08)
}

// toastViewport 是自定义的 toast 外观（原版 .toast-glass）：右上角（fixed
// top-4 right-4）、玻璃卡片 + 135° 类型渐变 + 左侧类型图标 + 文案，2s 后
// 自动消失（退场由 toastTransition 上滑淡出）。
func (a *App) toastViewport(c *ui.Context) {
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		// 原版 toast 在右上角（fixed top-4 right-4），彼此 gap 8。
		// 注意 viewport 由框架固定成 Absolute().Left(0).Top(0).Right(0).Bottom(0)
		// 的全窗容器（Justify(End) 即贴底），所以不能靠 Top/Right 改位置——
		// 要用主轴/交叉轴对齐把它挪到顶部靠右。
		viewport.Justify(ui.Start).AlignItems(ui.End).Padding(16).Gap(8)
		for _, t := range toasts {
			p := ui.ToastBase(c, t)
			tint, ic := toastAccent(t.Type)
			from, to := toastFillStops(tint)
			// 入场/退场：原版是 CSS transition（上方 8px、300ms）。
			p.Root.Transition(toastTransition())
			// 材质用「只模糊不染色」的 glass.Blur 打底（原版 .toast-glass 的
			// `backdrop-filter: blur(20px) saturate(1.45)`；saturate 框架没有对应
			// 项）。**不能**用 glass.Glass{}：原版 Toast.tsx 的内联 background
			// 把 .toast-glass 的白 .24 底色一起顶掉了，Regular 玻璃的提亮会让整块
			// 比原版白一档（这正是 11-07 剩余差异 ④）。
			p.Root.Row().Gap(8).Padding(8, 16).Radius(toastRadius).
				Material(blurLayer(glass.Blur{Radius: glassBlurSigma})).
				Border(1, a.Theme.GlassBorder).
				Shadow(0, 14, 40, 0, ui.Hex("#0f172a").Alpha(0.10)).
				AlignItems(ui.Center)
			ins := toastSpecularInsets()
			p.Root.Children(func() {
				ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
					Radius(toastRadius).PassThrough().
					LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 135})
				ui.Icon(c, ic).Size(20, 20).Shrink(0).TextColor(tint)
				// 原版：text-sm（14）font-medium text-slate-800。
				ui.Text(c, t.Title).FontSize(14).FontWeight(500).
					TextColor(ui.Hex("#1e293b"))
				// `.toast-glass::after`（globals.css:5345）的 `--glass-sheen-static`：
				// 左上径向热点（白 .24 @22% -5%，48% 归零）+ 170° 斜向洗光。
				// 伪元素是**定位**元素，CSS 的绘制顺序把它排在流式子元素（图标 /
				// 文案）之后 —— 所以这层必须建成最后一个 Absolute 子盒，不能像
				// 设置菜单那样用面板自己的 Draw（Draw 在子元素之前）。
				// 原版 `.toast-glass` 的 `--glass-specular` 两层内影也挂在这一层：
				// CSS 的 inset box-shadow 画在 background-image 之上、内容之下，
				// 而我们的类型渐变是子盒（在元素自身的 inset 之上），所以内影必须
				// 跟着这层「最上面」的 Absolute 子盒走，否则会被渐变盖掉（实测顶行
				// 反而比内部暗 8 luma）。它只占 1~2px 的缘，压不到文案。
				ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
					Radius(toastRadius).PassThrough().
					InsetShadow(0, 1, 1, 0, ins[0]).
					InsetShadow(0, -1, 1, 0, ins[1]).
					Draw(func(p *ui.Painter, r ui.Rect) {
						paintBakedSheen(p, r, toastRadius, sheenGlassBaked)
					})
			})
		}
	})
}

// toastAccent 把 toast 类型映射成原版 utils/toast.ts 的图标与颜色
// （success 勾 / error 叉 / warning 感叹 / 默认信息）。
func toastAccent(typ string) (ui.Color, *ui.SVG) {
	switch typ {
	case "error":
		return ui.Hex("#f43f5e"), iconCloseOne
	case "warning":
		return ui.Hex("#f59e0b"), iconWarn
	case "success":
		return ui.Hex("#22c55e"), iconCheckOne
	default:
		return ui.Hex("#0ea5e9"), iconInfo
	}
}
