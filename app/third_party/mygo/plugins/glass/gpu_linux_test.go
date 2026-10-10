//go:build amd64 || arm64

package glass

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/gpu/gl"
	"github.com/egoist/mygo/internal/gpu/gputest"
	"github.com/egoist/mygo/internal/scene"
)

// TestOpenGL checks that OpenGL 3.3 and OpenGL ES 3.0 draw the glass and
// the blur as the CPU does.
func TestOpenGL(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	for _, es := range []bool{false, true} {
		for _, sc := range []struct {
			name  string
			scene func() *scene.Scene
		}{{"glass-gl", testScene}, {"blur-gl", blurScene}} {
			s := sc.scene()
			o, err := gl.NewOffscreen(es, s.Width, s.Height)
			if err != nil {
				t.Skip(err)
			}
			t.Log(o.Info())
			pix, err := o.Render(s)
			o.Release()
			if err != nil {
				t.Fatal(err)
			}
			gputest.Compare(t, sc.name, pix, s.Width*4, s)
		}
	}
}
