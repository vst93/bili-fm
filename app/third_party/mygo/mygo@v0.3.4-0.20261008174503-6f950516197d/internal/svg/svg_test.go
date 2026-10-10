package svg

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"testing"
)

var red = color.NRGBA{255, 0, 0, 255}

func draw(t *testing.T, src string, w, h int) *image.RGBA {
	t.Helper()
	d, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	d.Draw(img, red, false)
	return img
}

func at(img *image.RGBA, x, y int) color.RGBA { return img.RGBAAt(x, y) }

// alphaSum is the total coverage of an image, in pixels.
func alphaSum(img *image.RGBA) float64 {
	var s float64
	for i := 3; i < len(img.Pix); i += 4 {
		s += float64(img.Pix[i]) / 255
	}
	return s
}

func TestFill(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" fill="#00f"/></svg>`, 20, 20)
	if c := at(img, 10, 10); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("inside: %v", c)
	}
	if c := at(img, 4, 4); c.A != 255 {
		t.Errorf("the first pixel inside: %v", c)
	}
	if c := at(img, 3, 3); c.A != 0 {
		t.Errorf("outside: %v", c)
	}
	if got := alphaSum(img); math.Abs(got-144) > 0.5 {
		t.Errorf("coverage %v, want 144", got)
	}
}

func TestPathSyntax(t *testing.T) {
	// The same square, written in the ways path data allows.
	want := draw(t, `<svg viewBox="0 0 10 10"><path d="M1 1 L9 1 L9 9 L1 9 Z"/></svg>`, 10, 10)
	for _, d := range []string{
		"M1,1L9,1L9,9L1,9Z",
		"M1 1h8v8h-8z",
		"m1 1 8 0 0 8-8 0z",
		"M1 1 9 1 9 9 1 9z",
		"M+1.0e0,1.L9,1,9,9,1,9Z",
		"M1 1H9V9H1Z M20 20", // a lone move draws nothing
	} {
		got := draw(t, `<svg viewBox="0 0 10 10"><path d="`+d+`"/></svg>`, 10, 10)
		if string(got.Pix) != string(want.Pix) {
			t.Errorf("%q draws another shape", d)
		}
	}
	// Path data in error draws up to the error.
	part := draw(t, `<svg viewBox="0 0 10 10"><path d="M1 1H9V9H1Z M0 0 L5 x"/></svg>`, 10, 10)
	if string(part.Pix) != string(want.Pix) {
		t.Error("path data in error does not draw what comes before the error")
	}
	if img := draw(t, `<svg viewBox="0 0 10 10"><path d="L1 1 9 9 1 9Z"/></svg>`, 10, 10); alphaSum(img) != 0 {
		t.Error("path data that does not start with a move draws")
	}
}

func TestArcs(t *testing.T) {
	circle := draw(t, `<svg viewBox="0 0 20 20"><circle cx="10" cy="10" r="8"/></svg>`, 40, 40)
	arcs := draw(t, `<svg viewBox="0 0 20 20"><path d="M2 10a8 8 0 1 1 16 0A8 8 0 1 1 2 10z"/></svg>`, 40, 40)
	want := math.Pi * 16 * 16
	for name, img := range map[string]*image.RGBA{"circle": circle, "arcs": arcs} {
		if got := alphaSum(img); math.Abs(got-want) > want*0.01 {
			t.Errorf("%s covers %v pixels, want %v", name, got, want)
		}
	}
	// Flags packed without separators, and radii too small to reach.
	packed := draw(t, `<svg viewBox="0 0 20 20"><path d="M2 10a1 1 0 1116 0a1,1,0,1,1-16,0z"/></svg>`, 40, 40)
	if got := alphaSum(packed); math.Abs(got-want) > want*0.01 {
		t.Errorf("packed arcs cover %v pixels, want %v", got, want)
	}
}

func TestFillRule(t *testing.T) {
	const rings = `M0 0H10V10H0Z M2 2H8V8H2Z`
	nz := draw(t, `<svg viewBox="0 0 10 10"><path d="`+rings+`"/></svg>`, 10, 10)
	eo := draw(t, `<svg viewBox="0 0 10 10"><path fill-rule="evenodd" d="`+rings+`"/></svg>`, 10, 10)
	if at(nz, 5, 5).A != 255 {
		t.Error("the non-zero rule does not fill contours that turn the same way")
	}
	if at(eo, 5, 5).A != 0 || at(eo, 1, 1).A != 255 {
		t.Errorf("the even-odd rule: hole %v, ring %v", at(eo, 5, 5), at(eo, 1, 1))
	}
}

