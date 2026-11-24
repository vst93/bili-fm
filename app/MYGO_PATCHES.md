# mygo 补丁维护

唯一业务补丁分支：[vst93/mygo:bili-fm-patches](https://github.com/vst93/mygo/tree/bili-fm-patches)。
`app/go.mod` 固定该分支提交对应的 Go 伪版本；不发布 `v*` 标签，避免触发框架的
完整 release、npm 和网站发布流程。旧版本标签保留，保证历史应用版本可复现。
伪版本名称里的 `avf.8` 来自历史祖先标签，并不表示依赖旧的 AVF 分支；实际固定的
是 `bili-fm-patches` 提交 `4a657d5be469`。

## 2026-11-24 上游核对

已将 fork / upstream 的 `main`（两者相同）
`49fb992c3ae2de5b9484b5089bba945a884abc35` 合入补丁分支，保留已发布历史，
没有 force push。上游自 v0.10.0 以来的升级评估见 [PLAN.md](PLAN.md)。

| 旧分支 | 判断与处理 | 核对依据 |
| --- | --- | --- |
| `avf-fast-rate` | 已收进补丁分支，删除旧分支 | macOS >2 倍速压缩时间轴、异步 retime、位置与 seek 容差修复仍需要 |
| `fix/windows-seek-buffering` | 已收进补丁分支，删除旧分支 | Windows 成功传入新帧后退出 Buffering；共享 UI 同帧重读状态 |
| `fix/mac-fast-start` | 已收进补丁分支，删除旧分支 | 元数据延迟时保留倍速、seek 与播放意图，加载后自动继续 |
| `feat/autoplay-policy` | 上游已覆盖，删除旧分支 | 上游 `03ac0702` / #856 明确基于 vst93 的两个补丁；Windows 参数改为环境选项，三平台均有实现与测试 |
| `feat/bitmap-rotate` | 上游通用变换已替代，删除旧分支 | 上游 `a28ff74b` / #110 的 `ui/svg.go:Rotate` 对非 Icon 使用 `Rotation` 变换，CPU / GPU 共用；不恢复旧的 CPU 专用采样。其父提交中的 autoplay 同上 |
| `docs/protocol-media` | 上游文档已覆盖，删除旧分支 | 上游 `d054a1f0` / #161；`docs/frontend.md` 与 `protocol.go:FileServer` 说明 Linux 自定义 scheme 媒体限制及 loopback HTTP 方案。保留上游当前的平台描述 |
| `fix/webview2-env-release` | 旧修法不再需要，删除旧分支 | 当前 `internal/windows/window.go:cleanup` 调用 controller.Close 并 release；`826384a5` / #1111 管理隐藏 Cookie WebView 的空闲关闭与会话保留，旧补丁只按页面窗口判断会遗漏这些使用者 |

WebView2 的判断还参考了微软 [ICoreWebView2Controller.Close 文档](https://learn.microsoft.com/en-us/microsoft-edge/webview2/reference/win32/icorewebview2controller#close)：
最后一个 WebView 关闭后浏览器实例退出。文档示例甚至保留 environment，等待
`BrowserProcessExited` 后再释放它，故 environment 的 COM 引用本身不是阻止
浏览器退出的依据。不再照搬旧补丁“释放 environment 才退出浏览器”的假设。
隐藏 Cookie WebView 在含会话 Cookie / InPrivate 数据时有意保留，不能误删。
旧补丁的 [PR #168 关闭说明](https://github.com/egoist/mygo/pull/168)
也记录了 vst93 当时的实测：关闭最后一个页面后 WebView2 进程数为 0，内存从
506 MB 回到 69 MB，`b.env` 仍持有。该 PR 因原先假设已被实测推翻而主动关闭。

上游在 `49fb992c` 中的 `ui/video/avf_darwin.go`、`ui/video/mf_windows.go`、
`ui/engine_video.go` 与 v0.10.0 基点逐字节相同，视频修复尚未被覆盖。
补丁分支相对最新 `main` 只多这三个运行时文件和四个测试文件。

### 体积核对（Linux amd64，release 参数）

| 示例 | 更新前补丁分支 | 最新 main + 补丁 | 最新 main |
| --- | ---: | ---: | ---: |
| counter-native | 10,223,328 B | 10,297,376 B | 10,297,344 B |
| hello | 9,854,752 B | 9,887,616 B | 9,887,616 B |

本次增量来自上游更新。counter-native 的额外 32 B 来自 VCS / 版本信息的
长度与对齐：用 `-buildvcs=false` 复测，补丁版和上游均为 10,297,184 B。
`-dumpdep` 也确认未使用视频的 counter-native 没有链接 `videoEngine.advance`
及原生解码器；没有因补丁将 opt-in 解码功能带入普通应用。

### 验证边界

- 框架根包、`ui/video`、`internal/raster`、`internal/scene` 及共享视频状态
  恢复测试通过。`ui` 完整测试中仅 `TestPainterRichTextKeepsSpans` 失败：40 个
  RichText 稳态分配 43 次，测试门槛为 2。干净的 upstream `49fb992c` 单独运行
  同一测试也得到 43 次；已确认是上游现有问题，不新增业务补丁掩盖。
- 本机已交叉编译 macOS amd64 / arm64 的原生视频测试。Mac 实际帧输出由
  GitHub Actions 两种 Mac runner 验证，Linux 本机编译不能代替原生执行。
- 上游 #1999 将 `AccessAll().Bounds` 改为裁剪后的几何，原矮窗菜单测试在
  h=340 时把只露出 8px 的条目误认为完整可见。已改成“可见高度等于自然行高
  34px + 完全位于面板内 + 点击回调实际触发”。两个相关测试通过；在隔离副本
  去掉 `ui.Scroll` 后测试按预期失败，保留了原来的滚动回归保护。
- 音频转视频测试保留 2.5–3.5 倍速与至少 5 个画面的断言；改为先观察画面实际
  更新再测稳态，并增加三点 RGB 采样和失败时的完整状态，避免只凭粘性的
  `Ready()` / 前进的时钟判断新时间轴已有画面。旧采样点经无头探针确认没有
  被控件条完全挡住，因此不能直接将历史 ARM 失败归因于遮挡或标为 flaky。

fork 另有与上游同名同提交的历史功能分支，以及 `benchmarks` / `website`
自动化分支。它们不是 bili-fm 的额外补丁线，不混入业务补丁或批量删除。
`website` 当前指向旧 AVF 提交是过去版本标签触发网站工作流的结果；其中全部
视频改动已在补丁分支内。日后只维护 `bili-fm-patches`，不再使用网站分支放补丁。

## 后续更新

1. fetch 并核对 upstream/main，与 fork/main 对齐。
2. 将 main 正常 merge 到 `bili-fm-patches`，避免改写已发布提交。
3. 按实际行为核对每项补丁是否已被上游覆盖；覆盖后在补丁分支提交撤除。
   squash 合并后提交哈希不同，不能只用 `git cherry` / 祖先关系判断。
4. 跑应用完整门禁、平台原生视频测试，更新应用的伪版本和本表。
5. 不再创建按平台拆分的长期补丁分支。
