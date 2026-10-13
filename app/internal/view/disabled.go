package view

// mygo 在绘制**禁用节点**时，会把该节点及其整棵子树的不透明度再乘 0.5
// （ui/paint.go：`if e.flags&flagDisabled != 0 { p.opacity *= 0.5 }`）。
// 而原版 CSS 的 `:disabled` 不透明度是**绝对值**，不是「在常态上再减半」：
//
//	.nav-icon-btn:disabled                     .34   （globals.css 里这条规则
//	                                                  出现两次：.3 与 .34，
//	                                                  源序在后的 .34 生效）
//	#player .player-button:disabled            .42
//	.watchlater-remove-btn:disabled            .55
//	.search-submit-btn:disabled                不改（搜索中仍是常态 .82）
//	.top-tool-btn:disabled                     不改
//
// 原生版此前是「每处各补一个近似系数」（disableNav 写 .6、disablePlayerButton
// 写 .84），对不上原版的地方就漏了：搜索中的提交键被压到 .41（原版 .82）、
// 稍后再看的移除键被压到 .275（原版 .55），比原版暗一半。这里把这件事收口成
// 一个显式的换算，分两种处理：
//
//	目标 ≤ .5  → 元素不透明度写 disabledDim(目标) = 目标 / 0.5 抵消这次减半；
//	目标 > .5  → 元素不透明度封顶 1，最多只能补到 .5，**补不回来** —— 这两处
//	             （search-submit .82 / wl-remove .55）不能借框架的 Disabled，
//	             改为自己在回调里挡住交互（原版那边同样只是 disabled 防连点）。
const frameworkDisabledDim = 0.5

// disabledDim 返回要写在**禁用元素**上的不透明度，使最终绘制值等于原版的绝对值
// target。target ≥ .5 时补不回来（返回 1，实际只会到 .5），调用点必须改用
// 「不用框架 Disabled」的办法（见 wlRemoveOpacity / searchField 的注释）。
func disabledDim(target float32) float32 {
	if target >= frameworkDisabledDim {
		return 1
	}
	return target / frameworkDisabledDim
}

// navIconDisabledOpacity 是 `.nav-icon-btn:disabled` 要落到画布上的绝对值。
const navIconDisabledOpacity = 0.34

// playerButtonDisabledOpacity 是 `#player .player-button:disabled` 要落到画布上的
// 绝对值（原版 globals.css）。
const playerButtonDisabledOpacity = 0.42

// navIconOpacityTarget 给出 `.nav-icon-btn` 这一帧该走的不透明度目标：常态是族级
// 透明度，禁用是原版 `.nav-icon-btn:disabled` 的 .34（经 disabledDim 抵消框架的
// 减半）。拆成纯函数便于测试直接钉住「禁用目标是真的 .34，而不是 .3 的近似」。
func navIconOpacityTarget(off bool, rest float32) float32 {
	if off {
		return disabledDim(navIconDisabledOpacity)
	}
	return rest
}
