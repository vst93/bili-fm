package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 桌面字幕面板**不许有多余边框**（用户 2026-11-11：「桌面字幕这一块不要有
// 多余的边框（现在 Linux 下就有边框）」，并明确要求三平台统一）。
//
// 历史：曾经在鼠标进入时画一圈选中蓝边（#7dd3fc），本意是「背景透明度调到 0
// 时也能看出字幕窗的位置和可设置区域」。实际效果是 Linux 上被合成器衬成一圈
// 灰边、看起来像「未选中的默认边框」；而「窗口在哪儿」的提示现在已经由
// 「指针进字幕区就浮出设置键」承担，那圈线是多余的。
//
// 判据：把背景设成全透明（bgOpacity=0）、文字设成透明，于是渲染结果应当**整片
// 透明** —— 任何一条边上有像素不透明，就说明有东西在画边框 / 投影 / 内高光。
func TestLyricsPanelHasNoBorder(t *testing.T) {
	for _, hover := range []bool{false, true} {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.LyricCurrent, a.LyricNext = "第一句", "第二句"
		a.LyricsBgOpacity = 0
		a.LyricsTextOpacity = 0
		h := LyricsDisplayHeight(a.LyricsFontSize)
		tt := ui.NewTester(func(c *ui.Context) { a.LyricsShell(c) }, 560, h)
		if hover {
			tt.Move(280, float32(h)/2)
		}
		tt.Frame()
		tt.Advance(200 * time.Millisecond)

		img := tt.Image()
		b := img.Bounds()
		// 四条边各扫一条，任何不透明像素都算「有边框」。
		check := func(name string, x, y int) {
			o := img.PixOffset(x, y)
			if a := img.Pix[o+3]; a != 0 {
				t.Errorf("hover=%v 时 %s 处有 alpha=%d 的像素 —— 面板不该画边框/投影",
					hover, name, a)
			}
		}
		for x := 0; x < b.Dx(); x += 7 {
			check("上边", x, 0)
			check("下边", x, b.Dy()-1)
		}
		for y := 0; y < b.Dy(); y += 7 {
			check("左边", 0, y)
			check("右边", b.Dx()-1, y)
		}
	}
}
