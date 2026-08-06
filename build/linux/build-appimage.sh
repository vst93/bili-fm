#!/usr/bin/env bash
# Build an AppImage for bili-FM using linuxdeploy + appimagetool.
# Usage: build-appimage.sh <arch: amd64|arm64> <binary-path> <output-dir>
set -euo pipefail

ARCH="$1"
BINARY="$2"
OUT_DIR="$3"

case "$ARCH" in
  amd64) LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-x86_64.AppImage"
         APPIMAGETOOL_URL="https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage"
         APPIMAGE_ARCH="x86_64" ;;
  arm64) LINUXDEPLOY_URL="https://github.com/linuxdeploy/linuxdeploy/releases/download/continuous/linuxdeploy-aarch64.AppImage"
         APPIMAGETOOL_URL="https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-aarch64.AppImage"
         APPIMAGE_ARCH="aarch64" ;;
  *) echo "Unsupported arch: $ARCH (expected amd64|arm64)" >&2; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

TOOLS_DIR="/tmp/bili-fm-appimage-tools"
mkdir -p "$TOOLS_DIR"
cd "$TOOLS_DIR"

# FUSE is usually unavailable in CI; use the extract-and-run mode.
export APPIMAGE_EXTRACT_AND_RUN=1

LINUXDEPLOY="$TOOLS_DIR/linuxdeploy.AppImage"
APPIMAGETOOL="$TOOLS_DIR/appimagetool.AppImage"
[ -f "$LINUXDEPLOY" ] || curl -L -o "$LINUXDEPLOY" "$LINUXDEPLOY_URL"
[ -f "$APPIMAGETOOL" ] || curl -L -o "$APPIMAGETOOL" "$APPIMAGETOOL_URL"
chmod +x "$LINUXDEPLOY" "$APPIMAGETOOL"

APPDIR="$OUT_DIR/bili-fm.AppDir"
rm -rf "$APPDIR"
mkdir -p "$APPDIR/usr/bin"
cp "$BINARY" "$APPDIR/usr/bin/bili-fm"
chmod 755 "$APPDIR/usr/bin/bili-fm"
cp "$SCRIPT_DIR/bili-fm.desktop" "$APPDIR/bili-fm.desktop"
cp "$REPO_ROOT/build/appicon.png" "$APPDIR/bili-FM.png"

# Bundle runtime libraries into the AppDir.
"$LINUXDEPLOY" \
  --appdir "$APPDIR" \
  --executable "$APPDIR/usr/bin/bili-fm" \
  --desktop-file "$APPDIR/bili-fm.desktop" \
  --icon-file "$APPDIR/bili-FM.png"

# Turn the AppDir into an AppImage.
mkdir -p "$OUT_DIR"
"$APPIMAGETOOL" "$APPDIR" "$OUT_DIR/bili-FM-${APPIMAGE_ARCH}.AppImage"

echo "=== AppImage ==="
ls -la "$OUT_DIR"/*.AppImage