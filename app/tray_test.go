package main

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/egoist/mygo"
)

// decodePNG 解一张图标；解不开直接 Fatal（资源必须永远是合法 PNG）。
func decodePNG(t *testing.T, name string, data []byte) image.Image {
	t.Helper()
	if len(data) == 0 {
		t.Fatalf("%s 是空文件", name)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("%s 不是合法 PNG: %v", name, err)
	}
	return img
}

// alphaStats 返回 (透明像素, 不透明像素, 半透明像素) 以及不透明像素的包围盒
// 占画布的比例（宽、高）。用来验证图标「既非空白、也非一整块实心」。
func alphaStats(img image.Image) (clear, opaque, partial int, bw, bh float64) {
	b := img.Bounds()
	minX, minY, maxX, maxY := b.Max.X, b.Max.Y, b.Min.X-1, b.Min.Y-1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()
			switch {
			case a == 0:
				clear++
			case a >= 0xffff:
				opaque++
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			default:
				partial++
			}
		}
	}
	if maxX < minX {
		return
	}
	bw = float64(maxX-minX+1) / float64(b.Dx())
	bh = float64(maxY-minY+1) / float64(b.Dy())
	return
}

// TestTrayIconFor 钉住「按平台选图标」：macOS 用模板图 + IconIsTemplate=true；
// Windows / Linux 用彩色图 + false。回归点：曾经在 macOS 上把彩色图当模板发
// 出去（IconIsTemplate 恒 true），模板只看 alpha，彩色图几乎整块不透明，于是
// 被渲染成一个实心方块 —— 台标消失，用户看到「空白」。
func TestTrayIconFor(t *testing.T) {
	data, tmpl := trayIconFor("darwin")
	if !tmpl {
		t.Error("macOS 应把图标当模板图（IconIsTemplate=true）")
	}
	if !bytes.Equal(data, trayIconTemplate) {
		t.Error("macOS 应选 tray-template.png（黑+透明的模板图）")
	}
	for _, goos := range []string{"windows", "linux", "freebsd"} {
		data, tmpl := trayIconFor(goos)
		if tmpl {
			t.Errorf("%s 不应把图标当模板图（系统不认）", goos)
		}
		if !bytes.Equal(data, trayIconColored) {
			t.Errorf("%s 应选彩色 tray.png", goos)
		}
	}
}

// TestTrayTemplateIsUsable 验证 macOS 模板图本身：尺寸合理、格式正确、
// alpha 同时有透明和不透明像素（不是空白也不是实心块）、发出去的像素是黑的
// （模板约定 black + alpha）。
func TestTrayTemplateIsUsable(t *testing.T) {
	img := decodePNG(t, "tray-template.png", trayIconTemplate)
	b := img.Bounds()
	if b.Dx() != 64 || b.Dy() != 64 {
		t.Errorf("模板图 = %dx%d，应为 64x64（菜单栏高度 18pt 下的 2x/高分辨率源）",
			b.Dx(), b.Dy())
	}
	clear, opaque, partial, bw, bh := alphaStats(img)
	t.Logf("模板图：透明=%d 不透明=%d 半透明=%d 不透明包围盒=%.0f%%×%.0f%%",
		clear, opaque, partial, bw*100, bh*100)
	if clear == 0 {
		t.Error("模板图没有透明像素：整张不透明 = macOS 下渲染成实心块（空白）")
	}
	if opaque == 0 {
		t.Error("模板图没有不透明像素：logo 是空的")
	}
	// 台标要足够大（用户报「偏小」）。原彩色图台标只占约 37% 宽，这里要求
	// 明显更大。
	if bw < 0.85 || bh < 0.6 {
		t.Errorf("模板台标包围盒 %.0f%%×%.0f%%，太小（应 ≥85%%×60%%）", bw*100, bh*100)
	}
	// 不透明占比要在合理区间：太低是没内容，太高会退化成实心块。
	frac := float64(opaque) / float64(b.Dx()*b.Dy())
	if frac < 0.2 || frac > 0.75 {
		t.Errorf("模板不透明像素占比 %.2f，超出 [0.20,0.75]（太低=空白/太小，太高=实心块）", frac)
	}
	// 发出去的像素必须是黑的（模板图只看 alpha，但约定如此；也防止以后误把
	// 彩色图改名当模板图）。
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if r != 0 || g != 0 || bl != 0 {
				t.Fatalf("模板图在 (%d,%d) 不是黑色：rgba=(%d,%d,%d)，模板图应为 black+alpha",
					x, y, r>>8, g>>8, bl>>8)
			}
		}
	}
}

