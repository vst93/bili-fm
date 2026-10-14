package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（标题栏过渡）的测试。原版 globals.css 给标题栏与迷你窗小键写了三条
// transition，原生版此前全是当帧硬切：
//
//	.app-title-bar::after / .app-title-bar > *   opacity 0.24s ease   hover-to-reveal 淡入淡出
//	.app-title-bar button / .mini-window-controls button
//	    color 0.18s ease, background-color 0.18s ease, opacity 0.16s ease, transform 0.16s ease
//	@media (prefers-reduced-motion: reduce) .mini-window-controls button { transition: none }
//
// 位移（transform）由框架的 AnimatePose 承担；这里补的是「不透明度 / 色值」这两路。

// TestTitleBarRevealFades 是无头接线实测：换桩 titleBarFade，统计 hover-to-reveal
// 的四层（顶沿投影 tb-edge / 白玻璃 tb-wash / 品牌 tb-brand / 直接子元素 tb-child）
// 静息与唤出两态的目标值。
//
// 反证：把 titleBarLayer 里的 titleBarFade 换回直接 .Opacity(opacity)（或在
// brandBox / actions 上换回 .Opacity(brand)/.Opacity(child)），对应那层就从
// seen 里消失，本测试变红。
func TestTitleBarRevealFades(t *testing.T) {
	orig := titleBarFade
	t.Cleanup(func() { titleBarFade = orig })

	seen := map[string]float32{}
	titleBarFade = func(el ui.Element, key string, target float32) float32 {
		seen[key] = target
		return target
	}

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tt.Frame()

	want := func(edge, wash, brand, child float32, when string) {
		t.Helper()
		for key, v := range map[string]float32{
			"tb-edge": edge, "tb-wash": wash, "tb-brand": brand, "tb-child": child,
		} {
			got, ok := seen[key]
			if !ok {
				t.Fatalf("%s：%q 那一层没有走 titleBarFade（reveal 那层还在硬切）", when, key)
			}
			if got != v {
				t.Errorf("%s：%q 目标 = %v，想 %v", when, key, got, v)
			}
		}
	}
	want(0.52, 0, 0.7, 0.32, "静息态")

	tt.Move(500, 18) // 指针落到标题栏上
	tt.Frame()
	want(0.78, 1, 1, 0.82, "唤出态")

	tt.Move(500, 400) // 移开
	tt.Frame()
	want(0.52, 0, 0.7, 0.32, "移开后")
}

// TestTitleBarFadeTransitions 实测 titleBarFade 真的以「过渡」而非「瞬时」返回
// 不透明度：目标变化的那一帧曲线 t=0 ⇒ 返回**上一帧的值**。与
// TestPlayerFadeColorTransitions 同源（只取换目标后的第一帧）。
func TestTitleBarFadeTransitions(t *testing.T) {
	target := float32(0.32)
	var samples []float32
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, titleBarFade(b, "probe", target))
	}, 60, 60)
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != target {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", target, samples[0])
	}

	samples = nil
	target = 1
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] == target {
		t.Errorf("换目标的当帧就跳到新值 %v —— 没走 0.24s 过渡", target)
	}
	if samples[0] != 0.32 {
		t.Errorf("过渡应从上一帧 0.32 起步，得到 %v", samples[0])
	}
}

// TestTitleBarBtnFadeTransitions 实测 titleBarBtnFade 的常态（reduce=false）：
// 底色 / 图标色 / 不透明度三者都以过渡返回（换目标当帧仍是旧值）；reduce=true
// （原版 `.mini-window-controls button` 的 `transition: none`）时当帧到位。
func TestTitleBarBtnFadeTransitions(t *testing.T) {
	type sample struct {
		bg, ink ui.Color
		op      float32
	}
	colA, colB := ui.Hex("#526174"), ui.Hex("#0284c7")
	bgA, bgB := ui.Transparent, ui.Hex("#ffffff").Alpha(0.38)

	run := func(reduce bool) []sample {
		bg, ink, op := bgA, colA, float32(0.72)
		var samples []sample
		tt := ui.NewTester(func(c *ui.Context) {
			b := ui.Box(c)
			gotBG, gotInk, gotOp := titleBarBtnFade(b, bg, ink, op, reduce)
			samples = append(samples, sample{gotBG, gotInk, gotOp})
		}, 60, 60)
		tt.Frame()
		if len(samples) == 0 {
			t.Fatal("探针视图没有被构建")
		}
		// 换目标，只取换目标后的第一帧。
		samples = nil
		bg, ink, op = bgB, colB, 1
		tt.Frame()
		return samples
	}

	got := run(false)
	if len(got) == 0 {
		t.Fatal("常态探针视图没有被构建")
	}
	if got[0].bg != bgA || got[0].ink != colA || got[0].op != 0.72 {
		t.Errorf("常态换目标当帧应仍是旧值 (bg=%v ink=%v op=%v)，得到 (bg=%v ink=%v op=%v)",
			bgA, colA, 0.72, got[0].bg, got[0].ink, got[0].op)
	}

	got = run(true)
	if len(got) == 0 {
		t.Fatal("减少动态效果探针视图没有被构建")
	}
	if got[0].bg != bgB || got[0].ink != colB || got[0].op != 1 {
		t.Errorf("减少动态效果下换目标应瞬时到位 (bg=%v ink=%v op=1)，得到 (bg=%v ink=%v op=%v)",
			bgB, colB, got[0].bg, got[0].ink, got[0].op)
	}
}

