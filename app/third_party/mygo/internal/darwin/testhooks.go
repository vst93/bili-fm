//go:build darwin

package darwin

import (
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo/internal/platform"
	"github.com/egoist/mygo/transfer"
)

// The functions in this file drive native UI the way a user would, for the
// GUI tests in internal/e2e. They must run on the main thread.

// TestPerformMenuItem activates an item of the application menu, found by
// the titles along its path, e.g. TestPerformMenuItem("File", "New").
func TestPerformMenuItem(path ...string) error {
	var err error
	withPool(func() {
		menu := send(theBackend.app, "mainMenu")
		for i, title := range path {
			if menu == 0 {
				err = fmt.Errorf("mygo: menu %q has no submenu", path[i-1])
				return
			}
			index := sendInt(menu, "indexOfItemWithTitle:", uintptr(nsString(title)))
			if index < 0 {
				err = fmt.Errorf("mygo: no menu item %q", title)
				return
			}
			if i == len(path)-1 {
				send(menu, "performActionForItemAtIndex:", uintptr(index))
				return
			}
			menu = send(send(menu, "itemAtIndex:", uintptr(index)), "submenu")
		}
	})
	return err
}

// TestPerformKeyEquivalent sends a key press with the Command key (and
// shift when requested) to the application menu and reports whether an item
// handled it.
func TestPerformKeyEquivalent(key string, shift bool) bool {
	handled := false
	withPool(func() {
		flags := uint(1 << 20)
		if shift {
			flags |= 1 << 17
		}
		ev := msgKeyEvent(class("NSEvent"), sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
			10 /* NSEventTypeKeyDown */, NSPoint{}, flags, 0, 0, 0, nsString(key), nsString(key), false, 0)
		handled = sendBool(send(theBackend.app, "mainMenu"), "performKeyEquivalent:", uintptr(ev))
	})
	return handled
}

// TestEndSheet ends the sheet attached to a window as if its first button
// was clicked, and reports whether there was one.
func TestEndSheet(handle uintptr) bool {
	sheet := send(id(handle), "attachedSheet")
	if sheet == 0 {
		return false
	}
	send(id(handle), "endSheet:returnCode:", uintptr(sheet), nsAlertFirstButtonReturn)
	return true
}

// TestClick sends a left mouse down and up at a point of a window's content
// (top-left origin, in points), like a user click.
func TestClick(handle uintptr, x, y float64) {
	withPool(func() {
		win := id(handle)
		content := msgRect(send(win, "contentView"), sel("frame"))
		loc := NSPoint{x, content.Size.Height - y}
		number := sendInt(win, "windowNumber")
		for _, typ := range []uint{1, 2} { // NSEventTypeLeftMouseDown, LeftMouseUp
			ev := msgMouseEvent(class("NSEvent"), sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				typ, loc, 0, 0, number, 0, 0, 1, 1)
			send(win, "sendEvent:", uintptr(ev))
		}
	})
}

// TestDrag presses the primary button at the first of points, in points
// from the top-left corner of a window's content, drags the pointer through
// the others, and releases it at the last.
func TestDrag(handle uintptr, points [][2]float64) {
	withPool(func() {
		win := id(handle)
		content := msgRect(send(win, "contentView"), sel("frame"))
		number := sendInt(win, "windowNumber")
		for i, p := range points {
			typ := uint(6) // NSEventTypeLeftMouseDragged
			switch i {
			case 0:
				typ = 1 // NSEventTypeLeftMouseDown
			case len(points) - 1:
				typ = 2 // NSEventTypeLeftMouseUp
			}
			ev := msgMouseEvent(class("NSEvent"), sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				typ, NSPoint{p[0], content.Size.Height - p[1]}, 0, 0, number, 0, 0, 1, 1)
			send(win, "sendEvent:", uintptr(ev))
		}
	})
}

