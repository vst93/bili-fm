package updater

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/updater/internal/markdown"
)

// harness runs the updater without windows: its open function records the sessions
// that would show one.
type harness struct {
	t          *testing.T
	u          *updater
	dir        string
	presented  chan *session
	relaunches atomic.Int32
	checks     atomic.Int32
	// result is what checks find.
	result func(ctx context.Context) (*release, error)
}

func newHarness(t *testing.T, opts Options) *harness {
	h := &harness{t: t, dir: t.TempDir(), presented: make(chan *session, 10)}
	saved := []func(){
		func(e func() bool) func() { return func() { enabled = e } }(enabled),
		func(c func(context.Context) (*release, error)) func() { return func() { check = c } }(check),
		func(r func()) func() { return func() { relaunch = r } }(relaunch),
		func(v func() string) func() { return func() { appVersion = v } }(appVersion),
		func(l func() string) func() { return func() { locale = l } }(locale),
		func(d func() (string, error)) func() { return func() { stateDir = d } }(stateDir),
		func(r func(func())) func() { return func() { runOnMain = r } }(runOnMain),
	}
	t.Cleanup(func() {
		for _, restore := range saved {
			restore()
		}
	})
	enabled = func() bool { return true }
	h.result = func(context.Context) (*release, error) { return nil, nil }
	check = func(ctx context.Context) (*release, error) {
		h.checks.Add(1)
		return h.result(ctx)
	}
	relaunch = func() { h.relaunches.Add(1) }
	appVersion = func() string { return "1.0.0" }
	locale = func() string { return "en-US" }
	stateDir = func() (string, error) { return h.dir, nil }
	runOnMain = func(fn func()) { fn() }
	h.u = newUpdater(opts, func(s *session) { h.presented <- s })
	return h
}

// begin starts a check and returns a channel closed when it ends.
func (h *harness) begin(user bool) chan struct{} {
	done := make(chan struct{})
	go func() {
		h.u.begin(user)
		close(done)
	}()
	return done
}

// shown returns the session that shows a window.
func (h *harness) shown() *session {
	h.t.Helper()
	select {
	case s := <-h.presented:
		return s
	case <-time.After(5 * time.Second):
		h.t.Fatal("no window was shown")
		return nil
	}
}

func (h *harness) noWindow() {
	h.t.Helper()
	select {
	case s := <-h.presented:
		v, _ := s.Current()
		h.t.Fatalf("a window was shown: %q", v.Title)
	default:
	}
}

// waitView waits for s to show a view whose title starts with title.
func waitView(t *testing.T, s *session, title string) view {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		v, changed := s.Current()
		if strings.HasPrefix(v.Title, title) {
			return v
		}
		select {
		case <-changed:
		case <-deadline:
			t.Fatalf("view %q, want %q", v.Title, title)
		}
	}
}

func waitDone(t *testing.T, done chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the check did not end")
	}
}

func (h *harness) saved() state {
	h.t.Helper()
	var st state
	data, err := os.ReadFile(filepath.Join(h.dir, stateFile))
	if err != nil {
		h.t.Fatal(err)
	}
	if err := json.Unmarshal(data, &st); err != nil {
		h.t.Fatal(err)
	}
	return st
}

func newRelease(version string, install func(ctx context.Context, progress func(downloaded, total int64)) error) *release {
	if install == nil {
		install = func(context.Context, func(int64, int64)) error { return nil }
	}
	return &release{version: version, notes: "### Fixed\n\n- A **bug**", install: install}
}

func TestUpToDate(t *testing.T) {
	h := newHarness(t, Options{})
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "You’re up to date!")
	if !strings.Contains(v.Message, "1.0.0 is currently the newest version") {
		t.Errorf("message %q", v.Message)
	}
	s.Respond(v.Prompt, actionOK, false)
	waitDone(t, done)
	if h.u.session != nil {
		t.Error("the session was not cleared")
	}
	if st := h.saved(); time.Since(st.LastCheck) > time.Minute {
		t.Errorf("last check %v", st.LastCheck)
	}
}

