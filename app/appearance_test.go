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