// TestControlClick Control-clicks (x, y), in points from the top-left
// corner of a window's content.
func TestControlClick(handle uintptr, x, y float64) {
	withPool(func() {
		win := id(handle)
		content := msgRect(send(win, "contentView"), sel("frame"))
		loc := NSPoint{x, content.Size.Height - y}
		number := sendInt(win, "windowNumber")
		for _, typ := range []uint{1, 2} { // NSEventTypeLeftMouseDown, LeftMouseUp
			ev := msgMouseEvent(class("NSEvent"), sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				typ, loc, 1<<18, 0, number, 0, 0, 1, 1) // NSEventModifierFlagControl
			send(win, "sendEvent:", uintptr(ev))
		}
	})
}

var (
	testSideOnce sync.Once
	// cgEventSetIntegerValueField is CGEventSetIntegerValueField.
	cgEventSetIntegerValueField func(event uintptr, field uint32, value int64)
)

// TestSideButton presses and releases a mouse's back or forward button
// (buttons 3 and 4) in a window showing native UI.
func TestSideButton(handle uintptr, back bool) bool {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return false
	}
	testSideOnce.Do(func() {
		purego.RegisterLibFunc(&cgEventSetIntegerValueField, libCG, "CGEventSetIntegerValueField")
	})
	button := int64(4)
	if back {
		button = 3
	}
	withPool(func() {
		number := sendInt(w.win, "windowNumber")
		for _, typ := range []uint{25, 26} { // NSEventTypeOtherMouseDown, OtherMouseUp
			ev := msgMouseEvent(class("NSEvent"), sel("mouseEventWithType:location:modifierFlags:timestamp:windowNumber:context:eventNumber:clickCount:pressure:"),
				typ, NSPoint{10, 10}, 0, 0, number, 0, 0, 1, 1)
			// NSEvent makes other buttons the middle one: number it.
			cg := send(ev, "CGEvent")
			cgEventSetIntegerValueField(uintptr(cg), 3, button) // kCGMouseEventButtonNumber
			ev = send(class("NSEvent"), "eventWithCGEvent:", uintptr(cg))
			name := "otherMouseDown:"
			if typ == 26 {
				name = "otherMouseUp:"
			}
			send(w.surface.view, name, uintptr(ev))
		}
	})
	return true
}

// TestTrafficLights returns where the top-left corner of a window's close
// button is, in points from the top-left corner of the window.
func TestTrafficLights(handle uintptr) (x, y float64) {
	win := id(handle)
	btn := send(win, "standardWindowButton:", 0)
	f := msgRect(btn, sel("frame"))
	parent := send(btn, "superview")
	a := msgPointFromView(parent, sel("convertPoint:toView:"), f.Origin, 0)
	b := msgPointFromView(parent, sel("convertPoint:toView:"), NSPoint{f.Origin.X + f.Size.Width, f.Origin.Y + f.Size.Height}, 0)
	return math.Min(a.X, b.X), msgRect(win, sel("frame")).Size.Height - math.Max(a.Y, b.Y)
}

// TestWebViewAttached reports whether a window's web view is in its view
// hierarchy.
func TestWebViewAttached(handle uintptr) bool {
	w := theBackend.byNSWindow[id(handle)]
	return w != nil && send(w.web, "window") == w.win
}

// TestDockInspector opens the web inspector of a window docked to it, and
// reports whether the window has one.
func TestDockInspector(handle uintptr) bool {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil {
		return false
	}
	in := w.inspector()
	if in == 0 {
		return false
	}
	send(in, "show")
	send(in, "attach")
	return true
}

// TestInspectorPlace reports where the docked inspector of a window is:
// "content" in its content view, "frame" elsewhere in the window, "" when
// the window has none.
func TestInspectorPlace(handle uintptr) string {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil {
		return ""
	}
	var find func(view id) id
	find = func(view id) id {
		if view != w.web && sendBool(view, "isKindOfClass:", uintptr(class("WKWebView"))) {
			return view
		}
		for _, sub := range arrayItems(send(view, "subviews")) {
			if found := find(sub); found != 0 {
				return found
			}
		}
		return 0
	}
	content := send(w.win, "contentView")
	switch in := find(send(content, "superview")); {
	case in == 0:
		return ""
	case sendBool(in, "isDescendantOf:", uintptr(content)):
		return "content"
	default:
		return "frame"
	}
}

