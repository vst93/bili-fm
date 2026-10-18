package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestToastAccent 钉住原版 utils/toast.ts 的类型 → 图标/颜色映射
// （success 勾 / error 叉 / warning 感叹 / 默认信息）。
func TestToastAccent(t *testing.T) {
	cases := []struct {
		typ  string
		ink  string
		icon *ui.SVG
	}{
		{"success", "#22c55e", iconCheckOne},
		{"error", "#f43f5e", iconCloseOne},
		{"warning", "#f59e0b", iconWarn},
		{"info", "#0ea5e9", iconInfo},
		{"", "#0ea5e9", iconInfo},
		{"nonsense", "#0ea5e9", iconInfo},
	}
	for _, tc := range cases {
		ink, icon := toastAccent(tc.typ)
		if icon != tc.icon {
			t.Errorf("toastAccent(%q) 图标 = %p，应为 %p", tc.typ, icon, tc.icon)
		}
		if want := ui.Hex(tc.ink); ink != want {
			t.Errorf("toastAccent(%q) 颜色 = %v，应为 %v", tc.typ, ink, want)
		}
	}
}

// TestToastFillStops 钉住原版根元素的内联背景
// `linear-gradient(135deg, rgba(255,255,255,0.5), accent 8%)`。
func TestToastFillStops(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"success", "#22c55e"},
		{"error", "#f43f5e"},
		{"warning", "#f59e0b"},
		{"info", "#0ea5e9"},
	}
	for _, tc := range cases {
		from, to := toastFillStops(ui.Hex(tc.want))
		if from != ui.Hex("#ffffff").Alpha(0.5) {
			t.Errorf("toastFillStops(%s) 起点 = %v，应为白 .5", tc.in, from)
		}
		if want := ui.Hex(tc.want).Alpha(0.08); to != want {
			t.Errorf("toastFillStops(%s) 终点 = %v，应为 %v", tc.in, to, want)
		}
	}
}

// TestToastTransition 钉住原版根的 `transition-all duration-300` +
// `-translate-y-2`（8px 向上）：入场/退场都是上方 8px、从/到透明。
func TestToastTransition(t *testing.T) {
	// 时长由 toastEnterDuration 驱动（测试会把它临时放长，见 TestToastFadesIn）。
	if toastEnterDuration != 300*time.Millisecond {
		t.Errorf("toastEnterDuration = %v，应为 300ms（原版 duration-300）", toastEnterDuration)
	}
	tr := toastTransition()
	if tr.Duration != toastEnterDuration {
		t.Errorf("时长 = %v，应为 %v", tr.Duration, toastEnterDuration)
	}
	for name, m := range map[string]*ui.Motion{"Enter": tr.Enter, "Exit": tr.Exit} {
		if m == nil {
			t.Fatalf("%s 为空", name)
		}
		if m.Y != -8 {
			t.Errorf("%s 的 Y = %v，应为 -8（上方 8px 的 -translate-y-2）", name, m.Y)
		}
		if m.Opacity != 0 {
			t.Errorf("%s 的不透明度 = %v，应为 0（从/到透明）", name, m.Opacity)
		}
	}
}

// TestNotifyToastDuration 钉住默认时长：原版 utils/toast.ts 是 2s，框架默认
// 4s，所以入队前必须显式写成 2s；调用点自己给了时长则原样保留。
func TestNotifyToastDuration(t *testing.T) {
	if toastDuration != 2*time.Second {
		t.Fatalf("toastDuration = %v，应为 2s", toastDuration)
	}
	a := NewApp(func() {})
	a.Notify("普通")
	a.NotifyType("error", "报错")
	a.notifyToast(ui.Toast{Title: "自定义", Timeout: 5 * time.Second})
	a.toastMu.Lock()
	got := append([]ui.Toast(nil), a.toasts...)
	a.toastMu.Unlock()
	if len(got) != 3 {
		t.Fatalf("入队 %d 条，应为 3 条", len(got))
	}
	for i, w := range []time.Duration{toastDuration, toastDuration, 5 * time.Second} {
		if got[i].Timeout != w {
			t.Errorf("第 %d 条时长 = %v，应为 %v", i, got[i].Timeout, w)
		}
	}
	if got[1].Type != "error" {
		t.Errorf("NotifyType 的类型 = %q，应为 error", got[1].Type)
	}
}

