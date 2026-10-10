package ui

import (
	"strings"
	"testing"
	"time"
)

func TestFrameStatsSetting(t *testing.T) {
	for _, tc := range []struct {
		v         string
		threshold time.Duration
		on        bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"off", 0, false},
		{"1", 8 * time.Millisecond, true},
		{"true", 8 * time.Millisecond, true},
		{"4", 4 * time.Millisecond, true},
		{"16.5", 16500 * time.Microsecond, true},
		{"all", 0, true},
		{"fast", 8 * time.Millisecond, true},
	} {
		threshold, on := frameStatsSetting(tc.v)
		if threshold != tc.threshold || on != tc.on {
			t.Errorf("MYGO_FRAME_STATS=%q: %v, %v; want %v, %v", tc.v, threshold, on, tc.threshold, tc.on)
		}
	}
}

func TestFrameStats(t *testing.T) {
	var logged []string
	defer func(f func(...any)) { frameLog = f }(frameLog)
	frameLog = func(v ...any) { logged = append(logged, v[0].(string)) }

	clicked := false
	tt := coreNewTester(func(c *context) {
		if coreButton(c, "Go").Clicked() {
			clicked = true
		}
		coreText(c, "Hello")
	}, 200, 100)
	tt.rt.stats = newFrameStats(0)
	if err := tt.Click("Go"); err != nil || !clicked {
		t.Fatal(err, clicked)
	}
	if len(logged) == 0 {
		t.Fatal("no frame logged")
	}
	line := logged[len(logged)-1]
	for _, want := range []string{"build ", "layout ", "paint ", "present ", "(drawn in memory)", "allocations", "KB", "text layouts"} {
		if !strings.Contains(line, want) {
			t.Errorf("%q lacks %q", line, want)
		}
	}
	// Frames under the threshold are not logged.
	logged = nil
	tt.rt.stats = newFrameStats(time.Hour)
	tt.Frame()
	if len(logged) > 0 {
		t.Errorf("a fast frame logged %q", logged)
	}
}
