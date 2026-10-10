package mygo

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo/internal/update"
)

// Set by `mygo build` with -ldflags -X when mygo.config.ts configures updates:
// the manifest of this build's target, or with a tag prefix on GitHub its
// feed in the newest tagged release (update.TaggedFeed), and the public key
// updates must be signed with.
var (
	packageUpdateFeed string
	packageUpdateKey  string
)

// startExe is the executable the app started from. On Linux, os.Executable
// reports the old file once an update replaced it.
var startExe, _ = os.Executable()

// UpdaterModule installs new versions of the app, which `mygo build`
// publishes when mygo.config.ts configures updates: signed archives and a
// manifest per platform. Use the Updater singleton:
//
//	update, err := mygo.Updater.Check(ctx)
//	if err == nil && update != nil {
//		if err := update.Install(ctx, nil); err == nil {
//			mygo.App.Relaunch()
//		}
//	}
type UpdaterModule struct{}

// Updater installs new versions of the app.
var Updater = &UpdaterModule{}

// ErrUpdatesDisabled is returned by Check for builds without updates:
// development builds, and builds of apps without updates in mygo.config.ts.
var ErrUpdatesDisabled = errors.New("mygo: updates are not enabled for this build")

// Update is a newer version of the app, found by Updater.Check.
type Update struct {
	// Version of the update, and its release notes in Markdown.
	Version string
	Notes   string
	// Date is when the update was built.
	Date time.Time

	manifest update.Manifest
	key      string
	// current is the version that Check compared with, which delta updates
	// update.
	current string
}

// Enabled reports whether the app can update itself: it was built with
// updates and can write where it is installed. Apps installed by a package
// manager, such as a .deb in /opt, are updated by it instead.
func (u *UpdaterModule) Enabled() bool {
	if packageUpdateFeed == "" || packageUpdateKey == "" || IsDev() {
		return false
	}
	target, err := installTarget()
	if err != nil {
		return false
	}
	probe, err := os.MkdirTemp(filepath.Dir(target), ".mygo-update-")
	if err != nil {
		return false
	}
	os.Remove(probe)
	return true
}

// Check asks the update feed for a version newer than the running one and
// returns it, or nil when the app is up to date.
func (u *UpdaterModule) Check(ctx context.Context) (*Update, error) {
	if !u.Enabled() {
		return nil, ErrUpdatesDisabled
	}
	return checkUpdate(ctx, packageUpdateFeed, packageUpdateKey, App.Version())
}

func checkUpdate(ctx context.Context, feed, key, current string) (*Update, error) {
	feed, err := update.ResolveFeed(ctx, feed, httpGet)
	if err != nil {
		return nil, fmt.Errorf("mygo: checking for updates: %w", err)
	}
	body, err := httpGet(ctx, feed)
	if err != nil {
		return nil, fmt.Errorf("mygo: checking for updates: %w", err)
	}
	defer body.Close()
	var m update.Manifest
	if err := json.UnmarshalRead(io.LimitReader(body, update.MaxManifestSize), &m); err != nil {
		return nil, fmt.Errorf("mygo: checking for updates: %w", err)
	}
	if m.Version == "" || m.URL == "" || m.Signature == "" {
		return nil, errors.New("mygo: checking for updates: incomplete manifest")
	}
	if update.Compare(m.Version, current) <= 0 {
		return nil, nil
	}
	up := &Update{Version: m.Version, Notes: m.Notes, manifest: m, key: key, current: current}
	up.Date, _ = time.Parse(time.RFC3339, m.Date)
	return up, nil
}

