package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// finderHasCustomIcon is the Finder flag of folders with a custom icon.
const finderHasCustomIcon = 0x0400

// dmgFileName is "<Name> <Version>.dmg".
func dmgFileName(c *Config) string {
	return fsName(c.Name) + " " + fsName(c.Version) + ".dmg"
}

// buildDMG packages app into a compressed, read-only disk image in dir and
// returns its path. Opening the image shows the app next to a link to
// /Applications; the Finder layout comes from a generated .DS_Store (see
// dsstore.go). Only hdiutil, which ships with macOS, is needed.
func buildDMG(c *Config, app, dir string, opts buildOptions) (string, error) {
	file := dmgFileName(c)
	logf("creating %s", file)
	work, err := os.MkdirTemp(dir, ".dmg-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(work)

	// The image starts as a copy of a folder holding only the app. ditto
	// keeps the bundle exactly as signed.
	src := filepath.Join(work, "src")
	appName := filepath.Base(app)
	if err := os.Mkdir(src, 0o755); err != nil {
		return "", err
	}
	if err := command("ditto", app, filepath.Join(src, appName)); err != nil {
		return "", err
	}
	rw := filepath.Join(work, "rw.dmg")
	if err := command("hdiutil", hdiutilCreateArgs(c.MacOS.DMGTitle, src, rw, dirSize(src))...); err != nil {
		return "", err
	}

	mnt := filepath.Join(work, "mnt")
	if err := os.Mkdir(mnt, 0o755); err != nil {
		return "", err
	}
	if err := command("hdiutil", "attach", "-quiet", "-readwrite", "-noverify", "-noautoopen", "-nobrowse", "-mountpoint", mnt, rw); err != nil {
		return "", err
	}
	attached := true
	defer func() {
		if attached {
			_ = exec.Command("hdiutil", "detach", "-quiet", "-force", mnt).Run()
		}
	}()
	if err := os.Symlink("/Applications", filepath.Join(mnt, "Applications")); err != nil {
		return "", err
	}
	// Finder hides the .app extension of bundles by default; hiding it with
	// the Finder flag, like create-dmg does, would make the signature fail
	// strict verification.
	if err := writeDSStore(filepath.Join(mnt, ".DS_Store"), appName); err != nil {
		return "", err
	}
	if icon := filepath.Join(app, "Contents", "Resources", bundleIcon); fileExists(icon) {
		if err := copyFile(icon, filepath.Join(mnt, ".VolumeIcon.icns"), 0o644); err != nil {
			return "", err
		}
		if err := setFinderFlags(mnt, finderHasCustomIcon); err != nil {
			return "", err
		}
	}
	// Created by macOS while the image is mounted.
	_ = os.RemoveAll(filepath.Join(mnt, ".fseventsd"))
	_ = os.RemoveAll(filepath.Join(mnt, ".Trashes"))
	if err := detach(mnt); err != nil {
		return "", err
	}
	attached = false

	// LZMA gives the smallest images; it needs macOS 10.15, below the
	// minimum MyGo supports.
	dmg := filepath.Join(dir, file)
	if err := command("hdiutil", "convert", "-quiet", rw, "-format", "ULMO", "-ov", "-o", dmg); err != nil {
		return "", err
	}
	if opts.sign != "-" {
		if err := command("codesign", "--force", "--timestamp", "--sign", opts.sign, dmg); err != nil {
			return "", err
		}
	}
	if c.MacOS.Notarize != nil && !opts.skipNotarize {
		if err := notarize(c.MacOS.Notarize, dmg); err != nil {
			return "", err
		}
		// The ticket covers the app too: staple it for updates, which
		// install the bare app.
		if err := command("xcrun", "stapler", "staple", app); err != nil {
			return "", err
		}
	}
	return dmg, nil
}

// hdiutilCreateArgs creates a writable HFS+ image from src with room for the
// layout files next to its contents (size is in bytes).
func hdiutilCreateArgs(volume, src, out string, size int64) []string {
	return []string{
		"create", "-quiet", "-ov",
		"-volname", volume,
		"-srcfolder", src,
		"-fs", "HFS+", "-fsargs", "-c c=64,a=16,e=16",
		"-format", "UDRW",
		"-size", fmt.Sprintf("%dm", size>>20+20),
		out,
	}
}

// setFinderFlags sets the Finder flags of a file or folder, which are
// stored at offset 8 of its com.apple.FinderInfo attribute.
func setFinderFlags(path string, flags uint16) error {
	var info [32]byte
	info[8], info[9] = byte(flags>>8), byte(flags)
	return command("/usr/bin/xattr", "-wx", "com.apple.FinderInfo", hex.EncodeToString(info[:]), path)
}

// detach unmounts a disk image, retrying while macOS services such as
// Spotlight still hold the volume, which on busy machines like CI runners
// can outlast a forced detach: after a few polite attempts it forces, for
// up to half a minute in all.
func detach(mnt string) error {
	var err error
	for i := range 10 {
		args := []string{"detach", "-quiet", mnt}
		if i >= 4 {
			args = []string{"detach", "-quiet", "-force", mnt}
		}
		if err = command("hdiutil", args...); err == nil {
			return nil
		}
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	return err
}

// notarize submits a disk image to Apple's notary service, waits for the
// verdict and staples the ticket to the image.
func notarize(n *Notarize, dmg string) error {
	logf("notarizing %s, which usually takes a few minutes", filepath.Base(dmg))
	args := []string{"notarytool", "submit", dmg, "--keychain-profile", n.KeychainProfile, "--wait", "--output-format", "json"}
	if n.Keychain != "" {
		args = append(args, "--keychain", n.Keychain)
	}
	cmd := exec.Command("xcrun", args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	var result struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Message string `json:"message"`
	}
	if i := strings.LastIndexByte(string(out), '{'); i >= 0 {
		_ = json.Unmarshal(out[i:], &result)
	}
	if err != nil || result.Status != "Accepted" {
		msg := fmt.Sprintf("notarization failed (%s", strings.TrimSpace(result.Status+" "+result.Message))
		if err != nil {
			msg += fmt.Sprintf(": %v", err)
		}
		msg += ")"
		if result.ID != "" {
			msg += fmt.Sprintf("; see xcrun notarytool log %s --keychain-profile %q", result.ID, n.KeychainProfile)
		}
		return fmt.Errorf("%s\n%s", msg, strings.TrimSpace(string(out)))
	}
	return command("xcrun", "stapler", "staple", dmg)
}

func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			total += info.Size()
		}
		return nil
	})
	return total
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
