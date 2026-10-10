package mygo

import (
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
)

var fb *fake.Backend

// TestMain runs the application on the main goroutine with the fake
// backend, like a real program, and the tests on another goroutine.
func TestMain(m *testing.M) {
	if os.Getenv("MYGO_TEST_SECOND_INSTANCE") == "1" {
		// Helper process for TestSingleInstance.
		packageIdentifier = os.Getenv("MYGO_TEST_IDENTIFIER")
		App.SetName("MyGoTest")
		if App.RequestSingleInstanceLock() {
			fmt.Println("locked")
			releaseSingleInstanceLock()
		} else {
			fmt.Println("forwarded")
		}
		os.Exit(0)
	}
	fb = fake.New()
	backendOnce.Do(func() { theBackend = fb })
	if os.Getenv("MYGO_TEST_BEFORE_RUN") == "1" {
		beforeRun()
	}
	App.SetName("MyGoTest")
	// Keep running when tests close their windows.
	App.OnWindowAllClosed(func() {})
	App.WhenReady(func() {
		go func() { os.Exit(m.Run()) }()
	})
	if err := App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// testWindow creates a window and returns it with its fake native window.
func testWindow(t *testing.T, opts WindowOptions) (*Window, *fake.Window) {
	t.Helper()
	w := NewWindow(opts)
	t.Cleanup(w.Destroy)
	wins := fb.Windows()
	fw := wins[len(wins)-1]
	// Like a page loaded with LoadHTML.
	onMain(func() { fw.H.NavigationCommitted("about:blank") })
	return w, fw
}

// readyWindow is testWindow for a window whose page is loaded.
func readyWindow(t *testing.T, opts WindowOptions) (*Window, *fake.Window) {
	t.Helper()
	w, fw := testWindow(t, opts)
	page(fw, `{"t":"dom-ready"}`)
	return w, fw
}

// page simulates the page bridge posting a message.
func page(fw *fake.Window, msg string) {
	s := secret(fw)
	onMain(func() { fw.H.Message(s + msg) })
}

// secret returns the secret the bridge of fw prefixes its messages with.
func secret(fw *fake.Window) string {
	m := regexp.MustCompile(`"secret":"([^"]+)"`).FindStringSubmatch(fw.Opts.UserScripts[0].Source)
	if m == nil {
		panic("no secret in the bridge configuration")
	}
	return m[1]
}

// received waits until the page received a message matching pred.
func received(t *testing.T, fw *fake.Window, pred func(m map[string]any) bool) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		for _, s := range fw.Scripts() {
			const prefix = "__mygo.receive("
			if !strings.HasPrefix(s, prefix) {
				continue
			}
			var msgs []map[string]any
			if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(s, prefix), ")")), &msgs); err != nil {
				t.Fatalf("bad script %q: %v", s, err)
			}
			for _, m := range msgs {
				if pred(m) {
					return m
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("message not received; scripts: %q", fw.Scripts())
	return nil
}

func call(t *testing.T, fw *fake.Window, id int, method string, args ...any) map[string]any {
	t.Helper()
	if args == nil {
		args = []any{}
	}
	a, _ := json.Marshal(args)
	page(fw, fmt.Sprintf(`{"t":"call","id":%d,"k":"tok","m":%q,"a":%s}`, id, method, a))
	return received(t, fw, func(m map[string]any) bool {
		return m["t"] == "reply" && m["id"] == float64(id)
	})
}

func TestWindowDefaults(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	o := fw.Opts
	if o.Width != 800 || o.Height != 600 || !o.Center || o.Title != "MyGoTest" {
		t.Errorf("unexpected defaults: %+v", o)
	}
	if !o.Resizable || !o.Closable || !o.Minimizable || !o.Maximizable || !o.HasShadow || o.Opacity != 1 || o.Zoom != 1 {
		t.Errorf("unexpected capability defaults: %+v", o)
	}
	if !o.DevTools {
		t.Error("devtools should be enabled in development")
	}
	if len(o.UserScripts) != 1 || !strings.Contains(o.UserScripts[0].Source, `"windowId":`+fmt.Sprint(w.ID())) {
		t.Errorf("bridge script not configured: %v", o.UserScripts)
	}
	if !w.IsVisible() {
		t.Error("window should be shown by default")
	}
	if WindowByID(w.ID()) != w {
		t.Error("WindowByID")
	}
}

func TestWindowOptions(t *testing.T) {
	parent, _ := testWindow(t, WindowOptions{})
	_, fw := testWindow(t, WindowOptions{
		Title: "Custom", Width: 300, Height: 200, X: 10, Y: 20,
		Hidden: true, DisableResize: true, BackgroundColor: "#11223380",
		Page: PageOptions{DevTools: DevToolsDisabled, PreloadScript: "window.x = 1"}, Parent: parent, URL: "https://example.com/",
	})
	o := fw.Opts
	if o.Title != "Custom" || o.Width != 300 || o.Center || o.Resizable || o.DevTools {
		t.Errorf("options not applied: %+v", o)
	}
	if c := o.BackgroundColor; c == nil || *c != (platform.Color{R: 0x11, G: 0x22, B: 0x33, A: 0x80}) {
		t.Errorf("background color: %v", c)
	}
	if len(o.UserScripts) != 2 || o.UserScripts[1].Source != "window.x = 1" {
		t.Errorf("preload script: %v", o.UserScripts)
	}
	if o.Parent == nil {
		t.Error("parent not passed")
	}
	if fw.IsVisible() {
		t.Error("hidden window was shown")
	}
	if fw.URL() != "https://example.com/" {
		t.Errorf("URL not loaded: %q", fw.URL())
	}
}

func TestInvalidBackgroundColor(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected panic for invalid color")
		}
	}()
	NewWindow(WindowOptions{BackgroundColor: "nope"})
}

func TestBackgroundFollowsTheme(t *testing.T) {
	light, dark := platform.Color{R: 0xf6, G: 0xf7, B: 0xf9, A: 255}, platform.Color{R: 0x0f, G: 0x11, B: 0x15, A: 255}
	w, fw := testWindow(t, WindowOptions{BackgroundColor: "light-dark(#f6f7f9, rgb(15, 17, 21))"})
	if c := fw.Opts.BackgroundColor; c == nil || *c != light {
		t.Errorf("initial background: %v", c)
	}
	Theme.SetSource(ThemeDark)
	defer Theme.SetSource(ThemeSystem)
	if fw.Background != dark {
		t.Errorf("dark background: %v", fw.Background)
	}
	_, dfw := testWindow(t, WindowOptions{BackgroundColor: "light-dark(#f6f7f9, #0f1115)"})
	if c := dfw.Opts.BackgroundColor; c == nil || *c != dark {
		t.Errorf("background of a window created in dark mode: %v", c)
	}
	_, tfw := testWindow(t, WindowOptions{Transparent: true, BackgroundColor: "light-dark(#f6f7f9, #0f1115)"})
	if err := w.SetBackgroundColor("light-dark(#fff, #000)"); err != nil || fw.Background != (platform.Color{A: 255}) {
		t.Errorf("SetBackgroundColor: %v, %v", fw.Background, err)
	}
	Theme.SetSource(ThemeLight)
	if fw.Background != (platform.Color{R: 255, G: 255, B: 255, A: 255}) {
		t.Errorf("light background: %v", fw.Background)
	}
	if tfw.Background != (platform.Color{}) {
		t.Errorf("a theme change gave a transparent window a background: %v", tfw.Background)
	}
	if err := w.SetBackgroundColor("light-dark(#fff)"); err == nil {
		t.Error("SetBackgroundColor accepted an invalid color")
	}
}

func TestCloseCanBePrevented(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	var allow atomic.Bool
	var closed atomic.Int32
	w.OnClose(func(e *CloseEvent) {
		if e.Window != w {
			t.Error("wrong window in CloseEvent")
		}
		if !allow.Load() {
			e.PreventDefault()
		}
	})
	w.OnClosed(func() { closed.Add(1) })

	w.Close()
	if w.IsDestroyed() || fw.IsClosed() {
		t.Fatal("close was not prevented")
	}
	if onMainValue(fw.UserClose) {
		t.Fatal("user close was not prevented")
	}
	allow.Store(true)
	w.Close()
	if !w.IsDestroyed() || !fw.IsClosed() || closed.Load() != 1 {
		t.Fatalf("window not closed: destroyed=%v closed=%v events=%d", w.IsDestroyed(), fw.IsClosed(), closed.Load())
	}
	for _, x := range Windows() {
		if x == w {
			t.Error("closed window still listed")
		}
	}
	// Methods on destroyed windows are no-ops.
	w.SetTitle("ignored")
	if w.Title() != "" || w.IsVisible() {
		t.Error("destroyed window should report zero values")
	}
}

