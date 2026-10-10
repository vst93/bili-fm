package accelerator

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		in   string
		goos string
		want Accelerator
	}{
		{"CmdOrCtrl+Q", "darwin", Accelerator{Super, "q"}},
		{"CmdOrCtrl+Q", "linux", Accelerator{Ctrl, "q"}},
		{"CommandOrControl+Shift+Z", "windows", Accelerator{Ctrl | Shift, "z"}},
		{"Alt+F4", "windows", Accelerator{Alt, "F4"}},
		{"Option+Cmd+I", "darwin", Accelerator{Alt | Super, "i"}},
		{"F11", "linux", Accelerator{0, "F11"}},
		{"Ctrl+Plus", "linux", Accelerator{Ctrl, "+"}},
		{"CmdOrCtrl++", "darwin", Accelerator{Super, "+"}},
		{"CmdOrCtrl+=", "darwin", Accelerator{Super, "="}},
		{"Cmd+,", "darwin", Accelerator{Super, ","}},
		{"Shift+Return", "darwin", Accelerator{Shift, "Enter"}},
		{"Esc", "darwin", Accelerator{0, "Escape"}},
		{"Ctrl+Num5", "linux", Accelerator{Ctrl, "Num5"}},
		{"Super+Space", "linux", Accelerator{Super, "Space"}},
		{"ctrl + shift + up", "linux", Accelerator{Ctrl | Shift, "Up"}},
	}
	for _, tt := range tests {
		got, err := Parse(tt.in, tt.goos)
		if err != nil {
			t.Errorf("Parse(%q): %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("Parse(%q, %s) = %+v, want %+v", tt.in, tt.goos, got, tt.want)
		}
	}
}

func TestParseErrors(t *testing.T) {
	for _, in := range []string{"", "Ctrl+", "Hyper+A", "Ctrl+F25", "Ctrl+Foo", "Ctrl+A+B"} {
		if _, err := Parse(in, "linux"); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestString(t *testing.T) {
	a, _ := Parse("Shift+CmdOrCtrl+k", "linux")
	if got := a.String(); got != "Ctrl+Shift+K" {
		t.Errorf("String() = %q", got)
	}
}

func TestXDGTrigger(t *testing.T) {
	for in, want := range map[string]string{
		"Shift+CmdOrCtrl+K":  "CTRL+SHIFT+k",
		"Super+Space":        "LOGO+space",
		"Alt+Shift+Enter":    "ALT+SHIFT+Return",
		"CmdOrCtrl+,":        "CTRL+comma",
		"Ctrl+Plus":          "CTRL+plus",
		"Ctrl+Alt+Super+F12": "CTRL+ALT+LOGO+F12",
		"Ctrl+Num7":          "CTRL+KP_7",
		"Ctrl+PageDown":      "CTRL+Page_Down",
		"MediaPlayPause":     "XF86AudioPlay",
		"Ctrl+1":             "CTRL+1",
		"Ctrl+é":             "CTRL+U00E9",
	} {
		a, err := Parse(in, "linux")
		if err != nil {
			t.Fatal(err)
		}
		if got := a.XDGTrigger(); got != want {
			t.Errorf("XDGTrigger(%q) = %q, want %q", in, got, want)
		}
	}
}
