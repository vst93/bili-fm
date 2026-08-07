#!/usr/bin/env bash
# Build an .rpm package for bili-FM.
# Requires `rpm` (rpmbuild) to be installed.
# Usage: build-rpm.sh <version> <arch: amd64|arm64> <binary-path> <output-dir>
set -euo pipefail

VERSION="$1"
ARCH="$2"
BINARY="$3"
OUT_DIR="$4"

case "$ARCH" in
  amd64) RPM_ARCH="x86_64" ;;
  arm64) RPM_ARCH="aarch64" ;;
  *) echo "Unsupported arch: $ARCH (expected amd64|arm64)" >&2; exit 1 ;;
esac

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Stage the payload once; rpmbuild %install copies from here.
ROOT="/tmp/bili-fm-rpm-root"
rm -rf "$ROOT"
mkdir -p "$ROOT/usr/bin" "$ROOT/usr/share/applications" "$ROOT/usr/share/pixmaps"
cp "$BINARY" "$ROOT/usr/bin/bili-fm"
chmod 755 "$ROOT/usr/bin/bili-fm"
cp "$SCRIPT_DIR/bili-fm.desktop" "$ROOT/usr/share/applications/bili-fm.desktop"
cp "$REPO_ROOT/build/appicon.png" "$ROOT/usr/share/pixmaps/bili-FM.png"

SPEC="/tmp/bili-fm.spec"
cat > "$SPEC" <<EOF
Name:       bili-FM
Version:    ${VERSION}
Release:    0
Summary:    A Bilibili audio player
License:    GPLv3
URL:        https://github.com/vst93/bili-fm
BuildArch:  ${RPM_ARCH}

%description
Listen to Bilibili content in audio-only mode.

%install
mkdir -p %{buildroot}/usr/bin %{buildroot}/usr/share/applications %{buildroot}/usr/share/pixmaps
cp "$ROOT/usr/bin/bili-fm" %{buildroot}/usr/bin/bili-fm
cp "$ROOT/usr/share/applications/bili-fm.desktop" %{buildroot}/usr/share/applications/bili-fm.desktop
cp "$ROOT/usr/share/pixmaps/bili-FM.png" %{buildroot}/usr/share/pixmaps/bili-FM.png

%files
/usr/bin/bili-fm
/usr/share/applications/bili-fm.desktop
/usr/share/pixmaps/bili-FM.png
EOF

mkdir -p "$OUT_DIR"
rpmbuild -bb \
  --define "_topdir /tmp/bili-fm-rpm-topdir" \
  --define "_rpmdir $OUT_DIR" \
  --define "_rpmfilename %{NAME}-%{VERSION}.%{ARCH}.rpm" \
  "$SPEC"

echo "=== .rpm ==="
ls -la "$OUT_DIR"/*.rpm