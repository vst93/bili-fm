package ui

import (
	"image"
	"testing"
)

func TestRichText(t *testing.T) {
	red, blue := RGB(255, 0, 0), RGB(0, 0, 255)
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Padding(10).Gap(10).AlignItems(Start).Children(func() {
			coreRichText(c, Span{Text: "plain "}, Span{Text: "RED", Color: red, Size: 30, Weight: 700}).Label("rich")
			coreRichText(c, Span{Text: "under", Underline: true, Color: blue}, Span{Text: " over"}).Label("lined")
			coreText(c, "plain RED").Label("text")
		})
	}, 400, 200)
	rich, _ := tt.Find("rich")
	lined, _ := tt.Find("lined")
	plain, _ := tt.Find("text")
	if rich.H < plain.H*1.5 {
		t.Errorf("a span of size 30 leaves the text %v high, plain text %v", rich.H, plain.H)
	}
	img := tt.Image()
	count := func(r Rect, match func(r, g, b uint8) bool) (n, widest int) {
		box := image.Rect(int(r.X), int(r.Y), int(r.X+r.W), int(r.Y+r.H))
		for y := box.Min.Y; y < box.Max.Y; y++ {
			row := 0
			for x := box.Min.X; x < box.Max.X; x++ {
				if c := img.RGBAAt(x, y); match(c.R, c.G, c.B) {
					n++
					row++
				}
			}
			widest = max(widest, row)
		}
		return n, widest
	}
	isRed := func(r, g, b uint8) bool { return r > 200 && g < 80 && b < 80 }
	isBlue := func(r, g, b uint8) bool { return b > 200 && r < 80 && g < 80 }
	if n, _ := count(rich, isRed); n < 30 {
		t.Errorf("%d red pixels in the red span", n)
	}
	if n, _ := count(plain, isRed); n != 0 {
		t.Errorf("%d red pixels in plain text", n)
	}
	// The underline is a row of blue under the first word alone.
	_, widest := count(lined, isBlue)
	if float32(widest) < lined.W*0.4 || float32(widest) > lined.W*0.8 {
		t.Errorf("the widest row of blue is %d pixels of %v", widest, lined.W)
	}
}

// TestMeasureTextAgain checks that measuring spans measured before
// allocates nothing, and that other spans measure anew.
func TestMeasureTextAgain(t *testing.T) {
	tt := coreNewTester(func(c *context) {}, 100, 100)
	c := &tt.rt.c
	short, _ := c.MeasureText(0, Span{Text: "000"})
	long, _ := c.MeasureText(0, Span{Text: "000000"})
	bold, _ := c.MeasureText(0, Span{Text: "000", Weight: 800, Size: 30})
	if again, _ := c.MeasureText(0, Span{Text: "000"}); again != short || long <= short || bold <= short {
		t.Errorf("widths %v, %v, %v, then %v", short, long, bold, again)
	}
	if n := testing.AllocsPerRun(20, func() { c.MeasureText(0, Span{Text: "000"}, Span{Text: "1", Weight: 700}) }); n != 0 {
		t.Errorf("measuring the same spans allocates %v times", n)
	}
}
