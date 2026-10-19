package view

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 氛围光（原版 .app-shell::before / ::after）的无头验收。
//
// 原版这两层的 background-image 一共五层（自上而下）：
//
//	::before  linear-gradient(180deg, rgba(255,255,255,.14), rgba(255,255,255,.03) 58%,
//	                          rgba(255,255,255,.12))          ← 白洗光
//	          radial-gradient(ellipse 72% 62% at 50% 46%,
//	                          transparent, rgba(241,245,249,.08) 58%, rgba(241,245,249,.24))
//	          var(--ambient-cover)                            ← 当前封面（背景层）
//	::after   linear-gradient(180deg, rgba(248,251,255,.22), rgba(248,251,255,.07) 34%,
//	                          rgba(248,251,255,.09) 72%, rgba(248,251,255,.16))
//	          radial-gradient(ellipse 70% 64% at 50% 46%,
//	                          transparent, rgba(248,251,255,.05) 66%, rgba(248,251,255,.14))
//
// ::before 带 `inset:-8%`、`transform:scale(1.03)`、`opacity:.31`（高级质感关闭时
// `.texture-basic` 把它换成 `filter:none; opacity:.35`）；::after 是 `inset:0`、无
// 额外不透明度。原生版此前只画了封面 + 一层两停靠点的 ::after 线性渐变。
//
// 下面的常量是**独立抄录**的一份 CSS 取值（与 shell.go 的生产常量同源但分开写），
// 生产代码抄错任何一处都会让采样点偏离。

type ambientCSSStop struct {
	at         float32
	r, g, b, a float32
}

var (
	testAmbientBeforeSheen = []ambientCSSStop{
		{0, 255, 255, 255, 0.14},
		{0.58, 255, 255, 255, 0.03},
		{1, 255, 255, 255, 0.12},
	}
	testAmbientBeforeVignette = []ambientCSSStop{
		{0, 241, 245, 249, 0},
		{0.58, 241, 245, 249, 0.08},
		{1, 241, 245, 249, 0.24},
	}
	testAmbientAfterSheen = []ambientCSSStop{
		{0, 248, 251, 255, 0.22},
		{0.34, 248, 251, 255, 0.07},
		{0.72, 248, 251, 255, 0.09},
		{1, 248, 251, 255, 0.16},
	}
	testAmbientAfterVignette = []ambientCSSStop{
		{0, 248, 251, 255, 0},
		{0.66, 248, 251, 255, 0.05},
		{1, 248, 251, 255, 0.14},
	}
)

// cssStopAt 按 CSS 语义在停靠点之间插值（RGB 直通 —— 四条渐变的起点与相邻
// 停靠点同色，所以直通与 premultiplied 等价）。
func cssStopAt(stops []ambientCSSStop, t float32) (r, g, b, a float32) {
	if t <= stops[0].at {
		s := stops[0]
		return s.r, s.g, s.b, s.a
	}
	last := stops[len(stops)-1]
	if t >= last.at {
		return last.r, last.g, last.b, last.a
	}
	for i := 1; i < len(stops); i++ {
		if t <= stops[i].at {
			p, q := stops[i-1], stops[i]
			k := (t - p.at) / (q.at - p.at)
			mix := func(u, v float32) float32 { return u + (v-u)*k }
			return mix(p.r, q.r), mix(p.g, q.g), mix(p.b, q.b), mix(p.a, q.a)
		}
	}
	return last.r, last.g, last.b, last.a
}

// cssEllipseT 是 `radial-gradient(ellipse <rx>% <ry>% at <cx>% <cy>%, …)` 的
// 渐变线参数：椭圆半径按盒宽 / 盒高取百分比，t=1 落在椭圆边界上。
func cssEllipseT(x, y, w, h, cxPct, cyPct, rxPct, ryPct float32) float32 {
	dx := (x - w*cxPct) / (w * rxPct)
	dy := (y - h*cyPct) / (h * ryPct)
	return float32(math.Hypot(float64(dx), float64(dy)))
}

// over 把颜色 col（alpha a）叠到 dst 上，返回新的 RGB。
func over(dst, col [3]float32, a float32) [3]float32 {
	var out [3]float32
	for i := range out {
		out[i] = dst[i]*(1-a) + col[i]*a
	}
	return out
}

