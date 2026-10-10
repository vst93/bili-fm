package ui

import (
	"image/color"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// TestBasesHaveNoLook builds every base on a white window: they draw
// nothing of their own.
func TestBasesHaveNoLook(t *testing.T) {
	var on, check bool
	var radio, tab int
	var value = 50.0
	var size, text = "M", "hello"
	open := true
	tt := coreNewTester(func(c *context) {
		th := *c.Theme()
		th.Background = RGB(255, 255, 255)
		c.SetTheme(&th)
		coreColumn(c).Fill().Padding(10).Gap(10).AlignItems(Start).Children(func() {
			coreButtonBase(c).Size(60, 20)
			coreCheckboxBase(c, &check).Size(60, 20)
			coreSwitchBase(c, &on).Size(60, 20)
			coreRadioBase(c, &radio, 1).Size(60, 20)
			coreSliderBase(c, &value, 0, 100).Size(60, 20)
			tabs := coreTabsBase(c, &tab, 2)
			tabs.List.Children(func() {
				tabs.Tab(0).Size(30, 20)
				tabs.Tab(1).Size(30, 20)
			})
			sel := coreSelectBase(c, &size)
			sel.Trigger.Size(60, 20)
			sel.Popup(func(panel *node) {
				sel.Item("S").Size(60, 20)
				sel.Item("M").Size(60, 20)
			})
			coreTextInputBase(c, &text).Width(60).TextColor(RGB(255, 255, 255))
			corePopoverBase(c, sel.Trigger, &open, func(panel *node) { coreBox(c).Size(30, 30) })
		})
	}, 300, 400)
	img := tt.Image()
	white := color.RGBA{255, 255, 255, 255}
	for y := range img.Rect.Dy() {
		for x := range img.Rect.Dx() {
			if c := img.RGBAAt(x, y); c != white {
				t.Fatalf("a base drew %v at (%d, %d)", c, x, y)
			}
		}
	}
}

func TestToggleBases(t *testing.T) {
	var check, on bool
	radio := "a"
	var changed []string
	tt := coreNewTester(func(c *context) {
		coreColumn(c).AlignItems(Start).Children(func() {
			if coreCheckboxBase(c, &check).Size(20, 20).Label("check").Changed() {
				changed = append(changed, "check")
			}
			coreSwitchBase(c, &on).Size(20, 20).Label("switch")
			coreRadioBase(c, &radio, "a").Size(20, 20).Label("a")
			if coreRadioBase(c, &radio, "b").Size(20, 20).Label("b").Changed() {
				changed = append(changed, "b")
			}
		})
	}, 200, 200)
	tt.Click("check")
	tt.Click("switch")
	tt.Click("b")
	if !check || !on || radio != "b" {
		t.Fatalf("clicks: check %v, switch %v, radio %q", check, on, radio)
	}
	if len(changed) != 2 {
		t.Errorf("Changed reported %q", changed)
	}
	// Space toggles the one with the focus.
	tt.Key(0, KeyTab)
	if !tt.Focused("check") {
		t.Fatal("Tab does not focus the check box")
	}
	tt.Key(0, KeySpace)
	if check {
		t.Error("Space does not toggle the check box")
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	for label, role := range map[string]platform.AccessRole{"check": platform.RoleCheckBox, "switch": platform.RoleSwitch, "b": platform.RoleRadio} {
		n := accessNode(t, tt.h.access, role, label)
		on := n.States&platform.AccessChecked != 0
		if want := label != "check"; on != want {
			t.Errorf("%s is checked %v to assistive technology, want %v", label, on, want)
		}
	}
}

func TestSliderBaseMapsItsContentBox(t *testing.T) {
	value := 0.0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).AlignItems(Start).Padding(10).Children(func() {
			coreSliderBase(c, &value, 0, 100).Size(120, 20).PaddingX(10).Label("volume")
		})
	}, 200, 100)
	r, _ := tt.Find("volume")
	// The content box spans 20 DIPs to 120 of the window: 100 DIPs.
	for _, tc := range []struct{ x, want float32 }{{r.X + 10, 0}, {r.X + 60, 50}, {r.X + 110, 100}, {r.X + 2, 0}, {r.X + 118, 100}} {
		tt.Press(tc.x, r.Y+10)
		tt.Release(tc.x, r.Y+10)
		if float32(value) != tc.want {
			t.Errorf("pressing at %v sets %v, want %v", tc.x, value, tc.want)
		}
	}
	tt.Key(0, KeyHome)
	if value != 0 {
		t.Errorf("Home sets %v", value)
	}
	tt.Key(0, KeyRight)
	if value != 1 {
		t.Errorf("Right sets %v", value)
	}
	tt.Key(0, KeyEnd)
	if value != 100 {
		t.Errorf("End sets %v", value)
	}
}

