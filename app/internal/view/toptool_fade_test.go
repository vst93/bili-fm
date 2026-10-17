package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTopToolColorFadeWired 钉住本轮补齐的两条非位移动画，并实测它们确实以
// 「过渡」而非「瞬时」的方式落到 AnimateColor / AnimateTo 上（原版
// `.top-tool-btn` / `.search-submit-btn` 的
// `transition: background-color 0.18s ease, opacity 0.16s ease`）。
//
// 判据与 TestNavIconColorFadeWired 相同：目标变化的那一帧，曲线 t=0 ⇒ 返回
// **上一帧的值**；直接铺目标值会当帧跳到新值。与墙钟无关（只取换目标后第一帧，
// 虽然 Tester.Frame 会 settle 到动画结束，但收集到的第 0 个样本就是那一帧）。
func TestTopToolColorFadeWired(t *testing.T) {
	if navIconBgAnim.Duration != 180*time.Millisecond {
		t.Errorf("底色过渡时长 = %v, want 180ms（原版 background-color 0.18s）", navIconBgAnim.Duration)
	}
	if navIconFadeAnim.Duration != 160*time.Millisecond {
		t.Errorf("不透明度过渡时长 = %v, want 160ms（原版 opacity 0.16s）", navIconFadeAnim.Duration)
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, a := range []ui.Animation{navIconBgAnim, navIconFadeAnim} {
		if a.Spring != (ui.SpringOptions{}) {
			t.Errorf("应为曲线动画，不是弹簧：%+v", a.Spring)
		}
		if a.Ease == nil {
			t.Fatal("缺少缓动曲线")
		}
		for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
			if d := math.Abs(float64(a.Ease(x) - want(x))); d > 1e-6 {
				t.Errorf("缓动在 %.2f 处 = %v, want %v", x, a.Ease(x), want(x))
			}
		}
	}

	bgA, bgB := ui.Hex("#3b82f6"), ui.Hex("#f8fafc")
	opA, opB := float32(0.3), float32(1)
	curBg, curOp := bgA, opA
	var bgSamples []ui.Color
	var opSamples []float32
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		bgSamples = append(bgSamples, topToolBgColor(b, curBg))
		opSamples = append(opSamples, topToolFadeValue(b, curOp))
	}, 60, 60)
	tt.Frame()
	if len(bgSamples) == 0 || len(opSamples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if bgSamples[0] != bgA || opSamples[0] != opA {
		t.Fatalf("首帧应直接取目标：bg=%v op=%v", bgSamples[0], opSamples[0])
	}

	// 换目标，只取换目标后的第一帧。
	bgSamples, opSamples = nil, nil
	curBg, curOp = bgB, opB
	tt.Frame()
	if len(bgSamples) == 0 || len(opSamples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if bgSamples[0] == bgB {
		t.Errorf("底色换目标的当帧就跳到新值 %v —— 没走 0.18s 过渡", bgB)
	}
	if !colorNear(bgSamples[0], bgA) {
		t.Errorf("底色过渡应从上一帧 %v 起步，得到 %v", bgA, bgSamples[0])
	}
	if opSamples[0] == opB {
		t.Errorf("不透明度换目标的当帧就跳到新值 %v —— 没走 0.16s 过渡", opB)
	}
	if opSamples[0] != opA {
		t.Errorf("不透明度过渡应从上一帧 %v 起步，得到 %v", opA, opSamples[0])
	}
}

// topToolSubject 造一个能悬停/静息的无头场景：真实的 App.Shell 里量搜索药丸
// 的四颗内容入口键（.top-tool-btn）。
func topToolSubject(t *testing.T, hover bool) (*image.RGBA, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find("动态")
	if !ok {
		t.Fatal("找不到「动态」入口键")
	}
	if hover {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Advance(300 * time.Millisecond)
	tt.Frame()
	return tt.Image(), r
}

// TestTopToolFadeWired 无头实测：搜索药丸两类小键真的接上了底色 / 不透明度过渡。
//
// 底色：悬停态把 topToolBgTo 换成 no-op（不铺白底）后，按钮矩形里大量像素变化。
// 不透明度：静息态把 topToolFade 换成 no-op（不再压到 .82）后，图标/聚光一起变亮。
func TestTopToolFadeWired(t *testing.T) {
	// 底色 —— 悬停态。
	on, rect := topToolSubject(t, true)
	savedBg := topToolBgTo
	topToolBgTo = func(b ui.Element, target ui.Color) ui.Element { return b }
	off, _ := topToolSubject(t, true)
	topToolBgTo = savedBg
	diff, total := diffInRect(on, off, rect)
	t.Logf("入口键悬停：%d/%d 像素随底色过渡变化", diff, total)
	if diff < total/10 {
		t.Errorf("只有 %d/%d 像素受悬停底色影响（<10%%）—— topToolBgTo 没接上？", diff, total)
	}

	// 不透明度 —— 静息态。
	rest, rect := topToolSubject(t, false)
	savedFade := topToolFade
	topToolFade = func(b ui.Element, target float32) ui.Element { return b }
	restOff, _ := topToolSubject(t, false)
	topToolFade = savedFade
	diff, total = diffInRect(rest, restOff, rect)
	t.Logf("入口键静息：%d/%d 像素随不透明度过渡变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受常态不透明度影响（<5%%）—— topToolFade 没接上？", diff, total)
	}
}

// TestTopToolInkFadeWired 钉住搜索药丸小键的第三条非位移动画：`color 0.18s ease`
// （图标 #64748b → #0284c7 的淡变），并实测它以**过渡**而非瞬时的方式落到
// AnimateColor 上。判据与底色/不透明度相同：换目标后的第一帧应返回上一帧的值。
func TestTopToolInkFadeWired(t *testing.T) {
	// 与原版 `color 0.18s ease` 同一条（时长 + CSS 默认缓动）。
	if navIconBgAnim.Duration != 180*time.Millisecond {
		t.Errorf("图标色过渡时长 = %v, want 180ms（原版 color 0.18s）", navIconBgAnim.Duration)
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	if navIconBgAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(navIconBgAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want %v", x, navIconBgAnim.Ease(x), want(x))
		}
	}

	inkA, inkB := ui.Hex("#64748b"), ui.Hex("#0284c7")
	cur := inkA
	var samples []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, topToolInkColor(b, cur))
	}, 60, 60)
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != inkA {
		t.Fatalf("首帧应直接取目标：%v", samples[0])
	}

	// 换目标，只取换目标后的第一帧。
	samples = nil
	cur = inkB
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] == inkB {
		t.Errorf("图标色换目标的当帧就跳到新值 %v —— 没走 0.18s 过渡", inkB)
	}
	if !colorNear(samples[0], inkA) {
		t.Errorf("图标色过渡应从上一帧 %v 起步，得到 %v", inkA, samples[0])
	}
}

