package ui

// InputKind is the kind of an InputEvent.
type InputKind uint8

// Kinds of input.
const (
	// InputKeyDown and InputKeyUp report a key with the modifiers held;
	// Repeat marks the auto-repeat of a key held down. A key that types
	// text comes before the text, an InputText event, on macOS and
	// Windows; on Linux, only the text comes while the element takes text
	// (TextCaret).
	InputKeyDown InputKind = iota + 1
	InputKeyUp
	// InputText is text typed, or committed by an input method.
	InputText
	// InputCompose is the composition of an input method, with its caret
	// at rune Caret; an empty Text ends it.
	InputCompose
	// InputCommand is an edit command of the menus, such as Copy of the
	// Edit menu: "copy", "cut", "paste", "selectAll", "undo", "redo" or
	// "delete".
	InputCommand
	// InputPointerDown and InputPointerUp report Button pressed or
	// released at X, Y, with Clicks the count of quick successive presses
	// (2 for a double click). Once the element takes an InputPointerDown,
	// the pointer's moves and its release come to it wherever they are,
	// and the release comes when the window loses the keyboard too.
	InputPointerDown
	InputPointerUp
	// InputPointerMove reports the pointer at X, Y, with Button held, or
	// -1 when none is.
	InputPointerMove
	// InputScroll scrolls by DX, DY DIPs at X, Y; Precise marks
	// touchpads, which scroll by pixels rather than by lines.
	InputScroll
)

// InputEvent is input an element takes as it comes (HandleInput).
type InputEvent struct {
	Kind   InputKind
	Key    Key
	Mods   Modifiers
	Repeat bool
	// Text of InputText, InputCompose and InputCommand, and Caret the
	// rune of an InputCompose's caret.
	Text  string
	Caret int
	// X and Y locate the pointer relative to the element's box.
	X, Y float32
	// Button is 0 for the primary button, 1 the secondary and 2 the
	// middle.
	Button int
	Clicks int
	// DX and DY are what InputScroll scrolls by; positive DY moves the view
	// down the content.
	DX, DY  float32
	Precise bool
}

// HandleInput has fn take the element's input as it comes, on the main
// thread, before the next frame is built: keys, text and edit commands
// while the element has the keyboard focus, the pointer pressed on it,
// moving over it or scrolling over it. fn reports whether it took the
// event; one it leaves goes on as it would without fn, to shortcuts, Tab,
// context menus and scroll containers. Keys that the window or an element
// around the focus handles as a Shortcut go to the shortcut first. A frame
// follows the events fn takes.
//
// It is for widgets that need every key as it is pressed, such as a
// terminal; most widgets ask about their input as they are built instead
// (Clicked, Shortcut, Dragged).
func (e *node) HandleInput(fn func(ev InputEvent) bool) *node {
	e.inputFn = fn
	return e
}

// TextCaret has the element take text from the system's input methods
// while it has the keyboard focus, with their composition at r, the caret,
// relative to the element's box: candidate windows show there. Its
// InputText and Compose events (HandleInput) bring the text.
func (e *node) TextCaret(r Rect) *node {
	e.caret, e.takesText = r, true
	return e
}

// handler returns the state of the innermost element of chain that
// handles its input.
func (rt *engine) handler(chain []uint64) *state {
	for _, id := range chain {
		if s := rt.states[id]; s != nil && s.input != nil && s.flags&flagDisabled == 0 {
			return s
		}
	}
	return nil
}

// deliver gives ev to the element of s, with the pointer's position
// relative to its box, and reports whether it took it.
func (rt *engine) deliver(s *state, ev InputEvent) bool {
	if s == nil || s.input == nil {
		return false
	}
	ev.X, ev.Y = rt.pointerX-s.x, rt.pointerY-s.y
	if !s.input(ev) {
		return false
	}
	rt.requestFrame()
	return true
}

// focusHandler returns the state of the focused element when it handles
// its input.
func (rt *engine) focusHandler() *state {
	if s := rt.states[rt.focused]; s != nil && s.input != nil && rt.windowFocused {
		return s
	}
	return nil
}
