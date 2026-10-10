package scene

import (
	"math"
	"testing"
)

// TestGammaRatios checks Direct2D's ratios for its default gamma, 1.8, as
// Windows Terminal lists them, and that gammas round to a tenth.
func TestGammaRatios(t *testing.T) {
	want := [4]float32{0.148054421, -0.894594550, 1.47590804, -0.324668258}
	got := GammaRatios(1.8)
	for i := range want {
		if math.Abs(float64(got[i]-want[i])) > 1e-6 {
			t.Fatalf("GammaRatios(1.8) = %v, want %v", got, want)
		}
	}
	if GammaRatios(1.84) != got || GammaRatios(1) != [4]float32{} || GammaRatios(3) != GammaRatios(2.2) {
		t.Error("gammas do not round to the table")
	}
}

// TestTextCoverage checks that no correction leaves coverage as it is,
// that coverage stays between 0 and 1 and keeps its ends, and that the
// contrast is enhanced for dark text alone.
func TestTextCoverage(t *testing.T) {
	g := GammaRatios(1.8)
	black, white := [3]float32{}, [3]float32{1, 1, 1}
	for a := float32(0); a <= 1; a += 1.0 / 16 {
		if v := TextCoverage(a, black, 0, 0, [4]float32{}); v != a {
			t.Fatalf("no correction turns %v into %v", a, v)
		}
		for _, c := range [][3]float32{black, white, {0.5, 0.2, 0.9}} {
			v := TextCoverage(a, c, 1, 0, g)
			if v < 0 || v > 1 {
				t.Fatalf("coverage %v of %v corrected to %v", a, c, v)
			}
			s := SubpixelCoverage([3]float32{a, a, a}, c, 0.5, ThinBoost, g)
			if s[0] < 0 || s[0] > 1 {
				t.Fatalf("subpixel coverage %v of %v corrected to %v", a, c, s)
			}
		}
	}
	for _, a := range []float32{0, 1} {
		if v := TextCoverage(a, black, 1, 0, g); v != a {
			t.Errorf("coverage %v of black corrected to %v", a, v)
		}
	}
	if TextCoverage(0.5, black, 1, 0, g) <= TextCoverage(0.5, black, 0, 0, g) {
		t.Error("the contrast of dark text is not enhanced")
	}
	// Light text takes no contrast, and its gamma correction thickens it.
	if TextCoverage(0.5, white, 1, 0, g) != TextCoverage(0.5, white, 0, 0, g) || TextCoverage(0.5, white, 0, 0, g) <= 0.5 {
		t.Error("light text")
	}
	// Thin glyphs take their boost whatever their color.
	if TextCoverage(0.5, white, 1, ThinBoost, g) <= TextCoverage(0.5, white, 1, 0, g) {
		t.Error("light thin text takes no boost")
	}
}
