package ui

import (
	"bytes"
	"fmt"
	"image/color"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// clockTester returns a tester whose frames happen at the time it sets.
func clockTester(view func(c *context), w, h int) (*Tester, *time.Time) {
	now := time.Unix(1_000_000, 0)
	tt := coreNewTester(view, w, h)
	tt.rt.clock = func() time.Time { return now }
	tt.Frame()
	return tt, &now
}

// at returns where the text s shows, failing when it does not.
func at(t *testing.T, tt *Tester, s string) Rect {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q does not show", s)
	}
	return r
}

func TestTransitionMovesElements(t *testing.T) {
	order := []string{"A", "B"}
	tt, now := clockTester(func(c *context) {
		coreColumn(c).Children(func() {
			for _, s := range order {
				coreRow(c).Key(s).Height(20).Transition(ElementTransition{Duration: 200 * time.Millisecond, Ease: Linear}).Children(func() {
					coreText(c, s)
				})
			}
		})
	}, 200, 100)
	// Where the texts are in their rows.
	off := at(t, tt, "A").Y
	if y := at(t, tt, "B").Y - off; y != 20 {
		t.Fatalf("B starts at %v", y)
	}
	order = []string{"B", "A"}
	tt.Frame()
	if y := at(t, tt, "B").Y - off; y != 20 {
		t.Errorf("B jumped to %v as the change came", y)
	}
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	b, a := at(t, tt, "B").Y-off, at(t, tt, "A").Y-off
	if b != 10 || a != 10 {
		t.Errorf("halfway, B is at %v and A at %v, not both at 10", b, a)
	}
	*now = now.Add(150 * time.Millisecond)
	tt.Frame()
	if b, a := at(t, tt, "B").Y-off, at(t, tt, "A").Y-off; b != 0 || a != 20 {
		t.Errorf("once over, B is at %v and A at %v", b, a)
	}
	if n := len(tt.rt.trans); n != 2 {
		t.Errorf("%d transitions kept for 2 elements", n)
	}
}

func TestTransitionResizesAndLaysOutContent(t *testing.T) {
	wide := false
	tt, now := clockTester(func(c *context) {
		w := float32(100)
		if wide {
			w = 300
		}
		coreBox(c).Width(w).Height(40).Transition(ElementTransition{Size: true, Ease: Linear}).Children(func() {
			coreText(c, "Right").AlignSelf(End)
		})
	}, 400, 100)
	if x := at(t, tt, "Right"); x.X+x.W > 101 {
		t.Fatalf("Right ends at %v", x.X+x.W)
	}
	wide = true
	tt.Frame()
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	// The content lays out at the size shown: halfway, 200 wide.
	if r := at(t, tt, "Right"); r.X+r.W < 195 || r.X+r.W > 205 {
		t.Errorf("halfway, Right ends at %v", r.X+r.W)
	}
	*now = now.Add(time.Second)
	tt.Frame()
	if r := at(t, tt, "Right"); r.X+r.W < 299 {
		t.Errorf("once over, Right ends at %v", r.X+r.W)
	}
}

