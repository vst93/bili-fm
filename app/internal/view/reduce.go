package view

import "github.com/egoist/mygo/ui"

// reduceMotion 报告桌面是否要求「减少动态效果」——macOS 的 Reduce Motion、
// Windows 关闭动画效果、GNOME 关闭动画，也就是原版 globals.css 里那六个
// `@media (prefers-reduced-motion: reduce)` 块的判据。
//
// 框架已经替原生版兜住了大部分：Animate / AnimateTo / AnimatePose 的位置位移
// 与 ElementTransition 的位移在 ReduceMotion 下会直接到位（ui/pose.go、
// ui/motion.go）。但有两类它按设计照常运行，必须由视图层自己接：
//
//  1. Loop()（ui/widgets.go）——它是「还在转＝还在忙」的指示，不随
//     ReduceMotion 停。原生版里对应历史刷新键的自转。
//  2. AnimateColor 的颜色过渡（ui/motion.go 明确写了「colors still change
//     smoothly while the desktop asks for less motion」）——原版对播放栏
//     与弹幕那两组选择器写了 `transition: none`，颜色也该当帧到位。
//
// 另外唱片自转的**目标角**每帧都在变（AnimatePose 在 ReduceMotion 下会瞬移到
// 目标，于是变成一秒一跳），原版是 `.record-disc { animation: none }`，所以
// 也得在目标角上单独处理（见 discTargetAngle / App.resetDisc）。
func reduceMotion(c *ui.Context) bool {
	return c.Preferences().ReduceMotion
}