// TestSetDroppedFiles makes paths the files of the next drop on a window's
// page, as if they had been dragged there.
func TestSetDroppedFiles(handle uintptr, paths []string) {
	if w := theBackend.byNSWindow[id(handle)]; w != nil {
		w.dropped = paths
	}
}

// TestDockTileImage renders the content of the Dock tile offscreen, as the
// Dock does, to PNG; nil when the tile shows the plain application icon.
func TestDockTileImage() []byte {
	var data []byte
	withPool(func() {
		view := send(send(theBackend.app, "dockTile"), "contentView")
		if view == 0 {
			return
		}
		bounds := msgRect(view, sel("bounds"))
		rep := msgInitRect(view, sel("bitmapImageRepForCachingDisplayInRect:"), bounds)
		msgInitRectID(view, sel("cacheDisplayInRect:toBitmapImageRep:"), bounds, rep)
		data = goBytes(send(rep, "representationUsingType:properties:", 4, uintptr(send(class("NSDictionary"), "dictionary"))))
	})
	return data
}

// TestDockMenu returns the titles of the Dock menu the app delegate
// returns, and clicks the item at click unless it is -1.
func TestDockMenu(click int) []string {
	var titles []string
	withPool(func() {
		menu := send(send(theBackend.app, "delegate"), "applicationDockMenu:", uintptr(theBackend.app))
		if menu == 0 {
			return
		}
		for _, it := range arrayItems(send(menu, "itemArray")) {
			titles = append(titles, goString(send(it, "title")))
		}
		if click >= 0 {
			send(menu, "performActionForItemAtIndex:", uintptr(click))
		}
	})
	return titles
}

// TestFullScreenHidesToolbar reports whether a window asks to hide its
// toolbar with the menu bar when it enters full screen.
func TestFullScreenHidesToolbar(handle uintptr) bool {
	win := id(handle)
	delegate := send(win, "delegate")
	if !respondsTo(delegate, "window:willUseFullScreenPresentationOptions:") {
		return false
	}
	options := send(delegate, "window:willUseFullScreenPresentationOptions:", uintptr(win), presentationFullScreen|presentationAutoHideMenuBar)
	return options&presentationAutoHideToolbar != 0
}

var (
	testInputOnce         sync.Once
	msgSetMarkedText      func(obj id, sel objc.SEL, text id, selected, replacement nsRange)
	msgInsertText         func(obj id, sel objc.SEL, text id, replacement nsRange)
	tisCopyCurrentSource  uintptr
	tisGetSourceProperty  uintptr
	cfEqual               uintptr
	tisPropertySourceType uintptr
	tisTypeKeyboardLayout uintptr
)

func loadTestInput() {
	testInputOnce.Do(func() {
		purego.RegisterFunc(&msgSetMarkedText, msgSendAddr)
		purego.RegisterFunc(&msgInsertText, msgSendAddr)
		tisCopyCurrentSource, _ = purego.Dlsym(libCarbon, "TISCopyCurrentKeyboardInputSource")
		tisGetSourceProperty, _ = purego.Dlsym(libCarbon, "TISGetInputSourceProperty")
		cfEqual, _ = purego.Dlsym(libCF, "CFEqual")
		for _, c := range []struct {
			name string
			to   *uintptr
		}{{"kTISPropertyInputSourceType", &tisPropertySourceType}, {"kTISTypeKeyboardLayout", &tisTypeKeyboardLayout}} {
			if p, err := purego.Dlsym(libCarbon, c.name); err == nil {
				*c.to = **(**uintptr)(unsafe.Pointer(&p))
			}
		}
	})
}