func TestStrokeCaps(t *testing.T) {
	line := func(cap string) *image.RGBA {
		return draw(t, `<svg viewBox="0 0 20 10"><path d="M5 5H15" stroke="#000" stroke-width="4" stroke-linecap="`+cap+`"/></svg>`, 20, 10)
	}
	butt, square, round := line("butt"), line("square"), line("round")
	if got := alphaSum(butt); math.Abs(got-40) > 0.5 {
		t.Errorf("butt caps cover %v, want 40", got)
	}
	if got := alphaSum(square); math.Abs(got-56) > 0.5 {
		t.Errorf("square caps cover %v, want 56", got)
	}
	if got, want := alphaSum(round), 40+4*math.Pi; math.Abs(got-want) > 0.5 {
		t.Errorf("round caps cover %v, want %v", got, want)
	}
	if at(butt, 4, 5).A != 0 || at(square, 4, 5).A != 255 {
		t.Error("caps do not end where they should")
	}
	// A subpath of no length is a dot with round caps.
	dot := draw(t, `<svg viewBox="0 0 10 10"><path d="M5 5z" stroke="#000" stroke-width="4" stroke-linecap="round"/></svg>`, 10, 10)
	if got, want := alphaSum(dot), 4*math.Pi; math.Abs(got-want) > 0.3 {
		t.Errorf("a dot covers %v, want %v", got, want)
	}
}

func TestStrokeJoins(t *testing.T) {
	corner := func(join string) *image.RGBA {
		return draw(t, `<svg viewBox="0 0 20 20"><path d="M2 10H10V18" fill="none" stroke="#000" stroke-width="4" stroke-linejoin="`+join+`"/></svg>`, 20, 20)
	}
	miter, bevel, round := corner("miter"), corner("bevel"), corner("round")
	// The outer corner of the turn is at (12, 8): the miter reaches it,
	// the bevel cuts it off, the round join rounds it.
	if at(miter, 11, 8).A != 255 {
		t.Errorf("the miter does not fill the corner: %v", at(miter, 11, 8))
	}
	if at(bevel, 11, 8).A > 128 {
		t.Errorf("the bevel fills the corner: %v", at(bevel, 11, 8))
	}
	m, b, r := alphaSum(miter), alphaSum(bevel), alphaSum(round)
	if !(m > r && r > b) {
		t.Errorf("coverage of miter %v, round %v, bevel %v", m, r, b)
	}
	// A sharp turn beyond the miter limit is beveled.
	sharp := func(limit string) float64 {
		return alphaSum(draw(t, `<svg viewBox="0 0 40 40"><path d="M2 20L38 22L2 24" fill="none" stroke="#000" stroke-width="2" stroke-miterlimit="`+limit+`"/></svg>`, 40, 40))
	}
	if sharp("4") >= sharp("100") {
		t.Error("the miter limit does not bevel a sharp turn")
	}
}

func TestDashes(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 20 4"><path d="M0 2H20" stroke="#000" stroke-width="2" stroke-dasharray="2 3"/></svg>`, 20, 4)
	for x, want := range []uint8{255, 255, 0, 0, 0, 255, 255, 0, 0, 0} {
		if got := at(img, x, 2).A; got != want {
			t.Errorf("pixel %d: %d, want %d", x, got, want)
		}
	}
	shifted := draw(t, `<svg viewBox="0 0 20 4"><path d="M0 2H20" stroke="#000" stroke-width="2" stroke-dasharray="2 3" stroke-dashoffset="1"/></svg>`, 20, 4)
	if at(shifted, 0, 2).A != 255 || at(shifted, 1, 2).A != 0 || at(shifted, 4, 2).A != 255 {
		t.Error("the dash offset does not shift the dashes")
	}
}

