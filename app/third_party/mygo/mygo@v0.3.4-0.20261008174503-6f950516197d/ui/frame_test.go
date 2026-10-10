package ui

import (
	"fmt"
	"runtime"
	"testing"
	"weak"
)

func TestValueHandleExpiresBeforeStorageReuse(t *testing.T) {
	var old Element
	first := true
	children := 0
	tt := NewTester(func(f *Context) {
		current := Button(f, "Current").Key("current")
		if first {
			old = current
			first = false
			return
		}
		if old.Valid() {
			t.Fatal("old handles became valid in another build")
		}
		f.rt.handleChecks = false
		old.Background(RGB(255, 0, 0)).Focus().Children(func() { children++ })
		if !current.Valid() {
			t.Fatal("the current handle expired")
		}
	}, 200, 100)
	tt.Frame()
	if children != 0 || tt.rt.focused != 0 {
		t.Fatal("an expired handle performed an action")
	}
}

func TestCapturedContextUsesCurrentParent(t *testing.T) {
	var outside, inside Element
	tt := NewTester(func(f *Context) {
		Column(f).Padding(20).Children(func() {
			inside = Text(f, "Inside")
			outside = Text(f, "Outside")
		})
	}, 200, 150)
	nIn, nOut := tt.rt.nodeAt(inside.slot), tt.rt.nodeAt(outside.slot)
	if nIn.parent != nOut.parent || nOut.parent == tt.rt.c.root {
		t.Fatal("captured contexts did not use the active parent")
	}
}