// keyboardLayoutSelected reports whether the input source is a keyboard
// layout, which inserts what keys type, rather than an input method,
// which composes it.
func keyboardLayoutSelected() bool {
	loadTestInput()
	if tisCopyCurrentSource == 0 || tisGetSourceProperty == 0 || tisPropertySourceType == 0 || tisTypeKeyboardLayout == 0 {
		return false
	}
	src, _, _ := purego.SyscallN(tisCopyCurrentSource)
	if src == 0 {
		return false
	}
	defer cfRelease(src)
	typ, _, _ := purego.SyscallN(tisGetSourceProperty, src, tisPropertySourceType)
	eq, _, _ := purego.SyscallN(cfEqual, typ, tisTypeKeyboardLayout)
	return typ != 0 && byte(eq) != 0
}

// TestClickAndType clicks (x, y) in a window showing native UI and types
// text there at once, before the window draws another frame, with key
// events that go through the input method as a keyboard's do. It reports
// false for a window showing a web page, and while the input source is an
// input method, which would compose the keys rather than insert them.
func TestClickAndType(handle uintptr, x, y float64, text string) bool {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil || !keyboardLayoutSelected() {
		return false
	}
	TestClick(handle, x, y)
	withPool(func() {
		number := sendInt(w.win, "windowNumber")
		for _, r := range text {
			chars := nsString(string(r))
			for _, typ := range []uint{10, 11} { // NSEventTypeKeyDown, KeyUp
				ev := msgKeyEvent(class("NSEvent"), sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
					typ, NSPoint{}, 0, 0, number, 0, chars, chars, false, 0)
				send(w.win, "sendEvent:", uintptr(ev))
			}
		}
	})
	return true
}

// TestKey presses and releases the key of virtual key code code, typing
// chars, in a window showing native UI, as the keyboard does: the events
// go through the window to its surface, and the input method.
func TestKey(handle uintptr, code uint16, chars string) bool {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return false
	}
	withPool(func() {
		number := sendInt(w.win, "windowNumber")
		s := nsString(chars)
		for _, typ := range []uint{10, 11} { // NSEventTypeKeyDown, KeyUp
			ev := msgKeyEvent(class("NSEvent"), sel("keyEventWithType:location:modifierFlags:timestamp:windowNumber:context:characters:charactersIgnoringModifiers:isARepeat:keyCode:"),
				typ, NSPoint{}, 0, 0, number, 0, s, s, false, code)
			send(w.win, "sendEvent:", uintptr(ev))
		}
	})
	return true
}

// TestCompose does what an input method does to a window showing native
// UI: it shows text as the composition, its caret at rune caret, or with
// commit, inserts it. It reports false for a window showing a web page.
func TestCompose(handle uintptr, text string, caret int, commit bool) bool {
	return TestComposeOver(handle, text, caret, commit, -1, 0)
}

// TestComposeOver is TestCompose replacing the length UTF-16 units of the
// input method's document at from, unless from is negative, as macOS's
// press and hold does with the letter it accents.
func TestComposeOver(handle uintptr, text string, caret int, commit bool, from, length int) bool {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return false
	}
	loadTestInput()
	withPool(func() {
		replace := nsRange{Location: nsNotFound}
		if from >= 0 {
			replace = nsRange{Location: uint(from), Length: uint(length)}
		}
		if commit {
			msgInsertText(w.surface.view, sel("insertText:replacementRange:"), nsString(text), replace)
			return
		}
		caretAt := nsRange{Location: uint(utf16Len(string([]rune(text)[:caret])))}
		msgSetMarkedText(w.surface.view, sel("setMarkedText:selectedRange:replacementRange:"), nsString(text), caretAt, replace)
	})
	return true
}

// TestInputClient returns what a window's surface tells input methods: the
// selected range and the whole text of their document.
func TestInputClient(handle uintptr) (selected [2]int, document string) {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return [2]int{-1, 0}, ""
	}
	r := w.surface.selectedRange()
	withPool(func() {
		doc, _, _ := w.surface.document()
		var actual nsRange
		length := uint(units(doc))
		if w.surface.input.Client != nil {
			length = 1<<31 - 1
		}
		document = stringOf(w.surface.substring(nsRange{Length: length}, &actual))
	})
	return [2]int{int(r.Location), int(r.Length)}, document
}

