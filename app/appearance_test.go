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

// TestAppearancePrefsSystemRoundTrip 钉住 "system" 作为偏好落盘并原样读回：
// 落盘的是 system，不是解析出来的 light/dark（与 auto 同一条规矩）——否则
// 重启后「跟随系统」就冻结成当时那一次的系统外观了。
func TestAppearancePrefsSystemRoundTrip(t *testing.T) {
	dir := t.TempDir()
	kv, err := store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &controller{kv: kv, app: view.NewApp(func() {})}
	c.app.SetAppearance("system", .237)
	c.persistAppearance()
	if got := c.kv.String(prefTheme); got != "system" {
		t.Fatalf("落盘的主题偏好应是 system，实际 %q", got)
	}
	c.kv, err = store.Open(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	c.app = view.NewApp(func() {})
	c.loadPrefs()
	if c.app.AppearancePref() != "system" || c.app.Theme.Transparency != .237 {
		t.Fatalf("system preference did not round trip: %s %v", c.app.AppearancePref(), c.app.Theme.Transparency)
	}
	if c.app.Theme.Mode != "light" && c.app.Theme.Mode != "dark" {
		t.Fatalf("system did not resolve to a concrete mode: %s", c.app.Theme.Mode)
	}
	// 读回后仍是「跟随系统」：读到系统深色就该立刻变深。
	if !c.app.SetSystemDark(true) || c.app.Theme.Mode != "dark" {
		t.Fatalf("读回的 system 不跟系统：mode=%s", c.app.Theme.Mode)
	}
}
