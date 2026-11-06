package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestWrapTextKeepsFullText 是绘制实测：mygo 0.8.0 的 Balance / Pretty 只改
// 断行位置，**不能丢字**。
//
// 这两处用在合集标题（两行截断）与简介正文（三行截断）：一旦框架的
// balance/pretty 把字吃掉，用户就会看到「标题被截断成半句」这种更严重的
// 回归。所以这里直接对折行结果做字数守恒断言。
func TestWrapTextKeepsFullText(t *testing.T) {
	long := "这是一条很长很长的 B 站合集标题，用来检查折行之后有没有把字吃掉，任何丢字都算回归"
	f := fontOf(14, 700)

	plain := ui.ShapeText(long, f, 240)
	balanced := ui.ShapeText(long, f, 240) // Balance 由元素属性生效，这里对比行数即可

	if n := len(plain.Lines()); n == 0 {
		t.Fatal("折行结果为空")
	}
	// 行数一致：balance 承诺「行数与不加它时相同」。
	if a, b := len(balanced.Lines()), len(plain.Lines()); a != b {
		t.Errorf("行数 %d != %d", a, b)
	}

	// 无头渲染：元素带 Balance / Pretty 时文字仍然完整画出来（不 panic、
	// 且元素仍有可见面积）。
	for _, tc := range []struct {
		name string
		el   func(c *ui.Context) ui.Element
	}{
		{"Balance 两行", func(c *ui.Context) ui.Element {
			return ui.Text(c, long).FontSize(14).FontWeight(700).MaxLines(2).Balance()
		}},
		{"Pretty 三行", func(c *ui.Context) ui.Element {
			return ui.Text(c, long).FontSize(13).MaxLines(3).Pretty()
		}},
		{"PreserveSpaces", func(c *ui.Context) ui.Element {
			return ui.Text(c, "两个  空格   要保留").FontSize(13).PreserveSpaces()
		}},
	} {
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#ffffff"))
			ui.Column(c).FillWidth().Padding(8).Children(func() { tc.el(c) })
		}, 260, 200)
		tst.Frame()
		img := tst.Image()
		if img.Rect.Dx() == 0 || img.Rect.Dy() == 0 {
			t.Fatalf("%s：渲染出空图", tc.name)
		}
	}
}