func TestDestroyClosesChildren(t *testing.T) {
	parent, _ := testWindow(t, WindowOptions{})
	child, _ := testWindow(t, WindowOptions{Parent: parent})
	parent.Destroy()
	if !child.IsDestroyed() {
		t.Error("child window survived its parent")
	}
}

func TestWindowStateAndEvents(t *testing.T) {
	w, _ := testWindow(t, WindowOptions{})
	var events []string
	var mu sync.Mutex
	record := func(name string) func() {
		return func() { mu.Lock(); events = append(events, name); mu.Unlock() }
	}
	w.OnMaximize(record("maximize"))
	w.OnUnmaximize(record("unmaximize"))
	w.OnMinimize(record("minimize"))
	w.OnRestore(record("restore"))
	w.OnResize(record("resize"))
	w.OnFocus(record("focus"))

	w.ToggleMaximize()
	if !w.IsMaximized() {
		t.Error("ToggleMaximize did not maximize")
	}
	w.ToggleMaximize()
	w.Minimize()
	w.Restore()
	w.SetSize(640, 480)
	w.Focus()
	if FocusedWindow() != w {
		t.Error("FocusedWindow")
	}
	if width, height := w.Size(); width != 640 || height != 480 {
		t.Errorf("Size = %d,%d", width, height)
	}
	mu.Lock()
	defer mu.Unlock()
	want := "maximize,unmaximize,minimize,restore,resize,focus"
	if got := strings.Join(events, ","); got != want {
		t.Errorf("events = %s, want %s", got, want)
	}
}

func TestPageTitleUpdatesWindowTitle(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Title: "Initial"})
	onMain(func() { fw.H.TitleChanged("From Page") })
	if w.Title() != "From Page" {
		t.Errorf("title = %q", w.Title())
	}
	w.Page().OnPageTitleUpdated(func(e *TitleEvent) { e.PreventDefault() })
	onMain(func() { fw.H.TitleChanged("Ignored") })
	if w.Title() != "From Page" {
		t.Errorf("prevented title changed the window: %q", w.Title())
	}
}

func TestWillNavigate(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	w.Page().OnWillNavigate(func(e *NavigateEvent) {
		if strings.Contains(e.URL, "blocked") {
			e.PreventDefault()
		}
	})
	check := func(nav platform.Navigation, want bool) {
		t.Helper()
		if got := onMainValue(func() bool { return fw.H.WillNavigate(nav) }); got != want {
			t.Errorf("WillNavigate(%+v) = %v, want %v", nav, got, want)
		}
	}
	check(platform.Navigation{URL: "https://ok.example", IsMainFrame: true}, true)
	check(platform.Navigation{URL: "https://blocked.example", IsMainFrame: true}, false)
	check(platform.Navigation{URL: "https://blocked.example", IsMainFrame: false}, true)
	check(platform.Navigation{URL: "https://blocked.example", IsMainFrame: true, IsReload: true}, true)
}

func TestReadyToShowFiresOnce(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Hidden: true})
	var n atomic.Int32
	w.OnReadyToShow(func() { n.Add(1) })
	var dom atomic.Int32
	w.Page().OnDOMReady(func() { dom.Add(1) })
	page(fw, `{"t":"dom-ready"}`)
	page(fw, `{"t":"dom-ready"}`)
	onMain(fw.H.LoadFinished)
	if n.Load() != 1 || dom.Load() != 2 {
		t.Errorf("ready-to-show fired %d times, dom-ready %d times", n.Load(), dom.Load())
	}
}

// Bound services.

type testUser struct {
	Name string   `json:"name"`
	Tags []string `json:"tags"`
}

type testService struct{ calls atomic.Int32 }

func (s *testService) Hello(name string) string { s.calls.Add(1); return "hello " + name }
func (s *testService) Sum(nums ...int) int {
	n := 0
	for _, x := range nums {
		n += x
	}
	return n
}
func (s *testService) Fail() error { return errors.New("nope") }
func (s *testService) Panic()      { panic("boom") }
func (s *testService) Echo(u testUser) (testUser, error) {
	u.Tags = append(u.Tags, "echoed")
	return u, nil
}
func (s *testService) Caller(ctx context.Context) int   { return CallerWindow(ctx).ID() }
func (s *testService) Nil() []string                    { return nil }
func (s *testService) Optional(a string, b *int) string { return fmt.Sprintf("%s:%v", a, b == nil) }

func TestCalls(t *testing.T) {
	svc := &testService{}
	bindForTest(t, "Svc", svc)
	w, fw := testWindow(t, WindowOptions{})

	tests := []struct {
		method string
		args   []any
		ok     bool
		want   any
	}{
		{"Svc.Hello", []any{"bun"}, true, "hello bun"},
		{"Svc.Sum", []any{1, 2, 3}, true, float64(6)},
		{"Svc.Sum", nil, true, float64(0)},
		{"Svc.Fail", nil, false, "nope"},
		{"Svc.Panic", nil, false, "panic: boom"},
		{"Svc.Echo", []any{map[string]any{"name": "a", "tags": []string{"x"}}}, true, map[string]any{"name": "a", "tags": []any{"x", "echoed"}}},
		{"Svc.Caller", nil, true, float64(w.ID())},
		{"Svc.Nil", nil, true, []any{}},
		{"Svc.Optional", []any{"a"}, true, "a:true"},
		{"Svc.Hello", []any{42}, false, nil},
		{"Svc.Missing", nil, false, "method Svc.Missing is not bound"},
	}
	for i, tt := range tests {
		m := call(t, fw, i+1, tt.method, tt.args...)
		if m["k"] != "tok" {
			t.Errorf("%s: token not echoed: %v", tt.method, m)
		}
		if m["ok"] != tt.ok {
			t.Errorf("%s(%v): ok = %v (%v)", tt.method, tt.args, m["ok"], m)
			continue
		}
		got := m["v"]
		if !tt.ok {
			got = m["e"]
			if tt.want == nil {
				continue
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(tt.want) {
			t.Errorf("%s(%v) = %v, want %v", tt.method, tt.args, got, tt.want)
		}
	}
	if svc.calls.Load() != 1 {
		t.Errorf("Hello called %d times", svc.calls.Load())
	}
}

func TestUntrustedOriginsCannotCall(t *testing.T) {
	bindForTest(t, "Trust", &testService{})
	if err := Protocol.Handle("trusted", http.NotFoundHandler()); err != nil {
		t.Fatal(err)
	}
	defer Protocol.Unhandle("trusted")
	_, fw := readyWindow(t, WindowOptions{Page: PageOptions{TrustedOrigins: []string{"https://partner.example"}}})
	cases := []struct {
		url string
		ok  bool
	}{
		{"https://evil.example/", false},
		{"https://partner.example/page", true},
		{"https://partner.example.evil.com/", false},
		{"trusted://localhost/", true},
		{"file:///tmp/index.html", true},
		{"http://localhost:5173/", true}, // loopback dev server in development
		{"http://192.168.1.2:5173/", false},
	}
	for i, c := range cases {
		onMain(func() { fw.H.NavigationCommitted(c.url) })
		m := call(t, fw, 100+i, "Trust.Hello", "x")
		if m["ok"] != c.ok {
			t.Errorf("call from %s: ok = %v (%v)", c.url, m["ok"], m["e"])
		}
	}
}

// TestMessagesNeedTheSecret: the message handler is reachable from every
// frame, so messages without the bridge's secret (an iframe of another
// origin posting directly) must not run anything.
func TestMessagesNeedTheSecret(t *testing.T) {
	svc := &testService{}
	bindForTest(t, "Secret", svc)
	_, fw := readyWindow(t, WindowOptions{})
	forged := `{"t":"call","id":1,"k":"tok","m":"Secret.Hello","a":["x"]}`
	onMain(func() { fw.H.Message(forged) })
	onMain(func() { fw.H.Message("wrong" + forged) })
	time.Sleep(50 * time.Millisecond)
	if n := svc.calls.Load(); n != 0 {
		t.Fatalf("forged messages ran the method %d times", n)
	}
	if m := call(t, fw, 2, "Secret.Hello", "x"); m["ok"] != true || svc.calls.Load() != 1 {
		t.Errorf("the bridge's call failed: %v", m)
	}
	if a, b := secret(fw), secret(fb.Windows()[0]); len(a) < 20 || (fw != fb.Windows()[0] && a == b) {
		t.Errorf("secrets must be random and per window: %q %q", a, b)
	}
}

func TestWindowControls(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	call(t, fw, 1, "mygo:window.Maximize")
	if !w.IsMaximized() {
		t.Error("mygo:window.Maximize")
	}
	m := call(t, fw, 2, "mygo:window.IsMaximized")
	if m["v"] != true {
		t.Errorf("IsMaximized = %v", m)
	}
	call(t, fw, 3, "mygo:window.SetTitle", "Set by page")
	if w.Title() != "Set by page" {
		t.Errorf("title = %q", w.Title())
	}
}

type badService struct{}

func (badService) Bad(ch chan int) {}

type emptyService struct{}

func TestBindErrors(t *testing.T) {
	mustPanic := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic", name)
			}
		}()
		fn()
	}
	mustPanic("chan param", func() { Bind(badService{}) })
	mustPanic("no methods", func() { Bind(emptyService{}) })
	mustPanic("nil", func() { Bind(nil) })
	mustPanic("bad name", func() { BindAs("not valid", &testService{}) })
	bindForTest(t, "Dup", &testService{})
	mustPanic("duplicate", func() { BindAs("Dup", &testService{}) })
}

