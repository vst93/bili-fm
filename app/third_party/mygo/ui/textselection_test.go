package ui

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func selectableAt(t *testing.T, tt *Tester, text string, at int) (float32, float32) {
	t.Helper()
	for _, s := range tt.rt.texts {
		if s.editor.source == text {
			x, y, h := s.editor.layout.Caret(at)
			return s.x + s.editor.originX + x, s.y + s.editor.originY + y + h/2
		}
	}
	t.Fatalf("no selectable paragraph %q", text)
	return 0, 0
}

func TestSelectableContainerMultiClick(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Padding(12).Gap(8).Children(func() {
			coreText(c, "Alpha beta")
			coreText(c, "Gamma delta")
		})
	}, 300, 150)
	for _, unit := range []int{2, 3} {
		x, y := selectableAt(t, tt, "Alpha beta", 7)
		tt.rt.lastPress.at = time.Time{}
		for range unit - 1 {
			tt.ClickAt(x, y)
		}
		tt.Press(x, y)
		x, y = selectableAt(t, tt, "Gamma delta", 2)
		tt.Move(x, y)
		tt.Release(x, y)
		tt.Key(Cmd, KeyC)
		want := "beta\nGamma"
		if unit == 3 {
			want = "Alpha beta\nGamma delta"
		}
		if tt.Clipboard() != want {
			t.Fatalf("%d-click drag copied %q, want %q", unit, tt.Clipboard(), want)
		}
	}
}

func TestSelectableContainerOrderAndOverlap(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreBox(c).Selectable().Size(300, 150).Children(func() {
			coreText(c, "Built first").Absolute().Top(80)
			coreText(c, "Underneath").Height(30)
			coreText(c, "On top").Absolute().Top(0).Width(250)
			coreText(c, "")
		})
	}, 400, 200)
	selectBetween(t, tt, "On top", 0, "On top", 6)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "On top" {
		t.Fatalf("the covered paragraph took the drag: %q", tt.Clipboard())
	}
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Built first\nUnderneath\nOn top\n" {
		t.Fatalf("copy lost build order or the empty paragraph: %q", tt.Clipboard())
	}
}

func TestSelectableContainerScroll(t *testing.T) {
	var scroll ScrollState
	var paragraphs []string
	for i := range 12 {
		paragraphs = append(paragraphs, fmt.Sprintf("Paragraph %02d", i))
	}
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Selectable().Size(300, 120).TrackScroll(&scroll).Gap(8).Children(func() {
			for _, p := range paragraphs {
				coreText(c, p).Shrink(0)
			}
		})
	}, 400, 200)
	x, y := selectableAt(t, tt, paragraphs[0], 0)
	tt.Press(x, y)
	tt.Move(280, 140) // outside the viewport, below the last visible text
	if scroll.Y == 0 {
		t.Fatal("dragging near the scroll edge did not scroll")
	}
	for i := 0; i < 10 && scroll.Y < scroll.MaxY; i++ {
		tt.Frame()
	}
	tt.Release(280, 140)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != strings.Join(paragraphs, "\n") {
		t.Fatalf("scrolling lost the anchor or selected text: %q", tt.Clipboard())
	}
	// Select All includes built paragraphs outside the viewport too.
	x, y = selectableAt(t, tt, paragraphs[len(paragraphs)-1], 3)
	tt.ClickAt(x, y)
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != strings.Join(paragraphs, "\n") {
		t.Fatalf("Select All copied only the visible paragraphs: %q", tt.Clipboard())
	}
}

func TestSelectableInlineParagraphRebuild(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreRichText(c).Selectable().Children(func() {
			coreText(c, "First ")
			coreText(c, "second").Bold()
		})
	}, 300, 100)
	selectBetween(t, tt, "First second", 0, "First second", 5)
	tt.Frame()
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "First" {
		t.Fatalf("rebuilding inline children lost their paragraph's selection: %q", tt.Clipboard())
	}
}

func selectBetween(t *testing.T, tt *Tester, first string, from int, last string, to int) {
	t.Helper()
	x0, y0 := selectableAt(t, tt, first, from)
	x1, y1 := selectableAt(t, tt, last, to)
	tt.rt.lastPress.at = time.Time{} // each drag is a new gesture
	tt.Press(x0, y0)
	tt.Move(x1, y1)
	tt.Release(x1, y1)
}

