package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLoginSheenStops 钉住 .login-glass::before 的三个停靠点
// （白 .2 → 白 .08 50% → 透明 100%）。
func TestLoginSheenStops(t *testing.T) {
	want := []struct {
		at    float32
		alpha float32
	}{{0, 0.20}, {0.5, 0.08}, {1, 0}}
	if len(loginSheenStops) != len(want) {
		t.Fatalf("停靠点 %d 个，应为 %d 个", len(loginSheenStops), len(want))
	}
	for i, w := range want {
		s := loginSheenStops[i]
		if s.At != w.at {
			t.Errorf("第 %d 个停靠点位置 = %v，应为 %v", i, s.At, w.at)
		}
		if s.C != ui.Hex("#ffffff").Alpha(w.alpha) {
			t.Errorf("第 %d 个停靠点颜色 = %v，应为白 .%v", i, s.C, w.alpha)
		}
	}
}

// TestLoginSheenRendered 是无头渲染实测：loginSheen 在一层深灰底上真的画出了
// 135° 白色光泽 —— 左上最亮、右下全透明（回到底色），且同一行左边比右边亮
// （证明角度是 135° 而不是 90°/180°）。
//
// 深灰底（#404040，亮度 64）而不是登录面板的浅玻璃：面板底已经接近饱和
// （~225），白 .2 叠上去只剩几个亮度，方向性会被噪声淹没。深底上光泽的
// 相对形状一目了然。
func TestLoginSheenRendered(t *testing.T) {
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(320, 300).Background(ui.Hex("#404040")).Children(func() {
			loginSheen(c)
		})
	}, 400, 400)
	tst.Frame()
	img := tst.Image()

	tl := lum(pixelAt(t, img, 10, 10))
	tr := lum(pixelAt(t, img, 310, 10))
	bl := lum(pixelAt(t, img, 10, 290))
	br := lum(pixelAt(t, img, 310, 290))
	base := lum(pixelAt(t, img, 310, 290))
	t.Logf("光泽四角亮度：左上 %d / 右上 %d / 左下 %d / 右下 %d（底色 %d）",
		tl, tr, bl, br, base)

	if tl <= tr {
		t.Errorf("同一行左 %d 应比右 %d 亮（135° 从左上起）", tl, tr)
	}
	if tr <= br {
		t.Errorf("同一列上 %d 应比下 %d 亮（渐变往右下淡出）", tr, br)
	}
	if tl-br < 30 {
		t.Errorf("左上 %d 应比右下 %d 亮至少 30（白 .2 起、右下全透明）", tl, br)
	}
	if got := pixelAt(t, img, 310, 290); got != [3]int{0x40, 0x40, 0x40} {
		t.Errorf("右下角 %v 应为底色 #404040（100%% 停靠点全透明）", got)
	}
}

// TestLoginPanelSheenWired 是无头渲染实测：登录面板顶部真的叠上了光泽层
// （顶部比底部亮），并且这层就是 loginSheen —— 把停靠点全设成透明后差值消失。
//
// 此前（上轮 1892 记的「下轮候选」）原生版没画 .login-glass::before，面板顶部
// 与底部亮度相同（差 0）；补上后顶部白 .2 让它比底部亮 ~5。
func TestLoginPanelSheenWired(t *testing.T) {
	tst := loginProbe(t)
	r, ok := tst.Find("关闭登录")
	if !ok {
		t.Fatalf("找不到登录关闭键：%v", tst.Texts())
	}
	// 面板几何从关闭键反推：键是 absolute right:-16 top:16 的 32 圆。
	pr := int(r.X + r.W - 16)
	pt := int(r.Y - 16)
	pl := pr - 320
	img := tst.Image()

	// 沿左侧一列（远离居中的标题 / 二维码）向下扫，亮度跌破 200 就是出了面板。
	col := pl + 20
	bottom := pt + 20
	for y := pt + 20; y < img.Rect.Dy(); y++ {
		if lum(pixelAt(t, img, col, y)) < 200 {
			break
		}
		bottom = y
	}
	if bottom-pt < 200 {
		t.Fatalf("面板高度只有 %d，扫描没找到底边（几何取错了？）", bottom-pt)
	}
	topY, botY := pt+20, bottom-20
	top := lum(pixelAt(t, img, col, topY))
	bot := lum(pixelAt(t, img, col, botY))
	t.Logf("面板 left=%d top=%d bottom=%d；顶部(%d) 亮度 %d vs 底部(%d) 亮度 %d，差 %d",
		pl, pt, bottom, topY, top, botY, bot, top-bot)
	if top-bot < 3 {
		t.Errorf("登录面板顶部不比底部亮：差 %d（应叠上 .login-glass::before 光泽）", top-bot)
	}

	// 反证：把光泽停靠点全设成透明，同一对取样点的差值应消失 —— 证明上面那点
	// 亮度来自 loginSheen，而不是面板玻璃自带的渐变。
	saved := loginSheenStops
	defer func() { loginSheenStops = saved }()
	loginSheenStops = []gradStop{
		{0, ui.Transparent}, {0.5, ui.Transparent}, {1, ui.Transparent},
	}
	img2 := loginProbe(t).Image()
	top2 := lum(pixelAt(t, img2, col, topY))
	bot2 := lum(pixelAt(t, img2, col, botY))
	t.Logf("停靠点全透明后：顶部 %d vs 底部 %d，差 %d", top2, bot2, top2-bot2)
	if top2-bot2 >= 3 {
		t.Errorf("光泽已置透明，顶部仍比底部亮 %d（面板玻璃自带渐变？断言前提不成立）", top2-bot2)
	}
}
