// Package fake is an in-memory platform.Backend for testing the public API
// without a GUI. Its event loop runs on the goroutine that calls Run, which
// is treated as the main thread.
package fake

import (
	"bytes"
	"cmp"
	"errors"
	"runtime"
	"strconv"
	"sync"
	"time"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
)

// ErrNoReply makes Window.CallAsyncFunction never call back, like a
// promise that never settles.
var ErrNoReply = errors.New("fake: no reply")

// Backend is a fake platform backend.
type Backend struct {
	h      platform.AppHandler
	mainID uint64
	signal chan struct{}
	wake   chan struct{}
	quit   chan struct{}

	mu               sync.Mutex
	windows          []*Window
	appMenu          *platform.Menu
	updates          []*platform.MenuItem
	hotkeys          map[int]string
	clipboard        transfer.Data
	clipboardRelease func()
	// ClipboardError injects a failed write while preserving existing ownership.
	ClipboardError error
	// ClipboardDuringFlush simulates a toolkit pumping a nested event loop
	// after releasing providers but before its storage operation returns.
	ClipboardDuringFlush func()
	theme                string
	prefs                platform.Preferences
	// Watching reports whether power events were asked for, Awake counts the
	// KeepAwake calls not released yet.
	Watching bool
	Awake    int
	// URLSchemes are the registered URL schemes, by scheme: "id name".
	URLSchemes map[string]string
	// LoginItem is the command that starts the app at login: "id name arg".
	LoginItem string
	// DockMenu is the menu of the Dock icon.
	DockMenu *platform.Menu
	// Cleared counts ClearBrowsingData calls.
	Cleared int
	// Dialog results returned by the next dialog.
	OpenResult    []string
	SaveResult    string
	MessageResult platform.MessageBoxResult
	// NotificationError, when set, is what showing a notification fails
	// with.
	NotificationError error
	// notifications are the notifications shown and not yet removed, by
	// id.
	notifications map[string]*platform.Notification
}

// New creates a fake backend.
func New() *Backend {
	return &Backend{
		signal:  make(chan struct{}, 1),
		wake:    make(chan struct{}, 1),
		quit:    make(chan struct{}),
		hotkeys: map[int]string{},
	}
}

func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	b := bytes.TrimPrefix(buf[:n], []byte("goroutine "))
	b = b[:bytes.IndexByte(b, ' ')]
	id, _ := strconv.ParseUint(string(b), 10, 64)
	return id
}

func (b *Backend) Name() string { return "fake" }

func (b *Backend) Init(h platform.AppHandler, _ platform.AppOptions) error {
	b.h = h
	b.mainID = goroutineID()
	return nil
}

func (b *Backend) Run() error {
	b.h.Ready()
	for {
		select {
		case <-b.quit:
			return nil
		case <-b.signal:
			b.h.Dispatch()
		case <-b.wake:
		}
	}
}

func (b *Backend) Quit() {
	select {
	case <-b.quit:
	default:
		close(b.quit)
	}
}

func (b *Backend) IsMainThread() bool { return b.mainID == 0 || goroutineID() == b.mainID }

func (b *Backend) Signal() {
	select {
	case b.signal <- struct{}{}:
	default:
	}
}

func (b *Backend) Step() {
	select {
	case <-b.signal:
		b.h.Dispatch()
	case <-b.wake:
	case <-time.After(50 * time.Millisecond):
	}
}

func (b *Backend) Wake() {
	select {
	case b.wake <- struct{}{}:
	default:
	}
}

// QuitRequested simulates Cmd+Q and reports whether the app agreed.
func (b *Backend) QuitRequested() bool {
	if b.h.QuitRequested() {
		b.Quit()
		return true
	}
	return false
}

// Windows returns the windows created so far.
func (b *Backend) Windows() []*Window {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*Window(nil), b.windows...)
}