// toastProbe 建一个已跑过一帧的窗口（让 toast viewport 先诞生），再弹一条
// toast、跑完入场动画，返回 tester 与 toast 文案的矩形。
func toastProbe(t *testing.T, typ, msg string) (*ui.Tester, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame() // viewport 先建出来，toast 才算「在已存在的父级里出现」（Enter 生效）
	a.NotifyType(typ, msg)
	tst.Frame()
	tst.Advance(400 * time.Millisecond) // 推过 300ms 入场（v0.5.5 冻结时钟）
	tst.Frame()
	r, ok := tst.Find(msg)
	if !ok {
		t.Fatalf("找不到 toast 文案 %q：%v", msg, tst.Texts())
	}
	return tst, r
}

// TestToastPlacement 钉住 toast 的位置：右上角（原版 fixed top-4 right-4 +
// viewport p-4），文案右缘离窗口右边 16 + 内边距 16 = 32。
func TestToastPlacement(t *testing.T) {
	_, r := toastProbe(t, "success", "投币成功")
	t.Logf("toast 文案 rect = %+v", r)
	if right := 1000 - (r.X + r.W); right < 20 || right > 40 {
		t.Errorf("文案右缘距窗口右边 %v，应在 20..40（16 视口 + 16 内边距）", right)
	}
	if r.Y < 8 || r.Y > 40 {
		t.Errorf("文案上缘 y = %v，应贴着窗口顶部（视口 16 + 内边距 8）", r.Y)
	}
}

// TestToastTypeIconPixels 无头渲染实测：toast 左侧真的画出了类型图标，且颜色
// 按类型取 —— error 红叉、success 绿勾（原版 utils/toast.ts 的 fill）。
func TestToastTypeIconPixels(t *testing.T) {
	cases := []struct {
		typ    string
		dom    int
		name   string
		other  int
		others string
	}{
		{"error", 0, "红", 1, "绿"},
		{"success", 1, "绿", 0, "红"},
	}
	for _, tc := range cases {
		tst, r := toastProbe(t, tc.typ, "操作完成")
		x0, x1 := int(r.X)-30, int(r.X)-6
		y0, y1 := int(r.Y), int(r.Y)+int(r.H)
		got := countDominant(tst.Image(), x0, y0, x1, y1, tc.dom)
		wrong := countDominant(tst.Image(), x0, y0, x1, y1, tc.other)
		t.Logf("%s：%s像素 %d，%s像素 %d（区域 %d,%d..%d,%d）",
			tc.typ, tc.name, got, tc.others, wrong, x0, y0, x1, y1)
		if got < 20 {
			t.Errorf("%s toast 文案左侧没有%s色图标像素：%d", tc.typ, tc.name, got)
		}
		if wrong > 2 {
			t.Errorf("%s toast 的图标带了%s色像素：%d（颜色取错了）", tc.typ, tc.others, wrong)
		}
	}
}

// countDark 数一块区域里足够暗的像素（正文色 #1e293b）。
func countDark(img *image.RGBA, x0, y0, x1, y1 int) int {
	n := 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			if avg := 0.299*float64(img.Pix[i]) + 0.587*float64(img.Pix[i+1]) + 0.114*float64(img.Pix[i+2]); avg < 90 {
				n++
			}
		}
	}
	return n
}

