package main

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// writeBundle assembles <dir>/<executable>.app around the executable bin,
// which is moved into it, with the icon icns (or nil) and the resources res,
// and returns the bundle path.
func writeBundle(c *Config, dir, bin string, icns []byte, res []resource) (string, error) {
	name := c.executableName()
	app := filepath.Join(dir, name+".app")
	if err := os.RemoveAll(app); err != nil {
		return "", err
	}
	contents := filepath.Join(app, "Contents")
	for _, d := range []string{"MacOS", "Resources"} {
		if err := os.MkdirAll(filepath.Join(contents, d), 0o755); err != nil {
			return "", err
		}
	}
	if err := os.Rename(bin, filepath.Join(contents, "MacOS", name)); err != nil {
		return "", err
	}
	iconFile := ""
	if icns != nil {
		iconFile = bundleIcon
		if err := os.WriteFile(filepath.Join(contents, "Resources", iconFile), icns, 0o644); err != nil {
			return "", err
		}
	}
	if err := copyResources(res, filepath.Join(contents, "Resources")); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(contents, "Info.plist"), infoPlist(c, name, iconFile), 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(contents, "PkgInfo"), []byte("APPL????"), 0o644); err != nil {
		return "", err
	}
	return app, nil
}

// bundleExecutable returns the path of the executable inside a bundle.
func bundleExecutable(app string) string {
	name := filepath.Base(app)
	return filepath.Join(app, "Contents", "MacOS", name[:len(name)-len(".app")])
}

// appIcon renders the configured icon as .icns, or returns nil without one.
func appIcon(c *Config) ([]byte, error) {
	if c.Icon == "" {
		return nil, nil
	}
	src, err := os.ReadFile(c.path(c.Icon))
	if err != nil {
		return nil, err
	}
	icns, err := pngToICNS(src)
	if err != nil {
		return nil, fmt.Errorf("icon %s: %w", c.Icon, err)
	}
	return icns, nil
}

