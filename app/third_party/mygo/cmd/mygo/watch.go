package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"
)

// buildInputs is what building the app reads, which is what mygo dev
// watches: the frontend is the dev server's business, so editing it never
// rebuilds the app. Resources are copied into the app, so they count.
type buildInputs struct {
	sourceDirs []string // directories of compiled packages: their .go and .s files
	fileDirs   []string // directories of embedded files: all their files
	files      []string // go.mod and go.sum files, the configuration, the icon, entitlements
	// resources is the resources directory: everything in it but the
	// platform directories of other platforms.
	resources string
	trees     []string // the listed resources: everything in them
}

// listBuildInputs asks go list for the packages the app is built from,
// leaving out the standard library and the module cache, which do not
// change.
func listBuildInputs(c *Config) (*buildInputs, error) {
	var out, stderr bytes.Buffer
	cmd := goCommand(c.root, nil, "list", "-e", "-deps", "-json=Dir,Standard,EmbedFiles,Module", c.Main)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}
	modcache := goEnv("GOMODCACHE")
	outside := func(dir string) bool {
		return dir == "" || modcache != "" && strings.HasPrefix(dir, modcache+string(filepath.Separator))
	}
	in := &buildInputs{
		files:     []string{filepath.Join(c.root, jsonConfig), filepath.Join(c.root, tsConfig)},
		resources: c.path(resourcesDir),
	}
	if c.Icon != "" {
		in.files = append(in.files, c.path(c.Icon))
	}
	if c.MacOS.Entitlements != "" {
		in.files = append(in.files, c.path(c.MacOS.Entitlements))
	}
	for _, f := range c.MacOS.HelperEntitlements {
		in.files = append(in.files, c.path(f))
	}
	for _, p := range c.Resources {
		in.trees = append(in.trees, c.path(p))
	}
	dec := json.NewDecoder(&out)
	for {
		var p struct {
			Dir        string
			Standard   bool
			EmbedFiles []string
			Module     *struct{ GoMod string }
		}
		if err := dec.Decode(&p); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("go list: %w", err)
		}
		if p.Standard || outside(p.Dir) {
			continue
		}
		in.sourceDirs = append(in.sourceDirs, p.Dir)
		for _, f := range p.EmbedFiles {
			path := filepath.Join(p.Dir, f)
			in.files = append(in.files, path)
			in.fileDirs = append(in.fileDirs, filepath.Dir(path))
		}
		if p.Module != nil && p.Module.GoMod != "" && !outside(filepath.Dir(p.Module.GoMod)) {
			in.files = append(in.files, p.Module.GoMod, filepath.Join(filepath.Dir(p.Module.GoMod), "go.sum"))
		}
	}
	for _, s := range []*[]string{&in.sourceDirs, &in.fileDirs, &in.files, &in.trees} {
		slices.Sort(*s)
		*s = slices.Compact(*s)
	}
	return in, nil
}

var goEnvCache sync.Map

func goEnv(key string) string {
	if v, ok := goEnvCache.Load(key); ok {
		return v.(string)
	}
	out, err := exec.Command("go", "env", key).Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	goEnvCache.Store(key, v)
	return v
}

// watcher detects changes to the build inputs by polling. Stat'ing a few
// hundred files a few times a second is cheap and needs no platform APIs.
type watcher struct {
	mu     sync.Mutex
	inputs *buildInputs
	reset  bool // inputs changed: take a new baseline
}

// set replaces what is watched. The same inputs keep their baseline, so
// that edits made during a build still count as changes.
func (w *watcher) set(in *buildInputs) {
	w.mu.Lock()
	if w.inputs == nil || !w.inputs.equal(in) {
		w.inputs, w.reset = in, true
	}
	w.mu.Unlock()
}

func (in *buildInputs) equal(o *buildInputs) bool {
	return slices.Equal(in.sourceDirs, o.sourceDirs) && slices.Equal(in.fileDirs, o.fileDirs) &&
		slices.Equal(in.files, o.files) && in.resources == o.resources && slices.Equal(in.trees, o.trees)
}

func fingerprint(in *buildInputs) uint64 {
	h := fnv.New64a()
	add := func(path string, info os.FileInfo) {
		fmt.Fprintf(h, "%s\x00%v\x00%d\x00%d\x00", path, info.Mode(), info.Size(), info.ModTime().UnixNano())
	}
	list := func(dir string, keep func(name string) bool) {
		entries, _ := readDir(dir)
		for _, e := range entries {
			if e.IsDir() || !keep(e.Name()) {
				continue
			}
			if info, err := e.Info(); err == nil {
				add(filepath.Join(dir, e.Name()), info)
			}
		}
	}
	for _, dir := range in.sourceDirs {
		list(dir, func(name string) bool {
			return !strings.HasSuffix(name, "_test.go") && (strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".s"))
		})
	}
	for _, dir := range in.fileDirs {
		// Builds write their Windows resources into the main package,
		// which may embed files too.
		list(dir, func(name string) bool { return !mygoSyso(name) })
	}
	for _, f := range in.files {
		if info, err := os.Stat(f); err == nil {
			add(f, info)
		} else {
			fmt.Fprintf(h, "%s\x00-\x00", f)
		}
	}
	tree := func(t string) {
		err := walkResource(t, func(path string, info fs.FileInfo) error {
			if info.IsDir() {
				// Its time changes with hidden files too.
				fmt.Fprintf(h, "%s\x00dir\x00", path)
			} else {
				add(path, info)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(h, "%s\x00-\x00", t)
		}
	}
	// The entries of the resources directory are followed like listed
	// resources, and so are those of the platform directories of this
	// platform, which is the one mygo dev builds for.
	var entries func(dir string, platforms bool)
	entries = func(dir string, platforms bool) {
		list, err := readDir(dir)
		if err != nil {
			fmt.Fprintf(h, "%s\x00-\x00", dir)
			return
		}
		for _, e := range list {
			path := filepath.Join(dir, e.Name())
			switch goos, goarch, ok := platformDir(e.Name()); {
			case hiddenName(e.Name()):
			case !platforms || !ok:
				tree(path)
			case goos == runtime.GOOS && (goarch == "" || goarch == runtime.GOARCH):
				entries(path, false)
			}
		}
	}
	if in.resources != "" {
		entries(in.resources, true)
	}
	for _, t := range in.trees {
		tree(t)
	}
	return h.Sum64()
}

// readDir is os.ReadDir, but lets others delete the directory while it
// reads it (openDir), as the watcher reads directories all the time.
func readDir(name string) ([]os.DirEntry, error) {
	f, err := openDir(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := f.ReadDir(-1)
	slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	return entries, err
}

// watch polls every interval and signals on the returned channel once
// inputs changed and then stayed unchanged for an interval, so that a burst
// of writes (a formatter, a branch switch) causes a single rebuild.
func (w *watcher) watch(ctx context.Context, interval time.Duration) <-chan struct{} {
	ch := make(chan struct{}, 1)
	go func() {
		var last uint64
		settling := false
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
			}
			w.mu.Lock()
			in, reset := w.inputs, w.reset
			w.reset = false
			w.mu.Unlock()
			if in == nil {
				continue
			}
			cur := fingerprint(in)
			switch {
			case reset:
				last, settling = cur, false
			case cur != last:
				last, settling = cur, true
			case settling:
				settling = false
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		}
	}()
	return ch
}
