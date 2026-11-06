package view

import "github.com/egoist/mygo/ui"

// 禁用态的置灰：原版 CSS 的 `:disabled` 不透明度是**绝对值**，
//
//	.nav-icon-btn:disabled                     .34   （globals.css 里这条规则
//	                                                  出现两次：.3 与 .34，
//	                                                  源序在后的 .34 生效）
//	#player .player-button:disabled            .42
//	#player input[type="range"]:disabled       .42
//	.watchlater-remove-btn:disabled            .55
//	.mini-window-controls button:disabled      .34
//	.search-submit-btn:disabled                不改（搜索中仍是常态 .82）
//	.top-tool-btn:disabled                     不改
//
// mygo 0.7.x 绘制**禁用节点**时会把该节点及其整棵子树的不透明度再乘 0.5
// （ui/paint.go：`if e.flags&flagDisabled != 0 { p.opacity *= 0.5 }`），而元素
// 不透明度封顶 1，所以此前只能「目标 ≤ .5 时写 target/0.5 抵消」（disabledDim），
// 目标 > .5 的两处（search-submit .82 / wl-remove .55）补不回来，只能放弃框架的
// Disabled、自己挡住交互 —— 代价是那两处没有 `:disabled` 的其余语义（焦点、
// 悬停时的 cursor 等）。
//
// mygo **v0.8.0 新增 `Element.DimDisabled(false)`**：禁用节点保持自身与子树
// 的不透明度，由调用方自己写禁用态的样子。于是这层补偿全部删掉：
//
//	disableNav / disablePlayerButton / 进度条 / 迷你窗控制键
//	    → Disabled(true) + DimDisabled(false) + 直接写原版的绝对值
//	稍后再看的移除键（.55）
//	    → 同上，不再需要「自己挡住交互」的绕法
//	搜索提交键（.82）
//	    → 仍**不用**框架 Disabled：原版 `.search-submit-btn:disabled` 照样吃
//	      `:hover`（底色 / 图标色都变），而 mygo 的 Hovered() 对禁用元素恒假。
//	      这条例外与不透明度无关了（DimDisabled(false) 已能保住 .82）。

// navIconDisabledOpacity 是 `.nav-icon-btn:disabled` 要落到画布上的绝对值。
const navIconDisabledOpacity = 0.34

// playerButtonDisabledOpacity 是 `#player .player-button:disabled` 要落到画布上的
// 绝对值（原版 globals.css）。
const playerButtonDisabledOpacity = 0.42

// miniControlDisabledOpacity 是 `body.mini-mode .mini-window-controls
// button:disabled` 的绝对值。
const miniControlDisabledOpacity float32 = 0.34

// setDisabledOpacity 把元素设成禁用态并直接铺原版的**绝对**不透明度：
// `DimDisabled(false)` 关掉框架那层减半，`Opacity` 就是最终落到画布上的值。
// target 传 0 表示「原版这一处不改不透明度」，只禁用不置灰。
func setDisabledOpacity(b ui.Element, off bool, target float32) ui.Element {
	b.Disabled(off).DimDisabled(false)
	if off && target > 0 {
		b.Opacity(target)
	}
	return b
}

// navIconOpacityTarget 给出 `.nav-icon-btn` 这一帧该走的不透明度目标：常态是族级
// 透明度，禁用是原版 `.nav-icon-btn:disabled` 的 .34（绝对值，无需补偿）。
// 拆成纯函数便于测试直接钉住「禁用目标是真的 .34，而不是 .3 的近似」。
func navIconOpacityTarget(off bool, rest float32) float32 {
	if off {
		return navIconDisabledOpacity
	}
	return rest
}
