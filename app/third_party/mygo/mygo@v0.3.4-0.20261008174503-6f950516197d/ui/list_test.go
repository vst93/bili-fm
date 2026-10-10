package ui

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// rowBox returns the box of row i of the list s in the last frame.
func rowBox(tt *Tester, s *ListState, i int) (Rect, bool) {
	for id, r := range s.rows {
		if r.row == i {
			if st := tt.rt.states[id]; st != nil {
				return Rect{st.x, st.y, st.w, st.h}, true
			}
		}
	}
	return Rect{}, false
}

// listStateOf returns the state of the element of the list s.
func listStateOf(s *ListState) *state { return s.frame.e.st }

// checkRows reports rows of the list s in view that do not follow each
// other gap apart, or leave the view uncovered between top and bottom.
func checkRows(t *testing.T, tt *Tester, s *ListState, gap, top, bottom float32) {
	t.Helper()
	first, last := s.Visible()
	if last < first {
		t.Fatalf("no row in view")
	}
	prev, _ := rowBox(tt, s, first)
	if prev.Y > top+0.01 {
		t.Errorf("row %d, the first in view, starts at %v, below the top %v", first, prev.Y, top)
	}
	for i := first + 1; i <= last; i++ {
		r, ok := rowBox(tt, s, i)
		if !ok {
			t.Fatalf("row %d in view was not built", i)
		}
		if d := r.Y - (prev.Y + prev.H + gap); math.Abs(float64(d)) > 0.01 {
			t.Errorf("row %d starts %v from where row %d ends", i, d, i-1)
		}
		prev = r
	}
	if s.last < s.heights.n-1 && prev.Y+prev.H < bottom-0.01 {
		t.Errorf("row %d, the last in view, ends at %v, above the bottom %v", last, prev.Y+prev.H, bottom)
	}
}

// varied is the height of row i of lists of rows of varied heights.
func varied(i int) float32 { return float32(20 + i*7%45) }

func TestListVariableHeights(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 10000, func(i int) {
			coreBox(c).Height(varied(i)).Children(func() { coreTextf(c, "Row %d", i) })
		}).Grow(1)
	}, 300, 400)
	if first, last := s.Visible(); first != 0 || last < 10 {
		t.Fatalf("shows rows %d to %d", first, last)
	}
	checkRows(t, tt, &s, 0, 0, 400)
	// The wheel moves the rows by as much as it scrolls.
	r5, _ := rowBox(tt, &s, 5)
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, 37.5)
	if r, _ := rowBox(tt, &s, 5); r.Y != r5.Y-37.5 {
		t.Errorf("scrolled 37.5: row 5 moved from %v to %v", r5.Y, r.Y)
	}
	checkRows(t, tt, &s, 0, 0, 400)
	// Far down, then back up through rows measured on the way.
	for range 200 {
		tt.Scroll(100, 100, 0, 300)
	}
	checkRows(t, tt, &s, 0, 0, 400)
	for range 401 {
		tt.Scroll(100, 100, 0, -150)
		checkRows(t, tt, &s, 0, 0, 400)
	}
	if first, _ := s.Visible(); first != 0 || listStateOf(&s).scrollY != 0 {
		t.Errorf("back at the top: row %d first, the offset %v", first, listStateOf(&s).scrollY)
	}
	if r, _ := rowBox(tt, &s, 0); r.Y != 0 {
		t.Errorf("back at the top, row 0 is at %v", r.Y)
	}
}

func TestListFillsTheFirstFrame(t *testing.T) {
	// Rows far smaller than the estimate: the list builds more as it lays
	// out, and the first frame shows no gap.
	var s ListState
	frames := 0
	tt := coreNewTester(func(c *context) {
		frames++
		coreList(c, &s, 1000, func(i int) { coreBox(c).Height(3) }).Grow(1)
	}, 300, 400)
	if first, last := s.Visible(); first != 0 || last < 133 {
		t.Errorf("the first frame shows rows %d to %d of 3 DIPs in 400", first, last)
	}
	checkRows(t, tt, &s, 0, 0, 400)
	if frames != 1 {
		t.Errorf("%d frames", frames)
	}
}

func TestListHugeScrollsByFractions(t *testing.T) {
	// 50 million rows of 20 DIPs: a billion DIPs of content, which float32
	// counts in steps of 64.
	const n = 50_000_000
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, n, func(i int) { coreBox(c).Height(20) }).Grow(1)
	}, 300, 400)
	s.ScrollTo(n/2, Start)
	tt.Frame()
	if r, ok := rowBox(tt, &s, n/2); !ok || r.Y != 0 {
		t.Fatalf("scrolled to row %d: it is at %v (%v)", n/2, r.Y, ok)
	}
	if y := listStateOf(&s).scrollY; y != n/2*20 {
		t.Errorf("the offset is %v", y)
	}
	tt.Move(100, 100)
	for k := 1; k <= 3; k++ {
		tt.Scroll(100, 100, 0, 0.25)
		if r, _ := rowBox(tt, &s, n/2); r.Y != -0.25*float32(k) {
			t.Errorf("scrolled by %v: the row is at %v", 0.25*float32(k), r.Y)
		}
	}
	checkRows(t, tt, &s, 0, 0, 400)
	tt.Key(0, KeyEnd)
	if r, _ := rowBox(tt, &s, n-1); r.Y+r.H != 400 {
		t.Errorf("at the end, the last row ends at %v", r.Y+r.H)
	}
}

