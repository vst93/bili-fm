//go:build !(linux && (amd64 || arm64))

package main

import "github.com/egoist/mygo"

// Windows / macOS：SetIgnoreMouseEvents 是**整窗**属性（WS_EX_TRANSPARENT /
// setIgnoresMouseEvents），没有「只留一块区域可交互」的形态。这两条平台上的
// 齿轮靠 app/lyrics.go 的指针轮询：指针一进齿轮热区就临时取消穿透（
// mygo.Screen.CursorScreenPoint 在 Windows 是 GetCursorPos、macOS 是
// NSEvent.mouseLocation，都是真正的全局坐标，可靠）。

// lyricsClickThroughShape 报告本平台是否用输入区域实现穿透：这两条平台没有，
// 所以 false（齿轮靠轮询）。
func lyricsClickThroughShape() bool { return false }

// setLyricsClickThrough 是这两条平台上的穿透：直接交给 mygo 的整窗穿透。
func setLyricsClickThrough(win *mygo.Window, clickThrough bool) {
	win.SetIgnoreMouseEvents(clickThrough)
}

// lyricsBeginWindowDrag 在 Windows / macOS 上没有原生的「开始拖动」入口（mygo
// 也没导出 StartDrag），返回 false —— app 侧退回逐帧 `SetPosition` 移动窗口
// （见 app/lyrics.go 的 dragLyricsWindow）。这两条平台的窗口位置完全由客户端
// 说了算，逐帧移动是可靠的。
func lyricsBeginWindowDrag(win *mygo.Window) bool { return false }
