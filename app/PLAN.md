# mygo 原生重构 · 计划与进度

分支：`main` = mygo 原生版；`tauri` = 旧 Tauri 版备份（可随时对照/回退）

## 最终目标（用户指令）

1. **界面样式复刻原版，用原生实现**（mygo `ui` 包，不用 webview）
2. **视频播放走临时 webview 弹窗窗口**（H.264 没有可用的纯 Go 解码器）
3. **整个重构完成后，用 workflow 发一个新预览版**

## 3.1.3 功能迭代（2026-10-21）

在 3.1.2 稳定版基础上新增 6 项功能，版本号不跨大版本，发版只把 GitHub Release
标成 prerelease（版本号 / tag 本身不带 `preview`）：

- 章节导航：复用 `x/player/v2` 的 `view_points`，章节页 + 进度条分界。
- 字幕文稿：复用 `subtitle.subtitles`，按需下载 BCC，字幕页跟随/点击跳转。
- 桌面字幕：字幕正文就绪后创建无边框置顶小窗，不抢焦点、不依赖 webview。
- 顺着听：以当前视频为种子拉 `archive/related`，队列尾部自动补相关推荐。
- 弹幕热力：按时间桶聚合弹幕，进度条热力线 + 右键跳到下一个高能。
- Windows SMTC：窗口创建后 AttachWindow，补齐 Windows 系统媒体中心
  （标题/封面/播放状态/媒体键/时间轴），macOS Now Playing / Linux MPRIS 保持原实现。

验证：`./check.sh`（vet + 全包测试 + 六平台编译）全绿。

## 当前观看人数「N 人在看」（2026-10-29）

右栏 UP 主行行尾新增一块实时在线人数，数据源是 B 站给网页播放器用的
`x/player/online/total`（`count` = 此刻正在看**这一集**的人数，`total` = 已格式化
的累计观看文本，`show_switch` = B 站自己的展示开关）。与已有的 `View`
（累计播放量，随详情一次性拿到）是两个不同的数，所以要**周期刷新**。

- 接口实测：未登录可取、几十字节；同一视频 3s 内 12 → 152 → 13（真·实时）。
  **不用 `x/player/v2` 的 `online_count`**：那是播放器配置快照字段，未登录时
  实测一部百万播放的视频也恒为 1，拿来显示就是永远「1 人在看」。
- 刷新：起播 / 换集立刻打一枪（`startCurrent` 的更新回合里），之后 30s 轮询
  （`app/viewers.go`）；主窗不可见（收托盘 / 迷你 / 最小化）时不打接口。
- 陈旧策略：换集先清空（绝不把上一集的数挂到新视频上，界面侧还有一道
  `OnlineCid` 归属校验）；单轮失败保留旧数，连着 3 轮（≈90s）失败才退回不显示；
  B 站关掉 `show_switch.count` 或 `count ≤ 0` 时整块不画（不写误导的「0 人在看」）。
- 展示：**纯文字，不加任何装饰**（无底色 / 描边 / 投影 / 圆点），12px / 500 字重 /
  `--studio-faint`。先前那版胶囊 + 玫瑰红圆点在 366 宽的右栏里过于扎眼，已全部
  撤掉。文案「N 人在看」，与 B 站网页播放器那个气泡同一口径（中途试过
  「同时在线 N 人」，用户最终仍要「在看」）。
- 位置：**选集胶囊（`.video-part-pill`）里、分集标题之后**。那颗胶囊写的就是
  「这一集在播什么」（来源 + 分集标题），在线人数是同一件事的另一面，读作
  「● 当前选集 P1 · 152 人在看」。它有自己的边界、文字在内部随内容排布，不像
  「贴到某一行行尾」那样孤零零地飘在右缘（用户试了五轮：UP 主行行尾 / kicker
  后面 / 操作行行尾都不满意）。
- 退路：单 P 视频没有分集标题 → 胶囊整块省略（`partPillShown`）→ 在线人数退到
  操作行行尾，不能因为这个数没地方挂就消失。
- 布局：右栏是固定 366 高的盒，新加一行会把下方整体顶下去，所以挂在 UP 主行
  **行尾**（行内唯一常年空着的横向空间）。为此给名字那层加了 `Shrink(1)+MinWidth(0)`：
  窄窗 + 长 UP 名时是名字先让位（实测 400 宽下不加就是名字撞进胶囊 40px），
  没有胶囊时名字仍是原版 240。
- 测试：`internal/view/viewers_test.go`（无头渲染：有数/无数、归属校验、像素可读性、
  不顶动下方布局、不与名字重叠）+ `viewers_test.go`（刷新决策的纯函数）+
  `internal/bilibili/online_test.go`（真实响应样本与容错解析）。
- 门槛：`./check.sh` 全绿。

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
- [x] 全局媒体键（macOS 走系统 Now Playing 派发，见下）
- [x] ~~播放队列持久化（`mygo_queue` / `mygo_index`）~~（用户要求去掉启动恢复，已整体移除落盘/恢复）
- [x] 系统媒体中心（macOS Now Playing + Linux MPRIS；Windows SMTC 待做）
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
  左右按 `ui.Context.TitleBar()` 留出原生控件的宽度。**hover-to-reveal**：静息时
  整条退到背景里（子元素 .32、品牌 .70、顶沿投影 .52、无白玻璃），鼠标落到标题栏上
  或里面的控件拿到焦点才亮起（.82 / 1 / .78 / 1）并浮出一层白玻璃（见
  `titleBarReveal` + `titleBarLayer`）。
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

参照系：**tauri 分支**（旧 Tauri 版；不是 wails —— wails 的前端少 17849 行，是更旧版本）。
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
  打包配置全部删掉，main 分支从此是纯 Go + mygo 原生应用。
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

### 细节对齐轮·第三批（实测反馈驱动）

- **进度条拖动**：松手后连发多次 seek（Slider 每帧写回值、Changed 每帧触发，
  而状态机在 Changed 里清 seekSent）→ 拖动中不再动 seekSent，松手对
  最终位置发一次 seek；拖动中零网络请求。
- **seek 后时间归零**：pipeline.seek 重建 WSOLA 而 SourcePosition 从
  WSOLA 累计——NewWSOLA 增加 startSample，seek 把目标位置传进去。
- **播放栏整体点不动**：广告段标记的 Absolute 定位容器把整个播放栏的
  命中吃了 → 删掉（标记暂缺，宁可没视觉也不能点不动）。
- **视频弹窗唯一性**：Manager.Open 加 opening 标志，快速连点不再多开。
- **Buffering 期间 seek**：对齐原版 safeSeek 的 readyState 保护——
  记 PendingSeek，起播完成后执行（排队不丢弃）。
- **seek 后时间接续（引擎层）**：配合 WSOLA startSample，
  pipeline.seek 用 SeekSample 一次定位（见下）。
- **go-m4a：Reader.SeekSample/SampleCount**（fork vst93/go-m4a v0.5.1，
  上游 PR tphakala/go-m4a#69）：seek 从「逐帧 ReadFrame（每帧一个
  HTTP Range 请求，一小时 ≈ 15 万个）」改为 O(chunks) 定位。
- **三键计数格式**：对齐 formatCompactCount（≥1亿 x.x亿 / ≥100万 整数 w /
  ≥1万 x.x w）；列表播放量对齐 formatViewCount（≥100万 整数万）。
- **图标重绘**：PlayOne/Pause/GoEnd/VolumeNotice/Mute/Comment/DoubleUp/
  ThumbsUp/HandleB/Star/VideoTwo 等，对齐 icon-park outline 风格。
- **相对时间阶梯**：对齐 dateFormLadder——今天/昨天/N天前/MM-DD(当年)/
  yyyy-MM-DD(跨年)/N个月前/N年前，单测覆盖。
- **状态圆点**：播放列表模式玫红（--studio-rose），主区胶囊 + mini 窗
  （含外圈光晕）。
- **收藏夹表头**：左右滚动键（collect-scroll-btn，每次 200px）。

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

### 细节对齐轮（2026-10 第一批，逐区域比照 tauri:src）

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

### 浮窗 UI/UX 细节轮（2026-10-28）

用户要求「优化当前应用 UI / UX 细节，注意这是桌面浮窗应用」。这一轮开始**不再逐像素
复刻原版**：凡是原版在 800×600 浮窗里明显不成立的地方（占位文案、被截断的数字、
窗口位置不记忆），按浮窗场景重做，并写清理由，后面不要按原版「修回去」。

- **空态不再把占位当内容画**：没有详情也没有当前曲目时，右栏只留一句标题
  「还没有正在听的内容」+ 一句可执行引导；不再画「神秘的UP主 / 暂无播放内容 /
  无描述 / 当前选集 无选集标题」以及一排全禁用的计数键。工具条保留（播放列表
  往往已经有歌，是空态里唯一能点的入口）。宽窗空态不再锁 366 的固定盒高
  （没有内容可排，固定盒高会把三行字顶在盒顶）。见 `home.go` 的 `emptyInfo` /
  `idle()`。
- **迷你窗空态**：第二行不再显示「无选集标题」，改成「播放列表 N 首」/「回到主窗
  开始搜索」，圆点用中性色，不冒充来源色。见 `mini.go`。
- **单 P 视频不再画「当前选集 无选集标题」**：`partTitle()` 没有分集就返回空串，
  胶囊只在有分集标题或从播放列表播入时出现。
- **搜索卡片两处数据缺陷**：
  - 发布时间全变 1970-01-01 —— `searchResultCards` 把展示文案塞进了只认 Unix 秒的
    `pubdate`，`pickInt` 的 `Sscanf("%d")` 把年份当秒。新增 `metaDateUnix` 供数值
    使用，展示仍走 `metaDateText`。
  - 播放量被截成「6652…」—— 接口给的是「665.2万」，解析回 6652000 再直出，窄列
    放不下。`toCards` 新增 `views_text`（接口文案直出）。
- **搜索抽屉表头带关键词**：抽屉会盖住主窗搜索药丸，标题从「搜索」改成
  `搜索「关键词」`（长词 10 字截断）。
- **主窗位置记忆**：此前只有 `miniWindowPosition`，主窗每次启动、每次退出迷你模式
  都居中。新增 `mainWindowPosition`，启动时恢复、退出迷你时回到上次位置
  （`restoreMainWindow` 一次 `SetBounds`，无记录才居中）；位置轮询主窗 / 迷你窗
  共用，隐藏与最小化时停（避免把 Windows 最小化的 -32000 当成用户选择存下来）。
- **关于对话框**文案压到一行，消除「工具。」孤行。
- 验证：`go test .` 全绿；`internal/view` 全量测试与改动前逐条对比无新增失败
  （24 条既有失败与本机字体/标题栏渲染相关，改动前同样失败）；`-shot` 截图核对
  空态、迷你空态、搜索结果（日期 / 播放量 / 表头）、关于对话框。
- 未验证：多显示器下主窗位置恢复的真实落点（本机无屏幕采集权限）、迷你↔主窗
  切换的实机过渡。

### 还没做（下一批）

- **玻璃材质**：已用上 `plugins/glass`（真 Liquid Glass）——搜索药丸、播放栏、
  弹窗、toast、迷你窗都是 `glass.Glass{}`；只有抽屉面板为性能降级成半透明填充
  （原版那层 `backdrop-filter` 在软件光栅下每帧重算，见「材质策略」一节）。
- ~~**合作视频 badge / 选择合作 UP 主**~~：**已完成**（`collabPicker`）：详情页
  `staff > 1` 时在 UP 名字旁画 `Peoples` 图标键，菜单列出全部参与者（主 UP +
  联合投稿者），选一个就把面板显示的 UP 主换成 TA 并打开 TA 的空间 —— 对齐原版
  `videoInfo` 的 `displayedOwner`。
- ~~**播放缓冲指示**~~：**已完成**（`Player.Buffered()` + 进度条缓冲层）。

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
`gh workflow run release-mygo.yml -f version=...`）。
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

唱片 22s 一圈的旋转在 mygo 里本来做不了（`Rotate` 只对矢量图标生效：
`paint.go` 只在 `kindIcon` 分支把角度传给渲染器，位图那条分支收不到）。
给框架打了补丁：`scene.Op` 新增 `Rotation`，CPU 渲染器的 `raster.image`
遍历旋转后包围盒、逐像素做逆变换取源像素（裁剪仍用原始 Rect 的 Radii，
所以旋转的封面依然是圆的），`Element.Rotate` 同时设置 Icon 与 Image 的角度。

**已作为 PR egoist/mygo#201 提交上游，等合并。**
本地 fork 已放弃（框架发版很频繁，维护 fork 的成本大于收益），
`go.mod` 的 `replace` 指回官方版本；等上游合并后直接用官方能力，
在那之前唱片保持静止。GPU 渲染器暂时忽略 `Rotation`（本机是 CPU 渲染）。

## 进度条这一块踩过的坑（2026-10，重要）

