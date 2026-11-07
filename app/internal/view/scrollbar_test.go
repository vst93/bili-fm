package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestScrollbarThumbAlpha 钉住滚动条拇指的不透明度取自原版**生效**的那条声明。
//
// 原版 globals.css 写了两条：
//
//   - { scrollbar-color: rgba(100,116,139,.35) transparent }      // Firefox 专用
//     *::-webkit-scrollbar-thumb { background: rgba(100,116,139,.3) } // WebView 实际生效
//
// Tauri 两端都是 WebView（Windows WebView2/Chromium、Linux WebKitGTK），渲染的是
// .3 那条。此前原生版抄了 .35，深 5% alpha。
func TestScrollbarThumbAlpha(t *testing.T) {
	th := Theme{}.uiTheme()
	if got := th.ScrollbarWidth; got != 6 {
		t.Fatalf("滚动条宽度 = %v，原版 ::-webkit-scrollbar { width: 6px }", got)
	}
	// ui.Color 的 A 是 8 位直通 alpha。
	got := float64(th.Scrollbar.A) / 255
	if math.Abs(got-0.30) > 0.01 {
		t.Fatalf("滚动条拇指 alpha = %.3f，原版 ::-webkit-scrollbar-thumb 是 .30", got)
	}
	// 反证：Firefox 的 scrollbar-color（.35）对不上。
	if math.Abs(got-0.35) <= 0.01 {
		t.Fatalf("滚动条拇指仍是 Firefox 那条的 .35，WebView 实际渲染是 .30")
	}
}

// TestKeepScrollbarsShowsWithoutHover 是**绘制实测**：`KeepScrollbars(true)` 的
// 滚动容器在内容溢出时，指针不悬停也画出滚动条拇指；不带这个开关的容器不画。
//
// 为什么要像素实测：`KeepScrollbars(true)` 是个声明式开关，写错 / 写漏在编译、
// vet、纯函数断言里全都看不出来 —— 只有真渲染一帧、数一数右缘有没有拇指颜色的
// 像素，才能证明它确实生效（同 mygo 0.8.0 那批「代码在、效果不对」的坑）。
func TestKeepScrollbarsShowsWithoutHover(t *testing.T) {
	// thumbPixels 返回「明显比行底色暗」的右缘像素数：拇指是
	// rgba(100,116,139,.30) 压在行底色上，比 #e2e8f0 的 231 暗一大截。
	thumbPixels := func(keep bool) int {
		tst := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(ThemeFor(PeriodAt(12)).uiTheme())
			// 背景必须由**父容器**铺：直接并列一个 Fill() 的 Box 会把列表挤到
			// 可视区之外（列表的高度变成 0，什么都画不出来）。
			ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Children(func() {
				st := &ui.ListState{}
				ui.List(c, st, 40, func(i int) {
					ui.Box(c).FillWidth().Height(30).Background(ui.Hex("#e2e8f0"))
				}).FillWidth().Grow(1).KeepScrollbars(keep)
			})
		}, 120, 100)
		tst.Frame()
		img := tst.Image()
		n := 0
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := img.Rect.Dx() - 10; x < img.Rect.Dx(); x++ {
				if lum(pixelAt(t, img, x, y)) < 215 { // 比行底色 231 明显暗 = 拇指
					n++
				}
			}
		}
		return n
	}

	withBar := thumbPixels(true)
	without := thumbPixels(false)
	t.Logf("右缘暗像素数：KeepScrollbars(true)=%d，false=%d", withBar, without)
	if withBar == 0 {
		t.Fatal("KeepScrollbars(true) 时右缘没有任何滚动条像素（开关没生效）")
	}
	if withBar <= without {
		t.Errorf("KeepScrollbars(true) 的滚动条像素 (%d) 应多于关掉时 (%d)", withBar, without)
	}
}
