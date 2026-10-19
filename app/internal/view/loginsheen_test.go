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
	// 右下角取样点必须真的落在 100% 停靠点之外（t = 1 在盒右下角**顶点**，
	// 而 135° 渐变线过中心，所以顶点附近的 t 只到 ≈0.97，那里还剩一点白）。
	// 取更右下的位置：`(310,290)` 的 t ≈ 0.969，`(318,298)` 的 t ≈ 0.99。
	if got := pixelAt(t, img, 318, 298); got != [3]int{0x40, 0x40, 0x40} {
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
	// 面板几何从关闭键反推：键是 absolute Right(loginCloseInset) /
	// Top(loginCloseInset) 的 32 圆，完整落在面板内。
	pr := int(r.X + r.W + loginCloseInset)
	pt := int(r.Y - loginCloseInset)
	pl := pr - 320
	img := tst.Image()

	// 沿左侧一列（远离居中的标题 / 二维码）向下扫，亮度跌回面板外背景水平
	// 就是出了面板。
	//
	// 阈值以前写的是绝对亮度 200：那时登录面板只有 Regular 玻璃的提亮（实测
	// 内侧 229），白 .28 那层被 Material 静默顶掉。底色真正铺上之后内侧是
	// 200.8（.28 白叠在 179.7 上），刚好压在 200 这条线上，扫描一步就断。
	// 改成相对面板外背景（+4）判定，与面板自身亮度解耦。
	col := pl + 20
	outside := lum(pixelAt(t, img, pl-20, pt+120))
	bottom := pt + 20
	for y := pt + 20; y < img.Rect.Dy(); y++ {
		if lum(pixelAt(t, img, col, y)) <= outside+4 {
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
	withSheen := top - bot
	if withSheen < 3 {
		t.Errorf("登录面板顶部不比底部亮：差 %d（应叠上 .login-glass::before 光泽）", withSheen)
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
	noSheen := top2 - bot2
	t.Logf("停靠点全透明后：顶部 %d vs 底部 %d，差 %d", top2, bot2, top2-bot2)
	// 判据用**差值的变化量**，不是「透明后差必须为 0」：面板背后的首页内容
	// （顶栏 / 内容区）本身在 y 方向有梯度，backdrop 模糊后残留 1~3 级 ——
	// 此前按绝对值断言 <3，全量跑偶发假红（基线 HEAD 同样复现）。
	if withSheen-noSheen < 3 {
		t.Errorf("光泽置透明后差值只从 %d 掉到 %d —— 顶部那点亮度不是 loginSheen 给的", withSheen, noSheen)
	}
}

// TestLoginSheenMatchesCSS 是无头像素实测：`loginSheen` 必须与 CSS 的
// `linear-gradient(135deg, 白 .2 → 白 .08 50% → 透明 100%)` 在 320×300 面板上
// 逐点一致（≤1 灰阶）。
//
// 反证（实测过）：退回「按 4 条等高带拼」的旧写法 → 最坏 **38 灰阶**
// （右下角一带原版几乎全透明，拼法还留着 .18 的白 —— 因为每带都把 135° 的
// 渐变线当成水平的重新投影）。见 below 的 TestLoginSheenCounterProof。
func TestLoginSheenMatchesCSS(t *testing.T) {
	if worst := loginSheenWorstDiff(t, 320, 300); worst > 1 {
		t.Errorf("loginSheen 与 CSS 135° 差 %d 灰阶（应 ≤1）", worst)
	}
}

// loginSheenWorstDiff 在一层中灰底上画 loginSheen，量「渲染 vs CSS 135° 精确
// 采样（gradAt + cssLinearT）」的最大逐通道差。
func loginSheenWorstDiff(t *testing.T, w, h int) int {
	t.Helper()
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(float32(w), float32(h)).Background(ui.Hex("#404040")).Children(func() {
			loginSheen(c)
		})
	}, w+40, h+40)
	tst.Frame()
	img := tst.Image()
	worst := 0
	for y := 25; y < h-15; y += 2 {
		for x := 25; x < w-15; x += 2 {
			want := gradAt(loginSheenStops, cssLinearT(float32(x)+0.5, float32(y)+0.5,
				float32(w), float32(h), 135))
			exp := int(0x40 + float32(want.A)/255*(255-0x40))
			o := img.PixOffset(x, y)
			if d := abs32(float32(int(img.Pix[o]) - exp)); int(d) > worst {
				worst = int(d)
			}
		}
	}
	return worst
}

// TestLoginSheenCounterProof 是 TestLoginSheenMatchesCSS 的反证：把实现退回
// 「4 条等高带拼」的旧写法，与同一 CSS 参考的差应显著 > 1。
func TestLoginSheenCounterProof(t *testing.T) {
	saved := loginSheenFn
	defer func() { loginSheenFn = saved }()
	loginSheenFn = func(c *ui.Context) {
		ui.Box(c).Absolute().Left(0).Top(0).Right(0).Bottom(0).
			Radius(loginSheenRadius).Clip().PassThrough().Children(func() {
			for _, b := range gradBands(loginSheenStops, 4) {
				ui.Box(c).FillWidth().Grow(1).PassThrough().
					LinearGradient(ui.LinearGradient{From: b[0], To: b[1], Angle: 135})
			}
		})
	}
	if worst := loginSheenWorstDiff(t, 320, 300); worst <= 1 {
		t.Errorf("退回分带拼法后与 CSS 只差 %d 灰阶，反证不成立", worst)
	} else {
		t.Logf("分带拼法实测最坏 %d 灰阶", worst)
	}
}
