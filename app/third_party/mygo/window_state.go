package mygo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

// Window state is what WindowOptions.StateKey remembers of a window, kept
// in window-state.json in PathUserData. It is captured shortly after the
// window moves, resizes or changes state, when it settled (maximizing and
// full screen transitions resize the window on the way), and when it
// closes; it is written when a window closes and when the app quits.

const windowStateFile = "window-state.json"

// How long a window's state must stay unchanged to be captured.
var windowStateDelay = 300 * time.Millisecond

// savedWindow is the state of a window: its normal bounds, which it gets
// back when it leaves the maximized or full screen state, and that state.
type savedWindow struct {
	X          int  `json:"x"`
	Y          int  `json:"y"`
	Width      int  `json:"width"`
	Height     int  `json:"height"`
	Maximized  bool `json:"maximized,omitempty"`
	FullScreen bool `json:"fullScreen,omitempty"`
}

// windowStates is the state file, loaded on first use. Main thread only.
var windowStates struct {
	loaded bool
	byKey  map[string]savedWindow
	dirty  bool
}

func loadWindowStates() {
	if windowStates.loaded {
		return
	}
	windowStates.loaded = true
	windowStates.byKey = map[string]savedWindow{}
	dir, err := App.Path(PathUserData)
	if err != nil {
		return
	}
	if data, err := os.ReadFile(filepath.Join(dir, windowStateFile)); err == nil {
		_ = json.Unmarshal(data, &windowStates.byKey) // a damaged file is ignored
	}
}

// saveWindowStates writes the state file if it changed.
func saveWindowStates() {
	if !windowStates.dirty {
		return
	}
	dir, err := App.Path(PathUserData)
	if err != nil {
		return
	}
	data, err := json.MarshalIndent(windowStates.byKey, "", "  ")
	if err != nil {
		return
	}
	path := filepath.Join(dir, windowStateFile)
	err = os.WriteFile(path+".tmp", data, 0o644)
	if err == nil {
		err = os.Rename(path+".tmp", path)
	}
	// Windows cannot replace a file that another program has open, such as
	// antivirus software scanning it: the next save tries again, at the
	// latest when the app quits.
	windowStates.dirty = err != nil
}

// restoreWindowState applies the state saved under key to p, if the
// window would show on a connected display. Otherwise the window keeps
// its saved size, as far as the display allows, and is centered.
func restoreWindowState(key string, p *platform.WindowOptions) {
	loadWindowStates()
	st, ok := windowStates.byKey[key]
	if !ok || st.Width <= 0 || st.Height <= 0 {
		return
	}
	r := Rectangle{st.X, st.Y, st.Width, st.Height}
	displays := backend().Screen().Displays()
	p.UseContentSize = false
	p.Maximized, p.FullScreen = st.Maximized, st.FullScreen
	if onDisplay(r, displays) {
		p.X, p.Y, p.Width, p.Height = r.X, r.Y, r.Width, r.Height
		p.Center = false
		return
	}
	p.Width, p.Height = r.Width, r.Height
	for _, d := range displays {
		if d.Primary {
			p.Width, p.Height = min(r.Width, d.WorkArea.Width), min(r.Height, d.WorkArea.Height)
		}
	}
	p.Center = true
}

// onDisplay reports whether enough of r shows on a display to reach it.
func onDisplay(r Rectangle, displays []platform.Display) bool {
	for _, d := range displays {
		a := d.WorkArea
		w := min(a.X+a.Width, r.X+r.Width) - max(a.X, r.X)
		h := min(a.Y+a.Height, r.Y+r.Height) - max(a.Y, r.Y)
		if w >= min(100, r.Width) && h >= min(50, r.Height) {
			return true
		}
	}
	return false
}

// initState records the state of a window just created with the options
// p. A window that starts maximized or full screen has not shown its normal
// bounds yet, so they are the requested ones.
func (w *Window) initState(p *platform.WindowOptions) {
	if _, ok := windowStates.byKey[w.stateKey]; !ok && (p.Maximized || p.FullScreen) {
		st := savedWindow{X: p.X, Y: p.Y, Width: p.Width, Height: p.Height, Maximized: p.Maximized, FullScreen: p.FullScreen}
		if p.Center {
			for _, d := range backend().Screen().Displays() {
				if d.Primary {
					st.X = d.WorkArea.X + (d.WorkArea.Width-st.Width)/2
					st.Y = d.WorkArea.Y + (d.WorkArea.Height-st.Height)/2
				}
			}
		}
		windowStates.byKey[w.stateKey] = st
		windowStates.dirty = true
		return
	}
	w.captureState()
}

// stateChanged captures the window's state once it stopped changing.
func (w *Window) stateChanged() {
	if w.stateKey == "" {
		return
	}
	if w.stateTimer != nil {
		w.stateTimer.Reset(windowStateDelay)
		return
	}
	w.stateTimer = time.AfterFunc(windowStateDelay, func() { postMain(w.captureState) })
}

// captureState records the window's current state under its key.
func (w *Window) captureState() {
	n := w.native
	if w.stateKey == "" || n == nil {
		return
	}
	loadWindowStates()
	st := windowStates.byKey[w.stateKey]
	st.Maximized, st.FullScreen = n.IsMaximized(), n.IsFullScreen()
	if !st.Maximized && !st.FullScreen && !n.IsMinimized() {
		if b := n.Bounds(); b.Width > 0 && b.Height > 0 {
			st.X, st.Y, st.Width, st.Height = b.X, b.Y, b.Width, b.Height
		}
	}
	if st.Width <= 0 || st.Height <= 0 {
		return
	}
	if prev, ok := windowStates.byKey[w.stateKey]; !ok || prev != st {
		windowStates.byKey[w.stateKey] = st
		windowStates.dirty = true
	}
}

// closeState captures the state of a closing window for the last time,
// and writes the states.
func (w *Window) closeState() {
	if w.stateTimer != nil {
		w.stateTimer.Stop()
		w.stateTimer = nil
	}
	w.captureState()
	saveWindowStates()
}
