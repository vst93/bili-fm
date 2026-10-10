//go:build linux && (amd64 || arm64)

package gl

import (
	"bytes"
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/gpu/gputest"
	"github.com/egoist/mygo/internal/raster"
)

// apis are the APIs the renderer draws with: OpenGL 3.3 and OpenGL ES 3.0.
var apis = []struct {
	name string
	es   bool
}{{"OpenGL", false}, {"OpenGL ES", true}}

// offscreen makes a context current on the calling thread, without a
// window, with a framebuffer of w×h pixels (see Offscreen), or skips t.
// read returns its pixels as the CPU renderer draws them.
func offscreen(t *testing.T, es bool, w, h int) (read func() []byte) {
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	o, err := NewOffscreen(es, w, h)
	if err != nil {
		t.Skip(err)
	}
	t.Cleanup(o.Release)
	t.Log(o.Info())
	return func() []byte {
		pix, err := ReadFramebuffer(w, h)
		if err != nil {
			t.Fatal(err)
		}
		return pix
	}
}

func TestDrawsAsTheCPURenderer(t *testing.T) {
	for _, api := range apis {
		t.Run(api.name, func(t *testing.T) {
			s := gputest.Scene()
			read := offscreen(t, api.es, s.Width, s.Height)
			r, err := New()
			if err != nil {
				t.Fatal(err)
			}
			defer r.Release()
			if r.es != api.es {
				t.Fatalf("the renderer takes the context for OpenGL ES: %v", r.es)
			}
			// Twice: the second frame updates what the first uploaded.
			for range 2 {
				if err := r.Render(s); err != nil {
					t.Fatal(err)
				}
				gputest.Compare(t, "gl", read(), s.Width*4, s)
			}
		})
	}
}

func TestPresentsFramesDrawnInMemory(t *testing.T) {
	for _, api := range apis {
		t.Run(api.name, func(t *testing.T) {
			s := gputest.Scene()
			read := offscreen(t, api.es, s.Width, s.Height)
			want := raster.NewImage(s.Width, s.Height)
			raster.Render(want, s)
			var p Presenter
			// Twice: the second frame reuses the texture.
			for range 2 {
				if err := p.Present(want.Pix, want.Stride, want.W, want.H); err != nil {
					t.Fatal(err)
				}
				if got := read(); !bytes.Equal(got, want.Pix[:len(got)]) {
					t.Fatal("the frame presented differs from the frame drawn")
				}
			}
		})
	}
}
