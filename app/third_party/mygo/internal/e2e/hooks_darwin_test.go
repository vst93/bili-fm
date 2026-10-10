//go:build darwin

package e2e

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/internal/darwin"
)

func activateMenu(w *mygo.Window, path ...string) (err error) {
	mygo.RunOnMain(func() { err = darwin.TestPerformMenuItem(path...) })
	return err
}

func pressShortcut(key string, shift bool) (handled bool, supported bool) {
	mygo.RunOnMain(func() { handled = darwin.TestPerformKeyEquivalent(key, shift) })
	return handled, true
}

func endSheet(w *mygo.Window) (ok bool, supported bool) {
	mygo.RunOnMain(func() { ok = darwin.TestEndSheet(w.NativeHandle()) })
	return ok, true
}

func click(w *mygo.Window, x, y float64) bool {
	mygo.RunOnMain(func() { darwin.TestClick(w.NativeHandle(), x, y) })
	return true
}

func sideButton(w *mygo.Window, back bool) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestSideButton(w.NativeHandle(), back) })
	return ok
}

func drag(w *mygo.Window, points [][2]float64) bool {
	mygo.RunOnMain(func() { darwin.TestDrag(w.NativeHandle(), points) })
	return true
}

func webViewAttached(w *mygo.Window) (attached bool, supported bool) {
	mygo.RunOnMain(func() { attached = darwin.TestWebViewAttached(w.NativeHandle()) })
	return attached, true
}

func dockDevTools(w *mygo.Window) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestDockInspector(w.NativeHandle()) })
	return ok
}

func dockedDevToolsPlace(w *mygo.Window) (place string) {
	mygo.RunOnMain(func() { place = darwin.TestInspectorPlace(w.NativeHandle()) })
	return place
}

// Context menus track the mouse in a modal loop; not automated.
func dismissPopups() (int, bool) { return 0, false }

func setDroppedFiles(w *mygo.Window, paths []string) bool {
	mygo.RunOnMain(func() { darwin.TestSetDroppedFiles(w.NativeHandle(), paths) })
	return true
}

func dockTileImage() (png []byte, supported bool) {
	mygo.RunOnMain(func() { png = darwin.TestDockTileImage() })
	return png, true
}

func defaultURLHandler(string) (string, bool) { return "", false }

func dockMenu(click int) (titles []string, supported bool) {
	mygo.RunOnMain(func() { titles = darwin.TestDockMenu(click) })
	return titles, true
}

// Posting key events needs the accessibility permission on macOS.
func pressCtrlShiftK() bool { return false }

// Only Linux binds global shortcuts through a desktop portal.
func usePortalShortcuts() (func(), bool) { return nil, false }

func pressKeys(...string) bool { return false }

func fullScreenHidesToolbar(w *mygo.Window) (hides bool, supported bool) {
	mygo.RunOnMain(func() { hides = darwin.TestFullScreenHidesToolbar(w.NativeHandle()) })
	return hides, true
}

func trafficLights(w *mygo.Window) (x, y float64, supported bool) {
	mygo.RunOnMain(func() { x, y = darwin.TestTrafficLights(w.NativeHandle()) })
	return x, y, true
}

// Frameless windows keep native resize borders here.
func movePointer(int, int) bool                { return false }
func pressButton(bool) bool                    { return false }
func resizeCursor(*mygo.Window) (string, bool) { return "", false }

// The menu bar belongs to the application on macOS.
func menuBarShown(*mygo.Window) (bool, bool)                { return false, false }
func activateAccelerator(*mygo.Window, string) (bool, bool) { return false, false }
func enterMenuBar(*mygo.Window, string) (bool, bool, bool)  { return false, false, false }
func openMenus(*mygo.Window, byte) bool                     { return false }
func closeMenus()                                           {}

// The traffic lights are AppKit's own.
func titleButtons(*mygo.Window) ([]string, bool) { return nil, false }
func pressTitleButton(*mygo.Window, string) bool { return false }

func topNonClient(*mygo.Window) (int32, int32, bool) { return 0, 0, false }

