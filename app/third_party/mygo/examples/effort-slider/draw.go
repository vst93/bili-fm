package main

import (
	"github.com/egoist/mygo/ui"
)

// word is a text shaped letter by letter, as Chrome lays out the demo's
// labels, whose letters animate each in its own element: each at its own
// advance, without the kerning and tracking Core Text adds between them.
type word struct {
	letters [][]ui.Glyph
	x       []float32 // where each letter starts
	width   float32
}

type wordKey struct {
	s    string
	font ui.Font
}

var words = map[wordKey]word{}

// shapeWord returns s shaped in f, once.
func shapeWord(s string, f ui.Font) word {
	k := wordKey{s, f}
	if w, ok := words[k]; ok {
		return w
	}
	var w word
	for _, r := range s {
		g := ui.Shape(string(r), f)
		w.letters = append(w.letters, g)
		w.x = append(w.x, w.width)
		for _, gl := range g {
			w.width += gl.Advance
		}
	}
	words[k] = w
	return w
}

// kerned returns s shaped as a whole, kerned, as Chrome lays out the
// demo's plain text, once.
func kerned(s string, f ui.Font) word {
	k := wordKey{"\x00" + s, f}
	if w, ok := words[k]; ok {
		return w
	}
	g := ui.Shape(s, f)
	w := word{letters: [][]ui.Glyph{g}, x: []float32{0}}
	for _, gl := range g {
		w.width = max(w.width, gl.X+gl.Advance)
	}
	words[k] = w
	return w
}

// drawWord draws w from x on the baseline at y.
func drawWord(p *ui.Painter, w word, x, y float32, c ui.Color) {
	for i, g := range w.letters {
		p.Glyphs(g, x+w.x[i], y, c)
	}
}

// The brain of Lucide 0.4, whose hemispheres close, so that they fill: 24
// units square, stroked 2 units wide with round caps and joins.
var (
	brainPaths = parsePaths(
		"M12 5a3 3 0 1 0-5.997.125 4 4 0 0 0-2.526 5.77 4 4 0 0 0 .556 6.588A4 4 0 1 0 12 18Z",
		"M12 5a3 3 0 1 1 5.997.125 4 4 0 0 1 2.526 5.77 4 4 0 0 1-.556 6.588A4 4 0 1 1 12 18Z",
		"M15 13a4.5 4.5 0 0 1-3-4 4.5 4.5 0 0 1-3 4",
		"M17.599 6.5a3 3 0 0 0 .399-1.375",
		"M6.003 5.125A3 3 0 0 0 6.401 6.5",
		"M3.477 10.896a4 4 0 0 1 .585-.396",
		"M19.938 10.5a4 4 0 0 1 .585.396",
		"M6 18a4 4 0 0 1-1.967-.516",
		"M19.967 17.484A4 4 0 0 1 18 18",
	)
)

func parsePaths(ds ...string) []svgPath {
	out := make([]svgPath, len(ds))
	for i, d := range ds {
		out[i] = parseSVGPath(d)
	}
	return out
}

// icon returns paths as a ui.Path size DIPs square centered on (cx, cy).
func icon(paths []svgPath, cx, cy, size float32) *ui.Path {
	var out ui.Path
	s := size / 24
	for _, sp := range paths {
		sp.appendTo(&out, cx-12*s, cy-12*s, s)
	}
	return &out
}

// The help button is a question mark in a circle of 1 DIP.
var helpFont = ui.Font{Family: "system-ui", Size: 8.8, Weight: 600, Antialiased: true}

// drawHelp draws the help button, centered on (cx, cy).
func drawHelp(p *ui.Painter, cx, cy float32, c ui.Color) {
	var ring ui.Path
	ring.Circle(cx, cy, helpRadius-0.5)
	p.StrokePath(&ring, 1, c)
	// The mark, centered in its line as CSS centers it.
	g := ui.Shape("?", helpFont)
	m := helpFont.Metrics()
	p.Glyphs(g, cx+helpMarkDX-g[0].Advance/2, cy+(m.Ascent-m.Descent)/2, c)
}

// drawBrain draws the brain filled with fill and outlined in stroke.
func drawBrain(p *ui.Painter, cx, cy, size float32, fill, stroke ui.Color) {
	p.FillPath(icon(brainPaths[:2], cx, cy, size), fill)
	p.StrokePath(icon(brainPaths, cx, cy, size), 2*size/24, stroke)
}
