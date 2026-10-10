package ui

import (
	"fmt"
	"math"
	"testing"
)

// BenchmarkFrame builds, lays out, paints and renders on the CPU a whole
// frame of a 980×720 window at twice the density, as on macOS and Linux.
func BenchmarkFrame(b *testing.B) {
	d := &demo{choice: "a", size: "Medium", volume: 40}
	tt := coreNewTester(d.view, 980, 720)
	tt.SetScale(2)
	for b.Loop() {
		tt.h.img.Invalidate()
		tt.Frame()
	}
}

// BenchmarkFrameHover renders the frames of the pointer going over a
// button and off it: the CPU renderer redraws the button alone.
func BenchmarkFrameHover(b *testing.B) {
	d := &demo{choice: "a", size: "Medium", volume: 40}
	tt := coreNewTester(d.view, 980, 720)
	tt.SetScale(2)
	r, _ := tt.Find("Increment")
	on := false
	for b.Loop() {
		if on = !on; on {
			tt.Move(r.X+5, r.Y+5)
		} else {
			tt.Move(r.X-5, r.Y-5)
		}
	}
}

// BenchmarkFrameAnimatedPaths renders the frames of a card drawn with a
// path that changes every frame, as the gallery's Drawing page: a stroked
// wave, whose mask is drawn anew each frame, and a disc, which is cached.
func BenchmarkFrameAnimatedPaths(b *testing.B) {
	frame := 0
	view := func(c *context) {
		frame++
		phase := float64(frame) * 0.05
		coreBox(c).Size(700, 320).Draw(func(p *Painter, r Rect) {
			var wave Path
			for i := 0; i <= 100; i++ {
				x := r.X + 20 + float32(i)*6
				y := r.Y + r.H/2 + float32(60*math.Sin(phase+float64(i)/12))
				if i == 0 {
					wave.MoveTo(x, y)
				} else {
					wave.LineTo(x, y)
				}
			}
			p.StrokePath(&wave, 3, RGB(220, 40, 40))
			var dot Path
			dot.Circle(r.X+r.W-70, r.Y+70, 36)
			p.FillPath(&dot, RGB(37, 99, 235))
		})
	}
	tt := coreNewTester(view, 980, 720)
	tt.SetScale(2)
	b.ReportAllocs()
	for b.Loop() {
		tt.Frame()
	}
}

// diffScene is a window as godiff's: a sidebar listing 200 commits beside
// a diff whose lines are rich texts of colored spans, both Lists.
type diffScene struct {
	commits []benchCommit
	lines   []benchLine
	diff    bool // whether the diff shows
	history ListState
	changes ListState
}

type benchCommit struct{ short, subject, author, ago, when string }

type benchLine struct {
	old, new string
	kind     int // 0 kept, 1 added, 2 deleted
	// spans are kept from frame to frame, as godiff keeps them.
	spans []Span
}

func newDiffScene(diff bool) *diffScene {
	d := &diffScene{diff: diff}
	words := []string{"fix", "the", "list", "scroll", "when", "rows", "change", "height", "and", "keep", "focus", "on", "row"}
	for i := range 200 {
		subject := ""
		for j := range 4 + i%7 {
			subject += words[(i*3+j)%len(words)] + " "
		}
		d.commits = append(d.commits, benchCommit{
			short: fmt.Sprintf("%07x", i*7919), subject: subject, author: []string{"Ada Lovelace", "Alan Turing", "Grace Hopper"}[i%3],
			ago: fmt.Sprintf("%dd ago", i+1), when: fmt.Sprintf("Mon Jan %d 15:04:05 2026", i%28+1),
		})
	}
	palette := []Color{RGB(207, 34, 46), RGB(5, 80, 174), RGB(130, 80, 223), RGB(17, 99, 41), RGB(149, 56, 0), RGB(36, 41, 47)}
	tokens := []string{"func", " ", "(w", " *window)", " diffRow", "(c", " *ui.Context,", " pal", " *palette,", " i", " int)", " {", "\t", "return", " nil", "//", " comment"}
	for i := range 400 {
		var spans []Span
		for j := range 5 + i%16 {
			s := Span{Text: tokens[(i+j)%len(tokens)], Color: palette[(i*5+j)%len(palette)]}
			if j == 3 && i%4 == 1 {
				s.Background = RGBA(46, 160, 67, 0.4)
			}
			spans = append(spans, s)
		}
		d.lines = append(d.lines, benchLine{old: fmt.Sprint(i + 1), new: fmt.Sprint(i + 3), kind: i % 3, spans: spans})
	}
	return d
}

