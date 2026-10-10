//go:build !mygo_noinspector

package ui

import (
	"fmt"
	"image/color"
	"io"
	"log"
	"os"
	"testing"
)

// pickLabel labels the inspector's picker.
const pickLabel = "Select an element in the window to inspect it"

// inspected returns a tester of view whose window lets the inspector open,
// and the width the view had in its last frame.
func inspected(view func(c *context)) (*Tester, *float32) { return inspectedAs(coreNewTester, view) }

// inspectedAs is inspected with a tester newTester makes.
func inspectedAs(newTester func(func(*context), int, int) *Tester, view func(c *context)) (*Tester, *float32) {
	var width float32
	tt := newTester(func(c *context) {
		width, _ = c.Size()
		view(c)
	}, 1000, 600)
	tt.rt.insp.enabled = true
	return tt, &width
}

// openInspector opens the inspector of tt.
func openInspector(tt *Tester) {
	tt.rt.toggleInspector()
	tt.Frame()
}

func TestInspectorOpensAndCloses(t *testing.T) {
	tt, width := inspected(func(c *context) {
		coreColumn(c).Padding(20).Children(func() { coreText(c, "Hello") })
	})
	tt.Key(0, KeyF12)
	if !tt.rt.insp.open {
		t.Fatal("F12 did not open the inspector")
	}
	if *width != 1000-inspectorWidth {
		t.Errorf("the content is %v wide beside the inspector", *width)
	}
	for _, s := range []string{"Elements", "Performance", "Issues", "<Root>", "<Text>Hello</Text>", "</Column>"} {
		if !tt.HasText(s) {
			t.Errorf("the inspector does not show %s: %q", s, tt.Texts())
		}
	}
	// The panel takes the pointer beside the content.
	if r, ok := tt.Find(pickLabel); !ok || r.X < 1000-inspectorWidth {
		t.Errorf("the picker is at %v", r)
	}
	tt.Key(0, KeyF12)
	if tt.rt.insp.open || *width != 1000 || tt.HasText("<Root>") {
		t.Error("F12 did not close the inspector")
	}
	// Alt+Cmd+I (Ctrl+Shift+I outside macOS) toggles it too, and
	// Shift+Cmd+C (Ctrl+Shift+C) opens it picking.
	mods, pick := Ctrl|Shift, Ctrl|Shift
	if Cmd == Super {
		mods, pick = Super|Alt, Super|Shift
	}
	tt.Key(mods, KeyI)
	if !tt.rt.insp.open {
		t.Error("the shortcut did not open the inspector")
	}
	tt.Key(mods, KeyI)
	tt.Key(pick, KeyC)
	if !tt.rt.insp.open || !tt.rt.insp.picking {
		t.Error("the pick shortcut did not open the inspector picking")
	}
	tt.Key(0, KeyEscape)
	if tt.rt.insp.picking {
		t.Error("Escape did not stop picking")
	}
	if err := tt.Click("Close"); err != nil || tt.rt.insp.open {
		t.Errorf("Close did not close the inspector: %v", err)
	}

	// Without DevTools, the keys go to the view.
	tt.rt.insp.enabled = false
	tt.Key(0, KeyF12)
	if tt.rt.insp.open {
		t.Error("the inspector opened without DevTools")
	}
}

