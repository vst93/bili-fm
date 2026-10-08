# mygo 原生重构 · 计划与进度

分支：`mygo`（Tauri 版保留在同一分支，未改动，可随时对照/回退）

## 最终目标（用户指令）

1. **界面样式复刻原版，用原生实现**（mygo `ui` 包，不用 webview）
2. **视频播放走临时 webview 弹窗窗口**（H.264 没有可用的纯 Go 解码器）
3. **整个重构完成后，用 workflow 发一个新预览版**

## 第 3 条的前置问题

现有的 `release-tauri.yml` 是**构建 Tauri 版**的（tauri-action + Rust 工具链），
构建不了 mygo 版。发 mygo 预览版需要新 workflow，见下面的「发版」一节。

## 里程碑

### M1 骨架（进行中）
- [x] 分支 `mygo`
- [x] `internal/bilibili`：B 站 API 客户端，自 Wails 版 `service/bl.go` 移植（2136 行，零外部依赖）
- [x] `internal/store`：本地键值存储，与旧版 dkv 同格式（老用户登录态无缝迁移）
- [x] `internal/view/backdrop.go`：11 个时段的窗口背景渐变（自 globals.css 生成）
- [x] `internal/view/theme.go`：玻璃/文字/面板令牌 + 深色时段判定
- [x] `internal/view/shell.go`：主窗口外壳（背景渐变 + 标题栏 + 卡片列表 + 播放栏）
- [x] 能构建、能启动、截图验证背景渐变与标题栏正确
- [x] 搜索接通真实数据（50 条 / 385ms）并在界面上渲染卡片
- [x] 封面图片：`internal/imagecache` 抓取 + `@240w.webp` 服务端下采样 + LRU 缓存
- [x] 截图验证完整布局（标题栏 / 3 行卡片 / 播放栏）

### 移植时发现的两个 bug（Tauri 版同样存在）
- [ ] **搜索时长角标不显示**：B 站现在把时长放在 `duration` 字段（"32:25"），
      `length` 已不存在。Tauri 版 `bilibili.rs:846` 只读 `length`，所以搜索卡片
      的时长角标一直是空的。Go 版已修（`duration` 优先、`length` 兜底）。
- [ ] **部分封面 URL 拼错**：`pic` 有时是完整 URL（`https://archive.biliimg.com/...`），
      直接拼 `https:` 会得到 `https:https://...`。Tauri 版同样拼错。Go 版已修。

### M2 播放（核心已完成）
- [x] `internal/media/wsola.go`：**WSOLA 变速不变调**，自写。
      实测：440Hz 输入 → 输出 439.995Hz（各速度一致）；长度误差 <8%；
      **吞吐 13.7x 实时**（单核，无 SIMD）。对比 nanowarp 相位声码器 3x 倍速
      要 0.75 个核心且需 cgo —— 这就是选 WSOLA 的理由。
- [x] `internal/media/remote.go`：HTTP Range 读成 io.ReadSeeker（256KB 块缓冲）
- [x] `internal/media/player.go`：解码 → WSOLA → 均衡 → oto 的流水线
- [x] `internal/media/eq.go`：压缩器，参数与旧版 DynamicsCompressor 一致
      （threshold -50 / knee 40 / ratio 12 / release 0.25，带过渡）
- [x] `internal/media/resample.go`：采样率转换（oto 不支持多 context）
- [x] **端到端实测**：真实 B 站视频（1944s）1.0x 与 3.0x + 均衡均正常出声
- [x] `internal/proxy`：图片 / 音频代理（127.0.0.1:4654，Range + Referer）
- [x] `internal/video`：视频弹窗（临时 webview + `PageOptions.Autoplay`）

### M3 功能对齐
- [x] 六个分区：推荐 / 热门 / 动态 / 收藏 / 历史 / 稍后再看
- [x] 卡片网格 + 封面（`@240w.webp` 服务端下采样 + LRU 缓存）
- [x] 播放栏：封面 / 标题 / 上一集 / 播放暂停 / 下一集 / 可拖动进度 / 倍速档位 /
      均衡 / 跳过赞助 / 弹幕 / 分集 / 详情 / 小窗 / 音量
- [x] 分集面板、弹幕面板（点弹幕跳转）、详情面板
- [x] 点赞 / 投币 / 收藏 / 关注
- [x] 评论
- [x] SponsorBlock 自动跳过（含会话缓存，见旧版教训）
- [x] 键盘快捷键（空格 / ←→ / ↑↓ / Esc）
- [x] 迷你模式（400×155 置顶小窗，三平台都可用 —— 原生窗口不受
      webkit2gtk 无法运行时改尺寸的限制）
