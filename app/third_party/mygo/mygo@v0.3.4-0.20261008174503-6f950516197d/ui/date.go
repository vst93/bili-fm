package ui

import (
	"fmt"
	"time"
)

// DateInput creates a field showing *date as 2006-01-02, which a calendar
// below it changes. A click, Enter or Space opens the calendar, where a
// click chooses a day, as do the arrows and Enter; Page Up and Page Down,
// or its buttons, move by months, and Escape closes it. Changed reports a
// new date, which keeps the time of day and location of *date.
func coreDateInput(c *context, date *time.Time) *node {
	t := c.theme
	b := coreButton(c, "")
	b.widget, b.role, b.accValue = "DateInput", RolePopUpButton, date.Format("2006-01-02")
	b.MinWidth(t.Space(32.5)).Justify(SpaceBetween)
	open := coreLocal(b, "open", func() bool { return false })
	// cursor is the day the keys move in the calendar.
	cursor := coreLocal(b, "cursor", func() time.Time { return *date })
	b.afterInput(func() {
		if b.Clicked() {
			*open = !*open
			*cursor = *date
		}
		b.expanded = *open
	})
	b.Children(func() {
		coreText(c, date.Format("2006-01-02")).SingleLine().FontFeatures("tnum")
		coreBox(c).Size(t.Space(3.5), t.Space(3.5)).Shrink(0).Draw(func(p *Painter, r Rect) {
			// A calendar page.
			p.Stroke(Rect{r.X + 1, r.Y + 2, r.W - 2, r.H - 3}, t.TextMuted, 2, 1.2)
			p.Fill(Rect{r.X + 1, r.Y + 5, r.W - 2, 1.2}, t.TextMuted, 0)
		})
	})
	choose := func(d time.Time) {
		if setDay(date, d) {
			b.st.markChanged()
			c.rt.consumed = true
		}
		*open = false
		b.Focus()
	}
	// The calendar keeps its own width under a wide field: not Popover,
	// whose panel is as wide as the field at least.
	corePopoverBase(c, b, open, func(panel *node) {
		stylePanel(c, panel)
		calendarGrid(c, date, cursor, false, choose).AutoFocus()
	})
	return b
}

// Calendar creates a month's calendar choosing *date, as SwiftUI's
// graphical date picker: a click chooses a day, as the arrows do while the
// calendar has the focus, Page Up and Page Down or its buttons move by
// months, and Home and End go to the first and the last day of the month.
// Changed reports a new date, which keeps the time of day and location of
// *date. Assistive technology reads the day chosen as the arrows move.
func coreCalendar(c *context, date *time.Time) *node {
	var g *node
	g = calendarGrid(c, date, date, true, func(d time.Time) {
		if setDay(date, d) {
			g.st.markChanged()
			c.rt.consumed = true
		}
	})
	g.widget = "Calendar"
	return g
}

// setDay sets the day of *date to d's, keeping its time of day and its
// location, and reports whether it changed.
func setDay(date *time.Time, d time.Time) bool {
	y, m, day := d.Date()
	h, mi, s := date.Clock()
	next := time.Date(y, m, day, h, mi, s, date.Nanosecond(), date.Location())
	if next.Equal(*date) {
		return false
	}
	*date = next
	return true
}

