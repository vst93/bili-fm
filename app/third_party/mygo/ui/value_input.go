package ui

// Input bindings are retained by ID. Each build updates their app pointers and
// settings, then response queries or the end of construction apply input.
// The function and queue are reused; steady frames do not allocate for them.
func valueBinding[T any](n *node) *T {
	p, ok := n.st.valueBinding.(*T)
	if !ok {
		p = new(T)
		n.st.valueBinding = p
	}
	return p
}

func (n *node) onValueInput(fn func(*node)) {
	if n.valueInput == nil {
		n.c.rt.inputs = append(n.c.rt.inputs, n)
	}
	n.valueInput = fn
}

func (rt *engine) beginInputs() {
	rt.inputAt, rt.afterInputAt = 0, 0
	// A rebuild must not report the same edit on a fresh local binding.
	// Keep typed input until this pass's composite controls have handled it.
	for _, s := range rt.notices {
		if s.typing {
			rt.typedInputs = append(rt.typedInputs, s)
		}
		s.changed, s.submitted, s.noticeQueued = false, false, false
		s.changeDelivered, s.submitDelivered = false, false
	}
	clear(rt.notices)
	rt.notices = rt.notices[:0]
}

// applyInputs finalizes the controls built so far. It can run more than once
// as the view queries responses and then constructs additional controls.
// Composite input actions may query their children without reentering it.
func (rt *engine) applyInputs() {
	if rt.applyingInputs {
		return
	}
	rt.applyingInputs = true
	defer func() { rt.applyingInputs = false }()
	for rt.inputAt < len(rt.inputs) {
		n := rt.inputs[rt.inputAt]
		rt.inputAt++
		if n.valueInput != nil {
			if n.st.pendingSubmit {
				n.st.pendingSubmit = false
				if !n.disabled() {
					n.st.markSubmitted()
				}
			}
			n.valueInput(n)
		}
	}
	for rt.afterInputAt < len(rt.afterInputs) {
		a := rt.afterInputs[rt.afterInputAt]
		rt.afterInputAt++
		if a.node != nil && !a.node.disabled() {
			a.fn()
		}
	}
	if len(rt.notices) > 0 {
		rt.consumed = true
	}
}

func (rt *engine) finishInputs() {
	for _, s := range rt.typedInputs {
		s.typing = false
	}
	clear(rt.typedInputs)
	rt.typedInputs = rt.typedInputs[:0]
}

func toggleInput(n *node) {
	if n.Clicked() {
		p := *valueBinding[*bool](n)
		*p = !*p
		n.st.markChanged()
	}
}

type radioValue[T comparable] struct {
	selected *T
	value    T
}

func radioInput[T comparable](n *node) {
	b := valueBinding[radioValue[T]](n)
	if n.Clicked() && *b.selected != b.value {
		*b.selected = b.value
		n.st.markChanged()
	}
}

type sliderValue struct {
	value        *float64
	lo, hi, step float64
}

func sliderInput(n *node) {
	if n.disabled() {
		return
	}
	b := valueBinding[sliderValue](n)
	settings := coreLocal(n, sliderKey{}, func() sliderSettings { return sliderSettings{} })
	step := b.step
	if settings.step > 0 {
		step = settings.step
	}
	set := func(v float64) {
		v = snap(max(b.lo, min(b.hi, v)), b.lo, b.hi, step)
		if *b.value != v {
			*b.value = v
			n.st.markChanged()
			n.c.rt.consumed = true
		}
	}
	st, rt := n.st, n.c.rt
	if st.pressed {
		if settings.vertical && st.ch > 0 {
			set(b.lo + float64(max(0, min(1, 1-(rt.pointerY-st.cy)/st.ch)))*(b.hi-b.lo))
		} else if !settings.vertical && st.cw > 0 {
			set(b.lo + float64(max(0, min(1, (rt.pointerX-st.cx)/st.cw)))*(b.hi-b.lo))
		}
	}
	if step <= 0 {
		step = (b.hi - b.lo) / 100
	}
	switch {
	case n.Shortcut(0, KeyLeft), n.Shortcut(0, KeyDown):
		set(*b.value - step)
	case n.Shortcut(0, KeyRight), n.Shortcut(0, KeyUp):
		set(*b.value + step)
	case n.Shortcut(0, KeyPageDown):
		set(*b.value - max(step, (b.hi-b.lo)/10))
	case n.Shortcut(0, KeyPageUp):
		set(*b.value + max(step, (b.hi-b.lo)/10))
	case n.Shortcut(0, KeyHome):
		set(b.lo)
	case n.Shortcut(0, KeyEnd):
		set(b.hi)
	}
}

