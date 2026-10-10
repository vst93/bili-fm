package e2e

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// The benchmarks time what the real backend adds to the Go side that the
// root package's benchmarks measure: the webview's messages, its custom
// scheme, and opening windows. Like the tests, they need a desktop session:
//
//	MYGO_E2E=1 go test -run '^$' -bench . ./internal/e2e

// Bench is the service the benchmarks' pages call.
type Bench struct{}

type BenchItem struct {
	ID    int      `json:"id"`
	Title string   `json:"title"`
	Done  bool     `json:"done"`
	Tags  []string `json:"tags"`
}

func (Bench) Noop()                           {}
func (Bench) Items(v []BenchItem) []BenchItem { return v }

var benchTick = mygo.NewEvent[Tick]("bench:tick")

// useBench serves the benchmarks' responses: /bench/1mb is a megabyte.
func useBench(mux *http.ServeMux) {
	mb := []byte(strings.Repeat("x", 1<<20))
	mux.HandleFunc("/bench/1mb", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(mb)))
		w.Write(mb)
	})
}

// benchWindow opens a visible window, as webviews in hidden ones may be
// throttled, with the page of the tests loaded.
func benchWindow(b *testing.B) *mygo.Window {
	b.Helper()
	w := mygo.NewWindow(mygo.WindowOptions{Title: "Bench", Width: 400, Height: 300})
	b.Cleanup(w.Destroy)
	if err := w.Page().LoadURL("app://localhost/"); err != nil {
		b.Fatal(err)
	}
	waitFor(b, w, "window.run")
	return w
}

// benchPage runs b.N rounds of the page's script, in which N stands for
// b.N: the page does the work, so each operation costs what it costs a
// page, without a round trip from Go.
func benchPage(b *testing.B, w *mygo.Window, script string) {
	b.Helper()
	b.ResetTimer()
	n, err := mygo.EvalAs[int](w.Page(), "(async (N) => {"+script+"; return N})("+strconv.Itoa(b.N)+")")
	b.StopTimer()
	if err != nil || n != b.N {
		b.Fatalf("the page ran %d of %d: %v", n, b.N, err)
	}
}

// BenchmarkPageCall awaits calls of a Go method from a page, one at a
// time.
func BenchmarkPageCall(b *testing.B) {
	w := benchWindow(b)
	benchPage(b, w, `for (let i = 0; i < N; i++) await mygo.call("Bench.Noop")`)
}

// BenchmarkPageCallItems sends a thousand structs from a page to Go and
// gets them back.
func BenchmarkPageCallItems(b *testing.B) {
	w := benchWindow(b)
	benchPage(b, w, `const items = Array.from({ length: 1000 }, (_, i) => ({ id: i, title: "Item number " + i, done: i % 2 === 0, tags: ["alpha", "beta"] }));
for (let i = 0; i < N; i++) {
  const got = await mygo.call("Bench.Items", items);
  if (got.length !== items.length) throw new Error("got " + got.length + " items");
}`)
}

// BenchmarkPageChannel streams values from a Go method to a page through a
// channel.
func BenchmarkPageChannel(b *testing.B) {
	w := benchWindow(b)
	benchPage(b, w, `const ch = mygo.channel();
const done = mygo.call("Streams.Count", N, ch);
let n = 0;
for await (const v of ch) if (v !== n++) throw new Error("out of order: " + v);
await done;
if (n !== N) throw new Error("got " + n + " values")`)
}

// BenchmarkPageEvent emits events to a page, until the page has them all.
func BenchmarkPageEvent(b *testing.B) {
	w := benchWindow(b)
	if _, err := w.Page().Eval(`window.benchEvents = (N) => new Promise((resolve) => {
  let n = 0;
  const off = mygo.on("bench:tick", () => { if (++n === N) { off(); resolve(n) } });
})`); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	done := make(chan error, 1)
	go func() {
		n, err := mygo.EvalAs[int](w.Page(), "benchEvents("+strconv.Itoa(b.N)+")")
		if err == nil && n != b.N {
			err = fmt.Errorf("the page got %d of %d events", n, b.N)
		}
		done <- err
	}()
	// The page listens once the script above has run, which a script
	// evaluated after it tells.
	if _, err := w.Page().Eval("0"); err != nil {
		b.Fatal(err)
	}
	for i := range b.N {
		if err := benchTick.Emit(w, Tick{N: i}); err != nil {
			b.Fatal(err)
		}
	}
	if err := <-done; err != nil {
		b.Fatal(err)
	}
}

// BenchmarkPageFetch1MB fetches a megabyte from the app's custom scheme in
// a page.
func BenchmarkPageFetch1MB(b *testing.B) {
	w := benchWindow(b)
	b.SetBytes(1 << 20)
	benchPage(b, w, `for (let i = 0; i < N; i++) {
  const body = await (await fetch("/bench/1mb")).arrayBuffer();
  if (body.byteLength !== 1 << 20) throw new Error("got " + body.byteLength + " bytes");
}`)
}

// BenchmarkWindowOpen opens a window loading a page from the app's scheme,
// until its DOM is ready.
func BenchmarkWindowOpen(b *testing.B) {
	ready := make(chan struct{}, 1)
	for b.Loop() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Bench", Width: 400, Height: 300})
		w.Page().OnDOMReady(func() { ready <- struct{}{} })
		if err := w.Page().LoadURL("app://localhost/"); err != nil {
			b.Fatal(err)
		}
		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			b.Fatal("the page did not load")
		}
		w.Destroy()
	}
}

// BenchmarkContentWindowOpen opens a window of native UI, until its view
// has built the first frame.
func BenchmarkContentWindowOpen(b *testing.B) {
	built := make(chan struct{}, 1)
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Padding(16).Gap(8).Children(func() {
			ui.Text(c, "Bench").FontSize(20)
			for i := range 20 {
				ui.Text(c, "Row "+strconv.Itoa(i))
			}
		})
		select {
		case built <- struct{}{}:
		default:
		}
	}
	for b.Loop() {
		w := mygo.NewWindow(mygo.WindowOptions{Title: "Bench", Width: 400, Height: 300, Content: ui.View(view)})
		select {
		case <-built:
		case <-time.After(10 * time.Second):
			b.Fatal("no frame")
		}
		w.Destroy()
	}
}