func TestListScrollTo(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 5000, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1).Padding(10, 0)
	}, 300, 400)
	for _, tc := range []struct {
		row   int
		align Align
	}{{3000, Center}, {1000, End}, {4000, Start}, {4999, Start}, {0, End}, {2500, Center}} {
		s.ScrollTo(tc.row, tc.align)
		tt.Frame()
		r, ok := rowBox(tt, &s, tc.row)
		var want float32
		switch tc.align {
		case Start:
			want = 10
		case Center:
			want = (400 - r.H) / 2
		case End:
			want = 400 - 10 - r.H
		}
		switch {
		case tc.row == 4999:
			// The end goes no further than the bottom.
			want = 400 - 10 - r.H
		case tc.row == 0:
			want = 10
		}
		if !ok || math.Abs(float64(r.Y-want)) > 0.001 {
			t.Errorf("ScrollTo(%d, %v): the row is at %v, not %v (%v)", tc.row, tc.align, r.Y, want, ok)
		}
		checkRows(t, tt, &s, 0, 10, 390)
	}
	// As little as shows it: not at all when it shows, to the bottom from
	// above, to the top from below.
	s.ScrollTo(2000, Center)
	tt.Frame()
	first, last := s.Visible()
	before, _ := rowBox(tt, &s, first+2)
	s.ScrollIntoView(first + 2)
	tt.Frame()
	if r, _ := rowBox(tt, &s, first+2); r != before {
		t.Errorf("ScrollIntoView of a row in view moved it from %v to %v", before, r)
	}
	s.ScrollIntoView(last + 10)
	tt.Frame()
	if r, _ := rowBox(tt, &s, last+10); r.Y+r.H != 400-10 {
		t.Errorf("ScrollIntoView of a row below: it ends at %v", r.Y+r.H)
	}
	s.ScrollIntoView(first - 50)
	tt.Frame()
	if r, _ := rowBox(tt, &s, first-50); r.Y != 10 {
		t.Errorf("ScrollIntoView of a row above: it starts at %v", r.Y)
	}
}

func TestListKeepsPlaceAsRowsAboveChange(t *testing.T) {
	var s ListState
	grow := float32(0)
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1000, func(i int) {
			h := float32(30)
			if i < 40 {
				h += grow
			}
			coreBox(c).Height(h).Children(func() { coreTextf(c, "Row %d", i) })
		}).Grow(1)
	}, 300, 400)
	s.ScrollTo(45, Start)
	tt.Frame()
	before, _ := rowBox(tt, &s, 50)
	// Rows above grow, and rows around shrink as the list measures them.
	grow = 25
	tt.Frame()
	if r, _ := rowBox(tt, &s, 50); r != before {
		t.Errorf("rows above grew: row 50 moved from %v to %v", before, r)
	}
	// Scrolled back up through them, the list reaches its start exactly.
	tt.Move(100, 100)
	for range 100 {
		tt.Scroll(100, 100, 0, -40)
		checkRows(t, tt, &s, 0, 0, 400)
	}
	if r, _ := rowBox(tt, &s, 0); r.Y != 0 || listStateOf(&s).scrollY != 0 {
		t.Errorf("at the top: row 0 at %v, the offset %v", r.Y, listStateOf(&s).scrollY)
	}
}

func TestListKeys(t *testing.T) {
	ids := make([]int, 200)
	for i := range ids {
		ids[i] = 1000 + i
	}
	opened := map[int]bool{}
	s := ListState{Key: func(i int) any { return ids[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(ids), func(i int) {
			id := ids[i]
			// State kept in the row: whether it was opened.
			open := coreLocal(coreBox(c).Height(30), "open", func() bool { return opened[id] })
			coreTextf(c, "Item %d %v", id, *open)
		}).Grow(1)
	}, 300, 400)
	s.ScrollTo(100, Start)
	tt.Frame()
	before, _ := tt.Find("Item 1100 false")
	// Older items load above: the list keeps showing the same items.
	older := make([]int, 50)
	for i := range older {
		older[i] = 500 + i
	}
	ids = append(older, ids...)
	tt.Frame()
	if r, ok := tt.Find("Item 1100 false"); !ok || r != before {
		t.Errorf("50 rows added above: item 1100 moved from %v to %v (%v)", before, r, ok)
	}
	if first, _ := s.Visible(); first != 150 {
		t.Errorf("the first row in view is %d", first)
	}
	// The row of an item keeps its state when its index changes.
	opened[1101] = true
	ids = ids[10:] // ten rows go away above
	tt.Frame()
	if _, ok := tt.Find("Item 1101 false"); !ok {
		t.Errorf("the row of item 1101 lost its state: %q", tt.Texts())
	}
	if r, ok := tt.Find("Item 1100 false"); !ok || r != before {
		t.Errorf("10 rows removed above: item 1100 moved from %v to %v (%v)", before, r, ok)
	}
}

