package ui

import (
	"testing"
	"time"

	"github.com/egoist/mygo/internal/platform"
)

func TestTabs(t *testing.T) {
	tab := 0
	changes := 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Children(func() {
			if coreTabs(c, &tab, "One", "Two", "Three").Changed() {
				changes++
			}
			coreTextf(c, "page %d", tab)
		})
	}, 400, 200)
	if err := tt.Click("Two"); err != nil {
		t.Fatal(err)
	}
	if tab != 1 || !tt.HasText("page 1") || changes != 1 {
		t.Fatalf("a click on Two: tab %d, %d changes, texts %q", tab, changes, tt.Texts())
	}
	// The arrows move the choice, and the focus with it.
	tt.Key(0, KeyRight)
	if tab != 2 || !tt.Focused("Three") {
		t.Errorf("Right: tab %d, Three focused %v", tab, tt.Focused("Three"))
	}
	tt.Key(0, KeyRight)
	if tab != 0 || !tt.Focused("One") {
		t.Errorf("Right past the last tab: tab %d", tab)
	}
	tt.Key(0, KeyEnd)
	if tab != 2 {
		t.Errorf("End: tab %d", tab)
	}
	tt.Key(0, KeyHome)
	if tab != 0 {
		t.Errorf("Home: tab %d", tab)
	}

	tt.rt.accessibilityOn()
	tt.Frame()
	var tabs, selected int
	for _, n := range tt.h.access.Nodes {
		if n.Role == platform.RoleTab {
			tabs++
			if n.States&platform.AccessChecked != 0 {
				selected++
			}
		}
	}
	if tabs != 3 || selected != 1 {
		t.Errorf("assistive technology sees %d tabs, %d selected", tabs, selected)
	}
}

func TestSplit(t *testing.T) {
	size := float32(120)
	tt := coreNewTester(func(c *context) {
		coreSplit(c, &size, func() { coreText(c, "left") }, func() { coreText(c, "right") }).Fill()
	}, 400, 200)
	if r, _ := tt.Find("right"); r.X < 120+1-0.5 || r.X > 120+1+0.5 {
		t.Fatalf("the second pane starts at %v", r.X)
	}
	// The divider moves with the pointer, within the window.
	x, y := float32(123), float32(100)
	tt.Press(x, y)
	tt.Move(x+50, y)
	tt.Release(x+50, y)
	if size != 170 {
		t.Errorf("dragged 50 DIPs right, the first pane is %v wide", size)
	}
	tt.Press(x+50, y)
	tt.Move(0, y)
	tt.Release(0, y)
	if size != 40 {
		t.Errorf("dragged to the edge, the first pane is %v wide", size)
	}
	// The arrows move it once it has the focus.
	tt.ClickAt(43, y)
	tt.Key(0, KeyRight)
	if size != 50 {
		t.Errorf("Right on the divider: %v", size)
	}
	if r, _ := tt.Find("right"); r.X != 51 {
		t.Errorf("the second pane follows to %v", r.X)
	}
	// The handle reaches over the first pane as over the second.
	tt.Press(48, y)
	tt.Move(68, y)
	tt.Release(68, y)
	if size != 70 {
		t.Errorf("dragged 20 DIPs right from over the first pane, it is %v wide", size)
	}
}

func TestSplitVertical(t *testing.T) {
	size := float32(60)
	tt := coreNewTester(func(c *context) {
		coreSplitVertical(c, &size, func() { coreText(c, "top") }, func() { coreText(c, "bottom") }).Fill()
	}, 300, 200)
	// The second pane starts below the divider and fits in the window.
	if r, _ := tt.Find("bottom"); r.Y < 60+1-0.5 || r.Y > 60+1+0.5 {
		t.Fatalf("the second pane starts at %v", r.Y)
	}
	n := 0
	for e := tt.rt.c.root.first; e != nil; e = e.next {
		n++
	}
	if n != 1 {
		t.Errorf("the split built %d elements in the root", n)
	}
	tt.Press(100, 61)
	tt.Move(100, 101)
	tt.Release(100, 101)
	if size != 100 {
		t.Errorf("dragged 40 DIPs down, the first pane is %v high", size)
	}
}

