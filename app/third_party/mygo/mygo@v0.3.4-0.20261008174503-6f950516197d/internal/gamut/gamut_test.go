package gamut

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestOklabKnownColors(t *testing.T) {
	// Reference values from Björn Ottosson's Oklab post.
	l, a, b := Oklab(1, 1, 1)
	if !near(l, 1, 1e-4) || !near(a, 0, 1e-4) || !near(b, 0, 1e-4) {
		t.Errorf("white = %v %v %v", l, a, b)
	}
	l, a, b = Oklab(1, 0, 0)
	if !near(l, 0.6280, 1e-3) || !near(a, 0.2249, 1e-3) || !near(b, 0.1258, 1e-3) {
		t.Errorf("red = %v %v %v", l, a, b)
	}
}

func TestOklabRoundTrip(t *testing.T) {
	for _, c := range [][3]float64{{0.2, 0.5, 0.8}, {0, 0, 0}, {1, 0.1, 0.9}, {-0.1, 0.4, 1.1}} {
		l, a, b := Oklab(c[0], c[1], c[2])
		r, g, bb := LinearSRGB(l, a, b)
		if !near(r, c[0], 1e-5) || !near(g, c[1], 1e-5) || !near(bb, c[2], 1e-5) {
			t.Errorf("%v came back as %v %v %v", c, r, g, bb)
		}
	}
}

func TestEncodeDecode(t *testing.T) {
	for _, v := range []float64{-0.5, -0.01, 0, 0.001, 0.5, 1, 1.3} {
		if got := Decode(Encode(v)); !near(got, v, 1e-9) {
			t.Errorf("Decode(Encode(%v)) = %v", v, got)
		}
	}
	if !near(Encode(0.5), 0.7354, 1e-3) {
		t.Errorf("Encode(0.5) = %v", Encode(0.5))
	}
}

func TestP3Matrices(t *testing.T) {
	r, g, b := LinearP3ToSRGB(LinearSRGBToP3(0.3, 0.6, 0.9))
	if !near(r, 0.3, 1e-4) || !near(g, 0.6, 1e-4) || !near(b, 0.9, 1e-4) {
		t.Errorf("round trip = %v %v %v", r, g, b)
	}
	// P3 white is sRGB white.
	r, g, b = LinearP3ToSRGB(1, 1, 1)
	if !near(r, 1, 1e-4) || !near(g, 1, 1e-4) || !near(b, 1, 1e-4) {
		t.Errorf("white = %v %v %v", r, g, b)
	}
	// Pure P3 green lies outside sRGB.
	if _, g, _ := LinearP3ToSRGB(0, 1, 0); g <= 1 {
		t.Errorf("P3 green in sRGB has g = %v, want above 1", g)
	}
}

func TestMapInGamutIsExact(t *testing.T) {
	l, a, b := Oklab(Decode(0.2), Decode(0.5), Decode(0.8))
	l, c, h := Oklch(l, a, b)
	r, g, bb := Map(SRGB, l, c, h)
	if !near(Encode(r), 0.2, 1e-3) || !near(Encode(g), 0.5, 1e-3) || !near(Encode(bb), 0.8, 1e-3) {
		t.Errorf("mapped to %v %v %v", Encode(r), Encode(g), Encode(bb))
	}
}

func TestMapKeepsLightnessAndHue(t *testing.T) {
	for _, sp := range []Space{SRGB, DisplayP3} {
		for _, c := range [][3]float64{{0.7, 0.4, 150}, {0.5, 0.35, 30}, {0.9, 0.3, 200}, {0.3, 0.3, 280}} {
			r, g, b := Map(sp, c[0], c[1], c[2])
			if !within(r, g, b) {
				t.Errorf("space %d, %v: %v %v %v outside the gamut", sp, c, r, g, b)
			}
			if sp == DisplayP3 {
				r, g, b = LinearP3ToSRGB(r, g, b)
			}
			l, a, bb := Oklab(r, g, b)
			l, ch, h := Oklch(l, a, bb)
			if !near(l, c[0], 0.03) {
				t.Errorf("space %d, %v: lightness %v", sp, c, l)
			}
			if ch > 0.02 && math.Abs(math.Mod(h-c[2]+540, 360)-180) > 10 {
				t.Errorf("space %d, %v: hue %v", sp, c, h)
			}
			if ch > c[1]+1e-6 {
				t.Errorf("space %d, %v: chroma grew to %v", sp, c, ch)
			}
		}
	}
}

func TestMapP3KeepsMoreThanSRGB(t *testing.T) {
	// A vivid green is further from gray in Display P3 than in sRGB.
	chroma := func(sp Space) float64 {
		r, g, b := Map(sp, 0.85, 0.3, 145)
		l, a, bb := fromSpace(sp, r, g, b)
		_, c, _ := Oklch(l, a, bb)
		return c
	}
	if cs, cp := chroma(SRGB), chroma(DisplayP3); cp <= cs {
		t.Errorf("chroma: sRGB %v, P3 %v", cs, cp)
	}
}

func TestMapOddInput(t *testing.T) {
	gray := func(sp Space, l, c, h float64) {
		t.Helper()
		r, g, b := Map(sp, l, c, h)
		if !near(r, g, 1e-6) || !near(g, b, 1e-6) {
			t.Errorf("Map(%v, %v, %v) = %v %v %v, want a gray", l, c, h, r, g, b)
		}
	}
	// A negative chroma is 0, as CSS clamps it, not the opposite hue.
	gray(SRGB, 0.7, -0.1, 30)
	gray(DisplayP3, 0.7, math.NaN(), 30)
	// An infinite chroma ends, as the most vivid color of the hue.
	inf := math.Inf(1)
	r, g, b := Map(SRGB, 0.5, inf, 30)
	r2, g2, b2 := Map(SRGB, 0.5, 0.6, 30)
	if !near(r, r2, 1e-3) || !near(g, g2, 1e-3) || !near(b, b2, 1e-3) {
		t.Errorf("infinite chroma = %v %v %v, want %v %v %v", r, g, b, r2, g2, b2)
	}
	for _, c := range [][3]float64{{math.NaN(), 0.2, 30}, {0.5, 0.2, inf}, {0.5, 0.2, math.NaN()}, {0.5, inf, inf}} {
		r, g, b := Map(DisplayP3, c[0], c[1], c[2])
		if !within(r, g, b) || math.IsNaN(r+g+b) {
			t.Errorf("Map(%v) = %v %v %v", c, r, g, b)
		}
	}
}

func TestMapExtremes(t *testing.T) {
	if r, g, b := Map(SRGB, 1.2, 0.3, 40); r != 1 || g != 1 || b != 1 {
		t.Errorf("light = %v %v %v", r, g, b)
	}
	if r, g, b := Map(DisplayP3, -0.1, 0.3, 40); r != 0 || g != 0 || b != 0 {
		t.Errorf("dark = %v %v %v", r, g, b)
	}
}
