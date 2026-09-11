# AGENT.md — bili-fm 协作约定与项目知识

> 面向 AI 代理（Hermes / pi / Codex 等）的项目内记忆。项目相关的事实、教训、流程都记这里，不要记到代理的个人记忆里。

## 协作分工

- 代码改动一律派 pi（`pi --provider mmz --model deepseek-v4.1-flash --thinking high`），Hermes 只做规格、审 diff、独立复验、commit+push。Hermes 不得直接改代码（2026-09-11 用户令，此前三次自改 CSS 均引发回归）。
- pi 不 commit 不 push；Hermes 审查后统一提交。
- 排障时用户报"按钮没反应"先确认哪个窗口（主窗/mini）、哪个界面（player bar / videoInfo 行）；headless 测试通过 ≠ 用户实机正常，注意平台差异（WKWebView 宽松 vs WebView2 严格 CSP）。
- 用户无法本地跑 Hermes 端验证，验证靠截图 + 代码推导 + headless 测试；验证通过即可 push（用户授权）。

## Git / 发版

- 双远端：origin (GitHub vst93/bili-fm) + gitee。**gitee 不用管版本代码对齐**（用户明确），留给 sync_gitee workflow（手动触发）。
- release workflow（release-tauri.yml，workflow_dispatch）：会 bump 版本号并提交 `release: x.y.z` 到 main——pi 干活期间若发过版，push 前必须先 rebase origin。
- 发版：版本号 `2.0.x` 无 -preview 后缀，预览版只是 release 标记 prerelease=true。Release notes 惯例格式：更新内容明细 + 「下载安装」表格（按平台列文件）+ xattr 提示（macOS `xattr -cr /Applications/bili-FM.app`）+ Gitee 镜像说明。
- 预览版不同步 gitee / homebrew-tap（cask 手动更新，update-formulas.yml workflow_dispatch）。
- gh 后台 shell 可能 401（token 过期）→ `gh auth token` 落盘后 `GH_TOKEN=$(cat ...)`；或用匿名 GitHub API 轮询公开仓库状态。

## 平台坑（已踩实）

- **CSP**：`tauri.conf.json` connect-src 白名单必须含 `https://bsbsb.top`（SponsorBlock API）。Windows WebView2 严格执行 CSP，macOS WKWebView 宽松——macOS 能用 ≠ Windows 能用。
- **bsbsb.top**：免费无 key，`GET /api/skipSegments?videoID=BV..&cid=..`。会话缓存只能存真实服务端结论，abort/超时/网络失败不得写入（StrictMode 双挂载会污染缓存，轮13 教训）。
- **icon-park**：无 FastForward 图标，广告开关用 `Ad`。
- **toggle 类控件**：查激活样式必须按时段（深色块 `html:is(...)` 特异度 1,2,1）/ prefers-contrast / 平台分支逐个过，不只看基态（轮17 教训：EQ 有深色激活规则而 sponsor 漏了，表现为"晚上点了不变蓝"）。
- **CSS 级联**：`.nav-icon-btn > *` 这类通配子选择器会覆盖同特异度的后文规则（轮16：stat value 的 absolute 被覆盖跌回 flex 行内截断成 "1..."）。
- **mini-mode class**：body 的 `mini-mode` 类唯一写权在 isMiniMode effect（state 为唯一事实源），switchWindowMode 及其回滚只改 state。
- **#video-info**：固定 height（当前 366px），新增展示区必须计入高度预算，否则被 overflow:hidden 裁掉。
- **卡片 meta 统一走 CardMeta**（src/components/cardMeta.tsx）：单行 nowrap、字段优先级 作者>播放>时长>发布时间>附加、过窄按优先级整字段隐藏、空值不渲染。新列表必须用它，别手写 meta 行。

## 硬性约束（每份 spec 都带）

- 禁止启动应用、禁止 GUI 自动化（研究靠读代码 + headless）。
- 玻璃红线：不新增 will-change / backdrop-filter。
- 不新增依赖需克制；验证 = `npx tsc --noEmit` + `npm run build` + `node --test tests/*.mjs`（Rust 改动加 `cd src-tauri && cargo check`）。
- 派发模板：`cd ~/workspace/bili-fm && pi --provider mmz --model deepseek-v4.1-flash --thinking high -p "$(cat /tmp/SPEC.md) …" > /tmp/LOG.log 2>&1`，background + notify。
- mmz 中转在 responses 协议下多步工具调用会 400（tool_call_sequence_broken）——`~/.pi/agent/models.json` 的 mmz.api 必须是 `openai-completions`。

## 用户裁决记录

- 永久否决：关窗行为选项、主题跟随系统、歌单导入导出（a1946d9 已删）。
- SponsorBlock 默认关，mini 模式不显示跳过入口（后台跳过仍生效）。
- 更新记录：`.pending-bugs.md`（每轮 commit 后追加一行）。
