// Package platform defines the contract between the public mygo API and the
// native backends (WKWebView on macOS, WebKitGTK on Linux, WebView2 on
// Windows).
//
// Threading rules: unless a method says otherwise it must be called on the
// main (UI) thread, and every handler callback is invoked on the main thread.
// The public package is responsible for hopping onto the main thread.
package platform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/egoist/mygo/transfer"
)

// ErrUnsupported is returned by features the current backend cannot provide.
var ErrUnsupported = errors.New("not supported on this platform")

// ErrNotificationsDenied is what ShowNotification fails with when the user
// does not allow the app to show notifications.
var ErrNotificationsDenied = errors.New("mygo: the user does not allow notifications")

// Point is a position in screen coordinates (DIPs, origin top-left of the
// primary display).
type Point struct{ X, Y int }

// Size is a width and height in DIPs.
type Size struct{ Width, Height int }

// Rect is a rectangle in screen coordinates (DIPs).
type Rect struct{ X, Y, Width, Height int }

// TitleBar is the room the window controls take at the top of a window
// with a hidden title bar, in DIPs: they sit in a band of Height along the
// top edge, Left wide from the left edge and Right wide from the right.
type TitleBar struct{ Height, Left, Right int }

// Color is an 8-bit RGBA color.
type Color struct{ R, G, B, A uint8 }

// Backend is implemented by every native backend.
type Backend interface {
	// Name identifies the backend, for example "darwin/wkwebview".
	Name() string

	// Init prepares the native application. It is called once, on the main
	// thread, before Run.
	Init(h AppHandler, opts AppOptions) error
	// Run runs the native event loop until Quit is called. h.Ready is
	// invoked once the application has finished launching.
	Run() error
	// Quit makes Run return.
	Quit()

	// IsMainThread reports whether the caller runs on the UI thread. Safe
	// to call from any goroutine.
	IsMainThread() bool
	// Signal asks the backend to invoke AppHandler.Dispatch on the main
	// thread as soon as possible. Safe to call from any goroutine.
	Signal()
	// Step blocks until at least one native event has been processed or
	// Wake has been called. Main thread only; used for nested waits.
	Step()
	// Wake unblocks a pending Step. Safe to call from any goroutine.
	Wake()

	NewWindow(opts *WindowOptions, h WindowHandler) (Window, error)

	SetApplicationMenu(m *Menu)
	// PopupMenu shows a context menu. pos is relative to the window's
	// content area; nil means the current mouse location. It blocks until
	// the menu is dismissed.
	PopupMenu(m *Menu, w Window, pos *Point)
	// UpdateMenuItem refreshes label/state of every native item created
	// for item.ID.
	UpdateMenuItem(item *MenuItem)

	App() AppController
	Dialogs() Dialogs
	Clipboard() Clipboard
	Shell() Shell
	Screen() Screen
	Theme() Theme
	Power() Power

	NewTray(h TrayHandler) (Tray, error)
	RegisterHotkey(id int, accelerator string) error
	UnregisterHotkey(id int)

	NotificationsSupported() bool
	// ShowNotification shows n; done runs on the main thread once the
	// system has it, or with why it could not.
	ShowNotification(n *Notification, done func(error))
	RemoveNotification(id string)
	// RemoveAllNotifications removes the app's notifications, on macOS
	// those of earlier runs too.
	RemoveAllNotifications()
}

// AppOptions configures Backend.Init.
type AppOptions struct {
	Name string
	// ActivationPolicy is "regular" (default), "accessory" or "prohibited".
	// Only meaningful on macOS.
	ActivationPolicy string
}