func infoPlist(c *Config, executable, icon string) []byte {
	d := map[string]any{
		"CFBundleName":                         c.Name,
		"CFBundleDisplayName":                  c.Name,
		"CFBundleIdentifier":                   c.Identifier,
		"CFBundleVersion":                      c.Version,
		"CFBundleShortVersionString":           c.Version,
		"CFBundleExecutable":                   executable,
		"CFBundlePackageType":                  "APPL",
		"CFBundleInfoDictionaryVersion":        "6.0",
		"LSMinimumSystemVersion":               c.MacOS.MinimumSystemVersion,
		"NSPrincipalClass":                     "NSApplication",
		"NSHighResolutionCapable":              true,
		"NSSupportsAutomaticGraphicsSwitching": true,
	}
	if icon != "" {
		d["CFBundleIconFile"] = icon
	}
	if c.Copyright != "" {
		d["NSHumanReadableCopyright"] = c.Copyright
	}
	var docs []any
	for _, fa := range c.FileAssociations {
		role := fa.Role
		if role == "" {
			role = "Editor"
		}
		name := fa.Name
		if name == "" {
			name = strings.ToUpper(fa.Ext[0]) + " file"
		}
		var exts []any
		for _, ext := range fa.Ext {
			exts = append(exts, ext)
		}
		doc := map[string]any{"CFBundleTypeName": name, "CFBundleTypeRole": role, "LSHandlerRank": "Default", "CFBundleTypeExtensions": exts}
		if fa.MimeType != "" {
			doc["CFBundleTypeMIMETypes"] = []any{fa.MimeType}
		}
		docs = append(docs, doc)
	}
	if docs != nil {
		d["CFBundleDocumentTypes"] = docs
	}
	if len(c.URLSchemes) > 0 {
		var schemes []any
		for _, s := range c.URLSchemes {
			schemes = append(schemes, s)
		}
		d["CFBundleURLTypes"] = []any{map[string]any{"CFBundleURLName": c.Identifier, "CFBundleURLSchemes": schemes}}
	}
	for k, v := range c.MacOS.InfoPlist {
		d[k] = v // the app's own keys win
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
`)
	writePlistValue(&b, d, "")
	b.WriteString("</plist>\n")
	return b.Bytes()
}

// writePlistValue writes a value decoded from JSON (or built like one) as
// XML property list, indented by indent.
func writePlistValue(b *bytes.Buffer, v any, indent string) {
	esc := func(s string) string {
		var e bytes.Buffer
		_ = xml.EscapeText(&e, []byte(s))
		return e.String()
	}
	switch v := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString(indent + "<dict>\n")
		for _, k := range keys {
			fmt.Fprintf(b, "%s\t<key>%s</key>\n", indent, esc(k))
			writePlistValue(b, v[k], indent+"\t")
		}
		b.WriteString(indent + "</dict>\n")
	case []any:
		b.WriteString(indent + "<array>\n")
		for _, x := range v {
			writePlistValue(b, x, indent+"\t")
		}
		b.WriteString(indent + "</array>\n")
	case bool:
		if v {
			b.WriteString(indent + "<true/>\n")
		} else {
			b.WriteString(indent + "<false/>\n")
		}
	case float64:
		if v == math.Trunc(v) && math.Abs(v) < 1<<53 {
			fmt.Fprintf(b, "%s<integer>%d</integer>\n", indent, int64(v))
		} else {
			fmt.Fprintf(b, "%s<real>%v</real>\n", indent, v)
		}
	case string:
		fmt.Fprintf(b, "%s<string>%s</string>\n", indent, esc(v))
	case nil:
		b.WriteString(indent + "<string></string>\n")
	default:
		fmt.Fprintf(b, "%s<string>%s</string>\n", indent, esc(fmt.Sprint(v)))
	}
}

// codesignArgs returns the codesign arguments that sign path. Real
// identities get the hardened runtime and a secure timestamp in production,
// which notarization requires.
func codesignArgs(path, identity, entitlements string, production bool) []string {
	args := []string{"--force", "--deep", "--sign", identity}
	if production && identity != "-" {
		args = append(args, "--options", "runtime", "--timestamp")
	}
	if entitlements != "" {
		args = append(args, "--entitlements", entitlements)
	}
	return append(args, path)
}

// codesign signs a bundle, and the code among its resources. Signing needs
// macOS: elsewhere only the executable's ad-hoc signature from the Go linker
// remains.
func codesign(c *Config, path, identity string, production bool) error {
	if runtime.GOOS != "darwin" {
		if identity != "-" {
			logf("not signing %s: code signing needs macOS", filepath.Base(path))
		}
		return nil
	}
	if err := signNestedCode(c, path, identity, production); err != nil {
		return err
	}
	entitlements := ""
	if c.MacOS.Entitlements != "" {
		entitlements = c.path(c.MacOS.Entitlements)
	}
	out, err := exec.Command("codesign", codesignArgs(path, identity, entitlements, production)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("codesign %s: %v\n%s", filepath.Base(path), err, out)
	}
	return nil
}

// replacePath moves src to dst, replacing what dst held. The old dst is
// renamed away before it is removed, so a running app keeps its files.
// Removing it is best effort: Windows keeps running executables.
func replacePath(src, dst string) error {
	old := ""
	if _, err := os.Lstat(dst); err == nil {
		old = filepath.Join(filepath.Dir(dst), fmt.Sprintf(".old-%d-%s", os.Getpid(), filepath.Base(dst)))
		_ = os.RemoveAll(old)
		if err := os.Rename(dst, old); err != nil {
			return err
		}
	}
	if err := os.Rename(src, dst); err != nil {
		if old != "" {
			_ = os.Rename(old, dst)
		}
		return err
	}
	if old != "" {
		_ = os.RemoveAll(old)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// writeUniversal combines Mach-O executables into a universal (fat)
// binary, like lipo, so no Xcode tools are needed.
func writeUniversal(out string, slices ...string) error {
	const (
		fatMagic = 0xcafebabe
		align    = 14 // 2^14 = 16 KiB
	)
	type arch struct {
		cpuType, cpuSubtype uint32
		data                []byte
	}
	var archs []arch
	for _, p := range slices {
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if len(data) < 12 || binary.LittleEndian.Uint32(data) != 0xfeedfacf {
			return fmt.Errorf("%s is not a 64-bit Mach-O file", p)
		}
		archs = append(archs, arch{binary.LittleEndian.Uint32(data[4:]), binary.LittleEndian.Uint32(data[8:]), data})
	}
	var buf bytes.Buffer
	_ = binary.Write(&buf, binary.BigEndian, [2]uint32{fatMagic, uint32(len(archs))})
	offset := uint32(1 << align)
	offsets := make([]uint32, len(archs))
	for i, a := range archs {
		offsets[i] = offset
		_ = binary.Write(&buf, binary.BigEndian, [5]uint32{a.cpuType, a.cpuSubtype, offset, uint32(len(a.data)), align})
		offset += (uint32(len(a.data)) + (1<<align - 1)) &^ (1<<align - 1)
	}
	for i, a := range archs {
		buf.Write(make([]byte, int(offsets[i])-buf.Len()))
		buf.Write(a.data)
	}
	return os.WriteFile(out, buf.Bytes(), 0o755)
}
