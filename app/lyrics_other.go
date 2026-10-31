//go:build !windows

package main

// 桌面字幕窗的 DWM 边框/圆角裁剪只有 Windows 有；其它平台是 no-op。

func lyricsTuneWindowFrame(uintptr) {}
