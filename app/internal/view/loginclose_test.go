package view

import (
	"image"
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestLoginClosePaint 钉住登录关闭键（.login-close-btn）的三态取值：常态是
// 半透明白圆（白 .2 底 + 白 .3 边 + 顶部内高光 + 墨 .12 投影），悬停转白 .28 /
// 边白 .4 / 投影抬到墨 .15，按下只缩到 .92（底/边/投影沿用悬停那套）。
func TestLoginClosePaint(t *testing.T) {
	rest := loginClosePaintFor(false, false)
	if rest.BG != ui.Hex("#ffffff").Alpha(0.20) {
		t.Errorf("常态底色 = %v，应为白 .20", rest.BG)
	}
	if rest.Border != ui.Hex("#ffffff").Alpha(0.30) {
		t.Errorf("常态描边 = %v，应为白 .30", rest.Border)
	}
	if len(rest.Inset) != 1 || rest.Inset[0] != (shadowSpec{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.35)}) {
		t.Errorf("常态内高光 = %+v，应为 inset 0 1 1 白 .35", rest.Inset)
	}
	if len(rest.Outer) != 1 || rest.Outer[0] != (shadowSpec{0, 4, 12, 0, ui.Hex("#0f172a").Alpha(0.12)}) {
		t.Errorf("常态外投影 = %+v，应为 0 4 12 墨 .12", rest.Outer)
	}
	if rest.Scale != 1 {
		t.Errorf("常态缩放 = %v，应为 1", rest.Scale)
	}

	hover := loginClosePaintFor(true, false)
	if hover.BG != ui.Hex("#ffffff").Alpha(0.28) ||
		hover.Border != ui.Hex("#ffffff").Alpha(0.40) {
		t.Errorf("悬停底色/描边 = %v / %v，应为白 .28 / 白 .40", hover.BG, hover.Border)
	}
	if len(hover.Inset) != 1 || hover.Inset[0] != (shadowSpec{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.40)}) {
		t.Errorf("悬停内高光 = %+v，应为 inset 0 1 1 白 .40", hover.Inset)
	}
	if len(hover.Outer) != 1 || hover.Outer[0] != (shadowSpec{0, 6, 16, 0, ui.Hex("#0f172a").Alpha(0.15)}) {
		t.Errorf("悬停外投影 = %+v，应为 0 6 16 墨 .15", hover.Outer)
	}
	if hover.Scale != 1 {
		t.Errorf("悬停缩放 = %v，应为 1（原版只有 :active 才缩）", hover.Scale)
	}

	// 按下：底/边/投影与悬停一致，只有缩放不同。
	pressed := loginClosePaintFor(true, true)
	if pressed.BG != hover.BG || pressed.Border != hover.Border {
		t.Errorf("按下态底/边应沿用悬停：%v / %v", pressed.BG, pressed.Border)
	}
	if pressed.Scale != 0.92 {
		t.Errorf("按下缩放 = %v，应为 .92", pressed.Scale)
	}
	// 未悬停时按下也缩（原版 :active 不依赖 :hover）。
	if got := loginClosePaintFor(false, true).Scale; got != 0.92 {
		t.Errorf("未悬停按下缩放 = %v，应为 .92", got)
	}
}

// TestModalScrimInk 钉住模态遮罩底色：对话框是 slate .2（原版 DialogProvider
// 的 rgba(15,23,42,.2)），登录是纯黑 .25（原版 bg-black/25）。
func TestModalScrimInk(t *testing.T) {
	if got := modalScrimInk("login"); got != ui.Hex("#000000").Alpha(0.25) {
		t.Errorf("登录遮罩 = %v，应为纯黑 .25", got)
	}
	for _, kind := range []string{"message", "", "nonsense"} {
		if got := modalScrimInk(kind); got != ui.Hex("#0f172a").Alpha(0.20) {
			t.Errorf("遮罩(%q) = %v，应为 slate .20", kind, got)
		}
	}
}

