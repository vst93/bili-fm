package view

import (
	"image"
	"testing"
)

// TestSkeletonItemCountMatchesOriginal 钉住骨架屏的占位卡数量与行数：
// 原版 `ListSkeleton` 的 `rows` 默认 6（三个调用点都用默认值），是**固定 6 张**
// 而不是「按列数铺满行数」——三列 2 排、两列 3 排。此前原生版写死 3 排，宽窗下
// 多出一整排（9 张）。
func TestSkeletonItemCountMatchesOriginal(t *testing.T) {
	if skeletonItems != 6 {
		t.Fatalf("skeletonItems = %d，原版 ListSkeleton 默认 rows = 6", skeletonItems)
	}
	for _, c := range []struct {
		width float32
		want  int
	}{
		{400, 3}, // 两列 → 3 排
		{560, 3},
		{639, 3},
		{640, 2}, // 三列 → 2 排（min-width 断点本身进三列）
		{800, 2},
		{1400, 2},
	} {
		if got := skeletonRows(c.width); got != c.want {
			t.Errorf("skeletonRows(%v) = %d，应为 %d（%d 张 / %d 列）",
				c.width, got, c.want, skeletonItems, gridColsFor(c.width))
		}
	}
}

// TestSkeletonPulseEased 钉住脉冲曲线的取值与「缓入缓出」形状：原版是
// `list-skeleton-pulse 1.4s ease-in-out infinite`（0%/100% .55、50% 1）。
// 线性三角波在 1/8 周期处是 0.6625，缓入后明显更低（≈0.578）——这条正是
// 「把 timing-function 从 linear 换成 ease-in-out」的反证。
func TestSkeletonPulseEased(t *testing.T) {
	approx := func(got, want float64) bool {
		d := got - want
		return d < 1e-9 && d > -1e-9
	}
	if got := skeletonPulse(0); !approx(got, 0.55) {
		t.Errorf("skeletonPulse(0) = %v，应为 0.55", got)
	}
	if got := skeletonPulse(0.5); !approx(got, 1.0) {
		t.Errorf("skeletonPulse(0.5) = %v，应为 1.0", got)
	}
	if got := skeletonPulse(1); !approx(got, 0.55) {
		t.Errorf("skeletonPulse(1) = %v，应为 0.55", got)
	}
	// 中点仍是 0.775（EaseInOut(0.5) = 0.5），但 1/4 三角处必须低于线性值，
	// 否则说明根本没走 ease-in-out（线性三角波处 = 0.6625）。
	if got := skeletonPulse(0.125); !(got < 0.62) {
		t.Errorf("skeletonPulse(0.125) = %v，应明显低于线性三角波的 0.6625（缓入）", got)
	}
	// 对称性：上半程与下半程镜像。
	for _, ph := range []float64{0.1, 0.25, 0.4, 0.125} {
		if a, b := skeletonPulse(ph), skeletonPulse(1-ph); !approx(a, b) {
			t.Errorf("脉冲应对称：skeletonPulse(%v)=%v != skeletonPulse(%v)=%v", ph, a, 1-ph, b)
		}
	}
}

// rowProfile 把一条横向带 [x0,x1) 压成每个 y 的平均亮度（columnProfile 的转置），
// 用来数骨架屏有几排占位卡。
func rowProfile(img *image.RGBA, x0, x1, y0, y1 int) []int {
	out := make([]int, y1-y0)
	for y := y0; y < y1; y++ {
		sum := 0
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			sum += (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
		}
		out[y-y0] = sum / (x1 - x0)
	}
	return out
}

// darkRuns 数一段亮度剖面里有几条「暗带」（连续低于自适应中点的段，宽于 minH）。
// 骨架卡片每张是「封面 + 两行文本条」三条暗带，条间 8px、排间 24px 都是浅色，
// 所以暗带数 = 3 × 排数。
func darkRuns(profile []int, minH int) int {
	lo, hi := 255, 0
	for _, v := range profile {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	th := (lo + hi) / 2
	runs, h := 0, 0
	for _, v := range profile {
		if v < th {
			h++
			continue
		}
		if h >= minH {
			runs++
		}
		h = 0
	}
	if h >= minH {
		runs++
	}
	return runs
}

// TestSkeletonGridRows 是无头像素实测：宽窗（三列）骨架屏只有 6 张（2 排 × 3 条
// 暗带 = 6 条），窄窗（两列）是 3 排（9 条）。此前写死 3 排时宽窗会量到 9 条。
func TestSkeletonGridRows(t *testing.T) {
	for _, c := range []struct {
		width  int
		want   int // 暗带数 = 3 × 排数
		reason string
	}{
		{800, 6, "宽窗三列：6 张 = 2 排"},
		{560, 9, "窄窗两列：6 张 = 3 排"},
	} {
		tst := skeletonProbe(c.width, 700)
		img := tst.Image()
		// x 带选在第一张卡封面的左半边（同时落在两行文本条内），避开网格间隙。
		runs := darkRuns(rowProfile(img, 40, 130, 0, 700), 6)
		if runs != c.want {
			t.Errorf("%dpx %s：实测 %d 条暗带，应为 %d", c.width, c.reason, runs, c.want)
		}
	}
}