// AppHandler receives application level events from the backend.
type AppHandler interface {
	Ready()
	// Dispatch drains the queue of functions scheduled for the main thread.
	Dispatch()
	// QuitRequested is called when the user or the OS asks the application
	// to quit (Cmd+Q, Dock > Quit, logout). Returning true lets the quit
	// proceed.
	QuitRequested() bool
	// Terminating is called right before the process is terminated by the
	// OS (logout/shutdown); Run will not return.
	Terminating()
	// Activated is called when the application is re-activated, e.g. by
	// clicking its Dock icon.
	Activated(hasVisibleWindows bool)
	DidBecomeActive()
	DidResignActive()
	OpenURLs(urls []string)
	OpenFiles(paths []string)
	MenuItemClicked(id int)
	ThemeChanged()
	DisplaysChanged()
	// PowerEvent is "suspend", "resume", "lock-screen" or "unlock-screen".
	PowerEvent(event string)
	HotkeyPressed(id int)
	NotificationClicked(id string)
}

// AppController exposes application level native features.
type AppController interface {
	SetActivationPolicy(policy string)
	Activate()
	Hide()
	Unhide()
	IsHidden() bool
	SetBadge(label string)
	Badge() string
	// Bounce requests user attention and returns an id for CancelBounce.
	Bounce(critical bool) int
	CancelBounce(id int)
	SetDockIcon(png []byte) error
	// SetDockMenu sets the menu of the Dock icon (macOS); nil removes it.
	SetDockMenu(m *Menu)
	// ClearBrowsingData deletes the data pages stored; done runs on the
	// main thread.
	ClearBrowsingData(done func(error))
	ShowAboutPanel(opts AboutPanelOptions)
	Locale() string
	// RegisterURLScheme makes the app, identified by id (its identifier)
	// and named name, the handler of the URLs of scheme for the user.
	RegisterURLScheme(scheme, id, name string) error
	UnregisterURLScheme(scheme, id, name string) error
	IsURLSchemeRegistered(scheme, id, name string) bool
	// SetOpenAtLogin makes the app start at login, passing arg where the
	// start command takes arguments; OpenedAtLogin reports whether the
	// system started it so without one (macOS login items).
	SetOpenAtLogin(open bool, id, name, arg string) error
	OpenAtLogin(id, name, arg string) bool
	OpenedAtLogin() bool
	// Package describes the application bundle the process runs from; ok
	// is false for a plain executable (e.g. `go run`). Safe to call from
	// any goroutine, before Init too.
	Package() (info PackageInfo, ok bool)
}

// PackageInfo is metadata of a packaged application.
type PackageInfo struct {
	Name       string
	Version    string
	Identifier string
}

// AboutPanelOptions configures the standard about panel.
type AboutPanelOptions struct {
	ApplicationName    string
	ApplicationVersion string
	Version            string
	Copyright          string
	Credits            string
}

// WindowOptions configures a native window and its webview. The public
// package fills in all defaults before calling NewWindow.
type WindowOptions struct {
	Title          string
	X, Y           int
	Width, Height  int
	Center         bool
	UseContentSize bool
	MinSize        Size
	MaxSize        Size

	Resizable      bool
	Movable        bool
	Minimizable    bool
	Maximizable    bool
	Closable       bool
	Focusable      bool
	Fullscreenable bool
	AlwaysOnTop    bool
	FullScreen     bool
	Maximized      bool
	SkipTaskbar    bool
	AutoHideMenu   bool
	HasShadow      bool
	Frameless      bool
	Transparent    bool
	// BackgroundColor is painted before the page renders; nil keeps the
	// platform default.
	BackgroundColor *Color
	// TitleBarStyle is "default", "hidden" or "hiddenInset". A hidden title
	// bar gives the page the whole window, under the window controls:
	// AppKit's traffic lights on macOS, and on Linux and Windows controls
	// the backend puts over the page (see Window.TitleBar).
	TitleBarStyle string
	// TitleBarHeight is the height of a hidden title bar in DIPs (Linux,
	// Windows); 0 is the backend's own.
	TitleBarHeight       int
	TrafficLightPosition *Point
	Vibrancy             string
	Opacity              float64
	Parent               Window
	Modal                bool

	// UserScripts are injected into every page, in order.
	UserScripts []UserScript
	// Schemes lists the custom protocols the webview must route to
	// WindowHandler.SchemeRequest.
	Schemes   []string
	DevTools  bool
	UserAgent string
	Zoom      float64
	// Autoplay lets the page play audible media without a user gesture.
	Autoplay bool
	// Native carries backend specific state, e.g. the WKWebViewConfiguration
	// WebKit hands us for window.open().
	Native uintptr

	// Surface creates the window with a Surface, which content MyGo draws
	// itself fills, instead of a webview: Window.Surface returns it, the
	// webview methods do nothing and the webview options are ignored.
	Surface bool
}

