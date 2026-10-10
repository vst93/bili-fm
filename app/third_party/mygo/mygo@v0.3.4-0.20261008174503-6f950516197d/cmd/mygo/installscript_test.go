package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/egoist/mygo/internal/update"
)

func TestInstallScript(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runs install.sh")
	}
	if os.Geteuid() == 0 {
		t.Skip("install.sh refuses to install for root")
	}
	// uname answers as an x86-64 Linux machine would.
	bin := t.TempDir()
	uname := "#!/bin/sh\ncase \"$1\" in -s) echo Linux ;; -m) echo x86_64 ;; esac\n"
	if err := os.WriteFile(filepath.Join(bin, "uname"), []byte(uname), 0o755); err != nil {
		t.Fatal(err)
	}
	// ldconfig lists $LD_CACHE as the library cache, and fails without it.
	ldconfig := "#!/bin/sh\n[ \"$1\" = -p ] && [ -n \"${LD_CACHE:-}\" ] || exit 1\nprintf '%s' \"$LD_CACHE\"\n"
	if err := os.WriteFile(filepath.Join(bin, "ldconfig"), []byte(ldconfig), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("LD_CACHE", "")
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	home := filepath.Join(t.TempDir(), "Jane Doe")
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "mygo.json"), []byte(`{
		"name": "My App",
		"icon": "icon.png",
		"urlSchemes": ["my-app"],
		"fileAssociations": [{"ext": ["note"], "name": "Note"}]
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dir, "icon.png"), []byte("png"), 0o644)
	// build stages a fake app of version and writes its archive and
	// install.sh, returning where they are.
	build := func(c *Config, version string) string {
		t.Helper()
		c.Version = version
		stage := filepath.Join(t.TempDir(), "linux-amd64")
		files := map[string]string{
			"my-app":                "#!/bin/sh\necho " + version + "\n",
			"only-" + version:       "",
			"resources/config.json": "{}",
		}
		for name, content := range files {
			os.MkdirAll(filepath.Join(stage, filepath.Dir(name)), 0o755)
			if err := os.WriteFile(filepath.Join(stage, name), []byte(content), 0o755); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := writeLinuxDesktop(c, stage, "my-app"); err != nil {
			t.Fatal(err)
		}
		entries, _ := os.ReadDir(stage)
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		if _, err := writeArchive(c, stage, "linux-amd64", names); err != nil {
			t.Fatal(err)
		}
		if _, err := writeInstallScript(c, stage); err != nil {
			t.Fatal(err)
		}
		return stage
	}
	run := func(script string, args ...string) string {
		t.Helper()
		out, err := exec.Command("sh", append([]string{script}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("install.sh %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}
	appDir := filepath.Join(home, ".local", "my-app.app")
	binDir := filepath.Join(home, ".local", "bin")
	installed := func(version string) {
		t.Helper()
		if out, err := exec.Command(filepath.Join(appDir, "my-app")).Output(); err != nil || string(out) != version+"\n" {
			t.Errorf("my-app printed %q, %v, want %s", out, err, version)
		}
		if _, err := os.Stat(filepath.Join(appDir, "resources", "config.json")); err != nil {
			t.Error(err)
		}
		entries, _ := os.ReadDir(appDir)
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "only-") && e.Name() != "only-"+version {
				t.Errorf("%s of another version stayed", e.Name())
			}
		}
		entry, _ := os.ReadFile(filepath.Join(home, ".local", "share", "applications", "my-app.desktop"))
		for _, line := range []string{"Name=My App", `Exec="` + appDir + `/my-app" %U`, "Icon=" + appDir + "/my-app.png", "MimeType=application/x-my-app-note;x-scheme-handler/my-app;"} {
			if !strings.Contains(string(entry), line+"\n") {
				t.Errorf("the desktop entry has no %s:\n%s", line, entry)
			}
		}
		if mime, _ := os.ReadFile(filepath.Join(home, ".local", "share", "mime", "packages", "my-app.xml")); !strings.Contains(string(mime), `<glob pattern="*.note"/>`) {
			t.Errorf("MIME package:\n%s", mime)
		}
		// Updates register the entry again as install.sh does.
		os.WriteFile(filepath.Join(home, ".local", "share", "applications", "my-app.desktop"), append(entry, "X-Stale=true\n"...), 0o644)
		if err := update.RefreshDesktopEntry(appDir, "my-app"); err != nil {
			t.Fatal(err)
		}
		if again, _ := os.ReadFile(filepath.Join(home, ".local", "share", "applications", "my-app.desktop")); string(again) != string(entry) {
			t.Errorf("updates register the entry as\n%s\ninstall.sh as\n%s", again, entry)
		}
	}

	// Without updates, the archive next to the script.
	c, err := loadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	local := build(c, "1.0.0")
	script, _ := os.ReadFile(filepath.Join(local, "install.sh"))
	if strings.Contains(string(script), "curl -fsSL http") {
		t.Errorf("install.sh without updates tells to download it:\n%s", script)
	}
	// Without linux.command, no command, and none left from earlier
	// scripts, which linked one named after the app.
	os.MkdirAll(binDir, 0o755)
	if err := os.Symlink(filepath.Join(appDir, "my-app"), filepath.Join(binDir, "my-app")); err != nil {
		t.Fatal(err)
	}
	if out := run(filepath.Join(local, "install.sh")); !strings.HasSuffix(out, "Installed My App: open it from the applications menu\n") {
		t.Errorf("install.sh printed:\n%s", out)
	}
	if entries, _ := os.ReadDir(binDir); len(entries) != 0 {
		t.Errorf("~/.local/bin holds %v", entries)
	}
	installed("1.0.0")

	// With updates, the latest version, which the manifest names.
	if _, err := exec.LookPath("curl"); err != nil {
		if _, err := exec.LookPath("wget"); err != nil {
			t.Skip("downloading needs curl or wget")
		}
	}
	pub, priv, _ := ed25519.GenerateKey(nil)
	t.Setenv("MYGO_UPDATER_PRIVATE_KEY", base64.StdEncoding.EncodeToString(priv))
	served := t.TempDir()
	srv := httptest.NewServer(http.FileServer(http.Dir(served)))
	defer srv.Close()
	none := 0
	c.Updates = &Updates{PublicKey: base64.StdEncoding.EncodeToString(pub), URL: srv.URL, Deltas: &none}
	c.Linux.Command = "my-app"
	published := build(c, "1.1.0")
	for _, name := range []string{"my-app-1.1.0-linux-amd64.tar.gz", "update-linux-amd64.json"} {
		b, err := os.ReadFile(filepath.Join(published, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(served, name), b, 0o644)
	}
	script, _ = os.ReadFile(filepath.Join(published, "install.sh"))
	if !strings.Contains(string(script), "curl -fsSL "+srv.URL+"/install.sh | sh") {
		t.Errorf("install.sh does not tell how to run it:\n%s", script)
	}
	alone := filepath.Join(t.TempDir(), "install.sh")
	os.WriteFile(alone, script, 0o755)
	if out := run(alone); !strings.Contains(out, "Downloading My App 1.1.0") || !strings.Contains(out, "run "+filepath.Join(binDir, "my-app")+"\n") {
		t.Errorf("install.sh printed:\n%s", out)
	}
	installed("1.1.0")
	// The command of linux.command.
	if out, err := exec.Command(filepath.Join(binDir, "my-app")).Output(); err != nil || string(out) != "1.1.0\n" {
		t.Errorf("the my-app command printed %q, %v", out, err)
	}

	// A command that another program has is left alone.
	notes := filepath.Join(binDir, "notes")
	os.WriteFile(notes, []byte("#!/bin/sh\necho notes\n"), 0o755)
	c.Linux.Command = "notes"
	withNotes := t.TempDir()
	if _, err := writeInstallScript(c, withNotes); err != nil {
		t.Fatal(err)
	}
	if out := run(filepath.Join(withNotes, "install.sh"), filepath.Join(local, "my-app-1.0.0-linux-amd64.tar.gz")); !strings.Contains(out, "Leaving "+notes+" alone: it is not My App's\n") ||
		!strings.HasSuffix(out, "open it from the applications menu\n") {
		t.Errorf("install.sh printed:\n%s", out)
	}
	if b, _ := os.ReadFile(notes); string(b) != "#!/bin/sh\necho notes\n" {
		t.Errorf("the notes command became %q", b)
	}

	// The archive given, over the installed version.
	run(alone, filepath.Join(local, "my-app-1.0.0-linux-amd64.tar.gz"))
	installed("1.0.0")

	// It warns when the library cache has no WebKitGTK, and only then.
	libc := "\tlibc.so.6 (libc6,x86-64, OS ABI: Linux 3.2.0) => /lib/x86_64-linux-gnu/libc.so.6\n"
	for _, cache := range []struct {
		libs  string
		warns bool
	}{
		{libs: "", warns: false}, // no cache
		{libs: "0 libs found in cache `/etc/ld.so.cache'\n", warns: false},
		{libs: libc + "\tlibwebkit2gtk-4.1.so.0 (libc6,x86-64) => /lib/x86_64-linux-gnu/libwebkit2gtk-4.1.so.0\n", warns: false},
		{libs: libc + "\tlibwebkit2gtk-4.0.so.37 (libc6,x86-64) => /lib/x86_64-linux-gnu/libwebkit2gtk-4.0.so.37\n", warns: false},
		{libs: libc + "\tlibgtk-3.so.0 (libc6,x86-64) => /lib/x86_64-linux-gnu/libgtk-3.so.0\n", warns: true},
	} {
		t.Setenv("LD_CACHE", cache.libs)
		out := run(alone, filepath.Join(local, "my-app-1.0.0-linux-amd64.tar.gz"))
		if warns := strings.Contains(out, "My App needs WebKitGTK, which is not installed. Install it with:\n  "); warns != cache.warns {
			t.Errorf("with the library cache\n%s\ninstall.sh printed:\n%s", cache.libs, out)
		}
	}
	t.Setenv("LD_CACHE", "")
	installed("1.0.0")
	// The libraries it looks for are those the app loads.
	ffi, err := os.ReadFile(filepath.Join("..", "..", "internal", "linux", "ffi.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(ffi), `open("libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37")`) {
		t.Error("install.sh looks for other WebKitGTK libraries than the app loads")
	}

	// Uninstalling removes what install.sh and the app made, and only that.
	handler := filepath.Join(home, ".local", "share", "applications", "com.mygo.my-app.url-handler.desktop")
	other := filepath.Join(home, ".local", "share", "applications", "other.url-handler.desktop")
	os.WriteFile(handler, []byte(`Exec="`+appDir+`/my-app" %u`+"\n"), 0o644)
	os.WriteFile(other, []byte("Exec=/usr/bin/other %u\n"), 0o644)
	run(alone, "--uninstall")
	for _, p := range []string{appDir, filepath.Join(home, ".local", "bin", "my-app"), handler,
		filepath.Join(home, ".local", "share", "applications", "my-app.desktop"),
		filepath.Join(home, ".local", "share", "mime", "packages", "my-app.xml")} {
		if _, err := os.Lstat(p); err == nil {
			t.Errorf("%s is still there", p)
		}
	}
	if _, err := os.Stat(other); err != nil {
		t.Errorf("the entry of another app was removed: %v", err)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Errorf("the command of another program was removed: %v", err)
	}
	if out, err := exec.Command("sh", alone, "--uninstall").CombinedOutput(); err == nil {
		t.Errorf("uninstalling twice succeeded:\n%s", out)
	}
}

