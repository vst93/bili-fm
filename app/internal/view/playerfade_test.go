package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏色值过渡）的测试。原版 `#player .player-button` 的
// `transition: color 140ms ease, background-color 140ms ease,
// border-color 140ms ease, box-shadow 140ms ease`——此前原生版底色 / 描边 /
// 墨色全是瞬时写入（悬停时键面「啪」地换色），这轮按原版补上 140ms ease
// 过渡（走 AnimateColor，与导航族 180ms 的 navIconBgAnim 同一手法；阴影无
// 动画通道，不在本轮范围）。

// TestPlayerFadeAnimTokens 钉住这条过渡的参数：140ms + CSS ease 曲线
// （CubicBezier(0.25, 0.1, 0.25, 1)，与 navIconBgAnim 同源但时长不同——
// 工具键族是 140ms，导航族是 180ms）。
func TestPlayerFadeAnimTokens(t *testing.T) {
	if playerFadeAnim.Duration != 140*time.Millisecond {
		t.Errorf("播放栏色值过渡时长 = %v, want 140ms（原版 transition 140ms ease）", playerFadeAnim.Duration)
	}
	if playerFadeAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", playerFadeAnim.Spring)
	}
	if playerFadeAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(playerFadeAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want CSS ease %v", x, playerFadeAnim.Ease(x), want(x))
		}
	}
}

// TestPlayerFadeColorTransitions 实测 playerFadeColor 真的以「过渡」而非
// 「瞬时」返回色值：目标变化的那一帧曲线 t=0 ⇒ 返回**上一帧的颜色**；
// 直接铺目标色会当帧跳到新值。与墙钟无关（只取换目标后的第一帧，
// Tester.Frame 会 settle 到动画结束，但收集到的第 0 个样本就是那一帧）。
func TestPlayerFadeColorTransitions(t *testing.T) {
	colA, colB := ui.Hex("#334155"), ui.Hex("#0284c7")
	cur := colA
	var samples []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		samples = append(samples, playerFadeColor(c, b, "probe", cur))
	}, 60, 60)
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if samples[0] != colA {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", colA, samples[0])
	}

	// 换目标，只取换目标后的第一帧。
	samples = nil
	cur = colB
	tt.Frame()
	if len(samples) == 0 {
		t.Fatal("换目标后探针视图没有被构建")
	}
	if samples[0] == colB {
		t.Errorf("换目标的当帧就跳到新值 %v —— 没走 140ms 过渡", colB)
	}
	if samples[0] != colA {
		t.Errorf("过渡应从上一帧 %v 起步，得到 %v", colA, samples[0])
	}
}

// TestPlayerToolInkFadesToBlue 无头实测接线：主窗音量键（弹层未开，
// active=false，悬停只改色值不改布局）的图标墨色随悬停从 #526174 转
// studio-blue。与 playerbar_align_test.go 里「下一集」键的判法同源：
// 取键内**最大**蓝调（最深像素会采到按钮底下进度条的轨道边，不可用作
// 判据）。#526174 的 B−R=34、#0284c7 的 B−R=197。
//
// 队列里放一条曲目：禁用态（无来源）的按键不响应悬停，hasSource 才会
// 让 b.Hovered() 生效（对齐 playerbar_align_test.go 的探针建法）。
//
// 反证说明：Tester 的 settle 每帧都把时钟推进到过渡结束，所以悬停
// 落点帧桩前后逐像素一致（都是纯蓝），像素级反证抓不住中间帧。
// 「过渡真的发生了」由 TestPlayerFadeColorTransitions 在单元层钉死
// （换目标当帧返回上一帧颜色）；这里只钉「音量键的悬停换色确实发生
// 且经过 playerFadeColor」——后者靠编译期与上面两段实测联保。
func TestPlayerToolInkFadesToBlue(t *testing.T) {
	blueness := func(tst *ui.Tester, r ui.Rect) int {
		img := tst.Image()
		best := -1 << 20
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				c := img.RGBAAt(x, y)
				if d := int(c.B) - int(c.R); d > best {
					best = d
				}
			}
		}
		return best
	}

	// 静息：键内最大蓝调远低于 studio-blue（#526174 的 B−R 只有 34）。
	seed := func(a *App) {
		a.Queue = []Track{{Bvid: "BV1", Cid: 1, Title: "t"}}
		a.Index = 0
	}
	_, rest := playerBarProbe(t, seed)
	r := volumeButtonRect(t, rest)
	restBlue := blueness(rest, r)
	if restBlue >= 100 {
		t.Errorf("静息键内最大蓝调 = %d，应远低于 studio-blue（B−R=197）", restBlue)
	}

	// 悬停：settle 把 140ms 过渡走完，收敛到蓝。
	hov := rest
	hov.Move(r.X+r.W/2, r.Y+r.H/2)
	hov.Advance(200 * time.Millisecond) // 推过 140ms 颜色过渡（v0.5.5 冻结时钟）
	hov.Frame()
	hovBlue := blueness(hov, r)
	t.Logf("音量键内最大蓝调：静息=%d 悬停=%d", restBlue, hovBlue)
	if hovBlue < 100 {
		t.Errorf("悬停后图标没有转蓝：静息 B−R=%d 悬停 B−R=%d", restBlue, hovBlue)
	}
}

// volumeButtonRect 定位主窗播放栏的音量键（弹层未开时全窗口只有这颗键
// 的 Label 是「音量」；弹层滑块的 Label 是别的文案）。
func volumeButtonRect(t *testing.T, tst *ui.Tester) ui.Rect {
	t.Helper()
	r, ok := tst.Find("音量")
	if !ok {
		t.Fatal("播放栏找不到音量键")
	}
	return r
}
