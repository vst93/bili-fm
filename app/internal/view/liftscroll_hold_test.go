package view

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// scrollHold 的实现（状态机本体）在 state.go；本文件做行为断言。

// obs 是 observe 的便捷断言封装：只关心锁定状态。
func obs(h *scrollHold, offset float64, shown bool) bool {
	locked, _ := h.observe(offset, shown)
	return locked
}

func TestScrollHoldStateMachine(t *testing.T) {
	var h scrollHold
	// 第一帧建立基线：不算滚动。
	if obs(&h, 0, true) {
		t.Fatal("首帧不该锁住")
	}
	if obs(&h, 0, true) {
		t.Fatal("静止第二帧不该锁住")
	}
	// 开始滚动：位移发生的本帧立即锁住（滚轮偏移在事件里已同步写回），
	// 并必须求下一帧来判定「停下」。
	if locked, needFrame := h.observe(100, true); !locked || !needFrame {
		t.Fatalf("位移发生的本帧必须锁住并求下一帧：locked=%v needFrame=%v", locked, needFrame)
	}
	if !obs(&h, 200, true) {
		t.Fatal("滚动中的第二帧必须锁住")
	}
	if !h.holding() {
		t.Fatal("滚动中 holding 应为真")
	}
	// 位置连续不变的第一帧：松开，并**不再**求帧（本帧把上浮画回去即可）。
	if locked, needFrame := h.observe(200, true); locked || needFrame {
		t.Fatalf("停下首帧应恢复且不再求帧：locked=%v needFrame=%v", locked, needFrame)
	}
	if h.holding() {
		t.Fatal("恢复后 holding 应为假")
	}
	// 静止中再来一帧，保持不锁也不再求帧（不能每帧空转重建视图）。
	if locked, needFrame := h.observe(200, true); locked || needFrame {
		t.Fatal("静止帧不该锁住也不该求帧")
	}
	// 又滚一格：立即重新按住。
	if !obs(&h, 300, true) {
		t.Fatal("再次滚动应重新锁住")
	}
	if !h.holding() {
		t.Fatal("再次滚动后 holding 应为真")
	}
}

func TestScrollHoldHiddenKeepsBaseline(t *testing.T) {
	var h scrollHold
	obs(&h, 0, true)
	obs(&h, 0, true)
	// 容器离开视图时滚动（观测不到）：隐藏帧返回当前锁定状态，
	// 不重置；再出现且位置变了，立即按住。
	if obs(&h, 0, false) {
		t.Fatal("隐藏帧沿用锁定状态（此前未锁）")
	}
	if obs(&h, 0, false) {
		t.Fatal("隐藏帧不该因缺观测而误锁")
	}
	// 隐藏期间位置变了：重现后第一帧建立新基线（不算滚动），第二帧起
	// 才按新基线判定 —— 隐藏期间的一次性跳变不会误锁。
	if obs(&h, 500, true) {
		t.Fatal("重现首帧重建基线，不该锁住")
	}
	if obs(&h, 500, true) {
		t.Fatal("重现后位置没再动，不该锁住")
	}
	if !obs(&h, 600, true) {
		t.Fatal("重现后再滚动应锁住")
	}
}

// passRec 是一次构建 pass（Shell 调用）的观测量：这一 pass 结束时滚动锁
// 是否按住、这一 pass 里新建了多少张卡的 lift 动画。
type passRec struct {
	held bool
	lift int64
}

// feedTester 起一个无头窗口跑真实 App.Shell，并逐 pass 记录锁定状态与
// lift 重建次数（渲染 pass 与构建 pass 在本视图里一一对应）。
func feedTester(t *testing.T, a *App) (*ui.Tester, *[]passRec) {
	t.Helper()
	var recs []passRec
	tt := ui.NewTester(func(c *ui.Context) {
		a.liftChannels.Store(0)
		a.Shell(c)
		recs = append(recs, passRec{held: a.scrollHoldHeld, lift: a.liftChannels.Load()})
	}, 800, 600)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	return tt, &recs
}

func feedApp(cards int) *App {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	cs := make([]Card, cards)
	for i := range cs {
		cs[i] = Card{
			Bvid:  fmt.Sprintf("BV%04d", i),
			Title: fmt.Sprintf("标题%d", i),
			Up:    "UP主",
		}
	}
	a.ListFor("feed").Cards = cs
	a.Drawer = "feed"
	return a
}

// scrollBurst 把指针停在网格里并滚几次，返回滚动周期的逐 pass 记录。
func scrollBurst(t *testing.T, a *App) []passRec {
	t.Helper()
	tt, recs := feedTester(t, a)
	tt.Move(400, 300)
	for i := 0; i < 10; i++ {
		tt.Frame()
	}
	// 只统计滚动周期内的 pass。
	*recs = (*recs)[:0]
	for i := 0; i < 4; i++ {
		tt.Scroll(400, 300, 0, 120)
	}
	return *recs
}

// TestScrollHoldLocksLiftWhileScrolling 端到端（无头）：指针停在网格里，
// 滚动进行中每一 pass 都不重建 lift 动画；停下后至少有一个 pass 把 lift
// 画回来（静止/悬停的选中效果没有被去掉）。
func TestScrollHoldLocksLiftWhileScrolling(t *testing.T) {
	a := feedApp(300)
	recs := scrollBurst(t, a)
	var heldPasses, lockedLift, restLift int64
	for _, r := range recs {
		if r.held {
			heldPasses++
			lockedLift += r.lift
		} else {
			restLift += r.lift
		}
	}
	if heldPasses == 0 {
		t.Fatal("真实滚动没有触发滚动锁（holdListScroll 没接上 ui.List 的偏移）")
	}
	if lockedLift != 0 {
		t.Fatalf("锁住期间仍新建了 %d 个 lift 动画（锁没拦住渲染）", lockedLift)
	}
	if restLift == 0 {
		t.Fatal("滚动停下后悬停卡的 lift 动画没有恢复（选中效果被误删）")
	}
	t.Logf("滚动周期内：按住 %d pass（lift 新建 0）→ 恢复帧 lift 新建 %d", heldPasses, restLift)
}

// TestScrollHoldDisabledRebuildsEveryFrame 是改前对照：关掉滚动锁
// （scrollHoldEnabled=false）复现改动前行为 —— 一次滚动锁都不按住，
// 每个 pass 都重建 lift 动画。证明上面的「锁住期间 0 重建」来自本轮
// 改动，而不是本来就没重建。
func TestScrollHoldDisabledRebuildsEveryFrame(t *testing.T) {
	scrollHoldEnabled = false
	defer func() { scrollHoldEnabled = true }()
	a := feedApp(300)
	recs := scrollBurst(t, a)
	var heldPasses, totalLift int64
	for _, r := range recs {
		if r.held {
			heldPasses++
		}
		totalLift += r.lift
	}
	if heldPasses != 0 {
		t.Fatalf("关锁后不该有任何按住 pass，实得 %d", heldPasses)
	}
	if totalLift == 0 {
		t.Fatal("关锁后每个 pass 都应重建 lift（对照基线失败）")
	}
	t.Logf("关锁对照：%d pass 共重建 lift 动画 %d 次（开锁时按住 pass 为 0 次）", len(recs), totalLift)
}
