package view

import "testing"

func TestLyricsOffsetLabel(t *testing.T) {
	cases := map[float64]string{0: "0.0s", 1.2: "+1.2s", -1.2: "-1.2s", 10: "+10.0s"}
	for in, want := range cases {
		if got := lyricsOffsetLabel(in); got != want {
			t.Errorf("offset %.1f label = %q, want %q", in, got, want)
		}
	}
}