// ambientModel 是「原版五层」在窗口坐标 (x,y) 处的合成结果：base 打底，然后
// 自下而上依次叠封面、::before 暗角、::before 洗光、::after 暗角、::after 洗光。
func ambientModel(x, y, w, h float32, base, cover [3]float32, opacity float32) [3]float32 {
	// ::before 的盒：inset -8% 再绕中心 scale(1.03)。
	hw := (w/2 + 0.08*w) * 1.03
	hh := (h/2 + 0.08*h) * 1.03
	px, py := w/2-hw, h/2-hh
	pw, ph := 2*hw, 2*hh

	out := base
	// 1) 封面（178% × 156%，position 50% 44%）：整窗都在图内，alpha 就是 opacity。
	out = over(out, cover, opacity)
	// 2) ::before 径向暗角（72% / 62%，中心 50% 46%）。
	_, _, _, va := cssStopAt(testAmbientBeforeVignette,
		cssEllipseT(x-px, y-py, pw, ph, 0.5, 0.46, 0.72, 0.62))
	out = over(out, [3]float32{241, 245, 249}, va*opacity)
	// 3) ::before 白洗光（180deg，盒 = ::before 盒）。
	_, _, _, sa := cssStopAt(testAmbientBeforeSheen, cssLinearT(x-px, y-py, pw, ph, 180))
	out = over(out, [3]float32{255, 255, 255}, sa*opacity)
	// 4) ::after 径向暗角（70% / 64%，中心 50% 46%，盒 = 整窗）。
	_, _, _, va2 := cssStopAt(testAmbientAfterVignette,
		cssEllipseT(x, y, w, h, 0.5, 0.46, 0.70, 0.64))
	out = over(out, [3]float32{248, 251, 255}, va2)
	// 5) ::after 白洗光（180deg，4 个停靠点）。
	_, _, _, sa2 := cssStopAt(testAmbientAfterSheen, cssLinearT(x, y, w, h, 180))
	return over(out, [3]float32{248, 251, 255}, sa2)
}

// oldAmbientModel 是**修复前**的写法（封面 Fit(Cover) 铺满 + 一层 .22→.14 的
// 两停靠点线性渐变），只用来做「本轮真的改了可见像素」的反证。
func oldAmbientModel(y, h float32, base, cover [3]float32) [3]float32 {
	out := over(base, cover, 0.31)
	t := cssLinearT(0, y, 1, h, 180)
	a := 0.22 + (0.14-0.22)*t
	return over(out, [3]float32{248, 251, 255}, a)
}

// solidAmbientCache 是一个只提供氛围光位图的假 Images（实现 ImageStore）。
type solidAmbientCache struct{ bmp *ui.Bitmap }

func (s solidAmbientCache) Bitmap(string, int) *ui.Bitmap        { return nil }
func (s solidAmbientCache) Get(string, int) *ui.Bitmap           { return nil }
func (s solidAmbientCache) Touch(string, int)                    {}
func (s solidAmbientCache) Retain(string, int)                   {}
func (s solidAmbientCache) Cancel(string, int)                   {}
func (s solidAmbientCache) Ambient(string, int, bool) *ui.Bitmap { return s.bmp }
func (s solidAmbientCache) Pending() int                         { return 0 }

// solidBitmap 造一张纯色位图。
func ambientSolidBitmap(r, g, b uint8) *ui.Bitmap {
	img := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = r, g, b, 255
	}
	return ui.NewBitmap(img)
}

// ambientSample 渲染一层氛围光（base 打底），返回位图与尺寸。
func ambientSample(t *testing.T, w, h int, premium bool) *image.RGBA {
	t.Helper()
	tst := ui.NewTester(func(c *ui.Context) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Premium = premium
		a.Images = solidAmbientCache{bmp: ambientSolidBitmap(51, 102, 153)}
		ui.Box(c).Fill().Background(ui.Hex("#808080")).Children(func() { a.ambient(c) })
	}, w, h)
	tst.Frame()
	return tst.Image()
}

