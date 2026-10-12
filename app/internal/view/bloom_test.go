package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// periodByName 取某个时段的令牌（测试里按名字固定时段，不随跑测试的钟点变）。
func periodByName(name string) Period {
	for _, p := range Periods {
		if p.Name == name {
			return p
		}
	}
	panic("unknown period " + name)
}

// pxAt 取某点的 RGBA（复用 drawerbg_test 的 rgbaAt）。
func pxAt(t *testing.T, img *image.RGBA, x, y int) [4]int {
	t.Helper()
	r, g, b, a := rgbaAt(t, img, x, y)
	return [4]int{r, g, b, a}
}

// nrgbaAt 取光场位图某点的 RGBA。
func nrgbaAt(img *image.NRGBA, x, y int) [4]int {
	i := img.PixOffset(x, y)
	return [4]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]), int(img.Pix[i+3])}
}

// TestBloomOpacity 钉住 #root::before 整层的不透明度：原版只在四个深色时段
// 下调（midnight .22 / predawn .20 / evening .30 / latenight .25），其余时段
// 是 CSS 默认的 1。
func TestBloomOpacity(t *testing.T) {
	want := map[string]float32{
		"midnight":  0.22,
		"predawn":   0.20,
		"dawn":      1,
		"morning":   1,
		"midday":    1,
		"noon":      1,
		"afternoon": 1,
		"golden":    1,
		"dusk":      1,
		"evening":   0.30,
		"latenight": 0.25,
	}
	for _, p := range Periods {
		if got := bloomOpacity(p.Name); got != want[p.Name] {
			t.Errorf("bloomOpacity(%s) = %v, want %v", p.Name, got, want[p.Name])
		}
	}
}

// TestBloomAlphaAt 钉住单条光晕的径向衰减：中心 = maxAlpha、到 stop 处 = 0、
// 中间线性插值，椭圆外为 0。
func TestBloomAlphaAt(t *testing.T) {
	bl := rootBlooms[0]
	// 中心：cx*bw+ox、cy*bh+oy（400×300 → bw=464 bh=348 ox=-32 oy=-24）。
	cx, cy := float32(-32+0.15*464), float32(-24+0.05*348)
	if got := bloomAlphaAt(bl, cx, cy, 400, 300); got < bl.alpha-1e-4 {
		t.Errorf("中心 alpha = %v, want %v", got, bl.alpha)
	}
	// stop 之外（椭圆的 0.62 半径外）：0。
	far := cx + bl.rx*464
	if got := bloomAlphaAt(bl, far, cy, 400, 300); got != 0 {
		t.Errorf("椭圆外 alpha = %v, want 0", got)
	}
	// 半径一半处（相对 stop 的 0.5）：alpha ≈ maxAlpha/2。
	half := cx + bl.rx*464*bl.stop*0.5
	if got := bloomAlphaAt(bl, half, cy, 400, 300); got < bl.alpha*0.45 || got > bl.alpha*0.55 {
		t.Errorf("半径一半处 alpha = %v, want ≈%v", got, bl.alpha/2)
	}
}

// TestLightFieldWarmCool 钉住方向光：午间（光罩不可见、光晕 opacity 1）左上
// 偏暖（R>B）、右下偏冷（B>R）。
func TestLightFieldWarmCool(t *testing.T) {
	img := lightFieldPixels(400, 300, "midday")
	tl := nrgbaAt(img, 40, 20)
	br := nrgbaAt(img, 360, 280)
	t.Logf("左上 %v 右下 %v", tl, br)
	if tl[0] <= tl[2] {
		t.Errorf("左上不是暖光：%v（应 R>B）", tl)
	}
	if br[2] <= br[0] {
		t.Errorf("右下不是冷光：%v（应 B>R）", br)
	}
	// 暖光颜色要准：中心附近的 straight RGB 应接近 rgba(255,238,218)。
	if tl[1] > 250 {
		t.Errorf("暖光的绿通道偏高（8 位预乘丢精度的老毛病）：%v", tl)
	}
}