// UserScript is JavaScript injected into pages.
type UserScript struct {
	Source string
	// AtDocumentEnd injects after the DOM is parsed instead of before any
	// page script runs.
	AtDocumentEnd bool
	// AllFrames injects into iframes too.
	AllFrames bool
}

// Window is a native window hosting a webview.
type Window interface {
	// Handle returns the native window (NSWindow*, GtkWindow*, HWND).
	Handle() uintptr
	// WebViewHandle returns the native webview object.
	WebViewHandle() uintptr
	// Surface returns the drawing surface of a window created with
	// WindowOptions.Surface, nil for others.
	Surface() Surface

	SetTitle(title string)
	Title() string
	SetBounds(r Rect)
	Bounds() Rect
	SetContentBounds(r Rect)
	ContentBounds() Rect
	SetMinimumSize(s Size)
	SetMaximumSize(s Size)
	SetResizable(v bool)
	IsResizable() bool
	SetMovable(v bool)
	IsMovable() bool
	SetMinimizable(v bool)
	IsMinimizable() bool
	SetMaximizable(v bool)
	IsMaximizable() bool
	SetClosable(v bool)
	IsClosable() bool
	SetAlwaysOnTop(v bool)
	IsAlwaysOnTop() bool
	Show()
	ShowInactive()
	Hide()
	IsVisible() bool
	Focus()
	Blur()
	IsFocused() bool
	Minimize()
	IsMinimized() bool
	Maximize()
	Unmaximize()
	IsMaximized() bool
	Restore()
	SetFullScreen(v bool)
	IsFullScreen() bool
	Center()
	SetBackgroundColor(c Color)
	SetOpacity(v float64)
	Opacity() float64
	SetHasShadow(v bool)
	HasShadow() bool
	SetIgnoreMouseEvents(v bool)
	SetContentProtection(v bool)
	SetVibrancy(material string)
	// SetProgressBar shows progress on the taskbar button, Dock icon or
	// launcher entry: state is "" (none), "normal", "indeterminate",
	// "paused" or "error", value between 0 and 1.
	SetProgressBar(state string, value float64)
	FlashFrame(flash bool)
	SetSkipTaskbar(v bool)
	SetVisibleOnAllWorkspaces(v bool)
	// SetIcon sets the window icon from a PNG image; nil restores the
	// application's.
	SetIcon(png []byte) error
	// SetMenu sets a per-window menu bar (Linux/Windows). No-op on macOS.
	SetMenu(m *Menu)
	// SetAutoHideMenu hides the menu bar until Alt or F10 brings the
	// keyboard to it (Linux/Windows). No-op on macOS.
	SetAutoHideMenu(v bool)
	// TitleBar returns the room the window controls of a window with a
	// hidden title bar take, which WindowHandler.TitleBarChanged reports
	// changes of. It is zero for other windows and while the controls are
	// hidden, in full screen.
	TitleBar() TitleBar
	// StartDrag moves the window with the mouse (frameless drag regions).
	StartDrag()
	// TitleBarDoubleClicked performs the platform action for a double click
	// on a custom title bar (zoom/minimize on macOS).
	TitleBarDoubleClicked()
	// Close destroys the window without asking WindowHandler.ShouldClose.
	Close()

	LoadURL(url string)
	LoadHTML(html, baseURL string)
	LoadFile(path, readAccessDir string)
	Reload(ignoreCache bool)
	StopLoading()
	GoBack()
	GoForward()
	CanGoBack() bool
	CanGoForward() bool
	URL() string
	IsLoading() bool
	// Eval runs JavaScript in the main frame and ignores the result.
	Eval(js string)
	// CallAsyncFunction runs body as the body of an async function in the
	// main frame, awaits it and reports the returned value, which the
	// caller arranges to be a string. err is non-nil when the body fails to
	// compile or the evaluation fails natively. Page CSP does not apply.
	CallAsyncFunction(body string, cb func(result string, err error))
	SetZoom(factor float64)
	Zoom() float64
	SetUserAgent(ua string)
	UserAgent() string
	OpenDevTools()
	CloseDevTools()
	IsDevToolsOpened() bool
	// CapturePage takes a PNG snapshot of the visible page.
	CapturePage(cb func(png []byte, err error))
	// PrintToPDF renders the page as a PDF document.
	PrintToPDF(opts PDFOptions, cb func(pdf []byte, err error))
	// DroppedFiles returns the paths of the files of the latest drop on
	// the page and forgets them. The page reports the drop itself (a
	// "drop" message); backends record the paths before the page sees it.
	DroppedFiles() []string
	Print()
}

