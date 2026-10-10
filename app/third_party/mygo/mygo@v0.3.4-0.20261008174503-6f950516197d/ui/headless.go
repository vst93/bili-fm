package ui

import (
	"fmt"
	"image"
	"strings"
	"sync/atomic"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/raster"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/transfer"
)

// headless renders frames in memory with the software renderer.
type headless struct {
	w, h, scale float32
	img         raster.Renderer
	// requested is set when the view asks for a frame, from any goroutine
	// for invalidate.
	requested atomic.Bool
	dark      bool
	prefs     platform.Preferences
	clipboard string
	cursor    Cursor
	ime       platform.TextInputState
	opened    []string
	openErr   error
	// later are what the view asked to run after the frame, as a window
	// posts them to the main thread.
	later  []func()
	bar    TitleBar
	access *platform.AccessTree
	// told are the announcements of the trees, and announced those the
	// view made.
	told, announced []string
	// menu is the context menu shown, at menuAt, and chosen takes its
	// choice.
	menu   *platform.Menu
	menuAt [2]float32
	chosen func(id int)
	// hz is the display's refresh rate; hidden tells that nothing of the
	// window shows.
	hz     float32
	hidden bool
	// material tells that the window shows one (SetVibrancy).
	material bool
	// reading runs once as the clipboard is next read, as GTK's nested
	// event loop may draw a frame then.
	reading func()
	// last is the scene of the last frame, which tests inspect.
	last        *scene.Scene
	dragData    transfer.Data
	dragLocal   any
	dragOptions transfer.DragOptions
}

func (h *headless) size() (float32, float32, float32) { return h.w, h.h, h.scale }
func (h *headless) refreshRate() float32              { return h.hz }
func (h *headless) occluded() bool                    { return h.hidden }
func (h *headless) present(s *scene.Scene) {
	h.last = s
	h.img.Render(s)
}
func (h *headless) framePath() string                          { return "drawn in memory" }
func (h *headless) requestFrame()                              { h.requested.Store(true) }
func (h *headless) setCursor(c Cursor)                         { h.cursor = c }
func (h *headless) setTextInput(t platform.TextInputState)     { h.ime = t }
func (h *headless) updateAccessibility(t *platform.AccessTree) { h.keepAccess(t) }
func (h *headless) writeClipboard(s string)                    { h.clipboard = s }
func (h *headless) startDrag()                                 {}
func (h *headless) setDropFormats([]transfer.Format)           {}
func (h *headless) readClipboard() string {
	if r := h.reading; r != nil {
		h.reading = nil
		r()
	}
	return h.clipboard
}

func (h *headless) startDataDrag(d transfer.Data, local any, o transfer.DragOptions, x, y float32) error {
	h.dragData, h.dragLocal, h.dragOptions = d.Snapshot(), local, o
	return nil
}
func (h *headless) cancelDataDrag() { h.finishDataDrag(transfer.Result{Canceled: true}) }
func (h *headless) finishDataDrag(r transfer.Result) {
	done := h.dragOptions.Done
	h.dragData, h.dragLocal, h.dragOptions = transfer.Data{}, nil, transfer.DragOptions{}
	if done != nil {
		done(r)
	}
}
func (h *headless) titleBarDoubleClicked()            {}
func (h *headless) isDark() bool                      { return h.dark }
func (h *headless) preferences() platform.Preferences { return h.prefs }
func (h *headless) titleBar() TitleBar                { return h.bar }
func (h *headless) vibrancy() bool                    { return h.material }
func (h *headless) invalidate()                       { h.requested.Store(true) }

// openURL notes the link, and gives done the error FailOpenURL set before
// the next frame, as a window gives it after the system opened the link.
func (h *headless) openURL(u string, done func(error)) {
	h.opened = append(h.opened, u)
	if done != nil {
		err := h.openErr
		h.later = append(h.later, func() { done(err) })
	}
}

// post asks for a frame rather than run fn: the Tester builds each frame
// anew.
func (h *headless) post(func()) { h.requested.Store(true) }

// keepAccess keeps the tree for assistive technology, and its
// announcements.
func (h *headless) keepAccess(t *platform.AccessTree) {
	h.access, h.told = t, append(h.told, t.Announcements...)
}