func TestCallContextCanceledOnNavigation(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	pc := w.pageContext()
	onMain(func() { fw.H.NavigationCommitted("https://next.example/") })
	select {
	case <-pc.Done():
	case <-time.After(time.Second):
		t.Fatal("page context not canceled after navigation")
	}
	if w.pageContext().Err() != nil {
		t.Error("new page context should be live")
	}
}

type progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

func TestEvents(t *testing.T) {
	ev := newEventForTest[progress](t, "test:progress")
	w, fw := readyWindow(t, WindowOptions{})
	if err := ev.Emit(w, progress{1, 3}); err != nil {
		t.Fatal(err)
	}
	m := received(t, fw, func(m map[string]any) bool { return m["t"] == "event" && m["n"] == "test:progress" })
	if fmt.Sprint(m["p"]) != "map[done:1 total:3]" {
		t.Errorf("payload = %v", m["p"])
	}
	if err := ev.Broadcast(progress{2, 3}); err != nil {
		t.Fatal(err)
	}
	received(t, fw, func(m map[string]any) bool {
		p, _ := m["p"].(map[string]any)
		return m["t"] == "event" && p["done"] == float64(2)
	})
	defer func() {
		if recover() == nil {
			t.Error("duplicate event name should panic")
		}
	}()
	NewEvent[int]("test:progress")
}

// Channels.

type streamer struct {
	sent    chan error // what Send returned
	stopped chan error // why Wait returned
}

// Count sends 0 to n-1.
func (s *streamer) Count(n int, ch *Channel[int]) error {
	for i := range n {
		if err := ch.Send(i); err != nil {
			return err
		}
	}
	return nil
}

// Blobs sends n strings of size bytes, reporting each Send.
func (s *streamer) Blobs(n, size int, ch *Channel[string]) {
	blob := strings.Repeat("x", size)
	for range n {
		s.sent <- ch.Send(blob)
	}
}

// Wait sends a value, then waits for the call to be canceled.
func (s *streamer) Wait(ctx context.Context, ch *Channel[string]) error {
	_ = ch.Send("ready")
	<-ctx.Done()
	s.sent <- ch.Send("late")
	s.stopped <- ctx.Err()
	return ctx.Err()
}

// pageMessages returns the messages the page received, in order.
func pageMessages(t *testing.T, fw *fake.Window) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, s := range fw.Scripts() {
		const prefix = "__mygo.receive("
		if !strings.HasPrefix(s, prefix) {
			continue
		}
		var msgs []map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSuffix(strings.TrimPrefix(s, prefix), ")")), &msgs); err != nil {
			t.Fatalf("bad script %q: %v", s, err)
		}
		out = append(out, msgs...)
	}
	return out
}

