package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// TestCSSGlassMatchesOriginal 钉住「搜索药丸/播放栏的玻璃按原版 CSS 的两层来」：
//
//	backdrop-filter: blur(20px) saturate(1.45)   → glass.Blur{Radius: 20}
//	background-color: rgba(255,255,255,.22)      → searchPillVeil()
//
// 而不是框架的 glass.Glass（Regular 把背景的黑抬到 54%，整块发白、不透）。
func TestCSSGlassMatchesOriginal(t *testing.T) {
	if liquidBlurRadius != 20 {
		t.Errorf("模糊半径 = %v，应为 20（原版 --liquid-blur: blur(20px)）", liquidBlurRadius)
	}
	if got, want := searchPillVeil(), ui.Hex("#ffffff").Alpha(0.22); got != want {
		t.Errorf("搜索药丸底色 = %v，应为 %v（原版 --liquid-bg）", got, want)
	}
}

// TestCSSGlassShowsBackdropThrough 是无头实测：同一块面铺在深/浅两种背景上，
// 透出来的亮度差越大 = 越透。
//
// 改前用 glass.Glass（Regular）时，深背景被抬到接近白，两种背景下的表面几乎
// 一样亮（差值小）—— 用户报的「透度不够」。改成 cssGlass（只模糊 + 白 .22 /
// 近白 .36 的底）后，差值应显著大于 Regular 玻璃。
func TestCSSGlassShowsBackdropThrough(t *testing.T) {
	// 在深/浅背景上渲染一块面，返回中心点的亮度。
	luma := func(dark bool, g ui.Material) int {
		from, to := ui.Hex("#e8eff8"), ui.Hex("#f4f8fc")
		if dark {
			from, to = ui.Hex("#1a2230"), ui.Hex("#2b3648")
		}
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().LinearGradient(ui.LinearGradient{From: from, To: to, Angle: 165}).
				Children(func() {
					ui.Box(c).FillWidth().Height(46).Margin(10).Radius(13).Material(g)
				})
		}, 300, 100)
		tst.Frame()
		img := tst.Image()
		i := img.PixOffset(150, 33)
		return (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
	}
	spread := func(g ui.Material) int { return luma(false, g) - luma(true, g) }

	css := cssGlassLayer(liquidBlurRadius, searchPillVeil(), nil, 1)
	regular := glass.Glass{}
	t.Logf("深/浅背景下的表面亮度差：cssGlass %d，glass.Glass(Regular) %d", spread(css), spread(regular))
	if spread(css) <= spread(regular) {
		t.Errorf("cssGlass 应比 Regular 玻璃透出更多背景：%d <= %d", spread(css), spread(regular))
	}
}

