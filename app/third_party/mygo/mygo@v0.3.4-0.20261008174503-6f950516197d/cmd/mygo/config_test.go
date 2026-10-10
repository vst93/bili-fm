package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestTSConfig(t *testing.T) {
	for _, rt := range []string{"bun", "node"} {
		t.Run(rt, func(t *testing.T) { testTSConfig(t, rt) })
		if runtime.GOOS != "windows" {
			continue
		}
		// npm installs commands on Windows as batch files, whose command
		// line cmd.exe parses.
		t.Run(rt+".cmd", func(t *testing.T) {
			path, err := exec.LookPath(rt)
			if err != nil {
				t.Skip(err)
			}
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, rt+".cmd"), []byte("@\""+path+"\" %*\r\n"), 0o755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			if p, _ := exec.LookPath(rt); !strings.EqualFold(filepath.Ext(p), ".cmd") {
				t.Fatalf("%s resolves to %s", rt, p)
			}
			testTSConfig(t, rt)
		})
	}
}

func testTSConfig(t *testing.T, rt string) {
	defer func(r []string) { configRuntimes = r }(configRuntimes)
	configRuntimes = []string{rt}
	if _, err := configRuntime(); err != nil {
		t.Skip(err)
	}
	dir := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{"name": "demo", "type": "module", "version": "2.3.4"}`)
	write(tsConfig, `import pkg from "./package.json" with { type: "json" };

interface Env {
  command: string;
}

console.log("printed by the configuration");

export default async ({ command }: Env) => ({
  name: "TS App",
  version: pkg.version,
  devUrl: command === "dev" ? "http://localhost:5173" : undefined,
  fileAssociations: [{ ext: ["md"], name: "Markdown" }],
  macos: { infoPlist: { NSCameraUsageDescription: "Scan documents." } },
});
`)
	defer func() { running = "" }()
	running = "dev"
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Name != "TS App" || c.Version != "2.3.4" || c.DevURL != "http://localhost:5173" || len(c.FileAssociations) != 1 ||
		c.MacOS.InfoPlist["NSCameraUsageDescription"] != "Scan documents." || c.configName() != tsConfig {
		t.Errorf("config: %+v", c)
	}
	running = "build"
	if c, err := loadConfig(dir); err != nil || c.DevURL != "" || c.Identifier != "com.mygo.tsapp" {
		t.Errorf("build config: %+v, %v", c, err)
	}

	for content, want := range map[string]string{
		`export default { devUrl: "ftp://example.com" }`: tsConfig + `: devUrl "ftp://example.com" is not an http(s) URL`,
		`export default 42`:                    "not a configuration object",
		`export default {`:                     tsConfig + ": ",
		`throw new Error("no config for you")`: "no config for you",
	} {
		write(tsConfig, content)
		if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: got %v, want %q", content, err, want)
		}
	}

	write(tsConfig, `export default {}`)
	write(jsonConfig, `{}`)
	if _, err := loadConfig(dir); err == nil || !strings.Contains(err.Error(), "keep one") {
		t.Errorf("both configurations: %v", err)
	}
	os.Remove(filepath.Join(dir, jsonConfig))

	// defineConfig of mygo-cli, which projects depend on.
	modules := filepath.Join(dir, "node_modules")
	os.Mkdir(modules, 0o755)
	cli, _ := filepath.Abs(filepath.Join("..", "..", "packages", "cli"))
	if err := os.Symlink(cli, filepath.Join(modules, "mygo-cli")); err != nil {
		if runtime.GOOS == "windows" {
			t.Skipf("no symbolic links: %v", err)
		}
		t.Fatal(err)
	}
	write(tsConfig, `import { defineConfig } from "mygo-cli";

export default defineConfig({ name: "Defined", out: "build" });
`)
	if c, err := loadConfig(dir); err != nil || c.Name != "Defined" || c.Out != "build" {
		t.Errorf("defineConfig: %+v, %v", c, err)
	}
}

// TestConfigTypes checks that the TypeScript types of mygo-cli describe the
// fields of Config.
func TestConfigTypes(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "packages", "cli", "index.d.ts"))
	if err != nil {
		t.Fatal(err)
	}
	src = bytes.ReplaceAll(src, []byte("\r\n"), []byte("\n")) // a Windows checkout
	property := regexp.MustCompile(`(?m)^  (\w+)\??:`)
	for name, typ := range map[string]reflect.Type{
		"Config":          reflect.TypeFor[Config](),
		"FileAssociation": reflect.TypeFor[FileAssociation](),
		"UpdatesConfig":   reflect.TypeFor[Updates](),
		"S3Config":        reflect.TypeFor[S3](),
		"MacOSConfig":     reflect.TypeFor[MacOS](),
		"NotarizeConfig":  reflect.TypeFor[Notarize](),
		"WindowsConfig":   reflect.TypeFor[Windows](),
		"LinuxConfig":     reflect.TypeFor[Linux](),
	} {
		m := regexp.MustCompile(`(?s)export interface ` + name + ` \{\n(.*?)\n\}`).FindSubmatch(src)
		if m == nil {
			t.Errorf("index.d.ts declares no %s", name)
			continue
		}
		var ts, goFields []string
		for _, p := range property.FindAllSubmatch(m[1], -1) {
			ts = append(ts, string(p[1]))
		}
		for f := range typ.Fields() {
			if tag, _, _ := strings.Cut(f.Tag.Get("json"), ","); tag != "" && tag != "-" {
				goFields = append(goFields, tag)
			}
		}
		slices.Sort(ts)
		slices.Sort(goFields)
		if !slices.Equal(ts, goFields) {
			t.Errorf("%s declares %v, Go has %v", name, ts, goFields)
		}
	}
}

func TestLinuxMaintainer(t *testing.T) {
	for _, tc := range []struct{ config, pkg, want string }{
		{`{"name": "My App"}`, ``, "My App"},
		{`{"name": "My App"}`, `{"name": "my-app"}`, "My App"},
		{`{"name": "My App"}`, `{"author": "Jane Doe <jane@example.com> (https://jane.dev)"}`, "Jane Doe <jane@example.com>"},
		{`{"name": "My App"}`, `{"author": "Jane Doe"}`, "Jane Doe"},
		{`{"name": "My App"}`, `{"author": {"name": "Jane Doe", "email": "jane@example.com", "url": "https://jane.dev"}}`, "Jane Doe <jane@example.com>"},
		{`{"name": "My App"}`, `{"author": "<jane@example.com>"}`, "My App"},
		{`{"name": "My App"}`, `{"author": `, "My App"}, // not JSON
		{`{"name": "My App", "linux": {"maintainer": "Acme <dev@acme.test>"}}`, `{"author": "Jane Doe"}`, "Acme <dev@acme.test>"},
	} {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, jsonConfig), []byte(tc.config), 0o644)
		if tc.pkg != "" {
			os.WriteFile(filepath.Join(dir, "package.json"), []byte(tc.pkg), 0o644)
		}
		c, err := loadConfig(dir)
		if err != nil {
			t.Fatal(err)
		}
		if c.Linux.Maintainer != tc.want {
			t.Errorf("with package.json %s and %s, the maintainer is %q, want %q", tc.pkg, tc.config, c.Linux.Maintainer, tc.want)
		}
	}
}
