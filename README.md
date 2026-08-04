# Bili FM

[English](#english) | [中文](#中文)

---

## 中文

![Bili FM 截图](screenshot02.png)

Bili FM 是一款通过音频收听 B 站视频内容的跨平台桌面应用，支持 Windows、macOS 和 Linux。它可以作为轻量音乐播放器，也适合用于课程、访谈、播客类视频和长视频内容的后台收听。

### 功能特性

- 采用液态玻璃风格 UI，半透明毛玻璃效果适配亮暗环境
- 支持关键词搜索 B 站视频，并可按时间、热度等条件排序
- 登录后可查看订阅、收藏、推荐等个人内容
- 支持播放、暂停、上一集、下一集、播放列表等常用播放控制
- 支持弹幕列表展示，方便在听音频时快速浏览视频互动内容
- 视频播放浮窗全屏铺满，带模糊遮罩与过渡动画
- 点击 UP 主名称或头像，可打开 UP 主作品列表
- 支持点赞、投币等常用互动操作
- 支持 Windows、macOS 和 Linux 多平台使用
- 应用内自动更新（Gitee 优先，GitHub 兜底）

### 适用场景

B 站电脑端暂未提供完整的"听视频"体验，Bili FM 主要面向以下场景：

- 后台收听知识类、访谈类、课程类和长视频内容
- 将视频内容作为音频播放，减少画面干扰
- 快速管理播放列表，连续收听多个视频
- 在桌面端获得更接近音乐播放器的 B 站收听体验

### 快捷键

| 快捷键 | 功能 |
| --- | --- |
| 空格 | 暂停 / 开始播放 |
| ← | 上一集 |
| -> | 下一集 |

### 安装

#### 一键安装（macOS / Linux）

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash
```

脚本会自动检测平台和架构，下载并安装最新稳定版。macOS 上优先使用 Homebrew，Linux 上安装 AppImage 并创建桌面快捷方式。

**安装预览版：**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --pre-release
```

**安装指定版本：**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --version 2.0.8-preview
```

#### Windows

从 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载 `.exe` 安装文件，双击运行即可。

> 首次安装时 Windows SmartScreen 可能提示"未知发布者"，点击"仍要运行"即可。

#### macOS

**Homebrew 安装（推荐）：**

```bash
brew install vst93/tap/bili-fm
```

更新到最新版本：

```bash
brew update
brew upgrade bili-fm
```

**手动安装：** 从 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载 `.dmg` 文件，打开后将 bili-FM 拖入 Applications 文件夹。

> Apple Silicon (M1/M2/M3/M4) 下载 `macos-apple-silicon` 版本，Intel 芯片下载 `macos-intel` 版本。

#### Linux

##### Arch Linux

从 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载对应包手动安装：

```bash
# AppImage（推荐，无需安装依赖）
chmod +x bili-FM-linux-x86_64.AppImage
./bili-FM-linux-x86_64.AppImage

# 或用 debtap 转换 .deb 包后安装
debtap bili-FM-linux-x86_64.deb
sudo pacman -U bili-fm-*.pkg.tar.zst
```

> **webkit2gtk 依赖**：Tauri v2 应用依赖 `webkit2gtk-4.1`。Arch Linux 默认仓库已包含此依赖，安装包时会自动拉取。

##### Ubuntu / Debian

```bash
sudo dpkg -i bili-FM-linux-x86_64.deb
sudo apt-get install -f  # 自动安装缺失依赖
```

> 需要 Ubuntu 24.04+（webkit2gtk-4.1）。Ubuntu 22.04 仅有 4.0 版本，不兼容预编译包。

> **EGL 错误排查**：如果启动时遇到 `Could not create default EGL display: EGL_BAD_PARAMETER`，这是 WebKit2GTK 的 dmabuf 渲染器在某些 GPU 驱动上的已知问题。通过安装脚本安装的版本已自动注入 `WEBKIT_DISABLE_DMABUF_RENDERER=1` 规避此问题；手动安装的用户可设置该环境变量后运行。

### macOS 特殊权限说明

由于应用未经 Apple Developer ID 签名和公证（notarization），首次打开时 macOS Gatekeeper 会拦截，提示：

> "bili-FM"已损坏，无法打开。你应该将它移到废纸篓。

在终端执行以下命令移除 quarantine 标记：

```bash
xattr -dr com.apple.quarantine /Applications/bili-FM.app
```

如果应用仍在下载目录，请将路径改为实际位置：

```bash
xattr -dr com.apple.quarantine ~/Downloads/bili-FM.app
```

执行后再次打开应用即可。

> 后续版本计划进行 Apple Developer ID 签名，届时无需此操作。

### Linux 补充说明

#### AppImage 无法运行

如果 AppImage 双击无反应，可能是缺少 FUSE 支持：

```bash
# Arch Linux
sudo pacman -S fuse2

# Ubuntu / Debian
sudo apt install libfuse2
```

#### 系统托盘不显示

部分桌面环境（如 GNOME）默认不启用托盘图标，需安装扩展：

- **GNOME**：安装 [AppIndicator 扩展](https://extensions.gnome.org/extension/615/appindicator-support-for-gnome-shell/)
- **KDE Plasma**：原生支持，无需额外配置

#### Wayland 下窗口装饰异常

如果使用 Wayland 且窗口标题栏显示异常，尝试设置环境变量：

```bash
WEBKIT_DISABLE_COMPOSITING_MODE=1 bili-fm
```

### 应用更新

Bili FM 内置自动更新功能：

1. 点击标题栏设置按钮 → "检查更新"
2. 弹窗即时显示加载动画，后台并行请求 Gitee 和 GitHub 检查新版本
3. 发现新版本后点击"立即更新"，应用内下载并自动安装重启
4. 国内用户优先从 Gitee 获取更新，网络不通时自动回退到 GitHub

> 也可手动前往 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载最新版本覆盖安装。

### 开发说明

- 项目使用 **Tauri v2**（Rust + React）开发
- 前端使用 React + HeroUI + TailwindCSS
- 后端使用 Rust，内嵌 HTTP 图片代理解决 B 站 CDN 防盗链
- 登录态兼容旧版 Wails 的 dkv 存储格式，升级无需重新登录
- 项目开源，欢迎提出 Issue、建议或 Pull Request

项目地址：[https://github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)

### 免责声明

本项目仅用于开发和学习。项目初衷是方便个人收听 B 站节目，不提供任何内容存储、分发或破解能力。所有视频、音频、弹幕等内容版权归原作者及哔哩哔哩所有。如有侵权，请联系删除。

### 感谢以下项目

- [Tauri](https://github.com/tauri-apps/tauri)
- [HeroUI](https://github.com/heroui-inc/heroui)
- [IconPark](https://github.com/bytedance/iconpark)
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)

---

## English

![Bili FM Screenshot](screenshot02.png)

Bili FM is a cross-platform desktop application that lets you listen to Bilibili video content as audio. It supports Windows, macOS, and Linux. It works as a lightweight music player and is also great for courses, interviews, podcasts, and long-form video content for background listening.

### Features

- Liquid glass UI design with translucent frosted glass effect that adapts to light/dark environments
- Search Bilibili videos by keyword, with sorting by time, popularity, etc.
- After login, access subscriptions, favorites, recommendations, and more
- Playback controls: play, pause, previous, next, playlist management
- Danmaku (bullet comments) list display for browsing interactions while listening
- Full-screen video player overlay with blur backdrop and smooth transitions
- Click a creator's name or avatar to open their video list
- Like, coin, and other common interactions supported
- Cross-platform: Windows, macOS, and Linux
- In-app auto-update (Gitee-first, GitHub fallback)

### Use Cases

Bilibili's desktop client does not offer a complete "listen to video" experience. Bili FM is designed for:

- Background listening to educational, interview, course, and long-form content
- Playing video content as audio to reduce visual distraction
- Quick playlist management for continuous listening
- A more music-player-like Bilibili experience on desktop

### Keyboard Shortcuts

| Shortcut | Action |
| --- | --- |
| Space | Pause / Play |
| ← | Previous |
| -> | Next |

### Installation

#### Quick Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash
```

The script auto-detects your platform and architecture, then downloads and installs the latest stable release. On macOS it prefers Homebrew; on Linux it installs the AppImage and creates a desktop shortcut.

**Install pre-release:**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --pre-release
```

**Install a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/tauri-rewrite/scripts/install.sh | bash -s -- --version 2.0.8-preview
```

#### Windows

Download the `.exe` installer from [GitHub Releases](https://github.com/vst93/bili-fm/releases) and run it.

> On first install, Windows SmartScreen may show "Unknown publisher" - click "Run anyway".

#### macOS

**Homebrew install (recommended):**

```bash
brew install vst93/tap/bili-fm
```

Update to the latest version:

```bash
brew update
brew upgrade bili-fm
```

**Manual install:** Download the `.dmg` file from [GitHub Releases](https://github.com/vst93/bili-fm/releases), open it and drag bili-FM to the Applications folder.

> Apple Silicon (M1/M2/M3/M4): download `macos-apple-silicon`. Intel: download `macos-intel`.

#### Linux

##### Arch Linux

Download from [GitHub Releases](https://github.com/vst93/bili-fm/releases) and install manually:

```bash
# AppImage (recommended, no dependencies needed)
chmod +x bili-FM-linux-x86_64.AppImage
./bili-FM-linux-x86_64.AppImage

# Or convert the .deb package with debtap and install
debtap bili-FM-linux-x86_64.deb
sudo pacman -U bili-fm-*.pkg.tar.zst
```

> **webkit2gtk dependency**: Tauri v2 apps require `webkit2gtk-4.1`. Arch Linux repos include this by default; it will be pulled in automatically.

##### Ubuntu / Debian

```bash
sudo dpkg -i bili-FM-linux-x86_64.deb
sudo apt-get install -f  # auto-install missing dependencies
```

> Requires Ubuntu 24.04+ (webkit2gtk-4.1). Ubuntu 22.04 only has 4.0, which is incompatible.

> **EGL troubleshooting**: If you see `Could not create default EGL display: EGL_BAD_PARAMETER` on launch, this is a known WebKit2GTK dmabuf renderer issue on certain GPU drivers. The install script automatically injects `WEBKIT_DISABLE_DMABUF_RENDERER=1` to work around this; for manual installs, set that environment variable before running.

### macOS Permissions

Since the app is not signed with an Apple Developer ID or notarized, macOS Gatekeeper will block it on first launch:

> "bili-FM" is damaged and can't be opened. You should move it to the Trash.

Remove the quarantine attribute in Terminal:

```bash
xattr -dr com.apple.quarantine /Applications/bili-FM.app
```

If the app is still in your Downloads folder, adjust the path:

```bash
xattr -dr com.apple.quarantine ~/Downloads/bili-FM.app
```

After that, open the app again.

> Future versions plan to include Apple Developer ID signing.

### Linux Notes

#### AppImage won't run

If the AppImage doesn't launch on double-click, you may be missing FUSE support:

```bash
# Arch Linux
sudo pacman -S fuse2

# Ubuntu / Debian
sudo apt install libfuse2
```

#### System tray not showing

Some desktop environments (e.g., GNOME) don't enable tray icons by default:

- **GNOME**: Install the [AppIndicator extension](https://extensions.gnome.org/extension/615/appindicator-support-for-gnome-shell/)
- **KDE Plasma**: Supported natively, no extra config needed

#### Window decoration issues on Wayland

If window title bars render incorrectly under Wayland, try:

```bash
WEBKIT_DISABLE_COMPOSITING_MODE=1 bili-fm
```

### Updates

Bili FM includes built-in auto-update:

1. Click the settings button in the title bar → "Check for Updates"
2. A loading dialog appears instantly, checking Gitee and GitHub in parallel
3. When an update is found, click "Update Now" to download and install in-app
4. China users get updates from Gitee first, falling back to GitHub automatically

> You can also manually download from [GitHub Releases](https://github.com/vst93/bili-fm/releases).

### Development

- Built with **Tauri v2** (Rust + React)
- Frontend: React + HeroUI + TailwindCSS
- Backend: Rust with embedded HTTP image proxy for Bilibili CDN
- Login state is compatible with the legacy Wails dkv storage format
- Open source — issues, suggestions, and pull requests welcome

Repository: [https://github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)

### Disclaimer

This project is for development and learning purposes only. The original goal is to facilitate personal listening to Bilibili content. It does not provide any content storage, distribution, or circumvention capabilities. All video, audio, and danmaku content belongs to the respective creators and Bilibili. If there is any infringement, please contact us for removal.

### Acknowledgements

- [Tauri](https://github.com/tauri-apps/tauri)
- [HeroUI](https://github.com/heroui-inc/heroui)
- [IconPark](https://github.com/bytedance/iconpark)
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
