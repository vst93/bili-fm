#!/usr/bin/env bash
#
# bili-FM install script
#
# Usage:
#   # Latest stable (default)
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash
#
#   # Latest including pre-releases
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --pre-release
#
#   # Specific version
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --version 2.0.8-preview
#
set -euo pipefail

REPO="vst93/bili-fm"
APP_NAME="bili-FM"
INSTALL_DIR="${HOME}/.local/bin"
APPDIR_DIR="${HOME}/.local/share/applications"
ICON_DIR="${HOME}/.local/share/icons/hicolor/512x512/apps"

# ── helpers ──────────────────────────────────────────────────────────────────

info()  { printf "\033[1;34m==>\033[0m %s\n" "$*" >&2; }
warn()  { printf "\033[1;33m!!\033[0m %s\n" "$*" >&2; }
error() { printf "\033[1;31m!!\033[0m %s\n" "$*" >&2; exit 1; }

need() { command -v "$1" >/dev/null 2>&1 || error "需要 '$1' 但未找到，请先安装。"; }

usage() {
  cat <<'EOF'
bili-FM 安装脚本

用法:
  install.sh [选项]

选项:
  -v, --version <tag>   安装指定版本 (如 2.0.8-preview)
  -p, --pre-release     安装最新版本 (包括预览版)
  -h, --help            显示帮助

示例:
  # 最新稳定版 (默认)
  install.sh

  # 最新版本，含预览版
  install.sh --pre-release

  # 指定版本
  install.sh --version 2.0.8-preview
EOF
  exit 0
}

# ── parse args ───────────────────────────────────────────────────────────────

OPT_VERSION=""
OPT_PRE_RELEASE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--version)    OPT_VERSION="$2"; shift 2 ;;
    -p|--pre-release) OPT_PRE_RELEASE=true; shift ;;
    -h|--help)       usage ;;
    *)               error "未知参数: $1 (使用 --help 查看帮助)" ;;
  esac
done

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

# ── resolve version ──────────────────────────────────────────────────────────

