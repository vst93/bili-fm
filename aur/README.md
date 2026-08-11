# bili-fm Linux Installation

## Arch Linux (PKGBUILD)

### Prebuilt package (recommended)

```bash
git clone https://github.com/vst93/bili-fm.git
cd bili-fm/aur
makepkg -si
```

This downloads the prebuilt `.pkg.tar.zst` from the latest GitHub Release.
The package is built by CI directly (not converted from .deb), so it installs
cleanly with pacman.

### Build from source (latest main)

```bash
git clone https://github.com/vst93/bili-fm.git
cd bili-fm/aur
cp PKGBUILD-git PKGBUILD
makepkg -si
```

## Ubuntu / Debian (.deb)

Download the `.deb` file from [GitHub Releases](https://github.com/vst93/bili-fm/releases/latest):

```bash
# Replace VERSION with the actual version, e.g. 2.0.0
VERSION=2.0.0
wget "https://github.com/vst93/bili-fm/releases/download/${VERSION}/bili-FM-${VERSION}-linux-x86_64.deb"
sudo dpkg -i "bili-FM-${VERSION}-linux-x86_64.deb"
sudo apt-get install -f  # fix missing dependencies
```

### Dependencies (Ubuntu 24.04+)

```bash
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 libsoup-3.0-0
```

## All platforms

See [GitHub Releases](https://github.com/vst93/bili-fm/releases/latest) for
macOS (.dmg), Windows (.exe), and Linux (.deb / .pkg.tar.zst / .rpm) downloads.
