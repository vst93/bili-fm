package terminal

import (
	"math"
	"runtime"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/plugins/terminal/internal/vt"
	"github.com/egoist/mygo/ui"
)

// View shows the terminal in a window of native UI, and takes its keyboard
// input once it has the focus, which a click gives it. Size it like any
// element, as with Fill or Grow; its size sets the terminal's, in cells of
// the font. A terminal shows in one view at a time.
//
// Copy and paste are Command+C and Command+V on macOS, which the Edit
// menu's roles also send, and Control+Shift+C and Control+Shift+V
// elsewhere; Shift+Page Up and Shift+Page Down scroll a page of the
// scrollback, as does the wheel, unless the program takes the mouse. Hold
// Shift to select while a program takes the mouse, and Command (Control
// elsewhere) to open a hyperlink (OSC 8) on click.
func View(c *ui.Context, t *Terminal) ui.Element {
	v := t.viewOf(c)
	e := ui.Box(c).Focusable().FocusRing(false).Cursor(ui.CursorText).Clip().Label("Terminal")
	v.build(c, e)
	e.HandleInput(v.input)
	e.TextCaret(v.caret())
	e.Draw(v.paint)
	e.ContextMenu(v.menu)
	return e
}

// viewOf returns the terminal's view state, made on the first frame
// showing it.
func (t *Terminal) viewOf(c *ui.Context) *view {
	if t.v == nil {
		t.v = &view{t: t, blinkStart: time.Now()}
		t.v.shaped.init(4096)
	}
	v := t.v
	if v.services != c.Services() {
		v.services = c.Services()
		draw := v.services.Invalidate
		t.draw.Store(&draw)
	}
	return v
}

// view is what a frame of the terminal needs from frame to frame. Main
// thread only.
type view struct {
	t        *Terminal
	services ui.Services

	theme   *Theme
	applied *Theme // the theme the emulator has

	// The grid, in device pixels: the cells, the baseline in a cell, and
	// the origin of the grid relative to the element; scale is device
	// pixels per DIP.
	font                   fontKey
	fonts                  [4]ui.Font
	scale                  float32
	cellW, cellH, baseline int
	ox, oy                 int
	cols, rows             int

	colors    vt.Colors
	cursor    vt.Cursor
	scrollbar vt.Scrollbar
	lines     []rowCache
	shaped    lru

	keys    *vt.KeyEncoder
	mouse   *vt.MouseEncoder
	gesture *vt.Gesture

	// pending is a key that types text, until the text comes; preedit
	// is an input method's composition and its caret.
	pending      *pendingKey
	preedit      string
	preeditCaret int

	focused    bool
	blinkStart time.Time
	caretCell  [2]int // the cursor's cell, for input methods
	// selecting tells that the primary button selects; reporting that a
	// press went to the program, which gets the release too.
	selecting, reporting bool
	pointer              [2]float32
	scrolled             float32 // scrolling not yet a whole row
	ticking              bool    // an autoscroll tick is due
}

type pendingKey struct {
	key       vt.Key
	unshifted rune
	mods      ui.Modifiers
	repeat    bool
}

// build updates the view as a frame builds.
func (v *view) build(c *ui.Context, e ui.Element) {
	t := v.t
	dark := c.Theme().Dark
	focused := e.Focused()
	t.mu.Lock()
	theme := t.opts.Theme
	if dark && t.opts.DarkTheme != nil {
		theme = t.opts.DarkTheme
	} else if theme == nil {
		theme = defaultTheme(dark)
	}
	v.theme = theme
	if t.term == nil {
		v.release()
	} else {
		if v.applied != theme {
			theme.apply(t.term)
			v.applied = theme
		}
		t.curTheme = theme
		if focused != v.focused && t.term.Mode(vt.ModeFocusEvent) {
			t.in.push(vt.EncodeFocus(focused))
		}
		// Input methods compose where the cursor is now, which the last
		// frame may not show yet.
		v.caretCell = [2]int{v.cursor.X, v.cursor.Y}
		if t.term.AtBottom() {
			x, y := t.term.Cursor()
			v.caretCell = [2]int{x, y}
		}
	}
	copied := t.clipOut
	t.clipOut = nil
	t.mu.Unlock()
	for _, s := range copied {
		c.WriteClipboard(s)
	}
	if focused != v.focused {
		v.focused = focused
		v.blinkStart = c.Now()
		if !focused {
			// Input methods drop their composition with the focus.
			v.preedit, v.pending = "", nil
		}
	}
	// The cursor blinks, every 600 ms, while the terminal has the focus.
	if v.focused && v.cursor.Blinking && v.cursor.Visible {
		since := c.Now().Sub(v.blinkStart)
		c.After(blinkPeriod - since%blinkPeriod)
	}
	if v.ticking {
		c.After(40 * time.Millisecond)
	}
}

