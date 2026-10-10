//go:build windows && (amd64 || arm64)

package e2e

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
	win "github.com/egoist/mygo/internal/windows"
)

func activateMenu(w *mygo.Window, path ...string) (err error) {
	mygo.RunOnMain(func() { err = win.TestActivateMenuItem(w.NativeHandle(), path...) })
	return err
}

// Keyboard, dialog and popup automation are not wired on Windows.
func pressShortcut(string, bool) (bool, bool) { return false, false }

func endSheet(*mygo.Window) (bool, bool) { return false, false }

// Clicks reach windows showing native UI only: WebView2 takes the mouse in
// windows of its own process.
func click(w *mygo.Window, x, y float64) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestClickSurface(w.NativeHandle(), x, y) })
	return ok
}

func sideButton(w *mygo.Window, back bool) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestSideButton(w.NativeHandle(), back) })
	return ok
}

func drag(w *mygo.Window, points [][2]float64) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestDragSurface(w.NativeHandle(), points) })
	return ok
}

func webViewAttached(*mygo.Window) (bool, bool) { return false, false }

// WebView2 opens DevTools in a window of its own.
func dockDevTools(*mygo.Window) bool { return false }

func dockedDevToolsPlace(*mygo.Window) string { return "" }

func dismissPopups() (int, bool) { return 0, false }

func setDroppedFiles(w *mygo.Window, paths []string) bool {
	mygo.RunOnMain(func() { win.TestSetDroppedFiles(w.NativeHandle(), paths) })
	return true
}

// The progress bar is shown by the shell, out of the app's reach.
func dockTileImage() ([]byte, bool) { return nil, false }

func defaultURLHandler(string) (string, bool) { return "", false }

func dockMenu(int) ([]string, bool) { return nil, false }

func pressCtrlShiftK() bool {
	mygo.RunOnMain(func() { win.TestPressKeys(0x11, 0x10, 'K') }) // VK_CONTROL, VK_SHIFT
	return true
}

// Only Linux binds global shortcuts through a desktop portal.
func usePortalShortcuts() (func(), bool) { return nil, false }

func pressKeys(...string) bool { return false }

// Only macOS windows have a toolbar.
func fullScreenHidesToolbar(*mygo.Window) (bool, bool) { return false, false }

// Only macOS windows have traffic lights.
func trafficLights(*mygo.Window) (float64, float64, bool) { return 0, 0, false }

// Frameless windows keep native resize borders here.
func movePointer(int, int) bool                { return false }
func pressButton(bool) bool                    { return false }
func resizeCursor(*mygo.Window) (string, bool) { return "", false }

func menuBarShown(w *mygo.Window) (shown, supported bool) {
	mygo.RunOnMain(func() { shown = win.TestMenuBarShown(w.NativeHandle()) })
	return shown, true
}

func activateAccelerator(w *mygo.Window, accel string) (handled, supported bool) {
	mygo.RunOnMain(func() { handled = win.TestActivateAccelerator(w.NativeHandle(), accel) })
	return handled, true
}

// enterMenuBar sends the window the SC_KEYMENU that Alt and F10 become, and
// leaves the menus from inside the menu loop, which runs the app's work.
func enterMenuBar(w *mygo.Window, _ string) (during, after, supported bool) {
	hwnd := w.NativeHandle()
	inLoop := make(chan bool, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		mygo.RunOnMain(func() {
			inLoop <- win.TestMenuBarShown(hwnd)
			win.TestEndMenu()
		})
	}()
	mygo.RunOnMain(func() { supported = win.TestEnterMenuBar(hwnd) })
	if !supported {
		return false, false, false
	}
	during = <-inLoop
	mygo.RunOnMain(func() { after = win.TestMenuBarShown(hwnd) })
	return during, after, true
}

// openMenus opens the window's menus from the keyboard: Alt alone, or Alt
// and letter (a virtual key, such as 'A'). Their loop runs the app's work
// until closeMenus.
func openMenus(w *mygo.Window, letter byte) (supported bool) {
	hwnd := w.NativeHandle()
	mygo.RunOnMain(func() { supported = win.TestForeground(hwnd) })
	if !supported {
		return false
	}
	if letter == 0 {
		go mygo.RunOnMain(func() { win.TestEnterMenuBar(hwnd) }) // returns once the menus close
	} else {
		mygo.RunOnMain(func() { win.TestPressKeys(0x12, letter) }) // VK_MENU
	}
	return true
}

// closeMenus leaves the menus, as Escape does.
func closeMenus() { mygo.RunOnMain(win.TestEndMenu) }

// titleButtons returns the window controls of a hidden title bar, named
// after the hit-test codes over them.
func titleButtons(w *mygo.Window) (names []string, supported bool) {
	mygo.RunOnMain(func() { names = win.TestCaptionButtons(w.NativeHandle()) })
	return names, true
}

func pressTitleButton(w *mygo.Window, name string) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestPressCaptionButton(w.NativeHandle(), name) })
	return ok
}

// topNonClient returns how many pixels at the top of a window are not its
// page's, and how many a hidden title bar keeps there.
func topNonClient(w *mygo.Window) (px, want int32, supported bool) {
	mygo.RunOnMain(func() { px, want = win.TestTopNonClient(w.NativeHandle()) })
	return px, want, true
}

