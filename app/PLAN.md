# mygo 原生重构 · 计划与进度

分支：`mygo`（Tauri 版保留在同一分支，未改动，可随时对照/回退）

## 最终目标（用户指令）

1. **界面样式复刻原版，用原生实现**（mygo `ui` 包，不用 webview）
2. **视频播放走临时 webview 弹窗窗口**（H.264 没有可用的纯 Go 解码器）
3. **整个重构完成后，用 workflow 发一个新预览版**

## 第 3 条的前置问题

现有的 `release-tauri.yml` 是**构建 Tauri 版**的（tauri-action + Rust 工具链），
构建不了 mygo 版。发 mygo 预览版需要新 workflow —— 已建好
（`.github/workflows/release-mygo.yml`），见下面的「发版」一节。

## 接手快照（2026-10-09，换机用）

```sh
cd app
go tool mygo dev                      # 开发运行（CLI 由 go.mod 的 tool 指令带，不用全局装）
go build ./... && go test ./...        # 编译 + 跑 internal/media 的测试
go tool mygo build                     # 出本机安装包到 build/（正式构建加 -tags=nethttpomithttp2）
```

- **分支模型**：原生版全在 `mygo` 分支的 `app/` 里（Go + mygo，不用 webview）；
  同一分支的 `src/`、`src-tauri/` 还是 Tauri 版，没动过，可以随时对照/回退。
  `main` 保持 Tauri 版。
- **进度与计划就是这个文件**，代码注释也按「为什么这么做」写。三个参照系：
  `src/`（旧版前端，界面还原的参照）、`src-tauri/src/bilibili.rs`（旧版接口）、
  `main` 分支（当前线上版）。
- **已发预览版**：`mygo-v3.0.0-preview.{1,2,3,4}`（preview.4 起带应用内更新）。
  这批界面还原**还没发**，下一个 tag 是 `mygo-v3.0.0-preview.5`。
- **更新签名私钥不在仓库里**：只在 GitHub secret `MYGO_UPDATER_PRIVATE_KEY`
  （本机 `~/.config/mygo/update-keys/` 已经没有了）。**不要重新 keygen** ——
  换密钥对等于断掉存量用户的更新通道。
- **无头调试开关**：`-shot <png>` 截图、`-load-info` 拉队列首条详情填右栏、
  `-drawer <key>` 直接开某个抽屉、`-no-tray -no-keys` 免托盘/免全局热键。
  禁止 GUI 自动化，验证靠截图 + 代码推导。
- **下一步优先级**见「还没做（下一批）」（界面）和「还没做的（已知缺口）」（功能）。

## 里程碑

### M1 骨架（已完成）
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

### 移植时发现的两个 bug（两个版本都有，都已修）
- [x] **搜索时长角标不显示**：B 站现在把时长放在 `duration` 字段（"32:25"），
      `length` 已不存在。Tauri 版 `bilibili.rs:846` 只读 `length`，所以搜索卡片
      的时长角标一直是空的。Go 版已修（`duration` 优先、`length` 兜底）；
      Tauri 版在 main 上单独修（39168ee）。
- [x] **部分封面 URL 拼错**：`pic` 有时是完整 URL（`https://archive.biliimg.com/...`），
      直接拼 `https:` 会得到 `https:https://...`。同样两边都已修（Tauri 版 39168ee）。

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
      （界面还原时改成旧版的 4 个入口抽屉，见「界面还原」一节的第二批）
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
- [x] 播放进度上报到 B 站（见「界面还原」一节的第三批 / `app/progress.go`）

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

## 界面还原：布局与交互（进行中）

用户反馈：mygo 版的界面和旧版「完全是两个应用」。根因是原生重写只搬了颜色令牌
（theme.go / backdrop.go），布局和组件结构是另起一套（分区药丸行 + 卡片网格 +
两行播放栏），而旧版是「居中搜索药丸 + 封面圆盘 + 视频信息两栏 + 单行播放栏 +
底部抽屉」。下面按旧版的结构重排。

