package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// isStoreInstall 报告当前进程是否由 Microsoft Store（MSIX full-trust）启动。
//
// Store 包安装到 C:\Program Files\WindowsApps，路径不可写，应用内更新既跑
// 不起来也不符合 Store 政策；旧 Tauri 版也有同样的判断（is_ms_store_install）。
// 这里按 exe 路径判断，Dev 构建/NSIS 安装/便携版都返回 false。
func isStoreInstall() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	return isWindowsAppsPath(exe)
}

// isWindowsAppsPath 是 isStoreInstall 的纯路径判断，便于单测。
func isWindowsAppsPath(path string) bool {
	// 不能只用 filepath.ToSlash：测试在 Linux 上跑时它不会转换 Windows 的
	// 反斜杠。显式两种分隔符都归一。
	p := strings.ToLower(strings.ReplaceAll(filepath.ToSlash(path), "\\", "/"))
	return strings.Contains(p, "/windowsapps/")
}
