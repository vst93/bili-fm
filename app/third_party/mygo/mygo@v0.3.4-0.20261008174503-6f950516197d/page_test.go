package mygo

import (
	"errors"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestPage(t *testing.T) {
	w := NewWindow(WindowOptions{Hidden: true, URL: "https://example.com/a", Page: PageOptions{
		PreloadScript: "window.preloaded = true", UserAgent: "agent", ZoomFactor: 1.5, DevTools: DevToolsEnabled,
	}})
	t.Cleanup(w.Destroy)
	fw := fb.Windows()[len(fb.Windows())-1]
	p := w.Page()
	if p == nil || p.Window() != w {
		t.Fatalf("Page %v", p)
	}
	o := fw.Opts
	preloaded := slices.ContainsFunc(o.UserScripts, func(s platform.UserScript) bool { return s.Source == "window.preloaded = true" })
	if o.UserAgent != "agent" || o.Zoom != 1.5 || !o.DevTools || !preloaded {
		t.Errorf("page options: user agent %q, zoom %v, devtools %v, preloaded %v", o.UserAgent, o.Zoom, o.DevTools, preloaded)
	}
	if got := p.URL(); got != "https://example.com/a" {
		t.Errorf("URL %q", got)
	}
	if err := p.LoadURL("https://example.com/b"); err != nil || p.URL() != "https://example.com/b" {
		t.Errorf("LoadURL: %v, URL %q", err, p.URL())
	}
	if _, err := EvalAs[int]((*Page)(nil), "1"); !errors.Is(err, errNoPage) {
		t.Errorf("EvalAs of no page: %v", err)
	}
}
