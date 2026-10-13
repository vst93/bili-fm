package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func sameInk(a, b ui.Color) bool {
	return a.R == b.R && a.G == b.G && a.B == b.B && a.A == b.A
}

// TestPartSearchPaintFor 钉住选集搜索框三态的解析值（原版 globals.css 里
// `[data-slot="wrapper"] header .part-search` 的 :hover / :focus-within 规则）。
//
// 此前原生版只画了常态度：悬停与聚焦都没有任何反馈，放大镜也偏浅（用了主题的
// Faint #64748b，而不是原版的 #475569）。
func TestPartSearchPaintFor(t *testing.T) {
	cases := []struct {
		name             string
		hovered, focused bool
		border, bg, icon ui.Color
		ring             ui.Color
		hasRing          bool
	}{
		{
			name: "常态",
			border: ui.Hex("#475569").Alpha(0.50),
			bg:     ui.Hex("#0f172a").Alpha(0.035),
			icon:   ui.Hex("#475569"),
		},
		{
			name:    "悬停",
			hovered: true,
			border:  ui.Hex("#475569").Alpha(0.65),
			bg:      ui.Hex("#0f172a").Alpha(0.035),
			icon:    ui.Hex("#475569"),
		},
		{
			name:    "聚焦",
			focused: true,
			border:  ui.Hex("#0ea5e9").Alpha(0.40),
			bg:      ui.Hex("#ffffff").Alpha(0.05),
			icon:    ui.Hex("#0284c7"),
			ring:    ui.Hex("#0ea5e9").Alpha(0.14),
			hasRing: true,
		},
		{
			name:    "聚焦+悬停",
			hovered: true,
			focused: true,
			border:  ui.Hex("#0ea5e9").Alpha(0.40),
			bg:      ui.Hex("#ffffff").Alpha(0.05),
			icon:    ui.Hex("#0284c7"),
			ring:    ui.Hex("#0ea5e9").Alpha(0.14),
			hasRing: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			p := partSearchPaintFor(c.hovered, c.focused)
			if !sameInk(p.Border, c.border) {
				t.Errorf("描边 = %v，应为 %v", p.Border, c.border)
			}
			if !sameInk(p.BG, c.bg) {
				t.Errorf("填充 = %v，应为 %v", p.BG, c.bg)
			}
			if !sameInk(p.Icon, c.icon) {
				t.Errorf("放大镜 = %v，应为 %v", p.Icon, c.icon)
			}
			if c.hasRing {
				if !sameInk(p.Ring, c.ring) {
					t.Errorf("外圈光环 = %v，应为 %v", p.Ring, c.ring)
				}
			} else if p.Ring.A != 0 {
				t.Errorf("非聚焦态不该有外圈光环，得到 %v", p.Ring)
			}
		})
	}
}

// TestPartSearchFocusRing 无头实测：选集搜索框聚焦后真的换上蓝色描边 + 蓝环 +
// 蓝放大镜。量的是框域内的「蓝度」总和 Σ(B−R)。
//
// 行为级反证：把 partSearchPaintFor 临时换成「永远返回常态度」后，聚焦带来的
// 额外蓝度应几乎消失。删掉 partsHeader 里对 box.FocusWithin() 的接线（或把聚焦
// 分支拆掉）后，正常蓝度会塌到和 stub 一样，本测试即红。
func TestPartSearchFocusRing(t *testing.T) {
	// bluenessFor 渲染一对「常态度 / 聚焦态」的帧，返回各自的框域蓝度。stub 为真时
	// 在渲染期间把 paint helper 固定成常态度。
	bluenessFor := func(stub bool) (offBlue, onBlue int) {
		saved := partSearchPaintFor
		if stub {
			partSearchPaintFor = func(hovered, focused bool) partSearchPaint {
				return partSearchPaint{
					Border: ui.Hex("#475569").Alpha(0.50),
					BG:     ui.Hex("#0f172a").Alpha(0.035),
					Icon:   ui.Hex("#475569"),
				}
			}
			defer func() { partSearchPaintFor = saved }()
		}

		build := func() (*ui.Tester, ui.Rect) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Info = newPartsInfo(3, "cover")
			a.Track = &Track{Bvid: "BV1", Cid: 1000, Title: "视频"}
			a.Drawer = DrawerParts
			tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
			tt.Frame()
			r, ok := tt.Find("搜索选集")
			if !ok {
				t.Fatal("找不到选集搜索框（label「搜索选集」）")
			}
			return tt, r
		}

		// 常态度。
		unfocused, r := build()
		off := unfocused.Image()

		// 聚焦态：点一下输入框，再重画一帧让 FocusWithin 生效。
		focused, _ := build()
		if err := focused.Click("搜索选集"); err != nil {
			t.Fatalf("点选集搜索框失败：%v", err)
		}
		focused.Frame()
		if !focused.Focused("搜索选集") {
			t.Fatal("点击后选集搜索框没拿到键盘焦点")
		}
		on := focused.Image()

		// 采样区：输入框外扩 12px，把容器描边、2px 光环与左侧放大镜都框进来。
		x0, y0 := int(r.X)-12, int(r.Y)-12
		x1, y1 := int(r.X+r.W)+12, int(r.Y+r.H)+12
		if x0 < 0 {
			x0 = 0
		}
		if y0 < 0 {
			y0 = 0
		}
		maxX, maxY := on.Bounds().Max.X, on.Bounds().Max.Y
		if x1 > maxX {
			x1 = maxX
		}
		if y1 > maxY {
			y1 = maxY
		}

		sum := func(img *image.RGBA) int {
			total := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					i := img.PixOffset(x, y)
					total += int(img.Pix[i+2]) - int(img.Pix[i])
				}
			}
			return total
		}
		return sum(off), sum(on)
	}

	// 正常代码：聚焦让框域明显变蓝。
	offBlue, onBlue := bluenessFor(false)
	// 反证基准：paint helper 固定成常态度后，聚焦只剩「光标 / 选中」等噪声。
	stubOff, stubOn := bluenessFor(true)
	normalGain := onBlue - offBlue
	stubGain := stubOn - stubOff
	t.Logf("选集搜索框蓝度：正常 %d→%d（+%d），stub %d→%d（+%d）",
		offBlue, onBlue, normalGain, stubOff, stubOn, stubGain)
	if normalGain-stubGain < 20000 {
		t.Errorf("聚焦带来的额外蓝度只有 %d（正常 +%d、stub +%d）：聚焦态的蓝色描边/光环/放大镜没接上？",
			normalGain-stubGain, normalGain, stubGain)
	}
}
