package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（迷你窗对照 main 原版）① 的测试：迷你窗那两颗窗口控制键在窗口操作
// 在途时（原版 isWindowControlPending）按 `button:disabled` 渲染并完全不响应点击。
//
// 原版：
//
//	index.tsx  setIsWindowControlPending(true) / finally 复位（set_window_size、
//	           set_window_always_on_top 在途），另有 windowModeChangingRef 防重入
//	miniVideoInfo.tsx  disabled={isWindowControlPending}（两颗键）
//	globals.css  body.mini-mode .mini-window-controls button:disabled {
//	                 opacity: 0.34; cursor: default; transform: none; }

// miniPendingProbe 建一个 400×155 的迷你窗，WindowControlPending = pending。
func miniPendingProbe(t *testing.T, pending bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Mini = true
	a.WindowControlPending = pending
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	return tst
}

// TestMiniControlDisabledStyleTarget 钉住禁用态的三项渲染参数：底色透明、
// 图标 --studio-muted、**绝对值** .34 的不透明度。
//
// mygo 绘制禁用节点时会再乘 0.5（paint.go），所以写给元素的是
// disabledDim(.34) = .68 —— 最终落到画布上才是原版那条 `opacity: 0.34`。
// 这里换桩 titleBarBtnFade 把传下来的三值截下来逐项断言。
func TestMiniControlDisabledStyleTarget(t *testing.T) {
	orig := titleBarBtnFade
	t.Cleanup(func() { titleBarBtnFade = orig })

	type call struct {
		bg, ink ui.Color
		op      float32
	}
	var calls []call
	titleBarBtnFade = func(b ui.Element, bg, ink ui.Color, opacity float32, reduce bool) (ui.Color, ui.Color, float32) {
		calls = append(calls, call{bg, ink, opacity})
		return bg, ink, opacity
	}

	miniPendingProbe(t, true)
	if len(calls) == 0 {
		t.Fatal("迷你窗没有渲染窗口控制键（titleBarBtnFade 未被调到）")
	}
	// settle 会跑多帧，每帧都重画这两颗键：逐次断言它们全是禁用态理即可。
	if len(calls)%2 != 0 {
		t.Errorf("titleBarBtnFade 被调了 %d 次，迷你窗每帧应恰好两颗键", len(calls))
	}
	want := disabledDim(0.34)
	for i, c := range calls {
		if c.bg != ui.Transparent {
			t.Errorf("第 %d 颗键禁用态底色 = %v，应为透明（没有悬停/按压高光）", i, c.bg)
		}
		if c.ink != ThemeFor(PeriodAt(12)).Muted {
			t.Errorf("第 %d 颗键禁用态图标色 = %v，应为 --studio-muted", i, c.ink)
		}
		if c.op != want {
			t.Errorf("第 %d 颗键禁用态元素不透明度 = %v，应为 disabledDim(.34) = %v（框架会再乘 %v）",
				i, c.op, want, frameworkDisabledDim)
		}
	}
	if want*frameworkDisabledDim != 0.34 {
		t.Errorf("补偿后落到画布上的不透明度 = %v，应等于原版的 0.34", want*frameworkDisabledDim)
	}
}

// TestMiniControlPendingIgnoresClicks 无头实测：pending 时两颗键**完全不响应点击**
// （框架 Disabled 让 Clicked() 恒假），且不会因为按在键上而触发行级 DragWindow 的
// 双保险把动作漏出去。恢复了（pending=false）才真正转发到 Act。
func TestMiniControlPendingIgnoresClicks(t *testing.T) {
	for _, pending := range []bool{true, false} {
		var mini, pin int
		a := NewApp(func() {})
		a.Mini = true
		a.WindowControlPending = pending
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		a.Act.SetMini = func(bool) { mini++ }
		a.Act.TogglePin = func() { pin++ }
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tst.Frame()

		for _, label := range []string{"切换到窗口模式", "窗口置顶"} {
			r, ok := tst.Find(label)
			if !ok {
				t.Fatalf("找不到迷你窗控制键 %q", label)
			}
			tst.ClickAt(r.X+r.W/2, r.Y+r.H/2)
			tst.Frame()
		}
		if pending {
			if mini != 0 || pin != 0 {
				t.Errorf("在途期间两颗键被点动了：setMini=%d togglePin=%d", mini, pin)
			}
		} else {
			if mini != 1 || pin != 1 {
				t.Errorf("不在途时应转发点击：setMini=%d togglePin=%d（各应为 1）", mini, pin)
			}
		}
	}
}

// miniBtnPixels 把按钮矩形的像素拍下来（用于比对悬停前后是否真的变了）。
func miniBtnPixels(tst *ui.Tester, r ui.Rect) []uint8 {
	img := tst.Image()
	out := make([]uint8, 0, int(r.W*r.H)*4)
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := img.PixOffset(x, y)
			out = append(out, img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3])
		}
	}
	return out
}

