//go:build linux && (amd64 || arm64)

package e2e

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/linux"
)

func activateMenu(w *mygo.Window, path ...string) (err error) {
	mygo.RunOnMain(func() { err = linux.TestActivateMenuItem(w.NativeHandle(), path...) })
	return err
}

// Key presses and dialogs need an input device under Xvfb; not covered.
func pressShortcut(string, bool) (bool, bool) { return false, false }

func endSheet(*mygo.Window) (bool, bool) { return false, false }

// Clicks reach windows showing native UI only, through XTEST.
func click(w *mygo.Window, x, y float64) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestClickSurface(w.NativeHandle(), x, y) })
	return ok
}

func sideButton(w *mygo.Window, back bool) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestSideButton(w.NativeHandle(), back) })
	return ok
}

func drag(w *mygo.Window, points [][2]float64) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestDragSurface(w.NativeHandle(), points) })
	return ok
}

func webViewAttached(*mygo.Window) (bool, bool) { return false, false }

// WebKitGTK docks the inspector inside the web view.
func dockDevTools(*mygo.Window) bool { return false }

func dockedDevToolsPlace(*mygo.Window) string { return "" }

func dismissPopups() (n int, supported bool) {
	mygo.RunOnMain(func() { n = linux.TestDismissPopups() })
	return n, true
}

func setDroppedFiles(w *mygo.Window, paths []string) bool {
	mygo.RunOnMain(func() { linux.TestSetDroppedFiles(w.NativeHandle(), paths) })
	return true
}

// The progress bar is shown by the shell, out of the app's reach.
func dockTileImage() ([]byte, bool) { return nil, false }

func defaultURLHandler(scheme string) (id string, supported bool) {
	mygo.RunOnMain(func() { id = linux.TestDefaultURLHandler(scheme) })
	return id, true
}

func dockMenu(int) ([]string, bool) { return nil, false }

// pressCtrlShiftK presses Ctrl+Shift+K like a keyboard.
func pressCtrlShiftK() (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestPressKeys("Control_L", "Shift_L", "k") })
	return ok
}

// usePortalShortcuts makes global shortcuts bind through the XDG desktop
// portal, as on Wayland, when the desktop offers it.
func usePortalShortcuts() (restore func(), ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestUsePortalShortcuts(true) })
	return func() { mygo.RunOnMain(func() { linux.TestUsePortalShortcuts(false) }) }, ok
}

// pressKeys presses keys, X keysym names such as "Control_L", together.
func pressKeys(keys ...string) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestPressKeys(keys...) })
	return ok
}

// Only macOS windows have a toolbar.
func fullScreenHidesToolbar(*mygo.Window) (bool, bool) { return false, false }

// Only macOS windows have traffic lights.
func trafficLights(*mygo.Window) (float64, float64, bool) { return 0, 0, false }

// movePointer moves the pointer to a point of the screen, and pressButton
// presses or releases the first mouse button, like a mouse.
func movePointer(x, y int) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestMovePointer(x, y) })
	return ok
}

func pressButton(press bool) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestPressButton(press) })
	return ok
}

func resizeCursor(w *mygo.Window) (name string, supported bool) {
	mygo.RunOnMain(func() { name = linux.TestResizeCursor(w.NativeHandle()) })
	return name, true
}

func menuBarShown(w *mygo.Window) (shown, supported bool) {
	mygo.RunOnMain(func() { shown, _ = linux.TestMenuBarState(w.NativeHandle()) })
	return shown, true
}

func activateAccelerator(w *mygo.Window, accel string) (handled, supported bool) {
	mygo.RunOnMain(func() { handled = linux.TestActivateAccelerator(w.NativeHandle(), accel) })
	return handled, true
}

// enterMenuBar presses key, an X keysym name, on the window: the bar should
// show with its first menu open. Then it closes the menus, as Escape does.
func enterMenuBar(w *mygo.Window, key string) (during, after, supported bool) {
	if !w.IsFocused() || !pressKeys(key) {
		return false, false, false
	}
	state := func() (shown, open bool) {
		mygo.RunOnMain(func() { shown, open = linux.TestMenuBarState(w.NativeHandle()) })
		return shown, open
	}
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if shown, open := state(); shown && open {
			during = true
			break
		}
	}
	mygo.RunOnMain(func() { linux.TestCloseMenuBar(w.NativeHandle()) })
	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if after, _ = state(); !after {
			break
		}
	}
	return during, after, true
}

// Alt opens the menus in the bar here (enterMenuBar), and the items of GTK
// menus have no letters.
func openMenus(*mygo.Window, byte) bool { return false }
func closeMenus()                       {}

// titleButtons returns GTK's title buttons over the page of a window with
// a hidden title bar.
func titleButtons(w *mygo.Window) (names []string, supported bool) {
	mygo.RunOnMain(func() { names = linux.TestTitleButtons(w.NativeHandle()) })
	return names, true
}