底部进度条前后回归过四五次，根因全是**对框架 API 的理解错了**，
不是逻辑写错。记在这里免得再犯：

1. **`Element.Dragging()` 不是指针拖动。** 它是 HTML5 拖放
   （`rt.drag`，只有调用 `ui.Drag()` 建立拖放源时才非空）的接口，
   对 Slider **永远返回 false**。用它判断「用户是否在拖进度条」=
   点击和拖动全都不触发。
2. **`Element.Pressed()` 不能用于「是否按住」。** 它要求指针在元素
   矩形内（`Rect{s.vx,s.vy,s.vw,s.vh}.Contains(pointerX,pointerY)`），
   而进度条轨道只有 6px 高——拖动时指针稍一偏离就返回 false，
   会被误判成松手（表现为「拖到一半失效/跳回」）。
   **正确信号是 `Element.Dragged()` 的 `ok`**：它就是 `st.pressed`，
   与元素矩形无关。
3. **框架的输入处理在「每帧构建之后」跑**（`runtime.go` 里
   `rt.view()` 之后才 `rt.applyInputs()`）。所以应用侧在构建阶段写
   绑定的 `*value` 是安全的（会在 `applyInputs` 里被指针位置覆盖）；
   但**不能用旧值去覆盖它**——松手当帧必须先读值发 seek、再做跟随，
   顺序反了就是「松手跳回原地」。
4. **「跟随播放位置」和「用户拖动」争同一个 `*value`**：用
   `SeekBusy`（seek 在跑）+ 一个 250ms 的交接窗口把两者错开，
   否则每 500ms 的进度上报会把滑块拽回去。

现在这段逻辑抽成了纯函数 `progressPlan`（`internal/view/state.go`），
四条不变量由 `TestProgressPlan` 的 7 个子测试钉住。

## 进度条视觉：自绘而不是用现成控件

原版这条轨道是纯 CSS 的：6px 高、左右内缩 7px（= 滑块半径）、1px 白边、
已播部分 90° 青 `#2dd4bf` → 蓝 `#0284c7` 渐变、滑块 14 圆 + 白边 + 蓝投影。
框架默认的 `Slider` 只有一条素色轨道，达不到这个质感，所以改用
`SliderBase`（只提供取值，不带外观）+ `Element.Draw` 自绘轨道/滑块/填充。

`SliderBase` 与 `Slider` 共用同一个 `sliderInput`，输入行为完全不变
（`Dragged()` / `Changed()` 照旧）。`PaddingX(7)` 让取值区间两端各缩进
一个滑块半径，滑块圆心才能走到轨道的两个端点（原生 range 的行为）。

## 端到端验证进度条（Xvfb + xdotool）

进度条的交互光看代码判断不出来，`./check-e2e.sh` 会在 Xvfb 里真跑一遍：
用 xdotool 按下拖动再松手，然后检查日志里有没有出现 seek 的痕迹
（`flushProgress` 会打「上报播放进度失败」或「跳转失败」）。
需要 `Xvfb`、`xdotool`、`imagemagick`。**只用于本地排查，CI 里不跑。**

## 一个**有意为之**的平台差异：Linux 上的玻璃质感

原版在 Linux 上把玻璃材质整体降级成近乎不透明的白：

```css
body.platform-linux #root {
    --glass-bg: rgba(255, 255, 255, 0.92);
    --glass-blur: none;
    --liquid-bg: rgba(255, 255, 255, 0.9);
    --liquid-blur: none;
}
```

注释写明了原因：Linux 的窗口管理器渲染不了外层圆角，而 webkit2gtk 上做
backdrop-filter 代价太高（原版在别处也写了「去掉全窗 blur 滤镜（GPU 最大
单项）」）。

**mygo 没有这个限制**——它的 CPU/GPU 渲染器能正常做背景模糊，本机在
Xvfb 里跑出来的截图里玻璃层是真模糊的。所以这里**保留玻璃质感**，
不回退到 Linux 那份不透明白：那份是给 webkit2gtk 打的补丁，不是设计意图，
照搬只会让界面比原版在该平台上的**意图外观**更差。

（macOS / Windows 上原版用的就是半透明 + 模糊，与我们的取值一致。）

## 原版 CSS 里「后面的规则覆盖前面的」要注意

`#video-desc` 在 globals.css 里出现了两次（717 行和 2326 行），
后者生效：`#9ca3af` → `var(--studio-muted)`、`margin-top: 6px` → `14px`、
加 3 行截断。对照尺寸时**必须往后看有没有覆盖规则**，否则会照抄到已经
失效的旧值。对照抽屉表头、卡片等其它样式时同理。

## 材质策略：用框架的 Liquid Glass，不复刻原版手搓玻璃（2026-10）

原版那套玻璃是「半透明白底 + 1px 白边 + inset 亮线/暗线 +
`backdrop-filter: blur()`」，本质是在 CSS 里手搓玻璃。mygo 有真正的
Liquid Glass（`plugins/glass`）：边缘折射、1px 描光聚光、可选的按下生长，
在 GPU 与 CPU 上由同一套着色器画出来，各平台一致。

所以**材质这一层不再照抄原版**，改用：

| 场景 | 用什么 |
|---|---|
| 结构面（播放栏、mini 播放栏、抽屉、模态框、Toast、弹层、搜索药丸、卡片、tab） | `glass.Glass{}` |
| 有「激活」色的小控件（EQ/恰饭开、tab 选中、卡片在播） | `glass.Glass{Tint: …}` |
| 主操作（播放键） | `glass.Glass{Interactive: true}` —— 按下时玻璃会长大，这是原版手搓 inset 想表达而做不到的 |
| 压在封面/照片上的小键 | `glass.Glass{Style: glass.Clear, Tint: 深色}` —— Clear 几乎不染色，只做轻微折射 |
| 常态无框的小图标键（内容操作行、dock、顶部分区） | **常态透明**，只在悬停/激活时铺玻璃 |

最后一条是实测调出来的：原版 `.nav-icon-btn` 常态是「透明 + 一点径向高光」，
给每颗都铺一块玻璃会显得"每个图标都套了个盒子"。判断依据就是
**原版有没有常驻底色/描边**：有（播放栏按钮、抽屉表头按钮、tab）就常驻玻璃，
没有（内容操作行、dock、顶部分区）就只在悬停/激活时铺。

另外玻璃面上**不要再叠手搓的 1px 边框与 inset 亮/暗线**——玻璃自带描光，
两层描边会显脏。

### 踩坑：`fill` 只有一个，Material 与 Background / LinearGradient 互相覆盖

`Element.Material` 设 `fill = fillMaterial`；`Background` 只在
`fill == fillGradient` 时才改回 `fillColor`；`LinearGradient` 直接设
`fill = fillGradient`（**会把 Material 整个顶掉**）。后果：

- `Material(glass.Glass{}).Background(白 .82)` 里的底色是**空操作**（被静默忽略）；
- 之前 Toast 写成 `Material(...).LinearGradient(...)`，等于一直没玻璃。

所以「一层颜色 + 一层玻璃」必须拆成两个元素（外层玻璃 + 内层
`PassThrough` 的颜色叠加），不能靠链式调用堆在同一层。

### 性能

实测应用是**按需重绘**（20 秒里不到 60 帧，多数是图片到位时触发的），
空闲时 0% CPU。所以玻璃的着色器成本可以忽略。若以后加入常驻动画
（例如封面旋转），需要重新测。

## 「锁里做 I/O」这一类卡顿（2026-10，修了两处 22 秒）

界面卡顿不一定在界面代码里。实测抓到两处**22.4 秒**的冻结，根因是同一个
模式：**某个锁在持有期间做了网络 I/O，而 UI 线程上的操作要拿同一把锁**。

1. `Player.Play` 握着 `p.mu` 调 `newPipeline()`，而它里面
   `m4a.NewReader(rf)` 要走 HTTP 解析 MP4 头。UI 上的 SetSpeed/SetVolume
   全卡住。→ 改成「锁外建流水线、锁内只做交换」，建好后用**当前值**重新
   套一遍 speed/eqOn（建的那几秒里用户的改动不能被快照吃掉）。
2. `pipeline.setEQ` 要拿流水线的锁，而 `Read`（oto 取数据的 goroutine）
   会在 `io.ReadFull` 网络读时长期持有它。→ `Compressor` 自己就有锁，
   那层流水线锁纯属多余，删掉。

同一模式还顺手修了 `reportLoop`：它握着 `p.mu` 调 `cur.state()`
（要拿流水线锁），每 500ms 就可能把 UI 挡一次。

**教训**：排查界面卡顿先看有没有「锁 + I/O」，而不是先怀疑渲染。

### 怎么查这类问题

`prefs.go` 里的 `dbgSlow(name, fn)` 会给一段操作打点，超过 30ms 才打印。
播放栏的所有操作都套了它。配合 `xdotool` 脚本依次点击各控件，日志里
出现 `[slow]` 就直接指到具体哪一步 —— 比盯着代码猜快得多。

## 音频流水线的设计（2026-10 重整）

这一块出过几次问题（倍速卡死、EQ 卡 22 秒、均衡把声音压没），根因都是
**局部补丁**式的改法。这里把整套设计写清楚，以后按这套判断。

### 数据流

```
HTTP Range 流
  └─ go-m4a 解封装（m4a.Reader，支持 SeekSample 直接定位到帧）
      └─ AAC 解码（aacpcm → s16le 交错）
          └─ WSOLA 变速不变调
              └─ 压缩器（音量均衡）
                  └─ 重采样（仅当流的采样率 ≠ 输出上下文采样率）
                      └─ s16le 字节 → oto 声卡输出
```

### 锁的模型（三把，各管一块）

| 锁 | 保护 | 谁在用 |
|---|---|---|
| `Player.mu` | `cur`/`op`/`speed`/`eqOn`/`vol` 这些**指针与标量** | 所有 Player 方法 |
| `pipeline.mu` | 解码器、输入输出缓冲、seek 重建 | `Read`（oto goroutine）、`seek` |
| `pipeline.wsMu` | **只有 `ws` 这一个指针** | `position()`、`setWS()` |
| `Compressor.mu` / `WSOLA.mu` | 各自的内部状态 | 各自的方法 |

**铁律：不许握着任何一把锁做网络 I/O。**

- `Play` 在锁外建流水线（`m4a.NewReader` 要发 HTTP），进锁只做交换；
  建好后用**当前值**重新套一遍 speed/eqOn（建的那几秒里用户的改动不能被
  快照吃掉）。
- `Read`/`seek` 里的网络读必须在 `pipeline.mu` 内（要与重建互斥），
  所以**任何会被 UI 线程或上报循环调用的读写都不能用这把锁**——
  `setEQ` 因此走 `Compressor.mu`，`position()` 因此走 `wsMu`。
- 违反这条的后果不是"慢一点"：实测 UI 卡过 22 秒。

### WSOLA 的游标：必须增量推进

合成端的标称分析游标 `pos` **只按 `hop × 当帧速度` 增量前进**，不写成
`frame × step`：

- `frame × step` 里 `step` 含 speed，改一次倍速会把**过去所有帧**也按新
  速度重算 —— 播 1 分钟后切 2x，游标一次性向前跳 1 分钟的源音频，输入
  永远跟不上，`produce()` 再也产不出东西 → **音频卡死、进度乱跳**。
- 增量推进时，改速度只影响**之后**每帧前进的距离，游标连续。
- 原来的写法是为了防漂移（不能用 `posPrev + step` 递推，因为互相关每帧
  能挪 ±search）。现在两者兼顾：**游标走标称值，互相关只决定"实际取哪
  一段"，不进游标**。`posPrev` 保留下来只作相似度模板。

改速度的契约：**任何时刻改速度都必须连续、不许停顿、不许让源位置回退**。
`TestWSOLASpeedChangeMidStream` / `TestWSOLAManySpeedChanges` 钉住这条
（含 0.5x/0.75x/2x/3x、以及连点档位）。

### 音量均衡：压缩 + **补偿增益**

原版用 Web Audio 的 `DynamicsCompressorNode`（threshold -50 / knee 40 /
ratio 12 / attack 0 / release 0.25）。它自带**自动补偿增益**（让 0 dBFS
输入仍是 0 dBFS）—— 只抄压缩曲线不抄补偿，就是"一开均衡所有声音变小"。
起音必须是瞬时的（原版 attack: 0）：用平滑包络会在峰值处停在"较低电平
对应的高增益"上，把满幅内容抬过 1.0 造成削波。