// TestLightFieldDimmedAtNight 钉住整层不透明度：同一位置的暖光午间比深夜强。
func TestLightFieldDimmedAtNight(t *testing.T) {
	day := nrgbaAt(lightFieldPixels(400, 300, "midday"), 40, 20)
	night := nrgbaAt(lightFieldPixels(400, 300, "midnight"), 40, 20)
	t.Logf("左上暖光 午间 %v 深夜 %v", day, night)
	if night[3] >= day[3] {
		t.Errorf("深夜光晕没有变弱：午间 alpha=%d 深夜 alpha=%d", day[3], night[3])
	}
}

// TestLightFieldOverlayTint 钉住时段光罩：dawn 的暖色光罩在顶部，往下淡出，
// 所以顶部比底部更暖（蓝通道更低、alpha 更高）。
func TestLightFieldOverlayTint(t *testing.T) {
	img := lightFieldPixels(400, 300, "dawn")
	top := nrgbaAt(img, 200, 5)
	bottom := nrgbaAt(img, 200, 290)
	t.Logf("dawn 光罩 顶 %v 底 %v", top, bottom)
	if top[3] <= bottom[3] {
		t.Errorf("顶部没有暖色光罩：顶 alpha=%d 底 alpha=%d", top[3], bottom[3])
	}
	// 光罩暖色 rgba(255,210,170)：红>蓝。
	if top[0] <= top[2] {
		t.Errorf("光罩顶部不是暖色：%v", top)
	}
}

// TestOverlayT 钉住 165° 渐变线：左上角 t≈0、右下角 t≈1，垂直方向也单调。
func TestOverlayT(t *testing.T) {
	tl := overlayT(0, 0, 400, 300)
	br := overlayT(400, 300, 400, 300)
	if tl > 0.05 {
		t.Errorf("左上角 t = %v, want ≈0", tl)
	}
	if br < 0.95 {
		t.Errorf("右下角 t = %v, want ≈1", br)
	}
	top := overlayT(200, 5, 400, 300)
	bottom := overlayT(200, 290, 400, 300)
	if !(top < bottom) {
		t.Errorf("垂直方向不单调：顶 %v 底 %v", top, bottom)
	}
}

// TestPeriodOverlayStops 抽查两条时段光罩的停靠点与原版 globals.css 一致
// （dawn 暖色到 .7 淡出、midnight 冷色到 .8 淡出）。
func TestPeriodOverlayStops(t *testing.T) {
	dawn := periodOverlayStops("dawn")
	if len(dawn) != 3 || dawn[0].C.R != 255 || dawn[0].C.G != 210 || dawn[0].C.B != 170 {
		t.Fatalf("dawn 停靠点 = %v", dawn)
	}
	if dawn[2].At != 0.7 || dawn[2].C.A != 0 {
		t.Errorf("dawn 应在 .7 淡出到透明：%v", dawn[2])
	}
	mid := periodOverlayStops("midnight")
	if mid[0].C.B != 90 || mid[2].At != 0.8 {
		t.Errorf("midnight 停靠点 = %v", mid)
	}
	if periodOverlayStops("midday") != nil {
		t.Errorf("midday 应无停靠点")
	}
}

// TestShellHasDirectionalLight 是整窗实测：Shell 真的把光场接上了（算成位图、
// 每帧贴图）—— 顶部左侧偏暖、右下偏冷（采样点在主区左右留白里，避开内容）。
func TestShellHasDirectionalLight(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(periodByName("noon"))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tst.Frame()
	img := tst.Image()
	tl := pxAt(t, img, 10, 40)
	br := pxAt(t, img, 790, 500)
	t.Logf("整窗 左上 %v 右下 %v", tl, br)
	if tl[0] <= tl[2] {
		t.Errorf("整窗左上不是暖光：%v", tl)
	}
	if br[2] <= br[0] {
		t.Errorf("整窗右下不是冷光：%v", br)
	}
	// 光场确实被算成位图（而不是每帧现画椭圆）。
	if a.bloomBmp == nil || a.bloomPeriod != "noon" {
		t.Errorf("光场没有烘焙：bmp=%v period=%q", a.bloomBmp != nil, a.bloomPeriod)
	}
}
