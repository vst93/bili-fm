//go:build linux && (amd64 || arm64)

package main

import (
	"log"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/egoist/mygo"
)

// 桌面字幕的「穿透，但设置齿轮固定可点」在 Linux 上用**输入区域**实现：把窗口的
// input shape 设成右上角齿轮那一小块 —— 区域之外（字幕文字 / 背景）的指针事件直接
// 落给下面的窗口，区域之内照常进我们的窗口，齿轮因此**固定**可点，不需要指针轮询。
//
// 为什么不用 mygo 的 Window.SetIgnoreMouseEvents：它的语义是**整窗**穿透（空区域），
// 没有「只留一块区域可交互」的形态；而它内部用的正是同一组 GDK / cairo 调用，这里
// 按同样的办法自己拼一个区域。库名与 mygo 一致（libgtk-3.so.0 / libgdk-3.so.0 /
// libcairo.so.2），符号拿不到时整体退回 mygo 的整窗穿透 + 指针轮询（见
// lyricsClickThroughShape），不会把界面卡死。
//
// 坐标系：输入区域用**控件/应用像素**（与 gtk_widget_get_allocation 同一空间）——
// mygo 给 WebView 挖洞的 shapeRegion 就是这么用的（internal/linux/webview.go），
// 所以齿轮的 DIP 坐标直接可用，不需要乘缩放系数。

var (
	lyricsGTK, lyricsGDK, lyricsCairo uintptr
	lyricsShapeOnce                   sync.Once
	lyricsShapeOK                     bool
	lyricsDragOK                      bool

	lyricsGTKWidgetGetWindow         func(w uintptr) uintptr
	lyricsGTKWidgetQueueDraw         func(w uintptr)
	lyricsGDKInputShapeCombineRegion func(w, region uintptr, x, y int32)
	lyricsCairoRegionCreateRectangle func(r *lyricsGdkRectangle) uintptr
	lyricsCairoRegionDestroy         func(region uintptr)
	// 拖动窗口（设置键按住拖动）。GTK 的 gtk_window_begin_move_drag 要一个
	// 触发拖动的事件时间戳，我们用 gtk_get_current_event_time 拿当前事件时间；
	// 拿不到（0）时退 0，GTK 仍会开始拖动。
	lyricsGTKWindowBeginMoveDrag   func(w, device uintptr, button, xRoot, yRoot int32, time uint32)
	lyricsGTKGetCurrentEventTime   func() uint32
	lyricsGDKSeatGetPointer        func(seat uintptr) uintptr
	lyricsGDKDisplayGetDefaultSeat func(display uintptr) uintptr
	lyricsGDKDisplayGetDefault     func() uintptr
	lyricsGTKWidgetGetDisplay      func(w uintptr) uintptr
)

// lyricsGdkRectangle 是 cairo 的 gdk_rectangle_t（GdkRectangle）：4×int32。
type lyricsGdkRectangle struct{ X, Y, Width, Height int32 }

// lyricsClickThroughShape 报告本平台用输入区域实现穿透（true：齿轮固定可点，
// 不需要指针轮询）。
func lyricsClickThroughShape() bool { return lyricsShapeAvailable() }