func TestSelectableContainer(t *testing.T) {
	const first, middle, last = "First paragraph.", "A styled paragraph.", "Second paragraph."
	tt := coreNewTester(func(c *context) {
		c.Theme().Selection = RGB(180, 210, 250)
		coreColumn(c).Selectable().Padding(20).Gap(12).Children(func() {
			coreText(c, first)
			coreBox(c).Children(func() {
				coreRichText(c, Span{Text: "A styled ", Weight: 700}, Span{Text: "paragraph."})
			})
			coreText(c, last)
		})
	}, 400, 200)
	for _, reverse := range []bool{false, true} {
		if reverse {
			selectBetween(t, tt, last, 6, first, 6)
		} else {
			selectBetween(t, tt, first, 6, last, 6)
		}
		tt.Key(Cmd, KeyC)
		if got, want := tt.Clipboard(), "paragraph.\nA styled paragraph.\nSecond"; got != want {
			t.Fatalf("reverse=%v: copied %q, want %q", reverse, got, want)
		}
		// Every selected paragraph paints its own highlight, including the
		// two that do not hold the keyboard focus.
		for _, name := range []string{first, middle, last} {
			r, _ := tt.Find(name)
			color := tt.rt.c.theme.Selection
			img, highlighted := tt.Image(), false
			for y := int(r.Y); y < int(r.Y+r.H); y++ {
				for x := int(r.X); x < int(r.X+r.W); x++ {
					p := img.RGBAAt(x, y)
					if p.R == color.R && p.G == color.G && p.B == color.B {
						highlighted = true
					}
				}
			}
			if !highlighted {
				t.Errorf("no selection painted in %q", name)
			}
		}
	}
	tt.SetClipboard("menu command")
	tt.Command("copy")
	if tt.Clipboard() != "paragraph.\nA styled paragraph.\nSecond" {
		t.Errorf("the application Copy command copied %q", tt.Clipboard())
	}
	x, y := selectableAt(t, tt, middle, 4)
	tt.RightClickAt(x, y)
	if !slices.Equal(tt.Menu(), []string{"Copy", "-", "Select All"}) {
		t.Fatalf("selection menu: %q", tt.Menu())
	}
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "paragraph.\nA styled paragraph.\nSecond" {
		t.Errorf("right-clicking the middle paragraph lost the shared selection: %q", tt.Clipboard())
	}
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != first+"\n"+middle+"\n"+last {
		t.Errorf("Select All copied %q", tt.Clipboard())
	}
	tt.Type("replacement")
	tt.Command("cut")
	tt.Command("delete")
	tt.Command("paste")
	if !tt.HasText(first) || !tt.HasText(middle) || !tt.HasText(last) || tt.h.ime.Active {
		t.Error("selectable text was edited or activated the input method")
	}
}

func TestSelectableContainerKeyboard(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Padding(20).Gap(10).Children(func() {
			coreText(c, "Alpha")
			coreText(c, "Beta")
		})
	}, 300, 150)
	selectBetween(t, tt, "Alpha", 5, "Alpha", 5)
	tt.Key(Shift, KeyRight) // the paragraph boundary
	tt.Key(Shift, KeyRight) // B
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "\nB" {
		t.Fatalf("extending right copied %q", tt.Clipboard())
	}
	tt.Key(Shift, KeyLeft)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "\n" {
		t.Fatalf("extending left copied %q", tt.Clipboard())
	}
	selectBetween(t, tt, "Beta", 0, "Beta", 0)
	tt.Key(Shift, KeyLeft)
	tt.Key(Shift, KeyLeft)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "a\n" {
		t.Fatalf("extending backwards copied %q", tt.Clipboard())
	}
	selectBetween(t, tt, "Alpha", 2, "Alpha", 2)
	x, y := selectableAt(t, tt, "Beta", 2)
	tt.ClickAtWith(Shift, x, y)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "pha\nBe" {
		t.Fatalf("Shift-click copied %q", tt.Clipboard())
	}
	selectBetween(t, tt, "Alpha", 0, "Alpha", 0)
	tt.Key(Shift, KeyDown)
	tt.Key(Shift, KeyEnd)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Alpha\nBeta" {
		t.Fatalf("extending down copied %q", tt.Clipboard())
	}
}

