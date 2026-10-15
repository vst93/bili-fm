package view

import (
	"testing"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// TestGlassSurfaceTintDarkens 钉住「玻璃表面染色」真的把框架 Liquid Glass 压暗。
// 框架的 Regular 玻璃会把背后的东西大幅提亮，搜索药丸因此比抽屉面板白一档；
// glassSurfaceTint() 是给这些表面叠的冷灰染色。
func TestGlassSurfaceTintDarkens(t *testing.T) {
	if glassSurfaceTint().A == 0 {
		t.Fatal("glassSurfaceTint() 是透明的，等于没染色")
	}
	render := func(g glass.Glass) int {
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Fill().LinearGradient(ui.LinearGradient{
				From: ui.Hex("#c8d6e6"), To: ui.Hex("#e6eef6"), Angle: 165,
			}).Children(func() {
				ui.Box(c).FillWidth().Height(46).Margin(10).Radius(13).Material(g)
			})
		}, 300, 100)
		tst.Frame()
		img := tst.Image()
		i := img.PixOffset(150, 33)
		return (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
	}
	plain := render(glass.Glass{})
	tinted := render(glass.Glass{Tint: glassSurfaceTint()})
	t.Logf("同一背景：素玻璃亮度 %d，染色后 %d", plain, tinted)
	if tinted >= plain {
		t.Fatalf("染色没有压暗玻璃（%d >= %d）", tinted, plain)
	}
}