func TestNumberInput(t *testing.T) {
	v := 5.0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).AlignItems(Start).Children(func() {
			coreNumberInput(c, &v, 0, 10, 0.5).Label("Count")
		})
	}, 400, 100)
	if err := tt.Click("Increase"); err != nil {
		t.Fatal(err)
	}
	tt.Click("Decrease")
	tt.Click("Decrease")
	if v != 4.5 {
		t.Errorf("one step up and two down from 5: %v", v)
	}
	r, _ := tt.Find("Count")
	tt.ClickAt(r.X+20, r.Y+r.H/2)
	tt.Key(Cmd, KeyA)
	tt.Type("7.5")
	if v != 7.5 {
		t.Errorf("typed 7.5: %v", v)
	}
	tt.Key(Cmd, KeyA)
	tt.Type("12")
	if v != 7.5 {
		t.Errorf("12 is out of range, yet the value is %v", v)
	}
	tt.Key(0, KeyUp)
	if v != 8 {
		t.Errorf("Up from 7.5: %v", v)
	}
	for range 5 {
		tt.Key(0, KeyUp)
	}
	if v != 10 {
		t.Errorf("Up past the top: %v", v)
	}
}

func TestToast(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		if coreButton(c, "Save").Clicked() {
			c.Toast("Saved")
		}
	}, 400, 300)
	if err := tt.Click("Save"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Saved") {
		t.Fatalf("no toast after the click: %q", tt.Texts())
	}
	tt.Click("Save")
	if len(tt.rt.toasts) != 1 {
		t.Errorf("the same message shows %d times", len(tt.rt.toasts))
	}
	tt.rt.toasts[0].at = time.Now().Add(-toastTime)
	tt.Frame()
	if tt.HasText("Saved") {
		t.Error("the toast stays once its time is over")
	}
}

func TestTable(t *testing.T) {
	names := make([]string, 100)
	for i := range names {
		names[i] = "file" + string(rune('A'+i%26)) + string(rune('0'+i/26))
	}
	sel, opened := -1, -1
	s := ListState{Selected: &sel}
	cols := []TableColumn{{Title: "Name"}, {Title: "Size", Width: 80, Align: End}}
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Padding(10).Children(func() {
			if coreTable(c, &s, cols, len(names), func(row, col int) {
				if col == 0 {
					coreText(c, names[row])
				} else {
					coreTextf(c, "%d KB", row)
				}
			}).Grow(1).Submitted() {
				opened = sel
			}
		})
	}, 400, 300)
	if !tt.HasText("Name") || !tt.HasText("fileA0") || tt.HasText(names[99]) {
		t.Fatalf("texts %q: the header and the first rows, not the last", tt.Texts())
	}
	if err := tt.Click("fileC0"); err != nil {
		t.Fatal(err)
	}
	if sel != 2 {
		t.Errorf("clicked the third row: selected %d", sel)
	}
	tt.Key(0, KeyDown)
	tt.Key(0, KeyDown)
	if sel != 4 {
		t.Errorf("Down twice from the third row: %d", sel)
	}
	tt.Key(0, KeyEnd)
	if sel != 99 || !tt.HasText(names[99]) {
		t.Errorf("End: selected %d, last row shown %v", sel, tt.HasText(names[99]))
	}
	tt.Key(0, KeyEnter)
	if opened != 99 {
		t.Errorf("Enter opened %d", opened)
	}
	tt.Key(0, KeyHome)
	r, _ := tt.Find("fileA0")
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if sel != 0 || opened != 0 {
		t.Errorf("a double click on the first row: selected %d, opened %d", sel, opened)
	}
}