func TestSelectableContainerScopesAndControls(t *testing.T) {
	value, clicks := "editable", 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Gap(8).Children(func() {
			coreText(c, "Outer start")
			if coreButton(c, "Press me").Clicked() {
				clicks++
			}
			coreTextInput(c, &value).Label("Input")
			coreColumn(c).Selectable().Children(func() {
				coreText(c, "Inner first")
				coreText(c, "Inner second")
			})
			coreText(c, "Hidden").Invisible()
			coreBox(c).Disabled(true).Children(func() { coreText(c, "Disabled") })
			coreText(c, "Excluded").Unselectable()
			coreBox(c).Unselectable().Children(func() { coreText(c, "Excluded child") })
			coreText(c, "Outer end")
		})
		coreText(c, "Outside").Selectable()
	}, 400, 450)
	selectBetween(t, tt, "Outer start", 0, "Outer end", 9)
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Outer start\nOuter end" {
		t.Fatalf("outer scope copied %q", tt.Clipboard())
	}
	tt.Click("Excluded")
	tt.SetClipboard("kept")
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Error("a press on excluded text started a selection")
	}
	if r, ok := tt.Find("Excluded"); ok {
		tt.Move(center(r))
		if tt.Cursor() != CursorDefault {
			t.Errorf("excluded text has cursor %v", tt.Cursor())
		}
	}
	selectBetween(t, tt, "Inner first", 0, "Inner second", 12)
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Inner first\nInner second" {
		t.Fatalf("inner scope copied %q", tt.Clipboard())
	}
	if err := tt.Click("Press me"); err != nil || clicks != 1 {
		t.Fatalf("button no longer clicks: err=%v, clicks=%d", err, clicks)
	}
	tt.SetClipboard("kept")
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Error("copy after leaving the selection changed the clipboard")
	}
	tt.Click("Input")
	tt.Key(Cmd, KeyA)
	tt.Type("changed")
	if value != "changed" {
		t.Errorf("the input no longer edits its own text: %q", value)
	}
	selectBetween(t, tt, "Outside", 0, "Outside", 7)
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Outside" {
		t.Errorf("a standalone selection copied %q", tt.Clipboard())
	}
}

func TestSelectableContainerInlineAndRebuild(t *testing.T) {
	first, shown := "café 日本語 🎉", true
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Padding(12).MaxWidth(160).Gap(8).Children(func() {
			if shown {
				coreText(c, first).Key("first")
			}
			coreRichText(c).Key("rich").Selectable().Children(func() {
				coreText(c, "Read ")
				coreLink(c, "the guide", "https://example.com")
				coreText(c, " and more.")
			})
		})
	}, 300, 180)
	const rich = "Read the guide and more."
	selectBetween(t, tt, first, 5, rich, 14)
	tt.Frame()
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "日本語 🎉\nRead the guide" {
		t.Fatalf("inline/Unicode selection copied %q", tt.Clipboard())
	}
	// Reflow preserves IDs and rune offsets, even with inline children.
	tt.SetSize(110, 250)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "日本語 🎉\nRead the guide" {
		t.Fatal("rebuilding or resizing lost the selection")
	}
	first = "different text"
	tt.Frame()
	tt.SetClipboard("kept")
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Fatal("a selection survived a change to its text")
	}
	selectBetween(t, tt, first, 0, rich, 14)
	shown = false
	tt.Frame()
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Fatal("a selection survived removal of its endpoint")
	}
}

func TestSelectableContainerChangingOtherText(t *testing.T) {
	other := "Unselected paragraph"
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Selectable().Gap(8).Children(func() {
			coreText(c, other)
			coreText(c, "Selected first")
			coreText(c, "Selected last")
		})
	}, 300, 200)
	selectBetween(t, tt, "Selected first", 0, "Selected last", 13)
	other = "Updated elsewhere"
	tt.Frame()
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "Selected first\nSelected last" {
		t.Fatalf("changing unselected text lost the selection: %q", tt.Clipboard())
	}
}

func TestSelectableContainerFocusAndDisable(t *testing.T) {
	disabled := false
	color := RGB(10, 30, 70)
	tt := coreNewTester(func(c *context) {
		c.Theme().Selection = color
		coreColumn(c).Selectable().Gap(8).Children(func() {
			coreText(c, "First").Disabled(disabled)
			coreText(c, "Last")
		})
	}, 200, 100)
	highlighted := func() bool {
		img := tt.Image()
		for y := range img.Bounds().Dy() {
			for x := range img.Bounds().Dx() {
				p := img.RGBAAt(x, y)
				if p.R == color.R && p.G == color.G && p.B == color.B {
					return true
				}
			}
		}
		return false
	}
	selectBetween(t, tt, "First", 0, "Last", 4)
	if !highlighted() {
		t.Fatal("the selection has no highlight")
	}
	tt.send(platform.SurfaceEvent{Kind: platform.SurfaceBlur})
	if highlighted() {
		t.Fatal("the shared selection still paints while the window is unfocused")
	}
	tt.send(platform.SurfaceEvent{Kind: platform.SurfaceFocus})
	if !highlighted() {
		t.Fatal("focusing the window did not restore the highlight")
	}
	disabled = true
	tt.Frame()
	if highlighted() {
		t.Fatal("disabling an endpoint left a stale selection highlight")
	}
	tt.SetClipboard("kept")
	tt.Key(Cmd, KeyA)
	tt.Key(Cmd, KeyC)
	if tt.Clipboard() != "kept" {
		t.Fatal("a disabled endpoint still handled selection commands")
	}
}

