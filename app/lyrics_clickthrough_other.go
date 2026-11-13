//go:build !(linux && (amd64 || arm64))

package main

import "github.com/egoist/mygo"

// Windows / macOS：穿透同样走 mygo 的 `Window.SetInputRegion`（见
// app/lyrics_clickthrough.go）。这两条平台上「只在区域里收点击」是 mygo 在
// 内部按指针进出区域切换整窗穿透实现的（Windows 是窗口定时器 15.6ms、
// macOS 是 NSEvent.mouseLocation），app 侧不再自己每 80ms 开关
// SetIgnoreMouseEvents —— 那会盖掉输入区域（它的优先级更高）。

// lyricsInputRegionAvailable 报告本平台能不能用输入区域实现穿透：两条平台
// mygo 都带 `InputRegionWindow` 实现，恒可用。
func lyricsInputRegionAvailable() bool { return true }

// lyricsBeginWindowDrag 在 Windows / macOS 上没有原生的「开始拖动」入口（mygo
// 也没导出 StartDrag），返回 false —— app 侧退回逐帧 `SetPosition` 移动窗口
// （见 app/lyrics.go 的 dragLyricsWindow）。这两条平台的窗口位置完全由客户端
// 说了算，逐帧移动是可靠的。
func lyricsBeginWindowDrag(win *mygo.Window) bool { return false }
