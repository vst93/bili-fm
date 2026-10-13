package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestCoverToggleOrder 钉住封面右下角三个小开关的**左右顺序**。
//
// 原版 videoCover.tsx 里三个键都是 absolute、靠各自的 `right` 贴边：
//
//	.cover-mode-toggle       right: 8px   ← 最右（模式切换 / Refresh）
//	.cover-background-toggle right: 42px  ← 中间（封面背景 / Halo）
//	.cover-premium-toggle    right: 76px  ← 最左（高级质感 / Sparkles）
//
// 即从左到右是 高级质感 → 封面背景 → 模式切换。原生版用单个 Row 表达这段几何，
// 子元素从左到右排布，若按「模式 / 背景 / 质感」的书写顺序摆就会**左右镜像**
// （最右变成高级质感）。这个测试在盘片与方块两种模式下都验证顺序与间距。
func TestCoverToggleOrder(t *testing.T) {
	label := func(mode string, ambient, premium bool) (string, string, string) {
		modeLabel := "切换为封面模式"
		if mode == "square" {
			modeLabel = "切换为碟片模式"
		}
		amb := "开启封面背景"
		if ambient {
			amb = "关闭封面背景"
		}
		prem := "开启高级质感"
		if premium {
			prem = "关闭高级质感（更省电）"
		}
		return prem, amb, modeLabel
	}

	cases := []struct {
		name             string
		mode             string
		ambient, premium bool
	}{
		{"碟片模式 默认", "disc", true, true},
		{"碟片模式 两个都关", "disc", false, false},
		{"方块模式", "square", true, true},
		{"方块模式 背景关", "square", false, true},
	}

	// 一个窗口就够了：每轮只改 App 状态再重画，比四个 Tester 便宜得多
	// （Tester 的入场/布局都走真实渲染，全包并行时开太多会把墙钟驱动的
	// 测试拖慢）。
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(cc *ui.Context) { a.Shell(cc) }, 1000, 700)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a.CoverMode = c.mode
			a.Ambient = c.ambient
			a.Premium = c.premium
			tst.Frame()

			premLabel, ambLabel, modeLabel := label(c.mode, c.ambient, c.premium)
			prem, ok := tst.Find(premLabel)
			if !ok {
				t.Fatalf("找不到高级质感键 %q", premLabel)
			}
			amb, ok := tst.Find(ambLabel)
			if !ok {
				t.Fatalf("找不到封面背景键 %q", ambLabel)
			}
			mode, ok := tst.Find(modeLabel)
			if !ok {
				t.Fatalf("找不到模式切换键 %q", modeLabel)
			}

			for _, r := range []ui.Rect{prem, amb, mode} {
				if r.W != 28 || r.H != 28 {
					t.Errorf("小开关尺寸 = %v×%v，应为 28×28（原版 .cover-mode-toggle）", r.W, r.H)
				}
			}
			// 从左到右：高级质感 < 封面背景 < 模式切换。
			if !(prem.X < amb.X && amb.X < mode.X) {
				t.Errorf("三个开关的 x = 高级质感 %.0f / 背景 %.0f / 模式 %.0f：应从左到右"+
					"是 高级质感→背景→模式（原版 right 依次 76/42/8），目前是左右镜像的",
					prem.X, amb.X, mode.X)
			}
			// 间距 34（28 宽 + 6 gap）。
			if gap1, gap2 := amb.X-prem.X, mode.X-amb.X; gap1 != 34 || gap2 != 34 {
				t.Errorf("开关间距 = %.0f / %.0f，应为 34 / 34（28 + gap 6）", gap1, gap2)
			}
			// 同一行。
			if prem.Y != amb.Y || amb.Y != mode.Y {
				t.Errorf("三个开关 y = %.0f / %.0f / %.0f：应在同一行", prem.Y, amb.Y, mode.Y)
			}
		})
	}
}
