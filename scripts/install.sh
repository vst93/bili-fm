#!/usr/bin/env bash
#
# bili-FM installer for macOS and Linux.
#
# Examples:
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --pre-release
#   curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --version 2.0.0

set -euo pipefail

REPO="vst93/bili-fm"
OPT_VERSION=""
OPT_PRE_RELEASE=false
OPT_YES=false
TEMP_DIR=""

if [[ -t 2 ]]; then
  BLUE='\033[1;34m'
  YELLOW='\033[1;33m'
  RED='\033[1;31m'
  RESET='\033[0m'
else
  BLUE=''
  YELLOW=''
  RED=''
  RESET=''
fi

info()  { printf "%b==>%b %s\n" "$BLUE" "$RESET" "$*" >&2; }
warn()  { printf "%b!!%b %s\n" "$YELLOW" "$RESET" "$*" >&2; }
error() { printf "%b!!%b %s\n" "$RED" "$RESET" "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || error "需要 '$1'，请先安装后重试。"
}

cleanup() {
  if [[ -n "$TEMP_DIR" && -d "$TEMP_DIR" ]]; then
    rm -rf "$TEMP_DIR"
  fi
}
trap cleanup EXIT

usage() {
  cat <<'EOF'
bili-FM 安装脚本

用法:
  install.sh [选项]

选项:
  -v, --version <tag>  安装指定版本 (如 2.0.0)
  -p, --pre-release    安装最新版本 (包括预览版)
  -y, --yes            不询问确认，适用于自动化环境
  -h, --help           显示帮助
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -v|--version)
      [[ $# -ge 2 && -n "$2" && "$2" != -* ]] || error "$1 需要一个版本号。"
      OPT_VERSION="$2"
      shift 2
      ;;
    -p|--pre-release)
      OPT_PRE_RELEASE=true
      shift
      ;;
    -y|--yes)
      OPT_YES=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *) error "未知参数: $1 (使用 --help 查看帮助)" ;;
  esac
done

if [[ -n "$OPT_VERSION" && "$OPT_PRE_RELEASE" == "true" ]]; then
  error "--version 和 --pre-release 不能同时使用。"
fi

detect_os() {
  case "$(uname -s)" in
    Darwin) echo "macos" ;;
    Linux) echo "linux" ;;
    *) error "不支持的操作系统: $(uname -s)" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64|amd64) echo "x86_64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) error "不支持的架构: $(uname -m)" ;;
  esac
}

resolve_tag() {
  if [[ -n "$OPT_VERSION" ]]; then
    info "指定版本: ${OPT_VERSION}"
    printf '%s\n' "$OPT_VERSION"
    return
  fi

  local api_url response tag
  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    api_url="https://api.github.com/repos/${REPO}/releases?per_page=20"
  else
    api_url="https://api.github.com/repos/${REPO}/releases/latest"
  fi

  response="$(curl -fsSL --retry 3 --connect-timeout 15 "$api_url")" \
    || error "无法访问 GitHub Releases，请检查网络或使用 --version 指定版本。"
  tag="$(sed -nE 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/p' <<< "$response" | sed -n '1p')"
  [[ -n "$tag" ]] || error "无法从 GitHub Releases 解析版本号。"

  if [[ "$OPT_PRE_RELEASE" == "true" ]]; then
    info "最新版本 (含预览版): ${tag}"
  else
    info "最新稳定版: ${tag}"
  fi
  printf '%s\n' "$tag"
}

make_temp_dir() {
  TEMP_DIR="$(mktemp -d)"
}

download_asset() {
  local url="$1" destination="$2"
  info "下载 $(basename "$destination") ..."
  if ! curl -fL --retry 3 --connect-timeout 15 --show-error --progress-bar \
    -o "$destination" "$url"; then
    error "下载失败: ${url}\n请确认该版本包含当前平台的安装包。"
  fi
}

confirm_action() {
  local prompt="$1" reply
  [[ "$OPT_YES" == "true" ]] && return

  printf "%b==>%b %s [Y/n] " "$BLUE" "$RESET" "$prompt" >/dev/tty 2>/dev/null \
    || error "当前环境无法交互确认；自动化安装请添加 --yes。"
  if ! IFS= read -r reply </dev/tty; then
    error "无法读取确认；自动化安装请添加 --yes。"
  fi
  case "$reply" in
    ''|y|Y|yes|YES) ;;
    *) error "已取消安装。" ;;
  esac
}