func (b *Backend) NewWindow(o *platform.WindowOptions, h platform.WindowHandler) (platform.Window, error) {
	w := &Window{b: b, H: h, Opts: o, title: o.Title, zoom: o.Zoom, bounds: platform.Rect{X: o.X, Y: o.Y, Width: o.Width, Height: o.Height}, maximized: o.Maximized, full: o.FullScreen}
	if o.Surface {
		w.surface = &Surface{w: w, Scale: 1}
	}
	if !o.Frameless && (o.TitleBarStyle == "hidden" || o.TitleBarStyle == "hiddenInset") {
		// Three buttons 46 wide at the top right, as on Windows.
		w.TitleBarRoom = platform.TitleBar{Height: cmp.Or(o.TitleBarHeight, 32), Right: 138}
	}
	b.mu.Lock()
	b.windows = append(b.windows, w)
	b.mu.Unlock()
	return w, nil
}

// AppMenu returns the last application menu.
func (b *Backend) AppMenu() *platform.Menu {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.appMenu
}

// MenuUpdates returns the items passed to UpdateMenuItem.
func (b *Backend) MenuUpdates() []*platform.MenuItem {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]*platform.MenuItem(nil), b.updates...)
}

// ClickMenuItem simulates choosing a menu item.
func (b *Backend) ClickMenuItem(id int) { b.h.MenuItemClicked(id) }

// ClickMenuItem simulates choosing an item of this window's menu.
func (w *Window) ClickMenuItem(id int) { w.H.MenuItemClicked(id) }

func (b *Backend) SetApplicationMenu(m *platform.Menu) {
	b.mu.Lock()
	b.appMenu = m
	b.mu.Unlock()
}

func (b *Backend) PopupMenu(*platform.Menu, platform.Window, *platform.Point) {}

func (b *Backend) UpdateMenuItem(it *platform.MenuItem) {
	b.mu.Lock()
	b.updates = append(b.updates, it)
	b.mu.Unlock()
}

func (b *Backend) App() platform.AppController { return app{b} }
func (b *Backend) Dialogs() platform.Dialogs   { return dialogs{b} }
func (b *Backend) Clipboard() platform.Clipboard {
	return clipboard{b}
}
func (b *Backend) Shell() platform.Shell   { return shell{} }
func (b *Backend) Screen() platform.Screen { return screen{} }
func (b *Backend) Theme() platform.Theme   { return theme{b} }
func (b *Backend) Power() platform.Power   { return power{b} }

func (b *Backend) NewTray(platform.TrayHandler) (platform.Tray, error) { return &tray{}, nil }

func (b *Backend) RegisterHotkey(id int, acc string) error {
	b.mu.Lock()
	b.hotkeys[id] = acc
	b.mu.Unlock()
	return nil
}

func (b *Backend) UnregisterHotkey(id int) {
	b.mu.Lock()
	delete(b.hotkeys, id)
	b.mu.Unlock()
}

// PressHotkey simulates a global shortcut.
func (b *Backend) PressHotkey(acc string) {
	b.mu.Lock()
	var hid = -1
	for id, a := range b.hotkeys {
		if a == acc {
			hid = id
		}
	}
	b.mu.Unlock()
	if hid >= 0 {
		b.h.HotkeyPressed(hid)
	}
}

// Notifications are recorded rather than shown, so that a test can see
// what the app asked for and report a click as a platform does.
func (b *Backend) NotificationsSupported() bool { return true }

func (b *Backend) ShowNotification(n *platform.Notification, done func(error)) {
	b.mu.Lock()
	if err := b.NotificationError; err != nil {
		b.mu.Unlock()
		done(err)
		return
	}
	if b.notifications == nil {
		b.notifications = map[string]*platform.Notification{}
	}
	// A copy, as a real backend keeps its own.
	c := *n
	b.notifications[n.ID] = &c
	b.mu.Unlock()
	done(nil)
}

func (b *Backend) RemoveNotification(id string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.notifications, id)
}

func (b *Backend) RemoveAllNotifications() {
	b.mu.Lock()
	defer b.mu.Unlock()
	clear(b.notifications)
}

