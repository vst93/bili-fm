package view

import (
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

	css := cssGlassLayer(liquidBlurRadius, searchPillVeil(), nil)
	regular := glass.Glass{}
	t.Logf("深/浅背景下的表面亮度差：cssGlass %d，glass.Glass(Regular) %d", spread(css), spread(regular))
	if spread(css) <= spread(regular) {
		t.Errorf("cssGlass 应比 Regular 玻璃透出更多背景：%d <= %d", spread(css), spread(regular))
	}
}
