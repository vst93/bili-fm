package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// shellCommand runs a command line such as "bun run dev" through the shell.
func shellCommand(dir, line string, env ...string) *exec.Cmd {
	cmd := shell(line)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), env...)
	setProcessGroup(cmd)
	return cmd
}

// command runs a tool quietly; its output becomes part of the error when
// it fails.
func command(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v\n%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

func run(dir, line string, env ...string) error {
	logf("%s", line)
	if err := shellCommand(dir, line, env...).Run(); err != nil {
		return fmt.Errorf("%s: %w", line, err)
	}
	return nil
}

// goCommand runs the go tool.
func goCommand(dir string, env []string, args ...string) *exec.Cmd {
	return goCommandContext(context.Background(), dir, env, args...)
}

func goCommandContext(ctx context.Context, dir string, env []string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = os.Environ()
	// MyGo needs no cgo by default, but an app's dependencies may.
	if os.Getenv("CGO_ENABLED") == "" {
		cmd.Env = append(cmd.Env, "CGO_ENABLED=0")
	}
	cmd.Env = append(cmd.Env, env...)
	return cmd
}

// buildBinary compiles the app package into out.
func buildBinary(c *Config, out string, env []string, flags ...string) error {
	return buildBinaryContext(context.Background(), c, out, env, flags...)
}

func buildBinaryContext(ctx context.Context, c *Config, out string, env []string, flags ...string) error {
	args := append([]string{"build", "-o", out}, flags...)
	args = append(args, c.Main)
	cmd := goCommandContext(ctx, c.root, env, args...)
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("go build failed: %w", err)
	}
	return nil
}

// generateBindings runs a compiled app in generate mode, which writes the
// TypeScript client when it changed.
func generateBindings(c *Config, binary string) error {
	if c.Bindings == "" {
		return nil // no frontend
	}
	out := c.path(c.Bindings)
	before, _ := os.ReadFile(out)
	cmd := exec.Command(binary)
	cmd.Dir = c.root
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "MYGO_GENERATE="+out)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generating the TypeScript client: %w", err)
	}
	if after, _ := os.ReadFile(out); !bytes.Equal(before, after) {
		rel, _ := filepath.Rel(c.root, out)
		logf("wrote %s", rel)
	}
	return nil
}

// writeClient builds the app for this computer and writes its TypeScript
// client, when it has a frontend.
func writeClient(c *Config) error {
	if c.Bindings == "" {
		return nil
	}
	bin := tempBinary(c.executableName())
	defer os.Remove(bin)
	if err := buildBinary(c, bin, nil); err != nil {
		return err
	}
	return generateBindings(c, bin)
}

// waitForURL polls url until it answers. It fails when ctx is done or the
// server exits first.
func waitForURL(ctx context.Context, url string, exited <-chan error) error {
	client := &http.Client{Timeout: time.Second}
	for {
		if resp, err := client.Get(url); err == nil {
			resp.Body.Close()
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("the dev server did not start at %s", url)
		case err := <-exited:
			return fmt.Errorf("the dev server exited: %v", err)
		case <-time.After(150 * time.Millisecond):
		}
	}
}

func tempBinary(name string) string {
	dir := filepath.Join(os.TempDir(), "mygo-dev")
	_ = os.MkdirAll(dir, 0o755)
	name = strings.ReplaceAll(name, " ", "-")
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(dir, fmt.Sprintf("%s-%d", name, os.Getpid())+filepath.Ext(name))
}
