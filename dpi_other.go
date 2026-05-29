//go:build !windows

package main

// getTextScaleFactor 非 Windows 平台不需要文本缩放补偿，固定返回 100
func getTextScaleFactor() uint32 {
	return 100
}
