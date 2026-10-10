package ui

import "testing"

// An optional element can be nil, or still point at a slot cleared when
// a loading or empty view replaced it. Both must be inert input scopes.
func TestOptionalElementInput(t *testing.T) {
	show := true
	var removed *node
	tt := coreNewTester(func(c *context) {
		coreText(c, "Keep the window alive")
		if show {
			removed = coreButton(c, "Temporary").AutoFocus()
			removed.Shortcut(Cmd, KeyK)
		}
	}, 200, 100)
	show = false
	tt.Frame()
	if removed == nil || removed.c != nil || removed.st != nil {
		t.Fatal("the removed element's arena slot was not cleared")
	}
	focus, regs, reveal := tt.rt.focused, len(tt.rt.nextRegs), len(tt.rt.c.reveal)
	for name, e := range map[string]*node{"nil": nil, "zero": {}, "cleared": removed} {
		t.Run(name, func(t *testing.T) {
			if e.Clicked() || e.DoubleClicked() || e.RightClicked() || e.Hovered() || e.Pressed() ||
				e.Focused() || e.FocusVisible() || e.FocusWithin() || e.Changed() || e.Submitted() ||
				e.Shortcut(Cmd, KeyK) || e.OverlayShortcut(0, KeyEscape) || e.PressedOutside() {
				t.Fatal("an absent element reported input")
			}
			if e.Clicks() != 0 || e.ClickModifiers() != 0 || e.Bounds() != (Rect{}) {
				t.Fatal("an absent element returned input or bounds")
			}
			if x, y, over := e.PointerPosition(); x != 0 || y != 0 || over {
				t.Fatal("an absent element tracked the pointer")
			}
			if x, y, ok := e.Dragged(); x != 0 || y != 0 || ok {
				t.Fatal("an absent element reported a drag")
			}
			if e.Focus() != e || e.AutoFocus() != e || e.ScrollIntoView() != e {
				t.Fatal("an absent element's action changed its receiver")
			}
		})
	}
	if tt.rt.focused != focus || len(tt.rt.nextRegs) != regs || len(tt.rt.c.reveal) != reveal {
		t.Fatal("an absent element took focus, registered shortcuts or asked to scroll")
	}
}

// ListState must resolve this build, even when the old owner is non-nil
// and now names a different control in the same arena slot.
func TestListStateFocusAndShortcutsFollowCurrentBuild(t *testing.T) {
	var s ListState
	show, focusRequested := true, true
	listKeys, windowKeys, rebuilds := 0, 0, 0
	var owner, other *node
	rowFocused := false
	tt := coreNewTester(func(c *context) {
		if s.coreFocused(c) || s.coreFocusWithin(c) || s.coreFocus(c) || s.coreShortcut(c, Cmd, KeyK) {
			t.Fatal("the previous build's list was available before this one was built")
		}
		if show {
			owner = coreList(c, &s, 1, func(i int) {
				rowFocused = s.coreFocusWithin(c)
				coreText(c, "File").Height(24)
			}).Grow(1).Label("Files")
			if focusRequested && s.coreFocus(c) {
				focusRequested = false
			}
			if coreButton(c, "Hide").Clicked() {
				show = false
			}
		} else {
			other = coreButton(c, "Other").Focus()
			if s.frame.frame == c.rt.frame && s.frame.pass != c.rt.pass {
				rebuilds++
			}
			if s.coreFocused(c) || s.coreFocusWithin(c) || s.coreFocus(c) {
				t.Fatal("the hidden list acted on its cleared or reused owner")
			}
		}
		if s.coreShortcut(c, Cmd, KeyK) {
			listKeys++
		}
		if c.Shortcut(Cmd, KeyK) {
			windowKeys++
		}
	}, 200, 150)
	tt.Frame()
	if focusRequested || !rowFocused || !tt.Focused("Files") {
		t.Fatal("the list did not take focus or expose it to its row builder")
	}
	tt.Key(Cmd, KeyK)
	if listKeys != 1 || windowKeys != 0 {
		t.Fatalf("the focused list's shortcut: list %d, window %d", listKeys, windowKeys)
	}
	if err := tt.Click("Hide"); err != nil {
		t.Fatal(err)
	}
	if show || rebuilds == 0 || owner != other || owner.c == nil {
		t.Fatal("hiding the list did not rebuild and reuse its owner for another control")
	}
	focusRequested = true
	tt.Frame()
	tt.Frame()
	tt.Key(Cmd, KeyK)
	if !focusRequested || !tt.Focused("Other") || listKeys != 1 || windowKeys != 1 {
		t.Fatalf("the hidden list took input or focus: pending %v, list %d, window %d", focusRequested, listKeys, windowKeys)
	}
	show = true
	tt.Frame()
	tt.Frame()
	tt.Key(Cmd, KeyK)
	if focusRequested || !rowFocused || !tt.Focused("Files") || listKeys != 2 || windowKeys != 1 {
		t.Fatalf("the returning list did not regain focus and shortcuts: pending %v, list %d, window %d", focusRequested, listKeys, windowKeys)
	}
}