func TestListFollowEnd(t *testing.T) {
	n := 5
	s := ListState{FollowEnd: true}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, n, func(i int) {
			coreBox(c).Height(varied(i)).Children(func() { coreTextf(c, "Message %d", i) })
		}).Grow(1)
	}, 300, 400)
	// Few rows fit, from the top.
	if r, _ := rowBox(tt, &s, 0); r.Y != 0 || !s.AtEnd() {
		t.Errorf("five rows: the first is at %v, at the end %v", r.Y, s.AtEnd())
	}
	// As rows come, the list follows the end.
	for range 40 {
		n++
		tt.Frame()
		if r, _ := rowBox(tt, &s, n-1); r.Y+r.H != 400 && n > 12 {
			t.Fatalf("%d rows: the last ends at %v", n, r.Y+r.H)
		}
	}
	// Scrolled away from the end, it stays where it is.
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, -100)
	r20, _ := rowBox(tt, &s, n-10)
	n += 3
	tt.Frame()
	if r, _ := rowBox(tt, &s, n-13); r != r20 || s.AtEnd() {
		t.Errorf("scrolled up, rows came: the row moved from %v to %v; at the end %v", r20, r, s.AtEnd())
	}
	// Back at the end, it follows again.
	s.ScrollToEnd()
	tt.Frame()
	n++
	tt.Frame()
	if r, _ := rowBox(tt, &s, n-1); r.Y+r.H != 400 || !s.AtEnd() {
		t.Errorf("back at the end: the last row ends at %v", r.Y+r.H)
	}
	// A last row that grows stays in view.
	tt.Scroll(100, 100, 0, 1e6)
	tt.Frame()
	if !s.AtEnd() {
		t.Error("the wheel to the end: not at the end")
	}
}

func TestListFollowEndStartsAtTheEnd(t *testing.T) {
	s := ListState{FollowEnd: true}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1000, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1)
	}, 300, 400)
	if r, _ := rowBox(tt, &s, 999); r.Y+r.H != 400 || !s.AtEnd() {
		t.Errorf("the first frame: the last row ends at %v", r.Y+r.H)
	}
	checkRows(t, tt, &s, 0, 0, 400)
}

func TestListJustifyEnd(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 3, func(i int) { coreBox(c).Height(50) }).Grow(1).Justify(End).Padding(8)
	}, 300, 400)
	for i := range 3 {
		if r, _ := rowBox(tt, &s, i); r.Y != 400-8-50*float32(3-i) {
			t.Errorf("row %d is at %v", i, r.Y)
		}
	}
}

func TestListGapAndPadding(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 100, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1).Gap(6).Padding(12, 4).Border(1, RGB(0, 0, 0))
	}, 300, 400)
	if r, _ := rowBox(tt, &s, 0); r.Y != 13 || r.X != 5 || r.W != 300-10 {
		t.Errorf("row 0 is at %v", r)
	}
	checkRows(t, tt, &s, 6, 13, 387)
	var sum float32
	for i := range 100 {
		sum += varied(i)
	}
	// Scrolled to the end through every row, which measures them all.
	tt.Move(100, 100)
	for !s.AtEnd() {
		tt.Scroll(100, 100, 0, 100)
		checkRows(t, tt, &s, 6, 13, 387)
	}
	if want := float64(2 + 24 + sum + 6*99); listStateOf(&s).contentH != want {
		t.Errorf("the content is %v high, not %v", listStateOf(&s).contentH, want)
	}
	if r, _ := rowBox(tt, &s, 99); r.Y+r.H != 400-13 {
		t.Errorf("at the end, the last row ends at %v", r.Y+r.H)
	}
}

func TestListRowsGoAway(t *testing.T) {
	n := 1000
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, n, func(i int) { coreBox(c).Height(30) }).Grow(1)
	}, 300, 400)
	s.ScrollToEnd()
	tt.Frame()
	n = 20
	tt.Frame()
	if r, _ := rowBox(tt, &s, 19); r.Y+r.H != 400 {
		t.Errorf("20 rows left: the last ends at %v", r.Y+r.H)
	}
	checkRows(t, tt, &s, 0, 0, 400)
	n = 5
	tt.Frame()
	if r, _ := rowBox(tt, &s, 0); r.Y != 0 {
		t.Errorf("5 rows left: the first is at %v", r.Y)
	}
	n = 0
	tt.Frame()
	if first, last := s.Visible(); last >= first || !s.AtEnd() {
		t.Errorf("no rows: %d to %d", first, last)
	}
}

