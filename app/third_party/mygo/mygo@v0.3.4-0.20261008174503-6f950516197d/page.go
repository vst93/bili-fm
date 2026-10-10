package mygo

// Page is the web page a window shows: what it loads, the scripts it runs,
// its developer tools and its events. A window showing native UI
// (WindowOptions.Content) has none.
//
//	win := mygo.NewWindow(mygo.WindowOptions{Title: "Editor", URL: "/"})
//	win.Page().OnDOMReady(func() { log.Println("ready") })
//
// Like the window's, its methods are safe from any goroutine.
type Page struct{ w *Window }

// Page returns the web page the window shows, or nil for a window that
// shows native UI.
func (w *Window) Page() *Page {
	if w.content != nil {
		return nil
	}
	return w.pg
}

// Window returns the window that shows the page.
func (p *Page) Window() *Window { return p.w }

// Autoplay decides whether a window's page may start audible media without a
// user gesture.
type Autoplay int

const (
	// AutoplayUserGesture, the zero value, is what the engines do by default:
	// a page plays audible media only once the user has interacted with it.
	// A window that is never shown never gets a gesture, so a hidden window
	// cannot play sound with this.
	AutoplayUserGesture Autoplay = iota
	// AutoplayAllow lets the page start audible media on its own, which a
	// window that is hidden needs. Starting muted and unmuting afterwards is
	// not a substitute: where the gesture is still required, unmuting pauses
	// the element again.
	//
	// On Windows the setting reaches WebView2 as the browser argument
	// --autoplay-policy=no-user-gesture-required, which the WebView2
	// environment carries, and the environment is shared by every window of
	// the app. The first window that shows a page decides there, so an app
	// whose pages need it should ask on that window, or on all of them.
	// macOS and Linux set it on the window's own web view.
	AutoplayAllow
)

// PageOptions configure the web page of a window.
type PageOptions struct {
	// PreloadScript is JavaScript injected into every page before the
	// page's own scripts, after window.mygo is available.
	PreloadScript string
	// TrustedOrigins lists extra origins, such as "https://example.com",
	// whose pages may call bound Go methods. By default only the app's own
	// content can: custom schemes registered with Protocol, file: and
	// about: pages, and loopback dev servers during development. "*"
	// trusts every origin.
	TrustedOrigins []string
	// DevTools controls the web inspector, and for a window showing
	// Content, the inspector of its native UI.
	DevTools DevTools
	// ZoomFactor of the page (default 1).
	ZoomFactor float64
	// UserAgent overrides the user agent string.
	UserAgent string
	// Autoplay decides whether the page may play audible media without a
	// user gesture (default AutoplayUserGesture).
	Autoplay Autoplay
}
