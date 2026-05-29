//go:build windows

package main

import (
	"syscall"
	"unsafe"
)

// getTextScaleFactor 读取 Windows 辅助功能中的文本大小缩放比例
// 注册表路径: HKEY_CURRENT_USER\Software\Microsoft\Accessibility
// 值名称: TextScaleFactor (REG_DWORD, 范围 100-225)
// 返回值: 缩放百分比，如 140 表示 140%，读取失败返回 100
func getTextScaleFactor() uint32 {
	var hKey syscall.Handle
	err := syscall.RegOpenKeyEx(
		syscall.HKEY_CURRENT_USER,
		syscall.StringToUTF16Ptr(`Software\Microsoft\Accessibility`),
		0,
		syscall.KEY_READ,
		&hKey,
	)
	if err != nil {
		return 100
	}
	defer syscall.RegCloseKey(hKey)

	var valueType uint32
	var value uint32
	var valueSize uint32 = 4

	err = syscall.RegQueryValueEx(
		hKey,
		syscall.StringToUTF16Ptr("TextScaleFactor"),
		nil,
		&valueType,
		(*byte)(unsafe.Pointer(&value)),
		&valueSize,
	)
	if err != nil || valueType != syscall.REG_DWORD {
		return 100
	}

	if value < 100 || value > 225 {
		return 100
	}

	return value
}
