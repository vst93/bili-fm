package ui

// Split lays out first and second side by side, with a divider between
// them that the user drags to resize them, or moves with the arrows once
// it has the keyboard focus: *size is the width of first, which the
// divider keeps 40 DIPs from either edge. Changed reports a move.
//
//	ui.Split(c, &app.sidebar, app.files, app.editor).Fill()
func coreSplit(c *context, size *float32, first, second func()) *node {
	return split(c, size, first, second, false)
}

// SplitVertical is Split with first above second, *size its height.
func coreSplitVertical(c *context, size *float32, first, second func()) *node {
	return split(c, size, first, second, true)
}

func split(c *context, size *float32, first, second func(), vertical bool) *node {
	t := c.theme
	minPane, grip := t.Space(10), t.Space(1.5)
	// One element or the other: creating one adds it to the parent.
	var e *node
	if vertical {
		e = coreColumn(c).AlignItems(Stretch)
	} else {
		e = coreRow(c).AlignItems(Stretch)
	}
	e.widget = "Split"
	e.Children(func() {
		// The panes meet at a line one DIP thick, with nothing between them
		// and it. The handle the pointer drags spans a few DIPs over either
		// pane, above them: it is placed in this box, which has no padding,
		// whatever the split's.
		var in *node
		if vertical {
			in = coreColumn(c).Grow(1).MinHeight(0).AlignItems(Stretch)
		} else {
			in = coreRow(c).Grow(1).MinWidth(0).AlignItems(Stretch)
		}
		// The room to share, as the last frame laid it out.
		total := in.st.w
		if vertical {
			total = in.st.h
		}
		set := func(v float32) {
			if total > 0 {
				v = min(v, total-1-minPane)
			}
			v = max(v, minPane)
			if v != *size {
				*size = v
				e.st.markChanged()
				c.rt.consumed = true
			}
		}
		in.Children(func() {
			a := coreBox(c).Shrink(0).Clip()
			if vertical {
				a.Height(*size)
			} else {
				a.Width(*size)
			}
			a.Children(first)

			// The line takes the focus between the panes, in their order.
			div := coreBox(c).Shrink(0).Focusable().Role(RoleSplitter).Label("Divider")
			div.widget = "Divider"
			div.flags |= flagOwnRing
			if vertical {
				div.Height(1)
			} else {
				div.Width(1)
			}
			back, forth := KeyLeft, KeyRight
			if vertical {
				back, forth = KeyUp, KeyDown
			}
			div.afterInput(func() {
				if div.Shortcut(0, back) {
					set(*size - t.Space(2.5))
				}
				if div.Shortcut(0, forth) {
					set(*size + t.Space(2.5))
				}
			})
			div.hasRange, div.accRange = true, [3]float64{float64(minPane), float64(max(total-1-minPane, minPane)), float64(*size)}
			div.accStep = float64(t.Space(2.5))

			b := coreBox(c).Grow(1).Shrink(1).Clip()
			if vertical {
				b.MinHeight(0)
			} else {
				b.MinWidth(0)
			}
			b.Children(second)

			handle := coreBox(c).Absolute()
			handle.flags |= flagDraggable | flagHover
			if vertical {
				handle.Left(0).Right(0).Top(*size - (grip-1)/2).Height(grip).Cursor(CursorResizeNS)
			} else {
				handle.Top(0).Bottom(0).Left(*size - (grip-1)/2).Width(grip).Cursor(CursorResizeEW)
			}
			// A press there focuses the line, as a press on it would.
			dx, dy, held := handle.Dragged()
			handle.afterInput(func() {
				if held {
					div.Focus()
					if vertical {
						set(*size + dy)
					} else {
						set(*size + dx)
					}
				}
			})
			div.Draw(func(p *Painter, r Rect) {
				line := t.Border
				if held || handle.Hovered() {
					line = t.Accent
				}
				p.Fill(r, line, 0)
			})
			handle.Draw(func(p *Painter, r Rect) {
				if div.FocusVisible() {
					p.FocusRing(r, [4]float32{})
				}
			})
		})
	})
	return e
}
