package glass

import (
	"runtime"
	"testing"

	"github.com/egoist/mygo/internal/gpu"
	"github.com/egoist/mygo/internal/gpu/d3d11"
	"github.com/egoist/mygo/internal/gpu/gputest"
)

// TestDirect3D checks that Direct3D 11 draws the glass as the CPU does.
func TestDirect3D(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	s := testScene()
	pix, stride, err := d3d11.RenderOffscreen(s)
	if err != nil {
		t.Skip("no Direct3D 11:", err)
	}
	gputest.Compare(t, "glass-d3d11", pix, stride, s)
	s = blurScene()
	if pix, stride, err = d3d11.RenderOffscreen(s); err != nil {
		t.Fatal(err)
	}
	gputest.Compare(t, "blur-d3d11", pix, stride, s)
}

// TestBytecode checks that the bytecode compiled ahead of time comes from
// glass.hlsl and blur.hlsl and the renderer's head and tail as they are.
func TestBytecode(t *testing.T) {
	if gpu.SourceSum(d3d11.EffectSource(hlslSource)) != hlslSum {
		t.Error("glass.hlsl or the Direct3D renderer's effect.hlsl changed since shaders_windows.go was generated: run go generate ./plugins/glass on Windows")
	}
	if gpu.SourceSum(d3d11.EffectSource(blurHLSLSource)) != blurHLSLSum {
		t.Error("blur.hlsl or the Direct3D renderer's effect.hlsl changed since shaders_windows.go was generated: run go generate ./plugins/glass on Windows")
	}
}
