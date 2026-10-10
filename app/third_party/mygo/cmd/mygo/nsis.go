package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The Windows installer is built with NSIS: a per-user install in
// %LOCALAPPDATA%\Programs, where the app can update itself, with a Start
// menu shortcut, a desktop shortcut the finish page offers, and an
// uninstaller that Settings > Apps lists. Windows rarely has NSIS
// installed, so there mygo build downloads it when it needs it, as Tauri
// and electron-builder do; elsewhere makensis must be installed.

// nsisRelease is the NSIS that mygo build downloads: the official zip,
// checked against its SHA-256 and unpacked into the user's cache
// directory, where makensis runs as it does from an installation. The
// urls are copies of that zip, tried in order: the asset of the
// nsis-<version> release of MyGo's repository first, since SourceForge,
// where NSIS publishes it, has been down for hours at a time, its mirrors
// redirecting to it.
var nsisRelease = struct {
	version, sha256 string
	urls            []string
}{
	version: "3.13",
	sha256:  "ba63dffc4410ee89193e1cb5a41989991bd77c61068da17e3156d136b7b0b3d8",
	urls: []string{
		"https://github.com/egoist/mygo/releases/download/nsis-3.13/nsis-3.13.zip",
		"https://downloads.sourceforge.net/project/nsis/NSIS%203/3.13/nsis-3.13.zip",
	},
}

// makensis finds the NSIS compiler: an installed one or, on Windows, the
// one an earlier build downloaded.
func makensis() string {
	if p, err := exec.LookPath("makensis"); err == nil {
		return p
	}
	if runtime.GOOS == "windows" {
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles"} {
			if p := filepath.Join(os.Getenv(env), "NSIS", "makensis.exe"); fileExists(p) {
				return p
			}
		}
		if dir := nsisDir(); dir != "" {
			return nsisExecutable(dir)
		}
	}
	return ""
}

// nsisCompiler returns the NSIS compiler, which it downloads on Windows
// when there is none, or "" on other systems without NSIS.
func nsisCompiler() (string, error) {
	if p := makensis(); p != "" || runtime.GOOS != "windows" {
		return p, nil
	}
	dir := nsisDir()
	if dir == "" {
		return "", errors.New("no cache directory to download NSIS into: install NSIS (makensis)")
	}
	return downloadNSIS(dir)
}

// nsisDir is where mygo build keeps the NSIS it downloads.
func nsisDir() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	return filepath.Join(cache, "mygo", "nsis-"+nsisRelease.version)
}