func TestTransitionEntersAndExits(t *testing.T) {
	items := []string{"One", "Two"}
	motion := ElementTransition{Ease: Linear, Enter: &Motion{Collapse: true}, Exit: &Motion{Collapse: true}}
	tt, now := clockTester(func(c *context) {
		coreColumn(c).Children(func() {
			for _, s := range items {
				coreRow(c).Key(s).Height(30).Transition(motion).Children(func() { coreText(c, s) })
			}
		})
	}, 200, 200)
	if _, ok := tt.Find("One"); !ok {
		t.Fatal("the first frame shows nothing")
	}
	// The first frame's elements appear at once: their parent is new too.
	if n := len(tt.rt.trans); n != 2 {
		t.Fatalf("%d transitions", n)
	}
	for _, r := range tt.rt.trans {
		if r.moving != 0 {
			t.Error("an element of the first frame came in")
		}
	}

	items = []string{"New", "One", "Two"}
	tt.Frame()
	row := func(s string) *node {
		for e := tt.rt.c.root.first.first; e != nil; e = e.next {
			if e.first != nil && e.first.text == s {
				return e
			}
		}
		return nil
	}
	if e := row("New"); e == nil || e.h != 0 || e.flags&flagClip == 0 {
		t.Fatalf("the new row starts at %+v", e)
	}
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if h := row("New").h; h < 14 || h > 16 {
		t.Errorf("halfway, the new row is %v high", h)
	}
	*now = now.Add(150 * time.Millisecond)
	tt.Frame()
	if e := row("New"); e.h != 30 {
		t.Errorf("once in, the new row is %v high", e.h)
	}

	// Gone, a row leaves as an inert copy, which collapses under its
	// siblings and takes no input.
	items = []string{"New", "Two"}
	tt.Frame()
	var ghost *node
	for e := tt.rt.c.root.first.first; e != nil; e = e.next {
		if e.leaving != 0 {
			ghost = e
		}
	}
	if ghost == nil || ghost.first == nil || ghost.first.text != "One" {
		t.Fatalf("no copy of the row gone: %+v", ghost)
	}
	if ghost.flags&flagInert == 0 || ghost.h != 30 {
		t.Errorf("the copy is not inert, or %v high", ghost.h)
	}
	if _, ok := tt.Find("One"); ok {
		t.Error("the row gone is still found")
	}
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	for e := tt.rt.c.root.first.first; e != nil; e = e.next {
		if e.leaving != 0 && (e.h < 14 || e.h > 16) {
			t.Errorf("halfway, the copy is %v high", e.h)
		}
	}
	*now = now.Add(150 * time.Millisecond)
	tt.Frame()
	for e := tt.rt.c.root.first.first; e != nil; e = e.next {
		if e.leaving != 0 {
			t.Error("the copy is still there once its exit ended")
		}
	}
	if n := len(tt.rt.trans); n != 2 {
		t.Errorf("%d transitions kept for 2 elements", n)
	}

	// Back while its copy was leaving, a row comes back from there.
	items = []string{"New", "Two", "Three"}
	tt.Frame()
	*now = now.Add(time.Second)
	tt.Frame()
	items = []string{"New", "Two"}
	tt.Frame()
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	items = []string{"New", "Two", "Three"}
	tt.Frame()
	leaving := 0
	for e := tt.rt.c.root.first.first; e != nil; e = e.next {
		if e.leaving != 0 {
			leaving++
		}
	}
	if leaving != 0 {
		t.Errorf("%d copies left as the row came back", leaving)
	}
	if e := row("Three"); e == nil || e.h < 14 || e.h > 16 {
		t.Errorf("the row back does not start from where its copy was: %+v", e)
	}
}

func TestTransitionFadesColors(t *testing.T) {
	bg := RGB(0, 0, 255)
	tt, now := clockTester(func(c *context) {
		coreBox(c).Size(50, 50).Background(bg).Transition(ElementTransition{Colors: true, Ease: Linear})
	}, 100, 100)
	pixel := func() color.RGBA { return tt.Image().RGBAAt(25, 25) }
	if p := pixel(); p.B != 255 {
		t.Fatalf("the box is %v", p)
	}
	bg = RGB(255, 0, 0)
	tt.Frame()
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if p := pixel(); p.R < 110 || p.R > 145 || p.B < 110 || p.B > 145 {
		t.Errorf("halfway, the box is %v", p)
	}
	*now = now.Add(time.Second)
	tt.Frame()
	if p := pixel(); p.R != 255 || p.B != 0 {
		t.Errorf("once over, the box is %v", p)
	}
}

func TestTransitionWithoutMotion(t *testing.T) {
	x := float32(0)
	tt, now := clockTester(func(c *context) {
		coreBox(c).Size(100, 100).Children(func() {
			coreText(c, "Moving").Absolute().Left(x).Transition(ElementTransition{})
		})
	}, 300, 200)
	tt.SetPreferences(Preferences{ReduceMotion: true})
	x = 50
	tt.Frame()
	if r := at(t, tt, "Moving"); r.X != 50 {
		t.Errorf("without motion, it shows at %v", r.X)
	}
	tt.SetPreferences(Preferences{})
	x = 0
	tt.Frame()
	if r := at(t, tt, "Moving"); r.X == 0 {
		t.Error("with motion, it jumped")
	}
	// Resizing the window moves nothing smoothly.
	*now = now.Add(time.Second)
	tt.Frame()
	x = 20
	tt.SetSize(400, 200)
	if r := at(t, tt, "Moving"); r.X != 20 {
		t.Errorf("as the window resized, it shows at %v", r.X)
	}
}

func TestDuplicateKeys(t *testing.T) {
	clicks := 0
	view := func(c *context) {
		coreColumn(c).Children(func() {
			for _, s := range []string{"First", "Second"} {
				coreRow(c).Key("same").Children(func() {
					if coreButton(c, s).Clicked() {
						clicks++
					}
				})
			}
		})
	}
	func() {
		defer func() {
			msg, _ := recover().(string)
			if !strings.Contains(msg, `key "same"`) {
				t.Errorf("tests do not panic on a duplicate key: %q", msg)
			}
		}()
		coreNewTester(view, 200, 100)
	}()

	// Apps log it once, and the inspector lists it.
	var out bytes.Buffer
	log.SetOutput(&out)
	defer log.SetOutput(os.Stderr)
	tt := lenientTester(view, 200, 100)
	tt.Frame()
	tt.Frame()
	if n := strings.Count(out.String(), `key "same"`); n != 1 {
		t.Errorf("logged %d times: %s", n, out.String())
	}
	if len(tt.rt.warnings) != 1 {
		t.Errorf("warnings: %q", tt.rt.warnings)
	}
}

