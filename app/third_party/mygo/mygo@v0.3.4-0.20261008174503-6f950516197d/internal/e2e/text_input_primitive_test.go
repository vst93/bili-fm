package e2e

import (
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"runtime"
	"sync/atomic"
	"testing"
)

type nativeInputClient struct {
	text      string
	selection ui.TextInputSelection
	mark      *ui.TextInputRange
}

func (c *nativeInputClient) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	a, z := ui.UTF16ByteOffset(c.text, r.Start), ui.UTF16ByteOffset(c.text, r.End)
	return c.text[a:z], ui.TextInputRange{Start: ui.UTF16Len(c.text[:a]), End: ui.UTF16Len(c.text[:z])}
}
func (c *nativeInputClient) Selection() ui.TextInputSelection { return c.selection }
func (c *nativeInputClient) MarkedRange() (ui.TextInputRange, bool) {
	if c.mark != nil {
		return *c.mark, true
	}
	return ui.TextInputRange{}, false
}
func (c *nativeInputClient) target(r *ui.TextInputRange) ui.TextInputRange {
	if r != nil {
		return *r
	}
	if c.mark != nil {
		return *c.mark
	}
	return c.selection.Range
}
func (c *nativeInputClient) ReplaceText(r *ui.TextInputRange, s string) {
	wanted := c.target(r)
	a, z := ui.UTF16ByteOffset(c.text, wanted.Start), ui.UTF16ByteOffset(c.text, wanted.End)
	c.text = c.text[:a] + s + c.text[z:]
	end := wanted.Start + ui.UTF16Len(s)
	c.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	c.mark = nil
}
func (c *nativeInputClient) SetMarkedText(r *ui.TextInputRange, s string, selected ui.TextInputRange) {
	wanted := c.target(r)
	c.ReplaceText(&wanted, s)
	mark := ui.TextInputRange{Start: wanted.Start, End: wanted.Start + ui.UTF16Len(s)}
	c.mark = &mark
	c.selection = ui.TextInputSelection{Range: ui.TextInputRange{Start: mark.Start + selected.Start, End: mark.Start + selected.End}}
}
func (c *nativeInputClient) UnmarkText() { c.mark = nil }
func (c *nativeInputClient) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	return ui.Rect{X: float32(r.Start)*8 + 8, Y: 8, W: 1, H: 20}, r, true
}
func (c *nativeInputClient) IndexForPoint(p ui.Point) (int, bool) {
	return max(0, min(int((p.X-8)/8), ui.UTF16Len(c.text))), true
}

func TestContentWindowTextInputPrimitive(t *testing.T) {
	var frames atomic.Int32
	client := &nativeInputClient{text: "A😀cafe", selection: ui.TextInputSelection{Range: ui.TextInputRange{Start: 7, End: 7}}}
	view := func(c *ui.Context) {
		ui.Box(c).Size(350, 70).HandleTextInput(client).AutoFocus().Label("Application text").Role(ui.RoleTextField).Draw(func(p *ui.Painter, r ui.Rect) { p.Text(r.X+8, r.Y+8, client.text, 18, c.Theme().Text) })
		frames.Add(1)
	}
	w := newWindow(t, mygo.WindowOptions{Title: "Text input primitive", Width: 400, Height: 160, Content: ui.View(view)})
	eventually(t, "a client frame", func() bool { return frames.Load() > 0 })
	if _, _, ok := inputClient(w); !ok {
		t.Skip("native input automation unavailable")
	}
	if !click(w, 120, 20) {
		t.Skip("native click automation unavailable")
	}
	// macOS exposes full client-document UTF-16 positions. Other platforms'
	// test hooks report surrounding context in runes, so verify native editing
	// independently of that diagnostic coordinate system.
	from := 6
	if runtime.GOOS != "darwin" {
		from = 5
	}
	composeOver(w, "e", 1, false, from, 1)
	eventually(t, "client-owned preedit", func() bool {
		var ok bool
		mygo.RunOnMain(func() { ok = client.mark != nil && client.text == "A😀cafe" })
		return ok
	})
	composeOver(w, "é", 1, true, -1, 0)
	eventually(t, "client-owned commit", func() bool {
		var ok bool
		mygo.RunOnMain(func() { ok = client.mark == nil && client.text == "A😀café" })
		return ok
	})
}