**2026-10-15 用户要求升级（已实现，有意偏离原版）**：「音量均衡效果太差，
感觉基本就是把小音量调大」。根因就是上面那条**固定**补偿增益 —— 它对安静
内容是一记 +45.8 dB 的常数抬升、对响亮内容几乎不动，压缩曲线等于没起作用。
现在改成 RMS 检测（200ms 滑动平均）+ 按目标响度（-5 dBFS RMS）反算 makeup
（`target - curve(target)`，不再是常数）+ 一级硬限幅保住不削波；threshold
由原版的 -50 提到 **-40**，让目标响度附近的内容几乎原样输出。切换过渡的
0.06s 也顺手对齐了原版 `EQ_TRANSITION_SECONDS`（此前写死的 0.12 是错的）。
均衡键图标同时按用户要求**换回 mygo 版自带的 16 画布竖线**（不跟原版的
icon-park Equalizer，见 `state.go` 的 iconEQ 注释与 `TestEQIconStaysMygoBars`）。

## mygo v0.4.6 升级评估（2026-10）

`app/go.mod` 从 v0.4.5 升到 **v0.4.6**（`require` 与 `replace` 同步，`go mod tidy`
后 go.sum 只多两行）。逐条读 release notes 对应 PR 的源码后，本项目实际能用上
的只有 **#258/#259（裁剪遮罩）**；其余是「用不上」——不是没读，是读完后确认
自然用点不存在：

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #273 | `package audio` 播放音频 | 我们有自己的 `internal/media`（WSOLA 变速不变调 + 均衡/压缩器 + oto 输出）。`audio` 的 `SetSpeed` 是**变速变调**（像唱片），没有均衡/压缩，替不了核心；`OpenStream` 只吃 `io.ReadSeeker` 的已解码流。只拿它当输出汇，得把整条流水线包成自定义 `Decoder`——大改、无头验证不了（CI 没声卡）、用户可感知收益为零。**不采用** |
| #258 | 只给需要 transform 的 clip 打遮罩 | **采用（升级即生效）**：抽屉面板/卡片/圆角胶囊数量多，v0.4.5 里只要画面里**任何**一个元素带 transform（比如播放中旋转的唱片），整帧所有圆角 clip 都会被 mask。 |
| #259 | 遮罩按 run 光栅化、跨帧保留 | **采用（升级即生效）**：实测见下。 |
| #248 | 文本布局缓存按「最近使用的工作集」保留 | 本项目布局工作集远小于旧的 8MB 预算（300 张卡 + 抽屉实测每帧只新建 0~3 个布局），不触发旧版「顺序复用互相驱逐」。**不采用（无收益）** |
| #254 | `OnScroll` 让终端接管滚轮 | 只在 `plugins/terminal`，本项目不用终端。**不适用** |
| #252 | 进场元素跟随不参与动画的布局 | 本项目没有用 `ui.Transition` / `Enter`（抽屉走布局位移、卡片走 `AnimatePose`）。**不适用** |
| #250 | toast 出现时恢复 popover 焦点 | 音量/倍速弹层是**手绘**的（absolute + scrim），没用 `ui.Popover`；`corePopover` 的焦点恢复不涉及。**不适用** |
| #247 | Windows 恢复最大化窗口内容 | 主窗没用 `StateKey`（不做窗口状态持久化）。**不适用** |
| #257 | 只在视图显示焦点时才因焦点变化重绘 | `home.go` 搜索框用了 `FocusWithin()`，`focusRead` 为真，焦点变化仍重绘——对我们无差别。 |

### #258/#259 实测（改前 vs 改后）

无头 `ui.Tester` 用的是**纯 Go 软件光栅器**（`internal/raster`），它不走 GPU 的
裁剪遮罩路径，所以 app 级的 build/layout/paint/present 看不到这两个 PR 的收益。
下表是 300 张卡、抽屉滚动、scale 1/2、逐帧帧日志（`MYGO_FRAME_STATS=all
go test -tags perfmeasure -run TestPerfNoCovers`）的聚合：

| 版本 | present avg / p50 | build avg | layout avg | paint avg | scroll（一次 Scroll 含 settle） |
|---|---|---|---|---|---|
| v0.4.5 | 65.0 / 53.9 ms | 0.30 | 0.40 | 0.17 | 695.9 / 808.5 ms |
| v0.4.6 | 56.0 / 43.5 ms | 0.26 | 0.34 | 0.15 | 639.0 / 793.8 ms |

差异在噪声范围内（软件光栅器两版逐字节一致）。

这两个 PR 真正改的是 GPU 路径（`internal/gpu`）里 `raster.ClipMask` 的光栅化
算法。用 mygo 自己的 `BenchmarkClipMaskRoundedCard`（同机同场景：3400×2000 圆角
卡 @2x + 嵌套 cell）对两版源码直接实测：

| 遮罩 | v0.4.5（逐像素） | v0.4.6（按 run） | 提速 |
|---|---|---|---|
| 圆角卡 3400×2000 | 123.3 ms | 1.94 ms | ~63× |
| 嵌套 cell 1900×64 | 4.12 ms | 0.011 ms | ~370× |

复现（在模块缓存里直接跑，不改仓库；v0.4.5 需先把同场景 benchmark 拷进源码副本）：

```sh
(cd "$(go env GOMODCACHE)/github.com/egoist/mygo@v0.4.6" && \
  GOFLAGS=-mod=mod GOPROXY=off go test -run x -bench BenchmarkClipMaskRoundedCard -benchtime 20x ./internal/raster/)
```

`#259` 的「跨帧保留」在 GPU 里是 `idleMaskBytes = 32MB` 的 LRU 遮罩槽：变换元素
来了又走时不再重光栅化，slot 也从「按下标」改成「按 shape+area」，抽屉开合导致
clip 顺序变化时不会整批失效。

### 顺手修的测量 bug

`perf_measure_test.go` 原来把 `tst.Scroll(...)` 之后那次 `tst.Frame()` 计时——
但 `Scroll` 自己已经 settle（真的渲染了那一帧），后面那次 Frame 是场景没变的
**空帧**（实测 ~0.5ms）。所以旧的 scroll 数字（1.4ms）其实什么都没量。现在计时
包住 `Scroll` 本身，scroll 数字变成真实的渲染成本（~640–800ms，软件光栅）。

## mygo v0.4.7 升级评估（2026-10）

`app/go.mod` 从 v0.4.6 升到 **v0.4.7**（`require` 与 `replace` 同步；`go mod tidy`
顺手清掉 go.sum 里残留的 v0.4.5 行）。逐条读 release notes 对应 PR 的源码后，
**没有自然用点**，只升级不改代码：

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #275 | `Bitmap.Pixelated` 放大时按像素方块画 | 本项目没有像素画。唯一「像素状」内容是登录二维码，但原版（Tauri）用 `QRCode.toDataURL({width:200})` 生成 200px 图再显示到 160px——是**缩小**，而 Pixelated 只在放大时生效（doc: “Where it shows smaller, it is as smooth as any”）。强行改成「按模块分辨率生成 + 放大」反而偏离原版。 | **不适用** |
| #276 | `Painter.ImageRegion` 画位图子区域（+ FlipX/Y/Tint） | 本项目没有雪碧图/图集，封面一律 `Fit(Cover)` 整图裁切，无子区域需求。 | **不适用** |
| #277 | `Painter.Transform` 在 `Draw` 回调里变换所画内容 | 本项目唯一的 `Draw` 回调是播放进度条（`paintTimeline`，纯矢量路径，无变换需求）；唱片旋转走元素级 `AnimatePose`（且刻意避免每帧重建视图，见 home.go 注释）。 | **不适用** |
| #286 | `ui.Canvas` 离屏画布（画一次、每帧当图显示） | 最接近的候选是窗口背景那 4 条时段渐变。但（a）mygo 的 GPU 渲染器把同帧所有 op 批进一个 scene pass，渐变只是片元着色器，改成纹理采样收益很小；（b）画布按窗口设备像素分配（1400×900@2x ≈ 20MB 常驻），比 4 条渐变昂贵；（c）软件光栅路径下 image op 是逐像素 4 抽样双线性（见 .pending-bugs.md 的实测），可能比渐变填充还慢——本项目 Linux 正是软件光栅瓶颈；（d）还要处理 resize/时段切换重建。**无收益** |
| #285 | `audio.Player.SetLoopRange` 循环片段 | 本项目的「单曲循环三态」是切歌级循环（`internal/media`），不是片段循环。 | **不适用** |
| #278/#282/#283/#284/#287 | sprite benchmark / farm·mayoi 示例 / gamepad / `game.View`·`game.Camera` | 只在 `plugins/game` 与 examples，本项目非游戏。 | **不适用** |

升级本身无行为变化（v0.4.7 对 `ui`/`scene`/`raster` 的改动全是新增 API，没有改
现有语义），`./check.sh` 全绿。

## mygo v0.6.3 升级评估（2026-10）

`app/go.mod` 从 v0.6.2 升到 **v0.6.3**（`go mod tidy` 后 go.sum 只多/换了 mygo 两行，
顺手把 mygo 自己也要的 `golang.org/x/net` 记成 indirect）。逐条读 release notes
对应 PR 的源码后：**能用上的都是「升级即生效」的框架侧改动**，业务代码没有新的
自然用点；额外做了四处应用侧分配优化（见下）。

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #640 | 文本布局缓存给「大段落」留出**与最近几帧共存的预算**（8MB 以上、最多 32MB，只看 8 帧内没被用到的条目来腾地方）+ 绘制一段文本时 `inlineTransforms` 只查一次 | 视频简介、长评论正文是大段落；按旧规则超过 8MB 的布局**永不缓存**，每次重建都要重新断行成型（抽屉翻页/滚动时正好每帧都在重排）。 | **采用（升级即生效）** |
| #610 | Windows/D3D11：**这一帧画出来和上一帧一样**（动画移动不足一像素）时，不再以主线程能跑多快就多快地重画，而是等显示器下一次刷新 | 唱片旋转、氛围光/洗光过渡里经常出现亚像素位移。此前这些帧在 Windows 上是空转（CPU 拉满、帧率虚高、别的窗口被抢），现在按 60Hz  paced。 | **采用（升级即生效）** |
| #624 | `ui`：隐藏状态的滚动条**不吞点击**（按上一帧是否真的画了滚动条判断），命中直接穿到下面的内容 | 本项目的列表滚动条只在悬停/拖动时显示；此前贴着右缘点最后一行/卡片时会被那条看不见的轨道吃掉，点了没反应。 | **采用（升级即生效）** |
| #650 | Metal：字形图集显式按 mip 0 采样 | macOS 上文字更锐（此前图集有 mip 时按级别插值，小字号发糊）。 | **采用（升级即生效）** |
| #658 | 文本框自己消费按键，Windows 上 Edit › Paste 的 Ctrl+V 不会再粘贴两次 | 主窗搜索框、迷你窗标题都用文本框；修的是「一次粘贴出现两遍」。 | **采用（升级即生效）** |
| #647 | Windows：WebView2 事件回调的 token 地址**随调用传**，不留在会移动的栈上 | 视频弹窗（`internal/video`）就是 WebView2 窗口，事件回调（导航/关闭/状态回传）正是这条路径。 | **采用（升级即生效）** |
| #621 | `mygo.CompileContentRules` + `PageOptions.ContentRules`：WebKit 内容拦截规则（拦加载 + `css-display-none` 隐藏元素） | 唯一的页面是视频弹窗，它加载的是**本地 `video.html`**，`src` 走本地代理的 MP4（`internal/proxy`），页面上没有任何第三方广告/追踪请求可拦。为它造规则只会拦到自己。 | **不适用** |
| #606 / #605 / #658 之外的 diff 视图与 ConPTY | 差异视图折叠、终端清屏 | 本项目不用 `ui/editor` 的 diff 视图、不用终端插件。 | **不适用** |
| #612 | Windows：隐藏标题栏 + 有窗口材质时，最大化窗口不再冒出 DWM 的 caption 键 | 主窗在 Windows 上是 Frameless + 自绘键、且 `DisableMaximize`，根本不会最大化；视频弹窗 Frameless 但不用材质。 | **无影响** |
| #627 | `ui.Table` 最后一列不再给一条「拖了也没用」的拉伸边 | 本项目没用 `ui.Table`。 | **不适用** |
| #641 | `DocumentsOptions.Equal` 在文档锁放下之后再调用（原来会死锁） | 本项目没用 `mygo.Document`。 | **不适用** |

### 应用侧：每一帧都在建的「聚光描边路径」改成缓存（本轮）

升级后顺手用**精确**的分配计数（不是 `-memprofile` 的采样估计）量了一帧的成本：

```sh
go test -tags perfmeasure -run XXX -bench BenchmarkDrawerFrameAllocs -benchtime 60x ./internal/view/
```

场景是稳态抽屉卡片网格（300 卡、虚拟化、无封面），每轮上下各滚一行（每次都真的重绘）。
改前/改后：

| 指标 | v0.6.3 原样 | 本轮改后 |
|---|---|---|
| 每帧分配字节 | 911,802 B | **267,530 B**（−71%） |
| 每帧分配对象 | 12,241 | **7,740**（−37%） |

四处改动（都是纯应用侧，像素结果不变 —— 描边/卡片/日期的既有像素测试全绿）：

