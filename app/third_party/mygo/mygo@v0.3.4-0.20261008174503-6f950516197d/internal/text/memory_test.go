package text

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
	"unsafe"
	"weak"
)

// Scrolling must not retain hundreds of frames of glyph arrays that
// cannot expire once the window stops drawing.
func TestLayoutCacheBoundsScrollingMemory(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	const rows = 40
	for frame := range 300 {
		for row := range rows {
			s.Layout(Params{Text: fmt.Sprintf("line %d: %s", frame*rows+row, strings.Repeat("code ", 20))})
		}
		s.EndFrame()
		if s.layoutBytes > maxLayoutBytes || len(s.layouts) > maxLayouts {
			t.Fatalf("frame %d retained %d bytes in %d layouts", frame, s.layoutBytes, len(s.layouts))
		}
	}
}

func TestLayoutCacheReusesDisplayedText(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	p := Params{Text: "unchanged visible text"}
	l := s.Layout(p)
	for range 10 {
		s.EndFrame()
		if got := s.Layout(p); got != l {
			t.Fatal("a new frame reshaped unchanged visible text")
		}
		if got := s.Layout(p); got != l {
			t.Fatal("repeated measurement in one frame reshaped the text")
		}
	}
	if n := s.LayoutsMade(); n != 1 {
		t.Fatalf("made %d layouts for unchanged text", n)
	}
}

func TestLayoutCacheReleasesEvictedText(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	old := weak.Make(s.Layout(Params{Text: "the first viewport"}))
	for i := range maxLayouts + 1 {
		s.Layout(Params{Text: fmt.Sprint(i)})
	}
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("evicted text is retained while the window is idle")
	}
	runtime.KeepAlive(s)
}

func TestLayoutCacheKeepsFrequentlyUsedText(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	p := Params{Text: "a frequently used label"}
	l := s.Layout(p)
	for i := range maxLayouts + 1 {
		s.Layout(Params{Text: fmt.Sprint(i)})
		if got := s.Layout(p); got != l {
			t.Fatal("cache pressure evicted a frequently used label")
		}
	}
}

func TestLayoutCacheSkipsOversizedParagraph(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	p := Params{Text: "a small label"}
	l := s.Layout(p)
	large := s.Layout(Params{Text: strings.Repeat("x", maxLayoutBytes/32)})
	if len(large.Runes) != maxLayoutBytes/32 || len(large.Lines) == 0 {
		t.Fatal("oversized text did not lay out")
	}
	if len(s.layouts) != 1 || s.Layout(p) != l {
		t.Fatal("an oversized paragraph displaced the ordinary labels")
	}
}

func TestRetainedLayoutDoesNotRetainClearedCache(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	kept := s.Layout(Params{Text: "kept by an element"})
	other := weak.Make(s.Layout(Params{Text: "no longer used"}))
	s.clearLayouts()
	runtime.GC()
	if other.Value() != nil || s.layoutBytes != 0 {
		t.Fatal("a retained layout keeps the cleared cache alive")
	}
	runtime.KeepAlive(kept)
	runtime.KeepAlive(s)
}

func cacheDocumentExcerpt(s *System) weak.Pointer[byte] {
	document := strings.Repeat("x", 1<<20)
	old := weak.Make(unsafe.StringData(document))
	s.Layout(Params{Text: document[100:110]})
	return old
}

func TestLayoutCacheDoesNotRetainExcerptSource(t *testing.T) {
	s := newSystem()
	s.eng = &stubEngine{}
	old := cacheDocumentExcerpt(s)
	runtime.GC()
	if old.Value() != nil {
		t.Fatal("a cached ten-character excerpt retains its entire source document")
	}
	if s.Layout(Params{Text: strings.Repeat("x", 10)}).Params.Text != strings.Repeat("x", 10) {
		t.Fatal("the cache changed the excerpt's text")
	}
	runtime.KeepAlive(s)
}