var (
	testDragOnce sync.Once
	testDragInfo id
	testDragAt   NSPoint
	testDragPB   id
	testDragMask uint = 1
)

// TestDropFiles drags files over (x, y), in points from the top-left
// corner of a window's content, and drops them there, as Finder would:
// it calls the surface's dragging methods with an NSDraggingInfo of its
// own. It reports whether the surface took the files over that point and
// whether it took the drop.
func TestDropFiles(handle uintptr, x, y float64, paths []string) (over, dropped bool) {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return false, false
	}
	testDragOnce.Do(func() {
		classDef("MyGoTestDraggingInfo", "NSObject", nil, []objc.MethodDef{
			method("draggingLocation", func(self id, _ objc.SEL) NSPoint { return testDragAt }),
			method("draggingPasteboard", func(self id, _ objc.SEL) id { return testDragPB }),
			method("draggingSourceOperationMask", func(self id, _ objc.SEL) uint { return testDragMask }),
		})
		testDragInfo = send(send(class("MyGoTestDraggingInfo"), "alloc"), "init")
		testDragPB = retain(send(class("NSPasteboard"), "pasteboardWithUniqueName"))
	})
	withPool(func() {
		content := msgRect(send(w.win, "contentView"), sel("frame"))
		testDragMask = 1
		testDragAt = NSPoint{x, content.Size.Height - y}
		send(testDragPB, "clearContents")
		var urls []id
		for _, p := range paths {
			urls = append(urls, send(class("NSURL"), "fileURLWithPath:", uintptr(nsString(p))))
		}
		send(testDragPB, "writeObjects:", uintptr(nsArray(urls...)))
		v := w.surface.view
		over = send(v, "draggingEntered:", uintptr(testDragInfo)) != 0
		over = send(v, "draggingUpdated:", uintptr(testDragInfo)) != 0 && over
		if !over {
			send(v, "draggingExited:", uintptr(testDragInfo))
			return
		}
		dropped = sendBool(v, "prepareForDragOperation:", uintptr(testDragInfo)) && sendBool(v, "performDragOperation:", uintptr(testDragInfo))
	})
	return over, dropped
}

// TestDropData exercises native source pasteboard providers and the
// destination protocol with serialized data (no core session token).
func TestDropData(handle uintptr, x, y float64, data transfer.Data, operations transfer.Operation) (operation transfer.Operation, dropped bool) {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return
	}
	TestDropFiles(handle, -1, -1, nil)
	withPool(func() {
		testDragMask = macOperations(operations)
		content := msgRect(send(w.win, "contentView"), sel("frame"))
		testDragAt = NSPoint{x, content.Size.Height - y}
		src := &macDataSource{r: platform.DragRequest{Data: data.Snapshot()}}
		defer func() {
			send(testDragPB, "clearContents")
			for _, p := range src.providers {
				delete(macDragProviders, p)
				release(p)
			}
		}()
		send(testDragPB, "clearContents")
		send(testDragPB, "writeObjects:", uintptr(nsArray(src.pasteboardItems()...)))
		v := w.surface.view
		operation = goOperations(uint(send(v, "draggingEntered:", uintptr(testDragInfo))))
		if operation != transfer.None {
			dropped = sendBool(v, "performDragOperation:", uintptr(testDragInfo))
		}
		send(v, "draggingExited:", uintptr(testDragInfo))
	})
	return
}

// TestAccessNode is an element of native UI as assistive technology reads
// it through NSAccessibility.
type TestAccessNode struct {
	Role, Subrole, Label, Value string
	// Frame is in points of the screen, its origin at the bottom left.
	Frame    [4]float64
	Focused  bool
	Depth    int
	Children int
	obj      id
}

