//go:build perfmeasure

// 无头渲染测量（受控场景 + 生产 surfaceFrame）。默认不编译（perfmeasure tag），
// 不参与 ./check.sh；手动跑：
//
//	MYGO_FRAME_STATS=all go test -tags perfmeasure -run TestPerfDrawerAndScroll -v ./internal/view/
//
// 每帧的 build/layout/paint/present 拆分由 mygo 的帧日志给出
// （ui/framestats.go，随 MYGO_FRAME_STATS 打开）；本文件负责把场景铺开
// （进抽屉、滚列表、换滚），并用 ui.Tester 的 headless + 真实 runFrame
// 驱动帧 —— 不启动任何窗口 / GUI。
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

func perfApp(t *testing.T, n int, scale float32, ambient bool) (*App, *ui.Tester, *httptest.Server) {
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
			Bvid:    fmt.Sprintf("BV%04d", i),
			Title:   fmt.Sprintf("标题%d", i),
			Up:      "UP主名字",
			Views:   "12.3万",
			Pubdate: "09-11",
			Cover:   fmt.Sprintf("%s/c%d.png", srv.URL, i),
		}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tst.SetScale(scale)
	return a, tst, srv
}

// waitCovers 驱动帧直到封面抓取收敛（Pending 归零）。
func waitCovers(t *testing.T, a *App, tst *ui.Tester) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tst.Frame()
		if a.Images.Pending() == 0 {
			tst.Frame()
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Log("covers did not settle")
}

type perfSum struct {
	n            int
	total, worst time.Duration
}

func (s *perfSum) add(d time.Duration) {
	s.n++
	s.total += d
	if d > s.worst {
		s.worst = d
	}
}

func (s perfSum) log(t *testing.T, label string) {
	if s.n == 0 {
		s.n = 1
	}
	t.Logf("%s: frames=%d avg=%.2fms worst=%.2fms",
		label, s.n,
		float64(s.total.Microseconds())/1000/float64(s.n),
		float64(s.worst.Microseconds())/1000)
}

// measure 跑 frames 帧，逐帧计总耗时。注意：场景不变时 Tester.Frame 走的是
// 「没有损伤、present 几乎为零」的空帧（实测 ~0.5ms），这里量的是空帧成本；
// 真正渲染那一帧的成本要用 Scroll（它自己 settle）或帧日志里的
// build/layout/paint/present 拆分来看。
func measure(tst *ui.Tester, frames int) perfSum {
	var s perfSum
	for i := 0; i < frames; i++ {
		start := time.Now()
		tst.Frame()
		s.add(time.Since(start))
	}
	return s
}

func TestPerfDrawerAndScroll(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		for _, ambient := range []bool{true, false} {
			a, tst, _ := perfApp(t, 300, scale, ambient)
			var entry perfSum
			for i := 0; i < 16; i++ {
				start := time.Now()
				tst.Frame()
				if i > 0 {
					entry.add(time.Since(start))
				}
			}
			waitCovers(t, a, tst)
			steady := measure(tst, 20)
			var scroll perfSum
			for i := 0; i < 30; i++ {
				// Scroll 自己会 settle（这一帧真的重绘），计时必须包住它；
				// 它后面那次 Frame 是场景没变的空帧，计它等于什么都没量。
				start := time.Now()
				tst.Scroll(400, 300, 0, 120)
				scroll.add(time.Since(start))
			}
			t.Logf("=== scale=%.0f ambient=%v", scale, ambient)
			entry.log(t, "entry")
			steady.log(t, "steady")
			scroll.log(t, "scroll")
		}
	}
}

func TestPerfAmbientOnly(t *testing.T) {
	for _, ambient := range []bool{true, false} {
		a, tst, _ := perfApp(t, 0, 1, ambient)
		a.Drawer = ""
		a.Track = &Track{Bvid: "BV1", Cover: a.Face, FirstFrame: a.Face}
		waitCovers(t, a, tst)
		steady := measure(tst, 20)
		t.Logf("=== home ambient=%v", ambient)
		steady.log(t, "home")
	}
}

func TestPerfNoCovers(t *testing.T) {
	for _, scale := range []float32{1, 2} {
		t.Run(fmt.Sprintf("scale%v", scale), func(t *testing.T) { perfNoCovers(t, scale) })
	}
}

// perfNoCovers：300 张卡但不给图床（封面永远抓不到），隔离「纯 UI」成本。
func perfNoCovers(t *testing.T, scale float32) {
	a := NewApp(func() {})
	cards := make([]Card, 300)
	for i := range cards {
		cards[i] = Card{Bvid: fmt.Sprintf("BV%04d", i), Title: fmt.Sprintf("标题%d", i), Up: "UP主", Views: "1万", Pubdate: "09-11"}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
	tst.SetScale(scale)
	for i := 0; i < 20; i++ {
		tst.Frame()
	}
	steady := measure(tst, 20)
	var scroll perfSum
	for i := 0; i < 30; i++ {
		start := time.Now()
		tst.Scroll(400, 300, 0, 120)
		scroll.add(time.Since(start))
	}
	t.Logf("=== no covers scale=%.0f", scale)
	steady.log(t, "steady")
	scroll.log(t, "scroll")
}