func TestChannels(t *testing.T) {
	s := &streamer{sent: make(chan error, 16), stopped: make(chan error, 1)}
	bindForTest(t, "Stream", s)
	_, fw := testWindow(t, WindowOptions{})

	// Values arrive in order, then the end, then the reply.
	call(t, fw, 1, "Stream.Count", 3, 7)
	var got []string
	for _, m := range pageMessages(t, fw) {
		switch {
		case m["t"] == "chan" && m["end"] == true:
			got = append(got, "end")
		case m["t"] == "chan":
			if m["c"] != float64(7) || m["k"] != "tok" {
				t.Errorf("value for another channel: %v", m)
			}
			got = append(got, fmt.Sprint(m["p"]))
		case m["t"] == "reply":
			got = append(got, "reply")
		}
	}
	if strings.Join(got, ",") != "0,1,2,end,reply" {
		t.Errorf("page received %v", got)
	}

	// The page takes a MiB at most before it acknowledges some.
	const size = 600 << 10
	page(fw, `{"t":"call","id":2,"k":"tok","m":"Stream.Blobs","a":[3,`+fmt.Sprint(size)+`,8]}`)
	for range 2 {
		if err := <-s.sent; err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-s.sent:
		t.Fatalf("Send did not wait for the page: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	asks := 0
	for _, m := range pageMessages(t, fw) {
		if m["c"] == float64(8) && m["a"] == float64(1) {
			asks++
		}
	}
	if asks != 2 {
		t.Errorf("%d values asked for an acknowledgment, want 2", asks)
	}
	page(fw, `{"t":"chan-ack","c":8,"k":"stale","n":2}`) // another page's
	select {
	case <-s.sent:
		t.Fatal("an acknowledgment of another page counted")
	case <-time.After(20 * time.Millisecond):
	}
	page(fw, `{"t":"chan-ack","c":8,"k":"tok","n":2}`)
	if err := <-s.sent; err != nil {
		t.Fatal(err)
	}
	received(t, fw, func(m map[string]any) bool { return m["t"] == "reply" && m["id"] == float64(2) })

	// The page closing the channel cancels the call.
	page(fw, `{"t":"call","id":3,"k":"tok","m":"Stream.Wait","a":[9]}`)
	received(t, fw, func(m map[string]any) bool { return m["c"] == float64(9) && m["p"] == "ready" })
	page(fw, `{"t":"chan-close","c":9,"k":"tok"}`)
	if err := <-s.stopped; !errors.Is(err, context.Canceled) {
		t.Errorf("call context: %v", err)
	}
	if err := <-s.sent; !errors.Is(err, ErrChannelClosed) {
		t.Errorf("Send after the page closed the channel = %v", err)
	}
	received(t, fw, func(m map[string]any) bool { return m["t"] == "reply" && m["id"] == float64(3) })
	for _, m := range pageMessages(t, fw) {
		if m["c"] == float64(9) && m["end"] == true {
			t.Error("the page was told about the end of a channel it closed")
		}
	}

	// So does navigating away.
	page(fw, `{"t":"call","id":4,"k":"tok","m":"Stream.Wait","a":[10]}`)
	received(t, fw, func(m map[string]any) bool { return m["c"] == float64(10) && m["p"] == "ready" })
	onMain(func() { fw.H.NavigationCommitted("about:blank#next") })
	if err := <-s.stopped; !errors.Is(err, context.Canceled) {
		t.Errorf("call context after navigating: %v", err)
	}
	if err := <-s.sent; !errors.Is(err, ErrChannelClosed) {
		t.Errorf("Send after navigating = %v", err)
	}

	// The page may close a channel before its call made it, e.g. by
	// aborting a request right after starting it: the call is canceled
	// as soon as it starts.
	page(fw, `{"t":"chan-close","c":11,"k":"tok"}`)
	page(fw, `{"t":"call","id":6,"k":"tok","m":"Stream.Wait","a":[11]}`)
	select {
	case err := <-s.stopped:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("call context of a channel closed early: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a channel the page closed before its call started did not cancel the call")
	}
	if err := <-s.sent; !errors.Is(err, ErrChannelClosed) {
		t.Errorf("Send on a channel closed early = %v", err)
	}

	m := call(t, fw, 5, "Stream.Count", 1)
	if m["ok"] != false || !strings.Contains(fmt.Sprint(m["e"]), "not a Channel") {
		t.Errorf("call without its channel: %v", m)
	}
}

func TestChannelsClosedAtOnce(t *testing.T) {
	// The page closes each channel right after its call, which may be
	// making it then: every call is canceled, whenever the close comes, and
	// the page is told about no end.
	const n = 200
	s := &streamer{sent: make(chan error, n), stopped: make(chan error, n)}
	bindForTest(t, "Stream", s)
	_, fw := testWindow(t, WindowOptions{})
	for id := 1; id <= n; id++ {
		page(fw, fmt.Sprintf(`{"t":"call","id":%d,"k":"tok","m":"Stream.Wait","a":[%d]}`, id, id))
		page(fw, fmt.Sprintf(`{"t":"chan-close","c":%d,"k":"tok"}`, id))
	}
	for range n {
		select {
		case <-s.stopped:
		case <-time.After(5 * time.Second):
			t.Fatal("a call whose channel the page closed was not canceled")
		}
	}
	// A channel ends before the reply of its call.
	for id := 1; id <= n; id++ {
		received(t, fw, func(m map[string]any) bool { return m["t"] == "reply" && m["id"] == float64(id) })
	}
	for _, m := range pageMessages(t, fw) {
		if m["t"] == "chan" && m["end"] == true {
			t.Fatalf("the page was told about the end of channel %v, which it closed", m["c"])
		}
	}
}

type badStreams struct{}

func (badStreams) Result() *Channel[int]            { return nil }
func (badStreams) Nested(struct{ C *Channel[int] }) {}
func (badStreams) Variadic(chs ...*Channel[int])    {}

func TestChannelsAreParameters(t *testing.T) {
	for _, name := range []string{"Result", "Nested", "Variadic"} {
		m, _ := reflect.TypeFor[badStreams]().MethodByName(name)
		if _, err := newMethod("Bad", m, reflect.ValueOf(badStreams{})); err == nil {
			t.Errorf("%s: a misplaced channel should not bind", name)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("an event of channels should panic")
		}
	}()
	NewEvent[*Channel[int]]("test:channel")
}

func TestMessagesAreBatched(t *testing.T) {
	ev := newEventForTest[int](t, "test:batch")
	w, fw := readyWindow(t, WindowOptions{})
	onMain(func() {
		for i := range 5 {
			_ = ev.Emit(w, i)
		}
	})
	received(t, fw, func(m map[string]any) bool { return m["n"] == "test:batch" && m["p"] == float64(4) })
	n := 0
	for _, s := range fw.Scripts() {
		if strings.Contains(s, "test:batch") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("5 events took %d evaluations, want 1", n)
	}
}

func TestEventsWaitForDOMReady(t *testing.T) {
	ev := newEventForTest[string](t, "test:early")
	w, fw := testWindow(t, WindowOptions{})
	if err := ev.Emit(w, "first"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	for _, s := range fw.Scripts() {
		if strings.Contains(s, "test:early") {
			t.Fatal("event delivered before the page was ready")
		}
	}
	page(fw, `{"t":"dom-ready"}`)
	received(t, fw, func(m map[string]any) bool { return m["n"] == "test:early" && m["p"] == "first" })

	// After a navigation, events wait for the new page.
	onMain(func() { fw.H.NavigationCommitted("https://next.example/") })
	n := len(fw.Scripts())
	_ = ev.Emit(w, "second")
	time.Sleep(20 * time.Millisecond)
	if len(fw.Scripts()) != n {
		t.Fatal("event delivered during navigation")
	}
	page(fw, `{"t":"dom-ready"}`)
	received(t, fw, func(m map[string]any) bool { return m["p"] == "second" })
}

func TestHiddenTitleBar(t *testing.T) {
	// titleBar matches the page event of the room the window controls take.
	titleBar := func(height, left, right float64) func(m map[string]any) bool {
		return func(m map[string]any) bool {
			p, _ := m["p"].(map[string]any)
			return m["n"] == "mygo:title-bar" && p["height"] == height && p["left"] == left && p["right"] == right
		}
	}
	w, fw := readyWindow(t, WindowOptions{TitleBarStyle: TitleBarHidden, TitleBarHeight: 52})
	if fw.Opts.TitleBarHeight != 52 {
		t.Errorf("TitleBarHeight = %d, want 52", fw.Opts.TitleBarHeight)
	}
	received(t, fw, titleBar(52, 0, 138))

	// The backend reports a change, such as another button layout.
	onMain(func() {
		fw.TitleBarRoom = platform.TitleBar{Height: 46, Left: 80}
		fw.H.TitleBarChanged()
	})
	received(t, fw, titleBar(46, 80, 0))

	// The page gets CSS pixels, which zoom makes larger.
	w.Page().SetZoomFactor(2)
	received(t, fw, titleBar(23, 40, 0))

	// A later page hears it once its DOM is ready: the script the backend
	// runs at document start may be older.
	onMain(func() { fw.H.NavigationCommitted("https://next.example/") })
	n := len(fw.Scripts())
	page(fw, `{"t":"dom-ready"}`)
	received(t, fw, titleBar(23, 40, 0))
	if len(fw.Scripts()) == n {
		t.Error("the new page did not hear the title bar")
	}

	// Windows with their title bar, or none at all, hear nothing.
	for _, opts := range []WindowOptions{{}, {Frameless: true, TitleBarStyle: TitleBarHidden}} {
		_, other := readyWindow(t, opts)
		onMain(func() { other.H.TitleBarChanged() })
		time.Sleep(20 * time.Millisecond)
		for _, s := range other.Scripts() {
			if strings.Contains(s, "mygo:title-bar") {
				t.Errorf("%+v: title bar sent to the page: %s", opts, s)
			}
		}
	}

	_, fw = testWindow(t, WindowOptions{TitleBarStyle: TitleBarHiddenInset, TitleBarHeight: -1})
	if fw.Opts.TitleBarHeight != 0 {
		t.Errorf("TitleBarHeight -1 = %d, want 0", fw.Opts.TitleBarHeight)
	}
}

func TestEval(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	var bodies []string
	fw.AsyncFunction = func(body string) (string, error) {
		bodies = append(bodies, body)
		switch {
		case strings.Contains(body, "await(\ndocument.title\n)"):
			return `{"ok":true,"v":"Title"}`, nil
		case strings.Contains(body, "await(\nlet x = 1; return x\n)"):
			return "", errors.New("SyntaxError: Unexpected token")
		case strings.Contains(body, "await(async()=>{\nlet x = 1; return x\n})()"):
			return `{"ok":true,"v":1}`, nil
		case strings.Contains(body, "boom"):
			return `{"ok":false,"e":"boom"}`, nil
		}
		return `{"ok":true}`, nil
	}
	v, err := w.Page().Eval("document.title;")
	if err != nil || v != "Title" {
		t.Errorf("Eval expression = %v, %v", v, err)
	}
	n, err := EvalAs[int](w.Page(), "let x = 1; return x")
	if err != nil || n != 1 {
		t.Errorf("Eval statements = %v, %v (bodies %q)", n, err, bodies)
	}
	var evalErr *EvalError
	if _, err := w.Page().Eval("boom()"); !errors.As(err, &evalErr) || evalErr.Message != "boom" {
		t.Errorf("Eval error = %v", err)
	}
	if v, err := w.Page().Eval("void 0"); v != nil || err != nil {
		t.Errorf("Eval undefined = %v, %v", v, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fw.AsyncFunction = func(string) (string, error) { return "", fake.ErrNoReply }
	if _, err := w.Page().EvalContext(ctx, "1"); !errors.Is(err, context.Canceled) {
		t.Errorf("EvalContext with canceled context = %v", err)
	}

	// Only a syntax error means the code did not run: other failures are
	// not retried, which could run the code twice.
	bodies = nil
	fw.AsyncFunction = func(body string) (string, error) {
		bodies = append(bodies, body)
		return "", errors.New("the page navigated")
	}
	if _, err := w.Page().Eval("sideEffect()"); err == nil || len(bodies) != 1 {
		t.Errorf("Eval after a failure = %v, ran %d times", err, len(bodies))
	}
}

// TestEvalLateReply: on the main thread, a reply arriving after the
// context was canceled must not block.
func TestEvalLateReply(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	fw.AsyncCallback = func(body string, reply func(string, error)) {
		postMain(func() {
			cancel()
			time.Sleep(50 * time.Millisecond) // the cancellation is delivered...
			reply(`{"ok":true,"v":1}`, nil)   // ...then the page answers
		})
	}
	done := make(chan error, 1)
	go onMain(func() {
		_, err := w.Page().EvalContext(ctx, "1")
		done <- err
	})
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("EvalContext = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the late reply blocked the main thread")
	}
}

// Custom protocols.

type recorder struct {
	mu       sync.Mutex
	status   int
	header   http.Header
	body     strings.Builder
	finished chan struct{}
}

func (r *recorder) Respond(status int, h http.Header) { r.status, r.header = status, h }
func (r *recorder) Write(p []byte)                    { r.body.Write(p) }
func (r *recorder) Finish()                           { close(r.finished) }
func (r *recorder) Fail(err error)                    { r.body.WriteString("FAIL " + err.Error()); close(r.finished) }

func request(t *testing.T, fw *fake.Window, method, url string, body io.Reader) *recorder {
	t.Helper()
	rec := &recorder{finished: make(chan struct{})}
	req := &platform.SchemeRequest{Context: context.Background(), Method: method, URL: url, Header: http.Header{}, Responder: rec}
	if body != nil {
		req.Body = io.NopCloser(body)
	}
	onMain(func() { fw.H.SchemeRequest(req) })
	select {
	case <-rec.finished:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not finish")
	}
	return rec
}

func TestProtocol(t *testing.T) {
	if err := Protocol.Handle("http", http.NotFoundHandler()); err == nil {
		t.Error("registering http should fail")
	}
	if err := Protocol.Handle("Bad Scheme", http.NotFoundHandler()); err == nil {
		t.Error("invalid scheme should fail")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /hello", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Test", r.URL.Query().Get("q"))
		fmt.Fprint(w, "<p>hi</p>")
	})
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusCreated)
		w.Write(b)
	})
	mux.HandleFunc("/stream", func(w http.ResponseWriter, r *http.Request) {
		for i := range 3 {
			fmt.Fprintf(w, "chunk%d;", i)
			w.(http.Flusher).Flush()
		}
	})
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) { panic("oops") })
	if err := Protocol.Handle("test", mux); err != nil {
		t.Fatal(err)
	}
	defer Protocol.Unhandle("test")
	if !Protocol.IsHandled("TEST") {
		t.Error("IsHandled should be case-insensitive")
	}

	_, fw := testWindow(t, WindowOptions{})
	if !strings.Contains(strings.Join(fw.Opts.Schemes, ","), "test") {
		t.Errorf("scheme not passed to the window: %v", fw.Opts.Schemes)
	}

	rec := request(t, fw, "GET", "test://localhost/hello?q=1", nil)
	if rec.status != 200 || rec.body.String() != "<p>hi</p>" || rec.header.Get("X-Test") != "1" {
		t.Errorf("GET: %d %q %v", rec.status, rec.body.String(), rec.header)
	}
	if ct := rec.header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content type not sniffed: %q", ct)
	}
	rec = request(t, fw, "POST", "test://localhost/echo", strings.NewReader("payload"))
	if rec.status != 201 || rec.body.String() != "payload" {
		t.Errorf("POST: %d %q", rec.status, rec.body.String())
	}
	rec = request(t, fw, "GET", "test://localhost/stream", nil)
	if rec.body.String() != "chunk0;chunk1;chunk2;" {
		t.Errorf("stream: %q", rec.body.String())
	}
	rec = request(t, fw, "GET", "test://localhost/panic", nil)
	if rec.status != 500 {
		t.Errorf("panic: status %d", rec.status)
	}
	rec = request(t, fw, "GET", "unknown://localhost/", nil)
	if rec.status != 404 {
		t.Errorf("unhandled scheme: status %d", rec.status)
	}
}