// Table's inner scroll element is not its focus owner. Resolve the owner
// before row building so List, Table and Outline share the API.
func TestListStateFocusDuringRowBuild(t *testing.T) {
	for _, name := range []string{"list", "table", "outline"} {
		t.Run(name, func(t *testing.T) {
			var list ListState
			var outline OutlineState[string]
			s := &list
			if name == "outline" {
				s = &outline.List
			}
			var id uint64
			keys := 0
			nested, within := false, false
			text := ""
			tt := coreNewTester(func(c *context) {
				row := func() {
					if !s.coreFocus(c) || !s.coreFocused(c) || !s.coreFocusWithin(c) {
						t.Fatal("the focus owner was unavailable during row building")
					}
					if nested {
						coreTextInput(c, &text).Focus().Label("Field")
						if s.coreFocused(c) {
							t.Fatal("the list did not distinguish its own focus from its child's")
						}
					} else {
						coreText(c, "Row")
					}
				}
				var e *node
				switch name {
				case "list":
					e = coreList(c, s, 1, func(int) { row() })
				case "table":
					e = coreTable(c, s, []TableColumn{{Title: "Name"}}, 1, func(int, int) { row() })
				case "outline":
					e = coreOutline(c, &outline, []string{"file"}, func(string) []string { return nil }, func(string) { row() })
				}
				e.Grow(1)
				id = e.ID()
				within = s.coreFocusWithin(c)
				if s.coreShortcut(c, Cmd, KeyK) {
					keys++
				}
			}, 200, 150)
			if tt.rt.focused != id {
				t.Fatal("focus went to the scroll element instead of the widget's owner")
			}
			tt.Key(Cmd, KeyK)
			if keys != 1 {
				t.Fatalf("the focus owner's shortcut ran %d times", keys)
			}
			nested = true
			tt.Frame()
			tt.Key(Cmd, KeyK)
			if !tt.Focused("Field") || !within || keys != 2 {
				t.Fatal("a focused descendant did not receive the list's shortcut")
			}
		})
	}
}

func TestListStateInputRequiresItsActiveContext(t *testing.T) {
	var s ListState
	owner := coreNewTester(func(c *context) {
		coreList(c, &s, 1, func(int) { coreText(c, "File") }).Grow(1)
		s.coreFocus(c)
	}, 200, 100)
	check := func(c *context, s *ListState) {
		t.Helper()
		if s.coreFocused(c) || s.coreFocusWithin(c) || s.coreFocus(c) || s.coreShortcut(c, Cmd, KeyK) {
			t.Fatal("an absent list or inactive context was used")
		}
	}
	check(nil, &s)
	check(&owner.rt.c, &s) // outside the view
	coreNewTester(func(c *context) {
		check(c, nil)
		check(c, &ListState{})
		check(c, &s) // another window, with the same frame and pass numbers
	}, 200, 100)
	owner.rt.close()
	check(&owner.rt.c, &s)
}
