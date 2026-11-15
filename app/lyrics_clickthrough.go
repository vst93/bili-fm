package main

// 桌面字幕「穿透，但设置键可点」的统一实现。
//
// mygo 0.9.0 的 `Window.SetInputRegion` 让窗口**只在给定矩形里收点击**，其余
// 部分把指针让给下面的窗口 —— 正是这条需求要的形态，三平台同一个调用。所以
// 三平台都走它：输入区域 = 右上角设置键那一块（几何唯一来源是
// lyricsGearShapeRect，与绘制出来的设置键同源）。
//
// mygo 0.9.1（#1758）补了两件事，本文件的形态跟着收敛：
//
//   - 穿透可以在**创建时**声明（`WindowOptions.IgnoreMouseEvents`）：窗口从
//     建出来到第一次下发输入区域之间（Hidden 创建 → ShowInactive → 同步）
//     不再有一个「整窗收点击」的空档。声明处见 lyrics.go 的窗口选项。
//   - 穿透状态可以**读回**（`Window.IgnoresMouseEvents`）：设完区域后读一次，
//     框架还说「这窗在穿透」，就说明这个后端没吃下区域，当场退回整窗穿透。
//
// 平台差异只在「输入区域能不能用」这一个布尔上：
//
//	Linux            X11 的 SHAPE 扩展 / Wayland 的 wl_surface 输入区域
//	Windows / macOS  mygo 在指针进出区域时切换整窗穿透（内部实现，15.6ms 轮询）
//
// 回退：`SetInputRegion` 不可用（后端没有实现 / 库缺失 / 窗口还没 realize 拿不到
// 区域矩形）时，退回旧的「整窗 SetIgnoreMouseEvents + 指针轮询临时取消穿透」，
// 用户不会因此失去「穿透 + 设置键可点」。选路逻辑见
// lyricsClickThroughRouteFor，纯函数，两个状态两条路径。

import "github.com/egoist/mygo"

// lyricsClickThroughRoute 是穿透的两条实现路径。
type lyricsClickThroughRoute int

const (
	// lyricsRouteInputRegion：只留设置键那一块收点击（首选，三平台）。
	lyricsRouteInputRegion lyricsClickThroughRoute = iota
	// lyricsRouteWholeWindow：整窗穿透，齿轮靠指针轮询临时取消穿透（回退）。
	lyricsRouteWholeWindow
)

// lyricsClickThroughRouteFor 选路径：要穿透且输入区域可用时走区域，否则退回
// 整窗穿透。纯逻辑（两个状态 → 两条路径），便于无头断言。
func lyricsClickThroughRouteFor(clickThrough, inputRegionOK bool) lyricsClickThroughRoute {
	if clickThrough && inputRegionOK {
		return lyricsRouteInputRegion
	}
	return lyricsRouteWholeWindow
}

// lyricsRegionTarget 是下发穿透状态需要的最小窗口能力。*mygo.Window 满足它；
// 抽成接口只为让「什么时候下发、下发什么」能在无头测试里断言（真窗口在测试里
// 建不出来）。
type lyricsRegionTarget interface {
	Bounds() mygo.Rectangle
	SetInputRegion(rects []mygo.Rectangle)
	SetIgnoreMouseEvents(v bool)
	// IgnoresMouseEvents 读回框架对穿透的回答（mygo 0.9.1 的
	// `Window.IgnoresMouseEvents`，三平台都是「最后一次下发的整窗穿透」）。
	// 输入区域与整窗穿透同时存在时后者优先，所以设完区域后读回仍是 true，
	// 就说明这个后端没吃下区域。
	IgnoresMouseEvents() bool
}

var _ lyricsRegionTarget = (*mygo.Window)(nil)

// lyricsInputRegionRects 是穿透时窗口的输入区域：只有设置键那一块（含 slop，
// 夹进窗口内）。几何唯一来源是 lyricsGearShapeRect —— 与绘制出来的设置键同源，
// 这里不许再写一份常量。
func lyricsInputRegionRects(w, h float32) []mygo.Rectangle {
	x, y, rw, rh := lyricsGearShapeRect(w, h)
	if rw <= 0 || rh <= 0 {
		// 窗口还没布局：没有可交互的矩形。调用方据此判定「下发不了」，
		// 这一轮退回整窗穿透，不能给一个空区域（那等于整窗穿透，齿轮就
		// 点不到了）。
		return nil
	}
	return []mygo.Rectangle{{
		X:      int(x),
		Y:      int(y),
		Width:  int(rw),
		Height: int(rh),
	}}
}

