package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（标题栏窗口键组）：原版 `#wds-handle-group` 的**内凹玻璃托盘**此前整层没画。
//
// 原版（main:src/styles/globals.css:1913-1922）：
//
//	#wds-handle-group {
//	    background: rgba(255, 255, 255, 0.06);
//	    border: 1px solid rgba(255, 255, 255, 0.18);
//	    border-top: 0;
//	    border-radius: 0 0 var(--app-window-radius) var(--app-window-radius);
//	    padding: 2px 4px !important;
//	    box-shadow:
//	        inset 0 2px 4px rgba(15, 23, 42, 0.08),
//	        inset 0 -1px 0 rgba(255, 255, 255, 0.12);
//	}
//
// 注释写明「no top highlight line」：托盘是从窗顶挂下来的**凹槽**，不是一颗浮起
// 的玻璃键 —— 上边没有边框、上面两个角不圆。此前原生版只对齐了组内几何
// （gap-1 / px-1，见 2026-11-04 那轮），这层托盘本身缺失，于是 Windows / Linux
// 上两颗窗口键直接压在标题栏白玻璃上，比原版少一层「压进表面」的深度。
//
// 下面四条把「几何 / 逐边逐角 / 内凹阴影 / 两处调用点」钉住。

// trayProbe 用换桩后的托盘色渲染一次标题栏，返回取样函数与两颗键的矩形。
//
// 换桩的四个颜色都取「通道可分」的纯色（红=底、绿=边框、品红=下沿内影、
// 蓝=上沿内影），这样每个像素到底画的是哪一层一眼可判，不必和标题栏自身的
// 垂直渐变（顶 224 → 底 234）抢那几级灰阶。
func trayProbe(t *testing.T) (*ui.Tester, func(x, y int) (int, int, int), ui.Rect, ui.Rect) {
	t.Helper()
	oldBg, oldBorder, oldInk, oldRim, oldRadius := tbTrayBg, tbTrayBorder, tbTrayInk, tbTrayRim, tbTrayRadius
	tbTrayBg = ui.Hex("#ff0000")
	tbTrayBorder = ui.Hex("#00ff00")
	tbTrayInk = ui.Hex("#0000ff").Alpha(0.5)
	tbTrayRim = ui.Hex("#ff00ff")
	tbTrayRadius = func() float32 { return 8 }
	t.Cleanup(func() {
		tbTrayBg, tbTrayBorder, tbTrayInk, tbTrayRim, tbTrayRadius = oldBg, oldBorder, oldInk, oldRim, oldRadius
	})
	return renderTitleBarProbe(t)
}

// renderTitleBarProbe 渲染一次标题栏并按当前（可能是换桩后的）令牌取样。
func renderTitleBarProbe(t *testing.T) (*ui.Tester, func(x, y int) (int, int, int), ui.Rect, ui.Rect) {
	t.Helper()
	tst := titleBarProbeAt(t, 1)
	min, ok := tst.Find("最小化")
	if !ok {
		t.Fatalf("标题栏里找不到「最小化」键：%v", tst.Texts())
	}
	cl, ok := tst.Find("隐藏窗口")
	if !ok {
		t.Fatalf("标题栏里找不到「隐藏窗口」键：%v", tst.Texts())
	}
	img := tst.Image()
	px := func(x, y int) (int, int, int) {
		i := img.PixOffset(x, y)
		return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
	}
	return tst, px, min, cl
}