### 已完成

- **窗口**：改回旧版的 800×600（`src-tauri/src/lib.rs` 的 `inner_size`）、固定
  尺寸（`resizable(false)` → `DisableResize`）、居中。窗口控件改用 mygo 的
  `TitleBarHidden`：macOS 是原生红绿灯、Windows/Linux 是原生标题栏按钮 ——
  旧版在 macOS 上就是 `decorations + TitleBarStyle::Overlay`。
- **标题栏**（36px，`.app-title-bar`）：品牌居中（logo 24×24 + 文字），
  左右按 `ui.Context.TitleBar()` 留出原生控件的宽度。
- **搜索药丸**（`.home-searchbar`）：max-width 520 / 高 46 / 圆角 13 / 内边距 4，
  内容是「白底输入内腔（含放大镜提交键）+ 四个内容入口图标 + 头像」。
  分区不再是单独一行药丸，而是回到搜索栏里的四个入口。
- **主区**（`.home-stage` + `.home-now-playing`）：两栏 grid 304px + 1fr、间距 28，
  左栏是 292px 封面圆盘（播放时 22s 转一圈，点一下暂停/播放），右栏是
  「UP 主行 / 标题 22px / 简介 3 行 / 选集胶囊 / 操作行（含计数）/ 工具条」。
- **播放栏**（`#player`）：改成单行 56px，列宽 80 / 56 / 1fr / 56 / 34 / 40 / 34 / 40，
  依次是 切曲 / 当前时间 / 进度 / 时长 / 音量 / 倍速 / 均衡 / 跳过赞助。
  旧版的播放栏里没有封面和标题，那些在主区。
- **抽屉**（HeroUI Drawer）：底部对齐、上圆角 18、高 min(92vh, 100vh-54px)、
  头部 48px，列表是 3 列卡片网格。搜索结果 / 四个分区 / 选集 / 弹幕 / 详情
  都走同一个抽屉组件，同一时刻只开一个。
- **迷你模式**：按 `body.mini-mode` 重排为 24（标题栏）+ 88（封面 64 + 标题 +
  右侧窗口控制）+ 43（播放栏）。补齐入口和出口：标题栏右侧的「切换到迷你模式」
  （旧版 `#switch-window-mode`，Linux 上不显示）、迷你窗里的「还原大窗」和「置顶」
  —— 之前这三处一个都没有（`Act.SetMini` 根本没接线，只有 `-mini` 启动参数能进，
  进去也出不来）。
- **数据**：`VideoInfo` 补上 `stat`（点赞/投币/收藏/播放数），主区右栏才有计数；
  详情（简介 / UP 主头像 / 分集）在起播时一次拉回来，右栏不用等选集抽屉。

### 第二批：抽屉表头 tab、UP 空间 / 合集

参照系：**main 分支**（不是 wails —— wails 的前端少 17849 行，是旧版）。
工作区的 `src/` 与 main 完全一致。

- **分区模型改成 4 个入口抽屉**（旧版就是 4 个，不是 6 个分区）：
  动态 / 热门与推荐（内含 热门·推荐 两个 tab）/ 收藏 / 历史（内含 观看历史·稍后再看）。
- **抽屉表头 tab**（`[data-slot="wrapper"] header [data-slot="tab"]`：高 32、
  内边距 0 14、圆角 8、字号 13/600，选中填充 rgba(255,255,255,0.52) + 文字 #0369a1）：
  - 搜索：综合 / 最多播放 / 最新发布（排序参数真的传给 SearchVideo）
  - 热门与推荐：热门（GetBLPopularList）/ 推荐（GetBLRCMDList）
  - 历史：观看历史 / 稍后再看 + **隐身开关**（原版 `.history-incognito-switch`）
  - 收藏：收藏夹胶囊「名字 (条数)」
  - 每个列表抽屉表头都有刷新键。
