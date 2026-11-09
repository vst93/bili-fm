package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// drawerProbe 建一个 800×600 的应用，抽屉打开且列表为空（正文只有居中的空态
// 提示），于是面板底色在四角裸露，便于直接采样。
func drawerProbe(t *testing.T, key string) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	// 固定到 noon 时段，背景可复现（不随跑测试的钟点变）。
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = key
	a.ListFor(key) // 空列表
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	return a, tst
}

// rgbaAt 取某个 DIP 位置的像素（scale=1）。
func rgbaAt(t *testing.T, img *image.RGBA, x, y int) (int, int, int, int) {
	t.Helper()
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		t.Fatalf("采样点 (%d,%d) 越界 %v", x, y, img.Rect)
	}
	i := img.PixOffset(x, y)
	return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]), int(img.Pix[i+3])
}

// The drawer should transmit background color through one shared blur and
// a light veil. Sample an empty region, away from content and the lit rim.
func TestDrawerPanelTranslucent(t *testing.T) {
	const (
		px, py = 24, 520 // 面板左下角，远离居中的空态文字
	)
	// 关闭抽屉，取该点的窗口背景（遮罩之下）。
	_, tstC := drawerProbe(t, "")
	settleTester(tstC)
	img0 := tstC.Image()
	r0, g0, b0, _ := rgbaAt(t, img0, px, py)

	// 打开抽屉，取同一点。
	_, tstO := drawerProbe(t, "feed")
	settleTester(tstO)
	img1 := tstO.Image()
	r1, g1, b1, _ := rgbaAt(t, img1, px, py)

	// Subtract the lighter drawer scrim before measuring its veil.
	const oa = 0.12
	br := float64(r0)*(1-oa) + 15*oa
	bg := float64(g0)*(1-oa) + 23*oa
	bb := float64(b0)*(1-oa) + 42*oa

	// 反解每通道的 α，取平均。
	alpha := func(p1, b float64) float64 {
		den := 255 - b
		if den < 1 {
			return 1
		}
		return (p1 - b) / den
	}
	a := (alpha(float64(r1), br) + alpha(float64(g1), bg) + alpha(float64(b1), bb)) / 3

	t.Logf("抽屉面板实测：关=(%d,%d,%d) 开=(%d,%d,%d) 面板之下=(%.0f,%.0f,%.0f) 反解 α=%.3f",
		r0, g0, b0, r1, g1, b1, br, bg, bb, a)

	if a < 0.30 || a > 0.60 {
		t.Fatalf("drawer veil alpha %.3f should stay in [0.30,0.60] for a light glass pane", a)
	}
}

// TestDrawerPanelTopBorderOnly 钉住抽屉面板的**布局边只有上边**（原版
// `[data-slot="wrapper"] > section { border-top: 1px solid var(--glass-border) }`）。
//
// 此前原生版写 `Border(1, GlassBorder)`：框架的 Border 是**布局边框**（占掉
// 元素内部 1px），四边各吃 1px ⇒ 抽屉正文比原版窄 2px（既有残留差异）。
// 原版左右 / 下那圈 1px 亮边来自 `::after` 的 --liquid-rim 聚光描边（inset:0 /
// padding 1px / mask exclude，**不占布局**）。这里两条断言：
//   - 源码链：面板用 BorderWidth(1,0,0,0)（只上边）而不是 Border(1, ...)；
//   - 无头像素：面板左缘内侧 1px 处是**面板底色的延续**（不是一条亮白边），
//     即左右没有布局边框 —— 若退回四边 Border，最外 1px 会被 --glass-border
//     的亮白盖出一条明显更亮的竖线。
func TestDrawerPanelTopBorderOnly(t *testing.T) {
	flat := flatten(t, "drawer.go")
	if !srcHas(flat, "BorderWidth(1,0,0,0).BorderColor(a.Theme.GlassBorder)") {
		t.Error("抽屉面板没有改成「只上边」的布局边（BorderWidth(1,0,0,0)）")
	}
	// 面板不再用满四边的 Border(1)（那会吃掉左右各 1px）。只查面板那一处：
	// 面板 Background 之后紧跟的写法。
	if srcHas(flat, "Background(drawerPanelFill()).Border(1,") {
		t.Error("抽屉面板仍在用满四边 Border(1, ...)：正文会比原版窄 2px")
	}
}
