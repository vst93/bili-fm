# Bili FM

Bili FM 是一款通过音频收听 B 站视频内容的跨平台桌面应用，支持 Windows、macOS 和 Linux。它可以作为轻量音乐播放器，也适合用于课程、访谈、播客类视频和长视频内容的后台收听。

![Bili FM 截图](https://gitee.com/vst93/bili-fm/raw/main/screenshot01.png)

## 功能特性

- 支持关键词搜索 B 站视频，并可按时间、热度等条件排序。
- 登录后可查看订阅、收藏、推荐等个人内容。
- 支持播放、暂停、上一集、下一集、播放列表等常用播放控制。
- 支持弹幕列表展示，方便在听音频时快速浏览视频互动内容。
- 点击 UP 主名称或头像，可打开 UP 主作品列表。
- 支持点赞、投币等常用互动操作。
- 支持 Windows、macOS 和 Linux 多平台使用。

## 适用场景

B 站电脑端暂未提供完整的“听视频”体验，Bili FM 主要面向以下场景：

- 后台收听知识类、访谈类、课程类和长视频内容。
- 将视频内容作为音频播放，减少画面干扰。
- 快速管理播放列表，连续收听多个视频。
- 在桌面端获得更接近音乐播放器的 B 站收听体验。

## 快捷键

| 快捷键 | 功能 |
| --- | --- |
| 空格 | 暂停 / 开始播放 |
| ← | 上一集 |
| → | 下一集 |

## 安装与更新

### GitHub Release

可前往 GitHub Release 页面下载对应平台的安装包：

https://github.com/vst93/bili-fm/releases

请根据自己的系统选择 Windows、macOS 或 Linux 版本。

### macOS 使用 Homebrew 安装

安装：

```bash
brew install vst93/tap/bili-fm
```

更新：

```bash
brew upgrade bili-fm
```

如果本地 Homebrew 未获取到最新版本，可先更新 Homebrew 索引后再升级：

```bash
brew update
brew upgrade bili-fm
```

## macOS 常见问题

### 提示“应用已损坏，无法打开”

如果你从 GitHub Release 或 Homebrew 安装后，macOS 提示：

> “bili-FM”已损坏，无法打开。你应该将它移到废纸篓。

这是因为应用暂未经过 Apple Developer ID 签名和 notarization，macOS Gatekeeper 会拦截从网络下载的应用。

可以在终端执行以下命令移除 quarantine 标记：

```bash
xattr -dr com.apple.quarantine /Applications/bili-FM.app
```

如果应用仍在下载目录，请将路径改为实际位置，例如：

```bash
xattr -dr com.apple.quarantine ~/Downloads/bili-FM.app
```

执行完成后，再次打开应用即可。

## 开发说明

- 项目使用 Wails 开发，是一个跨平台桌面应用。
- 前端使用 React + HeroUI 构建。
- 项目开源，欢迎提出 Issue、建议或 Pull Request。

项目地址：https://github.com/vst93/bili-fm

## 免责声明

本项目仅用于开发和学习。项目初衷是方便个人收听 B 站节目，不提供任何内容存储、分发或破解能力。所有视频、音频、弹幕等内容版权归原作者及哔哩哔哩所有。如有侵权，请联系删除。

## 感谢以下项目

- https://github.com/wailsapp/wails
- https://github.com/heroui-inc/heroui
- https://github.com/bytedance/iconpark
- https://github.com/SocialSisterYi/bilibili-API-collect
- https://github.com/riyaddecoder/react-audio-play
- https://github.com/tiny-craft/tiny-rdm