func (h *headless) popupMenu(m *platform.Menu, x, y float32, chosen func(int)) {
	h.menu, h.menuAt, h.chosen = m, [2]float32{x, y}, chosen
}
func (h *headless) image() *image.RGBA {
	m := &h.img.Image
	return &image.RGBA{Pix: m.RGBA(), Stride: 4 * m.W, Rect: image.Rect(0, 0, m.W, m.H)}
}

// Render draws a frame of view in a window of width×height DIPs at scale
// device pixels per DIP, without a window: for snapshots and tests.
func coreRender(view func(c *context), width, height int, scale float32) *image.RGBA {
	t := coreNewTester(view, width, height)
	t.SetScale(scale)
	return t.Image()
}

// Tester runs a view without a window, as a test drives it: it renders
// frames in memory and sends the view pointer and keyboard input, between
// which it settles the frames the view asks for.
type Tester struct {
	rt *engine
	h  *headless
}

// NewTester starts testing view in a window of width×height DIPs. Two
// elements given one key under one parent make it panic where the second
// was given, as apps only log it.
func coreNewTester(view func(c *context), width, height int) *Tester {
	h := &headless{w: float32(width), h: float32(height), scale: 1}
	t := &Tester{rt: newRuntime(view, h), h: h}
	t.rt.collect = true
	// Duplicate keys panic, so that the test fails where the key was given.
	t.rt.strict = true
	t.rt.handleChecks = true
	t.settle()
	return t
}

// settle runs frames until the view asks for no more, or 20 of them.
func (t *Tester) settle() {
	for i := 0; i < 20; i++ {
		t.h.requested.Store(false)
		for len(t.h.later) > 0 {
			fn := t.h.later[0]
			t.h.later = t.h.later[1:]
			fn()
		}
		t.rt.runFrame()
		if !t.h.requested.Load() && len(t.h.later) == 0 {
			return
		}
	}
}

func (t *Tester) send(ev platform.SurfaceEvent) {
	t.rt.event(ev)
	t.settle()
}

// SetSize resizes the window.
func (t *Tester) SetSize(width, height int) {
	t.h.w, t.h.h = float32(width), float32(height)
	t.settle()
}

// SetScale sets the device pixels per DIP.
func (t *Tester) SetScale(scale float32) {
	t.h.scale = scale
	t.settle()
}

// SetTitleBar sets the room the window controls of a window with a hidden
// title bar take, which Context.TitleBar returns.
func (t *Tester) SetTitleBar(bar TitleBar) {
	t.h.bar = bar
	t.Frame()
}

// SetVibrancy sets whether the window shows a material where the view
// draws no background, which Context.Vibrancy returns.
func (t *Tester) SetVibrancy(shows bool) {
	t.h.material = shows
	t.Frame()
}

// SetDark switches between the light and the dark appearance.
func (t *Tester) SetDark(dark bool) {
	t.h.dark = dark
	t.rt.themeChanged()
	t.settle()
}

// SetPreferences changes the desktop's settings that controls follow, as
// the user does in the system's settings.
func (t *Tester) SetPreferences(p Preferences) {
	t.h.prefs = platform.Preferences{Accent: platform.Color{R: p.Accent.R, G: p.Accent.G, B: p.Accent.B, A: p.Accent.A}, ReduceMotion: p.ReduceMotion,
		HighContrast: p.HighContrast, TextScale: float64(p.TextScale)}
	t.rt.themeChanged()
	t.settle()
}

// Frame renders another frame, as when the view's state changed.
func (t *Tester) Frame() { t.h.requested.Store(true); t.settle() }

// Image returns the last frame.
func (t *Tester) Image() *image.RGBA { return t.h.image() }

// Find returns the box of the first element showing text s, or labeled s.
func (t *Tester) Find(s string) (Rect, bool) {
	for _, n := range t.rt.labels {
		if n.text == s {
			return n.r, true
		}
	}
	return Rect{}, false
}

// Texts returns the texts of the last frame, in order.
func (t *Tester) Texts() []string {
	var out []string
	for _, n := range t.rt.labels {
		out = append(out, n.text)
	}
	return out
}

// HasText reports whether the last frame shows a text containing s.
func (t *Tester) HasText(s string) bool {
	for _, n := range t.rt.labels {
		if strings.Contains(n.text, s) {
			return true
		}
	}
	return false
}

