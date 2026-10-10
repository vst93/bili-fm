package main

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// Over the window's material, the sidebar draws no background, and a
// material chosen there shows how to set it; where no material shows, the
// sidebar draws its own and the pane says why.
func TestVibrancy(t *testing.T) {
	a := &app{chosen: "sidebar"}
	tt := ui.NewTester(a.view, 880, 640)
	tt.SetVibrancy(true)
	alpha := func(x, y int) uint8 {
		img := tt.Image()
		return img.Pix[img.PixOffset(x, y)+3]
	}
	if alpha(100, 600) != 0 || alpha(600, 600) != 255 {
		t.Errorf("over the material, the sidebar's alpha is %d and the pane's %d; want 0 and 255", alpha(100, 600), alpha(600, 600))
	}
	if err := tt.Click("Mica"); err != nil {
		t.Fatal(err)
	}
	if a.chosen != "mica" || !tt.HasText("win.SetVibrancy(mygo.VibrancyMica)") {
		t.Errorf("chose %q, texts %q", a.chosen, tt.Texts())
	}
	if err := tt.Click("Hide Sidebar"); err != nil {
		t.Fatal(err)
	}
	if !a.collapsed || !tt.HasText("Show Sidebar") {
		t.Errorf("the sidebar shows after Hide Sidebar: collapsed %v", a.collapsed)
	}
	if err := tt.Click("Show Sidebar"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond) // as it slides back in
	tt.SetVibrancy(false)
	if alpha(100, 600) != 255 || !tt.HasText("This window shows no material here, so the sidebar draws a background of its own: materials show on macOS, and on Windows 11 22H2 and later.") {
		t.Errorf("without a material, the sidebar's alpha is %d, texts %q", alpha(100, 600), tt.Texts())
	}
}