// TestAccessibility reads the elements of a window showing native UI as
// assistive technology does, depth first.
func TestAccessibility(handle uintptr) []TestAccessNode {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return nil
	}
	var out []TestAccessNode
	withPool(func() {
		var walk func(obj id, depth int)
		walk = func(obj id, depth int) {
			kids := arrayItems(send(obj, "accessibilityChildren"))
			if depth > 0 {
				n := TestAccessNode{
					Role:     goString(send(obj, "accessibilityRole")),
					Subrole:  goString(send(obj, "accessibilitySubrole")),
					Label:    goString(send(obj, "accessibilityLabel")) + goString(send(obj, "accessibilityTitle")),
					Focused:  sendBool(obj, "isAccessibilityFocused"),
					Depth:    depth,
					Children: len(kids),
					obj:      obj,
				}
				if v := send(obj, "accessibilityValue"); v != 0 {
					n.Value = goString(send(v, "description"))
				}
				f := msgRect(obj, sel("accessibilityFrame"))
				n.Frame = [4]float64{f.Origin.X, f.Origin.Y, f.Size.Width, f.Size.Height}
				out = append(out, n)
			}
			for _, k := range kids {
				walk(k, depth+1)
			}
		}
		walk(w.surface.view, 0)
	})
	return out
}

// TestAccessibilityPerform performs action ("press", "increment",
// "decrement", "focus" or "value" with value) on the element of a window
// showing native UI labeled label, as assistive technology does, and
// reports whether there was such an element that allowed it.
func TestAccessibilityPerform(handle uintptr, label, action, value string) bool {
	for _, n := range TestAccessibility(handle) {
		if n.Label != label {
			continue
		}
		ok := false
		withPool(func() {
			selector := map[string]string{"press": "accessibilityPerformPress", "increment": "accessibilityPerformIncrement",
				"decrement": "accessibilityPerformDecrement", "focus": "setAccessibilityFocused:", "value": "setAccessibilityValue:"}[action]
			if !sendBool(n.obj, "isAccessibilitySelectorAllowed:", uintptr(sel(selector))) {
				return
			}
			switch action {
			case "focus":
				send(n.obj, selector, 1)
			case "value":
				send(n.obj, selector, uintptr(nsString(value)))
			default:
				sendBool(n.obj, selector)
			}
			ok = true
		})
		return ok
	}
	return false
}

// TestAccessibilityAttribute reads an attribute of the element labeled
// label through AppKit's older API, as assistive technology does: its
// value, whether it is settable, and whether the element names it.
func TestAccessibilityAttribute(handle uintptr, label, attr string) (value string, settable, named, ok bool) {
	for _, n := range TestAccessibility(handle) {
		if n.Label != label {
			continue
		}
		withPool(func() {
			name := nsString(attr)
			if v := send(n.obj, "accessibilityAttributeValue:", uintptr(name)); v != 0 {
				value = goString(send(v, "description"))
			}
			settable = sendBool(n.obj, "accessibilityIsAttributeSettable:", uintptr(name))
			for _, a := range arrayItems(send(n.obj, "accessibilityAttributeNames")) {
				named = named || goString(a) == attr
			}
		})
		return value, settable, named, true
	}
	return "", false, false, false
}

// TestObserve has key-value observing watch the view of a window showing
// native UI and the elements of it that assistive technology reads, as
// other code may: the runtime gives each a generated subclass of its
// class, whose methods reach the overrides of MyGo's classes. stop ends it.
func TestObserve(handle uintptr) (stop func()) {
	w := theBackend.byNSWindow[id(handle)]
	if w == nil || w.surface == nil {
		return func() {}
	}
	type observed struct {
		obj id
		key string
	}
	list := []observed{{w.surface.view, "frame"}}
	for _, n := range TestAccessibility(handle) {
		list = append(list, observed{n.obj, "accessibilityValue"})
	}
	withPool(func() {
		for _, o := range list {
			// Kept until stop: an object observed must not go away.
			retain(o.obj)
			send(o.obj, "addObserver:forKeyPath:options:context:", uintptr(w.delegate), uintptr(nsString(o.key)), 0, 0)
		}
	})
	return func() {
		withPool(func() {
			for _, o := range list {
				send(o.obj, "removeObserver:forKeyPath:", uintptr(w.delegate), uintptr(nsString(o.key)))
				release(o.obj)
			}
		})
	}
}