func TestInstallScriptTagged(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runs install.sh")
	}
	if os.Geteuid() == 0 {
		t.Skip("install.sh refuses to install for root")
	}
	// uname answers as an x86-64 Linux machine would, and curl serves the
	// GitHub API and release downloads of me/my-app from $SERVED.
	bin := t.TempDir()
	scripts := map[string]string{
		"uname": "#!/bin/sh\ncase \"$1\" in -s) echo Linux ;; -m) echo x86_64 ;; esac\n",
		"curl": `#!/bin/sh
for url; do :; done
case "$url" in
"https://api.github.com/repos/me/my-app/releases?per_page=100&page="*) file="$SERVED/releases-${url##*page=}.json" ;;
"https://github.com/me/my-app/releases/download/"*) file="$SERVED/${url#https://github.com/me/my-app/releases/download/}" ;;
*) exit 22 ;;
esac
[ -f "$file" ] || exit 22
cat "$file"
`,
	}
	for name, script := range scripts {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	served := t.TempDir()
	t.Setenv("SERVED", served)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")

	pub, priv, _ := ed25519.GenerateKey(nil)
	t.Setenv("MYGO_UPDATER_PRIVATE_KEY", base64.StdEncoding.EncodeToString(priv))
	none := 0
	c := &Config{root: t.TempDir(), Name: "My App", Version: "1.1.0",
		Updates: &Updates{PublicKey: base64.StdEncoding.EncodeToString(pub), GitHub: "me/my-app", TagPrefix: "desktop-v", Deltas: &none}}
	stage := filepath.Join(t.TempDir(), "linux-amd64")
	os.MkdirAll(stage, 0o755)
	if err := os.WriteFile(filepath.Join(stage, "my-app"), []byte("#!/bin/sh\necho 1.1.0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := writeArchive(c, stage, "linux-amd64", []string{"my-app"}); err != nil {
		t.Fatal(err)
	}
	script, err := writeInstallScript(c, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	release := filepath.Join(served, "desktop-v1.1.0")
	os.MkdirAll(release, 0o755)
	for _, name := range []string{"my-app-1.1.0-linux-amd64.tar.gz", "update-linux-amd64.json"} {
		b, err := os.ReadFile(filepath.Join(stage, name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(release, name), b, 0o644)
	}
	// The API's releases, newest first, as it prints them.
	rel := func(tag string, draft, pre bool) string {
		return fmt.Sprintf("  {\n    \"tag_name\": %q,\n    \"name\": \"not \\\"tag_name\\\": \\\"desktop-v9.0.0\\\"\",\n    \"draft\": %t,\n    \"prerelease\": %t\n  }", tag, draft, pre)
	}
	page := func(releases ...string) {
		os.WriteFile(filepath.Join(served, "releases-1.json"), []byte("[\n"+strings.Join(releases, ",\n")+"\n]\n"), 0o644)
	}

	page(rel("cli-v2.0.0", false, false))
	if out, err := exec.Command("sh", script).CombinedOutput(); err == nil || !strings.Contains(string(out), "no release of My App is published") {
		t.Errorf("without a release, install.sh printed %v:\n%s", err, out)
	}
	page(rel("cli-v2.0.0", false, false), rel("desktop-v1.2.0", true, false), rel("desktop-v1.2.0-beta.1", false, true),
		rel("desktop-v1.1.0", false, false), rel("desktop-v1.0.0", false, false))
	out, err := exec.Command("sh", script).CombinedOutput()
	if err != nil || !strings.Contains(string(out), "Downloading My App 1.1.0") {
		t.Fatalf("install.sh printed %v:\n%s", err, out)
	}
	if out, err := exec.Command(filepath.Join(home, ".local", "my-app.app", "my-app")).Output(); err != nil || string(out) != "1.1.0\n" {
		t.Errorf("my-app printed %q, %v", out, err)
	}
}