// Install downloads the update, checks that it is signed with the app's
// key and replaces the app with it; progress, when not nil, is called with
// the bytes downloaded so far. When the update has a delta for the running
// version, only the delta is downloaded, unless it fails to make the new
// version: the whole update is downloaded then, and the progress starts
// over. The running app is not affected: the new version runs after
// App.Relaunch, or at the next launch.
//
// The app must be able to write where it is installed: a bundle in
// /Applications of an administrator, or an app directory the user owns on
// Linux and Windows. Apps installed by a package manager are updated by it.
func (up *Update) Install(ctx context.Context, progress func(downloaded, total int64)) error {
	target, err := installTarget()
	if err != nil {
		return err
	}
	if err := installUpdate(ctx, up.manifest, up.key, up.current, target, progress); err != nil {
		return fmt.Errorf("mygo: installing the update: %w", err)
	}
	if runtime.GOOS == "linux" {
		// install.sh registered the desktop entry of the version it installed.
		if err := update.RefreshDesktopEntry(target, filepath.Base(startExe)); err != nil {
			log.Printf("mygo: registering the desktop entry of the update: %v", err)
		}
	}
	return nil
}

// installTarget returns what an update replaces: the app bundle on macOS,
// the directory of the executable elsewhere.
func installTarget() (string, error) {
	exe := startExe
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	if runtime.GOOS == "darwin" {
		bundle := filepath.Dir(filepath.Dir(filepath.Dir(exe)))
		if !strings.HasSuffix(bundle, ".app") {
			return "", errors.New("mygo: only app bundles can update themselves")
		}
		return bundle, nil
	}
	if os.Getenv("APPIMAGE") != "" {
		return "", errors.New("mygo: AppImages cannot update themselves")
	}
	return filepath.Dir(exe), nil
}

// installUpdate installs the version of m in place of target, the app of
// version current: from its delta for current when there is one, else, or
// when the delta fails, from its archive.
func installUpdate(ctx context.Context, m update.Manifest, key, current, target string, progress func(downloaded, total int64)) error {
	pub, err := update.ParsePublicKey(key)
	if err != nil {
		return err
	}
	// Next to the app, so that it can be renamed into place.
	work, err := os.MkdirTemp(filepath.Dir(target), "."+filepath.Base(target)+".update-")
	if err != nil {
		return fmt.Errorf("cannot write next to the app: %w", err)
	}
	defer os.RemoveAll(work)
	unpacked := filepath.Join(work, "app")
	swap := swapFiles
	if runtime.GOOS == "darwin" {
		swap = swapBundle
	}

	if d := m.Delta(current); d != nil {
		err := applyDelta(ctx, *d, pub, m.Version, target, work, unpacked, progress)
		if err == nil {
			return swap(unpacked, target)
		}
		if ctx.Err() != nil {
			return err
		}
		log.Printf("mygo: downloading the whole update, as its delta update failed: %v", err)
		if err := os.RemoveAll(unpacked); err != nil {
			return err
		}
	}

	if m.Size <= 0 || m.Size > update.MaxArchiveSize {
		return fmt.Errorf("invalid size %d", m.Size)
	}
	archive := filepath.Join(work, "update.tar.gz")
	if err := download(ctx, m.URL, m.Size, m.Signature, pub, archive, progress); err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	err = update.ExtractArchive(f, unpacked)
	f.Close()
	if err != nil {
		return err
	}
	return swap(unpacked, target)
}

// applyDelta downloads the delta d into work and makes the new version of
// the app at target from it, in dir: its bundle on macOS, in dir too, as
// the bundle of an archive is.
func applyDelta(ctx context.Context, d update.Delta, pub ed25519.PublicKey, version, target, work, dir string, progress func(downloaded, total int64)) error {
	if d.Size <= 0 || d.Size > update.MaxArchiveSize {
		return fmt.Errorf("invalid size %d", d.Size)
	}
	path := filepath.Join(work, "update.delta")
	if err := download(ctx, d.URL, d.Size, d.Signature, pub, path, progress); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if runtime.GOOS == "darwin" {
		if err := os.Mkdir(dir, 0o755); err != nil {
			return err
		}
		dir = filepath.Join(dir, filepath.Base(target))
	}
	return update.ApplyDelta(f, d.Size, d.From, version, target, dir)
}