const blinkPeriod = 600 * time.Millisecond

// release frees what the view made of the library, once the terminal is
// closed.
func (v *view) release() {
	if v.keys != nil {
		v.keys.Free()
		v.keys = nil
	}
	if v.mouse != nil {
		v.mouse.Free()
		v.mouse = nil
	}
	if v.gesture != nil {
		v.gesture.Free(nil)
		v.gesture = nil
	}
}

// caret returns where input methods compose: the cursor's cell.
func (v *view) caret() ui.Rect {
	if v.scale == 0 {
		return ui.Rect{}
	}
	cw, ch := float32(v.cellW)/v.scale, float32(v.cellH)/v.scale
	x := float32(v.ox)/v.scale + float32(v.caretCell[0])*cw
	y := float32(v.oy)/v.scale + float32(v.caretCell[1])*ch
	return ui.Rect{X: x, Y: y, W: cw, H: ch}
}

// menu is the terminal's context menu.
func (v *view) menu(m *ui.Menu) {
	t := v.t
	cmd := ui.Cmd
	if runtime.GOOS != "darwin" {
		cmd |= ui.Shift
	}
	t.mu.Lock()
	text, selected := "", false
	if t.term != nil {
		text, selected = t.term.SelectionText()
	}
	t.mu.Unlock()
	if m.Item("Copy").Shortcut(cmd, ui.KeyC).Disabled(!selected || text == "").Chosen() {
		v.services.WriteClipboard(text)
	}
	if m.Item("Paste").Shortcut(cmd, ui.KeyV).Chosen() {
		v.paste()
	}
	if m.Item("Select All").Chosen() {
		v.selectAll()
	}
	m.Separator()
	if m.Item("Clear Scrollback").Chosen() {
		t.Feed([]byte("\x1b[3J"))
	}
}

// input handles the input of the view as it comes.
func (v *view) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputKeyDown:
		return v.keyDown(ev)
	case ui.InputKeyUp:
		return v.keyUp(ev)
	case ui.InputText:
		v.typed(ev.Text)
		return true
	case ui.InputCompose:
		v.preedit, v.preeditCaret, v.pending = ev.Text, ev.Caret, nil
		return true
	case ui.InputCommand:
		switch ev.Text {
		case "copy":
			v.copy()
		case "paste":
			v.paste()
		case "selectAll":
			v.selectAll()
		default:
			return false
		}
		return true
	case ui.InputPointerDown, ui.InputPointerUp, ui.InputPointerMove:
		return v.pointerEvent(ev)
	case ui.InputScroll:
		return v.scroll(ev)
	}
	return false
}

func (v *view) keyDown(ev ui.InputEvent) bool {
	t := v.t
	mac := runtime.GOOS == "darwin"
	mods, key := ev.Mods, ev.Key
	copyMods := ui.Ctrl | ui.Shift
	if mac {
		copyMods = ui.Super
	}
	switch {
	case mods == copyMods && key == ui.KeyC:
		v.copy()
		return true
	case mods == copyMods && key == ui.KeyV:
		v.paste()
		return true
	case mods == ui.Shift && (key == ui.KeyPageUp || key == ui.KeyPageDown) && v.scrollPage(key == ui.KeyPageUp):
		return true
	case mods&ui.Super != 0:
		return false // the app's shortcuts
	}
	k, unshifted := vtKey(key)
	if k == vt.KeyUnidentified {
		return false
	}
	altMeta := mods&ui.Alt != 0 && (!mac || t.opts.OptionAsAlt)
	// AltGr types text on Windows, where it is Control and Alt.
	altGr := runtime.GOOS == "windows" && mods&(ui.Ctrl|ui.Alt) == ui.Ctrl|ui.Alt
	if typesText(key) && (mods&(ui.Ctrl|ui.Alt) == 0 || mods&ui.Alt != 0 && !altMeta && mods&ui.Ctrl == 0 || altGr) {
		// The text it types comes next.
		v.pending = &pendingKey{key: k, unshifted: unshifted, mods: mods, repeat: ev.Repeat}
		return true
	}
	v.pending = nil
	text := ""
	if altMeta && typesText(key) && mods&ui.Ctrl == 0 {
		// Alt as Meta: the character, which the encoder prefixes with Esc.
		text = string(unshifted)
		if mods&ui.Shift != 0 {
			text = shiftedText(key, unshifted)
		}
	}
	action := vt.KeyPress
	if ev.Repeat {
		action = vt.KeyRepeat
	}
	v.sendKey(vt.KeyEvent{Action: action, Key: k, Mods: vtMods(mods), Text: text, Unshifted: unshifted})
	return true
}