// A ContextMenu on a Selectable container replaces its text's Copy and Select All, and EditItems
// puts them back where the menu wants them: its own item reaches its function, Copy copies the
// selection. Text without a ContextMenu keeps its own menu, and an element around the container
// keeps its menu beside the text.
func TestSelectableTextContextMenu(t *testing.T) {
	replied := 0
	tt := coreNewTester(func(c *context) {
		bubble := coreColumn(c).Padding(20).Gap(10)
		bubble.ContextMenu(func(m *Menu) { m.Item("Bubble") })
		bubble.Children(func() {
			coreColumn(c).Selectable().ContextMenu(func(m *Menu) {
				if m.Item("Reply").Chosen() {
					replied++
				}
				m.Separator()
				m.EditItems()
				m.Separator()
				m.Item("After")
			}).Gap(10).Children(func() {
				coreText(c, "Alpha")
				coreText(c, "Beta")
			})
			coreColumn(c).Selectable().Children(func() { coreText(c, "Gamma") })
		})
	}, 300, 200)
	selectBetween(t, tt, "Alpha", 0, "Beta", 4)
	x, y := selectableAt(t, tt, "Beta", 2)
	tt.RightClickAt(x, y)
	if want := []string{"Reply", "-", "Copy", "-", "Select All", "-", "After"}; !slices.Equal(tt.Menu(), want) {
		t.Fatalf("menu %q, want %q", tt.Menu(), want)
	}
	if err := tt.ChooseMenuItem("Reply"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if replied != 1 {
		t.Errorf("Reply reached the menu's function %d times", replied)
	}
	tt.RightClickAt(x, y)
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if got := tt.Clipboard(); got != "Alpha\nBeta" || replied != 1 {
		t.Errorf("Copy copied %q; Reply ran %d times", got, replied)
	}
	// Select All from the custom menu selects the container's text.
	tt.RightClickAt(x, y)
	if err := tt.ChooseMenuItem("Select All"); err != nil {
		t.Fatal(err)
	}
	tt.Key(Cmd, KeyC)
	if got := tt.Clipboard(); got != "Alpha\nBeta" {
		t.Errorf("after Select All, copied %q", got)
	}
	x, y = selectableAt(t, tt, "Gamma", 2)
	tt.RightClickAt(x, y)
	if want := []string{"Copy", "-", "Select All"}; !slices.Equal(tt.Menu(), want) {
		t.Errorf("menu of text without one %q, want %q", tt.Menu(), want)
	}
	tt.CloseMenu()
	tt.RightClickAt(5, 5)
	if want := []string{"Bubble"}; !slices.Equal(tt.Menu(), want) {
		t.Errorf("menu beside the text %q, want %q", tt.Menu(), want)
	}
}

// EditItems in a text input's ContextMenu adds its editing items, which edit it.
func TestTextInputContextMenuEditItems(t *testing.T) {
	value := "hello"
	tt := coreNewTester(func(c *context) {
		coreTextInput(c, &value).Label("Field").ContextMenu(func(m *Menu) {
			m.Item("Custom")
			m.Separator()
			m.EditItems()
		})
	}, 300, 100)
	r, _ := tt.Find("Field")
	tt.ClickAt(r.X+10, r.Y+r.H/2)
	tt.Key(Cmd, KeyA)
	tt.RightClickAt(r.X+10, r.Y+r.H/2)
	menu := tt.Menu()
	if len(menu) < 4 || menu[0] != "Custom" || menu[1] != "-" || !slices.Contains(menu, "Copy") || !slices.Contains(menu, "Select All") {
		t.Fatalf("menu %q", menu)
	}
	if err := tt.ChooseMenuItem("Copy"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); got != "hello" {
		t.Errorf("Copy copied %q", got)
	}
}

// SelectionColor paints a selection's highlight inside the element in its color.
func TestSelectionColor(t *testing.T) {
	highlight := RGB(255, 200, 0)
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(20).Gap(10).Children(func() {
			coreColumn(c).Selectable().SelectionColor(highlight).Children(func() { coreText(c, "Colored") })
			coreColumn(c).Selectable().Children(func() { coreText(c, "Plain") })
		})
	}, 300, 150)
	painted := func(name string, color Color) bool {
		r, _ := tt.Find(name)
		img := tt.Image()
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if p := img.RGBAAt(x, y); p.R == color.R && p.G == color.G && p.B == color.B {
					return true
				}
			}
		}
		return false
	}
	selectBetween(t, tt, "Colored", 0, "Colored", 7)
	if !painted("Colored", highlight) {
		t.Error("the selection is not in its color")
	}
	// Outside it, the theme's highlight: drawn, and not in the other's color.
	inked := func(name string) int {
		r, _ := tt.Find(name)
		img, n := tt.Image(), 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if p := img.RGBAAt(x, y); p.R != 255 || p.G != 255 || p.B != 255 {
					n++
				}
			}
		}
		return n
	}
	before := inked("Plain")
	selectBetween(t, tt, "Plain", 0, "Plain", 5)
	if painted("Plain", highlight) || inked("Plain") <= before {
		t.Error("the plain selection is not the theme's highlight")
	}
}