// WindowHandler receives window and webview events from the backend.
type WindowHandler interface {
	// ShouldClose is called when the user tries to close the window.
	ShouldClose() bool
	// Closed is called after the native window has been destroyed.
	Closed()
	Focused()
	Blurred()
	// MenuItemClicked reports an item chosen from this window's menu bar
	// or context menu, even while the menu has focus instead of the window.
	MenuItemClicked(id int)
	Resized()
	Moved()
	Minimized()
	Restored()
	Maximized()
	Unmaximized()
	EnteredFullScreen()
	LeftFullScreen()
	// TitleBarChanged is called when Window.TitleBar changes, e.g. when the
	// window enters full screen or the desktop's button layout changes.
	TitleBarChanged()

	// Message delivers a message posted by the renderer bridge.
	Message(msg string)
	// WillNavigate is called before a navigation; returning false cancels it.
	WillNavigate(nav Navigation) bool
	NavigationStarted(url string)
	NavigationCommitted(url string)
	LoadFinished()
	LoadFailed(url string, code int, description string)
	TitleChanged(title string)
	// NewWindow is called for window.open() and target=_blank links. It
	// returns the window that should host the new page or nil to deny it.
	NewWindow(req NewWindowRequest) Window
	// ClosedByPage is called when the page calls window.close().
	ClosedByPage()
	RenderProcessGone(reason string)
	SchemeRequest(req *SchemeRequest)
	// DownloadStarted returns where to save a download, "" to cancel it;
	// DownloadFinished reports how it ended.
	DownloadStarted(url, suggestedName string) string
	DownloadFinished(url, path string, err error)
	// SchemeDownload downloads a URL of a custom scheme, which the engine
	// cannot turn into a download, by serving it again.
	SchemeDownload(url string)
	// PermissionRequested asks whether the page at origin may use kinds:
	// "camera", "microphone", "geolocation" or "notifications".
	PermissionRequested(kinds []string, origin string) bool

	// SurfaceEvent delivers input on the window's Surface, and changes of
	// it. For FileDragOver and FileDrop it reports whether the content
	// takes the files where they are, and for KeyPressed whether it took
	// the key itself, which the system then leaves alone (Windows opens no
	// menu for Alt and the key); it returns false for other events.
	SurfaceEvent(ev SurfaceEvent) bool
}

// Navigation describes a pending navigation.
type Navigation struct {
	URL           string
	IsMainFrame   bool
	UserInitiated bool
	// IsReload is true for reloads and history navigations that the app
	// itself did not start.
	IsReload bool
}

// NewWindowRequest describes a window.open() or target=_blank request.
type NewWindowRequest struct {
	URL         string
	FrameName   string
	Disposition string
	// Width/Height/X/Y come from window.open() features; zero when absent.
	X, Y, Width, Height int
	// Native must be copied into WindowOptions.Native of the window that is
	// returned.
	Native uintptr
}

