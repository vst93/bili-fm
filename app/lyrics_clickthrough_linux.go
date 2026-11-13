//go:build linux && (amd64 || arm64)

package main

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// Linux：穿透用 mygo 的 `Window.SetInputRegion`（X11 的 SHAPE 扩展 / Wayland
// surface 的输入区域），实现见 app/lyrics_clickthrough.go。本文件只剩两件事：
//
//  1. 报告输入区域可用（mygo 的 Linux 后端用 GTK/GDK 设窗口输入形状，所以
//     判据是运行时的 GTK/GDK 库在不在）；
//  2. 拖动窗口的 GTK 符号（`gtk_window_begin_move_drag` 那一路）—— 字幕窗的
//     设置键既是按钮又能拖着走，mygo 没导出这个入口。
//
// 以前这里还自己用 cairo 拼 `gdk_window_input_shape_combine_region` 的输入形状
// （mygo 0.8.x 的 `SetIgnoreMouseEvents` 只会给空区域）。0.9.0 有
// `SetInputRegion` 之后那套是重复实现，已删。

var (
	lyricsGTK, lyricsGDK uintptr
	lyricsLibsOnce       sync.Once
	lyricsLibsOK         bool
	lyricsDragOnce       sync.Once
	lyricsDragOK         bool

	lyricsGTKWidgetGetDisplay func(w uintptr) uintptr
	// 拖动窗口（设置键按住拖动）。GTK 的 gtk_window_begin_move_drag 要一个
	// 触发拖动的事件时间戳，我们用 gtk_get_current_event_time 拿当前事件时间；
	// 拿不到（0）时退 0，GTK 仍会开始拖动。
	lyricsGTKWindowBeginMoveDrag   func(w, device uintptr, button, xRoot, yRoot int32, time uint32)
	lyricsGTKGetCurrentEventTime   func() uint32
	lyricsGDKSeatGetPointer        func(seat uintptr) uintptr
	lyricsGDKDisplayGetDefaultSeat func(display uintptr) uintptr
	lyricsGDKDisplayGetDefault     func() uintptr
)

// lyricsInputRegionAvailable 报告本平台能不能用输入区域实现穿透。
//
// 判据是「运行时的 GTK/GDK 库都在」：mygo 的 Linux 后端用它们给窗口设输入形状
// （X11 的 SHAPE 扩展 / Wayland 的 wl_surface 输入区域），库缺失时那条路径同样
// 不生效。返回 false 时穿透退回整窗 `SetIgnoreMouseEvents` + 指针轮询（见
// lyricsApplyClickThrough），用户不会失去「穿透 + 设置键可点」。
//
// 纯判据在 lyricsClickThroughRouteFor / lyricsApplyClickThrough 里，可无头测；
// 这里只提供运行时事实。
func lyricsInputRegionAvailable() bool { return lyricsGTKLibsOK() }

// lyricsGTKLibsOK 惰性 dlopen libgtk-3 / libgdk-3，返回是否都在（失败只记一次
// 日志）。拖动符号另外单独判（见 lyricsDragSymbolsOK）：缺拖动符号只该拖不动，
// 不该把穿透的实现方式一起换掉。
func lyricsGTKLibsOK() bool {
	lyricsLibsOnce.Do(func() {
		var err error
		if lyricsGTK, err = purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			log.Printf("桌面字幕：打不开 libgtk-3.so.0（穿透退回整窗，拖动退回逐帧移动窗口）")
			return
		}
		if lyricsGDK, err = purego.Dlopen("libgdk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			log.Printf("桌面字幕：打不开 libgdk-3.so.0（穿透退回整窗，拖动退回逐帧移动窗口）")
			return
		}
		lyricsLibsOK = true
	})
	return lyricsLibsOK
}

