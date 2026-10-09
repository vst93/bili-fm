# AGENT.md — bili-fm 协作约定与项目知识

> 面向 AI 代理（Hermes / pi / Codex 等）的项目内记忆。项目相关的事实、教训、流程都记这里，不要记到代理的个人记忆里。

## 项目现状（2026-10 起：纯 mygo 原生版）

- **本仓库（`mygo` 分支）已经是纯 Go + mygo 原生应用**，Tauri / React / WebView 前端
  已整体删除（`src/`、`src-tauri/`、Node/Vite/Tailwind 配置、`aur/`、MSIX 打包等）。
- 应用代码全在 **`app/`**（Go module `github.com/vst93/bili-fm/app`）：
  - `app/main.go`：应用组装、事件、调试开关
  - `app/internal/bilibili`：B 站 API 客户端（自旧 Wails/Tauri 版移植，零外部依赖）
  - `app/internal/media`：纯 Go 音频引擎（WSOLA 变速不变调 + 均衡 + oto 输出）
  - `app/internal/view`：原生 UI（mygo `ui` 包）：shell / home / playerbar / drawer / mini
  - `app/internal/proxy`、`app/internal/imagecache`、`app/internal/store`、`app/internal/video`
- **`main` 分支保留 Tauri 版**，是「功能对齐 / 界面还原」的唯一参照系（`main:src/` 是旧前端，
  `main:src-tauri/src/bilibili.rs` 是旧接口）。需要对照时用
  `git show main:src/...` 或另开 worktree，不要把旧代码合回本分支。
- **进度与计划在 `app/PLAN.md`**，接手先读它。

## 协作分工

- 代码改动一律派 pi（`pi --provider mmz --model deepseek-v4.1-flash --thinking high`），Hermes 只做规格、审 diff、独立复验、commit+push。Hermes 不得直接改代码（2026-09-11 用户令，此前三次自改 CSS 均引发回归）。
- pi 不 commit 不 push；Hermes 审查后统一提交。
- 排障时用户报"按钮没反应"先确认哪个窗口（主窗/mini）、哪个界面（player bar / videoInfo 行）；headless 测试通过 ≠ 用户实机正常，注意平台差异。
- 用户无法本地跑 Hermes 端验证，验证靠截图 + 代码推导；验证通过即可 push（用户授权）。

## Git / 发版

- 双远端：origin (GitHub vst93/bili-fm) + gitee。**gitee 不用管版本代码对齐**（用户明确），留给 sync_gitee workflow（手动触发）。
- mygo 版发版走 `release-mygo.yml`（推 `mygo-v*` tag 或 `gh workflow run release-mygo.yml --ref mygo -f version=...`，**必须带 `--ref mygo`**）。
- 更新签名私钥只在 GitHub secret `MYGO_UPDATER_PRIVATE_KEY`；公钥在 `app/mygo.json`。**不要重新 keygen**。
- Release notes 惯例：更新内容明细 +「下载安装」表格（按平台列文件）+ xattr 提示 + Gitee 镜像说明。

## 平台坑（已踩实）

- **CSP 已不存在**（原生版没有 webview 安全策略），旧 Tauri 的 CSP/WebView2 坑只作历史参考。
- **SponsorBlock（bsbsb.top）**：免费无 key，`GET /api/skipSegments?videoID=BV..&cid=..`。会话缓存只能存真实服务端结论，abort/超时/网络失败不得写入。
- **mini-mode**：state 是唯一事实源，`Act.SetMini` 是唯一写入口；布局/命中测试都要区分主窗与 mini。
- 原生 UI 画不了 `backdrop-filter`：玻璃用「半透明填充 + 1px 亮边 + 阴影」近似，窗口材质用 mygo `Vibrancy`。

## 硬性约束（每份 spec 都带）

- 禁止启动应用、禁止 GUI 自动化（研究靠读代码 + 无头 `-shot` 截图）。
- 不新增依赖需克制；验证 = `cd app && go build ./... && go test ./...`，改动 `internal/view` 用 `-shot` 截图对照。

## 用户裁决记录

- 永久否决：关窗行为选项、主题跟随系统、歌单导入导出。
- SponsorBlock 默认关，mini 模式不显示跳过入口（后台跳过仍生效）。
- 更新记录：`.pending-bugs.md`（每轮 commit 后追加一行）。
