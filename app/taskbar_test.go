package main

import (
	"testing"
	"unsafe"
)

// Windows 任务栏缩略图工具栏里与平台无关那一半的测试。syscall/COM 那一半
// （taskbar_windows.go）只能在 Windows 上编译，无法在 CI 宿主上运行，所以
// 这里把能确定的东西钉死：THUMBBUTTON 的内存布局、字形几何、光栅化输出、
// 按钮装填、主题判据。原版：main:src-tauri/src/taskbar.rs。

// TestTaskbarThumbButtonLayout 钉住 THUMBBUTTON 的字段偏移与大小 ——
// ThumbBarAddButtons 会直接按这段内存读，布局错了按钮就是乱码或崩溃。
func TestTaskbarThumbButtonLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("只在 64 位宿主上校验 THUMBBUTTON 布局")
	}
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		{"dwMask", unsafe.Offsetof(taskbarThumbButton{}.dwMask), 0},
		{"iId", unsafe.Offsetof(taskbarThumbButton{}.iId), 4},
		{"iBitmap", unsafe.Offsetof(taskbarThumbButton{}.iBitmap), 8},
		{"hIcon", unsafe.Offsetof(taskbarThumbButton{}.hIcon), 16},
		{"dwFlags", unsafe.Offsetof(taskbarThumbButton{}.dwFlags), 24},
		{"szTip", unsafe.Offsetof(taskbarThumbButton{}.szTip), 28},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s 偏移 = %d，应为 %d", c.name, c.got, c.want)
		}
	}
	if got := unsafe.Sizeof(taskbarThumbButton{}); got != taskbarThumbButtonSize {
		t.Errorf("THUMBBUTTON 大小 = %d，应为 %d", got, taskbarThumbButtonSize)
	}
}

// TestTaskbarGlyphGeometry 钉住四个字形的形状（原版 taskbar.rs 的 inside()）。
// prev = |◀（竖条 + 左指三角）、play = ▶、pause = ‖、next = ▶|。
func TestTaskbarGlyphGeometry(t *testing.T) {
	cases := []struct {
		name   string
		g      taskbarGlyph
		inside [][2]float64
		out    [][2]float64
	}{
		{"play", taskbarGlyphPlay,
			[][2]float64{{0.35, 0.50}, {0.60, 0.50}},
			[][2]float64{{0.10, 0.10}, {0.90, 0.50}, {0.50, 0.05}}},
		{"pause", taskbarGlyphPause,
			[][2]float64{{0.38, 0.50}, {0.62, 0.50}},
			[][2]float64{{0.50, 0.50}, {0.20, 0.50}, {0.80, 0.50}}},
		{"prev", taskbarGlyphPrev,
			[][2]float64{{0.26, 0.50}, {0.55, 0.50}},
			[][2]float64{{0.10, 0.50}, {0.90, 0.50}, {0.50, 0.05}}},
		{"next", taskbarGlyphNext,
			[][2]float64{{0.74, 0.50}, {0.45, 0.50}},
			[][2]float64{{0.90, 0.50}, {0.10, 0.50}, {0.50, 0.05}}},
	}
	for _, c := range cases {
		for _, p := range c.inside {
			if !taskbarGlyphInside(c.g, p[0], p[1]) {
				t.Errorf("%s: (%v,%v) 应落在字形内", c.name, p[0], p[1])
			}
		}
		for _, p := range c.out {
			if taskbarGlyphInside(c.g, p[0], p[1]) {
				t.Errorf("%s: (%v,%v) 不该落在字形内", c.name, p[0], p[1])
			}
		}
	}
}