func TestActionsRunAfterBuildOnce(t *testing.T) {
	building := false
	clicks := 0
	tt := NewTester(func(f *Context) {
		building = true
		Button(f, "Increment").Key("increment").OnClick(func() {
			if building {
				t.Fatal("handler ran while the view was being built")
			}
			clicks++
		})
		Textf(f, "Count %d", clicks)
		building = false
	}, 200, 100)
	if err := tt.Click("Increment"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 || !tt.HasText("Count 1") {
		t.Fatalf("clicks %d, text %q", clicks, tt.Texts())
	}
	tt.Frame()
	if clicks != 1 {
		t.Fatal("rebuilding repeated the action")
	}
}

func TestDeferredControlIdentityAndConfiguration(t *testing.T) {
	checked, disabled := false, true
	changes := 0
	tt := NewTester(func(f *Context) {
		Checkbox(f.Key("check"), &checked, "Check").Disabled(disabled).OnChange(func() { changes++ })
	}, 200, 100)
	tt.Click("Check")
	if checked || changes != 0 {
		t.Fatal("disabled control handled input")
	}
	disabled = false
	tt.Frame()
	tt.Click("Check")
	if !checked || changes != 1 {
		t.Fatalf("checked %v, changes %d", checked, changes)
	}
	tt.Frame()
	if !checked || changes != 1 {
		t.Fatal("keyed state did not survive a build")
	}
}

func TestReferenceFocusWaitsForControl(t *testing.T) {
	var ref Ref
	show := false
	ref.RequestFocus()
	tt := NewTester(func(f *Context) {
		if show {
			Button(f, "Target").Key("target").Ref(&ref)
		}
	}, 200, 100)
	if !ref.state.pending || tt.rt.focused != 0 {
		t.Fatal("an absent control took focus")
	}
	show = true
	tt.Frame()
	if !tt.Focused("Target") {
		t.Fatal("the returning control did not receive focus")
	}
	show = false
	tt.Frame()
	if tt.Focused("Target") {
		t.Fatal("a hidden control kept focus")
	}
	ref.RequestFocus()
	tt.rt.close()
	if !ref.state.bindings[0].closed || ref.state.bindings[0].requested {
		t.Fatal("closing a window retained its focus request")
	}
}

func TestDeferredInputOptions(t *testing.T) {
	var ref Ref
	text := "original"
	readOnly := true
	ref.RequestFocus()
	tt := NewTester(func(f *Context) {
		TextInput(f.Key("input"), &text).ReadOnly(readOnly).Placeholder("Type").Ref(&ref).Label("Input")
	}, 200, 100)
	tt.Type("x")
	if text != "original" {
		t.Fatal("read-only configuration was applied after editing")
	}
	readOnly = false
	tt.Frame()
	tt.Type("y")
	if text == "original" {
		t.Fatal("the keyed input did not retain its editable state")
	}
}

func TestValueHandlesDetachClosedWindow(t *testing.T) {
	var e Element
	var c *Context
	tt := NewTester(func(ctx *Context) { c = ctx; e = Text(ctx, "Temporary") }, 100, 50)
	owner := weak.Make(tt.rt)
	tt.rt.close()
	tt = nil
	runtime.GC()
	if owner.Value() != nil {
		t.Fatal("saved handles retained a closed window")
	}
	if e.Valid() || c.Valid() {
		t.Fatal("closed handles are valid")
	}
	runtime.KeepAlive(e)
	runtime.KeepAlive(c)
}

func TestCustomPartsUseCheckedValuesAndKeys(t *testing.T) {
	choice := "one"
	var saved SelectParts[string]
	first := true
	tt := NewTester(func(f *Context) {
		if !first && saved.Trigger.Valid() {
			t.Fatal("old custom parts became live again")
		}
		parts := SelectBase(f.Key("choice"), &choice)
		parts.Trigger.Label("Choice").Padding(6).Children(func() { Text(f, choice) })
		parts.Popup(func(panel Element) {
			panel.Padding(4).Background(f.Theme().Background)
			for _, value := range []string{"one", "two"} {
				parts.Item(value).Padding(6).Children(func() { Text(f, value) })
			}
		})
		saved = parts
		first = false
	}, 200, 150)
	if err := tt.Click("Choice"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("two") {
		t.Fatal("custom select did not build its popup")
	}
	if err := tt.Click("two"); err != nil {
		t.Fatal(err)
	}
	if choice != "two" {
		t.Fatalf("choice %q", choice)
	}
}

func TestPublicRowScopeAndKeyboardActions(t *testing.T) {
	var state ListState
	var selected Selection[string]
	var ref Ref
	keys := 0
	ref.RequestFocus()
	tt := NewTester(func(f *Context) {
		List(f.Key("items"), &state, 3).Ref(&ref).Grow(1).
			ItemKey(func(i int) any { return []string{"a", "b", "c"}[i] }).
			Selection(&selected).
			OnShortcut(Cmd, KeyK, func() { keys++ }).
			Rows(func(row ListRow) { Textf(row.Context, "Row %d", row.Index).Height(24) })
	}, 200, 150)
	tt.Key(Cmd, KeyK)
	if keys != 1 {
		t.Fatalf("list shortcut ran %d times", keys)
	}
	if err := tt.Click("Row 1"); err != nil {
		t.Fatal(err)
	}
	if !selected.Has("b") {
		t.Fatal("typed item selection did not use its key")
	}
}

func TestWindowServicesOutliveBuild(t *testing.T) {
	var services Services
	tt := NewTester(func(f *Context) { services = f.Services(); Text(f, "Window") }, 100, 50)
	services.WriteClipboard("value")
	if services.ReadClipboard() != "value" {
		t.Fatal("services expired with the frame")
	}
	services.Invalidate()
	if !tt.h.requested.Load() {
		t.Fatal("services did not request a frame")
	}
	tt.rt.close()
	if services.ReadClipboard() != "" {
		t.Fatal("services reached a closed window")
	}
}

func TestExpiredHandleDiagnosticsAndSlotReuse(t *testing.T) {
	var old Element
	first := true
	var message any
	tt := NewTester(func(c *Context) {
		current := Text(c, "current")
		if first {
			old = current
			first = false
			return
		}
		if old.slot != current.slot {
			t.Fatal("regression must reuse the node slot")
		}
		func() { defer func() { message = recover() }(); old.Focus() }()
	}, 100, 50)
	tt.Frame()
	if message == nil {
		t.Fatal("Tester did not diagnose an expired handle")
	}
}

func TestHandleQueriesBeforeBindingAndAcrossWindows(t *testing.T) {
	var h Handle
	var a, b *Context
	var before, after bool
	first := NewTester(func(c *Context) { a = c; before = h.Focused(c); Button(c, "A").Bind(&h); after = h.Focused(c) }, 100, 50)
	second := NewTester(func(c *Context) { b = c; Button(c, "B").Bind(&h) }, 100, 50)
	h.Focus(a)
	first.Frame()
	first.Frame()
	if !before || !after || !h.Focused(a) || h.Focused(b) {
		t.Fatal("focus depends on bind order or crossed windows")
	}
	h.Focus(b)
	second.Frame()
	if !h.Focused(a) || !h.Focused(b) {
		t.Fatal("window bindings did not remain independent")
	}
	first.rt.close()
	if h.Focused(a) || !h.Focused(b) {
		t.Fatal("closing one window cancelled another binding")
	}
}

func TestHandleShortcutBeforeBindingDropsHiddenControl(t *testing.T) {
	var h Handle
	show := true
	calls := 0
	h.Focus()
	tt := NewTester(func(c *Context) {
		h.OnShortcut(c, Cmd, KeyK, func() { calls++ })
		if show {
			Button(c, "Files").Bind(&h)
		}
	}, 100, 50)
	tt.Key(Cmd, KeyK)
	if calls != 1 {
		t.Fatal("shortcut before Bind was not handled")
	}
	show = false
	tt.Frame()
	tt.Key(Cmd, KeyK)
	if calls != 1 {
		t.Fatal("hidden control took a shortcut")
	}
	show = true
	tt.Frame()
	h.Focus()
	tt.Frame()
	tt.Key(Cmd, KeyK)
	if calls != 2 {
		t.Fatal("returning control did not take shortcut")
	}
}

func TestFocusBindingDistinguishesRequestedAndActual(t *testing.T) {
	type pane int
	const (
		none pane = iota
		files
		diff
	)
	desired := files
	showDiff := false
	var c *Context
	tt := NewTester(func(ctx *Context) {
		c = ctx
		Column(ctx).Children(func() {
			Button(ctx, "Files").FocusBind(&desired, files)
			if showDiff {
				Button(ctx, "Diff").FocusBind(&desired, diff)
			}
		})
	}, 150, 100)
	if !tt.Focused("Files") || FocusedValue(c, &desired) != files {
		t.Fatal("initial focus binding")
	}
	desired = diff
	tt.Frame()
	if desired != diff || FocusedValue(c, &desired) != files {
		t.Fatal("pending request pretended hidden pane had focus")
	}
	showDiff = true
	tt.Frame()
	if !tt.Focused("Diff") || FocusedValue(c, &desired) != diff {
		t.Fatal("pending request was not fulfilled")
	}
	desired = none
	tt.Frame()
	if FocusedValue(c, &desired) != none {
		t.Fatal("focus binding did not clear")
	}
}

func TestBoundInputRunsAfterAllConfiguration(t *testing.T) {
	checked := false
	disabled := false
	var building bool
	changes := 0
	tt := NewTester(func(c *Context) {
		building = true
		e := Checkbox(c.Key("check"), &checked, "Check")
		e.Children(func() { e.Disabled(disabled) })
		e.OnChange(func() {
			if building {
				t.Fatal("callback in view")
			}
			changes++
		})
		building = false
	}, 150, 100)
	disabled = true
	tt.Click("Check")
	if checked || changes != 0 {
		t.Fatal("configuration after an interaction query was too late")
	}
	disabled = false
	tt.Frame()
	tt.Click("Check")
	if !checked || changes != 1 {
		t.Fatal("input or notice was lost/repeated")
	}
}

func TestChangedCommitsLocalValueBeforeReturning(t *testing.T) {
	viewed := map[string]bool{"file": false}
	disabled, notices, callbacks := false, 0, 0
	tt := NewTester(func(c *Context) {
		value := viewed["file"]
		e := Checkbox(c.Key("file"), &value, "Viewed").Disabled(disabled)
		if e.Changed() {
			viewed["file"] = value
			notices++
			// Reading the response again must not toggle the binding twice.
			if !e.Changed() || value != viewed["file"] {
				t.Fatal("a repeated query changed the value or lost the response")
			}
		}
		e.OnChange(func() { callbacks++ })
		Textf(c, "Viewed: %v", viewed["file"])
	}, 200, 100)
	for i, want := range []bool{true, false} {
		if err := tt.Click("Viewed"); err != nil {
			t.Fatal(err)
		}
		if viewed["file"] != want || notices != i+1 || callbacks != i+1 || !tt.HasText(fmt.Sprintf("Viewed: %v", want)) {
			t.Fatalf("value %v, notices %d, callbacks %d", viewed["file"], notices, callbacks)
		}
		tt.Frame()
		if notices != i+1 || callbacks != i+1 {
			t.Fatal("a new build repeated the edit")
		}
	}
	disabled = true
	tt.Frame()
	tt.Click("Viewed")
	if viewed["file"] || notices != 2 || callbacks != 2 {
		t.Fatal("query ignored fluent Disabled configuration")
	}
}

func TestResponseQueriesCommitLocalText(t *testing.T) {
	for _, search := range []bool{false, true} {
		t.Run(fmt.Sprintf("search=%v", search), func(t *testing.T) {
			draft, sent := "", ""
			readOnly, changes, submits := false, 0, 0
			tt := NewTester(func(c *Context) {
				value := draft
				var input Element
				if search {
					input = SearchField(c.Key("input"), &value)
				} else {
					input = TextInput(c.Key("input"), &value)
				}
				input.Label("Draft")
				if search {
					input.Disabled(readOnly)
				} else {
					input.ReadOnly(readOnly)
				}
				if input.Changed() {
					draft = value
					changes++
				}
				if input.Submitted() {
					sent = value
					submits++
					return
				}
			}, 200, 100)
			tt.Click("Draft")
			tt.Type("Hello")
			if draft != "Hello" || changes != 1 {
				t.Fatalf("draft %q, changes %d", draft, changes)
			}
			tt.Key(0, KeyEnter)
			tt.Frame()
			if sent != "Hello" || changes != 1 || submits != 1 {
				t.Fatalf("sent %q, changes %d, submits %d", sent, changes, submits)
			}
			readOnly = true
			tt.Frame()
			tt.Type("x")
			if draft != "Hello" || changes != 1 {
				t.Fatal("query ignored fluent input configuration")
			}
		})
	}
}

func TestResponseQueriesProcessLaterControls(t *testing.T) {
	values := [2]bool{}
	changes := [2]int{}
	tt := NewTester(func(c *Context) {
		for i, label := range []string{"First", "Second"} {
			value := values[i]
			if Checkbox(c.Key(label), &value, label).Changed() {
				values[i] = value
				changes[i]++
			}
		}
	}, 200, 100)
	tt.Click("Second")
	if values != [2]bool{false, true} || changes != [2]int{0, 1} {
		t.Fatalf("later input was skipped or replayed: %v, %v", values, changes)
	}
	tt.Click("First")
	if values != [2]bool{true, true} || changes != [2]int{1, 1} {
		t.Fatalf("input was skipped or replayed: %v, %v", values, changes)
	}
}

func TestChangedCommitsLocalCompositeValues(t *testing.T) {
	choice, number, changes := 0, 1.0, 0
	tt := NewTester(func(c *Context) {
		selected := choice
		if Segmented(c.Key("choice"), &selected, "List", "Grid").Changed() {
			choice = selected
			changes++
		}
		value := number
		if NumberInput(c.Key("number"), &value, 0, 10, 1).Changed() {
			number = value
			changes++
		}
	}, 300, 150)
	tt.Click("Grid")
	if choice != 1 || changes != 1 {
		t.Fatalf("segment: choice %d, changes %d", choice, changes)
	}
	tt.Click("Increase")
	if number != 2 || changes != 2 {
		t.Fatalf("number: value %v, changes %d", number, changes)
	}
	tt.Frame()
	if changes != 2 {
		t.Fatal("composite changes repeated on rebuild")
	}
}

func TestChangedAppliesSliderOptionsBeforeInput(t *testing.T) {
	volume, changes := 0.0, 0
	tt := NewTester(func(c *Context) {
		value := volume
		if Slider(c.Key("volume"), &value, 0, 1).Step(0.25).Label("Volume").Changed() {
			volume = value
			changes++
		}
	}, 300, 100)
	tt.Key(0, KeyTab)
	tt.Key(0, KeyRight)
	if volume != 0.25 || changes != 1 {
		t.Fatalf("value %v, changes %d", volume, changes)
	}
	tt.Frame()
	if volume != 0.25 || changes != 1 {
		t.Fatal("slider input repeated on rebuild")
	}
}

func TestChangeActionsCommitDerivedValuesBeforeRebuild(t *testing.T) {
	checked, disabled, building := false, false, false
	changes, observers, notices := 0, 0, 0
	tt := NewTester(func(c *Context) {
		building = true
		value := checked
		e := Checkbox(c.Key("check"), &value, "Check").Disabled(disabled)
		if e.Changed() {
			notices++
		}
		e.OnChange(func() {
			if building {
				t.Fatal("callback ran during construction")
			}
			checked = value
			changes++
		}).OnChange(func() { observers++ })
		Textf(c, "Checked: %v", checked)
		building = false
	}, 200, 100)
	for _, want := range []bool{true, false} {
		if err := tt.Click("Check"); err != nil {
			t.Fatal(err)
		}
		if checked != want || !tt.HasText(fmt.Sprintf("Checked: %v", want)) {
			t.Fatalf("derived value was lost: checked %v, want %v", checked, want)
		}
	}
	if changes != 2 || observers != 2 || notices != 2 {
		t.Fatalf("changes %d, observers %d, polling notices %d", changes, observers, notices)
	}
	disabled = true
	tt.Frame()
	tt.Click("Check")
	if checked || changes != 2 || observers != 2 || notices != 2 {
		t.Fatal("a rebuild or disabled click repeated the action")
	}
}

func TestNoticeActionsCommitDerivedText(t *testing.T) {
	draft, sent := "", ""
	changes, submissions, notices := 0, 0, 0
	tt := NewTester(func(c *Context) {
		value := draft
		input := TextInput(c, &value).Label("Draft")
		if input.Submitted() {
			notices++
		}
		input.OnChange(func() {
			draft = value
			changes++
		}).OnSubmit(func() {
			sent, draft = value, ""
			submissions++
		})
	}, 200, 100)
	if err := tt.Click("Draft"); err != nil {
		t.Fatal(err)
	}
	tt.Type("Hello")
	if draft != "Hello" || changes != 1 {
		t.Fatalf("draft %q, changes %d", draft, changes)
	}
	tt.Key(0, KeyEnter)
	tt.Frame()
	if sent != "Hello" || draft != "" || changes != 1 || submissions != 1 || notices != 1 {
		t.Fatalf("sent %q, draft %q, changes %d, submissions %d, polling notices %d",
			sent, draft, changes, submissions, notices)
	}
}

func TestGenerationWrapRetiresOwner(t *testing.T) {
	var old, current Element
	tt := NewTester(func(c *Context) { current = Text(c, "current") }, 100, 50)
	old = current
	tt.rt.epoch = uint64(1)<<32 - 1
	tt.Frame()
	if old.owner == current.owner || old.Valid() {
		t.Fatal("generation wrap allowed an old alias")
	}
}

func TestHandleBoundsBetweenBuilds(t *testing.T) {
	var h Handle
	var c *Context
	show := true
	tt := NewTester(func(ctx *Context) {
		c = ctx
		if show {
			Box(ctx).Size(80, 40).Bind(&h)
		}
	}, 100, 60)
	if b := h.Bounds(c); b.W != 80 || b.H != 40 {
		t.Fatalf("bounds %+v", b)
	}
	show = false
	tt.Frame()
	if b := h.Bounds(c); b != (Rect{}) {
		t.Fatalf("hidden bounds %+v", b)
	}
	tt.rt.close()
	if b := h.Bounds(c); b != (Rect{}) {
		t.Fatalf("closed bounds %+v", b)
	}
}

// TestCloseWhileBuilding closes the window from its view and from a click's
// callback, as a close button does: Window.Close destroys it at once on
// Windows. The frame ends there, without building the view again for the
// click.
func TestCloseWhileBuilding(t *testing.T) {
	for _, callback := range []bool{false, true} {
		var tt *Tester
		closed := false
		close := func() {
			tt.rt.close()
			closed = true
		}
		tt = NewTester(func(f *Context) {
			if closed {
				t.Fatalf("callback %v: the view was built in a closed window", callback)
			}
			if f.Theme() == nil {
				t.Fatal("no theme")
			}
			b := Button(f, "Close")
			if callback {
				b.OnClick(close)
			} else if b.Clicked() {
				close()
			}
		}, 200, 100)
		if err := tt.Click("Close"); err != nil {
			t.Fatal(err)
		}
		if !closed {
			t.Fatalf("callback %v: the button was not clicked", callback)
		}
	}
}
