package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（进度条悬停时间气泡）测试。原版 .player-seek-bubble（globals.css:3160）
// 有两条此前原生版没对齐的行为：
//
//  1. `transition: opacity 120ms ease; opacity: 0` +
//     `[data-visible="true"] { opacity: 1 }` —— 气泡是淡入淡出的，不是硬切；
//  2. 文案 / 位置只在 handleTimelineHover 里写（`bubble.textContent`、
//     `bubble.style.left`），hideTimelineHover 只翻 data-visible —— 移开指针后
//     气泡停在**最后一次悬停的点**上淡出，不跟着指针滑走、也不缩窄。

// TestSeekBubbleFadeTokens 钉住过渡参数：120ms + ease（cubic-bezier(.25,.1,.25,1)）。
func TestSeekBubbleFadeTokens(t *testing.T) {
	if seekBubbleFadeAnim.Duration != 120*time.Millisecond {
		t.Errorf("Duration = %v，应为 120ms（原版 transition: opacity 120ms）", seekBubbleFadeAnim.Duration)
	}
	// Ease 是函数，不可比较，按取值比对。
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1) // CSS 关键字 ease
	for _, x := range []float32{0.1, 0.25, 0.5, 0.75, 0.9} {
		if got, w := seekBubbleFadeAnim.Ease(x), want(x); got != w {
			t.Errorf("Ease(%.2f) = %v，应为 %v（ease）", x, got, w)
		}
	}
}

// seekBubbleProbe 起一个主窗（亮色时段），装一条 200 秒的曲目、进度停在 1:40。
// 返回滑块矩形（进度条的矩形，气泡按它定位）与 tester。
func seekBubbleProbe(t *testing.T) (ui.Rect, *ui.Tester) {
	t.Helper()
	_, tst := playerBarProbe(t, nil)
	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	return r, tst
}

// TestSeekBubbleFreezesOnLeave 实测第 2 条：悬停时出现时间气泡（文案 = 指针
// 处的时间），把指针**挪到另一个 x** 离开后再看，气泡文案与位置都冻结在最后
// 悬停的点上（原版只翻 data-visible，不重算 textContent / style.left）。
//
// 反证：把位置/文案改成每帧按当前指针重算（旧实现），移开后文案会变成新
// 位置的时刻（或清空）→ 本测试红；把 over=false 时直接 return（硬切、
// 气泡被销毁）→ 移开后 Find 不到文案 → 也红。
func TestSeekBubbleFreezesOnLeave(t *testing.T) {
	r, tst := seekBubbleProbe(t)
	// 指针悬停在轨道 1/4 处：200s × 0.25 ≈ 0:48。
	hx := r.X + r.W*0.25
	tst.Move(hx, r.Y+r.H/2)
	tst.Frame()
	hover, ok := tst.Find("0:48")
	if !ok {
		t.Fatal("悬停后没有时间气泡（0:48）")
	}

	// 挪到另一个 x 离开（y 也离开轨道）：若位置/文案跟着重算，这里会变成
	// 0:15 附近（0.15 处）且 rect 移走。
	tst.Move(r.X+r.W*0.15, r.Y+r.H/2+60)
	tst.Frame()
	frozen, ok := tst.Find("0:48")
	if !ok {
		t.Fatal("移开指针后气泡文案被清掉/重算了：应冻结在最后悬停的 0:48")
	}
	if frozen != hover {
		t.Errorf("移开后气泡位置漂移：悬停 %+v → 移开 %+v", hover, frozen)
	}
	if _, ok := tst.Find("0:15"); ok {
		t.Error("移开后气泡跟到了新指针位置（0:15）：应冻结在原处")
	}
}

