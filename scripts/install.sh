#!/usr/bin/env bash
#
# bili-FM install script
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash
#
set -euo pipefail

REPO="vst93/bili-fm"
GITHUB_API="https://api.github.com/repos/${REPO}/releases/latest"
APP_NAME="bili-FM"
INSTALL_DIR="${HOME}/.local/bin"
APPDIR_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/512x512/apps"

# ── helpers ──────────────────────────────────────────────────────────────────

info()  { printf "\033[1;34m==>\033[0m %s\n" "$*"; }
warn()  { printf "\033[1;33m!!\033[0m %s\n" "$*"; }
error() { printf "\033[1;31m!!\033[0m %s\n" "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || error "需要 '$1' 但未找到，请先安装。"; }

# ── detect platform ──────────────────────────────────────────────────────────

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux)  echo "linux" ;;
    *)      error "不支持的操作系统: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64" ;;
    arm64|aarch64) echo "arm64" ;;
    *)             error "不支持的架构: $(uname -m)" ;;
  esac
}

# ── fetch latest release tag ─────────────────────────────────────────────────

get_latest_tag() {
  local tag
  if command -v gh >/dev/null 2>&1; then
    tag=$(gh release view --repo "$REPO" --json tagName -q '.tagName' 2>/dev/null || true)
  fi
  if [ -z "$tag" ]; then
    tag=$(curl -fsSL "$GITHUB_API" | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  fi
  [ -n "$tag" ] || error "无法获取最新版本号。"
  echo "$tag"
}

# ── macOS install ────────────────────────────────────────────────────────────

install_macos() {
  local arch="$1" tag="$2"
  local asset_prefix

  case "$arch" in
    arm64)   asset_prefix="bili-FM-macos-apple-silicon" ;;
    x86_64)  asset_prefix="bili-FM-macos-intel" ;;
  esac

  local dmg_url="https://github.com/${REPO}/releases/download/${tag}/${asset_prefix}.dmg"

  info "检测到 macOS (${arch})"
  info "最新版本: ${tag}"

  # Prefer Homebrew if available
  if command -v brew >/dev/null 2>&1; then
    info "检测到 Homebrew，使用 brew 安装..."
    brew install vst93/tap/bili-fm
    info "安装完成！运行: bili-fm"
    return
  fi

  # Fall back to manual .dmg install
  need curl
  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT

  local dmg="${tmpdir}/${asset_prefix}.dmg"
  info "下载 ${dmg_url} ..."
  curl -fSL -o "$dmg" "$dmg_url"

  # Mount, copy, unmount
  local mountpoint
  mountpoint="$(hdiutil attach "$dmg" -nobrowse -quiet | grep '/Volumes/' | tail -1 | awk '{print $NF}')"

  info "复制到 /Applications ..."
  cp -R "${mountpoint}/bili-FM.app" /Applications/

  hdiutil detach "$mountpoint" -quiet

  # Remove quarantine
  xattr -dr com.apple.quarantine /Applications/bili-FM.app 2>/dev/null || true

  info "安装完成！在启动台或 /Applications 中打开 bili-FM。"
}

# ── Linux install ────────────────────────────────────────────────────────────

install_linux() {
  local arch="$1" tag="$2"

  info "检测到 Linux (${arch})"
  info "最新版本: ${tag}"

  # AppImage is universal — download to ~/.local/bin
  local appimage_name="bili-FM-linux-${arch}.AppImage"
  local appimage_url="https://github.com/${REPO}/releases/download/${tag}/${appimage_name}"

  need curl

  mkdir -p "$INSTALL_DIR"

  local target="${INSTALL_DIR}/bili-fm"
  info "下载 ${appimage_url} ..."
  curl -fSL -o "$target" "$appimage_url"
  chmod +x "$target"

  # Create .desktop entry
  mkdir -p "$APPDIR_DIR" "$ICON_DIR"

  # Download icon from repo
  local icon_url="https://raw.githubusercontent.com/${REPO}/${tag}/src-tauri/icons/icon.png"
  curl -fsSL -o "${ICON_DIR}/bili-fm.png" "$icon_url" 2>/dev/null || true

  cat > "${APPDIR_DIR}/bili-fm.desktop" <<EOF
[Desktop Entry]
Name=bili-FM
Comment=Listen to Bilibili content in audio-only mode
Exec=${target}
Icon=bili-fm
Type=Application
Categories=AudioVideo;Audio;Player;
Terminal=false
StartupWMClass=bili-FM
EOF

  # Update desktop database if possible
  update-desktop-database "$APPDIR_DIR" 2>/dev/null || true

  info "安装完成！"
  info "  命令行运行: bili-fm"
  info "  或从应用菜单启动 bili-FM"

  # Warn if PATH doesn't include install dir
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *) warn "${INSTALL_DIR} 不在 PATH 中，请将其添加到 ~/.bashrc 或 ~/.zshrc:"
       warn "  export PATH=\"${INSTALL_DIR}:\$PATH\"" ;;
  esac
}

# ── main ─────────────────────────────────────────────────────────────────────

main() {
  local os arch tag

  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(get_latest_tag)"

  info "bili-FM 安装脚本"

  case "$os" in
    macos) install_macos "$arch" "$tag" ;;
    linux) install_linux  "$arch" "$tag" ;;
  esac
}

main "$@"