// scrollPage scrolls the scrollback a page, but for full-screen programs,
// which get the key, and reports whether it did.
func (v *view) scrollPage(up bool) bool {
	t := v.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil || t.term.AltScreen() {
		return false
	}
	_, rows := t.term.Size()
	page := max(rows-1, 1)
	if up {
		page = -page
	}
	t.term.ScrollBy(page)
	return true
}

func (v *view) keyUp(ev ui.InputEvent) bool {
	k, unshifted := vtKey(ev.Key)
	if k == vt.KeyUnidentified || ev.Mods&ui.Super != 0 {
		return false
	}
	if p := v.pending; p != nil && p.key == k {
		// No text came, as for Control, Alt and a letter on Windows,
		// where they may type one: the key alone.
		v.pending = nil
		action := vt.KeyPress
		if p.repeat {
			action = vt.KeyRepeat
		}
		text := ""
		if p.mods&ui.Ctrl == 0 {
			text = string(p.unshifted) // what the key types on a US keyboard
			if p.mods&ui.Shift != 0 {
				text = shiftedText(ev.Key, p.unshifted)
			}
		}
		v.sendKey(vt.KeyEvent{Action: action, Key: p.key, Mods: vtMods(p.mods), Text: text, Unshifted: p.unshifted})
	}
	return v.encodeKey(vt.KeyEvent{Action: vt.KeyRelease, Key: k, Mods: vtMods(ev.Mods), Unshifted: unshifted}, false)
}

// typed sends text typed or committed by an input method.
func (v *view) typed(text string) {
	v.preedit = ""
	p := v.pending
	v.pending = nil
	if p == nil {
		// Text without a key: of an input method, or every key typing
		// text on Linux.
		r, size := utf8.DecodeRuneInString(text)
		if size != len(text) || r == utf8.RuneError {
			v.sendText(text)
			return
		}
		k, unshifted, shift := keyOfRune(r)
		mods := ui.Modifiers(0)
		if shift {
			mods = ui.Shift
		}
		p = &pendingKey{key: k, unshifted: unshifted, mods: mods}
	}
	if utf8.RuneCountInString(text) > 1 {
		v.sendText(text)
		return
	}
	var consumed vt.Mods
	r, _ := utf8.DecodeRuneInString(text)
	if r != p.unshifted {
		// Shift or Option made the character.
		consumed = vtMods(p.mods & (ui.Shift | ui.Alt))
	}
	action := vt.KeyPress
	if p.repeat {
		action = vt.KeyRepeat
	}
	v.sendKey(vt.KeyEvent{Action: action, Key: p.key, Mods: vtMods(p.mods), Consumed: consumed, Text: text, Unshifted: p.unshifted})
}

// sendKey sends a key, scrolling to the bottom and clearing the selection
// as Ghostty does when it sends something.
func (v *view) sendKey(k vt.KeyEvent) {
	if v.encodeKey(k, true) {
		v.blinkStart = time.Now()
	}
}

func (v *view) encodeKey(k vt.KeyEvent, press bool) bool {
	t := v.t
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return false
	}
	if v.keys == nil {
		var err error
		if v.keys, err = vt.NewKeyEncoder(); err != nil {
			return false
		}
	}
	v.keys.Sync(t.term, t.opts.OptionAsAlt)
	b := v.keys.Encode(k)
	if len(b) == 0 {
		return false
	}
	t.in.push(append([]byte(nil), b...))
	if press {
		t.term.ScrollToBottom()
		t.term.SetSelection(nil)
	}
	return true
}

