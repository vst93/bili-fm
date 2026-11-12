//go:build windows && (amd64 || arm64)

package main

import (
	"syscall"
	"unsafe"
)

// 本文件调整 Windows 给顶层窗口画的那圈外框与圆角。
//
// 用户 2026-11-11：「窗口最外层有一圈深色的边」，深色模式尤其明显。它不是我
// 们画的内容 —— 无头渲染逐点采过主窗 / 迷你窗 / 字幕窗的四条边，alpha 与内侧
// 连续，没有多出来的线。来源是 DWM：
//
//   - Win11 给每个顶层窗口画 1px 边框（DWMWA_BORDER_COLOR，默认取系统的窗口
//     描边色，深浅随主题走）；
//   - DWM 的系统投影按窗口矩形投，不看内容 alpha（`DisableShadow` 关掉的是
//     mygo 经 DwmExtendFrameIntoClientArea 打开的那层，Win11 的边框仍在）。
//
// 背景不透明的应用可以不管（内容盖住那 1px）；**透明窗不行** —— 那一圈正好
// 压在内容半透明的边缘上，深色主题下像给窗口描了一道黑边。
//
// 但**圆角不能一起去掉**（用户 2026-11-11 第二轮：「整个窗口的四周圆角都没有
// 生效，现在是完全的直角」）。第一版把 DWMWA_WINDOW_CORNER_PREFERENCE 也设成
// DoNotRound，那是给桌面字幕窗写的 —— 字幕窗的内容自己画 14px 圆角面板，
// 不要 DWM 再裁一遍。主窗 / 迷你窗不一样：它们的内容是铺满整窗的渐变背景，
// **圆角本来就得由 DWM 裁**（mygo 对 Frameless 窗只在 `CornerRadius > 0` 时
// 才请求圆角，我们没给那个选项，靠的正是 Win11 给无边框窗的默认圆角）。
//
// 所以拆成两个入口：
//
//	tuneWindowFrame(hwnd)   只去边框，保留 DWM 圆角（主窗 / 迷你窗）
//	flatWindowFrame(hwnd)   再去圆角（桌面字幕窗：内容自己画圆角面板）
//
// mygo 没有这两项的公开接口，只能从 NativeHandle() 拿到 HWND 直接调 DWM。
// Win10 上这两个属性不存在，调用失败忽略即可。

const (
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	// DWMWA_COLOR_NONE：不画边框（注意不是「用强调色」，也不是 0）。
	dwmColorNone = 0xFFFFFFFE
	// DWM_WINDOW_CORNER_PREFERENCE：Default 让 DWM 按自己的规则给圆角
	// （Win11 给无边框窗默认圆角）；DoNotRound 是不裁。
	dwmCornerDefault    = 0
	dwmCornerDoNotRound = 1
)

var (
	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// tuneWindowFrame 去掉一个窗口的 DWM 1px 边框，**保留** DWM 圆角。
//
// 主窗与迷你窗用它：内容铺满整窗，圆角要靠 DWM 裁。hwnd 为 0（窗口还没建好）
// 或属性不受支持（Win10）时静默跳过。
func tuneWindowFrame(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	dwmSetU32(hwnd, dwmwaBorderColor, dwmColorNone)
	dwmSetU32(hwnd, dwmwaWindowCornerPreference, dwmCornerDefault)
}

// flatWindowFrame 再去掉 DWM 的圆角裁剪：内容自己画圆角面板。
//
// 桌面字幕窗用它 —— 面板是半径 14 的半透明圆角矩形，DWM 若按自己的圆角裁剪
// 窗口矩形并沿它画边框，与内容圆角并不重合，多出来的一圈就是「边框」。
func flatWindowFrame(hwnd uintptr) {
	tuneWindowFrame(hwnd)
	if hwnd == 0 {
		return
	}
	dwmSetU32(hwnd, dwmwaWindowCornerPreference, dwmCornerDoNotRound)
}

func dwmSetU32(hwnd uintptr, attr uint32, value uint32) {
	if err := procDwmSetWindowAttribute.Find(); err != nil {
		// Win10 的 dwmapi 没有这两个属性（函数本身在），属性不识别时
		// DwmSetWindowAttribute 返回 E_INVALIDARG；两种情况都只是没效果。
		return
	}
	procDwmSetWindowAttribute.Call(hwnd, uintptr(attr),
		uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
