package ui

import (
	"hash/fnv"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestSpinner(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreSpinner(c).Label("Loading")
	}, 200, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleProgress, "Loading"); n.Now >= n.Min {
		t.Errorf("the spinner is not of unknown length: %+v", n)
	}
	if !tt.rt.redraw || tt.rt.repaintDue.IsZero() {
		t.Error("the spinner is not painted again for its next spoke")
	}
}

func TestMeter(t *testing.T) {
	levels := &MeterLevels{Warning: 70, Critical: 90}
	battery := &MeterLevels{Warning: 20, Critical: 10}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(10).Padding(10).Children(func() {
			coreMeter(c, 50, 0, 100, levels).Width(100).Label("Fine")
			coreMeter(c, 80, 0, 100, levels).Width(100).Label("Warning")
			coreMeter(c, 95, 0, 100, levels).Width(100).Label("Critical")
			coreMeter(c, 15, 0, 100, battery).Width(100).Label("Battery")
		})
	}, 200, 200)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	img := tt.Image()
	th := LightTheme()
	for _, want := range []struct {
		name  string
		color Color
	}{{"Fine", th.Success}, {"Warning", th.Warning}, {"Critical", th.Danger}, {"Battery", th.Warning}} {
		n := accessNode(t, tt.h.access, platform.RoleMeter, want.name)
		if n.Min != 0 || n.Max != 100 {
			t.Errorf("%s: %+v", want.name, n)
		}
		// Inside the bar's fill, near its start.
		got := img.RGBAAt(int(n.Bounds.X)+5, int(n.Bounds.Y+n.Bounds.H/2))
		if !nearColor(got, want.color) {
			t.Errorf("%s is %v, not %v", want.name, got, want.color)
		}
	}
}

// nearColor reports whether a pixel is about a color.
func nearColor(got color.RGBA, want Color) bool {
	d := func(a, b uint8) bool { return int(a)-int(b) < 8 && int(b)-int(a) < 8 }
	return d(got.R, want.R) && d(got.G, want.G) && d(got.B, want.B)
}

func TestRating(t *testing.T) {
	stars, changes := 0, 0
	tt := coreNewTester(func(c *context) {
		if coreRating(c, &stars, 5).Label("Rating").Changed() {
			changes++
		}
	}, 300, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	r := accessNode(t, tt.h.access, platform.RoleSlider, "Rating").Bounds
	// Five stars of 16.25 DIPs, two DIPs apart, at the start of the row.
	star := float32(18.25)
	tt.ClickAt(float32(r.X)+star*2.5, float32(r.Y+r.H/2))
	if stars != 3 || changes != 1 {
		t.Fatalf("a click on the third star: %d (%d changes)", stars, changes)
	}
	// A click on the star set clears it.
	tt.ClickAt(float32(r.X)+star*2.5, float32(r.Y+r.H/2))
	if stars != 0 {
		t.Fatalf("a second click: %d", stars)
	}
	tt.Key(0, KeyRight)
	tt.Key(0, KeyRight)
	if stars != 2 {
		t.Fatalf("Right twice: %d", stars)
	}
	tt.Key(0, KeyEnd)
	if n := accessNode(t, tt.h.access, platform.RoleSlider, "Rating"); stars != 5 || n.Now != 5 || n.Max != 5 || n.Actions&platform.ActionIncrement == 0 {
		t.Errorf("End: %d, %+v", stars, n)
	}
}

func TestStepper(t *testing.T) {
	v := 5.0
	tt := coreNewTester(func(c *context) {
		coreStepper(c, &v, 0, 10, 0.5).Label("Copies")
	}, 200, 200)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	n := accessNode(t, tt.h.access, platform.RoleStepper, "Copies")
	b := n.Bounds
	up := func() (float32, float32) { return float32(b.X + b.W/2), float32(b.Y + b.H/4) }
	tt.ClickAt(up())
	if v != 5.5 {
		t.Fatalf("a click on the up arrow: %v", v)
	}
	tt.ClickAt(float32(b.X+b.W/2), float32(b.Y+b.H*3/4))
	if v != 5 {
		t.Fatalf("a click on the down arrow: %v", v)
	}
	tt.Key(0, KeyUp)
	tt.Key(0, KeyEnd)
	if v != 10 {
		t.Fatalf("Up and End: %v", v)
	}
	tt.ClickAt(up())
	if v != 10 {
		t.Errorf("past the top: %v", v)
	}
	// Assistive technology steps it.
	n = accessNode(t, tt.h.access, platform.RoleStepper, "Copies")
	if n.Max != 10 || n.Actions&platform.ActionDecrement == 0 {
		t.Fatalf("the stepper: %+v", n)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: n.ID, Action: platform.AccessDecrement})
	if v != 9.5 {
		t.Errorf("decremented to %v", v)
	}
	// Held, it keeps stepping.
	tt.Press(float32(b.X+b.W/2), float32(b.Y+b.H*3/4))
	time.Sleep(700 * time.Millisecond)
	tt.Frame()
	tt.Release(float32(b.X+b.W/2), float32(b.Y+b.H*3/4))
	if v > 8 {
		t.Errorf("held 0.7 s, it went down to %v", v)
	}
}