func TestInstallAndRelaunch(t *testing.T) {
	h := newHarness(t, Options{})
	proceed := make(chan struct{})
	h.result = func(context.Context) (*release, error) {
		return newRelease("2.0.0", func(ctx context.Context, progress func(int64, int64)) error {
			progress(5e6, 20e6)
			<-proceed
			progress(20e6, 20e6)
			return nil
		}), nil
	}
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "A new version of")
	if !v.Release || !v.Checkbox || v.Checked || !strings.Contains(markdown.HTML(v.Notes), "<li>A <strong>bug</strong></li>") {
		t.Errorf("available view: %+v", v)
	}
	if !strings.Contains(v.Message, "2.0.0 is now available—you have 1.0.0") {
		t.Errorf("message %q", v.Message)
	}
	if len(v.Buttons) != 3 || v.Buttons[0].Action != actionSkip || !v.Buttons[0].Aside || !v.Buttons[2].Default {
		t.Errorf("buttons %+v", v.Buttons)
	}
	s.Respond(v.Prompt, actionInstall, true)

	v = waitView(t, s, "Downloading update…")
	for v.Progress != 0.25 {
		_, changed := s.Current()
		<-changed
		v, _ = s.Current()
	}
	if v.Message != "5.0 MB of 20.0 MB" || !v.Bar || v.Release {
		t.Errorf("downloading view: %+v", v)
	}
	close(proceed)
	v = waitView(t, s, "Ready to Relaunch")
	if !strings.Contains(v.Message, "2.0.0 is installed") {
		t.Errorf("message %q", v.Message)
	}
	s.Respond(v.Prompt, actionRelaunch, false)
	waitDone(t, done)
	if h.relaunches.Load() != 1 {
		t.Error("the app did not relaunch")
	}
	if !h.saved().AutomaticDownloads {
		t.Error("the checkbox was not saved")
	}

	// The update runs at the next launch: checks do not offer it again.
	checks := h.checks.Load()
	waitDone(t, h.begin(false))
	h.noWindow()
	done = h.begin(true)
	s = h.shown()
	v = waitView(t, s, "Ready to Relaunch")
	s.Respond(v.Prompt, actionLater, false)
	waitDone(t, done)
	if h.checks.Load() != checks {
		t.Error("checked again after installing")
	}
	if h.relaunches.Load() != 1 {
		t.Error("Later relaunched")
	}
}

func TestSkipVersion(t *testing.T) {
	h := newHarness(t, Options{})
	h.result = func(context.Context) (*release, error) { return newRelease("2.0.0", nil), nil }
	done := h.begin(false)
	s := h.shown()
	if s.User() {
		t.Error("a background check counts as the user's")
	}
	v := waitView(t, s, "A new version of")
	s.Respond(v.Prompt, actionSkip, false)
	waitDone(t, done)
	if got := h.saved().SkippedVersion; got != "2.0.0" {
		t.Fatalf("skipped %q", got)
	}

	// Background checks leave it alone,
	waitDone(t, h.begin(false))
	h.noWindow()
	// the user still sees it,
	done = h.begin(true)
	s = h.shown()
	v = waitView(t, s, "A new version of")
	s.Respond(v.Prompt, actionLater, false)
	waitDone(t, done)
	// and a newer version is offered again.
	h.result = func(context.Context) (*release, error) { return newRelease("2.0.1", nil), nil }
	done = h.begin(false)
	s = h.shown()
	waitView(t, s, "A new version of")
	s.close()
	waitDone(t, done)
}

func TestAutomaticDownloads(t *testing.T) {
	h := newHarness(t, Options{})
	h.u.update(func(s *state) { s.AutomaticDownloads = true })
	installed := make(chan struct{})
	h.result = func(context.Context) (*release, error) {
		return newRelease("2.0.0", func(context.Context, func(int64, int64)) error {
			close(installed)
			return nil
		}), nil
	}
	waitDone(t, h.begin(false))
	<-installed
	h.noWindow()
	if h.u.installed == nil {
		t.Fatal("not installed")
	}
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "Ready to Relaunch")
	s.Respond(v.Prompt, actionRelaunch, false)
	waitDone(t, done)
	if h.relaunches.Load() != 1 || h.checks.Load() != 1 {
		t.Errorf("relaunches %d, checks %d", h.relaunches.Load(), h.checks.Load())
	}
}

