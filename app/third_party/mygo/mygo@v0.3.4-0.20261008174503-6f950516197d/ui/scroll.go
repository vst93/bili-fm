package ui

import "slices"

// ScrollState is how far a scroll container scrolls its content, kept in
// the app's state with TrackScroll.
type ScrollState struct {
	Handle

	// X and Y are how far the content is scrolled left and up, in DIPs.
	// Set them to scroll: the container keeps them within its content, so
	// that 0 shows the start and math.MaxFloat32 the end.
	X, Y float32
	// MaxX and MaxY are as far as X and Y go, as the frame laid the content
	// out: Y is MaxY at the end. The container sets them.
	MaxX, MaxY float32
}

// TrackScroll keeps the scrolling of a scroll container (Scroll,
// ScrollHorizontal, ScrollBoth, List) in *s: the container shows its
// content from s.X and s.Y, and writes there as the user scrolls it. The
// app reads where it is, scrolls it by setting them, and keeps a place
// with a ScrollState of its own for each page the container shows.
//
//	// A log that follows its end, unless the user scrolled up from it.
//	if app.log.Y >= app.log.MaxY {
//		app.log.Y = math.MaxFloat32
//	}
//	ui.Scroll(c).TrackScroll(&app.log).Children(app.lines)
//
// A List keeps its place by its rows instead, and scrolls to a row with
// its ListState.
func (e *node) TrackScroll(s *ScrollState) *node {
	if s != nil {
		wrapElement(e).Bind(&s.Handle)
	}
	e.track = s
	if e.adoptScroll() && e.first != nil {
		// What is built was built for the old place, as List's rows.
		e.c.rt.consumed = true
	}
	return e
}

// ScrollIntoView scrolls the containers around the element as little as
// shows it, once the frame is laid out: an element added in this frame
// comes into view with it. Call it in the frame it should come into view,
// as when it is added or chosen: in every frame, it would keep the user
// from scrolling it away.
func (e *node) ScrollIntoView() *node {
	if !e.hasState() {
		return e
	}
	e.c.reveal = append(e.c.reveal, e)
	return e
}

// adoptScroll takes the offset the app set in the element's ScrollState,
// and reports whether it was a new one.
func (e *node) adoptScroll() bool {
	st, t := e.st, e.track
	st.track = t
	if t == nil || t.X == float32(st.scrollX) && t.Y == float32(st.scrollY) {
		return false
	}
	st.beginMove(e.c.rt.frame)
	if t.X != float32(st.scrollX) {
		st.scrollX = float64(t.X)
	}
	if t.Y != float32(st.scrollY) {
		st.scrollY = float64(t.Y)
	}
	return true
}

// maxScroll returns as far as the element scrolls its content, on the axes
// it scrolls.
func (e *node) maxScroll() (x, y float64) {
	if e.flags&flagScrollX != 0 {
		x = max(0, e.contentW-float64(e.w))
	}
	if e.flags&flagScrollY != 0 {
		y = max(0, e.contentH-float64(e.h))
	}
	return x, y
}

// scrollTo moves the content of a scroll container, and the ScrollState
// tracking it.
func (s *state) scrollTo(x, y float64) {
	s.scrollX, s.scrollY = x, y
	if t := s.track; t != nil {
		t.X, t.Y = float32(x), float32(y)
	}
}

// beginMove notes where the offset was before the frame first moves it,
// other than an event does.
func (s *state) beginMove(frame uint64) {
	if s.moveFrame != frame {
		s.moveFrame, s.startX, s.startY = frame, s.scrollX, s.scrollY
	}
}

// movedIn reports whether frame moved the offset: what was built read the
// old one, and the next frame builds with the new.
func (s *state) movedIn(frame uint64) bool {
	return s.moveFrame == frame && (s.scrollX != s.startX || s.scrollY != s.startY)
}

// reveal scrolls element id into view in the next frame, once it is laid
// out.
func (rt *engine) reveal(id uint64) {
	if !slices.Contains(rt.revealIDs, id) {
		rt.revealIDs = append(rt.revealIDs, id)
	}
	rt.requestFrame()
}

// revealAll scrolls the elements of root with the given ids into view, in
// that order.
func revealAll(root *node, ids []uint64) {
	found := make([]*node, len(ids))
	var walk func(e *node)
	walk = func(e *node) {
		if i := slices.Index(ids, e.id); i >= 0 && found[i] == nil {
			found[i] = e
		}
		for ch := e.first; ch != nil; ch = ch.next {
			walk(ch)
		}
	}
	walk(root)
	for _, e := range found {
		if e != nil {
			reveal(e)
		}
	}
}

// reveal scrolls the containers around e, innermost first, as little as
// shows its box. The layout has sized and arranged the boxes, relative to
// their parents, but not placed them yet.
func reveal(e *node) {
	for e.isInline() {
		e = e.parent
	}
	x, y, w, h := e.x, e.y, e.w, e.h
	frame := e.c.rt.frame
	for ch, p := e, e.parent; p != nil; ch, p = p, p.parent {
		if p.scrolls() && ch.flags&flagAbsolute == 0 {
			p.adoptScroll()
			st := p.st
			mx, my := p.maxScroll()
			// Where the box is in the content: a List placed its rows
			// from scrollBase.
			cx, cy := float64(x), float64(y)+p.scrollBase
			sx := max(0, min(nearest(st.scrollX, cx, w, p.border[3], p.w-p.border[1]), mx))
			sy := max(0, min(nearest(st.scrollY, cy, h, p.border[0], p.h-p.border[2]), my))
			if sx != st.scrollX || sy != st.scrollY {
				st.beginMove(frame)
				if p.list != nil {
					// A List places its rows anew from there, building
					// those it lacked, which moves the row holding the box.
					was := ch.y
					p.relayoutList(ch, sy)
					cy = float64(y+ch.y-was) + p.scrollBase
				} else {
					st.scrollTo(sx, sy)
				}
			}
			x = float32(cx - st.scrollX)
			y = float32(cy - st.scrollY)
		}
		x += p.x
		y += p.y
	}
}

// nearest returns the offset that shows what spans [pos, pos+size) of a
// container's content in its viewport [lo, hi), scrolled by off, moving as
// little as it can: what is larger than the viewport shows from its start.
func nearest(off, pos float64, size, lo, hi float32) float64 {
	start, end := pos-off, pos+float64(size)-off
	switch {
	case start < float64(lo):
		return off - (float64(lo) - start)
	case end > float64(hi):
		return off + min(end-float64(hi), start-float64(lo))
	}
	return off
}