// TestSeekBubbleFadesOutOnLeave 实测第 1 条：移开指针后气泡不是立刻消失，
// 而是在最后悬停的位置上淡出。默认 120ms 短到一次 Frame 里就跑完，所以把
// 淡出用的时长临时放长到 10s，保证「移开后第一帧气泡仍在」可观测。
//
// 取样 = 气泡文字矩形扩一圈（气泡本体，避开轨道/滑块/悬停环：它们都在更下面）。
// 判据：
//   - 静息 → 悬停：该区域像素必须变化（气泡出现）；
//   - 静息 → 移开：该区域像素**仍**必须变化（气泡没硬切消失，正在淡出）；
//   - 开「减少动态效果」再跑一遍：移开后必须立刻与静息逐像素相同
//     （AnimateTo 直接落位），反证前面的差异确实来自这条过渡。
func TestSeekBubbleFadesOutOnLeave(t *testing.T) {
	r, tst := seekBubbleProbe(t)
	rest := cloneRGBA(tst.Image())
	hx := r.X + r.W*0.25
	tst.Move(hx, r.Y+r.H/2)
	tst.Frame()
	tr, ok := tst.Find("0:48")
	if !ok {
		t.Fatal("悬停后没有时间气泡（0:48）")
	}
	// 文字行两边各留 8px、上下各留 4px：都落在气泡 18px 高的白底里，
	// 不碰下面 24px 的进度条（悬停环/滑块肿胀都在那儿）。
	region := image.Rect(int(tr.X)-8, int(tr.Y)-4, int(tr.X+tr.W)+8, int(tr.Y+tr.H)+2)
	hover := cloneRGBA(tst.Image())
	if n := regionDiff(rest, hover, region); n == 0 {
		t.Fatal("悬停后气泡区域的像素没变化：气泡没出现")
	}

	orig := seekBubbleFadeAnim
	seekBubbleFadeAnim = ui.Animation{Duration: 10 * time.Second, Ease: orig.Ease}
	t.Cleanup(func() { seekBubbleFadeAnim = orig })

	tst.Move(r.X+r.W*0.15, r.Y+r.H/2+60)
	tst.Frame()
	fade := cloneRGBA(tst.Image())
	if n := regionDiff(rest, fade, region); n == 0 {
		t.Error("移开指针后气泡区域立刻回到静息：淡出没接上（硬切）")
	}

	// 减少动态效果：过渡整个关掉，气泡直接落位 → 移开后与静息逐像素相同。
	r2, tst2 := seekBubbleProbe(t)
	tst2.SetPreferences(ui.Preferences{ReduceMotion: true})
	rest2 := cloneRGBA(tst2.Image())
	tst2.Move(r2.X+r2.W*0.25, r2.Y+r2.H/2)
	tst2.Frame()
	tr2, ok := tst2.Find("0:48")
	if !ok {
		t.Fatal("减少动态效果下悬停也该显示气泡")
	}
	region2 := image.Rect(int(tr2.X)-8, int(tr2.Y)-4, int(tr2.X+tr2.W)+8, int(tr2.Y+tr2.H)+2)
	tst2.Move(r2.X+r2.W*0.15, r2.Y+r2.H/2+60)
	tst2.Frame()
	if n := regionDiff(rest2, cloneRGBA(tst2.Image()), region2); n != 0 {
		t.Errorf("减少动态效果下移开指针仍有 %d 处像素差异：应直接落位（原版 transition: none）", n)
	}
}

// cloneRGBA 拷一份帧，供不同时刻的像素比对（Image() 复用同一缓冲）。
func cloneRGBA(img *image.RGBA) *image.RGBA {
	cp := image.NewRGBA(img.Rect)
	copy(cp.Pix, img.Pix)
	return cp
}

// regionDiff 数两张帧在 rect 里逐字节不同的像素数。
func regionDiff(a, b *image.RGBA, rect image.Rectangle) int {
	n := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := a.PixOffset(x, y)
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] ||
				a.Pix[i+2] != b.Pix[i+2] || a.Pix[i+3] != b.Pix[i+3] {
				n++
			}
		}
	}
	return n
}
