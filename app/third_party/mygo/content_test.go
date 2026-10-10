package mygo

import (
	"bytes"
	"errors"
	"image/png"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/fake"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/ui"
)

// contentWindow creates a window showing view and draws its first frame.
func contentWindow(t *testing.T, view func(c *ui.Context)) (*Window, *fake.Window, *fake.Surface) {
	t.Helper()
	w := NewWindow(WindowOptions{Width: 300, Height: 200, Content: ui.View(view)})
	t.Cleanup(w.Destroy)
	wins := fb.Windows()
	fw := wins[len(wins)-1]
	s := fw.FakeSurface()
	if s == nil {
		t.Fatal("the window has no surface")
	}
	onMain(func() { s.Frame() })
	return w, fw, s
}

func TestContentDrawsAndHandlesInput(t *testing.T) {
	clicks := 0
	view := func(c *ui.Context) {
		c.Root().Background(ui.RGB(255, 0, 0))
		if ui.Button(c, "Press").Absolute().Left(10).Top(10).Size(100, 40).Clicked() {
			clicks++
		}
	}
	w, _, s := contentWindow(t, view)
	if s.Frames() != 1 {
		t.Fatalf("%d frames presented", s.Frames())
	}
	if p := s.Pixel(250, 150); p != [4]byte{0, 0, 255, 255} {
		t.Errorf("background pixel %v", p)
	}
	onMain(func() {
		s.Send(platform.SurfaceEvent{Kind: platform.PointerDown, X: 50, Y: 30})
		s.Send(platform.SurfaceEvent{Kind: platform.PointerUp, X: 50, Y: 30})
		s.Frame()
	})
	if clicks != 1 {
		t.Errorf("%d clicks", clicks)
	}

	// The window has no page. The methods of its Page, which only MyGo
	// reaches, report it or do nothing.
	p := w.pg
	if w.Page() != nil {
		t.Error("a window showing Content has a page")
	}
	if _, err := p.Eval("1"); !errors.Is(err, errNoPage) {
		t.Errorf("Eval: %v", err)
	}
	if err := p.LoadURL("https://example.com"); !errors.Is(err, errNoPage) || p.URL() != "" {
		t.Errorf("LoadURL: %v, URL %q", err, p.URL())
	}
	if err := p.LoadFile("index.html"); !errors.Is(err, errNoPage) {
		t.Errorf("LoadFile: %v", err)
	}
	if _, err := p.PrintToPDF(PDFOptions{}); !errors.Is(err, errNoPage) {
		t.Errorf("PrintToPDF: %v", err)
	}
	if _, err := p.FindInPage("x", FindOptions{}); !errors.Is(err, errNoPage) {
		t.Errorf("FindInPage: %v", err)
	}
	p.StopFindInPage()
	p.Reload()

	data, err := w.CapturePage()
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 300 || b.Dy() != 200 {
		t.Errorf("capture is %v", b)
	}
	if r, g, b, _ := img.At(250, 150).RGBA(); r>>8 != 255 || g != 0 || b != 0 {
		t.Errorf("captured pixel %d %d %d", r>>8, g>>8, b>>8)
	}
}

func TestContentInvalidateAndUpdate(t *testing.T) {
	n := 0
	view := func(c *ui.Context) { ui.Textf(c, "n = %d", n) }
	w, _, s := contentWindow(t, view)
	before := s.Frames()
	w.Update(func() { n = 5 })
	onMain(func() {})
	onMain(func() { s.Frame() })
	if s.Frames() != before+1 {
		t.Errorf("Update drew %d frames", s.Frames()-before)
	}
	w.Invalidate()
	w.Invalidate()
	onMain(func() {})
	onMain(func() { s.Frame() })
	if s.Frames() != before+2 {
		t.Errorf("two Invalidate calls drew %d frames", s.Frames()-before-1)
	}
}

// TestContentTitleBar checks that native UI gets the room the window
// controls of a hidden title bar take, and a frame when it changes.
func TestContentTitleBar(t *testing.T) {
	for _, style := range []TitleBarStyle{TitleBarHidden, TitleBarHiddenInset} {
		t.Run(string(style), func(t *testing.T) { testContentTitleBar(t, style) })
	}
}