// Delivered adds a notification the platform still shows from an earlier
// run of the app, which the app did not show in this one.
func (b *Backend) Delivered(n *platform.Notification) {
	b.ShowNotification(n, func(error) {})
}

// Notifications returns the notifications shown and not yet removed.
func (b *Backend) Notifications() []*platform.Notification {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]*platform.Notification, 0, len(b.notifications))
	for _, n := range b.notifications {
		out = append(out, n)
	}
	return out
}

// Notification returns the notification with an id, or nil.
func (b *Backend) Notification(id string) *platform.Notification {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.notifications[id]
}

// ClickNotification reports a click on a notification, as the platform
// does. It runs on the main thread, like the backends report it.
func (b *Backend) ClickNotification(id string) { b.h.NotificationClicked(id) }

// Window is a fake native window. Scripts evaluated in it are recorded.
type Window struct {
	b    *Backend
	H    platform.WindowHandler
	Opts *platform.WindowOptions

	mu        sync.Mutex
	scripts   []string
	title     string
	url       string
	bounds    platform.Rect
	visible   bool
	focused   bool
	minimized bool
	maximized bool
	full      bool
	zoom      float64
	closed    bool
	devtools  bool
	// Dropped is what DroppedFiles returns, once.
	Dropped []string
	// PDF holds the options of the last PrintToPDF.
	PDF platform.PDFOptions
	// What the window extras were last set to.
	Background      platform.Color
	Progress        string
	ProgressValue   float64
	Flashing        bool
	SkipsTaskbar    bool
	OnAllWorkspaces bool
	AutoHidesMenu   bool
	Icon            []byte
	// TitleBarRoom is what TitleBar returns: set for windows with a hidden
	// title bar, and changed by tests before they call H.TitleBarChanged.
	TitleBarRoom platform.TitleBar
	// AsyncFunction answers CallAsyncFunction.
	AsyncFunction func(body string) (string, error)
	// AsyncCallback, when set, receives CallAsyncFunction calls to answer
	// through reply whenever it wants, on the main thread.
	AsyncCallback func(body string, reply func(string, error))
	// OnEval, when set, receives the scripts passed to Eval instead of
	// recording them.
	OnEval func(js string)

	surface *Surface
}

// Surface returns the window's surface, nil unless it was created with
// WindowOptions.Surface.
func (w *Window) Surface() platform.Surface {
	if w.surface == nil {
		return nil
	}
	return w.surface
}

// FakeSurface returns the window's surface, for tests.
func (w *Window) FakeSurface() *Surface { return w.surface }

// Scripts returns the scripts passed to Eval.
func (w *Window) Scripts() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.scripts...)
}

func (w *Window) SetProgressBar(state string, value float64) {
	w.mu.Lock()
	w.Progress, w.ProgressValue = state, value
	w.mu.Unlock()
}
func (w *Window) FlashFrame(v bool)                { w.mu.Lock(); w.Flashing = v; w.mu.Unlock() }
func (w *Window) SetSkipTaskbar(v bool)            { w.mu.Lock(); w.SkipsTaskbar = v; w.mu.Unlock() }
func (w *Window) SetVisibleOnAllWorkspaces(v bool) { w.mu.Lock(); w.OnAllWorkspaces = v; w.mu.Unlock() }
func (w *Window) SetAutoHideMenu(v bool)           { w.mu.Lock(); w.AutoHidesMenu = v; w.mu.Unlock() }
func (w *Window) SetIcon(png []byte) error {
	if png != nil && !bytes.HasPrefix(png, []byte("\x89PNG")) {
		return errors.New("fake: not a PNG image")
	}
	w.mu.Lock()
	w.Icon = png
	w.mu.Unlock()
	return nil
}

func (w *Window) PrintToPDF(o platform.PDFOptions, cb func([]byte, error)) {
	w.mu.Lock()
	w.PDF = o
	w.mu.Unlock()
	cb([]byte("%PDF-1.4 fake"), nil)
}