func titleButtonColor(*mygo.Window, string) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

// AppKit composes the window: there is no redirection bitmap to go without.
func composition(*mygo.Window) (bool, bool, bool) { return false, false, false }
func loseComposition(bool) bool                   { return false }

// WebKit creates web views with their windows.
func failWebViews(int) bool { return false }

func clickAndType(w *mygo.Window, x, y float64, text string) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestClickAndType(w.NativeHandle(), x, y, text) })
	return ok
}

// pressKey presses the key of a virtual key code in a window of native UI,
// through the input method.
func pressKey(w *mygo.Window, code uint16, chars string) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestKey(w.NativeHandle(), code, chars) })
	return ok
}

func compose(w *mygo.Window, text string, caret int, commit bool) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestCompose(w.NativeHandle(), text, caret, commit) })
	return ok
}

func controlClick(w *mygo.Window, x, y float64) bool {
	mygo.RunOnMain(func() { darwin.TestControlClick(w.NativeHandle(), x, y) })
	return true
}

func composeOver(w *mygo.Window, text string, caret int, commit bool, from, length int) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestComposeOver(w.NativeHandle(), text, caret, commit, from, length) })
	return ok
}

func inputClient(w *mygo.Window) (selected [2]int, document string, ok bool) {
	mygo.RunOnMain(func() { selected, document = darwin.TestInputClient(w.NativeHandle()) })
	return selected, document, true
}

func dropFiles(w *mygo.Window, x, y float64, paths []string) (over, dropped, ok bool) {
	mygo.RunOnMain(func() { over, dropped = darwin.TestDropFiles(w.NativeHandle(), x, y, paths) })
	return over, dropped, true
}

// The roles of elements in the accessibility API of AppKit.
const roleText, roleButton, roleCheckBox, roleTextField, roleSlider = "AXStaticText", "AXButton", "AXCheckBox", "AXTextField", "AXSlider"

// roleListItem is the role of the rows of a List.
const roleListItem = "AXRow/AXTableRow"

func accessibility(w *mygo.Window) (nodes []accessNode, ok bool) {
	mygo.RunOnMain(func() {
		for _, n := range darwin.TestAccessibility(w.NativeHandle()) {
			role := n.Role
			if n.Subrole != "" {
				role += "/" + n.Subrole
			}
			nodes = append(nodes, accessNode{role, n.Label, n.Value})
		}
	})
	return nodes, true
}

// axAttribute reads an attribute of an element through AppKit's older
// accessibility API, which only macOS has.
func axAttribute(w *mygo.Window, label, attr string) (value string, settable, named, ok bool) {
	mygo.RunOnMain(func() { value, settable, named, ok = darwin.TestAccessibilityAttribute(w.NativeHandle(), label, attr) })
	return value, settable, named, ok
}

func accessPerform(w *mygo.Window, label, action, value string) (ok bool) {
	mygo.RunOnMain(func() { ok = darwin.TestAccessibilityPerform(w.NativeHandle(), label, action, value) })
	return ok
}

// observe has key-value observing watch the view of a window showing
// native UI and its elements, until stop.
func observe(w *mygo.Window) (stop func(), supported bool) {
	var end func()
	mygo.RunOnMain(func() { end = darwin.TestObserve(w.NativeHandle()) })
	return func() { mygo.RunOnMain(end) }, true
}

// Only Linux draws native UI in a GtkGLArea, nor waits to load the GPU's
// driver.
func glSurface(*mygo.Window) (string, []byte, int, int, bool) { return "", nil, 0, 0, false }
func memoryUI(*testing.T) bool                                { return false }
func surfaceOnScreen(*mygo.Window) ([]byte, bool)             { return nil, false }

// Only Windows reads the screen over native UI.
func screenColor(*mygo.Window, float64, float64) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

// Context menus are not automated on macOS: one shown waits for the user.
func rightClick(*mygo.Window, float64, float64) bool { return false }
func popupMenus() ([][]string, bool)                 { return nil, false }
func choosePopupItem(string) bool                    { return false }
