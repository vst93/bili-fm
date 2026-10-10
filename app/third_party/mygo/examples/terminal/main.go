// Terminal is a terminal app in native UI: the user's shell in a window,
// with the terminal plugin, which emulates it with Ghostty's libghostty-vt.
// The window takes the shell's title and closes when the shell exits.
//
//	go run ./examples/terminal
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func main() {
	mygo.App.WhenReady(func() {
		var win *mygo.Window
		term, err := terminal.New(terminal.Options{
			OnTitle: func(title string) { win.SetTitle(title) },
			OnExit:  func(int) { win.Close() },
		})
		if err != nil {
			log.Fatal(err)
		}
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:  "Terminal",
			Width:  760,
			Height: 480,
			Content: ui.View(func(c *ui.Context) {
				terminal.View(c, term).Fill().AutoFocus()
			}),
		})
		win.OnClosed(func() { term.Close() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
