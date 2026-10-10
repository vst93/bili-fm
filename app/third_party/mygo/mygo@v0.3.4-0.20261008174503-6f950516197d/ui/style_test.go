package ui

import (
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
	"time"
)

// boxes lays out view in a window w×h and returns the boxes of the
// labeled elements.
func boxes(t *testing.T, view func(c *context), w, h int, labels ...string) map[string]Rect {
	t.Helper()
	tt := coreNewTester(view, w, h)
	out := map[string]Rect{}
	for _, l := range labels {
		r, ok := tt.Find(l)
		if !ok {
			t.Fatalf("no element %q", l)
		}
		out[l] = r
	}
	return out
}

func near(a, b float32) bool { return math.Abs(float64(a-b)) < 0.51 }

func nearRect(a, b Rect) bool {
	return near(a.X, b.X) && near(a.Y, b.Y) && near(a.W, b.W) && near(a.H, b.H)
}

func TestGrid(t *testing.T) {
	cells := func(n int) []string {
		var out []string
		for i := range n {
			out = append(out, string(rune('a'+i)))
		}
		return out
	}
	t.Run("equal columns and spans", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreGrid(c).Columns(3).Gap(10).Padding(5).Width(320).Children(func() {
				coreBox(c).Height(20).Label("a")
				coreBox(c).Height(40).Label("b").ColumnSpan(2)
				coreBox(c).Height(10).Label("c").ColumnSpan(-1)
				coreBox(c).Height(10).Label("d").ColumnStart(3).RowStart(3)
				coreBox(c).Height(10).Label("e")
			})
		}, 400, 300, cells(5)...)
		// 310 DIPs of content and two gaps: columns of 96.67.
		col := (310 - 20) / float32(3)
		want := map[string]Rect{
			"a": {5, 5, col, 20},
			"b": {5 + col + 10, 5, 2*col + 10, 40},
			"c": {5, 55, 310, 10},
			"d": {5 + 2*col + 20, 75, col, 10},
			"e": {5, 75, col, 10},
		}
		for k, w := range want {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("tracks", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreGrid(c).ColumnTracks(Fixed(100), FitContent(), Fr(1), Fr(2)).GapX(10).Width(500).Children(func() {
				coreBox(c).Height(10).Label("a")
				coreBox(c).Size(60, 10).Label("b")
				coreBox(c).Height(10).Label("c")
				coreBox(c).Height(10).Label("d")
			})
		}, 600, 100, cells(4)...)
		// 500 - 100 - 60 - 30 of gaps leaves 310 for 3fr.
		for k, w := range map[string]Rect{"a": {0, 0, 100, 10}, "b": {110, 0, 60, 10}, "c": {180, 0, 310.0 / 3, 10}, "d": {190 + 310.0/3, 0, 620.0 / 3, 10}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("alignment in cells", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreGrid(c).Columns(2).RowTracks(Fixed(50)).Width(200).JustifyItems(Center).AlignItems(End).Children(func() {
				coreBox(c).Size(20, 10).Label("a")
				coreBox(c).Size(20, 10).Label("b").JustifySelf(Start).AlignSelf(Start)
				coreBox(c).Size(20, 10).Label("c").Margin(0, Auto)
			})
		}, 300, 200, cells(3)...)
		for k, w := range map[string]Rect{"a": {40, 40, 20, 10}, "b": {100, 0, 20, 10}, "c": {40, 50, 20, 10}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("fit content and stretch", func(t *testing.T) {
		var w float32
		b := boxes(t, func(c *context) {
			coreRow(c).Children(func() {
				g := coreGrid(c).ColumnTracks(FitContent(), FitContent()).Label("grid").Children(func() {
					coreBox(c).Size(30, 10).Label("a")
					coreBox(c).Size(50, 10).Label("b")
				})
				w = g.Bounds().W
			})
			coreGrid(c).ColumnTracks(FitContent(), FitContent()).Width(200).Children(func() {
				coreBox(c).Height(10).MinWidth(30).Label("c")
				coreBox(c).Height(10).MinWidth(50).Label("d")
			})
		}, 300, 200, "grid", "a", "b", "c", "d")
		if b["grid"].W != 80 || b["b"].X != 30 {
			t.Errorf("a grid of tracks fitting their content is %v wide, with b at %v", b["grid"], b["b"])
		}
		// The room left stretches the tracks.
		if b["c"].W != 90 || b["d"].W != 110 {
			t.Errorf("tracks did not stretch: %v %v", b["c"], b["d"])
		}
		_ = w
	})
	t.Run("rows grow with content", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreColumn(c).Children(func() {
				coreGrid(c).Columns(2).GapY(4).Label("grid").Children(func() {
					coreText(c, "one")
					coreText(c, strings.Repeat("word ", 30))
					coreBox(c).Height(25).Label("x")
				})
			})
		}, 200, 400, "grid", "x", "one")
		if b["x"].Y <= b["one"].Y+b["one"].H || b["grid"].H != b["x"].Y+25-b["grid"].Y {
			t.Errorf("rows did not take the wrapped text's height: grid %v, x %v", b["grid"], b["x"])
		}
	})
}