// nsisExecutable returns makensis in the NSIS directory dir, or "".
func nsisExecutable(dir string) string {
	for _, p := range []string{filepath.Join(dir, "Bin", "makensis.exe"), filepath.Join(dir, "makensis.exe")} {
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// downloadNSIS downloads nsisRelease into dir, unless it is there already,
// and returns its makensis. The archive is unpacked next to dir and renamed
// into place, so dir is complete whenever it exists.
func downloadNSIS(dir string) (string, error) {
	if p := nsisExecutable(dir); p != "" {
		return p, nil
	}
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return "", err
	}
	work, err := os.MkdirTemp(parent, ".nsis-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)
	logf("downloading NSIS %s for the Windows installer", nsisRelease.version)
	archive := filepath.Join(work, "nsis.zip")
	if err := downloadAny(nsisRelease.urls, nsisRelease.sha256, archive); err != nil {
		return "", fmt.Errorf("downloading NSIS %s (or install it: https://nsis.sourceforge.io):\n%w", nsisRelease.version, err)
	}
	unpacked := filepath.Join(work, "nsis")
	if err := unzipTop(archive, unpacked); err != nil {
		return "", fmt.Errorf("unpacking NSIS: %w", err)
	}
	if nsisExecutable(unpacked) == "" {
		return "", errors.New("unpacking NSIS: the archive holds no makensis.exe")
	}
	if err := os.Rename(unpacked, dir); err != nil {
		// Another build may have downloaded it meanwhile; anything else
		// there is broken.
		if p := nsisExecutable(dir); p != "" {
			return p, nil
		}
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
		if err := os.Rename(unpacked, dir); err != nil {
			return "", err
		}
	}
	return nsisExecutable(dir), nil
}

// downloadAny fetches the first of urls, copies of one file, that answers
// with a file whose SHA-256 is sum into the file path.
func downloadAny(urls []string, sum, path string) error {
	var errs []error
	for _, url := range urls {
		err := download(url, sum, path)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// downloadHeaderTimeout is how long download waits for a host to answer,
// so that one that takes connections and never answers gives way to the
// next copy.
var downloadHeaderTimeout = 30 * time.Second

// download fetches url into the file path and checks that its SHA-256 is
// sum.
func download(url, sum, path string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "mygo/"+version)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = downloadHeaderTimeout
	defer transport.CloseIdleConnections()
	resp, err := (&http.Client{Transport: transport, Timeout: 10 * time.Minute}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	h := sha256.New()
	_, err = io.Copy(io.MultiWriter(f, h), io.LimitReader(resp.Body, 64<<20))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != sum {
		return fmt.Errorf("%s has the SHA-256 %s, not %s", url, got, sum)
	}
	return nil
}

// unzipTop unpacks the zip archive at path, whose entries are all in one
// top directory, into dir without that directory.
func unzipTop(path, dir string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer r.Close()
	top := ""
	for _, f := range r.File {
		first, rest, ok := strings.Cut(f.Name, "/")
		if top == "" {
			top = first
		}
		if !ok || first != top {
			return fmt.Errorf("%q is not in the top directory %q", f.Name, top)
		}
		if rest == "" {
			continue // the top directory
		}
		name := strings.TrimSuffix(rest, "/")
		if !fs.ValidPath(name) || !filepath.IsLocal(filepath.FromSlash(name)) || strings.Contains(name, `\`) {
			return fmt.Errorf("invalid path %q", f.Name)
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		switch mode := f.Mode(); {
		case mode.IsDir():
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := unzipFile(f, target); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%q is neither a file nor a directory", f.Name)
		}
	}
	return nil
}

func unzipFile(f *zip.File, target string) error {
	src, err := f.Open()
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(target)
	if err != nil {
		return err
	}
	// The zip reader checks the CRC-32 of the file once it is read.
	_, err = io.Copy(dst, src)
	if cerr := dst.Close(); err == nil {
		err = cerr
	}
	return err
}

// writeInstaller builds "<Name> Setup <version>.exe" in stage from the app
// files installed, the executable exe among them, and returns its path, or
// "" without NSIS.
func writeInstaller(c *Config, stage, work, exe string, installed []string) (string, error) {
	tool, err := nsisCompiler()
	if err != nil {
		return "", err
	}
	if tool == "" {
		logf("skipping the Windows installer: install NSIS (makensis)")
		return "", nil
	}
	out := filepath.Join(stage, fsName(c.Name)+" Setup "+fsName(c.Version)+".exe")
	var files strings.Builder
	for _, name := range installed {
		p := filepath.Join(stage, name)
		if isDir(p) {
			fmt.Fprintf(&files, "  File /r %s\n", nsisString(p))
		} else {
			fmt.Fprintf(&files, "  File %s\n", nsisString(p))
		}
	}
	icon := ""
	if c.Icon != "" {
		src, err := os.ReadFile(c.path(c.Icon))
		if err != nil {
			return "", err
		}
		ico, err := pngToICO(src)
		if err != nil {
			return "", err
		}
		path := filepath.Join(work, "installer.ico")
		if err := os.WriteFile(path, ico, 0o644); err != nil {
			return "", err
		}
		icon = "!define MUI_ICON " + nsisString(path) + "\n!define MUI_UNICON " + nsisString(path) + "\n"
	}
	signing, env, err := uninstallerSigning(c, tool)
	if err != nil {
		return "", err
	}
	uninstallKey := `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + c.Identifier
	shortcut := nsisEscape(fsName(c.Name)) + ".lnk"
	register, unregister := nsisAssociations(c, exe)
	script := `Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma
Name ` + nsisString(c.Name) + `
OutFile ` + nsisString(out) + `
` + signing + `InstallDir "$LOCALAPPDATA\Programs\` + nsisEscape(fsName(c.Name)) + `"
RequestExecutionLevel user
BrandingText " "
` + icon + `!define MUI_FINISHPAGE_RUN "$INSTDIR\` + nsisEscape(exe) + `"
!define MUI_FINISHPAGE_SHOWREADME
!define MUI_FINISHPAGE_SHOWREADME_TEXT "Create a desktop shortcut"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!include "MUI2.nsh"
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Section
  SetOutPath "$INSTDIR"
` + files.String() + `  WriteUninstaller "$INSTDIR\Uninstall.exe"
  CreateShortCut "$SMPROGRAMS\` + shortcut + `" "$INSTDIR\` + nsisEscape(exe) + `"
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "DisplayName" ` + nsisString(c.Name) + `
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "DisplayVersion" ` + nsisString(c.Version) + `
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "DisplayIcon" "$INSTDIR\` + nsisEscape(exe) + `"
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "UninstallString" '"$INSTDIR\Uninstall.exe"'
  WriteRegStr HKCU ` + nsisString(uninstallKey) + ` "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
  WriteRegDWORD HKCU ` + nsisString(uninstallKey) + ` "NoModify" 1
  WriteRegDWORD HKCU ` + nsisString(uninstallKey) + ` "NoRepair" 1
` + register + `SectionEnd

; The finish page offers it, so silent installs (/S) make none.
Function CreateDesktopShortcut
  SetOutPath "$INSTDIR"
  CreateShortCut "$DESKTOP\` + shortcut + `" "$INSTDIR\` + nsisEscape(exe) + `"
FunctionEnd

; Deletes the shortcut on the stack, trying again for 5 s while it is in
; use: Explorer opens a new shortcut a few seconds after it appears, not
; sharing it for deletion, and Delete fails meanwhile. It succeeds when
; there is no shortcut.
Function un.DeleteShortcut
  Exch $0
  Push $1
  StrCpy $1 50
  retry:
    ClearErrors
    Delete $0
    IfErrors 0 done
    IntOp $1 $1 - 1
    IntCmp $1 0 done
    Sleep 100
    Goto retry
  done:
  Pop $1
  Pop $0
FunctionEnd

Section "Uninstall"
  Push "$SMPROGRAMS\` + shortcut + `"
  Call un.DeleteShortcut
  Push "$DESKTOP\` + shortcut + `"
  Call un.DeleteShortcut
  DeleteRegKey HKCU ` + nsisString(uninstallKey) + `
` + unregister + `  ; Last, so that the app's folder is gone once the uninstall is done:
  ; the uninstaller runs from a copy of itself that nothing waits for.
  RMDir /r "$INSTDIR"
SectionEnd
`
	nsi := filepath.Join(work, "installer.nsi")
	if err := os.WriteFile(nsi, []byte(script), 0o644); err != nil {
		return "", err
	}
	logf("creating %s", filepath.Base(out))
	if signing != "" {
		logf("signing Uninstall.exe")
	}
	cmd := exec.Command(tool, "-V2", nsi)
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("makensis: %v\n%s", err, out)
	}
	return out, nil
}

// uninstallerSigning returns the NSIS command that signs the uninstaller
// of a signed app, and the environment makensis needs to run it. makensis
// writes the uninstaller to a temporary file, runs the command with it and
// puts the signed file into the installer. The command runs mygo, which
// signs the uninstaller as it signs the app (runSignUninstaller); its path
// goes through the environment, since NSIS reads "$" in the script as its
// own syntax.
func uninstallerSigning(c *Config, tool string) (script string, env []string, err error) {
	if !c.Windows.signs() {
		return "", nil, nil
	}
	if major, minor := nsisVersion(tool); major < 3 || major == 3 && minor < 8 {
		logf("not signing Uninstall.exe: that needs NSIS 3.08 or later, not %s", tool)
		return "", nil, nil
	}
	self, err := os.Executable()
	if err != nil {
		return "", nil, err
	}
	settings, err := json.Marshal(signSettings{Root: c.root, Windows: c.Windows})
	if err != nil {
		return "", nil, err
	}
	signer := `"%` + signerEnv + `%"`
	if runtime.GOOS != "windows" {
		signer = `"$` + signerEnv + `"`
	}
	// "= 0" stops the build when signing fails; NSIS ignores the exit code
	// otherwise.
	return "!uninstfinalize '" + signer + ` sign-uninstaller "%1"' = 0` + "\n",
		[]string{signerEnv + "=" + self, signSettingsEnv + "=" + string(settings)}, nil
}

// nsisVersion returns the version of the NSIS compiler tool, or zeros when
// it does not tell.
func nsisVersion(tool string) (major, minor int) {
	out, err := exec.Command(tool, "-VERSION").Output()
	if err == nil {
		// "v3.13", or "v3.08-3" for a distribution's build.
		fmt.Sscanf(strings.TrimPrefix(strings.TrimSpace(string(out)), "v"), "%d.%d", &major, &minor)
	}
	return major, minor
}

// nsisAssociations returns the installer commands that register, and
// unregister, the file associations and URL schemes of the app for the
// user, opening them with exe.
func nsisAssociations(c *Config, exe string) (register, unregister string) {
	var r, u strings.Builder
	open := `'"$INSTDIR\` + nsisEscape(exe) + `" "%1"'`
	prefix := strings.Map(func(r rune) rune {
		if r < 0x80 && (r == '.' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
			return r
		}
		return -1
	}, c.Identifier)
	for _, fa := range c.FileAssociations {
		for _, ext := range fa.Ext {
			progID := prefix + "." + strings.ToLower(ext)
			class := `Software\Classes\` + progID
			fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"\" %s\n", nsisString(class), nsisString(fa.Name))
			fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"\" \"$INSTDIR\\%s,0\"\n", nsisString(class+`\DefaultIcon`), nsisEscape(exe))
			fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"\" %s\n", nsisString(class+`\shell\open\command`), open)
			fmt.Fprintf(&r, "  WriteRegStr HKCU %s %s \"\"\n", nsisString(`Software\Classes\.`+ext+`\OpenWithProgids`), nsisString(progID))
			fmt.Fprintf(&u, "  DeleteRegKey HKCU %s\n", nsisString(class))
			fmt.Fprintf(&u, "  DeleteRegValue HKCU %s %s\n", nsisString(`Software\Classes\.`+ext+`\OpenWithProgids`), nsisString(progID))
		}
	}
	for _, scheme := range c.URLSchemes {
		key := `Software\Classes\` + scheme
		fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"\" %s\n", nsisString(key), nsisString("URL:"+c.Name))
		fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"URL Protocol\" \"\"\n", nsisString(key))
		fmt.Fprintf(&r, "  WriteRegStr HKCU %s \"\" %s\n", nsisString(key+`\shell\open\command`), open)
		fmt.Fprintf(&u, "  DeleteRegKey HKCU %s\n", nsisString(key))
	}
	if r.Len() > 0 {
		// Tell Explorer the associations changed.
		notify := "  System::Call 'shell32::SHChangeNotify(i 0x08000000, i 0, p 0, p 0)'\n"
		r.WriteString(notify)
		u.WriteString(notify)
	}
	return r.String(), u.String()
}

// nsisEscape escapes text for an NSIS string in double quotes.
func nsisEscape(s string) string {
	return strings.NewReplacer(`$`, `$$`, `"`, `$\"`, "\n", `$\n`, "\r", `$\r`, "\t", `$\t`).Replace(s)
}

func nsisString(s string) string { return `"` + nsisEscape(s) + `"` }
