package ui

import (
	"fmt"
	"slices"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// drag presses at from, moves in steps to to, and releases there.
func drag(tt *Tester, fromX, fromY, toX, toY float32) {
	tt.Press(fromX, fromY)
	for k := 1; k <= 4; k++ {
		f := float32(k) / 4
		tt.Move(fromX+(toX-fromX)*f, fromY+(toY-fromY)*f)
	}
	tt.Release(toX, toY)
}

func TestDragAndDrop(t *testing.T) {
	var dropped []string
	var over bool
	clicks, numbers := 0, 0
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(20).Padding(20).Children(func() {
			src := coreBox(c).Size(100, 40).Label("Source").Drag("hello")
			if src.Clicked() {
				clicks++
			}
			target := coreBox(c).Size(200, 100).Label("Target")
			if v, ok := coreDrop[string](target); ok {
				dropped = append(dropped, v)
			}
			_, over = coreDragOver[string](target)
			// Inside the target, an element taking numbers, not text.
			inner := coreBox(c).Size(50, 30).Label("Numbers")
			if _, ok := coreDrop[int](inner); ok {
				numbers++
			}
		})
	}, 400, 400)
	src, _ := tt.Find("Source")
	target, _ := tt.Find("Target")
	inner, _ := tt.Find("Numbers")
	// A press that hardly moves is a click.
	drag(tt, src.X+10, src.Y+10, src.X+12, src.Y+11)
	if clicks != 1 || len(dropped) != 0 {
		t.Fatalf("a press moving 2 DIPs: %d clicks, dropped %q", clicks, dropped)
	}
	// Dragged over the target, then dropped on it: no click.
	tt.Press(src.X+10, src.Y+10)
	tt.Move(src.X+30, src.Y+30)
	tt.Move(target.X+20, target.Y+20)
	if !over {
		t.Fatal("the target does not see the value over it")
	}
	tt.Release(target.X+20, target.Y+20)
	if !slices.Equal(dropped, []string{"hello"}) || clicks != 1 || over {
		t.Fatalf("dropped %q, %d clicks, over %v", dropped, clicks, over)
	}
	// The element taking numbers does not take text.
	drag(tt, src.X+10, src.Y+10, inner.X+10, inner.Y+10)
	if numbers != 0 {
		t.Errorf("text dropped on an element taking numbers")
	}
	// Escape gives up the drag.
	tt.Press(src.X+10, src.Y+10)
	tt.Move(target.X+20, target.Y+20)
	tt.Image() // the copy following the pointer paints
	tt.Key(0, KeyEscape)
	tt.Release(target.X+20, target.Y+20)
	if len(dropped) != 1 || clicks != 1 {
		t.Errorf("after Escape: dropped %q, %d clicks", dropped, clicks)
	}
}

func TestListReorder(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e", "f"}
	var moves [][]int
	sel := -1
	var chosen Selection[string]
	s := ListState{
		Selected:  &sel,
		Selection: &chosen,
		Key:       func(i int) any { return items[i] },
		Reorder: func(rows []int, to int) {
			moves = append(moves, append(slices.Clone(rows), to))
			moved := make([]string, 0, len(rows))
			for _, r := range rows {
				moved = append(moved, items[r])
			}
			var rest []string
			at := 0
			for i, it := range items {
				if i == to {
					at = len(rest)
				}
				if !slices.Contains(rows, i) {
					rest = append(rest, it)
				}
			}
			if to == len(items) {
				at = len(rest)
			}
			items = slices.Insert(rest, at, moved...)
		},
	}
	tt := coreNewTester(func(c *context) {
		coreList(c, &s, len(items), func(i int) {
			coreText(c, items[i]).Padding(8)
		}).Grow(1)
	}, 300, 400)
	b, _ := tt.Find("b")
	d, _ := tt.Find("d")
	// b below d's middle: before e.
	drag(tt, b.X, b.Y+b.H/2, d.X, d.Y+d.H)
	if len(moves) != 1 || !slices.Equal(moves[0], []int{1, 4}) || !slices.Equal(items, []string{"a", "c", "d", "b", "e", "f"}) {
		t.Fatalf("moves %v, items %q", moves, items)
	}
	// The rows chosen move together.
	tt.Click("c")
	tt.ClickWith(Cmd, "e")
	e, _ := tt.Find("e")
	a, _ := tt.Find("a")
	drag(tt, e.X, e.Y+e.H/2, a.X, a.Y+2)
	if len(moves) != 2 || !slices.Equal(moves[1], []int{1, 4, 0}) || !slices.Equal(items, []string{"c", "e", "a", "d", "b", "f"}) {
		t.Errorf("moves %v, items %q", moves, items)
	}
	// Dropped where they are, nothing moves.
	f, _ := tt.Find("f")
	drag(tt, f.X, f.Y+f.H/2, f.X, f.Y+f.H/2+6)
	if len(moves) != 2 {
		t.Errorf("a row dropped on itself moved: %v", moves)
	}
	// They go where they dropped, though the pointer moved on before the
	// next frame.
	f, _ = tt.Find("f")
	c, _ := tt.Find("c")
	tt.Press(f.X, f.Y+f.H/2)
	tt.Move(f.X, f.Y)
	tt.Move(c.X, c.Y+2)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerUp, X: float64(c.X), Y: float64(c.Y + 2)})
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerMove, X: float64(f.X), Y: float64(f.Y + f.H)})
	tt.settle()
	if len(moves) != 3 || !slices.Equal(moves[2], []int{5, 0}) {
		t.Errorf("moves %v, items %q", moves, items)
	}
}

func TestGridReorder(t *testing.T) {
	var moves [][]int
	sel := -1
	s := GridState{Selected: &sel, Reorder: func(items []int, to int) {
		moves = append(moves, append(slices.Clone(items), to))
	}}
	tt := coreNewTester(func(c *context) {
		coreGridView(c, &s, 12, 100, 60, func(i int) {
			coreTextf(c, "Item %d", i)
		}).Grow(1)
	}, 448, 400)
	from, _ := tt.Find("Item 0")
	to, _ := tt.Find("Item 2")
	// Item 0 on the right half of item 2: before item 3.
	drag(tt, from.X, from.Y, to.X+to.W+20, to.Y)
	if len(moves) != 1 || !slices.Equal(moves[0], []int{0, 3}) {
		t.Fatalf("moves %v", moves)
	}
	// On the left half of an item: before it.
	five, _ := tt.Find("Item 5")
	drag(tt, to.X, to.Y, five.X-20, five.Y)
	if len(moves) != 2 || !slices.Equal(moves[1], []int{2, 5}) {
		t.Errorf("moves %v", fmt.Sprint(moves))
	}
	// Where they dropped, though the pointer moved on before the next
	// frame.
	tt.Press(from.X, from.Y)
	tt.Move(from.X+20, from.Y)
	tt.Move(to.X+to.W+20, to.Y)
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerUp, X: float64(to.X + to.W + 20), Y: float64(to.Y)})
	tt.rt.event(platform.SurfaceEvent{Kind: platform.PointerMove, X: float64(five.X), Y: float64(five.Y + 80)})
	tt.settle()
	if len(moves) != 3 || !slices.Equal(moves[2], []int{0, 3}) {
		t.Errorf("moves %v", fmt.Sprint(moves))
	}
}