func TestFlexOptions(t *testing.T) {
	t.Run("gaps and align content", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreRow(c).Wrap().Size(100, 100).GapX(10).GapY(6).AlignContent(Center).AlignItems(Start).Children(func() {
				coreBox(c).Size(40, 10).Label("a")
				coreBox(c).Size(40, 10).Label("b")
				coreBox(c).Size(40, 10).Label("c")
			})
		}, 200, 200, "a", "b", "c")
		// Two lines of 10 and a gap of 6: 74 left, 37 above.
		for k, w := range map[string]Rect{"a": {0, 37, 40, 10}, "b": {50, 37, 40, 10}, "c": {0, 53, 40, 10}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("stretched lines", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreRow(c).Wrap().Size(100, 100).AlignContent(Stretch).AlignItems(Stretch).Children(func() {
				coreBox(c).Width(60).Label("a")
				coreBox(c).Width(60).Label("b")
			})
		}, 200, 200, "a", "b")
		if b["a"].H != 50 || b["b"].Y != 50 {
			t.Errorf("lines did not stretch: %v %v", b["a"], b["b"])
		}
	})
	t.Run("reverse", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreRow(c).Reverse().Width(100).Gap(10).Children(func() {
				coreBox(c).Size(20, 10).Label("a").Margin(0, 5, 0, 0)
				coreBox(c).Size(20, 10).Label("b")
			})
			coreColumn(c).Reverse().Height(100).Children(func() {
				coreBox(c).Size(20, 10).Label("c")
				coreBox(c).Size(20, 10).Label("d")
			})
			coreRow(c).WrapReverse().Size(50, 40).AlignItems(Start).Children(func() {
				coreBox(c).Size(30, 10).Label("e")
				coreBox(c).Size(30, 10).Label("f")
			})
		}, 200, 300, "a", "b", "c", "d", "e", "f")
		want := map[string]Rect{
			"a": {75, 0, 20, 10}, "b": {45, 0, 20, 10}, // from the right, a's right margin first
			"c": {0, 100, 20, 10}, "d": {0, 90, 20, 10},
			"e": {0, 140, 30, 10}, "f": {0, 130, 30, 10}, // lines from the bottom
		}
		for k, w := range want {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("auto margins", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreColumn(c).Width(200).Children(func() {
				coreBox(c).Size(50, 10).Margin(0, Auto).Label("centered")
				coreRow(c).Height(40).Children(func() {
					coreBox(c).Size(20, 10).Label("left")
					coreBox(c).Size(20, 10).Margin(Auto, 0, Auto, Auto).Label("right")
				})
			})
			coreBox(c).Absolute().Left(0).Right(0).Top(100).Size(40, 40).Margin(0, Auto).Label("abs")
		}, 200, 200, "centered", "left", "right", "abs")
		for k, w := range map[string]Rect{"centered": {75, 0, 50, 10}, "left": {0, 25, 20, 10}, "right": {180, 25, 20, 10}, "abs": {80, 100, 40, 40}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("relative offsets and percentages", func(t *testing.T) {
		b := boxes(t, func(c *context) {
			coreRow(c).Width(200).Children(func() {
				coreBox(c).Size(20, 20).Top(5).Left(3).Label("moved")
				coreBox(c).Size(20, 20).Label("next")
				coreBox(c).BasisPercent(25).Height(20).Label("quarter")
				coreBox(c).Grow(1).MaxWidthPercent(10).Height(20).Label("tenth")
			})
			coreBox(c).Absolute().LeftPercent(50).TopPercent(50).Size(10, 10).Label("half")
		}, 200, 200, "moved", "next", "quarter", "tenth", "half")
		for k, w := range map[string]Rect{"moved": {3, 5, 20, 20}, "next": {20, 0, 20, 20}, "quarter": {40, 0, 50, 20}, "tenth": {90, 0, 20, 20}, "half": {100, 100, 10, 10}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
	t.Run("absolute in the padding box", func(t *testing.T) {
		// As in CSS, insets are measured from inside the border, whatever
		// the padding, and percentages from the padding box's size.
		b := boxes(t, func(c *context) {
			coreBox(c).Size(36, 20).Border(1, RGB(0, 0, 0)).Padding(5).Children(func() {
				coreBox(c).Absolute().Left(3).Top(3).Size(14, 14).Label("start")
				coreBox(c).Absolute().Right(3).Bottom(3).Size(4, 4).Label("end")
				coreBox(c).Absolute().LeftPercent(50).TopPercent(50).Size(2, 2).Label("half")
			})
		}, 200, 200, "start", "end", "half")
		for k, w := range map[string]Rect{"start": {4, 4, 14, 14}, "end": {28, 12, 4, 4}, "half": {18, 10, 2, 2}} {
			if !nearRect(b[k], w) {
				t.Errorf("%s at %v, want %v", k, b[k], w)
			}
		}
	})
}

func TestBorderWidths(t *testing.T) {
	red := RGB(220, 0, 0)
	var inner Rect
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(100, 60).Background(RGB(255, 255, 255)).BorderWidth(2, 0, 6, 10).BorderColor(red).Padding(4).Children(func() {
			coreBox(c).Fill().Label("inner")
		})
		coreBox(c).Size(100, 60).Margin(10, 0, 0, 0).Border(2, red).BorderStyle(BorderDashed)
	}, 120, 200)
	inner, _ = tt.Find("inner")
	if !nearRect(inner, Rect{14, 6, 82, 44}) {
		t.Errorf("the content box is %v inside a border of 2, 0, 6, 10", inner)
	}
	img := tt.Image()
	isRed := func(x, y int) bool { c := img.RGBAAt(x, y); return c.R > 200 && c.G < 40 }
	for _, p := range []struct {
		x, y int
		red  bool
	}{{5, 30, true}, {50, 0, true}, {50, 56, true}, {98, 30, false}, {50, 30, false}} {
		if isRed(p.x, p.y) != p.red {
			t.Errorf("pixel (%d, %d) red: %v", p.x, p.y, !p.red)
		}
	}
	// The dashed border alternates along its top.
	changes, last := 0, isRed(0, 71)
	for x := 1; x < 100; x++ {
		if r := isRed(x, 71); r != last {
			changes, last = changes+1, r
		}
	}
	if changes < 10 {
		t.Errorf("a dashed border changes %d times along its top", changes)
	}
	if !isRed(1, 71) || !isRed(98, 71) {
		t.Error("a dashed border does not start and end with a dash")
	}
}

func TestPaints(t *testing.T) {
	blue, yellow := RGB(0, 0, 255), RGB(255, 255, 0)
	tt := coreNewTester(func(c *context) {
		coreBox(c).Size(100, 20).Gradient(blue, yellow, 90)
		coreBox(c).Size(100, 20).LinearGradient(LinearGradient{From: blue, To: yellow, Angle: 90, Oklab: true})
		coreBox(c).Size(100, 20).LinearGradient(LinearGradient{From: blue, To: yellow, Angle: 90, Start: 0.5, End: 0.6})
		coreBox(c).Size(100, 20).Background(RGB(255, 255, 255)).Stripes(RGB(0, 0, 0), 4, 4, 0)
		coreBox(c).Size(40, 20).Draw(func(p *Painter, r Rect) {
			var path Path
			path.MoveTo(r.X, r.Y).LineTo(r.X+r.W, r.Y).LineTo(r.X+r.W, r.Y+r.H).LineTo(r.X, r.Y+r.H).Close()
			p.FillPathGradient(&path, LinearGradient{From: blue, To: yellow, Angle: 90})
		})
	}, 100, 100)
	img := tt.Image()
	srgb, oklab := img.RGBAAt(50, 10), img.RGBAAt(50, 30)
	// sRGB mixes blue and yellow into gray; Oklab keeps more light.
	if srgb.R < 110 || srgb.R > 145 || int(oklab.R)+int(oklab.G) < int(srgb.R)+int(srgb.G)+20 {
		t.Errorf("midpoints: sRGB %v, Oklab %v", srgb, oklab)
	}
	if c := img.RGBAAt(40, 50); c.B < 250 || c.R > 5 {
		t.Errorf("before the start of the stops: %v", c)
	}
	if c := img.RGBAAt(70, 50); c.R < 250 || c.B > 5 {
		t.Errorf("after the end of the stops: %v", c)
	}
	dark := 0
	for x := 0; x < 100; x++ {
		if img.RGBAAt(x, 70).R < 128 {
			dark++
		}
	}
	if dark < 40 || dark > 60 {
		t.Errorf("stripes of 4 every 8 cover %d of 100 pixels", dark)
	}
	if l, r := img.RGBAAt(2, 90), img.RGBAAt(37, 90); l.B < 200 || r.R < 200 {
		t.Errorf("a path filled with a gradient: %v to %v", l, r)
	}
}

func TestTextDecorations(t *testing.T) {
	red := RGB(255, 0, 0)
	isRed := func(c color.RGBA) bool { return c.R > 200 && c.G < 60 && c.B < 60 }
	reds := func(img *image.RGBA, r Rect) (n int, rows map[int]bool) {
		rows = map[int]bool{}
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				if isRed(img.RGBAAt(x, y)) {
					n++
					rows[y] = true
				}
			}
		}
		return n, rows
	}
	for _, tc := range []struct {
		name string
		view func(c *context)
		test func(n int, rows map[int]bool) bool
	}{
		{"underline color", func(c *context) {
			coreText(c, "Hello").Underline().DecorationColor(red).Label("t")
		}, func(n int, rows map[int]bool) bool { return n > 10 && len(rows) == 1 }},
		{"thick strikethrough", func(c *context) {
			coreText(c, "Hello").Strikethrough().DecorationColor(red).DecorationThickness(3).Label("t")
		}, func(n int, rows map[int]bool) bool { return len(rows) == 3 }},
		{"wavy underline", func(c *context) {
			coreText(c, "Hello world").WavyUnderline().DecorationColor(red).Label("t")
		}, func(n int, rows map[int]bool) bool { return len(rows) >= 2 }},
		{"background", func(c *context) {
			coreText(c, "Hello").TextBackground(red).Label("t")
		}, func(n int, rows map[int]bool) bool { return n > 100 }},
		{"spans", func(c *context) {
			coreRichText(c, Span{Text: "plain "}, Span{Text: "found", Background: red}, Span{Text: " typo", WavyUnderline: true, DecorationColor: red}).Label("t")
		}, func(n int, rows map[int]bool) bool { return n > 50 }},
	} {
		tt := coreNewTester(tc.view, 300, 60)
		r, _ := tt.Find("t")
		if n, rows := reds(tt.Image(), Rect{r.X, r.Y, r.W, r.H + 4}); !tc.test(n, rows) {
			t.Errorf("%s: %d red pixels on %d rows", tc.name, n, len(rows))
		}
	}
}

