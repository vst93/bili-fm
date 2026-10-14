package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（减少动态效果）的测试。原版 globals.css 有六个
// `@media (prefers-reduced-motion: reduce)` 块，其中三处落在 mygo 靠框架
// 兜不住的路径上：
//
//	#video-cover.record-disc { animation: none }        唱片停转、回正
//	.history-refresh-spinning { animation: none }       历史刷新图标停转
//	#player ... / 弹幕行 ... { transition: none }        色值过渡当帧到位
//
// （框架已替我们处理位置位移：AnimateTo / AnimatePose / Transition 在
// ReduceMotion 下直接到位；但 Loop 与 AnimateColor 仍照常运行，见 reduce.go。）

// TestDiscTargetAngleHonorsReduceMotion 钉住唱片目标角：减少动态效果时恒为 0
// （原版把 animation 整个撤掉，旋转回到 0°），否则播放中按 discSpinStep 往前
// 推一格、暂停 / 缓冲时停在当前角。
func TestDiscTargetAngleHonorsReduceMotion(t *testing.T) {
	rate := float32(discSpinStep) / float32(discSpinPeriod) * 360
	const angle = 123.5

	if got := discTargetAngle(true, true, angle); got != 0 {
		t.Errorf("减少动态效果·播放中 = %v，想 0（停转回正）", got)
	}
	if got := discTargetAngle(true, false, angle); got != 0 {
		t.Errorf("减少动态效果·暂停 = %v，想 0", got)
	}
	if got := discTargetAngle(false, true, angle); math.Abs(float64(got-(angle+rate))) > 1e-4 {
		t.Errorf("常态·播放中 = %v，想 %v（当前角 + 一步）", got, angle+rate)
	}
	if got := discTargetAngle(false, false, angle); got != angle {
		t.Errorf("常态·暂停 = %v，想停在 %v", got, angle)
	}
}

// TestResetDiscFreezes 实测 resetDisc 把唱片角度归零，并刷新了推进基准
// （discLast / discLastPos）——否则恢复播放时会拿停转期间的挂钟差当成一次
// 位移，补一大笔角度。这里用「复位后只过了 10ms」来判：若基准没刷新，推进量
// 会是秒级（>10°，远超下面 1° 的阈值）。
func TestResetDiscFreezes(t *testing.T) {
	a := NewApp(func() {})
	a.Playing = true
	a.Pos = 40 // 播放位置（秒）

	base := time.Unix(1_700_000_000, 0)
	a.advanceDisc(base) // 建立基准
	a.Pos = 41
	a.advanceDisc(base.Add(time.Second))
	if a.discAngle == 0 {
		t.Fatal("正常播放 1s 后角度仍为 0，基准没生效")
	}

	resetAt := base.Add(2 * time.Second)
	a.resetDisc(resetAt)
	if a.discAngle != 0 {
		t.Fatalf("复位后角度 = %v，想 0", a.discAngle)
	}
	if !a.discLast.Equal(resetAt) {
		t.Errorf("复位后基准时间 = %v，想 %v", a.discLast, resetAt)
	}
	if a.discLastPos != a.Pos {
		t.Errorf("复位后基准位置 = %v，想当前 %v", a.discLastPos, a.Pos)
	}

	// 复位后仅过 10ms：推进量应远小于 1°。
	a.advanceDisc(resetAt.Add(10 * time.Millisecond))
	if a.discAngle > 1 {
		t.Errorf("复位后 10ms 推进了 %v°，基准没被刷新（把停转的空档当成位移了）", a.discAngle)
	}
}

// TestRefreshKeySpinGate 钉住历史刷新图标的自转开关：只有「正在刷新且桌面没有
// 要求减少动态效果」才转；禁用归禁用，自转归自转。
func TestRefreshKeySpinGate(t *testing.T) {
	cases := []struct {
		spinning, reduce, want bool
	}{
		{true, false, true},
		{true, true, false},
		{false, false, false},
		{false, true, false},
	}
	for _, c := range cases {
		if got := refreshKeySpin(c.spinning, c.reduce); got != c.want {
			t.Errorf("refreshKeySpin(spinning=%v, reduce=%v) = %v, want %v",
				c.spinning, c.reduce, got, c.want)
		}
	}
}

