# Tauri → mygo 迁移说明（3.1.0 起；当前 3.1.1）

## 版本与渠道

| 渠道 | 版本 | 更新方式 |
|---|---|---|
| GitHub / Gitee | `mygo-v3.1.1`（非 prerelease） | mygo updater（Ed25519） |
| 老 Tauri 2.x | 自动迁移到 `3.1.0` | 旧 Tauri updater 下载双清单里的迁移包 |
| Microsoft Store | `3.1.1.0` MSIX | Store 自动更新 |

- GitHub Release：https://github.com/vst93/bili-fm/releases/tag/mygo-v3.1.1
- Store 当前在架：`2.0.37.0`；新包保持同一 Identity / Publisher / Store ID。
- Store ID：`9N0LNL3JM3GG`
- Store Identity：`vvv.bilifm`
- Store Publisher：`CN=21D233E1-6650-4700-A43E-EDA723C43F96`

## 老 Tauri 自动迁移

`release-mygo.yml` 的 `bridge` job 只对非 prerelease 运行：

1. 下载当前 stable release 的 mygo 资产；
2. macOS：`.app.tar.gz`；
3. Windows：`scripts/bridge/tauri-migrate.nsi` 编译自定义 NSIS 迁移器，
   嵌入 mygo NSIS setup；
4. Linux：复用 mygo deb/rpm；
5. 全部用旧的 `TAURI_SIGNING_PRIVATE_KEY` 签名；
6. 生成 `latest.json`（GitHub URL）和 `latest-gitee.json`（Gitee URL）。

Windows 迁移器：
- 只删除旧 Tauri 安装目录 `%LOCALAPPDATA%\bili-FM`、卸载项和快捷方式；
- **不删除** `%APPDATA%\bili-fm`、`%APPDATA%\com.bili-fm.app`、`%LOCALAPPDATA%\com.bili-fm.app`；
- 静默安装 mygo 到 `%LOCALAPPDATA%\Programs\bili-FM`，启动新程序。

数据迁移由 `app/internal/store/migrate.go` 完成：登录态、歌单、音量/倍速/均衡/封面模式等；
Tauri 仅存在 WebView localStorage 的 tab/迷你窗位置/本地断点不迁移（云端进度兜底）。

## Gitee

桥接 job 同时上传 `latest-gitee.json`。`sync_gitee.py` 会把它改名成 Gitee release 的
`latest.json`，里面的 URL 指向 Gitee 资产。手动同步：

```sh
gh workflow run sync_gitee.yml --ref mygo
```

## Microsoft Store

构建（不提交）：

```sh
gh workflow run release-store.yml --ref mygo -f version=3.1.1 -f submit=false
```

提交到 Partner Center：

```sh
gh workflow run release-store.yml --ref mygo -f version=3.1.1 -f submit=true
```

前置 secrets：`AZURE_TENANT_ID` / `AZURE_CLIENT_ID` / `AZURE_CLIENT_SECRET`。

Store 版：
- 检测 exe 路径包含 `WindowsApps`；
- 后台自动更新跳过；
- 用户点「检查更新」显示「Microsoft Store 版本会随商店自动更新」；
- 不触碰 GitHub 更新包（商店政策）。

Store 与 GitHub 是两个独立安装渠道，不建议同时安装。

## 发版检查清单

- [ ] `./check.sh` 全绿
- [ ] stable release 非 prerelease，`latest.json` / `latest-gitee.json` 已上传
- [ ] Tauri 签名可用旧公钥验证
- [ ] Gitee latest release 有 `latest.json` 且 URL 指向 Gitee
- [ ] Windows 旧 Tauri 2.0.37 → 3.1.1 实机迁移，检查登录/歌单/历史、只有一套安装
- [ ] macOS Tauri → 3.1.1，检查 Gatekeeper / 数据
- [ ] Linux deb/rpm Tauri → 3.1.1，检查 `/usr/bin/bili-fm` 和桌面项
- [ ] Store 2.0.37 → 3.1.1.0 内部 flight，检查数据和应用内更新提示
