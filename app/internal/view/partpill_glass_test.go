package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（videoInfo 详情区的选集胶囊 .video-part-pill 液态玻璃）的测试。原版
// globals.css 给这颗胶囊挂了两层此前原生版没画的材质：
//
//	::before  --glass-sheen-static（径向 140%×80% @22% -5% 白 .24 + 线性 170deg
//	          白 .07→.015→透明）铺在平铺的白 .15 底之上
//	::after   --liquid-rim 1px 聚光描边（上缘最亮）
//
// 原生版此前只有 Background(白 .15) + Border(白 .38) + 一条顶部内高光，胶囊比
// 原版平一档。

// partPillBackdrop 是像素对照用的深底：sheen / rim 都是白色光泽，铺在白底上量
// 不出信号（与 partSearchGlass 的测试同理）。
var partPillBackdrop = ui.Hex("#334155")

// TestPartPillGlassPaint 直接钉住 partPillGlass 画出的两层材质：在 300×36 的胶囊
// 上叠玻璃后，框域里有明显像素被改动（画的是光泽 / 描边，不是空操作）。
func TestPartPillGlassPaint(t *testing.T) {
	render := func(glass bool) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(partPillBackdrop)
			b := ui.Row(c).Width(300).Height(36).Radius(RadiusPill).
				Background(ui.Hex("#ffffff").Alpha(0.15))
			if glass {
				b = partPillGlass(b)
			}
		}, 340, 80, 1)
	}
	on, off := render(true), render(false)
	diff, total := 0, 0
	for y := 0; y < on.Bounds().Dy(); y++ {
		for x := 0; x < on.Bounds().Dx(); x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("partPillGlass：%d/%d 像素被改动", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受玻璃层影响（<5%%）—— sheen / rim 没画上？", diff, total)
	}
}

// TestPartPillGlassLayers 分辨两层各自的贡献，并钉住 sheen 的方向性（原版
// --glass-sheen-static 的径向中心在 22% 处，光聚在**左上**，右下几乎不亮）：
//
//	① 只铺平底（对照）
//	② 加 sheen 后：左上的平均亮度明显抬升，且左上 > 右上（径向偏心在左）
//	③ 加 rim 后：胶囊**最上沿**比最下沿亮（--liquid-rim 上缘最亮）
func TestPartPillGlassLayers(t *testing.T) {
	const w, h = 300, 36
	render := func(withSheen, withRim bool) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(partPillBackdrop)
			b := ui.Row(c).Width(w).Height(h).Radius(RadiusPill).
				Background(ui.Hex("#ffffff").Alpha(0.15))
			if withSheen || withRim {
				b = b.Draw(func(p *ui.Painter, r ui.Rect) {
					if withSheen {
						paintPlayerSheen(p, r, RadiusPill, sheenGlassBaked.bitmap())
					}
					if withRim {
						paintPlayerRim(p, r, RadiusPill, 1)
					}
				})
			}
		}, 340, 80, 1)
	}
	// 胶囊在 340×80 的画布上居中，先找到它的左上角。
	base := render(false, false)
	x0, y0 := -1, -1
	for y := 0; y < base.Bounds().Dy() && y0 < 0; y++ {
		for x := 0; x < base.Bounds().Dx(); x++ {
			i := base.PixOffset(x, y)
			if base.Pix[i+0] > 60 && base.Pix[i+1] > 70 && base.Pix[i+2] > 80 {
				x0, y0 = x, y
				break
			}
		}
	}
	if x0 < 0 {
		t.Fatal("在画布上找不到胶囊本体（白 .15 底）")
	}

	// 左上 1/4 × 上半 的矩形均值（避开 rim 的 1px，再往里缩 2px）。
	cellMean := func(img *image.RGBA, x, y, cw, ch int) float64 {
		sum, n := 0, 0
		for yy := y; yy < y+ch; yy++ {
			for xx := x; xx < x+cw; xx++ {
				i := img.PixOffset(xx, yy)
				sum += int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
				n += 3
			}
		}
		return float64(sum) / float64(n)
	}
	cw, ch := w/4, h/2-3
	tl := func(img *image.RGBA) float64 { return cellMean(img, x0+2, y0+2, cw, ch) }
	tr := func(img *image.RGBA) float64 { return cellMean(img, x0+w-cw-2, y0+2, cw, ch) }

	sheen := render(true, false)
	baseTL, sheenTL := tl(base), tl(sheen)
	t.Logf("左上亮度：平底 %.1f → 加 sheen %.1f；加 sheen 后左上 %.1f / 右上 %.1f",
		baseTL, sheenTL, sheenTL, tr(sheen))
	if sheenTL-baseTL < 1 {
		t.Errorf("sheen 只把左上抬高 %.2f（<1）—— --glass-sheen-static 没接上？", sheenTL-baseTL)
	}
	if tr(sheen) >= sheenTL {
		t.Errorf("sheen 右上 %.1f 不弱于左上 %.1f —— 径向中心不在左侧（22%%）？", tr(sheen), sheenTL)
	}

	// rim：只叠描边的图里，胶囊的第 1 行（最上沿）明显比最后一行亮。
	rim := render(false, true)
	topRow := cellMean(rim, x0+2, y0, w-4, 1)
	botRow := cellMean(rim, x0+2, y0+h-1, w-4, 1)
	t.Logf("rim 描边：上沿 %.1f / 下沿 %.1f", topRow, botRow)
	if topRow-botRow < 5 {
		t.Errorf("--liquid-rim 上沿 %.1f 未明显亮于下沿 %.1f —— 聚光描边方向不对？", topRow, botRow)
	}
}

