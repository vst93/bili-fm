package mygo

import (
	"testing"
	"time"
)

func TestPower(t *testing.T) {
	var got []string
	record := func(event string) func() { return func() { got = append(got, event) } }
	offs := []func(){
		Power.OnSuspend(record("suspend")),
		Power.OnResume(record("resume")),
		Power.OnLockScreen(record("lock")),
		Power.OnUnlockScreen(record("unlock")),
	}
	defer func() {
		for _, off := range offs {
			off()
		}
	}()
	deadline := time.Now().Add(time.Second)
	for !onMainValue(func() bool { return fb.Watching }) {
		if time.Now().After(deadline) {
			t.Fatal("the backend was not asked to watch power events")
		}
		time.Sleep(5 * time.Millisecond)
	}
	onMain(func() {
		for _, e := range []string{"suspend", "resume", "lock-screen", "unlock-screen", "unknown"} {
			fb.EmitPower(e)
		}
	})
	if want := "suspend resume lock unlock"; join(got) != want {
		t.Errorf("events = %q, want %q", join(got), want)
	}

	release := Power.KeepAwake("testing", true)
	if n := onMainValue(func() int { return fb.Awake }); n != 1 {
		t.Errorf("awake = %d", n)
	}
	release()
	release() // once
	if n := onMainValue(func() int { return fb.Awake }); n != 0 {
		t.Errorf("awake after release = %d", n)
	}
	if Power.IsOnBattery() || Power.IdleTime() != 42*time.Second {
		t.Error("IsOnBattery or IdleTime")
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += " "
		}
		out += x
	}
	return out
}

func TestDockMenuAndRelaunch(t *testing.T) {
	m := NewMenu([]*MenuItem{{Label: "New Window", ID: "new"}})
	App.Dock.SetMenu(m)
	if App.Dock.Menu() != m {
		t.Error("Dock.Menu")
	}
	dm := onMainValue(func() any { return fb.DockMenu })
	if dm == nil || len(fb.DockMenu.Items) != 1 || fb.DockMenu.Items[0].Label != "New Window" {
		t.Errorf("dock menu = %+v", dm)
	}
	App.Dock.SetMenu(nil)
	if onMainValue(func() bool { return fb.DockMenu != nil }) {
		t.Error("SetMenu(nil) kept the menu")
	}

	// A relaunch whose quit is canceled does nothing.
	canceled := make(chan struct{}, 1)
	off := App.OnBeforeQuit(func(e *QuitEvent) {
		e.PreventDefault()
		canceled <- struct{}{}
	})
	defer off()
	App.Relaunch()
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("Relaunch did not start quitting")
	}
	if onMainValue(func() bool { return App.relaunch }) {
		t.Error("a canceled relaunch is still pending")
	}
}

func TestPermissionHandler(t *testing.T) {
	w, fw := testWindow(t, WindowOptions{})
	ask := func(kinds []string, origin string) bool {
		return onMainValue(func() bool { return fw.H.PermissionRequested(kinds, origin) })
	}
	// By default the app's own pages get what they ask for.
	if !ask([]string{"camera"}, "mygo://localhost") || ask([]string{"camera"}, "https://evil.example") {
		t.Error("default permission decisions")
	}
	var got PermissionRequest
	w.Page().SetPermissionHandler(func(req PermissionRequest) bool {
		got = req
		return req.Origin == "https://meet.example"
	})
	if !ask([]string{"camera", "microphone"}, "https://meet.example") || ask([]string{"geolocation"}, "mygo://localhost") {
		t.Error("the handler's decisions were not applied")
	}
	if got.Origin != "mygo://localhost" || len(got.Permissions) != 1 || got.Permissions[0] != PermissionGeolocation {
		t.Errorf("last request = %+v", got)
	}
	w.Page().SetPermissionHandler(nil)
	if !ask([]string{"camera"}, "mygo://localhost") {
		t.Error("SetPermissionHandler(nil) did not restore the default")
	}
}

func TestClearBrowsingDataFake(t *testing.T) {
	before := onMainValue(func() int { return fb.Cleared })
	if err := App.ClearBrowsingData(); err != nil {
		t.Fatal(err)
	}
	if onMainValue(func() int { return fb.Cleared }) != before+1 {
		t.Error("the backend did not clear")
	}
}
