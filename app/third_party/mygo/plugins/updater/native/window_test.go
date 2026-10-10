package native

import (
	"bytes"
	"image"
	"image/png"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater/internal/frontend"
	"github.com/egoist/mygo/plugins/updater/internal/markdown"
	"github.com/egoist/mygo/ui"
)

// session is a frontend.Session that records what the window does.
type session struct {
	texts frontend.Texts
	icon  []byte

	mu        sync.Mutex
	view      frontend.View
	changed   chan struct{}
	responses []response
	fits      chan [2]int
}

type response struct {
	prompt    int
	action    frontend.Action
	automatic bool
}

func newSession(v frontend.View) *session {
	return &session{
		texts: frontend.Texts{
			Title: "Software Update", ReleaseNotes: "Release Notes:",
			AutomaticDownloads: "Automatically download and install updates in the future", Lang: "en",
		},
		view: v, changed: make(chan struct{}), fits: make(chan [2]int, 100),
	}
}

func (s *session) NewWindow(mygo.Content) *mygo.Window { panic("no windows in tests") }
func (s *session) Done() <-chan struct{}               { return nil }
func (s *session) Texts() frontend.Texts               { return s.texts }
func (s *session) Icon() []byte                        { return s.icon }
func (s *session) User() bool                          { return true }
func (s *session) Fit(width, height int)               { s.fits <- [2]int{width, height} }

func (s *session) Current() (frontend.View, <-chan struct{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.view, s.changed
}

func (s *session) set(v frontend.View) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.view = v
	close(s.changed)
	s.changed = make(chan struct{})
}

func (s *session) Respond(prompt int, a frontend.Action, automatic bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.responses = append(s.responses, response{prompt, a, automatic})
}

func (s *session) answers() []response {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.responses)
}

// fit returns the last size the window asked for.
func (s *session) fit(t *testing.T) [2]int {
	t.Helper()
	var size [2]int
	select {
	case size = <-s.fits:
	case <-time.After(5 * time.Second):
		t.Fatal("the window did not ask for a size")
	}
	for {
		select {
		case size = <-s.fits:
		case <-time.After(50 * time.Millisecond):
			return size
		}
	}
}

func available(prompt int, notes string) frontend.View {
	return frontend.View{
		Prompt:   prompt,
		Release:  true,
		Title:    "A new version of App is available!",
		Message:  "App 2.0 is now available—you have 1.0. Would you like to install it now?",
		Notes:    markdown.Parse(notes),
		Checkbox: true,
		Buttons: []frontend.Button{
			{Action: frontend.Skip, Label: "Skip This Version", Aside: true},
			{Action: frontend.Later, Label: "Remind Me Later", Cancel: true},
			{Action: frontend.Install, Label: "Install Update", Default: true},
		},
	}
}

func status(prompt int, title, message string) frontend.View {
	return frontend.View{Prompt: prompt, Title: title, Message: message, Buttons: []frontend.Button{
		{Action: frontend.OK, Label: "OK", Default: true, Cancel: true},
	}}
}

func TestAvailable(t *testing.T) {
	s := newSession(available(1, "### Fixed\n\n- [The editor](https://example.com/editor) lost a **bug**\n  - nested\n\n```\ncode\n```"))
	w := newWindow(s)
	tt := ui.NewTester(w.view, frontend.ReleaseWidth, frontend.ReleaseHeight)
	for _, want := range []string{"A new version of App is available!", "Release Notes:", "Fixed", "The editor lost a bug", "nested", "code", "•", "◦", s.texts.AutomaticDownloads} {
		if !tt.HasText(want) {
			t.Errorf("the window lacks %q: %q", want, tt.Texts())
		}
	}
	// The link of the notes opens in the browser.
	r, _ := tt.Find("The editor lost a bug")
	tt.ClickAt(r.X+10, r.Y+r.H/2)
	if got := tt.OpenedURLs(); !slices.Equal(got, []string{"https://example.com/editor"}) {
		t.Errorf("opened %v", got)
	}
	// The keyboard reaches it, and Enter opens it rather than installing.
	tt.Key(0, ui.KeyTab)
	if !tt.Focused(s.texts.AutomaticDownloads) {
		t.Fatal("Tab does not go from the link to the checkbox")
	}
	tt.Key(ui.Shift, ui.KeyTab)
	if !tt.Focused("The editor") {
		t.Fatal("Shift+Tab does not go back to the link of the notes")
	}
	tt.Key(0, ui.KeyEnter)
	if got := tt.OpenedURLs(); len(got) != 2 || len(s.answers()) != 0 {
		t.Fatalf("Enter on the link opened %v and answered %v", got, s.answers())
	}

	if err := tt.Click(s.texts.AutomaticDownloads); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Install Update"); err != nil {
		t.Fatal(err)
	}
	// The answer disables the buttons until the next prompt.
	tt.Click("Remind Me Later")
	tt.Key(0, ui.KeyEscape)
	if got := s.answers(); !slices.Equal(got, []response{{1, frontend.Install, true}}) {
		t.Fatalf("responses %v", got)
	}

	// A new prompt resets the checkbox, and takes an answer again.
	s.set(available(2, "Notes"))
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if got := s.answers(); !slices.Equal(got[1:], []response{{2, frontend.Later, false}}) {
		t.Fatalf("responses %v", got)
	}
}

