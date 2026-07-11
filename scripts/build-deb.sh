#!/bin/bash
# Build .deb package for bili-FM Linux
# Usage: ./build-deb.sh [version] [arch]
# Defaults: version from service/config.go, arch=amd64

set -e

VERSION="${1:-1.9.5}"
ARCH="${2:-amd64}"
PKG_NAME="bili-fm"
DEB_ROOT="/tmp/${PKG_NAME}-deb"

echo "Building .deb package: ${PKG_NAME} v${VERSION} (${ARCH})"

# Clean previous build
rm -rf "$DEB_ROOT"
mkdir -p "$DEB_ROOT/DEBIAN"
mkdir -p "$DEB_ROOT/usr/bin"
mkdir -p "$DEB_ROOT/usr/share/applications"
mkdir -p "$DEB_ROOT/usr/share/pixmaps"

# Copy binary
cp bili-FM "$DEB_ROOT/usr/bin/bili-fm"
chmod 755 "$DEB_ROOT/usr/bin/bili-fm"

# Copy desktop entry
cat > "$DEB_ROOT/usr/share/applications/bili-fm.desktop" << 'DESKTOP'
[Desktop Entry]
Type=Application
Name=bili-FM
Comment=A Bilibili audio player - listen to B站 content as music or podcasts
Exec=bili-fm
Icon=bili-FM
Terminal=false
Categories=AudioVideo;Audio;Player;
Keywords=bilibili;audio;music;podcast;
DESKTOP
chmod 644 "$DEB_ROOT/usr/share/applications/bili-fm.desktop"

# Copy icon
cp build/appicon.png "$DEB_ROOT/usr/share/pixmaps/bili-FM.png"
chmod 644 "$DEB_ROOT/usr/share/pixmaps/bili-FM.png"

# Generate control file
cat > "$DEB_ROOT/DEBIAN/control" << CONTROL
Package: ${PKG_NAME}
Version: ${VERSION}
Section: sound
Priority: optional
Architecture: ${ARCH}
Depends: libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, libgtk-3-0, libsoup-3.0-0 | libsoup2-4-1, libc6, libstdc++6
Maintainer: vst <vst93@users.noreply.github.com>
Description: A Bilibili audio player
 Listen to Bilibili content in audio-only mode - music, podcasts,
 knowledge videos, all as audio. Features a liquid glass UI with
 system tray support across Windows, macOS, and Linux.
 .
 Homepage: https://github.com/vst93/bili-fm
CONTROL
chmod 644 "$DEB_ROOT/DEBIAN/control"

# Build .deb
DEB_FILE="${PKG_NAME}_${VERSION}_${ARCH}.deb"
dpkg-deb --build --root-owner-group "$DEB_ROOT" "$DEB_FILE"

echo ""
echo "✅ Built: $DEB_FILE"
dpkg-deb --info "$DEB_FILE" | head -15
echo ""
echo "Contents:"
dpkg-deb --contents "$DEB_FILE"

# Cleanup
rm -rf "$DEB_ROOT"
