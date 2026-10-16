package view

import (
	"bytes"
	"image"
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
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
// CSS 的 ease-in-out 是 cubic-bezier(.42,0,.58,1)，与 mygo 的 ui.EaseInOut
// （标准三次 in-out）在 1/8 周期处差约 0.03 不透明度。
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
	// 1/8 周期处三角波进度是 0.25，CSS cubic-bezier(.42,0,.58,1) 给 ≈0.1294
	// → 不透明度 0.55 + 0.45×0.1294 ≈ 0.6082（线性三角波此处是 0.6625，
	// ui.EaseInOut 是 0.578）。这条同时钉住「缓入」与「用的是 CSS 曲线而非
	// 框架的三次 in-out」。
	got := skeletonPulse(0.125)
	if got < 0.600 || got > 0.616 {
		t.Errorf("skeletonPulse(0.125) = %v，应 ≈0.6082（CSS cubic-bezier(.42,0,.58,1)）", got)
	}
	if mygo := 0.55 + 0.45*float64(ui.EaseInOut(0.25)); got <= mygo {
		t.Errorf("skeletonPulse(0.125) = %v 不应 ≤ ui.EaseInOut 的 %v（说明没用 CSS 曲线）", got, mygo)
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

// TestSkeletonTriggerScope 钉住骨架屏的**触发范围**：原版 ListSkeleton 只被
// searchList / collectList / recommendList 三处 import（main:src/components/
// listSkeleton.tsx 的引用），其余卡片网格抽屉（动态 / 历史 / UP 空间 / 合集）
// 首载渲染的是空网格。此前原生版对所有卡片网格抽屉都铺骨架屏。
func TestSkeletonTriggerScope(t *testing.T) {
	want := map[string]bool{
		DrawerSearch:   true,  // searchList
		"favorite":     true,  // collectList（收藏）
		"popular":      true,  // recommendList（热门与推荐）
		"feed":         false, // feedList
		"history":      false, // historyList
		DrawerUp:       false, // upVideoList
		DrawerSeries:   false, // seriesList
		DrawerParts:    false, // pageList
		DrawerPlaylist: false,
		DrawerDanmaku:  false,
		"":             false,
	}
	for key, w := range want {
		if got := skeletonFor(key); got != w {
			t.Errorf("skeletonFor(%q) = %v，应为 %v", key, got, w)
		}
	}
	// 反证：集合恰好是那三个调用点，不多不少。
	if len(skeletonDrawers) != 3 {
		t.Errorf("skeletonDrawers 有 %d 项，原版 ListSkeleton 只有 3 个调用点", len(skeletonDrawers))
	}
}

// TestSkeletonTriggerWiredInSource 在源码里确认 listBody 真的用 skeletonFor 门控
// 骨架屏（写了纯函数没接上等于没写），并确认骨架容器标了装饰态（原版
// .list-skeleton-grid 的 aria-hidden="true"，mygo 用等价的 RoleNone）。
func TestSkeletonTriggerWiredInSource(t *testing.T) {
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败：%v", err)
	}
	flat := bytes.Join(bytes.Fields(src), []byte(" "))
	for _, want := range [][]byte{
		[]byte("if skeletonFor(a.Drawer) { a.skeletonGrid(c) }"),
		[]byte("ui.Column(c).Gap(8).Role(ui.RoleNone)"),
	} {
		if !bytes.Contains(flat, want) {
			t.Errorf("drawer.go 里找不到 %q", want)
		}
	}
}

// listBodyProbe 只渲染 listBody 本身（白底，不搭抽屉/遮罩）：加载中、无卡片。
// 用来量某个抽屉首载是否铺了骨架屏（骨架有 6 张占位卡，会铺出大片非白像素）。
func listBodyProbe(drawer string) *ui.Tester {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	l := a.ListFor(drawer)
	l.Loading, l.Cards = true, nil
	a.Drawer = drawer
	return ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Fill().Padding(0, 24).Children(func() { a.listBody(c) })
	}, 900, 700)
}

// TestSkeletonRendersOnlyForOriginalDrawers 是无头像素实测：只有原版 ListSkeleton
// 的三个调用点（搜索 / 收藏 / 热门与推荐）首载铺骨架屏；其余卡片网格抽屉（动态 /
// 历史）首载是空网格（原版如此），不铺骨架屏。
//
// 反证：把 listBody 里的 skeletonFor 门控去掉（对所有抽屉都铺）后，feed/history
// 的 nonWhite 会从 0 变成≈25 万，这条变红。
func TestSkeletonRendersOnlyForOriginalDrawers(t *testing.T) {
	nonWhite := func(drawer string) int {
		img := listBodyProbe(drawer).Image()
		n := 0
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				i := img.PixOffset(x, y)
				if img.Pix[i] < 250 || img.Pix[i+1] < 250 || img.Pix[i+2] < 250 {
					n++
				}
			}
		}
		return n
	}
	for _, drawer := range []string{DrawerSearch, "favorite", "popular"} {
		if got := nonWhite(drawer); got < 1000 {
			t.Errorf("%s 首载应铺骨架屏，实测非白像素只有 %d", drawer, got)
		}
	}
	for _, drawer := range []string{"feed", "history"} {
		if got := nonWhite(drawer); got != 0 {
			t.Errorf("%s 首载不该铺骨架屏（原版是空网格），实测非白像素 %d", drawer, got)
		}
	}
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
