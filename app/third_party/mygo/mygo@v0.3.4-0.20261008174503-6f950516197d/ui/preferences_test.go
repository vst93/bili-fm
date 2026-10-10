package ui

import (
	"testing"
	"time"
)

func TestThemeFollowsTheAccent(t *testing.T) {
	var theme *Theme
	tt := coreNewTester(func(c *context) { theme = c.Theme() }, 200, 100)
	if theme.Accent != LightTheme().Accent {
		t.Fatalf("without an accent of the desktop: %v", theme.Accent)
	}
	pink := Color{R: 219, G: 39, B: 119, A: 255}
	tt.SetPreferences(Preferences{Accent: pink})
	if theme.Accent != pink || theme.AccentText != (Color{R: 255, G: 255, B: 255, A: 255}) || theme.AccentHover == pink {
		t.Errorf("pink: accent %v, text %v, hover %v", theme.Accent, theme.AccentText, theme.AccentHover)
	}
	if theme.Focus.A == 0 || theme.Selection.A == 0 || theme.Focus.R != pink.R {
		t.Errorf("pink: focus %v, selection %v", theme.Focus, theme.Selection)
	}
	// Text on a light accent is dark.
	tt.SetPreferences(Preferences{Accent: Color{R: 250, G: 204, B: 21, A: 255}})
	if l := theme.AccentText.gray().R; l > 100 {
		t.Errorf("text on yellow: %v", theme.AccentText)
	}
	tt.SetDark(true)
	if !theme.Dark || theme.Accent != (Color{R: 250, G: 204, B: 21, A: 255}) {
		t.Errorf("dark: %+v", theme)
	}
}

func TestThemeFollowsContrastAndTextSize(t *testing.T) {
	var theme *Theme
	var prefs Preferences
	tt := coreNewTester(func(c *context) { theme, prefs = c.Theme(), c.Preferences() }, 200, 100)
	if prefs.TextScale != 1 || prefs.HighContrast || prefs.ReduceMotion {
		t.Fatalf("by default: %+v", prefs)
	}
	base := LightTheme()
	tt.SetPreferences(Preferences{HighContrast: true, TextScale: 1.5})
	if theme.FontSize != base.FontSize*1.5 {
		t.Errorf("font size %v, not %v", theme.FontSize, base.FontSize*1.5)
	}
	if theme.Border.gray().R >= base.Border.gray().R || theme.TextMuted.gray().R >= base.TextMuted.gray().R || theme.Focus.A != 255 {
		t.Errorf("high contrast: border %v, muted %v, focus %v", theme.Border, theme.TextMuted, theme.Focus)
	}
}

func TestAnimateWithoutMotion(t *testing.T) {
	target := float32(0)
	var got float32
	tt := coreNewTester(func(c *context) {
		got = coreBox(c).Size(10, 10).Animate("x", target, time.Second)
	}, 200, 100)
	target = 1
	tt.Frame()
	if got == 1 {
		t.Fatal("Animate jumped with motion")
	}
	tt.SetPreferences(Preferences{ReduceMotion: true})
	target = 2
	tt.Frame()
	if got != 2 {
		t.Errorf("without motion, Animate is at %v", got)
	}
}

func TestOwnThemeIgnoresPreferences(t *testing.T) {
	var accent Color
	tt := coreNewTester(func(c *context) {
		c.SetTheme(LightTheme())
		accent = c.Theme().Accent
	}, 200, 100)
	tt.SetPreferences(Preferences{Accent: Color{R: 219, G: 39, B: 119, A: 255}})
	if accent != LightTheme().Accent {
		t.Errorf("a theme of the app's took the desktop's accent: %v", accent)
	}
}
