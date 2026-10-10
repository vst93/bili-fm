package platform

import (
	"github.com/egoist/mygo/transfer"
	"image"
)

// Surface is the drawing area of a window created with
// WindowOptions.Surface, which shows content MyGo draws itself (package
// ui) instead of a webview. Its methods run on the main thread.
type Surface interface {
	// Native returns the native objects a GPU renderer draws into, which it
	// calls before making one: a surface that shows frames drawn in memory
	// through those objects (Windows' Composed) lets go of them then.
	Native() SurfaceNative
	// Size returns the size of the drawing area in DIPs and how many
	// device pixels a DIP is.
	Size() (width, height, scale float64)
	// RequestFrame asks for a SurfaceFrame event at the next opportunity
	// to draw: the next display refresh, WM_PAINT or GTK draw. Requests
	// made before it comes coalesce.
	RequestFrame()
	// RefreshRate returns how many times a second the display showing the
	// surface refreshes, at most, 0 when unknown.
	RefreshRate() float64
	// PresentPixels shows a frame drawn in memory: premultiplied BGRA rows
	// of stride bytes, width×height device pixels. It is called while
	// handling a SurfaceFrame event.
	PresentPixels(pix []byte, stride, width, height int)
	// SetCursor sets the pointer's shape over the surface.
	SetCursor(c Cursor)
	// SetTextInput turns text input (IME composition) on for an editable
	// element, telling input methods where its caret is and the text
	// around it, or off.
	SetTextInput(t TextInputState)
	// UpdateAccessibility gives assistive technology the content's
	// elements. The content calls it after every frame once the surface
	// sent AccessibilityOn.
	UpdateAccessibility(tree *AccessTree)
	// StartDataDrag starts a native transfer; CancelDataDrag cancels the
	// source owned by this surface. Both run on the main thread.
	StartDataDrag(DragRequest)
	CancelDataDrag()
	// SetDropFormats registers the formats the content can receive.
	SetDropFormats([]transfer.Format)
}

// DamageSurface is a Surface that shows a frame drawn in memory by what
// changed since the last one: Linux's, whose toolkit then repaints, and
// the compositor takes, only that.
type DamageSurface interface {
	// PresentDamage is PresentPixels for a frame that differs from the
	// last one presented only within damage, in device pixels.
	PresentDamage(pix []byte, stride, width, height int, damage []image.Rectangle)
}

// OccludableSurface is a Surface that tells when nothing of it shows, as
// when other windows cover its window, while the system may still give it
// frames: the content then draws nothing that moves until it shows again.
type OccludableSurface interface {
	// Occluded reports whether nothing of the surface shows on screen. The
	// surface sends SurfaceShown once some of it shows again.
	Occluded() bool
}

// WideGamutSurface is a Surface that tells whether its window's screen
// shows colors outside the sRGB gamut (macOS's).
type WideGamutSurface interface {
	// WideGamut reports whether the screen showing most of the window
	// shows colors outside the sRGB gamut, as Display P3 screens do.
	WideGamut() bool
}

// MaterialSurface is a Surface whose window can show a material
// (vibrancy) behind the content, wherever its frames are transparent.
type MaterialSurface interface {
	// ShowsMaterial reports whether the window shows a material behind the
	// content now: what its frames leave transparent shows it, rather than
	// nothing.
	ShowsMaterial() bool
}

// IdleSurface is a Surface that can give back memory once frames stop.
type IdleSurface interface {
	// Idle tells that no frame came for a while: the surface may give
	// back memory that drawing frames took and freed.
	Idle()
}

// TextInputState is the state of the text input that has the keyboard,
// for input methods.
type TextInputState struct {
	// Active is true while an editable element has the keyboard.
	Active bool
	// Caret is the caret's rectangle, in DIPs relative to the surface.
	Caret RectF
	// Text is the input's text around the caret, and Start and End are the
	// selection in it, in runes: both the caret when nothing is selected.
	// Input methods read it to edit what was typed before, as the accents
	// of macOS's press and hold replace the letter they decorate.
	Text       string
	Start, End int
	// Client supplies full text and geometry for a custom text element. The
	// Text/Start/End snapshot remains the plain-widget and TextCaret fallback.
	Client TextInputClient
}

// SurfaceNative holds the native objects of a Surface.
type SurfaceNative struct {
	// HWND is the surface's child window (Windows).
	HWND uintptr
	// Composed tells that the window has no redirection bitmap, which
	// would hide its material (Windows): a GPU renderer presents through
	// DirectComposition, with the frames' alpha, which the material shows
	// through.
	Composed bool
	// View is the surface's NSView and Layer its layer (macOS).
	View, Layer uintptr
	// Widget is the surface's GtkGLArea or GtkDrawingArea, and GLArea the
	// GtkGLArea while its render signal draws a frame with OpenGL, in the
	// context it made current (Linux).
	Widget, GLArea uintptr
}

// RectF is a rectangle in DIPs with fractional coordinates.
type RectF struct{ X, Y, W, H float64 }

// SurfaceEventKind is the kind of a SurfaceEvent.
type SurfaceEventKind uint8

