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
#cgo pkg-config: gtk+-3.0
#include <gtk/gtk.h>

// GtkStatusIcon is deprecated in GTK3 but still functional and requires
// no extra dependencies (unlike AppIndicator).

// We use gpointer for callback params to avoid cgo type-matching issues
// with typedef'd pointer types.

extern void onTrayActivate(gpointer icon, gpointer user_data);
extern void onTrayPopupMenu(gpointer icon, guint button, guint activate_time, gpointer user_data);
extern void onTrayMenuItemShow(gpointer user_data);
extern void onTrayMenuItemQuit(gpointer user_data);

static gpointer createStatusIcon(const char *icon_path) {
	GtkStatusIcon *icon = gtk_status_icon_new_from_file(icon_path);
	gtk_status_icon_set_tooltip_text(icon, "bili-FM");
	gtk_status_icon_set_visible(icon, TRUE);
	return (gpointer)icon;
}

static void connectTraySignals(gpointer icon_ptr) {
	GtkStatusIcon *icon = GTK_STATUS_ICON(icon_ptr);
	g_signal_connect(icon, "activate", G_CALLBACK(onTrayActivate), NULL);
	g_signal_connect(icon, "popup-menu", G_CALLBACK(onTrayPopupMenu), NULL);
}

static void showTrayMenu(gpointer icon_ptr, guint button, guint activate_time) {
	GtkMenu *menu = GTK_MENU(gtk_menu_new());
	GtkWidget *item_show = gtk_menu_item_new_with_label("显示窗口");
	GtkWidget *item_sep = gtk_separator_menu_item_new();
	GtkWidget *item_quit = gtk_menu_item_new_with_label("退出");

	g_signal_connect_swapped(item_show, "activate", G_CALLBACK(onTrayMenuItemShow), NULL);
	g_signal_connect_swapped(item_quit, "activate", G_CALLBACK(onTrayMenuItemQuit), NULL);

	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_show);
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_sep);
	gtk_menu_shell_append(GTK_MENU_SHELL(menu), item_quit);
	gtk_widget_show_all(GTK_WIDGET(menu));

	gtk_menu_popup(menu, NULL, NULL, gtk_status_icon_position_menu,
		GTK_STATUS_ICON(icon_ptr), button, activate_time);
}

static void setTrayVisible(gpointer icon_ptr, gboolean visible) {
	gtk_status_icon_set_visible(GTK_STATUS_ICON(icon_ptr), visible);
}
*/
import "C"

var (
	linuxTrayOnce sync.Once
	linuxTrayIcon C.gpointer
	linuxWailsCtx context.Context
	linuxOnExit   func()
	linuxExiting  bool
)

//export onTrayActivate
func onTrayActivate(icon C.gpointer, userData C.gpointer) {
	if linuxWailsCtx != nil {
		runtime.Show(linuxWailsCtx)
	}
}

//export onTrayPopupMenu
func onTrayPopupMenu(icon C.gpointer, button C.guint, activateTime C.guint, userData C.gpointer) {
	C.showTrayMenu(linuxTrayIcon, button, activateTime)
}

//export onTrayMenuItemShow
func onTrayMenuItemShow(userData C.gpointer) {
	if linuxWailsCtx != nil {
		runtime.Show(linuxWailsCtx)
	}
}

//export onTrayMenuItemQuit
func onTrayMenuItemQuit(userData C.gpointer) {
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

// initTrayLinux creates a GTK StatusIcon tray on Linux.
// Must be called from the GTK main thread (wails OnStartup runs on it).
func initTrayLinux(ctx context.Context, exitFn func()) {
	linuxTrayOnce.Do(func() {
		linuxWailsCtx = ctx
		linuxOnExit = exitFn

		// Use the app icon - try /usr/share/pixmaps first (installed path),
		// then fall back to build/appicon.png (dev/build path)
		iconPath := "/usr/share/pixmaps/bili-FM.png"
		if _, err := os.Stat(iconPath); err != nil {
			iconPath = "build/appicon.png"
		}

		cPath := C.CString(iconPath)
		defer C.free(unsafe.Pointer(cPath))
		linuxTrayIcon = C.createStatusIcon(cPath)
		C.connectTraySignals(linuxTrayIcon)
	})
}

func removeTrayLinux() {
	if linuxTrayIcon != nil {
		C.setTrayVisible(linuxTrayIcon, C.FALSE)
	}
}

func linuxShowExistingWindow() {
	if linuxWailsCtx != nil {
		runtime.Show(linuxWailsCtx)
	}
}

// ---- Common API (matches tray_windows.go / tray_other.go) ----

// Stubs for Windows-only functions (not used on Linux)
func checkSingleInstanceWindows() (bool, uintptr) { return true, 0 }
func closeMutex(handle uintptr)                   {}
func findExistingWindow() uintptr                 { return 0 }
func restoreExistingWindow(hwnd uintptr)           {}
func initTrayWindows(showFn func(), exitFn func()) {}
func removeTrayWindows()                          {}
func setWailsContext(ctx context.Context)         {}
func bringWindowToFront()                         {}

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