- **列表按抽屉各存一份**（`App.Lists`）。之前所有抽屉共用 `a.Cards`，切抽屉会
  互相覆盖、还会闪上一个抽屉的内容 —— 旧版每个抽屉有自己的 state。
- **UP 空间抽屉**（点主区 UP 头像/名字进）：`「名字」的空间` + 粉丝数 + 关注/已关注
  + 视频/合集两个 tab。空间接口返回的是**动态卡片**，视频在
  `modules.module_dynamic.major.archive`（播放量是已经格式化好的字符串），
  所以单写了 `toUpCards`。
- **合集抽屉**：从 UP 空间的「合集」tab 选一个进（胶囊「名字 (条数)」），
  表头有「播放全部」；主区工具条补上了「合集」键（旧版四个键齐了）。
- 顺带修了 `follow()`：原来用的是本地存的 mid —— 那是**登录用户自己**，
  等于在关注自己。改成当前视频的 UP（或 UP 空间里那个）。

### 第三批：对齐 main 的进度同步

`internal/bilibili` 的接口面本来就覆盖了 main 的 Rust（只差 `get_play_progress`），
但**行为**缺了进度同步。按 `src/components/player.tsx` 移植（`app/progress.go`）：

- **云端进度读取**：`x/player/v2` 的 `last_play_time`（先认 `last_play_cid`），
  失败再翻最多 5 页观看历史找 aid+cid 匹配的那条（`GetPlayProgress`）。
- **本地断点**（不依赖账号）：每 5 秒落盘，暂停/切歌/跳转时补写，7 天过期，
  最多 50 条。
- **续播点取 max(云端, 本地)**：本地可能比云端新。云端最多等 800ms
  （`CLOUD_PROGRESS_STARTUP_BUDGET_MS`），超时就用本地断点，晚到的响应丢掉，
  免得把已经在播的曲目跳走。跳转放在**起播之前** —— `media.Player.Seek` 会重建
  解码器，起播后再跳会卡一下。
- **上报**：播放中每 30 秒一次（`PLAY_PROGRESS_REPORT_INTERVAL_MS`），
  暂停/跳转/切歌立刻补报。
- **隐身模式**：读和写都不碰云端，只用本地断点。

### 已完成（2026-10 本机重构轮）

这一轮把界面与交互继续拉齐到旧版（参照 `main` 分支），并修了一批 bug：

- **删除 Tauri 前端**：`src/`、`src-tauri/` 及 Node/Vite/Tailwind/MSIX/aur
  打包配置全部删掉，mygo 分支从此是纯 Go + mygo 原生应用。
- **通知**：新增 toast（原来 28 处 `Status` 文案没有任何显示），自定义成
  旧版的白玻璃卡片（类型色条 + 关闭键）。
- **对话框**：扫码登录（真二维码，修了 `GetLoginQRCodeStatus` 永不返回的
  守卫 bug 与阻塞 sleep）、关于应用、快捷键、更新（走 mygo updater 窗口）。
- **标题栏**：新增「设置」下拉（关于/快捷键/检查更新/退出）。
- **封面**：碟片/封面模式 + 封面背景（氛围光）+ 高级质感三个开关，全部落盘。
- **播放列表**：我的列表 + 合集列表、顺序/单曲循环/随机、添加/全部添加/
  删除/上移下移/清空/定位当前，落盘。
- **选集面板**：三列卡片 + 每集「添加到播放列表」+ 全部添加。
- **弹幕/评论**：合成一个抽屉两个 tab，评论分页 + 楼中楼预览，弹幕自动跟随。
- **互动状态**：起播时异步拉点赞/投币/收藏/关注，修了关注判断拿错 mid 的 bug。
- **SponsorBlock**：状态点、进度条广告段标记、跳过 toast（频控）、
  `FetchSponsorSegments` 区分失败与无分段。