func TestTextOptions(t *testing.T) {
	b := boxes(t, func(c *context) {
		coreColumn(c).Width(60).AlignItems(Start).Children(func() {
			coreText(c, "one\ntwo").FixedLineHeight(30).Label("fixed")
			coreText(c, "a long line of words").NoWrap().Label("nowrap")
			coreText(c, "a long line of words").Label("wraps")
		})
	}, 300, 300, "fixed", "nowrap", "wraps")
	if b["fixed"].H != 60 {
		t.Errorf("two lines 30 high take %v", b["fixed"].H)
	}
	if b["nowrap"].W <= 60 || b["nowrap"].H >= b["wraps"].H {
		t.Errorf("text that does not wrap: %v, text that does: %v", b["nowrap"], b["wraps"])
	}
}

// Even when a button grows, its centered label takes its intrinsic width.
// That width must fit the whole label, including kerned runs such as "11".
func TestButtonLabelsAtIntrinsicWidth(t *testing.T) {
	for _, family := range []string{"sans-serif", "monospace"} {
		t.Run(family, func(t *testing.T) {
			var buttons []*node
			tt := coreNewTester(func(c *context) {
				buttons = buttons[:0]
				coreRow(c).Gap(2).Children(func() {
					for _, label := range []string{"1911", "2011", "2111", "2211", "2012", "2011-04-17", "11", "11:30"} {
						buttons = append(buttons, coreButton(c, label).Font(family).FontSize(12).Grow(1).Height(28))
					}
				})
			}, 1000, 100)
			check := func() {
				for _, button := range buttons {
					label := button.first
					l := label.tl
					if l == nil {
						t.Fatalf("button label %q has no layout", label.text)
					}
					if len(l.Lines) != 1 || l.Truncated || l.Lines[0].End != len(l.Runes) {
						t.Errorf("button label %q at width %g: %d lines, truncated %v", label.text, label.w, len(l.Lines), l.Truncated)
					}
				}
			}
			// Grow gives every button the same width. Leave enough room
			// for the widest label in the font this machine uses.
			var widest float32
			for _, button := range buttons {
				widest = max(widest, intrinsic(button, true))
			}
			width := int(math.Ceil(float64(widest)*float64(len(buttons))+2*float64(len(buttons)-1))) + 1
			tt.SetSize(width, 100)
			check()
			tt.SetSize(width+160, 100)
			check()
		})
	}
}