func TestListSelection(t *testing.T) {
	sel := -1
	s := ListState{Selected: &sel}
	submitted := 0
	var list *node
	tt := coreNewTester(func(c *context) {
		list = coreList(c, &s, 500, func(i int) {
			coreBox(c).Height(varied(i)).Children(func() { coreTextf(c, "Row %d", i) })
		}).Grow(1)
		if list.Submitted() {
			submitted++
		}
	}, 300, 400)
	if err := tt.Click("Row 2"); err != nil {
		t.Fatal(err)
	}
	if sel != 2 || tt.rt.focused != list.id {
		t.Fatalf("clicked row 2: chose %d, focus %v", sel, tt.rt.focused == list.id)
	}
	// Down moves the choice, and the list shows it.
	for k := 3; k < 60; k++ {
		tt.Key(0, KeyDown)
		r, ok := rowBox(tt, &s, sel)
		if sel != k || !ok || r.Y < 0 || r.Y+r.H > 400 {
			t.Fatalf("Down: chose %d, which is at %v (%v)", sel, r, ok)
		}
	}
	tt.Key(0, KeyEnd)
	if r, _ := rowBox(tt, &s, 499); sel != 499 || r.Y+r.H != 400 {
		t.Errorf("End chose %d, the last row ends at %v", sel, r.Y+r.H)
	}
	tt.Key(0, KeyHome)
	if r, _ := rowBox(tt, &s, 0); sel != 0 || r.Y != 0 {
		t.Errorf("Home chose %d, the first row is at %v", sel, r.Y)
	}
	tt.Key(0, KeyUp)
	if sel != 0 {
		t.Errorf("Up at the start chose %d", sel)
	}
	tt.Key(0, KeyEnter)
	if submitted != 1 {
		t.Errorf("Enter: submitted %d times", submitted)
	}
	// The chosen row tells assistive technology.
	tt.rt.access = true
	tt.Frame()
	tree := tt.rt.accessTree()
	rows := 0
	for _, n := range tree.Nodes {
		if n.Role == platform.RoleListItem {
			rows++
		}
	}
	if rows == 0 {
		t.Error("no rows for assistive technology")
	}
}

func TestListKeysKeepTheChoice(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	sel := 2
	s := ListState{Selected: &sel, Key: func(i int) any { return ids[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(ids), func(i int) { coreText(c, ids[i]).Height(20) }).Grow(1)
	}, 300, 400)
	ids = append([]string{"z", "y"}, ids...)
	tt.Frame()
	if sel != 4 || ids[sel] != "c" {
		t.Errorf("two rows added above: chose %d", sel)
	}
	// The app choosing another row is not rows moving.
	sel = 0
	tt.Frame()
	if sel != 0 {
		t.Errorf("chose row 0, it is %d", sel)
	}
}

func TestListKeepsTheFocusedRow(t *testing.T) {
	texts := make([]string, 300)
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(texts), func(i int) {
			coreRow(c).Height(30).Children(func() {
				coreTextInput(c, &texts[i]).Label(fmt.Sprintf("Field %d", i))
			})
		}).Grow(1)
	}, 300, 400)
	r, _ := tt.Find("Field 3")
	tt.ClickAt(r.X+5, r.Y+5)
	focus := tt.rt.focused
	if focus == 0 {
		t.Fatal("no focus")
	}
	// Scrolled far away, the field keeps the focus and takes the keys.
	tt.Move(100, 100)
	for range 20 {
		tt.Scroll(100, 100, 0, 200)
	}
	if first, _ := s.Visible(); first < 100 {
		t.Fatalf("scrolled to row %d", first)
	}
	tt.Type("kept")
	if tt.rt.focused != focus || texts[3] != "kept" {
		t.Errorf("typed away from the field: %q, the focus moved %v", texts[3], tt.rt.focused != focus)
	}
}

func TestListStickyHeaders(t *testing.T) {
	// Sections of a header and 20 rows of 30 DIPs.
	header := func(i int) bool { return i%21 == 0 }
	s := ListState{Header: header}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 2100, func(i int) {
			if header(i) {
				coreBox(c).Height(24).Background(RGB(200, 200, 200)).Children(func() { coreTextf(c, "Section %d", i/21) })
				return
			}
			coreBox(c).Height(30).Children(func() { coreTextf(c, "Row %d", i) })
		}).Grow(1)
	}, 300, 400)
	if r, _ := rowBox(tt, &s, 0); r.Y != 0 {
		t.Errorf("the first header is at %v", r.Y)
	}
	// Into the middle of section 5, far down: its header is at the top.
	s.ScrollTo(5*21+8, Start)
	tt.Frame()
	if r, ok := rowBox(tt, &s, 5*21); !ok || r.Y != 0 {
		t.Errorf("in section 5, its header is at %v (%v)", r.Y, ok)
	}
	if r, _ := tt.Find("Section 5"); r.Y != 0 {
		t.Errorf("the pinned header's text is at %v", r.Y)
	}
	// Section 6's header pushes it up.
	s.ScrollTo(6*21, Start)
	tt.Frame()
	tt.Move(100, 100)
	tt.Scroll(100, 100, 0, -14)
	next, _ := rowBox(tt, &s, 6*21)
	if r, _ := rowBox(tt, &s, 5*21); next.Y != 14 || r.Y != -10 {
		t.Errorf("the next header at %v pushes the pinned one to %v", next.Y, r)
	}
	// A header partly scrolled out pins where it is: the rows under it
	// stay put from frame to frame.
	s.ScrollTo(5*21, Start)
	tt.Frame()
	tt.Scroll(100, 100, 0, 10)
	row, _ := rowBox(tt, &s, 5*21+1)
	tt.Frame()
	if r, _ := rowBox(tt, &s, 5*21+1); r != row || r.Y != 14 {
		t.Errorf("the row under a header scrolled out by 10 moved from %v to %v", row, r)
	}
	if r, _ := rowBox(tt, &s, 5*21); r.Y != 0 {
		t.Errorf("the header scrolled out by 10 is at %v", r.Y)
	}
	// The pinned header takes the pointer over the rows under it.
	s.ScrollTo(5*21+8, Start)
	tt.Frame()
	chain := tt.rt.hitChain(10, 10)
	var pinned uint64
	for id, r := range s.rows {
		if r.row == 5*21 {
			pinned = id
		}
	}
	if !slices.Contains(chain, pinned) {
		t.Error("the pointer over the pinned header is not over it")
	}
}

