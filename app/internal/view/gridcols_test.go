package view

import (
	"fmt"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestGridColsFor 钉住卡片网格的 sm 断点：原版所有列表抽屉都是
// `grid grid-cols-2 sm:grid-cols-3`（searchList / collectList / recommendList /
// upVideoList / feedList / historyList / seriesList / pageList），骨架屏
// .list-skeleton-grid 也是同一对。Tailwind 的 `sm` 是**视口** min-width 640。
func TestGridColsFor(t *testing.T) {
	cases := []struct {
		width float32
		want  int
	}{
		{400, 2}, // 原生窗口下限 400
		{560, 2},
		{639, 2}, // 断点前一格
		{640, 3}, // 断点本身进三列（min-width）
		{641, 3},
		{800, 3}, // 原版主窗尺寸
		{1400, 3},
	}
	for _, c := range cases {
		if got := gridColsFor(c.width); got != c.want {
			t.Errorf("gridColsFor(%v) = %d，应为 %d", c.width, got, c.want)
		}
	}
	// 断点常量本身是 640（写死在这里，免得哪天被顺手改掉）。
	if cardGridBreakpoint != 640 {
		t.Errorf("cardGridBreakpoint = %v，应为 640（Tailwind sm）", cardGridBreakpoint)
	}
}

// gridProbe 起一个开了「动态」抽屉的 app：卡片标题唯一，便于用 Find 取矩形。
func gridProbe(t *testing.T, width, height, n int, loading bool) *ui.Tester {
	t.Helper()
	cards := make([]Card, n)
	for i := range cards {
		cards[i] = Card{
			Bvid:  fmt.Sprintf("BV%03d", i),
			Title: fmt.Sprintf("标题%d", i),
		}
	}
	a := NewApp(func() {})
	a.Images = newRecordingCache()
	l := a.ListFor("feed")
	l.Cards = cards
	l.Loading = loading
	a.Drawer = "feed"
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, width, height)
	// 头两帧让抽屉入场动画停稳、行高量出来（相对位置不受动画影响，
	// 但量出来的行高决定后面几排落在哪）。
	tst.Frame()
	tst.Frame()
	return tst
}

// sameRow 报告两个矩形是否在同一排（Y 相差不超过 2px）。
func sameRow(a, b ui.Rect) bool {
	d := a.Y - b.Y
	return d > -2 && d < 2
}

// TestCardGridResponsiveColumns 是无头渲染实测：≥640 的窗口每排 3 张卡，
// 窄窗（<640）每排 2 张 —— 原版的 grid-cols-2 sm:grid-cols-3。
//
// 之前原生版把列数写死成 3，窄窗（原生窗口可以缩到 400）下每张卡只有
// (400-48-16)/3-12 ≈ 100px 宽，卡片的 meta 行按容器宽把所有字段都隐藏了。
func TestCardGridResponsiveColumns(t *testing.T) {
	t.Run("宽窗 800 三列", func(t *testing.T) {
		tst := gridProbe(t, 800, 720, 9, false)
		r0, ok0 := tst.Find("标题0")
		r1, ok1 := tst.Find("标题1")
		r2, ok2 := tst.Find("标题2")
		if !ok0 || !ok1 || !ok2 {
			t.Fatalf("首屏应构建前三张卡：%v", tst.Texts())
		}
		if !sameRow(r0, r1) || !sameRow(r0, r2) {
			t.Errorf("800px 下前三张应同排：y = %v / %v / %v", r0.Y, r1.Y, r2.Y)
		}
		if !(r1.X > r0.X && r2.X > r1.X) {
			t.Errorf("同排的卡片应从左到右排开：x = %v / %v / %v", r0.X, r1.X, r2.X)
		}
		// 第四张落到第二排。
		r3, ok3 := tst.Find("标题3")
		if !ok3 {
			t.Fatalf("首屏应构建第二排：%v", tst.Texts())
		}
		if !(r3.Y > r0.Y+10) {
			t.Errorf("800px 下第 4 张应在第二排：y = %v（第一排 %v）", r3.Y, r0.Y)
		}
	})

	t.Run("窄窗 560 两列", func(t *testing.T) {
		tst := gridProbe(t, 560, 720, 9, false)
		r0, ok0 := tst.Find("标题0")
		r1, ok1 := tst.Find("标题1")
		if !ok0 || !ok1 {
			t.Fatalf("首屏应构建前两张卡：%v", tst.Texts())
		}
		if !sameRow(r0, r1) {
			t.Errorf("560px 下前两张应同排：y = %v / %v", r0.Y, r1.Y)
		}
		if !(r1.X > r0.X) {
			t.Errorf("560px 下第二张应在第一张右边：x = %v / %v", r0.X, r1.X)
		}
		// 关键差异：第三张换排、回到第一列（写死 3 列时它会在第一排最右）。
		r2, ok2 := tst.Find("标题2")
		if !ok2 {
			t.Fatalf("首屏应构建第三张卡（第二排首张）：%v", tst.Texts())
		}
		if sameRow(r0, r2) {
			t.Errorf("560px 下第 3 张不该和第一排同排（每排 2 张）：y = %v / %v", r0.Y, r2.Y)
		}
		if d := r2.X - r0.X; d > 2 || d < -2 {
			t.Errorf("560px 下第 3 张应回到第一列：x = %v（第 1 张 %v）", r2.X, r0.X)
		}
		// 卡片不该越出窗口右缘。
		if r1.X+r1.W > 560 {
			t.Errorf("第二张卡越出窗口：right = %v", r1.X+r1.W)
		}
	})

	// 反证：把列数写死回 3 时，560px 下第 3 张会落回第一排（测试变红）。
	t.Run("反证", func(t *testing.T) {
		if got := gridColsFor(800); got != 3 {
			t.Fatalf("前提不成立：gridColsFor(800) = %d", got)
		}
	})
}

