# Bili FM

通过音频来听 B 站节目。你可以把它作为一个音乐播放器，也可以把它作为知识学习和长视频收听工具。

![Bili FM 截图](https://gitee.com/vst93/bili-fm/raw/main/screenshot.png)

## 功能

迫于 B 站电脑端没有“听视频”模式，故做了这个小工具方便自己使用。

- 输入关键词在线搜索视频，支持按时间、热度等排序。
- 登录后可查看自己的订阅、收藏和推荐内容。
- 支持播放控制、播放列表、弹幕列表等常用功能。
- 点击 UP 主名称或头像，可以打开 UP 主作品列表。
- 支持点赞、投币等互动操作。

## 快捷键

- 空格：暂停 / 开始播放
- ←：上一集
- →：下一集

## 安装说明

### macOS 提示“应用已损坏，无法打开”

如果你从 GitHub Release 或 Homebrew 安装后，macOS 提示：

> “bili-FM”已损坏，无法打开。你应该将它移到废纸篓。

这是因为应用未经过 Apple Developer ID 签名和 notarization，macOS Gatekeeper 会拦截从网络下载的应用。

可以在终端执行以下命令移除 quarantine 标记：

```bash
xattr -dr com.apple.quarantine /Applications/bili-FM.app
```

如果应用还在下载目录，请将路径改为实际位置，例如：

```bash
xattr -dr com.apple.quarantine ~/Downloads/bili-FM.app
```

然后再次打开应用即可。

## 开发说明

- 项目使用 Wails 开发，是一个跨平台桌面应用。
- 前端已使用 React + HeroUI 重写。
- 项目开源，欢迎提出意见和建议：https://github.com/vst93/bili-fm

## 免责声明

本项目仅用于开发和学习。项目初衷是方便个人收听 B 站节目，如果有侵权，请联系我们删除。

## 感谢以下项目

- https://github.com/wailsapp/wails
- https://github.com/heroui-inc/heroui
- https://github.com/bytedance/iconpark
- https://github.com/SocialSisterYi/bilibili-API-collect
- https://github.com/riyaddecoder/react-audio-play
- https://github.com/tiny-craft/tiny-rdm
