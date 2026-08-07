#!/usr/bin/env bash

set -euo pipefail

if [[ $# -ne 4 ]]; then
  echo "Usage: $0 <deb> <version> <x86_64|arm64> <output.pkg.tar.zst>" >&2
  exit 2
fi

deb_path="$1"
version="$2"
release_arch="$3"
output_path="$4"

for command_name in ar tar zstd; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Missing required command: $command_name" >&2
    exit 1
  fi
done

[[ -f "$deb_path" ]] || { echo "Deb package not found: $deb_path" >&2; exit 1; }

case "$release_arch" in
  x86_64) package_arch="x86_64" ;;
  arm64) package_arch="aarch64" ;;
  *) echo "Unsupported release architecture: $release_arch" >&2; exit 1 ;;
esac

# Arch package versions cannot contain hyphens.
package_version="${version//-/_}"
if [[ ! "$package_version" =~ ^[0-9A-Za-z.+_]+$ ]]; then
  echo "Version cannot be represented as an Arch package version: $version" >&2
  exit 1
fi

deb_path="$(cd "$(dirname "$deb_path")" && pwd)/$(basename "$deb_path")"
mkdir -p "$(dirname "$output_path")"
output_path="$(cd "$(dirname "$output_path")" && pwd)/$(basename "$output_path")"

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
package_root="${work_dir}/root"
mkdir -p "$package_root"

data_member="$(ar t "$deb_path" | sed -n '/^data\.tar\./p' | sed -n '1p')"
[[ -n "$data_member" ]] || { echo "The deb package has no data archive" >&2; exit 1; }
case "$data_member" in
  *.tar.gz) tar_compression=(-z) ;;
  *.tar.xz) tar_compression=(-J) ;;
  *.tar.zst) tar_compression=(--zstd) ;;
  *.tar.bz2) tar_compression=(-j) ;;
  *.tar.lzma) tar_compression=(--lzma) ;;
  *) tar_compression=() ;;
esac
ar p "$deb_path" "$data_member" | tar "${tar_compression[@]}" -xf - -C "$package_root"
[[ -x "${package_root}/usr/bin/bili-fm" ]] || {
  echo "The deb package does not contain usr/bin/bili-fm" >&2
  exit 1
}

# Lets the app disable Tauri's deb/rpm installer on pacman-managed systems.
mkdir -p "${package_root}/usr/share/bili-fm"
printf 'pacman\n' > "${package_root}/usr/share/bili-fm/package-manager-pacman"

installed_size="$(du -sk "$package_root" | awk '{print $1 * 1024}')"
build_date="${SOURCE_DATE_EPOCH:-$(date +%s)}"

cat > "${package_root}/.PKGINFO" <<EOF
pkgname = bili-fm
pkgbase = bili-fm
pkgver = ${package_version}-1
pkgdesc = Listen to Bilibili content in audio-only mode
url = https://github.com/vst93/bili-fm
builddate = ${build_date}
packager = bili-FM GitHub Actions
size = ${installed_size}
arch = ${package_arch}
license = MIT
depend = webkit2gtk-4.1
depend = libappindicator-gtk3
depend = librsvg
EOF

if tar --version 2>/dev/null | grep -q 'GNU tar'; then
  tar \
    --sort=name \
    --mtime="@${build_date}" \
    --owner=0 \
    --group=0 \
    --numeric-owner \
    -C "$package_root" \
    -cf - .PKGINFO usr \
    | zstd -19 -T0 -f -o "$output_path"
else
  tar --uid 0 --gid 0 -C "$package_root" -cf - .PKGINFO usr \
    | zstd -19 -T0 -f -o "$output_path"
fi

echo "Created $output_path"
