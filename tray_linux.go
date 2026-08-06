//go:build linux

package main

import (
	"context"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

/*
#cgo pkg-config: gtk+-3.0 ayatana-appindicator3-0.1
#include <gtk/gtk.h>
#include <libayatana-appindicator/app-indicator.h>

// 使用 libayatana-appindicator (AppIndicator)，通过 StatusNotifierItem 协议
// 同时支持 X11 与 Wayland（GNOME 需安装 AppIndicator 扩展，KDE 原生支持）。
// GTK StatusIcon 已废弃且 Wayland 下不工作。

extern void onIndicatorMenuShow(gpointer user_data);
extern void onIndicatorMenuQuit(gpointer user_data);

static AppIndicator *createIndicator(const char *icon_path) {
	AppIndicator *indicator = app_indicator_new("bili-FM", "bili-FM",
		APP_INDICATOR_CATEGORY_APPLICATION_STATUS);
	if (indicator) {
		GError *error = NULL;
		GdkPixbuf *pixbuf = gdk_pixbuf_new_from_file(icon_path, &error);
		if (pixbuf) {
			app_indicator_set_icon_full(indicator, pixbuf, "bili-FM");
			g_object_unref(pixbuf);
		} else if (error) {
			g_error_free(error);
		}
		app_indicator_set_status(indicator, APP_INDICATOR_STATUS_ACTIVE);
	}
	return indicator;
}

static GtkWidget *buildIndicatorMenu() {
	GtkWidget *menu = gtk_menu_new();
	GtkWidget *item_show = gtk_menu_item_new_with_label("显示窗口");
	GtkWidget *item_sep = gtk_separator_menu_item_new();
	GtkWidget *item_quit = gtk_menu_item_new_with_label("退出");

	g_signal_connect_swapped(item_show, "activate", G_CALLBACK(onIndicatorMenuShow), NULL);
	g_signal_connect_swapped(item_quit, "activate", G_CALLBACK(onIndicatorMenuQuit), NULL);

	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_show);
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_sep);
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_quit);
	gtk_widget_show_all(menu);
	return menu;
}

static void attachIndicatorMenu(AppIndicator *indicator, GtkWidget *menu) {
	app_indicator_set_menu(indicator, GTK_MENU(menu));
}

static void destroyIndicator(AppIndicator *indicator) {
	if (indicator) {
		app_indicator_set_status(indicator, APP_INDICATOR_STATUS_PASSIVE);
		g_object_unref(indicator);
	}
}
*/
import "C"

var (
	linuxTrayOnce  sync.Once
	linuxIndicator *C.AppIndicator
	linuxWailsCtx  context.Context
	linuxOnExit    func()
	linuxExiting   bool
)

//export onIndicatorMenuShow
func onIndicatorMenuShow(userData C.gpointer) {
	if linuxWailsCtx != nil {
		runtime.Show(linuxWailsCtx)
	}
}

//export onIndicatorMenuQuit
func onIndicatorMenuQuit(userData C.gpointer) {
	linuxDoExit()
}

func linuxDoExit() {
	if linuxExiting {
		return
	}
	linuxExiting = true
	if linuxOnExit != nil {
		linuxOnExit()
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()
}

// mustRunOnMainThread 在 GTK 主线程上执行（Wails OnStartup 运行在主线程）。
func initTrayLinux(ctx context.Context, exitFn func()) {
	linuxTrayOnce.Do(func() {
		linuxWailsCtx = ctx
		linuxOnExit = exitFn

		// 图标路径：优先已安装路径，回退到构建路径
		iconPath := "/usr/share/pixmaps/bili-FM.png"
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = "build/appicon.png"
		}

		cPath := C.CString(iconPath)
		defer C.free(unsafe.Pointer(cPath))

		linuxIndicator = C.createIndicator(cPath)
		if linuxIndicator != nil {
			menu := C.buildIndicatorMenu()
			C.attachIndicatorMenu(linuxIndicator, menu)
		}
	})
}

func removeTrayLinux() {
	if linuxIndicator != nil {
		C.destroyIndicator(linuxIndicator)
		linuxIndicator = nil
	}
}

func linuxShowExistingWindow() {
	if linuxWailsCtx != nil {
		runtime.Show(linuxWailsCtx)
	}
}

// ---- Common API (匹配 tray_windows.go / tray_other.go / tray_darwin.go) ----

// Stubs for Windows-only functions (not used on Linux)
func checkSingleInstanceWindows() (bool, uintptr)  { return true, 0 }
func closeMutex(handle uintptr)                    {}
func findExistingWindow() uintptr                  { return 0 }
func restoreExistingWindow(hwnd uintptr)           {}
func initTrayWindows(showFn func(), exitFn func()) {}
func removeTrayWindows()                           {}
func setWailsContext(ctx context.Context)          {}

// Stubs for macOS-only functions (not used on Linux)
func initTrayDarwin(ctx context.Context, showFn func(), exitFn func()) {}
func removeTrayDarwin()                                                {}

// bringWindowToFront 非 Linux 平台的 stub
func bringWindowToFront() {}

// IsExiting checks if the app is in the process of exiting.
func IsExiting() bool {
	return linuxExiting
}

// SetExiting marks the app as exiting.
func SetExiting() {
	linuxExiting = true
}

// showExistingWindow is called when a second instance launches.
// On Linux, we use the Wails runtime to show the window.
func showExistingWindow() {
	linuxShowExistingWindow()
}
