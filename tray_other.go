//go:build !windows

package main

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

// IsExiting 非 Windows 平台的 stub
func IsExiting() bool {
	return false
}

// SetExiting 非 Windows 平台的 stub
func SetExiting() {}