func TestTableRows(t *testing.T) {
	// Sections of a header and 9 rows; every seventh row says a lot.
	header := func(i int) bool { return i%10 == 0 }
	s := ListState{Header: header}
	headerCols := 0
	cols := []TableColumn{{Title: "Name"}, {Title: "Notes", Width: 120}}
	tt := coreNewTester(func(c *context) {
		coreTable(c, &s, cols, 1000, func(row, col int) {
			switch {
			case header(row):
				if col != 0 {
					headerCols++
				}
				coreTextf(c, "Section %d", row/10)
			case col == 0:
				coreTextf(c, "Row %d", row)
			case row%7 == 0:
				coreText(c, "A note long enough to wrap over a few lines of its column")
			default:
				coreText(c, "Short")
			}
		}).Grow(1)
	}, 400, 600)
	row1, _ := rowBox(tt, &s, 1)
	if row1.H != 32 {
		t.Errorf("a row of one line is %v high", row1.H)
	}
	row7, _ := rowBox(tt, &s, 7)
	if row7.H < 3*16 {
		t.Errorf("a row of a cell wrapping over lines is %v high", row7.H)
	}
	if r, ok := tt.Find("Row 7"); !ok || r.Y+r.H/2 < row7.Y+row7.H/2-1 || r.Y+r.H/2 > row7.Y+row7.H/2+1 {
		t.Errorf("the short cell of a tall row is at %v, not in the middle of %v", r, row7)
	}
	if top, _ := rowBox(tt, &s, 0); top.W != 400 || headerCols != 0 {
		t.Errorf("a header row is %v wide and built %d other columns", top.W, headerCols)
	}
	// The state scrolls the rows, below the column headers; the header of
	// the section they are in stays at the top.
	s.ScrollTo(505, Start)
	tt.Frame()
	head, _ := tt.Find("Name")
	r505, _ := rowBox(tt, &s, 505)
	r500, ok := rowBox(tt, &s, 500)
	if !ok || r500.Y < head.Y+head.H || r500.Y+r500.H > r505.Y {
		t.Errorf("row 505 at %v under the column headers ending at %v; its section's header at %v (%v)", r505, head.Y+head.H, r500, ok)
	}
	if first, _ := s.Visible(); first > 505 || first < 503 {
		t.Errorf("scrolled to row 505, the first row in view is %d", first)
	}
}

func TestTree(t *testing.T) {
	srcOpen, cmdOpen := false, true
	var clicked string
	tt := coreNewTester(func(c *context) {
		coreTree(c, func() {
			item := func(label string, open *bool, children func()) {
				if coreTreeItem(c, label, open, children).Selected(clicked == label).Clicked() {
					clicked = label
				}
			}
			item("src", &srcOpen, func() {
				item("main.go", nil, nil)
				item("cmd", &cmdOpen, func() {
					item("tool.go", nil, nil)
				})
			})
			item("go.mod", nil, nil)
		})
	}, 400, 300)
	if tt.HasText("main.go") || !tt.HasText("go.mod") {
		t.Fatalf("a closed folder shows its items: %q", tt.Texts())
	}
	// Its arrow opens it.
	r, _ := tt.Find("src")
	tt.ClickAt(r.X-10, r.Y+r.H/2)
	if !srcOpen || !tt.HasText("main.go") || !tt.HasText("tool.go") || clicked != "" {
		t.Fatalf("after a click on the arrow: open %v, clicked %q, texts %q", srcOpen, clicked, tt.Texts())
	}
	// Nested items are indented.
	if m, _ := tt.Find("main.go"); m.X <= r.X {
		t.Errorf("main.go at %v, src at %v", m.X, r.X)
	}
	tt.Click("src")
	if clicked != "src" {
		t.Errorf("clicked %q", clicked)
	}
	// The arrows move the focus through the items in view.
	tt.Key(0, KeyDown)
	if !tt.Focused("main.go") {
		t.Error("Down from src does not focus main.go")
	}
	tt.Key(0, KeyDown)
	tt.Key(0, KeyLeft) // closes cmd
	if cmdOpen || tt.HasText("tool.go") {
		t.Errorf("Left on the open cmd leaves it open %v", cmdOpen)
	}
	tt.Key(0, KeyLeft) // to its parent
	if !tt.Focused("src") {
		t.Error("Left on the closed cmd does not focus src")
	}
	tt.Key(0, KeyEnter)
	if clicked != "src" {
		t.Errorf("Enter chose %q", clicked)
	}
	tt.Key(0, KeyLeft)
	if srcOpen || tt.HasText("main.go") {
		t.Error("Left on src does not close it")
	}
}

