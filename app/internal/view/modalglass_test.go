package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮清的是「模态框面板的底色从来没画上去」这一族（挂到 11-08 剩余差异 ①②）：
//
//	.login-glass   原版 background 白 .28 + backdrop-filter blur(28px)
//	.liquid-dialog 原版 backdrop-filter blur(24px) + 内联 linear-gradient(145deg,
//	               白 .88, 白 .75 55%, accentForType(type))
//
// 两处此前的写法都是 `Material(glass.Glass{…})` 后面跟一个 `Background(...)`：
// Material 把 fill 置成 material，其后的 Background 是空操作（element.go:935 只在
// fill 是 gradient/conic 时才改回 fillColor），于是那条底色被静默丢掉，面板只剩
// Regular 玻璃的「模糊 + 提亮」。
//
// 下面三条：① 钉住停靠点/半径常量；② 无头实测登录面板的白 .28 真的铺上了
// （把 loginPanelFill 改成透明，提亮必须消失）；③ 无头实测对话框的 145° 渐变真的
// 铺上了（类型强调色到达面板右下角，且随类型变色）。

// TestModalGlassParams 钉住两个面板的模糊半径与对话框渐变的停靠点。
func TestModalGlassParams(t *testing.T) {
	if loginGlassBlur != 28 {
		t.Errorf("loginGlassBlur = %v，应为 .login-glass 的 blur(28px)", loginGlassBlur)
	}
	if dialogGlassBlur != 24 {
		t.Errorf("dialogGlassBlur = %v，应为 .liquid-dialog 的 blur(24px)", dialogGlassBlur)
	}
	if loginPanelFill != ui.Hex("#ffffff").Alpha(0.28) {
		t.Errorf("loginPanelFill = %v，应为 .login-glass 的 rgba(255,255,255,.28)", loginPanelFill)
	}
	g := dialogPanelGradient(ui.Hex("#ef4444").Alpha(0.10))
	if g.Angle != 145 {
		t.Errorf("对话框渐变角度 = %v，应为 145deg", g.Angle)
	}
	want := []ui.GradientStop{
		{Color: ui.Hex("#ffffff").Alpha(0.88), At: 0},
		{Color: ui.Hex("#ffffff").Alpha(0.75), At: 0.55},
		{Color: ui.Hex("#ef4444").Alpha(0.10), At: 1},
	}
	if len(g.Stops) != len(want) {
		t.Fatalf("对话框渐变 %d 个停靠点，应为 %d 个", len(g.Stops), len(want))
	}
	for i, w := range want {
		if g.Stops[i] != w {
			t.Errorf("停靠点 %d = %+v，应为 %+v", i, g.Stops[i], w)
		}
	}
}

// loginPanelSample 返回登录面板内侧一点与面板正左方（同一 y）背景的亮度。
//
// 面板几何从「关闭登录」键反推（与 loginsheen_test.go 同一套）：键是 absolute
// Right/Top(loginCloseInset) 的 32 圆，完整落在面板内。取 x = 面板左缘 +20
// （避开居中的二维码卡片），y = 面板顶 +120（避开 ::before 光泽最亮的顶沿）。
func loginPanelSample(t *testing.T) (inside, outside float64) {
	t.Helper()
	tst := loginProbe(t)
	r, ok := tst.Find("关闭登录")
	if !ok {
		t.Fatalf("找不到登录关闭键：%v", tst.Texts())
	}
	pl := int(r.X+r.W+loginCloseInset) - loginPanelWidth
	pt := int(r.Y - loginCloseInset)
	img := tst.Image()
	return avgLuma(img, pl+16, pt+116, pl+24, pt+124),
		avgLuma(img, pl-24, pt+116, pl-16, pt+124)
}

