package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestDanmakuTextShadowHalo 钉住弹幕文字那条白色柔光阴影
//（原版 `.danmaku-message-text { text-shadow: 0 1px 1px rgba(255,255,255,.68) }`）
// 又被画回来了，并且画的是**字形副本**，不是 4408e18 那块被用户否掉的方形白底。
//
// 反证：把 danmakuTextInkShadow 换成全透明 → 与真实阴影的帧逐像素相同 → 红；
// 把字形副本换成一整块矩形 → 差异像素数逼近文字盒面积 → 红。
func TestDanmakuTextShadowHalo(t *testing.T) {
	render := func(shadow func() ui.Color) *ui.Tester {
		orig := danmakuTextInkShadow
		t.Cleanup(func() { danmakuTextInkShadow = orig })
		danmakuTextInkShadow = shadow
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Danmaku = []Danmaku{{Time: 0, Text: "弹幕阴影测试"}}
		a.Pos = -1
		a.Drawer = DrawerDanmaku
		tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
		tt.Frame()
		return tt
	}
	diffPixels := func(a, b *ui.Tester) int {
		ia, ib := a.Image(), b.Image()
		n := 0
		for y := 0; y < 720; y++ {
			for x := 0; x < 900; x++ {
				i, j := ia.PixOffset(x, y), ib.PixOffset(x, y)
				if ia.Pix[i] != ib.Pix[j] || ia.Pix[i+1] != ib.Pix[j+1] || ia.Pix[i+2] != ib.Pix[j+2] {
					n++
				}
			}
		}
		return n
	}

	real := render(danmakuTextInkShadow)
	off := render(func() ui.Color { return ui.Transparent })
	n := diffPixels(real, off)
	if n == 0 {
		t.Fatal("白色柔光阴影整条没画出来（与全透明的帧逐像素相同）")
	}

	// 形状判定：差异像素必须远小于文字盒面积 —— 矩形白底会填满整块。
	r, ok := real.Find("弹幕阴影测试")
	if !ok {
		t.Fatal("找不到弹幕文字")
	}
	area := int(r.W*r.H) + 8*int(r.W) // 盒面积 + 上下各 1px 的偏移余量
	t.Logf("阴影差异像素 %d；文字盒面积约 %d", n, area)
	if n*100 >= area*70 {
		t.Errorf("差异像素 %d 接近文字盒面积 %d：阴影像是整块矩形（用户否掉的那种），不是字形副本", n, area)
	}
}