// SchemeRequest is a request for a custom protocol.
type SchemeRequest struct {
	// Context is cancelled when the webview no longer needs the response.
	Context context.Context
	Method  string
	URL     string
	Header  http.Header
	// Body is nil for requests without a body.
	Body io.ReadCloser
	// Responder methods must be called on the main thread, Respond first.
	Responder SchemeResponder
}

// SchemeResponder delivers a custom protocol response to the webview.
type SchemeResponder interface {
	Respond(status int, header http.Header)
	Write(p []byte)
	Finish()
	Fail(err error)
}

// SchemeBodyWriter is implemented by SchemeResponders that take the body
// from the goroutine serving the request instead of through Write,
// blocking while the webview has not read enough of it: a large response
// then takes little memory however slowly the webview reads it.
type SchemeBodyWriter interface {
	// WriteBody writes p once Respond has run. It is called off the main
	// thread, never at the same time as the other methods, and fails
	// once the webview no longer reads the response.
	WriteBody(p []byte) error
}

// MenuItemType is the kind of a menu item.
type MenuItemType int

// Menu item kinds.
const (
	MenuItemNormal MenuItemType = iota
	MenuItemSeparator
	MenuItemSubmenu
	MenuItemCheckbox
	MenuItemRadio
)

// Menu is a snapshot of a menu tree.
type Menu struct {
	Items []*MenuItem
}

// MenuItem is a snapshot of a menu item. ID identifies the public item the
// native item was built from.
type MenuItem struct {
	ID          int
	Type        MenuItemType
	Label       string
	Role        string
	Accelerator string
	Enabled     bool
	Visible     bool
	Checked     bool
	ToolTip     string
	Submenu     *Menu
}

// FileFilter restricts the files shown in file dialogs.
type FileFilter struct {
	Name       string
	Extensions []string
}

// OpenDialogOptions configures an open panel.
type OpenDialogOptions struct {
	Title                      string
	DefaultPath                string
	ButtonLabel                string
	Message                    string
	Filters                    []FileFilter
	OpenFiles                  bool
	OpenDirectories            bool
	Multiple                   bool
	ShowHiddenFiles            bool
	CreateDirectories          bool
	TreatPackagesAsDirectories bool
}

// SaveDialogOptions configures a save panel.
type SaveDialogOptions struct {
	Title                      string
	DefaultPath                string
	ButtonLabel                string
	Message                    string
	NameFieldLabel             string
	Filters                    []FileFilter
	ShowHiddenFiles            bool
	CreateDirectories          bool
	TreatPackagesAsDirectories bool
}

// MessageBoxOptions configures a message box.
type MessageBoxOptions struct {
	// Type is "none", "info", "error", "question" or "warning".
	Type            string
	Title           string
	Message         string
	Detail          string
	Buttons         []string
	DefaultID       int
	CancelID        int
	CheckboxLabel   string
	CheckboxChecked bool
}

// MessageBoxResult is the outcome of a message box.
type MessageBoxResult struct {
	Response        int
	CheckboxChecked bool
}

// Dialogs shows native dialogs. Callbacks run on the main thread exactly
// once. parent may be nil for application modal dialogs.
type Dialogs interface {
	ShowOpenDialog(parent Window, opts *OpenDialogOptions, cb func(paths []string, err error))
	ShowSaveDialog(parent Window, opts *SaveDialogOptions, cb func(path string, err error))
	ShowMessageBox(parent Window, opts *MessageBoxOptions, cb func(res MessageBoxResult, err error))
}

