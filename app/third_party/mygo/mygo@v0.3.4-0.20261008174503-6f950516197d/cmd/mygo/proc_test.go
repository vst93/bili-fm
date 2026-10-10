package main

import (
	"os"
	"strings"
	"testing"
)

func TestGoCommandCGO(t *testing.T) {
	dir := t.TempDir()
	for _, tt := range []struct {
		name  string
		value string
		unset bool
		env   []string
		want  string
	}{
		{name: "unset", unset: true, want: "0"},
		{name: "empty", want: "0"},
		{name: "disabled", value: "0", want: "0"},
		{name: "enabled", value: "1", want: "1"},
		{name: "command enables", value: "0", env: []string{"CGO_ENABLED=1"}, want: "1"},
		{name: "command disables", value: "1", env: []string{"CGO_ENABLED=0"}, want: "0"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CGO_ENABLED", tt.value)
			if tt.unset {
				if err := os.Unsetenv("CGO_ENABLED"); err != nil {
					t.Fatal(err)
				}
			}
			// Ask the child go tool, so this checks the effective value
			// even when the command's environment has duplicate keys.
			cmd := goCommand(dir, tt.env, "env", "CGO_ENABLED")
			cmd.Stdout, cmd.Stderr = nil, nil
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("go env: %v\n%s", err, out)
			}
			if got := strings.TrimSpace(string(out)); got != tt.want {
				t.Errorf("CGO_ENABLED = %q, want %q", got, tt.want)
			}
		})
	}
}
