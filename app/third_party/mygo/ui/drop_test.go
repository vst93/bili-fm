package ui

import (
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestFileDrops(t *testing.T) {
	var dropped []string
	over := false
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			zone := coreBox(c).Size(200, 100)
			if files := zone.DroppedFiles(); files != nil {
				dropped = append(dropped, files...)
			}
			over = zone.FileDragOver()
			coreBox(c).Size(200, 100)
		})
	}, 300, 300)
	if tt.send2(platform.SurfaceEvent{Kind: platform.FileDragOver, X: 50, Y: 150}) || over {
		t.Error("files are taken outside of the drop zone")
	}
	if !tt.send2(platform.SurfaceEvent{Kind: platform.FileDragOver, X: 50, Y: 50}) || !over {
		t.Error("files are not taken over the drop zone")
	}
	tt.send2(platform.SurfaceEvent{Kind: platform.FileDragLeave})
	if over {
		t.Error("the zone shows files over it after they left")
	}
	if tt.send2(platform.SurfaceEvent{Kind: platform.FileDrop, X: 50, Y: 150, Files: []string{"/tmp/a"}}) || dropped != nil {
		t.Error("files dropped outside of the zone reached it")
	}
	if !tt.send2(platform.SurfaceEvent{Kind: platform.FileDrop, X: 10, Y: 10, Files: []string{"/tmp/a", "/tmp/b"}}) {
		t.Error("the zone did not take the dropped files")
	}
	if !slices.Equal(dropped, []string{"/tmp/a", "/tmp/b"}) || over {
		t.Errorf("dropped %q, over %v", dropped, over)
	}
	tt.Frame()
	if len(dropped) != 2 {
		t.Errorf("the files were dropped again: %q", dropped)
	}
}

// send2 sends an event, settles the frames, and returns what the content
// answered.
func (t *Tester) send2(ev platform.SurfaceEvent) bool {
	taken := t.rt.event(ev)
	t.settle()
	return taken
}