// TestTopToolInkWired 无头实测：图标色确实经 topToolInk 落到两类小键的图标上。
// 把 topToolInk 换成只记录目标色、仍写 TextColor 的桩，悬停入口键后应记录到目标
// 色 #0284c7 —— 反证这条过渡真的接在按钮图标上，而不只是孤立的 helper。
func TestTopToolInkWired(t *testing.T) {
	saved := topToolInk
	var seenBlue bool
	topToolInk = func(b ui.Element, target ui.Color) ui.Element {
		if target == ui.Hex("#0284c7") {
			seenBlue = true
		}
		return b.TextColor(target)
	}
	defer func() { topToolInk = saved }()

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tt := shellTester(t, a)
	r, ok := tt.Find("动态")
	if !ok {
		t.Fatal("找不到「动态」入口键")
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Frame()
	if !seenBlue {
		t.Errorf("悬停入口键后没见过目标色 #0284c7 —— topToolInk 没接在图标上？")
	}
}

// TestTopToolPoseFor 钉住搜索药丸小键的位移取值（原版 globals.css）：
//
//	.top-tool-btn:hover   translateY(-1px)
//	.top-tool-btn:active  scale(.92) —— 源序在 :hover 之后，整条替换 ⇒ 只缩放
func TestTopToolPoseFor(t *testing.T) {
	for _, tc := range []struct {
		pressed, hovered bool
		want             ui.Pose
	}{
		{false, false, ui.Pose{}},
		{false, true, ui.Pose{Y: -1}},
		{true, false, ui.Pose{Scale: 0.92}},
		{true, true, ui.Pose{Scale: 0.92}},
	} {
		if got := topToolPoseFor(tc.pressed, tc.hovered); got != tc.want {
			t.Errorf("topToolPoseFor(%v,%v) = %+v, want %+v", tc.pressed, tc.hovered, got, tc.want)
		}
	}
	// `.top-tool-btn` 的 :active 缩放（.92）与 `.nav-icon-btn`（.94）不同。
	if topToolPoseFor(true, false) == navIconPoseFor(true, false) {
		t.Error("top-tool 的按下缩放应与 nav-icon 不同（.92 vs .94）")
	}
}
