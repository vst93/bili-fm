// A custom text field built from MyGo's text-input and layout primitives.
// Run with: go run ./examples/text-input
package main

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"log"
)

// field is application-owned state. A larger editor can substitute its own
// buffer, selections, range attributes and history without changing the API.
type field struct {
	text      string
	selection ui.TextInputSelection
	marked    *ui.TextInputRange
	layout    *ui.TextLayout
}

func (f *field) shape() { f.layout = ui.ShapeText(f.text, ui.Font{Size: 18}, 0) }
func (f *field) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	a, z := ui.UTF16ByteOffset(f.text, r.Start), ui.UTF16ByteOffset(f.text, r.End)
	return f.text[a:z], ui.TextInputRange{Start: ui.UTF16Len(f.text[:a]), End: ui.UTF16Len(f.text[:z])}
}
func (f *field) Selection() ui.TextInputSelection { return f.selection }
func (f *field) MarkedRange() (ui.TextInputRange, bool) {
	if f.marked != nil {
		return *f.marked, true
	}
	return ui.TextInputRange{}, false
}
func (f *field) target(r *ui.TextInputRange) ui.TextInputRange {
	if r != nil {
		return *r
	}
	if f.marked != nil {
		return *f.marked
	}
	return f.selection.Range
}
func (f *field) ReplaceText(r *ui.TextInputRange, text string) {
	wanted := f.target(r)
	a, z := ui.UTF16ByteOffset(f.text, wanted.Start), ui.UTF16ByteOffset(f.text, wanted.End)
	f.text = f.text[:a] + text + f.text[z:]
	end := wanted.Start + ui.UTF16Len(text)
	f.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	f.marked = nil
	f.shape()
}
func (f *field) SetMarkedText(r *ui.TextInputRange, text string, selected ui.TextInputRange) {
	wanted := f.target(r)
	f.ReplaceText(&wanted, text)
	mark := ui.TextInputRange{Start: wanted.Start, End: wanted.Start + ui.UTF16Len(text)}
	f.marked = &mark
	f.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: mark.Start + selected.Start, End: mark.Start + selected.End}}
}
func (f *field) UnmarkText() { f.marked = nil }
func (f *field) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	b := f.layout.Caret(r.Start)
	if r.Start != r.End {
		rects := f.layout.SelectionRects(r)
		if len(rects) > 0 {
			b = rects[0]
		}
	}
	b.X += 12
	b.Y += 12
	return b, r, true
}
func (f *field) IndexForPoint(p ui.Point) (int, bool) {
	return f.layout.IndexAt(ui.Point{X: p.X - 12, Y: p.Y - 12}), true
}
func (f *field) move(to int, extend bool) {
	if !extend {
		f.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: to, End: to}}
		return
	}
	anchor := f.selection.Range.Start
	if f.selection.Reversed {
		anchor = f.selection.Range.End
	}
	f.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: min(anchor, to), End: max(anchor, to)}, Reversed: to < anchor}
}
func (f *field) input(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		to, _ := f.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
		f.move(to, ev.Mods&ui.Shift != 0)
		return true
	case ui.InputPointerMove:
		if ev.Button == 0 {
			to, _ := f.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
			f.move(to, true)
			return true
		}
	case ui.InputPointerUp:
		return true
	case ui.InputKeyDown:
		if ev.Mods == ui.Cmd && ev.Key == ui.KeyA {
			f.selection = ui.TextInputSelection{Range: ui.TextInputRange{End: ui.UTF16Len(f.text)}}
			return true
		}
		switch ev.Key {
		case ui.KeyHome:
			f.move(0, ev.Mods&ui.Shift != 0)
			return true
		case ui.KeyEnd:
			f.move(ui.UTF16Len(f.text), ev.Mods&ui.Shift != 0)
			return true
		case ui.KeyBackspace, ui.KeyDelete:
			r := f.selection.Range
			if r.Start == r.End {
				next := f.layout.PreviousBoundary(r.Start)
				if ev.Key == ui.KeyDelete {
					next = f.layout.NextBoundary(r.End)
				}
				r = ui.TextInputRange{Start: min(next, r.Start), End: max(next, r.End)}
			}
			f.ReplaceText(&r, "")
			return true
		}
	case ui.InputCommand:
		switch ev.Text {
		case "selectAll":
			f.selection = ui.TextInputSelection{Range: ui.TextInputRange{End: ui.UTF16Len(f.text)}}
		case "copy", "cut":
			s, _ := f.TextForRange(f.selection.Range)
			mygo.Clipboard.WriteText(s)
			if ev.Text == "cut" {
				f.ReplaceText(nil, "")
			}
		case "paste":
			f.ReplaceText(nil, mygo.Clipboard.ReadText())
		case "delete":
			f.ReplaceText(nil, "")
		default:
			return false
		}
		return true
	}
	return false
}
func (f *field) view(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Text(c, "Application-owned text input").FontSize(22).Bold()
		ui.Text(c, "Click, type, compose with your input method, select with Shift, and use the Edit menu.").TextColor(t.TextMuted)
		ui.Box(c).Height(60).Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Label("Custom text field").Role(ui.RoleTextField).Value(f.text).HandleTextInput(f).AutoFocus().HandleInput(func(ev ui.InputEvent) bool { return f.input(ev) }).Draw(func(p *ui.Painter, r ui.Rect) {
			for _, b := range f.layout.SelectionRects(f.selection.Range) {
				b.X += r.X + 12
				b.Y += r.Y + 12
				p.Fill(b, t.Selection, 0)
			}
			p.TextLayout(f.layout, r.X+12, r.Y+12, t.Text)
			b := f.layout.Caret(f.selection.Caret())
			b.X += r.X + 12
			b.Y += r.Y + 12
			p.Fill(b, t.Accent, 0)
		})
		ui.Text(c, "Storage, navigation, clipboard policy and rendering belong to this example; the primitive supplies native text services.").TextColor(t.TextMuted)
	})
}
func main() {
	f := &field{text: "Type here: é, 😀, العربية, עברית"}
	end := ui.UTF16Len(f.text)
	f.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	f.shape()
	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
		mygo.NewWindow(mygo.WindowOptions{Title: "Text input primitive", Width: 760, Height: 240, Content: ui.View(f.view)})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
