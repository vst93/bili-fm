// Package e2e drives the real native backend. It needs a desktop session,
// so it only runs with MYGO_E2E=1:
//
//	MYGO_E2E=1 go test ./internal/e2e
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type Greeter struct{}

func (Greeter) Greet(name string) string { return "Hello, " + name + "!" }

func (Greeter) Divide(a, b float64) (float64, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

func (Greeter) WindowID(ctx context.Context) int { return mygo.CallerWindow(ctx).ID() }

// Probe counts calls, to tell whether a forged call ran.
type Probe struct{ n atomic.Int32 }

func (p *Probe) Touch() { p.n.Add(1) }

var probe = &Probe{}

type Tick struct {
	N int `json:"n"`
}

// Streams streams numbers through channels.
type Streams struct{ stopped chan error }

// Count sends 0 to n-1.
func (Streams) Count(n int, ch *mygo.Channel[int]) error {
	for i := range n {
		if err := ch.Send(i); err != nil {
			return err
		}
	}
	return nil
}

// Forever sends numbers until the page stops it.
func (s *Streams) Forever(ctx context.Context, ch *mygo.Channel[int]) error {
	for i := 0; ; i++ {
		if err := ch.Send(i); err != nil {
			s.stopped <- ctx.Err()
			return err
		}
	}
}

var streams = &Streams{stopped: make(chan error, 1)}

var ticked = mygo.NewEvent[Tick]("ticked")

const page = `<!doctype html><html><head><title>E2E</title></head>
<body style="margin:0;background:#1e90ff"><h1 id="h">MyGo</h1>
<script>
window.results = [];
mygo.on('ticked', (t) => results.push('tick' + t.n));
window.run = async () => {
  results.push(await mygo.call('Greeter.Greet', 'e2e'));
  results.push(await mygo.call('Greeter.Divide', 10, 4));
  try { await mygo.call('Greeter.Divide', 1, 0) } catch (e) { results.push(e.name + ':' + e.message) }
  results.push(await mygo.call('Greeter.WindowID') === mygo.windowId);
  const r = await fetch('/echo', { method: 'POST', body: 'ping' });
  results.push(r.status + ':' + await r.text());
  return results;
};
</script></body></html>`

func TestMain(m *testing.M) {
	if os.Getenv("MYGO_E2E") == "" {
		fmt.Println("skipping e2e tests; set MYGO_E2E=1 to run them in a desktop session")
		os.Exit(0)
	}
	if mode := os.Getenv("MYGO_E2E_CLIPBOARD_PEER"); mode != "" {
		clipboardPeer(mode)
		return
	}
	if os.Getenv("MYGO_E2E_QUIT_DURING_DIALOG") == "1" {
		quitDuringDialog()
		return
	}
	if s := os.Getenv("MYGO_E2E_BEFORE_RUN"); s != "" {
		beforeRun(mygo.ThemeSource(s))
		return
	}
	mygo.Bind(Greeter{}, probe, streams, Bench{})
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, page)
	})
	mux.HandleFunc("/report.bin", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="report.bin"`)
		io.WriteString(w, "binary report")
	})
	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "echo:%s", b)
	})
	usePlugins(mux)
	useBench(mux)
	if err := mygo.Protocol.Handle("app", mux); err != nil {
		panic(err)
	}
	mygo.App.OnWindowAllClosed(func() {})
	// A window requested from a goroutine before Run waits for the app to
	// be ready (TestEarlyWindow).
	go func() { earlyWindow <- mygo.NewWindow(mygo.WindowOptions{Hidden: true, Title: "early"}) }()
	code := 1
	mygo.App.WhenReady(func() {
		go func() {
			warmUp()
			code = m.Run()
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(code)
}

// warmUp shows a page once before the tests, so that the first test to
// show one does not pay for starting the webview: WebView2 starts its
// browser, GPU and renderer processes with the first page, which took a
// slow runner over ten seconds.
func warmUp() {
	start := time.Now()
	w := mygo.NewWindow(mygo.WindowOptions{Hidden: true, Title: "warm-up"})
	defer w.Destroy()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if _, err := w.Page().EvalContext(ctx, "1"); err != nil {
		fmt.Printf("e2e: the webview did not start in %v: %v\n", time.Since(start).Round(time.Millisecond), err)
	} else if d := time.Since(start); d > 3*time.Second {
		fmt.Printf("e2e: the webview took %v to start\n", d.Round(time.Millisecond))
	}
}

// quitDuringDialog is a helper process for TestQuitDuringDialog: it quits
// while an application-modal dialog is open, and exits once Run returns.
func quitDuringDialog() {
	mygo.App.WhenReady(func() {
		go func() {
			res, err := mygo.Dialog.Message(mygo.MessageOptions{Message: "Quitting soon", Buttons: []string{"OK", "Cancel"}})
			fmt.Printf("dialog: %d %v\n", res.Button, err)
		}()
		go func() {
			time.Sleep(500 * time.Millisecond)
			mygo.App.Quit()
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("run returned")
}

func TestQuitDuringDialog(t *testing.T) {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "MYGO_E2E_QUIT_DURING_DIALOG=1")
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err != nil || !strings.Contains(out.String(), "run returned") {
			t.Errorf("exit: %v, output %q", err, out.String())
		}
		// The dialog was dismissed with its cancel button.
		if strings.Contains(out.String(), "dialog:") && !strings.Contains(out.String(), "dialog: 1 <nil>") {
			t.Errorf("dialog result: %q", out.String())
		}
	case <-time.After(10 * time.Second):
		cmd.Process.Kill()
		t.Fatalf("the app did not quit while a dialog was open; output %q", out.String())
	}
}

// beforeRunCalls need the running app. Made before Run, they used to crash
// on Linux, whose backend loads GTK in Init, while on macOS the dialog
// hung, Theme.IsDark answered false and NewTray failed.
var beforeRunCalls = []struct {
	name string
	call func()
}{
	{"Clipboard.ReadText", func() { mygo.Clipboard.ReadText() }},
	{"Theme.IsDark", func() { mygo.Theme.IsDark() }},
	{"Dialog.Message", func() { mygo.Dialog.Message(mygo.MessageOptions{Message: "Too early"}) }},
	{"NewTray", func() { mygo.NewTray(mygo.TrayOptions{}) }},
}

// beforeRun is a helper process for TestBeforeRun: it does what main may
// do before Run, when the backend is not initialized yet (on Linux, GTK is
// not even loaded), and what it may not, then reports what took effect
// once the app is ready.
func beforeRun(source mygo.ThemeSource) {
	mygo.Theme.SetSource(source)
	clicked := make(chan struct{}, 1)
	mygo.App.Dock.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: "New Window", Click: func(*mygo.MenuItem, *mygo.Window) { clicked <- struct{}{} }},
	}))
	fmt.Println("locale:", mygo.App.Locale() != "")
	mygo.Power.IsOnBattery()
	for _, c := range beforeRunCalls {
		func() {
			defer func() { fmt.Printf("%s: %v\n", c.name, recover()) }()
			c.call()
		}()
	}
	mygo.App.WhenReady(func() {
		fmt.Println("dark:", mygo.Theme.IsDark())
		go func() {
			defer mygo.App.Quit()
			titles, ok := dockMenu(0)
			if !ok {
				return
			}
			select {
			case <-clicked:
				fmt.Printf("dock menu: %q clicked\n", titles)
			case <-time.After(3 * time.Second):
				fmt.Printf("dock menu: %q\n", titles)
			}
		}()
	})
	if err := mygo.App.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// TestBeforeRun: apps set up and configure themselves in main, before Run,
// e.g. with a saved appearance, while calls that need the running app fail
// clearly on every platform. Both appearances are tried, so one differs
// from the system's.
func TestBeforeRun(t *testing.T) {
	for _, source := range []mygo.ThemeSource{mygo.ThemeDark, mygo.ThemeLight} {
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
		cmd.Env = append(os.Environ(), "MYGO_E2E_BEFORE_RUN="+string(source))
		b, err := cmd.CombinedOutput()
		cancel()
		out := string(b)
		if err != nil {
			t.Errorf("%s: %v:\n%s", source, err, out)
			continue
		}
		wants := []string{"locale: true\n", fmt.Sprintf("dark: %v\n", source == mygo.ThemeDark)}
		for _, c := range beforeRunCalls {
			wants = append(wants, c.name+": mygo: "+c.name+" called before ")
		}
		if runtime.GOOS == "darwin" {
			wants = append(wants, `dock menu: ["New Window"] clicked`)
		}
		for _, want := range wants {
			if !strings.Contains(out, want) {
				t.Errorf("%s: no %q in the output:\n%s", source, want, out)
			}
		}
	}
}

// TestIframeCannotCall: the message handler is reachable from iframes, but
// only the page's bridge may call Go.
func TestIframeCannotCall(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	forged := `{"t":"call","id":1,"k":"x","m":"Probe.Touch","a":[]}`
	frame := `<script>
const h = window.webkit && webkit.messageHandlers && webkit.messageHandlers.mygo;
if (h) h.postMessage(` + "`" + forged + "`" + `);
parent.postMessage(h ? "posted" : "no handler", "*");
</script>`
	src, _ := json.Marshal(frame)
	w.Page().LoadHTML(`<p>main</p><script>
addEventListener("message", (e) => { window.frameResult = e.data });
const f = document.createElement("iframe");
f.src = "data:text/html," + encodeURIComponent(`+string(src)+`);
document.body.append(f);
</script>`, "app://localhost/")
	waitFor(t, w, "window.frameResult")
	result, _ := mygo.EvalAs[string](w.Page(), "window.frameResult")
	if result != "posted" {
		t.Skipf("the iframe had no message handler (%q)", result)
	}
	time.Sleep(200 * time.Millisecond)
	if n := probe.n.Load(); n != 0 {
		t.Fatalf("a call forged by an iframe ran %d times", n)
	}
	if _, err := w.Page().Eval("mygo.call('Probe.Touch')"); err != nil || probe.n.Load() != 1 {
		t.Errorf("the page's own call: %v, count %d", err, probe.n.Load())
	}
	probe.n.Store(0)
}

func TestPopupMenu(t *testing.T) {
	if _, ok := dismissPopups(); !ok {
		t.Skip("popup automation not available on this platform")
	}
	clicks := make(chan *mygo.Window, 1)
	menu := mygo.NewMenu([]*mygo.MenuItem{{Label: "One", Click: func(_ *mygo.MenuItem, win *mygo.Window) {
		clicks <- win
	}}, {Label: "Two"}})
	popup := func(show func()) (shown int) {
		t.Helper()
		done := make(chan struct{})
		go func() { show(); close(done) }()
		time.Sleep(300 * time.Millisecond)
		shown, _ = dismissPopups()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("the popup did not return")
		}
		return shown
	}
	// Without a window GTK cannot place the menu: it must not block.
	if n := popup(func() { menu.Popup(nil) }); n != 0 {
		t.Errorf("popup without a window: %d menus shown", n)
	}
	w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200})
	w.Page().LoadHTML("<p>menu</p>", "")
	waitFor(t, w, "document.readyState === 'complete'")
	if n := popup(func() { menu.PopupAt(w, 20, 20) }); n != 1 {
		t.Errorf("PopupAt: %d menus shown", n)
	}
	if runtime.GOOS != "linux" {
		return
	}
	done := make(chan struct{})
	go func() { menu.PopupAt(w, 20, 20); close(done) }()
	t.Cleanup(func() { dismissPopups() })
	eventually(t, "the context menu", func() bool {
		menus, _ := popupMenus()
		return len(menus) == 1
	})
	if !choosePopupItem("One") {
		t.Fatal("the context menu has no item One")
	}
	select {
	case got := <-clicks:
		if got != w {
			t.Errorf("context menu window = %v, want window %d", got, w.ID())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the context menu click was not delivered")
	}
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("the context menu did not return after choosing an item")
	}
}

var (
	earlyWindow  = make(chan *mygo.Window, 1)
	earlyChecked atomic.Bool
)

func TestEarlyWindow(t *testing.T) {
	// The window is requested once per process, before Run.
	if earlyChecked.Swap(true) {
		t.Skip("the window requested before Run was checked in the first run")
	}
	select {
	case w := <-earlyWindow:
		defer w.Destroy()
		if w.Title() != "early" {
			t.Errorf("title = %q", w.Title())
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the window requested before Run was not created")
	}
}

// waitFor polls the page until expr is truthy. Until a page being loaded
// commits, expr runs in the previous one, such as the empty document a new
// window starts with, which is complete already: wait for something of the
// page itself. The 10 seconds start once the page first answers: WebView2
// creates a window's webview asynchronously, and evaluating waits for it,
// which may take longer than that on a slow runner.
func waitFor(t testing.TB, w *mygo.Window, expr string) {
	t.Helper()
	p := w.Page()
	check := "!!(" + expr + ")"
	start := time.Now()
	first, cancel := context.WithTimeout(t.Context(), time.Minute)
	v, err := p.EvalContext(first, check)
	late := err != nil && first.Err() != nil
	cancel()
	answered := time.Since(start)
	if late {
		t.Fatalf("waiting for %s: the page did not answer in %v: %v; %s", expr, answered.Round(time.Millisecond), err, pageState(p))
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for v != true {
		select {
		case <-ctx.Done():
			msg := fmt.Sprintf("timed out waiting for %s, 10s after the page first answered (in %v)", expr, answered.Round(time.Millisecond))
			if err != nil {
				msg += "; the last check failed: " + err.Error()
			}
			t.Fatalf("%s; %s", msg, pageState(p))
		case <-time.After(20 * time.Millisecond):
		}
		v, err = p.EvalContext(ctx, check)
	}
}

// pageState describes where a page is, for failures: what the webview
// reports, and what the page itself says, if it answers.
func pageState(p *mygo.Page) string {
	s := fmt.Sprintf("the webview is at %q (loading: %v)", p.URL(), p.IsLoading())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	v, err := p.EvalContext(ctx, "[location.href, document.title, document.readyState]")
	if err != nil {
		return s + ", and the page did not answer: " + err.Error()
	}
	a, _ := v.([]any)
	if len(a) != 3 {
		return fmt.Sprintf("%s, and the page answered %v", s, v)
	}
	return fmt.Sprintf("%s; location.href %q, document.title %q, document.readyState %q", s, a[0], a[1], a[2])
}

func newWindow(t *testing.T, opts mygo.WindowOptions) *mygo.Window {
	t.Helper()
	w := mygo.NewWindow(opts)
	t.Cleanup(w.Destroy)
	return w
}

func TestIPCAndProtocol(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "IPC", Width: 400, Height: 300})
	var dom atomic.Bool
	w.Page().OnDOMReady(func() { dom.Store(true) })
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.run")
	got, err := mygo.EvalAs[[]any](w.Page(), "run()")
	if err != nil {
		t.Fatal(err)
	}
	want := "[Hello, e2e! 2.5 CallError:division by zero true 200:echo:ping]"
	if fmt.Sprint(got) != want {
		t.Errorf("results = %v, want %v", got, want)
	}
	if !dom.Load() {
		t.Error("OnDOMReady not called")
	}
	for i := 1; i <= 3; i++ {
		if err := ticked.Emit(w, Tick{N: i}); err != nil {
			t.Fatal(err)
		}
	}
	waitFor(t, w, "results.includes('tick3')")
	ticks, _ := mygo.EvalAs[string](w.Page(), "results.filter(r => String(r).startsWith('tick')).join(',')")
	if ticks != "tick1,tick2,tick3" {
		t.Errorf("events arrived as %q", ticks)
	}
	if title := w.Title(); title != "E2E" {
		t.Errorf("window title should follow the page, got %q", title)
	}
	if u := w.Page().URL(); u != "app://localhost/" {
		t.Errorf("URL = %q", u)
	}
}

func TestChannels(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.run")
	start := time.Now()
	got, err := mygo.EvalAs[string](w.Page(), `(async () => {
		const ch = mygo.channel();
		const done = mygo.call("Streams.Count", 100000, ch);
		let n = 0, sum = 0;
		for await (const v of ch) {
			if (v !== n++) throw new Error("out of order: " + v);
			sum += v;
		}
		await done;
		return n + ":" + sum;
	})()`)
	if err != nil || got != "100000:4999950000" {
		t.Fatalf("streamed %q, %v", got, err)
	}
	t.Logf("100000 values in %v", time.Since(start))

	// Leaving the loop stops the method: Send fails, the context is canceled.
	got, err = mygo.EvalAs[string](w.Page(), `(async () => {
		const ch = mygo.channel();
		const done = mygo.call("Streams.Forever", ch);
		for await (const v of ch) if (v === 50000) break;
		return await done.then(() => "resolved", (e) => e.message);
	})()`)
	if err != nil || got != mygo.ErrChannelClosed.Error() {
		t.Errorf("stopped stream: %q, %v", got, err)
	}
	select {
	case err := <-streams.stopped:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("context of the stopped call: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the method did not stop")
	}
}

func TestEvalForms(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	w.Page().LoadHTML("<p>eval</p>", "")
	waitFor(t, w, "document.querySelector('p')")
	cases := []struct{ code, want string }{
		{"1 + 1", "2"},
		{"document.querySelector('p').textContent;", "eval"},
		{"const xs = [1, 2]; return xs.map(x => x * 3)", "[3 6]"},
		{"new Promise(r => setTimeout(() => r({ok: true}), 10))", "map[ok:true]"},
		{"undefined", "<nil>"},
	}
	for _, c := range cases {
		v, err := w.Page().Eval(c.code)
		if err != nil || fmt.Sprint(v) != c.want {
			t.Errorf("Eval(%q) = %v, %v; want %s", c.code, v, err, c.want)
		}
	}
	var evalErr *mygo.EvalError
	if _, err := w.Page().Eval("throw new Error('boom')"); !errors.As(err, &evalErr) || evalErr.Message != "boom" {
		t.Errorf("Eval(throw) = %v", err)
	}
}

func TestVibrancy(t *testing.T) {
	for _, material := range []mygo.Vibrancy{mygo.VibrancySidebar, mygo.VibrancyMica, "no-such-material"} {
		w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200, Vibrancy: material, Transparent: true})
		w.Page().LoadHTML("<p>vibrancy</p>", "")
		waitFor(t, w, "document.readyState === 'complete'")
		if attached, ok := webViewAttached(w); ok && !attached {
			t.Errorf("vibrancy %q: the page is not in the window", material)
		}
	}
}

// On Windows, a window with a material behind its page has no redirection
// bitmap, which hides what GDI draws: the controls of a hidden title bar
// show through DirectComposition, in every window again once its device
// is gone, and the menus open in a popup, with no bar to show.
func TestVibrancyWindowControls(t *testing.T) {
	prev := mygo.App.Menu()
	defer mygo.App.SetMenu(prev)
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{{Label: "Item"}}}}))
	opts := mygo.WindowOptions{X: 40, Y: 40, Width: 360, Height: 200, TitleBarStyle: mygo.TitleBarHidden,
		Vibrancy: mygo.VibrancyMica, Transparent: true}
	a := newWindow(t, opts)
	opts.X = 440
	b := newWindow(t, opts)
	noRedirect, _, ok := composition(a)
	if !ok {
		t.Skip("only Windows windows have redirection bitmaps")
	}
	if !noRedirect {
		t.Skip("no system backdrops before Windows 11 22H2, or no DirectComposition")
	}
	composed := func(when string) {
		t.Helper()
		for _, w := range []*mygo.Window{a, b} {
			eventually(t, "the window controls through DirectComposition "+when, func() bool {
				_, composed, _ := composition(w)
				return composed
			})
		}
	}
	composed("in new windows")
	if names, _ := titleButtons(a); fmt.Sprint(names) != "[minimize maximize close]" {
		t.Errorf("the window controls are %q", names)
	}

	// What the user sees: the close button red under the pointer, as in a
	// window without a material, unless the screen cannot be read.
	red := func(w *mygo.Window) bool {
		r, g, bl, _ := titleButtonColor(w, "close")
		return r == 0xC4 && g == 0x2B && bl == 0x1C || r == 0xE8 && g == 0x11 && bl == 0x23
	}
	plain := newWindow(t, mygo.WindowOptions{X: 40, Y: 280, Width: 360, Height: 200, TitleBarStyle: mygo.TitleBarHidden})
	readable := false
	for deadline := time.Now().Add(3 * time.Second); !readable && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		readable = red(plain)
	}
	if readable {
		eventually(t, "the close button red under the pointer, through DirectComposition", func() bool { return red(b) })
	} else {
		t.Log("the screen does not show the close button of a window without a material: not comparing")
	}

	if loseComposition(false) {
		composed("after their device failed")
	}
	if loseComposition(true) {
		composed("after the GPU device was removed")
	}

	w := newWindow(t, mygo.WindowOptions{X: 440, Y: 280, Width: 360, Height: 200, Vibrancy: mygo.VibrancyMica})
	if shown, _ := menuBarShown(w); shown {
		t.Error("a window with a material has a menu bar, which would not show")
	}
	w.Focus()
	time.Sleep(200 * time.Millisecond)
	if openMenus(w, 0) {
		var menus [][]string
		eventually(t, "Alt to open the menus in a popup", func() bool {
			menus, _ = popupMenus()
			return len(menus) > 0
		})
		if fmt.Sprint(menus) != "[[App]]" {
			t.Errorf("Alt opened %q, want the menus of the bar", menus)
		}
		closeMenus()
		eventually(t, "the menus to close", func() bool { menus, _ = popupMenus(); return len(menus) == 0 })
	}
}

// The toolbar that insets the traffic lights holds no items: in full screen
// it must hide with the menu bar instead of covering the top of the page.
func TestFullScreenToolbar(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200, TitleBarStyle: mygo.TitleBarHiddenInset})
	hides, ok := fullScreenHidesToolbar(w)
	if !ok {
		t.Skip("only macOS windows have a toolbar")
	}
	if !hides {
		t.Error("the toolbar of an inset title bar stays in full screen")
	}
}

// AppKit lays the title bar out again when the title or the appearance
// changes, which must not move the traffic lights back.
func TestTrafficLightPosition(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 240, TitleBarStyle: mygo.TitleBarHidden,
		TrafficLightPosition: &mygo.Point{X: 18, Y: 19}})
	if _, _, ok := trafficLights(w); !ok {
		t.Skip("only macOS windows have traffic lights")
	}
	placed := func(when string) {
		t.Helper()
		eventually(t, "the close button at (18, 19) "+when, func() bool {
			x, y, _ := trafficLights(w)
			return x == 18 && y == 19
		})
	}
	placed("in a new window")
	w.Page().LoadHTML("<title>Traffic lights</title>", "")
	eventually(t, "the page's title", func() bool { return w.Title() == "Traffic lights" })
	placed("once the window takes the page's title")
	defer mygo.Theme.SetSource(mygo.Theme.Source())
	if mygo.Theme.IsDark() {
		mygo.Theme.SetSource(mygo.ThemeLight)
	} else {
		mygo.Theme.SetSource(mygo.ThemeDark)
	}
	placed("after the appearance changed")
}

// GTK gives frameless windows no resize borders, so the outer pixels of
// the page resize them instead, as the borders do on macOS and Windows.
func TestFramelessResizeEdges(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Frameless: true, X: 60, Y: 60, Width: 320, Height: 240})
	if _, ok := resizeCursor(w); !ok {
		t.Skip("frameless windows keep native resize borders on this platform")
	}
	w.Page().LoadHTML(`<body style="margin:0;height:100vh" onmousedown="window.pressed=(window.pressed||0)+1"></body>`, "")
	waitFor(t, w, "document.readyState === 'complete'")
	cursor := func(want string) {
		t.Helper()
		eventually(t, fmt.Sprintf("the cursor %q", want), func() bool { c, _ := resizeCursor(w); return c == want })
	}
	// Window managers grab the pointer a moment after the press.
	drag := func(x, y int) {
		pressButton(true)
		time.Sleep(100 * time.Millisecond)
		movePointer(x, y)
		time.Sleep(100 * time.Millisecond)
		pressButton(false)
	}

	b := w.Bounds()
	if !movePointer(b.X+b.Width-2, b.Y+b.Height/2) {
		t.Skip("moving the pointer needs an X server")
	}
	cursor("e-resize")
	drag(b.X+b.Width+58, b.Y+b.Height/2)
	eventually(t, "the right edge to follow the pointer", func() bool { return w.Bounds().Width == b.Width+60 })

	// Corners reach further along the edges.
	b = w.Bounds()
	movePointer(b.X+b.Width-10, b.Y+b.Height-2)
	cursor("se-resize")
	drag(b.X+b.Width-40, b.Y+b.Height-22)
	eventually(t, "the corner to follow the pointer", func() bool {
		r := w.Bounds()
		return r.Width == b.Width-30 && r.Height == b.Height-20
	})

	// Elsewhere the page gets the mouse and shows its own cursor; it saw
	// none of the presses on the edges.
	movePointer(b.X+100, b.Y+100)
	cursor("")
	pressButton(true)
	pressButton(false)
	waitFor(t, w, "window.pressed === 1")
}

func TestDockedDevTools(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "DevTools", Width: 800, Height: 600, Page: mygo.PageOptions{DevTools: mygo.DevToolsEnabled}})
	w.Page().LoadHTML("<p>inspect me</p>", "")
	waitFor(t, w, "document.readyState === 'complete'")
	if !dockDevTools(w) {
		t.Skip("docking the inspector is not automated here")
	}
	defer w.Page().CloseDevTools()
	eventually(t, "the inspector to open", w.Page().IsDevToolsOpened)
	// Docked anywhere else in the window, it breaks the title bar.
	if place := dockedDevToolsPlace(w); place != "content" {
		t.Errorf("docked inspector place = %q, want %q", place, "content")
	}
}

func TestWindowGeometryAndState(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 500, Height: 400, X: 100, Y: 120})
	b := w.Bounds()
	if b.Width != 500 || b.Height != 400 || b.X != 100 || b.Y != 120 {
		t.Errorf("bounds = %+v", b)
	}
	w.SetBounds(mygo.Rectangle{X: 150, Y: 160, Width: 420, Height: 320})
	if b := w.Bounds(); b != (mygo.Rectangle{X: 150, Y: 160, Width: 420, Height: 320}) {
		t.Errorf("SetBounds -> %+v", b)
	}
	cw, ch := w.ContentSize()
	widthOK := cw == 420
	if runtime.GOOS == "windows" {
		widthOK = cw > 380 && cw <= 420 // the bounds include the resize borders
	}
	if !widthOK || ch > 320 || (runtime.GOOS != "linux" && ch == 320) {
		t.Errorf("content size = %dx%d, expected the title bar to be excluded", cw, ch)
	}
	w.SetTitle("Renamed")
	if w.Title() != "Renamed" {
		t.Errorf("title = %q", w.Title())
	}
	if !w.IsVisible() {
		t.Error("window should be visible")
	}
	w.Hide()
	if w.IsVisible() {
		t.Error("Hide")
	}
	w.Show()
	w.SetAlwaysOnTop(true)
	if !w.IsAlwaysOnTop() {
		t.Error("SetAlwaysOnTop")
	}
	w.SetOpacity(0.5)
	if o := w.Opacity(); o < 0.49 || o > 0.51 {
		t.Errorf("opacity = %v", o)
	}
	w.Page().SetZoomFactor(1.5)
	if z := w.Page().ZoomFactor(); z != 1.5 {
		t.Errorf("zoom = %v", z)
	}
}

// A window the user cannot resize still takes the sizes the app gives it,
// smaller ones too: GTK keeps such windows at least as large as their
// default size.
func TestResizeFixedWindow(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 500, Height: 400, UseContentSize: true, DisableResize: true})
	w.Page().LoadHTML("<p>fixed</p>", "")
	for _, size := range [][2]int{{360, 240}, {540, 420}} {
		w.SetContentSize(size[0], size[1])
		eventually(t, fmt.Sprintf("a %dx%d page", size[0], size[1]), func() bool {
			got, _ := mygo.EvalAs[[2]int](w.Page(), "[innerWidth, innerHeight]")
			return got == size
		})
	}
	if w.IsResizable() {
		t.Error("the window became resizable")
	}
}

// A page whose Content Security Policy runs only its own scripts still
// talks to Go: the bridge and the messages it receives are not the page's.
func TestStrictCSP(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200})
	w.Page().LoadHTML(`<meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-e2e'">
<script nonce="e2e">
window.results = [];
(async () => {
  results.push(await mygo.call("Greeter.Greet", "csp"));
  const values = [];
  await mygo.call("Streams.Count", 3, mygo.channel((v) => values.push(v)));
  results.push(values.join(","));
})().catch((e) => results.push("error: " + e.message));
</script>`, "")
	waitFor(t, w, "window.results && results.length === 2")
	got, err := mygo.EvalAs[[]string](w.Page(), "results")
	if err != nil || fmt.Sprint(got) != "[Hello, csp! 0,1,2]" {
		t.Errorf("results = %q, %v", got, err)
	}
}

// A window with an empty menu of its own has no menu bar, where windows
// get the application's (Linux, Windows).
func TestEmptyWindowMenu(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200})
	if _, supported := menuBarShown(w); !supported {
		t.Skip("the menu bar belongs to the application")
	}
	prev := mygo.App.Menu()
	defer mygo.App.SetMenu(prev)
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{{Label: "Item"}}}}))
	if shown, _ := menuBarShown(w); !shown {
		t.Fatal("the window has no menu bar of the application")
	}
	w.SetMenu(mygo.NewMenu(nil))
	if shown, _ := menuBarShown(w); shown {
		t.Error("an empty menu left the menu bar")
	}
	w.SetMenu(nil)
	if shown, _ := menuBarShown(w); !shown {
		t.Error("SetMenu(nil) did not bring back the menu bar of the application")
	}
}

// A window in full screen has no menu bar, so the page fills the screen,
// and Alt opens its menus all the same. It gets the bar back, and its size,
// when it leaves full screen, and the app hears of no size in between.
func TestFullScreenMenuBar(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("the macOS menu bar belongs to the application")
	}
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	prev := mygo.App.Menu()
	defer mygo.App.SetMenu(prev)
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{{Label: "Item"}}}}))
	if shown, _ := menuBarShown(w); !shown {
		t.Fatal("the window has no menu bar of the application")
	}
	width, height := w.ContentSize()
	var heard []string // the content sizes of OnResize, on the main thread
	w.OnResize(func() {
		width, height := w.ContentSize()
		heard = append(heard, fmt.Sprintf("%dx%d", width, height))
	})
	// Windows lays the window out at each step of SetFullScreen, which runs
	// on the main thread: the app hears of the last only.
	heardOnce := func(when string) {
		t.Helper()
		var got []string
		mygo.RunOnMain(func() { got, heard = heard, nil })
		width, height := w.ContentSize()
		want := fmt.Sprintf("%dx%d", width, height)
		if runtime.GOOS == "windows" && slices.ContainsFunc(got, func(s string) bool { return s != want }) {
			t.Errorf("%s, the app heard of the sizes %q, want %s only", when, got, want)
		}
	}

	w.SetFullScreen(true)
	for deadline := time.Now().Add(5 * time.Second); !w.IsFullScreen(); time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Skip("full screen needs a window manager, which Xvfb lacks")
		}
	}
	if shown, _ := menuBarShown(w); shown {
		t.Error("the menu bar shows in full screen")
	}
	eventually(t, "the page to fill the window in full screen", func() bool {
		_, h := w.Size()
		_, ch := w.ContentSize()
		return ch == h
	})
	heardOnce("entering full screen")
	w.Focus()
	time.Sleep(200 * time.Millisecond)
	if openMenus(w, 0) { // Windows: in a popup
		var menus [][]string
		eventually(t, "Alt to open the menus in full screen", func() bool {
			menus, _ = popupMenus()
			return len(menus) > 0
		})
		if fmt.Sprint(menus) != "[[App]]" {
			t.Errorf("Alt opened %q, want the menus of the bar", menus)
		}
		closeMenus()
		eventually(t, "the menus to close", func() bool { menus, _ = popupMenus(); return len(menus) == 0 })
	} else if during, after, ok := enterMenuBar(w, "Alt_L"); ok { // Linux: in the bar
		if !during {
			t.Error("Alt did not open the menus in full screen")
		}
		if after {
			t.Error("the menu bar still shows in full screen once its menus closed")
		}
	}

	w.SetFullScreen(false)
	eventually(t, "the window out of full screen", func() bool { return !w.IsFullScreen() })
	eventually(t, "the menu bar back after full screen", func() bool { shown, _ := menuBarShown(w); return shown })
	eventually(t, "the size from before full screen", func() bool {
		cw, ch := w.ContentSize()
		return cw == width && ch == height
	})
	heardOnce("leaving full screen")
}

// Alt and a letter open the menu of the letter in native UI, which passes
// them on, unlike the page: in the bar, and in the popup of a window in
// full screen. Full screen closes the menus of the bar it takes away.
func TestMenuLetters(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows menus take letters here")
	}
	prev := mygo.App.Menu()
	defer mygo.App.SetMenu(prev)
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{{Label: "Item"}}}}))
	view := func(c *ui.Context) { ui.Box(c).Fill() }
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, FullScreen: true, Content: ui.View(view)})
	eventually(t, "the window in full screen", w.IsFullScreen)
	if shown, _ := menuBarShown(w); shown {
		t.Error("a window created in full screen shows its menu bar")
	}
	menusAfter := func(letter byte, want [][]string) {
		t.Helper()
		if !openMenus(w, letter) {
			t.Skip("the window cannot come to the front, where the keyboard types")
		}
		var menus [][]string
		if len(want) == 0 {
			time.Sleep(500 * time.Millisecond)
			menus, _ = popupMenus()
		}
		for deadline := time.Now().Add(3 * time.Second); fmt.Sprint(menus) != fmt.Sprint(want) && time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
			menus, _ = popupMenus()
		}
		if fmt.Sprint(menus) != fmt.Sprint(want) {
			t.Errorf("Alt+%c opened %q, want %q", letter, menus, want)
		}
	}

	menusAfter('A', [][]string{{"Item"}, {"App"}}) // the menu of the letter above the popup
	closeMenus()
	menusAfter('Z', nil) // names no menu
	closeMenus()

	w.SetFullScreen(false)
	eventually(t, "the menu bar back after full screen", func() bool { shown, _ := menuBarShown(w); return shown })
	menusAfter('A', [][]string{{"Item"}}) // the bar is no popup
	w.SetFullScreen(true)
	eventually(t, "full screen to close the menus of the bar", func() bool { menus, _ := popupMenus(); return len(menus) == 0 })
	closeMenus()
	if shown, _ := menuBarShown(w); shown {
		t.Error("the menu bar shows in full screen")
	}
}

// Resizing a centered window keeps its position, also when it was
// resized right before, which GTK has not confirmed yet.
func TestCenterThenResize(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, X: 10, Y: 10})
	w.SetSize(380, 280)
	w.Center()
	w.SetSize(360, 260)
	eventually(t, "a centered window", func() bool {
		b := w.Bounds()
		return b.X > 60 && b.Y > 60 && b.Width == 360 && b.Height == 260
	})
}

// New windows report the bounds they were given right away and while the
// window manager places them, also when they show again: X11 has a window
// where GTK created it until then, and a reparenting window manager its
// frame where it created that.
func TestNewWindowBounds(t *testing.T) {
	steady := func(w *mygo.Window, what string, want mygo.Rectangle) {
		t.Helper()
		for end := time.Now().Add(200 * time.Millisecond); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
			if b := w.Bounds(); b != want {
				t.Fatalf("%s: bounds = %+v, want %+v", what, b, want)
			}
		}
	}
	for i := range 4 {
		want := mygo.Rectangle{X: 100 + 20*i, Y: 120 + 10*i, Width: 400, Height: 300}
		w := newWindow(t, mygo.WindowOptions{X: want.X, Y: want.Y, Width: want.Width, Height: want.Height})
		steady(w, fmt.Sprintf("window %d", i), want)
		w.Hide()
		steady(w, fmt.Sprintf("hidden window %d", i), want)
		w.Show()
		steady(w, fmt.Sprintf("window %d shown again", i), want)
	}
	w := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200})
	steady(w, "centered window", w.Bounds())
}

// Small windows the user cannot resize keep the sizes the app gives them:
// GTK made them at least 200x200, their natural size with only a web view,
// and asked for their previous size again from an early report of it.
func TestSmallFixedWindow(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, UseContentSize: true, DisableResize: true})
	w.Page().LoadHTML("<p>small</p>", "")
	for _, size := range [][2]int{{300, 150}, {300, 100}, {280, 80}} {
		w.SetContentSize(size[0], size[1])
		page := func() [2]int {
			got, _ := mygo.EvalAs[[2]int](w.Page(), "[innerWidth, innerHeight]")
			return got
		}
		eventually(t, fmt.Sprintf("a %dx%d page", size[0], size[1]), func() bool { return page() == size })
		time.Sleep(300 * time.Millisecond) // and it stays so
		width, height := w.ContentSize()
		if got := page(); got != size || width != size[0] || height != size[1] {
			t.Errorf("asked for %v: the page is %v, the content size %dx%d", size, got, width, height)
		}
	}
}

// Bounds reports the size GTK gives a window, not one it refused.
func TestResizeBelowMinimum(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("GTK keeps windows within their minimum size")
	}
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, MinWidth: 320, MinHeight: 240, UseContentSize: true})
	w.Page().LoadHTML("<p>minimum</p>", "")
	w.SetContentSize(200, 100)
	if width, height := w.ContentSize(); width != 320 || height != 240 {
		t.Errorf("content size = %dx%d, want the minimum", width, height)
	}
	eventually(t, "a 320x240 page", func() bool {
		got, _ := mygo.EvalAs[[2]int](w.Page(), "[innerWidth, innerHeight]")
		return got == [2]int{320, 240}
	})
	w.SetContentSize(320, 200) // the window already has the size GTK gives
	if width, height := w.ContentSize(); width != 320 || height != 240 {
		t.Errorf("content size = %dx%d, want the minimum", width, height)
	}
}

// eventually waits for cond, which polls the window system.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// windowStateRuns numbers the runs of TestWindowState: the state file is
// loaded once per process, so each run uses keys of its own.
var windowStateRuns atomic.Int32

func TestWindowState(t *testing.T) {
	run := strconv.Itoa(int(windowStateRuns.Add(1)))
	mygo.App.SetPath(mygo.PathUserData, t.TempDir())
	defer mygo.App.SetPath(mygo.PathUserData, "")
	closeWindow := func(w *mygo.Window) {
		closed := make(chan struct{})
		w.OnClosed(func() { close(closed) })
		w.Close()
		<-closed
	}

	w := mygo.NewWindow(mygo.WindowOptions{StateKey: "e2e-" + run, X: 140, Y: 160, Width: 480, Height: 360})
	want := mygo.Rectangle{X: 170, Y: 180, Width: 520, Height: 380}
	w.SetBounds(want)
	closeWindow(w)
	w = newWindow(t, mygo.WindowOptions{StateKey: "e2e-" + run, Width: 300, Height: 200})
	if b := w.Bounds(); b != want {
		t.Errorf("restored bounds = %+v, want %+v", b, want)
	}

	if runtime.GOOS == "linux" {
		t.Skip("maximizing needs a window manager, which Xvfb lacks")
	}
	w = mygo.NewWindow(mygo.WindowOptions{StateKey: "e2e-max-" + run, Maximized: true, Width: 500, Height: 400})
	eventually(t, "a maximized window", w.IsMaximized)
	closeWindow(w)
	w = newWindow(t, mygo.WindowOptions{StateKey: "e2e-max-" + run})
	eventually(t, "a restored maximized window", w.IsMaximized)
	w.Unmaximize()
	eventually(t, "the normal size", func() bool {
		b := w.Bounds()
		return b.Width == 500 && b.Height == 400
	})
}

// TestFileDrop drops files with synthetic DOM events, the platform side
// being played by a test hook, and checks that the page's own drag and
// drop is undisturbed.
func TestFileDrop(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Drop", Width: 400, Height: 300})
	drops := make(chan *mygo.FileDropEvent, 4)
	w.OnFileDrop(func(e *mygo.FileDropEvent) { drops <- e })
	w.Page().LoadHTML(`<body style="margin:0;height:300px"><div id=zone style="height:100px"></div><div id=item draggable=true>item</div><script>
window.log = [];
mygo.on("mygo:file-drop", (p) => log.push("event:" + p.paths.join(",") + "@" + p.x + "," + p.y));
zone.addEventListener("dragover", (e) => { e.preventDefault(); log.push("zone:dragover"); });
zone.addEventListener("drop", (e) => { e.preventDefault(); log.push("zone:drop:" + [...e.dataTransfer.files].map((f) => f.name)); });
document.addEventListener("dragover", (e) => log.push("document:dragover:" + e.defaultPrevented));
item.addEventListener("dragstart", () => log.push("item:dragstart"));
// Dispatches a drag event carrying a file at x, y and reports whether it
// was canceled.
window.drag = (target, type, x, y) => {
  const dt = new DataTransfer();
  dt.items.add(new File(["hi"], "dropped.txt"));
  return !target.dispatchEvent(new DragEvent(type, { dataTransfer: dt, bubbles: true, cancelable: true, clientX: x, clientY: y }));
};
</script></body>`, "")
	waitFor(t, w, "window.drag")
	expectDrop := func(want bool, what string) *mygo.FileDropEvent {
		t.Helper()
		select {
		case e := <-drops:
			if !want {
				t.Errorf("%s: unexpected OnFileDrop %+v", what, e)
			}
			return e
		case <-time.After(time.Second):
			if want {
				t.Fatalf("%s: OnFileDrop was not called", what)
			}
		}
		return nil
	}
	eval := func(js string) any {
		t.Helper()
		v, err := w.Page().Eval(js)
		if err != nil {
			t.Fatalf("%s: %v", js, err)
		}
		return v
	}
	if !setDroppedFiles(w, nil) {
		t.Skip("no drop hook on this platform")
	}

	// Dropped on the page's drop zone: the page handles it as usual, and Go
	// gets the path.
	path := filepath.Join(t.TempDir(), "dropped.txt")
	setDroppedFiles(w, []string{path})
	eval(`[drag(zone, "dragover", 20, 30), drag(zone, "drop", 20, 30)]`)
	if e := expectDrop(true, "drop zone"); len(e.Paths) != 1 || e.Paths[0] != path || e.X != 20 || e.Y != 30 {
		t.Errorf("drop zone: %+v", e)
	}
	waitFor(t, w, `log.includes("event:`+strings.ReplaceAll(path, `\`, `\\`)+`@20,30")`)
	if got := eval(`log.slice(0, 3).join(" ")`); got != "zone:dragover document:dragover:true zone:drop:dropped.txt" {
		t.Errorf("the page's drag and drop: %v", got)
	}

	// Elsewhere, the page's listeners see the events untouched, and the
	// bridge then accepts the files instead of letting the engine open them.
	setDroppedFiles(w, []string{path})
	if canceled := eval(`(log.length = 0, drag(document.body, "dragover", 50, 200))`); canceled != true {
		t.Error("an unhandled file drag was not accepted")
	}
	if got := eval(`log.join(" ")`); got != "document:dragover:false" {
		t.Errorf("the page saw %v", got)
	}
	if canceled := eval(`drag(document.body, "drop", 50, 200)`); canceled != true {
		t.Error("an unhandled file drop was not canceled")
	}
	expectDrop(true, "unhandled area")

	// A drag that starts in the page is left alone.
	setDroppedFiles(w, []string{path})
	eval(`item.dispatchEvent(new DragEvent("dragstart", { dataTransfer: new DataTransfer(), bubbles: true }))`)
	if canceled := eval(`drag(document.body, "dragover", 50, 200)`); canceled != false {
		t.Error("an in-page drag was accepted by the bridge")
	}
	if canceled := eval(`drag(document.body, "drop", 50, 200)`); canceled != false {
		t.Error("an in-page drop was canceled by the bridge")
	}
	expectDrop(false, "in-page drag")
	eval(`item.dispatchEvent(new DragEvent("dragend", { bubbles: true }))`)
	setDroppedFiles(w, nil)
}

// TestWindowExtras runs the taskbar and Dock features on the real backends.
func TestWindowExtras(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Extras", Width: 300, Height: 200, SkipTaskbar: true})
	for _, p := range []mygo.ProgressBar{{Value: 0.3}, {State: mygo.ProgressIndeterminate}, {State: mygo.ProgressError, Value: 0.8}, {}} {
		w.SetProgressBar(p)
	}
	if _, ok := dockTileImage(); ok {
		// The Dock draws the tile offscreen: the bar must show there.
		w.SetProgressBar(mygo.ProgressBar{Value: 0.5})
		data, _ := dockTileImage()
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("Dock tile: %v", err)
		}
		b := img.Bounds()
		blue := func(x, y int) bool {
			r, g, bl, _ := img.At(x, y).RGBA()
			return bl>>8 > 200 && r>>8 < 120 && g>>8 > 100 && g>>8 < 200
		}
		y := b.Max.Y - b.Dy()*13/128 // the middle of the bar
		if !blue(b.Min.X+b.Dx()/5, y) || blue(b.Max.X-b.Dx()/5, y) {
			t.Errorf("the Dock tile does not show half a progress bar")
		}
		w.SetProgressBar(mygo.ProgressBar{})
		if data, _ := dockTileImage(); data != nil {
			t.Error("the Dock tile still shows progress")
		}
	}
	w.FlashFrame(true)
	w.FlashFrame(false)
	w.SetSkipTaskbar(false)
	w.SetSkipTaskbar(true)
	w.SetVisibleOnAllWorkspaces(true)
	w.SetVisibleOnAllWorkspaces(false)
	var icon bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 64, 64))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{200, 40, 40, 255}}, image.Point{}, draw.Src)
	if err := png.Encode(&icon, img); err != nil {
		t.Fatal(err)
	}
	if err := w.SetIcon(icon.Bytes()); err != nil {
		t.Errorf("SetIcon: %v", err)
	}
	if err := w.SetIcon(nil); err != nil {
		t.Errorf("SetIcon(nil): %v", err)
	}
	if err := w.SetIcon([]byte("not a png")); err == nil && runtime.GOOS != "darwin" {
		t.Error("SetIcon accepted a bad image")
	}
	if w.Title() != "Extras" {
		t.Error("the window broke")
	}
}

func TestURLScheme(t *testing.T) {
	const scheme = "mygo-e2e"
	switch runtime.GOOS {
	case "darwin":
		// The test binary has no Info.plist to declare the scheme in.
		if err := mygo.App.RegisterURLScheme(scheme); err == nil || !strings.Contains(err.Error(), "urlSchemes") {
			t.Errorf("RegisterURLScheme of an undeclared scheme: %v", err)
		}
		if mygo.App.IsURLSchemeRegistered(scheme) {
			t.Error("an undeclared scheme is registered")
		}
		return
	}
	// GLib reads the XDG directories once: it only sees the handler where
	// the runner points them, as the container of the GUI tests does.
	// Elsewhere the handler is kept out of the user's configuration.
	glib := os.Getenv("XDG_DATA_HOME") != "" && os.Getenv("XDG_CONFIG_HOME") != ""
	if runtime.GOOS == "linux" && !glib {
		t.Setenv("XDG_DATA_HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	if mygo.App.IsURLSchemeRegistered(scheme) {
		t.Fatal("registered before RegisterURLScheme")
	}
	if err := mygo.App.RegisterURLScheme(scheme); err != nil {
		t.Fatal(err)
	}
	defer mygo.App.UnregisterURLScheme(scheme)
	if !mygo.App.IsURLSchemeRegistered(scheme) {
		t.Error("not registered after RegisterURLScheme")
	}
	if runtime.GOOS == "linux" {
		entries, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_DATA_HOME"), "applications", "*.url-handler.desktop"))
		mimeapps, _ := os.ReadFile(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "mimeapps.list"))
		if len(entries) != 1 || !strings.Contains(string(mimeapps), "x-scheme-handler/"+scheme+"="+filepath.Base(entries[0])) {
			t.Errorf("handler %q, mimeapps.list:\n%s", entries, mimeapps)
		} else if entry, _ := os.ReadFile(entries[0]); !strings.Contains(string(entry), "MimeType=x-scheme-handler/"+scheme+";") || !strings.Contains(string(entry), `" %u`) {
			t.Errorf("handler entry:\n%s", entry)
		} else if _, ok := defaultURLHandler(scheme); ok && glib {
			// GLib, like xdg-open, opens the scheme with the handler.
			eventually(t, "GLib to find the handler", func() bool {
				id, _ := defaultURLHandler(scheme)
				return id == filepath.Base(entries[0])
			})
		}
	}
	if err := mygo.App.UnregisterURLScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if mygo.App.IsURLSchemeRegistered(scheme) {
		t.Error("still registered after UnregisterURLScheme")
	}
}