// TestTrayRetainedOnController 钉住「托盘被 controller 强引用」这条不变量。
//
// 用户报「Linux 下托盘菜单的控制按钮点了没反应」，根因不在应用逻辑：mygo 的
// 菜单点击派发用 weak.Pointer（menu.go 的 menuItems.byID），而 MenuItem 只被
// Menu 强引用、Menu 只被 *Tray 强引用。macOS / Windows 的原生托盘把 mygo 的
// TrayHandler 存了下来（handler 反向持有 *Tray），链子不断；Linux（AppIndicator）
// 的 tray 结构体根本没有这个字段，于是 *Tray 变成孤儿，随时可被 GC 回收——回收后
// lookupMenuItem 返回 nil，托盘菜单的所有项（含播放/暂停、上一集、下一集）都
// 静默失效。修复=controller 持有 *Tray。
//
// 这里用反射 + 源码链两条断言，任一条被删都会红：
//  1. controller 有 tray 字段且类型为 *mygo.Tray；
//  2. setupTray 真的把 NewTray 的返回值写进它。
func TestTrayRetainedOnController(t *testing.T) {
	ft, ok := reflect.TypeOf(controller{}).FieldByName("tray")
	if !ok {
		t.Fatal("controller 没有 tray 字段：Linux 托盘会因 GC 回收而点击失效")
	}
	if ft.Type != reflect.TypeOf((*mygo.Tray)(nil)) {
		t.Errorf("controller.tray 类型 = %v，应为 *mygo.Tray", ft.Type)
	}

	flat := flattenSrc(t, "tray.go")
	if !strings.Contains(flat, "c.tray=tray") {
		t.Error("setupTray 没有把 NewTray 的返回值记到 c.tray（托盘会被 GC 回收）")
	}
}

// flattenSrc 读一份源码并压掉所有空白，便于逐字匹配源码链。
func flattenSrc(t *testing.T, path string) string {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", path, err)
	}
	return strings.Join(strings.Fields(string(src)), "")
}

// TestTrayColoredIconIsNotTemplate 记一笔「为什么彩色图不能当模板」：它的 alpha
// 几乎铺满整张画布（白底圆角方块），在 macOS 模板渲染下就是一个实心块。
func TestTrayColoredIconIsNotTemplate(t *testing.T) {
	img := decodePNG(t, "tray.png", trayIconColored)
	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Errorf("彩色 tray.png = %dx%d，应为 32x32", b.Dx(), b.Dy())
	}
	_, opaque, partial, _, _ := alphaStats(img)
	frac := float64(opaque+partial) / float64(b.Dx()*b.Dy())
	t.Logf("彩色 tray.png 非透明（alpha>0）占比 %.2f", frac)
	if frac < 0.7 {
		t.Errorf("彩色 tray.png 非透明占比只有 %.2f，与「不能用模板」的前提不符", frac)
	}
}

