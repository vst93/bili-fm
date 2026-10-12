package view

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// eqBarVisible 报告一个像素是否是均衡器小柱的浅蓝（#7dd3fc）。
func eqBarVisible(pix []uint8, i int) bool {
	r, g, b := int(pix[i]), int(pix[i+1]), int(pix[i+2])
	return b > r+30 && b > 90 && g > 60
}

// eqBarColumns 量出渲染图里各根小柱的竖直高度（按 x 分组，取每组最高一列）。
func eqBarColumns(img *image.RGBA) []int {
	var heights []int
	cur := -1
	for x := 0; x < img.Rect.Dx(); x++ {
		h := 0
		for y := 0; y < img.Rect.Dy(); y++ {
			if eqBarVisible(img.Pix, img.PixOffset(x, y)) {
				h++
			}
		}
		if h > 0 {
			if cur < 0 {
				heights = append(heights, 0)
				cur = len(heights) - 1
			}
			if h > heights[cur] {
				heights[cur] = h
			}
		} else {
			cur = -1
		}
	}
	return heights
}

// eqBarsProbe 在一个固定相位上渲染三根小柱（深底、浅蓝柱，便于逐列量高）。
func eqBarsProbe(phase float32) *image.RGBA {
	return ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#0f172a"))
		ui.Row(c).Absolute().Left(10).Top(20).Children(func() {
			eqBars(c, phase)
		})
	}, 62, 42, 1)
}

// eqBarPeakPhase 返回第 bar 根小柱到满高（keyframes 的 50%）时的循环相位。
func eqBarPeakPhase(bar int) float32 {
	return fracPhase(0.5 - eqBarDelays[bar]/float32(eqPeriod.Seconds()))
}

// TestEqBarScale 钉住均衡器的相位数学：每根柱在自己的相位过半时到 1、两端回
// 到 .35，峰值两侧对称、全程不越界，三根因负延迟错开不相位同步。
func TestEqBarScale(t *testing.T) {
	for bar := range eqBarHeights {
		peak := eqBarPeakPhase(bar)
		if got := eqBarScale(peak, bar); math.Abs(float64(got-eqMaxScale)) > 1e-4 {
			t.Errorf("柱%d 在峰值相位 %v 应为满高 1，得到 %v", bar, peak, got)
		}
		l := eqBarScale(fracPhase(peak-0.2), bar)
		r := eqBarScale(fracPhase(peak+0.2), bar)
		if math.Abs(float64(l-r)) > 1e-4 {
			t.Errorf("柱%d 峰值两侧不对称：%v vs %v", bar, l, r)
		}
		for i := 0; i <= 100; i++ {
			s := eqBarScale(float32(i)/100, bar)
			if s < eqMinScale-1e-3 || s > eqMaxScale+1e-3 {
				t.Fatalf("柱%d 相位 %v 越界到 %.3f（应落在 [.35,1]）", bar, float32(i)/100, s)
			}
		}
	}
	// 三根错开：柱0 到峰值时，柱1/柱2 都明显更低。
	peak0 := eqBarPeakPhase(0)
	if eqBarScale(peak0, 0) <= eqBarScale(peak0, 1) || eqBarScale(peak0, 0) <= eqBarScale(peak0, 2) {
		t.Errorf("柱0 峰值相位上方三根同步了：%v/%v/%v",
			eqBarScale(peak0, 0), eqBarScale(peak0, 1), eqBarScale(peak0, 2))
	}
}

// TestEqBarsRender 是无头渲染实测：三根小柱真的按相位各自伸缩——
// 柱1 峰值时中间那根 ≈10px、两侧已缩到 ~2px；柱0 峰值时最左那根 ≈5.5px。
func TestEqBarsRender(t *testing.T) {
	mid := eqBarColumns(eqBarsProbe(eqBarPeakPhase(1)))
	if len(mid) != 3 {
		t.Fatalf("应渲染出三根小柱，实际 %d 根（高度 %v）", len(mid), mid)
	}
	// 柱1 峰值：中间最高（满高 10），两侧缩到底。
	if mid[1] < 8 {
		t.Errorf("柱1 峰值时中间柱应≈10px，得到 %d（全部 %v）", mid[1], mid)
	}
	if mid[0] > 4 || mid[2] > 4 {
		t.Errorf("柱1 峰值时两侧应缩到 ~2px，得到 %d / %d", mid[0], mid[2])
	}

	left := eqBarColumns(eqBarsProbe(eqBarPeakPhase(0)))
	if len(left) != 3 {
		t.Fatalf("应渲染出三根小柱，实际 %d 根（高度 %v）", len(left), left)
	}
	if left[0] < 5 || left[0] > 6 {
		t.Errorf("柱0 峰值时最左柱应≈5.5px，得到 %d（全部 %v）", left[0], left)
	}
	// 同一根柱在两个相位下高度不同 ⇒ 动画确实是活的。
	if left[0] <= mid[0]+1 {
		t.Errorf("柱0 在峰值（%d）应比低谷（%d）明显高", left[0], mid[0])
	}
}

// TestSpinDegrees 钉住刷新键「转圈」的开关与角度：静止不转，刷新中按进度匀速
// 转满一圈（原版 .history-refresh-spinning 800ms linear infinite）。
func TestSpinDegrees(t *testing.T) {
	if got := spinDegrees(false, 0.37); got != 0 {
		t.Errorf("静止时不应旋转，得到 %v", got)
	}
	if got := spinDegrees(true, 0.25); math.Abs(float64(got-90)) > 1e-4 {
		t.Errorf("进度 .25 应为 90 度，得到 %v", got)
	}
	if got := spinDegrees(true, 0.5); math.Abs(float64(got-180)) > 1e-4 {
		t.Errorf("进度 .5 应为 180 度，得到 %v", got)
	}
}

// TestRefreshSpinRenders 是无头渲染实测：给刷新图标套上旋转确实会改变像素。
func TestRefreshSpinRenders(t *testing.T) {
	renderAt := func(progress float32) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.Hex("#ffffff"))
			ui.Box(c).Absolute().Left(10).Top(10).Size(32, 32).Center().Children(func() {
				ui.Icon(c, iconRefresh).Size(20, 20).TextColor(ui.Hex("#333333")).
					Rotate(spinDegrees(true, progress))
			})
		}, 52, 52, 1)
	}
	rotated := renderAt(0.25) // 90 度
	straight := renderAt(0)
	diff := 0
	for i := range straight.Pix {
		if straight.Pix[i] != rotated.Pix[i] {
			diff++
		}
	}
	if diff == 0 {
		t.Fatal("旋转 90 度后刷新图标没有任何像素变化")
	}
}
