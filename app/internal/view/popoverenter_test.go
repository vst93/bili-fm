package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（音量弹层入场过渡）测试。原版 .player-volume-popover 有一条
// `@starting-style` 入场过渡（opacity 0→1、scale .96→1，transform-origin 右中），
// 160ms cubic-bezier(.23,1,.32,1)。原生版此前没有这层，弹层是硬切出现的。

// TestVolumePopoverEnterTokens 钉住过渡的三条核心参数：时长 160ms、
// Enter 的 Scale=0.96 + Opacity=0、Ease 是 cubic-bezier(.23,1,.32,1)。
func TestVolumePopoverEnterTokens(t *testing.T) {
	c := &ui.Context{}
	tr, ok := volumePopoverEnter(c)
	if !ok {
		t.Fatal("常态下应返回 ok=true")
	}
	if tr.Duration != 160*time.Millisecond {
		t.Errorf("Duration = %v，应为 160ms", tr.Duration)
	}
	if tr.Enter == nil {
		t.Fatal("Enter 不应为 nil")
	}
	if tr.Enter.Scale != 0.96 {
		t.Errorf("Enter.Scale = %v，应为 0.96", tr.Enter.Scale)
	}
	if tr.Enter.Opacity != 0 {
		t.Errorf("Enter.Opacity = %v，应为 0", tr.Enter.Opacity)
	}
	got := tr.Ease(0.5)
	want := ui.CubicBezier(0.23, 1, 0.32, 1)(0.5)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("Ease(0.5) = %v，应为 cubic-bezier(.23,1,.32,1)(0.5)=%v", got, want)
	}
}

// TestVolumePopoverEnterReducedMotion：减少动态效果时返回 ok=false（原版
// `@media (prefers-reduced-motion: reduce)` 把 transition 整个关掉）。
// SetPreferences 后需再调 Frame()，因为 SetPreferences 的 settle 可能在偏好生效前运行帧。
func TestVolumePopoverEnterReducedMotion(t *testing.T) {
	var gotOK bool
	tst := ui.NewTester(func(c *ui.Context) {
		_, gotOK = volumePopoverEnter(c)
	}, 100, 100)
	tst.SetPreferences(ui.Preferences{ReduceMotion: true})
	tst.Frame()
	if gotOK {
		t.Error("减少动态效果时应返回 ok=false")
	}
}
