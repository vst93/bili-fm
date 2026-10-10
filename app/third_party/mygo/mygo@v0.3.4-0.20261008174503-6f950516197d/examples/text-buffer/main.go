// A large editable document backed by indexed text chunks.
package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func document() *ui.TextBuffer {
	var source strings.Builder
	for i := range 100000 {
		fmt.Fprintf(&source, "Line %d: editable text, 😀, العربية, עברית\n", i+1)
	}
	return ui.NewTextBuffer(source.String())
}

func main() {
	buffer := document()
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: "Indexed text buffer", Width: 900, Height: 650, Content: ui.View(func(c *ui.Context) {
			ui.Column(c).Fill().Padding(20).Gap(10).Children(func() {
				ui.Text(c, "Indexed text input").FontSize(22).Bold()
				ui.Text(c, fmt.Sprintf("%d lines · %d runes · version %d", buffer.LineCount(), buffer.Len(), buffer.Version()))
				ui.TextAreaBuffer(c, buffer).Fill().Font("monospace").Label("Document").AutoFocus()
			})
		})})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