- **交互对齐**：点列表卡片打开选集面板（不直接起播）；搜索框粘 B 站链接
  直接打开；「浏览器打开」接上系统浏览器；迷你窗音量弹层；迷你窗位置记忆。

### 细节对齐轮·第二批（组件+状态机级对照，未用截图）

逐个抽屉组件数 hook（danmakuList 20 个、upVideoList 9 个、historyList 8 个…）
比照内部逻辑，补了这些**行为**差异：

- **弹幕列表**（原版 505 行 / 20 hooks，差距最大）：按秒分组、同秒同文本合并
  `×N`、组内按次数排序、组头「时间 + N 条」、文字用弹幕自身颜色
  （YIQ 亮度换对比色）。抽成 `groupDanmaku`/`danmakuInk` 纯函数 + 单测。
- **无限滚动**：所有列表抽屉都是「距底 80px（评论 100px）自动翻页」，
  不是「加载更多」按钮。用 mygo `ui.Scroll.TrackScroll` 读 Y/MaxY 实现，
  滚动位置同时记在 `List.Scroll`（顺带解决切抽屉回不来的问题）。
- **评论时间倒序**（原版 sortedReplies）。
- **请求代号**：搜索（`SearchRequestID`）与起播（`PlaybackRequestID`）都加了
  代号，慢的旧响应不得覆盖新的 —— 原版 `searchRequestIdRef` /
  `playbackRequestIdRef`。起播在「取地址后」还多校验一次，连点不串台。
- **搜索**：结果上限 160（原版 MAX_RETAINED_LIST_ITEMS）、空关键词清结果关抽屉。
- **稍后再看**：移除按钮请求中置灰防连点（原版 pendingAids）。
- **UP 空间**：刷新同时重查关注状态与粉丝数（原版 handleRefresh）。

### 细节对齐轮（2026-10 第一批，逐区域比照 main:src）

- **操作行**：图标+计数横排（原是竖排）、激活色按动作（点赞红 `#e11d48`、
  投币黄 `#ca8a04`、收藏黄 `#eab308`）、无视频时禁用。
- **dock**：播放列表数量角标（>99 → 99+，合集来源紫色 `#a855f7`）、
  选集非播放列表时蓝色、搜索/合集无数据禁用。
- **标题栏**：品牌 macOS 居中 / Windows/Linux 靠左 + 设置在旁（≤600px 隐藏）。
- **播放栏**：顶边渐隐（`#player::before` 的 10px 渐变）。
- **迷你窗**：整行可拖动 + 封面高光。
- **抽屉表头**：分割线加白高光（玻璃截面两层）。
- **音量**：弹层内独立静音键、播放栏音量键图标随静音变化。
- **卡片 meta**：发布时间三来源（view_at/pubdate/ctime）、播放量缺失回退弹幕数、
  搜索卡补第三列发布时间、1970 占位不显示、标题单行截断。
- **快捷键**：Esc 关视频弹窗/模态框（原版视频模式的屏蔽规则）。
- **倍速档位**：去掉 2.5（原版是 0.5/0.75/1/1.25/1.5/2/3）。
- **其他**：骨架屏、卡片 meta 按宽度隐藏字段、tab 记忆、标题/简介可选中、
  列表有界滑动窗口（上限 160）。
- **测试**：`internal/view` 与 `main` 包新增 Go 单测（`go test -race ./...` 里
  `internal/media` 的吞吐断言会因 -race 变慢而失败，是测试自身的前提不成立，
  不是代码问题；不带 -race 全绿）。
- **系统媒体中心（Linux）**：`internal/mediactl` 用 MPRIS 暴露曲目/状态/位置，
  桌面环境与媒体键可以直接控制；实测 Play/Pause/Next 与元数据都对。
  Volume / Rate / LoopStatus / Shuffle 还是**可写属性**，系统媒体控件可以直接
  调音量、倍速、切循环/随机（introspection 声明 readwrite，gdbus 实测生效）。
  非 Linux 是 no-op。

### 本轮修的 bug（都是实测/审码发现的）

