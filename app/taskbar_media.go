package main

// 这个文件是 Windows 任务栏缩略图工具栏（ThumbBarAddButtons）里**与平台无关**
// 的那一半：图标字形、光栅化、THUMBBUTTON 的内存布局。刻意不加 build tag，
// 让几何与结构体布局在任何宿主机上都能单测（CI 在 Linux 上跑测试，Windows
// 的 syscall 那一半只能交叉编译）。
//
// 原版实现：main:src-tauri/src/taskbar.rs（Tauri 版）。Tauri 在 Windows 上
// 悬停任务栏图标时，预览浮层里有 上一首 / 播放暂停 / 下一首 三个按钮；
// 原生版此前完全没有这块（mygo 只把 ITaskbarList3 用来画进度）。

// taskbarGlyph 是四个任务栏图标之一。
type taskbarGlyph int

const (
	taskbarGlyphPrev taskbarGlyph = iota
	taskbarGlyphPlay
	taskbarGlyphPause
	taskbarGlyphNext
)

// taskbarThumbButton 逐字段对应 Win32 的 THUMBBUTTON（ShObjIdl_core.h）：
//
//	typedef struct _THUMBBUTTON {
//	    THUMBBUTTONMASK dwMask;   // UINT
//	    UINT iId;
//	    UINT iBitmap;
//	    HICON hIcon;              // 指针，按 8 字节对齐
//	    THUMBBUTTONFLAGS dwFlags; // UINT
//	    WCHAR szTip[260];
//	} THUMBBUTTON;
//
// COM 调用直接按这段内存读取，字段顺序 / 对齐必须逐字节一致；布局由
// taskbar_media_test.go 钉住。
type taskbarThumbButton struct {
	dwMask  uint32
	iId     uint32
	iBitmap uint32
	hIcon   uintptr
	dwFlags uint32
	szTip   [260]uint16
}

// THUMBBUTTONMASK / THUMBBUTTONFLAGS（ShObjIdl_core.h）。
const (
	taskbarThbBitmap  = 0x1
	taskbarThbIcon    = 0x2
	taskbarThbTooltip = 0x4
	taskbarThbFlags   = 0x8

	taskbarThbEnabled  = 0x0
	taskbarThbDisabled = 0x1
)

// 任务栏按钮 id（原版 BTN_PREV / BTN_PLAY / BTN_NEXT）。
const (
	taskbarBtnPrev = 1
	taskbarBtnPlay = 2
	taskbarBtnNext = 3
)

// taskbarThumbButtonSize 是 THUMBBUTTON 的字节大小（amd64 / arm64）：
// 3×UINT(12) + 4 字节对齐填充 + HICON(8) + UINT(4) + szTip(520) = 548，
// 再按 8 字节对齐 = 552。
const taskbarThumbButtonSize = 552

// taskbarSupersample 是字形的超采样倍率：16px 下按像素中心硬采样锯齿明显，
// 4×4 后边缘像素的 alpha 是覆盖率，缩到 16px 仍干净。
const taskbarSupersample = 4

// taskbarGlyphInside 报告归一化坐标 (u,v)（都以 [0,1] 向右/向下）是否落在
// 字形内部。几何逐值照抄原版 taskbar.rs 的 inside()。
func taskbarGlyphInside(g taskbarGlyph, u, v float64) bool {
	switch g {
	case taskbarGlyphPlay:
		return taskbarInTriangle(u, v, 0.30, 0.20, 0.30, 0.80, 0.78, 0.50)
	case taskbarGlyphPause:
		return taskbarInRect(u, v, 0.32, 0.20, 0.44, 0.80) ||
			taskbarInRect(u, v, 0.56, 0.20, 0.68, 0.80)
	case taskbarGlyphPrev:
		return taskbarInRect(u, v, 0.22, 0.20, 0.30, 0.80) ||
			taskbarInTriangle(u, v, 0.80, 0.20, 0.80, 0.80, 0.34, 0.50)
	case taskbarGlyphNext:
		return taskbarInRect(u, v, 0.70, 0.20, 0.78, 0.80) ||
			taskbarInTriangle(u, v, 0.20, 0.20, 0.20, 0.80, 0.66, 0.50)
	}
	return false
}

func taskbarInRect(u, v, x0, y0, x1, y1 float64) bool {
	return u >= x0 && u <= x1 && v >= y0 && v <= y1
}

// taskbarInTriangle 是三角形命中测试：三条边的叉积同号即在内（含边界）。
func taskbarInTriangle(u, v, ax, ay, bx, by, cx, cy float64) bool {
	cross := func(px, py, qx, qy float64) float64 {
		return (u-px)*(qy-py) - (v-py)*(qx-px)
	}
	d1 := cross(ax, ay, bx, by)
	d2 := cross(bx, by, cx, cy)
	d3 := cross(cx, cy, ax, ay)
	hasNeg := d1 < 0 || d2 < 0 || d3 < 0
	hasPos := d1 > 0 || d2 > 0 || d3 > 0
	return !(hasNeg && hasPos)
}

// rasterTaskbarGlyph 把字形光栅化成 size×size 的 32bpp BGRA 像素（直通
// alpha），4×4 超采样。深色主题用白图标、浅色主题用近黑图标（原版
// channel: 255 / 32），保证在任务栏预览浮层的底色上都看得见。
func rasterTaskbarGlyph(size int, g taskbarGlyph, dark bool) []byte {
	if size <= 0 {
		return nil
	}
	px := make([]byte, size*size*4)
	ch := byte(32)
	if dark {
		ch = 255
	}
	ss := taskbarSupersample
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			hits := 0
			for sy := 0; sy < ss; sy++ {
				for sx := 0; sx < ss; sx++ {
					u := (float64(x) + (float64(sx)+0.5)/float64(ss)) / float64(size)
					v := (float64(y) + (float64(sy)+0.5)/float64(ss)) / float64(size)
					if taskbarGlyphInside(g, u, v) {
						hits++
					}
				}
			}
			alpha := byte(hits * 255 / (ss * ss))
			o := (y*size + x) * 4
			px[o], px[o+1], px[o+2], px[o+3] = ch, ch, ch, alpha // BGRA
		}
	}
	return px
}

// taskbarMakeButton 拼装一个 THUMBBUTTON。szTip 以 NUL 结尾（零值数组已清空，
// 这里只覆盖实际文本）。
func taskbarMakeButton(id uint32, icon uintptr, tip string, enabled bool) taskbarThumbButton {
	b := taskbarThumbButton{
		dwMask: taskbarThbIcon | taskbarThbTooltip | taskbarThbFlags,
		iId:    id,
		hIcon:  icon,
	}
	if !enabled {
		b.dwFlags = taskbarThbDisabled
	}
	for i, r := range []rune(tip) {
		if i >= len(b.szTip)-1 {
			break
		}
		b.szTip[i] = uint16(r)
	}
	return b
}

// taskbarThemeIsDark 把注册表 AppsUseLightTheme 的值映射成「系统是否深色」：
// 1 = 浅色，0 = 深色。读不到（键缺失/无权限）时按浅色处理，避免白底白图标。
func taskbarThemeIsDark(value uint32, ok bool) bool {
	return ok && value == 0
}