func TestAmbientOverlaysMatchCSS(t *testing.T) {
	const w, h = 800, 600
	img := ambientSample(t, w, h, true)
	base := [3]float32{128, 128, 128}
	cover := [3]float32{51, 102, 153}

	points := [][2]float32{
		{1, 1}, {w - 2, 1}, {1, h - 2}, {w - 2, h - 2},
		{w / 2, h / 2}, {w / 2, 1}, {w / 2, h - 2},
		{w * 0.2, h * 0.35}, {w * 0.8, h * 0.62}, {w * 0.35, h * 0.9},
	}
	worst := 0
	for _, pt := range points {
		x, y := pt[0], pt[1]
		want := ambientModel(x, y, w, h, base, cover, 0.31)
		o := img.PixOffset(int(x), int(y))
		for i := 0; i < 3; i++ {
			d := int(abs32(float32(img.Pix[o+i]) - want[i]))
			if d > worst {
				worst = d
			}
		}
	}
	// 容差 3：8 位合成取整 + 「逐层乘 alpha」与「整组 opacity」的 ≤1.6 灰阶差
	//（见 shell.go 的 ambientSheenGradient 注释）。
	if worst > 3 {
		t.Fatalf("氛围光五层与原版 CSS 的最大通道差 %d 灰阶，超过容差 3", worst)
	}
}

// TestAmbientBasicTextureOpacity 钉住 `.app-shell.texture-basic::before`：
// 高级质感关闭时整层不透明度是 .35 而不是 .31。
func TestAmbientBasicTextureOpacity(t *testing.T) {
	const w, h = 400, 300
	img := ambientSample(t, w, h, false)
	base := [3]float32{128, 128, 128}
	cover := [3]float32{51, 102, 153}
	// 取中心点：五层里三层 alpha 都最小，能放大 opacity 的差。
	x, y := float32(w/2), float32(h/2)
	want := ambientModel(x, y, w, h, base, cover, 0.35)
	old := ambientModel(x, y, w, h, base, cover, 0.31)
	o := img.PixOffset(int(x), int(y))
	for i := 0; i < 3; i++ {
		got := float32(img.Pix[o+i])
		if abs32(got-want[i]) > 3 {
			t.Fatalf("通道 %d 实测 %.0f，按 .35 应为 %.1f（按 .31 是 %.1f）", i, got, want[i], old[i])
		}
	}
}

// TestAmbientIsNotTheOldApproximation 是反证：修复前的「封面 + 一层 .22→.14」
// 与现在的五层在中段差出 10 灰阶以上 —— 说明本轮改的是可见像素，不是等价重写。
func TestAmbientIsNotTheOldApproximation(t *testing.T) {
	const w, h = 800, 600
	base := [3]float32{128, 128, 128}
	cover := [3]float32{51, 102, 153}
	x, y := float32(w/2), float32(h*0.5)
	now := ambientModel(x, y, w, h, base, cover, 0.31)
	old := oldAmbientModel(y, h, base, cover)
	worst := 0
	for i := 0; i < 3; i++ {
		d := int(abs32(now[i] - old[i]))
		if d > worst {
			worst = d
		}
	}
	if worst < 10 {
		t.Fatalf("新写法与原写法在中段只差 %d 灰阶，反证失效（改动不可见？）", worst)
	}
}

// TestAmbientSheenAlphaPremultiplied 钉住「整层 opacity 乘进停靠点 alpha」。
func TestAmbientSheenAlphaPremultiplied(t *testing.T) {
	prem := ambientSheenGradient(ambientBeforeSheenStops, ambientOpacity)
	// .14/.03/.12 先各自量化成 8 位 alpha（36/8/31），再乘 .31 取整 → 11/2/10。
	want := []uint8{11, 2, 10}
	if len(prem.Stops) != len(want) {
		t.Fatalf("停靠点数 %d，应为 %d", len(prem.Stops), len(want))
	}
	for i, w := range want {
		if got := prem.Stops[i].Color.A; got != w {
			t.Fatalf("停靠点 %d 的 alpha 是 %d，应为 %d", i, got, w)
		}
	}
	if prem.Angle != 180 {
		t.Fatalf("洗光角度 %v，原版是 180deg", prem.Angle)
	}
}

// TestAmbientVignetteBakeSize 钉住径向暗角的烘焙尺寸：氛围光是全窗大面积、
// 低频的一层，用 384²（cardglow 的精度）会白白常驻 1.1MB。
func TestAmbientVignetteBakeSize(t *testing.T) {
	if ambientVignetteBake != 256 {
		t.Fatalf("暗角烘焙边长 %d，应为 256", ambientVignetteBake)
	}
	if b := ambientBeforeVignetteBmp(); b == nil {
		t.Fatal("::before 暗角位图没烘出来")
	}
	if b := ambientAfterVignetteBmp(); b == nil {
		t.Fatal("::after 暗角位图没烘出来")
	}
}