func TestTransforms(t *testing.T) {
	plain := draw(t, `<svg viewBox="0 0 20 20"><rect x="5" y="0" width="5" height="5"/></svg>`, 20, 20)
	for _, rect := range []string{
		`<rect width="5" height="5" transform="translate(5)"/>`,
		`<rect width="5" height="5" transform="translate(5 0)"/>`,
		`<rect width="5" height="5" transform=" matrix(1,0,0,1,5,0) "/>`,
		`<rect width="10" height="10" transform="translate(5 0) scale(0.5)"/>`,
		`<rect width="5" height="5" transform="rotate(90 5 5)"/>`,
		`<g transform="translate(5)"><rect width="10" height="5" transform="scale(.5 1)"/></g>`,
	} {
		got := draw(t, `<svg viewBox="0 0 20 20">`+rect+`</svg>`, 20, 20)
		if alphaSum(got) == 0 || string(got.Pix) != string(plain.Pix) {
			t.Errorf("%s moves the rectangle elsewhere", rect)
		}
	}
	// A transform in error transforms nothing.
	got := draw(t, `<svg viewBox="0 0 20 20"><rect x="5" width="5" height="5" transform="translate(5 0"/></svg>`, 20, 20)
	if string(got.Pix) != string(plain.Pix) {
		t.Error("a transform in error moves the rectangle")
	}
}

func TestCurrentColor(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 2 1"><rect width="1" height="1" fill="currentColor"/><g color="#0f0"><rect x="1" width="1" height="1" fill="currentColor"/></g></svg>`, 2, 1)
	if c := at(img, 0, 0); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("currentColor is %v, not the color it is drawn with", c)
	}
	if c := at(img, 1, 0); c != (color.RGBA{0, 255, 0, 255}) {
		t.Errorf("currentColor is %v, not the color property", c)
	}
}

func TestColors(t *testing.T) {
	for v, want := range map[string]color.RGBA{
		"#f00":                     {255, 0, 0, 255},
		"#FF0000":                  {255, 0, 0, 255},
		"rgb(0, 0, 255)":           {0, 0, 255, 255},
		"rgb(0 0 100%)":            {0, 0, 255, 255},
		"rgba(0,0,255,0.5)":        {0, 0, 128, 128},
		"rgb(0 0 255 / 50%)":       {0, 0, 128, 128},
		"#0000ff80":                {0, 0, 128, 128},
		"hsl(120, 100%, 50%)":      {0, 255, 0, 255},
		"hsl(240deg 100% 50%)":     {0, 0, 255, 255},
		"DarkOrange":               {255, 140, 0, 255},
		"transparent":              {},
		"#ff0000 icc-color(x, 1)":  {255, 0, 0, 255},
		"none":                     {},
		"url(#nothing) #00f":       {0, 0, 255, 255},
		"url(#nothing)":            {},
		"not a color":              {0, 0, 0, 255}, // ignored: black, the initial fill
		"rgb(0,0,0":                {0, 0, 0, 255},
		"#12345":                   {0, 0, 0, 255},
		"hsl(120, 100, 50)":        {0, 0, 0, 255},
		"rgba(255, 255, 255, 0.0)": {},
	} {
		img := draw(t, `<svg viewBox="0 0 1 1"><rect width="1" height="1" fill="`+v+`"/></svg>`, 1, 1)
		if got := at(img, 0, 0); got != want {
			t.Errorf("fill %q: %v, want %v", v, got, want)
		}
	}
}

func TestLinearGradient(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 100 10">
		<defs><linearGradient id="g"><stop offset="0" stop-color="#f00"/><stop offset="100%" stop-color="#00f"/></linearGradient></defs>
		<rect width="100" height="10" fill="url(#g)"/></svg>`, 100, 10)
	left, mid, right := at(img, 0, 5), at(img, 50, 5), at(img, 99, 5)
	if left.R < 250 || left.B > 5 || right.B < 250 || right.R > 5 {
		t.Errorf("ends: %v, %v", left, right)
	}
	if mid.R < 115 || mid.R > 140 || mid.B < 115 || mid.B > 140 {
		t.Errorf("middle: %v", mid)
	}
	// Stops and coordinates inherited through href, in user space, and
	// spread.
	img = draw(t, `<svg viewBox="0 0 100 10">
		<linearGradient id="a"><stop offset="0" stop-color="#000"/><stop offset="1" stop-color="#fff"/></linearGradient>
		<linearGradient id="b" href="#a" gradientUnits="userSpaceOnUse" x1="0" x2="50" spreadMethod="reflect"/>
		<rect width="100" height="10" fill="url(#b)"/></svg>`, 100, 10)
	if c := at(img, 49, 5); c.R < 245 {
		t.Errorf("at the end of the gradient: %v", c)
	}
	if c := at(img, 99, 5); c.R > 10 {
		t.Errorf("reflected: %v", c)
	}
}

func TestRadialGradient(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 20 20">
		<radialGradient id="g"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#000"/></radialGradient>
		<rect width="20" height="20" fill="url(#g)"/></svg>`, 20, 20)
	center, edge, corner := at(img, 10, 10), at(img, 10, 0), at(img, 0, 0)
	if center.R < 230 || edge.R > 30 || corner != (color.RGBA{0, 0, 0, 255}) {
		t.Errorf("center %v, edge %v, corner %v", center, edge, corner)
	}
	// A focal point off center.
	img = draw(t, `<svg viewBox="0 0 20 20">
		<radialGradient id="g" fx="0.25"><stop offset="0" stop-color="#fff"/><stop offset="1" stop-color="#000"/></radialGradient>
		<rect width="20" height="20" fill="url(#g)"/></svg>`, 20, 20)
	if at(img, 5, 10).R < 230 || at(img, 15, 10).R > at(img, 5, 10).R {
		t.Errorf("focal point: %v, %v", at(img, 5, 10), at(img, 15, 10))
	}
}