func TestOpenAtLogin(t *testing.T) {
	if mygo.App.WasOpenedAtLogin() {
		t.Error("the tests were not opened at login")
	}
	if runtime.GOOS == "darwin" {
		// Only app bundles can be login items: nothing is registered.
		if err := mygo.App.SetOpenAtLogin(true); err == nil || !strings.Contains(err.Error(), "bundle") {
			t.Errorf("SetOpenAtLogin without a bundle: %v", err)
		}
		if mygo.App.OpenAtLogin() {
			t.Error("the test binary opens at login")
		}
		return
	}
	if runtime.GOOS == "linux" {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	if mygo.App.OpenAtLogin() {
		t.Fatal("opens at login before SetOpenAtLogin")
	}
	if err := mygo.App.SetOpenAtLogin(true); err != nil {
		t.Fatal(err)
	}
	defer mygo.App.SetOpenAtLogin(false)
	if !mygo.App.OpenAtLogin() {
		t.Error("does not open at login after SetOpenAtLogin")
	}
	if runtime.GOOS == "linux" {
		entries, _ := filepath.Glob(filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "autostart", "*.desktop"))
		if len(entries) != 1 {
			t.Fatalf("autostart entries: %q", entries)
		}
		entry, _ := os.ReadFile(entries[0])
		if !strings.Contains(string(entry), `" --mygo-opened-at-login`) {
			t.Errorf("autostart entry:\n%s", entry)
		}
		// Turned off in the desktop's settings.
		if err := os.WriteFile(entries[0], append(entry, "Hidden=true\n"...), 0o644); err != nil {
			t.Fatal(err)
		}
		if mygo.App.OpenAtLogin() {
			t.Error("a hidden autostart entry opens the app")
		}
	}
	if err := mygo.App.SetOpenAtLogin(false); err != nil {
		t.Fatal(err)
	}
	if mygo.App.OpenAtLogin() {
		t.Error("still opens at login after SetOpenAtLogin(false)")
	}
}