func TestCancelDownload(t *testing.T) {
	h := newHarness(t, Options{})
	canceled := make(chan error, 1)
	h.result = func(context.Context) (*release, error) {
		return newRelease("2.0.0", func(ctx context.Context, progress func(int64, int64)) error {
			progress(1, 10)
			<-ctx.Done()
			canceled <- ctx.Err()
			return ctx.Err()
		}), nil
	}
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "A new version of")
	s.Respond(v.Prompt, actionInstall, false)
	v = waitView(t, s, "Downloading update…")
	s.Respond(v.Prompt, actionCancel, false)
	waitDone(t, done)
	if err := <-canceled; !errors.Is(err, context.Canceled) {
		t.Errorf("install ended with %v", err)
	}
	if h.u.installed != nil {
		t.Error("a canceled update counts as installed")
	}
}

func TestCheckErrors(t *testing.T) {
	h := newHarness(t, Options{})
	h.result = func(context.Context) (*release, error) { return nil, errors.New("offline") }
	waitDone(t, h.begin(false))
	h.noWindow()
	if h.u.failed.IsZero() {
		t.Error("the failure was not recorded")
	}

	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "Update Error!")
	if v.Detail != "offline" {
		t.Errorf("detail %q", v.Detail)
	}
	s.Respond(v.Prompt, actionOK, false)
	waitDone(t, done)

	h.result = func(context.Context) (*release, error) {
		return newRelease("2.0.0", func(context.Context, func(int64, int64)) error {
			return errors.New("bad signature")
		}), nil
	}
	done = h.begin(true)
	s = h.shown()
	v = waitView(t, s, "A new version of")
	s.Respond(v.Prompt, actionInstall, false)
	v = waitView(t, s, "Update Error!")
	if v.Detail != "bad signature" || !strings.Contains(v.Message, "installing") {
		t.Errorf("error view %+v", v)
	}
	s.Respond(v.Prompt, actionOK, false)
	waitDone(t, done)
}

func TestUnavailable(t *testing.T) {
	h := newHarness(t, Options{})
	enabled = func() bool { return false }
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "Updates Unavailable")
	s.Respond(v.Prompt, actionOK, false)
	waitDone(t, done)
	if h.checks.Load() != 0 {
		t.Error("checked for updates")
	}
}

func TestResponses(t *testing.T) {
	h := newHarness(t, Options{})
	h.result = func(context.Context) (*release, error) { return newRelease("2.0.0", nil), nil }
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "A new version of")
	s.Respond(v.Prompt-1, actionSkip, false)   // an earlier prompt
	s.Respond(v.Prompt, actionRelaunch, false) // no such button
	s.Respond(v.Prompt, actionLater, false)    // counts
	s.Respond(v.Prompt, actionSkip, false)     // a second response
	waitDone(t, done)
	if st := h.saved(); st.SkippedVersion != "" {
		t.Errorf("skipped %q", st.SkippedVersion)
	}
}

func TestPromote(t *testing.T) {
	h := newHarness(t, Options{})
	proceed := make(chan struct{})
	h.result = func(context.Context) (*release, error) {
		<-proceed
		return nil, nil
	}
	done := h.begin(false)
	for h.checks.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	h.noWindow()
	waitDone(t, h.begin(true)) // returns at once
	s := h.shown()
	waitView(t, s, "Checking for updates…")
	close(proceed)
	v := waitView(t, s, "You’re up to date!")
	s.Respond(v.Prompt, actionOK, false)
	waitDone(t, done)
	if h.checks.Load() != 1 {
		t.Errorf("%d checks", h.checks.Load())
	}
}

