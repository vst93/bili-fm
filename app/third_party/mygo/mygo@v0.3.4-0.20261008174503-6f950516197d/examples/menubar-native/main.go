// Menubar-native is a menu bar app whose icon opens a window of native UI.
// It needs Go alone: no web page, JavaScript, or webview.
//
//	go run ./examples/menubar-native
package main

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// All state is read and changed on the main thread, by tray callbacks and
// the view. Hiding the window keeps its editor, selection, and undo history.
type notes struct {
	win      *mygo.Window
	tray     *mygo.Tray
	editor   ui.Handle
	text     string
	status   string
	quitting bool
}

func (s *notes) view(c *ui.Context) {
	if c.Shortcut(0, ui.KeyEscape) || c.Shortcut(ui.Cmd, ui.KeyW) {
		s.win.Hide()
	}
	c.OnShortcut(ui.Cmd, ui.KeyQ, mygo.App.Quit)
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Row(c).Gap(12).Children(func() {
			ui.Column(c).Grow(1).Gap(4).Children(func() {
				ui.Text(c, "Quick Notes").FontSize(22).Bold()
				ui.Text(c, "A little space for a quick thought.").FontSize(12).TextColor(t.TextMuted)
			})
			if ui.Button(c, "Hide").Label("Hide Quick Notes").Clicked() {
				s.win.Hide()
			}
		})
		if ui.TextArea(c.Key("note"), &s.text).Grow(1).Bind(&s.editor).
			Label("Note").Placeholder("Write something to remember…").Changed() {
			s.status = ""
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "Clear").Disabled(s.text == "").Clicked() {
				s.text, s.status = "", ""
				s.editor.Focus()
			}
			ui.Spacer(c)
			if ui.PrimaryButton(c, "Copy note").Disabled(s.text == "").Clicked() {
				mygo.Clipboard.WriteText(s.text)
				s.status = "Copied to clipboard."
			}
		})
		ui.Divider(c)
		ui.Row(c).Gap(8).Children(func() {
			status := s.status
			if status == "" {
				status = "Kept here until you quit."
			}
			ui.Text(c, status).Grow(1).FontSize(11).TextColor(t.TextMuted)
			ui.Button(c, "Quit").OnClick(mygo.App.Quit)
		})
	})
}

// show places the panel next to the tray icon on its display. macOS icons
// are at the top; a bottom taskbar puts the panel above its icon instead.
func (s *notes) show() {
	anchor := s.tray.Bounds()
	if anchor.Width == 0 || anchor.Height == 0 {
		// AppIndicator reports no bounds. This also handles a Windows icon
		// hidden in the notification area's overflow panel.
		s.win.Center()
	} else {
		area := mygo.Screen.DisplayNearestPoint(mygo.Point{
			X: anchor.X + anchor.Width/2,
			Y: anchor.Y + anchor.Height/2,
		}).WorkArea
		b := s.win.Bounds()
		x := anchor.X + (anchor.Width-b.Width)/2
		y := anchor.Y + anchor.Height + 6
		if y+b.Height > area.Y+area.Height {
			y = anchor.Y - b.Height - 6
		}
		// Keep the entire panel in the work area, including at the edge
		// of a secondary display or beside a vertical taskbar.
		x = max(area.X, min(x, area.X+max(0, area.Width-b.Width)))
		y = max(area.Y, min(y, area.Y+max(0, area.Height-b.Height)))
		s.win.SetPosition(x, y)
	}
	s.editor.Focus()
	s.win.Show()
}

func (s *notes) toggle() {
	if s.win.IsVisible() {
		s.win.Hide()
	} else {
		s.show()
	}
}

func (s *notes) ready() {
	var err error
	s.tray, err = mygo.NewTray(mygo.TrayOptions{
		Icon:           trayIcon(),
		IconIsTemplate: true,
		ToolTip:        "Quick Notes — click to open",
	})
	if err != nil {
		log.Fatal("tray: ", err)
	}
	s.win = mygo.NewWindow(mygo.WindowOptions{
		Title:             "Quick Notes",
		Width:             380,
		Height:            340,
		Hidden:            true,
		Frameless:         true,
		DisableResize:     true,
		DisableMinimize:   true,
		DisableMaximize:   true,
		DisableFullScreen: true,
		AlwaysOnTop:       true,
		SkipTaskbar:       true,
		Content:           ui.View(s.view),
	})
	s.win.OnBlur(s.win.Hide)
	s.win.OnClose(func(e *mygo.CloseEvent) {
		// Cmd+W/Alt+F4 dismiss the panel. App.Quit must still be able to
		// close it, so OnBeforeQuit lets that close pass through.
		if !s.quitting {
			e.PreventDefault()
			s.win.Hide()
		}
	})

	menu := mygo.NewMenu([]*mygo.MenuItem{
		{Label: "Open Quick Notes", Click: func(*mygo.MenuItem, *mygo.Window) { s.show() }},
		mygo.Separator(),
		{Role: mygo.RoleQuit},
	})
	if runtime.GOOS == "linux" {
		// AppIndicator only supports menus. Its Open action shows the
		// same native window as a direct icon click on macOS/Windows.
		s.tray.SetMenu(menu)
	} else {
		// A TrayOptions.Menu would consume the primary click on macOS.
		// Leave it unset so the icon toggles our actual native window.
		s.tray.OnClick(s.toggle)
		s.tray.OnRightClick(func() {
			s.win.Hide()
			// Install the menu for this popup only, so AppKit opens it
			// beneath the status item and primary clicks keep toggling
			// the notes window after the menu closes.
			s.tray.SetMenu(menu)
			defer s.tray.SetMenu(nil)
			s.tray.PopUpMenu()
		})
	}
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { s.tray.Destroy() })
}

// trayIcon draws a note, black on transparent for macOS to tint. Elsewhere
// a blue icon stays visible on both light and dark taskbars.
func trayIcon() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	ink := color.NRGBA{R: 79, G: 124, B: 255, A: 255}
	if runtime.GOOS == "darwin" {
		ink = color.NRGBA{A: 255}
	}
	for _, r := range []image.Rectangle{
		image.Rect(5, 3, 27, 6), image.Rect(5, 26, 27, 29),
		image.Rect(5, 6, 8, 26), image.Rect(24, 6, 27, 26),
		image.Rect(11, 10, 21, 12), image.Rect(11, 16, 21, 18),
		image.Rect(11, 22, 18, 24),
	} {
		draw.Draw(img, r, image.NewUniform(ink), image.Point{}, draw.Src)
	}
	var b bytes.Buffer
	_ = png.Encode(&b, img)
	return b.Bytes()
}

func main() {
	s := &notes{}
	mygo.App.SetName("Quick Notes")
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { s.quitting = true })
	mygo.App.WhenReady(s.ready)
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
