package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMain lets the test binary play a development build of an app for
// the launch tests, and mygo for makensis, which runs it to sign
// uninstallers.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "sign-uninstaller" {
		if err := runSignUninstaller(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "mygo:", err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	mode := os.Getenv("MYGO_FAKE_APP")
	if mode == "relaunch" {
		// mygo.App.Relaunch the first time, then like "ready".
		marker := os.Getenv("MYGO_FAKE_MARKER")
		if _, err := os.Stat(marker); err != nil {
			_ = os.WriteFile(marker, nil, 0o644)
			os.Exit(devRelaunchCode)
		}
		mode = "ready"
	}
	switch mode {
	case "":
		os.Exit(m.Run())
	case "ready":
		// Like the mygo package: report ready, run until SIGTERM.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGTERM)
		conn, err := net.Dial("unix", os.Getenv("MYGO_READY_SOCKET"))
		if err != nil {
			os.Exit(2)
		}
		_, _ = conn.Write([]byte("ready\n"))
		conn.Close()
		<-sig
		os.Exit(0)
	case "exit":
		os.Exit(3)

	case "hang":
		signal.Ignore(syscall.SIGTERM)
		time.Sleep(time.Hour)

	// A build's processes for TestKillSparesWebView2: a child of its own,
	// and a WebView2 browser (a copy of the test binary) with a child too.
	// Each quits after a minute, whatever happens to the test.
	case "tree":
		_ = fakeChild(os.Args[0], "sleep").Start()
		_ = fakeChild(os.Getenv("MYGO_FAKE_WEBVIEW"), "webview").Start()
		time.Sleep(time.Minute)
	case "webview":
		_ = fakeChild(os.Args[0], "sleep").Start()
		time.Sleep(time.Minute)
	case "sleep":
		time.Sleep(time.Minute)
	}
}

func fakeChild(exe, mode string) *exec.Cmd {
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), "MYGO_FAKE_APP="+mode)
	return cmd
}

func TestDevLaunch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix only")
	}
	defer func(d time.Duration) { stopGrace = d }(stopGrace)
	stopGrace = 5 * time.Second // exiting takes a while under the race detector
	s := &devSession{root: t.TempDir(), readyTimeout: 5 * time.Second}
	ctx := context.Background()

	t.Setenv("MYGO_FAKE_APP", "ready")
	p, err := s.launch(ctx, os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	p.stop()
	if p.err != nil {
		t.Errorf("a stopped build should quit cleanly: %v", p.err)
	}

	// A build that relaunches itself before it is ready starts again.
	t.Setenv("MYGO_FAKE_APP", "relaunch")
	t.Setenv("MYGO_FAKE_MARKER", filepath.Join(t.TempDir(), "launched"))
	p, err = s.launch(ctx, os.Args[0])
	if err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	p.stop()

	t.Setenv("MYGO_FAKE_APP", "exit")
	if _, err := s.launch(ctx, os.Args[0]); err == nil || !strings.Contains(err.Error(), "exited before it was ready: exit status 3") {
		t.Errorf("early exit: %v", err)
	}

	t.Setenv("MYGO_FAKE_APP", "hang")
	s.readyTimeout = 300 * time.Millisecond
	stopGrace = 200 * time.Millisecond
	start := time.Now()
	if _, err := s.launch(ctx, os.Args[0]); err == nil || !strings.Contains(err.Error(), "did not get ready") {
		t.Errorf("hang: %v", err)
	}
	if d := time.Since(start); d > 3*time.Second {
		t.Errorf("a hanging build took %v to be killed", d)
	}
}