func TestStepSlider(t *testing.T) {
	v := 20.0
	tt := coreNewTester(func(c *context) {
		coreStepSlider(c, &v, 0, 100, 25).Width(220).Label("Quality")
	}, 300, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	r := accessNode(t, tt.h.access, platform.RoleSlider, "Quality").Bounds
	// 10 DIPs of padding on each side, 200 of track: 60% is 60, 50 nearest.
	tt.ClickAt(float32(r.X)+10+120, float32(r.Y+10))
	if v != 50 {
		t.Fatalf("a click at 60%%: %v", v)
	}
	tt.Key(0, KeyRight)
	if v != 75 {
		t.Errorf("Right: %v", v)
	}
	if tickCount(0, 100, 25) != 5 || tickCount(0, 1, 0.01) != 0 || snap(0.30000000000000004, 0, 1, 0.1) != 0.3 {
		t.Error("ticks or snapping")
	}
}

func TestRangeSlider(t *testing.T) {
	low, high := 20.0, 80.0
	tt := coreNewTester(func(c *context) {
		coreRangeSlider(c, &low, &high, 0, 100, 10).Width(220).Label("Price")
	}, 300, 100)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	minimum := accessNode(t, tt.h.access, platform.RoleSlider, "Price minimum")
	maximum := accessNode(t, tt.h.access, platform.RoleSlider, "Price maximum")
	if minimum.Now != 20 || maximum.Now != 80 {
		t.Fatalf("the knobs: %+v, %+v", minimum, maximum)
	}
	// A press on the track moves the nearest knob.
	r := accessNode(t, tt.h.access, platform.RoleGroup, "Price").Bounds
	tt.ClickAt(float32(r.X)+10+200*0.68, float32(r.Y+10))
	if high != 70 || low != 20 {
		t.Fatalf("a click at 68%%: %v to %v", low, high)
	}
	// The keys move the knob with the focus, which Tab moves between.
	tt.Key(Shift, KeyTab)
	tt.Key(0, KeyRight)
	if low != 30 {
		t.Fatalf("Right on the low knob: %v", low)
	}
	tt.Key(0, KeyEnd)
	if low != 70 {
		t.Errorf("End stops the low knob at the high one: %v", low)
	}
}

func TestAvatar(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreRow(c).Gap(8).Children(func() {
			coreAvatar(c, "Ada Lovelace", nil)
			coreAvatar(c, "grace", nil)
		})
	}, 200, 100)
	if !tt.HasText("AL") || !tt.HasText("G") {
		t.Errorf("initials: %q", tt.Texts())
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	accessNode(t, tt.h.access, platform.RoleImage, "Ada Lovelace")
}

func TestInitials(t *testing.T) {
	for name, want := range map[string]string{
		"Ada Lovelace":          "AL",
		"grace":                 "G",
		"  jean-luc  picard ":   "JP",
		"Grace Brewster Hopper": "GH",
		"李 小龙":                  "李小",
		"--":                    "",
		"":                      "",
		"élodie 2nd":            "É2",
		"o'brien":               "OB",
		"Ada\tLovelace\nByron ": "AB",
	} {
		if got := initials(name); got != want {
			t.Errorf("initials(%q) = %q, want %q", name, got, want)
		}
	}
	// The color is the hue of FNV-1a, as hash/fnv computes it.
	h := fnv.New32a()
	h.Write([]byte("Ada Lovelace"))
	want := hslColor(float64(h.Sum32()%360), 0.45, 0.55)
	var got Color
	coreNewTester(func(c *context) { got = coreAvatar(c, "Ada Lovelace", nil).bg }, 100, 100)
	if got != want {
		t.Errorf("Avatar's color is %v, want %v", got, want)
	}
}
