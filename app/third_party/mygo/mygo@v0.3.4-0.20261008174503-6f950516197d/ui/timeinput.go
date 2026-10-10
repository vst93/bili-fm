package ui

import (
	"fmt"
	"strconv"
	"time"
)

// TimeInput creates a field of the time of day of *tm, as 15:04, whose
// hours and minutes each take the focus, as the segments of AppKit's date
// picker: Up and Down step the one with the focus round the clock, digits
// typed set it, going on to the minutes once the hours are typed, and Left
// and Right move between them. Changed reports a new time, which keeps the
// date and the location of *tm. Assistive technology sees two spin
// buttons, named after the field's Label and "hours" and "minutes".
//
//	ui.TimeInput(c, &app.alarm).Label("Alarm")
func coreTimeInput(c *context, tm *time.Time) *node {
	t := c.theme
	f := coreRow(c).AlignItems(Center).Padding(t.Space(1.5), t.Space(2)).Radius(t.Radius).Background(t.Surface).Border(1, t.Border).Shrink(0).Role(RoleGroup)
	f.widget = "TimeInput"
	set := func(h, m int) {
		y, mo, d := tm.Date()
		next := time.Date(y, mo, d, h, m, tm.Second(), tm.Nanosecond(), tm.Location())
		if !next.Equal(*tm) {
			*tm = next
			f.st.markChanged()
			c.rt.consumed = true
		}
	}
	var segs [2]*node
	f.Children(func() {
		for k, name := range []string{"hours", "minutes"} {
			if k == 1 {
				coreText(c, ":").TextColor(t.TextMuted).Padding(0, 1).Role(RoleNone)
			}
			v := tm.Hour()
			if k == 1 {
				v = tm.Minute()
			}
			seg := coreBox(c).Padding(0, t.Space(0.5)).Radius(t.Space(1)).Focusable().FocusRing(false).Role(RoleStepper)
			seg.flags |= flagTypeSelect
			seg.label, seg.nameFrom, seg.nameJoin = name, f, true
			seg.hasRange, seg.accRange, seg.accStep = true, [3]float64{0, float64([]int{23, 59}[k]), float64(v)}, 1
			if seg.Focused() {
				seg.Background(t.Accent).TextColor(t.AccentText)
			}
			seg.Children(func() { coreText(c, fmt.Sprintf("%02d", v)).FontFeatures("tnum") })
			segs[k] = seg
		}
	})
	f.afterInput(func() {
		h, m := tm.Hour(), tm.Minute()
		for k, seg := range segs {
			top := []int{24, 60}[k]
			value := []*int{&h, &m}[k]
			step := 0
			switch {
			case seg.Shortcut(0, KeyUp):
				step = 1
			case seg.Shortcut(0, KeyDown):
				step = -1
			case seg.Shortcut(0, KeyRight) && k == 0:
				segs[1].Focus()
				c.rt.focusVisible = true
			case seg.Shortcut(0, KeyLeft) && k == 1:
				segs[0].Focus()
				c.rt.focusVisible = true
			}
			if step != 0 {
				*value = (*value + step + top) % top
				set(h, m)
			}
			// Digits typed: the first sets the segment, a second makes two
			// digits of them, or starts anew past the top; both typed, or one
			// that can't begin two, go on to the minutes.
			if s := seg.st; s.typing && s.typed != "" {
				typed := s.typed
				n, err := strconv.Atoi(typed)
				if err != nil || len(typed) > 2 || n >= top {
					typed = typed[len(typed)-1:]
					n, err = strconv.Atoi(typed)
					s.typed = typed
				}
				if err == nil {
					*value = n
					set(h, m)
					if (len(typed) == 2 || n*10 >= top) && k == 0 {
						s.typed = ""
						segs[1].Focus()
					}
				}
			}
		}
	})
	f.styleFn = func(f *node) {
		if segs[0].Focused() || segs[1].Focused() {
			f.borderC = t.Accent
		}
	}
	return f
}