func stringInput(n *node) {
	ed, st, rt := n.st.editor, n.st, n.c.rt
	value := ed.value
	if ed.nativeDirty {
		before := ed.nativeValue
		ed.nativeDirty, ed.nativeValue = false, ""
		if *value == before {
			if n.disabled() || ed.readOnly {
				ed.setText(*value)
				ed.compose = ""
			} else if *value != ed.buf.s {
				*value = ed.buf.s
				st.markChanged()
			}
		} else {
			ed.setText(*value)
			ed.compose = ""
		}
	}
	if n.disabled() {
		clear(ed.queue)
		ed.queue = ed.queue[:0]
		ed.compose = ""
		return
	}
	if rt.focused == n.id || len(ed.queue) > 0 {
		version := ed.buf.version
		ed.process(n.c, n)
		if ed.buf.version != version && ed.buf.s != *value {
			*value = ed.buf.s
			st.markChanged()
		}
	}
	if rt.focused != n.id {
		ed.compose = ""
	}
}

func bufferInput(n *node) {
	ed, st, rt := n.st.editor, n.st, n.c.rt
	if ed.bufferDirty && (n.disabled() || ed.readOnly) {
		snapshot, _ := ed.document.compareRestore(ed.published, ed.nativeBufferBefore.root)
		ed.loadBuffer(snapshot)
		ed.bufferDirty = false
	}
	if n.disabled() {
		clear(ed.queue)
		ed.queue = ed.queue[:0]
		ed.compose = ""
		return
	}
	if ed.bufferDirty {
		st.markChanged()
		ed.bufferDirty = false
	}
	if rt.focused == n.id || len(ed.queue) > 0 {
		before := ed.buf.version
		ed.process(n.c, n)
		if ed.buf.version != before {
			ed.client.publish()
			st.markChanged()
			ed.bufferDirty = false
		}
	}
	if rt.focused != n.id {
		ed.compose = ""
	}
}

type listRowInput struct {
	f *listFrame
	i int
}

func listRowValueInput(n *node) {
	b := valueBinding[listRowInput](n)
	f := b.f
	if n.disabled() || f.owner.disabled() {
		return
	}
	if n.Clicked() {
		f.click(b.i, n.ClickModifiers())
		f.owner.Focus()
	}
	if n.DoubleClicked() {
		f.owner.st.markSubmitted()
	}
}
func listInput(n *node) {
	if n.disabled() || n.rowsOf == nil {
		return
	}
	f := n.rowsOf
	if f.s.cursor() != nil {
		f.navigate()
	}
	f.reorder()
}

func (s *state) notice() {
	if s.rt != nil && !s.noticeQueued {
		s.rt.notices = append(s.rt.notices, s)
		s.noticeQueued = true
	}
}
func (s *state) markChanged() {
	s.changed, s.changeDelivered = true, false
	s.notice()
}
func (s *state) markSubmitted() {
	if s.rt != nil && !s.rt.inFrame {
		s.pendingSubmit = true
		return
	}
	s.submitted, s.submitDelivered = true, false
	s.notice()
}
func (s *state) markTyping() { s.typing = true; s.notice() }

// Composite controls schedule small input actions after their children.
// Queries apply them once, or they run after construction if not queried.
type inputAction struct {
	node *node
	fn   func()
}

func (n *node) afterInput(fn func()) {
	n.c.rt.afterInputs = append(n.c.rt.afterInputs, inputAction{node: n, fn: fn})
}