// sendText sends text that is no key, as of an input method.
func (v *view) sendText(text string) {
	t := v.t
	t.mu.Lock()
	if t.term != nil {
		t.term.ScrollToBottom()
		t.term.SetSelection(nil)
	}
	t.mu.Unlock()
	t.Send([]byte(text))
	v.blinkStart = time.Now()
}

func (v *view) copy() {
	t := v.t
	t.mu.Lock()
	text, ok := "", false
	if t.term != nil {
		text, ok = t.term.SelectionText()
	}
	t.mu.Unlock()
	if ok && text != "" {
		v.services.WriteClipboard(text)
	}
}

func (v *view) paste() {
	if text := v.services.ReadClipboard(); text != "" {
		v.t.Paste(text)
	}
}

func (v *view) selectAll() {
	t := v.t
	t.mu.Lock()
	if t.term != nil {
		if s, ok := t.term.SelectAll(); ok {
			t.term.SetSelection(&s)
		}
	}
	t.mu.Unlock()
	v.services.Invalidate()
}

// cellAt returns the cell under a point of the element, in DIPs, within
// the grid.
func (v *view) cellAt(x, y float32) (col, row int) {
	if v.cellW == 0 {
		return 0, 0
	}
	col = int(math.Floor(float64((x*v.scale - float32(v.ox)) / float32(v.cellW))))
	row = int(math.Floor(float64((y*v.scale - float32(v.oy)) / float32(v.cellH))))
	return max(0, min(col, v.cols-1)), max(0, min(row, v.rows-1))
}

// pointerEvent reports the pointer to the program that takes the mouse, or
// selects.
func (v *view) pointerEvent(ev ui.InputEvent) bool {
	t := v.t
	v.pointer = [2]float32{ev.X, ev.Y}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil || v.cellW == 0 {
		return false
	}
	tracking := t.term.MouseTracking() && ev.Mods&ui.Shift == 0
	if tracking && !v.selecting || v.reporting {
		return v.report(ev)
	}
	if v.gesture == nil {
		g, err := vt.NewGesture(uint64(500*time.Millisecond), 5*float64(v.scale))
		if err != nil {
			return false
		}
		v.gesture = g
	}
	col, row := v.cellAt(ev.X, ev.Y)
	px, py := float64(ev.X*v.scale), float64(ev.Y*v.scale)
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false // the context menu
		}
		if ev.Mods&ui.Cmd != 0 {
			// A hyperlink (OSC 8), or a URL printed as text.
			url := ""
			if ref, ok := t.term.CellAt(col, row); ok {
				url = t.term.Hyperlink(ref)
			}
			if url == "" {
				text, cols := t.term.RowText(row)
				url = urlAt(text, cols, col)
			}
			if url != "" {
				v.services.OpenURL(url)
				return true
			}
		}
		ref, ok := t.term.CellAt(col, row)
		if !ok {
			return false
		}
		v.selecting = true
		if s, ok := v.gesture.Press(t.term, ref, px, py, uint64(time.Now().UnixNano())); ok {
			t.term.SetSelection(&s)
		} else {
			t.term.SetSelection(nil)
		}
		return true
	case ui.InputPointerMove:
		if !v.selecting {
			return false
		}
		ref, ok := t.term.CellAt(col, row)
		if !ok {
			return true
		}
		if s, ok := v.gesture.Drag(t.term, ref, px, py, v.geometry(), ev.Mods&ui.Alt != 0); ok {
			t.term.SetSelection(&s)
		}
		v.ticking = v.gesture.Autoscroll(t.term) != 0
		return true
	case ui.InputPointerUp:
		if !v.selecting || ev.Button != 0 {
			return false
		}
		v.selecting, v.ticking = false, false
		ref, ok := t.term.CellAt(col, row)
		if ok {
			v.gesture.Release(t.term, &ref)
		} else {
			v.gesture.Release(t.term, nil)
		}
		return true
	}
	return false
}

// geometry is the grid in pixels, for selection gestures.
func (v *view) geometry() vt.Geometry {
	return vt.Geometry{Columns: v.cols, CellWidth: v.cellW, PadLeft: v.ox, Height: v.oy + v.rows*v.cellH}
}