func TestListRowsShowBelowThePinnedHeader(t *testing.T) {
	// Sections of a header of 24 DIPs and 20 rows of 30.
	header := func(i int) bool { return i%21 == 0 }
	sel := 300
	s := ListState{Header: header, Selected: &sel}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 2100, func(i int) {
			if header(i) {
				coreBox(c).Height(24).Background(RGB(200, 200, 200))
				return
			}
			coreBox(c).Height(30)
		}).Grow(1)
	}, 300, 400)
	s.ScrollTo(300, Start)
	tt.Frame()
	if r, _ := rowBox(tt, &s, 300); r.Y != 24 {
		t.Errorf("ScrollTo(300, Start) put it at %v, under its section's header", r.Y)
	}
	// Up moves the choice above the view: it shows below the header.
	tt.rt.focused = s.frame.e.id
	for k := 1; k <= 5; k++ {
		tt.Key(0, KeyUp)
		if r, _ := rowBox(tt, &s, sel); sel != 300-k || r.Y < 24 {
			t.Fatalf("Up %d times: chose %d, at %v", k, sel, r.Y)
		}
	}
}

func TestListStickyHeadersSkipTheChoice(t *testing.T) {
	header := func(i int) bool { return i%5 == 0 }
	sel := 1
	s := ListState{Header: header, Selected: &sel}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 50, func(i int) { coreBox(c).Height(20) }).Grow(1)
	}, 300, 400)
	tt.rt.focused = s.frame.e.id
	for range 3 {
		tt.Key(0, KeyDown)
	}
	tt.Key(0, KeyDown)
	if sel != 6 {
		t.Errorf("Down past a header chose %d", sel)
	}
	tt.Key(0, KeyHome)
	if sel != 1 {
		t.Errorf("Home chose %d", sel)
	}
}

func TestListScrollBarDragReachesTheEnd(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 5000, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1)
	}, 300, 400)
	tt.Move(150, 200)
	st := listStateOf(&s)
	g := scrollBars(Rect{st.x, st.y, st.w, st.h}, st.barInset, float32(st.contentH), float32(st.contentH), 0, float32(st.scrollY), st.flags, 6)
	x, y := g.v.X+g.v.W/2, g.v.Y+g.v.H/2
	tt.Move(x, y)
	tt.Press(x, y)
	for k := 1; k <= 40; k++ {
		tt.Move(x, y+float32(k)*10)
		tt.Frame()
		checkRows(t, tt, &s, 0, 0, 400)
	}
	tt.Release(x, y+400)
	if r, _ := rowBox(tt, &s, 4999); !s.AtEnd() || r.Y+r.H != 400 {
		t.Errorf("dragged to the end: the last row ends at %v", r.Y+r.H)
	}
}

// A drag of the thumb moves through the rows as far as the pointer moves
// along the track, though the rows measured meanwhile turn out much
// shorter than estimated from the tall rows at the top.
func TestListScrollBarDragFollowsThePointerAsEstimatesChange(t *testing.T) {
	var s ListState
	height := func(i int) float32 {
		if i < 8 {
			return 300
		}
		return 20
	}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 5000, func(i int) { coreBox(c).Height(height(i)) }).Grow(1)
	}, 300, 400)
	tt.Move(150, 200)
	st := listStateOf(&s)
	g := scrollBars(Rect{st.x, st.y, st.w, st.h}, st.barInset, float32(st.contentH), float32(st.contentH), 0, float32(st.scrollY), st.flags, 6)
	x, y := g.v.X+g.v.W/2, g.v.Y+g.v.H/2
	travel := g.vTrack.H - 4 - g.v.H
	tt.Move(x, y)
	tt.Press(x, y)
	last := -1
	for k := 1; float32(k)*8 < travel; k++ {
		tt.Move(x, y+float32(k)*8)
		first, _ := s.Visible()
		if first <= last || s.AtEnd() {
			t.Fatalf("%.0f%% along the track: the first row in view is %d after %d, at the end %v", 100*float32(k)*8/travel, first, last, s.AtEnd())
		}
		last = first
		checkRows(t, tt, &s, 0, 0, 400)
		// The thumb shows as far along the track as the offset is
		// through the content.
		along := float32(k) * 8 / travel
		through := float32(st.scrollY / (st.contentH - float64(st.h)))
		if math.Abs(float64(along-through)) > 0.02 {
			t.Errorf("%.0f%% along the track, the offset is %.0f%% through the content", 100*along, 100*through)
		}
	}
	tt.Move(x, y+travel)
	tt.Release(x, y+travel)
	if r, _ := rowBox(tt, &s, 4999); !s.AtEnd() || r.Y+r.H != 400 {
		t.Errorf("dragged to the end: the last row ends at %v", r.Y+r.H)
	}
}

func TestListStateOfTwoListsPanics(t *testing.T) {
	var s ListState
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	coreNewTester(func(c *context) {
		coreList(c, &s, 3, func(int) {}).Grow(1)
		coreList(c, &s, 3, func(int) {}).Grow(1)
	}, 300, 400)
}

