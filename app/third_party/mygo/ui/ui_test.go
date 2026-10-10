package ui

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// savePNG writes the tester's frame into the directory MYGO_UI_PNG names,
// for looking at.
func savePNG(t *testing.T, tt *Tester, name string) {
	dir := os.Getenv("MYGO_UI_PNG")
	if dir == "" {
		return
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

type demo struct {
	count    int
	name     string
	agree    bool
	dark     bool
	volume   float64
	choice   string
	size     string
	notes    string
	selected int
}

func (d *demo) view(c *context) {
	t := c.Theme()
	coreColumn(c).Fill().Padding(20).Gap(14).Children(func() {
		coreRow(c).Gap(10).Children(func() {
			coreText(c, "MyGo UI").FontSize(24).Bold()
			coreSpacer(c)
			coreText(c, "GPU rendered, pure Go").TextColor(t.TextMuted)
		})
		coreRow(c).Gap(8).Children(func() {
			if coreButton(c, "Increment").Clicked() {
				d.count++
			}
			if corePrimaryButton(c, "Reset").Clicked() {
				d.count = 0
			}
			coreTextf(c, "Count: %d", d.count)
		})
		coreRow(c).Gap(16).Children(func() {
			coreCheckbox(c, &d.agree, "I agree")
			coreSwitch(c, &d.dark)
			coreRadio(c, &d.choice, "a", "Alpha")
			coreRadio(c, &d.choice, "b", "Beta")
		})
		coreRow(c).Gap(10).Children(func() {
			coreTextInput(c, &d.name).Placeholder("Your name").Grow(1)
			coreSelect(c, &d.size, []string{"Small", "Medium", "Large"})
		})
		coreSlider(c, &d.volume, 0, 100)
		coreProgress(c, d.volume/100)
		coreBox(c).Padding(12).Radius(10).Background(t.Surface).Border(1, t.Border).Shadow(0, 2, 10, 0, RGBA(0, 0, 0, 0.12)).Children(func() {
			coreText(c, "A card with a shadow and a long text that wraps across lines when the window is narrow enough to need it.")
		})
		coreList(c, nil, 1000, func(i int) {
			row := coreRow(c).Height(28).PaddingX(8).Gap(8)
			if i == d.selected {
				row.Background(t.Accent).TextColor(t.AccentText).Radius(4)
			}
			if row.Clicked() {
				d.selected = i
			}
			row.Children(func() {
				coreTextf(c, "Row %d", i).Grow(1)
				coreText(c, fmt.Sprint(i*i)).TextColor(t.TextMuted)
			})
		}).Grow(1).Border(1, t.Border).Radius(6)
	})
}

func TestDemoRenders(t *testing.T) {
	d := &demo{choice: "a", size: "Medium", volume: 40}
	tt := coreNewTester(d.view, 640, 600)
	savePNG(t, tt, "demo")
	for _, s := range []string{"MyGo UI", "Count: 0", "I agree", "Row 0", "Medium"} {
		if _, ok := tt.Find(s); !ok {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	tt.SetScale(2)
	savePNG(t, tt, "demo@2x")
	tt.SetDark(true)
	savePNG(t, tt, "demo-dark")
}

func TestClickCounts(t *testing.T) {
	d := &demo{}
	tt := coreNewTester(d.view, 640, 600)
	for i := 0; i < 3; i++ {
		if err := tt.Click("Increment"); err != nil {
			t.Fatal(err)
		}
	}
	if d.count != 3 || !tt.HasText("Count: 3") {
		t.Fatalf("count %d, texts %q", d.count, tt.Texts())
	}
	tt.Click("I agree")
	if !d.agree {
		t.Error("the checkbox did not toggle")
	}
	tt.Click("Beta")
	if d.choice != "b" {
		t.Errorf("radio chose %q", d.choice)
	}
}

func TestTyping(t *testing.T) {
	d := &demo{}
	tt := coreNewTester(d.view, 640, 600)
	r, ok := tt.Find("Your name")
	if !ok {
		// The placeholder is painted, not an element: find the input by
		// its neighbor instead.
		r, _ = tt.Find("I agree")
		r.Y += 40
	}
	tt.ClickAt(r.X+20, r.Y+r.H/2)
	tt.Type("Héllo wörld")
	if d.name != "Héllo wörld" {
		t.Fatalf("typed %q", d.name)
	}
	tt.Key(0, KeyBackspace)
	word := Ctrl // the modifier that moves by words
	if runtime.GOOS == "darwin" {
		word = Alt
	}
	tt.Key(word, KeyLeft)
	tt.Type("big ")
	if d.name != "Héllo big wörl" {
		t.Fatalf("edited to %q", d.name)
	}
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Héllo big wörl" {
		t.Errorf("copied %q", tt.Clipboard())
	}
	tt.Key(Cmd, KeyZ)
	if d.name != "Héllo wörl" {
		t.Errorf("undid to %q", d.name)
	}
	savePNG(t, tt, "typing")
}

// TestTextInputFollowsTheFocusAtOnce checks that input moving the focus
// turns text input on or off before the next frame: a platform sends the
// text of keys to the view only while it is on, and keys may come before
// that frame.
func TestTextInputFollowsTheFocusAtOnce(t *testing.T) {
	d := &demo{}
	tt := coreNewTester(d.view, 640, 600)
	r, _ := tt.Find("I agree")
	r.Y += 40
	x, y := float64(r.X+20), float64(r.Y+r.H/2)
	// Events without frames between them, as between two display refreshes.
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerDown, X: x, Y: y})
	if !tt.h.ime.Active {
		t.Fatal("pressing the text input turned text input on only at the next frame")
	}
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerUp, X: x, Y: y})
	tt.rt.event(platform.SurfaceEvent{Kind: platform.TextInput, Text: "Ada"})
	tt.rt.event(platform.SurfaceEvent{Kind: platform.KeyPressed, Key: platform.KeyTab})
	if tt.h.ime.Active {
		t.Error("Tab out of the text input turned text input off only at the next frame")
	}
	tt.Frame()
	if d.name != "Ada" {
		t.Errorf("typed %q", d.name)
	}
}

