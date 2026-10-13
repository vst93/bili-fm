package view

import (
	"image"
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
		{"success", "#22c55e", iconCheck},
		{"error", "#f43f5e", iconClose},
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
	tr := toastTransition()
	if tr.Duration != 300*time.Millisecond {
		t.Errorf("时长 = %v，应为 300ms", tr.Duration)
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
	time.Sleep(400 * time.Millisecond) // 等 300ms 入场结束
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
// 淡入的 —— 刚入队的首帧几乎全透明（几乎数不到暗色正文像素），等过 300ms 入场
// 结束后正文清晰可辨。去掉 Transition 后首帧就完全显形，本测试按预期变红。
//
// 入场是墙钟驱动的：首帧的采样点落在 Frame 内部连续跑完的若干帧之后，宿主
// 内存回收/调度抖动偶尔会让这几帧耗时 >225ms，首帧就被当成「已淡入」。
// 这不是产品缺陷（无头也拿不到 mygo 的测试时钟），所以重试几次，任意一次
// 采到「首帧明显比入场结束淡」即通过；真去掉 Transition 每次都会同样失败。
func TestToastFadesIn(t *testing.T) {
	const attempts = 3
	var born, settled int
	var measured bool
	for i := 0; i < attempts; i++ {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.Frame()
		a.NotifyType("error", "登录失败")
		tst.Frame()
		r, ok := tst.Find("登录失败")
		if !ok {
			t.Fatalf("找不到刚入队的 toast 文案：%v", tst.Texts())
		}
		x0, x1 := int(r.X), int(r.X)+int(r.W)
		y0, y1 := int(r.Y), int(r.Y)+int(r.H)
		born = countDark(tst.Image(), x0, y0, x1, y1)
		time.Sleep(400 * time.Millisecond)
		tst.Frame()
		settled = countDark(tst.Image(), x0, y0, x1, y1)
		measured = true
		if born <= settled/4 {
			break
		}
	}
	t.Logf("首帧暗色像素 %d，入场结束后 %d", born, settled)
	if !measured {
		t.Fatal("没有采到任何一帧")
	}
	if born > settled/4 {
		t.Errorf("首帧就有 %d 个暗色像素（入场结束后 %d）——toast 没有淡入", born, settled)
	}
	if settled < 20 {
		t.Errorf("入场结束后正文仍只有 %d 个暗色像素，没显出来", settled)
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