func TestWatcher(t *testing.T) {
	dir := t.TempDir()
	// put writes a file in dir at once, as editors save files: the watcher
	// must not see it, or the directories made for it, half written, which
	// it reports as a change of its own when the writing takes longer than
	// an interval, as it may on a busy machine.
	put := func(name, content string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		top := path // the first part of the path that does not exist yet
		for p := filepath.Dir(path); !fileExists(p); p = filepath.Dir(p) {
			top = p
		}
		staged := filepath.Join(t.TempDir(), filepath.Base(top))
		rel, err := filepath.Rel(top, path)
		if err != nil {
			t.Fatal(err)
		}
		writeFiles(t, staged, map[string]string{filepath.ToSlash(rel): content})
		if err := os.Rename(staged, top); err != nil {
			t.Fatal(err)
		}
	}
	put("main.go", "package main")
	w := &watcher{}
	w.set(&buildInputs{sourceDirs: []string{dir}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	changes := w.watch(ctx, 20*time.Millisecond)
	expect := func(changed bool, what string) {
		t.Helper()
		select {
		case <-changes:
			if !changed {
				t.Fatalf("%s: change reported", what)
			}
		case <-time.After(300 * time.Millisecond):
			if changed {
				t.Fatalf("%s: change not reported", what)
			}
		}
	}
	expect(false, "nothing")
	for _, f := range []string{"main_test.go", "notes.txt", "web.ts"} {
		put(f, "x")
	}
	expect(false, "files the build ignores")
	put("main.go", "package main // edited")
	expect(true, "edited source")
	put("new.go", "package main")
	expect(true, "new source")

	// An edit made while a build runs is not lost when the build hands
	// over the same inputs.
	put("main.go", "package main // edited during a build")
	w.set(&buildInputs{sourceDirs: []string{dir}})
	expect(true, "edit during a build")

	// The directories of embedded files are watched whole, but for the
	// Windows resources that builds write into the main package.
	w.set(&buildInputs{sourceDirs: []string{dir}, fileDirs: []string{dir}})
	expect(false, "embedded files")
	put("mygo_windows_amd64.syso", "x")
	expect(false, "resources of a build")
	if err := os.Remove(filepath.Join(dir, "mygo_windows_amd64.syso")); err != nil {
		t.Fatal(err)
	}
	expect(false, "resources of a build removed")
	put("index.html", "x")
	expect(true, "new embedded file")
	w.set(&buildInputs{sourceDirs: []string{dir}})
	expect(false, "new inputs")

	// Resources are watched whole, and may not exist yet, but for the
	// platform directories of other platforms, which development builds
	// do not ship.
	res := filepath.Join(dir, "resources")
	w.set(&buildInputs{sourceDirs: []string{dir}, resources: res})
	expect(false, "new inputs")
	put("resources/data/words.txt", "hello")
	expect(true, "new resources")
	put("resources/data/.DS_Store", "junk")
	expect(false, "hidden file in resources")
	put("resources/data/words.txt", "hello, world")
	expect(true, "edited resource")
	other := "windows-arm64"
	if runtime.GOOS == "windows" {
		other = "linux-amd64"
	}
	put("resources/"+other+"/bin/server", "x")
	put("resources/darwin-universal/bin/server", "x")
	expect(false, "resources of other platforms")
	put("resources/"+runtime.GOOS+"-"+runtime.GOARCH+"/bin/server", "x")
	expect(true, "resources of this platform")
	put("resources/"+runtime.GOOS+"/bin/tool", "x")
	expect(true, "resources of this system")
	if runtime.GOOS != "windows" {
		// Linked entries are followed, as builds follow them.
		sidecar := filepath.Join(dir, "sidecar")
		writeFiles(t, sidecar, map[string]string{"server": "1"})
		if err := os.Symlink(sidecar, filepath.Join(res, runtime.GOOS, "sidecar")); err != nil {
			t.Fatal(err)
		}
		expect(true, "linked resource")
		put("sidecar/server", "12")
		expect(true, "edited linked resource")
	}
	if err := os.RemoveAll(res); err != nil {
		t.Fatal(err)
	}
	expect(true, "removed resources")
}

// TestReadDir lists a directory as os.ReadDir does, and lets others
// delete it meanwhile, which a directory os.Open opened on Windows does
// not.
func TestReadDir(t *testing.T) {
	// Longer than MAX_PATH.
	dir := filepath.Join(t.TempDir(), strings.Repeat("a", 100), strings.Repeat("b", 100), strings.Repeat("c", 100))
	writeFiles(t, dir, map[string]string{"b.txt": "", "a/c.txt": "", "C.txt": ""})
	entries, err := readDir(dir)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if err != nil || strings.Join(names, " ") != "C.txt a b.txt" {
		t.Errorf("readDir = %q, %v", names, err)
	}
	f, err := openDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.RemoveAll(dir); err != nil {
		t.Errorf("deleting a directory being read: %v", err)
	}
	if _, err := readDir(dir); !os.IsNotExist(err) {
		t.Errorf("readDir of a deleted directory: %v", err)
	}
}

// TestListBuildInputs lists the inputs of the mygo command itself.
func TestListBuildInputs(t *testing.T) {
	root, _ := filepath.Abs("../..")
	c := &Config{root: root, Main: "./cmd/mygo", Icon: "icon.png", Resources: []string{"notes"},
		MacOS: MacOS{Entitlements: "app.plist", HelperEntitlements: map[string]string{"bin/tool": "tool.plist"}}}
	in, err := listBuildInputs(c)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(in.sourceDirs, filepath.Join(root, "cmd", "mygo")) {
		t.Errorf("sourceDirs lacks cmd/mygo: %q", in.sourceDirs)
	}
	for _, d := range in.sourceDirs {
		if strings.Contains(d, string(filepath.Separator)+"pkg"+string(filepath.Separator)+"mod"+string(filepath.Separator)) || strings.HasPrefix(d, goEnv("GOROOT")) {
			t.Errorf("sourceDirs has %s, which never changes", d)
		}
	}
	for _, f := range []string{filepath.Join(root, "go.mod"), filepath.Join(root, "mygo.json"), filepath.Join(root, "icon.png"),
		filepath.Join(root, "app.plist"), filepath.Join(root, "tool.plist")} {
		if !slices.Contains(in.files, f) {
			t.Errorf("files lacks %s: %q", f, in.files)
		}
	}
	if !slices.Equal(in.trees, []string{filepath.Join(root, "notes")}) || in.resources != filepath.Join(root, "resources") {
		t.Errorf("trees = %q, resources = %q", in.trees, in.resources)
	}
	// The CLI embeds its project template.
	if !slices.Contains(in.fileDirs, filepath.Join(root, "cmd", "mygo", "template", "web")) {
		t.Errorf("fileDirs lacks the embedded template: %q", in.fileDirs)
	}
}

// TestHashEntitlements tells changes to the entitlements of development
// apps apart, which relaunch them.
func TestHashEntitlements(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"app.plist": "app", "tool.plist": "tool"})
	c := &Config{root: dir}
	hash := func() string {
		t.Helper()
		var b bytes.Buffer
		if err := hashEntitlements(&b, c); err != nil {
			t.Fatal(err)
		}
		return b.String()
	}
	seen := map[string]string{}
	for _, step := range []struct {
		name string
		edit func()
	}{
		{"none", func() {}},
		{"app", func() { c.MacOS.Entitlements = "app.plist" }},
		{"helper", func() { c.MacOS.HelperEntitlements = map[string]string{"bin/tool": "tool.plist"} }},
		{"edited", func() { writeFiles(t, dir, map[string]string{"tool.plist": "tool, edited"}) }},
		{"renamed", func() { c.MacOS.HelperEntitlements = map[string]string{"bin/other": "tool.plist"} }},
	} {
		step.edit()
		h := hash()
		if prev, ok := seen[h]; ok {
			t.Errorf("%s: same hash as %s", step.name, prev)
		}
		seen[h] = step.name
	}
	c.MacOS.Entitlements = "missing.plist"
	if err := hashEntitlements(io.Discard, c); err == nil {
		t.Error("missing entitlements hashed")
	}
}