// A native UI window has no webview. On Linux, even the function that
// reads its zoom is nil until the first webview is created.
type contentTitleBarWindow struct {
	platform.Window
	t *testing.T
}

func (w *contentTitleBarWindow) Zoom() float64 {
	w.t.Error("queried webview zoom for a native UI window")
	return 1
}

func testContentTitleBar(t *testing.T, style TitleBarStyle) {
	t.Helper()
	var bar ui.TitleBar
	view := func(c *ui.Context) { bar = c.TitleBar() }
	w := NewWindow(WindowOptions{Width: 300, Height: 200, TitleBarStyle: style, TitleBarHeight: 52, Content: ui.View(view)})
	t.Cleanup(w.Destroy)
	wins := fb.Windows()
	fw := wins[len(wins)-1]
	s := fw.FakeSurface()
	onMain(func() {
		w.native = &contentTitleBarWindow{Window: w.native, t: t}
		s.Frame()
	})
	if bar != (ui.TitleBar{Height: 52, Right: 138}) {
		t.Errorf("TitleBar = %+v", bar)
	}
	var framed bool
	onMain(func() {
		fw.TitleBarRoom = platform.TitleBar{Height: 46, Left: 80}
		fw.H.TitleBarChanged()
		framed = s.Frame()
	})
	if !framed || bar != (ui.TitleBar{Height: 46, Left: 80}) {
		t.Errorf("after a change: frame %v, TitleBar = %+v", framed, bar)
	}
	// Full screen removes the controls, and leaving it restores them.
	// Native UI must hear both changes without queuing events for a page
	// that does not exist (or asking the backend for its webview's zoom).
	for _, room := range []platform.TitleBar{{}, {Height: 46, Left: 80}} {
		onMain(func() {
			fw.TitleBarRoom = room
			fw.H.TitleBarChanged()
			framed = s.Frame()
		})
		if !framed || bar != (ui.TitleBar{Height: float32(room.Height), Left: float32(room.Left), Right: float32(room.Right)}) {
			t.Errorf("after a full screen change: frame %v, TitleBar = %+v, want %+v", framed, bar, room)
		}
	}
	w.outMu.Lock()
	queued := len(w.held) + len(w.outbox)
	w.outMu.Unlock()
	if queued != 0 || len(fw.Scripts()) != 0 {
		t.Errorf("title bar changes reached a page: %d queued events, scripts %q", queued, fw.Scripts())
	}

	// A window with its title bar has no room to keep clear of.
	contentWindow(t, view)
	if bar != (ui.TitleBar{}) {
		t.Errorf("TitleBar of a window with a title bar = %+v", bar)
	}
}

// TestContentFileDrop checks that files dropped on native UI go to the
// element that takes them, and the others to OnFileDrop, when it has
// listeners.
func TestContentFileDrop(t *testing.T) {
	var zone []string
	view := func(c *ui.Context) {
		if files := ui.Box(c).Size(100, 50).DroppedFiles(); files != nil {
			zone = files
		}
	}
	w, _, s := contentWindow(t, view)
	send := func(ev platform.SurfaceEvent) (taken bool) {
		onMain(func() {
			taken = s.Send(ev)
			s.Frame()
		})
		return taken
	}
	if send(platform.SurfaceEvent{Kind: platform.FileDragOver, X: 200, Y: 150}) {
		t.Error("a window without OnFileDrop listeners took files outside of its drop zone")
	}
	if !send(platform.SurfaceEvent{Kind: platform.FileDragOver, X: 20, Y: 20}) {
		t.Error("the drop zone did not take files")
	}
	var events []*FileDropEvent
	w.OnFileDrop(func(e *FileDropEvent) { events = append(events, e) })
	if !send(platform.SurfaceEvent{Kind: platform.FileDragOver, X: 200, Y: 150}) {
		t.Error("a window with OnFileDrop listeners did not take files")
	}
	if !send(platform.SurfaceEvent{Kind: platform.FileDrop, X: 200, Y: 150, Files: []string{"/a"}}) || len(events) != 1 ||
		events[0].Paths[0] != "/a" || events[0].X != 200 || events[0].Y != 150 || zone != nil {
		t.Errorf("dropped outside of the zone: events %+v, zone %q", events, zone)
	}
	if !send(platform.SurfaceEvent{Kind: platform.FileDrop, X: 20, Y: 20, Files: []string{"/b"}}) || len(events) != 1 || len(zone) != 1 || zone[0] != "/b" {
		t.Errorf("dropped on the zone: events %d, zone %q", len(events), zone)
	}
}

