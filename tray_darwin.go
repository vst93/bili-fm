//go:build darwin

package main

import (
	"context"
	"os"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void initAppTray(const char* iconPath);
void destroyAppTray(void);
*/
import "C"

var (
	darwinWailsCtx context.Context
	darwinOnExit   func()
	darwinExiting  bool
	darwinTrayInit bool
)

//export trayShowWindow
func trayShowWindow() {
	if darwinWailsCtx != nil {
		runtime.Show(darwinWailsCtx)
	}
}

//export trayQuit
func trayQuit() {
	darwinDoExit()
}

func darwinDoExit() {
	if darwinExiting {
		return
	}
	darwinExiting = true
	if darwinOnExit != nil {
		darwinOnExit()
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}

// initTrayDarwin 在 macOS 状态栏创建托盘。
// Wails OnStartup 运行在主线程，因此 AppKit 调用安全。
func initTrayDarwin(ctx context.Context, showFn func(), exitFn func()) {
	if darwinTrayInit {
		return
	}
	darwinTrayInit = true
	darwinWailsCtx = ctx
	darwinOnExit = exitFn

	println("[tray] initTrayDarwin called")

	iconPath := ""
	if _, err := os.Stat("build/appicon.png"); err == nil {
		iconPath = "build/appicon.png"
	}
	println("[tray] icon path:", iconPath)
	var cPath *C.char
	if iconPath != "" {
		cPath = C.CString(iconPath)
		defer C.free(unsafe.Pointer(cPath))
	}
	C.initAppTray(cPath)
}

func removeTrayDarwin() {
	C.destroyAppTray()
}

// ---- Common API (stubs for other platforms) ----

func checkSingleInstanceWindows() (bool, uintptr)      { return true, 0 }
func closeMutex(handle uintptr)                        {}
func findExistingWindow() uintptr                      { return 0 }
func restoreExistingWindow(hwnd uintptr)               {}
func initTrayWindows(showFn func(), exitFn func())     {}
func removeTrayWindows()                               {}
func initTrayLinux(ctx context.Context, exitFn func()) {}
func removeTrayLinux()                                 {}
func setWailsContext(ctx context.Context)              {}
func bringWindowToFront()                              {}

func IsExiting() bool { return darwinExiting }
func SetExiting()     { darwinExiting = true }

func showExistingWindow() {
	if darwinWailsCtx != nil {
		runtime.Show(darwinWailsCtx)
	}
}
