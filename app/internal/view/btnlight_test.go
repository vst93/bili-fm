package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestBtnLightStops 钉住四条玻璃聚光的停靠点（逐字取自 globals.css）：
//
//	nav   .nav-icon-btn / .app-title-bar button:hover  白 .38 → .12(30%) → .02(58%) → 0(72%)
//	tool  .top-tool-btn / .search-submit-btn           白 .36 → .10(30%) → 0(72%)
//	close .app-title-bar-close:hover                   白 .30 → 0(72%)
//	press 以上各族 :active                              白 .16 → .04(38%) → 0(65%)
func TestBtnLightStops(t *testing.T) {
	cases := []struct {
		name string
		l    btnLight
		want []struct {
			at    float32
			alpha float32
		}
	}{
		{"nav", btnLightNav, []struct {
			at    float32
			alpha float32
		}{{0, 0.38}, {0.30, 0.12}, {0.58, 0.02}, {0.72, 0}}},
		{"tool", btnLightTool, []struct {
			at    float32
			alpha float32
		}{{0, 0.36}, {0.30, 0.10}, {0.72, 0}}},
		{"close", btnLightClose, []struct {
			at    float32
			alpha float32
		}{{0, 0.30}, {0.72, 0}}},
		{"press", btnLightPress, []struct {
			at    float32
			alpha float32
		}{{0, 0.16}, {0.38, 0.04}, {0.65, 0}}},
	}
	for _, tc := range cases {
		if tc.l.n != len(tc.want) {
			t.Errorf("%s：停靠点 %d 个，应为 %d 个", tc.name, tc.l.n, len(tc.want))
			continue
		}
		for i, w := range tc.want {
			s := tc.l.stops[i]
			if s.At != w.at {
				t.Errorf("%s 第 %d 个停靠点位置 = %v，应为 %v", tc.name, i, s.At, w.at)
			}
			if s.C != ui.Hex("#ffffff").Alpha(w.alpha) {
				t.Errorf("%s 第 %d 个停靠点颜色 = %v，应为白 .%v", tc.name, i, s.C, w.alpha)
			}
		}
	}
}

// TestBtnLightAlphaAt 钉住径向几何（CSS `radial-gradient(80% 110% at 30% 8%)`）：
// 光心在左上 30%/8% 处最亮，越往外越淡、过最后一个停靠点归零；按下那条约居中的
// 光在正中（50%/50%）最亮。
func TestBtnLightAlphaAt(t *testing.T) {
	// 光心处 = 第一个停靠点。
	if got := btnLightNav.alphaAt(0.30, 0.08, 1, 1); abs32(got-0.38) > 0.005 {
		t.Errorf("nav 光心 alpha = %v，应为 .38", got)
	}
	// (0.3,0.08)→(1,1) 的距离 = √(0.875²+0.836²) ≈ 1.21 > 0.72 ⇒ 0。
	if got := btnLightNav.alphaAt(1, 1, 1, 1); got != 0 {
		t.Errorf("nav 右下角 alpha = %v，应为 0（已过 .72 停靠点）", got)
	}
	// 左上角 (0,0) 落在 .12→.02 之间。
	got := btnLightNav.alphaAt(0, 0, 1, 1)
	if got <= 0.02 || got >= 0.12 {
		t.Errorf("nav 左上角 alpha = %v，应落在 (.02,.12) 之间", got)
	}
	if btnLightNav.alphaAt(0.05, 0.05, 1, 1) <= got {
		t.Error("越靠近光心应越亮（(0.05,0.05) 应亮于 (0,0)）")
	}

	// 按下那条约居中：正中 50%/50% 最亮 = .16。
	if got := btnLightPress.alphaAt(0.50, 0.50, 1, 1); abs32(got-0.16) > 0.005 {
		t.Errorf("press 光心 alpha = %v，应为 .16", got)
	}
	if btnLightPress.alphaAt(0, 0, 1, 1) != 0 {
		t.Errorf("press 左上角 alpha = %v，应为 0（半径只到 65%%）", btnLightPress.alphaAt(0, 0, 1, 1))
	}

	// 空几何（测试里用来「关掉」这层）返回 0。
	var empty btnLight
	if empty.alphaAt(0.3, 0.08, 1, 1) != 0 {
		t.Error("空 btnLight 的 alpha 应为 0")
	}
}

