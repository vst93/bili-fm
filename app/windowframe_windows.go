//go:build windows && (amd64 || arm64)

package main

import (
	"syscall"
	"unsafe"
)

// 本文件去掉 Windows 给**每个顶层窗口**画的那圈外框。
//
// 用户 2026-11-11：「窗口最外层有一圈深色的边」，深色模式尤其明显。它不是我
// 们画的内容 —— 无头渲染逐点采过主窗 / 迷你窗 / 字幕窗的四条边，alpha 与内侧
// 连续，没有多出来的线。来源是 DWM：
//
//   - Win11 给每个顶层窗口画 1px 边框（DWMWA_BORDER_COLOR，默认取系统的窗口
//     描边色，深浅随主题走）；
//   - 窗口是 `Frameless`，Windows 11 对无边框窗仍会按自己的圆角（
//     DWMWA_WINDOW_CORNER_PREFERENCE）裁剪窗口矩形，并沿这个圆角画边框 ——
//     我们画的内容圆角与它并不重合，多出来的那一圈就是用户看到的「深色边」。
//   - DWM 的系统投影按窗口矩形投，不看内容 alpha（`DisableShadow` 关掉的是
//     mygo 经 DwmExtendFrameIntoClientArea 打开的那层，Win11 的边框仍在）。
//
// 背景不透明的应用可以不管（内容盖住那 1px）；**透明窗不行** —— 那一圈正好
// 压在内容半透明的边缘上，深色主题下像给窗口描了一道黑边。
//
// 处理：边框色设成 DWMWA_COLOR_NONE（不画），圆角偏好设成 DoNotRound（不裁
// 剪）—— 圆角由我们自己的内容负责，窗口保持矩形透明，四角露出桌面。
//
// mygo 没有这两项的公开接口，只能从 NativeHandle() 拿到 HWND 直接调 DWM。
// Win10 上这两个属性不存在，调用失败忽略即可。

const (
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	// DWMWA_COLOR_NONE：不画边框（注意不是「用强调色」，也不是 0）。
	dwmColorNone = 0xFFFFFFFE
	// DWM_WINDOW_CORNER_PREFERENCE：DoNotRound，不让 DWM 裁圆角。
	dwmCornerDoNotRound = 1
)

var (
	dwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	procDwmSetWindowAttribute = dwmapi.NewProc("DwmSetWindowAttribute")
)

// tuneWindowFrame 去掉一个窗口的 DWM 边框与圆角裁剪：内容自己负责圆角。
// hwnd 为 0（窗口还没建好）或属性不受支持（Win10）时静默跳过。
//
// 所有**透明**窗口都要调（主窗、迷你窗、桌面字幕窗）：不透明窗的内容会把
// DWM 那 1px 盖住，调了也没有副作用，所以判据只用「窗口建好没有」。
func tuneWindowFrame(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	dwmSetU32(hwnd, dwmwaBorderColor, dwmColorNone)
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