func TestTabsBase(t *testing.T) {
	tab, changes := 0, 0
	tt := coreNewTester(func(c *context) {
		tabs := coreTabsBase(c, &tab, 3)
		tabs.List.Children(func() {
			for i, name := range []string{"One", "Two", "Three"} {
				tabs.Tab(i).Padding(4).Children(func() { coreText(c, name) })
			}
		})
		if tabs.List.Changed() {
			changes++
		}
	}, 300, 100)
	tt.Click("Three")
	if tab != 2 || changes != 1 {
		t.Fatalf("a click chooses tab %d, %d changes", tab, changes)
	}
	tt.Key(0, KeyRight)
	if tab != 0 || !tt.Focused("One") {
		t.Errorf("Right from the last tab chooses %d, focused %v", tab, tt.Focused("One"))
	}
	tt.Key(0, KeyEnd)
	if tab != 2 || !tt.Focused("Three") {
		t.Errorf("End chooses %d", tab)
	}
}

func TestSelectBase(t *testing.T) {
	size := "M"
	sizes := []string{"S", "M", "L", "XL"}
	highlighted := map[string]bool{}
	tt := coreNewTester(func(c *context) {
		sel := coreSelectBase(c, &size)
		sel.Trigger.Label("size").Padding(4).Children(func() { coreText(c, "Size: "+size) })
		clear(highlighted)
		sel.Popup(func(panel *node) {
			panel.Label("popup")
			for _, s := range sizes {
				item := sel.Item(s).Padding(4).Label("item " + s)
				if item.Highlighted() {
					highlighted[s] = true
				}
				item.Children(func() { coreText(c, s) })
			}
		})
	}, 300, 300)
	if tt.HasText("XL") {
		t.Fatal("the popup shows before the trigger is clicked")
	}
	tt.Click("size")
	if !tt.HasText("XL") || !highlighted["M"] || len(highlighted) != 1 {
		t.Fatalf("the open popup highlights %v, not the choice", highlighted)
	}
	// The arrows move the highlight, Enter chooses it.
	tt.Key(0, KeyDown)
	tt.Key(0, KeyDown)
	if !highlighted["XL"] || len(highlighted) != 1 {
		t.Errorf("Down twice highlights %v", highlighted)
	}
	tt.Key(0, KeyUp)
	tt.Key(0, KeyEnter)
	if size != "L" || tt.HasText("XL") {
		t.Errorf("Enter chooses %q and leaves the popup open: %v", size, tt.HasText("XL"))
	}
	// Down opens the popup from the focused trigger, Escape closes it.
	tt.Key(0, KeyDown)
	if !tt.HasText("XL") {
		t.Fatal("Down does not open the popup")
	}
	tt.Key(0, KeyEscape)
	if tt.HasText("XL") {
		t.Error("Escape does not close the popup")
	}
	// The pointer highlights what it moves onto, and a click chooses.
	tt.Click("size")
	r, _ := tt.Find("item S")
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	if !highlighted["S"] || len(highlighted) != 1 {
		t.Errorf("the pointer highlights %v", highlighted)
	}
	// While it rests there, the arrows still move the highlight.
	tt.Key(0, KeyDown)
	if !highlighted["M"] {
		t.Errorf("Down under a resting pointer highlights %v", highlighted)
	}
	tt.Click("item XL")
	if size != "XL" || tt.HasText("item S") {
		t.Errorf("a click chooses %q", size)
	}
}

func TestDialogAndPopoverBases(t *testing.T) {
	dialog, popover := true, false
	var back *node
	tt := coreNewTester(func(c *context) {
		b := coreButtonBase(c).Size(40, 20).Label("anchor")
		if b.Clicked() {
			popover = true
		}
		corePopoverBase(c, b, &popover, func(panel *node) {
			panel.Label("popover").Size(50, 50)
		})
		coreDialogBase(c, &dialog, func(backdrop, panel *node) {
			back = backdrop
			backdrop.Background(RGBA(0, 0, 0, 0.5))
			panel.Label("dialog").Size(100, 60)
		})
	}, 400, 300)
	if back == nil {
		t.Fatal("DialogBase does not pass its backdrop")
	}
	r, ok := tt.Find("dialog")
	if !ok || r.X != 150 || r.Y != 120 {
		t.Errorf("the dialog is at %v, not centered", r)
	}
	tt.ClickAt(10, 290)
	if dialog {
		t.Error("a click on the backdrop does not close the dialog")
	}
	tt.Click("anchor")
	p, ok := tt.Find("popover")
	if !ok || p.Y != 20 {
		t.Errorf("the popover is at %v, not under its anchor", p)
	}
	tt.Key(0, KeyEscape)
	if popover {
		t.Error("Escape does not close the popover")
	}
}

