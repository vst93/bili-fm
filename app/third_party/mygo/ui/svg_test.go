package ui

import (
	"image"
	"image/color"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

// A square in black, and a check mark in currentColor, as icon sets draw
// them.
const (
	squareSVG = `<svg viewBox="0 0 10 10"><rect width="10" height="10"/></svg>`
	checkSVG  = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`
	// A logo of two colors, and a third that follows the text.
	logoSVG = `<svg viewBox="0 0 30 10"><rect width="10" height="10" fill="#f00"/><rect x="10" width="10" height="10" fill="#0f0"/><rect x="20" width="10" height="10" fill="currentColor"/></svg>`
)

// colorIn returns the color at the center of r in img, at scale.
func colorIn(img *image.RGBA, r Rect, scale float32) color.RGBA {
	return img.RGBAAt(int((r.X+r.W/2)*scale), int((r.Y+r.H/2)*scale))
}

func TestIconTakesTheTextColorAndSize(t *testing.T) {
	square := MustParseSVG([]byte(squareSVG))
	blue := RGB(0, 0, 255)
	view := func(c *context) {
		coreColumn(c).Fill().Padding(10).Gap(10).Background(RGB(255, 255, 255)).Children(func() {
			coreIcon(c, square).Label("plain")
			coreRow(c).TextColor(blue).FontSize(24).Children(func() {
				coreIcon(c, square).Label("inherited")
			})
			coreIcon(c, square).Size(40, 20).TextColor(blue).Label("sized")
		})
	}
	tt := coreNewTester(view, 200, 200)
	theme := tt.rt.c.theme
	plain, _ := tt.Find("plain")
	if plain.W != theme.FontSize || plain.H != theme.FontSize {
		t.Errorf("an icon is %vx%v, not the font size, %v", plain.W, plain.H, theme.FontSize)
	}
	if plain.X != 10 {
		t.Errorf("an icon stretches across its column: at %v", plain.X)
	}
	inherited, _ := tt.Find("inherited")
	if inherited.W != 24 || inherited.H != 24 {
		t.Errorf("an icon is %vx%v, not its row's font size", inherited.W, inherited.H)
	}
	img := tt.Image()
	if c := colorIn(img, plain, 1); c != (color.RGBA{theme.Text.R, theme.Text.G, theme.Text.B, 255}) {
		t.Errorf("an icon is %v, not the theme's text color, %v", c, theme.Text)
	}
	if c := colorIn(img, inherited, 1); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("an icon is %v, not its row's text color", c)
	}
	// A size of another aspect ratio fits the icon in, centered.
	sized, _ := tt.Find("sized")
	if sized.W != 40 || sized.H != 20 {
		t.Fatalf("a sized icon is %vx%v", sized.W, sized.H)
	}
	if c := img.RGBAAt(int(sized.X+2), int(sized.Y+10)); c != (color.RGBA{255, 255, 255, 255}) {
		t.Errorf("the icon does not keep its aspect ratio: %v beside it", c)
	}
	if c := img.RGBAAt(int(sized.X+20), int(sized.Y+10)); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("the icon is not centered: %v", c)
	}
}

func TestIconDrawsItsShapeOnce(t *testing.T) {
	check := MustParseSVG([]byte(checkSVG))
	hot := false
	view := func(c *context) {
		col := RGB(0, 0, 0)
		if hot {
			col = RGB(255, 0, 0)
		}
		coreIcon(c, check).FontSize(32).TextColor(col).Label("check")
	}
	tt := coreNewTester(view, 100, 100)
	tt.SetScale(2)
	draws := 0
	j := &tt.rt.svgs.job
	j.draw = func() (int, int, []byte) { draws++; return tt.rt.rasterizeIcon() }
	for i := range 6 {
		hot = i%2 == 1
		tt.Frame()
		r, _ := tt.Find("check")
		// The corner of the check mark, at (9, 17) of 24.
		c := tt.Image().RGBAAt(int((r.X+r.W*9/24)*2), int((r.Y+r.H*17/24)*2))
		if want := uint8(0); hot {
			want = 255
			if c.R != want || c.G != 0 {
				t.Errorf("frame %d: the check mark is %v, not red", i, c)
			}
		} else if c.R != 0 || c.A != 255 {
			t.Errorf("frame %d: the check mark is %v, not black", i, c)
		}
	}
	// The mask is drawn for the first frame, and once more as the frame
	// after it finds it drawn recently and keeps it: changes of color
	// leave it be.
	if draws > 2 {
		t.Errorf("the icon's shape was drawn %d times for changes of color", draws)
	}
	r, _ := tt.Find("check")
	if r.W != 32 || r.H != 32 {
		t.Errorf("the icon is %vx%v DIPs", r.W, r.H)
	}
}

func TestImageOfAnSVG(t *testing.T) {
	logo := MustParseSVG([]byte(logoSVG))
	if w, h := logo.Size(); w != 30 || h != 10 {
		t.Fatalf("Size %vx%v", w, h)
	}
	view := func(c *context) {
		coreColumn(c).Fill().Background(RGB(255, 255, 255)).TextColor(RGB(0, 0, 255)).AlignItems(Start).Children(func() {
			coreImage(c, logo).Label("logo")
			coreImage(c, logo).Size(60, 60).Fit(Cover).Label("cover")
			coreImage(c, logo).Size(60, 10).Fit(FillBox).Label("stretched")
		})
	}
	tt := coreNewTester(view, 200, 200)
	img := tt.Image()
	r, _ := tt.Find("logo")
	if r.W != 30 || r.H != 10 {
		t.Errorf("the logo is %vx%v, not its size", r.W, r.H)
	}
	for i, want := range []color.RGBA{{255, 0, 0, 255}, {0, 255, 0, 255}, {0, 0, 255, 255}} {
		if c := img.RGBAAt(int(r.X)+i*10+5, int(r.Y)+5); c != want {
			t.Errorf("part %d of the logo is %v, want %v", i, c, want)
		}
	}
	// Cover fills the box with the middle of the logo.
	r, _ = tt.Find("cover")
	if c := img.RGBAAt(int(r.X)+2, int(r.Y)+30); c != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("the covering logo shows %v at its left", c)
	}
	r, _ = tt.Find("stretched")
	if c := img.RGBAAt(int(r.X)+55, int(r.Y)+5); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("the stretched logo shows %v at its right", c)
	}
}

func TestPictureFollowsTheTextColor(t *testing.T) {
	logo := MustParseSVG([]byte(logoSVG))
	col := RGB(0, 0, 255)
	view := func(c *context) {
		coreImage(c, logo).TextColor(col).Label("logo")
	}
	tt := coreNewTester(view, 100, 100)
	first := tt.rt.svgs.pictures
	if len(first) != 1 {
		t.Fatalf("%d pictures", len(first))
	}
	var img any
	for _, p := range first {
		img = p.img
	}
	col = RGB(255, 0, 255)
	tt.Frame()
	r, _ := tt.Find("logo")
	if c := tt.Image().RGBAAt(int(r.X+r.W*25/30), int(r.Y+r.H/2)); c != (color.RGBA{255, 0, 255, 255}) {
		t.Errorf("currentColor is %v after the text color changed", c)
	}
	pics := tt.rt.svgs.pictures
	if len(pics) != 1 {
		t.Fatalf("%d pictures after a change of color", len(pics))
	}
	for _, p := range pics {
		if any(p.img) != img {
			t.Error("a change of color made another picture, and another texture")
		}
	}
	// Pictures no frame draws go.
	col = RGB(1, 2, 3)
	for range 40 {
		tt.Frame()
	}
	if n := len(tt.rt.svgs.pictures); n != 1 {
		t.Errorf("%d pictures kept", n)
	}
}

func TestIconsInAccessibility(t *testing.T) {
	check := MustParseSVG([]byte(checkSVG))
	tt := coreNewTester(func(c *context) {
		coreRow(c).Children(func() {
			coreIcon(c, check)
			coreIcon(c, check).Label("Done")
		})
	}, 100, 50)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	var images []string
	for _, n := range tt.h.access.Nodes {
		if n.Role == platform.RoleImage {
			images = append(images, n.Label)
		}
	}
	if len(images) != 1 || images[0] != "Done" {
		t.Errorf("images seen: %q", images)
	}
}

func TestPainterDrawsSVGs(t *testing.T) {
	check, logo := MustParseSVG([]byte(checkSVG)), MustParseSVG([]byte(logoSVG))
	img := coreRender(func(c *context) {
		coreBox(c).Fill().Background(RGB(255, 255, 255)).Draw(func(p *Painter, r Rect) {
			p.Icon(check, Rect{0, 0, 48, 48}, RGB(0, 128, 0))
			p.Image(logo, Rect{0, 60, 30, 10}, Contain)
		})
	}, 100, 100, 1)
	if c := img.RGBAAt(18, 34); c.G < 100 || c.R > 50 {
		t.Errorf("the icon is %v", c)
	}
	if c := img.RGBAAt(5, 65); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("the image is %v", c)
	}
}

func TestParseSVGErrors(t *testing.T) {
	if _, err := ParseSVG([]byte("<html/>")); err == nil {
		t.Error("ParseSVG takes a document that is not SVG")
	}
	defer func() {
		if recover() == nil {
			t.Error("MustParseSVG takes a document that is not SVG")
		}
	}()
	MustParseSVG([]byte("nope"))
}
