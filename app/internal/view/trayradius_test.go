package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 窗口控制按钮组（最小化 / 关闭）的**跨窗口一致性**：主窗与迷你窗必须是同一套
// 样式，只有尺寸不同。
//
// 用户 2026-11-11 在 Linux 上实测「主窗那颗托盘的左下角 / 右下角没有圆角，而
// Windows 下明明有」，并追问「难道 Linux 跟 Windows 的自定义菜单还是分别写的
// 一套吗？」—— 答案是：**样式实现本来就是同一套**（`titleBarCtrlTray` 铺托盘、
// `windowControlButtonSized` 画键），但圆角常量按平台分了叉：
//
//	tbTrayRadius()    Windows 8 / 其它 0     ← 原版 --app-window-radius
//	miniTrayRadius()  Windows 8 / 其它 0
//
// 那个「Linux 0」是从原版 WebView 的 `body.platform-linux #root
// { --app-window-radius: 0 }` 抄来的，语义是「窗口由 GTK 装饰、内容层画圆角会
// 与窗口方角错位」。原生版不是这个模型：Linux 是 Frameless 透明窗，GTK 不画
// 装饰（mygo 的 useCSD 排除透明窗），内容铺满整窗、形状由内容自己负责。所以
// 「Linux 没有圆角」是照抄原版留下的错误分叉，不是设计。
//
// 现在三平台统一走一个 trayRadius()，下面几条把它钉住。

// TestTrayRadiusSameOnAllPlatforms 钉住托盘圆角不再按平台分叉。
//
// 平台分叉在无头测试里量不出来（runtime.GOOS 改不了），所以这条从**源码**
// 层面钉：`trayRadius` 里不许再出现按 GOOS 分支，也不许再出现第二个托盘圆角
// 常量（两套值正是用户看到「Linux 没有、Windows 有」的原因）。
func TestTrayRadiusSameOnAllPlatforms(t *testing.T) {
	src, err := os.ReadFile("theme.go")
	if err != nil {
		t.Fatalf("读 theme.go 失败：%v", err)
	}
	// 取 trayRadius 声明到下一个空行为止的那一小段。
	start := strings.Index(string(src), "var trayRadius = func() float32 {")
	if start < 0 {
		t.Fatal("theme.go 里找不到 trayRadius 声明")
	}
	end := strings.Index(string(src)[start:], "\n\n")
	if end < 0 {
		end = len(src) - start
	}
	block := string(src)[start : start+end]
	if strings.Contains(block, "viewGOOS") || strings.Contains(block, "runtime.GOOS") {
		t.Errorf("trayRadius 又按平台分叉了 —— 主窗与迷你窗的窗控按钮组必须是同一套样式：\n%s", block)
	}
	if got := trayRadius(); got != 8 {
		t.Errorf("trayRadius() = %v，应为 8（Win11 给无边框窗的默认圆角；Linux 内容层自己圆 8）", got)
	}

	// 不许再出现第二个托盘圆角常量。
	for _, banned := range []string{"miniTrayRadius", "tbTrayRadius"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("theme.go 里仍有 %q：托盘圆角只该有 trayRadius() 一个来源", banned)
		}
	}
}

// TestWindowControlTrayShared 钉住「主窗与迷你窗调用的是同一个托盘函数」。
//
// 这正是用户问的那件事：如果哪天有人给迷你窗再写一套托盘（而不是改尺寸），
// 两边的圆角 / 边框 / 内凹阴影就会再次漂移。
func TestWindowControlTrayShared(t *testing.T) {
	for _, file := range []string{"shell.go", "mini.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", file, err)
		}
		s := string(src)
		if !strings.Contains(s, "titleBarCtrlTray(wc, trayRadius())") {
			t.Errorf("%s 没有用 titleBarCtrlTray(wc, trayRadius()) —— 窗控按钮组的托盘必须主窗/迷你窗共用", file)
		}
	}
}

// TestTrayBottomCornersRounded 是无头渲染实测：托盘的**下面两个角**是圆的，
// 上面两个角是方的（原版 `border-radius: 0 0 r r`，托盘是从窗顶挂下来的凹槽）。
//
// 用换桩的纯色托盘（红底）铺在已知底色上，沿托盘左右两列从上到下扫：
// 下沿那几个像素若被圆角裁掉，露出的就是底色（不是红），据此判定角是否圆。
func TestTrayBottomCornersRounded(t *testing.T) {
	oldBg, oldRadius := tbTrayBg, trayRadius
	tbTrayBg = ui.Hex("#ff0000")
	trayRadius = func() float32 { return 8 }
	t.Cleanup(func() { tbTrayBg, trayRadius = oldBg, oldRadius })

	tst, _, minRect, _ := renderTitleBarProbe(t)
	img := tst.Image()

	// 托盘左下角的像素：矩形左下角那一列的最下一行，圆角会把这块裁掉。
	// 取「角上 1px 处」与「沿左边界往上 8px 处」比较：前者应几乎不是红色，
	// 后者应是托盘底色（红）。
	corner := img.RGBAAt(int(minRect.X)-tbWinCtrlPadX, int(minRect.Y+minRect.H)+tbWinCtrlPadY-1)
	inside := img.RGBAAt(int(minRect.X)-tbWinCtrlPadX, int(minRect.Y+minRect.H)-2)
	redness := func(c interface{ RGBA() (uint32, uint32, uint32, uint32) }) float64 {
		r, g, b, _ := c.RGBA()
		return float64(r>>8) - (float64(g>>8)+float64(b>>8))/2
	}
	t.Logf("托盘左下角像素=%v 内侧像素=%v（红度 %.1f vs %.1f）", corner, inside, redness(corner), redness(inside))
	if redness(inside) <= 20 {
		t.Fatalf("托盘内侧不是红色（%v）—— 换桩没生效，这条测不到圆角", inside)
	}
	if redness(corner) >= redness(inside)-10 {
		t.Errorf("托盘左下角像素 %v 与内侧 %v 一样红 —— 角没有被裁圆（trayRadius 没生效）", corner, inside)
	}
}