// setLyricsClickThrough 把穿透状态应用到字幕窗：clickThrough 为真时窗口只保留
// 齿轮那一块输入区域，其余部分把指针让给下面的窗口。
func setLyricsClickThrough(win *mygo.Window, clickThrough bool) {
	if !lyricsShapeAvailable() {
		// 退化：库 / 符号缺失，用 mygo 的整窗穿透；齿轮靠指针轮询临时取消穿透。
		win.SetIgnoreMouseEvents(clickThrough)
		return
	}
	handle := win.NativeHandle() // GtkWindow*
	if handle == 0 {
		return
	}
	gw := lyricsGTKWidgetGetWindow(handle) // GdkWindow*
	if gw == 0 {
		return
	}
	if !clickThrough {
		// 0 = 去掉输入区域（恢复整窗可交互）—— mygo 的 applyIgnoreMouseEvents
		// 也是用 0 撤销的。
		lyricsGDKInputShapeCombineRegion(gw, 0, 0, 0)
		lyricsGTKWidgetQueueDraw(handle)
		return
	}
	b := win.Bounds()
	x, y, w, h := lyricsGearShapeRect(float32(b.Width), float32(b.Height))
	rect := lyricsGdkRectangle{X: int32(x), Y: int32(y), Width: int32(w), Height: int32(h)}
	region := lyricsCairoRegionCreateRectangle(&rect)
	lyricsGDKInputShapeCombineRegion(gw, region, 0, 0)
	lyricsCairoRegionDestroy(region)
	// Wayland：输入区域随下一帧提交生效（mygo 的 applyIgnoreMouseEvents 同样
	// 在设完形状后 queue 一次重绘）。
	lyricsGTKWidgetQueueDraw(handle)
}

// lyricsShapeAvailable 惰性加载 libgtk-3 / libgdk-3 / libcairo 并绑定所需符号，
// 返回是否可用（失败只记一次日志）。
func lyricsShapeAvailable() bool {
	lyricsShapeOnce.Do(func() {
		var err error
		if lyricsGTK, err = purego.Dlopen("libgtk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			log.Printf("桌面字幕穿透：打不开 libgtk-3.so.0（退回整窗穿透）：%v", err)
			return
		}
		if lyricsGDK, err = purego.Dlopen("libgdk-3.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			log.Printf("桌面字幕穿透：打不开 libgdk-3.so.0（退回整窗穿透）：%v", err)
			return
		}
		if lyricsCairo, err = purego.Dlopen("libcairo.so.2", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
			log.Printf("桌面字幕穿透：打不开 libcairo.so.2（退回整窗穿透）：%v", err)
			return
		}
		syms := []struct {
			lib  uintptr
			fn   any
			name string
		}{
			{lyricsGTK, &lyricsGTKWidgetGetWindow, "gtk_widget_get_window"},
			{lyricsGTK, &lyricsGTKWidgetQueueDraw, "gtk_widget_queue_draw"},
			{lyricsGDK, &lyricsGDKInputShapeCombineRegion, "gdk_window_input_shape_combine_region"},
			{lyricsCairo, &lyricsCairoRegionCreateRectangle, "cairo_region_create_rectangle"},
			{lyricsCairo, &lyricsCairoRegionDestroy, "cairo_region_destroy"},
		}
		for _, s := range syms {
			sym, symErr := purego.Dlsym(s.lib, s.name)
			if symErr != nil || sym == 0 {
				log.Printf("桌面字幕穿透：符号 %s 不可用（退回整窗穿透）", s.name)
				return
			}
			purego.RegisterFunc(s.fn, sym)
		}
		// 拖动相关的符号**可选**：缺了只是拖不动（退回 app 侧的 SetPosition
		// 兜底），不该把「穿透时设置键可点」这条主功能一起废掉。
		dragSyms := []struct {
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
		lyricsDragOK = true
		for _, s := range dragSyms {
			sym, symErr := purego.Dlsym(s.lib, s.name)
			if symErr != nil || sym == 0 {
				log.Printf("桌面字幕拖动：符号 %s 不可用（退回逐帧移动窗口）", s.name)
				lyricsDragOK = false
				break
			}
			purego.RegisterFunc(s.fn, sym)
		}
		lyricsShapeOK = true
	})
	return lyricsShapeOK
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
	if !lyricsShapeAvailable() || !lyricsDragOK || win == nil {
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
	if !lyricsShapeAvailable() {
		return false
	}
	// 面板窗（WindowOptions.Panel）在 Wayland 上就是 layer surface；app 侧
	// 用同一个判据（lyrics.go 的 openDesktopLyricsIfReady 里 Panel: true）。
	return waylandSession()
}
