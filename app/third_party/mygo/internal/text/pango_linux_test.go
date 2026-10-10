//go:build linux && (amd64 || arm64)

package text

import (
	"fmt"
	"testing"
)

// A flex item may give text exactly the width of its unconstrained
// layout. Kerning in strings such as "11" must not make Pango wrap it.
func TestPangoLayoutAtMeasuredWidth(t *testing.T) {
	s := newSystem()
	for _, family := range []string{"sans-serif", "serif", "monospace"} {
		for _, size := range []float32{10, 12, 14, 16} {
			for _, value := range []string{"11", "10", "12", "1911", "2011", "2111", "2211", "2001", "2002", "2012", "2021", "2011-04-17", "2011-06-17", "11:30", "2.11.1", "AV", "office"} {
				t.Run(fmt.Sprintf("%s/%g/%s", family, size, value), func(t *testing.T) {
					p := Params{Text: value, Style: Style{Family: family, Size: size}}
					p.Width = s.Layout(p).Width
					for _, maxLines := range []int{0, 1} {
						p.MaxLines = maxLines
						l := s.Layout(p)
						if len(l.Lines) != 1 || l.Truncated || l.Lines[0].End != len(l.Runes) {
							t.Errorf("at measured width %g, MaxLines %d: %d lines, truncated %v, end %d of %d", p.Width, maxLines, len(l.Lines), l.Truncated, l.Lines[0].End, len(l.Runes))
						}
					}
				})
			}
		}
	}
}
