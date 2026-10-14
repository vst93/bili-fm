package view

import "github.com/egoist/mygo/ui"

// 键盘焦点环（原版各组件 CSS 的 `:focus-visible`）。
//
// 原版是浏览器应用：Tab 到某个控件时，CSS 的 `:focus-visible` 给它画一圈
// 蓝色描边（`outline` 或 `box-shadow`），有的还顺带改图标色 / 整圈不透明度。
// 原生版此前一处都没接：mygo 会给聚焦的元素画自己的默认环（2px、离盒 3px、
// 主题 Focus 色 —— 见 ui/paint.go 的 Painter.FocusRing），颜色与偏移都跟原版
// 对不上，标题栏 / 收藏滚动键那种「聚焦时图标转蓝、铺一层内高光」的整态差异
// 更是完全没有。
//
// 这里把原版各处 `:focus-visible` 的描边环收成一份 focusRing 取值 + 一个
// 绘制入口：聚焦时关掉框架默认环、按原版的宽度 / 颜色 / outline-offset 自己画
// 一圈。原版没写 `:focus-visible` 的控件保持框架默认环（浏览器自身也有一条
// 默认 outline，两者语义一致）；写了 `:focus-visible` 的一律接线——哪怕只是
// 颜色/偏移的差异（框架是 2px、离盒 3px、主题 Focus 色，原版是 2px、离盒
// 2px、--studio-blue 系）。
//
// 描边几何对齐 CSS 的 outline：`outline-offset` 是描边**内缘**离盒边的距离
// （正数在外、负数画进盒内），宽度从内缘往外量。Painter.Stroke 画的是「贴
// rect 内缘、宽 width」的一圈，所以把 rect 按 `offset+width` 外扩、圆角同步
// 加同样的量，就得到 CSS 的 outline。
type focusRing struct {
	Color  ui.Color // outline color
	Width  float32  // outline-width
	Offset float32  // outline-offset（负数表示画进盒内）
}

var (
	// `outline: 2px solid var(--studio-blue); outline-offset: 2px`
	// （#player .player-button、#video-owner-name、合作 UP 主选择键）。
	focusStudio = focusRing{ui.Hex("#0284c7"), 2, 2}
	// `.home-user-button`：同样的描边，offset 3。
	focusStudioWide = focusRing{ui.Hex("#0284c7"), 2, 3}
	// 合作 UP 主下拉条目：`outline: 2px solid var(--studio-blue);
	// outline-offset: -2px`（画进条目里，免得被面板圆角裁掉）。
	focusStudioInset = focusRing{ui.Hex("#0284c7"), 2, -2}
	// `.top-tool-btn`：`2px solid rgba(14,165,233,.45)`，offset 2。
	focusSky45 = focusRing{ui.Hex("#0ea5e9").Alpha(0.45), 2, 2}
	// `.watchlater-remove-btn`：`2px solid rgba(14,165,233,.62)`，offset 2。
	focusSky62 = focusRing{ui.Hex("#0ea5e9").Alpha(0.62), 2, 2}
	// `.history-incognito-switch`：`2px solid rgba(14,165,233,.44)`，offset 2。
	focusSky44 = focusRing{ui.Hex("#0ea5e9").Alpha(0.44), 2, 2}
	// 标题栏 / 迷你窗窗口控制键 / 收藏左右滚动键：原版是
	// `box-shadow: … 0 0 0 2px rgba(2,132,199,.4)`（无 offset、无模糊的
	// spread 环，等价于 outline-offset 0）。标题栏 / 迷你窗那几条还各带一层
	// 内高光与换色，由调用方另铺（见 shell.go / mini.go 的 focus 分支）。
	focusGlow = focusRing{ui.Hex("#0284c7").Alpha(0.40), 2, 0}
)

// focusOutline 在原版该控件有 `:focus-visible` 规则时接上它的描边环：
// 关掉 mygo 的默认环（原版这些规则都写 `outline: 0` 或自绘 box-shadow），
// 聚焦时按 ring 自绘一圈。返回 e 便于链式调用。
//
// 只在聚焦那一帧挂 DrawOver：DrawOver 不是子元素，不参与元素 id 的编号
// （mygo 的元素 id 由「父 + 已建子元素个数」决定），所以聚焦 / 失焦来回切
// 不会让同层的兄弟换 id、丢状态。
func focusOutline(e ui.Element, radius float32, ring focusRing) ui.Element {
	e.FocusRing(false)
	if !e.FocusVisible() {
		return e
	}
	return e.DrawOver(func(p *ui.Painter, box ui.Rect) {
		r, rr := focusRingRect(box, radius, ring)
		p.Stroke(r, ring.Color, rr, ring.Width)
	})
}

// focusRingRect 把盒矩形按 CSS outline 的几何换算成 Painter.Stroke 要画的
// 矩形与圆角：`outline-offset` 是描边内缘离盒边的距离，宽度从内缘往外量，
// 所以外扩量是 `offset + width`（负 offset 可以把这个量压到 0 或负值 ——
// 压到 0 时描边正好画在盒的最外圈以内）。圆角同步加同一个量。
//
// 抽成纯函数是为了让测试能逐条钉住各 ring 的几何，不必渲染像素。
func focusRingRect(box ui.Rect, radius float32, ring focusRing) (ui.Rect, float32) {
	n := ring.Offset + ring.Width
	return ui.Rect{X: box.X - n, Y: box.Y - n, W: box.W + 2*n, H: box.H + 2*n}, radius + n
}
