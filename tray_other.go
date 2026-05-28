//go:build !windows

package main

import "fmt"

// checkSingleInstanceWindows 非 Windows 平台的 stub
func checkSingleInstanceWindows() (bool, uintptr) {
	return true, 0
}

// closeMutex 非 Windows 平台的 stub
func closeMutex(handle uintptr) {}

// findExistingWindow 非 Windows 平台的 stub
func findExistingWindow() uintptr {
	return 0
}

// restoreExistingWindow 非 Windows 平台的 stub
func restoreExistingWindow(hwnd uintptr) {}

// initTrayWindows 非 Windows 平台的 stub
func initTrayWindows(showFn func(), exitFn func()) {
	fmt.Println("System tray not implemented for this platform")
}

// removeTrayWindows 非 Windows 平台的 stub
func removeTrayWindows() {}

// setWindowVisible 设置窗口可见（平台无关）
func setWindowVisible(ctx interface{}, visible bool) {
	// 这个函数在 platform_main.go 中实现
}