// TestTitleBarCtrlTrayGeometry 钉住托盘的盒模型：`padding: 2px 4px` + 下/左/右
// 各 1px 边框、上边无边框（box-sizing: border-box 下边框算在组宽之内）。
//
// 期望值写死成原版 CSS 的数字（不引用布局常量），否则「常量改错、断言跟着改错」
// 会互相掩护。
func TestTitleBarCtrlTrayGeometry(t *testing.T) {
	_, _, min, cl := trayProbe(t)
	// 两颗键 30×30、上下各 2px 内边距、下面 1px 边框 ⇒ 组高 35；36px 的栏里
	// 居中后组顶 = 0.5，所以键顶 = 0.5 + 2 = 2.5（原版同为 2.5，不是 3 ——
	// py-0.5 把键整体往下推了 2，居中又抵消 2.5）。
	if min.Y != 2.5 {
		t.Errorf("最小化键顶 %v，应为 2.5（栏 36 − 组 35 = 1，居中 0.5 + py-0.5 2）", min.Y)
	}
	// 组右缘 = 窗口右 − 栏内边距 16；组宽 = 2×30 + gap 4 + 2×px-1 8 + 2×边框 1 = 74。
	if right := float32(1000) - (cl.X + cl.W + 1 + 4); right != 16 {
		t.Errorf("托盘右缘距窗口右边 %v，应为 16（栏 padding）", right)
	}
	// 托盘左缘：最小化键左侧 = 4（px-1）+ 1（边框）。
	if left := min.X - 5; left != 910 {
		t.Errorf("托盘左缘 %v，应为 910（984 − 74）", left)
	}
}

// TestTitleBarCtrlTrayPaint 逐边逐角确认托盘真的画出来了：左/右/下三条 1px
// 边框（上边**没有**）、下沿 1px 白色内影，以及「只有下面两个角圆」。
//
// 量法是**差分**：把某一层换成不透明纯色、其余层置透明，与该层也置透明的那次
// 渲染逐点比。两个原因让绝对色值不可用：① 标题栏顶部那两层绝对定位的贴边渐变
//（原版 `::before` / `::after`，z-index 0）**画在内容之上**，托盘与键都被它洗过
// 一层；② 整组还乘着 `.app-title-bar > *` 的静息不透明度 0.32。所以纯白只贡献
// ~6 灰阶（0.32 × 23），阈值按这个量级定 —— 层在不在，差的是「6 对 0」。
func TestTitleBarCtrlTrayPaint(t *testing.T) {
	type stub struct {
		bg, border, ink, rim ui.Color
		radius               float32
	}
	render := func(st stub) func(x, y int) (int, int, int) {
		old := [5]any{tbTrayBg, tbTrayBorder, tbTrayInk, tbTrayRim, tbTrayRadius}
		tbTrayBg, tbTrayBorder, tbTrayInk, tbTrayRim = st.bg, st.border, st.ink, st.rim
		tbTrayRadius = func() float32 { return st.radius }
		t.Cleanup(func() {
			tbTrayBg, tbTrayBorder, tbTrayInk, tbTrayRim = old[0].(ui.Color), old[1].(ui.Color), old[2].(ui.Color), old[3].(ui.Color)
			tbTrayRadius = old[4].(func() float32)
		})
		_, px, _, _ := renderTitleBarProbe(t)
		return px
	}
	luma := func(r, g, b int) float64 { return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b) }
	none := stub{ui.Transparent, ui.Transparent, ui.Transparent, ui.Transparent, 8}
	base := render(none)
	diff := func(px func(x, y int) (int, int, int), x, y int) float64 {
		r0, g0, b0 := base(x, y)
		r1, g1, b1 := px(x, y)
		return luma(r1, g1, b1) - luma(r0, g0, b0)
	}

	// 几何取自第一次渲染（stub 不影响盒模型）。
	_, _, min, cl := renderTitleBarProbe(t)
	trayL := int(min.X) - 5
	trayR := int(cl.X+cl.W) + 4
	// 组底那一行是 1px 下边框：键底 32.5 + py-0.5 2 = 34.5（边框画在高度之外）
	// → 行 34；下沿内影在它上面一行。
	trayB := int(min.Y + min.H + tbWinCtrlPadY)
	gapX := int(min.X + min.W + 2)

	// 左 / 右 / 下三条 1px 边框。
	borderOnly := render(stub{ui.Transparent, ui.Hex("#ffffff"), ui.Transparent, ui.Transparent, 8})
	if d := diff(borderOnly, trayL, 20); d < 4 {
		t.Errorf("托盘左边框只带来 %.1f 灰阶，应 ≥4（1px rgba(255,255,255,.18)，静息 32%% 不透明度下约 6）", d)
	}
	if d := diff(borderOnly, trayR, 20); d < 4 {
		t.Errorf("托盘右边框只带来 %.1f 灰阶，应 ≥4", d)
	}
	if d := diff(borderOnly, gapX, trayB); d < 4 {
		t.Errorf("托盘下边框（行 %d）只带来 %.1f 灰阶，应 ≥4", trayB, d)
	}
	// 上边**没有**边框（border-top: 0）。
	if d := diff(borderOnly, gapX, 1); d > 1 || d < -1 {
		t.Errorf("托盘上沿出现了边框（%.1f 灰阶）—— 原版 border-top: 0（凹槽从窗顶挂下来）", d)
	}
	// 面：两颗键之间的空隙（4px 宽）铺托盘底色。
	if d := diff(render(stub{ui.Hex("#ffffff"), ui.Transparent, ui.Transparent, ui.Transparent, 8}), gapX, 20); d < 4 {
		t.Errorf("托盘底色只带来 %.1f 灰阶，应 ≥4（rgba(255,255,255,.06) 的白洗）", d)
	}
	// 下沿那条 `inset 0 -1px 0 rgba(255,255,255,.12)`。
	if d := diff(render(stub{ui.Transparent, ui.Transparent, ui.Transparent, ui.Hex("#ffffff"), 8}), gapX, trayB-1); d < 3 {
		t.Errorf("托盘下沿内影只带来 %.1f 灰阶，应 ≥3（inset 0 -1px 0 白 12%%）", d)
	}
	// 只有下面两个角圆：左下角被圆弧切掉（边框贡献 ≈0），左上角是直边。
	if d := diff(borderOnly, trayL, trayB-1); d > 2 {
		t.Errorf("左下角仍有边框（%.1f 灰阶）—— 下沿圆角没生效（原版 0 0 R R）", d)
	}
	if d := diff(borderOnly, trayL, 1); d < 4 {
		t.Errorf("左上角没有边框（%.1f 灰阶）—— 原版上面两个角不圆（radius 0 0 R R）", d)
	}
}

