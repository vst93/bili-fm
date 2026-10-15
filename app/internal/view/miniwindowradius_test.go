package view

import (
	"os"
	"strings"
	"testing"
)

// absInt 是 int 的绝对值。
func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// 本轮（迷你窗对照 main 原版）⑤ 的评估测试：迷你窗 #root 的 12px 窗口圆角。
//
// 结论：**放弃**（不修）。原版 `body.mini-mode #root { --app-window-radius: 12px;
// border-radius: var(--app-window-radius) }` 是 WebView **内容层**的圆角 ——
// 窗口本身由系统的无边框窗口（Windows/Linux）/ 原生红绿灯（macOS，L4743-4746
// 把变量覆盖成 0）负责。mygo 这一支走的是另一条路径：
//
//	mygo.WindowOptions（v0.5.1 window.go）没有窗口圆角选项，Window 方法表
//	    也没有圆角接口；内部只有 flyout 用了
//	    `DwmSetWindowAttribute(DWMWA_WINDOW_CORNER_PREFERENCE = 33)` 请求**系统
//	    默认**圆角（Win11 ≈8px，不是精确 12px，且 Win10 没有）；Linux GTK 没有
//	    公开的圆角控制；macOS 原生窗口圆角由系统给（原版那边也覆盖成 0）。
//	内容层圆角（给 Root 加一张 12px 圆角 Clip）只能裁**内容**，OS 窗口本身仍是
//	    方角 —— 四角会露出一块不透明的窗口底色，比不裁更难看，它也不能解决
//	    「OS 的圆角是否精确 12px」。
//
// 所以 (c) 放弃：保留窗口交给系统的那点圆角（与原版 macOS 那条覆盖同效 ——
// 系统默认，而非 12px），mygo 的渲染模型下**没有**与原版 12px WebView 圆角
// 对应的东西。下面把「当前没有内容层圆角」这个现状钉住，免得以后有人误加。

// TestMiniContentHasNoRoundedWindowClip 钉住迷你窗内容层**没有** 12px 圆角：
// 四个角都是实心像素（窗口底色一直画到边），而不是被圆角裁掉的透明/缺口。
//
// 一旦有人给 mini 内容层加上圆角 Clip，这条会失败 —— 那正是本测试要拦的
// 「用内容圆角假装窗口圆角」的做法（OS 窗口仍是方角，四角会露底色）。
func TestMiniContentHasNoRoundedWindowClip(t *testing.T) {
	tst := miniProbe(t, false)
	img := tst.Image()
	if img.Rect.Dx() != miniWidth || img.Rect.Dy() != miniHeight {
		t.Fatalf("画布尺寸 = %v，应为 %dx%d", img.Rect, miniWidth, miniHeight)
	}
	// 每个角像素都该与「往里 6px」的像素接近（同一块背景渐变）。若给内容层加了
	// 12px 圆角 Clip，角上会露出一块**不是背景**的窗口底色 —— 这个差值会突然拉大。
	for _, c := range [][4]int{{0, 0, 6, 6}, {miniWidth - 1, 0, miniWidth - 7, 6},
		{0, miniHeight - 1, 6, miniHeight - 7}, {miniWidth - 1, miniHeight - 1, miniWidth - 7, miniHeight - 7}} {
		a := img.RGBAAt(c[0], c[1])
		b := img.RGBAAt(c[2], c[3])
		if a.A != 255 {
			t.Fatalf("迷你窗角 (%d,%d) 不是实心像素（alpha=%d）", c[0], c[1], a.A)
		}
		for _, d := range []int{absInt(int(a.R) - int(b.R)), absInt(int(a.G) - int(b.G)), absInt(int(a.B) - int(b.B))} {
			if d > 40 {
				t.Errorf("迷你窗角 (%d,%d)=%v 与内侧 (%d,%d)=%v 差 %d —— 角上像有一块被裁出来的缺口",
					c[0], c[1], a, c[2], c[3], b, d)
				break
			}
		}
	}
}

// TestMiniWindowRadiusHasNoNativeAPI 钉住「原生窗口没有精确圆角接口」这个评估结论：
// 改动链里不能出现 `Radius` 出现在窗口（mygo.Window）上，也不能出现
// `SetWindowRoundedCorner` / `DwmSetWindowAttribute` 这类绕过框架调用的尝试 ——
// app 只依赖 mygo 的公开方法表（它没有圆角接口）。改用内容层圆角会在
// TestMiniContentHasNoRoundedWindowClip 里失败。
func TestMiniWindowRadiusHasNoNativeAPI(t *testing.T) {
	body, err := os.ReadFile("../../main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	src := strings.Join(strings.Fields(string(body)), "")
	if strings.Contains(src, "Win.SetCornerRadius") || strings.Contains(src, "Win.Radius") {
		t.Error("main.go 里出现了窗口圆角调用 —— mygo.WindowOptions / Window 方法表都没有这一项")
	}
}