func TestDividers(t *testing.T) {
	red := RGB(255, 0, 0)
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Width(100).Gap(10).Dividers(2, red).Children(func() {
			for range 3 {
				coreBox(c).Height(20)
			}
		})
		coreRow(c).Height(20).Gap(10).Dividers(1, red).Children(func() {
			coreBox(c).Width(30)
			coreBox(c).Width(30)
		})
	}, 200, 200)
	img := tt.Image()
	isRed := func(x, y int) bool { p := img.RGBAAt(x, y); return p.R == 255 && p.G == 0 }
	// Between the boxes of the column, at 25 and 55, across it; not after
	// the last.
	for _, y := range []int{24, 25, 54, 55} {
		if !isRed(0, y) || !isRed(99, y) {
			t.Errorf("no divider at y %d", y)
		}
	}
	for _, y := range []int{10, 23, 26, 40, 85} {
		if isRed(50, y) {
			t.Errorf("a divider at y %d", y)
		}
	}
	// The row's, at x 35, below the column.
	if !isRed(35, 90) || isRed(35, 70) || isRed(65, 90) {
		t.Error("the row's divider is not between its children")
	}
}

func TestListDividers(t *testing.T) {
	red := RGB(255, 0, 0)
	tt := coreNewTester(func(c *context) {
		coreList(c, nil, 3, func(i int) { coreBox(c).Height(20) }).Height(100).Dividers(1, red)
	}, 200, 100)
	img := tt.Image()
	for _, y := range []int{20, 40} {
		if p := img.RGBAAt(50, y); p != (color.RGBA{255, 0, 0, 255}) {
			t.Errorf("no divider at y %d: %v", y, p)
		}
	}
	if p := img.RGBAAt(50, 60); p.R == 255 && p.G == 0 {
		t.Error("a divider after the last row")
	}
}

func TestAttach(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(100, 60).Margin(20).Padding(5).Children(func() {
			coreText(c, "Badge").Size(20, 10).Attach(AnchorTopRight, AnchorCenter)
			coreText(c, "Corner").Size(30, 10).Attach(AnchorBottomRight, AnchorBottomRight).Right(4).Bottom(4)
			coreText(c, "Middle").Size(40, 10).Attach(AnchorCenter, AnchorCenter)
		})
	}, 300, 200)
	for _, c := range []struct {
		s    string
		x, y float32
	}{
		{"Badge", 20 + 100 - 10, 20 - 5},
		{"Corner", 20 + 100 - 30 - 4, 20 + 60 - 10 - 4},
		{"Middle", 20 + 30, 20 + 25},
	} {
		if r := at(t, tt, c.s); r.X != c.x || r.Y != c.y {
			t.Errorf("%s at %v, %v; want %v, %v", c.s, r.X, r.Y, c.x, c.y)
		}
	}
}

func TestTransitionResizesAList(t *testing.T) {
	open := true
	tt, now := clockTester(func(c *context) {
		coreRow(c).Fill().Children(func() {
			w := float32(0)
			if open {
				w = 200
			}
			coreColumn(c).Width(w).FillHeight().ClipX().Transition(ElementTransition{Size: true, Ease: Linear}).Children(func() {
				coreList(c, nil, 1000, func(i int) {
					coreText(c, fmt.Sprintf("Row %d", i)).Padding(4)
				}).Grow(1)
			})
			coreText(c, "Main").Grow(1)
		})
	}, 600, 300)
	if !tt.HasText("Row 0") {
		t.Fatal("the list shows no rows")
	}
	open = false
	tt.Frame()
	for i := 1; i <= 4; i++ {
		*now = now.Add(50 * time.Millisecond)
		tt.Frame()
		if r, ok := tt.Find("Row 0"); i < 4 && (!ok || r.W <= 0) {
			t.Errorf("after %d ms, the list shows no rows", i*50)
		}
	}
	if r, ok := tt.Find("Row 0"); ok && r.W > 0 {
		t.Errorf("the closed list still shows rows: %v", r)
	}
	open = true
	tt.Frame()
	*now = now.Add(100 * time.Millisecond)
	tt.Frame()
	if r, ok := tt.Find("Row 0"); !ok || r.W <= 0 {
		t.Error("the list opening shows no rows")
	}
}

// lenientTester is NewTester as an app's window is: duplicate keys only
// log.
func lenientTester(view func(c *context), width, height int) *Tester {
	h := &headless{w: float32(width), h: float32(height), scale: 1}
	t := &Tester{rt: newRuntime(view, h), h: h}
	t.rt.collect = true
	t.settle()
	return t
}