- `GetLoginQRCodeStatus` 用 `LoginStatus` 当守卫，而 `LoginStatus` 只在登录
  成功后为真 —— 轮询永远返回 false（二维码扫了也登不上）；另去掉了阻塞 sleep。
- 视频弹窗的 IPC 服务名错了：mygo 按**类型名**绑定，`video.Service` 绑成
  `Service`，而页面调 `Video.Log/Report/Close/Ended` → 全部静默失败（弹窗状态
  回不来、关不掉、播完不续）。改 `BindAs("Video", …)`。
- 从选集抽屉点分集播不动：`playIndex` 只认列表卡片，而选集不在卡片列表里；
  现在以 `a.Info` 的分集为准建队列。
- 互动状态（点赞/投币/收藏/关注）从未被加载；且关注判断用的是**自己**的 mid
  （等于关注自己）。
- 普通队列只有一条（单集视频）时「下一集」会自我重播 → 现在无下一首就停/
  禁用按钮。
- 动态（feed）列表用 `toCards` 解析，而接口返的是动态卡片 → 登录下也是空的；
  改用 `toUpCards`，且翻页改用 offset 游标（之前用页码会重复追加第一页）。
- 收藏夹「加载更多」永远为真 → 不足一页时停止。
- 本地存储写入非原子（并发/崩溃可能读到半个文件）→ 临时文件 + 改名，读加锁。
- WSOLA / 均衡器与播放器回调的**数据竞争**（oto 取数、上报、界面三条 goroutine）；
  `-race` 下全链路无竞争。
- 「浏览器打开」是空实现；URL 链接不会直开；迷你音量键点了没反应。
- 视频弹窗页面错误处理里先取不存在的 `#err` 元素会抛异常。

### 框架能力限制：唱片旋转做不了（2026-10 定论）

原版 `#video-cover.record-disc` 是 22s 一圈的 CSS 动画（转整个位图）。
mygo 的 `Rotate` **只对矢量图标生效**：`paint.go` 里只有 `kindIcon` 分支把
rotate 传给 `drawIcon`，`kindImage`（位图）走 `p.image()` 完全不带旋转；
`drawBitmap` 也没有角度参数。SVG 渲染器（`internal/svg`）又不支持
`<image>` 元素，所以「位图包进 SVG 再转」也走不通。

试过的替代方案都卡在同一处：
- 高光弧 Icon 叠在封面 Button 里 → Button 的 `Clip()`（圆形裁剪必需，
  否则方形封面）会把旋转后的边界框裁掉，弧线看不见；
- Icon 提到 ring 层用 Absolute 叠放 → Absolute 的参照与层叠不可控，
  且 `Loop()` 驱动的动画元素偶发不绘制。

**结论**：封面圆盘保持静止，`spinDisc`/`iconDiscSheen` 已回退。
等 mygo 支持位图旋转（或给出位图元素的变换 API）再做。

### 还没做（下一批）

- **玻璃材质**：mygo 有 `plugins/glass`（真 Liquid Glass），现在还是「半透明
  填充 + 1px 亮边 + `glass.Blur` 氛围光」的近似。抽屉/药丸可以再换真的。
- **合作视频 badge / 选择合作 UP 主**：仅详情页有判据（staff 非空），未做。
- **播放缓冲指示**：`media` 是按需 Range 读，没有现成的 buffered 区间。

### 调试开关（为了截图对照）

`-shot` 截图、`-load-info` 拉当前队列第一条的详情填右栏、`-drawer <key>` 直接
打开某个抽屉、`-modal about|shortcuts|login`、`-toast <文本>`、`-play <n>`、
`-addall`、`-video`、`-playlist`（预填播放列表）、`-danmaku N`（注入假弹幕）、
`-window-pos X,Y`、`-series <id>`。都只在开发时用。