func pressTitleButton(w *mygo.Window, name string) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestPressTitleButton(w.NativeHandle(), name) })
	return ok
}

func topNonClient(*mygo.Window) (int32, int32, bool) { return 0, 0, false }

func titleButtonColor(*mygo.Window, string) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

// Linux shows no material.
func composition(*mygo.Window) (bool, bool, bool) { return false, false, false }
func loseComposition(bool) bool                   { return false }

// WebKitGTK creates web views with their windows.
func failWebViews(int) bool { return false }

// A Control-click is a secondary click on macOS only.
func controlClick(*mygo.Window, float64, float64) bool { return false }

// composeOver does what an input method that edits typed text does through
// GTK: it deletes the text around the caret it composes over, then
// composes or commits.
func composeOver(w *mygo.Window, text string, caret int, commit bool, from, length int) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestComposeOver(w.NativeHandle(), text, caret, commit, from, length) })
	return ok
}

// inputClient returns the caret and the text around it that input methods
// get through GTK, which knows no selection, and no composition in it.
func inputClient(w *mygo.Window) (selected [2]int, document string, ok bool) {
	mygo.RunOnMain(func() {
		var caret int
		document, caret, ok = linux.TestSurrounding(w.NativeHandle())
		selected = [2]int{caret, 0}
	})
	return selected, document, ok
}

func dropFiles(w *mygo.Window, x, y float64, paths []string) (over, dropped, ok bool) {
	mygo.RunOnMain(func() { over, dropped = linux.TestDropFiles(w.NativeHandle(), x, y, paths) })
	return over, dropped, true
}

// The roles of elements in ATK.
const roleText, roleButton, roleCheckBox, roleTextField, roleSlider = "label", "button", "check box", "entry", "slider"

// roleListItem is the role of the rows of a List.
const roleListItem = "list item"

func accessibility(w *mygo.Window) (nodes []accessNode, ok bool) {
	mygo.RunOnMain(func() {
		var list []linux.TestAccessNode
		list, ok = linux.TestAccessibility(w.NativeHandle())
		for _, n := range list {
			nodes = append(nodes, accessNode{n.Role, n.Label, n.Value})
		}
	})
	return nodes, ok
}

func accessPerform(w *mygo.Window, label, action, value string) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestAccessibilityPerform(w.NativeHandle(), label, action, value) })
	return ok
}

// Typing into native UI is only automated on macOS.
func clickAndType(*mygo.Window, float64, float64, string) bool { return false }
func compose(*mygo.Window, string, int, bool) bool             { return false }

// glSurface returns how the native UI of a window draws ("opengl",
// "memory", "cairo") and, for a GtkGLArea, the pixels of its framebuffer,
// premultiplied BGRA rows from the top.
func glSurface(w *mygo.Window) (how string, pix []byte, width, height int, supported bool) {
	mygo.RunOnMain(func() { how, pix, width, height = linux.TestSurfaceGL(w.NativeHandle()) })
	return how, pix, width, height, true
}

// memoryUI has the windows of native UI created from now on draw in
// memory, in a GtkDrawingArea, as without a GPU.
func memoryUI(t *testing.T) bool {
	t.Setenv("MYGO_GPU", "0")
	return true
}

// surfaceOnScreen returns, as a PNG, what the display shows of a window's
// native UI.
// Only Windows reads the screen over native UI.
func screenColor(*mygo.Window, float64, float64) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

func surfaceOnScreen(w *mygo.Window) (png []byte, supported bool) {
	mygo.RunOnMain(func() { png = linux.TestSurfaceOnScreen(w.NativeHandle()) })
	return png, true
}

// rightClick clicks (x, y) in a window showing native UI with the
// secondary button, through XTEST.
func rightClick(w *mygo.Window, x, y float64) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestRightClickSurface(w.NativeHandle(), x, y) })
	return ok
}

// popupMenus returns the labels of the items of the context menus shown.
func popupMenus() (menus [][]string, supported bool) {
	mygo.RunOnMain(func() { menus = linux.TestPopups() })
	return menus, true
}

func choosePopupItem(label string) (ok bool) {
	mygo.RunOnMain(func() { ok = linux.TestChoosePopupItem(label) })
	return ok
}

// Only macOS has key-value observing.
func observe(*mygo.Window) (func(), bool) { return nil, false }

// Input methods keep the keys typed while they compose from the content
// themselves here (GTK's filtering, IMM32's VK_PROCESSKEY), which a
// composition the test makes up does not show.
func pressKey(*mygo.Window, uint16, string) bool { return false }

// AppKit's older accessibility API is macOS's.
func axAttribute(*mygo.Window, string, string) (string, bool, bool, bool) {
	return "", false, false, false
}
