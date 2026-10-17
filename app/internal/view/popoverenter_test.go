package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（音量弹层入场过渡）测试。原版 .player-volume-popover 有一条
// `@starting-style` 入场过渡：
//
//	transition: opacity 140ms ease, transform 160ms cubic-bezier(.23,1,.32,1);
//	@starting-style { opacity: 0; transform: translateY(-50%) scale(.96); }
//	transform-origin: 100% 50%;
//
// 弹层从「右中、缩到 96%、全透明」淡入放大到常态。原生版此前没有这层，
// 弹层是硬切出现的（原版倍速弹层确实没有这条，所以只给音量弹层）。

// popoverEnterOf 在一个真实 context 里取一次 volumePopoverEnter（免手搓 ui.Context）。
func popoverEnterOf(t *testing.T, prefs *ui.Preferences) (ui.ElementTransition, bool) {
	t.Helper()
	var (
		tr ui.ElementTransition
		ok bool
	)
	tst := ui.NewTester(func(c *ui.Context) {
		tr, ok = volumePopoverEnter(c)
	}, 100, 100)
	if prefs != nil {
		tst.SetPreferences(*prefs)
	}
	tst.Frame()
	return tr, ok
}

// TestVolumePopoverEnterTokens 钉住过渡的四条核心参数：默认时长 160ms、
// Enter 的 Scale=0.96 + Opacity=0（不 Collapse，只缩放不挤兄弟）、缓动是
// cubic-bezier(.23,1,.32,1)。
func TestVolumePopoverEnterTokens(t *testing.T) {
	if volumePopoverEnterDuration != 160*time.Millisecond {
		t.Errorf("默认时长 = %v, want 160ms（原版 transform 那条）", volumePopoverEnterDuration)
	}

	tr, ok := popoverEnterOf(t, nil)
	if !ok {
		t.Fatal("常态下 volumePopoverEnter 应返回 ok=true")
	}
	if tr.Duration != volumePopoverEnterDuration {
		t.Errorf("Duration = %v, want %v", tr.Duration, volumePopoverEnterDuration)
	}
	if tr.Enter == nil {
		t.Fatal("Enter 不应为 nil")
	}
	if tr.Enter.Scale != 0.96 {
		t.Errorf("Enter.Scale = %v, want 0.96（@starting-style 的 scale(.96)）", tr.Enter.Scale)
	}
	if tr.Enter.Opacity != 0 {
		t.Errorf("Enter.Opacity = %v, want 0（@starting-style 的 opacity: 0）", tr.Enter.Opacity)
	}
	if tr.Enter.Collapse {
		t.Error("Enter 不应 Collapse：原版只 scale，不塌缩兄弟")
	}
	// Easing 是函数，不可比较，按取值比对。
	want := ui.CubicBezier(0.23, 1, 0.32, 1)
	for _, x := range []float32{0.1, 0.25, 0.5, 0.75, 0.9} {
		if d := math.Abs(float64(tr.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("Ease(%.2f) = %v, want %v（cubic-bezier(.23,1,.32,1)）", x, tr.Ease(x), want(x))
		}
	}
}

// TestVolumePopoverEnterReducedMotion：桌面要求减少动态效果时返回 ok=false
// —— 原版 `@media (prefers-reduced-motion: reduce)` 把这条 transition 整个
// 关掉（transition: none），弹层直接出现，不淡也不缩。
func TestVolumePopoverEnterReducedMotion(t *testing.T) {
	_, ok := popoverEnterOf(t, &ui.Preferences{ReduceMotion: true})
	if ok {
		t.Error("减少动态效果时应返回 ok=false")
	}
	// 对照：同一套参数下不要求减少动态效果时必须是 ok=true。
	if _, ok := popoverEnterOf(t, &ui.Preferences{}); !ok {
		t.Error("常态下应返回 ok=true（对照）")
	}
}

// TestVolumePopoverEnterWired：入场过渡真的挂到了音量弹层上。默认 160ms 短到
// 一次 settle（最多 20 帧）里就跑完，观察不到中间帧，所以把时长临时放长到 10s：
// 打开弹层后连续两帧的像素必须不同（动画在跑）。去掉面板上的 `.Transition(tr)`
// 这一帧就会逐像素相同，测试即红。
//
// 再开「减少动态效果」跑同一段：volumePopoverEnter 返回 ok=false，不挂过渡，
// 即便时长仍放长，两帧也必须逐像素相同（反证上面的不同确实来自这条过渡）。
func TestVolumePopoverEnterWired(t *testing.T) {
	orig := volumePopoverEnterDuration
	volumePopoverEnterDuration = 10 * time.Second
	t.Cleanup(func() { volumePopoverEnterDuration = orig })

	build := func() (*App, *ui.Tester) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		a.Dur = 200
		a.Pos = 100
		a.SeekValue = 100
		a.Volume = 0.5
		return a, ui.NewTester(func(c *ui.Context) { a.playerBar(c) }, 500, 120)
	}
	// 打开弹层后连续两帧的像素差。
	openDiff := func(a *App, tst *ui.Tester) int {
		tst.Frame()
		a.ShowVolume = true
		tst.Frame()
		f2 := tst.Image()
		tst.Advance(50 * time.Millisecond) // v0.5.5 冻结时钟：推一点让入场过渡走动
		tst.Frame()
		f3 := tst.Image()
		n := 0
		for i := range f2.Pix {
			if f2.Pix[i] != f3.Pix[i] {
				n++
			}
		}
		return n
	}

	a, tst := build()
	if d := openDiff(a, tst); d == 0 {
		t.Error("打开音量弹层后连续两帧逐像素相同：入场过渡没挂上")
	}

	a2, tst2 := build()
	tst2.SetPreferences(ui.Preferences{ReduceMotion: true})
	if d := openDiff(a2, tst2); d != 0 {
		t.Errorf("减少动态效果下音量弹层仍有两帧差异 %d：应直接出现（原版 transition: none）", d)
	}
}
