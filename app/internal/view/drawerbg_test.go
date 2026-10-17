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

// TestDrawerPanelNotTooTransparent 是无头渲染实测：抽屉面板区域最终合成出的
// 像素应当接近不透明（底下的窗口背景几乎不可见），否则文字压在半透明的
// 背景上，阅读性差——这正是用户反馈的「抽屉背景太透」。
//
// 量法：面板底色是「白色 @ α」叠在「遮罩 + 窗口背景」之上。同一位置在
// 抽屉关闭时是 P0，打开后是 P1。先扣掉遮罩（rgba(15,23,42,.28)）得到面板
// 之下的底色 B，再由 P1 = B·(1-α) + 255·α 反解 α。α 越大越不透明。
func TestDrawerPanelNotTooTransparent(t *testing.T) {
	const (
		px, py = 24, 520 // 面板左下角，远离居中的空态文字
	)
	// 关闭抽屉，取该点的窗口背景（遮罩之下）。
	closed, tstC := drawerProbe(t, "feed")
	closed.Drawer = ""
	tstC.Frame()
	img0 := tstC.Image()
	r0, g0, b0, _ := rgbaAt(t, img0, px, py)

	// 打开抽屉，取同一点。
	_, tstO := drawerProbe(t, "feed")
	tstO.Frame()
	img1 := tstO.Image()
	r1, g1, b1, _ := rgbaAt(t, img1, px, py)

	// 遮罩 rgba(15,23,42,0.28) 先压一层。
	const oa = 0.28
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

	// 本轮把面板从「几乎不透明白板」往回收：既不能回到 0.22 那种太透
	// （影响阅读），也不该再是 0.9 的白板。可读的下限 0.6，上限 0.8。
	if a < 0.6 {
		t.Fatalf("抽屉面板不透明度 α=%.3f < 0.6：背景透出过多，影响阅读", a)
	}
	if a > 0.8 {
		t.Fatalf("抽屉面板不透明度 α=%.3f > 0.8：面板偏白偏实，与其余玻璃表面不统一", a)
	}
}

// TestDrawerPanelFillIsOpaqueToken 钉住面板填充令牌本身：不再是 0.22 的
// 半透明 --liquid-bg，也不是 0.9 的白板，而是折中的 0.70（更暗更通）。
func TestDrawerPanelFillIsOpaqueToken(t *testing.T) {
	fill := drawerPanelFill()
	alpha := float64(fill.A) / 255
	if alpha < 0.6 || alpha > 0.8 {
		t.Fatalf("drawerPanelFill alpha=%.3f，应落在 [0.6,0.8]", alpha)
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