// TestKeyOnAWidget checks that keying a widget which used its state as it
// was created panics rather than lose its input: the radio would never
// see its clicks.
func TestKeyOnAWidget(t *testing.T) {
	defer func() {
		if r := recover(); r == nil || !strings.Contains(fmt.Sprint(r), "Key on a Radio") {
			t.Errorf("Key on a Radio: recovered %v", r)
		}
	}()
	choice := "a"
	coreNewTester(func(c *context) {
		coreRow(c).Key("ok").Children(func() {
			coreRadio(c, &choice, "b", "Beta").Key("b")
		})
	}, 200, 100)
}

func TestTitleBar(t *testing.T) {
	var got TitleBar
	tt := coreNewTester(func(c *context) { got = c.TitleBar() }, 200, 100)
	if got != (TitleBar{}) {
		t.Errorf("TitleBar without one = %+v", got)
	}
	tt.SetTitleBar(TitleBar{Height: 28, Left: 72})
	if got != (TitleBar{Height: 28, Left: 72}) {
		t.Errorf("TitleBar = %+v", got)
	}
}

// TestEmacsKeys checks the Control keys of macOS text fields.
func TestEmacsKeys(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only macOS text fields have them")
	}
	text := "one two\nthree"
	tt := coreNewTester(func(c *context) { coreTextArea(c, &text).AutoFocus().Fill() }, 300, 200)
	tt.Key(Ctrl, KeyA) // the start of "three"
	tt.Type(">")
	if text != "one two\n>three" {
		t.Fatalf("Control-A, then typing: %q", text)
	}
	tt.Key(Ctrl, KeyP) // up into "one two"
	tt.Key(Ctrl, KeyE)
	tt.Key(Ctrl, KeyK) // at the end, joins the next paragraph
	if text != "one two>three" {
		t.Fatalf("Control-P, -E, -K: %q", text)
	}
	tt.Key(Ctrl, KeyA)
	tt.Key(Ctrl, KeyF)
	tt.Key(Ctrl, KeyD)
	if text != "oe two>three" {
		t.Fatalf("Control-A, -F, -D: %q", text)
	}
	tt.Key(Ctrl, KeyK)
	tt.Key(Ctrl, KeyH)
	if text != "" {
		t.Errorf("Control-K, -H: %q", text)
	}
}

func TestListScrollsAndSelects(t *testing.T) {
	d := &demo{}
	tt := coreNewTester(d.view, 640, 600)
	r, ok := tt.Find("Row 0")
	if !ok {
		t.Fatal("no first row")
	}
	tt.Scroll(r.X+10, r.Y+10, 0, 28*50)
	if _, ok := tt.Find("Row 0"); ok {
		t.Error("row 0 still built after scrolling")
	}
	if !tt.HasText("Row 52") {
		t.Errorf("row 52 not in view: %q", tt.Texts())
	}
	tt.Click("Row 52")
	if d.selected != 52 {
		t.Errorf("selected %d", d.selected)
	}
	savePNG(t, tt, "scrolled")
}