func TestInvisible(t *testing.T) {
	clicked := false
	b := boxes(t, func(c *context) {
		coreRow(c).Children(func() {
			if coreButtonBase(c).Size(40, 20).Invisible().Children(func() { coreText(c, "hidden") }).Clicked() {
				clicked = true
			}
			coreBox(c).Size(10, 10).Label("after")
		})
	}, 100, 100, "after")
	if b["after"].X != 40 {
		t.Errorf("an invisible element gave up its room: %v", b["after"])
	}
	tt := coreNewTester(func(c *context) {
		if coreButtonBase(c).Size(40, 20).Invisible().Background(RGB(255, 0, 0)).Children(func() { coreText(c, "hidden") }).Clicked() {
			clicked = true
		}
	}, 100, 100)
	if tt.HasText("hidden") || tt.Click("hidden") == nil {
		t.Error("invisible text can be found")
	}
	tt.ClickAt(10, 10)
	if clicked {
		t.Error("an invisible button was clicked")
	}
	if c := tt.Image().RGBAAt(10, 10); c.R == 255 && c.G == 0 {
		t.Error("an invisible element painted")
	}
}

func TestClipOneWay(t *testing.T) {
	img := coreRender(func(c *context) {
		coreBox(c).Size(50, 50).Margin(20).ClipX().Children(func() {
			coreBox(c).Size(100, 100).Margin(-10, 0, 0, -10).Background(RGB(255, 0, 0))
		})
	}, 150, 150, 1)
	red := func(x, y int) bool { c := img.RGBAAt(x, y); return c.R == 255 && c.G == 0 }
	if red(15, 40) || red(80, 40) {
		t.Error("ClipX let through what is left or right of the element")
	}
	if !red(40, 15) || !red(40, 80) {
		t.Error("ClipX cut what is above or below the element")
	}
}

