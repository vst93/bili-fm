package view

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// 原版 globals.css 的选集「正在播放」胶囊左侧有一组均衡器小柱
// （.part-playing-eq i + @keyframes part-eq）：三根 2px 宽、圆角 1 的小柱，
// 满高是 10px 行高上的 55% / 100% / 70%（即 5.5 / 10 / 7），以 scaleY 在
// 0.35 ↔ 1 之间按 1.1s ease-in-out 无限循环，三根分别带 -0.4s / -0.8s /
// -0.2s 的负延迟错开相。
//
// 原生版此前只画了三根**静止**的柱子（高度还是 4 / 9 / 6 的估计值），既不动、
// 数值也不对——这轮按 CSS 原样补上（transform-origin 在底部，柱子向上伸缩）。
const (
	// eqPeriod 是均衡器动画一个循环的周期（原版 1.1s）。
	eqPeriod = 1100 * time.Millisecond
	// eqMinScale / eqMaxScale 是 keyframes part-eq 的两端（0%,100% → .35，
	// 50% → 1）。
	eqMinScale = float32(0.35)
	eqMaxScale = float32(1)
)

// eqBarHeights 是三根小柱的满高（原版 10px 行高上的 55% / 100% / 70%）。
var eqBarHeights = [3]float32{5.5, 10, 7}

// eqBarDelays 是三根小柱的 animation-delay（原版 -0.4s / -0.8s / -0.2s；
// 负延迟即相位提前，这里取绝对值当提前量）。
var eqBarDelays = [3]float32{0.4, 0.8, 0.2}

// eqBarScale 返回第 bar 根小柱在循环相位 p（[0,1)）时的 scaleY：先按该柱的
// 延迟把相位提前，再走 part-eq 的 0% → 50% → 100% 两段。CSS 的 ease-in-out
// 是**逐段**生效的（每段从 0 到 1 各缓动一次），所以不能对整程只缓动一次。
func eqBarScale(p float32, bar int) float32 {
	q := fracPhase(p + eqBarDelays[bar]/float32(eqPeriod.Seconds()))
	var u float32
	if q < 0.5 {
		u = q / 0.5
	} else {
		u = (1 - q) / 0.5
	}
	return eqMinScale + (eqMaxScale-eqMinScale)*ui.EaseInOut(u)
}

// fracPhase 取任意实数的小数部分（落到 [0,1)）。
func fracPhase(v float32) float32 {
	v -= float32(int(v))
	if v < 0 {
		v++
	}
	return v
}

// eqBars 画三根均衡器小柱；phase 是循环进度（来自 Loop）。
func eqBars(c *ui.Context, phase float32) {
	ui.Row(c).Gap(1.5).Height(10).AlignItems(ui.End).Children(func() {
		for i, h := range eqBarHeights {
			bar := ui.Box(c).Width(2).Height(h).Radius(1).Background(ui.Hex("#7dd3fc"))
			// transform-origin 在底边中点（原版 .part-playing-eq i 的
			// transform-origin: bottom），scaleY 让柱子向上伸缩。
			bar.TransformOrigin(0.5, 1).Scale(1, eqBarScale(phase, i))
		}
	})
}

// refreshSpinPeriod 是历史抽屉刷新键「转圈」一圈的时长（原版
// .history-refresh-spinning 的 800ms linear infinite）。
const refreshSpinPeriod = 800 * time.Millisecond

// spinDegrees 返回刷新图标的旋转角：刷新中（active）按进度匀速转一整圈，
// 静止时不转（原版只在 isRefreshing 时挂 history-refresh-spinning）。
func spinDegrees(active bool, progress float32) float32 {
	if !active {
		return 0
	}
	return 360 * progress
}