1. `playerbar.go` 的聚光环路径缓存（`rimPathCache`）。描边画在 `Draw`/`DrawOver`
   回调里，**每一帧都跑**（唱片旋转这种只重绘不重建的帧也一样）：一圈环是外圈 +
   内圈两条闭合路径（20 条 pathCmd），大表面还要拆成 12 块各建一条。改前
   `ui.(*Path).CubeTo` 一项就占总分配量的 22%、`playerRimRingParts` 再占 14%。
   现在按 `(rect, radius, width)` 缓存几何（不缓存颜色 —— conic 停靠点随 opacity
   变），上限 64 条/每种、合计常驻 ≈150KB，超出按 map 随机淘汰。
2. `playerRimConicStops` 按 opacity 缓存，基色提到包级（不再每次 `ui.Hex` 解析）。
3. `drawer.go` 的 `(*App).card` 不再让整张 `Card`（≈340 字节）逃逸：闭包只捕获
   `bvid`。`cardSpec.Fields` 从切片改成内联 `[4]cardField` —— `spec` 里存闭包必然
   逃逸，切片字段就等于**每张卡每帧**多一次底层数组分配。
4. `state.go` 的 `pubdateCandidates` 加长度/格式快路径：`time.ParseInLocation` 的
   layout 只接受正好 10 位的 `YYYY-MM-DD`，而卡片上的时间绝大多数是 `09-11` /
   `昨天` / `3天前` —— 改前每张卡每帧都白建一个 `time.ParseError`（fmt 级别的
   分配 + 格式化），占总对象数的 ~11%。

墙钟时间：这台机器上（软件光栅 + 其他负载）逐次波动 ±30%，量不出可信差异；
分配数才是可比的 —— 它直接决定 GC 频率，而 GC 是「播放中 60fps 动画」这类
常驻重绘场景里最常见的掉帧来源。新增 `BenchmarkDrawerFrameAllocs`（`perfmeasure`
tag，不进 `./check.sh`）与 `playerrim_cache_test.go`（`AllocsPerRun` 钉住稳态
零分配、键包含 rect/radius、缓存封顶）。

## mygo v0.7.0 升级评估（2026-10）

`app/go.mod` 从 v0.6.4 升到 **v0.7.0**（go.sum 只换 mygo 两行）。逐条读
release notes 对应 PR 的源码后：**能用上的仍然是「升级即生效」的框架侧改动**，
业务代码只有**配置**和**发版脚本**两处必须动，其余读完后确认没有自然用点。

先给结论：`go vet` / `go tool mygo vet` / 全包测试 / 六平台编译全绿；
`-shot` 截图与升级前一致；每帧分配（`BenchmarkDrawerFrameAllocs`，300 卡稳态
滚动）**268,533 B / 7,740 objects**，与 v0.6.3 那轮记录（267,530 B / 7,740）
持平 —— 升级本身没有引入每帧开销。

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #759 | **只重绘「要求重绘」的那几个图元**：一帧如果只有动画在动（`AnimatePose` / `Painter.AnimationFrame`），只跑那几个回调，把结果替换进上一帧的 scene，不再整窗重画 | 本项目到处是「只动不重建」的动画：唱片自转（`home.go` 的 `AnimatePose`）、卡片悬停上浮 / 按压（`theme.go` 的 `pressScale`）、设置菜单条目、合作 UP 主选择器、播放列表行。这些帧此前要把整窗的背景/光罩/氛围光/播放栏重新跑一遍。 | **采用（升级即生效）** |
| #754 | GPU 上**按多个损伤矩形分别重绘**，不再取它们的并集 | 列表滚动：上下两行的损伤矩形并集是整条横带（含背景、播放栏），分开重绘只碰两行。 | **采用（升级即生效）** |
| #726 / #795 | **只做平移的 clip 折叠成普通矩形**，不再每帧打遮罩 | 抽屉卡片列表、播放列表行都在一个滚动容器里（clip 只跟着滚动平移）；此前每帧给整块打一次遮罩。 | **采用（升级即生效）** |
| #758 / #756 | 画在内存里的帧（GDI / CPU 渲染）**按显示器刷新率节流** | 本项目给用户的 CPU 渲染开关（`BILIFM_LOW_MEMORY=1` / `BILIFM_GPU=0`，见 `memory.go`）在 Windows 上就是「画在内存里」：此前动画帧会跑成每秒上千帧，现在跟着刷新率走。 | **采用（升级即生效）** |
| #757 | 圆环与阴影按圆角分带绘制（band 跟着圆角走） | 播放栏的圆角描边、卡片阴影、玻璃面板投影都是圆角阴影。 | **采用（升级即生效）** |
| #769 | 磁盘镜像与 Windows 安装器**按 slug 命名**：`mygo build` 产出 `bili-fm-setup-<ver>-<arch>.exe` | `release-mygo.yml` 的收集步骤按 `*Setup*.exe`（大小写敏感）找安装包 —— 0.7.0 之后**匹配不到**，安装包会静默漏传，同 workflow 的 store / Tauri 迁移 job 再硬失败。 | **必须改**（已改：两代名字都收，且找不到就 exit 1） |
| #768 | 新增 `slug` 配置项（CJK 名转拼音） | 本项目 `name` 是 ASCII（推导值就是 `bili-fm`），但显式钉死它，避免以后改名或改推导规则把已发布文件名换掉。 | **采用**（`mygo.json` 加 `"slug": "bili-fm"`） |
| #782 | Windows `windows.displayName`（可执行文件名与显示名分离）+ `publisher` | `publisher` 之前是空的（Windows「设置 → 应用」与文件属性里没有发布者）。`displayName` 用不上（名字本来就是 ASCII）。 | **采用 `publisher`** |
| #727 | Linux Wayland：无边框窗 / 隐藏标题栏窗开始带 GTK 的 CSD（投影 + 描边 + 从边缩放） | 主窗在 Linux 上是 `Frameless`、非透明 → 0.7.0 起会多一圈 GTK 投影/描边（原 Tauri 版 `decorations:false` 没有）。透明窗（桌面字幕）、panel、X11 不受影响。 | **先保持框架默认**；判定与原版不符时给主窗加 `DisableShadow: true` 即可回到 0.6.4 观感 |
| #765 | macOS：拖拽区**轻按即拖**（不必先等双击判定） | 标题栏、迷你窗标题栏、桌面字幕都是 `DragWindow()`。 | **采用（升级即生效）** |
| #800 | Linux：原生 UI 支持触摸屏滚动 / 捏合 / 点按 | Linux 平板/触摸本上列表可滚。 | **采用（升级即生效）** |
| #805 / #806 / #747 | 退出竞态、macOS 更新替换 bundle 后 Dock 图标保留、经 Launch Services 重启 | 应用内更新走 `up.Install` + `App.Relaunch`（`prefs.go`）。 | **采用（升级即生效）** |
| #654 | **`mygo.NowPlaying`**：一套代码覆盖 MPRIS / SMTC / MPNowPlayingInfoCenter + 媒体键 | 能替掉 `internal/mediactl` 那 ~1300 行平台代码。**不迁**，两个硬理由：① 它只给 Play/Pause/Stop/Next/Previous/Seek，本项目 MPRIS 还实现了 `Volume`/`Rate`/`LoopStatus`/`Shuffle` 四个**可写**属性（`playerctl volume` / `loop` / `shuffle` 能用），迁过去是功能倒退；② 它要 `Artwork []byte`（应用自己抓图），现实现传 `ArtURL` 让系统去取。真要迁，先定「MPRIS 扩展属性」的替代方案。 | **不采用（本轮）** |
| #761 | `Updater.SetPrereleases`：让选了 beta 的用户收到 prerelease | 本项目预览版发的就是 GitHub **prerelease**，所以预览版用户现在其实收不到更新。但 `update_source.go` 的「Gitee 优先」传输层把 `/repos/.../releases` 列表请求**合成成一条最新正式版**，开了也拿不到预览版。要做先让传输层透出 prerelease 列表。 | **不采用（本轮）** |
| #739 | `Context.SetZoom`：整窗内容按比例缩放（矢量重排，不是放大的位图） | 可以补一个界面缩放（Ctrl/Cmd +/-/0）。但界面是原版的**逐像素复刻**，缩放会改掉所有断点行为，且原版没有这个入口。 | **不采用（等用户提）** |
| #760 | `Element.Cache`：把静止子树烘成一张位图 | 本项目最贵的三块（`bloom.go` 的光罩、`shell.go` 的氛围光、播放栏洗光）**已经**是「一张预烘位图 + 一个 `DrawOver`」，再 Cache 一层只多一次拷贝；唱片自转是单张 `ui.Image` 的 `AnimatePose`，同理。 | **不采用（无收益）** |
| #717 | `Bitmap.ReleasePixels`：`DecodeBitmap` 造的位图在 GPU 上传后可丢 CPU 像素 | 本项目封面走 `ui.NewBitmap(image.Image)`（`imagecache.download` 刻意避免解两遍，见那里的注释），没有原始字节可重解，用不上。 | **不适用** |
| #748 | `install.ps1` + `SHA256SUMS`：Windows 一行命令安装 | 白送一个安装入口，但它按 `bili-fm-setup-<ver>-<arch>.exe` 这个**原名**去 release 里找包，而本 workflow 会把安装包改名成 `bili-FM-<ver>-windows-<arch>-setup.exe`（release 说明表格与 store job 都按这个名走）。要做需先定改名策略。 | **不采用（本轮）** |
| #772 | `Element.Hidden`：`display:none` 式隐藏且保留内部状态 | 本项目所有「条件显示」都已经按 mygo 的元素 id 规则处理（`titleBar` 的 `showBrand` 只不填子节点、`shell.go` 的注释写了原因），没有需要「保留滚动位置/编辑中文本」的隐藏点。 | **不适用** |
| #774 | `Path.Cap/Join/MiterLimit/Dash` | 本项目描边（播放栏聚光环、弹幕热力多边形、章节刻线）都是闭合/实线，默认的圆头圆角正是原版要的；没有虚线需求。 | **不适用** |
| #753 | `Painter.AnimationFrameRate`：让自绘动画降帧 | 本项目唯一的 `AnimationFrame()` 用法（`playlist.go`）是「保证下一帧一定到来」，不是持续动画循环。 | **不适用** |
| #724 / #728 / #737 / #781 / #778 / #777 / #787 / #776 / #766 / #764 / #723 / #721 / #722 / #734 / #804 / #719 | UI Automation 零分配、编辑器着色、Markdown 行内公式、`Page.IsAudible`/`ThemeColor`/`ProcessID`、flyout/窗口 FitContent、菜单图标、macOS 托盘标题定宽、子窗跟随父窗、最大化/顶部缩放/无边框材质/DWM caption、Linux 隐藏标题栏菜单栏、Liquid Glass | 视频弹窗已经用频道把 `muted` 推给页面（`video.html`），不需要 `SetAudioMuted`；本项目不用编辑器/Markdown/flyout/系统菜单图标/终端；主窗 `DisableResize` + `DisableMaximize`，不最大化、不从边缩放；不用窗口材质（玻璃是自己画的）。 | **不适用 / 无影响** |

## mygo v0.7.1 升级评估（2026-10）

`app/go.mod` 从 v0.7.0 升到 **v0.7.1**（go.sum 只换 mygo 两行）。v0.7.1 是
两个改动的小版本，逐条对照源码后没有 API 破坏，也没有必须改的业务代码。

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #811 | macOS：用拖拽区（`DragWindow()` / `--app-region: drag`）拖窗口时不再抖动 | 主窗标题栏、迷你窗标题栏、桌面字幕整块面板都是 `DragWindow()`。0.7.0 的「原生拖拽 / 框架手动移动」竞态会在 macOS 上表现为窗口拖动时来回抖一下。 | **采用（升级即生效）**；桌面字幕的位置落盘也依赖 `OnMove`，抖动修掉后保存的坐标更稳定 |
| #808 / #807 | `Font.LineHeight`：`ShapeText` / `ShapeRichText` 也按字号倍数设置行高 | 本项目自绘多行文本（`multilinetext.go` 的 `paintTextLines` / `paintHangingLines`）是**逐行成型 + 显式 linePitch**：原版 CSS 的行高可以小于字体自然行盒（如 22px 字号配 1.18），而框架的行高仍被夹在 `max(指定值, ascent+descent)`，`Font.LineHeight` 只能放大、不能压矮，替不掉现有逐行落点。`commentwrap` 等单行元素已用 `Element.LineHeight`，也不需要改。 | **不采用（现有自绘路径更准）** |

升级门槛：`./check.sh`（vet + 全包测试 + linux/windows/darwin ×
amd64/arm64 六平台编译）全绿。`-shot` 无需重截：两个改动都不改布局/绘制。

## mygo v0.7.2 / v0.7.3 升级评估（2026-10-29）

`app/go.mod` 从 v0.7.1 升到 **v0.7.3**（go.sum 只多 mygo 两行）。这两个版本
没有删除任何我们用的 API（编译 / vet 直接过），但 **0.7.2 的 #799 把
`update_source.go` 的整套「Gitee 优先」机制打死了**，必须改，见下表第一行。