// lyricsDragSymbolsOK 绑定拖动所需的 GTK/GDK 符号，返回是否可用。
func lyricsDragSymbolsOK() bool {
	if !lyricsGTKLibsOK() {
		return false
	}
	lyricsDragOnce.Do(func() {
		syms := []struct {
			lib  uintptr
			fn   any
			name string
		}{
			{lyricsGTK, &lyricsGTKWindowBeginMoveDrag, "gtk_window_begin_move_drag"},
			{lyricsGTK, &lyricsGTKGetCurrentEventTime, "gtk_get_current_event_time"},
			{lyricsGTK, &lyricsGTKWidgetGetDisplay, "gtk_widget_get_display"},
			{lyricsGDK, &lyricsGDKDisplayGetDefault, "gdk_display_get_default"},
			{lyricsGDK, &lyricsGDKDisplayGetDefaultSeat, "gdk_display_get_default_seat"},
			{lyricsGDK, &lyricsGDKSeatGetPointer, "gdk_seat_get_pointer"},
		}
		for _, s := range syms {
			sym, symErr := purego.Dlsym(s.lib, s.name)
			if symErr != nil || sym == 0 {
				log.Printf("桌面字幕拖动：符号 %s 不可用（退回逐帧移动窗口）", s.name)
				return
			}
			purego.RegisterFunc(s.fn, sym)
		}
		lyricsDragOK = true
	})
	return lyricsDragOK
}

// lyricsBeginWindowDrag 在 Linux 上用 GTK 的窗口拖动开始一次拖拽，返回是否已经
// 交给合成器（true 时 app 侧不再逐帧移动窗口）。
//
// mygo 没有导出 `Window.StartDrag`（它的 platform.Window.StartDrag 只给 WebView
// 的拖拽区用），而设置键既是按钮又要能拖动 —— mygo 的 `DragWindow()` 只在元素
// **没有**其它交互标志时生效。所以这里直接调 GTK：
//
//	gtk_window_begin_move_drag(win, seat 的指针 device, 1, rootX, rootY, eventTime)
//
// 已知限制（与面板本身的拖拽同一条，见 AGENTS.md）：Wayland 上带 gtk-layer-shell
// 的合成器（Sway/Hyprland/KDE/COSMIC）里字幕窗是 overlay 层 surface，
// begin_move_drag 不生效 —— 那里拖不动，位置仍由 KV 记忆。
func lyricsBeginWindowDrag(win *mygo.Window) bool {
	if !lyricsDragSymbolsOK() || win == nil {
		return false
	}
	handle := win.NativeHandle()
	if handle == 0 {
		return false
	}
	display := lyricsGTKWidgetGetDisplay(handle)
	if display == 0 {
		display = lyricsGDKDisplayGetDefault()
	}
	if display == 0 {
		return false
	}
	seat := lyricsGDKDisplayGetDefaultSeat(display)
	if seat == 0 {
		return false
	}
	device := lyricsGDKSeatGetPointer(seat)
	if device == 0 {
		return false
	}
	cursor := mygo.Screen.CursorScreenPoint()
	lyricsGTKWindowBeginMoveDrag(handle, device, 1,
		int32(cursor.X), int32(cursor.Y), lyricsGTKGetCurrentEventTime())
	// 交给合成器后由它接管指针。但**带 gtk-layer-shell 的 Wayland 合成器**
	// （Sway/Hyprland/KDE/COSMIC）里字幕窗是 overlay 层 surface，
	// begin_move_drag 是空操作 —— 那里必须退回 app 自己的逐帧摆位，否则拖动
	// 完全没反应（用户 2026-11-11 报「没有跟着鼠标动」）。
	return !lyricsLayerSurface()
}

// lyricsLayerSurface 报告字幕窗是不是 layer surface（Wayland + gtk-layer-shell）。
// 这条路径下 GTK 的窗口拖动不生效，必须由 app 自己摆位置。
func lyricsLayerSurface() bool {
	// 面板窗（WindowOptions.Panel）在 Wayland 上就是 layer surface；app 侧
	// 用同一个判据（lyrics.go 的 openDesktopLyricsIfReady 里 Panel: true）。
	return waylandSession()
}
