// Clipboard demonstrates one data model for copying and dragging a note.
// Run: go run ./examples/clipboard
package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const noteFormat transfer.Format = "application/vnd.mygo.clipboard-note+json"

type note struct{ Title, Body string }
type demo struct {
	note                note
	path, status        string
	providers, releases int
	win                 *mygo.Window
}

func (d *demo) data() transfer.Data {
	frozen := d.note // all alternatives describe this same frozen value
	return transfer.New(transfer.NewItem(
		transfer.Bytes(transfer.Text, []byte(frozen.Title+"\n"+frozen.Body)),
		transfer.Bytes(transfer.HTML, []byte("<h1>"+html.EscapeString(frozen.Title)+"</h1><p>"+html.EscapeString(frozen.Body)+"</p>")),
		transfer.Lazy(noteFormat, func() ([]byte, error) {
			d.providers++
			d.win.Invalidate()
			return json.Marshal(frozen)
		}),
	))
}

func (d *demo) receive(data transfer.Data) {
	if f, ok := data.Preferred(noteFormat, transfer.Text); ok {
		b, err := data.Read(f)
		if err != nil {
			d.status = err.Error()
			return
		}
		if f == noteFormat {
			if err := json.Unmarshal(b, &d.note); err != nil {
				d.status = err.Error()
				return
			}
		} else {
			d.note.Body = string(b)
		}
		d.status = "Received " + string(f)
	} else if files, err := data.Files(); err == nil {
		d.status = "Files: " + strings.Join(files, ", ")
	}
}

func (d *demo) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Text(c, "Clipboard and drag data").FontSize(24).Bold()
		ui.Text(c, "Copy or drag the same note as text, HTML, or lazy JSON. Paste in another app to use its preferred representation.").Wrap().TextColor(t.TextMuted)
		ui.TextInput(c, &d.note.Title).Placeholder("Title")
		ui.TextArea(c, &d.note.Body).Height(90)
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "Copy note").Clicked() {
				err := mygo.Clipboard.Write(d.data(), mygo.ClipboardOptions{OnRelease: func() { d.releases++; d.win.Invalidate() }})
				if err != nil {
					d.status = err.Error()
				} else {
					d.status = "Copied; JSON is produced only if requested"
				}
			}
			if ui.Button(c, "Paste").Clicked() {
				preferred, _ := transfer.PreferredFormat(mygo.Clipboard.Formats(), noteFormat, transfer.Text, transfer.FileList, transfer.URIList)
				if preferred == "" {
					d.status = "No supported clipboard format"
				} else if data, err := mygo.Clipboard.Read(preferred); err != nil {
					d.status = err.Error()
				} else {
					d.receive(data)
				}
			}
			if ui.Button(c, "Flush").Clicked() {
				if err := mygo.Clipboard.Flush(); err != nil {
					d.status = err.Error()
				} else {
					d.status = "Clipboard persisted; provider resources released"
				}
			}
		})
		ui.Box(c).Padding(16).Border(1, t.Border).Radius(8).DragDataFrom(d.data).Children(func() { ui.Text(c, "Drag this note").Bold() })
		zone := ui.Box(c).Padding(16).Border(1, t.Border).Radius(8)
		opts := transfer.DropOptions{Formats: []transfer.Format{noteFormat, transfer.Text, transfer.FileList, transfer.URIList}}
		if _, over := ui.DataDragOver(zone, opts); over {
			zone.Border(2, t.Accent)
		}
		zone.Children(func() { ui.Text(c, "Drop notes, text, or files here") })
		if drop, ok := ui.DropData(zone, opts); ok {
			d.receive(drop.Data)
		}
		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, &d.path).Grow(1).Placeholder("Absolute file path")
			if ui.Button(c, "Copy file").Clicked() {
				if err := mygo.Clipboard.WriteFiles(d.path); err != nil {
					d.status = err.Error()
				} else {
					d.status = "Copied native file list"
				}
			}
		})
		ui.Text(c, fmt.Sprintf("JSON requests: %d · Clipboard releases: %d", d.providers, d.releases)).TextColor(t.TextMuted)
		ui.Text(c, d.status).Wrap()
	})
}

func main() {
	d := &demo{note: note{Title: "A portable note", Body: "Shared clipboard and drag representations."}}
	mygo.App.WhenReady(func() {
		d.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGo Clipboard", Width: 640, Height: 580, Content: ui.View(d.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