func TestContentTextInputTurnsOnIME(t *testing.T) {
	name := ""
	view := func(c *ui.Context) {
		ui.TextInput(c, &name).Absolute().Left(10).Top(10).Width(200)
	}
	_, _, s := contentWindow(t, view)
	onMain(func() {
		s.Send(platform.SurfaceEvent{Kind: platform.PointerDown, X: 30, Y: 25})
		s.Send(platform.SurfaceEvent{Kind: platform.PointerUp, X: 30, Y: 25})
		s.Frame()
		s.Send(platform.SurfaceEvent{Kind: platform.TextInput, Text: "héllo"})
		s.Frame()
	})
	if name != "héllo" {
		t.Errorf("typed %q", name)
	}
	if on, caret := s.TextInput(); !on || caret.X < 10 || caret.H <= 0 {
		t.Errorf("text input %v at %+v", on, caret)
	}
	if c := s.Cursor(); c != platform.CursorText {
		t.Errorf("cursor %v over the input", c)
	}
}

func TestContentFollowsPreferences(t *testing.T) {
	var prefs ui.Preferences
	var accent ui.Color
	_, _, s := contentWindow(t, func(c *ui.Context) {
		prefs, accent = c.Preferences(), c.Theme().Accent
	})
	t.Cleanup(func() { onMain(func() { fb.SetPreferences(platform.Preferences{}) }) })
	onMain(func() {
		fb.SetPreferences(platform.Preferences{Accent: platform.Color{R: 255, G: 128, A: 255}, ReduceMotion: true, TextScale: 1.25})
		s.Frame()
	})
	if !prefs.ReduceMotion || prefs.TextScale != 1.25 || accent != ui.RGB(255, 128, 0) {
		t.Errorf("the content sees %+v, accent %v", prefs, accent)
	}
}

func TestContentMenuRoles(t *testing.T) {
	name := "Ada"
	view := func(c *ui.Context) {
		ui.TextInput(c, &name).Absolute().Left(10).Top(10).Width(200)
	}
	w, fw, s := contentWindow(t, view)
	onMain(func() {
		s.Send(platform.SurfaceEvent{Kind: platform.PointerDown, X: 30, Y: 25})
		s.Send(platform.SurfaceEvent{Kind: platform.PointerUp, X: 30, Y: 25})
		s.Frame()
		// The page roles find no page; the edit roles edit the input.
		for _, role := range []MenuRole{RoleReload, RoleForceReload, RoleToggleDevTools, RoleZoomIn, RoleResetZoom} {
			performRole(role, w)
		}
		performRole(RoleSelectAll, w)
		s.Frame()
		performRole(RoleCut, w)
		s.Frame()
	})
	if fw.IsDevToolsOpened() || len(fw.Scripts()) != 0 {
		t.Errorf("page roles reached the window: devtools %v, scripts %q", fw.IsDevToolsOpened(), fw.Scripts())
	}
	if name != "" {
		t.Errorf("Select All and Cut left %q", name)
	}
	if text := Clipboard.ReadText(); text != "Ada" {
		t.Errorf("the clipboard has %q", text)
	}
}

func TestContentDuplicateKeyTellsWhere(t *testing.T) {
	view := func(c *ui.Context) {
		ui.Row(c.Key(1))
		ui.Row(c.Key(1))
	}
	var out bytes.Buffer
	log.SetOutput(&out)
	defer log.SetOutput(os.Stderr)
	_, _, _ = contentWindow(t, view)
	if s := out.String(); !strings.Contains(s, "key 1") || !strings.Contains(s, "content_test.go:") {
		t.Errorf("a duplicate key logs %q", s)
	}
}