// TestRasterTaskbarGlyph 钉住光栅化：尺寸、BGRA 通道、alpha 超采样覆盖率，
// 以及深色主题用白（255）、浅色用近黑（32）。
func TestRasterTaskbarGlyph(t *testing.T) {
	const size = 16
	for _, dark := range []bool{false, true} {
		ch := byte(32)
		if dark {
			ch = 255
		}
		for _, g := range []taskbarGlyph{taskbarGlyphPrev, taskbarGlyphPlay, taskbarGlyphPause, taskbarGlyphNext} {
			px := rasterTaskbarGlyph(size, g, dark)
			if len(px) != size*size*4 {
				t.Fatalf("dark=%v glyph=%d: 像素字节数 = %d，应为 %d", dark, g, len(px), size*size*4)
			}
			var opaque, clear, partial int
			for i := 0; i < len(px); i += 4 {
				if px[i] != ch || px[i+1] != ch || px[i+2] != ch {
					t.Fatalf("dark=%v glyph=%d: 偏移 %d 的 RGB = (%d,%d,%d)，应为 %d",
						dark, g, i, px[i], px[i+1], px[i+2], ch)
				}
				switch a := px[i+3]; {
				case a == 0:
					clear++
				case a == 255:
					opaque++
				default:
					partial++
				}
			}
			cov := float64(opaque+partial) / float64(size*size)
			if clear == 0 {
				t.Errorf("glyph=%d: 没有透明像素（图标铺满整张画布？）", g)
			}
			if cov < 0.10 || cov > 0.35 {
				t.Errorf("glyph=%d: 覆盖率 %.3f 超出 [0.10,0.35]（原版四个图标约 15%%~18%%）", g, cov)
			}
			// 超采样应当产生半透明边缘像素（硬采样只有 0/255）。
			if partial == 0 {
				t.Errorf("glyph=%d: 没有半透明边缘像素，超采样没生效", g)
			}
		}
	}
}

// TestTaskbarMakeButton 钉住按钮装填：mask、id、图标、禁用位、tooltip 的
// UTF-16 NUL 结尾裁剪。
func TestTaskbarMakeButton(t *testing.T) {
	const icon = 0xdead_beef
	b := taskbarMakeButton(taskbarBtnPlay, icon, "暂停", true)
	if b.dwMask != taskbarThbIcon|taskbarThbTooltip|taskbarThbFlags {
		t.Errorf("dwMask = %#x，应为 icon|tooltip|flags", b.dwMask)
	}
	if b.iId != taskbarBtnPlay || b.hIcon != icon || b.iBitmap != 0 {
		t.Errorf("id/icon/bitmap = %d/%#x/%d", b.iId, b.hIcon, b.iBitmap)
	}
	if b.dwFlags != taskbarThbEnabled {
		t.Errorf("启用位 = %#x，应为 0", b.dwFlags)
	}
	if b.szTip[0] != '暂' || b.szTip[1] != '停' || b.szTip[2] != 0 {
		t.Errorf("szTip = %v，应为「暂停」+ NUL", b.szTip[:4])
	}
	disabled := taskbarMakeButton(taskbarBtnNext, 1, "下一首", false)
	if disabled.dwFlags != taskbarThbDisabled {
		t.Errorf("禁用位 = %#x，应为 1", disabled.dwFlags)
	}
	// 超长 tooltip 必须截断且保持 NUL 结尾（szTip 只有 260 个 WCHAR）。
	long := make([]rune, 400)
	for i := range long {
		long[i] = 'x'
	}
	tb := taskbarMakeButton(1, 1, string(long), true)
	if tb.szTip[len(tb.szTip)-1] != 0 {
		t.Error("超长 tooltip 没有 NUL 结尾")
	}
}

// TestTaskbarThemeIsDark 钉住注册表值的映射：1 = 浅色（不是深色）、0 = 深色，
// 读不到按浅色（避免白底白图标）。
func TestTaskbarThemeIsDark(t *testing.T) {
	if taskbarThemeIsDark(1, true) {
		t.Error("AppsUseLightTheme=1 是浅色，不该判为深色")
	}
	if !taskbarThemeIsDark(0, true) {
		t.Error("AppsUseLightTheme=0 是深色")
	}
	if taskbarThemeIsDark(0, false) {
		t.Error("读不到时应按浅色处理")
	}
}
