#!/bin/bash
# Build bili-FM with native installers (Wails official packaging).
#
# Usage:
#   ./build.sh [platform ...]
#   ./build.sh                       # builds all default platforms
#   ./build.sh linux/amd64           # build one platform
#   ./build.sh darwin/arm64 linux/amd64
#
# Outputs (per platform):
#   macOS  -> build/bin/bili-FM.app  +  bili-FM-macos-<arch>.dmg
#   Windows-> build/bin/bili-FM-setup.exe  (NSIS installer)
#   Linux  -> build/bin/bili-FM      +  .deb / .rpm / .AppImage
#
# Requires native tooling for each platform:
#   macOS: hdiutil (built-in)
#   Windows: makensis (installed by `wails build -nsis`)
#   Linux: dpkg-deb + rpm (rpmbuild) + curl (AppImage tools)
set -euo pipefail

VERSION=$(grep -oP 'const APP_VERSION = "\K[^"]+' service/config.go)
PLATFORMS=("${@:-darwin/arm64 darwin/amd64 windows/amd64 linux/amd64}")

for PLATFORM in "${PLATFORMS[@]}"; do
  OS=$(echo "$PLATFORM" | cut -d/ -f1)
  ARCH=$(echo "$PLATFORM" | cut -d/ -f2)
  echo "=== Building ${PLATFORM} (v${VERSION}) ==="

  case "$OS" in
    darwin)
      wails build -platform "$PLATFORM" -clean
      test -d "build/bin/bili-FM.app" || { echo "build/bin/bili-FM.app missing" >&2; exit 1; }
      hdiutil create \
        -volname "bili-FM" \
        -srcfolder "build/bin/bili-FM.app" \
        -ov -format UDZO \
        "bili-FM-macos-${ARCH}.dmg"
      ;;

    windows)
      wails build -platform "$PLATFORM" -clean -nsis
      echo "NSIS installer: build/bin/bili-FM-setup.exe"
      ;;

    linux)
      wails build -platform "$PLATFORM" -clean
      test -x "build/bin/bili-FM" || { echo "build/bin/bili-FM missing" >&2; exit 1; }
      ./build/linux/build-deb.sh "$VERSION" "$ARCH" "build/bin/bili-FM" .
      ./build/linux/build-rpm.sh "$VERSION" "$ARCH" "build/bin/bili-FM" .
      ./build/linux/build-appimage.sh "$ARCH" "build/bin/bili-FM" .
      ;;

    *)
      echo "Unknown OS: $OS (expected darwin|windows|linux)" >&2
      exit 1
      ;;
  esac
done

echo "=== All builds complete ==="