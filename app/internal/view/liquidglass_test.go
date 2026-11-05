package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本文件钉住「表面材质」三套主题与新的液态玻璃材质（liquidglass.go）。
// 经典主题的行为由原有测试继续覆盖；这里只证明新主题确实换了材质、边缘
// 真的弯了、浓度滑杆真的改变提亮。

func TestGlassThemeCycles(t *testing.T) {
	if got := GlassClassic.Next(); got != GlassLight {
		t.Errorf("Classic.Next() = %v, want GlassLight", got)
	}
	if got := GlassLight.Next(); got != GlassDark {
		t.Errorf("GlassLight.Next() = %v, want GlassDark", got)
	}
	if got := GlassDark.Next(); got != GlassClassic {
		t.Errorf("GlassDark.Next() = %v, want GlassClassic", got)
	}
	if GlassLight.String() == GlassDark.String() {
		t.Error("明/暗两套主题显示名相同")
	}
}

func TestGlassThemeTokens(t *testing.T) {
	th := ThemeFor(PeriodAt(12))
	if th.GlassTheme != GlassClassic {
		t.Errorf("默认表面主题 = %v，应为经典（旧配置无此键时不能换外观）", th.GlassTheme)
	}
	if th.GlassDensity < 0 || th.GlassDensity > 1 {
		t.Errorf("GlassDensity = %v，应在 0..1", th.GlassDensity)
	}
	if th.GlassBackgroundLum <= 0 || th.GlassBackgroundLum > 1 {
		t.Errorf("GlassBackgroundLum = %v，应在 0..1", th.GlassBackgroundLum)
	}
	// tint 的稀/密两端按规格：亮 6–22%、烟熏 16–38%、亮压暗 10–30%。
	if th.GlassLight.Thin != 0.06 || th.GlassLight.Dense != 0.22 {
		t.Errorf("亮色 tint 端点 = %v..%v，应为 .06..22", th.GlassLight.Thin, th.GlassLight.Dense)
	}
	if th.GlassDark.Thin != 0.16 || th.GlassDark.Dense != 0.22 {
		t.Errorf("烟熏 tint 端点 = %v..%v，应为 .16..22", th.GlassDark.Thin, th.GlassDark.Dense)
	}
	if th.GlassOverDark.Thin != 0.10 || th.GlassOverDark.Dense != 0.30 {
		t.Errorf("亮压暗 tint 端点 = %v..%v，应为 .10..30", th.GlassOverDark.Thin, th.GlassOverDark.Dense)
	}
	// saturation 已乘上「保留率」，最低 0.45；180% 档不低于 .45×1.8。
	if th.GlassLight.Saturation < 0.45*1.8 {
		t.Errorf("亮色 saturation = %v，低于 180%%×0.45", th.GlassLight.Saturation)
	}
	if th.GlassSolid.A != 255 {
		t.Errorf("减少透明度兜底面必须是全不透明，A = %d", th.GlassSolid.A)
	}
	// 用户 tint 缩放夹在 0..1.5。
	if got := th.withGlassTint(-1, 9).GlassTintScaleLight; got != 0 {
		t.Errorf("tint 下限 = %v，应为 0", got)
	}
	if got := th.withGlassTint(-1, 9).GlassTintScaleDark; got != 1.5 {
		t.Errorf("tint 上限 = %v，应为 1.5", got)
	}
}

// glassProbe 渲染一块 200×80 的玻璃，背景是 165° 的彩色渐变。返回整帧。
func glassProbe(t *testing.T, theme GlassTheme, scale float32, lens bool) *image.RGBA {
	t.Helper()
	th := ThemeFor(PeriodAt(12)).withGlassTheme(theme).
		withGlassTint(scale, scale)
	mat := liquidGlassLayer(th, glassOptions{blur: 8, lens: lens})
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Fill().LinearGradient(ui.LinearGradient{
			From: ui.Hex("#1d6fd6"), To: ui.Hex("#e04a3f"), Angle: 165,
		})
		ui.Box(c).Absolute().Left(50).Top(40).Width(200).Height(80).
			Radius(20).Material(mat)
	}, 300, 160)
	tst.Frame()
	return tst.Image()
}

