package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// pluginFile is what a Go package that is a plugin of MyGo tells the CLI,
// next to its sources: the native libraries it needs, which the apps whose
// packages include it ship among their resources, downloaded once per user. The terminal plugin
// names libghostty-vt so:
//
//	{"libraries": [{"name": "libghostty-vt", "files": {
//		"darwin-arm64": {"name": "libghostty-vt.dylib", "url": "https://…", "sha256": "…"},
//		…
//	}}]}
//
// with a file per platform (GOOS-GOARCH), which the package loads from
// mygo.PathResources at run time.
const pluginFile = "mygo-plugin.json"

type nativeLibraries struct {
	Libraries []struct {
		Name  string                `json:"name"`
		Files map[string]nativeFile `json:"files"`
	} `json:"libraries"`
}

type nativeFile struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// appResources lists the resources of the app for goos/goarch (see
// resources), with the native libraries its packages need.
func (c *Config) appResources(goos, goarch string) ([]resource, error) {
	natives, err := c.natives(goos, goarch)
	if err != nil {
		return nil, err
	}
	return c.resourcesWith(goos, goarch, natives, reservedNames(c, goos)...)
}

// natives returns the native libraries the packages of the app for
// goos/goarch name in their mygo-plugin.json, downloaded into the user's
// cache, as resources: both architectures of a universal macOS app, which
// combine into one file.
func (c *Config) natives(goos, goarch string) ([]source, error) {
	arch := goarch
	if goarch == "universal" {
		arch = "arm64" // the packages are the same
	}
	var out, stderr bytes.Buffer
	cmd := goCommand(c.root, []string{"GOOS=" + goos, "GOARCH=" + arch}, "list", "-e", "-deps", "-f", "{{.Dir}}", c.Main)
	cmd.Stdout, cmd.Stderr = &out, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go list: %v\n%s", err, strings.TrimSpace(stderr.String()))
	}
	var list []source
	seen := map[string]bool{}
	for dir := range strings.Lines(out.String()) {
		dir = strings.TrimSpace(dir)
		data, err := os.ReadFile(filepath.Join(dir, pluginFile))
		if errors.Is(err, fs.ErrNotExist) || dir == "" {
			continue
		} else if err != nil {
			return nil, err
		}
		var n nativeLibraries
		if err := json.Unmarshal(data, &n); err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Join(dir, pluginFile), err)
		}
		for _, l := range n.Libraries {
			if seen[l.Name] {
				continue
			}
			seen[l.Name] = true
			pick := func(target string) (string, string, error) {
				f, ok := l.Files[target]
				if !ok {
					return "", "", fmt.Errorf("%s needs %s, which has no build for %s", dir, l.Name, strings.Replace(target, "-", "/", 1))
				}
				path, err := fetchNative(f)
				return f.Name, path, err
			}
			if goarch == "universal" {
				name, arm, err := pick("darwin-arm64")
				if err != nil {
					return nil, err
				}
				amdName, amd, err := pick("darwin-amd64")
				if err != nil {
					return nil, err
				}
				if amdName != name {
					return nil, fmt.Errorf("%s: %s is %s on arm64 but %s on amd64", filepath.Join(dir, pluginFile), l.Name, name, amdName)
				}
				list = append(list, source{name: name, path: arm, amd64: amd, listed: true})
				continue
			}
			name, path, err := pick(goos + "-" + goarch)
			if err != nil {
				return nil, err
			}
			list = append(list, source{name: name, path: path, listed: true})
		}
	}
	return list, nil
}

// fileSHA256 returns the SHA-256 of a file, in hex.
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fetchNative returns the path of a native library in the user's cache,
// downloading it there first when it is missing:
// <cache>/mygo/natives/<sha256>/<name>, where the packages that load it
// look too when they run outside an app built by the CLI.
func fetchNative(f nativeFile) (string, error) {
	// The SHA-256 and the name are parts of a path: a package may not
	// name another file than the library it downloads.
	sum, err := hex.DecodeString(f.SHA256)
	validName := f.Name != "" && f.Name != "." && f.Name != ".." && filepath.Base(f.Name) == f.Name && !strings.ContainsAny(f.Name, `/\`)
	if f.URL == "" || err != nil || len(sum) != sha256.Size || f.SHA256 != strings.ToLower(f.SHA256) || !validName {
		return "", fmt.Errorf("invalid native library %+v", f)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cache, "mygo", "natives", f.SHA256)
	path := filepath.Join(dir, f.Name)
	if fileExists(path) {
		// What the app ships must be what the package names.
		if got, err := fileSHA256(path); err == nil && got == f.SHA256 {
			return path, nil
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	logf("downloading %s", f.URL)
	tmp, err := os.CreateTemp(dir, ".download-*")
	if err != nil {
		return "", err
	}
	tmp.Close()
	defer os.Remove(tmp.Name())
	if err := download(f.URL, f.SHA256, tmp.Name()); err != nil {
		return "", fmt.Errorf("downloading %s: %w", f.Name, err)
	}
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		// Another process downloaded it meanwhile, and may have it loaded,
		// which Windows won't replace.
		if got, serr := fileSHA256(path); serr == nil && got == f.SHA256 {
			return path, nil
		}
		return "", err
	}
	return path, nil
}
