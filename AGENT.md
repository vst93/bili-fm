# AGENT.md — bili-fm 协作约定与项目知识

> 面向 AI 代理（Hermes / pi / Codex 等）的项目内记忆。项目相关的事实、教训、流程都记这里，不要记到代理的个人记忆里。

## 项目现状（2026-10 起：纯 mygo 原生版）

- **依赖 mygo v0.7.3**（`app/go.mod`）。每次升级都写一份「逐 PR 能不能用上」的
  评估表到 `app/PLAN.md`（v0.4.6 / v0.4.7 / v0.6.3 / v0.7.0 / v0.7.1 /
  v0.7.2+v0.7.3 各一节），当轮的坑与门槛写进 `.pending-bugs.md`。
  **先读这两处再动依赖。**

- **更新源必须用 `mygo.Updater.SetSource`**（`app/update_source.go`）。0.7.2 起
  mygo 的 updater 走 `netproxy.Transport()`，不再碰 `http.DefaultTransport`：
  往那里塞自定义 RoundTripper 既拦不到请求，还会让 netproxy 的类型断言 panic。

- **本仓库 `main` 分支已经是纯 Go + mygo 原生应用**，Tauri / React / WebView 前端
  已整体删除（`src/`、`src-tauri/`、Node/Vite/Tailwind 配置、`aur/`、MSIX 打包等）。
- 应用代码全在 **`app/`**（Go module `github.com/vst93/bili-fm/app`）：
  - `app/main.go`：应用组装、事件、调试开关
  - `app/internal/bilibili`：B 站 API 客户端（自旧 Wails/Tauri 版移植，零外部依赖）
  - `app/internal/media`：纯 Go 音频引擎（WSOLA 变速不变调 + 均衡 + oto 输出）
  - `app/internal/view`：原生 UI（mygo `ui` 包）：shell / home / playerbar / drawer / mini
  - `app/internal/proxy`、`app/internal/imagecache`、`app/internal/store`、`app/internal/video`
- **`tauri` 分支保留旧 Tauri 版**，只作历史参照 / 紧急维护（`tauri:src/` 是旧前端，
  `tauri:src-tauri/src/bilibili.rs` 是旧接口）。需要对照时用
  `git show tauri:src/...`；不要把旧代码合回 `main`。
- **进度与计划在 `app/PLAN.md`**，接手先读它。

## 协作分工

- 代码改动一律派 pi（`pi --provider mmz --model deepseek-v4.1-flash --thinking high`），Hermes 只做规格、审 diff、独立复验、commit+push。Hermes 不得直接改代码（2026-09-11 用户令，此前三次自改 CSS 均引发回归）。
- pi 不 commit 不 push；Hermes 审查后统一提交。
- 排障时用户报"按钮没反应"先确认哪个窗口（主窗/mini）、哪个界面（player bar / videoInfo 行）；headless 测试通过 ≠ 用户实机正常，注意平台差异。
- 用户无法本地跑 Hermes 端验证，验证靠截图 + 代码推导；验证通过即可 push（用户授权）。

## Git / 发版

- 双远端：origin (GitHub vst93/bili-fm) + gitee。**gitee 不用管版本代码对齐**（用户明确），留给 sync_gitee workflow（手动触发）。
- 原生版发版走 `release-mygo.yml`（推 `mygo-v*` tag 或 `gh workflow run release-mygo.yml -f version=...`）；稳定版会自动附带 Tauri 迁移包，见 `MIGRATION.md`。
- Microsoft Store 包走 `release-store.yml`（只构建；Partner Center 手动提交）。
- 更新签名私钥只在 GitHub secret `MYGO_UPDATER_PRIVATE_KEY`；公钥在 `app/mygo.json`。**不要重新 keygen**。
- Release notes 惯例：更新内容明细 +「下载安装」表格（按平台列文件）+ xattr 提示 + Gitee 镜像说明。

## 平台坑（已踩实）

- **发布产物命名**：mygo 0.7.0 起磁盘镜像与 Windows 安装器**按 `slug` 命名**
  （`bili-fm-setup-<ver>-<arch>.exe`），0.6.x 是 `bili-FM Setup <ver> <arch>.exe`。
  `release-mygo.yml` 收集时要两代都匹配，且**找不到就 exit 1** —— 静默漏传会
  连带 store / Tauri 迁移 job 硬失败。可执行文件仍按 `name`（`bili-FM.exe`）。
  `mygo.json` 里的 `slug` 已显式钉成 `bili-fm`，别删。
- **Linux Wayland 无边框窗**：0.7.0 起框架给 Frameless 主窗加 GTK CSD
  （投影 + 描边，mygo #727）。透明窗（桌面字幕）、panel、X11 不受影响。

- **CSP 已不存在**（原生版没有 webview 安全策略），旧 Tauri 的 CSP/WebView2 坑只作历史参考。
- **SponsorBlock（bsbsb.top）**：免费无 key，`GET /api/skipSegments?videoID=BV..&cid=..`。会话缓存只能存真实服务端结论，abort/超时/网络失败不得写入。
- **mini-mode**：state 是唯一事实源，`Act.SetMini` 是唯一写入口；布局/命中测试都要区分主窗与 mini。
- 原生 UI 画不了 `backdrop-filter`：玻璃用「半透明填充 + 1px 亮边 + 阴影」近似，窗口材质用 mygo `Vibrancy`。

## 硬性约束（每份 spec 都带）

- 禁止启动应用、禁止 GUI 自动化（研究靠读代码 + 无头 `-shot` 截图）。
- 不新增依赖需克制；验证 = 仓库根的 **`./check.sh`**（vet + 全包测试 +
  linux/windows/darwin × amd64/arm64 六平台编译，与 CI 门禁同口径；`./check.sh fast`
  只跑 vet+test）。改动 `internal/view` 另用 `-shot` 截图对照。

## 用户裁决记录

- 永久否决：关窗行为选项、主题跟随系统、歌单导入导出。
- SponsorBlock 默认关，mini 模式不显示跳过入口（后台跳过仍生效）。
- 更新记录：`.pending-bugs.md`（每轮 commit 后追加一行）。

## 进度条对齐（2026-10-15）

- 原版 `tauri:src/styles/globals.css`（3067-3277 区间）/ mygo `app/internal/view/rangeslider.go` + `app/internal/view/playerbar.go` 进度条区域。
- 修复 3 项差异：① 禁用态 opacity 从 0.5 对齐到 0.42（`disabledDim` 抵消框架减半）；② 滑块 scale 从瞬时改为 100ms ease 过渡（`AnimateTo`）；③ 滑块高光 alpha 亮色 0.86→0.98、深色 0.78→0.90。
- 已知未对齐：Seek bubble 在触摸设备（`@media (hover:none), (pointer:coarse)`）原版 `display:none`，mygo 无法检测指针类型。
- volume thumb 圆心 1px 偏移（原版 `margin-top:-5px`）在 mygo 中已修正为居中，不回退。