func (d *diffScene) view(c *context) {
	t := c.Theme()
	muted := t.TextMuted
	coreRow(c).Grow(1).AlignItems(Stretch).Children(func() {
		coreList(c, &d.history, len(d.commits), func(i int) { d.commitRow(c, i, muted) }).
			Width(300).Shrink(0).Padding(2, 8).Gap(1).Focusable().FocusRing(false).Label("History")
		if !d.diff {
			return
		}
		coreList(c, &d.changes, len(d.lines), func(i int) { d.lineRow(c, i) }).
			Grow(1).Padding(0, 12, 24).Background(RGB(246, 248, 250)).Label("Changes")
	})
}

func (d *diffScene) commitRow(c *context, i int, muted Color) {
	cm := &d.commits[i]
	row := coreRow(c).Gap(8).Padding(5, 8).Radius(6).AlignItems(Start).Role(RoleButton).Label(cm.subject)
	if row.Hovered() {
		row.Background(RGBA(127, 127, 127, 0.08))
	}
	row.Clicked()
	row.Children(func() {
		coreText(c, cm.short).Font("SF Mono, Menlo, monospace").FontSize(12).TextColor(RGB(5, 80, 174)).Width(56).Shrink(0)
		coreColumn(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			coreText(c, cm.subject).FontSize(12).SingleLine().Tooltip(cm.subject)
			coreRow(c).Gap(6).Children(func() {
				coreText(c, cm.author).FontSize(10).SingleLine().TextColor(muted).Grow(1).MinWidth(0)
				coreText(c, cm.ago).FontSize(10).TextColor(muted).Shrink(0).Tooltip(cm.when)
			})
		})
	})
}

func (d *diffScene) lineRow(c *context, i int) {
	l := &d.lines[i]
	const lh, gutter = 20, 44
	accent := c.Theme().Accent
	bg, gutterBg, bar := RGB(255, 255, 255), RGB(255, 255, 255), Transparent
	switch l.kind {
	case 1:
		bg, gutterBg, bar = RGB(230, 255, 236), RGB(204, 255, 216), RGB(46, 160, 67)
	case 2:
		bg, gutterBg, bar = RGB(255, 235, 233), RGB(255, 215, 213), RGB(207, 34, 46)
	}
	coreRow(c).BorderWidth(0, 1, 0, 1).BorderColor(RGB(208, 215, 222)).Children(func() {
		cell := coreRow(c).Grow(1).MinWidth(0).AlignItems(Stretch).Background(bg)
		hovered := cell.Hovered()
		if hovered {
			cell.Background(RGB(240, 240, 240))
		}
		cell.Clicked()
		cell.Children(func() {
			coreRow(c).Shrink(0).AlignItems(Start).Background(gutterBg).Children(func() {
				coreBox(c).Width(4).AlignSelf(Stretch).Background(bar)
				coreText(c, l.old).Font("SF Mono, Menlo, monospace").FontSize(12).FixedLineHeight(lh).TextColor(RGB(110, 119, 129)).
					Width(gutter-4).TextAlign(End).Padding(0, 8, 0, 0).Shrink(0)
				coreText(c, l.new).Font("SF Mono, Menlo, monospace").FontSize(12).FixedLineHeight(lh).TextColor(RGB(110, 119, 129)).
					Width(gutter-4).TextAlign(End).Padding(0, 8, 0, 0).Shrink(0)
			})
			plus := coreButtonBase(c).Absolute().Top((lh-18)/2).Left(2*gutter-13).Size(18, 18).Radius(5).Center().
				Background(accent).Label("Comment").Tooltip("Comment on this line").FocusRing(false)
			if !hovered {
				plus.Opacity(0)
			}
			plus.Clicked()
			coreBox(c).Grow(1).Basis(0).MinWidth(0).ClipX().Margin(0, 10).Children(func() {
				coreRichText(c, l.spans...).Font("SF Mono, Menlo, monospace").FontSize(13).FixedLineHeight(lh).
					TextColor(RGB(36, 41, 47)).NoWrap().AlignSelf(Start)
			})
		})
	})
}