resolve_tag() {
  # If user specified a version, use it directly
  if [[ -n "$OPT_VERSION" ]]; then
    info "指定版本: ${OPT_VERSION}"
    printf '%s\n' "$OPT_VERSION"
    return
  fi

  local api_url tag

  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    # List all releases (includes pre-releases), take the first one
    api_url="https://api.github.com/repos/${REPO}/releases?per_page=1"
    tag=$(curl -fsSL "$api_url" | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  else
    # Latest stable release only
    api_url="https://api.github.com/repos/${REPO}/releases/latest"
    tag=$(curl -fsSL "$api_url" | grep '"tag_name"' | head -1 | sed -E 's/.*"([^"]+)".*/\1/')
  fi

  [[ -n "$tag" ]] || error "无法获取版本号。请检查网络或使用 --version 指定版本。"

  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    info "最新版本 (含预览版): ${tag}"
  else
    info "最新稳定版: ${tag}"
  fi

  printf '%s\n' "$tag"
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

  # Use Homebrew only for latest stable; for specific/pre-release versions, go manual
  if [[ -z "$OPT_VERSION" && "$OPT_PRE_RELEASE" == "false" ]] && command -v brew >/dev/null 2>&1; then
    info "检测到 Homebrew，使用 brew 安装..."
    brew install vst93/tap/bili-fm
    info "安装完成！运行: bili-fm"
    return
  fi

  # Manual .dmg install
  need curl
  if [[ "$OPT_PRE_RELEASE" == "true" || -n "$OPT_VERSION" ]]; then
    info "使用手动安装 (Homebrew 仅支持最新稳定版)..."
  fi

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

  need curl

  # Arch Linux / pacman 系统: AppImage 在 Mesa 26+ 上有 EGL_BAD_PARAMETER 崩溃,
  # 改用 .deb + debtap 转换安装, 走系统 webkit2gtk/mesa。
  if command -v pacman &>/dev/null; then
    install_linux_arch "$arch" "$tag"
    return
  fi

  # Debian/Ubuntu: 直接用 .deb
  if command -v dpkg &>/dev/null; then
    install_linux_deb "$arch" "$tag"
    return
  fi

  # 其他发行版: 回退到 AppImage
  install_linux_appimage "$arch" "$tag"
}

# ── Arch Linux (pacman + debtap) ─────────────────────────────────────────────

install_linux_arch() {
  local arch="$1" tag="$2"
  local deb_name="bili-FM-linux-${arch}.deb"
  local deb_url="https://github.com/${REPO}/releases/download/${tag}/${deb_name}"

  info "检测到 Arch Linux (pacman)"
  info ""
  info "⚠ 为什么不装 AppImage？"
  info "  AppImage 自带的 webkit2gtk 库在 Arch 的滚动版 Mesa 驱动上"
  info "  会触发 EGL_BAD_PARAMETER 崩溃（已知 bug，影响所有 Tauri AppImage）。"
  info "  改用 .deb 包解压安装，走系统 webkit2gtk/mesa，可避免此问题。"
  info ""

  # 自动安装缺失依赖
  local missing=()
  pacman -Q binutils &>/dev/null || missing+=("binutils")
  pacman -Q webkit2gtk-4.1 &>/dev/null || missing+=("webkit2gtk-4.1")
  pacman -Q librsvg &>/dev/null || missing+=("librsvg")
  if [ ${#missing[@]} -gt 0 ]; then
    warn "安装缺失依赖: ${missing[*]}"
    sudo pacman -S --noconfirm --needed "${missing[@]}"
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' RETURN

  local deb="${tmpdir}/${deb_name}"
  info "下载 ${deb_url} ..."
  curl -fSL -o "$deb" "$deb_url"

  info "解压 .deb ..."
  cd "$tmpdir"
  # .deb 是 ar 归档，里面含 data.tar.*
  ar x "$deb_name"
  # 解压 data.tar
  tar xf data.tar.*

  # 手动复制文件到系统目录
  info "安装文件..."
  # 二进制
  if [ -f usr/bin/bili-fm ]; then
    sudo cp usr/bin/bili-fm /usr/bin/bili-fm
    sudo chmod +x /usr/bin/bili-fm
  fi
  # 资源目录 (如果有)
  if [ -d usr/lib/bili-fm ]; then
    sudo cp -r usr/lib/bili-fm /usr/lib/
  fi
  # .desktop
  if [ -f usr/share/applications/*.desktop ]; then
    sudo cp usr/share/applications/*.desktop /usr/share/applications/ 2>/dev/null || true
  fi
  # 图标
  if [ -d usr/share/icons ]; then
    sudo cp -r usr/share/icons/* /usr/share/icons/ 2>/dev/null || true
  fi
  cd - >/dev/null

  # 确保有 .desktop 和图标 (如果 .deb 里没有)
  mkdir -p "$APPDIR_DIR" "$ICON_DIR"
  local icon_url="https://raw.githubusercontent.com/${REPO}/${tag}/src-tauri/icons/icon.png"
  curl -fsSL -o "${ICON_DIR}/bili-fm.png" "$icon_url" 2>/dev/null || true

  if [ ! -f /usr/share/applications/bili-fm.desktop ]; then
    sudo tee /usr/share/applications/bili-fm.desktop <<EOF
[Desktop Entry]
Name=bili-FM
Comment=Listen to Bilibili content in audio-only mode
Exec=bili-fm
Icon=bili-fm
Type=Application
Categories=AudioVideo;Audio;Player;
Terminal=false
StartupWMClass=bili-FM
EOF
  fi
  update-desktop-database /usr/share/applications 2>/dev/null || true

  info "安装完成！"
  info "  命令行运行: bili-fm"
  info "  或从应用菜单启动 bili-FM"

  _warn_path
}

# ── Debian/Ubuntu (dpkg) ─────────────────────────────────────────────────────

install_linux_deb() {
  local arch="$1" tag="$2"
  local deb_name="bili-FM-linux-${arch}.deb"
  local deb_url="https://github.com/${REPO}/releases/download/${tag}/${deb_name}"

  info "检测到 Debian/Ubuntu (dpkg)"

  # 自动安装缺失依赖
  local missing=()
  dpkg -s libwebkit2gtk-4.1-0 &>/dev/null || missing+=("libwebkit2gtk-4.1-0")
  dpkg -s librsvg2-2 &>/dev/null || missing+=("librsvg2-2")
  if [ ${#missing[@]} -gt 0 ]; then
    warn "安装缺失依赖: ${missing[*]}"
    sudo apt-get update -qq
    sudo apt-get install -y "${missing[@]}"
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' RETURN

  local deb="${tmpdir}/${deb_name}"
  info "下载 ${deb_url} ..."
  curl -fSL -o "$deb" "$deb_url"

  info "安装 ${deb_name} ..."
  sudo dpkg -i "$deb" 2>/dev/null || sudo apt-get install -f -y

  info "安装完成！"
  info "  命令行运行: bili-fm"
  info "  或从应用菜单启动 bili-FM"
}

# ── AppImage (fallback) ──────────────────────────────────────────────────────

install_linux_appimage() {
  local arch="$1" tag="$2"
  local appimage_name="bili-FM-linux-${arch}.AppImage"
  local appimage_url="https://github.com/${REPO}/releases/download/${tag}/${appimage_name}"

  info "使用 AppImage 安装"
  warn "注意: AppImage 在某些滚动发行版 (Arch/Fedora) + Mesa 26+ 上可能白屏。"
  warn "如遇白屏, 请改用 .deb 包安装。"

  mkdir -p "$INSTALL_DIR"

  local appimage="${INSTALL_DIR}/${appimage_name}"
  info "下载 ${appimage_url} ..."
  curl -fSL -o "$appimage" "$appimage_url"
  chmod +x "$appimage"

  # Wrapper: 设置 WebKit2GTK 兼容环境变量
  local wrapper="${INSTALL_DIR}/bili-fm"
  local WAYLAND_ENV=""
  if [ -n "${WAYLAND_DISPLAY:-}" ] || [ "${XDG_SESSION_TYPE:-}" = "wayland" ]; then
    WAYLAND_ENV="WEBKIT_DISABLE_COMPOSITING_MODE=1 "
  fi

  cat > "$wrapper" <<EOF
#!/usr/bin/env bash
exec env WEBKIT_DISABLE_DMABUF_RENDERER=1 ${WAYLAND_ENV}"\${BASH_SOURCE[0]%/*}/${appimage_name}" "\$@"
EOF
  chmod +x "$wrapper"

  # .desktop entry
  mkdir -p "$APPDIR_DIR" "$ICON_DIR"
  local icon_url="https://raw.githubusercontent.com/${REPO}/${tag}/src-tauri/icons/icon.png"
  curl -fsSL -o "${ICON_DIR}/bili-fm.png" "$icon_url" 2>/dev/null || true

  cat > "${APPDIR_DIR}/bili-fm.desktop" <<EOF
[Desktop Entry]
Name=bili-FM
Comment=Listen to Bilibili content in audio-only mode
Exec=${wrapper}
Icon=bili-fm
Type=Application
Categories=AudioVideo;Audio;Player;
Terminal=false
StartupWMClass=bili-FM
EOF
  update-desktop-database "$APPDIR_DIR" 2>/dev/null || true

  info "安装完成！"
  info "  命令行运行: bili-fm"
  info "  或从应用菜单启动 bili-FM"

  _warn_path
}

_warn_path() {
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      warn "${INSTALL_DIR} 不在 PATH 中，请将其添加到 ~/.bashrc 或 ~/.zshrc:"
      warn "  export PATH=\"${INSTALL_DIR}:\$PATH\""
      ;;
  esac
}

# ── main ─────────────────────────────────────────────────────────────────────

main() {
  local os arch tag

  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(resolve_tag)"

  info "bili-FM 安装脚本"

  case "$os" in
    macos) install_macos "$arch" "$tag" ;;
    linux) install_linux  "$arch" "$tag" ;;
  esac
}

main
