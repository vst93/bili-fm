package glass

import (
	"testing"

	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/gpu/gputest"
	"github.com/egoist/mygo/internal/gpu/metal"
)

// TestMetal checks that Metal draws the glass as the CPU does.
func TestMetal(t *testing.T) {
	s := testScene()
	pix, err := metal.RenderOffscreen(s)
	if err != nil {
		t.Skip("no Metal:", err)
	}
	gputest.Compare(t, "glass-metal", pix, s.Width*4, s)
	// With continuous corners, as macOS's.
	for i := range s.Ops {
		s.Ops[i].Continuous = true
	}
	if pix, err = metal.RenderOffscreen(s); err != nil {
		t.Fatal(err)
	}
	gputest.Compare(t, "glass-metal-continuous", pix, s.Width*4, s)
}

// TestMetalBlur checks that Metal draws the blur as the CPU does.
func TestMetalBlur(t *testing.T) {
	s := blurScene()
	pix, err := metal.RenderOffscreen(s)
	if err != nil {
		t.Skip("no Metal:", err)
	}
	gputest.Compare(t, "blur-metal", pix, s.Width*4, s)
	for i := range s.Ops {
		s.Ops[i].Continuous = true
	}
	if pix, err = metal.RenderOffscreen(s); err != nil {
		t.Fatal(err)
	}
	gputest.Compare(t, "blur-metal-continuous", pix, s.Width*4, s)
}

// TestMetalLibrary checks that the libraries compiled ahead of time come
// from glass.metal and blur.metal and the renderer's head and tail as
// they are.
func TestMetalLibrary(t *testing.T) {
	if gpu.SourceSum(metal.EffectSource(metalSource)) != metalSum {
		t.Error("glass.metal or the Metal renderer's effect.metal changed since shaders_darwin.go was generated: run go generate ./plugins/glass on macOS")
	}
	if gpu.SourceSum(metal.EffectSource(blurMetalSource)) != blurMetalSum {
		t.Error("blur.metal or the Metal renderer's effect.metal changed since shaders_darwin.go was generated: run go generate ./plugins/glass on macOS")
	}
}