// TestGlassPaneVeilThinAndSheenFollows 钉住这轮用户要求：
//   - 搜索药丸 / 播放栏只保留原版玻璃令牌的四成（更透），并随窗口
//     透明度按平方曲线继续变透；
//   - 烘焙的静态洗光是固定位图，窗口透明度升高时必须一起淡下去，
//     否则 veil 已经变薄、白色反射仍是面上一块不透明的纱。
func TestGlassPaneVeilThinAndSheenFollows(t *testing.T) {
	if glassPaneVeil > .45 || glassPaneVeilDark > .12 {
		t.Fatalf("大面积玻璃的 veil 比例仍偏实：light=%v dark=%v", glassPaneVeil, glassPaneVeilDark)
	}
	light := ThemeFor(PeriodAt(12))
	search := light.glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark)
	if got, want := search.A, uint8(22); got > want+1 {
		t.Errorf("搜索药丸白底 = %d，应约为 %d（白 .22 × .4）", got, want)
	}
	bar := playerBarTint(light)
	if bar.A == 0 || bar.A > 41 {
		t.Errorf("播放栏底色 alpha = %d，应约为 37（.36 × .4）", bar.A)
	}
	dark := light.withAppearance("dark", 0)
	// 深色：搜索药丸 alpha 只剩 ~15（#18273b .06 × 四成）—— 比浅色更透，
	// 用户 2026-11-11 明确要求「深色下再透一点」。
	if got, want := dark.glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark).A, uint8(15); got > want+1 {
		t.Errorf("深色搜索药丸 alpha = %d，应约为 %d（#18273b .06 × 四成）", got, want)
	}
	if dark.glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark).A >=
		light.glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark).A {
		t.Error("深色大面积玻璃应比浅色更透")
	}
	if got := light.glassPaneSheenFade(); math.Abs(float64(got-glassPaneSheenBase)) > .001 {
		t.Errorf("默认外观的大面积洗光 = %v，应约为 %v", got, glassPaneSheenBase)
	}
	if got := light.withAppearance("light", .4).glassPaneSheenFade(); math.Abs(float64(got)-.2592) > .001 {
		t.Errorf("40%% 透明度下大面积洗光 = %v，应约为 .2592", got)
	}
	if got := light.withAppearance("light", .25).glassPaneSheenFade(); math.Abs(float64(got)-.405) > .001 {
		t.Errorf("25%% 透明度下大面积洗光 = %v，应约为 .405", got)
	}
	// 平方曲线：40% 时大面板底色只剩默认的 36%，抽屉仍走线性的 60%。
	if got := light.withAppearance("light", .4).glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark).A; got > 10 {
		t.Errorf("40%% 透明度下搜索药丸 alpha = %d，应约为 8（默认 22 × .36）", got)
	}
	if got := light.withAppearance("light", .4).glassSheenFade(); math.Abs(float64(got)-.6) > .001 {
		t.Errorf("40%% 透明度下抽屉洗光 = %v，应约为 .6", got)
	}
	// 深色底上的白洗光比浅色更显“蒙灰”，默认 base 收到 .05。
	darkPane := light.withAppearance("dark", 0)
	if got := darkPane.glassPaneSheenFade(); math.Abs(float64(got-glassPaneSheenBaseDark)) > .001 {
		t.Errorf("深色默认大面积洗光 = %v，应约为 %v", got, glassPaneSheenBaseDark)
	}
	if got := darkPane.withAppearance("dark", .4).glassPaneSheenFade(); math.Abs(float64(got)-.018) > .001 {
		t.Errorf("深色 40%% 透明度下大面积洗光 = %v，应约为 .018", got)
	}
	if got := darkPane.withAppearance("dark", .4).glassPaneTint(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark).A; got > 6 {
		t.Errorf("深色 40%% 透明度下搜索药丸 alpha = %d，应约为 5", got)
	}
	if got := light.withAppearance("light", .4).withPreferences(false, true).glassPaneSheenFade(); got != 1 {
		t.Errorf("减少透明度下大面积洗光不应淡出，got %v", got)
	}
}

// TestGlassSheenFadePaints 是无头反证：fade < 1 的那条 ImageRegion+Tint
// 路径真的把洗光画淡了（只测 glassSheenFade 的返回值抓不到接线漏掉）。
func TestGlassSheenFadePaints(t *testing.T) {
	th := ThemeFor(PeriodAt(12)).withAppearance("light", .4)
	luma := func(fade float32) int {
		tst := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(th.uiTheme())
			c.Root().Background(ui.Hex("#101820"))
			ui.Box(c).Fill().Material(cssGlassLayer(liquidBlurRadius,
				th.glassVeil(searchPillVeil().Alpha(glassPaneVeil), "#18273b", glassPaneVeilDark),
				sheenGlassBaked, fade))
		}, 200, 80)
		tst.Frame()
		p := tst.Image().RGBAAt(40, 8)
		return (int(p.R)*299 + int(p.G)*587 + int(p.B)*114) / 1000
	}
	full, faded := luma(1), luma(.5)
	if faded >= full {
		t.Fatalf("洗光 fade .5 应比原值暗：%d >= %d", faded, full)
	}
	if full-faded < 3 {
		t.Fatalf("洗光 fade .5 几乎没变化：%d → %d（接线可能没走 ImageRegion/Tint）", full, faded)
	}
}