func TestInspectorFollowsTheContent(t *testing.T) {
	n := 0
	tt, _ := inspected(func(c *context) {
		coreColumn(c).Padding(20).Gap(8).Children(func() {
			coreText(c, fmt.Sprintf("Count %d", n))
			if coreButton(c, "Add").Clicked() {
				n++
			}
		})
	})
	openInspector(tt)
	if !tt.HasText("<Text>Count 0</Text>") {
		t.Fatalf("the tree shows %q", tt.Texts())
	}
	if err := tt.Click("Add"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("<Text>Count 1</Text>") || tt.HasText("Count 0") {
		t.Errorf("the tree did not follow the content: %q", tt.Texts())
	}
}

func TestInspectorPicksAndDescribes(t *testing.T) {
	clicks := 0
	tt, _ := inspected(func(c *context) {
		coreColumn(c).Padding(20).Children(func() {
			if coreButton(c, "Press").Width(120).Clicked() {
				clicks++
			}
		})
	})
	openInspector(tt)
	if !tt.HasText("Choose an element in the tree, or pick one in the window.") {
		t.Errorf("no hint before an element is chosen: %q", tt.Texts())
	}
	if err := tt.Click(pickLabel); err != nil {
		t.Fatal(err)
	}
	if !tt.rt.insp.picking {
		t.Fatal("the picker does not pick")
	}
	r, _ := tt.Find("Press")
	x, y := r.X+r.W/2, r.Y+r.H/2
	before := tt.Image().RGBAAt(int(r.X)+1, int(y))
	tt.Move(x, y)
	if tt.rt.insp.hovered == 0 || tt.rt.insp.hoverElem == nil {
		t.Fatal("picking does not follow the pointer")
	}
	// The element under the pointer is tinted blue over the content,
	// whatever is under it there (text, on Windows in ClearType's colors).
	bluer := func(c color.RGBA) int { return int(c.B) - int(c.R) }
	if after := tt.Image().RGBAAt(int(r.X)+1, int(y)); bluer(after) <= bluer(before) {
		t.Errorf("the element picked is not highlighted: %v, then %v", before, after)
	}
	tt.ClickAt(x, y)
	if clicks != 0 {
		t.Error("the click that picked reached the button")
	}
	if tt.rt.insp.picking || tt.rt.insp.selected == 0 {
		t.Fatal("the click did not pick")
	}
	// Its styles, own and inherited, as CSS rules.
	for _, s := range []string{"Text {", "Inherited from", "font-size", "}"} {
		if !tt.HasText(s) {
			t.Errorf("the Styles tab lacks %s: %q", s, tt.Texts())
		}
	}
	tt.Click("Computed")
	for _, s := range []string{"margin", "border", "padding", "display", "width"} {
		if !tt.HasText(s) {
			t.Errorf("the Computed tab lacks %s: %q", s, tt.Texts())
		}
	}
	tt.Click("Properties")
	for _, s := range []string{"Accessibility", "keyboard-focusable", "role"} {
		if !tt.HasText(s) {
			t.Errorf("the Properties tab lacks %s: %q", s, tt.Texts())
		}
	}
	tt.Click("Press")
	if clicks != 1 {
		t.Errorf("after picking, the button took %d clicks", clicks)
	}
}

func TestInspectorTree(t *testing.T) {
	tt, _ := inspected(func(c *context) {
		coreColumn(c).Padding(20).Children(func() {
			coreRow(c).Key("toolbar").Children(func() {
				coreRow(c).Children(func() { coreText(c, "Deep") })
			})
			coreText(c, "Inside")
		})
	})
	openInspector(tt)
	// Nodes deeper than the root's grandchildren start closed; keys show
	// as attributes.
	if !tt.HasText(`<Row key="toolbar">`) || !tt.HasText("<Row>…</Row>") || tt.HasText("<Text>Deep</Text>") {
		t.Fatalf("the tree shows %q", tt.Texts())
	}
	if err := tt.Click("<Text>Inside</Text>"); err != nil {
		t.Fatal(err)
	}
	sel := tt.rt.insp.selected
	if s := tt.rt.states[sel]; s == nil || !tt.HasText("Text {") {
		t.Fatalf("choosing the text in the tree shows %q", tt.Texts())
	}
	// The breadcrumbs show its path.
	for _, s := range []string{"Root", "Column", "Text"} {
		if _, ok := tt.Find(s); !ok {
			t.Errorf("the breadcrumbs lack %s: %q", s, tt.Texts())
		}
	}
	// Hovering a row of the tree highlights its element over the content.
	r, _ := tt.Find("<Text>Inside</Text>")
	tt.Move(r.X+2, r.Y+2)
	if tt.rt.insp.hoverElem == nil || tt.rt.insp.hoverElem.id != sel {
		t.Error("hovering the tree does not highlight the element")
	}
	// Up goes to the row above, the closing tag of the toolbar's row,
	// which chooses the row, and Down to the row inside it; Right opens
	// that, Left closes it, then goes to its parent, then closes that.
	tt.Key(0, KeyUp)
	if n := tt.rt.insp.nodeOf(tt.rt.insp.selected); n < 0 || tt.rt.insp.nodes[n].keyText != "toolbar" {
		t.Errorf("Up chose %v", n)
	}
	tt.Key(0, KeyDown)
	if n := tt.rt.insp.nodeOf(tt.rt.insp.selected); n < 0 || tt.rt.insp.nodes[n].name != "Row" || tt.rt.insp.nodes[n].keyText != "" {
		t.Errorf("Down did not reach the inner row: %v", n)
	}
	tt.Key(0, KeyRight)
	if !tt.HasText("<Text>Deep</Text>") {
		t.Errorf("Right did not open the row: %q", tt.Texts())
	}
	tt.Key(0, KeyLeft)
	if tt.HasText("<Text>Deep</Text>") {
		t.Errorf("Left did not close the row: %q", tt.Texts())
	}
	tt.Key(0, KeyLeft)
	if n := tt.rt.insp.nodeOf(tt.rt.insp.selected); n < 0 || tt.rt.insp.nodes[n].keyText != "toolbar" {
		t.Errorf("Left did not go to the parent: %v", n)
	}
	tt.Key(0, KeyLeft)
	if !tt.HasText(`<Row key="toolbar">…</Row>`) {
		t.Errorf("Left did not close the toolbar's row: %q", tt.Texts())
	}

	// Cmd+F finds elements by their text, and shows the one found.
	tt.Key(Cmd, KeyF)
	if !tt.rt.insp.finding {
		t.Fatal("Cmd+F did not open the find bar")
	}
	tt.Type("deep")
	if n := tt.rt.insp.nodeOf(tt.rt.insp.selected); n < 0 || tt.rt.insp.nodes[n].text != "Deep" {
		t.Errorf("finding chose %v: %q", n, tt.Texts())
	}
	if !tt.HasText("<Text>Deep</Text>") {
		t.Error("finding did not show the element found")
	}
}

func TestInspectorTabs(t *testing.T) {
	log.SetOutput(io.Discard)
	defer log.SetOutput(os.Stderr)
	tt, width := inspectedAs(lenientTester, func(c *context) {
		coreRow(c).Key("twice")
		coreRow(c).Key("twice")
	})
	openInspector(tt)
	if !tt.HasText("Issues 1") {
		t.Fatalf("the toolbar does not count the issue: %q", tt.Texts())
	}
	if err := tt.Click("Issues 1"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(`key "twice"`) {
		t.Errorf("the Issues tab does not list the issue: %q", tt.Texts())
	}
	tt.Click("Performance")
	for _, s := range []string{"Last frame", "Average", "Slowest", "Build", "Layout", "Paint", "60 Hz", "16.7 ms, a frame at 60 Hz"} {
		if !tt.HasText(s) {
			t.Errorf("the Performance tab lacks %s: %q", s, tt.Texts())
		}
	}
	// The budget of a frame is the display's.
	tt.h.hz = 120
	tt.Frame()
	if !tt.HasText("120 Hz") || !tt.HasText("8.3 ms, a frame at 120 Hz") {
		t.Errorf("at 120 Hz, the Performance tab shows %q", tt.Texts())
	}
	if tt.rt.insp.frames == 0 {
		t.Error("no frames noted")
	}
	// Dragging the panel's edge resizes it.
	x := 1000 - float32(inspectorWidth)
	tt.Press(x, 300)
	tt.Move(x-100, 300)
	tt.Release(x-100, 300)
	if *width != x-100 {
		t.Errorf("after dragging the edge, the content is %v wide", *width)
	}
}

func TestInspectorDarkPalette(t *testing.T) {
	tt, _ := inspected(func(c *context) { coreText(c, "Hi") })
	tt.SetDark(true)
	openInspector(tt)
	if p := tt.Image().RGBAAt(990, 300); p.R > 60 {
		t.Errorf("the panel is %v in the dark", p)
	}
}

// TestInspectorSettles checks that the inspector asks for frames only
// while what it shows changes, in all its tabs and while picking.
func TestInspectorSettles(t *testing.T) {
	tt, _ := inspected(func(c *context) {
		coreColumn(c).Padding(20).Gap(8).Children(func() {
			coreText(c, "Title").Bold()
			for i := range 3 {
				coreRow(c).Key(i).Transition(ElementTransition{}).Children(func() { coreButton(c, fmt.Sprint("Item ", i)) })
			}
		})
	})
	openInspector(tt)
	check := func(what string) {
		t.Helper()
		tt.Frame()
		if tt.h.requested.Load() {
			t.Errorf("%s: frames keep coming", what)
		}
	}
	check("open")
	tt.Click("<Text>Title</Text>")
	check("chosen")
	for _, tab := range []string{"Computed", "Properties", "Styles", "Performance", "Issues", "Elements"} {
		tt.Click(tab)
		check(tab)
	}
	r, _ := tt.Find("<Text>Title</Text>")
	tt.Move(r.X+5, r.Y+5)
	check("hovering the tree")
	tt.Click(pickLabel)
	tt.Move(100, 30)
	check("picking")
}
