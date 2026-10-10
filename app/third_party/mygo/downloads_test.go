package mygo

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDownloadEvents(t *testing.T) {
	dir := t.TempDir()
	App.SetPath(PathDownloads, dir)
	defer App.SetPath(PathDownloads, "")
	w, fw := testWindow(t, WindowOptions{})

	// The default path is the suggested name in Downloads, numbered.
	os.WriteFile(filepath.Join(dir, "report.csv"), nil, 0o644)
	start := func(url, name string) string {
		return onMainValue(func() string { return fw.H.DownloadStarted(url, name) })
	}
	if got := start("https://x/r", "report.csv"); got != filepath.Join(dir, "report (2).csv") {
		t.Errorf("path = %q", got)
	}
	if got := start("https://x/r", "../../etc/passwd"); got != filepath.Join(dir, "passwd") {
		t.Errorf("unsafe name: path = %q", got)
	}
	off := w.Page().OnWillDownload(func(e *DownloadEvent) { e.PreventDefault() })
	if got := start("https://x/r", "a.txt"); got != "" {
		t.Errorf("a prevented download got path %q", got)
	}
	off()

	// Custom schemes are downloaded from their handler.
	if err := Protocol.HandleFunc("dltest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", `attachment; filename="data.json"`)
		w.Write([]byte(`{"ok":true}`))
	}); err != nil {
		t.Fatal(err)
	}
	defer Protocol.Unhandle("dltest")
	done := make(chan *Download, 1)
	defer w.Page().OnDownloadDone(func(d *Download) { done <- d })()
	onMain(func() { fw.H.SchemeDownload("dltest://localhost/export") })
	select {
	case d := <-done:
		if d.Err != nil || filepath.Base(d.Path) != "data.json" {
			t.Fatalf("download = %+v", d)
		}
		if b, _ := os.ReadFile(d.Path); string(b) != `{"ok":true}` {
			t.Errorf("saved %q", b)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("the scheme download did not finish")
	}
}