func TestListTabRevealsRows(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1000, func(i int) {
			coreRow(c).Height(float32(44 + i%3*10)).Children(func() { coreButton(c, fmt.Sprintf("Button %d", i)) })
		}).Grow(1)
	}, 300, 400)
	for k := 0; k < 60; k++ {
		tt.Key(0, KeyTab)
		f := tt.rt.states[tt.rt.focused]
		if f == nil || f.y < 0 || f.y+f.h > 400 {
			t.Fatalf("Tab %d: the focus is at %+v", k, f)
		}
		checkRows(t, tt, &s, 0, 0, 400)
	}
	if first, _ := s.Visible(); first < 40 {
		t.Errorf("tabbed through 60 rows, the list shows row %d first", first)
	}
}

func TestListViewSeesWhereItIs(t *testing.T) {
	// A view showing a button while the list is away from its end: the
	// frame that scrolls away builds another, which shows it, so that the
	// buttons beside it are where the next click finds them.
	s := ListState{FollowEnd: true}
	n, older := 200, 0
	tt := coreNewTester(func(c *context) {
		coreRow(c).Gap(8).Children(func() {
			coreTextf(c, "%d messages", n).Grow(1)
			if coreButton(c, "Load older").Clicked() {
				older++
			}
			if !s.AtEnd() && coreButton(c, "Jump to latest").Clicked() {
				s.ScrollToEnd()
			}
		})
		coreList(c, &s, n, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1)
	}, 500, 400)
	tt.Move(100, 300)
	tt.Scroll(100, 300, 0, -230)
	if !tt.HasText("Jump to latest") {
		t.Fatal("scrolled away from the end: no button to jump back")
	}
	if tt.Click("Load older"); older != 1 {
		t.Errorf("clicked Load older: %d", older)
	}
	if tt.Click("Jump to latest"); !s.AtEnd() || tt.HasText("Jump to latest") {
		t.Errorf("jumped to the end: at the end %v", s.AtEnd())
	}
}

func TestListKeepsPlaceAcrossResizes(t *testing.T) {
	// Rows of text that wraps anew at each width.
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1000, func(i int) {
			coreTextf(c, "Row %d %s", i, strings.Repeat("words that wrap ", 1+i%7))
		}).Grow(1)
	}, 400, 400)
	s.ScrollTo(500, Start)
	tt.Frame()
	for _, w := range []int{300, 220, 500, 400} {
		tt.SetSize(w, 400)
		if r, ok := rowBox(tt, &s, 500); !ok || r.Y != 0 || r.W != float32(w) {
			t.Errorf("%d wide: row 500 is at %v (%v)", w, r, ok)
		}
		checkRows(t, tt, &s, 0, 0, 400)
	}
}

func TestListKeepsPlaceWhenBuiltAnew(t *testing.T) {
	var s ListState
	show := true
	tt := coreNewTester(func(c *context) {
		if !show {
			coreText(c, "Another page")
			return
		}
		coreList(c, &s, 1000, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1)
	}, 300, 400)
	s.ScrollTo(500, Start)
	tt.Frame()
	before, _ := rowBox(tt, &s, 500)
	show = false
	tt.Frame()
	show = true
	tt.Frame()
	if r, ok := rowBox(tt, &s, 500); !ok || r != before {
		t.Errorf("shown again: row 500 moved from %v to %v (%v)", before, r, ok)
	}
}

// A new ListState starts at its start, though the element showing it is
// one that was scrolled: as when an app shows other rows with a new
// ListState, and the element taking the list's place while they load
// kept its state.
func TestListNewStateStartsAtTheStart(t *testing.T) {
	var s ListState
	loading := false
	tt := coreNewTester(func(c *context) {
		if loading {
			coreBox(c).Grow(1)
			return
		}
		coreList(c, &s, 1000, func(i int) { coreBox(c).Height(varied(i)) }).Grow(1)
	}, 300, 400)
	s.ScrollTo(500, Start)
	tt.Frame()
	s = ListState{}
	loading = true
	tt.Frame()
	loading = false
	tt.Frame()
	if first, _ := s.Visible(); first != 0 {
		t.Errorf("a new ListState shows row %d first", first)
	}
	checkRows(t, tt, &s, 0, 0, 400)
}

func TestListHeights(t *testing.T) {
	// Against heights kept row by row.
	const n = 1000
	var hs listHeights
	hs.def = 32
	hs.resize(n)
	want := make([]float64, n)
	known := make([]bool, n)
	model := func(i int) float64 {
		if known[i] {
			return want[i]
		}
		sum, count := 0.0, 0
		for j := range n {
			if known[j] {
				sum += want[j]
				count++
			}
		}
		if count == 0 {
			return 32
		}
		return sum / float64(count)
	}
	check := func(step string) {
		t.Helper()
		for i := 0; i <= n; i += 37 {
			top := 0.0
			for j := range i {
				top += model(j) + 2
			}
			if d := hs.top(i, 2) - top; d > 1e-6 || d < -1e-6 {
				t.Fatalf("%s: row %d starts at %v, not %v", step, i, hs.top(i, 2), top)
			}
			if i < n {
				if r := hs.rowAt(top+model(i)/2, 2); r != i {
					t.Fatalf("%s: the middle of row %d is row %d", step, i, r)
				}
			}
		}
	}
	check("unmeasured")
	for k := range 300 {
		i := k * 7919 % n
		h := float32(10 + k%13)
		hs.set(i, h)
		want[i], known[i] = float64(h), true
	}
	check("measured")
	hs.shift(5)
	copy(want[5:], want[:n-5])
	copy(known[5:], known[:n-5])
	for i := range 5 {
		known[i] = false
	}
	check("shifted")
	hs.resize(500)
	if hs.top(1000, 0) != hs.top(500, 0) {
		t.Error("resized: rows past the end")
	}
	hs.clear()
	if hs.estimate() != 32 || hs.top(10, 0) != 320 {
		t.Errorf("cleared: estimate %v", hs.estimate())
	}
}

