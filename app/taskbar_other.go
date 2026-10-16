//go:build !windows

package main

// 任务栏缩略图工具栏只有 Windows 有；其它平台是 no-op（taskbar.go 的接线
// 仍然跑，但什么都不做）。

func taskbarInit(uintptr) {}

func taskbarSetState(playing, canPrev, canNext bool) {}
