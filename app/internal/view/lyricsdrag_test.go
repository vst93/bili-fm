package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLyricsGearDragSequence 钉住「按住设置键拖动」的完整时序。
//
// 这一条是回归测试：上一版用 `Dragged()` 的返回值**累加**位移，而 mygo 的
// `Dragged()` 报的是**每帧**增量（forgetInput 每帧把 dragX 清零）—— 于是窗口
// 只跟着动一帧就停住（用户 2026-11-11 报「拖动没有跟着鼠标动」）。现在视图只用
// 它判「越过起步阈值」，位置由 app 按指针绝对坐标算（Begin 记偏移、Move 每帧摆、
// End 落盘），所以这里断言的是**调用时序**而不是位移。
func TestLyricsGearDragSequence(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LyricCurrent, a.LyricNext = "第一句", "第二句"
	begins, moves, ends, toggles := 0, 0, 0, 0
	a.Act.BeginLyricsDrag = func() { begins++ }
	a.Act.MoveLyricsDrag = func() { moves++ }
	a.Act.EndLyricsDrag = func() { ends++ }
	a.Act.ToggleLyricsConfig = func() { toggles++ }
	tst := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, LyricsDisplayHeight(a.LyricsFontSize))
	tst.Frame()
	tst.Move(280, 40)
	tst.Frame()
	r, ok := tst.Find("桌面字幕设置")
	if !ok {
		t.Fatal("设置键不在（未穿透时指针在窗内应显示）")
	}
	cx, cy := r.X+r.W/2, r.Y+r.H/2

	tst.Press(cx, cy)
	tst.Frame()
	if begins != 0 {
		t.Fatalf("只按下没移动就调了 Begin（%d 次）：点一下设置键不该拖动窗口", begins)
	}
	// 逐帧小幅移动：每帧 5px，累积越过 4px 阈值后开始拖。
	// **向左**拖：无头测试里窗口不会跟着指针走（moveLyricsDrag 是替身），
	// 往右拖会把指针带出窗口，设置键就不在布局里了 —— 那不是被测的行为。
	for i := 1; i <= 4; i++ {
		tst.Move(cx-float32(i)*5, cy)
		tst.Frame()
	}
	if begins != 1 {
		t.Fatalf("越过阈值后 Begin 调用 %d 次，应为 1", begins)
	}
	if moves < 3 {
		t.Fatalf("拖动期间 Move 只调了 %d 次，应每帧都调", moves)
	}
	// 拖动中设置键必须还在树里：否则 `Dragged()` 没人读，拖动会中断。
	if _, ok := tst.Find("桌面字幕设置"); !ok {
		t.Fatal("拖动过程中设置键从布局里消失了（拖动会中断、松手收不到 End）")
	}
	tst.Release(cx-20, cy)
	tst.Frame()
	if ends == 0 {
		t.Fatal("松手后没有调 End（位置不会落盘）")
	}
	// 点击是**松手之后那一帧**才交给视图的（mygo 在 build 之后的 applyInputs
	// 里落 click），所以要多推一帧才看得到「有没有被吞掉」。
	tst.Frame()
	// 拖动之后松手：**不该**触发点击（弹出设置页）。用户 2026-11-11 的要求。
	if toggles != 0 {
		t.Fatalf("拖动结束时触发了 %d 次设置页切换，应吞掉这次点击", toggles)
	}

	// 松手后再按一次、不动就松开：不该触发拖动，但**要**触发点击。
	begins, ends, toggles = 0, 0, 0
	tst.Press(cx, cy)
	tst.Frame()
	tst.Release(cx, cy)
	tst.Frame()
	tst.Frame()
	if begins != 0 {
		t.Errorf("原地点击也触发了拖动（Begin %d 次）", begins)
	}
	if toggles != 1 {
		t.Errorf("原地点击切换设置页 %d 次，应为 1", toggles)
	}

	// 再拖一次：仍然只有拖动、没有点击（「吞掉点击」的标记不能残留成永久静音）。
	begins, toggles = 0, 0
	tst.Press(cx, cy)
	tst.Frame()
	for i := 1; i <= 4; i++ {
		tst.Move(cx-float32(i)*5, cy)
		tst.Frame()
	}
	tst.Release(cx-20, cy)
	tst.Frame()
	tst.Frame()
	if begins != 1 {
		t.Errorf("第二次拖动 Begin 调用 %d 次，应为 1", begins)
	}
	if toggles != 0 {
		t.Errorf("第二次拖动又触发了 %d 次设置页切换", toggles)
	}
	// 松手后再点一下：点击必须恢复正常（标记不能卡住）。
	toggles = 0
	tst.Press(cx, cy)
	tst.Frame()
	tst.Release(cx, cy)
	tst.Frame()
	tst.Frame()
	if toggles != 1 {
		t.Errorf("拖动之后再点击：切换 %d 次，应为 1（「吞点击」标记不能残留）", toggles)
	}
}
