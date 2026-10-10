package mygo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
)

// The benchmarks measure the Go side of IPC and custom schemes on the fake
// backend: decoding, calling, encoding and batching, without a webview.

type benchItem struct {
	ID    int      `json:"id"`
	Title string   `json:"title"`
	Done  bool     `json:"done"`
	Tags  []string `json:"tags"`
}

type benchService struct{}

func (benchService) Noop()                           {}
func (benchService) Echo(s string) string            { return s }
func (benchService) Items(v []benchItem) []benchItem { return v }

// Count sends 0 to n-1.
func (benchService) Count(n int, ch *Channel[int]) error {
	for i := range n {
		if err := ch.Send(i); err != nil {
			return err
		}
	}
	return nil
}

var (
	benchOnce  sync.Once
	benchEvent *Event[benchItem]
)

// benchWindow returns a ready window whose scripts go to eval.
func benchWindow(b *testing.B, eval func(js string)) (*Window, *fake.Window) {
	b.Helper()
	benchOnce.Do(func() {
		BindAs("Bench", benchService{})
		benchEvent = NewEvent[benchItem]("bench:item")
	})
	w := NewWindow(WindowOptions{})
	b.Cleanup(w.Destroy)
	wins := fb.Windows()
	fw := wins[len(wins)-1]
	onMain(func() {
		fw.OnEval = eval
		fw.H.NavigationCommitted("about:blank")
	})
	page(fw, `{"t":"dom-ready"}`)
	return w, fw
}

func benchmarkCall(b *testing.B, method string, args any) {
	replies := make(chan struct{}, 1)
	_, fw := benchWindow(b, func(js string) {
		if strings.Contains(js, `"t":"reply"`) {
			replies <- struct{}{}
		}
	})
	a, _ := json.Marshal(args)
	msg := secret(fw) + `{"t":"call","id":1,"k":"tok","m":"Bench.` + method + `","a":` + string(a) + `}`
	b.SetBytes(int64(len(msg)))
	b.ReportAllocs()
	for b.Loop() {
		onMain(func() { fw.H.Message(msg) })
		<-replies
	}
}

func benchItems(n int) []benchItem {
	items := make([]benchItem, n)
	for i := range items {
		items[i] = benchItem{ID: i, Title: "Item number " + strconv.Itoa(i), Done: i%2 == 0, Tags: []string{"alpha", "beta"}}
	}
	return items
}

// BenchmarkCallNoop calls a method that takes and returns nothing.
func BenchmarkCallNoop(b *testing.B) { benchmarkCall(b, "Noop", []any{}) }

// BenchmarkChannel1K calls a method that streams a thousand values through
// a channel.
func BenchmarkChannel1K(b *testing.B) { benchmarkCall(b, "Count", []any{1000, 1}) }

// BenchmarkCallEcho1KB calls a method that returns the kilobyte string it
// gets.
func BenchmarkCallEcho1KB(b *testing.B) { benchmarkCall(b, "Echo", []any{strings.Repeat("x", 1<<10)}) }

// BenchmarkCallEcho1MB calls a method that returns the megabyte string it
// gets.
func BenchmarkCallEcho1MB(b *testing.B) { benchmarkCall(b, "Echo", []any{strings.Repeat("x", 1<<20)}) }

// BenchmarkCallItems1K calls a method that returns the thousand structs it
// gets.
func BenchmarkCallItems1K(b *testing.B) { benchmarkCall(b, "Items", []any{benchItems(1000)}) }

// BenchmarkEmit emits an event to a window's page.
func BenchmarkEmit(b *testing.B) {
	flushed := make(chan struct{}, 1)
	w, _ := benchWindow(b, func(string) { flushed <- struct{}{} })
	item := benchItems(1)[0]
	b.ReportAllocs()
	for b.Loop() {
		_ = benchEvent.Emit(w, item)
		<-flushed
	}
}

type discardResponder struct{ finished chan struct{} }

func (discardResponder) Respond(int, http.Header) {}
func (discardResponder) Write([]byte)             {}
func (r discardResponder) Finish()                { r.finished <- struct{}{} }
func (r discardResponder) Fail(error)             { r.finished <- struct{}{} }

// BenchmarkScheme8MB serves 8 MB from a custom scheme's handler.
func BenchmarkScheme8MB(b *testing.B) {
	const size = 8 << 20
	chunk := make([]byte, 32<<10)
	_ = Protocol.HandleFunc("bench", func(w http.ResponseWriter, r *http.Request) {
		for n := 0; n < size; n += len(chunk) {
			_, _ = w.Write(chunk)
		}
	})
	b.Cleanup(func() { Protocol.Unhandle("bench") })
	_, fw := benchWindow(b, func(string) {})
	resp := discardResponder{finished: make(chan struct{}, 1)}
	b.SetBytes(size)
	b.ReportAllocs()
	for b.Loop() {
		req := &platform.SchemeRequest{Context: context.Background(), Method: "GET", URL: "bench://localhost/", Header: http.Header{}, Body: io.NopCloser(strings.NewReader("")), Responder: resp}
		onMain(func() { fw.H.SchemeRequest(req) })
		<-resp.finished
	}
}