注意：视频弹窗的页面是 `mygo build` 时才内嵌的；普通 `go build` 出的二进制
没有前端资源，弹窗会加载不出来（只有原生 UI 能跑）。要测弹窗用
`go tool mygo build` 或 `go tool mygo dev`。

## 还没做的（已知缺口）

- **系统媒体中心**：**Linux 已接**（`internal/mediactl`，MPRIS：桌面「正在播放」卡片、
  媒体键、锁屏控件，用 `godbus/dbus`）。macOS Now Playing / Windows SMTC 还是
  no-op，接口已留好（`Callbacks`/`Controller`），接上层不用改。
  **为什么没直接写**：两块都是几百行平台 FFI（WinRT COM / ObjC runtime），
  这台机器上既编译不了也跑不了，盲写上去出问题没法定位 —— 要做需要能起
  Windows/macOS 环境的人配合验证；状态帧等纯逻辑已抽出来可以单测
  （`smtcState.same`）。
- **macOS 全局媒体键**：mygo 的 Carbon 热键没有媒体键码，注册会失败（已静默降级）。
- ~~应用内更新~~：**已接**（3.0.0-preview.4）。见下面的「应用内更新」一节。
- ~~评论只有第一页~~：**已修**（弹幕/评论抽屉支持加载更多）。
- **MS Store(MSIX)**：mygo 不产出，应用商店那条链路要单独做。
- **首页/推荐未登录是空的**：B 站接口如此，原版也一样。
- **`PageOptions.Autoplay` 依赖 fork**：mygo PR #167 合入后，把 go.mod 里的
  `replace` 删掉即可。

## 发版：`release-mygo.yml`（已建好）

`release-tauri.yml` 用 tauri-action 构建 Rust 版，mygo 版走不通，所以另建了
`.github/workflows/release-mygo.yml`（推 `mygo-v*` tag 或
`gh workflow run release-mygo.yml --ref mygo -f version=...`；**必须带 `--ref mygo`**）。
各平台产出对比：

| 平台 | mygo `build` 直接产出 | 现有 Tauri 版产出 | 缺口 |
|---|---|---|---|
| macOS | `.app` + `.dmg` | dmg ×2（arm64 / x64） | 无 |
| Windows | NSIS `Setup.exe` | nsis ×2 + MS Store(MSIX) | **MSIX / 应用商店** |
| Linux | `.deb` + `tar.gz` + `install.sh` | deb / rpm / pkg.tar.zst | **rpm、pacman** |

- 需要 Go 1.27+（`GOTOOLCHAIN=go1.27.1` 或 setup-go）
- 需要 `mygo` CLI（`go tool mygo build`）或直接 `go build` + 手工打包
- rpm / pacman 是 workflow 里补的（nfpm + 手工 tar），deb 由 mygo 原生产出
- **应用内更新不兼容**：mygo 用自己的签名格式（`mygo keygen`），与 Tauri 的
  minisign 不同 → 存量用户**无法应用内升级到 mygo 版**，必须手动重装。
  预览版正好可以先验证这一点。

## 已知的取舍

- **`PageOptions.Autoplay` 还没合入上游**（mygo PR #167）。在它合入前，`go.mod`
  用 `replace` 指向 fork；合入后删掉 replace、改回 `github.com/egoist/mygo`。
- mygo 目前 **v0.3.4**，仓库很新，API 会变。`v0.3.0` 对 `ui` 包是破坏性更新。
- 原生 UI 画不了 `backdrop-filter`：液态玻璃靠「半透明填充 + 1px 亮边 + 阴影」
  近似，窗口材质用 mygo 的 `Vibrancy`（Windows 11 / macOS 支持）。


## 安装包体积分析（3.0.0-preview.2 实测）

### 与 Tauri 版对比