// miniHoverReaction 量「把指针压在置顶键上」这一帧相对「指针不在键上」那一帧
// 改变了多少个像素。原版悬停会给键铺一层白玻璃 + 图标转蓝（几百个像素变），
// 而禁用态（`button:disabled`）让 `:hover:not(:disabled)` 整条不生效 —— 一个像素
// 都不该变。
func miniHoverReaction(t *testing.T, pending bool) int {
	t.Helper()
	build := func() *ui.Tester {
		a := NewApp(func() {})
		a.Mini = true
		a.WindowControlPending = pending
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tst.Frame()
		return tst
	}
	avert := build()
	r, ok := avert.Find("窗口置顶")
	if !ok {
		t.Fatal("找不到置顶键")
	}
	avert.Move(miniWidth-2, 2) // 指针停在远处（标题栏右上角）
	settleTitleBarFade(avert)
	over := build()
	over.Move(r.X+r.W/2, r.Y+r.H/2) // 压在置顶键上
	settleTitleBarFade(over)
	x, y := miniBtnPixels(avert, r), miniBtnPixels(over, r)
	if len(x) != len(y) {
		t.Fatalf("两侧像素数组长度不一致：%d vs %d", len(x), len(y))
	}
	n := 0
	for i := range x {
		if x[i] != y[i] {
			n++
		}
	}
	return n
}

// TestMiniControlPendingKillsHover 无头像素实测：
//
//	pending=false：悬停置顶键会给它铺白玻璃 + 图标转蓝 —— 键矩形里成百上千个像素变
//	pending=true ：禁用态（原版 `button:disabled`，`:hover:not(:disabled)` 不生效）——
//	                指针压上去一个像素都不变
//
// 反证：把 miniControl 里的 `b.Disabled(pending)` 删掉，第二条子例会看到悬停反应。
func TestMiniControlPendingKillsHover(t *testing.T) {
	live, dead := miniHoverReaction(t, false), miniHoverReaction(t, true)
	t.Logf("置顶键悬停反应像素数：不在途 %d、在途 %d", live, dead)
	if live < 100 {
		t.Errorf("不在途时悬停置顶键应改变大量像素（白玻璃 + 蓝图标），得到 %d", live)
	}
	if dead != 0 {
		t.Errorf("在途时悬停置顶键改变了 %d 个像素 —— 禁用态没有生效，悬停高光还在", dead)
	}
}

// TestMiniControlPendingDimsButton 无头像素实测：在途时两颗键整体被压暗
// （原版 .34 绝对值），与常态比对比度明显更低。取按钮矩形内的明暗极差。
func TestMiniControlPendingDimsButton(t *testing.T) {
	contrast := func(pending bool) int {
		a := NewApp(func() {})
		a.Mini = true
		a.WindowControlPending = pending
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tst.Frame()
		r, ok := tst.Find("切换到窗口模式")
		if !ok {
			t.Fatal("找不到「切换到窗口模式」键")
		}
		img := tst.Image()
		lo, hi := 255, 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				v := grayAt(img, x, y)
				lo, hi = min(lo, v), max(hi, v)
			}
		}
		return hi - lo
	}
	normal, dimmed := contrast(false), contrast(true)
	t.Logf("「切换到窗口模式」矩形内明暗极差：常态 %d、在途 %d", normal, dimmed)
	if normal == 0 {
		t.Fatal("常态按钮没有任何墨迹（测量点选错了）")
	}
	if dimmed >= normal {
		t.Errorf("在途时按钮没有被压暗：常态极差 %d、在途 %d", normal, dimmed)
	}
}

// TestMiniControlNoPressScaleWhenPending 钉住原版 `transform: none`：框架
// Disabled 让 Pressed() 恒假，pressScale 这条通路整条关掉。换桩不了 pressScale
// （它是普通函数），所以从「按下时矩形是否会缩小」这个可观测结果上验：
// 在途时按下键，键的标签矩形尺寸不变。
func TestMiniControlNoPressScaleWhenPending(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.WindowControlPending = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	before, ok := tst.Find("窗口置顶")
	if !ok {
		t.Fatal("找不到置顶键")
	}
	tst.Press(before.X+before.W/2, before.Y+before.H/2)
	time.Sleep(200 * time.Millisecond) // 按压缩放（160ms）若生效早该跑完
	tst.Frame()
	after, ok := tst.Find("窗口置顶")
	if !ok {
		t.Fatal("按下后找不到置顶键")
	}
	if before != after {
		t.Errorf("在途时按下键的矩形变了（%+v → %+v）——原版 :disabled 是 transform: none", before, after)
	}
	tst.Release(before.X+before.W/2, before.Y+before.H/2)
}