// Click clicks the center of the element showing text or labeled s.
func (t *Tester) Click(s string) error {
	r, ok := t.Find(s)
	if !ok {
		return fmt.Errorf("ui: no element shows %q", s)
	}
	t.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	return nil
}

// ClickAt clicks at (x, y), in DIPs.
func (t *Tester) ClickAt(x, y float32) { t.ClickAtWith(0, x, y) }

// ClickWith clicks the center of the element showing text or labeled s
// holding the modifier keys mods, as a Shift-click.
func (t *Tester) ClickWith(mods Modifiers, s string) error {
	r, ok := t.Find(s)
	if !ok {
		return fmt.Errorf("ui: no element shows %q", s)
	}
	t.ClickAtWith(mods, r.X+r.W/2, r.Y+r.H/2)
	return nil
}

// ClickAtWith clicks at (x, y), in DIPs, holding the modifier keys mods.
func (t *Tester) ClickAtWith(mods Modifiers, x, y float32) {
	m := platform.Modifiers(mods)
	t.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: float64(x), Y: float64(y), Mods: m})
	t.send(platform.SurfaceEvent{Kind: platform.PointerUp, X: float64(x), Y: float64(y), Mods: m})
}

// RightClick clicks the center of the element showing text or labeled s
// with the secondary button.
func (t *Tester) RightClick(s string) error {
	r, ok := t.Find(s)
	if !ok {
		return fmt.Errorf("ui: no element shows %q", s)
	}
	t.RightClickAt(r.X+r.W/2, r.Y+r.H/2)
	return nil
}

// RightClickAt clicks at (x, y), in DIPs, with the secondary button.
func (t *Tester) RightClickAt(x, y float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: float64(x), Y: float64(y), Button: 1})
	t.send(platform.SurfaceEvent{Kind: platform.PointerUp, X: float64(x), Y: float64(y), Button: 1})
}

// Menu returns the labels of the items of the context menu shown, "-" for
// separators, or nil when none is.
func (t *Tester) Menu() []string {
	if t.h.menu == nil {
		return nil
	}
	out := []string{}
	for _, it := range t.h.menu.Items {
		label := it.Label
		if it.Type == platform.MenuItemSeparator {
			label = "-"
		}
		out = append(out, label)
	}
	return out
}

// ChooseMenuItem chooses the item of the context menu shown labeled with
// the last of path, in the submenus the others label, and closes the menu.
func (t *Tester) ChooseMenuItem(path ...string) error {
	m := t.h.menu
	if m == nil {
		return fmt.Errorf("ui: no context menu is shown")
	}
	if len(path) == 0 {
		return fmt.Errorf("ui: no item to choose")
	}
	var found *platform.MenuItem
	for i, label := range path {
		found = nil
		for _, it := range m.Items {
			if it.Label == label && it.Type != platform.MenuItemSeparator {
				found = it
				break
			}
		}
		switch {
		case found == nil:
			return fmt.Errorf("ui: the context menu has no item %q", strings.Join(path[:i+1], " > "))
		case !found.Enabled:
			return fmt.Errorf("ui: the item %q is disabled", strings.Join(path[:i+1], " > "))
		case i < len(path)-1:
			if found.Submenu == nil {
				return fmt.Errorf("ui: the item %q has no submenu", strings.Join(path[:i+1], " > "))
			}
			m = found.Submenu
		}
	}
	if found.Type == platform.MenuItemSubmenu {
		return fmt.Errorf("ui: %q opens a submenu", strings.Join(path, " > "))
	}
	chosen := t.h.chosen
	t.h.menu, t.h.chosen = nil, nil
	chosen(found.ID)
	t.settle()
	return nil
}

// CloseMenu closes the context menu shown without choosing an item.
func (t *Tester) CloseMenu() { t.h.menu, t.h.chosen = nil, nil }

// Press presses (x, y) without releasing; Release releases it.
func (t *Tester) Press(x, y float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerDown, X: float64(x), Y: float64(y)})
}

// Release releases the pointer at (x, y).
func (t *Tester) Release(x, y float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerUp, X: float64(x), Y: float64(y)})
}

// Move moves the pointer to (x, y).
func (t *Tester) Move(x, y float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerMove, X: float64(x), Y: float64(y)})
}

