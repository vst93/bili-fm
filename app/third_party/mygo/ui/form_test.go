package ui

import (
	"math"
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestField(t *testing.T) {
	name, email, font, size := "", "", "Arial", "Small"
	notify, express := false, false
	emailError := "Enter an email address."
	tt := coreNewTester(func(c *context) {
		coreColumn(c).Gap(12).Children(func() {
			coreField(c, "Name", func() { coreTextInput(c, &name) }).Description("As on your passport.")
			coreField(c, "Email", func() { coreTextInput(c, &email) }).Description("For receipts.").Error(emailError)
			coreField(c, "Notifications", func() { coreCheckbox(c, &notify, "Send emails") })
			coreField(c, "Delivery", func() {
				coreRadioGroup(c, func() {
					coreRadio(c, &size, "Small", "Small")
					coreRadio(c, &size, "Large", "Large")
				})
			})
			coreField(c, "Font", func() { coreCombobox(c, &font, fonts) })
			coreField(c, "Express", func() { coreSwitch(c, &express) })
			coreButton(c, "Cut").Tooltip("Cut the selection")
		})
	}, 500, 700)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	// The label names the control, and the texts below describe it, the
	// error first.
	if n := accessNode(t, tree, platform.RoleTextField, "Name"); n.Description != "As on your passport." || n.States&platform.AccessInvalid != 0 {
		t.Errorf("Name: %+v", n)
	}
	if n := accessNode(t, tree, platform.RoleTextField, "Email"); n.Description != emailError+"\nFor receipts." || n.States&platform.AccessInvalid == 0 {
		t.Errorf("Email: %+v", n)
	}
	if !tt.HasText(emailError) || !tt.HasText("For receipts.") {
		t.Errorf("the texts below do not show: %q", tt.Texts())
	}
	// A check box names itself; the field names the group around it.
	accessNode(t, tree, platform.RoleCheckBox, "Send emails")
	accessNode(t, tree, platform.RoleGroup, "Notifications")
	accessNode(t, tree, platform.RoleRadioGroup, "Delivery")
	accessNode(t, tree, platform.RoleComboBox, "Font")
	accessNode(t, tree, platform.RoleSwitch, "Express")
	if n := accessNode(t, tree, platform.RoleButton, "Cut"); n.Description != "Cut the selection" {
		t.Errorf("the tooltip does not describe the button: %+v", n)
	}
	// A click on the label focuses the control, or clicks it.
	tt.Click("Name")
	if !tt.Focused("Name") {
		t.Error("a click on the label did not focus the input")
	}
	tt.Type("Ada")
	if name != "Ada" {
		t.Errorf("typed %q", name)
	}
	// The text, not the group it names.
	var label Rect
	for _, n := range tt.rt.labels {
		if n.text == "Notifications" {
			label = n.r
		}
	}
	tt.ClickAt(label.X+label.W/2, label.Y+label.H/2)
	if !notify {
		t.Error("a click on the label did not check the check box")
	}
	tt.Click("Express")
	if !express {
		t.Error("a click on the label did not turn the switch on")
	}
	// In a group, the one chosen.
	tt.Click("Delivery")
	if !tt.Focused("Small") || size != "Small" {
		t.Errorf("a click on the label of a radio group: focused %v, chose %q", tt.Focused("Small"), size)
	}
}

func TestFieldDisabled(t *testing.T) {
	name, saved := "", 0
	tt := coreNewTester(func(c *context) {
		coreFieldset(c, "Account", func() {
			coreField(c, "Name", func() { coreTextInput(c, &name) })
			if coreButton(c, "Save").Clicked() {
				saved++
			}
		}).Disabled(true)
	}, 400, 300)
	// The fieldset disables what is inside it, though it does so after
	// building it.
	tt.Click("Name")
	if tt.Focused("Name") {
		t.Error("a click on the label focused a disabled input")
	}
	tt.Click("Save")
	if saved != 0 || tt.Focused("Save") {
		t.Errorf("a disabled button: %d clicks, focused %v", saved, tt.Focused("Save"))
	}
	r, _ := tt.Find("Name")
	tt.ClickAt(r.X+r.W/2, r.Y+r.H+20) // the input
	if tt.Focused("Name") {
		t.Error("a click focused a disabled input")
	}
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	if n := accessNode(t, tt.h.access, platform.RoleGroup, "Account"); n.States&platform.AccessDisabled == 0 {
		t.Errorf("the fieldset: %+v", n)
	}
}

func TestFormLayout(t *testing.T) {
	name, email, city := "", "", ""
	notify, express := false, false
	tt := coreNewTester(func(c *context) {
		coreForm(c, func() {
			coreField(c, "Name", func() { coreTextInput(c, &name) })
			coreField(c, "Email address", func() { coreTextInput(c, &email) }).Description("For receipts.")
			coreField(c, "Notifications", func() { coreCheckbox(c, &notify, "Send emails") })
			coreField(c, "Express", func() { coreSwitch(c, &express) })
			coreFieldset(c, "Shipping", func() {
				coreField(c, "City", func() { coreTextInput(c, &city) })
			})
		}).Width(480)
	}, 500, 500)
	tt.send(platform.SurfaceEvent{Kind: platform.AccessibilityOn})
	tree := tt.h.access
	box := func(role platform.AccessRole, name string) platform.RectF {
		return accessNode(t, tree, role, name).Bounds
	}
	find := func(s string) Rect {
		r, ok := tt.Find(s)
		if !ok {
			t.Fatalf("no %q", s)
		}
		return r
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1 }
	// The controls start at one edge, beside the widest label, those of a
	// fieldset too, and the labels end at one.
	nameIn, emailIn, cityIn := box(platform.RoleTextField, "Name"), box(platform.RoleTextField, "Email address"), box(platform.RoleTextField, "City")
	if !near(nameIn.X, emailIn.X) || !near(nameIn.X, cityIn.X) || !near(nameIn.X, box(platform.RoleCheckBox, "Send emails").X) {
		t.Errorf("the controls start at %v, %v, %v", nameIn.X, emailIn.X, cityIn.X)
	}
	nameL, emailL := find("Name"), find("Email address")
	if !near(float64(nameL.X+nameL.W), float64(emailL.X+emailL.W)) || float64(emailL.X+emailL.W) > emailIn.X {
		t.Errorf("the labels end at %v and %v, the inputs start at %v", nameL.X+nameL.W, emailL.X+emailL.W, emailIn.X)
	}
	// Labels are level with the text of their control: inside the input's
	// border and padding, on the check box's text, and centered on a
	// switch.
	if !near(float64(nameL.Y), nameIn.Y+7) || !near(float64(emailL.Y), emailIn.Y+7) {
		t.Errorf("the labels are at %v and %v, the inputs at %v and %v", nameL.Y, emailL.Y, nameIn.Y, emailIn.Y)
	}
	if notifyL, own := find("Notifications"), find("Send emails"); !near(float64(notifyL.Y), float64(own.Y)) {
		t.Errorf("the label is at %v, the check box's text at %v", notifyL.Y, own.Y)
	}
	sw := box(platform.RoleSwitch, "Express")
	if l := find("Express"); math.Abs(float64(l.Y+l.H/2)-(sw.Y+sw.H/2)) > 1.5 {
		t.Errorf("the label's center is at %v, the switch's at %v", l.Y+l.H/2, sw.Y+sw.H/2)
	}
	// The description sits below its input, in the controls' column.
	if d := find("For receipts."); !near(float64(d.X), emailIn.X) || float64(d.Y) < emailIn.Y+emailIn.H {
		t.Errorf("the description is at %v, the input at %v", d, emailIn)
	}
}