func TestScrollBoth(t *testing.T) {
	var sc *node
	var content Rect
	tt := coreNewTester(func(c *context) {
		sc = coreScrollBoth(c).Size(100, 100).Children(func() {
			coreBox(c).Size(400, 300).Draw(func(p *Painter, r Rect) { content = r })
		})
	}, 200, 200)
	tt.Scroll(50, 50, 30, 40)
	if content.X != -30 || content.Y != -40 {
		t.Fatalf("scrolled to %v", content)
	}
	// Drag the horizontal thumb to the end.
	tt.Move(50, 50)
	st := sc.st
	g := scrollBars(Rect{st.x, st.y, st.w, st.h}, st.barInset, float32(st.contentW), float32(st.contentH), float32(st.scrollX), float32(st.scrollY), st.flags, 6)
	if !g.horizontal || !g.vertical {
		t.Fatalf("scroll bars %+v", g)
	}
	x, y := g.h.X+g.h.W/2, g.h.Y+g.h.H/2
	tt.Press(x, y)
	tt.Move(x+200, y)
	tt.Release(x+200, y)
	if content.X != -300 || content.Y != -40 {
		t.Errorf("dragging the horizontal thumb scrolled to %v", content)
	}
}

func TestScrollbarInsets(t *testing.T) {
	var sc *node
	var content Rect
	tt := coreNewTester(func(c *context) {
		sc = coreScroll(c).Size(100, 200).ScrollbarInsets(50, 4, 10).Children(func() {
			coreBox(c).Size(100, 800).Draw(func(p *Painter, r Rect) { content = r })
		})
	}, 200, 200)
	tt.Move(50, 100)
	st := sc.st
	g := scrollBars(Rect{st.x, st.y, st.w, st.h}, st.barInset, float32(st.contentW), float32(st.contentH), float32(st.scrollX), float32(st.scrollY), st.flags, 6)
	// The track runs from 50 below the top to 10 above the bottom, 4 in
	// from the right; the thumb shows a quarter of it, the content's
	// share in view.
	if g.vTrack.Y != 50 || g.vTrack.Y+g.vTrack.H != 190 || g.vTrack.X+g.vTrack.W != 96 {
		t.Fatalf("the track is %+v", g.vTrack)
	}
	if g.v.Y != 52 || g.v.H != (140-4)/4 {
		t.Errorf("the thumb is %+v", g.v)
	}
	// A press above the track, under the inset, does not page.
	tt.Press(93, 20)
	tt.Release(93, 20)
	if content.Y != 0 {
		t.Errorf("a press above the track scrolled to %v", content.Y)
	}
	// Dragging the thumb to the track's end scrolls to the content's.
	x, y := g.v.X+g.v.W/2, g.v.Y+g.v.H/2
	tt.Press(x, y)
	tt.Move(x, y+200)
	tt.Release(x, y+200)
	if content.Y != -600 {
		t.Errorf("dragging the thumb scrolled to %v", content.Y)
	}
}