// TestDevWindowsResources links the resources of the development app into
// a Windows build of the package that Main names, and leaves no .syso there.
func TestDevWindowsResources(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles a program")
	}
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"go.mod":          "module example.com/devres\n\n" + goDirective(t) + "\n",
		"cmd/app/main.go": "package main\n\nfunc main() {}\n",
		"icon.png":        string(defaultIcon()),
	})
	c := &Config{root: dir, Name: "Res Test", Identifier: "com.example.restest", Icon: "icon.png", Main: "./cmd/app"}
	c.applyDefaults()
	s := &devSession{root: dir}
	cleanup, err := s.windowsResources(c, devConfig(c))
	if err != nil {
		t.Fatal(err)
	}
	pkg := filepath.Join(dir, "cmd", "app")
	if files := sysoFiles(pkg); len(files) != 1 {
		t.Errorf("the package has %q", files)
	}
	exe := filepath.Join(t.TempDir(), "app.exe")
	err = buildBinary(c, exe, []string{"GOOS=windows", "GOARCH=" + runtime.GOARCH})
	cleanup()
	if err != nil {
		t.Fatal(err)
	}
	if left := sysoFiles(pkg); len(left) > 0 {
		t.Errorf("left %q in the package", left)
	}
	data := peResources(t, exe)
	for what, want := range map[string][]byte{
		"name":       utf16le("Res Test Dev"),
		"identifier": utf16le("com.example.restest.dev"),
		"icon":       []byte("\x89PNG"),
	} {
		if !bytes.Contains(data, want) {
			t.Errorf("the resources lack the development app's %s", what)
		}
	}
}

func TestDevConfig(t *testing.T) {
	c := &Config{Name: "Todo", Identifier: "dev.mygo.todo"}
	d := devConfig(c)
	if d.Name != "Todo Dev" || d.Identifier != "dev.mygo.todo.dev" || c.Name != "Todo" {
		t.Errorf("devConfig = %q %q (original %q)", d.Name, d.Identifier, c.Name)
	}
}