// setLyricsInputRegion 把穿透状态下发给窗口，返回是否真的下发了。
//
// clickThrough 为真时区域 = 设置键那一块；为假时 `nil` 把整个窗口还给窗口
// 自己（mygo：nil = 全部收回，空切片 = 一块都不收）。
//
// 返回 false 的两种情形都要求调用方回退整窗穿透：窗口拿不到有效尺寸，或者
// 平台根本没有输入区域能力。
func setLyricsInputRegion(t lyricsRegionTarget, clickThrough bool) bool {
	if t == nil {
		return false
	}
	if !clickThrough {
		t.SetInputRegion(nil)
		return true
	}
	b := t.Bounds()
	rects := lyricsInputRegionRects(float32(b.Width), float32(b.Height))
	if len(rects) == 0 {
		return false
	}
	t.SetInputRegion(rects)
	return true
}

// clearLyricsInputRegion 把输入区域全部还给窗口。
//
// 穿透关掉时必须显式清一次：mygo 的 `SetIgnoreMouseEvents(false)` 在 Linux 上
// 只是**重新应用**当前输入区域，不会把区域撤掉 —— 只关整窗穿透的话，齿轮之外
// 的地方会一直点不动。
func clearLyricsInputRegion(t lyricsRegionTarget) {
	if t == nil {
		return
	}
	t.SetInputRegion(nil)
}

// lyricsApplyClickThrough 是穿透下发的可测内核：选路 → 下发区域 → 读回 → 失败回退。
//
//   - click          此刻该不该穿透（lyricsIgnoreMouse 的结果）
//   - regionOK       输入区域是否可用（lyricsInputRegionAvailable）
//   - gearHot        回退路径用：指针是否压在设置键热区上（true = 临时取消穿透）
//
// 返回 region（是否走了输入区域）与 wholeWindow（整窗穿透设成了什么）。
//
// 走区域时整窗穿透**必须**显式设成 false —— mygo 文档写明
// `SetIgnoreMouseEvents` 的优先级高于 `SetInputRegion`，留着 true 会让新 API
// 完全失效（这正是删掉 Windows/macOS 上逐 tick 开关的原因）。设完再读回一次
// （mygo 0.9.1 的 `IgnoresMouseEvents`）：读回仍是 true 就是「这个后端没吃下
// 区域」，这一轮按回退路径走，而不是留一个既穿透又点不到齿轮的窗口。
func lyricsApplyClickThrough(t lyricsRegionTarget, click, regionOK, gearHot bool) (region, wholeWindow bool) {
	if lyricsClickThroughRouteFor(click, regionOK) == lyricsRouteInputRegion {
		if setLyricsInputRegion(t, true) {
			t.SetIgnoreMouseEvents(false)
			if !t.IgnoresMouseEvents() {
				return true, false
			}
			// 读回仍是整窗穿透：区域没生效（后端没实现 / 库缺失 /
			// 窗口还没 realize）。下面按回退路径重来一次。
		}
		// 下发不了（窗口还没 realize / 拿不到尺寸）：这一轮退回整窗穿透。
	}
	// 回退路径：先把输入区域整个还给窗口，否则「区域之外」还是点不动。
	clearLyricsInputRegion(t)
	whole := click && !gearHot
	t.SetIgnoreMouseEvents(whole)
	return false, whole
}

// lyricsRegionChanged 报告要不要重新下发输入区域：穿透开关变了，或者窗口尺寸
// 变了（区域跟着设置键的位置走）。纯逻辑，便于无头断言。
func lyricsRegionChanged(prevOn bool, prevW, prevH float32, on bool, w, h float32) bool {
	if prevOn != on {
		return true
	}
	if !on {
		return false
	}
	return prevW != w || prevH != h
}
