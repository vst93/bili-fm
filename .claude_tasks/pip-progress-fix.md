# 任务：修复 macOS 画中画模式无法调整播放进度

## 项目背景

bili-fm 是一个 Tauri v2 + React + TypeScript 桌面应用，代码根目录：`/home/tar/workspace/bili-fm`。
视频播放组件是 `src/components/playerVideo.tsx`（`<video controls>` 标签）。
音频播放组件是 `src/components/player.tsx`（自定义控件）。
两者独立工作：默认音频播放，点击视频图标会启动 `playerVideo` 覆盖在主页 `<section className="home-stage">` 上。

## 问题描述

用户在 macOS 下：
1. 打开视频播放（playerVideo 浮窗）
2. 点击 `<video>` 控件条左上角的画中画（Picture-in-Picture）按钮
3. 视频脱离应用，进入 macOS 画中画小窗
4. **尝试在画中画小窗里拖动进度条/点击进度条 — 没有反应，进度调整无效**

**根因（重要，必须先验证）**：
macOS WebKit（WKWebView / Safari）对 `<video>` 元素画的画中画浮窗，只渲染视频画面，**不挂载用户的控件条**。
具体到本项目，`playerVideo.tsx` 第 177-226 行的 `<video controls>` 在画中画模式下显示的是系统默认控件而不是用户控件，**且 macOS 画中画浮窗是只读预览（read-only snapshot）**，用户交互（点击/拖动进度条）会被 WebKit 丢弃。
这不是项目代码 bug，而是 macOS WebKit 的已知行为。

## 任务目标

**让用户在 macOS 画中画模式下也能调整视频播放进度。**

## 约束

1. **不要破坏现有功能**：
   - 不要改音频播放流程（player.tsx）
   - 不要改播放进度上报逻辑（reportCloudProgress）
   - 不要改 mini-mode（窗口化迷你模式，跟画中画无关）
2. **只动 `src/components/playerVideo.tsx`** 及相关的 CSS 文件
3. 保持现有 CSS 风格（项目用 Tailwind + 自定义 CSS module 风格）
4. 不要引入新的 npm 依赖
5. 完成后必须能 `npm run build` 通过 TypeScript 检查

## 探索步骤（请按顺序执行）

### 第 1 步：完全理解现有代码
- 读完 `src/components/playerVideo.tsx`（237 行）
- 读 `src/pages/index.tsx` 中第 2140-2160 行（playerVideo 的父组件传参）
- 在项目里 grep `requestPictureInPicture` / `document.pictureInPicture` / `enterpictureinpicture` / `leavepictureinpicture` 确认目前**完全没接入**画中画 API
- 读 `src/components/playerVideo.tsx` 的 CSS（`src/css/` 目录或同目录 `.css` 文件）

### 第 2 步：验证根因
搜索 WebKit / macOS 上 `<video controls>` 进入画中画后进度条无效的已知问题。
可以参考：
- WebKit Bug 187178 - "PiP video is read-only"
- Safari TP release notes 中关于 PiP 的限制

确认问题确实是 macOS WebKit 行为而不是代码 bug。

### 第 3 步：设计方案（多个方案对比）
列出 2-3 种可行方案，推荐其中一种：

**方案 A：用户自定义进度条覆盖在 video 上**（推荐）
- 在 `<video>` 上层盖一个自定义 React 控件条
- 当用户点击画中画按钮时，**记录视频 currentTime，调用 `video.requestPictureInPicture()` 进入 PiP**
- 监听 `leavepictureinpicture` 事件，重新显示自定义控件
- 自定义控件条包含：进度条（拖动）、播放/暂停、关闭 PiP、当前时间/总时长
- 进度条拖动 → `video.currentTime = newTime` → PiP 浮窗自动同步

**方案 B：自定义控件 + 画中画浮窗内不可控**
- 自定义控件条只对未进入 PiP 时有效
- PiP 模式下进度条不响应，但其他按钮（关闭 PiP）有效
- 不推荐，因为没解决问题

**方案 C：完全禁用原生 PiP，用 Tauri 窗口模拟**
- 移除 `controls` 属性，自己实现一个迷你悬浮窗口
- 改动太大，不推荐

### 第 4 步：实施方案 A
具体改动：

1. **修改 `playerVideo.tsx`**：
   - 移除 `controls` 属性
   - 添加 `useState` 跟踪 `isInPiP`（是否在画中画模式）
   - 添加 `useEffect` 监听 `enterpictureinpicture` / `leavepictureinpicture` 事件
   - 添加自定义控件条 JSX：进度条、播放/暂停、当前时间/总时长、关闭 PiP 按钮
   - 进度条拖动时调用 `video.currentTime = newTime`
   - 画中画按钮调用 `video.requestPictureInPicture()`（带 `displayMode: 'picture-in-picture'` 兼容性 fallback）
   - 关闭 PiP 按钮调用 `document.exitPictureInPicture()`
   - 画中画模式下隐藏自定义控件条，浮窗本身是只读的

2. **CSS（如果需要）**：
   - 进度条样式跟 `src/components/player.tsx` 里 `.player-timeline` 风格保持一致
   - 控件条布局：底部水平排列，半透明背景

3. **图标**：用 `@icon-park/react`（项目已用），找画中画 / 播放 / 暂停 / 关闭对应图标

### 第 5 步：边界处理
- 检查 `document.pictureInPictureEnabled` 才允许进入 PiP
- PiP 失败时（用户拒绝/不支持）回退到原状态
- PiP 模式下窗口最小化时 PiP 浮窗继续保持
- 关闭 playerVideo 时如果还在 PiP 模式，先 `exitPictureInPicture()` 再 `setIsplay(false)`
- 切换 src 时如果还在 PiP 模式，先 `exitPictureInPicture()`

### 第 6 步：构建验证
- `npm run build` 必须通过
- 如有 lint 错误，修复

### 第 7 步：报告
给我一个简洁的总结（1-3 段）：
- 改动了哪些文件
- 核心改动是什么
- 怎么测试（macOS 下手动测试步骤）
- 任何已知限制

## 重要注意事项

- 这是 Tauri v2 + React + TypeScript，`<video>` 在 WebView 内运行
- macOS WebView 引擎是 WKWebView
- 用户已经在拉取后的最新 main 分支上（`3a62ef3`）
- 不要 commit / push，只改本地代码
- 不要改 Rust 代码（除非 Tauri 那边需要权限，目前 PiP API 是纯 Web API，不需要后端配合）
- 如果发现 Rust 端需要新增权限，**只列出建议**，不要擅自动 Rust

## 完成标志

1. `npm run build` 通过
2. 代码改动在 `src/components/playerVideo.tsx`（主） 和相关 CSS
3. 给了我一段简洁的总结，列出改动文件和测试步骤