- [x] 系统托盘 + 关闭到托盘
- [x] 全局媒体键（macOS 上 mygo 的 Carbon 热键没有媒体键码，静默降级）
- [x] 播放队列持久化（`mygo_queue` / `mygo_index`）
- [ ] 系统媒体中心（macOS Now Playing / Windows SMTC / Linux MPRIS）
- [ ] 播放进度上报到 B 站

### M4 发版
- [x] `app/mygo.json`（mygo 构建配置）
- [x] `.github/workflows/release-mygo.yml`：macOS(dmg) / Windows(NSIS) /
      Linux(deb + tar.gz + install.sh)，另用 nfpm 补 rpm、手工 tar 补 pacman
- [x] 本地验证：`go tool mygo build` 四平台全部通过
      （linux deb 5.8MB / windows exe 14.3MB / darwin universal app 27.7MB）
- [x] 发预览版：`mygo-v3.0.0-preview.1`
      https://github.com/vst93/bili-fm/releases/tag/mygo-v3.0.0-preview.1

      | 平台 | 产物 |
      |---|---|
      | macOS | dmg 8.8MB、app.zip 11.2MB（universal） |
      | Windows | setup.exe 4.5/3.8MB、免安装 exe 14.3/13.0MB |
      | Linux | deb 5.2/5.8MB、rpm 5.2/5.7MB、pkg.tar.zst 5.7MB、tar.gz 5.2/5.7MB、install.sh |

      比 Tauri 版（2.8~4.6MB）大，是 Go 运行时的体积代价。

## 还没做的（已知缺口）

- **系统媒体中心**：macOS Now Playing / Windows SMTC / Linux MPRIS 都没接。
  现在系统「正在播放」卡片里看不到曲目信息。
- **播放进度上报**：没有调 `ReportPlayProgress`，所以 B 站网页端的观看进度不会同步。
- **macOS 全局媒体键**：mygo 的 Carbon 热键没有媒体键码，注册会失败（已静默降级）。
- **应用内更新**：mygo 用自己的签名格式（`mygo keygen`），与 Tauri 的 minisign
  不兼容，所以从旧版升级要手动装一次。新 workflow 目前也还没配 `updates`。
- **MS Store(MSIX)**：mygo 不产出，应用商店那条链路要单独做。
- **首页/推荐未登录是空的**：B 站接口如此，原版也一样。
- **评论只有第一页**、**收藏只取第一个收藏夹**。
- **`PageOptions.Autoplay` 依赖 fork**：mygo PR #167 合入后，把 go.mod 里的
  `replace` 删掉即可。

## 发版：需要新 workflow

`release-tauri.yml` 用 tauri-action 构建 Rust 版，mygo 版走不通。需要新增
`.github/workflows/release-mygo.yml`：

| 平台 | mygo `build` 直接产出 | 现有 Tauri 版产出 | 缺口 |
|---|---|---|---|
| macOS | `.app` + `.dmg` | dmg ×2（arm64 / x64） | 无 |
| Windows | NSIS `Setup.exe` | nsis ×2 + MS Store(MSIX) | **MSIX / 应用商店** |
| Linux | `.deb` + `tar.gz` + `install.sh` | deb / rpm / pkg.tar.zst | **rpm、pacman** |

- 需要 Go 1.27+（`GOTOOLCHAIN=go1.27.1` 或 setup-go）
- 需要 `mygo` CLI（`go tool mygo build`）或直接 `go build` + 手工打包
- 现有 workflow 里 pacman 包是手工打的，可以照搬
- **应用内更新不兼容**：mygo 用自己的签名格式（`mygo keygen`），与 Tauri 的
  minisign 不同 → 存量用户**无法应用内升级到 mygo 版**，必须手动重装。
  预览版正好可以先验证这一点。

## 已知的取舍

- **`PageOptions.Autoplay` 还没合入上游**（mygo PR #167）。在它合入前，`go.mod`
  用 `replace` 指向 fork；合入后删掉 replace、改回 `github.com/egoist/mygo`。
- mygo 目前 **v0.3.4**，仓库很新，API 会变。`v0.3.0` 对 `ui` 包是破坏性更新。
- 原生 UI 画不了 `backdrop-filter`：液态玻璃靠「半透明填充 + 1px 亮边 + 阴影」
  近似，窗口材质用 mygo 的 `Vibrancy`（Windows 11 / macOS 支持）。