func (w *Window) DroppedFiles() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	paths := w.Dropped
	w.Dropped = nil
	return paths
}

// IsClosed reports whether Close was called.
func (w *Window) IsClosed() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.closed
}

func (w *Window) TitleBar() platform.TitleBar {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.TitleBarRoom
}

func (w *Window) Handle() uintptr        { return 1 }
func (w *Window) WebViewHandle() uintptr { return 2 }
func (w *Window) SetTitle(t string)      { w.mu.Lock(); w.title = t; w.mu.Unlock() }
func (w *Window) Title() string          { w.mu.Lock(); defer w.mu.Unlock(); return w.title }
func (w *Window) SetBackgroundColor(c platform.Color) {
	w.mu.Lock()
	w.Background = c
	w.mu.Unlock()
}

func (w *Window) SetBounds(r platform.Rect) {
	w.mu.Lock()
	w.bounds = r
	w.mu.Unlock()
	w.H.Resized()
}
func (w *Window) Bounds() platform.Rect              { w.mu.Lock(); defer w.mu.Unlock(); return w.bounds }
func (w *Window) SetContentBounds(r platform.Rect)   { w.SetBounds(r) }
func (w *Window) ContentBounds() platform.Rect       { return w.Bounds() }
func (w *Window) SetMinimumSize(platform.Size)       {}
func (w *Window) SetMaximumSize(platform.Size)       {}
func (w *Window) SetResizable(bool)                  {}
func (w *Window) IsResizable() bool                  { return w.Opts.Resizable }
func (w *Window) SetMovable(bool)                    {}
func (w *Window) IsMovable() bool                    { return w.Opts.Movable }
func (w *Window) SetMinimizable(bool)                {}
func (w *Window) IsMinimizable() bool                { return w.Opts.Minimizable }
func (w *Window) SetMaximizable(bool)                {}
func (w *Window) IsMaximizable() bool                { return w.Opts.Maximizable }
func (w *Window) SetClosable(bool)                   {}
func (w *Window) IsClosable() bool                   { return w.Opts.Closable }
func (w *Window) SetAlwaysOnTop(bool)                {}
func (w *Window) IsAlwaysOnTop() bool                { return w.Opts.AlwaysOnTop }
func (w *Window) Show()                              { w.mu.Lock(); w.visible, w.focused = true, true; w.mu.Unlock() }
func (w *Window) ShowInactive()                      { w.mu.Lock(); w.visible = true; w.mu.Unlock() }
func (w *Window) Hide()                              { w.mu.Lock(); w.visible = false; w.mu.Unlock() }
func (w *Window) IsVisible() bool                    { w.mu.Lock(); defer w.mu.Unlock(); return w.visible }
func (w *Window) Focus()                             { w.H.Focused() }
func (w *Window) Blur()                              { w.H.Blurred() }
func (w *Window) IsFocused() bool                    { w.mu.Lock(); defer w.mu.Unlock(); return w.focused }
func (w *Window) Minimize()                          { w.mu.Lock(); w.minimized = true; w.mu.Unlock(); w.H.Minimized() }
func (w *Window) IsMinimized() bool                  { w.mu.Lock(); defer w.mu.Unlock(); return w.minimized }
func (w *Window) Maximize()                          { w.mu.Lock(); w.maximized = true; w.mu.Unlock(); w.H.Maximized() }
func (w *Window) Unmaximize()                        { w.mu.Lock(); w.maximized = false; w.mu.Unlock(); w.H.Unmaximized() }
func (w *Window) IsMaximized() bool                  { w.mu.Lock(); defer w.mu.Unlock(); return w.maximized }
func (w *Window) Restore()                           { w.mu.Lock(); w.minimized = false; w.mu.Unlock(); w.H.Restored() }
func (w *Window) SetFullScreen(v bool)               { w.mu.Lock(); w.full = v; w.mu.Unlock() }
func (w *Window) IsFullScreen() bool                 { w.mu.Lock(); defer w.mu.Unlock(); return w.full }
func (w *Window) Center()                            {}
func (w *Window) SetOpacity(float64)                 {}
func (w *Window) Opacity() float64                   { return 1 }
func (w *Window) SetHasShadow(bool)                  {}
func (w *Window) HasShadow() bool                    { return true }
func (w *Window) SetIgnoreMouseEvents(bool)          {}
func (w *Window) SetContentProtection(bool)          {}
func (w *Window) SetVibrancy(string)                 {}
func (w *Window) SetMenu(*platform.Menu)             {}
func (w *Window) StartDrag()                         {}
func (w *Window) TitleBarDoubleClicked()             {}
func (w *Window) LoadURL(url string)                 { w.mu.Lock(); w.url = url; w.mu.Unlock() }
func (w *Window) LoadHTML(string, string)            { w.mu.Lock(); w.url = "about:blank"; w.mu.Unlock() }
func (w *Window) LoadFile(path, _ string)            { w.mu.Lock(); w.url = "file://" + path; w.mu.Unlock() }
func (w *Window) Reload(bool)                        {}
func (w *Window) StopLoading()                       {}
func (w *Window) GoBack()                            {}
func (w *Window) GoForward()                         {}
func (w *Window) CanGoBack() bool                    { return false }
func (w *Window) CanGoForward() bool                 { return false }
func (w *Window) URL() string                        { w.mu.Lock(); defer w.mu.Unlock(); return w.url }
func (w *Window) IsLoading() bool                    { return false }
func (w *Window) SetZoom(f float64)                  { w.mu.Lock(); w.zoom = f; w.mu.Unlock() }
func (w *Window) Zoom() float64                      { w.mu.Lock(); defer w.mu.Unlock(); return w.zoom }
func (w *Window) SetUserAgent(string)                {}
func (w *Window) UserAgent() string                  { return "fake" }
func (w *Window) OpenDevTools()                      { w.mu.Lock(); w.devtools = true; w.mu.Unlock() }
func (w *Window) CloseDevTools()                     { w.mu.Lock(); w.devtools = false; w.mu.Unlock() }
func (w *Window) IsDevToolsOpened() bool             { w.mu.Lock(); defer w.mu.Unlock(); return w.devtools }
func (w *Window) CapturePage(cb func([]byte, error)) { cb([]byte("png"), nil) }
func (w *Window) Print()                             {}

