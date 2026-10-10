package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestCalendar(t *testing.T) {
	day := time.Date(2026, 10, 15, 9, 41, 0, 0, time.UTC)
	changes := 0
	tt := coreNewTester(func(c *context) {
		if coreCalendar(c, &day).Changed() {
			changes++
		}
	}, 400, 400)
	tt.Click("October 20, 2026")
	if !sameDay(day, time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)) || day.Hour() != 9 || changes != 1 {
		t.Fatalf("a click: %v (%d changes)", day, changes)
	}
	// The arrows choose as they move.
	for _, step := range []struct {
		key  Key
		want string
	}{{KeyRight, "2026-10-21"}, {KeyDown, "2026-10-28"}, {KeyEnd, "2026-10-31"}, {KeyHome, "2026-10-01"}, {KeyPageDown, "2026-11-01"}, {KeyLeft, "2026-10-31"}} {
		tt.Key(0, step.key)
		if got := day.Format("2006-01-02"); got != step.want {
			t.Fatalf("%v: %s, not %s", step.key, got, step.want)
		}
	}
	if !tt.HasText("October 2026") {
		t.Error("the calendar does not follow the day into its month")
	}
	// Assistive technology reads the day chosen.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleButton, "October 31, 2026"); tt.h.access.Focus != n.ID || n.States&platform.AccessChecked == 0 {
		t.Errorf("the focus on %d, the day %+v", tt.h.access.Focus, n)
	}
}

func TestTimeInput(t *testing.T) {
	alarm := time.Date(2026, 10, 4, 7, 30, 0, 0, time.UTC)
	changes := 0
	tt := coreNewTester(func(c *context) {
		if coreTimeInput(c, &alarm).Label("Alarm").Changed() {
			changes++
		}
	}, 300, 100)
	if !tt.HasText("07") || !tt.HasText("30") {
		t.Fatalf("shows %q", tt.Texts())
	}
	tt.Click("07")
	tt.Key(0, KeyUp)
	if alarm.Hour() != 8 || changes != 1 {
		t.Fatalf("Up: %v", alarm)
	}
	// Round the clock.
	for range 9 {
		tt.Key(0, KeyDown)
	}
	if alarm.Hour() != 23 {
		t.Fatalf("Down nine times from 8: %v", alarm)
	}
	// Digits typed set the hours, then go on to the minutes.
	tt.Key(0, Key1)
	tt.Key(0, Key5)
	if alarm.Hour() != 15 || !tt.Focused("30") {
		t.Fatalf("1 and 5: %v, minutes focused %v", alarm, tt.Focused("30"))
	}
	tt.Key(0, Key4)
	tt.Key(0, Key5)
	if alarm.Minute() != 45 {
		t.Fatalf("4 and 5 in the minutes: %v", alarm)
	}
	tt.Key(0, KeyLeft)
	tt.Key(0, Key9)
	if alarm.Hour() != 9 || !tt.Focused("45") {
		t.Errorf("9, which can't begin two digits of hours: %v, minutes focused %v", alarm, tt.Focused("45"))
	}
	if y, m, d := alarm.Date(); y != 2026 || m != 10 || d != 4 {
		t.Errorf("the date changed: %v", alarm)
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	hours := accessNode(t, tt.h.access, platform.RoleStepper, "Alarm hours")
	if hours.Now != 9 || hours.Max != 23 {
		t.Errorf("hours: %+v", hours)
	}
	accessNode(t, tt.h.access, platform.RoleStepper, "Alarm minutes")
	// Assistive technology steps the hours, not the focus to the minutes.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessAction, ID: hours.ID, Action: platform.AccessIncrement})
	if alarm.Hour() != 10 {
		t.Errorf("incremented the hours to %v", alarm)
	}
}

func TestColorPicker(t *testing.T) {
	color := Hex("#3b82f6")
	changes := 0
	tt := coreNewTester(func(c *context) {
		if coreColorPicker(c, &color).Changed() {
			changes++
		}
	}, 400, 500)
	// A swatch.
	tt.Click("Red")
	if color != Hex("#ef4444") || changes != 1 {
		t.Fatalf("Red: %v", hexOf(color))
	}
	// The hex typed.
	tt.Click("Hex")
	tt.Key(Cmd, KeyA)
	tt.Type("#00ff0080")
	if color != Hex("#00ff0080") {
		t.Fatalf("typed: %v", hexOf(color))
	}
	// The square: its top-right corner is the hue itself; its bottom black.
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	sq := accessNode(t, tt.h.access, platform.RoleSlider, "Saturation and brightness").Bounds
	tt.ClickAt(float32(sq.X+sq.W-1), float32(sq.Y+1))
	if h := toHSVA(color); h.s < 0.98 || h.v < 0.98 || h.h < 115 || h.h > 125 {
		t.Fatalf("the top-right corner: %v (%+v)", hexOf(color), h)
	}
	// A gray keeps the hue it came from.
	tt.ClickAt(float32(sq.X+1), float32(sq.Y+sq.H/2))
	tt.Key(0, KeyRight)
	if h := toHSVA(color); h.h < 115 || h.h > 125 {
		t.Errorf("out of the grays, the hue is %v", h.h)
	}
	// The app sets another color.
	color = Hex("#ffffff")
	tt.Frame()
	if n := accessNode(t, tt.h.access, platform.RoleTextField, "Hex"); n.Value != "#ffffff" {
		t.Errorf("the hex shows %q", n.Value)
	}
	n := accessNode(t, tt.h.access, platform.RoleSlider, "Hue")
	if n.Max != 360 {
		t.Errorf("the hue: %+v", n)
	}
}

func TestColorWell(t *testing.T) {
	color := Hex("#3b82f6")
	tt := coreNewTester(func(c *context) {
		coreColorWell(c, &color).Label("Tint")
	}, 400, 500)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleColorWell, "Tint"); n.Value != "#3b82f6" || n.States&platform.AccessExpanded != 0 {
		t.Fatalf("the well: %+v", n)
	}
	tt.Click("Tint")
	if !tt.HasText("Red") {
		t.Fatal("the picker did not open")
	}
	tt.Click("Green")
	if color != Hex("#22c55e") {
		t.Errorf("chose %v", hexOf(color))
	}
	if n := accessNode(t, tt.h.access, platform.RoleColorWell, "Tint"); n.Value != "#22c55e" || n.States&platform.AccessExpanded == 0 {
		t.Errorf("the well: %+v", n)
	}
	tt.Key(0, KeyEscape)
	if tt.HasText("Red") {
		t.Error("Escape left the picker open")
	}
}
