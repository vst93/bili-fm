package ui

import (
	"testing"

	"github.com/egoist/mygo/internal/platform"
)

func TestActionQueriesReadPendingBoundEdit(t *testing.T) {
	for _, name := range []string{"Clicked", "Clicks", "DoubleClicked", "RightClicked", "ElementShortcut", "ContextShortcut"} {
		t.Run(name, func(t *testing.T) {
			value, saved, actions := "old", "", 0
			tt := NewTester(func(c *Context) {
				scope := Column(c)
				scope.Children(func() {
					TextInput(c.Key("input"), &value).Label("Input")
					button := Button(c.Key("save"), "Save")
					handled := false
					switch name {
					case "Clicked":
						handled = button.Clicked()
					case "Clicks":
						handled = button.Clicks() > 0
					case "DoubleClicked":
						handled = button.DoubleClicked()
					case "RightClicked":
						handled = button.RightClicked()
					case "ElementShortcut":
						handled = scope.Shortcut(Cmd, KeyS)
					case "ContextShortcut":
						handled = c.Shortcut(Cmd, KeyS)
					}
					if handled {
						saved = value
						actions++
					}
				})
			}, 300, 120)
			if err := tt.Click("Input"); err != nil {
				t.Fatal(err)
			}
			tt.Key(Cmd, KeyA)
			// Native events can arrive together before the next frame. Do not
			// settle between the edit and the action, as Tester.Type does.
			tt.rt.event(platform.SurfaceEvent{Kind: platform.TextInput, Text: "new"})
			if name == "ElementShortcut" || name == "ContextShortcut" {
				tt.rt.event(platform.SurfaceEvent{Kind: platform.KeyPressed, Mods: platform.Modifiers(Cmd), Key: platform.Key(KeyS)})
			} else {
				r, ok := tt.Find("Save")
				if !ok {
					t.Fatal("Save button missing")
				}
				button, presses := 0, 1
				if name == "RightClicked" {
					button = 1
				}
				if name == "DoubleClicked" {
					presses = 2
				}
				for range presses {
					ev := platform.SurfaceEvent{Kind: platform.PointerDown, X: float64(r.X + r.W/2), Y: float64(r.Y + r.H/2), Button: button}
					tt.rt.event(ev)
					ev.Kind = platform.PointerUp
					tt.rt.event(ev)
				}
			}
			tt.Frame()
			if saved != "new" || value != "new" || actions != 1 {
				t.Fatalf("saved %q, bound value %q, actions %d", saved, value, actions)
			}
			tt.Frame()
			if actions != 1 {
				t.Fatal("another frame repeated the action")
			}
		})
	}
}

func TestScopedShortcutQueriesReadPendingBoundEdit(t *testing.T) {
	for _, name := range []string{"list", "overlay"} {
		t.Run(name, func(t *testing.T) {
			value, saved, actions := "old", "", 0
			var list ListState
			tt := NewTester(func(c *Context) {
				if name == "list" {
					List(c, &list, 1, func(int) {
						TextInput(c, &value).Label("Input")
						if list.Shortcut(c, Cmd, KeyS) {
							saved = value
							actions++
						}
					})
				} else {
					TextInput(c, &value).Label("Input")
					Overlay(c, func() {
						if Box(c).Size(0, 0).OverlayShortcut(Cmd, KeyS) {
							saved = value
							actions++
						}
					})
				}
			}, 300, 120)
			if err := tt.Click("Input"); err != nil {
				t.Fatal(err)
			}
			tt.Key(Cmd, KeyA)
			tt.rt.event(platform.SurfaceEvent{Kind: platform.TextInput, Text: "new"})
			tt.rt.event(platform.SurfaceEvent{Kind: platform.KeyPressed, Mods: platform.Modifiers(Cmd), Key: platform.Key(KeyS)})
			tt.Frame()
			if saved != "new" || value != "new" || actions != 1 {
				t.Fatalf("saved %q, bound value %q, actions %d", saved, value, actions)
			}
			tt.Frame()
			if actions != 1 {
				t.Fatal("another frame repeated the action")
			}
		})
	}
}

func TestActionQueriesRespectFluentInputConfiguration(t *testing.T) {
	for _, search := range []bool{false, true} {
		name := "ReadOnly"
		if search {
			name = "DisabledSearchField"
		}
		t.Run(name, func(t *testing.T) {
			value, saved, blocked := "old", "", false
			tt := NewTester(func(c *Context) {
				Column(c).Children(func() {
					if search {
						SearchField(c.Key("input"), &value).Label("Input").Disabled(blocked)
					} else {
						TextInput(c.Key("input"), &value).Label("Input").ReadOnly(blocked)
					}
					if Button(c, "Save").Clicked() {
						saved = value
					}
				})
			}, 300, 120)
			if err := tt.Click("Input"); err != nil {
				t.Fatal(err)
			}
			tt.Key(Cmd, KeyA)
			tt.rt.event(platform.SurfaceEvent{Kind: platform.TextInput, Text: "new"})
			blocked = true
			if err := tt.Click("Save"); err != nil {
				t.Fatal(err)
			}
			if saved != "old" || value != "old" {
				t.Fatalf("saved %q, bound value %q: input applied before fluent configuration", saved, value)
			}
		})
	}
}