func TestPower(t *testing.T) {
	off := mygo.Power.OnSuspend(func() {})
	defer off()
	const reason = "MyGo e2e keeps the display on"
	release := mygo.Power.KeepAwake(reason, true)
	assertions := func() string {
		out, _ := exec.Command("pmset", "-g", "assertions").Output()
		return string(out)
	}
	if runtime.GOOS == "darwin" && !strings.Contains(assertions(), reason) {
		t.Error("KeepAwake made no power assertion")
	}
	release()
	if runtime.GOOS == "darwin" {
		eventually(t, "the assertion to go", func() bool { return !strings.Contains(assertions(), reason) })
	}
	t.Logf("on battery: %v, idle: %v", mygo.Power.IsOnBattery(), mygo.Power.IdleTime())
	if idle := mygo.Power.IdleTime(); idle < 0 || runtime.GOOS == "darwin" && idle == 0 {
		t.Errorf("idle time = %v", idle)
	}
}

func TestDockMenu(t *testing.T) {
	clicked := make(chan struct{}, 1)
	mygo.App.Dock.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		{Label: "New Window", Click: func(*mygo.MenuItem, *mygo.Window) { clicked <- struct{}{} }},
		mygo.Separator(),
		{Label: "Settings"},
	}))
	defer mygo.App.Dock.SetMenu(nil)
	titles, ok := dockMenu(0)
	if !ok {
		t.Skip("no Dock on this platform")
	}
	if len(titles) != 3 || titles[0] != "New Window" || titles[2] != "Settings" {
		t.Errorf("Dock menu = %q", titles)
	}
	select {
	case <-clicked:
	case <-time.After(3 * time.Second):
		t.Error("clicking the Dock menu item did not reach its handler")
	}
	mygo.App.Dock.SetMenu(nil)
	if titles, _ := dockMenu(-1); titles != nil {
		t.Errorf("Dock menu after SetMenu(nil) = %q", titles)
	}
}