// titleButtonColor puts the pointer over a window control of a hidden
// title bar and returns the color on the screen beside its glyph.
func titleButtonColor(w *mygo.Window, name string) (r, g, b uint8, ok bool) {
	mygo.RunOnMain(func() { r, g, b, ok = win.TestCaptionColor(w.NativeHandle(), name) })
	return r, g, b, ok
}

// composition reports whether a window has no redirection bitmap, for the
// material behind its page, and whether the controls of its hidden title
// bar show through DirectComposition.
func composition(w *mygo.Window) (noRedirect, composed, supported bool) {
	mygo.RunOnMain(func() { noRedirect, composed = win.TestComposed(w.NativeHandle()) })
	return noRedirect, composed, true
}

// loseComposition takes the device of DirectComposition away, as a draw
// that finds it invalid does, or, with removed, as a removed GPU does.
func loseComposition(removed bool) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestLoseComposition(removed) })
	return ok
}

// failWebViews makes WebView2 fail to create the next n webviews.
func failWebViews(n int) bool {
	mygo.RunOnMain(func() { win.TestFailWebViews(n) })
	return true
}

// A Control-click is a secondary click on macOS only.
func controlClick(*mygo.Window, float64, float64) bool { return false }

// composeOver does what an input method that converts typed text again
// does through IMM32: it settles the range it reconverts, then composes or
// commits.
func composeOver(w *mygo.Window, text string, caret int, commit bool, from, length int) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestComposeOver(w.NativeHandle(), text, caret, commit, from, length) })
	return ok
}

// inputClient returns the selection and the text around it that input
// methods get with IMR_DOCUMENTFEED, which holds no composition.
func inputClient(w *mygo.Window) (selected [2]int, document string, ok bool) {
	mygo.RunOnMain(func() {
		var start, length int
		document, start, length, ok = win.TestDocumentFeed(w.NativeHandle())
		selected = [2]int{start, length}
	})
	return selected, document, ok
}

func dropFiles(w *mygo.Window, x, y float64, paths []string) (over, dropped, ok bool) {
	mygo.RunOnMain(func() { over, dropped = win.TestDropFiles(w.NativeHandle(), x, y, paths) })
	return over, dropped, true
}

// The control types of elements in UI Automation.
const roleText, roleButton, roleCheckBox, roleTextField, roleSlider = "Text", "Button", "CheckBox", "Edit", "Slider"

// roleListItem is the role of the rows of a List.
const roleListItem = "ListItem"

func accessibility(w *mygo.Window) (nodes []accessNode, ok bool) {
	mygo.RunOnMain(func() {
		var list []win.TestAccessNode
		list, ok = win.TestAccessibility(w.NativeHandle())
		for _, n := range list {
			nodes = append(nodes, accessNode{n.Role, n.Label, n.Value})
		}
	})
	return nodes, ok
}

func accessPerform(w *mygo.Window, label, action, value string) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestAccessibilityPerform(w.NativeHandle(), label, action, value) })
	return ok
}

// Typing into native UI is only automated on macOS.
func clickAndType(*mygo.Window, float64, float64, string) bool { return false }
func compose(*mygo.Window, string, int, bool) bool             { return false }

// Only Linux draws native UI in a GtkGLArea, nor waits to load the GPU's
// driver.
func glSurface(*mygo.Window) (string, []byte, int, int, bool) { return "", nil, 0, 0, false }
func memoryUI(*testing.T) bool                                { return false }
func surfaceOnScreen(*mygo.Window) ([]byte, bool)             { return nil, false }

// screenColor returns the color the screen shows at (x, y), in DIPs, in a
// window showing native UI, or false when the screen cannot be read.
func screenColor(w *mygo.Window, x, y float64) (r, g, b uint8, ok bool) {
	mygo.RunOnMain(func() { r, g, b, ok = win.TestSurfacePixel(w.NativeHandle(), x, y) })
	return r, g, b, ok
}

// Only macOS has key-value observing.
func observe(*mygo.Window) (func(), bool) { return nil, false }

// rightClick clicks (x, y) in a window showing native UI with the
// secondary button, with the messages a mouse sends.
func rightClick(w *mygo.Window, x, y float64) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestRightClickSurface(w.NativeHandle(), x, y) })
	return ok
}

// popupMenus returns the labels of the items of the context menus shown.
// The modal loop of a menu runs the main thread's work.
func popupMenus() (menus [][]string, supported bool) {
	mygo.RunOnMain(func() { menus = win.TestPopups() })
	return menus, true
}

func choosePopupItem(label string) (ok bool) {
	mygo.RunOnMain(func() { ok = win.TestChoosePopupItem(label) })
	return ok
}

// Input methods keep the keys typed while they compose from the content
// themselves here (GTK's filtering, IMM32's VK_PROCESSKEY), which a
// composition the test makes up does not show.
func pressKey(*mygo.Window, uint16, string) bool { return false }

// AppKit's older accessibility API is macOS's.
func axAttribute(*mygo.Window, string, string) (string, bool, bool, bool) {
	return "", false, false, false
}
