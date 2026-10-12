//go:build perfmeasure

package view

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func perfPNG(w, h int) []byte {
	im := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := im.PixOffset(x, y)
			im.Pix[i], im.Pix[i+1], im.Pix[i+2], im.Pix[i+3] = uint8(x%256), uint8(y%256), 120, 255
		}
	}
	var b bytes.Buffer
	_ = png.Encode(&b, im)
	return b.Bytes()
}

func perfApp(t *testing.T, n int, scale float32, ambient bool) (*App, *ui.PerfTester, *httptest.Server) {
	t.Helper()
	data := perfPNG(256, 144)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Write(data)
	}))
	t.Cleanup(srv.Close)
	a := NewApp(func() {})
	a.Ambient = ambient
	a.Face = srv.URL + "/face.png"
	cards := make([]Card, n)
	for i := range cards {
		cards[i] = Card{
			Bvid:  fmt.Sprintf("BV%04d", i),
			Title: fmt.Sprintf("标题%d", i),
			Up:    "UP主名字",
			Views: "12.3万",
			Pubdate: "09-11",
			Cover: fmt.Sprintf("%s/c%d.png", srv.URL, i),
		}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"
	pt := ui.NewPerfTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	pt.SetScale(scale)
	return a, pt, srv
}

func waitCovers(t *testing.T, a *App, pt *ui.PerfTester) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		pt.Advance(16 * time.Millisecond)
		pt.Step()
		if a.Images.Pending() == 0 {
			// one more frame to show them
			pt.Advance(16 * time.Millisecond)
			pt.Step()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Log("covers did not settle")
}

type perfSum struct {
	n                                              int
	build, layout, paint, present, total time.Duration
	worst                                          time.Duration
}

func (s *perfSum) add(f ui.FramePerf) {
	s.n++
	s.build += f.Build
	s.layout += f.Layout
	s.paint += f.Paint
	s.present += f.Present
	s.total += f.Total
	if f.Total > s.worst {
		s.worst = f.Total
	}
}

func (s perfSum) log(t *testing.T, label string) {
	if s.n == 0 {
		s.n = 1
	}
	t.Logf("%s: frames=%d build=%.2fms layout=%.2fms paint=%.2fms present=%.2fms total=%.2fms worst=%.2fms",
		label, s.n,
		float64(s.build.Microseconds())/1000/float64(s.n),
		float64(s.layout.Microseconds())/1000/float64(s.n),
		float64(s.paint.Microseconds())/1000/float64(s.n),
		float64(s.present.Microseconds())/1000/float64(s.n),
		float64(s.total.Microseconds())/1000/float64(s.n),
		float64(s.worst.Microseconds())/1000)
}

func TestPerfDrawerAndScroll(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for _, ambient := range []bool{true, false} {
			a, pt, _ := perfApp(t, 300, scale, ambient)
			// entry: new drawer -> first frames animate.
			var entry perfSum
			for i := 0; i < 16; i++ {
				pt.Advance(16 * time.Millisecond)
				f := pt.Step()
				if i > 0 {
					entry.add(f)
				}
			}
			waitCovers(t, a, pt)
			// steady: no animation
			var steady perfSum
			for i := 0; i < 20; i++ {
				pt.Advance(16 * time.Millisecond)
				steady.add(pt.Step())
			}
			// scroll
			var scroll perfSum
			ui.ResetRasterOps()
			for i := 0; i < 30; i++ {
				pt.Scroll(400, 300, 0, 120)
				pt.Advance(16 * time.Millisecond)
				scroll.add(pt.Step())
			}
			ops := ui.RasterOpTimesNow()
			t.Logf("=== scale=%.0f ambient=%v", scale, ambient)
			entry.log(t, "entry")
			steady.log(t, "steady")
			scroll.log(t, "scroll")
			t.Logf("   scroll ops=%d fill=%.1f image=%.1f glyph=%.1f shadow=%.1f effect=%.1f hole=%.1f (ms total over %d frames)",
				ops.Ops, ops.Fill.Seconds()*1000, ops.Image.Seconds()*1000, ops.Glyph.Seconds()*1000,
				ops.Shadow.Seconds()*1000, ops.Effect.Seconds()*1000, ops.Hole.Seconds()*1000, scroll.n)
		}
	}
}

func TestPerfAmbientOnly(t *testing.T) {
	// home (no drawer): measure the ambient full-window image cost.
	for _, ambient := range []bool{true, false} {
		a, pt, _ := perfApp(t, 0, 1, ambient)
		a.Drawer = ""
		a.Track = &Track{Bvid: "BV1", Cover: a.Face, FirstFrame: a.Face}
		waitCovers(t, a, pt)
		var steady perfSum
		for i := 0; i < 20; i++ {
			pt.Advance(16 * time.Millisecond)
			steady.add(pt.Step())
		}
		t.Logf("=== home ambient=%v", ambient)
		steady.log(t, "home")
	}
}

func TestPerfNoCovers(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("scale%v", scale), func(t *testing.T) { perfNoCovers(t, scale) })
	}
}

func perfNoCovers(t *testing.T, scale float32) {
	// drawer with 300 cards but images never load (no server): isolates image cost.
	a := NewApp(func() {})
	cards := make([]Card, 300)
	for i := range cards {
		cards[i] = Card{Bvid: fmt.Sprintf("BV%04d", i), Title: fmt.Sprintf("标题%d", i), Up: "UP主", Views: "1万", Pubdate: "09-11"}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"
	pt := ui.NewPerfTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	pt.SetScale(scale)
	for i := 0; i < 20; i++ {
		pt.Advance(16 * time.Millisecond)
		pt.Step()
	}
	var steady perfSum
	for i := 0; i < 20; i++ {
		pt.Advance(16 * time.Millisecond)
		steady.add(pt.Step())
	}
	var scroll perfSum
	for i := 0; i < 30; i++ {
		pt.Scroll(400, 300, 0, 120)
		pt.Advance(16 * time.Millisecond)
		scroll.add(pt.Step())
	}
	t.Logf("=== no covers scale=%.0f", scale)
	steady.log(t, "steady")
	scroll.log(t, "scroll")
}
