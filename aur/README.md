# bili-fm Linux Installation

## Arch Linux (PKGBUILD)

### Prebuilt binary (recommended)

```bash
git clone https://github.com/vst93/bili-fm.git
cd bili-fm/aur
makepkg -si
```

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
wget https://github.com/vst93/bili-fm/releases/download/1.9.5/bili-FM-linux-amd64.deb
sudo dpkg -i bili-FM-linux-amd64.deb
sudo apt-get install -f  # fix missing dependencies
```

### Dependencies (Ubuntu 24.04+)

```bash
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 libsoup-3.0-0
```

### Dependencies (Ubuntu 22.04)

Ubuntu 22.04 only has webkit2gtk-4.0, which is incompatible with the prebuilt
binary (built against 4.1). Build from source with the `webkit2_41` tag on
24.04+, or use the 4.0-compatible build:

```bash
# On Ubuntu 24.04+:
sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0 libsoup-3.0-0
```

## All platforms

See [GitHub Releases](https://github.com/vst93/bili-fm/releases/latest) for
macOS (.zip), Windows (.zip), and Linux (.zip + .deb) downloads.