func (w *Window) Eval(js string) {
	if w.OnEval != nil {
		w.OnEval(js)
		return
	}
	w.mu.Lock()
	w.scripts = append(w.scripts, js)
	w.mu.Unlock()
}

func (w *Window) CallAsyncFunction(body string, cb func(string, error)) {
	if w.AsyncCallback != nil {
		w.AsyncCallback(body, cb)
		return
	}
	if w.AsyncFunction == nil {
		cb("", platform.ErrUnsupported)
		return
	}
	res, err := w.AsyncFunction(body)
	if err == ErrNoReply {
		return
	}
	cb(res, err)
}

func (w *Window) Close() {
	w.mu.Lock()
	if w.closed {
		w.mu.Unlock()
		return
	}
	w.closed = true
	w.mu.Unlock()
	w.H.Closed()
}

// UserClose simulates a click on the close button.
func (w *Window) UserClose() bool {
	if !w.H.ShouldClose() {
		return false
	}
	w.Close()
	return true
}

type app struct{ b *Backend }

func (app) SetActivationPolicy(string)                {}
func (app) Activate()                                 {}
func (app) Hide()                                     {}
func (app) Unhide()                                   {}
func (app) IsHidden() bool                            { return false }
func (app) SetBadge(string)                           {}
func (app) Badge() string                             { return "" }
func (app) Bounce(bool) int                           { return 0 }
func (app) CancelBounce(int)                          {}
func (app) SetDockIcon([]byte) error                  { return nil }
func (app) ShowAboutPanel(platform.AboutPanelOptions) {}
func (app) Locale() string                            { return "en-US" }
func (app) Package() (platform.PackageInfo, bool)     { return platform.PackageInfo{}, false }

