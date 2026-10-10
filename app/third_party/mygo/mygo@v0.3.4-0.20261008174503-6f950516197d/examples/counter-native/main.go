// Counter-native is a counter in native UI: a window MyGo draws itself from
// Go, with no web page. The arrow keys count too.
//
//	go run ./examples/counter-native
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// counter is the state the window shows.
type counter struct{ n int }

// view builds the interface from the state, on the main thread, whenever the
// window needs a frame.
func (s *counter) view(c *ui.Context) {
	if c.Shortcut(0, ui.KeyUp) {
		s.n++
	}
	if c.Shortcut(0, ui.KeyDown) {
		s.n--
	}
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.Textf(c, "%d", s.n).FontSize(56).Bold()
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c.Key("decrement"), "−").Label("Decrement").Width(44).Clicked() {
				s.n--
			}
			if ui.Button(c.Key("reset"), "Reset").Disabled(s.n == 0).Clicked() {
				s.n = 0
			}
			if ui.PrimaryButton(c.Key("increment"), "+").Label("Increment").Width(44).Clicked() {
				s.n++
			}
		})
		ui.Text(c, "↑ and ↓ count too").FontSize(12).TextColor(t.TextMuted)
	})
}

func main() {
	s := &counter{}
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title:   "Counter",
			Width:   360,
			Height:  260,
			Content: ui.View(s.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
