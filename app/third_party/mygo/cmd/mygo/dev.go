package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

func runDev(args []string) error {
	flags := newFlags("dev", "[flags] [dir]", `Develops the app with live reload. It writes the TypeScript client of a
frontend, runs devCommand from mygo.json (such as a Vite dev server),
waits for devUrl to answer, then builds a development app, which loads
devUrl in place of its built frontend, and launches it. Without devUrl,
the app serves frontendDist from disk. The development app is "<name> Dev" with the
identifier "<identifier>.dev", in .mygo/dev: a real bundle on macOS, and
on Windows an executable with the icon, manifest and version information
that mygo build embeds.

Changes to the Go code, mygo.json or mygo.config.ts, the icon or the
resources rebuild the app, regenerate the TypeScript client and relaunch
it. The new build replaces the running one once it has started, so a build
that fails or crashes keeps the previous one running. Frontend changes are
left to the dev server. Quitting the app ends mygo dev.`)
	skipDevCommand := flags.Bool("skip-dev-command", false, "do not run devCommand")
	sign := flags.String("sign", "-", "macOS signing identity for the development app")
	if err := flags.Parse(args); err != nil {
		return err
	}
	c, err := loadConfig(dirArg(flags.Args()))
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// The TypeScript client comes first: the frontend imports it.
	if err := writeClient(c); err != nil {
		return err
	}
	s := &devSession{root: c.root, sign: *sign, env: []string{"MYGO_ENV=development"}}
	var exited chan error // the dev command's
	if c.DevCommand != "" && !*skipDevCommand {
		logf("starting %s", c.DevCommand)
		cmd := shellCommand(c.root, c.DevCommand)
		// Stopping the command makes script runners complain; that is noise.
		out, errOut := &gate{w: os.Stdout}, &gate{w: os.Stderr}
		cmd.Stdout, cmd.Stderr = out, errOut
		if isTerminal(os.Stdout) && os.Getenv("NO_COLOR") == "" {
			cmd.Env = append(cmd.Env, "FORCE_COLOR=1") // it now writes to a pipe
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		defer func() {
			out.close()
			errOut.close()
			terminate(cmd)
		}()
		exited = make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
	}
	switch {
	case c.DevURL != "":
		if exited == nil {
			logf("waiting for %s", c.DevURL)
		}
		wait, cancel := context.WithTimeout(ctx, 30*time.Second)
		err := waitForURL(wait, c.DevURL, exited)
		cancel()
		if err != nil {
			return err
		}
		s.env = append(s.env, "MYGO_DEV_URL="+c.DevURL)
	case c.FrontendDist != "":
		s.env = append(s.env, "MYGO_FRONTEND_DIST="+c.path(c.FrontendDist))
	}
	return s.run(ctx, c)
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// gate writes to w until it is closed.
type gate struct {
	mu     sync.Mutex
	w      io.Writer
	closed bool
}

func (g *gate) Write(p []byte) (int, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.closed {
		_, _ = g.w.Write(p)
	}
	return len(p), nil
}

func (g *gate) close() {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
}

// devSession builds and runs development builds of an app.
type devSession struct {
	root string
	sign string
	env  []string // for the app

	readyTimeout time.Duration // how long a launch may take (default 20s)

	// Used by one build at a time.
	iconKey  string
	icns     []byte
	mainFor  string // the Main that mainDir is the package directory of
	mainDir  string
	launches int

	// Used by the run loop only.
	app *devProcess // the running build
	sum [32]byte    // fingerprint of the running build

	// live is app, which a build stops before it starts.
	liveMu sync.Mutex
	live   *devProcess
}

// setApp makes p the running build.
func (s *devSession) setApp(p *devProcess) {
	s.app = p
	s.liveMu.Lock()
	s.live = p
	s.liveMu.Unlock()
}

// stopLive stops the running build for the one about to start: builds
// never overlap, so the single instance lock, the web view's profile and
// any other state of the app's are free.
func (s *devSession) stopLive() {
	s.liveMu.Lock()
	p := s.live
	s.liveMu.Unlock()
	if p != nil {
		p.replaced.Store(true)
		p.stop()
	}
}

var errUnchanged = errors.New("unchanged")

// devRelaunchCode is the exit code of a build that mygo.App.Relaunch asks
// to start again (see login.go in package mygo).
const devRelaunchCode = 75

// run launches the app and rebuilds it on changes until the app quits or
// ctx is done.
func (s *devSession) run(ctx context.Context, c *Config) error {
	w := &watcher{}
	if in, err := listBuildInputs(c); err == nil {
		w.set(in)
	} else {
		logf("%v", err)
	}
	changes := w.watch(ctx, 250*time.Millisecond)

	type result struct {
		p      *devProcess
		sum    [32]byte
		err    error
		inputs *buildInputs
	}
	results := make(chan result, 1)
	builds, cancel := context.WithCancel(ctx)
	building, pending := false, false
	start := func() {
		building, pending = true, false
		running := s.sum
		go func() {
			p, sum, err := s.buildAndLaunch(builds, running)
			// What the build reads may have changed, e.g. a new import.
			var inputs *buildInputs
			if c, cerr := loadConfig(s.root); cerr == nil {
				inputs, _ = listBuildInputs(c)
			}
			results <- result{p, sum, err, inputs}
		}()
	}
	var stopping sync.WaitGroup
	defer func() {
		cancel()
		if building {
			if r := <-results; r.p != nil {
				r.p.stop()
			}
		}
		if s.app != nil {
			s.app.stop()
		}
		stopping.Wait()
	}()

	start()
	for {
		var exited <-chan struct{}
		if s.app != nil {
			exited = s.app.done
		}
		select {
		case <-ctx.Done():
			return nil
		case <-exited:
			if s.app.replaced.Load() {
				s.setApp(nil) // stopped for the next build
				continue
			}
			err, exe := s.app.err, s.app.exe
			s.setApp(nil)
			var exit *exec.ExitError
			if errors.As(err, &exit) && exit.ExitCode() == devRelaunchCode {
				// mygo.App.Relaunch: start the same build again.
				logf("relaunching")
				if p, err := s.launch(ctx, exe); err != nil {
					logf("%v; waiting for changes", err)
					s.sum = [32]byte{}
				} else {
					s.setApp(p)
				}
				continue
			}
			s.sum = [32]byte{}
			if err == nil {
				logf("the app quit")
				return nil
			}
			logf("the app exited (%v); waiting for changes", err)
		case <-changes:
			if building {
				pending = true
			} else {
				start()
			}
		case r := <-results:
			building = false
			if r.inputs != nil {
				w.set(r.inputs)
			}
			switch {
			case errors.Is(r.err, errUnchanged):
				logf("the build did not change")
			case r.err != nil:
				logf("%v", r.err)
				if s.app != nil && !s.app.replaced.Load() {
					logf("keeping the previous build running")
				}
			default:
				prev := s.app
				s.setApp(r.p)
				s.sum = r.sum
				if prev != nil {
					stopping.Add(1)
					go func() {
						defer stopping.Done()
						prev.stop()
					}()
				}
			}
			if pending {
				start()
			}
		}
	}
}

// devConfig configures the development app: "<Name> Dev" with the
// identifier "<identifier>.dev", so that its data, preferences and single
// instance lock stay apart from the production app's.
func devConfig(c *Config) *Config {
	d := *c
	d.Name += " Dev"
	d.Identifier += ".dev"
	return &d
}

// buildAndLaunch builds the app and launches it once it differs from the
// running build, whose fingerprint is running. It returns the new process
// after it reported ready.
func (s *devSession) buildAndLaunch(ctx context.Context, running [32]byte) (*devProcess, [32]byte, error) {
	var sum [32]byte
	c, err := loadConfig(s.root)
	if err != nil {
		return nil, sum, err
	}
	started := time.Now()
	dir := filepath.Join(s.root, ".mygo", "dev", runtime.GOOS+"-"+runtime.GOARCH)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, sum, err
	}
	stage, err := os.MkdirTemp(dir, ".staging-")
	if err != nil {
		return nil, sum, err
	}
	defer os.RemoveAll(stage)

	dc := devConfig(c)
	name := dc.executableName()
	switch runtime.GOOS {
	case "darwin":
	case "windows":
		name += ".exe"
	default:
		name = slugify(dc.Name)
	}
	bin := filepath.Join(stage, name)
	if running == ([32]byte{}) {
		logf("building %s", dc.Name)
	} else {
		logf("rebuilding")
	}
	cleanup := func() {}
	if runtime.GOOS == "windows" {
		if cleanup, err = s.windowsResources(c, dc); err != nil {
			return nil, sum, err
		}
	}
	err = buildBinaryContext(ctx, c, bin, nil, "-ldflags", strings.TrimSpace(packageFlags(dc)))
	cleanup()
	if err != nil {
		return nil, sum, err
	}

	res, err := dc.appResources(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return nil, sum, err
	}

	// Fingerprint what the app is made of, to skip relaunching when a
	// change did not affect it.
	h := sha256.New()
	f, err := os.Open(bin)
	if err != nil {
		return nil, sum, err
	}
	_, err = io.Copy(h, f)
	f.Close()
	if err != nil {
		return nil, sum, err
	}
	if err := hashResources(h, res); err != nil {
		return nil, sum, err
	}
	var icns []byte
	if runtime.GOOS == "darwin" {
		if icns, err = s.icon(c); err != nil {
			return nil, sum, err
		}
		iconFile := ""
		if icns != nil {
			iconFile = bundleIcon
		}
		h.Write(icns)
		h.Write(infoPlist(dc, name, iconFile))
		if err := hashEntitlements(h, dc); err != nil {
			return nil, sum, err
		}
	}
	copy(sum[:], h.Sum(nil))
	if sum == running {
		return nil, sum, errUnchanged
	}

	if err := generateBindings(c, bin); err != nil {
		return nil, sum, err
	}
	exe := filepath.Join(dir, name)
	if runtime.GOOS == "darwin" {
		app, err := writeBundle(dc, stage, bin, icns, res)
		if err != nil {
			return nil, sum, err
		}
		if err := codesign(dc, app, s.sign, false); err != nil {
			return nil, sum, err
		}
		final := filepath.Join(dir, filepath.Base(app))
		if err := replacePath(app, final); err != nil {
			return nil, sum, err
		}
		exe = bundleExecutable(final)
	} else if err := placeBuild(stage, dir, res); err != nil {
		return nil, sum, err
	}
	built := time.Since(started)

	s.stopLive()
	p, err := s.launch(ctx, exe)
	if err != nil {
		return nil, sum, err
	}
	verb := "started"
	if running != ([32]byte{}) {
		verb = "reloaded"
	}
	logf("%s (built in %.1fs, ready in %.1fs)", verb, built.Seconds(), (time.Since(started) - built).Seconds())
	return p, sum, nil
}

// hashEntitlements writes the entitlements that sign the app and code among
// its resources to w, to tell whether they changed.
func hashEntitlements(w io.Writer, c *Config) error {
	files := map[string]string{"": c.MacOS.Entitlements} // "": the app
	maps.Copy(files, c.MacOS.HelperEntitlements)
	for _, name := range slices.Sorted(maps.Keys(files)) {
		if files[name] == "" {
			continue
		}
		b, err := os.ReadFile(c.path(files[name]))
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%s\x00%d\x00%s", name, len(b), b)
	}
	return nil
}

// placeBuild moves the executable in stage and the resources res next to
// it into dir, and removes what an earlier build left there and this one
// does not have.
func placeBuild(stage, dir string, res []resource) error {
	if err := copyResources(res, stage); err != nil {
		return err
	}
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	placed := map[string]bool{}
	for _, e := range entries {
		if err := replacePath(filepath.Join(stage, e.Name()), filepath.Join(dir, e.Name())); err != nil {
			return err
		}
		placed[e.Name()] = true
	}
	entries, err = os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !placed[e.Name()] && !hiddenName(e.Name()) {
			_ = os.RemoveAll(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

// windowsResources puts the resources of the development app (icon,
// manifest, version) in the main package for one build, as mygo build does:
// its windows take the executable's icon. It returns the function that
// removes them.
func (s *devSession) windowsResources(c, dc *Config) (cleanup func(), err error) {
	if c.Main != s.mainFor || s.mainDir == "" {
		dir, err := packageDir(c)
		if err != nil {
			return nil, err
		}
		s.mainFor, s.mainDir = c.Main, dir
	}
	return windowsResources(dc, s.mainDir, runtime.GOARCH)
}

// icon returns the app icon as .icns, rendering it only when it changed.
func (s *devSession) icon(c *Config) ([]byte, error) {
	if c.Icon == "" {
		return nil, nil
	}
	info, err := os.Stat(c.path(c.Icon))
	if err != nil {
		return nil, err
	}
	key := fmt.Sprint(c.path(c.Icon), info.Size(), info.ModTime().UnixNano())
	if key != s.iconKey {
		icns, err := appIcon(c)
		if err != nil {
			return nil, err
		}
		s.iconKey, s.icns = key, icns
	}
	return s.icns, nil
}

// launch starts a build and waits until it reports ready: the app connects
// to the Unix socket passed in MYGO_READY_SOCKET once its first window is
// ready to show. A build that relaunches itself first is started again.
func (s *devSession) launch(ctx context.Context, exe string) (*devProcess, error) {
	for range 10 {
		p, err := s.launchOnce(ctx, exe)
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != devRelaunchCode {
			return p, err
		}
		logf("relaunching") // mygo.App.Relaunch before the app was ready
	}
	return nil, errors.New("the app relaunched itself 10 times without getting ready")
}

func (s *devSession) launchOnce(ctx context.Context, exe string) (*devProcess, error) {
	s.launches++
	sock := readySocketPath(s.launches)
	_ = os.Remove(sock)
	ln, err := net.Listen("unix", sock)
	if err != nil {
		return nil, err
	}
	defer ln.Close() // also removes the socket file
	ready := make(chan struct{})
	go func() {
		if conn, err := ln.Accept(); err == nil {
			conn.Close()
			close(ready)
		}
	}()

	cmd := exec.Command(exe)
	cmd.Dir = s.root
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(append(os.Environ(), s.env...), "MYGO_READY_SOCKET="+sock)
	setProcessGroup(cmd)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &devProcess{exe: exe, cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()

	wait := s.readyTimeout
	if wait == 0 {
		wait = 20 * time.Second
	}
	timeout := time.NewTimer(wait)
	defer timeout.Stop()
	select {
	case <-ready:
		return p, nil
	case <-p.done:
		if p.err == nil {
			return nil, errors.New("the app exited before it was ready")
		}
		return nil, fmt.Errorf("the app exited before it was ready: %w", p.err)
	case <-timeout.C:
		p.stop()
		return nil, fmt.Errorf("the app did not get ready within %v", wait)
	case <-ctx.Done():
		p.stop()
		return nil, ctx.Err()
	}
}

// readySocketPath returns a socket path short enough for Unix sockets
// (about 100 bytes).
func readySocketPath(n int) string {
	name := fmt.Sprintf("mygo-ready-%d-%d.sock", os.Getpid(), n)
	dir := os.TempDir()
	if len(dir)+len(name) >= 100 {
		dir = "/tmp"
	}
	return filepath.Join(dir, name)
}

// devProcess is a running development build.
type devProcess struct {
	exe  string
	cmd  *exec.Cmd
	done chan struct{} // closed when the process exited
	err  error         // how it exited, set before done is closed
	// replaced is set when the build is stopped for the next one.
	replaced atomic.Bool
}

// stopGrace is how long a stopped build may take to quit.
var stopGrace = 3 * time.Second

// stop asks the app to quit, like the Quit menu item, and kills it when it
// has not exited after stopGrace. Only the app is asked: it ends the
// processes it started, such as its web view's, which could lose what they
// had not written yet if asked along with it. Those it leaves are killed.
func (p *devProcess) stop() {
	select {
	case <-p.done:
		return
	default:
	}
	interrupt(p.cmd)
	select {
	case <-p.done:
	case <-time.After(stopGrace):
	}
	kill(p.cmd)
	<-p.done
}