as_root() {
  if [[ ${EUID:-$(id -u)} -eq 0 ]]; then
    "$@"
  else
    need sudo
    sudo "$@"
  fi
}

install_macos() {
  local arch="$1" tag="$2" asset_prefix dmg_url dmg mountpoint

  case "$arch" in
    arm64) asset_prefix="bili-FM-${tag}-macos-apple-silicon" ;;
    x86_64) asset_prefix="bili-FM-${tag}-macos-intel" ;;
  esac

  info "检测到 macOS (${arch})"
  if [[ -z "$OPT_VERSION" && "$OPT_PRE_RELEASE" == "false" ]] \
    && command -v brew >/dev/null 2>&1; then
    info "使用 Homebrew 安装，后续可通过 brew upgrade bili-fm 更新。"
    brew install vst93/tap/bili-fm
    info "安装完成。"
    return
  fi

  need hdiutil
  make_temp_dir
  dmg_url="https://github.com/${REPO}/releases/download/${tag}/${asset_prefix}.dmg"
  dmg="${TEMP_DIR}/${asset_prefix}.dmg"
  download_asset "$dmg_url" "$dmg"

  mountpoint="$(hdiutil attach "$dmg" -nobrowse -quiet | sed -n 's#^.*\(/Volumes/.*\)$#\1#p' | tail -1)"
  [[ -n "$mountpoint" && -d "${mountpoint}/bili-FM.app" ]] || error "无法挂载安装镜像。"
  confirm_action "将 bili-FM 安装到 /Applications？"
  as_root ditto "${mountpoint}/bili-FM.app" /Applications/bili-FM.app
  hdiutil detach "$mountpoint" -quiet
  as_root xattr -dr com.apple.quarantine /Applications/bili-FM.app 2>/dev/null || true
  info "安装完成，可从启动台打开 bili-FM。"
}

find_legacy_arch_files() {
  local package="$1" relative target package_has_lowercase_desktop=false
  LEGACY_ARCH_FILES=()
  PACMAN_OVERWRITE_ARGS=()
  LEGACY_ARCH_CLEANUP_FILES=()

  while IFS= read -r relative; do
    relative="${relative#./}"
    case "$relative" in
      ''|.PKGINFO|.MTREE|.BUILDINFO|*/) continue ;;
    esac
    target="/${relative}"
    if [[ "$relative" == "usr/share/applications/bili-fm.desktop" ]]; then
      package_has_lowercase_desktop=true
    fi
    if [[ -e "$target" || -L "$target" ]] && ! pacman -Qo -- "$target" >/dev/null 2>&1; then
      LEGACY_ARCH_FILES+=("$target")
      PACMAN_OVERWRITE_ARGS+=(--overwrite "$relative")
    fi
  done < <(bsdtar -tf "$package")

  target="/usr/share/applications/bili-fm.desktop"
  if [[ "$package_has_lowercase_desktop" == "false" && -f "$target" ]] \
    && ! pacman -Qo -- "$target" >/dev/null 2>&1; then
    LEGACY_ARCH_FILES+=("$target")
    LEGACY_ARCH_CLEANUP_FILES+=("$target")
  fi
}

