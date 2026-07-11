//go:build !windows && !linux

package main

import "context"

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
func initTrayWindows(showFn func(), exitFn func()) {}

// removeTrayWindows 非 Windows 平台的 stub
func removeTrayWindows() {}

// removeTrayLinux 非 Linux 平台的 stub
func removeTrayLinux() {}

// initTrayLinux 非 Linux 平台的 stub
func initTrayLinux(ctx context.Context, exitFn func()) {}

// IsExiting 非 Windows 平台的 stub
func IsExiting() bool {
	return false
}

// SetExiting 非 Windows 平台的 stub
func SetExiting() {}

// setWailsContext 非 Windows 平台的 stub
func setWailsContext(ctx context.Context) {}

// showExistingWindow 非 Windows 平台的 stub
func showExistingWindow() {}

// bringWindowToFront 非 Windows 平台的 stub
func bringWindowToFront() {}