func TestScrollContentShrinks(t *testing.T) {
	long := true
	var top *node
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Children(func() {
			if coreButton(c, "Switch").Clicked() {
				long = !long
			}
			coreScroll(c).Grow(1).Children(func() {
				top = coreBox(c).Height(20).Shrink(0)
				if long {
					coreBox(c).Height(1000).Shrink(0)
				}
			})
		})
	}, 200, 200)
	r, _ := tt.Find("Switch")
	start := top.y
	tt.Move(100, 150)
	tt.Scroll(100, 150, 0, 2000)
	if top.y >= 0 {
		t.Fatalf("scrolled to the end, the top is at %v", top.y)
	}
	// The content no longer reaches as far: the frame that shows it
	// shows it from the top, not from where the long content was.
	tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	if top.y != start {
		t.Errorf("the short content starts at %v, not %v", top.y, start)
	}
}

func TestEasingAndLoop(t *testing.T) {
	for name, e := range map[string]Easing{"linear": Linear, "in": EaseIn, "out": EaseOut, "in out": EaseInOut} {
		if e(0) != 0 || e(1) != 1 || e(0.5) <= 0 || e(0.5) >= 1 {
			t.Errorf("%s: %v %v %v", name, e(0), e(0.5), e(1))
		}
	}
	if EaseIn(0.5) >= 0.5 || EaseOut(0.5) <= 0.5 || EaseInOut(0.5) != 0.5 {
		t.Error("easings bend the wrong way")
	}
	if b := Bounce(Linear); b(0.25) != 0.5 || b(0.5) != 1 || b(0.75) != 0.5 {
		t.Errorf("bounce: %v %v %v", b(0.25), b(0.5), b(0.75))
	}
	var values []float32
	tt := coreNewTester(func(c *context) {
		e := coreBox(c)
		values = append(values, e.Loop("spin", 50*time.Millisecond, Linear))
	}, 50, 50)
	time.Sleep(20 * time.Millisecond)
	tt.Frame()
	moved := false
	for _, v := range values {
		if v < 0 || v >= 1 {
			t.Fatalf("Loop returned %v", v)
		}
		moved = moved || v != values[0]
	}
	if !moved {
		t.Error("Loop does not move")
	}
}