// sameDay reports whether a and b fall on the same day.
func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// calendarGrid creates the month around *cursor, the day the keys move,
// with *date chosen; choose chooses a day, as a click and Enter do, and
// every move does with moveChooses.
func calendarGrid(c *context, date, cursor *time.Time, moveChooses bool, choose func(time.Time)) *node {
	t := c.theme
	cur := *cursor
	month := time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, time.UTC)
	grid := coreColumn(c).Gap(t.Space(0.5)).Focusable().Shrink(0).Role(RoleTable).Label(month.Format("January 2006"))
	grid.flags |= flagOwnRing
	move := func(d time.Time) {
		// Choosing it moves the cursor where it is the date chosen.
		if moveChooses {
			choose(d)
		} else {
			*cursor = d
		}
		c.rt.consumed = true
	}
	grid.afterInput(func() {
		switch {
		case grid.Shortcut(0, KeyLeft):
			move(cur.AddDate(0, 0, -1))
		case grid.Shortcut(0, KeyRight):
			move(cur.AddDate(0, 0, 1))
		case grid.Shortcut(0, KeyUp):
			move(cur.AddDate(0, 0, -7))
		case grid.Shortcut(0, KeyDown):
			move(cur.AddDate(0, 0, 7))
		case grid.Shortcut(0, KeyPageUp):
			move(cur.AddDate(0, -1, 0))
		case grid.Shortcut(0, KeyPageDown):
			move(cur.AddDate(0, 1, 0))
		case grid.Shortcut(0, KeyHome):
			move(time.Date(cur.Year(), cur.Month(), 1, 0, 0, 0, 0, cur.Location()))
		case grid.Shortcut(0, KeyEnd):
			move(time.Date(cur.Year(), cur.Month()+1, 0, 0, 0, 0, 0, cur.Location()))
		case grid.Shortcut(0, KeyEnter), grid.Shortcut(0, KeySpace):
			choose(cur)
		}
	})
	grid.Children(func() {
		coreRow(c).AlignItems(Center).Gap(t.Space(1)).Children(func() {
			prev := coreButton(c, "‹").Padding(t.Space(0.5), t.Space(2.5)).Label("Previous month")
			prev.flags |= flagClickable
			prev.afterInput(func() {
				if prev.Clicked() {
					move(cur.AddDate(0, -1, 0))
				}
			})
			coreText(c, month.Format("January 2006")).Bold().Grow(1).TextAlign(Center)
			next := coreButton(c, "›").Padding(t.Space(0.5), t.Space(2.5)).Label("Next month")
			next.flags |= flagClickable
			next.afterInput(func() {
				if next.Clicked() {
					move(cur.AddDate(0, 1, 0))
				}
			})
		})
		coreRow(c).Children(func() {
			for _, d := range []string{"Mo", "Tu", "We", "Th", "Fr", "Sa", "Su"} {
				coreText(c, d).Width(t.Space(8)).TextAlign(Center).FontSize(t.FontSize - 2).TextColor(t.TextMuted).Role(RoleColumnHeader)
			}
		})
		// Six weeks from the Monday on or before the first.
		start := month.AddDate(0, 0, -((int(month.Weekday()) + 6) % 7))
		today := c.now
		for w := range 6 {
			coreRow(c).Role(RoleRow).Children(func() {
				for d := range 7 {
					day := start.AddDate(0, 0, w*7+d)
					cell := coreBox(c).Size(t.Space(8), t.Space(7)).Center().Radius(t.Radius).Role(RoleButton).Label(day.Format("January 2, 2006"))
					cell.flags |= flagClickable | flagHover
					cell.afterInput(func() {
						if cell.Clicked() {
							choose(day)
							if cursor != date {
								*cursor = day
							}
						}
					})
					chosen, atCursor := sameDay(day, *date), sameDay(day, cur)
					fg := t.Text
					if day.Month() != month.Month() {
						fg = t.TextMuted
					}
					switch {
					case chosen:
						cell.Background(t.Accent)
						fg = t.AccentText
					case atCursor:
						cell.Background(t.SurfaceHover)
					}
					if sameDay(day, today) && !chosen {
						cell.Border(1, t.Accent)
					}
					cell.styleFn = func(cell *node) {
						if !chosen && cell.Hovered() {
							cell.bg = t.SurfaceHover
						}
					}
					cell.checked = 1 + int8(b2f(chosen))
					if atCursor {
						// Assistive technology reads the day the keys are on.
						grid.activeDescendant = cell
					}
					cell.Children(func() {
						coreText(c, fmt.Sprint(day.Day())).TextColor(fg).FontFeatures("tnum")
					})
				}
			})
		}
	})
	grid.DrawOver(func(p *Painter, r Rect) {
		if grid.FocusVisible() {
			p.FocusRing(r, [4]float32{t.Radius, t.Radius, t.Radius, t.Radius})
		}
	})
	return grid
}
