package vt

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/plugins/terminal/internal/library"
)

// loadLib loads the libghostty-vt of the terminal plugin, which it
// downloads into the user's cache when missing, or the one
// $MYGO_GHOSTTY_VT names; -short skips the tests then.
func loadLib(t *testing.T) {
	t.Helper()
	if testing.Short() && os.Getenv("MYGO_GHOSTTY_VT") == "" {
		t.Skip("downloads libghostty-vt")
	}
	manifest, err := os.ReadFile("../../mygo-plugin.json")
	if err != nil {
		t.Fatal(err)
	}
	path, err := library.Find(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := Load(path); err != nil {
		t.Fatal(err)
	}
}

func rowText(r *RenderState) []string {
	var rows []string
	r.Rows()
	for r.NextRow() {
		var b strings.Builder
		for _, c := range r.RowCells() {
			if c.Codepoint == 0 {
				b.WriteByte(' ')
			} else if c.Wide != SpacerTail {
				b.WriteRune(c.Codepoint)
			}
		}
		rows = append(rows, strings.TrimRight(b.String(), " "))
	}
	return rows
}

func TestTerminal(t *testing.T) {
	loadLib(t)
	var replies []string
	var titled bool
	term, err := NewTerminal(20, 5, Effects{
		WritePTY:     func(p []byte) { replies = append(replies, string(p)) },
		TitleChanged: func() { titled = true },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Free()
	term.Write([]byte("Hello, \x1b[1;32mworld\x1b[0m!\r\nsecond line\x1b]2;the title\x07\x1b[6n"))
	if want := []string{"\x1b[2;12R"}; strings.Join(replies, "") != want[0] {
		t.Errorf("replies = %q, want %q", replies, want)
	}
	if !titled || term.Title() != "the title" {
		t.Errorf("title = %q (changed %v)", term.Title(), titled)
	}
	r, err := NewRenderState()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Free()
	if err := r.Update(term); err != nil {
		t.Fatal(err)
	}
	if cols, rows := r.Size(); cols != 20 || rows != 5 {
		t.Errorf("size = %d×%d", cols, rows)
	}
	if got := rowText(r); got[0] != "Hello, world!" || got[1] != "second line" {
		t.Errorf("rows = %q", got)
	}
	r.Rows()
	r.NextRow()
	cells := r.RowCells()
	if cells[7].Style == 0 {
		t.Fatal("w has the default style")
	}
	s := r.CellStyle(7)
	if !s.Bold || s.FG != (Color{Kind: 1, Index: 2}) {
		t.Errorf("style of w = %+v", s)
	}
	if got := term.Text(); got != "Hello, world!\nsecond line" {
		t.Errorf("text = %q", got)
	}
	if c := r.Cursor(); !c.InView || c.X != 11 || c.Y != 1 {
		t.Errorf("cursor = %+v", c)
	}
}

func TestWideAndGraphemes(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(10, 2, Effects{})
	defer term.Free()
	term.Write([]byte("\x1b[?2027h世👍🏽"))
	r, _ := NewRenderState()
	defer r.Free()
	r.Update(term)
	r.Rows()
	r.NextRow()
	cells := r.RowCells()
	if cells[0].Codepoint != '世' || cells[0].Wide != WideChar || cells[1].Wide != SpacerTail {
		t.Errorf("cells = %+v", cells[:2])
	}
	if !cells[2].Grapheme {
		t.Fatalf("cell 2 = %+v", cells[2])
	}
	if got := string(r.CellGraphemes(2, nil)); got != "👍🏽" {
		t.Errorf("graphemes = %q", got)
	}
}

func TestEncoders(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(80, 24, Effects{})
	defer term.Free()
	k, err := NewKeyEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer k.Free()
	k.Sync(term, false)
	for _, c := range []struct {
		ev   KeyEvent
		want string
	}{
		{KeyEvent{Action: KeyPress, Key: KeyEnter}, "\r"},
		{KeyEvent{Action: KeyPress, Key: KeyC, Mods: ModCtrl, Unshifted: 'c'}, "\x03"},
		{KeyEvent{Action: KeyPress, Key: KeyArrowUp}, "\x1b[A"},
		{KeyEvent{Action: KeyPress, Key: KeyA, Text: "a", Unshifted: 'a'}, "a"},
		{KeyEvent{Action: KeyPress, Key: KeyA, Mods: ModShift, Consumed: ModShift, Text: "A", Unshifted: 'a'}, "A"},
		{KeyEvent{Action: KeyRelease, Key: KeyA, Unshifted: 'a'}, ""},
		{KeyEvent{Action: KeyPress, Key: KeyEscape}, "\x1b"},
		{KeyEvent{Action: KeyPress, Key: KeyF1}, "\x1bOP"},
	} {
		if got := string(k.Encode(c.ev)); got != c.want {
			t.Errorf("Encode(%+v) = %q, want %q", c.ev, got, c.want)
		}
	}
	term.Write([]byte("\x1b[?1h"))
	k.Sync(term, false)
	if got := string(k.Encode(KeyEvent{Action: KeyPress, Key: KeyArrowUp})); got != "\x1bOA" {
		t.Errorf("up in cursor key mode = %q", got)
	}

	m, err := NewMouseEncoder()
	if err != nil {
		t.Fatal(err)
	}
	defer m.Free()
	m.Sync(term, 800, 480, 10, 20, 0, 0, false)
	if got := m.Encode(MousePress, ButtonLeft, 0, 5, 5); got != nil {
		t.Errorf("press without tracking = %q", got)
	}
	term.Write([]byte("\x1b[?1000h\x1b[?1006h"))
	m.Sync(term, 800, 480, 10, 20, 0, 0, false)
	if got := string(m.Encode(MousePress, ButtonLeft, 0, 25, 45)); got != "\x1b[<0;3;3M" {
		t.Errorf("press = %q", got)
	}
	if got := string(m.Encode(MouseRelease, ButtonLeft, 0, 25, 45)); got != "\x1b[<0;3;3m" {
		t.Errorf("release = %q", got)
	}

	if got := string(EncodeFocus(true)); got != "\x1b[I" {
		t.Errorf("focus = %q", got)
	}
	if got := string(EncodePaste("a\nb", false)); got != "a\rb" {
		t.Errorf("paste = %q", got)
	}
	if got := string(EncodePaste("a\nb", true)); got != "\x1b[200~a\nb\x1b[201~" {
		t.Errorf("bracketed paste = %q", got)
	}
	if PasteIsSafe("rm -rf /\n") || !PasteIsSafe("ls") {
		t.Error("PasteIsSafe")
	}
}

func TestSelection(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(20, 3, Effects{})
	defer term.Free()
	term.Write([]byte("hello world\r\nsecond"))
	g, err := NewGesture(500e6, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Free(term)
	geo := Geometry{Columns: 20, CellWidth: 10, Height: 60}
	a, _ := term.CellAt(0, 0)
	g.Press(term, a, 2, 5, 1)
	b, _ := term.CellAt(4, 0)
	s, ok := g.Drag(term, b, 46, 5, geo, false)
	if !ok {
		t.Fatal("no selection")
	}
	term.SetSelection(&s)
	g.Release(term, &b)
	if text, ok := term.SelectionText(); !ok || text != "hello" {
		t.Errorf("selection = %q, %v", text, ok)
	}
	// A double click selects the word.
	c, _ := term.CellAt(7, 0)
	g.Press(term, c, 72, 5, 600e6)
	g.Release(term, &c)
	s, ok = g.Press(term, c, 72, 5, 700e6)
	if !ok {
		t.Fatal("double click selected nothing")
	}
	term.SetSelection(&s)
	if text, _ := term.SelectionText(); text != "world" {
		t.Errorf("word = %q", text)
	}
	all, _ := term.SelectAll()
	term.SetSelection(&all)
	if text, _ := term.SelectionText(); text != "hello world\nsecond" {
		t.Errorf("all = %q", text)
	}
	term.SetSelection(nil)
	if _, ok := term.SelectionText(); ok {
		t.Error("selection after clearing it")
	}
}

func TestScrollback(t *testing.T) {
	loadLib(t)
	term, _ := NewTerminal(10, 3, Effects{})
	defer term.Free()
	for i := range 10 {
		term.Write([]byte(strings.Repeat(string(rune('0'+i)), 3) + "\r\n"))
	}
	sb := term.Scrollbar()
	if sb.Total != 11 || sb.Len != 3 || sb.Offset != 8 || !term.AtBottom() {
		t.Errorf("scrollbar = %+v", sb)
	}
	term.ScrollBy(-2)
	if term.AtBottom() || term.Scrollbar().Offset != 6 {
		t.Errorf("after scrolling up: %+v", term.Scrollbar())
	}
	r, _ := NewRenderState()
	defer r.Free()
	r.Update(term)
	if got := rowText(r); got[0] != "666" {
		t.Errorf("rows = %q", got)
	}
	term.ScrollToBottom()
	if !term.AtBottom() {
		t.Error("not at the bottom")
	}
	term.Resize(5, 3, 10, 20)
	if cols, rows := term.Size(); cols != 5 || rows != 3 {
		t.Errorf("size = %d×%d", cols, rows)
	}
}