func TestListPinnedHeaderHandlesAClickOnce(t *testing.T) {
	// Rows come every frame, which a header built as the list lays out
	// would see as a click every frame.
	header := func(i int) bool { return i%21 == 0 }
	s := ListState{Header: header}
	n, clicks := 2100, 0
	tt := coreNewTester(func(c *context) {
		n++
		coreList(c, &s, n, func(i int) {
			if header(i) {
				coreBox(c).Height(24).Background(RGB(200, 200, 200)).Children(func() {
					if coreTextf(c, "Section %d", i/21).Clicked() {
						clicks++
					}
				})
				return
			}
			coreBox(c).Height(30)
		}).Grow(1)
	}, 300, 400)
	s.ScrollTo(5*21+15, Start)
	tt.Frame()
	if err := tt.Click("Section 5"); err != nil {
		t.Fatal(err)
	}
	for range 5 {
		tt.Frame()
	}
	if clicks != 1 {
		t.Errorf("one click on the pinned header: %d", clicks)
	}
}

func TestListScrollAndRowsAboveInOneFrame(t *testing.T) {
	ids := make([]int, 200)
	for i := range ids {
		ids[i] = 1000 + i
	}
	s := ListState{Key: func(i int) any { return ids[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(ids), func(i int) { coreTextf(c, "Item %d", ids[i]).Height(30) }).Grow(1)
	}, 300, 400)
	s.ScrollTo(100, Start)
	tt.Frame()
	tt.Move(100, 100)
	before, _ := tt.Find("Item 1101")
	// Older items load in the frame the wheel scrolls, as they do while
	// the user scrolls up.
	older := make([]int, 50)
	for i := range older {
		older[i] = 500 + i
	}
	ids = append(older, ids...)
	tt.Scroll(100, 100, 0, 10)
	if r, ok := tt.Find("Item 1101"); !ok || r.Y != before.Y-10 {
		t.Errorf("50 rows above and 10 DIPs down: item 1101 went from %v to %v (%v)", before, r, ok)
	}
}

func TestListEndShowsTheEnd(t *testing.T) {
	// The last rows are taller than the heights known estimate them.
	var s ListState
	var sc ScrollState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 2000, func(i int) {
			h := float32(20)
			if i >= 1000 {
				h = 100
			}
			coreBox(c).Height(h)
		}).Grow(1).TrackScroll(&sc)
	}, 300, 400)
	tt.Move(100, 100)
	tt.Key(0, KeyEnd)
	if r, _ := rowBox(tt, &s, 1999); !s.AtEnd() || r.Y+r.H != 400 {
		t.Errorf("End: at the end %v, the last row ends at %v", s.AtEnd(), r.Y+r.H)
	}
	tt.Key(0, KeyHome)
	sc.Y = math.MaxFloat32
	tt.Frame()
	if r, _ := rowBox(tt, &s, 1999); !s.AtEnd() || r.Y+r.H != 400 {
		t.Errorf("ScrollState at the end: at the end %v, the last row ends at %v", s.AtEnd(), r.Y+r.H)
	}
}

func TestListScrollsByTheStepIntoRowsNotMeasured(t *testing.T) {
	// A chat read upward from its end: the rows above were never measured,
	// and every fifth is ten times as tall as the others.
	s := ListState{FollowEnd: true}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 5000, func(i int) {
			h := float32(24)
			if i%5 == 0 {
				h = 240
			}
			coreBox(c).Height(h)
		}).Grow(1)
	}, 300, 400)
	tt.Move(100, 100)
	for k, step := range []float32{-120, -200, -120, -37, -200, -120, -200, -5, -300, -120} {
		first, _ := s.Visible()
		before, _ := rowBox(tt, &s, first)
		tt.Scroll(100, 100, 0, step)
		if r, _ := rowBox(tt, &s, first); r.Y != before.Y-step {
			t.Fatalf("step %d, %v DIPs: row %d moved from %v to %v", k, step, first, before.Y, r.Y)
		}
		checkRows(t, tt, &s, 0, 0, 400)
	}
	// Page Up keeps a line of the page before, among taller rows.
	for range 5 {
		first, _ := s.Visible()
		before, _ := rowBox(tt, &s, first)
		tt.Key(0, KeyPageUp)
		if r, _ := rowBox(tt, &s, first); r.Y != before.Y+360 {
			t.Fatalf("Page Up: row %d moved from %v to %v", first, before.Y, r.Y)
		}
	}
}