// loginProbe 建一个 1000×700 的主窗，可选打开登录面板。入场动画按 modalOpenedAt
// 算进度：把时间戳拨到过去，首帧直接取终值（遮罩 alpha 满、面板 opacity 1）。
func loginProbe(t *testing.T) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Modal = &Modal{Kind: "login", Title: "使用 B站 App 扫码登录"}
	a.modalOpenedAt = time.Now().Add(-time.Second)
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return tst
}

// TestModalScrimDimsWindow 是无头渲染实测：登录面板打开时整窗真的被压暗了。
// 取远离面板的左中位置，对比「无模态」与「登录面板打开」两张图的同一像素——
// 纯黑 .25 叠上去后应明显更暗。此前 DialogBase 的 backdrop 完全没上色，两图
// 逐像素相同（该断言会红）。
func TestModalScrimDimsWindow(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	before := pixelAt(t, tst.Image(), 20, 300)

	withLogin := loginProbe(t)
	after := pixelAt(t, withLogin.Image(), 20, 300)

	lumBefore := lum(before)
	lumAfter := lum(after)
	t.Logf("左中像素：无模态 %v(亮度 %d) → 登录打开 %v(亮度 %d)", before, lumBefore, after, lumAfter)
	if lumAfter >= lumBefore-30 {
		t.Errorf("登录面板打开后窗口没有明显压暗：亮度 %d → %d（应为纯黑 .25）", lumBefore, lumAfter)
	}
}

// TestLoginCloseButtonGlassCircle 是无头渲染实测：登录关闭键画成了半透明白色
// 磨砂圆，而不是裸图标。关闭键贴在面板右缘，右半落在遮罩上——取键内右侧
// （图标之外）的像素，应比键外同一行的遮罩像素更亮（白 .2 底 + 白 .3 描边）。
//
// 阈值 **5**：真实键实测差 **7**，换成裸图标（下面 TestLoginCloseButtonNakedCounterProof
// 把取值函数换成「无底无边」）实测 **0** —— 5 落在两者中间。
//
// 注：这个阈值此前写的是 8。那是**随被感染的实现走的旧断言**：登录面板那层
// 135° 光泽用「4 条等高带拼」近似时（见 loginsheen.go），面板右下角一带被
// 多铺了约 .18 的白，把遮罩与键内的基线一起抬高、差值也跟着虚大到 8。光泽
// 改成真多停靠点渐变后基线回正（实测差 7），8 就成了假红。这正是「旧断言拿
// 被感染的实现当期望值」的又一例。
func TestLoginCloseButtonGlassCircle(t *testing.T) {
	d := loginCloseDelta(t)
	if d < 5 {
		t.Errorf("关闭键内不比遮罩亮：差 %d（应为半透明白圆，非裸图标）", d)
	}
}

// loginCloseDelta 返回「关闭键内右侧像素 − 键外同一行遮罩像素」的亮度差。
func loginCloseDelta(t *testing.T) int {
	t.Helper()
	tst := loginProbe(t)
	r, ok := tst.Find("关闭登录")
	if !ok {
		t.Fatalf("找不到登录关闭键：%v", tst.Texts())
	}
	cy := int(r.Y + r.H/2)
	inside := pixelAt(t, tst.Image(), int(r.X+r.W-4), cy)
	outside := pixelAt(t, tst.Image(), int(r.X+r.W+6), cy)
	delta := lum(inside) - lum(outside)
	t.Logf("关闭键 rect=%+v；键内 %v(亮度 %d) vs 键外遮罩 %v(亮度 %d)，差 %d",
		r, inside, lum(inside), outside, lum(outside), delta)
	return delta
}

// TestLoginCloseButtonNakedCounterProof 是上一条的反证：把关闭键的取值换成
// 「无底、无边、无影」（裸图标），键内与键外遮罩应逐像素相同。
func TestLoginCloseButtonNakedCounterProof(t *testing.T) {
	saved := loginClosePaintFn
	defer func() { loginClosePaintFn = saved }()
	loginClosePaintFn = func(hovered, pressed bool) loginClosePaint {
		return loginClosePaint{Scale: 1}
	}
	if d := loginCloseDelta(t); d != 0 {
		t.Errorf("裸图标时键内比遮罩亮 %d，反证不成立（应为 0）", d)
	}
}