结论：`./check.sh`（vet + 全包测试 + 六平台编译）全绿；`internal/view` 的
无头像素测试全过（升级没改布局/绘制结果），不需要重截 `-shot`。

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #799 | **新增 `netproxy` 包**：mygo 自己的 HTTP 请求（更新器、fetch/websocket 插件）改走系统代理（WinHTTP / CFNetwork / GIO + PAC） | 两个致命影响：① updater 不再用 `http.DefaultClient`，`update_source.go` 那层套在 `http.DefaultTransport` 上的「Gitee 优先」传输层**根本收不到请求**，国内用户等于退回 GitHub 单源；② `netproxy.Transport()` 会 `http.DefaultTransport.(*http.Transport)` 断言（`netproxy.go:84`），我们换进去的 `*giteeFirstTransport` 会让它**直接 panic** —— 一检查更新就崩。已用 `_test.go` 里的探针实测确认 panic。 | **必须改**（已改：见下一行） |
| #820 | **`Updater.SetSource(UpdateSource)`**：官方扩展点，替换「清单从哪来 / 文件从哪下」 | 把 `update_source.go` 从「劫持全局 transport」重写成实现 `mygo.UpdateSource` 的 `giteeFirstSource`：`Manifest` 先问 Gitee release 列表 → 取 `update-<GOOS>-<GOARCH>.json`；`Open` 把清单里的 GitHub 资产 URL 换成 Gitee 同名路径先下。回落顺序、HTML 伪 200 校验、只改写本仓库资产这些老约定全部保留，测试同步重写（`update_source_test.go`，11 个用例）。请求仍走 `netproxy.Transport()`，与框架自己的更新请求同一条代理链路。 | **采用（本轮适配核心）** |
| #846 | **破坏性**：捏合缩放 / 触摸板滑动前进后退**默认关闭**（Windows / Linux 原本默认开，现在三平台都关；要用得显式设 `PageOptions.PinchZoom` / `SwipeNavigation`） | 视频弹窗（`internal/video`）只设了 `UserAgentSuffix`，没设这两项 → 0.7.1 在 Windows/Linux 上可以捏合放大整个播放页，0.7.3 起不行了。 | **采用新默认**：播放页不需要页面缩放，误触捏合把 `<video>` 连同控件一起放大反而是回归；真要开，在 `video.go` 的 `PageOptions` 加一行即可 |
| #856 | `PageOptions.Autoplay: AutoplayAllow`：隐藏窗口也能自动播放有声媒体 | 视频弹窗是**可见**窗口，起播走页面里的 `video.play()`，当前正常。`AutoplayAllow` 的用点是「纯原生壳 + 隐藏 webview 出声」那种结构，本项目音频走纯 Go 引擎，不适用。 | **不采用** |
| #831 | Windows：隐藏的窗口不再把 webview 留在屏幕上挡点击 | 视频弹窗最小化 / 被主窗挡住时不再留一层透明 webview 吃鼠标。 | **采用（升级即生效）** |
| #877 | Linux：窗口自己隐藏后再显示不再空白 | 桌面字幕开关（隐藏 → 显示）、迷你模式切回主窗都走这条路径。 | **采用（升级即生效）** |
| #889 | macOS：没有输入框聚焦时，窗口级 Cmd+C/V/X/A/Z、Cmd+Shift+Z 也生效 | 搜索框 / 设置里的输入框失焦后仍能粘贴（0.7.1 会没反应）。 | **采用（升级即生效）** |
| #818 | macOS：Control+点击托盘图标打开右键菜单 | 托盘有「显示主窗 / 播放暂停 / 退出」菜单。 | **采用（升级即生效）** |
| #827 | 滚轮越过不吃滚动的元素（编辑器标注、悬停卡片）继续交给外层 | 抽屉卡片列表、播放列表、弹幕列表都是滚动容器 + 内部有悬停卡片。 | **采用（升级即生效）** |
| #826 | 屏幕阅读器按行内控件标签朗读整行（如勾选行读「完成」） | 设置菜单的复选框行、播放列表行。 | **采用（升级即生效）** |
| #660 | Windows：透明原生 UI 窗口露出背后的内容，不再是纯黑 | 桌面字幕窗是 `Transparent: true`，Windows 上本来就靠 `Panel: true`（WS_POPUP、无重定向位图）绕开黑底（`lyrics.go` 有注释）。框架修好了普通透明窗，但**不改**现有方案：Panel 路径已稳定，换回去只会引入定位/焦点差异。 | **保持现状（无影响）** |
| #893 / #855 | Windows：缩放原生 UI 窗口时内容不再放大错位、边缘不再黑 | 主窗固定尺寸（`DisableResize`），视频弹窗是纯 webview 窗，都不走「原生 UI 窗口缩放」。 | **无影响** |
| #842 | `PageOptions.PictureInPicture` + `Page.TogglePictureInPicture`（macOS / Windows） | 可以给视频弹窗补一个画中画按钮（后台听歌时把画面钉在角落）。属于新功能，不混进升级轮。 | **候选（下轮功能）** |
| #844 / #848 / #847 | 页面会话状态（前进后退 / 滚动 / 表单）、独立 session（cookie/存储隔离）、`BackgroundThrottling` | 视频弹窗是临时窗口、每次换源只改 `<video>.src`，不恢复表单/滚动；登录态在原生侧 KV，视频页不登录；也没有「隐藏窗口持续播放」场景。 | **不采用** |
| #849 / #852 / #886 / #884 | 页面右键菜单回调、主题内绘制 `PopoverMenu` / `PopoverMenuButton`、`SearchField` 占位符 | 右键菜单（进度条「跳到下一个高能」）是自绘的液态玻璃面板；搜索框是自绘 `ui.Input`，没有菜单按钮/占位符需求。 | **不采用（现有自绘更贴原版）** |
| #853 / #850 | 排版增强：`TextLayout.Lines`、`TextShape.Justify/Indent`、软连字符、`Span.BaselineShift`、段落间距 | 多行文本（`multilinetext.go`）刻意逐行成型 + 显式 linePitch，理由同 v0.7.1 的 `Font.LineHeight` 一条：CSS 行高可以小于字体自然行盒，框架行高会被夹到 ascent+descent。 | **不采用（现有自绘路径更准）** |
| #839 / #868 / #858 | `ui.Video`（先支持 GIF）、`DecodeImage`/`ImageSize`（含 SVG）、释放像素后无 GPU 绘制不再崩 | 视频仍必须走 webview（H.264 无纯 Go 解码器）；封面是 B 站位图，没有 SVG；封面用 `ui.NewBitmap(image.Image)`，不用 `ReleasePixels`。 | **不适用** |
| #819 / #854 / #833 | `windows.installerInclude`、`windows.installDirName`、`macos.icon` 收 `.icns` | 安装包名 / 安装目录维持现状（改 `installDirName` 会换安装路径，影响自更新定位与存量用户）；图标用单张 `resources/icon.png` 够用。 | **不采用** |
| #851 / #891 / #880 / #874 / #859 / #829 / #843 / #845 / #871 | automation 插件、终端滚动倍率、diff 删除条、日历箭头、更新窗错误详情、`Untrusted` 页面、采集状态、HTTP 认证/证书回调、`PreloadAllFrames` | 本项目没有终端 / diff / 日历 / 独立更新窗（更新走应用内液态玻璃对话框），视频页只加载自己的本地 HTML。 | **不适用 / 无影响** |

升级门槛：`./check.sh`（vet + 全包测试 + linux/windows/darwin ×
amd64/arm64 六平台编译）全绿。

## mygo v0.7.4 升级评估（2026-11-01）

`app/go.mod` 从 v0.7.3 升到 **v0.7.4**（go.sum 只多 mygo 两行）。没有删除我们
用的任何 API，`go build` / `go vet` 直接过；**0.7.4 新增了
`Painter.FillRadialGradient` / `Element.RadialGradient`**，而这个仓库此前有
8 处「mygo 没有径向渐变接口 → 逐像素烘位图」的绕法，其中**正圆**的那几条
现在可以直画，本轮已改（见下表「采用」几行）。

结论：`./check.sh`（vet + 全包测试 + 六平台编译）全绿；`internal/view` 的无头
像素测试全过（含改写后的光晕/滑块/迷你窗断言）。

| PR | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| #910 | **`Painter.FillRadialGradient` / `Element.RadialGradient`**：正圆径向渐变（Center 是盒宽高的分数、Radius 是 DIP、0 = farthest-corner，停靠点按 CSS 的预乘 alpha 插值） | 卡片指针光晕（`cardglow.go`，原来烘 3 张 384² = **1.7MB** 位图）、按下态两个径向高光、滑块玻璃高光（`rangeslider.go`，原来按皮肤各烘一张 384²）、迷你窗背景两条光晕（`minibg.go`，原来整块 400×155 位图 = 248KB）。全部改为框架渐变后：**位图从常驻内存里消失**，且任意半径都是解析的（位图拉伸到 200px 半径有采样误差）。 | **采用（本轮主要优化，省约 2.6MB 常驻 + 更准）** |
| #910 | 同上 | **椭圆**径向渐变（`radial-gradient(ellipse 72% 62% …)`：氛围光的两条径向暗角 `shell.go`、`bloom.go` 的光场、`btnlight.go` 的按钮聚光）仍走位图：新接口的 Radius 只有一个值，只能画正圆，椭圆得自己上 `Painter.Transform` 做非等比缩放 —— 那会把整块全窗图层从「一个 Image 图元」变成「逐像素反变换」，是负优化。 | **不采用（保持位图，理由已写进代码注释）** |
| #895 / #918 | Windows：原生 UI 窗口缩放时边缘不再黑；Win10 无边框窗口内容铺到边、不再有一圈白线 | 桌面字幕窗（`Frameless` + `Transparent` + `DisableResize`）会随字号变高变矮；Win10 上那圈白线此前被误当成「边框」。 | **采用（升级即生效）** |
| #913 / #915 | 原生 UI 窗口「画好第一帧才显示」；`Hidden: true` 创建的窗口在隐藏期间就画第一帧，`OnReadyToShow` 可用来「画好再 Show」 | 桌面字幕窗用 `Hidden: true` + `ShowInactive()` 显示 —— 0.7.4 起 Windows 上不会先闪一帧空白。 | **采用（升级即生效，无需改代码）** |
| #928 | macOS：空闲原生 UI 窗口的唤醒从 ~135 次/秒降到 ~13 次/秒（CPU 0.8% → 0.3%） | 迷你模式 / 主窗静止时。 | **采用（升级即生效）** |
| #941 | 大应用的增量包（bsdiff）能算完了（匹配长度加了 64KB 上限，避免二次复杂度） | 发版流水线的 `.delta` 生成；我们的包 8MB 级，本来没触发，但这是纯收益。 | **采用（升级即生效）** |
| #934 | `mygo build` 会在 Linux 归档旁写一份 `PKGBUILD`（`makepkg -si` 可用） | 发布产物里我们手搓 `.pkg.tar.zst`（`release-mygo.yml`）。新 PKGBUILD 是**给用户自己 makepkg** 用的，不替代我们的产物；要不要一起发是产品决定。 | **暂不采用（产物清单不变）** |
| #972 / #927 / #969 | `Element.Scrollbars(false)`（隐藏滚动条但保留滚轮）、`Element.IgnoreInput`（鼠标穿透但读屏仍可点）、`PlaceholderColor` / `CaretWidth` / `DottedUnderline` / `Context.Instant` | 我们的滚动容器都按主题画 6px 细滚动条，不需要隐藏；悬停卡片用 `PassThrough` 已够；输入框是自绘 `ui.Input`，占位色/光标宽度现状即原版。 | **不采用（现状即原版）** |
| #932 | `ui.Themed(c, t)`：给窗口的一部分单独套主题 | 目前只有「播放栏深色 / 正文浅色」这一处双主题，已经由 `Theme.Dark` 分支处理，不需要第二套 ui.Theme。 | **不适用** |
| #971 | 原生 UI 的编辑菜单（撤销/剪切/复制…）跟随焦点自动启用/禁用 | 我们有「全选」这一项（`menu.go`），此前恒可用；升级后没有焦点内容时会自动变灰。 | **采用（升级即生效）** |
| #960 | `updater.AvailableVersion()`：拿到上次检查发现的版本号 | 「检查更新」目前只有弹窗结论，没有常驻展示位。属新功能，不混进升级轮。 | **候选（下轮功能）** |
| #937 / #903 / #949 / #967 / #944 / #935 / #947 / #911 / #920 / #923 / #975 / #950 / #942 / #938 / #925 / #953 / #662 / #900 / #902 / #943 | `DecimalInput`、日历翻页、`ui.Scrollbar`、`HandleInput` 的读屏改写、编辑器/终端/富文本滚动条与 diff 修复、Win11 高缩放下菜单文字截断、页面首帧就按强制明暗渲染、macOS 首次点击、托盘图标副本、对话框 Esc 取消、macOS 27 dmg、病毒扫描占位、安装器复选框本地化、`linux.command` 列表、`-skip-installer`、`WebViewOptions.ScrollToContainer` | 本项目没有金额输入 / 日历 / 终端 / diff / 代码编辑器；菜单是自绘液态玻璃面板（不是系统菜单）；视频页只加载自己的本地 HTML，且是独立窗口，没有「webview 嵌在原生滚动容器里」的结构；`linux.command` 保持单个 `bili-fm`。 | **不适用 / 无影响** |

