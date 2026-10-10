package mygo

import (
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
)

func TestURLSchemes(t *testing.T) {
	defer func(s string) { packageURLSchemes = s }(packageURLSchemes)
	packageURLSchemes = "myapp,Other"
	if err := App.RegisterURLScheme("Run.Time+x"); err != nil {
		t.Fatal(err)
	}
	defer App.UnregisterURLScheme("run.time+x")
	for _, bad := range []string{"", "1abc", "a b", "a:b"} {
		if err := App.RegisterURLScheme(bad); err == nil {
			t.Errorf("RegisterURLScheme(%q) succeeded", bad)
		}
	}
	id := appID()
	if got := fb.URLSchemes["run.time+x"]; got != id+" "+App.Name() {
		t.Errorf("registered %q", got)
	}
	if !App.IsURLSchemeRegistered("run.time+x") || App.IsURLSchemeRegistered("myapp") {
		t.Error("IsURLSchemeRegistered")
	}

	args := []string{"--flag", "MyApp://a", "other:b", "run.time+x://c", "https://example.com", `C:\file.txt`, "nope://d", "myapp"}
	if got, want := urlArgs(args), []string{"MyApp://a", "other:b", "run.time+x://c"}; !slices.Equal(got, want) {
		t.Errorf("urlArgs = %q, want %q", got, want)
	}
	var opened []string
	off := App.OnOpenURL(func(u string) { opened = append(opened, u) })
	onMain(func() { deliverArgs(args, "") })
	off()
	if len(opened) != 3 {
		t.Errorf("OnOpenURL got %q", opened)
	}

	if err := App.UnregisterURLScheme("run.time+x"); err != nil {
		t.Fatal(err)
	}
	if handlesScheme("run.time+x") || App.IsURLSchemeRegistered("run.time+x") {
		t.Error("the scheme is still handled after UnregisterURLScheme")
	}
}

func TestPackageInfo(t *testing.T) {
	defer func(n, v, id string) { packageName, packageVersion, packageIdentifier = n, v, id }(packageName, packageVersion, packageIdentifier)
	if _, ok := packageInfo(); ok {
		t.Fatal("the test binary is not packaged")
	}
	packageName, packageVersion, packageIdentifier = "My App", "1.2.3", "com.example.app"
	if info, ok := packageInfo(); !ok || info.Name != "My App" || info.Version != "1.2.3" || !App.IsPackaged() {
		t.Errorf("linked package info = %+v %v", info, ok)
	}
	if appID() != "com.example.app" {
		t.Errorf("handler id = %q", appID())
	}
}

func TestOpenAtLogin(t *testing.T) {
	if App.OpenAtLogin() || App.WasOpenedAtLogin() {
		t.Fatal("the test binary does not start at login")
	}
	if err := App.SetOpenAtLogin(true); err != nil {
		t.Fatal(err)
	}
	if !App.OpenAtLogin() || fb.LoginItem != appID()+" "+App.Name()+" "+loginArg {
		t.Errorf("login item = %q", fb.LoginItem)
	}
	if err := App.SetOpenAtLogin(false); err != nil || App.OpenAtLogin() {
		t.Errorf("SetOpenAtLogin(false): %v", err)
	}

	args, ok := takeLoginArg([]string{"app", "-v", loginArg, "file.txt"})
	if !ok || !slices.Equal(args, []string{"app", "-v", "file.txt"}) {
		t.Errorf("takeLoginArg = %q, %v", args, ok)
	}
	if _, ok := takeLoginArg([]string{"app", "file.txt"}); ok {
		t.Error("takeLoginArg found the argument in a plain command line")
	}
}

func TestFileArgs(t *testing.T) {
	defer func(s string) { packageFileExtensions = s }(packageFileExtensions)
	packageFileExtensions = "md,txt"
	dir := t.TempDir()
	for _, f := range []string{"a.md", "b.TXT", "c.png"} {
		if err := os.WriteFile(filepath.Join(dir, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	fileURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(dir, "b.TXT"))}).String()
	if runtime.GOOS == "windows" {
		fileURL = "file:///" + filepath.ToSlash(filepath.Join(dir, "b.TXT"))
	}
	got := fileArgs([]string{"--flag", "a.md", fileURL, "c.png", "missing.md", dir}, dir)
	want := []string{filepath.Join(dir, "a.md"), filepath.Join(dir, "b.TXT")}
	if !slices.Equal(got, want) {
		t.Errorf("fileArgs = %q, want %q", got, want)
	}
	var opened []string
	off := App.OnOpenFile(func(p string) { opened = append(opened, p) })
	onMain(func() { deliverArgs([]string{"a.md"}, dir) })
	off()
	if len(opened) != 1 {
		t.Errorf("OnOpenFile got %q", opened)
	}
}