// TestLoginPanelFillPainted 无头渲染实测：.login-glass 的白 .28 真的铺上去了。
//
// 取同一像素在「白 .28 在位」与「白 .28 改透明」两张图里的亮度：后者仍比面板外
// 背景亮 ~14（.login-glass::before 的光泽层），前者必须再亮 ≈ .28×(255-后者)
// —— 这正是 0.28 的白叠上去的解析值（实测 200.8 vs 179.7，差 21.1 = 0.28×75.3）。
// 底色被 Background 静默丢掉时（上一版），两张图逐像素相同（差 0）。
func TestLoginPanelFillPainted(t *testing.T) {
	on, outside := loginPanelSample(t)
	saved := loginPanelFill
	defer func() { loginPanelFill = saved }()
	loginPanelFill = ui.Transparent
	off, _ := loginPanelSample(t)

	lift := on - off
	want := 0.28 * (255 - off)
	t.Logf("登录面板：白 .28 在位 %.1f / 改透明 %.1f（面板外背景 %.1f）；提亮 %.1f，白 .28 应值 %.1f",
		on, off, outside, lift, want)
	if lift < 10 {
		t.Errorf("把白 .28 改透明只少了 %.1f —— 那层底色没画上（Material 之后的 Background 是空操作）", lift)
	}
	if lift > want+6 || lift < want-6 {
		t.Errorf("白 .28 贡献 %.1f，应为 %.1f（.28×(255-底色)）", lift, want)
	}
}

// dialogPanelSample 返回对话框面板左上角内侧与右下角内侧的像素。
//
// 面板几何从标题文案反推：左侧内边距 32 + 图标 28 + gap 16，顶部内边距 28；
// 宽度取最小宽 360（.liquid-dialog min-width，本用例内容短）；底边靠沿面板左
// 侧一列向下扫、亮度回到面板外背景水平处判定。
func dialogPanelSample(t *testing.T, typ string) (tl, br [3]int, left, top, right, bottom int) {
	t.Helper()
	const title = "关于 bili-FM"
	tst := modalProbe(t, &Modal{Kind: "message", Type: typ, Title: title, Message: "bili-FM 3.0.0"})
	r, ok := tst.Find(title)
	if !ok {
		t.Fatalf("找不到对话框标题 %q：%v", title, tst.Texts())
	}
	left = int(r.X) - 32 - 28 - 16
	top = int(r.Y) - 28
	right = left + 360
	img := tst.Image()
	outside := lum(pixelAt(t, img, left-16, top+40))
	bottom = top + 20
	for y := top + 20; y < img.Rect.Dy(); y++ {
		if lum(pixelAt(t, img, left+20, y)) <= outside+4 {
			break
		}
		bottom = y
	}
	if bottom-top < 100 {
		t.Fatalf("%s 面板高度只扫到 %d，几何取错了（left=%d top=%d）", typ, bottom-top, left, top)
	}
	return pixelAt(t, img, left+8, top+8), pixelAt(t, img, right-8, bottom-8),
		left, top, right, bottom
}

// TestDialogPanelGradientPainted 无头渲染实测：.liquid-dialog 的 145° 内联渐变
// 真的铺上去了 —— ① 左上角（白 .88 + ::after 光泽）比右下角（渐变末端的类型色，
// 只有 .10 alpha）亮一大截；② 同一取样点随类型变色（error 偏红、success 偏绿）。
//
// 上一版把这条渐变整个丢掉（Material 之后的 Background 是空操作），面板只剩
// 玻璃材质：左上与右下亮度几乎相同，两种类型也渲染成一样 —— ①② 同时红。
func TestDialogPanelGradientPainted(t *testing.T) {
	got := map[string][3]int{}
	for _, typ := range []string{"error", "success"} {
		tl, br, left, top, right, bottom := dialogPanelSample(t, typ)
		got[typ] = br
		t.Logf("%s：面板 left=%d top=%d right=%d bottom=%d，左上 %v（亮度 %d）右下 %v（亮度 %d）",
			typ, left, top, right, bottom, tl, lum(tl), br, lum(br))
		if d := lum(tl) - lum(br); d < 30 {
			t.Errorf("%s 面板左上 %d 只比右下 %d 亮 %d（应 ≥30）—— 145° 渐变没画上",
				typ, lum(tl), lum(br), d)
		}
	}
	errPx, okPx := got["error"], got["success"]
	t.Logf("右下角：error %v / success %v", errPx, okPx)
	if errPx[0]-okPx[0] < 8 {
		t.Errorf("error 右下角 %v 的红通道不比 success %v 高 8 —— 类型强调色没到渐变末端",
			errPx, okPx)
	}
	if okPx[1]-errPx[1] < 8 {
		t.Errorf("success 右下角 %v 的绿通道不比 error %v 高 8 —— 类型强调色没到渐变末端",
			okPx, errPx)
	}
}