func (a app) RegisterURLScheme(scheme, id, name string) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	if a.b.URLSchemes == nil {
		a.b.URLSchemes = map[string]string{}
	}
	a.b.URLSchemes[scheme] = id + " " + name
	return nil
}

func (a app) UnregisterURLScheme(scheme, id, name string) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	delete(a.b.URLSchemes, scheme)
	return nil
}

func (a app) SetOpenAtLogin(open bool, id, name, arg string) error {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	a.b.LoginItem = ""
	if open {
		a.b.LoginItem = id + " " + name + " " + arg
	}
	return nil
}

func (a app) OpenAtLogin(id, name, arg string) bool {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	return a.b.LoginItem == id+" "+name+" "+arg
}

func (app) OpenedAtLogin() bool { return false }

func (a app) SetDockMenu(m *platform.Menu) { a.b.mu.Lock(); a.b.DockMenu = m; a.b.mu.Unlock() }

func (a app) ClearBrowsingData(done func(error)) {
	a.b.mu.Lock()
	a.b.Cleared++
	a.b.mu.Unlock()
	done(nil)
}

func (a app) IsURLSchemeRegistered(scheme, id, name string) bool {
	a.b.mu.Lock()
	defer a.b.mu.Unlock()
	return a.b.URLSchemes[scheme] == id+" "+name
}

type dialogs struct{ b *Backend }

func (d dialogs) ShowOpenDialog(_ platform.Window, _ *platform.OpenDialogOptions, cb func([]string, error)) {
	cb(d.b.OpenResult, nil)
}
func (d dialogs) ShowSaveDialog(_ platform.Window, _ *platform.SaveDialogOptions, cb func(string, error)) {
	cb(d.b.SaveResult, nil)
}
func (d dialogs) ShowMessageBox(_ platform.Window, _ *platform.MessageBoxOptions, cb func(platform.MessageBoxResult, error)) {
	cb(d.b.MessageResult, nil)
}

type clipboard struct{ b *Backend }

func (c clipboard) ReadText() string   { b, _ := c.b.clipboard.Read(transfer.Text); return string(b) }
func (c clipboard) WriteText(s string) { _ = c.WriteData(transfer.TextData(s), nil) }
func (c clipboard) ReadHTML() string   { b, _ := c.b.clipboard.Read(transfer.HTML); return string(b) }
func (c clipboard) WriteHTML(s string) {
	_ = c.WriteData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.HTML, []byte(s)))), nil)
}
func (c clipboard) ReadImage() []byte { b, _ := c.b.clipboard.Read(transfer.PNG); return b }
func (c clipboard) WriteImage(b []byte) error {
	return c.WriteData(transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, b))), nil)
}
func (c clipboard) Clear() { _ = c.WriteData(transfer.Data{}, nil) }
func (c clipboard) AvailableFormats() []string {
	var out []string
	for _, f := range c.Formats() {
		out = append(out, string(f))
	}
	return out
}
func (c clipboard) Formats() []transfer.Format { return c.b.clipboard.Formats() }
func (c clipboard) ReadData(formats []transfer.Format) (transfer.Data, error) {
	if len(formats) == 0 {
		formats = c.Formats()
		if len(formats) == 0 {
			return transfer.Data{}, nil
		}
	}
	return c.b.clipboard.Materialize(formats)
}
func (c clipboard) WriteData(d transfer.Data, released func()) error {
	if c.b.ClipboardError != nil {
		return c.b.ClipboardError
	}
	previous := c.b.clipboardRelease
	c.b.clipboard, c.b.clipboardRelease = d, released
	if previous != nil {
		previous()
	}
	if len(d.Formats()) == 0 {
		c.release()
	}
	return nil
}
func (c clipboard) release() {
	if fn := c.b.clipboardRelease; fn != nil {
		c.b.clipboardRelease = nil
		fn()
	}
}
func (c clipboard) Flush() error {
	if len(c.Formats()) == 0 {
		return nil
	}
	d, err := c.ReadData(nil)
	if err != nil {
		return err
	}
	c.b.clipboard = d
	c.release()
	if fn := c.b.ClipboardDuringFlush; fn != nil {
		fn()
	}
	return nil
}
func (c clipboard) Close() {
	if c.b.clipboardRelease != nil {
		c.b.clipboard = transfer.Data{}
		c.release()
	}
}