// TestPartPillGlassWired 无头反证：真是 partPill 接上了 partPillGlass —— 把
// partPillGlass 临时换成 no-op 后，胶囊区域的像素跟着变。
func TestPartPillGlassWired(t *testing.T) {
	render := func(stub bool) *image.RGBA {
		if stub {
			saved := partPillGlass
			partPillGlass = func(b ui.Element) ui.Element { return b }
			defer func() { partPillGlass = saved }()
		}
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Part: "第一集"}
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(partPillBackdrop)
			ui.Column(c).Padding(20).Children(func() { a.partPill(c) })
		}, 520, 120, 1)
	}
	on, off := render(false), render(true)
	diff, total := 0, 0
	for y := 0; y < on.Bounds().Dy(); y++ {
		for x := 0; x < on.Bounds().Dx(); x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("选集胶囊：%d/%d 像素随 partPillGlass 开关变化", diff, total)
	if diff < total/40 {
		t.Errorf("只有 %d/%d 像素随 partPillGlass 变化（<2.5%%）—— partPill 没接玻璃层？", diff, total)
	}
}

// TestPartPillGlassSheenSpec 钉住 partPillGlass 用的是 --glass-sheen-static
// （浅色通用那套），不是烟熏玻璃的 --player-sheen：两者的径向中心 / 半径 /
// 线性色停都不同，接错会在浅色面板上铺出偏暗的一层。
//
// 除常量外还做一次逐像素恒等：partPillGlass 的输出必须与「手动画
// sheenGlassBaked 位图 + rim opacity 1」**逐字节**一致；换成 sheenDarkBaked 后
// 同一张底就得跟着变（证明这条判据真的能看出层选错）。
func TestPartPillGlassSheenSpec(t *testing.T) {
	if sheenGlass.RCX != 0.22 || sheenGlass.RCY != -0.05 || sheenGlass.RRX != 1.40 || sheenGlass.RRY != 0.80 {
		t.Errorf("--glass-sheen-static 几何 = %+v，与原版 radial-gradient(140%% 80%% at 22%% -5%%) 不符", sheenGlass)
	}
	if sheenGlass.RCol != ui.Hex("#ffffff").Alpha(0.24) {
		t.Errorf("--glass-sheen-static 径向色 = %v，应为白 .24", sheenGlass.RCol)
	}
	if len(sheenGlass.LStops) != 3 || sheenGlass.LStops[0].Col != ui.Hex("#ffffff").Alpha(0.07) ||
		sheenGlass.LStops[1].Col != ui.Hex("#ffffff").Alpha(0.015) {
		t.Errorf("--glass-sheen-static 线性色停 = %+v，应为白 .07 → .015 → 透明", sheenGlass.LStops)
	}
	// 反证：烟熏玻璃那套的径向中心在 y=-20%，不能拿来当胶囊的光泽。
	if sheenDark.RCY == sheenGlass.RCY {
		t.Errorf("--player-sheen 与 --glass-sheen-static 的径向中心相同了，判据失效")
	}

	const w, h = 300, 36
	// mode: 0 = 只铺底（对照）、1 = 走 partPillGlass、2 = 手动画 glass sheen + rim、
	// 3 = 手动画 dark sheen + rim。
	render := func(mode int) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(partPillBackdrop)
			b := ui.Row(c).Width(w).Height(h).Radius(RadiusPill).
				Background(ui.Hex("#ffffff").Alpha(0.15))
			switch mode {
			case 1:
				partPillGlass(b)
			case 2, 3:
				sheen := sheenGlassBaked
				if mode == 3 {
					sheen = sheenDarkBaked
				}
				b.Draw(func(p *ui.Painter, r ui.Rect) {
					paintPlayerSheen(p, r, RadiusPill, sheen.bitmap())
					paintPlayerRim(p, r, RadiusPill, 1)
				})
			}
		}, 340, 80, 1)
	}
	same := func(x, y *image.RGBA) bool { return string(x.Pix) == string(y.Pix) }
	if !same(render(1), render(2)) {
		t.Error("partPillGlass 的输出与「--glass-sheen-static + rim opacity 1」不一致——层选错或漏了一层")
	}
	if same(render(1), render(3)) {
		t.Error("换成 --player-sheen 后像素完全相同，恒等判据失效")
	}
}