升级门槛：`./check.sh`（vet + 全包测试 + linux/windows/darwin ×
amd64/arm64 六平台编译）全绿。

## mygo v0.8.0 升级评估（2026-11-06）

`app/go.mod` 从 v0.7.4 升到 **v0.8.0**（go.sum 删掉 v0.7.1 / v0.7.3 / v0.7.4 六行
旧校验、加 v0.8.0 两行）。**没有删除任何我们用到的 API**（逐函数名 diff 0.7.4 → 0.8.0：ui 包 2905 个函数名只少了 5 个
内部实现辅助函数 `animateMatches` / `choosable` / `dragCount` / `rowAtPointer`，
公共面一个没少），`go build` / `go vet` 直接过 —— 但 0.8.0 有**一批行为变更**
必须逐条核对（见下表「Breaking」几行），否则会出「测试全绿、用户实机不对」的
静默回归。

本轮同时按用户 issue #37 的两条交互反馈做了改进（见本节末尾）。

结论：`./check.sh` 等价门禁（vet + 全包测试 30m 超时 + 六平台编译）全绿；
`internal/view` 无头测试 760s 全过。

### Breaking changes 与行为变更（逐条核对的结论）

| 变更 | 内容 | 本项目受影响点 | 结论 |
|---|---|---|---|
| #1328 | **`InputEvent.Code` + 每个键都到 `HandleInput`**（含 `KeyUnknown` 的锁键 / 小键盘 / 第 102 键；单独按修饰键也会到） | 本仓库**没有** `HandleInput` / `ObserveInput` 处理器（全局快捷键全走 `OnShortcut` / `Context.OnShortcut`，是 `switch` 取键），所以「按键按位置命名」「修饰键单独触发」都碰不到我们。逐条 grep 确认 0 处 `HandleInput`。 | **无影响（已核对）** |
| #1022 | **非活动窗口的强调色淡成灰**（`Theme.InactiveAccent`，默认给一个灰） | 本仓库绝大多数强调色是**自绘**的（直写 `t.Blue`），不会跟着灰 —— 若不显式设置，后台窗口会出现「框架控件（输入框 / 滚动条 / 焦点环 / 复选框）变灰、自绘控件仍蓝」的半灰界面。这是常驻托盘的浮窗应用，窗口失焦（去浏览器挑视频）是常态。 | **已修：`uiTheme()` 显式 `th.InactiveAccent = th.Accent`**（三个时段都断言，见 `inactiveaccent_test.go`） |
| #1321 | **`Opacity` 把元素与子树当一个整体淡出**（CSS 语义，重叠子元素不再互相叠暗） | 本仓库的父级淡出都是「整组淡入淡出」（标题栏 hover-to-reveal、封面开关组、抽屉遮罩、toast 入场），没有「父淡 + 子各自半透明」的重叠叠加。禁用节点的 .5 减半是另一条路径（`paint.go`），已由下一条的 `DimDisabled` 接管。 | **无影响（已核对）** |
| #1156 | **按下 / 拖动只跟主键**（`Pressed` / `Dragged` / `DragVelocity` / `DragEnded` 不再由右键或中键起；`PressedButton` 报告是哪个键） | 进度条、音量条、播放列表拖拽排序都只该响应主键 —— 升级前右键拖也能拖动进度条。这是**修好了**一个隐患。 | **采用（升级即生效）** |
| #1365 | **被系统取消的按下什么都不提交**（无 click / 无 `DragEnded` / 无右键菜单；`InputEvent.Canceled` + `PressCanceled` 用于回滚预览） | **进度条有实际影响**：`progressPlan` 用 `wasHeld && !held` 判「松手」，而取消时 `held` 也会变假 —— 会把「窗口失去鼠标捕获 / 菜单拿走释放」误判成松手，提交一次用户**根本没选**的 seek。 | **已修：`progressInput` 加 `cancel` 字段，取消时回落到真实播放位置、不提交**（含两条子测试，见 `view_test.go` 的 `TestProgressPlan`） |
| #1165 | **`ui.Shape` 在 Windows 上给视觉顺序的字形**（与 macOS/Linux 一致） | 本仓库只用 `ui.ShapeText`（折行 / 量宽 / `TextLayout`），没有直接用 `ui.Shape` + `Painter.Glyphs`。 | **无影响（已核对）** |
| #1063 / #1167 | **`mygo build` 不再把 `resources/` 里的 Go 源码打进包**；`resourcesExclude` 支持路径模式；资源目录里的点文件会打进去 | 我们的 `resources/` 只有三张图（`icon.png` / `tray.png` / `tray-template.png`），`mygo.json` **没有** `resourcesExclude`，也没有 `//go:embed` 读自身资源的包。 | **无影响（已核对）** |
| #1064 / #1369 | **toast 的计时在后台继续走**（托盘应用主窗隐藏时不再显示过期 toast） | 本仓库 toast 是应用内自绘（`toast.go` 的 `ui.ToastViewportBase` + 2s 时长），不是系统通知；主窗隐藏时 `Shell` 直接不构建。 | **无影响（已核对）** |
| #1325 | Linux/Windows 上「只做动画的帧」在最小化时也停 | 主窗隐藏时本来就 `Background` 早退（不构建），这是纯收益。 | **采用（升级即生效）** |
| #1211 | 不带通道名的 `Transition` 也淡阴影 | 我们的 `ElementTransition` 只有 toast 与音量弹层两处，都显式写了 Enter/Exit。 | **无影响** |
| #1012 / #1318 / #1333 | Windows 每帧 GPU 时间约减半；macOS 文本重排 CPU 约减半；字形图集按内容定尺寸（各平台文本内存下降） | 三平台纯收益，无需改代码。 | **采用（升级即生效）** |

### 新能力：采用

| 能力 | 内容 | 本项目用点 | 结论 |
|---|---|---|---|
| **`Element.DimDisabled(false)`** | 禁用元素保持自身与子树的不透明度，由调用方自己写禁用态的样子 | 直接**删掉了本仓库整套「.5 补偿」体系**：`disabledDim()` / `frameworkDisabledDim` 整块删除，`disableNav` / `disablePlayerButton` / 进度条 / 迷你窗控制键改为 `Disabled + DimDisabled(false) + 原版绝对值`。**「稍后再看」的移除键**（原版 `.55`）此前因为 .55 > .5 补不回来，只能放弃框架 `Disabled`、自己挡交互 —— 现在可以真正用框架的禁用语义了。 | **采用（本轮主要收益，见 `disabled.go`）** |
| **`Context.WindowHidden`** | 窗口什么都没显示（最小化 / 隐藏 / 别的 Space / **被其他窗口挡住**），语义同 `document.hidden` | 在线人数轮询（`viewers.go`）此前只判 `Win.IsVisible()`：用户去浏览器挑视频、主窗被盖住时 30s 一轮的接口照打（这一栏只在主窗右栏画）。改为 `shouldPollOnlineViewers(WindowHidden, IsVisible)`。`Shell` / `MiniShell` 在 `Background` 早退**之前**记录它（框架的规矩是「隐藏时构建一次、显示时再构建一次」，从它第一次被问开始）。 | **采用（省后台请求 + 更准）；接线实测 `TestWindowHiddenReachesAppState`：把读取挪到早退之后会红** |
| **`Context.ShortcutRepeated`** | 报告「这个快捷键是按住不放的自动重复」 | 全局快捷键此前**全都吃重复**：按住空格会把播放/暂停来回切十几次、按住 ← 会一路退回整个队列。现在按语义分两类：音量 ↑↓ 连续步进（吃重复，与原生 range 一致）、空格与 ←→ 只吃首次按下。 | **采用（修了实机手感 bug）** |
| **`Element.TextShadow`** | 文字投影（字形轮廓 + 偏移 + 模糊） | **桌面字幕**（`lyrics.go`）：字幕窗背景透明度可调到 0（纯文字浮在任意画面上），那时白字压在浅色内容上几乎看不见。新增 `lyricsTextShadowInk`：只在「面板够透 + 文字够浅」时画，alpha 随两者单调走，上限 .55。 | **采用（可读性刚需）** |
| **`Element.HitTest`** | 元素只在你说的地方吃指针（如圆按钮的圆内） | 唱片（263px 正圆）与 UP 头像（44px 正圆）此前命中区是整个**方盒**：四角点到空白也在按唱片（用户报的「点封面边角也能暂停」）。`circleHitTest` 让它们只在圆内吃指针。 | **采用（修了误触）；行为实测：点唱片方盒右上角不再触发播放** |
| **`Theme.TooltipDelay`** | tooltip 的等待时长（0 = 0.6s 框架默认） | 本仓库 55 处 tooltip，全是播放栏 / 抽屉表头的一排图标键。浮窗应用里鼠标顺手划过去要等 0.6s，用起来就当成「没有 tooltip」。收到 **0.35s**（仍留 250ms 的「停一下」，不误触）。 | **采用；`TestTooltipDelayReachesFrameTheme` 验证它确实进了帧内主题（框架就是在构建时读这个值）** |
| **`Element.KeepScrollbars(true)`** | 内容溢出时滚动条常驻（不再只在悬停时出现） | 卡片网格靠「滚到底自动翻页」、播放列表可能上百条 —— 拇指一闪一闪看不出位置，也判断不了「到底了没有」。只对这两种**长列表**开，短列表保持原版的悬停行为。 | **采用；绘制实测 `TestKeepScrollbarsShowsWithoutHover`（悬停为否时右缘拇指像素 140 vs 0）** |
| **`Element.Balance` / `Element.Pretty`** | 两行等长折行 / 正文最后一行不留孤字（CSS `text-wrap: balance` / `pretty`） | 合集标题（两行截断）用 `Balance`：原来常出现「第一行顶满、第二行只剩两三个字」；简介正文（三行截断）用 `Pretty`。 | **采用；`Balance` 有绘制实测 `TestBalanceMakesTwoLinesEven`（两行墨迹差 70px → 15px）；`Pretty` 是同一机制的正文变体，由「折行不丢字」的渲染回归 `TestWrapTextKeepsFullText` 覆盖** |
| **`MaxWidth` + `MaxWidthPercent` 同时生效** | 两种上限同时生效、取更小者，等价 CSS 的 `max-width: min(78%, 34rem)` | 选集胶囊是 `MaxWidth(430)` 的定宽盒，而右栏内容区在最窄窗（`MinWidth: 400`）里只有 ~366 —— 会横向溢出到栏外。补一条 `MaxWidthPercent(100)` 后：宽窗仍是 430、窄窗自动收到可用宽度。 | **采用（`home.go` 的 partPill，含三档窗宽的绘制实测）** |
| **`App.SetDMABUFRenderer`** | Linux 上 WebKitGTK 的 DMA-BUF 渲染器：mygo 默认**关**（NVIDIA 驱动 / 虚拟机 / 容器里会画空白窗），能用的主机上开着滚动帧率是数倍、CPU 更低；`Auto` 只在已知能用的主机（Wayland + Mesa GPU + 非容器/虚拟机）上开 | 视频弹窗是**唯一**用 WebKitGTK 的地方（主窗是原生 UI）。Linux 默认 `Auto`，`BILIFM_DMABUF=off/on` 可强制（排查「视频弹窗白屏」时最有用）。**必须在 `mygo.App.Run()` 之前调用**：框架把这项设置随 AppOptions 交给后端，Linux 后端在 `Init` 里据此设 `WEBKIT_DISABLE_DMABUF_RENDERER`，之后再改字段没有任何地方会读（第一版写在 `WhenReady` 里就是静默失效，已加调用时机测试）。 | **采用（`memory.go` 的 `applyDMABUFRenderer`，调用点在 `Run` 之前）** |

### 新能力：评估后不采用

