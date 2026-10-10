package e2e

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// TestContentWindowResizeFromFirstFrame resizes native UI from its first
// frame, before the window is activated: on Wayland, the activation's
// configure event must not bring the size back.
func TestContentWindowResizeFromFirstFrame(t *testing.T) {
	for _, tc := range []struct {
		name     string
		initial  int
		requests []int
	}{
		{"grow", 300, []int{600}},
		{"shrink", 600, []int{300}},
		{"latest request", 300, []int{450, 600}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var w *mygo.Window
			var rendered atomic.Int32
			requested := false
			mygo.RunOnMain(func() {
				w = newWindow(t, mygo.WindowOptions{
					Title: "First-frame resize", Width: 600, Height: tc.initial,
					UseContentSize: true, Hidden: true,
					Content: ui.View(func(c *ui.Context) {
						ui.Column(c).Fill().DrawOver(func(_ *ui.Painter, r ui.Rect) {
							rendered.Store(int32(r.H))
							if requested || w == nil {
								return
							}
							requested = true
							w.Update(func() {
								for _, height := range tc.requests {
									w.SetContentSize(600, height)
								}
							})
						}).Children(func() { ui.Text(c, "Resize from the first frame") })
					}),
				})
				w.Show()
			})
			target := int32(tc.requests[len(tc.requests)-1])
			eventually(t, "the requested rendered height", func() bool { return rendered.Load() == target })
			// A delayed compositor configure must not undo a locally accepted resize.
			time.Sleep(200 * time.Millisecond)
			if got := rendered.Load(); got != target {
				t.Fatalf("resize reverted: got %d, want %d", got, target)
			}
			w.SetContentSize(600, 420)
			eventually(t, "a later resize", func() bool { return rendered.Load() == 420 })
			time.Sleep(200 * time.Millisecond)
			if got := rendered.Load(); got != 420 {
				t.Fatalf("startup resize overrode a later one: got %d", got)
			}
		})
	}
}