// TestTitleBarBtnLight 钉住标题栏 / 迷你窗按钮该铺哪条光：常态不铺，悬停铺左上
// .38，按下铺居中 .16（原版这两族常态没有 background-image）。
func TestTitleBarBtnLight(t *testing.T) {
	if _, ok := titleBarBtnLight(false, false); ok {
		t.Error("常态不该铺高光")
	}
	if l, ok := titleBarBtnLight(false, true); !ok || l != btnLightNav {
		t.Error("悬停应铺 btnLightNav")
	}
	if l, ok := titleBarBtnLight(true, true); !ok || l != btnLightPress {
		t.Error("按下应铺 btnLightPress（盖过悬停）")
	}
	if l, ok := titleBarBtnLight(true, false); !ok || l != btnLightPress {
		t.Error("按下（未悬停）也应铺 btnLightPress")
	}
}

// TestBtnLightRendered 是无头渲染实测：btnLightNav 在一层深灰底上真的画出了
// 「左上亮、往右下淡出」的径向聚光 —— 左上角比正中亮、正中比右下亮、右下
// 回到底色；按下那条则正中亮、四角淡。
func TestBtnLightRendered(t *testing.T) {
	probe := func(l btnLight) *image.RGBA {
		tst := ui.NewTester(func(c *ui.Context) {
			box := ui.Box(c).Size(64, 64).Radius(8).Background(ui.Hex("#404040")).Clip()
			l.draw(box, 8)
		}, 80, 80)
		tst.Frame()
		return tst.Image()
	}

	img := probe(btnLightNav)
	tl := lum(pixelAt(t, img, 6, 6))
	ct := lum(pixelAt(t, img, 32, 32))
	br := lum(pixelAt(t, img, 58, 58))
	base := 64 // #404040 的亮度
	t.Logf("nav 聚光：左上 %d / 正中 %d / 右下 %d（底色 %d）", tl, ct, br, base)

	if tl <= ct {
		t.Errorf("左上 %d 应比正中 %d 亮", tl, ct)
	}
	if ct <= br {
		t.Errorf("正中 %d 应比右下 %d 亮", ct, br)
	}
	if tl-br < 30 {
		t.Errorf("左上 %d 应比右下 %d 亮至少 30（白 .38 起、右下全透明）", tl, br)
	}
	if br != base {
		t.Errorf("右下角亮度 %d 应回到底色 %d（过 .72 停靠点即全透明）", br, base)
	}

	// 按下那条：正中亮、四角暗。
	pimg := probe(btnLightPress)
	pc := lum(pixelAt(t, pimg, 32, 32))
	pc0 := lum(pixelAt(t, pimg, 6, 6))
	pc1 := lum(pixelAt(t, pimg, 58, 58))
	if pc <= pc0 || pc <= pc1 {
		t.Errorf("按下聚光应正中 %d 最亮，得到左上 %d / 右下 %d", pc, pc0, pc1)
	}
}

// TestBtnLightWired 是无头渲染实测：真实的按钮（搜索药丸的「动态」入口 +
// 主区操作行的图标键）真的铺上了这层聚光 —— 把对应族的高光临时换成全透明后，
// 同一按钮区域里大量像素跟着变，反证那层光来自 btnLight* 而不是别处。
//
// 只比较按钮自身的矩形（由 Tester.Find 取），不受窗口其它地方影响。
func TestBtnLightWired(t *testing.T) {
	type probe struct {
		label string
		fam   *btnLight
		setup func(a *App)
	}
	cases := []probe{
		{"动态", &btnLightTool, nil},
		{"浏览器打开", &btnLightNav, func(a *App) {
			a.Info = &Info{Bvid: "BV1sheen", Aid: 1, Cid: 2, Title: "标题"}
			a.Track = &Track{Bvid: "BV1sheen", Title: "标题"}
			a.Queue = []Track{*a.Track}
			a.Index = 0
		}},
	}
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			frame := func() (*image.RGBA, ui.Rect, bool) {
				a := NewApp(func() {})
				if tc.setup != nil {
					tc.setup(a)
				}
				tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
				tt.Frame()
				tt.Frame()
				r, ok := tt.Find(tc.label)
				return tt.Image(), r, ok
			}

			on, rect, ok := frame()
			if !ok {
				t.Fatalf("找不到按钮 %q", tc.label)
			}

			saved := *tc.fam
			*tc.fam = newBtnLight(0.30, 0.08, 0.80, 1.10, gradStop{0, ui.Hex("#ffffff").Alpha(0)})
			defer func() { *tc.fam = saved }()
			off, _, _ := frame()

			diff, total := 0, 0
			for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
				for x := int(rect.X); x < int(rect.X+rect.W); x++ {
					total++
					i := on.PixOffset(x, y)
					if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
						diff++
					}
				}
			}
			t.Logf("%s：按钮矩形 %v，%d/%d 像素随聚光开关而变化", tc.label, rect, diff, total)
			if diff < total/10 {
				t.Errorf("%s 只有 %d/%d 像素受高光影响（<10%%），按钮上没铺上聚光？", tc.label, diff, total)
			}
		})
	}
}