| 能力 | 内容 | 本项目情况 | 结论 |
|---|---|---|---|
| `NotificationOptions.Image` | 通知显示一张图（macOS 附件 / Linux 通知图 / Windows 大图标） | 本仓库**不发系统通知**：播放状态走媒体中心（Windows SMTC / Linux MPRIS / macOS NowPlaying），应用内提示走自绘 toast。 | **不适用** |
| `audio.Player.SetPreservesPitch` | 框架音频播放器的变速不变调 | 我们的音频引擎是**自研**的（`internal/media`，WSOLA 变速不变调 + 均衡 + oto 输出），根本不经过 `audio.Player`。 | **不适用（自研引擎已覆盖）** |
| `Theme.WindowButtons` | 桌面自己标题栏里有哪些窗口键（gtk-decoration-layout） | **评估后不采用，而且是刻意的**：这个 API 描述的是**桌面自己标题栏**里的键，而本仓库窗口是 `Frameless` —— 合成器根本不提供标题栏。Linux 平铺合成器（Hyprland / Sway）会返回「一个键都没有」，跟着不画就等于重新引入用户 2026-10-17 报告 ③ 的回归（「Wayland 下合成器不给原生按钮 → 关闭/最小化消失」）。无边框窗口必须自绘这两颗键，决定权不在桌面布局上。 | **不采用（Frameless 必须自绘）** |
| `Element.Sticky` | 像 CSS `position: sticky` 那样在滚动容器里保持可见 | 弹幕时间分组 / 字幕行是**虚拟化列表的行**，`Sticky` 对 `List` 的行不生效（行的定位由 `ListState` 接管）；做成「分组标题粘顶」得改成非虚拟化，代价远大于收益。 | **不采用（虚拟化冲突）** |
| `ScrollSnap` / `SnapAlign` | 滚动容器吸附到元素标记的位置 | 本仓库没有轮播 / 分页画廊。抽屉是底部滑入的单面板。 | **不适用** |
| `Element.ZIndex` | 同级元素的绘制顺序（CSS `z-index`） | 现有叠层（遮罩 / 面板 / 关闭键 / 封面角标）靠**建序**就已经正确，没有需要插队的地方。 | **不采用（现状即正确）** |
| `ClipInset` / `MaskLinearGradient` / `MaskRadialGradient` / `Masks` / `ObjectPosition` | 裁剪与遮罩系列 | 我们的裁切都靠 `Clip()` + `Radius`（圆角/正圆），遮罩需求由**框架 Liquid Glass** 与自绘渐变满足；没有「按渐变淡出图片」这类用法。 | **不采用** |
| `Element.TextStroke` | 文字描边 | 桌面字幕用 `TextShadow` 就够（描边会让中文字形变粗、糊边）。 | **不采用（投影更合适）** |
| `PreserveSpaces` | 保留连续空格（`white-space: break-spaces`） | 本仓库没有需要保留空格的文本（简介 / 标题都按自然折行）。 | **不适用** |
| `Element.NoShadow` | 去掉元素已有的投影 | 我们的投影都是逐处显式写的，没有「helper 返回带影子的卡片」这种结构。 | **不适用** |
| 菜单 `Icon` / `Subtitle` / `Tooltip` | 原生菜单项带图标 / 副标题 / 提示 | 只有 macOS 的原生菜单栏（`menu.go`）用到 `mygo.MenuItem`，那里是与原版逐项对齐的四项纯文字动作；Windows/Linux 的菜单是自绘液态玻璃面板。 | **暂不采用（无对应原版设计）** |
| `MenuBase` / `ContextMenuBase` / `AccordionBase` / `TreeBase` / `ComboboxBase` / `SelectBase.SetOpen` | 自定义外观的菜单 / 手风琴 / 树 / 组合框 | 本仓库的菜单（`settingsmenu.go`）、下拉（`collabpicker.go`）都是自绘面板，不依赖这些 Base。 | **不适用** |
| `Context.SetMenuPresentation(PopoverMenus)` | 所有菜单都画在窗口内 | 我们的菜单本来就是自绘在窗口内。 | **不适用（已是现状）** |
| `Context.KeyDown` / `KeysDown` / `OnWindowShortcut` | 窗口当前按住的键；从树里任意位置声明窗口快捷键 | `OnWindowShortcut` 的语义我们已由 `Context.OnShortcut`（在根处注册）达到；没有「快捷键速查表要高亮按住的键」这类界面。 | **不采用（现状已等价）** |
| `TabStop(false)` / `FocusLeft` / `LabelledBy` / `FocusTakesText` | 焦点与无障碍细化 | 本仓库的可聚焦元素都靠 `ButtonBase` 自带语义；没有「可聚焦但不在 Tab 序」的自定义控件。 | **不适用** |
| `InViewMargin` / `InViewAmount` | 元素是否在视野内（带边距 / 按面积比例），如提前加载缩略图 | 封面预取已由**虚拟化列表的可见窗口**（`ListState.Visible()`）+ 连续两帧稳定的闸门（`prefetchGate`）实现，比 IntersectionObserver 更精确（我们要的就是「哪几行在屏上」）。 | **不采用（现有机制更准）** |
| `Local` 的 `io.Closer` 生命周期 | 元素消失时关闭 `Local` 里的 `io.Closer` | 本仓库的逐元素资源（缩略图请求）由 `imagecache` 的 `BeginFrame` / `Retain` / `Cancel` 管；没有需要 `Closer` 的逐元素句柄。 | **不适用** |
| `Preferences.Pointer` / `AnyPointer` / `NoHover` | 指针是否粗粒度 / 能否悬停（CSS 媒体查询） | 桌面应用，指针恒为精细且可悬停；本仓库也没有触屏布局分支。 | **不适用** |
| `TranslateZ` / `Translation3D` / `Magic Move`（`ElementTransition.Fly`）/ `ContentTransition.Morph` | 3D 位移与共享元素飞行转场 | 界面是单窗口、无页面路由（`Router` 也没用），没有「卡片打开成详情页」的转场。 | **不适用** |
| `Flyouts` 的 `FitContent` / `Modal` | 弹出层按内容定尺寸；窗口以 flyout 为父 | 我们的浮层都是自绘（`ui.PopoverBase` / 自绘面板），没有系统 flyout。 | **不适用** |
| `plugins/permissionflow` / `plugins/sfsymbols` / `plugins/automation` | 权限引导 / SF Symbols / AI 代理自动化 | 本仓库不需要系统权限（无麦克风/摄像头/录屏）；图标是逐字拷贝的 icon-park 路径（三平台一致，不用 SF Symbols）；没有自动化脚本需求。 | **不适用** |
| `markdown.Options.LinkMenu` / `markdown.Revealed` | markdown 链接右键菜单 / 流式回复结束通知 | 本仓库不用 `ui.Markdown`（简介是纯文本 `ui.Text`）。 | **不适用** |
| `terminal` / `code-editor` / `diff-viewer` 系列（`CellWidth`、`HoverDelay`、`SplitPercent`、`KeepScrollbars`…） | 编辑器 / 终端 / diff 的新选项 | 本仓库没有代码编辑器 / 终端 / diff。 | **不适用** |
| `plugins/glass` 的 `Native: true` / `glass.Group` | macOS 26 原生 Liquid Glass、多面板合并 | 我们的玻璃是自绘材质（`glass.Glass{}` / `glass.Blur`）＋逐处烘焙的令牌；macOS 26 原生玻璃是另一套观感，与原版 CSS 对齐的目标冲突。 | **不采用（观感不符原版）** |
| `Theme.SetGTKCSS` | Linux 上给 GTK 自身的控件（标题栏 / 标题按钮）上样式 | 主窗在 Linux 上是 `Frameless` + 全自绘标题栏，没有 GTK 自带控件可上样式；视频弹窗是无边框的纯 WebView。 | **不采用（无 GTK 控件）** |
| `ListState.HeaderCell` / `CellsStyle` / `DragRows` | 表头单元格 / 电子表格单元格样式 / 行拖出 | 播放列表的拖拽排序已用 `ui.Draggable` 自绘实现；没有表格。 | **不适用** |
| `Theme.InactiveAccent`（作为「淡成灰」的默认行为） | 见 Breaking 表 | 已显式关掉。 | **不采用（见上）** |

### 用户 issue #37 的两条交互改进（同轮落地）

用户在 2026-11-05 报（维护者回「下个版本优化上」）：

1. **动态 / 收藏 / 历史等页面增加返回功能，点返回回到前面滚动到的位置** ——
   这样想连续加歌时不用重复点开列表并滚到对应位置。
2. **播放列表里有视频时，播放键应该是可操作状态**，不需要先点开播放列表选一条。

实现要点：

- 新增 `App.DrawerBack`（来源抽屉）与 `partsBackTarget()`：只有**列表类**抽屉
  （动态 / 热门与推荐 / 收藏 / 历史 / 搜索 / UP 空间 / 合集 / 播放列表）进选集后
  才长出「返回」键；从主区 / 托盘进来的选集没有。
- `backToDrawer()` 返回普通**已初始化**来源时**不重拉第一页** —— 滚动位置本来
  就存在 `List.GridScroll` / `List.Scroll` 里，一重拉就会被 `ResetScroll` 抹掉，
  那正是用户说的「要重新滚到对应位置」的根因。例外：来源已被换账号作废
  （列表清空、`Initialized=false`）时按 `NeedsLoad` 惰性重载一次，不能把用户
  丢在一个空壳列表上。

- **所有关闭路径统一走 `closeDrawer()`**：关闭键 / `Esc` / 点遮罩 / 下滑关闭 /
  各入口「再点一次关掉」/ 装配层 `playIndex`（选集里点一集播放，走导出的
  `CloseDrawer()`）。从列表进来的选集「关掉」= 回到列表，而不是落回主区。

- **`playIndex` 里必须「先起播、再关面板」**：`startCurrent` 自己也会按
  `keepDrawerOnPlay` 决定抽屉去留，而它只把「选集 / 播放列表」当成要保留的。
  先 `CloseDrawer()` 会把从列表进来的选集恢复成 `history`，紧接着 `startCurrent`
  看不到 `DrawerParts`，又把刚恢复的列表关掉 —— 用户点一集播放后落回主区。
  这条有时序陷阱，所以加了**行为**测试（把起播那一步换成忠实复刻
  `keepDrawerOnPlay` 副作用的替身），顺序写反会红。

- **残留自愈**：`DrawerBack` 只在「选集」这一层有意义。`Shell` 每帧开头有一条
  不变量 —— `if a.Drawer != DrawerParts { a.DrawerBack = "" }`。原因是抽屉的关闭
  路径不止一条（上面那串），漏掉任何一条都会让下一次从主区打开选集时错误地长出
  「返回上一层」，点了还会跳到那个早就不相关的列表。入口处（`openSection` /
  `openParts` / `openSeries` / `openDanmaku` / 播放列表键 / `seriesChip`）另外显式
  清一次，保证**同一帧内**就正确。
- 播放键：`hasSource()` 改为 `Current() != nil || playlistPlayable(...)`；
  `PlaylistStart()` 决定从哪个列表、第几条开始（当前 tab 非空就用它，否则退到
  另一个非空的）；控制器 `togglePlay` 在 `Current() == nil` 时走 `resumePlaylist()`。

测试：`issue37_test.go`（含滚动位置保留的强断言：`GridScroll` / `Scroll` / 页码 /
卡片数逐个钉住；绘制实测点返回键；反证「从主区进来不该有返回键」；
`TestDrawerBackResidueIsCleared` 覆盖残留自愈的四条抽屉值；
`TestAllDrawerClosePathsShareOneEntry` 逐个入口验证「从列表进选集 → 再触发同一
入口 = 回到列表」）、`playlist_test.go` 的 `TestResumePlaylist*` 与源码级契约
`TestTogglePlayFallsBackToPlaylist`。

## 抽屉会话、搜索入口与生产图片磁盘缓存（2026-11-10）

关闭抽屉或切到其他分区后，再次打开会恢复本次运行中的列表和滚动位置。
图片另用内存与磁盘两层缓存，浏览状态只保存卡片元数据和封面 URL。

### 抽屉返回与搜索入口

- 每个固定抽屉保留最后一次浏览上下文，分区 A → B → A 不再因全局 Section
  改变而重拉第一页。成功的空列表也标记已初始化；同一上下文正在加载时不重复
  请求，首页失败后重新打开可重试。手动刷新、切 tab / 收藏夹仍回到第一页。
- 浏览状态只保留到应用退出，不跨重启恢复；每个列表最多 128 条。裁头时复制
  保留部分，释放被丢弃卡片的底层数组引用。切换账号清空浏览状态和分页游标，
  请求代号阻止旧账号、旧上下文或刷新前的响应覆盖新数据。
- 普通已初始化列表的返回（`backToDrawer`）继续保留卡片、页码、`GridScroll` /
  `Scroll`，不触发第一页请求。换账号把来源列表作废后，返回时按
  `List.NeedsLoad` 惰性重载；播放列表是本地数据，不参与重载。
- 搜索结果入口不再用 `len(Cards)==0` 禁用，只要求输入框有非空关键词。点击时
  先切回搜索抽屉：已初始化（含空结果）与在途请求只复用；失败 / 被账号作废 /
  上下文变了才走一次 `Reload`。搜索提交键仍是显式刷新，每次提交都会重拉第一页。
