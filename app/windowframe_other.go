//go:build !windows

package main

// DWM 的窗口边框 / 圆角裁剪只有 Windows 有；其它平台是 no-op。
//
// Linux Wayland 那圈「GTK 画的投影与描边」是另一个来源，而且**只在不透明窗
// 上出现**：mygo 的 `useCSD` 明确排除 `Transparent`（装饰会沿窗口矩形给一块
// 自己画形状的页面描边），主窗与字幕窗都是透明窗，走的是
// `gtk_window_set_decorated(false)`，没有这层。macOS 的窗口边框与圆角由系统画，
// 无边框窗没有可调的边框色。

func tuneWindowFrame(uintptr) {}

func flatWindowFrame(uintptr) {}