const (
	// SurfaceFrame asks the content to draw a frame now.
	SurfaceFrame SurfaceEventKind = iota
	// SurfaceResize reports a new Size; a frame follows.
	SurfaceResize
	// PointerMove reports the pointer at X, Y.
	PointerMove
	// PointerDown and PointerUp report Button pressed or released at X,
	// Y; Clicks counts quick successive presses (2 for a double click).
	PointerDown
	PointerUp
	// PointerLeave reports the pointer left the surface.
	PointerLeave
	// PointerScroll scrolls by DX, DY DIPs at X, Y: positive DY moves the
	// view down the content, as dragging a scroll bar's thumb down does.
	PointerScroll
	// KeyPressed and KeyReleased report Key with Mods; Repeat marks
	// auto-repeat.
	KeyPressed
	KeyReleased
	// ModifiersChanged reports the modifier keys held, Mods, as one of
	// them goes down or up on its own.
	ModifiersChanged
	// TextInput inserts Text, typed or committed by an input method.
	TextInput
	// TextComposition shows Text as the input method's composition, its
	// caret at rune Caret; an empty Text ends the composition.
	TextComposition
	// SurfaceFocus and SurfaceBlur report the surface gaining or losing
	// the keyboard.
	SurfaceFocus
	SurfaceBlur
	// SurfaceCommand performs the edit command Text: "copy", "cut",
	// "paste", "selectAll", "undo", "redo" or "delete" (Edit menu roles).
	SurfaceCommand
	// FileDragOver reports files of another app dragged over X, Y, and
	// FileDragLeave that they left. FileDrop drops Files at X, Y.
	// WindowHandler.SurfaceEvent returns whether the content takes them
	// there.
	FileDragOver
	FileDragLeave
	FileDrop
	// AccessibilityOn reports that assistive technology asked about the
	// surface: the content calls UpdateAccessibility at once and after
	// every frame from then on.
	AccessibilityOn
	// AccessAction performs Action on the element ID of the accessibility
	// tree, with Text the value of AccessSetValue.
	AccessAction
	// SurfaceShown reports that some of an OccludableSurface shows again
	// after none did.
	SurfaceShown
	// DataDragOver queries a destination; DataDrop commits a transfer.
	// DataDragLeave clears its hover state. Drag carries the query/result.
	DataDragOver
	DataDragLeave
	DataDrop
)

// SurfaceEvent is input on a Surface, or a change of it.
type SurfaceEvent struct {
	Kind SurfaceEventKind
	// X and Y locate the pointer in DIPs relative to the surface.
	X, Y float64
	// Button is 0 for the primary button, 1 the secondary, 2 the middle.
	Button int
	Clicks int
	// DX and DY are scroll distances in DIPs; Precise marks touchpads and
	// other devices that scroll by pixels rather than by lines.
	DX, DY  float64
	Precise bool
	Key     Key
	Mods    Modifiers
	Repeat  bool
	Text    string
	Caret   int
	// Replace makes TextInput replace, and TextComposition compose over,
	// the runes From to To of the last TextInputState.Text, instead of the
	// selection.
	Replace  bool
	From, To int
	// Files are the paths of FileDrop's files, or original native paths
	// for DataDrop's legacy file-listener fallback.
	Files []string
	Drag  *DataDragEvent
	// ID and Action are AccessAction's.
	ID     uint64
	Action AccessActionKind
}

// Modifiers are the modifier keys held during an event.
type Modifiers uint8

const (
	ModShift Modifiers = 1 << iota
	ModCtrl
	ModAlt
	// ModSuper is Command on macOS and the Windows key elsewhere.
	ModSuper
)

// Key identifies a key. Letter and digit keys follow the keyboard layout:
// KeyA is the key that types an a.
type Key uint16

const (
	KeyUnknown Key = iota
	KeyEnter
	KeyEscape
	KeyBackspace
	KeyTab
	KeySpace
	KeyDelete
	KeyInsert
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyLeft
	KeyRight
	KeyUp
	KeyDown
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyMinus
	KeyEqual
	KeyComma
	KeyPeriod
	KeySlash
	KeySemicolon
	KeyQuote
	KeyBracketLeft
	KeyBracketRight
	KeyBackslash
	KeyBackquote
	KeyContextMenu
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	// KeyBack and KeyForward go back and forward in a history: the side
	// buttons of a mouse, and the keys of keyboards that have them.
	KeyBack
	KeyForward
)

// KeyForRune returns the key typing r unshifted, KeyUnknown for others.
func KeyForRune(r rune) Key {
	switch {
	case r >= 'a' && r <= 'z':
		return KeyA + Key(r-'a')
	case r >= 'A' && r <= 'Z':
		return KeyA + Key(r-'A')
	case r >= '0' && r <= '9':
		return Key0 + Key(r-'0')
	}
	switch r {
	case '-':
		return KeyMinus
	case '=':
		return KeyEqual
	case ',':
		return KeyComma
	case '.':
		return KeyPeriod
	case '/':
		return KeySlash
	case ';':
		return KeySemicolon
	case '\'':
		return KeyQuote
	case '[':
		return KeyBracketLeft
	case ']':
		return KeyBracketRight
	case '\\':
		return KeyBackslash
	case '`':
		return KeyBackquote
	case ' ':
		return KeySpace
	}
	return KeyUnknown
}

// Cursor is a pointer shape.
type Cursor uint8

const (
	CursorDefault Cursor = iota
	CursorPointer
	CursorText
	CursorMove
	CursorResizeEW
	CursorResizeNS
	CursorResizeNWSE
	CursorResizeNESW
	CursorNotAllowed
	CursorCrosshair
	CursorGrab
	CursorGrabbing
	CursorResizeN
	CursorResizeE
	CursorResizeS
	CursorResizeW
	CursorResizeColumn
	CursorResizeRow
	CursorVerticalText
	CursorCopy
	CursorAlias
	CursorContextMenu
	CursorNone
)
