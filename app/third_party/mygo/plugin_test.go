package mygo

import (
	"errors"
	"strings"
	"testing"
)

type pluginService struct{}

func (pluginService) Ping(s string) string { return "pong " + s }

func TestUse(t *testing.T) {
	setups := 0
	useForTest(t, Plugin{Name: "test-ping", Service: pluginService{}, Setup: func() error { setups++; return nil }})
	if setups != 1 {
		t.Errorf("Setup ran %d times", setups)
	}
	_, fw := testWindow(t, WindowOptions{})
	if m := call(t, fw, 1, "plugin:test-ping.Ping", "a"); m["v"] != "pong a" {
		t.Errorf("plugin call: %v", m)
	}
	if m := call(t, fw, 2, "plugin:unused.Ping"); !strings.Contains(m["e"].(string), "the unused plugin is not used") {
		t.Errorf("unused plugin: %v", m)
	}
	if m := call(t, fw, 3, "plugin:test-ping.Missing"); m["e"] != "method plugin:test-ping.Missing is not bound" {
		t.Errorf("missing method: %v", m)
	}

	src, err := GenerateTypeScript()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "ping") {
		t.Errorf("the generated client has the plugin's service:\n%s", src)
	}

	for name, p := range map[string]Plugin{
		"duplicate":  {Name: "test-ping"},
		"empty name": {},
		"upper case": {Name: "Fetch"},
		"reserved":   {Name: "mygo"},
		"bad":        {Name: "test-bad", Service: badService{}},
		"setup":      {Name: "test-setup", Setup: func() error { return errors.New("no") }},
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: Use did not panic", name)
				}
			}()
			useForTest(t, p)
		}()
	}
}