// TestToastFadesIn 无头渲染实测：新 toast 是被 ElementTransition 由上方 8px
// 淡入的 —— 刚入队的首帧几乎全透明（几乎数不到暗色正文像素），随后连续帧的像素
// 必须逐渐变化（入场在跑）。去掉 Transition 的 Enter 后首帧就完全显形，本测试
// 按预期变红。
//
// 入场是墙钟驱动的：`Tester.settle` 会在一次 `Frame()` 里连着跑到没有新帧为止
// （最多 20 帧），宿主内存回收/调度抖动偶尔会让这几帧的墙钟耗时 >300ms，固定
// 300ms 的入场就被当成「已淡入」，首帧采样假红（历史上全量测试时多次偶发）。
// 这里把 toastEnterDuration 临时放长到 10s（沿用 TestVolumePopoverEnterWired
// 的同一手法）：一次 settle 的墙钟远小于它，首帧稳定停在 Enter 的 opacity 0
// 起点，断言不再抖。
func TestToastFadesIn(t *testing.T) {
	origDur, origTr := toastEnterDuration, toastTransition
	toastEnterDuration = 10 * time.Second
	t.Cleanup(func() { toastEnterDuration, toastTransition = origDur, origTr })

	build := func() (*ui.Tester, ui.Rect) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.Frame() // viewport 先建出来，toast 才算「在已存在的父级里出现」（Enter 生效）
		a.NotifyType("error", "登录失败")
		tst.Frame() // 入场首帧
		r, ok := tst.Find("登录失败")
		if !ok {
			t.Fatalf("找不到刚入队的 toast 文案：%v", tst.Texts())
		}
		return tst, r
	}
	box := func(r ui.Rect) (int, int, int, int) {
		return int(r.X), int(r.Y), int(r.X) + int(r.W), int(r.Y) + int(r.H)
	}

	tst, r := build()
	x0, y0, x1, y1 := box(r)
	born := countDark(tst.Image(), x0, y0, x1, y1)
	// 再走两帧：入场还没跑完（10s），像素必须仍有变化。v0.5.5 冻结时钟，
	// 两帧之间用 Advance 推进（否则时钟不动、画面逐像素相同）。
	tst.Advance(100 * time.Millisecond)
	tst.Frame()
	f2 := tst.Image()
	tst.Advance(100 * time.Millisecond)
	tst.Frame()
	f3 := tst.Image()
	diff := 0
	for i := range f2.Pix {
		if f2.Pix[i] != f3.Pix[i] {
			diff++
		}
	}
	t.Logf("入场首帧暗色像素 %d；随后的连续两帧差异 %d", born, diff)
	if born > 2 {
		t.Errorf("入场首帧就有 %d 个暗色正文像素，应在 Enter 的 opacity 0 起点（几乎全透明）", born)
	}
	if diff == 0 {
		t.Error("入场首帧之后的连续两帧逐像素相同：淡入过渡没在跑")
	}

	// 反证：去掉 Enter 后，同一位置首帧就完全显形。
	toastTransition = func() ui.ElementTransition {
		return ui.ElementTransition{Duration: toastEnterDuration,
			Exit: &ui.Motion{Y: -8, Opacity: 0}}
	}
	tst2, r2 := build()
	bx0, by0, bx1, by1 := box(r2)
	if solid := countDark(tst2.Image(), bx0, by0, bx1, by1); solid <= 2 {
		t.Errorf("去掉 Enter 后首帧仍只有 %d 个暗色像素，反证不成立", solid)
	}
}