// columnProfile 把一条纵向带 [y0,y1) 压成每个 x 的平均亮度。骨架屏的脉冲
// 动画（1.4s 周期）会让绝对亮度随时变，所以不能拿固定阈值判封面；压成列剖面
// 后各层的相对关系（面板 > 卡片底 > 卡间隙 > 封面）不变。
func columnProfile(img *image.RGBA, y0, y1, x0, x1 int) []int {
	out := make([]int, x1-x0)
	for x := x0; x < x1; x++ {
		sum := 0
		for y := y0; y < y1; y++ {
			i := img.PixOffset(x, y)
			sum += (int(img.Pix[i])*299 + int(img.Pix[i+1])*587 + int(img.Pix[i+2])*114) / 1000
		}
		out[x-x0] = sum / (y1 - y0)
	}
	return out
}

// coverRuns 数列剖面里有几块「封面」：以剖面自身的（最低+最高）/2 为中点阈值
// 划分，只算宽于 minW 的连续段——卡片左右各 8px 的 padding 与 8px 的网格间距
// （总共 24px）会把相邻封面隔开，且它们比卡片底暗、比封面亮。
//
// 用自适应中点而不是固定亮度：骨架脉冲会让整层一起变亮/变暗，固定阈值会在
// 某些相位错数（实测 560px 下会数出 4 或 6 块）。
func coverRuns(profile []int, minW int) int {
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
	runs, runW := 0, 0
	for _, v := range profile {
		if v < th {
			runW++
			continue
		}
		if runW >= minW {
			runs++
		}
		runW = 0
	}
	if runW >= minW {
		runs++
	}
	return runs
}

// skeletonProbe 只渲染骨架屏本身（不搭整套 Shell / 抽屉）：骨架的列数只看
// 窗口宽度，单独起一个最小视图能把这条规则量得快而准（~25ms 一个窗口）。
func skeletonProbe(width, height int) *ui.Tester {
	a := NewApp(func() {})
	return ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Fill().Padding(0, 24).Children(func() { a.skeletonGrid(c) })
	}, width, height)
}

// skeletonCols 无头量一次骨架屏有几列（扫第一排封面的那条带）。
func skeletonCols(width int) int {
	tst := skeletonProbe(width, 260)
	return coverRuns(columnProfile(tst.Image(), 10, 120, 12, width-12), 60)
}

// TestSkeletonGridResponsiveColumns 是无头渲染实测：加载中的骨架屏跟着同一个
// sm 断点走（原版 .list-skeleton-grid 基线 repeat(2, 1fr)，globals.css 里
// `@media (min-width: 640px)` 才变三列）。
func TestSkeletonGridResponsiveColumns(t *testing.T) {
	for _, c := range []struct {
		width int
		want  int
	}{
		{400, 2}, // 原生窗口下限
		{560, 2},
		{639, 2}, // 断点前一格
		{640, 3}, // 断点本身进三列（min-width）
		{800, 3},
	} {
		if got := skeletonCols(c.width); got != c.want {
			t.Errorf("%dpx 骨架屏实测 %d 列，应为 %d", c.width, got, c.want)
		}
	}
}
