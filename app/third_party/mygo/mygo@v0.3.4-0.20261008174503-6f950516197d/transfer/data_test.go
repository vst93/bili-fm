package transfer

import (
	"bytes"
	"errors"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRepresentationsAndSnapshots(t *testing.T) {
	var calls atomic.Int32
	input := []byte("plain")
	d := New(NewItem(Bytes(Text, input), Lazy(HTML, func() ([]byte, error) {
		calls.Add(1)
		return []byte("<b>plain</b>"), nil
	})))
	input[0] = 'x'
	if got := d.Formats(); !slices.Equal(got, []Format{Text, HTML}) || calls.Load() != 0 {
		t.Fatalf("formats %v, calls %d", got, calls.Load())
	}
	b, _ := d.Read(Text)
	if string(b) != "plain" {
		t.Fatalf("stored input changed: %q", b)
	}
	b[0] = 'x'
	b, _ = d.Read(Text)
	if string(b) != "plain" {
		t.Fatal("read exposed mutable storage")
	}
	snapshot := d.Snapshot()
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			b, err := snapshot.Read(HTML)
			if err != nil || string(b) != "<b>plain</b>" {
				t.Errorf("read %q: %v", b, err)
			}
		})
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("provider called %d times", calls.Load())
	}
	d.Snapshot().Read(HTML)
	if calls.Load() != 2 {
		t.Fatal("a new session reused the old provider cache")
	}
}

func TestMaterializeAndProviderFailure(t *testing.T) {
	boom := errors.New("encode failed")
	n := 0
	d := New(NewItem(Bytes(Text, []byte("ok")), Lazy(HTML, func() ([]byte, error) { n++; return nil, boom })))
	m, err := d.Materialize([]Format{Text})
	if err != nil || n != 0 || !slices.Equal(m.Formats(), []Format{Text}) {
		t.Fatalf("materialize %v: %v, calls %d", m.Formats(), err, n)
	}
	for range 2 {
		if _, err := d.Materialize([]Format{HTML}); !errors.Is(err, boom) {
			t.Fatalf("failed format: %v", err)
		}
	}
	if n != 1 {
		t.Fatal("provider failures were not cached")
	}
	if _, err := d.Materialize([]Format{PNG}); !errors.Is(err, ErrFormat) {
		t.Fatalf("empty match: %v", err)
	}
	empty := New(NewItem(Bytes(Text, nil)))
	if b, err := empty.Read(Text); err != nil || len(b) != 0 {
		t.Fatalf("empty data: %q, %v", b, err)
	}
}

func TestFileAndURLRepresentations(t *testing.T) {
	paths := []string{filepath.Join(t.TempDir(), "a #%.txt"), filepath.Join(t.TempDir(), "日本語.txt")}
	d, err := FileData(paths...)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := d.Files(); err != nil || !slices.Equal(got, paths) {
		t.Fatalf("files %q: %v", got, err)
	}
	if len(d.Items()) != 2 {
		t.Fatal("file item boundaries lost")
	}
	urls, _ := d.URLs()
	if !bytes.Contains([]byte(urls[0]), []byte("%23")) {
		t.Fatal("file path was not URL escaped")
	}
	other, _ := URLData("https://example.com/a#b")
	if files, err := other.Files(); err != nil || len(files) != 0 {
		t.Fatalf("web URL became a file: %q, %v", files, err)
	}
	remote := New(NewItem(Bytes(URIList, []byte("# comment\r\nfile://remote/tmp/a\r\nhttps://example.com\r\n"))))
	if files, _ := remote.Files(); len(files) != 0 {
		t.Fatalf("remote paths %q", files)
	}
	if _, err := FileData("relative"); err == nil {
		t.Fatal("relative file accepted")
	}
	if _, err := URLData("no-scheme"); err == nil {
		t.Fatal("relative URL accepted")
	}
}

func TestMultipleItemsCoalesceAndKeepAlternatives(t *testing.T) {
	custom := Format("application/vnd.mygo.test")
	d := New(NewItem(Bytes(Text, []byte("a")), Bytes(custom, []byte{0, 1})), NewItem(Bytes(Text, []byte("b")), Bytes(custom, []byte{2, 3})))
	b, _ := d.Read(Text)
	if string(b) != "a\nb" {
		t.Fatalf("text %q", b)
	}
	b, _ = d.Read(custom)
	if !bytes.Equal(b, []byte{0, 1}) {
		t.Fatalf("custom bytes %v", b)
	}
	m, err := d.Materialize([]Format{custom})
	if err != nil || len(m.Items()) != 2 {
		t.Fatalf("materialized items: %d, %v", len(m.Items()), err)
	}
}

func TestOperationNegotiation(t *testing.T) {
	for _, tt := range []struct{ source, destination, suggested, want Operation }{
		{Copy | Move, 0, Move, Copy}, {Copy | Move, Copy | Move, Move, Move},
		{Move, Copy, Move, None}, {Copy, Move, Copy, None},
		{Copy | Move, Copy | Move, None, Copy}, {Move, Move, Copy, Move},
		{None, Copy | Move, Copy, None},
	} {
		if got := Negotiate(tt.source, tt.destination, tt.suggested); got != tt.want {
			t.Errorf("%v & %v suggested %v: %v, want %v", tt.source, tt.destination, tt.suggested, got, tt.want)
		}
	}
}

func TestProviderPanicIsCachedAndNegotiationIsLazy(t *testing.T) {
	var calls atomic.Int32
	d := New(NewItem(Lazy(HTML, func() ([]byte, error) { calls.Add(1); panic("codec failed") }), Bytes(Text, []byte("fallback"))))
	if f, ok := d.Preferred(PNG, Text, HTML); !ok || f != Text || calls.Load() != 0 {
		t.Fatal("negotiation read a provider")
	}
	if _, ok := d.Preferred(PNG); ok {
		t.Fatal("unavailable preference matched")
	}
	for range 2 {
		b, err := d.Read(HTML)
		if !errors.Is(err, ErrProviderPanic) || len(b) != 0 {
			t.Fatalf("panic result %q: %v", b, err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("panicking provider retried")
	}
	if b, err := d.Read(Text); err != nil || string(b) != "fallback" {
		t.Fatal("panic lost the alternative")
	}
}