// Clipboard accesses the system clipboard.
type Clipboard interface {
	// Data reads copy accepted representations, so returned data outlives
	// native ownership. Nil formats reads all; a missing explicit match is
	// transfer.ErrFormat. WriteData takes an application-owned snapshot;
	// released runs once when native providers are no longer needed, never
	// after a failed write. Neither operation depends on a source window.
	ReadData(formats []transfer.Format) (transfer.Data, error)
	WriteData(data transfer.Data, released func()) error
	Formats() []transfer.Format
	// Flush materializes owned providers and asks the system to persist the
	// data. Close drops any remaining owned providers at app shutdown.
	Flush() error
	Close()
	ReadText() string
	WriteText(text string)
	ReadHTML() string
	WriteHTML(markup string)
	ReadImage() []byte
	WriteImage(png []byte) error
	Clear()
	AvailableFormats() []string
}

// Shell integrates with the desktop environment.
type Shell interface {
	OpenExternal(url string) error
	OpenPath(path string) error
	ShowItemInFolder(path string)
	TrashItem(path string) error
	Beep()
}

// Display describes a monitor.
type Display struct {
	ID          int64
	Label       string
	Bounds      Rect
	WorkArea    Rect
	ScaleFactor float64
	Rotation    int
	Internal    bool
	Primary     bool
}

// Screen queries monitors.
type Screen interface {
	Displays() []Display
	CursorPoint() Point
}

// Theme reads and overrides the system appearance.
type Theme interface {
	IsDark() bool
	// SetSource is "system", "light" or "dark".
	SetSource(source string)
	// UIFont returns the family of the desktop's interface font where the
	// system's text stack does not know it, as on Linux, else "".
	UIFont() string
	// FontRendering returns how the desktop's settings say to rasterize
	// text where the system's text stack does not know them, as on Linux.
	FontRendering() FontRendering
	// Preferences returns the settings of the desktop that the system's
	// controls follow. A change calls Handler.ThemeChanged.
	Preferences() Preferences
}

// Preferences are settings of the desktop that the system's own controls
// follow, and native UI with them.
type Preferences struct {
	// Accent is the accent color the user chose; A is 0 where the desktop
	// has none.
	Accent Color
	// ReduceMotion asks for less motion: macOS's Reduce Motion, Windows's
	// animation effects and GNOME's animations turned off.
	ReduceMotion bool
	// HighContrast asks for more contrast: macOS's Increase Contrast,
	// Windows's contrast themes, the desktop portal's higher contrast.
	HighContrast bool
	// TextScale is how many times larger than usual text should be, as
	// Windows's and GNOME's text size settings say; 0 or 1 for usual.
	TextScale float64
}

// FontRendering is how the desktop's settings say to rasterize text, as
// GTK's gtk-xft settings: each field is "" for the default, or unknown.
type FontRendering struct {
	// Antialias is "none", "gray" or "subpixel".
	Antialias string
	// Hinting is "none", "slight", "medium" or "full".
	Hinting string
	// Subpixels is the order of the screen's subpixels: "rgb", "bgr",
	// "vrgb" or "vbgr".
	Subpixels string
}

// PDFOptions configures Window.PrintToPDF; lengths are in inches.
type PDFOptions struct {
	Landscape                                        bool
	PageWidth, PageHeight                            float64
	MarginTop, MarginRight, MarginBottom, MarginLeft float64
	Background                                       bool
}

// Power reports power and session events and keeps the computer awake.
type Power interface {
	// Watch starts reporting AppHandler.PowerEvent.
	Watch()
	// KeepAwake keeps the system, and with display the display, from
	// sleeping while idle until release is called.
	KeepAwake(display bool, reason string) (release func())
	OnBattery() bool
	IdleTime() time.Duration
}

// TrayHandler receives tray icon events.
type TrayHandler interface {
	Clicked()
	RightClicked()
}

// Tray is a status bar / notification area icon.
type Tray interface {
	SetImage(png []byte, template bool) error
	SetTitle(title string)
	SetToolTip(tip string)
	SetMenu(m *Menu)
	PopUpMenu(m *Menu)
	Bounds() Rect
	Destroy()
}

// Notification is a desktop notification.
type Notification struct {
	ID       string
	Title    string
	Subtitle string
	Body     string
	Silent   bool
	// Group gathers the notifications that share it (threadIdentifier on
	// macOS).
	Group string
}