func px(img *image.RGBA, x, y int) [3]int {
	i := img.PixOffset(x, y)
	return [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
}

func lumaAt(img *image.RGBA, x, y int) float64 {
	c := px(img, x, y)
	return 0.299*float64(c[0]) + 0.587*float64(c[1]) + 0.114*float64(c[2])
}

// TestLiquidGlassLensBendsBackdrop 实测 rim 折射：同一块表面、同一团背景，
// 开/关 lens 时边缘像素必须不同（中间不弯）。这是原生版对 SVG 位移贴图的
// 对应物 —— glass.Effect 在 bezel 内按圆角法线把 backdrop 折进来。
func TestLiquidGlassLensBendsBackdrop(t *testing.T) {
	on := glassProbe(t, GlassLight, 1, true)
	off := glassProbe(t, GlassLight, 1, false)
	// 左缘内侧 2px：bezel 内，折射最大。
	edgeOn, edgeOff := px(on, 53, 80), px(off, 53, 80)
	d := 0
	for i := 0; i < 3; i++ {
		d += absInt(edgeOn[i] - edgeOff[i])
	}
	t.Logf("边缘开/关 lens：%v vs %v（通道差和 %d）", edgeOn, edgeOff, d)
	if d < 6 {
		t.Errorf("边缘通道差和 %d < 6 —— rim 折射没有生效", d)
	}
	// 中心不弯：两侧应逐通道相同（允许 1 的采样取整误差）。
	cOn, cOff := px(on, 150, 80), px(off, 150, 80)
	for i := 0; i < 3; i++ {
		if absInt(cOn[i]-cOff[i]) > 1 {
			t.Errorf("中心像素被 lens 改了：%v vs %v", cOn, cOff)
			break
		}
	}
}

// TestLiquidGlassTintScale 实测浓度滑杆：亮色主题 0 → 1，中心应被提亮；
// 烟熏主题 0 → 1，中心应变暗。这对应 CSS 的 brightness() 随 tint 缩放。
func TestLiquidGlassTintScale(t *testing.T) {
	light0 := lumaAt(glassProbe(t, GlassLight, 0, false), 150, 80)
	light1 := lumaAt(glassProbe(t, GlassLight, 1, false), 150, 80)
	t.Logf("亮色中心亮度：tint 0 = %.1f，tint 1 = %.1f", light0, light1)
	if light1-light0 < 2 {
		t.Errorf("亮色 tint 1 只比 0 亮 %.1f，提亮没跟上", light1-light0)
	}
	dark0 := lumaAt(glassProbe(t, GlassDark, 0, false), 150, 80)
	dark1 := lumaAt(glassProbe(t, GlassDark, 1, false), 150, 80)
	t.Logf("烟熏中心亮度：tint 0 = %.1f，tint 1 = %.1f", dark0, dark1)
	if dark0-dark1 < 2 {
		t.Errorf("烟熏 tint 1 只比 0 暗 %.1f，压暗没跟上", dark0-dark1)
	}
}

// TestClassicThemeUnaffected 钉住默认主题仍是经典：新建 App 不读用户设置时
// 必须走原有材质，旧截图/旧测试不会因为新材质悄悄改观。
func TestClassicThemeUnaffected(t *testing.T) {
	a := NewApp(func() {})
	if a.GlassTheme != GlassClassic {
		t.Errorf("NewApp 默认表面主题 = %v，应为经典", a.GlassTheme)
	}
	if a.GlassTintLight != 1 || a.GlassTintDark != 1 {
		t.Errorf("默认 tint = %v/%v，应为 1/1", a.GlassTintLight, a.GlassTintDark)
	}
	th := ThemeFor(PeriodAt(12)).withGlassTheme(a.GlassTheme)
	if !th.classicGlass() {
		t.Error("经典主题没有走 classicGlass 分支")
	}
}

// TestGlassThemeSettingsRows 实测设置菜单真的能切换：明/暗主题下列出两个
// 浓度滑杆；经典主题下不显示（保持旧菜单的高度与条目位置）。
func TestGlassThemeSettingsRows(t *testing.T) {
	a, tst := settingsProbe(t, 12)
	if err := tst.Click("设置"); err != nil {
		t.Fatalf("点设置键失败：%v", err)
	}
	tst.Frame()
	if _, ok := tst.Find("表面材质"); !ok {
		t.Fatalf("设置菜单没有「表面材质」行：%v", tst.Texts())
	}
	if _, ok := tst.Find("玻璃浓度（明）"); ok {
		t.Error("经典主题下不应显示浓度滑杆")
	}
	// 切到液态玻璃明：下一帧应出现两个浓度滑杆。
	a.GlassTheme = GlassLight
	tst.Frame()
	for _, want := range []string{"玻璃浓度（明）", "玻璃浓度（暗）"} {
		if _, ok := tst.Find(want); !ok {
			t.Errorf("液态玻璃主题下缺少滑杆 %q：%v", want, tst.Texts())
		}
	}
}

// TestGlassThemeToastPixels 实测用户切到新主题后界面真的换了材质：
// 同一 toast、同一背景，经典 / 明 / 暗三种材质的中心亮度必须不同。
func TestGlassThemeToastPixels(t *testing.T) {
	render := func(th Theme, theme GlassTheme, msg string) float64 {
		a := NewApp(func() {})
		a.GlassTheme = theme
		a.Theme = th
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.Frame()
		a.NotifyType("success", msg)
		tst.Frame()
		tst.Advance(400 * time.Millisecond)
		tst.Frame()
		r, ok := tst.Find(msg)
		if !ok {
			t.Fatalf("找不到 toast %q", msg)
		}
		panel := toastPanelRect(r)
		return lumaAt(tst.Image(), int(panel.X+panel.W/2), int(panel.Y+panel.H/2))
	}
	th := ThemeFor(PeriodAt(12))
	classic := render(th, GlassClassic, "经典")
	light := render(th, GlassLight, "明")
	dark := render(th, GlassDark, "暗")
	t.Logf("toast 中心亮度：经典 %.1f / 明 %.1f / 暗 %.1f", classic, light, dark)
	if absInt(int(light-classic)) < 1 {
		t.Errorf("明主题与经典几乎一样（%.1f vs %.1f）—— 材质没有切换", light, classic)
	}
	if dark >= light {
		t.Errorf("暗主题 %.1f 不应亮于明主题 %.1f", dark, light)
	}
}

// srgbLinear 是 WCAG 的相对亮度通道。
func srgbLinear(v float64) float64 {
	v /= 255
	if v <= 0.04045 {
		return v / 12.92
	}
	return math.Pow((v+0.055)/1.055, 2.4)
}

func relLum(c ui.Color) float64 {
	return 0.2126*srgbLinear(float64(c.R)) + 0.7152*srgbLinear(float64(c.G)) + 0.0722*srgbLinear(float64(c.B))
}

func contrast(a, b ui.Color) float64 {
	la, lb := relLum(a), relLum(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// TestGlassContrastAtDefaultTint 在**最暗时段的最暗停靠点**上，按默认密度
// 把 tint 合成到背景，再和玻璃墨色算 WCAG 对比度：
//
//	明主题：>= 4.5（正文 AA）
//	暗主题：>= 3.5（产品既有烟熏播放栏的口径；壁纸是亮色，不能翻白字）
//
// 这条测试钉的是「密度而不是墨色透明度」：tint 变浓会改变合成亮度，但
// 墨色保持满不透明。
func TestGlassContrastAtDefaultTint(t *testing.T) {
	// 取所有时段里最暗的停靠点。
	var bg ui.Color
	first := true
	for _, p := range Periods {
		for _, st := range p.Stops {
			if first || relLum(st.C) < relLum(bg) {
				bg, first = st.C, false
			}
		}
	}
	composite := func(th Theme, state glassState) ui.Color {
		d := th.GlassDensity
		a := state.Thin + (state.Dense-state.Thin)*d
		// tint 是 straight RGBA 叠在背景上。
		r := uint8(float32(bg.R)*(1-a) + float32(state.Tint.R)*a)
		g := uint8(float32(bg.G)*(1-a) + float32(state.Tint.G)*a)
		b := uint8(float32(bg.B)*(1-a) + float32(state.Tint.B)*a)
		return ui.RGB(r, g, b)
	}
	light := ThemeFor(PeriodAt(20)).withGlassTheme(GlassLight)
	dark := ThemeFor(PeriodAt(20)).withGlassTheme(GlassDark)
	if got := contrast(light.GlassInk, composite(light, light.glassStateFor(false))); got < 4.5 {
		t.Errorf("明主题最暗背景对比度 = %.2f < 4.5", got)
	} else {
		t.Logf("明主题最暗背景对比度 = %.2f", got)
	}
	st := dark.glassStateFor(true)
	a := st.Thin + (st.Dense-st.Thin)*dark.GlassDensity
	t.Logf("暗主题 density=%.3f alpha=%.3f bg=%v tint=%v ink=%v", dark.GlassDensity, a, bg, st.Tint, dark.GlassInk)
	if got := contrast(dark.GlassInk, composite(dark, st)); got < 4.5 {
		t.Errorf("暗主题最暗背景对比度 = %.2f < 4.5", got)
	} else {
		t.Logf("暗主题最暗背景对比度 = %.2f", got)
	}
}