| 平台 | Tauri 2.0.37 | mygo 3.0.0-preview.2 | 倍数 |
|---|---|---|---|
| macOS Apple Silicon dmg | 4.05 MB | **4.01 MB** | **0.99x** |
| macOS Intel dmg | 4.30 MB | 4.83 MB | 1.12x |
| Windows x86_64 setup | 3.07 MB | 4.36 MB | 1.42x |
| Windows ARM64 setup | 2.81 MB | 3.66 MB | 1.30x |
| Linux x86_64 deb | 4.61 MB | 5.55 MB | 1.20x |
| Arch pkg.tar.zst | 3.62 MB | 5.43 MB | 1.50x |

### 已经做的优化

1. **macOS 按架构分开出包**（最大的一笔）。原来出 universal 单包 8.77MB，
   里面塞了两个架构的二进制；改成 amd64 / arm64 两个 dmg 后各 4.01 / 4.83MB，
   用户只下自己架构那个。Tauri 版也是分开出的。
2. **`-tags=nethttpomithttp2`**：去掉 net/http 的 HTTP/2 实现。B 站 API 与
   CDN 用 HTTP/1.1 完全够（旧版 Tauri 的 reqwest 也特意 `ForceAttemptHTTP2=false`）。
   每个安装包省约 0.2MB。
3. `-s -w` 已经是 mygo build 的默认行为。

### 剩下的差距来自 Go 本身，不建议再压

按包拆解未 strip 的二进制（21MB → strip 后 14MB）：

| 大小 | 内容 | 能不能省 |
|---|---|---|
| 8.13 MB | `go:func.*` 函数元数据（GC 栈扫描/反射/panic 恢复用） | **不能**。`-s -w` 只删符号表，删不掉元数据 |
| 4.32 MB | mygo 框架（UI 工具包 + 文字排版 + 几百个 purego FFI 绑定） | 不能，都是用到的 |
| 2.59 MB | `crypto`（TLS） | 不能，HTTPS 必需 |
| 1.12 MB | `net` | 不能 |
| 0.61 MB | 我们自己的代码 | — |

**UPX 能做到 2MB 级安装包，但不建议**：UPX 是恶意软件常用的壳，Windows
Defender 等会误报，而且启动要解压。对一个已经上架应用商店的产品不划算。

**结论**：1.2~1.5x 是这个技术栈的合理区间；macOS 已经和 Tauri 持平。


## Linux 包格式验证（3.0.0-preview.2 实测）

mygo 原生只出 **deb**（`cmd/mygo/deb.go` 是唯一的 Linux 打包器），rpm 和
pacman 是 workflow 里自己补的。三种格式逐个验证过：

| 格式 | 来源 | 验证方式 | 结果 |
|---|---|---|---|
| `.deb` | mygo 原生 | `dpkg-deb -I` / `-c` | ✅ 依赖 `libgtk-3-0 \| libgtk-3-0t64, libwebkit2gtk-4.1-0`，装到 `/opt/bili-fm` |
| `.rpm` | nfpm | `bsdtar -tvf` + 手工解析 header | ✅ magic/版本/架构/header 结构合法，文件清单正确 |
| `.pkg.tar.zst` | 手工 tar | `pacman -Qip` / `-Qlp` | **❌→✅ 一开始是无效的**，见下 |
| `.tar.gz` + `install.sh` | mygo 原生 | 假 HOME 沙箱实装 | ✅ 装到 `~/.local/bili-fm.app`，命令与桌面项都建好 |

### 踩到的坑：pacman 版本号不能含 `-`

第一次生成的包 pacman 直接拒绝：

```
错误：软件包 bili-fm-3.0.0-preview.2-1 的元数据无效（软件包版本包含无效字符）
```

pacman 的版本格式是 `pkgver-pkgrel`，**pkgver 里不允许 `-`**；Arch 也没有
Debian / rpm 那种 `~` 预发布约定。改用 `.`（`3.0.0.preview.2-1`）后正常。
顺带补了 `.MTREE`（pacman 的文件完整性清单，缺了会警告）。

顺带记录：deb 用 `~`（mygo 自己处理的），rpm 用 `~`（nfpm 自动把 `-preview`
规范成 `~preview`），pacman 用 `.` —— 三家的预发布约定各不相同。