install_linux_arch() {
  local arch="$1" tag="$2" package_name package_url package
  package_name="bili-FM-${tag}-linux-${arch}.pkg.tar.zst"
  package_url="https://github.com/${REPO}/releases/download/${tag}/${package_name}"

  info "检测到 Arch Linux / pacman"
  need pacman
  need bsdtar
  make_temp_dir
  package="${TEMP_DIR}/${package_name}"
  download_asset "$package_url" "$package"

  find_legacy_arch_files "$package"
  if [[ ${#LEGACY_ARCH_FILES[@]} -gt 0 ]]; then
    warn "检测到旧安装脚本留下的未受 pacman 管理文件，将迁移给原生包："
    printf '  %s\n' "${LEGACY_ARCH_FILES[@]}" >&2
    confirm_action "使用 pacman 接管旧文件并安装 ${package_name}？"
  else
    confirm_action "使用 pacman 安装 ${package_name} 及所需依赖？"
  fi

  as_root pacman -U --needed --noconfirm "${PACMAN_OVERWRITE_ARGS[@]}" "$package"
  if [[ ${#LEGACY_ARCH_CLEANUP_FILES[@]} -gt 0 ]]; then
    as_root rm -f -- "${LEGACY_ARCH_CLEANUP_FILES[@]}"
  fi
  info "安装完成。以后可重新运行本脚本升级，卸载请执行: sudo pacman -R bili-fm"
}

install_linux_deb() {
  local arch="$1" tag="$2" package_name package_url package
  package_name="bili-FM-${tag}-linux-${arch}.deb"
  package_url="https://github.com/${REPO}/releases/download/${tag}/${package_name}"

  info "检测到 Debian / Ubuntu"
  need apt-get
  make_temp_dir
  package="${TEMP_DIR}/${package_name}"
  download_asset "$package_url" "$package"
  confirm_action "使用 apt 安装 ${package_name} 及所需依赖？"
  as_root apt-get install -y "$package"
  info "安装完成。以后可重新运行本脚本升级。"
}

install_linux_rpm() {
  local arch="$1" tag="$2" manager="$3" package_name package_url package
  package_name="bili-FM-${tag}-linux-${arch}.rpm"
  package_url="https://github.com/${REPO}/releases/download/${tag}/${package_name}"

  info "检测到 RPM 系发行版 (${manager})"
  make_temp_dir
  package="${TEMP_DIR}/${package_name}"
  download_asset "$package_url" "$package"
  confirm_action "使用 ${manager} 安装 ${package_name} 及所需依赖？"

  case "$manager" in
    dnf) as_root dnf install -y "$package" ;;
    yum) as_root yum install -y "$package" ;;
    zypper) as_root zypper --non-interactive install "$package" ;;
  esac
  info "安装完成。以后可重新运行本脚本升级。"
}

install_linux() {
  local arch="$1" tag="$2"
  info "检测到 Linux (${arch})"

  if command -v pacman >/dev/null 2>&1; then
    install_linux_arch "$arch" "$tag"
  elif command -v apt-get >/dev/null 2>&1 && command -v dpkg >/dev/null 2>&1; then
    install_linux_deb "$arch" "$tag"
  elif command -v dnf >/dev/null 2>&1; then
    install_linux_rpm "$arch" "$tag" dnf
  elif command -v yum >/dev/null 2>&1; then
    install_linux_rpm "$arch" "$tag" yum
  elif command -v zypper >/dev/null 2>&1; then
    install_linux_rpm "$arch" "$tag" zypper
  else
    error "暂不支持此 Linux 发行版。支持 pacman、apt、dnf、yum 和 zypper。\n可前往 https://github.com/${REPO}/releases 查看可用安装包。"
  fi

  check_gstreamer_plugins
}

# WebKitGTK 的媒体解码依赖系统 GStreamer 插件；部分最小化安装的发行版默认不带。
# 缺失时只提示，不代装。
check_gstreamer_plugins() {
  local missing=() pkg
  if command -v pacman >/dev/null 2>&1; then
    for pkg in gst-plugins-good gst-libav; do
      pacman -Qq "$pkg" &>/dev/null || missing+=("$pkg")
    done
  elif command -v dpkg >/dev/null 2>&1; then
    for pkg in gstreamer1.0-plugins-good gstreamer1.0-libav; do
      dpkg -s "$pkg" &>/dev/null || missing+=("$pkg")
    done
  elif command -v rpm >/dev/null 2>&1; then
    for pkg in gstreamer1-plugins-good gstreamer1-libav; do
      rpm -q "$pkg" &>/dev/null || missing+=("$pkg")
    done
  else
    return 0
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    local install_cmd="sudo pacman -S --needed"
    command -v apt-get >/dev/null 2>&1 && install_cmd="sudo apt-get install"
    command -v dnf >/dev/null 2>&1 && install_cmd="sudo dnf install"
    command -v yum >/dev/null 2>&1 && install_cmd="sudo yum install"
    command -v zypper >/dev/null 2>&1 && install_cmd="sudo zypper install"
    warn "未检测到 GStreamer 解码插件: ${missing[*]}"
    warn "缺失时音频/视频可能无法播放（首次播放卡住、界面无响应）。"
    warn "建议安装: ${install_cmd} ${missing[*]}"
  fi
}

main() {
  local os arch tag
  need curl
  os="$(detect_os)"
  arch="$(detect_arch)"
  tag="$(resolve_tag)"
  info "bili-FM 安装脚本"

  case "$os" in
    macos) install_macos "$arch" "$tag" ;;
    linux) install_linux "$arch" "$tag" ;;
  esac
}

main