func avgLuma(img *image.RGBA, x0, y0, x1, y1 int) float64 {
	var sum float64
	var n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			sum += 0.299*float64(img.Pix[i]) + 0.587*float64(img.Pix[i+1]) + 0.114*float64(img.Pix[i+2])
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// TestToastGradientPixels 无头渲染实测：135° 类型渐变真的盖在玻璃上 ——
// 左上角（白 .5）比右下角（类型色 8%，如 error 的玫红）亮。去掉那层
// LinearGradient（退回匀色玻璃）后两侧亮度会拉平，本测试按预期变红。
func TestToastGradientPixels(t *testing.T) {
	tst, r := toastProbe(t, "error", "投币失败")
	// toast 整体：文案左缘往左 8(间距) + 20(图标) + 16(内边距) = 44，
	// 右缘往右 16(内边距)，上缘往上 8、下缘往下 8。
	tx0, ty0 := int(r.X)-44, int(r.Y)-8
	tx1, ty1 := int(r.X)+int(r.W)+16, int(r.Y)+int(r.H)+8
	// 避开玻璃的 1px 白边（rim），四角各取 6×6 内缩 3px。
	tl := avgLuma(tst.Image(), tx0+3, ty0+3, tx0+9, ty0+9)
	br := avgLuma(tst.Image(), tx1-9, ty1-9, tx1-3, ty1-3)
	t.Logf("toast %d,%d..%d,%d 左上亮度 %.1f / 右下亮度 %.1f", tx0, ty0, tx1, ty1, tl, br)
	if tl-br < 5 {
		t.Errorf("左上 %.1f 未明显亮于右下 %.1f（135° 白→类型色渐变没画出来）", tl, br)
	}
}

// toastPanelRect 由文案矩形反推 .toast-glass 整块：左 16(内边距) + 20(图标) +
// 8(间距) = 44，上下各 8，右 16。
func toastPanelRect(r ui.Rect) ui.Rect {
	return ui.Rect{X: r.X - 44, Y: r.Y - 8, W: r.W + 60, H: r.H + 16}
}

// TestToastSheen 钉住 `.toast-glass::after`（globals.css:5345）的
// `--glass-sheen-static` 洗光：
//
//   - 结构：渲染一条 toast 时，`playerSheenPaint` 必须被调用一次，且矩形正好是
//     toast 整块、圆角正好是 13（说明洗光建在 toast 的 Absolute 子层上，而不是
//     漏画或画到别处）；
//   - 像素：开/关洗光相减，面板左上角必须真的变亮（toast 底接近纯白，白 .24 的
//     热点在 luma 上只有 1 上下，阈值按实测取 0.3；右下角径向 48% / 线性 60%
//     已归零，必须几乎不变）。
//
// 反证：删掉最后一层 Absolute 洗光，`playerSheenPaint` 在 toast 矩形上一次都
// 不会被调到（结构断言红），左上角差也归零（像素断言红）。
func TestToastSheen(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	a.NotifyType("info", "已复制链接")
	tst.Frame()
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	r, ok := tst.Find("已复制链接")
	if !ok {
		t.Fatalf("找不到 toast 文案：%v", tst.Texts())
	}
	panel := toastPanelRect(r)

	// 结构：换桩记录每一次洗光绘制的（矩形、圆角）。
	type sheenCall struct {
		r      ui.Rect
		radius float32
	}
	var calls []sheenCall
	saved := playerSheenPaint
	playerSheenPaint = func(p *ui.Painter, rr ui.Rect, radius float32, _ *ui.Bitmap) {
		calls = append(calls, sheenCall{rr, radius})
	}
	tst.Frame()
	playerSheenPaint = saved
	hit := false
	for _, c := range calls {
		if c.radius == toastRadius &&
			math.Abs(float64(c.r.W-panel.W)) < 1 && math.Abs(float64(c.r.H-panel.H)) < 1 {
			hit = true
		}
	}
	if !hit {
		t.Errorf("toast 面板 %+v 上没有画 `--glass-sheen-static` 洗光（调用 %d 次：%+v）",
			panel, len(calls), calls)
	}

	// 像素：开/关洗光的逐像素差。先按真实绘制再跑一帧（上一帧是记录桩，
	// 画面里没有洗光）。
	tst.Frame()
	on := tst.Image()
	playerSheenPaint = func(*ui.Painter, ui.Rect, float32, *ui.Bitmap) {}
	tst.Frame()
	off := tst.Image()
	playerSheenPaint = saved
	// 左上采样取「径向热点所在、且避开图标（x 偏移 16..36、y 25..45）」的横带：
	// x 偏移 20..50、y 偏移 2..7。圆角 13 让 x 偏移 3..13 那一角落在圆角外，
	// 采到的是背景，不能用来量洗光。
	top := panelRegion{int(panel.X) + 20, int(panel.Y) + 2, int(panel.X) + 50, int(panel.Y) + 7}
	bottom := panelRegion{
		int(panel.X+panel.W) - 13, int(panel.Y+panel.H) - 11,
		int(panel.X+panel.W) - 3, int(panel.Y+panel.H) - 3,
	}
	dTop := meanDiff(t, on, off, top)
	dBottom := meanDiff(t, on, off, bottom)
	t.Logf("toast 面板 %+v 洗光开/关 平均差：左上 %.2f / 右下 %.2f", panel, dTop, dBottom)
	if dTop < 0.3 {
		t.Errorf("toast 左上角只差 %.2f —— 洗光画了但没落到面板上", dTop)
	}
	if dBottom > 0.5 {
		t.Errorf("toast 右下角也差 %.2f —— 洗光不该到底部还亮", dBottom)
	}
}

// TestToastSpecularInsets 实测 `.toast-glass` 的 `--glass-specular` 顶缘内高光
// （inset 0 1px 1px .5px 白 .9）：关掉洗光后，面板顶行仍应比内部亮。
//
// 反证：去掉 toastSpecularInsets 的接线，顶行与内部亮度差归零。
func TestToastSpecularInsets(t *testing.T) {
	tst, r := toastProbe(t, "success", "已收藏")
	panel := toastPanelRect(r)
	saved := playerSheenPaint
	playerSheenPaint = func(*ui.Painter, ui.Rect, float32, *ui.Bitmap) {}
	tst.Frame()
	img := tst.Image()
	playerSheenPaint = saved
	// 避开圆角（半径 13）与图标 / 文案：取面板中段上方的横带。
	// `inset 0 1px 1px` 的高光落在最顶那一行，第二行是模糊衰减。
	x0, x1 := int(panel.X)+50, int(panel.X)+80
	row := func(y0, y1 int) float64 {
		return avgLuma(img, x0, y0, x1, y1)
	}
	topRow := row(int(panel.Y), int(panel.Y)+1)
	inner := row(int(panel.Y)+4, int(panel.Y)+8)
	t.Logf("toast 面板 %+v 顶行亮度 %.1f / 内部 %.1f", panel, topRow, inner)
	if topRow-inner < 3 {
		t.Errorf("顶行 %.1f 未明显亮于内部 %.1f —— --glass-specular 顶缘内高光没接上",
			topRow, inner)
	}
}
