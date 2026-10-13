package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// freezeNavIconFades 把本轮补上的**底色 / 不透明度过渡**换成瞬时写入。
//
// 像素对照测试要比较「同一状态两次渲染」（例如把某个 helper 换成 no-op 前后），
// 而底色/不透明度是随墙钟推进的动画，两次渲染的相位不同会让整块按钮的像素都
// 不一样，从而掩盖真正要测的那一层。过渡本身的正确性由
// TestNavIconColorFadeWired 单独钉住，这里只保证其它测试的确定性。
func freezeNavIconFades(t *testing.T) {
	t.Helper()
	bg, fade := navIconBgTo, navIconFade
	navIconBgTo = func(b ui.Element, target ui.Color) ui.Element {
		return b.Background(target)
	}
	navIconFade = func(b ui.Element, off bool, rest float32) ui.Element {
		target := rest
		if off {
			target = 0.6
		}
		return b.Opacity(target)
	}
	t.Cleanup(func() { navIconBgTo, navIconFade = bg, fade })
}

// TestPressShadowSpecs 钉住 `.nav-icon-btn:active` / `.top-tool-btn:active` /
// `.search-submit-btn:active` 逐字相同的那条内压阴影：
//
//	box-shadow: inset 0 2px 4px rgba(15, 23, 42, 0.08)
func TestPressShadowSpecs(t *testing.T) {
	got := pressShadowSpecs()
	if len(got) != 1 {
		t.Fatalf("按下内压应 1 条，得到 %d", len(got))
	}
	if want := (shadowSpec{0, 2, 4, 0, ui.Hex("#0f172a").Alpha(0.08)}); got[0] != want {
		t.Errorf("按下内压 = %+v, want %+v", got[0], want)
	}
}

// TestNavIconColorFadeWired 钉住本轮补齐的两条非位移动画，并实测它们确实以
// 「过渡」而非「瞬时」的方式落到 AnimateColor / AnimateTo 上（原版
// `transition: background-color 0.18s ease, opacity 0.16s ease`）。
//
// 判据：目标变化的那一帧，曲线 t=0 ⇒ 返回**上一帧的值**；直接铺目标色会当帧
// 跳到新值。这与墙钟无关（settle 把后续帧也跑了，所以只取换目标后的第一帧）。
func TestNavIconColorFadeWired(t *testing.T) {
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
		bgSamples = append(bgSamples, navIconBgColor(b, curBg))
		opSamples = append(opSamples, navIconFadeValue(b, curOp))
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

// colorNear 比较两个颜色的各通道（容忍 1 级舍入）。
func colorNear(a, b ui.Color) bool {
	near8 := func(x, y uint8) bool {
		d := int(x) - int(y)
		return d >= -1 && d <= 1
	}
	return near8(a.R, b.R) && near8(a.G, b.G) && near8(a.B, b.B) && near8(a.A, b.A)
}

// navPressRender 渲染主窗，把指针按在（或移过）目标按钮上，返回帧与按钮矩形。
// freezeNavIconFades 由调用方负责（避免底色动画相位干扰）。
func navPressRender(t *testing.T, label string, press bool) (*image.RGBA, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.Queue = []Track{*a.Track}
	a.Index = 0
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("找不到按钮 %q", label)
	}
	if press {
		tt.Press(r.X+r.W/2, r.Y+r.H/2)
	} else {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Frame()
	return tt.Image(), r
}

// TestNavPressShadowWired 无头实测：按下内容操作行的 `.nav-icon-btn` 时真的叠上了
// 原版 `:active` 的内压阴影 —— 把 navPressShadow 换成 no-op 后，同一按钮矩形里有
// 像素跟着变，反证这层内压来自这里。底色/不透明度过渡先冻结，避免动画相位造成
// 假差异。
func TestNavPressShadowWired(t *testing.T) {
	freezeNavIconFades(t)
	on, rect := navPressRender(t, "浏览器打开", true)
	saved := navPressShadow
	navPressShadow = func(b ui.Element) ui.Element { return b }
	defer func() { navPressShadow = saved }()
	off, _ := navPressRender(t, "浏览器打开", true)

	diff, total := diffInRect(on, off, rect)
	t.Logf("内容操作行按下：%d/%d 像素随 :active 内压变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受按下内压影响（<5%%）—— :active box-shadow 没接上？", diff, total)
	}
}

// TestTopToolPressShadowWired 同上，针对搜索药丸的内容入口键（.top-tool-btn）。
func TestTopToolPressShadowWired(t *testing.T) {
	freezeNavIconFades(t)
	on, rect := navPressRender(t, "动态", true)
	saved := navPressShadow
	navPressShadow = func(b ui.Element) ui.Element { return b }
	defer func() { navPressShadow = saved }()
	off, _ := navPressRender(t, "动态", true)

	diff, total := diffInRect(on, off, rect)
	t.Logf("内容入口键按下：%d/%d 像素随 :active 内压变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受按下内压影响（<5%%）—— :active box-shadow 没接上？", diff, total)
	}
}