func TestDateInput(t *testing.T) {
	date := time.Date(2026, 10, 3, 9, 30, 0, 0, time.UTC)
	changes := 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).AlignItems(Start).Children(func() {
			if coreDateInput(c, &date).Label("Due").Changed() {
				changes++
			}
		})
	}, 400, 420)
	if !tt.HasText("2026-10-03") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("Due"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("October 2026") {
		t.Fatalf("no calendar after a click: %q", tt.Texts())
	}
	if err := tt.Click("October 15, 2026"); err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 15, 9, 30, 0, 0, time.UTC); !date.Equal(want) || changes != 1 || tt.HasText("October 2026") {
		t.Fatalf("a click on the 15th: %v, %d changes, calendar shown %v", date, changes, tt.HasText("October 2026"))
	}
	// From the keyboard: Enter opens it, the arrows and Page Down move,
	// Enter chooses.
	tt.Key(0, KeyEnter)
	tt.Key(0, KeyRight)
	tt.Key(0, KeyDown)
	tt.Key(0, KeyPageDown)
	tt.Key(0, KeyEnter)
	if want := time.Date(2026, 11, 23, 9, 30, 0, 0, time.UTC); !date.Equal(want) {
		t.Errorf("Right, Down and Page Down from the 15th chose %v", date)
	}
	tt.Click("Due")
	tt.Click("Next month")
	if !tt.HasText("December 2026") {
		t.Errorf("Next month shows %q", tt.Texts())
	}
	tt.Key(0, KeyEscape)
	if tt.HasText("December 2026") {
		t.Error("Escape leaves the calendar open")
	}
}

func TestDateInputWide(t *testing.T) {
	date := time.Date(2026, 10, 3, 0, 0, 0, 0, time.UTC)
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Children(func() { coreDateInput(c, &date).Label("Due") })
	}, 800, 420)
	tt.Click("Due")
	field, _ := tt.Find("Due")
	next, ok := tt.Find("Next month")
	if !ok {
		t.Fatalf("no calendar after a click: %q", tt.Texts())
	}
	if next.X+next.W > field.X+field.W/2 {
		t.Errorf("the calendar under a field %v wide reaches %v", field.W, next.X+next.W)
	}
}

func TestProgressReverse(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		img := coreRender(func(c *context) {
			coreColumn(c).Padding(10).Children(func() {
				bar := coreProgress(c, 0.25).Height(10)
				if reverse {
					bar.Reverse()
				}
			})
		}, 220, 30, 1)
		accent := LightTheme().Accent
		filled := func(x int) bool {
			c := img.RGBAAt(x, 15)
			return c.R == accent.R && c.G == accent.G && c.B == accent.B
		}
		// The fill takes a quarter of the 200 DIPs of the bar, from the
		// left, or from the right when reversed.
		if filled(20) == reverse || filled(200) != reverse || filled(110) {
			t.Errorf("reverse %v: filled at 20 %v, at 110 %v, at 200 %v", reverse, filled(20), filled(110), filled(200))
		}
	}
}

func TestTooltipHidesOnPress(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreRow(c).Gap(40).Padding(40).Children(func() {
			coreButton(c, "Save").Tooltip("Save the file")
			coreMenuButton(c, "More", func(m *Menu) { m.Item("Duplicate") }).Tooltip("More actions")
		})
	}, 400, 200)
	rest := func(x, y float32) {
		tt.Move(x, y)
		tt.rt.tips.hoverSince = time.Now().Add(-time.Second)
		tt.Frame()
	}
	r, _ := tt.Find("Save")
	x, y := center(r)
	rest(x, y)
	if !tt.HasText("Save the file") {
		t.Fatal("no tooltip over the button")
	}
	// A click hides it, and it stays hidden while the pointer rests on
	// the button.
	tt.ClickAt(x, y)
	rest(x+2, y)
	if tt.HasText("Save the file") {
		t.Error("the tooltip shows again after a click")
	}
	// It shows again as the pointer comes back.
	rest(5, 5)
	rest(x, y)
	if !tt.HasText("Save the file") {
		t.Error("no tooltip as the pointer comes back")
	}
	// A click by the keys closes it too.
	tt.Key(0, KeyEnter)
	if tt.HasText("Save the file") {
		t.Error("the tooltip shows after Enter clicked the button")
	}
	// A press before it shows keeps it from showing.
	rest(5, 5)
	tt.Move(x, y)
	tt.ClickAt(x, y)
	rest(x, y)
	if tt.HasText("Save the file") {
		t.Error("the tooltip shows after a click before the delay")
	}
	// The press opening a menu button's menu closes it.
	r, _ = tt.Find("More")
	x, y = center(r)
	rest(x, y)
	if !tt.HasText("More actions") {
		t.Fatal("no tooltip over the menu button")
	}
	tt.Press(x, y)
	if tt.Menu() == nil || tt.HasText("More actions") {
		t.Errorf("menu %q; the tooltip shows with the menu", tt.Menu())
	}
	tt.Release(x, y)
	tt.CloseMenu()
	rest(x, y)
	if tt.HasText("More actions") {
		t.Error("the tooltip shows again after the menu")
	}
}
