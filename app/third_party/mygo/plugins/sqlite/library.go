package sqlite

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

//go:generate go run ./internal/libbuild

//go:embed mygo-plugin.json
var natives []byte

var load struct {
	once sync.Once
	err  error
}

// Load loads the plugin's SQLite library once. Open calls it lazily, so
// binding the plugin and generating clients require no native library.
func Load() error {
	load.once.Do(func() {
		if !supported {
			load.err = fmt.Errorf("sqlite: %s/%s: %w", runtime.GOOS, runtime.GOARCH, errors.ErrUnsupported)
			return
		}
		path, err := LibraryPath()
		if err == nil {
			err = loadNative(path)
		}
		load.err = err
	})
	return load.err
}

type nativeFile struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// LibraryPath finds the library named by MYGO_SQLITE_LIBRARY, the app's
// resources, or the executable's directory, then the CLI's SHA-256-checked
// cache. Unpackaged programs download a missing library into that cache;
// packaged apps require the library that mygo dev/build put into them.
func LibraryPath() (string, error) {
	if path := os.Getenv("MYGO_SQLITE_LIBRARY"); path != "" {
		return filepath.Abs(path)
	}
	var m struct {
		Libraries []struct {
			Name  string                `json:"name"`
			Files map[string]nativeFile `json:"files"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(natives, &m); err != nil {
		return "", err
	}
	var file nativeFile
	for _, lib := range m.Libraries {
		if lib.Name == "mygo-sqlite3" {
			file = lib.Files[runtime.GOOS+"-"+runtime.GOARCH]
		}
	}
	sum, err := hex.DecodeString(file.SHA256)
	if err != nil || len(sum) != sha256.Size || file.SHA256 != hex.EncodeToString(sum) || file.URL == "" ||
		file.Name == "" || file.Name == "." || file.Name == ".." || filepath.Base(file.Name) != file.Name || strings.ContainsAny(file.Name, `/\`) {
		return "", fmt.Errorf("sqlite: no valid library for %s/%s; run go generate ./plugins/sqlite", runtime.GOOS, runtime.GOARCH)
	}
	var dirs []string
	if dir, err := mygo.App.Path(mygo.PathResources); err == nil {
		dirs = append(dirs, dir)
	}
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}
	for _, dir := range dirs {
		path := filepath.Join(dir, file.Name)
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			return path, nil
		}
	}
	if mygo.App.IsPackaged() {
		return "", fmt.Errorf("sqlite: %s is missing from the app; build it with mygo build", file.Name)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(cache, "mygo", "natives", file.SHA256, file.Name)
	if checkFile(path, file.SHA256) {
		return path, nil
	}
	if err := downloadLibrary(file, path); err != nil {
		return "", err
	}
	return path, nil
}

func checkFile(path, digest string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	h := sha256.New()
	_, err = io.Copy(h, f)
	return err == nil && hex.EncodeToString(h.Sum(nil)) == digest
}

func downloadLibrary(file nativeFile, path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, file.URL, nil)
	if err != nil {
		return err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sqlite: downloading %s: %w", file.Name, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sqlite: downloading %s: %s", file.URL, res.Status)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".download-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(res.Body, 64<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != file.SHA256 {
		return fmt.Errorf("sqlite: %s: SHA-256 mismatch", file.URL)
	}
	if err := os.Chmod(f.Name(), 0o755); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), path); err != nil && !checkFile(path, file.SHA256) {
		return err
	}
	return nil
}