func TestTabFocus(t *testing.T) {
	d := &demo{}
	tt := coreNewTester(d.view, 640, 600)
	tt.Key(0, KeyTab)
	if !tt.Focused("Increment") {
		t.Fatal("Tab did not focus the first button")
	}
	tt.Key(0, KeyEnter)
	if d.count != 1 {
		t.Errorf("Enter on the focused button: count %d", d.count)
	}
	tt.Key(0, KeyTab)
	if !tt.Focused("Reset") {
		t.Error("Tab did not move to the second button")
	}
	savePNG(t, tt, "focus")
}

func TestAutoFocus(t *testing.T) {
	var open bool
	var name string
	view := func(c *context) {
		if coreButton(c, "Rename").Clicked() {
			open = true
		}
		coreModal(c, &open, func() {
			coreTextInput(c, &name).Label("Name").AutoFocus()
			if coreButton(c, "Done").Clicked() {
				open = false
			}
		})
	}
	tt := coreNewTester(view, 400, 300)
	tt.Click("Rename")
	if !tt.Focused("Name") {
		t.Fatal("the input of the opened dialog has no focus")
	}
	tt.Type("Ada")
	tt.Key(0, KeyTab)
	tt.Frame()
	if !tt.Focused("Done") {
		t.Fatal("Tab did not move the focus away from the input")
	}
	tt.Key(0, KeyEnter)
	if open || name != "Ada" {
		t.Fatalf("open %v, name %q", open, name)
	}
	tt.Click("Rename")
	if !tt.Focused("Name") {
		t.Error("the input of the reopened dialog has no focus")
	}
}

// listScroll returns how far the demo's list of a thousand rows scrolled,
// and its height.
func listScroll(tt *Tester) (y, h float32) {
	for _, s := range tt.rt.states {
		if s.flags&flagScrollY != 0 && s.contentH > 1000*28-1 {
			return float32(s.scrollY), s.h
		}
	}
	return -1, 0
}

func TestKeyboardScrolling(t *testing.T) {
	d := &demo{volume: 40}
	tt := coreNewTester(d.view, 640, 600)
	row, _ := tt.Find("Row 1")
	// Without a focus, keys scroll what the pointer is over, although the
	// slider elsewhere takes arrow keys while it has the focus.
	tt.Move(row.X+10, row.Y+10)
	tt.Key(0, KeyDown)
	if y, _ := listScroll(tt); y != 40 {
		t.Fatalf("Down scrolled to %v", y)
	}
	tt.Key(0, KeyPageDown)
	if y, h := listScroll(tt); abs32(y-h) > 0.01 {
		t.Errorf("PageDown scrolled to %v, the list being %v high", y, h)
	}
	tt.Key(0, KeyEnd)
	if y, h := listScroll(tt); abs32(y+h-1000*28-2) > 0.01 { // 2: the border
		t.Errorf("End scrolled to %v, the list being %v high", y, h)
	}
	if !tt.HasText("Row 999") {
		t.Error("the last row is not in view")
	}
	tt.Key(Shift, KeySpace)
	tt.Key(0, KeyHome)
	if y, _ := listScroll(tt); y != 0 {
		t.Errorf("Home scrolled to %v", y)
	}

	// The focused slider takes the arrow keys.
	tt.Key(0, KeyTab)
	if !tt.Focused("Increment") {
		t.Fatal("no first focus")
	}
	for i := 0; i < 20 && d.volume == 40; i++ {
		tt.Key(0, KeyTab)
		tt.Key(0, KeyRight)
	}
	if d.volume != 41 {
		t.Fatalf("the slider is at %v", d.volume)
	}
	tt.Key(0, KeyDown)
	if y, _ := listScroll(tt); d.volume != 40 || y != 0 {
		t.Errorf("Down moved the slider to %v and the list to %v", d.volume, y)
	}
}

