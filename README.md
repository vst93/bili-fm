# Bili FM

[English](#english) | [中文](#中文)

---

## 中文

![Bili FM 截图](screenshot02.png)

Bili FM 是一款通过音频收听 B 站视频内容的跨平台桌面应用，支持 Windows、macOS 和 Linux。它可以作为轻量音乐播放器，也适合用于课程、访谈、播客类视频和长视频内容的后台收听。

当前稳定版：**3.1.1**（mygo 原生版）。从旧 Tauri 版升级会自动迁移，详见[从 Tauri 版升级](#从-tauri-版升级)。

### 功能特性

- 通过音频收听 B 站视频：搜索、订阅、收藏、推荐一站式浏览
- 社区广告分段自动跳过（基于 SponsorBlock，可选开启）
- 0.5x ~ 3.0x 倍速、音量均衡（EQ）、弹幕列表
- UP 主合集与个人播放列表独立管理，自动保存恢复
- 点赞、投币等常用互动，全局媒体键随时控制播放
- 液态玻璃风格 UI，适配亮暗环境
- Windows、macOS、Linux 多平台；原生版稳定版支持应用内自动更新，Microsoft Store 版由商店更新

### 音量均衡（EQ）

不同 B 站视频的默认音量差异很大，切换视频时经常需要手动调音量。开启音量均衡后，应用通过动态范围压缩器自动平衡音量，安静的视频和响亮的视频都能以更接近的音量播放。默认关闭，点击播放栏的 EQ 按钮开启。

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
| 倍速 | 播放栏倍速按钮选择 |

### 安装

#### 一键安装（macOS / Linux）

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash
```

脚本会自动检测平台和架构，下载并安装最新稳定版。macOS 直接安装与 Release 对应的 `.dmg`；Linux 上使用发行版原生包，由 pacman、apt、dnf/yum 或 zypper 负责依赖、升级和卸载。

**安装预览版：**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --pre-release
```

**安装指定版本：**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --version 3.1.1
```

#### Windows

##### 微软应用商店（推荐）

现已上架 Microsoft Store，可直接在商店中搜索 **Bili FM** 安装，支持自动更新。

[Microsoft Store → Bili FM](https://apps.microsoft.com/detail/9N0LNL3JM3GG)

> 商店包更新需要微软审核；新版原生 MSIX 通过审核后会自动推送给商店用户。当前商店版本可能暂时滞后于 GitHub。

##### GitHub Release

如果更偏好手动安装包，可前往 GitHub Release 页面下载 `.exe` 安装文件：

[GitHub Releases](https://github.com/vst93/bili-fm/releases)

> 首次安装时 Windows SmartScreen 可能提示"未知发布者"，点击"仍要运行"即可。

#### macOS

**手动安装（推荐）：** 从 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载 `.dmg` 文件，打开后将 bili-FM 拖入 Applications 文件夹。

Homebrew tap（`vst93/tap/bili-fm`）可能暂时仍是旧 Tauri 版本；使用前请确认 tap 已更新到 3.1.x 原生版，否则请用上面的 dmg。

> Apple Silicon (M1/M2/M3/M4) 下载 `macos-apple-silicon` 版本，Intel 芯片下载 `macos-intel` 版本。

#### Linux

##### Arch Linux

推荐使用上方一键安装命令。脚本会下载原生 pacman 包，并自动处理旧版脚本手工安装遗留的未托管文件。也可从 [GitHub Releases](https://github.com/vst93/bili-fm/releases) 下载后手动安装：

```bash
sudo pacman -U ./bili-FM-3.1.1-linux-x86_64.pkg.tar.zst
```

pacman 会自动安装 `webkit2gtk-4.1` 等依赖，并完整跟踪应用文件。卸载可执行 `sudo pacman -R bili-fm`。

##### Ubuntu / Debian

```bash
sudo apt install ./bili-FM-3.1.1-linux-x86_64.deb
```

> 需要 Ubuntu 24.04+（webkit2gtk-4.1）。Ubuntu 22.04 仅有 4.0 版本，不兼容预编译包。

##### Fedora / RHEL / openSUSE

一键安装脚本会自动使用 dnf、yum 或 zypper。手动安装示例：

```bash
sudo dnf install ./bili-FM-3.1.1-linux-x86_64.rpm
```

Linux 不再发布 AppImage。AppImage 捆绑的 WebKit 运行时在部分 Mesa/滚动发行版环境中会出现白屏或 EGL 初始化失败；原生包直接使用系统 WebKit2GTK，兼容性和包管理体验更稳定。

### 从 Tauri 版升级

旧 Tauri 版用户升级到 3.1.x 时不需要手动导出数据：

- **Windows**：应用内更新会运行迁移器，移除旧 Tauri 安装目录/快捷方式，保留用户数据，然后安装原生版并启动。
- **macOS / Linux**：替换应用或由包管理器升级，数据目录保留。
- **Microsoft Store**：继续由商店更新，不经过 GitHub 更新包。

登录态、云历史、歌单、音量/倍速/均衡/封面模式等会保留。Tauri 只写在 WebView localStorage 里的「推荐/历史 tab、迷你窗位置、本地断点」不会迁移；登录用户的播放进度仍有 B 站云端兜底。更多细节见 [`MIGRATION.md`](MIGRATION.md)。

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

#### 系统托盘不显示

部分桌面环境（如 GNOME）默认不启用托盘图标，需安装扩展：

- **GNOME**：安装 [AppIndicator 扩展](https://extensions.gnome.org/extension/615/appindicator-support-for-gnome-shell/)
- **KDE Plasma**：原生支持，无需额外配置

#### Wayland 环境注意事项

原生包通常无需额外配置。如果在 Wayland 下仍遇到 `EGL_BAD_PARAMETER` 或白屏，可临时禁用 WebKit2GTK 的 dmabuf 渲染器确认是否为驱动兼容问题：

```bash
WEBKIT_DISABLE_DMABUF_RENDERER=1 bili-fm
```

### 应用更新

原生版稳定版使用 mygo 的签名更新通道：

1. 点击标题栏设置按钮 → "检查更新"
2. 从 GitHub Release 的非预览版 `mygo-v*` 中检查新版本
3. 发现新版本后点击"立即更新"，应用内下载、校验签名并重启
4. 从 **3.1.1**（第一个 mygo 正式版）起，稳定版 Release 同时附带 Tauri 迁移包，老 Tauri 用户可通过原有更新入口自动迁移；后续新版本继续由原生版 updater 接手

> Gitee 镜像用于给国内用户提供安装包/迁移包下载；原生版 updater 以 GitHub Release 为主。旧 Tauri 版的更新器会优先检查 Gitee 的 `latest.json`，再回退 GitHub。
>
> Microsoft Store 版只由商店更新，应用内不会下载 GitHub 更新包。Linux 的 deb/rpm/pacman 安装由系统包管理器更新；tar.gz/一键脚本安装的用户可重新运行安装脚本。更多说明见 [`MIGRATION.md`](MIGRATION.md)。

### 开发说明

- 项目使用 **[mygo](https://github.com/egoist/mygo)**（Go 原生 UI 框架）开发，界面由 Go 直接绘制（不依赖 webview）
- 音频是纯 Go 引擎：AAC 解码 + WSOLA 变速不变调 + 均衡 + oto 输出
- 视频播放单独开一个临时 webview 弹窗（H.264 没有可用的纯 Go 解码器）
- 登录态兼容旧版 Wails / Tauri 的 dkv 存储格式，升级无需重新登录
- 项目开源，欢迎提出 Issue、建议或 Pull Request

#### 从源码构建

需要 Go 1.27+（`go.mod` 里的 `tool` 指令会带上 mygo 的 CLI，无需全局安装）：

```sh
cd app
go tool mygo dev     # 开发运行（热重载）
go build ./... && go test ./...   # 编译 + 测试
go tool mygo build   # 出本机安装包到 app/build/
```

原生版开发在 `main` 分支；旧 Tauri 源码备份在 `tauri` 分支。代码结构、计划与已知缺口见 [`app/PLAN.md`](app/PLAN.md)，迁移与 Store 发版见 [`MIGRATION.md`](MIGRATION.md)。

项目地址：[https://github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)

### 免责声明

本项目仅用于开发和学习。项目初衷是方便个人收听 B 站节目，不提供任何内容存储、分发或破解能力。所有视频、音频、弹幕等内容版权归原作者及哔哩哔哩所有。如有侵权，请联系删除。

### 感谢以下项目

- [mygo](https://github.com/egoist/mygo)（原生 UI 框架）
- [oto](https://github.com/ebitengine/oto)（音频输出）
- [go-m4a](https://github.com/tphakala/go-m4a) / [go-aac](https://github.com/tphakala/go-aac)（MP4 解封装与 AAC 解码）
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
- [SponsorBlock](https://github.com/ajayyy/SponsorBlock)（跳过广告功能使用的社区分段数据）

---

## English

![Bili FM Screenshot](screenshot02.png)

Bili FM is a cross-platform desktop application that lets you listen to Bilibili video content as audio. It supports Windows, macOS, and Linux. It works as a lightweight music player and is also great for courses, interviews, podcasts, and long-form video content for background listening.

Current stable release: **3.1.1** (mygo native build). Users on the old Tauri build are migrated automatically; see [Upgrading from Tauri](#upgrading-from-tauri).

### Features

- Listen to Bilibili videos as audio: search, subscriptions, favorites, and recommendations in one place
- Community-powered sponsor segment auto-skip (based on SponsorBlock, opt-in)
- 0.5x–3.0x playback speed, loudness equalizer (EQ), danmaku list
- Creator series and personal playlists managed separately, auto-saved and restored
- Like, coin, and other interactions; global media keys control playback anytime
- Liquid glass UI that adapts to light/dark environments
- Cross-platform: Windows, macOS, and Linux; native stable builds support in-app updates, and the Microsoft Store build updates through the Store

### Loudness Equalizer (EQ)

Different Bilibili videos have vastly different default loudness. With EQ enabled, a dynamic range compressor automatically balances the volume so quiet and loud videos play at more consistent levels. Off by default; click the EQ button in the player bar to enable.

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
| Speed | Player bar speed button |

### Installation

#### Quick Install (macOS / Linux)

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash
```

The script auto-detects your platform and architecture, then downloads and installs the latest stable release. On macOS it installs the `.dmg` matching the release; on Linux it installs a native package through pacman, apt, dnf/yum, or zypper so dependencies, upgrades, and removal remain managed by the system.

**Install pre-release:**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --pre-release
```

**Install a specific version:**

```bash
curl -fsSL https://raw.githubusercontent.com/vst93/bili-fm/main/scripts/install.sh | bash -s -- --version 3.1.1
```

#### Windows

##### Microsoft Store (Recommended)

Bili FM is available on the Microsoft Store. Search for **Bili FM** in the store to install with automatic updates.

[Microsoft Store → Bili FM](https://apps.microsoft.com/detail/9N0LNL3JM3GG)

> Store package updates require Microsoft certification; once the new native MSIX clears review it rolls out automatically. The Store version may temporarily lag behind GitHub.

##### GitHub Release

Prefer a manual installer? Download the `.exe` from the GitHub Release page:

[GitHub Releases](https://github.com/vst93/bili-fm/releases)

> On first install, Windows SmartScreen may show "Unknown publisher" - click "Run anyway".

#### macOS

**Manual install (recommended):** Download the `.dmg` file from [GitHub Releases](https://github.com/vst93/bili-fm/releases), open it and drag bili-FM to the Applications folder.

The Homebrew tap (`vst93/tap/bili-fm`) may still point at the old Tauri build. Confirm it has been updated to the 3.1.x native release before using it; otherwise use the dmg above.

> Apple Silicon (M1/M2/M3/M4): download `macos-apple-silicon`. Intel: download `macos-intel`.

#### Linux

##### Arch Linux

The quick-install command above is recommended. It installs a native pacman package and migrates unmanaged files left by older versions of the script. To install a package downloaded from [GitHub Releases](https://github.com/vst93/bili-fm/releases) manually:

```bash
sudo pacman -U ./bili-FM-3.1.1-linux-x86_64.pkg.tar.zst
```

pacman installs dependencies such as `webkit2gtk-4.1` and tracks all application files. Remove the app with `sudo pacman -R bili-fm`.

##### Ubuntu / Debian

```bash
sudo apt install ./bili-FM-3.1.1-linux-x86_64.deb
```

> Requires Ubuntu 24.04+ (webkit2gtk-4.1). Ubuntu 22.04 only has 4.0, which is incompatible.

##### Fedora / RHEL / openSUSE

The quick installer automatically uses dnf, yum, or zypper. For manual installation:

```bash
sudo dnf install ./bili-FM-3.1.1-linux-x86_64.rpm
```

AppImage is no longer published. Its bundled WebKit runtime can produce blank windows or EGL initialization failures with some Mesa and rolling-release configurations. Native packages use the system WebKit2GTK runtime and provide more reliable package management.

### Upgrading from Tauri

Users on the old Tauri build do not need to export data manually when upgrading to 3.1.x:

- **Windows**: the in-app updater runs a migrator that removes the old Tauri install directory/shortcuts, preserves user data, installs the native build, and launches it.
- **macOS / Linux**: the app or package is replaced, and the data directory is kept.
- **Microsoft Store**: continues to update through the Store and never downloads GitHub update packages.

Login state, cloud history, playlists, volume/speed/EQ/cover preferences are preserved. Items stored only in Tauri's WebView localStorage (recommended/history tabs, mini-window position, local resume points) are not migrated; signed-in users still have Bilibili cloud progress as a fallback. See [`MIGRATION.md`](MIGRATION.md) for details.

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

#### System tray not showing

Some desktop environments (e.g., GNOME) don't enable tray icons by default:

- **GNOME**: Install the [AppIndicator extension](https://extensions.gnome.org/extension/615/appindicator-support-for-gnome-shell/)
- **KDE Plasma**: Supported natively, no extra config needed

#### Wayland Notes

Native packages normally need no extra configuration. If Wayland still produces `EGL_BAD_PARAMETER` or a blank window, temporarily disable WebKit2GTK's dmabuf renderer to confirm a driver compatibility issue:

```bash
WEBKIT_DISABLE_DMABUF_RENDERER=1 bili-fm
```

### Updates

Native stable builds use mygo's signed update channel:

1. Click the settings button in the title bar → "Check for Updates"
2. The app checks non-prerelease `mygo-v*` releases on GitHub
3. When an update is found, click "Update Now" to download, verify the signature, and restart
4. From **3.1.1**, the first stable mygo release, stable releases also contain a Tauri migration package, so old Tauri users can migrate through their existing update UI; later releases are handled by the native updater

> The Gitee mirror provides installers/migration packages for users in China; the native updater primarily uses GitHub Releases. The old Tauri updater checks Gitee's `latest.json` first and falls back to GitHub.
>
> Microsoft Store builds update only through the Store and never download GitHub update packages. Linux deb/rpm/pacman installs are updated by the system package manager; tar.gz/quick-install users can re-run the installer. See [`MIGRATION.md`](MIGRATION.md) for details.

### Development

- Built with **[mygo](https://github.com/egoist/mygo)**, a native Go UI framework — the interface is drawn by Go directly, with no webview
- Audio is a pure-Go engine: AAC decode + WSOLA time-stretch + equalizer + oto output
- Video playback opens a temporary webview popup (there is no usable pure-Go H.264 decoder)
- Login state is compatible with the legacy Wails / Tauri dkv storage format
- Open source — issues, suggestions, and pull requests welcome

#### Building from source

Go 1.27+ is required (the `tool` directive in `go.mod` provides the mygo CLI; no global install needed):

```sh
cd app
go tool mygo dev     # development run (live reload)
go build ./... && go test ./...   # build + test
go tool mygo build   # produce a local installer in app/build/
```

The native build lives on `main`; the legacy Tauri source is kept on the `tauri` branch. See [`app/PLAN.md`](app/PLAN.md) for the code layout and known gaps, and [`MIGRATION.md`](MIGRATION.md) for migration/Store release details.

Repository: [https://github.com/vst93/bili-fm](https://github.com/vst93/bili-fm)

### Disclaimer

This project is for development and learning purposes only. The original goal is to facilitate personal listening to Bilibili content. It does not provide any content storage, distribution, or circumvention capabilities. All video, audio, and danmaku content belongs to the respective creators and Bilibili. If there is any infringement, please contact us for removal.

### Acknowledgements

- [mygo](https://github.com/egoist/mygo) (native UI framework)
- [oto](https://github.com/ebitengine/oto) (audio output)
- [go-m4a](https://github.com/tphakala/go-m4a) / [go-aac](https://github.com/tphakala/go-aac) (MP4 demux and AAC decode)
- [bilibili-API-collect](https://github.com/SocialSisterYi/bilibili-API-collect)
- [SponsorBlock](https://github.com/ajayyy/SponsorBlock) (community sponsor-segment data powering the ad-skip feature)