// TestTitleBarBtnHighlightFadeTransitions 实测 titleBarBtnHighlightFade 真的以
// 「过渡」而非「瞬时」返回 ::after 的不透明度（原版 `transition: opacity 0.18s ease`）：
// 目标变化的那一帧曲线 t=0 ⇒ 返回**上一帧的值**（同 TestTitleBarFadeTransitions）。
func TestTitleBarBtnHighlightFadeTransitions(t *testing.T) {
	target := float32(0)
	var samples []float32
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, titleBarBtnHighlightFade(b, target))
	}, 60, 60)
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != 0 {
		t.Fatalf("首帧应直接取目标 0，得到 %v", samples[0])
	}

	samples = nil
	target = 1
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] == 1 {
		t.Error("换目标的当帧就跳到 1 —— 没走 0.18s 过渡")
	}
	if samples[0] != 0 {
		t.Errorf("过渡应从上一帧 0 起步，得到 %v", samples[0])
	}
}

// TestTitleBarBtnHighlightFadeWired 是无头接线实测：主窗「设置」键（它也是
// `.app-title-bar button`）悬停时 ::after 那层的不透明度目标要走 1、移开走 0。
// 换桩 titleBarBtnHighlightFade 统计传下来的目标值。
//
// 反证：把 shell.go 里 settingsButton 的那句 titleBarBtnHighlight 删掉，
// 本测试在「悬停」那步收不到 1。
func TestTitleBarBtnHighlightFadeWired(t *testing.T) {
	orig := titleBarBtnHighlightFade
	t.Cleanup(func() { titleBarBtnHighlightFade = orig })

	var targets []float32
	titleBarBtnHighlightFade = func(b ui.Element, target float32) float32 {
		targets = append(targets, target)
		return target
	}

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tt.Frame()
	if len(targets) == 0 {
		t.Fatal("主窗没有渲染标题栏小键（titleBarBtnHighlightFade 未被调到）")
	}
	for _, v := range targets {
		if v != 0 {
			t.Fatalf("静息时 ::after 目标 = %v，应为 0", v)
		}
	}

	r, ok := tt.Find("设置")
	if !ok {
		t.Fatal("标题栏里找不到「设置」键")
	}
	targets = nil
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	sawOne := false
	for _, v := range targets {
		if v == 1 {
			sawOne = true
		}
	}
	if !sawOne {
		t.Errorf("悬停「设置」键时没有把 ::after 目标调到 1（收到 %v）", targets)
	}
}

// TestMiniControlFadeHonorsReduceMotion 是无头接线实测：换桩 titleBarBtnFade，
// 渲染迷你窗（miniControl 画「还原大窗」与「置顶」两颗键），统计传下来的
// reduce 参数。原版的 `transition: none` 只针对 `.mini-window-controls button`，
// 所以桌面要求减少动态效果时迷你窗这两颗键必须当帧到位。
//
// 反证：把 miniControl 里的 `reduceMotion(c)` 改成常量 false，第二条子例变红。
func TestMiniControlFadeHonorsReduceMotion(t *testing.T) {
	orig := titleBarBtnFade
	t.Cleanup(func() { titleBarBtnFade = orig })

	var seen []bool
	titleBarBtnFade = func(b ui.Element, bg, ink ui.Color, opacity float32, reduce bool) (ui.Color, ui.Color, float32) {
		seen = append(seen, reduce)
		return bg, ink, opacity
	}

	render := func(reduce bool) []bool {
		a := NewApp(func() {})
		a.Mini = true
		a.Theme = ThemeFor(PeriodAt(12))
		tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		seen = nil
		tt.Frame()
		return seen
	}

	normal := render(false)
	if len(normal) == 0 {
		t.Fatal("迷你窗没有渲染窗口控制键（titleBarBtnFade 未被调到）")
	}
	for i, r := range normal {
		if r {
			t.Errorf("常态下第 %d 颗迷你窗控制键收到了 reduce=true", i)
		}
	}

	reduced := render(true)
	if len(reduced) == 0 {
		t.Fatal("减少动态效果下迷你窗没有渲染窗口控制键")
	}
	for i, r := range reduced {
		if !r {
			t.Errorf("减少动态效果下第 %d 颗迷你窗控制键没有当帧到位（reduce=false）", i)
		}
	}
}
