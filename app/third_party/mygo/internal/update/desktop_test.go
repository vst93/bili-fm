package update

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestDesktopEntry(t *testing.T) {
	dir := t.TempDir()
	packaged := "[Desktop Entry]\nName=My App\nExec=my-app %U\nIcon=my-app\nX-Other=Exec=my-app\nTryExec=my-app-helper\nKeywords=a;b;"
	want := "[Desktop Entry]\nName=My App\nExec=\"" + dir + "/my-app\" %U\nIcon=my-app\nX-Other=Exec=my-app\nTryExec=my-app-helper\nKeywords=a;b;\n"
	if got := desktopEntry(packaged, dir, "my-app"); got != want {
		t.Errorf("without an icon:\n%s\nwant:\n%s", got, want)
	}
	os.WriteFile(filepath.Join(dir, "my-app.png"), nil, 0o644)
	want = "[Desktop Entry]\nName=My App\nExec=\"" + dir + "/my-app\"\nIcon=" + dir + "/my-app.png\n"
	if got := desktopEntry("[Desktop Entry]\nName=My App\nExec=my-app\nIcon=my-app\n", dir, "my-app"); got != want {
		t.Errorf("with an icon:\n%s\nwant:\n%s", got, want)
	}
	if got := desktopEntry("[Desktop Entry]\nExec=my-app-helper\n", dir, "my-app"); got != "[Desktop Entry]\nExec=my-app-helper\n" {
		t.Errorf("another executable: %q", got)
	}
}

func TestRefreshDesktopEntry(t *testing.T) {
	data := t.TempDir()
	t.Setenv("XDG_DATA_HOME", data)
	root := t.TempDir()
	dir := filepath.Join(root, "my-app.app")
	// The install as install.sh may know it, through a link.
	link := filepath.Join(t.TempDir(), "home")
	if err := os.Symlink(root, link); err != nil {
		// ERROR_PRIVILEGE_NOT_HELD: Windows creates links for
		// administrators and in developer mode only.
		if errors.Is(err, syscall.Errno(1314)) {
			t.Skip("creating a link needs a privilege:", err)
		}
		t.Fatal(err)
	}
	linked := filepath.Join(link, "my-app.app")
	entry := func(mime string) string {
		return "[Desktop Entry]\nType=Application\nName=My App\nExec=my-app %U\nIcon=my-app\nMimeType=" + mime + "\n"
	}
	os.MkdirAll(dir, 0o755)
	for name, content := range map[string]string{"my-app": "", "my-app.png": "", "my-app.desktop": entry("application/x-note;"), "my-app.xml": "<note/>"} {
		os.WriteFile(filepath.Join(dir, name), []byte(content), 0o755)
	}
	entryPath := filepath.Join(data, "applications", "my-app.desktop")
	mimePath := filepath.Join(data, "mime", "packages", "my-app.xml")
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}

	// Not installed by install.sh: nothing is registered.
	if err := RefreshDesktopEntry(dir, "my-app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(data, "applications")); err == nil {
		t.Error("registered an app that was not")
	}
	os.MkdirAll(filepath.Dir(entryPath), 0o755)
	other := "[Desktop Entry]\nExec=/opt/my-app/my-app %U\n"
	os.WriteFile(entryPath, []byte(other), 0o644)
	if err := RefreshDesktopEntry(dir, "my-app"); err != nil {
		t.Fatal(err)
	}
	if got := read(entryPath); got != other {
		t.Errorf("replaced the entry of another install:\n%s", got)
	}

	// Installed by install.sh: an update with more file types.
	os.WriteFile(entryPath, []byte(desktopEntry(entry("application/x-note;"), linked, "my-app")), 0o644)
	os.WriteFile(filepath.Join(dir, "my-app.desktop"), []byte(entry("application/x-note;application/x-todo;")), 0o644)
	os.WriteFile(filepath.Join(dir, "my-app.xml"), []byte("<note/><todo/>"), 0o644)
	if err := RefreshDesktopEntry(dir, "my-app"); err != nil {
		t.Fatal(err)
	}
	want := "[Desktop Entry]\nType=Application\nName=My App\nExec=\"" + linked + "/my-app\" %U\nIcon=" + linked + "/my-app.png\nMimeType=application/x-note;application/x-todo;\n"
	if got := read(entryPath); got != want {
		t.Errorf("entry:\n%s\nwant:\n%s", got, want)
	}
	if got := read(mimePath); got != "<note/><todo/>" {
		t.Errorf("MIME package %q", got)
	}

	// An update without file types.
	os.Remove(filepath.Join(dir, "my-app.xml"))
	if err := RefreshDesktopEntry(dir, "my-app"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mimePath); err == nil {
		t.Error("the MIME package of a version without file types stayed")
	}
}