func TestGroupOpacity(t *testing.T) {
	const rects = `<rect width="6" height="10"/><rect x="4" width="6" height="10"/>`
	group := draw(t, `<svg viewBox="0 0 10 10"><g opacity="0.5">`+rects+`</g></svg>`, 10, 10)
	each := draw(t, `<svg viewBox="0 0 10 10"><g fill-opacity="0.5">`+rects+`</g></svg>`, 10, 10)
	if a, b := at(group, 5, 5).A, at(group, 1, 5).A; a != b || a < 126 || a > 129 {
		t.Errorf("group opacity: overlap %d, alone %d", a, b)
	}
	if at(each, 5, 5).A <= at(each, 1, 5).A {
		t.Error("shapes with their own opacity do not add up where they overlap")
	}
	// Fill and stroke of one shape with opacity.
	img := draw(t, `<svg viewBox="0 0 10 10"><rect x="2" y="2" width="6" height="6" stroke="#000" stroke-width="2" opacity="0.5"/></svg>`, 10, 10)
	if a, b := at(img, 2, 2).A, at(img, 5, 5).A; a != b {
		t.Errorf("an element's opacity: stroke over fill %d, fill %d", a, b)
	}
}

func TestClipPath(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 20 20">
		<clipPath id="c"><circle cx="10" cy="10" r="8"/></clipPath>
		<rect width="20" height="20" clip-path="url(#c)"/></svg>`, 20, 20)
	if at(img, 10, 10).A != 255 || at(img, 0, 0).A != 0 {
		t.Errorf("clipped: center %v, corner %v", at(img, 10, 10), at(img, 0, 0))
	}
	if got, want := alphaSum(img), math.Pi*64; math.Abs(got-want) > want*0.02 {
		t.Errorf("clipped coverage %v, want %v", got, want)
	}
	// objectBoundingBox units, with an even-odd clip rule.
	img = draw(t, `<svg viewBox="0 0 20 20">
		<clipPath id="c" clipPathUnits="objectBoundingBox"><path clip-rule="evenodd" d="M0 0H1V1H0Z M.25 .25H.75V.75H.25Z"/></clipPath>
		<g clip-path="url(#c)"><rect x="4" y="4" width="12" height="12"/></g></svg>`, 20, 20)
	if at(img, 10, 10).A != 0 || at(img, 5, 5).A != 255 || at(img, 2, 2).A != 0 {
		t.Errorf("box clip: hole %v, ring %v, outside %v", at(img, 10, 10), at(img, 5, 5), at(img, 2, 2))
	}
}

func TestMask(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 20 10">
		<mask id="m" maskUnits="userSpaceOnUse" x="0" y="0" width="20" height="10">
			<rect width="10" height="10" fill="#fff"/><rect x="10" width="10" height="10" fill="#000"/>
		</mask>
		<rect width="20" height="10" fill="#00f" mask="url(#m)"/></svg>`, 20, 10)
	if at(img, 5, 5).A != 255 || at(img, 15, 5).A != 0 {
		t.Errorf("masked: white %v, black %v", at(img, 5, 5), at(img, 15, 5))
	}
}

