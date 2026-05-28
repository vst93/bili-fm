//go:build !windows

package main

import "fmt"

// CheckSingleInstance 非 Windows 平台的 stub
func CheckSingleInstance(uniqueId string) (bool, uintptr) {
	fmt.Println("Single instance check not implemented for this platform")
	return true, 0
}

// InitTray 非 Windows 平台的 stub
func InitTray(appName string, showFn func(), exitFn func()) {
	fmt.Println("System tray not implemented for this platform")
}

// RemoveTray 非 Windows 平台的 stub
func RemoveTray() {}

// FindExistingWindow 非 Windows 平台的 stub
func FindExistingWindow(className, windowName string) uintptr {
	return 0
}

// RestoreExistingWindow 非 Windows 平台的 stub
func RestoreExistingWindow(hwnd uintptr) {}