func TestCancelCheck(t *testing.T) {
	h := newHarness(t, Options{})
	h.result = func(ctx context.Context) (*release, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	done := h.begin(true)
	s := h.shown()
	v := waitView(t, s, "Checking for updates…")
	s.Respond(v.Prompt, actionCancel, false)
	waitDone(t, done)
	if !h.u.failed.IsZero() {
		t.Error("canceling counts as a failure")
	}
}

func TestPreferences(t *testing.T) {
	h := newHarness(t, Options{DisableAutomaticChecks: true})
	active.Store(h.u)
	t.Cleanup(func() { active.Store(nil) })
	if AutomaticChecks() || AutomaticDownloads() || !LastCheck().IsZero() {
		t.Fatal("wrong defaults")
	}
	SetAutomaticChecks(true)
	SetAutomaticDownloads(true)
	if !AutomaticChecks() || !AutomaticDownloads() {
		t.Fatal("not set")
	}
	// Another run reads them back.
	u := newUpdater(Options{DisableAutomaticChecks: true}, nil)
	u.load()
	if !u.automaticChecks() || !u.state.AutomaticDownloads {
		t.Errorf("state %+v", u.state)
	}
	if u := newUpdater(Options{}, nil); u.opts.Interval != 24*time.Hour {
		t.Errorf("interval %v", u.opts.Interval)
	}
}

func TestOnChange(t *testing.T) {
	h := newHarness(t, Options{})
	active.Store(h.u)
	t.Cleanup(func() { active.Store(nil) })
	changes := make(chan struct{}, 10)
	off := OnChange(func() { changes <- struct{}{} })
	told := func(want bool, what string) {
		t.Helper()
		wait := 50 * time.Millisecond
		if want {
			wait = 5 * time.Second
		}
		select {
		case <-changes:
			if !want {
				t.Errorf("%s: told of a change", what)
			}
		case <-time.After(wait):
			if want {
				t.Errorf("%s: not told", what)
			}
		}
	}

	SetAutomaticDownloads(true)
	told(true, "SetAutomaticDownloads")
	SetAutomaticDownloads(true)
	told(false, "the same value")

	waitDone(t, h.begin(false))
	told(true, "a check")
	h.result = func(context.Context) (*release, error) { return nil, errors.New("offline") }
	waitDone(t, h.begin(false))
	told(false, "a failed check")

	h.result = func(context.Context) (*release, error) { return newRelease("2.0.0", nil), nil }
	done := h.begin(true)
	s := h.shown()
	told(true, "the user's check")
	v := waitView(t, s, "A new version of")
	s.Respond(v.Prompt, actionSkip, true)
	waitDone(t, done)
	told(true, "Skip This Version")

	off()
	SetAutomaticDownloads(false)
	told(false, "a function removed")
}

func TestSchedule(t *testing.T) {
	h := newHarness(t, Options{})
	h.u.running = true
	h.u.schedule()
	if h.u.timer == nil {
		t.Fatal("no check scheduled")
	}
	h.u.update(func(s *state) { f := false; s.AutomaticChecks = &f })
	h.u.schedule()
	if h.u.timer != nil {
		t.Error("scheduled with automatic checks off")
	}
}

func TestNextCheck(t *testing.T) {
	now := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	for _, tc := range []struct {
		name         string
		last, failed time.Time
		interval     time.Duration
		want         time.Time
	}{
		{"never checked", time.Time{}, time.Time{}, day, time.Time{}.Add(day)},
		{"checked an hour ago", now.Add(-time.Hour), time.Time{}, day, now.Add(23 * time.Hour)},
		{"due check failed", now.Add(-2 * day), now.Add(-time.Minute), day, now.Add(59 * time.Minute)},
		{"a check of the user failed", now.Add(-time.Hour), now.Add(-time.Minute), day, now.Add(23 * time.Hour)},
		{"short interval", now.Add(-time.Hour), now, 10 * time.Minute, now.Add(10 * time.Minute)},
		{"clock set back", now.Add(time.Hour), time.Time{}, day, now.Add(day)},
	} {
		if got := nextCheck(tc.last, tc.failed, tc.interval, now); !got.Equal(tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestPage(t *testing.T) {
	fr := newText("fr-FR", map[string]Strings{"fr": {ReleaseNotes: "<b>Notes</b>"}})
	v := view{Title: "</script><script>alert(1)</script>", Notes: markdown.Parse("A **bug**"), Buttons: fr.ok()}
	p := page(fr.texts(), "data:image/png;base64,AAAA", v)
	if strings.Contains(p, "{{") {
		t.Error("a placeholder was left")
	}
	if strings.Contains(p, "</script><script>alert") {
		t.Error("the view is not escaped")
	}
	i := strings.Index(p, "'nonce-")
	nonce := p[i+7 : i+7+strings.IndexByte(p[i+7:], '\'')]
	if len(nonce) < 20 || !strings.Contains(p, `<script nonce="`+nonce+`">`) {
		t.Errorf("nonce %q", nonce)
	}
	if !strings.Contains(p, `src="data:image/png;base64,AAAA"`) {
		t.Error("no icon")
	}
	for _, want := range []string{
		`<html lang="fr" dir="ltr">`, "<title>Mise à jour de logiciels</title>", "&lt;b&gt;Notes&lt;/b&gt;",
		`"notes":"\u003cp\u003eA \u003cstrong\u003ebug\u003c/strong\u003e\u003c/p\u003e"`, // the notes in HTML, escaped
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the page lacks %s", want)
		}
	}
}
