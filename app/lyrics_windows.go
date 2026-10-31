//go:build windows && (amd64 || arm64)

package main

import (
	"syscall"
	"unsafe"
)

// 桌面字幕窗在 Windows 上去掉 DWM 画的那圈外框。
//
// 背景透明度调到 0（纯文字浮在桌面上）后，窗口矩形上仍有一圈细边框和一点
// 投影感，来源不是我们画的内容，而是 DWM：
//
//   - Win11 给**每个顶层窗口**画 1px 边框（DWMWA_BORDER_COLOR，默认取系统
//     的窗口描边色）；
//   - mygo 对 Panel 请求了「圆角」偏好（DwmSetWindowAttribute
//     DWMWA_WINDOW_CORNER_PREFERENCE = round，见 internal/windows/flyout.go
//     的 roundFlyout），DWM 于是按圆角裁剪窗口矩形，并沿这个圆角画边框 ——
//     我们画的是半径 14 的半透明面板，裁剪边缘与面板边缘并不重合，多出来
//     的那一圈就是用户看到的「边框」。
//   - DWM 的系统投影按窗口矩形投，不看内容 alpha（SetHasShadow(false) 关掉
//     的是 mygo 通过 DwmExtendFrameIntoClientArea 打开的那层，Win11 的边框
//     仍在）。
//
// 处理：边框色设成 DWMWA_COLOR_NONE（不画），圆角偏好设成 DoNotRound（不裁
// 剪）—— 圆角由我们自己的内容负责，窗口保持矩形透明，四角露出桌面。
//
// mygo 没有这两项的公开接口，只能从 NativeHandle() 拿到 HWND 直接调 DWM。
// 这是 app 里唯一一处绕过框架调 Win32 的地方（另一处是任务栏缩略图工具栏，
// 那本来就没有框架接口）；Win10 上这两个属性不存在，调用失败忽略即可。

const (
	dwmwaWindowCornerPreference = 33
	dwmwaBorderColor            = 34

	// DWMWA_COLOR_NONE：不画边框（注意不是「用强调色」，也不是 0）。
	dwmColorNone = 0xFFFFFFFE
	// DWM_WINDOW_CORNER_PREFERENCE：DoNotRound，不让 DWM 裁圆角。
	dwmCornerDoNotRound = 1
)

var (
	lyricsDwmapi                    = syscall.NewLazyDLL("dwmapi.dll")
	lyricsProcDwmSetWindowAttribute = lyricsDwmapi.NewProc("DwmSetWindowAttribute")
)

// lyricsTuneWindowFrame 去掉桌面字幕窗的 DWM 边框与圆角裁剪。
// hwnd 为 0（窗口还没建好）或属性不受支持（Win10）时静默跳过。
func lyricsTuneWindowFrame(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	dwmSetU32(hwnd, dwmwaBorderColor, dwmColorNone)
	dwmSetU32(hwnd, dwmwaWindowCornerPreference, dwmCornerDoNotRound)
}

func dwmSetU32(hwnd uintptr, attr uint32, value uint32) {
	if err := lyricsProcDwmSetWindowAttribute.Find(); err != nil {
		// Win10 的 dwmapi 没有这两个属性（函数本身在），属性不识别时
		// DwmSetWindowAttribute 返回 E_INVALIDARG；两种情况都只是没效果。
		return
	}
	lyricsProcDwmSetWindowAttribute.Call(hwnd, uintptr(attr),
		uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}
