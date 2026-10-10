//go:build !darwin && !(linux && (amd64 || arm64)) && !(windows && (amd64 || arm64))

package e2e

import (
	"errors"
	"testing"

	"github.com/egoist/mygo"
)

// Platforms without a backend run no GUI tests.
func activateMenu(*mygo.Window, ...string) error { return errors.New("unsupported") }

func pressShortcut(string, bool) (bool, bool) { return false, false }

func endSheet(*mygo.Window) (bool, bool) { return false, false }

func click(*mygo.Window, float64, float64) bool { return false }

func drag(*mygo.Window, [][2]float64) bool { return false }

func sideButton(*mygo.Window, bool) bool { return false }

func webViewAttached(*mygo.Window) (bool, bool) { return false, false }

func dockDevTools(*mygo.Window) bool { return false }

func dockedDevToolsPlace(*mygo.Window) string { return "" }

func dismissPopups() (int, bool) { return 0, false }

func setDroppedFiles(*mygo.Window, []string) bool { return false }

// The progress bar is shown by the shell, out of the app's reach.
func dockTileImage() ([]byte, bool) { return nil, false }

func defaultURLHandler(string) (string, bool) { return "", false }

func dockMenu(int) ([]string, bool) { return nil, false }

// Posting key events needs the accessibility permission on macOS.
func pressCtrlShiftK() bool { return false }

// Only Linux binds global shortcuts through a desktop portal.
func usePortalShortcuts() (func(), bool) { return nil, false }

func pressKeys(...string) bool { return false }

// Only macOS windows have a toolbar.
func fullScreenHidesToolbar(*mygo.Window) (bool, bool) { return false, false }

// Only macOS windows have traffic lights.
func trafficLights(*mygo.Window) (float64, float64, bool) { return 0, 0, false }

func movePointer(int, int) bool                { return false }
func pressButton(bool) bool                    { return false }
func resizeCursor(*mygo.Window) (string, bool) { return "", false }

func menuBarShown(*mygo.Window) (bool, bool)                { return false, false }
func activateAccelerator(*mygo.Window, string) (bool, bool) { return false, false }
func enterMenuBar(*mygo.Window, string) (bool, bool, bool)  { return false, false, false }
func openMenus(*mygo.Window, byte) bool                     { return false }
func closeMenus()                                           {}

func titleButtons(*mygo.Window) ([]string, bool) { return nil, false }
func pressTitleButton(*mygo.Window, string) bool { return false }

func topNonClient(*mygo.Window) (int32, int32, bool) { return 0, 0, false }

func titleButtonColor(*mygo.Window, string) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

func composition(*mygo.Window) (bool, bool, bool) { return false, false, false }
func loseComposition(bool) bool                   { return false }

func failWebViews(int) bool { return false }

// A Control-click is a secondary click on macOS only.
func controlClick(*mygo.Window, float64, float64) bool { return false }

// Input methods, file drops and assistive technology are only automated
// on macOS.
func composeOver(*mygo.Window, string, int, bool, int, int) bool { return false }
func inputClient(*mygo.Window) ([2]int, string, bool)            { return [2]int{}, "", false }
func dropFiles(*mygo.Window, float64, float64, []string) (bool, bool, bool) {
	return false, false, false
}
func accessibility(*mygo.Window) ([]accessNode, bool)         { return nil, false }
func accessPerform(*mygo.Window, string, string, string) bool { return false }

const roleText, roleButton, roleCheckBox, roleTextField, roleSlider = "text", "button", "check box", "text field", "slider"

// roleListItem is the role of the rows of a List.
const roleListItem = "list item"

// Typing into native UI is only automated on macOS.
func clickAndType(*mygo.Window, float64, float64, string) bool { return false }
func compose(*mygo.Window, string, int, bool) bool             { return false }

// Only Linux draws native UI in a GtkGLArea, nor waits to load the GPU's
// driver.
func glSurface(*mygo.Window) (string, []byte, int, int, bool) { return "", nil, 0, 0, false }
func memoryUI(*testing.T) bool                                { return false }
func surfaceOnScreen(*mygo.Window) ([]byte, bool)             { return nil, false }

// Only Windows reads the screen over native UI.
func screenColor(*mygo.Window, float64, float64) (uint8, uint8, uint8, bool) { return 0, 0, 0, false }

// Only macOS has key-value observing.
func observe(*mygo.Window) (func(), bool) { return nil, false }

func rightClick(*mygo.Window, float64, float64) bool { return false }
func popupMenus() ([][]string, bool)                 { return nil, false }
func choosePopupItem(string) bool                    { return false }

// Input methods keep the keys typed while they compose from the content
// themselves here (GTK's filtering, IMM32's VK_PROCESSKEY), which a
// composition the test makes up does not show.
func pressKey(*mygo.Window, uint16, string) bool { return false }

// AppKit's older accessibility API is macOS's.
func axAttribute(*mygo.Window, string, string) (string, bool, bool, bool) {
	return "", false, false, false
}
