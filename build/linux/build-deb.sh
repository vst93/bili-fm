#!/usr/bin/env bash
# Build a .deb package for bili-FM.
# Usage: build-deb.sh <version> <arch: amd64|arm64> <binary-path> <output-dir>
set -euo pipefail

VERSION="$1"
ARCH="$2"
BINARY="$3"
OUT_DIR="$4"

case "$ARCH" in
  amd64) DEB_ARCH="amd64" ;;
  arm64) DEB_ARCH="arm64" ;;
  *) echo "Unsupported arch: $ARCH (expected amd64|arm64)" >&2; exit 1 ;;
esac

ROOT="/tmp/bili-fm-deb"
rm -rf "$ROOT"
mkdir -p "$ROOT/DEBIAN" "$ROOT/usr/bin" \
  "$ROOT/usr/share/applications" "$ROOT/usr/share/pixmaps"

cp "$BINARY" "$ROOT/usr/bin/bili-fm"
chmod 755 "$ROOT/usr/bin/bili-fm"

# Locate the desktop file / icon from the repo root (relative to this script).
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

cp "$SCRIPT_DIR/bili-fm.desktop" "$ROOT/usr/share/applications/bili-fm.desktop"
cp "$REPO_ROOT/build/appicon.png" "$ROOT/usr/share/pixmaps/bili-FM.png"

cat > "$ROOT/DEBIAN/control" <<EOF
Package: bili-fm
Version: ${VERSION}
Section: sound
Priority: optional
Architecture: ${DEB_ARCH}
Depends: libwebkit2gtk-4.1-0 | libwebkit2gtk-4.0-37, libgtk-3-0, libsoup-3.0-0 | libsoup2-4-1, libc6, libstdc++6
Maintainer: vst <vst93@users.noreply.github.com>
Description: A Bilibili audio player
 Listen to Bilibili content in audio-only mode.
EOF

mkdir -p "$OUT_DIR"
dpkg-deb --build --root-owner-group "$ROOT" "$OUT_DIR/bili-FM_${VERSION}_${DEB_ARCH}.deb"
echo "=== .deb ==="
ls -la "$OUT_DIR"/*.deb