// TestTrayItemsAllRouteThroughTrayAction 钉住「托盘菜单的每一项都经 trayAction
// 派发」这条约定。
//
// 背景：Linux 的托盘是 AppIndicator/DBusMenu，点击由 GTK 的 "activate" 信号
// 派发（internal/linux/menu.go 的 cbMenuActivate），Windows/macOS 的原生托盘
// 各自在自己的消息循环里回调 —— 回调线程**不保证**是 mygo 的事件循环线程。
// trayAction 是唯一的兜底（主线程上零开销直调、否则投递并等待）并留下
// 「托盘菜单：xxx」日志，便于把「回调没进来」与「动作没生效」分开。
//
// 此前「检查更新…」「退出 bili-FM」两项直接调动作，是这条约定唯一的例外
// （用户报的「Linux 托盘控制按钮点击无效果」正是这类不一致的表现）。反证：
// 把任一项改回直接调 → 本测试红。
func TestTrayItemsAllRouteThroughTrayAction(t *testing.T) {
	src, err := os.ReadFile("tray.go")
	if err != nil {
		t.Fatalf("读 tray.go 失败：%v", err)
	}
	flat := string(src)
	for _, name := range []string{"show", "togglePlay", "prev", "next", "checkUpdate", "quit"} {
		if want := `c.trayAction("` + name + `"`; !strings.Contains(flat, want) {
			t.Errorf("托盘项 %q 没有经 trayAction 派发（应有 %s）", name, want)
		}
	}
	for _, bad := range []string{"{ c.menuCheckUpdate() }", "{ c.quit() }"} {
		if strings.Contains(flat, bad) {
			t.Errorf("托盘项绕过 trayAction 直接调动作：%s", bad)
		}
	}
}

// TestTrayLeftClickShowsWindow 钉住「左键点托盘 logo 显示主窗口」这条路径。
//
// 之前 setupTray 只传了 Menu、从没注册 OnClick：Windows 左键回调 Clicked
// 有来，但没有任何监听者，用户点logo完全没反应（右键才能开菜单）。
// macOS 更糟：常驻菜单会把左键直接吃掉。现在菜单按平台分流：
// Windows/macOS 左键 OnClick → showWindow，右键临时挂菜单；Linux 只能
// 常驻菜单（AppIndicator 不给点击事件），第一项仍是「显示主窗口」。
//
// 反证：删掉 tray.OnClick 的接线 → 本测试红。
func TestTrayLeftClickShowsWindow(t *testing.T) {
	flat := flattenSrc(t, "tray.go")
	if !strings.Contains(flat, "tray.OnClick(func(){c.trayAction(\"show\",c.showWindow)})") {
		t.Error("托盘左键没有接到 showWindow（Windows/macOS 点 logo 不会显示窗口）")
	}
	if !strings.Contains(flat, "tray.OnRightClick") {
		t.Error("托盘右键没有接弹出菜单")
	}
	if !strings.Contains(flat, `runtime.GOOS=="linux"`) || !strings.Contains(flat, "tray.SetMenu(menu)") {
		t.Error("Linux 没有常驻菜单（AppIndicator 只能通过菜单恢复窗口）")
	}
	// 窗口已显示时再 Show 是幂等的：双击两次不能把窗口又藏起来。
	if strings.Contains(flat, "func(c*controller)showWindow(){") &&
		strings.Contains(flat, "w.Hide()") {
		t.Error("showWindow 里不应有 Hide（双击第二次会把窗口藏掉）")
	}
}

// TestTrayColoredIconFillsCanvas 钉住 Windows/Linux 托盘图标的尺寸与底色：
// 用户先报「logo 偏小」，改成青色底后报「背景色不对、还是小一圈」。现在与
// Tauri 版一致：白色圆角底（不是青/透明块）+ 放大的蓝色台标（宽约 26/32），
// 不再缩在小白块里。
func TestTrayColoredIconFillsCanvas(t *testing.T) {
	img := decodePNG(t, "tray.png", trayIconColored)
	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("彩色 tray.png = %dx%d，应为 32x32", b.Dx(), b.Dy())
	}
	clear, _, _, bw, bh := alphaStats(img)
	if clear > b.Dx()*b.Dy()/10 {
		t.Fatalf("托盘图标透明像素 %d 太多（圆角只该占少量）", clear)
	}
	if bw < 0.9 || bh < 0.9 {
		t.Fatalf("非透明内容包围盒 %.0f%%×%.0f%%，应铺满画布（≥90%%）", bw*100, bh*100)
	}
	// 台标本体（蓝色）要够大：旧图只有 14×12，缩到 16px 后约 7×6。
	blueMinX, blueMinY, blueMaxX, blueMaxY := -1, -1, -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, bl, a := img.At(x, y).RGBA()
			if a < 0x8000 || !(bl>>8 > (r>>8)+30 && bl>>8 > 150) {
				continue
			}
			if blueMinX < 0 || x < blueMinX {
				blueMinX = x
			}
			if blueMinY < 0 || y < blueMinY {
				blueMinY = y
			}
			if x > blueMaxX {
				blueMaxX = x
			}
			if y > blueMaxY {
				blueMaxY = y
			}
		}
	}
	if blueMaxX < 0 {
		t.Fatal("托盘图标没有蓝色台标像素")
	}
	bw = float64(blueMaxX-blueMinX+1) / float64(b.Dx())
	bh = float64(blueMaxY-blueMinY+1) / float64(b.Dy())
	if bw < 0.75 || bh < 0.6 {
		t.Fatalf("蓝色台标包围盒 %.0f%%×%.0f%%，太小（应 ≥75%%×60%%）", bw*100, bh*100)
	}
	// 底色必须是白色（用户报「背景色不对」）：取圆角方块内、台标上方的一点。
	r, g, bl, a := img.At(28, 3).RGBA()
	if a < 0x8000 || r>>8 < 240 || g>>8 < 240 || bl>>8 < 240 {
		t.Fatalf("托盘底色 = rgba(%d,%d,%d,%d)，应为白色", r>>8, g>>8, bl>>8, a>>8)
	}
}