func TestIconRotates(t *testing.T) {
	bar := MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="0" y="10" width="24" height="4"/></svg>`))
	draw := func(deg float32) *image.RGBA {
		return coreRender(func(c *context) {
			coreIcon(c, bar).FontSize(40).TextColor(RGB(0, 0, 0)).Rotate(deg)
		}, 60, 60, 1)
	}
	dark := func(img *image.RGBA, x, y int) bool { return img.RGBAAt(x, y).R < 100 }
	flat, turned := draw(0), draw(90)
	if !dark(flat, 2, 20) || dark(flat, 20, 2) {
		t.Error("the icon is not a horizontal bar")
	}
	if dark(turned, 2, 20) || !dark(turned, 20, 2) {
		t.Error("the icon turned by 90 degrees is not a vertical bar")
	}
}

func TestImageFitsAndGray(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for i := 0; i < len(src.Pix); i += 4 {
		src.Pix[i], src.Pix[i+3] = 255, 255 // red
	}
	bm := NewBitmap(src)
	img := coreRender(func(c *context) {
		coreImage(c, bm).Size(50, 50).Fit(ScaleDown)
		coreImage(c, bm).Size(50, 50).Fit(Contain).Grayscale()
	}, 50, 100, 1)
	if c := img.RGBAAt(25, 25); c.R != 255 || c.G != 0 {
		t.Errorf("the center of a picture scaled down: %v", c)
	}
	if c := img.RGBAAt(10, 10); c.R == 255 && c.G == 0 {
		t.Error("ScaleDown enlarged a small picture")
	}
	if c := img.RGBAAt(25, 75); c.R != c.G || c.G != c.B || c.R == 255 {
		t.Errorf("a grayscale picture: %v", c)
	}
}

func TestMeasureAndDrawText(t *testing.T) {
	var mw, mh, dw, dh float32
	img := coreRender(func(c *context) {
		mw, mh = c.MeasureText(0, Span{Text: "Total", Weight: 700, Size: 20})
		coreBox(c).Fill().Draw(func(p *Painter, r Rect) {
			dw, dh = p.RichText(10, 10, 0, Span{Text: "Total", Weight: 700, Size: 20, Color: RGB(255, 0, 0)})
		})
	}, 200, 60, 1)
	if mw <= 0 || mw != dw || mh != dh || mh < 20 {
		t.Errorf("measured %v×%v, drew %v×%v", mw, mh, dw, dh)
	}
	n := 0
	for y := 0; y < 60; y++ {
		for x := 0; x < 200; x++ {
			if c := img.RGBAAt(x, y); c.R > 200 && c.G < 80 {
				n++
			}
		}
	}
	if n < 20 {
		t.Errorf("Painter.RichText drew %d red pixels", n)
	}
}

func TestDebugAndCursors(t *testing.T) {
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Fill().Debug().Children(func() {
			coreBox(c).Size(40, 40).Margin(10).Padding(5).Cursor(CursorNone).Label("box")
		})
	}, 100, 100)
	r, _ := tt.Find("box")
	tt.Move(r.X+20, r.Y+20)
	if tt.Cursor() != CursorNone {
		t.Errorf("cursor %v over an element hiding it", tt.Cursor())
	}
	img := tt.Image()
	if c := img.RGBAAt(int(r.X), int(r.Y+20)); c.B < 150 {
		t.Errorf("no outline on the box's edge: %v", c)
	}
}

func TestThemeUnits(t *testing.T) {
	th := LightTheme()
	th.Spacing, th.FontSize = 5, 16
	if th.Space(2) != 10 || th.Rem(1.5) != 24 {
		t.Errorf("Space(2) = %v, Rem(1.5) = %v", th.Space(2), th.Rem(1.5))
	}
}

func TestReviewedEdges(t *testing.T) {
	// Stops at one place make a hard edge.
	img := coreRender(func(c *context) {
		coreBox(c).Size(100, 10).LinearGradient(LinearGradient{From: RGB(255, 0, 0), To: RGB(0, 0, 255), Angle: 90, Start: 0.5, End: 0.5})
		// A 20×20 bitmap shows at 20×20 DIPs, as its Image lays out.
		src := image.NewRGBA(image.Rect(0, 0, 20, 20))
		for i := 0; i < len(src.Pix); i += 4 {
			src.Pix[i+1], src.Pix[i+3] = 255, 255
		}
		coreImage(c, NewBitmap(src)).Size(60, 60).Fit(NaturalSize)
	}, 100, 100, 2)
	if l, r := img.RGBAAt(90, 10), img.RGBAAt(110, 10); l.R != 255 || r.B != 255 {
		t.Errorf("a hard stop: %v and %v", l, r)
	}
	green := 0
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			if c := img.RGBAAt(x, y); c.G == 255 && c.R == 0 {
				green++
			}
		}
	}
	if green != 40*40 {
		t.Errorf("a 20×20 bitmap at its natural size covers %d device pixels at scale 2", green)
	}

	// An element that turns invisible gives up the focus.
	hide, clicks := false, 0
	tt := coreNewTester(func(c *context) {
		b := coreButtonBase(c).Size(40, 20).Label("b")
		if hide {
			b.Invisible()
		}
		if b.Clicked() {
			clicks++
		}
	}, 100, 100)
	tt.Key(0, KeyTab)
	if !tt.Focused("b") {
		t.Fatal("Tab did not focus the button")
	}
	hide = true
	tt.Frame()
	tt.Key(0, KeyEnter)
	if clicks != 0 {
		t.Error("Enter pressed an invisible button")
	}
}