// TestLoginCloseButtonInsidePanel 是无头几何实测：登录关闭键必须完整落在面板
// **里面**。原版是 `-right-4 top-4`（探出面板右缘 16px），用户反馈那样会掉到
// 背景框外面；改成右／上各内缩 loginCloseInset。面板 320 宽、在 1000 宽窗口里
// 居中，右缘在 (1000+320)/2 = 660。
func TestLoginCloseButtonInsidePanel(t *testing.T) {
	tst := loginProbe(t)
	r, ok := tst.Find("关闭登录")
	if !ok {
		t.Fatalf("找不到登录关闭键：%v", tst.Texts())
	}
	panelRight := float32(1000+loginPanelWidth) / 2
	panelLeft := float32(1000-loginPanelWidth) / 2
	t.Logf("关闭键 rect=%+v，面板右缘=%.1f", r, panelRight)
	if r.X+r.W > panelRight {
		t.Errorf("关闭键右缘 %.1f 越出面板右缘 %.1f（应落在面板内）", r.X+r.W, panelRight)
	}
	if r.X < panelLeft {
		t.Errorf("关闭键左缘 %.1f 越出面板左缘 %.1f", r.X, panelLeft)
	}
	if r.Y < 0 {
		t.Errorf("关闭键上缘 %.1f 越出面板", r.Y)
	}
}

// pixelAt 取一张 RGBA 图里某点的 RGB（越界返回黑色，测试里由断言暴露）。
func pixelAt(t *testing.T, img *image.RGBA, x, y int) [3]int {
	t.Helper()
	if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
		t.Fatalf("取样点 (%d,%d) 越出 %v", x, y, img.Rect)
	}
	i := img.PixOffset(x, y)
	return [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
}

// lum 是感知亮度（与原版 YIQ 同系数，测试里只用来比较明暗）。
func lum(c [3]int) int {
	return (c[0]*299 + c[1]*587 + c[2]*114) / 1000
}

// TestLoginQRCardPaint 钉住登录二维码卡片（原版 .login-qr-card）的底 / 边 / 影：
//
//	background: rgba(255,255,255,.95)
//	border:     1px solid rgba(255,255,255,.6)
//	box-shadow: inset 0 1px 0 rgba(255,255,255,1),
//	            0 8px 24px rgba(15,23,42,.1),
//	            0 1px 2px rgba(15,23,42,.06)
//
// 原生版此前画成纯白实底 + 单条 Tailwind shadow-lg 的近似（0 10 15 墨 .12），
// 少了 1px 白描边、顶部内高光与贴身暗影 —— 卡片贴在同样近白的登录玻璃面板上
// 会糊成一片、看不出边界。
func TestLoginQRCardPaint(t *testing.T) {
	p := loginQRCardPaint()
	if p.BG != ui.Hex("#ffffff").Alpha(0.95) {
		t.Errorf("二维码卡片底色 = %v，应为白 .95", p.BG)
	}
	if p.Border != ui.Hex("#ffffff").Alpha(0.60) {
		t.Errorf("二维码卡片描边 = %v，应为白 .60", p.Border)
	}
	if len(p.Inset) != 1 || p.Inset[0] != (shadowSpec{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(1)}) {
		t.Errorf("二维码卡片内高光 = %+v，应为 inset 0 1 0 白 1", p.Inset)
	}
	want := []shadowSpec{
		{0, 8, 24, 0, ui.Hex("#0f172a").Alpha(0.10)},
		{0, 1, 2, 0, ui.Hex("#0f172a").Alpha(0.06)},
	}
	if !slices.Equal(p.Outer, want) {
		t.Errorf("二维码卡片外投影 = %+v，应为 0 8 24 墨 .1 + 0 1 2 墨 .06", p.Outer)
	}
}