// TestAppIconFillsCanvas 钉住任务栏 / 应用图标：旧 icon.png 的白底只有 84%
// 画布、蓝台标只有 44%，任务栏上显得比别的应用小一圈。新图白底铺满 512，
// 蓝台标放到约 76% 宽。
func TestAppIconFillsCanvas(t *testing.T) {
	data, err := os.ReadFile("resources/icon.png")
	if err != nil {
		t.Fatalf("读 resources/icon.png 失败：%v", err)
	}
	img := decodePNG(t, "icon.png", data)
	b := img.Bounds()
	if b.Dx() != 512 || b.Dy() != 512 {
		t.Fatalf("应用图标 = %dx%d，应为 512x512", b.Dx(), b.Dy())
	}
	blueMinX, blueMinY, blueMaxX, blueMaxY := -1, -1, -1, -1
	alphaMinX, alphaMinY, alphaMaxX, alphaMaxY := -1, -1, -1, -1
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, bl, a := img.At(x, y).RGBA()
			if a >= 0x8000 {
				if alphaMinX < 0 || x < alphaMinX {
					alphaMinX = x
				}
				if alphaMinY < 0 || y < alphaMinY {
					alphaMinY = y
				}
				if x > alphaMaxX {
					alphaMaxX = x
				}
				if y > alphaMaxY {
					alphaMaxY = y
				}
			}
			if a < 0x8000 || !(bl>>8 > (r>>8)+30 && bl>>8 > 150) {
				continue
			}
			if blueMinX < 0 || x < blueMinX {
				blueMinX = x
			}
			if blueMinY < 0 || y < blueMinY {
				blueMinY = y
			}
			if x > blueMaxX {
				blueMaxX = x
			}
			if y > blueMaxY {
				blueMaxY = y
			}
		}
	}
	if blueMaxX < 0 {
		t.Fatal("应用图标没有蓝色台标像素")
	}
	abw := float64(alphaMaxX-alphaMinX+1) / float64(b.Dx())
	abh := float64(alphaMaxY-alphaMinY+1) / float64(b.Dy())
	if abw < 0.9 || abh < 0.9 {
		t.Fatalf("应用图标非透明包围盒 %.0f%%×%.0f%%，应铺满画布（≥90%%）", abw*100, abh*100)
	}
	bw := float64(blueMaxX-blueMinX+1) / float64(b.Dx())
	bh := float64(blueMaxY-blueMinY+1) / float64(b.Dy())
	if bw < 0.7 || bh < 0.6 {
		t.Fatalf("应用图标蓝台标包围盒 %.0f%%×%.0f%%，太小（应 ≥70%%×60%%）", bw*100, bh*100)
	}
	r, g, bl, a := img.At(256, 20).RGBA()
	if a < 0x8000 || r>>8 < 240 || g>>8 < 240 || bl>>8 < 240 {
		t.Fatalf("任务栏图标底色 = rgba(%d,%d,%d,%d)，应为白色", r>>8, g>>8, bl>>8, a>>8)
	}
}