## 应用内更新（3.0.0-preview.4 起可用）

### 密钥

- **算法**：Ed25519（`mygo keygen` 生成）。公钥 `ziZNrW3/2vAI+e+geB6RTlQ/dYvBgEiqdWAbu0cwJjY=`，
  写在 `mygo.json` 的 `updates.publicKey`，**可以公开**（它本来就编译进应用）。
- **私钥**：只在本机 `~/.config/mygo/update-keys/mygo-update.key`（权限 600）
  和 CI secret `MYGO_UPDATER_PRIVATE_KEY` 里。**绝不能进仓库**（已核对全历史无泄漏）。
- **丢了会怎样**：存量用户再也收不到更新（应用只认这把公钥签的包）。

为什么不用 SSH 密钥：`~/.ssh/id_rsa` 是 **RSA 3072**，两个更新器都只支持
Ed25519，算法都不同；而且 SSH 私钥是身份凭证（能推代码、登服务器），把它放进
构建环境风险太大，轮换 SSH 密钥还会直接断掉更新通道。

为什么不用 Tauri 那把：都是 Ed25519，技术上能转换，但清单格式不同（Tauri 是
`latest.json`，mygo 是 `update-<target>.json`），复用省不了任何事，反而一旦
转换出错两条通道一起坏。

### 更新源

`updates.github: vst93/bili-fm` + `tagPrefix: mygo-v`。tagPrefix 是必须的 ——
仓库里还有 Tauri 版的 release（tag 形如 `2.0.37`，而且是最新 release），
不加前缀 mygo 会去找错 release。

### 哪些安装方式能自更新

| 安装方式 | 装到哪 | 自更新 |
|---|---|---|
| macOS dmg | `/Applications` | ✅ |
| Windows NSIS setup | `%LOCALAPPDATA%\Programs` | ✅ |
| Linux tar.gz + install.sh | `~/.local/bili-fm.app` | ✅ |
| Linux deb / rpm / pacman | `/opt/bili-fm` | ❌ 由包管理器负责 |

不能自更新时 `Updater.Enabled()` 返回 false，用户点「检查更新」会看到原因。

### 验证

发布流程里加了自动校验：逐个读 `update-*.json` 的 url，确认对应文件真的在
release 里，不在就直接让 workflow 失败。

加这道校验是因为踩过一次：第一版把 Linux 归档改名、又漏传了 darwin/windows
的归档，结果 **6 个清单全部指向不存在的文件**，应用内更新会在所有平台 404，
而且只有用户点了更新才会发现。

## 给框架补了「位图旋转」能力（2026-10）

唱片 22s 一圈的旋转在 mygo 里本来做不了（`Rotate` 只对矢量图标生效）。
没有放弃，直接给框架打了补丁，fork 在 **vst93/mygo** 的
`feat/bitmap-rotate` 分支（commit 396651b4bd43）：

- `scene.Op` 新增 `Rotation`（OpImage 顺时针角度，绕 Rect 中心）；
- CPU 渲染器 `raster.image` 实现了旋转：遍历**旋转后包围盒**、
  逐像素做逆变换取源像素，但**裁剪仍用原始 Rect 的 Radii**——
  旋转的封面依然是圆，不会变成方形卡片；
- `Element.Rotate` 同时设置 Icon 与 Image 的角度，`Element.Loop`
  照旧驱动进度（暂停即停帧，不会跳）。

应用侧 `go.mod` 的 replace 指到这个 fork 的 pseudo-version。
**GPU 渲染器暂时忽略 Rotation**（本机正好是 CPU 渲染，已实机验证
旋转生效）；等 GPU 侧补上后，CPU/GPU 表现要一致（框架的既有约定）。

给框架提的注意点：`Rotation` 只在 CPU renderer 生效这件事必须写进
`scene.Op` 的注释里（已写），不然 GPU 侧不知道要补。