func TestStyleSheets(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 4 1">
		<style><![CDATA[
			/* Illustrator writes classes */
			.a { fill: #00f }
			rect.b, .c { fill: #0f0 !important }
			#d { fill: #f0f }
			@media print { .a { fill: #fff } }
			g > rect { fill: #fff }
		]]></style>
		<rect class="a" width="1" height="1"/>
		<rect class="a b" x="1" width="1" height="1"/>
		<rect class="a" id="d" x="2" width="1" height="1"/>
		<rect class="a" x="3" width="1" height="1" fill="#f00" style="fill: #ff0"/>
	</svg>`, 4, 1)
	for x, want := range []color.RGBA{{0, 0, 255, 255}, {0, 255, 0, 255}, {255, 0, 255, 255}, {255, 255, 0, 255}} {
		if got := at(img, x, 0); got != want {
			t.Errorf("rect %d: %v, want %v", x, got, want)
		}
	}
}

func TestUse(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 20 10">
		<defs><rect id="r" width="5" height="5"/>
		<symbol id="s" viewBox="0 0 1 1"><rect width="1" height="1" fill="#00f"/></symbol></defs>
		<use href="#r" x="5"/>
		<use xlink:href="#s" x="10" y="0" width="5" height="5" xmlns:xlink="http://www.w3.org/1999/xlink"/>
		<use href="#loop" id="loop"/>
	</svg>`, 20, 10)
	if at(img, 1, 1).A != 0 || at(img, 6, 1).A != 255 {
		t.Error("use does not draw its element where it says")
	}
	if c := at(img, 12, 2); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("a symbol: %v", c)
	}
	if at(img, 12, 7).A != 0 {
		t.Error("a symbol draws outside its viewport")
	}
}

func TestDisplayAndVisibility(t *testing.T) {
	img := draw(t, `<svg viewBox="0 0 3 1">
		<rect width="1" height="1" display="none"/>
		<g visibility="hidden"><rect x="1" width="1" height="1"/><rect x="2" width="1" height="1" visibility="visible"/></g>
	</svg>`, 3, 1)
	if at(img, 0, 0).A != 0 || at(img, 1, 0).A != 0 || at(img, 2, 0).A != 255 {
		t.Errorf("display none %v, hidden %v, visible %v", at(img, 0, 0), at(img, 1, 0), at(img, 2, 0))
	}
}

func TestSizeAndAspect(t *testing.T) {
	for src, want := range map[string][2]float64{
		`<svg width="24" height="24" viewBox="0 0 48 48"/>`:   {24, 24},
		`<svg viewBox="0 0 48 24"/>`:                          {48, 24},
		`<svg width="10" viewBox="0 0 48 24"/>`:               {10, 5},
		`<svg width="100%" height="100%" viewBox="0 0 8 4"/>`: {8, 4},
		`<svg width="1in" height="12pt"/>`:                    {96, 16},
		`<svg/>`:                                              {300, 150},
	} {
		d, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if w, h := d.Size(); w != want[0] || h != want[1] {
			t.Errorf("%s: %vx%v, want %vx%v", src, w, h, want[0], want[1])
		}
	}
	// A wide viewBox in a square: centered, unless aligned or stretched.
	const wide = `<rect width="10" height="5"/></svg>`
	mid := draw(t, `<svg viewBox="0 0 10 5">`+wide, 10, 10)
	if at(mid, 5, 1).A != 0 || at(mid, 5, 5).A != 255 || at(mid, 5, 8).A != 0 {
		t.Error("the viewBox is not centered")
	}
	top := draw(t, `<svg viewBox="0 0 10 5" preserveAspectRatio="xMinYMin meet">`+wide, 10, 10)
	if at(top, 5, 1).A != 255 || at(top, 5, 8).A != 0 {
		t.Error("the viewBox is not aligned to the top")
	}
	d, _ := Parse([]byte(`<svg viewBox="0 0 10 5">` + wide))
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	d.Draw(img, red, true)
	if alphaSum(img) != 100 {
		t.Error("the viewBox is not stretched")
	}
}

func TestOldEditors(t *testing.T) {
	// Illustrator's declarations, entities, namespaces and switch.
	src := `<?xml version="1.0" encoding="iso-8859-1"?>
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd" [
	<!ENTITY ns_svg "http://www.w3.org/2000/svg">
]>
<svg xmlns="&ns_svg;" xmlns:i="&ns_ai;" viewBox="0 0 10 10">
<switch>
	<foreignObject requiredExtensions="&ns_ai;" x="0" y="0" width="1" height="1"/>
	<g i:extraneous="self"><rect width="10" height="10" fill="#00f"/></g>
</switch>
<!-- caf` + "\xe9" + ` -->
</svg>`
	img := draw(t, src, 10, 10)
	if c := at(img, 5, 5); c != (color.RGBA{0, 0, 255, 255}) {
		t.Errorf("drawn %v", c)
	}
}

func TestErrors(t *testing.T) {
	for _, src := range []string{``, `<html/>`, `<svg><rect`, `not xml`} {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%q parses", src)
		}
	}
}

func TestIcons(t *testing.T) {
	// A check mark stroked in currentColor, as icon sets draw them.
	check := draw(t, checkIcon, 24, 24)
	if c := at(check, 9, 16); c != (color.RGBA{255, 0, 0, 255}) {
		t.Errorf("the check mark's corner: %v", c)
	}
	if at(check, 2, 2).A != 0 {
		t.Error("the check mark covers its corner")
	}
	// A house filled in a viewBox above the origin, with a door cut out.
	house := draw(t, `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 -960 960 960"><path fill-rule="evenodd" d="M160-120V-600L480-840L800-600V-120ZM400-120V-360H560V-120Z"/></svg>`, 24, 24)
	if at(house, 12, 6).A != 255 || at(house, 6, 15).A != 255 || at(house, 12, 18).A != 0 || at(house, 2, 2).A != 0 {
		t.Errorf("the house: roof %v, wall %v, door %v", at(house, 12, 6), at(house, 6, 15), at(house, 12, 18))
	}
}

func FuzzParse(f *testing.F) {
	for _, s := range []string{
		`<svg viewBox="0 0 10 10"><path d="M1 1a5 5 0 1 0 8 8z" stroke="red" stroke-dasharray="1 2"/></svg>`,
		`<svg><g opacity=".5" clip-path="url(#c)"><use href="#u"/></g><clipPath id="c"><use href="#r"/></clipPath><rect id="r" width="5" height="5"/></svg>`,
		`<svg><mask id="m"><rect width="1" height="1" mask="url(#m)"/></mask><circle r="5" mask="url(#m)"/></svg>`,
		`<svg><linearGradient id="a" href="#b"/><radialGradient id="b" href="#a" fr="2"/><rect width="9" height="9" fill="url(#a)"/></svg>`,
	} {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		d, err := Parse(data)
		if err != nil {
			return
		}
		d.Draw(image.NewRGBA(image.Rect(0, 0, 16, 16)), red, false)
	})
}

const checkIcon = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M20 6 9 17l-5-5"/></svg>`

// gear returns an icon of a gear: arcs and lines with round joins.
func gear(teeth int) string {
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="`)
	for i := range teeth * 2 {
		r := 10.0
		if i%2 == 1 {
			r = 7.5
		}
		a0, a1 := float64(i)*math.Pi/float64(teeth), float64(i+1)*math.Pi/float64(teeth)
		cmd := "L"
		if i == 0 {
			cmd = "M"
		}
		fmt.Fprintf(&b, "%s%.2f %.2fA%.1f %.1f 0 0 1 %.2f %.2f", cmd, 12+r*math.Cos(a0), 12+r*math.Sin(a0), r, r, 12+r*math.Cos(a1), 12+r*math.Sin(a1))
	}
	b.WriteString(`Z"/><circle cx="12" cy="12" r="3"/></svg>`)
	return b.String()
}

var gearIcon = gear(8)

// BenchmarkParse parses an icon of a gear drawn with arcs.
func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		if _, err := Parse([]byte(gearIcon)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDrawIcon draws icons of a check and of the gear at 16 to 256
// pixels.
func BenchmarkDrawIcon(b *testing.B) {
	for _, bc := range []struct {
		name string
		src  string
		size int
	}{{"check16", checkIcon, 16}, {"gear16", gearIcon, 16}, {"gear32", gearIcon, 32}, {"gear256", gearIcon, 256}} {
		b.Run(bc.name, func(b *testing.B) {
			d, _ := Parse([]byte(bc.src))
			img := image.NewRGBA(image.Rect(0, 0, bc.size, bc.size))
			for b.Loop() {
				clear(img.Pix)
				d.Draw(img, red, false)
			}
		})
	}
}

func TestUsesCurrentColor(t *testing.T) {
	for src, want := range map[string]bool{
		checkIcon: true,
		`<svg><rect width="1" height="1" fill="#000"/></svg>`:                                                                             false,
		`<svg color="red"><rect width="1" height="1" fill="currentColor"/></svg>`:                                                         false,
		`<svg><linearGradient id="g"><stop stop-color="currentColor"/></linearGradient><rect width="1" height="1" fill="url(#g)"/></svg>`: true,
	} {
		d, err := Parse([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		if got := d.UsesCurrentColor(); got != want {
			t.Errorf("%s: UsesCurrentColor %v", src, got)
		}
	}
}