type shell struct{}

func (shell) OpenExternal(string) error { return nil }
func (shell) OpenPath(string) error     { return nil }
func (shell) ShowItemInFolder(string)   {}
func (shell) TrashItem(string) error    { return nil }
func (shell) Beep()                     {}

type power struct{ b *Backend }

func (p power) Watch() { p.b.mu.Lock(); p.b.Watching = true; p.b.mu.Unlock() }
func (p power) KeepAwake(display bool, reason string) func() {
	p.b.mu.Lock()
	p.b.Awake++
	p.b.mu.Unlock()
	return func() { p.b.mu.Lock(); p.b.Awake--; p.b.mu.Unlock() }
}
func (power) OnBattery() bool         { return false }
func (power) IdleTime() time.Duration { return 42 * time.Second }

// EmitPower simulates a power or session event.
func (b *Backend) EmitPower(event string) { b.h.PowerEvent(event) }

type screen struct{}

func (screen) Displays() []platform.Display {
	return []platform.Display{{
		ID: 1, Label: "Fake", Primary: true, ScaleFactor: 2,
		Bounds:   platform.Rect{Width: 1440, Height: 900},
		WorkArea: platform.Rect{Y: 25, Width: 1440, Height: 875},
	}}
}
func (screen) CursorPoint() platform.Point { return platform.Point{X: 10, Y: 20} }

type theme struct{ b *Backend }

func (t theme) IsDark() bool {
	t.b.mu.Lock()
	defer t.b.mu.Unlock()
	return t.b.theme == "dark"
}
func (theme) UIFont() string { return "" }

func (theme) FontRendering() platform.FontRendering { return platform.FontRendering{} }

func (t theme) SetSource(s string) {
	t.b.mu.Lock()
	t.b.theme = s
	t.b.mu.Unlock()
	t.b.h.ThemeChanged()
}

func (t theme) Preferences() platform.Preferences {
	t.b.mu.Lock()
	defer t.b.mu.Unlock()
	return t.b.prefs
}

// SetPreferences changes the desktop's preferences, as the user does in
// the system's settings.
func (b *Backend) SetPreferences(p platform.Preferences) {
	b.mu.Lock()
	b.prefs = p
	b.mu.Unlock()
	b.h.ThemeChanged()
}

type tray struct{}

func (*tray) SetImage([]byte, bool) error { return nil }
func (*tray) SetTitle(string)             {}
func (*tray) SetToolTip(string)           {}
func (*tray) SetMenu(*platform.Menu)      {}
func (*tray) PopUpMenu(*platform.Menu)    {}
func (*tray) Bounds() platform.Rect       { return platform.Rect{} }
func (*tray) Destroy()                    {}

// Surface is a fake window surface: it records what the content asks of
// it, and tests deliver events through Send.
type Surface struct {
	w *Window
	// Scale is the device pixels per DIP.
	Scale float64

	mu          sync.Mutex
	requests    int
	frames      int
	pixels      []byte
	pixW        int
	pixH        int
	cursor      platform.Cursor
	textInput   platform.TextInputState
	access      *platform.AccessTree
	accessN     int
	rate        float64
	dataDrag    *platform.DragRequest
	dropFormats []transfer.Format
}

