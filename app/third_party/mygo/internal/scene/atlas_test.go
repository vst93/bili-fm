package scene

import (
	"image"
	"testing"
)

func TestAtlasZones(t *testing.T) {
	a := NewAtlas(1, 128, 128)
	lx, ly, ok := a.Alloc(10, 10)
	if !ok || lx != 0 || ly != 0 {
		t.Fatalf("lasting at %d,%d %v", lx, ly, ok)
	}
	tx, ty, ok := a.AllocTransient(10, 10)
	if !ok || tx != 0 || ty != 128-12 { // shelves round up to 4 rows
		t.Fatalf("transient at %d,%d %v", tx, ty, ok)
	}
	// The zones share the rows between them and never overlap.
	var lasting, transient []image.Rectangle
	for {
		x, y, ok := a.Alloc(20, 20)
		if !ok {
			break
		}
		lasting = append(lasting, image.Rect(x, y, x+21, y+21))
		x, y, ok = a.AllocTransient(20, 20)
		if !ok {
			break
		}
		transient = append(transient, image.Rect(x, y, x+21, y+21))
	}
	if len(lasting) == 0 || len(transient) == 0 {
		t.Fatalf("%d lasting, %d transient", len(lasting), len(transient))
	}
	for _, l := range lasting {
		for _, r := range transient {
			if l.Overlaps(r) {
				t.Fatalf("%v overlaps %v", l, r)
			}
		}
	}
	a.ResetTransient()
	if x, y, ok := a.AllocTransient(10, 10); !ok || x != tx || y != ty {
		t.Errorf("after ResetTransient at %d,%d %v, want %d,%d", x, y, ok, tx, ty)
	}
}

func TestAtlasPutClearsPadding(t *testing.T) {
	a := NewAtlas(1, 16, 16)
	x, y, _ := a.AllocTransient(3, 3)
	a.Put(x, y, 3, 3, []byte{9, 9, 9, 9, 9, 9, 9, 9, 9}, 3)
	a.ResetTransient()
	x, y, _ = a.AllocTransient(2, 2)
	a.Put(x, y, 2, 2, []byte{1, 1, 1, 1}, 2)
	for j := range 3 {
		for i := range 3 {
			want := byte(0)
			if i < 2 && j < 2 {
				want = 1
			}
			if got := a.Pix[(y+j)*a.W+x+i]; got != want {
				t.Errorf("pixel %d,%d is %d, want %d", i, j, got, want)
			}
		}
	}
	rects, full := a.Changes(a.Generation(), a.Version()-1)
	if full || len(rects) != 1 || rects[0] != image.Rect(x, y, x+3, y+3) {
		t.Errorf("changes %v %v", rects, full)
	}
}

func TestAtlasRepackAndGrow(t *testing.T) {
	a := NewAtlas(1, 32, 32)
	var rects []image.Rectangle
	for i := range 6 {
		w, h := 4+i, 6-i/2
		x, y, ok := a.Alloc(w, h)
		if !ok {
			t.Fatal("full")
		}
		pix := make([]byte, w*h)
		for k := range pix {
			pix[k] = byte(10*i + k%7)
		}
		a.Put(x, y, w, h, pix, w)
		rects = append(rects, image.Rect(x, y, x+w, y+h))
	}
	kept := []image.Rectangle{rects[1], rects[4], rects[5]}
	want := make([][]byte, len(kept))
	for i, r := range kept {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			want[i] = append(want[i], a.Pix[y*a.W+r.Min.X:y*a.W+r.Max.X]...)
		}
	}
	check := func(what string, pos []image.Point) {
		t.Helper()
		for i, r := range kept {
			var got []byte
			for y := pos[i].Y; y < pos[i].Y+r.Dy(); y++ {
				got = append(got, a.Pix[y*a.W+pos[i].X:y*a.W+pos[i].X+r.Dx()]...)
			}
			if string(got) != string(want[i]) {
				t.Errorf("%s: rectangle %d holds %v, want %v", what, i, got, want[i])
			}
		}
	}
	gen := a.Generation()
	pos, ok := a.Repack(kept)
	if !ok || a.Generation() == gen {
		t.Fatalf("Repack: %v, generation %d", ok, a.Generation())
	}
	check("repacked", pos)
	if _, ok := a.Repack([]image.Rectangle{image.Rect(0, 0, 40, 4)}); ok {
		t.Error("Repack placed a rectangle wider than the atlas")
	}
	check("after a failed repack", pos)
	if !a.Grow() || a.W != 64 || a.H != 64 {
		t.Fatalf("Grow: %dx%d", a.W, a.H)
	}
	check("grown", pos)
	if _, _, ok := a.Alloc(40, 40); !ok {
		t.Error("no room after growing")
	}
}