func TestTextInputBase(t *testing.T) {
	text := ""
	var in *node
	tt := coreNewTester(func(c *context) {
		in = coreTextInputBase(c, &text).Width(100).Label("name").AutoFocus()
	}, 200, 100)
	tt.Type("Ada")
	if text != "Ada" {
		t.Errorf("typed %q", text)
	}
	if in.pad != [4]float32{} || in.border != [4]float32{} || in.bg.A != 0 {
		t.Errorf("the base has a look: padding %v, border %v, background %v", in.pad, in.border, in.bg)
	}
}

func TestFocusRing(t *testing.T) {
	ringed := func(show bool) bool {
		tt := coreNewTester(func(c *context) {
			th := *c.Theme()
			th.Background = RGB(255, 255, 255)
			c.SetTheme(&th)
			coreColumn(c).Padding(20).AlignItems(Start).Children(func() {
				coreButtonBase(c).Size(40, 20).Label("b").FocusRing(show)
			})
		}, 100, 60)
		tt.Key(0, KeyTab)
		img := tt.Image()
		// Just outside the button's box.
		return img.RGBAAt(18, 30) != color.RGBA{255, 255, 255, 255}
	}
	if !ringed(true) {
		t.Error("no ring around the focused button")
	}
	if ringed(false) {
		t.Error("a ring around a button that turned it off")
	}
}

// TestPopupsOpenWhereTheyFit opens popovers where there is no room below
// or to the right of their anchors: the first frame that paints them has
// them where they fit, instead of moving them once it knows their size.
func TestPopupsOpenWhereTheyFit(t *testing.T) {
	for _, tc := range []struct {
		name    string
		justify Align
		align   Align
	}{{"bottom right", End, End}, {"top left", Start, Start}} {
		open := false
		var first *Rect
		tt := coreNewTester(func(c *context) {
			coreColumn(c).Fill().Padding(10).Justify(tc.justify).AlignItems(tc.align).Children(func() {
				b := coreButtonBase(c).Size(80, 24).Label("anchor")
				if b.Clicked() {
					open = true
				}
				corePopoverBase(c, b, &open, func(panel *node) {
					panel.Size(160, 120).Margin(4, 0, 0, 0).Draw(func(p *Painter, r Rect) {
						if first == nil {
							first = &r
						}
					})
				})
			})
		}, 300, 300)
		tt.Click("anchor")
		a, _ := tt.Find("anchor")
		if first == nil {
			t.Fatalf("%s: the popover was not painted", tc.name)
		}
		r := *first
		if r.X < 4 || r.X+r.W > 296 {
			t.Errorf("%s: the popover first shows at %v, out of the window", tc.name, r)
		}
		if tc.justify == End && r.Y+r.H != a.Y-4 {
			t.Errorf("%s: the popover first shows at %v, not 4 DIPs above its anchor at %v", tc.name, r, a)
		}
		if tc.justify == Start && (r.Y != a.Y+a.H+4 || r.X != a.X) {
			t.Errorf("%s: the popover first shows at %v, not 4 DIPs below its anchor at %v", tc.name, r, a)
		}
	}
}

func TestEnterWithWindowShortcut(t *testing.T) {
	window, button, checked := 0, 0, false
	tt := coreNewTester(func(c *context) {
		if c.Shortcut(0, KeyEnter) {
			window++
		}
		coreColumn(c).Children(func() {
			if coreButton(c, "Other").Clicked() {
				button++
			}
			coreCheckbox(c, &checked, "Agree")
		})
	}, 300, 200)
	tt.Key(0, KeyEnter)
	if window != 1 || button != 0 {
		t.Fatalf("Enter without a focus: window %d, button %d", window, button)
	}
	// A focused button takes Enter before the window.
	tt.Key(0, KeyTab)
	tt.Key(0, KeyEnter)
	if window != 1 || button != 1 {
		t.Fatalf("Enter on a focused button: window %d, button %d", window, button)
	}
	// A focused check box leaves Enter to the window and takes Space.
	tt.Key(0, KeyTab)
	tt.Key(0, KeyEnter)
	if window != 2 || checked {
		t.Fatalf("Enter on a focused check box: window %d, checked %v", window, checked)
	}
	tt.Key(0, KeySpace)
	if window != 2 || !checked {
		t.Fatalf("Space on a focused check box: window %d, checked %v", window, checked)
	}
}