func TestKeys(t *testing.T) {
	s := newSession(available(1, ""))
	tt := ui.NewTester(newWindow(s).view, frontend.StatusWidth, 200)
	tt.Key(0, ui.KeyEnter)
	if got := s.answers(); !slices.Equal(got, []response{{1, frontend.Install, false}}) {
		t.Fatalf("Enter answered %v", got)
	}
	s.set(available(2, ""))
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	if got := s.answers()[1:]; !slices.Equal(got, []response{{2, frontend.Later, false}}) {
		t.Fatalf("Escape answered %v", got)
	}
	// Disabled buttons do not answer.
	v := status(3, "Installing update…", "")
	v.Buttons = []frontend.Button{{Action: frontend.Cancel, Label: "Cancel", Cancel: true, Disabled: true}}
	s.set(v)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Click("Cancel")
	if got := s.answers(); len(got) != 2 {
		t.Fatalf("a disabled button answered %v", got[2:])
	}
}

func TestProgress(t *testing.T) {
	s := newSession(frontend.View{
		Prompt: 1, Title: "Downloading update…", Message: "5.0 MB of 20.0 MB", Bar: true, Progress: 0.25,
		Buttons: []frontend.Button{{Action: frontend.Cancel, Label: "Cancel", Cancel: true}},
	})
	tt := ui.NewTester(newWindow(s).view, frontend.StatusWidth, frontend.StatusHeight)
	if !tt.HasText("5.0 MB of 20.0 MB") {
		t.Errorf("texts %q", tt.Texts())
	}
	if err := tt.Click("Cancel"); err != nil {
		t.Fatal(err)
	}
	if got := s.answers(); !slices.Equal(got, []response{{1, frontend.Cancel, false}}) {
		t.Fatalf("responses %v", got)
	}
}

func TestFit(t *testing.T) {
	s := newSession(status(1, "You’re up to date!", "App 1.0 is currently the newest version available."))
	tt := ui.NewTester(newWindow(s).view, frontend.StatusWidth, frontend.StatusHeight)
	short := s.fit(t)
	if short[0] > frontend.StatusWidth || short[1] < 60 || short[1] > frontend.StatusHeight+40 {
		t.Errorf("a short status asks for %v", short)
	}
	// A longer text asks for more height, at the window's width.
	v := status(2, "Update Error!", "An error occurred while checking for updates. Please try again later.")
	v.Detail = strings.Repeat("dial tcp: lookup updates.example.com: no such host ", 4)
	s.set(v)
	tt.Frame()
	long := s.fit(t)
	if long[1] <= short[1] {
		t.Errorf("an error asks for %v, a short status for %v", long, short)
	}
	// The session widens the window to the usual width at least.
	tt.SetSize(max(long[0], frontend.StatusWidth), long[1])
	select {
	case got := <-s.fits:
		t.Errorf("the window asks for %v once it has the size it asked for, %v", got, long)
	case <-time.After(100 * time.Millisecond):
	}

	// Long labels widen the window.
	v = available(3, "Notes")
	for i := range v.Buttons {
		v.Buttons[i].Label = strings.Repeat(v.Buttons[i].Label+" ", 3)
	}
	s.set(v)
	tt.SetSize(frontend.ReleaseWidth, frontend.ReleaseHeight)
	if wide := s.fit(t); wide[0] <= frontend.ReleaseWidth {
		t.Errorf("long labels ask for %v", wide)
	}
}

func TestLayout(t *testing.T) {
	var icon bytes.Buffer
	png.Encode(&icon, image.NewRGBA(image.Rect(0, 0, 128, 128)))
	for _, rtl := range []bool{false, true} {
		s := newSession(available(1, "- An English item"))
		s.icon, s.texts.RTL = icon.Bytes(), rtl
		tt := ui.NewTester(newWindow(s).view, frontend.ReleaseWidth, frontend.ReleaseHeight)
		find := func(text string) ui.Rect {
			t.Helper()
			r, ok := tt.Find(text)
			if !ok {
				t.Fatalf("no %q", text)
			}
			return r
		}
		// The title, stretched across the column of texts, which the
		// icon leaves.
		body := find("A new version of App is available!")
		skip, later, install := find("Skip This Version"), find("Remind Me Later"), find("Install Update")
		// The checkbox's box sits between its label and the edge of the
		// texts' column where they start.
		label := find(s.texts.AutomaticDownloads)
		left, right := label.X-body.X, body.X+body.W-(label.X+label.W)
		bullet, item := find("•"), find("An English item")
		if !rtl {
			if body.X < padSide+iconSize+iconGap || skip.X > later.X || later.X > install.X {
				t.Errorf("left to right: texts at %v, Skip at %v, Later at %v, Install at %v", body, skip, later, install)
			}
			if left < 4 || left > right {
				t.Errorf("left to right: the checkbox's label at %v in %v", label, body)
			}
		} else {
			if body.X+body.W > frontend.ReleaseWidth-padSide-iconSize-iconGap+1 || skip.X < later.X || later.X < install.X {
				t.Errorf("right to left: texts at %v, Skip at %v, Later at %v, Install at %v", body, skip, later, install)
			}
			if right < 4 || right > left {
				t.Errorf("right to left: the checkbox's label at %v in %v", label, body)
			}
		}
		// Notes take the direction of their language, not the window's.
		if bullet.X > item.X {
			t.Errorf("rtl %v: the bullet of English notes at %v, the item at %v", rtl, bullet, item)
		}
		s.set(available(2, "- פריט בעברית"))
		tt.Frame()
		if bullet, item := find("•"), find("פריט בעברית"); bullet.X < item.X {
			t.Errorf("rtl %v: the bullet of Hebrew notes at %v, the item at %v", rtl, bullet, item)
		}
	}
}