// diffSceneTester shows a diffScene in a 1280×800 window at twice the
// density, its history scrolled through once, so that what comes into
// view has been laid out before, as in a window used for a while.
func diffSceneTester(diff bool) (*Tester, *diffScene) {
	d := newDiffScene(diff)
	tt := coreNewTester(d.view, 1280, 800)
	tt.SetScale(2)
	tt.Move(150, 400)
	s := &sidebarScroller{tt: tt, d: d}
	for range 600 {
		s.step()
	}
	return tt, d
}

// sidebarScroller scrolls the history by 40 DIPs a frame, down to its end
// and back up again, or within the rows of lo to hi. Over the whole list,
// rows come into view whose texts were not laid out for a while, as when
// scrolling through a long history: the text system lays them out again.
type sidebarScroller struct {
	tt     *Tester
	d      *diffScene
	down   bool
	lo, hi int
}

func (s *sidebarScroller) step() {
	first, last := s.d.history.Visible()
	lo, hi := s.lo, s.hi
	if hi == 0 {
		lo, hi = 0, len(s.d.commits)-1
	}
	switch {
	case last >= hi:
		s.down = false
	case first <= lo:
		s.down = true
	}
	dy := float32(-40)
	if s.down {
		dy = 40
	}
	s.tt.Scroll(150, 400, 0, dy)
}

// BenchmarkDiffSteady renders a frame of the diff scene where nothing
// changed, as Invalidate asks for.
func BenchmarkDiffSteady(b *testing.B) {
	tt, _ := diffSceneTester(true)
	b.ReportAllocs()
	for b.Loop() {
		tt.Frame()
	}
}

// BenchmarkDiffScrollSidebar renders the frames scrolling the history by
// 40 DIPs without the diff, and BenchmarkDiffScroll with it.
// BenchmarkDiffScrollSidebarWarm scrolls within 40 rows, whose texts stay
// laid out.
func BenchmarkDiffScrollSidebar(b *testing.B)     { benchDiffScroll(b, false, 0, 0) }
func BenchmarkDiffScroll(b *testing.B)            { benchDiffScroll(b, true, 0, 0) }
func BenchmarkDiffScrollSidebarWarm(b *testing.B) { benchDiffScroll(b, false, 80, 120) }

func benchDiffScroll(b *testing.B, diff bool, lo, hi int) {
	tt, d := diffSceneTester(diff)
	s := &sidebarScroller{tt: tt, d: d, lo: lo, hi: hi}
	for range 100 {
		s.step()
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		s.step()
	}
}

// TestDiffFrameAllocs reports the allocations of the diff scene's frames,
// and checks those of frames that lay nothing out.
func TestDiffFrameAllocs(t *testing.T) {
	if testing.Short() {
		t.Skip("measures")
	}
	tt, _ := diffSceneTester(true)
	steady := testing.AllocsPerRun(100, tt.Frame)
	tt, d := diffSceneTester(false)
	s := &sidebarScroller{tt: tt, d: d}
	sidebar := testing.AllocsPerRun(100, s.step)
	tt, d = diffSceneTester(true)
	s = &sidebarScroller{tt: tt, d: d}
	both := testing.AllocsPerRun(100, s.step)
	tt, d = diffSceneTester(false)
	s = &sidebarScroller{tt: tt, d: d, lo: 80, hi: 120}
	for range 100 {
		s.step()
	}
	warm := testing.AllocsPerRun(100, s.step)
	t.Logf("allocations a frame: steady %.0f, scrolling the sidebar %.0f, with the diff %.0f, within rows laid out %.0f", steady, sidebar, both, warm)
	// Frames that lay nothing out allocate twice on every platform, however
	// many cores draw them: drawing on several cores once allocated for each
	// area (raster's team).
	if steady > 2 || warm > 2 {
		t.Errorf("a steady frame allocates %.0f times, and one scrolling within rows laid out %.0f, not 2", steady, warm)
	}
}
