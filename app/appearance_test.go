package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

func TestAppearancePrefsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}
	c.app.SetAppearance("dark", .237)
	c.persistAppearance()
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if c.app.Theme.Mode != "dark" || c.app.Theme.Transparency != .237 {
		t.Fatalf("preferences did not round trip: %s %v", c.app.Theme.Mode, c.app.Theme.Transparency)
	}
}

// TestAppearancePrefsAutoRoundTrip 钉住 "auto" 作为偏好落盘并原样读回，
// 而不是被解析后的 light/dark 覆盖。
func TestAppearancePrefsAutoRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}
	c.app.SetAppearance("auto", .237)
	c.persistAppearance()
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if c.app.AppearancePref() != "auto" || c.app.Theme.Transparency != .237 {
		t.Fatalf("auto preference did not round trip: %s %v", c.app.AppearancePref(), c.app.Theme.Transparency)
	}
	if c.app.Theme.Mode != "light" && c.app.Theme.Mode != "dark" {
		t.Fatalf("auto did not resolve to a concrete mode: %s", c.app.Theme.Mode)
	}
}