// autoscroll scrolls a selection dragged past the top or the bottom; t.mu
// is held.
func (v *view) autoscroll() {
	t := v.t
	dir := v.gesture.Autoscroll(t.term)
	if dir == 0 || !v.selecting {
		v.ticking = false
		return
	}
	t.term.ScrollBy(dir)
	col, row := v.cellAt(v.pointer[0], v.pointer[1])
	if s, ok := v.gesture.Tick(t.term, col, row, float64(v.pointer[0]*v.scale), float64(v.pointer[1]*v.scale), v.geometry(), false); ok {
		t.term.SetSelection(&s)
	}
}

// report reports the pointer to the program; t.mu is held.
func (v *view) report(ev ui.InputEvent) bool {
	t := v.t
	if v.mouse == nil {
		m, err := vt.NewMouseEncoder()
		if err != nil {
			return false
		}
		v.mouse = m
	}
	button := []int{vt.ButtonLeft, vt.ButtonRight, vt.ButtonMiddle}
	var action vt.MouseAction
	b := vt.ButtonNone
	switch ev.Kind {
	case ui.InputPointerDown:
		action = vt.MousePress
		v.reporting = true
	case ui.InputPointerUp:
		action = vt.MouseRelease
		v.reporting = false
	default:
		action = vt.MouseMotion
	}
	if ev.Button >= 0 && ev.Button < len(button) {
		b = button[ev.Button]
	}
	pressed := v.reporting
	v.mouse.Sync(t.term, v.ox*2+v.cols*v.cellW, v.oy*2+v.rows*v.cellH, v.cellW, v.cellH, v.ox, v.oy, pressed)
	out := v.mouse.Encode(action, b, vtMods(ev.Mods), ev.X*v.scale, ev.Y*v.scale)
	if len(out) > 0 {
		t.in.push(append([]byte(nil), out...))
	}
	return ev.Kind != ui.InputPointerMove || len(out) > 0
}

// scroll scrolls the scrollback, or reports the wheel to the program.
func (v *view) scroll(ev ui.InputEvent) bool {
	t := v.t
	if v.cellH == 0 {
		return false
	}
	ch := float32(v.cellH) / v.scale
	dy := ev.DY
	if !ev.Precise {
		dy = dy / 40 * 3 * ch // a notch scrolls three rows
	}
	v.scrolled += dy
	rows := int(v.scrolled / ch)
	v.scrolled -= float32(rows) * ch
	if rows == 0 {
		return true
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.term == nil {
		return false
	}
	switch {
	case t.term.MouseTracking() && ev.Mods&ui.Shift == 0:
		if v.mouse == nil {
			m, err := vt.NewMouseEncoder()
			if err != nil {
				return false
			}
			v.mouse = m
		}
		v.mouse.Sync(t.term, v.ox*2+v.cols*v.cellW, v.oy*2+v.rows*v.cellH, v.cellW, v.cellH, v.ox, v.oy, false)
		button := vt.WheelDown
		if rows < 0 {
			button = vt.WheelUp
		}
		for range abs(rows) {
			if out := v.mouse.Encode(vt.MousePress, button, vtMods(ev.Mods), ev.X*v.scale, ev.Y*v.scale); len(out) > 0 {
				t.in.push(append([]byte(nil), out...))
			}
		}
	case t.term.AltScreen() && t.term.Mode(vt.ModeAltScroll):
		// Full-screen programs scroll with the arrows (mode 1007).
		if v.keys == nil {
			var err error
			if v.keys, err = vt.NewKeyEncoder(); err != nil {
				return false
			}
		}
		v.keys.Sync(t.term, false)
		key := vt.KeyArrowDown
		if rows < 0 {
			key = vt.KeyArrowUp
		}
		for range abs(rows) {
			t.in.push(append([]byte(nil), v.keys.Encode(vt.KeyEvent{Action: vt.KeyPress, Key: key})...))
		}
	default:
		t.term.ScrollBy(rows)
	}
	return true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// shiftedText returns what a key types with Shift on a US keyboard.
func shiftedText(k ui.Key, unshifted rune) string {
	for r, key := range shifted {
		if key == k {
			return string(r)
		}
	}
	if 'a' <= unshifted && unshifted <= 'z' {
		return string(unshifted - 'a' + 'A')
	}
	return string(unshifted)
}