- UP 空间的关注状态与合集列表（`SeriesList`）除了 mid / 账号代号，还捕获发起
  时的 UP 列表 `Context + Gen` 并在落地前复核。A→B→A、刷新、视频/合集 tab
  切换后旧辅助响应不会覆盖新数据；新关注请求会置 pending，只有当前代号的响应
  能清掉它。
- 搜索翻页在抽屉隐藏后仍可完成并清除 Loading；清空关键词会作废在途搜索。
  网络结果经窗口 Update 回到 UI 线程，再由 `acceptListResult` /
  `acceptSearchResult` / `upAuxCurrent` 校验请求归属。

### 图片磁盘缓存（生产策略）

- `view.NewApp` 仍是纯 RAM（`imagecache.New`），测试 / 无头不会碰用户缓存目录。
  生产只在 `main` 的窗口创建前显式换成 `imagecache.NewPersistent(repaint)`，
  复用同一个重绘回调；构造时同步扫一次目录（只读文件头，不 decode）。
- 磁盘只存**下载到的压缩字节**，不存解码位图；默认 128MiB / 7 天 / 8192 条，
  按最近使用淘汰，读命中不续期。目录为 `os.UserCacheDir()/bili-fm/images-v1`，
  文件名使用实际缩略图 URL 的 SHA-256。启动时清理，使用中每隔 10 分钟有机会
  触发后台维护，写入时检查容量；不在每次重启时清空全部缓存。
- RAM LRU 保留 64 条 / 12MiB 的基础位图预算，氛围光派生图也计入并立即淘汰。
  窗口隐藏时 Purge 仅清 RAM 和在途请求；恢复后按需从磁盘解码。Purge 后旧请求
  和旧氛围光结果不能重新填回 RAM。
- 整条读取 / 下载 / 解码 / 写入管线最多 16 路，解码最多 2 路，避免压缩数据
  在解码前无界积压。压缩响应超过 4MiB、图片超过 4Mi 像素或单边超过 8192
  会在完整解码前被拒绝。动图 WebP 的 JPEG 兜底图同样可从磁盘离线恢复。
- 目录不可写、扫描失败、坏文件等只降级为纯网络，不影响抓图；启动时用
  `DiskInfo().Err` 记一条日志，不弹 UI 警告。窗口创建前扫描意味着启动会有一次
  目录遍历（条目上限约束下可控），这是为「不阻塞首个窗口之后的交互」做的取舍。

### 验证与已知限制

- 新增无头整窗会话回归（`drawer_session_test.go`，浅色 / 深色各跑一遍）：
  - `TestDrawerSessionPartsReturnRenderedAToBToASameScroll`：既有 #37 路径，
    90 张卡片真实滚动 → 进选集 → 点返回键。
  - `TestDrawerSessionSectionAToBToASameScroll`：固定分区经 `openSection` 做
    feed → popular（预置已初始化的第二份上下文，不联网）→ feed，两次切换
    都真实渲染并推进抽屉入场动画。
  - `TestDrawerSessionCloseSettledReopenRestoresSameScroll`：再点同一入口关闭，
    等退场动画完整结束、抽屉真正卸载（`drawerShown` 清空）后，再经
    `openSection` 重开，证明恢复的是列表会话而不是抽屉 key 残留。
  三条都断言可见行、锚点卡片、`GridScroll` 不变且 `Loading=false` / 不触发
  reload。fixture 只记录图片请求，画面仍是占位图，不声称真图已加载。设置
  `BILI_DRAWER_SESSION_SHOTS=<目录>` 可导出 `section-*` / `close-reopen-*` /
  `parts-return-*` 的 before/after PNG（关闭路径另有 `hidden`）供人工对照。
- 独立运行 `./check.sh`：vet 与 linux/windows/darwin × amd64/arm64 六平台
  编译通过；全量测试仅有已在修改前复现的 `TestDrawerPanelClip`、
  `TestChapterMarksAreTicks`、`TestSettingsMenuIncognitoGone` 三条失败，无新增失败。
- 独立运行 `go test -race -count=1 -timeout=5m ./internal/imagecache` 通过；
  覆盖离线重启复用、TTL、容量与文件数淘汰、坏文件与目录不可用回退、取消与
  Purge 竞态、解码限制，以及 120 张 480×270 封面的连续轮换。
- 独立重跑三条黑白主题无头会话测试并检查 PNG，滚动锚点与偏移恢复正确。
  缓存预算不包括框架的 mipmap、GPU 纹理和视频窗口；未测量实机 RSS 降幅、
  用户数据下的命中率或启动耗时。无头回归和六平台编译不能替代实机验证。

## HE-AAC 稿件「音频解码失败」→ 回退 DASH AAC-LC 音轨（2026-10-30）

用户报 BV1k3411f7mX（许嵩歌曲 151 首精选）起播即「音频解码失败」。查实不是网络、
不是链接过期，而是**编码 profile**：

- `GetUrlByCid` 走的是 `platform=html5` 的 360P 合流 mp4，这一稿的音频是
  **HE-AAC**（ffprobe：`profile=HE-AAC`、`mp4a.40.5`、49.9 kbps；ASC `2b920800`
  = 显式 AOT 5 / SBR，核心 22.05 kHz）。纯 Go 的 `go-aac v0.7.0` 只做 AAC-LC，
  `NewDecoder` 直接报 `ErrUnsupportedSBR`（显式 ASC 和帧内 fill element 两条路
  都拒绝），所以起播必然失败。
- 同一集 DASH 音轨里 30232/30280 是正常 AAC-LC（`mp4a.40.2`，117/185 kbps），
  只有 30216 是 HE-AAC。B 站这样编码是为了给 360P 合流省码率。
- 抽样 30 条热门视频的 `qn=0` 合流全是 AAC-LC，说明 HE-AAC 不是主流，但一撞上
  就是整条视频播不了，必须兜住。

修法（**不改主路径**：正常稿件仍走原来的 360P 合流，行为和带宽不变）：

1. `media`：`aacpcm.ErrUnsupportedSBR` → 打上 `media.ErrHEAAC` 标记（`aacProfileErr`），
   解码器建不起来 / 播放中途 / seek 重建三条路都覆盖；原始错误链保留，日志照旧。
   顺带补上 `Player.Play` 建流水线失败时 `rf.Close()` —— 换流重试会连着失败两次，
   不关会把 HTTP 连接挂到 30s 超时。
2. `bilibili.GetDashAudioByCid`：`fnval=16` 取 `data.dash.audio[]`，**只认
   `mp4a.40.2`**（HE-AAC 40.5/40.29、Dolby ec-3、Hi-Res fLaC 全部跳过），同 codec
   取码率最高；没有可用的返回空（调用方退回合流地址）。挑轨抽成纯函数
   `pickDashAudioURL`，脱网可测。
3. `main.startCurrent`：起播返回 `ErrHEAAC` 时静默换 DASH 音轨重试（期间有
   `stale()` 复核，快速切歌不会误播）。换流成功时用户只看到「起播中…」多停
   一下，不会先弹「音频解码失败」。`blprobe info/play` 同路径，便于抓日志。

实测（BV1k3411f7mX P1）：合流起播报 `media: HE-AAC（SBR）暂不支持`，
DASH 音轨建流水线 1.3 s、时长 238.3 s、解码 3 s 无欠载。

已知代价（记录在案，不改）：go-m4a 读 fMP4 要扫完所有 moof 才能建样本表，
所以 DASH 音轨**起播**会先把整条音轨 m4s 拉一遍（4 分钟约 5.3 MB；`readHigh`
在定位回 0 后会回落，缓冲条不会假装 100%）。**seek 不再重扫**：fMP4 的
Reader 支持原地 SeekTime（go-m4a 文档明确允许 EOF 之后重定位，实测原地
seek 与全新 Reader 取到的帧逐字节一致），所以 `pipeline.seek` 对
`fragmentedBrand`（iso5/iso6/dash/cmfc…）不重开 reader，4 分钟的歌从
~1.4 s 降到 ~3 ms；合流 mp4 仍是原来的「重开 + 重解析」路径。长视频撞上
HE-AAC 时「先下完整条音轨」的代价已在 2026-11-02 的流式 fMP4 一节解决
（有 sidx 的 DASH 音轨不再走 go-m4a 的全量扫描）。

测试：`media/errors_test.go`（HE-AAC 标记与不误标、类别仍为 KindDecode）、
`media/seektime_test.go` 的 `TestFragmentedBrand`（fMP4 走原地 seek）、
`bilibili/dash_audio_test.go`（只挑 LC、取最高码率、空地址/全不可解码）、
`main_test.go` 的接线反证扩到「起播失败必须换 DASH 重试」。
门槛：`go vet` + 相关包测试 + 六平台编译。`./check.sh fast` 在本机因
`internal/view` 的 mygo 光栅测试偶发超时（HEAD 同样复现，与本次改动无关），
媒体/接口/主包测试均全绿。


## DASH fMP4 流式起播：长视频不再整条下载（2026-11-02）

用户报 3 条音乐合集（`BV1oceneaEdv` 40 分钟、`BV1vr421w7iS` 54 分钟、
`BV1ARMUzJEih` 42 分钟）「解析还是不行」。三条共同点：360P 合流都是
HE-AAC，都会走 DASH AAC-LC 回退；而上一轮的 HE-AAC 修法在 DASH 上仍用
go-m4a 的 `NewReader`，它要扫完**每一个 moof** 才能建样本表。按 192 kbps
算，1 小时音频约 75MB；`scanTopLevel` 虽然只读 box 头，但每个 moof 都夹在
mdat 中间，`remoteFile` 的 512KB 块会把整条音轨都拉下来。实测起播
16.4s / 26.3s / 31.9s（本机千兆），慢网就是「一直起播中」，还容易在某个
Range 请求上撞到 30 秒超时后报「音频格式不支持或加载失败」。

### 修法

1. 新增 `internal/media/fmp4.go`：只对**带 sidx 的 fMP4** 走流式解封装。
   - 从头读 `ftyp/moov/sidx`（B 站实测 <10KB），从 sidx 拿到每条
     reference 的 `[offset, size, duration]` 与总时长。
   - 播放时按分片读 `moof/mdat`，按 `tfhd/trun`（含 trun 的逐样本
     duration/size、tfhd 默认值、trex 默认值）展开 AAC access unit，再用
     现有 `aacpcm` 解码；`RawStream` 输出与 go-m4a 相同的「2 字节长度 +
     access unit」格式。
   - seek 用 sidx 二分定位分片，再按 trun 的采样时长丢弃目标前采样；
     不再重开解析器。
   - 分片里允许以 `styp` 开头（CMAF 常见），`findMoof` 会跳过。
   - 初始化段的格式信息复用 go-m4a：把 `moov` 里的 `mvex` 去掉后按普通
     moov 解析，取 `SampleRate/Channels/ASC`；`Duration` 用 sidx 覆盖。
2. `pipeline` 把 `rd *m4a.Reader` 抽成 `streamDemuxer` 接口；`openDemuxer`
   先尝试 `fmp4Stream`，不是带 sidx 的 fMP4（或解析失败）再退回原
   `m4a.NewReader`，没有 sidx 的流行为不变。`pipeline.fragmented` 明确
   标记原地 seek（B 站 DASH 的 ftyp brand 常是 `isom`，只看 brand 会误判）。
3. `remoteFile` 增加 `chunk` 字段（默认 512KB）；fMP4 路径改为 128KB，
   慢网下不会为了一次 512KB Range 等到 30 秒超时。顺序读时预取仍然有效
   （预取的起点正好是当前块的末尾）。

### 实测（同一网络，blprobe play）

| 视频 | 修改前起播 | 修改后起播 | seek 1200s / 300s / 2400s / 60s |
|---|---|---|---|
| `BV1oceneaEdv` (2411s) | 15.8s | 4.2s（冷 CDN，复测 4.9s） | 均正常，位置落在目标 ±0.1s |
| `BV1vr421w7iS` (3221s) | 26.3s | 1.5s | 均正常 |
| `BV1ARMUzJEih` (2535s) | 31.9s | 1.2s | 均正常 |

连续播放 20 秒跨多个分片无欠载；2x 变速 20 秒（消费分片速度翻倍）也正常。
普通 AAC-LC 合流（`BV1xx411c7mD`）仍走原路径，2.6s 起播，无回归。

### 测试与边界

- `media/fmp4_test.go`：sidx v0 解析、层级 reference 拒绝、moof/trun
  展开、`styp` 跳过、用 go-m4a `FragmentWriter` 生成真实 init+分片后验证
  读取与 seek、无 sidx 的回归。
- `./check.sh fast` 全绿；linux/windows/darwin × amd64/arm64 六平台编译全绿。
- 只支持 `reference_type=0` 的 sidx；层级 sidx、无 sidx、损坏 box 一律退回
  go-m4a（慢但不错）。单个分片上限 32MB；B 站 5 秒分片约 50–130KB。
- 初始化段仍要求 go-m4a 能解析（`mvex` 去掉后）；解析失败同样退回旧路径。
