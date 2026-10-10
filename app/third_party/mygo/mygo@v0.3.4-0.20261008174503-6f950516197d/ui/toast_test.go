package ui

import (
	"slices"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestToastViewportBase(t *testing.T) {
	var shown []Toast
	var lefts []time.Duration
	var closed, undone []string
	tt, now := clockTester(func(c *context) {
		shown, lefts = shown[:0], lefts[:0]
		if coreButton(c, "Delete").Clicked() {
			c.AddToast(Toast{ID: "deleted", Title: "Note deleted", Action: "Undo", Type: "info",
				OnAction: func() { undone = append(undone, "deleted") },
				OnClose:  func() { closed = append(closed, "deleted") }})
		}
		coreToastViewportBase(c, func(viewport *node, toasts []Toast) {
			shown = append(shown, toasts...)
			viewport.Padding(16).AlignItems(End).Gap(8)
			for _, t := range toasts {
				toast := coreToastBase(c, t)
				lefts = append(lefts, toast.Left())
				toast.Root.Row().Gap(8).Children(func() {
					coreText(c, t.Title)
					if t.Action != "" {
						toast.ActionButton().Children(func() { coreText(c, t.Action) })
					}
					toast.CloseButton().Label("Close").Children(func() { coreText(c, "×") })
				})
			}
		})
	}, 400, 300)
	tick := ticker(tt, now)
	tt.Click("Delete")
	if len(shown) != 1 || shown[0].Type != "info" || !tt.HasText("Note deleted") {
		t.Fatalf("toasts %v, texts %q", shown, tt.Texts())
	}
	if lefts[0] != toastTime {
		t.Errorf("a toast without a Timeout shows for %v", lefts[0])
	}
	// The theme's look does not show them too.
	if n := len(slices.DeleteFunc(tt.Texts(), func(s string) bool { return s != "Note deleted" })); n != 1 {
		t.Errorf("the toast shows %d times", n)
	}
	r, _ := tt.Find("Note deleted")
	if r.X+r.W < 300 || r.Y < 250 {
		t.Errorf("the viewport did not go to the bottom right: %v", r)
	}
	tick(time.Second)
	if lefts[0] != toastTime-time.Second {
		t.Errorf("a second later, %v left", lefts[0])
	}
	// Its action runs, and closes it.
	tt.Click("Undo")
	if !slices.Equal(undone, []string{"deleted"}) || !slices.Equal(closed, []string{"deleted"}) || tt.HasText("Note deleted") {
		t.Errorf("after Undo: undone %v, closed %v, texts %q", undone, closed, tt.Texts())
	}
	// Its time over, it closes.
	tt.Click("Delete")
	tick(toastTime)
	if len(closed) != 2 || tt.HasText("Note deleted") {
		t.Errorf("after its time: closed %v, texts %q", closed, tt.Texts())
	}
	// Close, and Escape with the focus in it, close it.
	tt.Click("Delete")
	tt.Click("Close")
	if len(closed) != 3 || tt.HasText("Note deleted") {
		t.Errorf("after Close: closed %v, texts %q", closed, tt.Texts())
	}
	tt.Click("Delete")
	for range 4 {
		if tt.Focused("Undo") {
			break
		}
		tt.Key(0, KeyTab)
	}
	if !tt.Focused("Undo") {
		t.Fatal("Tab does not reach the toast")
	}
	tt.Key(0, KeyEscape)
	if len(closed) != 4 || tt.HasText("Note deleted") {
		t.Errorf("after Escape: closed %v, texts %q", closed, tt.Texts())
	}
}

func TestToastManager(t *testing.T) {
	var shown []string
	var lefts []time.Duration
	var c *context
	tt, now := clockTester(func(ctx *context) {
		c = ctx
		shown, lefts = shown[:0], lefts[:0]
		coreToastViewportBase(c, func(viewport *node, toasts []Toast) {
			for _, t := range toasts {
				toast := coreToastBase(c, t)
				shown, lefts = append(shown, t.Title), append(lefts, toast.Left())
				toast.Root.Children(func() { coreText(c, t.Title) })
			}
		})
	}, 400, 300)
	tick := ticker(tt, now)
	ids := map[string]string{}
	for _, s := range []string{"A", "B", "C", "D"} {
		ids[s] = c.AddToast(Toast{Title: s})
	}
	tt.Frame()
	// The newest three show, oldest first.
	if !slices.Equal(shown, []string{"B", "C", "D"}) {
		t.Errorf("four toasts show %q", shown)
	}
	if ids["A"] == ids["B"] || ids["A"] == "" {
		t.Errorf("ids %v", ids)
	}
	// Adding one with the ID of a toast showing changes it, where it is.
	tick(time.Second)
	c.AddToast(Toast{ID: ids["C"], Title: "C2"})
	tt.Frame()
	if !slices.Equal(shown, []string{"B", "C2", "D"}) || lefts[1] != toastTime {
		t.Errorf("after changing C: %q, %v left", shown, lefts)
	}
	c.CloseToast(ids["D"])
	tt.Frame()
	if !slices.Equal(shown, []string{"A", "B", "C2"}) {
		t.Errorf("after closing D: %q", shown)
	}
	c.CloseToast("")
	tt.Frame()
	if len(shown) != 0 || len(tt.rt.toasts) != 0 {
		t.Errorf("after closing them all: %q", shown)
	}
	// One with a negative Timeout stays.
	c.AddToast(Toast{Title: "Uploading", Timeout: -1})
	tick(time.Hour)
	if !slices.Equal(shown, []string{"Uploading"}) || lefts[0] != 0 {
		t.Errorf("a toast without end: %q, %v left", shown, lefts)
	}
	c.CloseToast("")
	// Their time stops while the pointer rests on them, and while the
	// window is in the background.
	c.AddToast(Toast{Title: "Saved"})
	tt.Frame()
	r, _ := tt.Find("Saved")
	tt.Move(r.X+1, r.Y+1)
	tick(time.Minute)
	if !slices.Equal(shown, []string{"Saved"}) {
		t.Fatalf("the pointer on the toast did not stop its time: %q", shown)
	}
	tt.Move(399, 1)
	tick(time.Second)
	tt.SetFocused(false)
	tick(time.Minute)
	if !slices.Equal(shown, []string{"Saved"}) || lefts[0] != toastTime-time.Second {
		t.Fatalf("the window in the background did not stop its time: %q, %v left", shown, lefts)
	}
	tt.SetFocused(true)
	tick(toastTime)
	if len(shown) != 0 {
		t.Errorf("the toast stays after its time: %q", shown)
	}
}

func TestToastDescription(t *testing.T) {
	var c *context
	tt := coreNewTester(func(ctx *context) { c = ctx }, 400, 300)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	c.AddToast(Toast{Title: "Upload failed", Description: "The server is not reachable."})
	tt.Frame()
	if !tt.HasText("Upload failed") || !tt.HasText("The server is not reachable.") {
		t.Errorf("texts %q", tt.Texts())
	}
	if a := tt.Announcements(); !slices.Equal(a, []string{"Upload failed", "The server is not reachable."}) {
		t.Errorf("announcements %q", a)
	}
}

func TestToastExits(t *testing.T) {
	var c *context
	red := RGB(255, 0, 0)
	tt, now := clockTester(func(ctx *context) {
		c = ctx
		coreToastViewportBase(c, func(viewport *node, toasts []Toast) {
			for _, t := range toasts {
				toast := coreToastBase(c, t)
				toast.Root.Size(100, 40).Background(red).
					Transition(ElementTransition{Duration: 200 * time.Millisecond, Ease: Linear, Exit: &Motion{Y: 40}})
			}
		})
	}, 200, 100)
	tick := ticker(tt, now)
	reddish := func() bool {
		px := tt.Image().RGBAAt(50, 80)
		return px.R > 200 && px.G < 100
	}
	id := c.AddToast(Toast{Title: "Saved"})
	tick(16 * time.Millisecond)
	if !reddish() {
		t.Fatal("the toast does not show")
	}
	// The last toast goes with its exit transition.
	c.CloseToast(id)
	tick(16 * time.Millisecond)
	if !reddish() {
		t.Error("the last toast went without its exit transition")
	}
	tick(300 * time.Millisecond)
	if reddish() {
		t.Error("the toast stays after its exit transition")
	}
}