// download fetches url into the file path and checks its size and
// signature.
func download(ctx context.Context, url string, size int64, signature string, pub ed25519.PublicKey, path string, progress func(downloaded, total int64)) error {
	body, err := httpGet(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	sum := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, sum), &progressReader{r: io.LimitReader(body, size+1), total: size, fn: progress})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("downloading: %w", err)
	}
	if n != size {
		return fmt.Errorf("downloaded %d bytes, want %d", n, size)
	}
	return update.Verify(pub, sum.Sum(nil), signature)
}

// swapBundle replaces the bundle at target with the one in dir.
func swapBundle(dir, target string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 1 || !strings.HasSuffix(entries[0].Name(), ".app") {
		return errors.New("the update holds no app bundle")
	}
	old := filepath.Join(filepath.Dir(target), "."+filepath.Base(target)+".old-"+strconv.Itoa(os.Getpid()))
	if err := os.Rename(target, old); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(dir, entries[0].Name()), target); err != nil {
		_ = os.Rename(old, target)
		return err
	}
	// The running app keeps its open files.
	_ = os.RemoveAll(old)
	return nil
}

// swapFiles replaces the entries of the app directory target with those in
// dir. The running executable is renamed away, which Windows allows, and
// removed at the next launch (cleanUpdateLeftovers).
func swapFiles(dir, target string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("the update is empty")
	}
	suffix := ".old-" + strconv.Itoa(os.Getpid())
	var moved [][2]string // renamed away: old name, new name
	undo := func() {
		for i := len(moved) - 1; i >= 0; i-- {
			_ = os.RemoveAll(moved[i][0])
			_ = rename(moved[i][1], moved[i][0])
		}
	}
	for _, e := range entries {
		dst := filepath.Join(target, e.Name())
		if _, err := os.Lstat(dst); err == nil {
			aside := filepath.Join(target, "."+e.Name()+suffix)
			if err := rename(dst, aside); err != nil {
				undo()
				return err
			}
			moved = append(moved, [2]string{dst, aside})
		}
		if err := rename(filepath.Join(dir, e.Name()), dst); err != nil {
			undo()
			return err
		}
	}
	for _, m := range moved {
		_ = os.RemoveAll(m[1])
	}
	return nil
}

// rename renames a file of the app, retrying for a few seconds while
// Windows reports it in use: antivirus software opens new executables, such
// as the app that just started and the one of the update, to scan them.
func rename(from, to string) error {
	for i := 1; ; i++ {
		err := os.Rename(from, to)
		if err == nil || i == 10 || runtime.GOOS != "windows" ||
			// ERROR_SHARING_VIOLATION, or ERROR_ACCESS_DENIED for a directory
			// holding an open file.
			!errors.Is(err, syscall.Errno(32)) && !errors.Is(err, syscall.Errno(5)) {
			return err
		}
		time.Sleep(time.Duration(i) * 100 * time.Millisecond)
	}
}

// cleanUpdateLeftovers removes what an update could not remove while the
// previous version ran (Windows keeps running executables).
func cleanUpdateLeftovers() {
	if runtime.GOOS == "darwin" || startExe == "" {
		return
	}
	olds, _ := filepath.Glob(filepath.Join(filepath.Dir(startExe), ".*.old-*"))
	for _, p := range olds {
		_ = os.RemoveAll(p)
	}
}

// httpGet fetches a URL over HTTPS (or plain HTTP from the loopback
// interface, for tests).
func httpGet(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && isLoopback(u.Hostname())) {
		return nil, fmt.Errorf("refusing to download %s: updates need HTTPS", rawURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MyGo/"+Version+" "+App.Name()+"/"+App.Version())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", rawURL, resp.Status)
	}
	return resp.Body, nil
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// progressReader reports how much of a download was read.
type progressReader struct {
	r     io.Reader
	n     int64
	total int64
	fn    func(downloaded, total int64)
	last  time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.n += int64(n)
	if p.fn != nil && (err != nil || p.n == p.total || time.Since(p.last) > 100*time.Millisecond) {
		p.last = time.Now()
		p.fn(p.n, p.total)
	}
	return n, err
}