func TestListTrackScrollBeforeTheFirstFrame(t *testing.T) {
	sc := ScrollState{Y: 10000}
	tt := coreNewTester(func(c *context) {
		coreList(c, nil, 1000, func(i int) { coreTextf(c, "Row %d", i).Height(20) }).Grow(1).TrackScroll(&sc)
	}, 300, 400)
	if r, ok := tt.Find("Row 500"); !ok || r.Y != 0 || sc.Y != 10000 {
		t.Errorf("set to 10000 before the list showed: row 500 at %v (%v), the state %v", r.Y, ok, sc.Y)
	}
}

func TestListOfManyStates(t *testing.T) {
	// One list showing the place of each page.
	var pages [2]ListState
	page := 0
	tt := coreNewTester(func(c *context) {
		coreList(c, &pages[page], 1000, func(i int) { coreTextf(c, "Page %d row %d", page, i).Height(20) }).Grow(1)
	}, 300, 400)
	pages[0].ScrollTo(500, Start)
	tt.Frame()
	page = 1
	tt.Frame()
	if r, _ := tt.Find("Page 1 row 0"); r.Y != 0 {
		t.Errorf("page 1 shows from %v", r.Y)
	}
	page = 0
	tt.Frame()
	if r, ok := tt.Find("Page 0 row 500"); !ok || r.Y != 0 {
		t.Errorf("back on page 0: row 500 at %v (%v)", r.Y, ok)
	}
}

func TestListRevealsAFarRowWithoutAGap(t *testing.T) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1000, func(i int) {
			coreRow(c).Height(float32(40 + i%3*10)).Children(func() { coreButton(c, fmt.Sprintf("Button %d", i)) })
		}).Grow(1)
	}, 300, 400)
	if err := tt.Click("Button 3"); err != nil {
		t.Fatal(err)
	}
	s.ScrollTo(250, Start)
	tt.Frame()
	// The focus, kept out of view, comes into view in one frame.
	tt.rt.reveal(tt.rt.focused)
	tt.rt.runFrame()
	checkRows(t, tt, &s, 0, 0, 400)
	if f := tt.rt.states[tt.rt.focused]; f == nil || f.y < -0.01 || f.y+f.h > 400.01 {
		t.Errorf("the focus is at %+v", f)
	}
}

func TestListSizedByItsRows(t *testing.T) {
	// A list without a size is as high as its rows, frame after frame, and
	// scrolls once it is bounded.
	var s, bounded ListState
	tt := coreNewTester(func(c *context) {
		coreScroll(c).Grow(1).Children(func() {
			coreList(c, &s, 30, func(i int) { coreBox(c).Height(20) }).Shrink(0)
			coreList(c, &bounded, 1000, func(i int) { coreBox(c).Height(20) }).MaxHeight(200).Shrink(0)
		})
	}, 300, 400)
	for range 5 {
		tt.Frame()
		if h := listStateOf(&s).h; h != 600 {
			t.Fatalf("a list of 30 rows of 20 DIPs is %v high", h)
		}
		if h := listStateOf(&bounded).h; h != 200 {
			t.Fatalf("a list bounded to 200 DIPs is %v high", h)
		}
	}
}

func TestListEmpty(t *testing.T) {
	// What else is built in a list shows while it has no rows.
	n := 0
	tt := coreNewTester(func(c *context) {
		coreList(c, nil, n, func(i int) { coreTextf(c, "Row %d", i) }).Grow(1).Children(func() {
			if n == 0 {
				coreText(c, "No rows")
			}
		})
	}, 300, 400)
	if r, ok := tt.Find("No rows"); !ok || r.H == 0 || r.W == 0 {
		t.Errorf("an empty list shows %q, the text in %v", tt.Texts(), r)
	}
	n = 3
	tt.Frame()
	if !tt.HasText("Row 2") || tt.HasText("No rows") {
		t.Errorf("three rows: %q", tt.Texts())
	}
}

func TestListChoiceAtTheEnds(t *testing.T) {
	sel := 499
	ids := make([]int, 500)
	for i := range ids {
		ids[i] = i
	}
	s := ListState{Selected: &sel, Key: func(i int) any { return ids[i] }}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(ids), func(i int) { coreBox(c).Height(30) }).Grow(1)
	}, 300, 400)
	tt.rt.focused = s.frame.e.id
	s.ScrollTo(100, Start)
	tt.Frame()
	// Down at the last row shows it again.
	tt.Key(0, KeyDown)
	if r, ok := rowBox(tt, &s, 499); sel != 499 || !ok || r.Y+r.H != 400 {
		t.Errorf("Down at the last row: chose %d, at %v (%v)", sel, r, ok)
	}
	// A chosen item that goes away leaves no choice.
	ids = ids[:400]
	tt.Frame()
	if sel != -1 {
		t.Errorf("the chosen item went away: chose %d", sel)
	}
}

// BenchmarkListScroll scrolls a list of a million rows of varied heights,
// a frame a step.
func BenchmarkListScroll(b *testing.B) {
	var s ListState
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, 1_000_000, func(i int) {
			coreRow(c).Height(varied(i)).Gap(8).Children(func() {
				coreTextf(c, "Row %d", i).Grow(1)
				coreText(c, "detail").TextColor(c.Theme().TextMuted)
			})
		}).Grow(1)
	}, 980, 720)
	tt.Move(400, 300)
	b.ReportAllocs()
	for b.Loop() {
		tt.Scroll(400, 300, 0, 97)
	}
}
