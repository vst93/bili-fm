package view

import (
	"sync/atomic"

	"github.com/egoist/mygo/plugins/glass"
)

// backdropLayers 统计视图自上次清零以来构建出的「读 backdrop 的玻璃层」数量。
//
// 为什么需要它：mygo 的渲染器对每个 `Backdrop: true` 的效果都要单独断一次渲染
// 通道 + 回读下采样 + 两次模糊（gpu/gpu.go 的 addEffect、metal 的 readBackdrop），
// 层数直接乘进每帧成本。上一轮实测（见 .pending-bugs.md）正是靠「降层数」把
// 抽屉滚动/入场从 ~481/662ms 拉下来。这里把它做成可断言的数字，防止以后有人
// 又给卡片/按钮盖真玻璃。
//
// 计数发生在**构建期**（视图这一帧建了几个玻璃层），不是绘制期——绘制期会随
// shadow/重试重复调用，构建期才等于「渲染器这一帧要读几个 backdrop」。
var backdropLayers atomic.Int32

// glassLayer 记一层 Liquid Glass 并原样返回。所有 `glass.Glass{}` 的
// `Material(...)` 都应当经它包一层。
func glassLayer(g glass.Glass) glass.Glass {
	backdropLayers.Add(1)
	return g
}

// BackdropLayers 返回并清零自上次调用以来构建的 backdrop 层数，供测试断言。
func BackdropLayers() int { return int(backdropLayers.Swap(0)) }
