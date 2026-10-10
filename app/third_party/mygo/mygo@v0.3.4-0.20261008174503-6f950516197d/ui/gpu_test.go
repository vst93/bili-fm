package ui

import (
	"errors"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/internal/scene"
	"github.com/egoist/mygo/internal/surface"
	"github.com/egoist/mygo/transfer"
)

// testSurface is a surface that counts the frames drawn in memory.
type testSurface struct{ pixels int }

func (s *testSurface) Native() platform.SurfaceNative           { return platform.SurfaceNative{HWND: 1} }
func (s *testSurface) Size() (float64, float64, float64)        { return 200, 100, 1 }
func (s *testSurface) RequestFrame()                            {}
func (s *testSurface) RefreshRate() float64                     { return 60 }
func (s *testSurface) PresentPixels([]byte, int, int, int)      { s.pixels++ }
func (s *testSurface) SetCursor(platform.Cursor)                {}
func (s *testSurface) SetTextInput(platform.TextInputState)     {}
func (s *testSurface) UpdateAccessibility(*platform.AccessTree) {}
func (s *testSurface) StartDataDrag(r platform.DragRequest) {
	r.Done(transfer.Result{Err: platform.ErrUnsupported})
}
func (s *testSurface) CancelDataDrag()                  {}
func (s *testSurface) SetDropFormats([]transfer.Format) {}

// testGPU is a GPU renderer whose device goes away when fail is set.
type testGPU struct {
	fail             bool
	frames, released int
}

func (g *testGPU) Render(*scene.Scene) error {
	if g.fail {
		return errors.New("the device was removed")
	}
	g.frames++
	return nil
}

func (g *testGPU) Release() { g.released++ }

// gpuHost returns a window host on a test surface whose GPU renderers
// newGPU makes, and a function drawing a frame.
func gpuHost(t *testing.T, make func() (gpuRenderer, error)) (*windowHost, *testSurface, func()) {
	t.Setenv("MYGO_GPU", "")
	newGPU = func(platform.SurfaceNative) (gpuRenderer, error) { return make() }
	t.Cleanup(func() { newGPU = newGPURenderer })
	s := &testSurface{}
	h := &windowHost{conn: &surface.Conn{Surface: s}}
	h.rt = newRuntime(func(c *context) { coreText(c, "Hello") }, h)
	return h, s, func() { h.event(platform.SurfaceEvent{Kind: platform.SurfaceFrame}) }
}

// softGPU is a renderer drawing on the CPU, as Direct3D's WARP.
type softGPU struct{ testGPU }

func (g *softGPU) Software() bool { return true }

func TestGPUComesBack(t *testing.T) {
	var made []*testGPU
	gone := false
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) {
		if gone {
			return nil, errors.New("no device")
		}
		g := &testGPU{}
		made = append(made, g)
		return g, nil
	})
	frame()
	if len(made) != 1 || made[0].frames != 1 || s.pixels != 0 {
		t.Fatalf("first frame: %d renderers, %d frames in memory", len(made), s.pixels)
	}

	// A renderer that fails gives way to another at once.
	made[0].fail = true
	frame()
	if made[0].released != 1 || len(made) != 2 || made[1].frames != 1 || s.pixels != 0 {
		t.Fatalf("after a failure: released %d, %d renderers, %d frames in memory", made[0].released, len(made), s.pixels)
	}

	// Without a device, frames are drawn in memory until the wait is over.
	gone = true
	made[1].fail = true
	frame()
	frame()
	if len(made) != 2 || s.pixels != 2 || h.backoff != time.Second {
		t.Fatalf("without a device: %d renderers, %d frames in memory, wait %v", len(made), s.pixels, h.backoff)
	}
	// None either when the wait is over: the next wait is longer.
	h.retryAt = time.Now()
	frame()
	if s.pixels != 3 || h.backoff != 2*time.Second || h.retryAt.IsZero() {
		t.Fatalf("still without a device: %d frames in memory, wait %v", s.pixels, h.backoff)
	}
	// It is back.
	gone = false
	h.retryAt = time.Now()
	frame()
	if len(made) != 3 || made[2].frames != 1 || s.pixels != 3 || !h.retryAt.IsZero() {
		t.Fatalf("once back: %d renderers, %d frames in memory", len(made), s.pixels)
	}
}

func TestSoftwareUntilTheGPUIsBack(t *testing.T) {
	onGPU := true
	var gpus []*testGPU
	var soft *softGPU
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) {
		if onGPU {
			g := &testGPU{}
			gpus = append(gpus, g)
			return g, nil
		}
		soft = &softGPU{}
		return soft, nil
	})
	frame()
	// The GPU goes away: a renderer drawing in software takes its place,
	// and frames still show.
	onGPU = false
	gpus[0].fail = true
	frame()
	if soft == nil || soft.frames != 1 || s.pixels != 0 || !h.degraded || h.retryAt.IsZero() {
		t.Fatalf("after the loss: software %v, %d frames in memory, degraded %v", soft != nil, s.pixels, h.degraded)
	}
	// When the wait is over, a frame tries for the GPU, still away.
	first := soft
	h.retryAt = time.Now()
	frame()
	if first.released != 1 || soft == first || soft.frames != 1 || h.backoff != 2*time.Second {
		t.Fatalf("trying again: released %d, wait %v", first.released, h.backoff)
	}
	// It is back.
	onGPU = true
	h.retryAt = time.Now()
	frame()
	if soft.released != 1 || len(gpus) != 2 || gpus[1].frames != 1 || h.degraded || !h.retryAt.IsZero() {
		t.Errorf("once back: software released %d, %d GPU renderers, degraded %v", soft.released, len(gpus), h.degraded)
	}
}

func TestNoGPUFromTheStart(t *testing.T) {
	tries := 0
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) {
		tries++
		return nil, errors.New("no device")
	})
	frame()
	frame()
	if tries != 1 || s.pixels != 2 || !h.retryAt.IsZero() {
		t.Errorf("%d tries, %d frames in memory, retry at %v", tries, s.pixels, h.retryAt)
	}

	// A machine whose renderer draws in software from the start keeps it.
	tries = 0
	h, s, frame = gpuHost(t, func() (gpuRenderer, error) {
		tries++
		return &softGPU{}, nil
	})
	frame()
	frame()
	if tries != 1 || s.pixels != 0 || h.degraded || !h.retryAt.IsZero() {
		t.Errorf("software from the start: %d tries, degraded %v, retry at %v", tries, h.degraded, h.retryAt)
	}
}

// TestEveryFrameOnTheGPU checks that with a GPU, every frame draws on it:
// the first, one changing little after a pause, and one changing little
// while frames follow each other.
func TestEveryFrameOnTheGPU(t *testing.T) {
	g := &testGPU{}
	h, s, frame := gpuHost(t, func() (gpuRenderer, error) { return g, nil })
	x := float32(10)
	h.rt = newRuntime(func(c *context) {
		coreBox(c).Fill().Background(RGB(200, 200, 200)).Children(func() {
			coreBox(c).Size(10, 10).Background(RGB(0, 0, 255)).Absolute().Left(x).Top(10)
		})
	}, h)
	frame()
	h.lastFrame = time.Now().Add(-time.Second)
	x = 30
	frame()
	x = 40
	frame()
	if g.frames != 3 || s.pixels != 0 || h.path != "drawn on the GPU" {
		t.Errorf("%d frames on the GPU, %d drawn in memory, the last %q", g.frames, s.pixels, h.path)
	}
}
