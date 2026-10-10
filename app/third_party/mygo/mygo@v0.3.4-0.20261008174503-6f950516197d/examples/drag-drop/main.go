// Drag-drop exchanges notes, text, URLs and files between two native UI
// windows and other applications. Hold Option (macOS) or Ctrl (elsewhere)
// to request a copy; Cmd (macOS) or Shift (elsewhere) requests a move.
//
// go run ./examples/drag-drop
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

const noteFormat transfer.Format = "application/vnd.mygo.example-note+json"

type note struct {
	ID    int
	Title string
}
type pane struct {
	name   string
	notes  []*note
	file   string
	status string
	win    *mygo.Window
	app    *demo
}
type demo struct{ nextID int }

func (p *pane) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(20).Gap(12).Children(func() {
		ui.Text(c, p.name).FontSize(24).Bold()
		ui.Text(c, "Drag a card to the other window, a text editor, or Finder/Explorer. Drop files, text, and URLs here.").Wrap().TextColor(t.TextMuted)
		for _, n := range p.notes {
			n := n
			card := ui.Box(c.Key(n.ID)).Padding(14).Radius(8).Border(1, t.Border).Background(t.Surface)
			card.Drag(n).DragDataFrom(func() transfer.Data {
				frozen := *n
				return transfer.New(transfer.NewItem(transfer.Bytes(transfer.Text, []byte(frozen.Title)), transfer.Lazy(noteFormat, func() ([]byte, error) { return json.Marshal(frozen) })))
			}, transfer.DragOptions{Operations: transfer.Copy | transfer.Move, Done: func(r transfer.Result) {
				p.completed(r)
				if r.Operation == transfer.Move && r.Err == nil {
					p.notes = slices.DeleteFunc(p.notes, func(v *note) bool { return v == n })
				}
			}}).Children(func() { ui.Text(c, n.Title) })
			if card.Dragging() {
				card.Opacity(0.5)
			}
		}
		url, _ := transfer.URLData("https://mygo.egoist.dev")
		ui.Box(c).Padding(12).Border(1, t.Border).Radius(8).DragData(url, transfer.DragOptions{Done: p.completed}).Children(func() { ui.Text(c, "Drag the MyGo website URL") })
		files, _ := transfer.FileData(p.file)
		ui.Box(c).Padding(12).Border(1, t.Border).Radius(8).DragData(files, transfer.DragOptions{Done: p.completed}).Children(func() { ui.Text(c, "Drag a sample .txt file") })
		zone := ui.Box(c).Grow(1).MinHeight(100).Padding(16).Radius(8).Border(2, t.Border)
		opts := transfer.DropOptions{Formats: []transfer.Format{noteFormat, transfer.Text, transfer.FileList, transfer.URIList}, Operations: transfer.Copy | transfer.Move}
		if _, over := ui.DataDragOver(zone, opts); over {
			zone.Border(2, t.Accent).Background(t.Surface)
		}
		zone.Children(func() { ui.Text(c, "Drop here").Bold() })
		if drop, ok := ui.DropData(zone, opts); ok {
			p.receive(drop)
		}
		ui.Text(c, p.status).Wrap().FontSize(12).TextColor(t.TextMuted)
	})
}

func (p *pane) completed(r transfer.Result) {
	switch {
	case r.Err != nil:
		p.status = r.Err.Error()
	case r.Canceled:
		p.status = "Drag canceled"
	case r.Operation == transfer.Move:
		p.status = "Move completed"
	default:
		p.status = "Copy completed"
	}
	log.Printf("%s: %s", p.name, p.status)
}

func (p *pane) receive(drop transfer.Drop) {
	if b, err := drop.Data.Read(noteFormat); err == nil {
		var n note
		if err := json.Unmarshal(b, &n); err != nil {
			p.status = err.Error()
			return
		}
		p.app.nextID++
		n.ID = p.app.nextID
		p.notes = append(p.notes, &n)
		p.status = "Received note: " + n.Title
	} else if paths, err := drop.Data.Files(); err == nil && len(paths) > 0 {
		p.status = "Received files: " + strings.Join(paths, ", ")
	} else if urls, err := drop.Data.URLs(); err == nil && len(urls) > 0 {
		p.status = "Received URLs: " + strings.Join(urls, ", ")
	} else if b, err := drop.Data.Read(transfer.Text); err == nil {
		p.status = "Received text: " + string(b)
	}
	log.Printf("%s: %s (effect %d)", p.name, p.status, drop.Operation)
}

func main() {
	dir, err := os.MkdirTemp("", "mygo-drag-demo-")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(dir)
	file := dir + "/MyGo drag sample.txt"
	if err := os.WriteFile(file, []byte("A file dragged from MyGo native UI.\n"), 0600); err != nil {
		log.Fatal(err)
	}
	app := &demo{nextID: 2}
	mygo.App.WhenReady(func() {
		for i, name := range []string{"Left", "Right"} {
			p := &pane{name: name, file: file, app: app, status: "Escape cancels a drag.", notes: []*note{{ID: i + 1, Title: fmt.Sprintf("A note from %s", name)}}}
			p.win = mygo.NewWindow(mygo.WindowOptions{Title: "MyGo Drag — " + name, Width: 440, Height: 580, X: 60 + i*480, Y: 80, Content: ui.View(p.view)})
		}
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