// Scroll scrolls by dx, dy DIPs with the pointer at (x, y).
func (t *Tester) Scroll(x, y, dx, dy float32) {
	t.send(platform.SurfaceEvent{Kind: platform.PointerScroll, X: float64(x), Y: float64(y), DX: float64(dx), DY: float64(dy)})
}

// HoldModifiers presses or lets go of modifier keys on their own, as holding
// Cmd does, leaving mods held.
func (t *Tester) HoldModifiers(mods Modifiers) {
	t.send(platform.SurfaceEvent{Kind: platform.ModifiersChanged, Mods: platform.Modifiers(mods)})
}

// Key presses a key with modifiers.
func (t *Tester) Key(mods Modifiers, key Key) {
	t.send(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: platform.Key(key), Mods: platform.Modifiers(mods)})
	t.send(platform.SurfaceEvent{Kind: platform.KeyReleased, Key: platform.Key(key), Mods: platform.Modifiers(mods)})
}

// TypeKey presses a key that types text, as a keyboard does: the key goes
// down, the text comes, then the key goes up.
func (t *Tester) TypeKey(mods Modifiers, key Key, text string) {
	t.send(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: platform.Key(key), Mods: platform.Modifiers(mods)})
	t.send(platform.SurfaceEvent{Kind: platform.TextInput, Text: text})
	t.send(platform.SurfaceEvent{Kind: platform.KeyReleased, Key: platform.Key(key), Mods: platform.Modifiers(mods)})
}

// Type types text into the focused text input.
func (t *Tester) Type(s string) {
	t.send(platform.SurfaceEvent{Kind: platform.TextInput, Text: s})
}

// SetFocused gives the window the keyboard, or takes it away, as when the
// user switches to another window.
func (t *Tester) SetFocused(focused bool) {
	kind := platform.SurfaceBlur
	if focused {
		kind = platform.SurfaceFocus
	}
	t.send(platform.SurfaceEvent{Kind: kind})
}

// Compose shows text as an input method's composition, with its caret at
// rune caret, as while typing in Chinese or Japanese; an empty text ends
// it. Type commits the text.
func (t *Tester) Compose(text string, caret int) {
	t.send(platform.SurfaceEvent{Kind: platform.TextComposition, Text: text, Caret: caret})
}

// Command performs an edit command of the menus, as Copy of the Edit menu:
// "copy", "cut", "paste", "selectAll", "undo", "redo" or "delete".
func (t *Tester) Command(name string) {
	t.send(platform.SurfaceEvent{Kind: platform.SurfaceCommand, Text: name})
}

// TextCaret returns where input methods compose, and whether one would:
// the caret of the focused text input, or of an element's TextCaret.
func (t *Tester) TextCaret() (Rect, bool) {
	c := t.h.ime.Caret
	return Rect{float32(c.X), float32(c.Y), float32(c.W), float32(c.H)}, t.h.ime.Active
}

// Clipboard returns the text the view copied; SetClipboard sets the text
// it pastes.
func (t *Tester) Clipboard() string { return t.h.clipboard }

// SetClipboard sets the clipboard's text.
func (t *Tester) SetClipboard(s string) { t.h.clipboard = s }

// Cursor returns the pointer shape the view shows.
func (t *Tester) Cursor() Cursor { return t.h.cursor }

// OpenedURLs returns the links the view opened.
func (t *Tester) OpenedURLs() []string { return t.h.opened }

// FailOpenURL makes the links the view opens from now on fail with err, as
// those no app opens, or opens them again for nil.
func (t *Tester) FailOpenURL(err error) { t.h.openErr = err }

// Announcements returns what the view asked screen readers to read out
// (Context.Announce), as a Router the titles of the pages it showed, and
// forgets it.
func (t *Tester) Announcements() []string {
	a := t.h.announced
	t.h.announced = nil
	return a
}

// Focused reports whether the element showing text or labeled s, or its
// container, has the keyboard focus.
func (t *Tester) Focused(s string) bool {
	for _, n := range t.rt.labels {
		if n.text != s {
			continue
		}
		for id := n.id; id != 0; {
			if id == t.rt.focused {
				return true
			}
			st := t.rt.states[id]
			if st == nil {
				break
			}
			id = st.parent
		}
	}
	return false
}
