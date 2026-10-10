package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/text"
)

func TestLetterSpacingAndFeatures(t *testing.T) {
	var params text.Params
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(8).AlignItems(Start).Children(func() {
			coreText(c, "Wide").Label("plain")
			coreText(c, "Wide").LetterSpacing(6).Label("spaced")
			coreRow(c).FontFeatures("tnum", "liga=0").LetterSpacing(1).Children(func() {
				params = coreText(c, "12:30").textParams(0)
			})
		})
	}, 400, 200)
	plain, _ := tt.Find("plain")
	spaced, _ := tt.Find("spaced")
	if spaced.W < plain.W+3*6 {
		t.Errorf("letter spacing of 6 makes Wide %v wide, from %v", spaced.W, plain.W)
	}
	if s := params.Style; s.Features != "tnum,liga=0" || s.LetterSpacing != 1 {
		t.Errorf("the text inherits features %q and letter spacing %v", s.Features, s.LetterSpacing)
	}
}
