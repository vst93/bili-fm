package mygo

import (
	"fmt"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// DownloadEvent is passed to Page.OnWillDownload listeners. Setting Path
// chooses where the file goes; preventing the event cancels the download.
type DownloadEvent struct {
	Preventable
	// URL of the download.
	URL string
	// SuggestedName is the file name the page or the server suggests.
	SuggestedName string
	// Path is where the file is saved: by default SuggestedName in the
	// Downloads directory, numbered when a file has that name. A file
	// already at a Path listeners set is replaced.
	Path string
}

// Download is a finished download, passed to Page.OnDownloadDone.
type Download struct {
	URL  string
	Path string
	// Err is why the download failed, or nil.
	Err error
}

// OnWillDownload is called when a page starts a download: a link with the
// download attribute, or a response the page cannot show, such as a file
// sent as an attachment. Without listeners, downloads are saved to the
// Downloads directory.
func (p *Page) OnWillDownload(fn func(e *DownloadEvent)) (off func()) {
	return p.w.onWillDownload.add(fn, false)
}

// OnDownloadDone is called when a download ended.
func (p *Page) OnDownloadDone(fn func(d *Download)) (off func()) {
	return p.w.onDownloadDone.add(fn, false)
}

func (h *windowHandler) DownloadStarted(url, suggested string) string {
	name := filepath.Base(filepath.Clean("/" + strings.ReplaceAll(suggested, `\`, "/")))
	if name == "/" || name == "." || name == "" {
		name = "download"
	}
	e := &DownloadEvent{URL: url, SuggestedName: name}
	if dir, err := App.Path(PathDownloads); err == nil && os.MkdirAll(dir, 0o755) == nil {
		e.Path = uniquePath(filepath.Join(dir, name))
	}
	fire1(&h.w.onWillDownload, e)
	if e.prevented || e.Path == "" {
		return ""
	}
	return e.Path
}

func (h *windowHandler) DownloadFinished(url, path string, err error) {
	fire1(&h.w.onDownloadDone, &Download{URL: url, Path: path, Err: err})
}

// uniquePath numbers path, like "name (2).ext", when a file has its name.
func uniquePath(path string) string {
	if _, err := os.Lstat(path); err != nil {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 2; ; i++ {
		p := base + " (" + strconv.Itoa(i) + ")" + ext
		if _, err := os.Lstat(p); err != nil {
			return p
		}
	}
}

func (h *windowHandler) SchemeDownload(rawURL string) { go downloadScheme(h, rawURL) }

// downloadScheme serves a URL of a custom scheme into a download.
func downloadScheme(h *windowHandler, rawURL string) {
	scheme, _, _ := strings.Cut(rawURL, ":")
	scheme = strings.ToLower(scheme)
	Protocol.mu.RLock()
	handler := Protocol.handlers[scheme]
	Protocol.mu.RUnlock()
	if handler == nil && scheme == frontendScheme {
		handler = frontendHandler()
	}
	finish := func(path string, err error) {
		postMain(func() { h.DownloadFinished(rawURL, path, err) })
	}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil || handler == nil {
		finish("", fmt.Errorf("mygo: cannot download %s", rawURL))
		return
	}
	rw := &downloadWriter{h: h, url: rawURL, header: http.Header{}}
	handler.ServeHTTP(rw, req)
	rw.WriteHeader(http.StatusOK) // nothing written
	if rw.file == nil {
		if rw.err != nil {
			finish("", rw.err)
		}
		return // canceled
	}
	if err := rw.file.Close(); rw.err == nil {
		rw.err = err
	}
	if rw.err != nil {
		os.Remove(rw.path)
	}
	finish(rw.path, rw.err)
}

// downloadWriter writes a response into the file OnWillDownload chose.
type downloadWriter struct {
	h      *windowHandler
	url    string
	header http.Header
	wrote  bool
	path   string
	file   *os.File
	err    error
}

func (d *downloadWriter) Header() http.Header { return d.header }

func (d *downloadWriter) WriteHeader(status int) {
	if d.wrote {
		return
	}
	d.wrote = true
	if status >= 400 {
		d.err = fmt.Errorf("mygo: downloading %s: %d %s", d.url, status, http.StatusText(status))
		return
	}
	name := ""
	if _, params, err := mime.ParseMediaType(d.header.Get("Content-Disposition")); err == nil {
		name = params["filename"]
	}
	if name == "" {
		if u, err := url.Parse(d.url); err == nil {
			name = path.Base(u.Path)
		}
	}
	d.path = onMainValue(func() string { return d.h.DownloadStarted(d.url, name) })
	if d.path == "" {
		return
	}
	d.file, d.err = os.Create(d.path)
	if d.err != nil {
		d.file = nil
	}
}

func (d *downloadWriter) Write(b []byte) (int, error) {
	d.WriteHeader(http.StatusOK)
	if d.file == nil || d.err != nil {
		return len(b), nil // canceled: drain
	}
	n, err := d.file.Write(b)
	if err != nil {
		d.err = err
	}
	return n, err
}