// bodyRecorder takes the body from the goroutine serving the request, as
// the Linux backend does.
type bodyRecorder struct {
	recorder
	writes int
	err    error // returned by WriteBody
}

func (r *bodyRecorder) Write([]byte) { panic("Write called on a SchemeBodyWriter") }

func (r *bodyRecorder) WriteBody(p []byte) error {
	if isMainThread() || r.status == 0 {
		panic("WriteBody on the main thread or before Respond")
	}
	r.writes++
	r.body.Write(p)
	return r.err
}

func TestProtocolBodyWriter(t *testing.T) {
	const size = 640 << 10 // two and a half chunks
	var written int
	var writeErr error
	if err := Protocol.HandleFunc("body", func(w http.ResponseWriter, r *http.Request) {
		chunk := []byte(strings.Repeat("0123456789abcdef", 2048)) // 32 KiB
		for written = 0; written < size; written += len(chunk) {
			if _, writeErr = w.Write(chunk); writeErr != nil {
				return
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer Protocol.Unhandle("body")
	_, fw := testWindow(t, WindowOptions{})
	serve := func(rec *bodyRecorder) {
		t.Helper()
		rec.finished = make(chan struct{})
		req := &platform.SchemeRequest{Context: context.Background(), Method: "GET", URL: "body://localhost/", Header: http.Header{}, Responder: rec}
		onMain(func() { fw.H.SchemeRequest(req) })
		select {
		case <-rec.finished:
		case <-time.After(3 * time.Second):
			t.Fatal("request did not finish")
		}
	}

	rec := &bodyRecorder{}
	serve(rec)
	if rec.status != 200 || rec.body.Len() != size || rec.writes != 3 {
		t.Errorf("status %d, %d bytes in %d writes", rec.status, rec.body.Len(), rec.writes)
	}
	if writeErr != nil {
		t.Errorf("Write: %v", writeErr)
	}

	// Once the webview stops reading, the handler learns it from Write.
	rec = &bodyRecorder{err: errors.New("gone")}
	serve(rec)
	if writeErr == nil || writeErr.Error() != "gone" || written >= size {
		t.Errorf("after WriteBody failed: Write = %v, %d bytes written", writeErr, written)
	}
}

func TestWindowAllClosedOnce(t *testing.T) {
	if n := len(Windows()); n != 0 {
		t.Skipf("%d windows are open", n)
	}
	var count atomic.Int32
	off := App.OnWindowAllClosed(func() { count.Add(1) })
	defer off()
	parent := NewWindow(WindowOptions{})
	child := NewWindow(WindowOptions{Parent: parent})
	NewWindow(WindowOptions{Parent: child})
	parent.Close()
	onMain(func() {})
	if n := len(Windows()); n != 0 {
		t.Fatalf("%d windows left", n)
	}
	if n := count.Load(); n != 1 {
		t.Errorf("OnWindowAllClosed fired %d times", n)
	}
}

func TestFrontend(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	if !slices.Contains(fw.Opts.Schemes, "mygo") {
		t.Errorf("windows must handle the mygo scheme: %v", fw.Opts.Schemes)
	}
	load := func(url string) string {
		t.Helper()
		if err := w.Page().LoadURL(url); err != nil {
			t.Fatal(err)
		}
		return get(w, platform.Window.URL)
	}
	if got := load("/settings?tab=1"); got != "mygo://localhost/settings?tab=1" {
		t.Errorf("relative URL loaded %q", got)
	}
	if got := load("https://example.com/"); got != "https://example.com/" {
		t.Errorf("absolute URL loaded %q", got)
	}

	// No frontend: a page explains how to get one.
	rec := request(t, fw, "GET", "mygo://localhost/", nil)
	if rec.status != 404 || !strings.Contains(rec.body.String(), "mygo dev") {
		t.Errorf("no frontend: %d %q", rec.status, rec.body.String())
	}
	SetFrontend(fstest.MapFS{"index.html": {Data: []byte("<p>app</p>")}, "app.js": {Data: []byte("go()")}})
	defer SetFrontend(nil)
	if rec := request(t, fw, "GET", "mygo://localhost/app.js", nil); rec.status != 200 || rec.body.String() != "go()" {
		t.Errorf("app.js: %d %q", rec.status, rec.body.String())
	}
	if rec := request(t, fw, "GET", "mygo://localhost/todos/1", nil); rec.body.String() != "<p>app</p>" {
		t.Errorf("client-side route: %q", rec.body.String())
	}
	if !w.isTrusted("mygo://localhost/") {
		t.Error("the frontend must be trusted")
	}

	// The app's own handler wins.
	if err := Protocol.Handle("mygo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "own") })); err != nil {
		t.Fatal(err)
	}
	if rec := request(t, fw, "GET", "mygo://localhost/app.js", nil); rec.body.String() != "own" {
		t.Errorf("app handler: %q", rec.body.String())
	}
	Protocol.Unhandle("mygo")

	// During mygo dev, the dev server.
	t.Setenv("MYGO_DEV_URL", "http://192.168.1.2:5173")
	if got := load("/"); got != "http://192.168.1.2:5173/" {
		t.Errorf("dev: loaded %q", got)
	}
	if !w.isTrusted("http://192.168.1.2:5173/") || w.isTrusted("http://192.168.1.3:5173/") {
		t.Error("only the dev server is trusted")
	}
	// ... or frontendDist from disk without a dev server.
	t.Setenv("MYGO_DEV_URL", "")
	SetFrontend(nil)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<p>disk</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MYGO_FRONTEND_DIST", dir)
	if rec := request(t, fw, "GET", "mygo://localhost/", nil); rec.body.String() != "<p>disk</p>" {
		t.Errorf("frontendDist on disk: %q", rec.body.String())
	}
}

func TestFileServer(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html":    {Data: []byte("<!doctype html>index")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	if err := Protocol.Handle("fs", FileServer(fsys)); err != nil {
		t.Fatal(err)
	}
	defer Protocol.Unhandle("fs")
	_, fw := testWindow(t, WindowOptions{})
	for url, want := range map[string]string{
		"fs://localhost/":              "<!doctype html>index",
		"fs://localhost/assets/app.js": "console.log(1)",
		"fs://localhost/settings/me":   "<!doctype html>index",
	} {
		rec := request(t, fw, "GET", url, nil)
		if rec.status != 200 || rec.body.String() != want {
			t.Errorf("%s: %d %q", url, rec.status, rec.body.String())
		}
	}
	if rec := request(t, fw, "GET", "fs://localhost/missing.css", nil); rec.status != 404 {
		t.Errorf("missing file: %d", rec.status)
	}
	rec := request(t, fw, "GET", "fs://localhost/assets/app.js", nil)
	if ct := rec.header.Get("Content-Type"); !strings.Contains(ct, "javascript") {
		t.Errorf("js content type = %q", ct)
	}
}

// Menus.

func TestMenuRolesAndState(t *testing.T) {
	var clicks []string
	var mu sync.Mutex
	click := func(item *MenuItem, win *Window) {
		mu.Lock()
		clicks = append(clicks, item.ID)
		mu.Unlock()
	}
	menu := NewMenu([]*MenuItem{
		{Label: "File", Submenu: []*MenuItem{
			{ID: "new", Label: "New", Accelerator: "CmdOrCtrl+N", Click: click},
			Separator(),
			{Role: RoleQuit},
		}},
		{Label: "View", Submenu: []*MenuItem{
			{ID: "grid", Label: "Grid", Type: MenuItemCheckbox, Click: click},
			{ID: "small", Label: "Small", Type: MenuItemRadio, Checked: true},
			{ID: "large", Label: "Large", Type: MenuItemRadio},
		}},
		{Role: RoleEditMenu},
	})
	snap := menu.snapshot()
	if snap.Items[0].Type != platform.MenuItemSubmenu || snap.Items[0].Submenu.Items[1].Type != platform.MenuItemSeparator {
		t.Errorf("types not inferred: %+v", snap.Items[0])
	}
	quit := snap.Items[0].Submenu.Items[2]
	if !strings.HasPrefix(quit.Label, "Quit") && quit.Label != "Exit" {
		t.Errorf("quit label = %q", quit.Label)
	}
	edit := snap.Items[2]
	if edit.Label != "Edit" || len(edit.Submenu.Items) < 6 || edit.Submenu.Items[0].Accelerator != "CmdOrCtrl+Z" {
		t.Errorf("edit menu not expanded: %+v", edit)
	}

	grid := menu.ItemByID("grid")
	small, large := menu.ItemByID("small"), menu.ItemByID("large")
	onMain(func() { fb.ClickMenuItem(grid.uid) })
	if !grid.IsChecked() {
		t.Error("checkbox not toggled")
	}
	onMain(func() { fb.ClickMenuItem(large.uid) })
	if !large.IsChecked() || small.IsChecked() {
		t.Error("radio group not exclusive")
	}
	onMain(func() { fb.ClickMenuItem(menu.ItemByID("new").uid) })
	mu.Lock()
	if strings.Join(clicks, ",") != "grid,new" {
		t.Errorf("clicks = %v", clicks)
	}
	mu.Unlock()

	menu.ItemByID("new").SetEnabled(false)
	onMain(func() {}) // the update reaches the backend on the main thread
	updates := fb.MenuUpdates()
	last := updates[len(updates)-1]
	if last.ID != menu.ItemByID("new").uid || last.Enabled {
		t.Errorf("SetEnabled not propagated: %+v", last)
	}
}

func TestDefaultMenu(t *testing.T) {
	m := fb.AppMenu()
	if runtime.GOOS != "darwin" {
		if m != nil {
			t.Error("no default menu expected outside macOS")
		}
		return
	}
	if m == nil || len(m.Items) != 5 || m.Items[0].Label != "MyGoTest" {
		t.Fatalf("default menu: %+v", m)
	}
}

func TestMenuRolesOnWindow(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	w.Focus()
	onMain(func() {
		performRole(RoleZoomIn, w)
		performRole(RoleZoomIn, w)
	})
	if z := fw.Zoom(); z != 1.25 {
		t.Errorf("zoom after two zoomIn = %v", z)
	}
	onMain(func() { performRole(RoleToggleDevTools, w) })
	if !w.Page().IsDevToolsOpened() {
		t.Error("toggleDevTools")
	}
}

func TestWindowMenuActivation(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{Hidden: true})
	other, otherNative := testWindow(t, WindowOptions{Hidden: true})
	other.Focus()
	var clicked *Window
	var checked bool
	var zoom float64
	clicks := 0
	menu := NewMenu([]*MenuItem{{Label: "View", Submenu: []*MenuItem{
		{ID: "small", Label: "Small", Type: MenuItemRadio, Checked: true},
		{ID: "large", Label: "Large", Type: MenuItemRadio, Role: RoleZoomIn, Click: func(it *MenuItem, win *Window) {
			clicked, checked = win, it.IsChecked()
			if win != nil {
				zoom = win.Page().ZoomFactor()
			}
			clicks++
		}},
	}}})
	item := menu.ItemByID("large")
	onMain(func() { fw.ClickMenuItem(item.uid) })
	if clicked != w || !checked || zoom != 1.1 || menu.ItemByID("small").IsChecked() {
		t.Errorf("click = (%v, checked %v, zoom %v), want the menu's window after its state and role changed", clicked, checked, zoom)
	}
	if other.Page().ZoomFactor() != 1 {
		t.Error("the role changed the focused window instead of the menu's window")
	}
	if FocusedWindow() != other {
		t.Error("choosing a menu item changed window focus")
	}

	// A submenu taking focus clears FocusedWindow; its owner still applies.
	onMain(otherNative.Blur)
	if FocusedWindow() != nil {
		t.Fatal("a window still has focus")
	}
	onMain(func() { fw.ClickMenuItem(item.uid) })
	if clicked != w || zoom != 1.25 {
		t.Errorf("click without focus = (%v, zoom %v), want the menu's window", clicked, zoom)
	}

	// Application and tray menus without a window use the focused window.
	other.Focus()
	onMain(func() { fb.ClickMenuItem(item.uid) })
	if clicked != other || zoom != 1.1 {
		t.Errorf("application click = (%v, zoom %v), want the focused window", clicked, zoom)
	}
	onMain(otherNative.Blur)
	onMain(func() { fb.ClickMenuItem(item.uid) })
	if clicked != nil {
		t.Error("a menu without a window supplied one while none had focus")
	}

	w.Destroy()
	before := clicks
	onMain(func() { fw.ClickMenuItem(item.uid) })
	if clicks != before {
		t.Error("a destroyed window still delivered a menu click")
	}
}

func TestNextZoom(t *testing.T) {
	if nextZoom(1, 1) != 1.1 || nextZoom(1, -1) != 0.9 || nextZoom(5, 1) != 5 || nextZoom(0.25, -1) != 0.25 {
		t.Error("zoom steps")
	}
}

// Application.

func TestQuitCanBeCanceled(t *testing.T) {
	w, _ := testWindow(t, WindowOptions{})
	off := App.OnBeforeQuit(func(e *QuitEvent) { e.PreventDefault() })
	if onMainValue(fb.QuitRequested) {
		t.Fatal("quit was not canceled by OnBeforeQuit")
	}
	off()
	offClose := w.OnClose(func(e *CloseEvent) { e.PreventDefault() })
	if onMainValue(fb.QuitRequested) {
		t.Fatal("quit was not canceled by a window")
	}
	offClose()
	offWill := App.OnWillQuit(func(e *QuitEvent) { e.PreventDefault() })
	defer offWill()
	if onMainValue(fb.QuitRequested) {
		t.Fatal("quit was not canceled by OnWillQuit")
	}
	if !w.IsDestroyed() {
		t.Error("windows are closed before OnWillQuit")
	}
}

func TestPaths(t *testing.T) {
	dir, err := App.Path(PathUserData)
	if err != nil || !strings.HasSuffix(dir, "MyGoTest") {
		t.Fatalf("userData = %q, %v", dir, err)
	}
	defer os.Remove(dir)
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		t.Error("userData was not created")
	}
	App.SetPath(PathDownloads, "/tmp/dl")
	if p, _ := App.Path(PathDownloads); p != "/tmp/dl" {
		t.Errorf("override = %q", p)
	}
	if _, err := App.Path("nope"); err == nil {
		t.Error("unknown path should fail")
	}
	wd, _ := os.Getwd()
	exe, _ := os.Executable()
	if p, err := App.Path(PathResources); err != nil || p != resourcesDir(exe, wd) {
		t.Errorf("resources = %q, %v", p, err)
	}
	for _, c := range []struct{ exe, want string }{
		{"/Applications/My App.app/Contents/MacOS/My App", "/Applications/My App.app/Contents/Resources"},
		{"/opt/my-app/my-app", "/opt/my-app"},
		{`/tmp/go-build123/b001/exe/app`, "/work/resources"},
		{`/tmp/go-build123/b001/app.test`, "/work/resources"},
	} {
		exe, want := filepath.FromSlash(c.exe), filepath.FromSlash(c.want)
		if got := resourcesDir(exe, filepath.FromSlash("/work")); got != want {
			t.Errorf("resourcesDir(%q) = %q, want %q", exe, got, want)
		}
	}
}

func TestModules(t *testing.T) {
	Clipboard.WriteText("copied")
	if Clipboard.ReadText() != "copied" {
		t.Error("clipboard")
	}
	if d := Screen.PrimaryDisplay(); d.Label != "Fake" || d.WorkArea.Y != 25 {
		t.Errorf("primary display = %+v", d)
	}
	if d := Screen.DisplayNearestPoint(Point{5000, 5000}); d.Label != "Fake" {
		t.Error("DisplayNearestPoint")
	}
	var themed atomic.Int32
	off := Theme.OnUpdated(func() { themed.Add(1) })
	defer off()
	Theme.SetSource(ThemeDark)
	if !Theme.IsDark() || Theme.Source() != ThemeDark || themed.Load() != 1 {
		t.Error("theme")
	}
	Theme.SetSource(ThemeSystem)

	fb.OpenResult = []string{"/a.txt"}
	if paths, err := Dialog.Open(OpenDialogOptions{}); err != nil || len(paths) != 1 {
		t.Errorf("Dialog.Open = %v, %v", paths, err)
	}
	fb.MessageResult = platform.MessageBoxResult{Response: 1, CheckboxChecked: true}
	if res, err := Dialog.Message(MessageOptions{Buttons: []string{"OK", "Cancel"}}); err != nil || res.Button != 1 || !res.CheckboxChecked {
		t.Errorf("Dialog.Message = %+v, %v", res, err)
	}
}

// needsAppCalls are the calls that need the running app, which main may
// not make before Run.
var needsAppCalls = []struct {
	name string
	call func()
}{
	{"App.Focus", func() { App.Focus() }},
	{"App.Hide", func() { App.Hide() }},
	{"App.Show", func() { App.Show() }},
	{"App.IsHidden", func() { App.IsHidden() }},
	{"App.SetBadgeCount", func() { App.SetBadgeCount(1) }},
	{"App.ShowAboutPanel", func() { App.ShowAboutPanel(AboutPanelOptions{}) }},
	{"App.ClearBrowsingData", func() { App.ClearBrowsingData() }},
	{"App.Dock.SetBadge", func() { App.Dock.SetBadge("1") }},
	{"App.Dock.Badge", func() { App.Dock.Badge() }},
	{"App.Dock.Bounce", func() { App.Dock.Bounce(false) }},
	{"App.Dock.CancelBounce", func() { App.Dock.CancelBounce(1) }},
	{"App.Dock.SetIcon", func() { App.Dock.SetIcon(nil) }},
	{"Shell.OpenExternal", func() { Shell.OpenExternal("https://example.com") }},
	{"Shell.OpenPath", func() { Shell.OpenPath("/") }},
	{"Shell.ShowItemInFolder", func() { Shell.ShowItemInFolder("/") }},
	{"Shell.TrashItem", func() { Shell.TrashItem("/nonexistent") }},
	{"Shell.Beep", func() { Shell.Beep() }},
	{"Clipboard.ReadText", func() { Clipboard.ReadText() }},
	{"Clipboard.WriteText", func() { Clipboard.WriteText("x") }},
	{"Clipboard.ReadHTML", func() { Clipboard.ReadHTML() }},
	{"Clipboard.WriteHTML", func() { Clipboard.WriteHTML("x") }},
	{"Clipboard.ReadImage", func() { Clipboard.ReadImage() }},
	{"Clipboard.WriteImage", func() { Clipboard.WriteImage(nil) }},
	{"Clipboard.Clear", func() { Clipboard.Clear() }},
	{"Clipboard.AvailableFormats", func() { Clipboard.AvailableFormats() }},
	{"Clipboard.Write", func() { Clipboard.Write(transfer.Data{}) }},
	{"Clipboard.Read", func() { Clipboard.Read() }},
	{"Clipboard.Formats", func() { Clipboard.Formats() }},
	{"Clipboard.ReadFormat", func() { Clipboard.ReadFormat(transfer.Text) }},
	{"Clipboard.ReadFiles", func() { Clipboard.ReadFiles() }},
	{"Clipboard.WriteFiles", func() { Clipboard.WriteFiles() }},
	{"Clipboard.Flush", func() { Clipboard.Flush() }},
	{"Screen.Displays", func() { Screen.Displays() }},
	{"Screen.PrimaryDisplay", func() { Screen.PrimaryDisplay() }},
	{"Screen.CursorScreenPoint", func() { Screen.CursorScreenPoint() }},
	{"Screen.DisplayNearestPoint", func() { Screen.DisplayNearestPoint(Point{}) }},
	{"Screen.DisplayMatching", func() { Screen.DisplayMatching(Rectangle{}) }},
	{"Theme.IsDark", func() { Theme.IsDark() }},
	{"GlobalShortcut.Register", func() { GlobalShortcut.Register("CmdOrCtrl+Shift+K", func() {}) }},
	{"NewTray", func() { NewTray(TrayOptions{}) }},
	{"NotificationsSupported", func() { NotificationsSupported() }},
	{"Notification.Show", func() { NewNotification(NotificationOptions{}).Show() }},
	{"Power.KeepAwake", func() { Power.KeepAwake("test", false) }},
	{"Power.IdleTime", func() { Power.IdleTime() }},
	{"Dialog.Open", func() { Dialog.Open(OpenDialogOptions{}) }},
	{"Dialog.Save", func() { Dialog.Save(SaveDialogOptions{}) }},
	{"Dialog.Message", func() { Dialog.Message(MessageOptions{}) }},
	{"Dialog.Error", func() { Dialog.Error("title", "content") }},
	{"Menu.Popup", func() { NewMenu(nil).Popup(nil) }},
	{"Menu.PopupAt", func() { NewMenu(nil).PopupAt(nil, 0, 0) }},
}

// beforeRun is a helper process for TestBeforeRun: it does what main may
// do before Run, when the backend is not initialized, and what it may
// not, then exits.
func beforeRun() {
	// Settings, which Run applies.
	Theme.SetSource(ThemeDark)
	App.Dock.SetMenu(NewMenu([]*MenuItem{{Label: "New Window"}}))
	// Calls that work without the running app.
	App.Locale()
	Power.IsOnBattery()
	GlobalShortcut.UnregisterAll()
	NewNotification(NotificationOptions{}).Close()
	// Nothing can be shown before Run, so there is nothing to clear and
	// no backend to ask.
	ClearNotifications()
	for _, c := range needsAppCalls {
		func() {
			defer func() { fmt.Printf("%s: %v\n", c.name, recover()) }()
			c.call()
		}()
	}
	App.WhenReady(func() {
		fmt.Println("dark:", Theme.IsDark())
		fmt.Println("dock menu:", fb.DockMenu != nil && fb.DockMenu.Items[0].Label == "New Window")
		App.Quit()
	})
	if err := App.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

// TestBeforeRun: main may make settings before Run, which apply once Run
// has initialized the backend, while calls that need the running app fail
// with a message naming them, the same on every platform rather than
// crashing, hanging or doing nothing depending on it. Generate mode,
// which never initializes the backend, still writes the client.
func TestBeforeRun(t *testing.T) {
	run := func(env ...string) string {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		cmd.Env = append(append(os.Environ(), "MYGO_TEST_BEFORE_RUN=1"), env...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", err, out)
		}
		return string(out)
	}
	out := run()
	wants := []string{"dark: true\n", "dock menu: true\n"}
	for _, c := range needsAppCalls {
		wants = append(wants, c.name+": mygo: "+c.name+" called before ")
	}
	for _, want := range wants {
		if !strings.Contains(out, want) {
			t.Errorf("no %q in the output:\n%s", want, out)
		}
	}

	client := filepath.Join(t.TempDir(), "mygo.ts")
	run("MYGO_GENERATE=" + client)
	if _, err := os.Stat(client); err != nil {
		t.Error(err)
	}
}

func TestGlobalShortcut(t *testing.T) {
	pressed := make(chan struct{}, 1)
	if err := GlobalShortcut.Register("CmdOrCtrl+Shift+K", func() { pressed <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	if err := GlobalShortcut.Register("Shift+CmdOrCtrl+k", func() {}); err == nil {
		t.Error("equivalent accelerator registered twice")
	}
	if !GlobalShortcut.IsRegistered("cmdorctrl+shift+k") {
		t.Error("IsRegistered")
	}
	onMain(func() { fb.PressHotkey("CmdOrCtrl+Shift+K") })
	select {
	case <-pressed:
	case <-time.After(time.Second):
		t.Error("shortcut callback not called")
	}
	GlobalShortcut.Unregister("CmdOrCtrl+Shift+K")
	if GlobalShortcut.IsRegistered("CmdOrCtrl+Shift+K") {
		t.Error("Unregister")
	}
}

// TestNotificationShowFails: Show returns what the backend answers, and
// forgets a notification it could not show, which nothing will click.
func TestNotificationShowFails(t *testing.T) {
	onMain(func() { fb.NotificationError = ErrNotificationsDenied })
	t.Cleanup(func() { onMain(func() { fb.NotificationError = nil }) })
	n := NewNotification(NotificationOptions{Title: "Export finished"})
	if err := n.Show(); !errors.Is(err, ErrNotificationsDenied) {
		t.Errorf("Show = %v", err)
	}
	notifications.Lock()
	_, kept := notifications.byID[n.id]
	notifications.Unlock()
	if kept {
		t.Error("the notification is kept")
	}
}

func TestParseColor(t *testing.T) {
	rgba := func(r, g, b, a uint8) platform.Color { return platform.Color{R: r, G: g, B: b, A: a} }
	tests := map[string]platform.Color{
		"#fff":                  rgba(255, 255, 255, 255),
		"#11223344":             rgba(0x11, 0x22, 0x33, 0x44),
		"#1e1e1e":               rgba(0x1e, 0x1e, 0x1e, 255),
		"rgb(10, 20, 30)":       rgba(10, 20, 30, 255),
		"rgba(10,20,30,0.5)":    rgba(10, 20, 30, 128),
		"rgb(100% 0% 0% / 50%)": rgba(255, 0, 0, 128),
		"transparent":           {},
	}
	for in, want := range tests {
		got, err := parseColor(in)
		if err != nil || got != want {
			t.Errorf("parseColor(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "#12", "red", "rgb(1,2)", "#zzzzzz"} {
		if _, err := parseColor(bad); err == nil {
			t.Errorf("parseColor(%q) should fail", bad)
		}
	}
}

func TestParseBackground(t *testing.T) {
	white, black := platform.Color{R: 255, G: 255, B: 255, A: 255}, platform.Color{A: 255}
	tests := map[string]background{
		"#fff":                                 {white, white},
		"light-dark(#fff, #000)":               {white, black},
		" Light-Dark( #FFF,rgb(0, 0, 0) ) ":    {white, black},
		"light-dark(rgb(255 255 255), #000f)":  {white, black},
		"light-dark(transparent, transparent)": {},
	}
	for in, want := range tests {
		if got, err := parseBackground(in); err != nil || got != want {
			t.Errorf("parseBackground(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "light-dark(#fff)", "light-dark(#fff, #000", "light-dark(#fff, #000, #111)", "light-dark(#fff, red)", "light-dark()"} {
		if _, err := parseBackground(bad); err == nil {
			t.Errorf("parseBackground(%q) should fail", bad)
		}
	}
}

func TestEncodeReply(t *testing.T) {
	b := encodeReply(7, "k\"1", map[string]any{"line": "a b"}, nil).appendTo(nil)
	var m map[string]jsontext.Value
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s: %v", b, err)
	}
	if strings.Contains(string(b), " ") {
		t.Error("U+2028 must be escaped for JavaScript")
	}
	b = encodeReply(8, "k", make(chan int), nil).appendTo(nil)
	if !strings.Contains(string(b), `"ok":false`) {
		t.Errorf("unencodable result should be an error: %s", b)
	}
}

// TestSingleInstanceIdentifier checks that an app with another identifier
// but the same name, as the development app of mygo dev that calls SetName,
// does not hand over to the running one.
func TestSingleInstanceIdentifier(t *testing.T) {
	if !App.RequestSingleInstanceLock() {
		t.Fatal("first instance did not get the lock")
	}
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_TEST_SECOND_INSTANCE=1", "MYGO_TEST_IDENTIFIER=com.example.mygotest.dev")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "locked" {
		t.Fatalf("an app with another identifier printed %q", out)
	}
}

func TestSingleInstance(t *testing.T) {
	if !App.RequestSingleInstanceLock() {
		t.Fatal("first instance did not get the lock")
	}
	got := make(chan []string, 1)
	off := App.OnSecondInstance(func(args []string, wd string) {
		if wd == "" {
			t.Error("working directory not forwarded")
		}
		got <- args
	})
	defer off()
	urls := make(chan string, 1)
	defer App.OnOpenURL(func(u string) { urls <- u })()
	// The main thread reads the schemes when it delivers forwarded arguments.
	schemes := packageURLSchemes
	onMain(func() { packageURLSchemes = "mygo-test" })
	defer onMain(func() { packageURLSchemes = schemes })
	cmd := exec.Command(os.Args[0], "-test.run=^$", "--from-second", "instance", "mygo-test://open?x=1")
	cmd.Env = append(os.Environ(), "MYGO_TEST_SECOND_INSTANCE=1")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(out)) != "forwarded" {
		t.Fatalf("second instance printed %q", out)
	}
	select {
	case args := <-got:
		if !slices.Contains(args, "--from-second") || !slices.Contains(args, "instance") {
			t.Errorf("forwarded args = %q", args)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnSecondInstance not called")
	}
	// Deep links started a second instance: the first one opens them.
	select {
	case u := <-urls:
		if u != "mygo-test://open?x=1" {
			t.Errorf("OnOpenURL got %q", u)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("OnOpenURL not called for the forwarded URL")
	}
}

func TestDevReadySignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix sockets only")
	}
	dir, err := os.MkdirTemp("", "mygo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	sock := filepath.Join(dir, "ready.sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	onMain(func() {
		devReady.socket = sock
		devReady.once = sync.Once{}
	})
	defer onMain(func() { devReady.socket = "" })

	accepted := make(chan string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		msg, _ := io.ReadAll(conn)
		accepted <- string(msg)
	}()
	_, fw := testWindow(t, WindowOptions{})
	select {
	case <-accepted:
		t.Fatal("ready before the page loaded")
	case <-time.After(50 * time.Millisecond):
	}
	page(fw, `{"t":"dom-ready"}`)
	select {
	case msg := <-accepted:
		if msg != "ready\n" {
			t.Errorf("ready message = %q", msg)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the app did not report ready")
	}
}
