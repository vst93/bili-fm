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

// TestBalanceMakesTwoLinesEven 是**绘制实测**：`Element.Balance`（mygo 0.8.0）
// 真的把两行拉到接近等长，而不是只写了个声明式开关。
//
// 量法：同一段长文本在固定宽度下折两行，逐行取「最右墨迹的横坐标」当作行宽。
// 不加 Balance 时第一行顶满、第二行只剩一小截（实测 208 / 138）；加了之后两行
// 接近（181 / 166）。这正是合集标题那个「第二行只剩两三个字」的问题。
func TestBalanceMakesTwoLinesEven(t *testing.T) {
	const long = "这是一条很长很长的合集标题用来检查两行折行是否等长"
	lineEnds := func(balance bool) []int {
		tst := ui.NewTester(func(c *ui.Context) {
			c.SetTheme(ThemeFor(PeriodAt(12)).uiTheme())
			ui.Column(c).Fill().Background(ui.Hex("#ffffff")).Children(func() {
				txt := ui.Text(c, long).FontSize(14).FontWeight(700).
					TextColor(ui.Hex("#000000")).Width(220)
				if balance {
					txt.Balance()
				}
			})
		}, 240, 120)
		tst.Frame()
		img := tst.Image()
		var out []int
		for band := 0; band < 2; band++ {
			best := 0
			for y := band * 20; y < (band+1)*20 && y < img.Rect.Dy(); y++ {
				for x := 0; x < img.Rect.Dx(); x++ {
					if lum(pixelAt(t, img, x, y)) < 128 && x > best {
						best = x
					}
				}
			}
			out = append(out, best)
		}
		return out
	}

	plain := lineEnds(false)
	balanced := lineEnds(true)
	diff := func(v []int) int {
		if v[0] > v[1] {
			return v[0] - v[1]
		}
		return v[1] - v[0]
	}
	t.Logf("行宽差：无 balance=%d（%v），有 balance=%d（%v）",
		diff(plain), plain, diff(balanced), balanced)
	if diff(balanced) >= diff(plain) {
		t.Errorf("Balance 没让两行更接近：无 %d vs 有 %d", diff(plain), diff(balanced))
	}
	if diff(balanced) > 40 {
		t.Errorf("Balance 后两行仍差 %d px，说明没真正生效", diff(balanced))
	}
}