// TestElementsGoneInARebuild forgets the elements a frame built, then left
// out when it built again: a field that comes back is new, and takes the
// focus with AutoFocus again.
func TestElementsGoneInARebuild(t *testing.T) {
	text, open := "", true
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(8).Children(func() {
			opener := coreButton(c, "Open")
			if opener.Clicked() {
				open = true
			}
			if open {
				coreTextInput(c, &text).AutoFocus().Label("Field")
				if coreButton(c, "Close").Clicked() {
					open = false
					opener.Focus()
				}
			}
		})
	}, 300, 200)
	if !tt.Focused("Field") {
		t.Fatal("the field does not take the focus at first")
	}
	tt.Click("Close")
	tt.Key(0, KeyEnter) // on Open
	if !open || !tt.Focused("Field") {
		t.Errorf("the field that came back does not take the focus (open %v)", open)
	}
}

// TestRecycledStateIsFresh checks that an element taking the state of one
// that went away, as rows coming into a list's view do, keeps nothing of
// it.
func TestRecycledStateIsFresh(t *testing.T) {
	key, inits := "a", 0
	var st *state
	tt := coreNewTester(func(c *context) {
		if key == "" {
			return
		}
		e := coreScroll(c).Key(key).Size(100, 50).Children(func() { coreBox(c).Height(200) })
		n := coreLocal(e, "n", func() int { inits++; return 1 })
		*n *= 2
		e.Clicked()
		st = e.st
	}, 200, 200)
	tt.Scroll(10, 10, 0, 30)
	a := st
	if a.scrollY == 0 || len(a.locals) == 0 {
		t.Fatalf("a kept no scroll offset (%v) or local", a.scrollY)
	}
	key = ""
	tt.Frame() // a goes, and its states are free
	gone := map[*state]bool{}
	for _, s := range tt.rt.free {
		gone[s] = true
	}
	if !gone[a] {
		t.Fatal("a's state is not free")
	}
	key = "b"
	tt.Frame()
	if !gone[st] {
		t.Fatal("b made a state rather than take one that is free")
	}
	if st.scrollY != 0 || st.clicks != 0 || inits != 2 || *st.locals["n"].(*int) != 2 {
		t.Errorf("b took a's scroll offset %v, clicks %d, or local (%d inits)", st.scrollY, st.clicks, inits)
	}
}

// TestHoverWhilePressed checks that the elements around the one pressed
// stay hovered while the pointer is over them: a button that shows over a
// row is still there to take its release. Others hover no more.
func TestHoverWhilePressed(t *testing.T) {
	var rowHovered, otherHovered bool
	removes, keeps := 0, 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Children(func() {
			row := coreRow(c).Size(200, 40)
			rowHovered = row.Hovered()
			row.Children(func() {
				if coreButton(c, "Keep").Clicked() {
					keeps++
				}
				coreSpacer(c)
				if rowHovered && coreButton(c, "Remove").Clicked() {
					removes++
				}
			})
			other := coreBox(c).Size(200, 40)
			otherHovered = other.Hovered()
		})
	}, 300, 200)
	press := func(label string) (x, y float32) {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("no %q; texts %q", label, tt.Texts())
		}
		x, y = r.X+r.W/2, r.Y+r.H/2
		tt.Press(x, y)
		tt.Frame()
		return x, y
	}
	tt.Move(150, 20)
	tt.Frame()
	// The button showing over the row takes its click.
	x, y := press("Remove")
	if !rowHovered {
		t.Error("pressing the button unhovered the row around it")
	}
	tt.Release(x, y)
	tt.Frame()
	if removes != 1 {
		t.Fatalf("%d clicks of the button showing over the row", removes)
	}
	// Pressing a button and leaving the row for another element: neither
	// hovers until the release.
	press("Keep")
	tt.Move(20, 60)
	tt.Frame()
	if rowHovered || otherHovered {
		t.Errorf("pressing a button, the pointer elsewhere: the row hovered %v, the element under the pointer %v", rowHovered, otherHovered)
	}
	tt.Release(20, 60)
	tt.Frame()
	if !otherHovered || keeps != 0 {
		t.Errorf("after the release: the element under the pointer hovered %v, %d clicks", otherHovered, keeps)
	}
	// Pressing the button over the row and leaving the row, which hides
	// the button: the element under the pointer hovers then.
	tt.Move(150, 20)
	tt.Frame()
	x, y = press("Remove")
	tt.Move(20, 60)
	tt.Frame()
	if rowHovered || !otherHovered || tt.HasText("Remove") {
		t.Errorf("the button over the row gone: the row hovered %v, the element under the pointer %v", rowHovered, otherHovered)
	}
	tt.Release(20, 60)
	tt.Frame()
	if removes != 1 {
		t.Errorf("%d clicks of the button, released away from it", removes)
	}
}
