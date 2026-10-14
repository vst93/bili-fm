package view

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestNumericTextTabularFigures 钉住 numericText 真的打开了 tabular-nums：
// 用数字本来不等宽的界面字体（GNOME 的 Adwaita Sans），开 tnum 后 "1111" 与
// "8888" 的宽度应当相等。
//
// 环境没装这个字体、或字体没有 tnum 特性时跳过（测不出差异，不算回归）。
// 反证：把 numericText 里的 FontFeatures 去掉，两条宽度会重新不等。
func TestNumericTextTabularFigures(t *testing.T) {
	const fam = "Adwaita Sans"
	var plain1, plain8, feat1, feat8 float32
	tt := ui.NewTester(func(c *ui.Context) {
		plain1, _ = c.MeasureText(1000, ui.Span{Text: "1111", Size: 20, Font: fam})
		plain8, _ = c.MeasureText(1000, ui.Span{Text: "8888", Size: 20, Font: fam})
		feat1, _ = c.MeasureText(1000, ui.Span{Text: "1111", Size: 20, Font: fam, Features: "tnum"})
		feat8, _ = c.MeasureText(1000, ui.Span{Text: "8888", Size: 20, Font: fam, Features: "tnum"})
		ui.Column(c).Gap(4).Children(func() {
			row := func(label, s string, numeric bool) {
				ui.Row(c).Shrink(0).Children(func() {
					var e ui.Element
					if numeric {
						e = numericText(c, s)
					} else {
						e = ui.Text(c, s)
					}
					e.Font(fam).FontSize(20).SingleLine().Label(label)
				})
			}
			row("plain-1", "1111", false)
			row("plain-8", "8888", false)
			row("tab-1", "1111", true)
			row("tab-8", "8888", true)
		})
	}, 300, 240)
	tt.Frame()
	if plain1 == plain8 {
		t.Skipf("字体 %q 的数字本来就等宽，测不出 tnum", fam)
	}
	if feat1 != feat8 {
		t.Skipf("字体 %q 没有 tnum 特性", fam)
	}
	tab1, ok1 := tt.Find("tab-1")
	tab8, ok8 := tt.Find("tab-8")
	if !ok1 || !ok8 {
		t.Fatal("渲染帧里找不到 tab-1 / tab-8")
	}
	if tab1.W != tab8.W {
		t.Errorf("numericText 没打开 tabular-nums：1111 宽 %v，8888 宽 %v", tab1.W, tab8.W)
	}
}

// TestNumericTextCallSites 是本轮改动的接线测试：换桩 numericText，跑真实
// 的主窗与弹幕抽屉，确认那些原版写了 tabular-nums 的数值文本确实都走
// numericText（播放时间、赞/币/藏计数、倍速键、弹幕时间与 ×N）。
func TestNumericTextCallSites(t *testing.T) {
	orig := numericText
	defer func() { numericText = orig }()
	var got []string
	numericText = func(c *ui.Context, s string) ui.Element {
		got = append(got, s)
		return ui.Text(c, s).FontFeatures(tabularNums...)
	}

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	focusedInfo(a)
	a.Info.Like = 1234
	a.Info.Coin = 5678
	a.Info.Favorite = 9012
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tt.Frame()
	for _, want := range []string{"0:00", "1234", "5678", "9012", "1.0x"} {
		if !slices.Contains(got, want) {
			t.Errorf("主窗没有用 numericText 画 %q（收集到 %v）", want, got)
		}
	}

	// 弹幕抽屉：时间标签 + 重复数 ×N。
	got = got[:0]
	a.Drawer = DrawerDanmaku
	a.Danmaku = []Danmaku{{Time: 65, Text: "弹幕"}, {Time: 65, Text: "弹幕"}}
	tt.Frame()
	for _, want := range []string{"01:05", "×2"} {
		if !slices.Contains(got, want) {
			t.Errorf("弹幕抽屉没有用 numericText 画 %q（收集到 %v）", want, got)
		}
	}
}