// TestHistoryRefreshSpinHonorsReduceMotion 是无头接线实测：换桩 spinRefreshIcon
// 记录「本地图这一帧有没有被套上自转」，历史抽屉刷新中在常态下被调到、在
// 减少动态效果下不被调到；未刷新时两态都不该被调到。
//
// 反证：把 headerButtonOff 里的 `refreshKeySpin(spin, reduceMotion(c))` 换回
// `spin`（忽略偏好），「减少动态效果」那条会变红。
func TestHistoryRefreshSpinHonorsReduceMotion(t *testing.T) {
	orig := spinRefreshIcon
	t.Cleanup(func() { spinRefreshIcon = orig })

	render := func(loading, reduce bool) int {
		calls := 0
		spinRefreshIcon = func(icon ui.Element, progress float32) {
			calls++
			orig(icon, progress)
		}
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = "history"
		a.HistTab = HistHistory
		a.ListFor("history").Loading = loading
		tt := shellTester(t, a)
		// 首帧（未设偏好）已经转了几帧；从设完偏好起重新计数。
		calls = 0
		tt.SetPreferences(ui.Preferences{ReduceMotion: reduce})
		tt.Frame()
		return calls
	}

	if n := render(true, false); n == 0 {
		t.Error("常态刷新中：刷新图标应被套上自转（spinRefreshIcon 未被调到）")
	}
	if n := render(true, true); n != 0 {
		t.Errorf("减少动态效果下：刷新图标不该自转（被调 %d 次）", n)
	}
	if n := render(false, false); n != 0 {
		t.Errorf("未刷新时不该自转（被调 %d 次）", n)
	}
}

// TestCoverDiscHonorsReduceMotion 是无头接线实测：换桩 discTargetAngle 记录
// 「本地图传给唱片目标角的 reduce 参数」，桌面要求减少动态效果时渲染主窗
// （播放中 + 唱片模式），coverDisc 这一路应把 reduce=true 传下来。
//
// 反证：把 coverDisc 里 `discTargetAngle(reduce, ...)` 的 reduce 改回 false，
// 本测试变红。
func TestCoverDiscHonorsReduceMotion(t *testing.T) {
	orig := discTargetAngle
	t.Cleanup(func() { discTargetAngle = orig })
	var seen []bool
	discTargetAngle = func(reduce, playing bool, angle float32) float32 {
		seen = append(seen, reduce)
		return orig(reduce, playing, angle)
	}

	a := NewApp(func() {})
	a.CoverMode = "disc"
	a.Playing = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})
	seen = nil
	tt.Frame()
	if len(seen) == 0 {
		t.Fatal("唱片没有渲染（discTargetAngle 未被调到）")
	}
	for _, r := range seen {
		if !r {
			t.Errorf("唱片目标角收到的 reduce=false，减少动态效果偏好没传到封面")
		}
	}
}

// TestPlayerFadeColorInstantUnderReduceMotion 实测：桌面要求减少动态效果时
// playerFadeColor 不再走 140ms 过渡，换目标当帧就返回新值（原版对
// `#player .player-button` 等写了 `transition: none`）。与
// TestPlayerFadeColorTransitions 互为对照（后者断言常态下当帧返回旧值）。
func TestPlayerFadeColorInstantUnderReduceMotion(t *testing.T) {
	colA, colB := ui.Hex("#334155"), ui.Hex("#0284c7")
	cur := colA
	var samples []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, playerFadeColor(c, b, "probe", cur))
	}, 60, 60)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})

	samples = nil
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != colA {
		t.Fatalf("首帧应取当前色 %v，得到 %v", colA, samples[0])
	}

	samples = nil
	cur = colB
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] != colB {
		t.Errorf("减少动态效果下换目标应瞬时到位，得到 %v（还在走过渡）", samples[0])
	}
}

// TestDrawerFadeColorInstantUnderReduceMotion 同上，落在抽屉弹幕 / 评论的
// 色值过渡（原版对 .danmaku-time-group / .danmaku-message-row 等写了
// `transition: none`）。
func TestDrawerFadeColorInstantUnderReduceMotion(t *testing.T) {
	colA, colB := ui.Hex("#ffffff").Alpha(0.32), ui.Hex("#0ea5e9").Alpha(0.22)
	cur := colA
	var samples []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, drawerFadeColor(c, b, "probe", cur, drawerFadeAnim))
	}, 60, 60)
	tt.SetPreferences(ui.Preferences{ReduceMotion: true})

	samples = nil
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != colA {
		t.Fatalf("首帧应取当前色 %v，得到 %v", colA, samples[0])
	}

	samples = nil
	cur = colB
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] != colB {
		t.Errorf("减少动态效果下换目标应瞬时到位，得到 %v（还在走过渡）", samples[0])
	}
}
