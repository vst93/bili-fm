package main

import (
	"bytes"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"testing"
)

func TestInstallSkills(t *testing.T) {
	for _, mode := range []string{"current directory", "explicit directory"} {
		t.Run(mode, func(t *testing.T) {
			dir := t.TempDir()
			var args []string
			if mode == "current directory" {
				t.Chdir(dir)
			} else {
				args = []string{dir}
			}
			preserved := map[string]string{
				"main.go":                        "package main\n",
				".agents/skills/custom/SKILL.md": "custom skill",
				".agents/skills/mygo-maintenance/references/local.md": "local guidance",
			}
			for path, content := range preserved {
				full := filepath.Join(dir, filepath.FromSlash(path))
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			for _, phase := range []string{"install", "update", "repeat"} {
				if err := runInstallSkills(args); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"SKILL.md", "agents/openai.yaml"} {
					path := ".agents/skills/mygo-maintenance/" + name
					want, err := templateFS.ReadFile("template/shared/" + path)
					if err != nil {
						t.Fatal(err)
					}
					full := filepath.Join(dir, filepath.FromSlash(path))
					got, err := os.ReadFile(full)
					if err != nil {
						t.Fatal(err)
					}
					if !bytes.Equal(got, want) {
						t.Errorf("%s: %s differs from the bundled skill", phase, path)
					}
					if phase == "install" {
						if err := os.WriteFile(full, []byte("old skill content"), 0o644); err != nil {
							t.Fatal(err)
						}
					}
				}
				for path, want := range preserved {
					got, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(path)))
					if err != nil || string(got) != want {
						t.Errorf("%s: %s was changed: %q, %v", phase, path, got, err)
					}
				}
			}
		})
	}
}

func TestInstallSkillsErrors(t *testing.T) {
	if err := runInstallSkills([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help: %v", err)
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-unknown"}, {dir, dir}, {filepath.Join(dir, "missing")}, {file}} {
		if err := runInstallSkills(args); err == nil {
			t.Errorf("%v: expected an error", args)
		}
	}
	// A file blocking the destination is an error, not a successful install.
	if err := os.WriteFile(filepath.Join(dir, ".agents"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInstallSkills([]string{dir}); err == nil {
		t.Error("blocked destination: expected an error")
	}
}