func TestPrintToPDF(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "PDF", Width: 400, Height: 300})
	w.Page().LoadHTML(`<style>p + p { page-break-before: always }</style><p>one</p><p>two</p><p>three</p>`, "")
	waitFor(t, w, `document.readyState === "complete" && document.querySelectorAll("p").length === 3`)
	pages := regexp.MustCompile(`/Type\s*/Page[^s]`)
	mediaBox := regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)`)
	for _, landscape := range []bool{false, true} {
		pdf, err := w.Page().PrintToPDF(mygo.PDFOptions{PageSize: mygo.PageA4, Landscape: landscape})
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
			t.Fatalf("not a PDF: %.20q", pdf)
		}
		if n := len(pages.FindAll(pdf, -1)); n != 3 {
			t.Errorf("landscape=%v: %d pages, want 3", landscape, n)
		}
		m := mediaBox.FindSubmatch(pdf)
		if m == nil {
			t.Fatal("no page size in the PDF")
		}
		width, _ := strconv.ParseFloat(string(m[1]), 64)
		height, _ := strconv.ParseFloat(string(m[2]), 64)
		want := [2]float64{595, 842} // A4 in points
		if landscape {
			want = [2]float64{842, 595}
		}
		if math.Abs(width-want[0]) > 2 || math.Abs(height-want[1]) > 2 {
			t.Errorf("landscape=%v: page size %vx%v, want %v", landscape, width, height, want)
		}
	}
}

func TestPermissions(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Permissions", Width: 300, Height: 200})
	// A secure context: notifications are not for opaque origins.
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	origin := "app://localhost"
	if runtime.GOOS == "windows" {
		origin = "http://app.localhost" // how WebView2 serves custom schemes
	}
	waitFor(t, w, `location.origin === "`+origin+`" && document.readyState === "complete"`)
	// The app's own pages are secure contexts with a real origin, which
	// secure-context APIs (camera, Web Crypto) and storage need.
	if got := mustEval(t, w, `[window.isSecureContext, typeof crypto.subtle].join(" ")`); got != "true object" {
		t.Errorf("%s: %v", origin, got)
	}
	if ok, _ := mygo.EvalAs[bool](w.Page(), `typeof Notification !== "undefined" && !!Notification.requestPermission`); !ok {
		t.Skip("the engine has no Notification API")
	}
	asked := make(chan mygo.PermissionRequest, 4)
	w.Page().SetPermissionHandler(func(req mygo.PermissionRequest) bool {
		asked <- req
		return true
	})
	got, err := mygo.EvalAs[string](w.Page(), `Notification.requestPermission()`)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case req := <-asked:
		if len(req.Permissions) != 1 || req.Permissions[0] != mygo.PermissionNotifications {
			t.Errorf("request = %+v", req)
		}
		if got != "granted" {
			t.Errorf("the page got %q", got)
		}
	case <-time.After(3 * time.Second):
		t.Skipf("the engine decided alone (%q)", got)
	}
}

func mustEval(t *testing.T, w *mygo.Window, js string) any {
	t.Helper()
	v, err := w.Page().Eval(js)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDownloads(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Downloads", Width: 300, Height: 200})
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, `window.run`)
	dir := t.TempDir()
	started := make(chan *mygo.DownloadEvent, 2)
	done := make(chan *mygo.Download, 2)
	w.Page().OnWillDownload(func(e *mygo.DownloadEvent) {
		if e.Path == "" || filepath.Base(e.Path) != e.SuggestedName {
			t.Errorf("default path %q for %q", e.Path, e.SuggestedName)
		}
		e.Path = filepath.Join(dir, e.SuggestedName)
		started <- e
	})
	w.Page().OnDownloadDone(func(d *mygo.Download) { done <- d })
	check := func(what, name, content string) {
		t.Helper()
		select {
		case e := <-started:
			if e.SuggestedName != name {
				t.Errorf("%s: suggested name %q, want %q", what, e.SuggestedName, name)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: OnWillDownload was not called", what)
		}
		select {
		case d := <-done:
			if d.Err != nil {
				t.Fatalf("%s: %v", what, d.Err)
			}
			if b, err := os.ReadFile(d.Path); err != nil || string(b) != content {
				t.Errorf("%s: saved %q, %v", what, b, err)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("%s: OnDownloadDone was not called", what)
		}
	}
	// A link with the download attribute.
	mustEval(t, w, `(() => { const a = document.createElement("a"); a.href = "data:text/csv,a%2Cb"; a.download = "report.csv"; document.body.append(a); a.click(); return true })()`)
	check("download link", "report.csv", "a,b")
	// A response sent as an attachment.
	mustEval(t, w, `(location.href = "report.bin", true)`)
	check("attachment", "report.bin", "binary report")
}

func TestClearBrowsingData(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Data", Width: 300, Height: 200})
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, `window.run`)
	mustEval(t, w, `(localStorage.setItem("token", "secret"), window.oldPage = true)`)
	if err := mygo.App.ClearBrowsingData(); err != nil {
		t.Fatal(err)
	}
	w.Page().Reload()
	waitFor(t, w, `window.run && !window.oldPage`)
	if got := mustEval(t, w, `localStorage.getItem("token")`); got != nil {
		t.Errorf("after ClearBrowsingData the token is %v", got)
	}
}

func TestFindInPage(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Title: "Find", Width: 400, Height: 300})
	w.Page().LoadHTML(`<p>MyGo is a framework. mygo apps are small.</p><p style="display:none">mygo hidden</p><p>Say mygo</p>`, "")
	waitFor(t, w, `document.readyState === "complete" && document.querySelectorAll("p").length === 3`)
	find := func(text string, opts mygo.FindOptions) mygo.FindResult {
		t.Helper()
		res, err := w.Page().FindInPage(text, opts)
		if err != nil {
			t.Fatal(err)
		}
		return res
	}
	if res := find("mygo", mygo.FindOptions{}); res != (mygo.FindResult{Matches: 3, Active: 1}) {
		t.Errorf("find = %+v, want 3 visible matches", res)
	}
	if res := find("mygo", mygo.FindOptions{FindNext: true}); res.Active != 2 {
		t.Errorf("next = %+v", res)
	}
	if res := find("mygo", mygo.FindOptions{FindNext: true, Backward: true}); res.Active != 1 {
		t.Errorf("previous = %+v", res)
	}
	if res := find("mygo", mygo.FindOptions{FindNext: true, Backward: true}); res.Active != 3 {
		t.Errorf("previous wraps around: %+v", res)
	}
	if res := find("MyGo", mygo.FindOptions{MatchCase: true}); res.Matches != 1 {
		t.Errorf("match case = %+v", res)
	}
	if res := find("nothing", mygo.FindOptions{}); res != (mygo.FindResult{}) {
		t.Errorf("no match = %+v", res)
	}
	w.Page().StopFindInPage()
}

func TestGlobalShortcut(t *testing.T) {
	pressed := make(chan struct{}, 1)
	if err := mygo.GlobalShortcut.Register("Ctrl+Shift+K", func() { pressed <- struct{}{} }); err != nil {
		t.Fatal(err)
	}
	defer mygo.GlobalShortcut.Unregister("Ctrl+Shift+K")
	if !pressCtrlShiftK() {
		t.Skip("no keyboard automation on this platform")
	}
	select {
	case <-pressed:
	case <-time.After(3 * time.Second):
		t.Fatal("the global shortcut was not reported")
	}
}

// TestGlobalShortcutPortal has the desktop bind global shortcuts through
// the XDG desktop portal, as on Wayland, where apps grab no keys.
func TestGlobalShortcutPortal(t *testing.T) {
	restore, ok := usePortalShortcuts()
	if !ok {
		t.Skip("no GlobalShortcuts portal")
	}
	defer restore()
	pressed := make(chan string, 8)
	register := func(acc string) {
		t.Helper()
		if err := mygo.GlobalShortcut.Register(acc, func() { pressed <- acc }); err != nil {
			t.Fatal(err)
		}
	}
	// The desktop binds shortcuts after Register returns, in a new session
	// whenever they change.
	expect := func(acc string, keys ...string) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); time.Now().Before(deadline); {
			if !pressKeys(keys...) {
				t.Skip("no keyboard automation")
			}
			select {
			case got := <-pressed:
				if got != acc {
					t.Fatalf("pressing %v reported %s", keys, got)
				}
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
		t.Fatalf("%s was not reported", acc)
	}
	register("Ctrl+Shift+K")
	defer mygo.GlobalShortcut.UnregisterAll()
	expect("Ctrl+Shift+K", "Control_L", "Shift_L", "k")
	register("Alt+Shift+J")
	expect("Alt+Shift+J", "Alt_L", "Shift_L", "j")
	expect("Ctrl+Shift+K", "Control_L", "Shift_L", "k")

	mygo.GlobalShortcut.Unregister("Ctrl+Shift+K")
	expect("Alt+Shift+J", "Alt_L", "Shift_L", "j")
	pressKeys("Control_L", "Shift_L", "k")
	select {
	case got := <-pressed:
		t.Fatalf("%s was reported after it was unregistered", got)
	case <-time.After(time.Second):
	}
}

func TestCloseEvents(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	var prevent atomic.Bool
	prevent.Store(true)
	closed := make(chan struct{})
	w.OnClose(func(e *mygo.CloseEvent) {
		if prevent.Load() {
			e.PreventDefault()
		}
	})
	w.OnClosed(func() { close(closed) })
	w.Close()
	if w.IsDestroyed() {
		t.Fatal("close was not prevented")
	}
	prevent.Store(false)
	w.Close()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("OnClosed not called")
	}
}

func TestCapturePage(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 320, Height: 240})
	w.Page().LoadHTML(`<body style="margin:0;background:rgb(255,0,0)"></body>`, "")
	waitFor(t, w, "document.readyState === 'complete'")
	time.Sleep(200 * time.Millisecond)
	png, err := w.CapturePage()
	if err != nil {
		t.Fatal(err)
	}
	if len(png) < 100 || !strings.HasPrefix(string(png), "\x89PNG") {
		t.Fatalf("not a PNG (%d bytes)", len(png))
	}
	if out := os.Getenv("MYGO_E2E_SNAPSHOT"); out != "" {
		_ = os.WriteFile(out, png, 0o644)
	}
}

// callEnd is how a call of pageCalls ended.
type callEnd struct {
	name string
	err  error
}

// pageCalls makes the calls that wait for a page's answer, each on a
// goroutine of its own, and tells how they end.
func pageCalls(w *mygo.Window) <-chan callEnd {
	ended := make(chan callEnd, 3)
	call := func(name string, fn func() error) {
		go func() { ended <- callEnd{name, fn()} }()
	}
	call("Eval", func() error { _, err := w.Page().Eval("1"); return err })
	call("CapturePage", func() error { _, err := w.CapturePage(); return err })
	call("PrintToPDF", func() error { _, err := w.Page().PrintToPDF(mygo.PDFOptions{}); return err })
	return ended
}

// waitCalls waits for the calls of pageCalls to end, and returns their
// errors by name. A slow runner takes seconds to create a webview, and a
// window may ask for its webview again.
func waitCalls(t *testing.T, ended <-chan callEnd, what string) map[string]error {
	t.Helper()
	got := map[string]error{}
	for range 3 {
		select {
		case e := <-ended:
			got[e.name] = e.err
		case <-time.After(time.Minute):
			t.Fatalf("%s: only these calls ended: %v", what, got)
		}
	}
	return got
}

// TestCallsEndWithTheWindow: calls on a new window's page, which on
// Windows wait for WebView2 to create its webview, end when the window is
// destroyed.
func TestCallsEndWithTheWindow(t *testing.T) {
	w := mygo.NewWindow(mygo.WindowOptions{Hidden: true})
	ended := pageCalls(w)
	time.Sleep(20 * time.Millisecond)
	mygo.RunOnMain(func() {}) // the calls reached the window
	w.Destroy()
	waitCalls(t, ended, "after Destroy")
}

// TestWebViewFails: WebView2 may fail to create a window's webview, as it
// does under load. The window asks again, and once it gives up, the calls
// waiting for the webview fail with the reason, as do later ones.
func TestWebViewFails(t *testing.T) {
	if !failWebViews(1) {
		t.Skip("only WebView2 creates webviews after their windows")
	}
	t.Cleanup(func() { failWebViews(0) })
	const reason = "creating the WebView2 controller"
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	got := waitCalls(t, pageCalls(w), "after a failure")
	for name, err := range got {
		if err != nil && strings.Contains(err.Error(), reason) {
			t.Errorf("after a failure, %s: %v", name, err)
		}
	}
	if got["Eval"] != nil {
		t.Errorf("after a failure, Eval: %v", got["Eval"])
	}

	failWebViews(1 << 10)
	w = newWindow(t, mygo.WindowOptions{Hidden: true})
	for _, when := range []string{"waiting", "later"} {
		for name, err := range waitCalls(t, pageCalls(w), when) {
			if err == nil || !strings.Contains(err.Error(), reason) {
				t.Errorf("%s, %s: %v", when, name, err)
			}
		}
	}
}

func TestMenuAndClipboard(t *testing.T) {
	clicked := make(chan string, 1)
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "Test", Submenu: []*mygo.MenuItem{
			{ID: "ping", Label: "Ping", Accelerator: "CmdOrCtrl+Shift+P", Click: func(item *mygo.MenuItem, _ *mygo.Window) { clicked <- item.ID }},
			{ID: "check", Label: "Check", Type: mygo.MenuItemCheckbox},
		}},
		{Role: mygo.RoleEditMenu},
	})
	mygo.App.SetMenu(menu)
	defer mygo.App.SetMenu(nil)
	menu.ItemByID("check").SetChecked(true)
	if !menu.ItemByID("check").IsChecked() {
		t.Error("SetChecked")
	}
	mygo.Clipboard.WriteText("mygo clipboard ✓")
	if got := mygo.Clipboard.ReadText(); got != "mygo clipboard ✓" {
		t.Errorf("clipboard = %q", got)
	}
	if ds := mygo.Screen.Displays(); len(ds) == 0 || ds[0].Bounds.Width == 0 {
		t.Errorf("displays = %+v", ds)
	}
}

func TestWindowOpenHandler(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Hidden: true})
	opened := make(chan string, 1)
	w.Page().SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		opened <- req.URL
		return nil
	})
	w.Page().LoadHTML(`<a id="l" href="https://example.com/x" target="_blank">x</a>`, "https://example.com/")
	waitFor(t, w, "document.getElementById('l')")
	if _, err := w.Page().Eval("window.open('https://example.com/popup')"); err != nil {
		t.Fatal(err)
	}
	select {
	case u := <-opened:
		if u != "https://example.com/popup" {
			t.Errorf("opened %q", u)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("window open handler not called")
	}
}

func TestMenuActivation(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	w.Page().LoadHTML("<p>menus</p>", "")
	clicks := make(chan string, 4)
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "Test", Submenu: []*mygo.MenuItem{
			{ID: "ping", Label: "Ping", Accelerator: "CmdOrCtrl+Shift+Y", Click: func(item *mygo.MenuItem, win *mygo.Window) {
				clicks <- item.ID
			}},
			{ID: "check", Label: "Check", Type: mygo.MenuItemCheckbox},
			{ID: "checked", Label: "Checked", Type: mygo.MenuItemCheckbox, Checked: true, Click: func(item *mygo.MenuItem, win *mygo.Window) {
				clicks <- item.ID
			}},
		}},
	})
	mygo.App.SetMenu(menu)
	defer mygo.App.SetMenu(nil)
	w.Focus()
	time.Sleep(100 * time.Millisecond)

	// Building the menu and changing state from code are not clicks.
	menu.ItemByID("checked").SetChecked(false)
	menu.ItemByID("checked").SetChecked(true)
	select {
	case id := <-clicks:
		t.Fatalf("%q clicked without user action", id)
	case <-time.After(300 * time.Millisecond):
	}
	if !menu.ItemByID("checked").IsChecked() {
		t.Fatal("checked item lost its state")
	}

	if err := activateMenu(w, "Test", "Ping"); err != nil {
		t.Fatal(err)
	}
	expectClick(t, clicks, "ping")
	if err := activateMenu(w, "Test", "Check"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !menu.ItemByID("check").IsChecked() && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !menu.ItemByID("check").IsChecked() {
		t.Error("checkbox not toggled by activation")
	}
	if handled, ok := pressShortcut("y", true); ok {
		if !handled {
			t.Error("key equivalent not handled by the menu")
		}
		expectClick(t, clicks, "ping")
	}
}

// A submenu may have focus instead of its window. Its callback must still
// receive the window whose menu was chosen, including for shared menus.
func TestMenuActivationWindow(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skip("menu bars belong to the application")
	}
	for _, ownMenu := range []bool{false, true} {
		name := "application"
		if ownMenu {
			name = "window"
		}
		t.Run(name, func(t *testing.T) {
			wins := []*mygo.Window{
				newWindow(t, mygo.WindowOptions{Hidden: true, Width: 400, Height: 300}),
				newWindow(t, mygo.WindowOptions{Hidden: true, Width: 400, Height: 300}),
			}
			if mygo.FocusedWindow() != nil {
				t.Fatal("a window has focus before activating the menu")
			}
			clicks := make(chan *mygo.Window, 1)
			menu := mygo.NewMenu([]*mygo.MenuItem{{Label: "Demo", Submenu: []*mygo.MenuItem{
				{Label: "Sizes", Submenu: []*mygo.MenuItem{
					{ID: "small", Label: "Small", Type: mygo.MenuItemRadio, Checked: true},
					{ID: "large", Label: "Large", Type: mygo.MenuItemRadio, Click: func(_ *mygo.MenuItem, win *mygo.Window) {
						if win != nil {
							win.SetSize(1000, 720)
						}
						clicks <- win
					}},
				}},
			}}})
			if ownMenu {
				for _, w := range wins {
					w.SetMenu(menu)
				}
			} else {
				mygo.App.SetMenu(menu)
				defer mygo.App.SetMenu(nil)
			}
			for _, w := range wins {
				menu.ItemByID("small").SetChecked(true)
				if err := activateMenu(w, "Demo", "Sizes", "Large"); err != nil {
					t.Fatal(err)
				}
				select {
				case got := <-clicks:
					if got != w {
						t.Fatalf("click window = %v, want window %d", got, w.ID())
					}
				case <-time.After(3 * time.Second):
					t.Fatal("menu click was not delivered")
				}
				if width, height := w.Size(); width != 1000 || height != 720 {
					t.Errorf("size after click = %dx%d, want 1000x720", width, height)
				}
				if !menu.ItemByID("large").IsChecked() || menu.ItemByID("small").IsChecked() {
					t.Error("radio group was not updated")
				}
			}
		})
	}
}

func TestAutoHideMenuBar(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, AutoHideMenuBar: true})
	w.Page().LoadHTML("<p>menu bar</p>", "")
	clicks := make(chan string, 4)
	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Test", Submenu: []*mygo.MenuItem{
			{ID: "ping", Label: "Ping", Accelerator: "CmdOrCtrl+Shift+J", Click: func(item *mygo.MenuItem, win *mygo.Window) {
				clicks <- item.ID
			}},
		}},
	})
	mygo.App.SetMenu(menu)
	defer mygo.App.SetMenu(nil)
	w.Focus()
	time.Sleep(200 * time.Millisecond)

	shown, supported := menuBarShown(w)
	if !supported {
		t.Skip("the menu bar belongs to the application")
	}
	if shown {
		t.Fatal("the menu bar shows before Alt")
	}
	if handled, _ := activateAccelerator(w, "CmdOrCtrl+Shift+J"); !handled {
		t.Error("the hidden menu bar lost its shortcut")
	} else {
		expectClick(t, clicks, "ping")
	}
	for _, key := range []string{"Alt_L", "F10"} {
		during, after, ok := enterMenuBar(w, key)
		if !ok {
			t.Logf("%s: the keyboard cannot reach the window", key)
			continue
		}
		if !during {
			t.Errorf("%s: the menu bar did not show", key)
		}
		if after {
			t.Errorf("%s: the menu bar still shows once its menus closed", key)
		}
	}

	w.SetAutoHideMenuBar(false)
	if shown, _ := menuBarShown(w); !shown {
		t.Error("SetAutoHideMenuBar(false) left the menu bar hidden")
	}
	w.SetAutoHideMenuBar(true)
	if shown, _ := menuBarShown(w); shown {
		t.Error("SetAutoHideMenuBar(true) left the menu bar shown")
	}
}

// A hidden title bar leaves the window controls over the page, which hears
// the room they take in CSS variables, and they work as the system's do.
func TestHiddenTitleBar(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{X: 80, Y: 80, Width: 480, Height: 320,
		TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: 40})
	closing := make(chan struct{}, 1)
	w.OnClose(func(e *mygo.CloseEvent) {
		e.PreventDefault()
		closing <- struct{}{}
	})
	w.Page().LoadHTML("<p>hidden title bar</p>", "")
	waitFor(t, w, "document.querySelector('p')")
	room := func(win *mygo.Window) (r [3]string) {
		v, _ := mygo.EvalAs[[]string](win.Page(), `(() => {
			const s = getComputedStyle(document.documentElement);
			return ["height", "inset-left", "inset-right"].map((k) => s.getPropertyValue("--mygo-titlebar-" + k).trim());
		})()`)
		copy(r[:], v)
		return r
	}
	shown := room(w)
	switch runtime.GOOS {
	case "windows":
		if shown != [3]string{"40px", "0px", "138px"} {
			t.Errorf("room = %q, want the title bar's height and three buttons 46 wide at the right", shown)
		}
	case "linux":
		if shown[0] != "40px" {
			t.Errorf("room = %q, want the title bar's height", shown)
		}
		if shown[1] == "0px" && shown[2] == "0px" {
			t.Log("the desktop's button layout names no buttons")
		}
	case "darwin":
		if shown[0] == "0px" || shown[1] == "0px" || shown[2] != "0px" {
			t.Errorf("room = %q, want the traffic lights at the left of the title bar", shown)
		}
	}

	// The page of a window with its title bar hears nothing.
	plain := newWindow(t, mygo.WindowOptions{Width: 300, Height: 200})
	plain.Page().LoadHTML("<p>title bar</p>", "")
	waitFor(t, plain, "document.querySelector('p')")
	if r := room(plain); r != ([3]string{}) {
		t.Errorf("a window with its title bar: room = %q", r)
	}

	// Without any non-client area at its top, a window that is not maximized
	// gets no snap layouts over its maximize button on Windows 11, and
	// Windows 10 draws its title bar over a window that keeps any.
	if px, want, ok := topNonClient(w); ok && px != want {
		t.Errorf("non-client area at the top = %d px, want %d", px, want)
	}

	if names, ok := titleButtons(w); ok {
		if runtime.GOOS == "windows" && !slices.Equal(names, []string{"minimize", "maximize", "close"}) {
			t.Errorf("buttons = %q, want minimize, maximize and close", names)
		}
		// Maximizing needs a window manager, which Xvfb lacks.
		if runtime.GOOS != "linux" && pressTitleButton(w, "maximize") {
			eventually(t, "the maximize button to maximize the window", w.IsMaximized)
			pressTitleButton(w, "maximize")
			eventually(t, "the restore button to restore the window", func() bool { return !w.IsMaximized() })
		}
		if slices.Contains(names, "close") && pressTitleButton(w, "close") {
			select {
			case <-closing:
			case <-time.After(3 * time.Second):
				t.Error("the close button did not ask to close the window")
			}
		}
	}

	// Full screen hides the controls, and the page gets their room back.
	if runtime.GOOS != "windows" {
		return // no window manager under Xvfb; the macOS animation is slow
	}
	w.SetFullScreen(true)
	eventually(t, "no room for the controls in full screen", func() bool { return room(w) == [3]string{"0px", "0px", "0px"} })
	w.SetFullScreen(false)
	eventually(t, "the room of the controls back", func() bool { return room(w) == shown })
}

func expectClick(t *testing.T, clicks chan string, want string) {
	t.Helper()
	select {
	case got := <-clicks:
		if got != want {
			t.Errorf("clicked %q, want %q", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatalf("menu item %q was not clicked", want)
	}
}

func TestJavaScriptAlert(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	w.Page().LoadHTML("<p>alert</p>", "")
	waitFor(t, w, "document.querySelector('p')")
	if _, ok := endSheet(w); !ok {
		t.Skip("dialog automation not available on this platform")
	}
	// alert() blocks the page until the sheet is dismissed, and WebKit
	// often answers the Eval that schedules it only after the timer ran:
	// dismiss the sheet without waiting for the Eval.
	evaluated := make(chan error, 1)
	go func() {
		_, err := w.Page().Eval("setTimeout(() => { alert('hello'); window.alertDone = true }, 0)")
		evaluated <- err
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if ok, _ := endSheet(w); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("alert sheet never appeared")
		}
		time.Sleep(20 * time.Millisecond)
	}
	select {
	case err := <-evaluated:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Eval did not return after the alert was dismissed")
	}
	waitFor(t, w, "window.alertDone")
}

func TestWindowOpenAllowed(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	created := make(chan *mygo.Window, 1)
	off := mygo.App.OnWindowCreated(func(c *mygo.Window) { created <- c })
	defer off()
	w.Page().SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
		return &mygo.WindowOptions{Width: 320, Height: 240}
	})
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, w, "window.run")
	if _, err := w.Page().Eval("void window.open('app://localhost/?child=1')"); err != nil {
		t.Fatal(err)
	}
	var child *mygo.Window
	select {
	case child = <-created:
	case <-time.After(5 * time.Second):
		t.Fatal("child window not created")
	}
	defer child.Destroy()
	waitFor(t, child, "window.run && location.search === '?child=1'")
	// The child has its own bridge: calls identify the child window.
	same, err := mygo.EvalAs[bool](child.Page(), "mygo.call('Greeter.WindowID').then(id => id === mygo.windowId)")
	if err != nil || !same {
		t.Errorf("call from child: %v, %v", same, err)
	}
	if id, _ := mygo.EvalAs[int](child.Page(), "mygo.windowId"); id != child.ID() {
		t.Errorf("child bridge reports window %d, want %d", id, child.ID())
	}
	// On Linux new windows are independent pages (see Window.SetWindowOpenHandler).
	if opener, _ := mygo.EvalAs[bool](child.Page(), "window.opener !== null"); !opener && runtime.GOOS == "darwin" {
		t.Error("window.opener is not set")
	}
}

// TestClick is a regression test: clicking the page used to crash on macOS.
func TestClick(t *testing.T) {
	w := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300})
	w.Page().LoadHTML(`<body style="margin:0"><button id="b" style="width:200px;height:100px" onclick="window.clicked=(window.clicked||0)+1">x</button></body>`, "")
	waitFor(t, w, "document.getElementById('b')")
	if !click(w, 50, 50) {
		t.Skip("click automation not available on this platform")
	}
	click(w, 60, 40)
	waitFor(t, w, "window.clicked === 2")

	// Clicking a drag region of a frameless window starts a native drag.
	f := newWindow(t, mygo.WindowOptions{Width: 400, Height: 300, Frameless: true})
	f.Page().LoadHTML(`<body style="margin:0"><div id="bar" style="--app-region:drag;height:40px" onmousedown="window.pressed=true"></div></body>`, "")
	waitFor(t, f, "document.getElementById('bar')")
	click(f, 100, 20)
	waitFor(t, f, "window.pressed === true")
	if !f.IsVisible() {
		t.Error("frameless window disappeared after a drag click")
	}
}

// deviceScale returns the device pixels per DIP of the display w shows on.
func deviceScale(w *mygo.Window) float64 {
	b := w.Bounds()
	return mygo.Screen.DisplayNearestPoint(mygo.Point{X: b.X + b.Width/2, Y: b.Y + b.Height/2}).ScaleFactor
}

// TestContentWindowTextSelection drags across independently laid-out
// paragraphs and copies their selection through the native Edit menu.
func TestContentWindowTextSelection(t *testing.T) {
	var frames atomic.Int32
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Gap(20).Selectable().Children(func() {
			ui.Text(c, "First paragraph.").Height(30)
			ui.Text(c, "Second paragraph.").Height(30)
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Text selection", Width: 400, Height: 200, Content: ui.View(view)})
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleEditMenu}}))
	defer mygo.App.SetMenu(nil)
	clipboard := mygo.Clipboard.ReadText()
	defer mygo.Clipboard.WriteText(clipboard)
	w.Focus()
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	var copied string
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("last copied text: %q", copied)
		}
	})
	for _, reverse := range []bool{false, true} {
		points := [][2]float64{{20, 28}, {380, 78}}
		if reverse {
			slices.Reverse(points)
		}
		before := frames.Load()
		if !drag(w, points) {
			t.Skip("drag automation not available on this platform")
		}
		eventually(t, "a frame after the drag", func() bool { return frames.Load() > before })
		mygo.Clipboard.WriteText("before copy")
		if err := activateMenu(w, "Edit", "Copy"); err != nil {
			t.Fatal(err)
		}
		eventually(t, "both paragraphs copied", func() bool {
			copied = mygo.Clipboard.ReadText()
			return copied == "First paragraph.\nSecond paragraph."
		})
	}
}

// TestContentWindowInputMethod checks that input methods see the text
// around the caret of native UI and replace what was typed, as macOS's
// press and hold does with the letter it accents.
func TestContentWindowInputMethod(t *testing.T) {
	var frames atomic.Int32
	name := "cafe"
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Children(func() { ui.TextInput(c, &name) })
	}
	text := func() (s string) {
		mygo.RunOnMain(func() { s = name })
		return s
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Input method", Width: 400, Height: 200, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if _, _, ok := inputClient(w); !ok {
		t.Skip("input method automation not available on this platform")
	}
	if !click(w, 300, 36) { // after the text
		t.Skip("click automation not available on this platform")
	}
	eventually(t, "the caret after the text", func() bool {
		sel, doc, _ := inputClient(w)
		return sel == [2]int{4, 0} && doc == "cafe"
	})
	composeOver(w, "e", 1, false, 3, 1)
	// The text input methods get on macOS holds what they compose, the text
	// they get through GTK and IMM32 does not.
	doc, sel := "cafe", [2]int{4, 0}
	if runtime.GOOS != "darwin" {
		doc, sel = "caf", [2]int{3, 0}
	}
	eventually(t, "the composition over the e", func() bool {
		s, d, _ := inputClient(w)
		return text() == "caf" && d == doc && s == sel
	})
	composeOver(w, "é", 1, true, -1, 0)
	eventually(t, "the accented letter", func() bool { return text() == "café" })
}

// TestContentWindowTextAreaInputMethod is TestContentWindowInputMethod in
// the second paragraph of a text area, which lays its paragraphs out
// apart.
func TestContentWindowTextAreaInputMethod(t *testing.T) {
	var frames atomic.Int32
	notes := "first\ncafe"
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Children(func() { ui.TextArea(c, &notes).Height(120) })
	}
	text := func() (s string) {
		mygo.RunOnMain(func() { s = notes })
		return s
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Input method", Width: 400, Height: 200, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if _, _, ok := inputClient(w); !ok {
		t.Skip("input method automation not available on this platform")
	}
	if !click(w, 300, 56) { // after the text of the second line
		t.Skip("click automation not available on this platform")
	}
	eventually(t, "the caret after the text", func() bool {
		sel, doc, _ := inputClient(w)
		return sel == [2]int{10, 0} && doc == "first\ncafe"
	})
	composeOver(w, "e", 1, false, 9, 1)
	doc, sel := "first\ncafe", [2]int{10, 0}
	if runtime.GOOS != "darwin" {
		doc, sel = "first\ncaf", [2]int{9, 0}
	}
	eventually(t, "the composition over the e", func() bool {
		s, d, _ := inputClient(w)
		return text() == "first\ncaf" && d == doc && s == sel
	})
	composeOver(w, "é", 1, true, -1, 0)
	eventually(t, "the accented letter", func() bool { return text() == "first\ncafé" })
}

// TestContentWindowFileDrop drops files on native UI: on the element that
// takes them, and elsewhere for OnFileDrop.
func TestContentWindowFileDrop(t *testing.T) {
	var frames atomic.Int32
	var zone []string
	view := func(c *ui.Context) {
		frames.Add(1)
		if files := ui.Box(c).Size(200, 100).Background(ui.RGB(200, 200, 200)).DroppedFiles(); files != nil {
			zone = files
		}
	}
	got := func() (s []string) {
		mygo.RunOnMain(func() { s = zone })
		return s
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Drop", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	over, dropped, ok := dropFiles(w, 50, 50, []string{"/tmp/a.txt", "/tmp/b.txt"})
	if !ok {
		t.Skip("drag and drop automation not available on this platform")
	}
	if !over || !dropped {
		t.Fatalf("the drop zone took files: over %v, dropped %v", over, dropped)
	}
	eventually(t, "the files in the zone", func() bool { return slices.Equal(got(), []string{"/tmp/a.txt", "/tmp/b.txt"}) })
	if over, _, _ := dropFiles(w, 300, 250, []string{"/tmp/c.txt"}); over {
		t.Error("files were taken outside of the zone without OnFileDrop listeners")
	}
	var events atomic.Pointer[mygo.FileDropEvent]
	w.OnFileDrop(func(e *mygo.FileDropEvent) { events.Store(e) })
	if over, dropped, _ := dropFiles(w, 300, 250, []string{"/tmp/c.txt"}); !over || !dropped {
		t.Errorf("OnFileDrop did not take files outside of the zone: over %v, dropped %v", over, dropped)
	}
	if e := events.Load(); e == nil || !slices.Equal(e.Paths, []string{"/tmp/c.txt"}) || e.X != 300 || e.Y != 250 {
		t.Errorf("OnFileDrop got %+v", e)
	}
}

// accessNode is an element as assistive technology reads it.
type accessNode struct{ role, label, value string }

// TestContentWindowAccessibility reads native UI as assistive technology,
// such as VoiceOver, does, and acts on it.
func TestContentWindowAccessibility(t *testing.T) {
	var frames atomic.Int32
	count, agree, name, volume := 0, false, "Ada", 30.0
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Gap(10).Children(func() {
			ui.Text(c, "Settings")
			if ui.Button(c, "Save").Clicked() {
				count++
			}
			ui.Checkbox(c, &agree, "Agree")
			ui.TextInput(c, &name).Label("Name")
			ui.Slider(c, &volume, 0, 100).Label("Volume")
		})
	}
	state := func(fn func()) { mygo.RunOnMain(fn) }
	w := newWindow(t, mygo.WindowOptions{Title: "Accessibility", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	nodes, ok := accessibility(w)
	if !ok {
		t.Skip("accessibility automation not available on this platform")
	}
	find := func(role, label string) (accessNode, bool) {
		nodes, _ = accessibility(w)
		for _, n := range nodes {
			if n.role == role && n.label == label {
				return n, true
			}
		}
		return accessNode{}, false
	}
	want := []accessNode{{roleText, "Settings", "Settings"}, {roleButton, "Save", ""}, {roleCheckBox, "Agree", "0"},
		{roleTextField, "Name", "Ada"}, {roleSlider, "Volume", "30"}}
	for _, n := range want {
		eventually(t, n.role+" "+n.label, func() bool {
			got, ok := find(n.role, n.label)
			return ok && got.value == n.value
		})
	}
	if !accessPerform(w, "Save", "press", "") {
		t.Fatal("cannot press the button")
	}
	eventually(t, "the press", func() bool {
		var c int
		state(func() { c = count })
		return c == 1
	})
	accessPerform(w, "Agree", "press", "")
	eventually(t, "the check box checked", func() bool {
		n, _ := find(roleCheckBox, "Agree")
		return n.value == "1"
	})
	accessPerform(w, "Name", "value", "Grace")
	eventually(t, "the text field's new value", func() bool {
		var s string
		state(func() { s = name })
		return s == "Grace"
	})
	accessPerform(w, "Volume", "increment", "")
	eventually(t, "the slider incremented", func() bool {
		var v float64
		state(func() { v = volume })
		return v > 30
	})
	if accessPerform(w, "Settings", "press", "") {
		t.Error("a text could be pressed")
	}
}

// TestContentWindowObserved resizes a window of native UI and acts on its
// elements as assistive technology does while key-value observing watches
// them, as other code may: the runtime gives each a generated subclass of
// its class, so their overrides must call the superclass of the class they
// are defined in, not of the object's, which would call them again until
// the stack overflows (#79).
func TestContentWindowObserved(t *testing.T) {
	var frames atomic.Int32
	name, notes := "Ada", "Read only"
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Gap(10).Children(func() {
			ui.Text(c, "Settings")
			ui.TextInput(c, &name).Label("Name")
			ui.TextInput(c, &notes).Label("Notes").ReadOnly(true)
			ui.Column(c).Role(ui.RoleMenu).Label("Edit menu").Children(func() {
				ui.Text(c, "Bold").Role(ui.RoleMenuItemCheckBox).Checked(true)
			})
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Observed", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	eventually(t, "the text field", func() bool {
		nodes, _ := accessibility(w)
		return slices.ContainsFunc(nodes, func(n accessNode) bool { return n.role == roleTextField })
	})
	stop, ok := observe(w)
	if !ok {
		t.Skip("only macOS has key-value observing")
	}
	defer stop()
	before := frames.Load()
	w.SetSize(500, 400)
	eventually(t, "a frame at the new size", func() bool { return frames.Load() > before })
	if !accessPerform(w, "Name", "value", "Grace") {
		t.Fatal("cannot set the text field's value")
	}
	eventually(t, "the text field's new value", func() bool {
		var s string
		mygo.RunOnMain(func() { s = name })
		return s == "Grace"
	})
	// AppKit answers for what the element does not.
	if accessPerform(w, "Settings", "press", "") {
		t.Error("a text could be pressed")
	}
	// The overrides of the older API answer too.
	if _, settable, _, ok := axAttribute(w, "Name", "AXValue"); ok && !settable {
		t.Error("the text field's value is not settable")
	}
	if _, settable, _, ok := axAttribute(w, "Notes", "AXValue"); ok && settable {
		t.Error("the read-only text field's value is settable")
	}
	if mark, _, named, ok := axAttribute(w, "Bold", "AXMenuItemMarkChar"); ok && (mark != "✓" || !named) {
		t.Errorf("the menu item's mark is %q, named %v", mark, named)
	}
}

// TestContentWindowListAccessibility reads the rows of a List as assistive
// technology does, and chooses one by pressing it.
func TestContentWindowListAccessibility(t *testing.T) {
	var frames atomic.Int32
	chosen := -1
	var list ui.ListState
	view := func(c *ui.Context) {
		frames.Add(1)
		list.Selected = &chosen
		ui.List(c, &list, 10000, func(i int) {
			ui.Textf(c, "Item %d", i).Padding(6, 10)
		}).Fill().Label("Items")
	}
	w := newWindow(t, mygo.WindowOptions{Title: "List accessibility", Width: 300, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if _, ok := accessibility(w); !ok {
		t.Skip("accessibility automation not available on this platform")
	}
	eventually(t, "the rows", func() bool {
		nodes, _ := accessibility(w)
		rows := 0
		for _, n := range nodes {
			if n.role == roleListItem && strings.HasPrefix(n.label, "Item ") {
				rows++
			}
		}
		return rows > 5 && rows < 100 // those in view, and a few beyond
	})
	if !accessPerform(w, "Item 3", "press", "") {
		t.Fatal("cannot press row 3")
	}
	eventually(t, "row 3 chosen", func() bool {
		var c int
		mygo.RunOnMain(func() { c = chosen })
		return c == 3
	})
}

// TestContentWindowListTypeToChoose types the first letters of a row of a
// focused list, which chooses it: the keys come to a list, which takes no
// text, as they do to a text input.
func TestContentWindowListTypeToChoose(t *testing.T) {
	var frames atomic.Int32
	chosen := -1
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel"}
	label := func(i int) string { return fmt.Sprintf("%s %d", words[i%len(words)], i) }
	var list ui.ListState
	view := func(c *ui.Context) {
		frames.Add(1)
		list.Selected, list.Label = &chosen, label
		ui.List(c, &list, 1000, func(i int) {
			ui.Text(c, label(i)).Padding(6, 10)
		}).Fill()
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Type to choose", Width: 300, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	get := func() (c int) {
		mygo.RunOnMain(func() { c = chosen })
		return c
	}
	switch {
	case clickAndType(w, 100, 12, "hot"):
	case click(w, 100, 12):
		eventually(t, "the first row chosen", func() bool { return get() == 0 })
		if !pressKeys("h") || !pressKeys("o") || !pressKeys("t") {
			t.Skip("typing automation not available on this platform")
		}
	default:
		t.Skip("typing automation not available on this platform")
	}
	eventually(t, "hotel 7 chosen", func() bool { return get() == 7 })
}

// TestContentWindowTyping types into native UI right after a click, before
// the window draws another frame, and composes text with an input method.
func TestContentWindowTyping(t *testing.T) {
	var frames atomic.Int32
	var name string
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Column(c).Fill().Padding(20).Children(func() {
			ui.TextInput(c, &name)
		})
	}
	text := func() (s string) {
		mygo.RunOnMain(func() { s = name })
		return s
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Typing", Width: 400, Height: 200, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if !clickAndType(w, 100, 36, "héllo") {
		t.Skip("typing automation not available on this platform or input source")
	}
	eventually(t, "the typed text", func() bool { return text() == "héllo" })
	compose(w, "にほん", 3, false)
	compose(w, "日本", 0, true)
	eventually(t, "the composed text", func() bool { return text() == "héllo日本" })
}

// TestContentWindowComposingKeys presses Escape and Return in a dialog's
// text input while an input method composes: they are the input method's,
// and neither close the dialog nor submit the input.
func TestContentWindowComposingKeys(t *testing.T) {
	var frames atomic.Int32
	open, submitted, name := true, 0, ""
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Modal(c, &open, func() {
			if ui.TextInput(c, &name).AutoFocus().Submitted() {
				submitted++
			}
		})
	}
	state := func() (o bool, s int, n string) {
		mygo.RunOnMain(func() { o, s, n = open, submitted, name })
		return o, s, n
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Composing", Width: 400, Height: 200, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 1 })
	compose(w, "ni", 2, false)
	if !pressKey(w, 53, "\x1b") { // Escape
		t.Skip("key automation not available on this platform")
	}
	pressKey(w, 36, "\r") // Return
	compose(w, "你", 0, true)
	eventually(t, "the composed text", func() bool { _, _, n := state(); return n == "你" })
	if o, s, _ := state(); !o || s != 0 {
		t.Fatalf("keys typed while composing: the dialog open %v, the input submitted %d times", o, s)
	}
	pressKey(w, 53, "\x1b")
	eventually(t, "Escape closing the dialog", func() bool { o, _, _ := state(); return !o })
}

// TestContentWindow shows native UI: frames, input from the platform,
// Update, capture, and page methods that fail.
func TestContentWindow(t *testing.T) {
	var frames, clicks, rightClicks atomic.Int32
	var label atomic.Value
	label.Store("before")
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Box(c).Fill().Background(ui.RGB(30, 144, 255)).Children(func() {
			b := ui.Box(c).Size(200, 100).Background(ui.RGB(255, 0, 0))
			if b.Clicked() {
				clicks.Add(1)
			}
			if b.RightClicked() {
				rightClicks.Add(1)
			}
			ui.Text(c, label.Load().(string))
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Content", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })

	if w.Page() != nil {
		t.Error("a window showing Content has a page")
	}
	data, err := w.CapturePage()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	// The capture has the window's device pixels, which a tiling window
	// manager may have made more than 400 DIPs wide.
	s := deviceScale(w)
	at := func(x, y float64) color.RGBA {
		return color.RGBAModel.Convert(img.At(b.Min.X+int(x*s), b.Min.Y+int(y*s))).(color.RGBA)
	}
	if c := at(100, 50); c.R < 200 || c.B > 60 {
		t.Errorf("the red box is %v", c)
	}
	if c := at(300, 250); c.B < 200 || c.R > 60 {
		t.Errorf("the background is %v", c)
	}
	// With MYGO_GPU=1 Linux draws with OpenGL without a GPU too: the
	// capture is drawn on the CPU, the GtkGLArea shows what OpenGL drew.
	if os.Getenv("MYGO_GPU") == "1" {
		if how, pix, gw, _, ok := glSurface(w); ok {
			if how != "opengl" || len(pix) == 0 {
				t.Fatalf("the surface draws %q, not with OpenGL", how)
			}
			bgra := func(x, y float64) []byte {
				return pix[(int(y*s)*gw+int(x*s))*4:][:4]
			}
			if c := bgra(100, 50); c[2] < 200 || c[0] > 60 {
				t.Errorf("the red box is %v (BGRA) in the GtkGLArea", c)
			}
			if c := bgra(300, 250); c[0] < 200 || c[2] > 60 {
				t.Errorf("the background is %v (BGRA) in the GtkGLArea", c)
			}
		}
	}

	before := frames.Load()
	w.Update(func() { label.Store("after") })
	eventually(t, "a frame after Update", func() bool { return frames.Load() > before })

	if !click(w, 100, 50) {
		t.Skip("click automation not available on this platform")
	}
	eventually(t, "the click", func() bool { return clicks.Load() == 1 })
	click(w, 300, 250) // outside the box
	click(w, 150, 80)
	eventually(t, "the second click", func() bool { return clicks.Load() == 2 })

	// A Control-click is a secondary click on macOS.
	if controlClick(w, 100, 50) {
		eventually(t, "the Control-click", func() bool { return rightClicks.Load() == 1 })
		if clicks.Load() != 2 {
			t.Errorf("the Control-click clicked: %d clicks", clicks.Load())
		}
	}
}

// TestContentWindowVibrancy shows native UI over the window's material,
// which the view learns shows (ui.Context.Vibrancy) on macOS and on Windows
// 11 22H2 and later: there the window has no redirection bitmap, and the
// screen shows the material where the view draws nothing, beside its
// opaque pane, not black. Without a material, the view is told so.
func TestContentWindowVibrancy(t *testing.T) {
	var frames atomic.Int32
	var shows atomic.Bool
	view := func(c *ui.Context) {
		frames.Add(1)
		shows.Store(c.Vibrancy())
		if c.Vibrancy() {
			c.Root().Background(ui.Transparent)
		}
		ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
			ui.Box(c).Width(200)
			ui.Box(c).Grow(1).Background(ui.RGB(255, 0, 0))
		})
	}
	w := newWindow(t, mygo.WindowOptions{X: 40, Y: 40, Width: 400, Height: 300, Vibrancy: mygo.VibrancyMica, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	noRedirect, _, onWindows := composition(w)
	want := runtime.GOOS == "darwin" || onWindows && noRedirect
	eventually(t, fmt.Sprintf("the view told the material shows: %v", want), func() bool { return shows.Load() == want })

	if onWindows && noRedirect {
		red := func(x float64) bool {
			r, g, b, _ := screenColor(w, x, 150)
			return r > 200 && g < 60 && b < 60
		}
		readable := false
		for deadline := time.Now().Add(3 * time.Second); !readable && time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			readable = red(300)
		}
		if readable {
			if r, g, b, _ := screenColor(w, 100, 150); int(r)+int(g)+int(b) < 24 {
				t.Errorf("the screen shows %d, %d, %d beside the pane, not the material", r, g, b)
			}
			// As the window changes size, its frames show as drawn, not
			// magnified: the pane still starts 200 DIPs in. Only until the
			// renderer settles, a second after, would they show otherwise.
			w.SetSize(440, 320)
			resized := time.Now()
			if !red(215) && time.Since(resized) < 500*time.Millisecond {
				t.Error("once the window grew, the screen shows the pane magnified")
			}
		} else {
			t.Log("the screen does not show the window's pane: not reading the material")
		}
	}

	w.SetVibrancy(mygo.VibrancyNone)
	eventually(t, "the view told no material shows", func() bool { return !shows.Load() })
}

// TestContentWindowRepaintsWhatChanged moves the red row of a window of
// native UI drawing in memory under a menu bar, and reads what the display
// shows. On Linux, GTK repaints only what frames drawn in memory changed,
// below the menu bar.
func TestContentWindowRepaintsWhatChanged(t *testing.T) {
	if !memoryUI(t) {
		t.Skip("only Linux repaints what frames drawn in memory changed")
	}
	prev := mygo.App.Menu()
	defer mygo.App.SetMenu(prev)
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Label: "App", Submenu: []*mygo.MenuItem{{Label: "Item"}}}}))
	var frames, red atomic.Int32
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Box(c).Fill().Padding(20).Gap(20).Background(ui.RGB(30, 144, 255)).Children(func() {
			for i := range int32(3) {
				color := ui.RGB(255, 255, 255)
				if i == red.Load() {
					color = ui.RGB(255, 0, 0)
				}
				ui.Box(c).Size(300, 40).Background(color)
			}
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Repaint", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	s := deviceScale(w)
	// rows tells what the display shows at the top and the bottom of each
	// row: r for red, w for white.
	rows := func() string {
		b, _ := surfaceOnScreen(w)
		m, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			return ""
		}
		shown := ""
		for i := range 3 {
			for _, y := range []float64{25, 55} {
				r, g, _, _ := m.At(int(200*s), int((float64(i*60)+y)*s)).RGBA()
				switch {
				case r>>8 > 200 && g>>8 < 60:
					shown += "r"
				case r>>8 > 200:
					shown += "w"
				default:
					shown += "?"
				}
			}
		}
		return shown
	}
	for _, to := range []int{2, 0, 1} {
		before := frames.Load()
		w.Update(func() { red.Store(int32(to)) })
		eventually(t, "a frame after Update", func() bool { return frames.Load() > before })
		want := strings.Repeat("ww", to) + "rr" + strings.Repeat("ww", 2-to)
		deadline := time.Now().Add(5 * time.Second)
		for shown := rows(); shown != want; shown = rows() {
			if time.Now().After(deadline) {
				t.Fatalf("with row %d red, the display shows %q, not %q", to, shown, want)
			}
			time.Sleep(20 * time.Millisecond)
		}
	}
}

// TestContentWindowMenuButton opens a menu button's menu, which shows as
// the button goes down, and chooses an item.
func TestContentWindowMenuButton(t *testing.T) {
	var frames atomic.Int32
	var chosen atomic.Value
	chosen.Store("")
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Box(c).Fill().Padding(20).Children(func() {
			ui.MenuButton(c, "Export", func(m *ui.Menu) {
				for _, label := range []string{"As PDF", "As PNG"} {
					if m.Item(label).Chosen() {
						chosen.Store(label)
					}
				}
			}).Size(120, 40)
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Menu button", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if _, ok := popupMenus(); !ok || !click(w, 80, 40) {
		t.Skip("menu automation not available on this platform")
	}
	var menus [][]string
	eventually(t, "the menu", func() bool {
		menus, _ = popupMenus()
		return len(menus) == 1
	})
	if want := []string{"As PDF", "As PNG"}; !slices.Equal(menus[0], want) {
		t.Errorf("the menu shows %q, want %q", menus[0], want)
	}
	if !choosePopupItem("As PNG") {
		t.Fatal("the menu has no item As PNG")
	}
	eventually(t, "the choice", func() bool { return chosen.Load() == "As PNG" })
	eventually(t, "the menu to close", func() bool {
		menus, _ = popupMenus()
		return len(menus) == 0
	})
}

// TestReorderByDragging drags a row of a list below another with the
// events of a real mouse.
func TestReorderByDragging(t *testing.T) {
	var frames atomic.Int32
	var order atomic.Value
	items := []string{"a", "b", "c", "d", "e"}
	order.Store(strings.Join(items, ""))
	s := ui.ListState{Key: func(i int) any { return items[i] }}
	s.Reorder = func(rows []int, to int) {
		moved := items[rows[0]]
		items = slices.Delete(slices.Clone(items), rows[0], rows[0]+1)
		if to > rows[0] {
			to--
		}
		items = slices.Insert(items, to, moved)
		order.Store(strings.Join(items, ""))
	}
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.List(c, &s, len(items), func(i int) {
			ui.Box(c).Height(30).Children(func() { ui.Text(c, items[i]) })
		}).Fill()
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Reorder", Width: 300, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	// Row b, from 30 to 60, to the bottom of row d, from 90 to 120.
	if !drag(w, [][2]float64{{50, 45}, {50, 50}, {50, 70}, {50, 100}, {50, 118}}) {
		t.Skip("pointer automation not available on this platform")
	}
	eventually(t, "the new order", func() bool { return order.Load() == "acdbe" })
}

// TestRouterSideButtons goes back and forward in a router with the side
// buttons of a mouse, which arrive as keys.
func TestRouterSideButtons(t *testing.T) {
	var frames atomic.Int32
	var path atomic.Value
	r := ui.NewRouter("/a")
	r.Push("/b")
	r.Transition = ui.TransitionNone
	view := func(c *ui.Context) {
		frames.Add(1)
		r.View(c, func(rt *ui.Route) {
			path.Store(rt.Path())
			ui.Text(c, rt.Path())
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Router", Width: 300, Height: 200, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if !sideButton(w, true) {
		t.Skip("pointer automation not available on this platform")
	}
	eventually(t, "back to /a", func() bool { return path.Load() == "/a" })
	sideButton(w, false)
	eventually(t, "forward to /b", func() bool { return path.Load() == "/b" })
}

func TestContentWindowContextMenu(t *testing.T) {
	var frames atomic.Int32
	var chosen atomic.Value
	chosen.Store("")
	view := func(c *ui.Context) {
		frames.Add(1)
		ui.Box(c).Fill().Children(func() {
			ui.Box(c).Size(200, 100).Background(ui.RGB(255, 0, 0)).ContextMenu(func(m *ui.Menu) {
				for _, label := range []string{"First", "Second"} {
					if m.Item(label).Chosen() {
						chosen.Store(label)
					}
				}
				m.Separator()
				m.Item("Third").Disabled(true)
			})
		})
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Context menu", Width: 400, Height: 300, Content: ui.View(view)})
	eventually(t, "a frame", func() bool { return frames.Load() > 0 })
	if _, ok := popupMenus(); !ok || !rightClick(w, 100, 50) {
		t.Skip("context menu automation not available on this platform")
	}
	var menus [][]string
	eventually(t, "the context menu", func() bool {
		menus, _ = popupMenus()
		return len(menus) == 1
	})
	if want := []string{"First", "Second", "-", "Third"}; !slices.Equal(menus[0], want) {
		t.Errorf("the menu shows %q, want %q", menus[0], want)
	}
	if !choosePopupItem("Second") {
		t.Fatal("the menu has no item Second")
	}
	eventually(t, "the choice", func() bool { return chosen.Load() == "Second" })
	eventually(t, "the menu to close", func() bool {
		menus, _ = popupMenus()
		return len(menus) == 0
	})
}