// TestTitleBarCtrlTrayInsetShadow 用**真实**颜色量那条上沿内凹阴影
// （`inset 0 2px 4px rgba(15,23,42,.08)`）：托盘顶部头几行必须比「关掉这条
// 内影」时暗，而托盘中段必须不变。
//
// 反证：把 tbTrayInk 换成透明后第一条断言的差值掉到 0 → 报红。
func TestTitleBarCtrlTrayInsetShadow(t *testing.T) {
	luma := func(r, g, b int) float64 { return 0.2126*float64(r) + 0.7152*float64(g) + 0.0722*float64(b) }
	sample := func() (top, mid float64) {
		_, px, min, _ := renderTitleBarProbe(t)
		gapX := int(min.X + min.W + 2)
		r, g, b := px(gapX, 1)
		top = luma(r, g, b)
		r, g, b = px(gapX, 20)
		mid = luma(r, g, b)
		return top, mid
	}
	withShadowTop, withShadowMid := sample()
	oldInk := tbTrayInk
	tbTrayInk = ui.Transparent
	plainTop, plainMid := sample()
	tbTrayInk = oldInk

	if d := plainTop - withShadowTop; d < 2 {
		t.Errorf("上沿内影只让首行暗了 %.1f 灰阶（有影 %.1f / 无影 %.1f），应 ≥2", d, withShadowTop, plainTop)
	}
	if d := plainMid - withShadowMid; d > 1 || d < -1 {
		t.Errorf("托盘中段被内影影响了 %.1f 灰阶（有影 %.1f / 无影 %.1f），应为 0（阴影只贴上下沿）", d, withShadowMid, plainMid)
	}
}

// TestCtrlTrayWired 钉住两处调用点都走同一个 titleBarCtrlTray：主窗标题栏
// （shell.go）与迷你标题栏（mini.go）。迷你模式原版只改 padding 与键尺寸，
// 托盘本身照旧（globals.css:4774 的覆写只动 padding）。
func TestCtrlTrayWired(t *testing.T) {
	for _, f := range []string{"shell.go", "mini.go"} {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", f, err)
		}
		if !strings.Contains(string(src), "titleBarCtrlTray(") {
			t.Errorf("%s 没有调用 titleBarCtrlTray —— 窗口键组缺原版 #wds-handle-group 的内凹托盘", f)
		}
	}
}