func (s *Surface) StartDataDrag(r platform.DragRequest) { s.dataDrag = &r }
func (s *Surface) CancelDataDrag()                      { s.FinishDataDrag(transfer.Result{Canceled: true}) }
func (s *Surface) SetDropFormats(f []transfer.Format) {
	s.dropFormats = append([]transfer.Format(nil), f...)
}

// DataDrag returns the active native-source request. Main thread only.
func (s *Surface) DataDrag() *platform.DragRequest { return s.dataDrag }

// FinishDataDrag delivers the native source's result once. Main thread only.
func (s *Surface) FinishDataDrag(r transfer.Result) {
	if d := s.dataDrag; d != nil {
		s.dataDrag = nil
		d.Done(r)
	}
}

func (s *Surface) Native() platform.SurfaceNative { return platform.SurfaceNative{} }

func (s *Surface) Size() (float64, float64, float64) {
	s.w.mu.Lock()
	defer s.w.mu.Unlock()
	return float64(s.w.bounds.Width), float64(s.w.bounds.Height), s.Scale
}

// RefreshRate returns the display's refresh rate, 60 unless
// SetRefreshRate set another.
func (s *Surface) RefreshRate() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.rate == 0 {
		return 60
	}
	return s.rate
}

// SetRefreshRate sets the display's refresh rate.
func (s *Surface) SetRefreshRate(hz float64) {
	s.mu.Lock()
	s.rate = hz
	s.mu.Unlock()
}

func (s *Surface) RequestFrame() {
	s.mu.Lock()
	s.requests++
	s.mu.Unlock()
}

func (s *Surface) PresentPixels(pix []byte, stride, width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frames++
	s.pixW, s.pixH = width, height
	s.pixels = make([]byte, 4*width*height)
	for y := 0; y < height; y++ {
		copy(s.pixels[y*4*width:(y+1)*4*width], pix[y*stride:])
	}
}

func (s *Surface) SetCursor(c platform.Cursor) {
	s.mu.Lock()
	s.cursor = c
	s.mu.Unlock()
}

func (s *Surface) SetTextInput(t platform.TextInputState) {
	s.mu.Lock()
	s.textInput = t
	s.mu.Unlock()
}

func (s *Surface) UpdateAccessibility(tree *platform.AccessTree) {
	s.mu.Lock()
	s.access = tree
	s.accessN++
	s.mu.Unlock()
}

// Send delivers an event to the window's content, on the main thread, and
// returns what the content answered.
func (s *Surface) Send(ev platform.SurfaceEvent) bool { return s.w.H.SurfaceEvent(ev) }

// Frame draws a frame when the content asked for one since the last, as
// the display would, and reports whether it did.
func (s *Surface) Frame() bool {
	s.mu.Lock()
	pending := s.requests > 0
	s.requests = 0
	s.mu.Unlock()
	if pending {
		s.Send(platform.SurfaceEvent{Kind: platform.SurfaceFrame})
	}
	return pending
}

// Frames returns how many frames were presented.
func (s *Surface) Frames() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.frames
}

// Pixel returns the color of a pixel of the last frame, as BGRA.
func (s *Surface) Pixel(x, y int) [4]byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if x < 0 || y < 0 || x >= s.pixW || y >= s.pixH {
		return [4]byte{}
	}
	p := s.pixels[(y*s.pixW+x)*4:]
	return [4]byte{p[0], p[1], p[2], p[3]}
}

// Cursor returns the pointer shape the content set; TextInput whether it
// asked for text input.
func (s *Surface) Cursor() platform.Cursor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cursor
}

// TextInput reports whether text input is on, and the caret.
func (s *Surface) TextInput() (bool, platform.RectF) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.textInput.Active, s.textInput.Caret
}

// TextInputState returns the last state of text input the content set.
func (s *Surface) TextInputState() platform.TextInputState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.textInput
}

// Accessibility returns the last accessibility tree the content gave, and
// how many it gave.
func (s *Surface) Accessibility() (*platform.AccessTree, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.access, s.accessN
}
