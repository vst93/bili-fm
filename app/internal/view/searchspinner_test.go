package view

import (
	"math"
	"testing"
)

// TestSearchSubmitSpinnerRing 无头实测：搜索中提交键里画的是原版
// `.search-submit-spinner` 的「细环 + 顶弧」（14×14、2px 描边），而不是 mygo
// 的辐条 Spinner、也不是对话框那枚 28×28。取按钮矩形里的泛蓝像素包围盒——
// 应 ≈ 14×14，且中心线半径（(14−2)/2 = 6）上的圆环连续（辐条 spinner 在半径
// 6 上只覆盖 ≈1/3 的方向）。
func TestSearchSubmitSpinnerRing(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	l := a.list()
	l.Loading = true // searchField 用 list().Loading 判「搜索中」
	tt := shellTester(t, a)
	r, ok := tt.Find("搜索中")
	if !ok {
		t.Fatalf("找不到搜索提交键：%v", tt.Texts())
	}
	img := tt.Image()
	bluish := func(x, y int) bool {
		if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
			return false
		}
		i := img.PixOffset(x, y)
		return int(img.Pix[i+2])-int(img.Pix[i]) > 30
	}
	x0, y0, x1, y1 := int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H)
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if bluish(x, y) {
				minX, minY = min(minX, x), min(minY, y)
				maxX, maxY = max(maxX, x), max(maxY, y)
			}
		}
	}
	if maxX < 0 {
		t.Fatalf("搜索键里没有泛蓝像素：spinner 没画出来（键 rect %+v）", r)
	}
	w, h := maxX-minX+1, maxY-minY+1
	t.Logf("搜索 spinner 泛蓝包围盒 %d,%d..%d,%d（%d×%d）", minX, minY, maxX, maxY, w, h)
	if w < 11 || w > 16 || h < 11 || h > 16 {
		t.Errorf("搜索 spinner 包围盒 %d×%d，应 ≈ 14×14（原版 .search-submit-spinner）", w, h)
	}
	cx, cy := (minX+maxX)/2, (minY+maxY)/2
	const rad = 6.0
	hit := 0
	for k := 0; k < 72; k++ {
		ang := float64(k) * 5 * math.Pi / 180
		ok := false
		for d := -1.2; d <= 1.2; d += 0.4 {
			rr := rad + d
			px := cx + int(math.Round(rr*math.Cos(ang)))
			py := cy + int(math.Round(rr*math.Sin(ang)))
			if bluish(px, py) {
				ok = true
				break
			}
		}
		if ok {
			hit++
		}
	}
	coverage := float64(hit) / 72
	t.Logf("半径 %v 上 72 方向的泛蓝覆盖率 = %.2f", rad, coverage)
	if coverage < 0.9 {
		t.Errorf("搜索 spinner 不是连续圆环：覆盖率 %.2f（辐条 spinner ≈ 0.35）", coverage)
	}
}
