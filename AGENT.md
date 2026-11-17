# AGENT.md — bili-fm 协作约定与项目知识

> 面向 AI 代理（Hermes / pi / Codex 等）的项目内记忆。项目相关的事实、教训、流程都记这里，不要记到代理的个人记忆里。

## 项目现状（2026-10 起：纯 mygo 原生版）

- **依赖 mygo v0.9.2**（`app/go.mod`）。每次升级都写一份「逐 PR 能不能用上」的
  评估表到 `app/PLAN.md`（v0.4.6 / v0.4.7 / v0.6.3 / v0.7.0 / v0.7.1 /
  v0.7.2+v0.7.3 / v0.7.4 / v0.8.0 / v0.8.1–v0.8.3 / v0.9.0 / v0.9.1 / v0.9.2
  各一节），当轮的坑与门槛写进 `.pending-bugs.md`。**先读这两处再动依赖。**
- **单实例锁由框架持有到进程退出**（v0.9.2 #1909）：更新后的「立即重启」只能走
  `mygo.App.Relaunch()`，别自己拼 `os.StartProcess` / `exec.Command` —— 新进程会
  等旧进程真正退出再接手，自拼启动撞上仍持锁的旧进程就表现为「点了重启没反应」。
  `app/single_instance_test.go` 用 go/parser 扫 prefs/update_*/main 的 AST 拦下来。
- **列表行的「来去」用框架的 `ListState.MoveAnimation`**（v0.9.2 #1904）：删一条 /
  拖一条换位置后，其余行滑到新位置而不是一帧跳过去。前提是行键稳定
  （`ListState.Key`），播放列表用记录 ID；滚动与系统「减少动态效果」由框架处理成
  瞬移，不用自己判断。
- **看不见就别渲染**：`Shell` / `MiniShell` 在 `a.Background` **或**
  `a.WindowHidden`（框架的 `Context.WindowHidden`：最小化 / 托盘 / 别的桌面 /
  被别的窗口盖住）时整棵早退；后台周期性更新（播放器进度、视频弹窗状态）一律
  走 `runOnUI(c.app, !c.app.Hidden(), fn)` —— 不可见时用 `mygo.RunOnMain`
  （只改状态、**不请求帧**）。判定读 `App.Hidden()`（`WindowHidden` 的原子镜像，
  `noteHidden` 在 Shell/MiniShell 每帧一起写），别读裸字段（UI 线程写、后台
  goroutine 读 = 数据竞争）。实测：不可见时一帧 1.02ms → 0.015ms。
- **起播不关抽屉**（`keepDrawerOnPlay`）：切歌 / 自动续播 / 播完下一集时，
  用户正在浏览的抽屉（动态 / 历史 / 搜索 / UP 空间 / 合集 / 收藏 / 播放列表 /
  选集…）**一律保留** —— 起播不是「离开浏览」的动作。唯一例外是弹幕 / 评论抽屉
  （内容跟着正在播放的那一集走，`startCurrent` 紧接着就清空 `a.Danmaku`）。
  点卡片换抽屉由 `openCard` / `BrowseTo` 负责；手动从选集点一集仍由 `playIndex`
  起播后 `CloseDrawer()`（issue #37 ①：从列表进来的要回到那个列表）。
  **所有起播路径必须经过 `runStartPlayback(c)`**（`step` / `handleEnded` /
  `playIndex` 两条分支 / `playPlaylist`）—— 它是行为测试唯一的替身入口，
  绕过去就会在测试里真起播（`c.bl` 为 nil → 后台 goroutine panic）。
- **透明窗在 Windows 上要去掉 DWM 的 1px 边框**：Win11 给每个顶层窗口画
  `DWMWA_BORDER_COLOR` 边框并按圆角偏好裁剪，不透明窗的内容能盖住，**透明窗
  盖不住**（深色主题下就是「最外层一圈深色边」）。`tuneWindowFrame(hwnd)`
  （`app/windowframe_windows.go`）把边框色设 `DWMWA_COLOR_NONE`（`0xFFFFFFFE`，
  不是 0）、圆角设 `DoNotRound`；主窗 / 迷你窗 / 桌面字幕窗三个窗口都要在
  `NewWindow` 之后立刻调。Linux 不用管（mygo 的 `useCSD` 排除透明窗，走
  `gtk_window_set_decorated(false)`）。
- **「减少透明度」是系统偏好，不是本应用的开关**：Windows 关闭「透明效果」
  （`Personalize\EnableTransparency == 0`）会让 mygo 报 `ReduceTransparency`。
  正确的兼容方式是「功能照常可用 + 说清为什么暂时看不出效果」，**不是**跟着
  禁用功能（用户 2026-11-11 实测后被这一点坑了三处）。三个已知落点：
  1. **播放栏边缘那层不许用 `glass.ScrollEdge`**：它在 `ReduceTransparency` 下
     会把背景 alpha 拉成 255 并铺满**整块**（mygo `plugins/glass/edge.go`），而
     那块绝对定位、绘制顺序在播放栏之后 —— 整条栏（含按钮）被刷成实色
     （实测 `#475569`、亮度极差 0）。走 `playerBarEdgeLayer` 的自绘渐变分支，
     主窗与迷你窗共用。测试 `TestPlayerBarVisibleUnderReducedTransparency`。
  2. **透明度滑块 / 预设不许 `Disabled(t.ReducedTransparency)`**：值要能先设好，
     系统开关恢复后立即生效；旁边那句提示必须写出「设置 › 个性化 › 颜色」。
  3. **卡片指针光晕不受 `reduceMotion` 限制**：Windows「关闭动画效果」会被读成
     `ReduceMotion`，用户只想少动画、不是想删掉静态高亮。做法是光晕照画、只把
     淡入淡出时长收到 0（`cardGlowLayer`）。
- **窗口圆角与 DWM 边框是两个开关，别一起关**：`tuneWindowFrame(hwnd)` 只去
  Win11 那圈 1px 边框、**保留** DWM 圆角（主窗 / 迷你窗的内容铺满整窗，圆角必须
  由 DWM 裁）；`flatWindowFrame(hwnd)` 才连圆角一起关（桌面字幕窗的内容自己画
  14px 圆角面板）。上一轮把两者合成一个，直接把主窗的圆角干掉了。
- **窗控按钮组只有一套实现，圆角也只有一个来源**（`trayRadius() = 8`）：主窗
  （`titleBarCtrlTray` + `windowControlButtonSized`，30/14）与迷你窗（同函数，
  20/12）**共用**，差异只在尺寸。别按 `viewGOOS` 给圆角分叉 —— 原版那个
  `body.platform-linux #root { --app-window-radius: 0 }` 是 WebView 模型的产物
  （窗口由 GTK 装饰），原生版 Linux 是 Frameless 透明窗，内容自己负责形状。
- **桌面字幕：面板不画任何描边 / 投影**（`TestLyricsPanelHasNoBorder` 用「背景与
  文字全透明 → 整片 alpha 必须 0」做判据）。「窗口在哪儿」的提示由设置键承担：
  指针进字幕区就浮出。齿轮显隐判据是
  `over || a.LyricsHover || a.LyricsGearHot || a.lyricsDragging` ——
  **只有 `LyricsGearHot` 是不够的**（那只是设置键那一小块热区，穿透时用户得去
  猜位置）。`lyricsPollNeeded()` 三平台都返回 true：Linux 的输入区域同样让窗口
  在其余部分收不到 motion 事件，效果与整窗穿透一样。
- **登录过期要能自愈**：接口回 `-101`（账号未登录）时 `bilibili.apiError` 会给
  `ErrNotLoggedIn`，app 侧 `controller.listLoadFailed` 静默调 `expireLogin()`
  （清登录态 + 失效 cookie + KV 里的资料，抽屉空态提示去登录），其余错误才 toast。
  启动时 `verifySessionAsync` 用 nav 校验一次（`SessionValid` / `sessionVerdict`：
  只有 0 / -101 算结论，风控 -352 不登出）。**别再让列表直接吃 -101**。
- **「需要登录才行」的动作一律静默限制**：字幕（字幕轨 + `aisubtitle` 正文）
  与云端进度读写（`x/v2/history/report` / `GetPlayProgress`）都要登录。判据只有
  两处：`controller.subtitlesAvailable()` 与 `controller.cloudProgressOn()`
  （后者 = `LoggedIn && !Incognito`）。未登录时**一个请求都不发**、不记失败日志、
  不写 `SubtitleError`。章节（view_points）不需要登录，别一起挡掉。
- **视频 / 弹幕设置的唯一入口是视频窗顶栏的齿轮键**（`view.VideoChrome`
  的 `settingsGearKey` → `internal/view/videosettings.go` 的 `ui.PopoverBase`
  面板）。弹幕 5 项（总开关 / 不透明度 / 字号 / 显示区域 / 同屏上限）、播放
  2 项（倍速 / 广告跳过）与「原生视频播放」都只在这里；主窗设置
  下拉与 macOS 原生菜单里一处都不留（用户 2026-11：「播放页面的设置不要放在
  菜单栏下」，明确不要两处入口）。**视频窗没有音量均衡入口**：原生视频的 PCM
  在系统媒体框架内部就送去声卡，`internal/media` 的 `Compressor` 只挂在 oto
  解码后的自己的流水线上，`ui/video` 的导出面也没有音频数据钩子 —— 这一项
  在视频窗里**不生效**，用户 2026-11 裁决直接去掉（控件条不画键、设置面板
  不留项，主窗的均衡键与共享 `PlayerSettings.EQ` 一律不动）。原因与上游待补
  写在 `app/PLAN.md` 的「原生视频里的广告跳过 / 音量均衡」。
  落盘仍是既有入口与既有键：弹幕走 `Act.SetDanmakuStyle`
  （`danmaku*`），原生开关走 `setNativeVideo`（`nativeVideoPlayer`）。面板
  打开期间控件条不自动隐藏（`holdChrome` 里的 `showSettings`）；高度按窗口
  封顶、内部 `ui.Scroll` —— 320×200 最小窗口也能开、能滚、能关（关闭键在
  固定头部）。Esc 在面板开着时只关面板（`do()` 的 `videoKeyExit*`），全屏
  同理。
- **视频窗控件条 == 主窗播放栏 + 全屏键 − 均衡键**（用户 2026-11）：
  控件条的播放/下一集/进度/时间/音量/倍速/跳过/**全屏**全部来自
  `playerbar_shared.go`（绑定目标是视频播放器），视频窗**不许再写第二套控件**：
  `videoPanelKeys`、内联音量条（`volumeSlider`）、自绘全屏键、
  `videoPlaySurface/videoPlayInk` 都已删。键尺寸只有一份
  `playerBarKeyMetrics`（播放 42 / 工具键 32 / 均衡·恰饭 34 / 倍速 40 / 时间列
  56），图标尺寸只有 `playIconSize/nextIconSize/volumeIconSize/eqIconSize`
  （+ 全屏键的 `fullScreenIconSize`）—— 视频窗若再出现一套 16/18 的图标令牌，
  就是用户报的「下一首的图标被改了」。图标本身只有 `state.go` 那一份 icon-park
  定义（全屏键的 `iconFullScreen/iconExitFullScreen` 也在那里），`videochrome.go`
  里**不许**再出现 `iconPark(` / `<path d=`。
  热力条与主窗同一份数据（`App.DanmakuHeat`，同一个 cid 缓存，经
  `video.Options.DanmakuHeat` + `Manager.SetDanmakuHeat` 推过来）、同一个
  `playerHeatPainter`；横向范围永远是**进度条轨道矩形**（两窗都绑
  `TimelineHandle`），不许退回整条栏宽。
  弹出式而非平铺：音量/倍速在任何宽度都只在弹层里（`videoChromeLayout` 没有
  「内联音量条」这一档）。
  悬浮闪烁的根因：视频窗的播放键曾同时画两层图标（玻璃播放栏那层白色
  drop-shadow 副本 + 真图标），全屏键则来自顶栏那套键（hover 才加一层
  `InsetShadow` + 描边），hover 时图标墨色在 140ms 里过渡、另一层不动 →
  看起来在闪。现在控件条按键只画**一层**图标（`PlayerBarBind.OnPanel` 时跳过
  `playerIconShadowFn`），材质层序在 hover 前后不变。
- **「加载中 / 缓冲中」只有播放键上一套指示器**（issue #42）：`App.Buffering`
  仍是**起播初始化**的唯一闸门（`c.seek` 的音频排队只认它，且视频窗占着输出
  时不排队）；播放中的欠载另走 `App.Waiting`，由 `media.Player.Buffering()`
  （oto 在播 + 流水线原子 waiting 位）经 reportLoop 500ms 心跳写回。流水线的
  waiting 在创建、seek 清队列、`Read` 等满 `readWait` 时为真，取到**任意**
  PCM 字节（含全零静音）即为假；EOF/error/close 置原子 `terminal`，seek 在
  `p.mu` 里复位 terminal 并置 waiting。界面读的 `waitingForData` 是**只读
  谓词**：terminal/stopped 时返回 false 但**不回写** waiting（否则并发 seek
  的新信号会被读掉），清位都在 `p.mu` 临界区内完成。
  暂停时 `Player.Buffering()` 恒 false。
  **「点击播放 → 窗口开出来」这段也有忙态**：此刻 `VideoOpen` 还是假、没有
  播放器能回传，靠 `App.VideoOpening`（`playerBusy()` 优先级 VideoOpen >
  VideoOpening > 音频）。开成 / 取地址失败 / Open 失败 / 提前关窗 / 切集五条
  出口都走 `finishVideoOpen()`，漏一条就是播放键永远转圈。同一处的
  `videoOpenID` 代号保证切集后旧的那次取地址不会把上一集开出来；
  `openVideo` 的 goroutine **不许**直接读 `a.Pos/Speed/Muted/Volume/VideoPinned`
  （界面线程写的字段），一律在界面线程取快照。
  播放键的忙态是共享 `PlayerBusy`（`PlayerLoading` / `PlayerBuffering`），
  图标复用 `ringSpinner`（42px 键 18/2、mini 28px 键 16/2），文案分别是
  「加载中…」「缓冲中 · 点击暂停」；主窗/迷你窗/视频窗同一套，不占额外布局。
  视频窗直接读播放器（`VideoChrome.playerBusy`：Playing 且 Buffering / 未
  Ready 才算忙，暂停/片尾不转；`view.VideoPlayer` 没有 Err，原生错误走
  `onFailed` 的失败/回退路径），主窗播放栏用 `video.State.Buffering`
  经 `App.VideoBusy` 回传。视频窗进度条上旧的呼吸描边 `videoBufferingPulse`
  已删。窗口 `WindowHidden` 时 `View` **不能**整棵早退：`Content` 就是生产
  路径的 `ui.Video`，框架要靠它每帧 `advance` 去轮询 State、更新
  Ready/Buffering，并保留框架自己的媒体推进 / hold（遮挡暂停）处理；隐藏分支
  只跳过控件条 / 忙态圆环 / 弹幕 / 点击层，`Content` 照建。这里只保证框架
  的处理照常发生，**不**承诺隐藏后 muted 视频继续播（框架会故意 hold 被遮挡的
  muted 视频）。`App.playerBusy` 先看 `VideoOpen`
  再看音频 `Buffering`（视频占着输出时，后台残留的音频初始化不能把播放键卡在
  「加载中」）；`c.seek` 只在 `Buffering && !VideoOpen` 时排队。起播初始化期间
  播放键直接忽略（不许 Resume 旧/空引擎），起播收尾在界面线程读**当前**
  Speed/EQ/Volume 下发（不能用起播开始时的快照；音量复用 `c.applyVolume`
  以尊重静音）。
- **音频起播先取 DASH AAC-LC**（issue #42 后续，2026-11-17）：`startCurrent`
  不再先取 360P 合流、等 HE-AAC 才换轨。`startAudioSource`（app/audio_start.go）
  先 `GetDashAudioByCid`，没有可用 URL 或首次 Play 失败才回退**一次**
  `GetUrlByCid`；同一 URL 不重播，`ErrSuperseded` / stale 在每次查找/Play
  前后立即退出。两条都空只报本地空地址哨兵（沿用「暂时无法播放」warning），
  有实际 Play 错误则报最后一条；合流报 HE-AAC 时不再重复查 DASH。实测
  BV1BFsA6TERt（aid 117624373577490 / cid 43646192976）：合流 MP4
  179,509,885B、moov 6,546,892B，首块 PCM 9.72s / 203 次 Range / 26.3MB；
  DASH 30280 54,383,143B，首块 PCM 0.26s / 3 次 Range（第 3 次为预取）/ 131KB。Range 预取、
  块大小、sidx、WSOLA/EQ/重采样/200ms 队列与分段均未改；视频源仍是合流。
- **视频源优先 DASH 分片（本地 HLS）**（issue #42 视频那一半，2026-11-17）：
  视频窗不再直接播 360P 合流 MP4。合流的长视频 `moov` 在文件头且有几 MB
  （BV1BFsA6TERt 是 6,546,892B），解第一帧前必须先读完 —— 实测首帧前 6.88MB，
  1MiB/s 下 6.57s。`internal/dashhls` 把 DASH 的 `moof+mdat` 分片**原样**当
  本地 HLS 播：sidx 给精确字节区间，编进分片 URL 查询串，音画同步 / 拖动 /
  倍速交给平台媒体框架。**不改写任何字节**（没有 muxer）。
  三条硬规矩：
  1. **不能用 `#EXT-X-BYTERANGE`**：GStreamer 的 `hlsdemux` 忽略它，请求回来
     是不带 Range 的整条流，等于把文件下完（实测首帧 11s）。区间放查询串。
  2. **分片必须复用 keep-alive 连接**：每个分片新建 TLS 时握手成本直接进首帧
     （实测某台 CDN 主机握手 3.5s → 首帧 11s，比旧路径还慢）。
     `newClient` 的 `MaxIdleConnsPerHost` 与测试里的连接计数就是钉这条。
  3. **`#EXT-X-MAP`（init 段）在 media 区间之前**，分片越界检查要放行它，
     否则 hlsdemux 取 init 就 416，整份列表起不来。
  实测（Linux GStreamer，音频 fakesink 无窗口）：首帧中位 569ms（旧路径 1380ms），
  首帧前 476–593KB（旧 12.3MB）；时长 / seek / 2 倍速正常。**Windows 与 macOS
  未实测**。
  自动降级：`video.Options.SrcFallback` 在分片源起播失败时用合流地址重开一次，
  仍失败才问用户要不要 WebView；`ErrUnsupported` 不重试（换地址救不了）。
  **换播放器必须重新挂 `onFailed`**（`wireNativeFail`）：`report` 触发后会把它
  置 nil，`switchNative` 换了播放器不重挂，降级后再失败就没人接手。
  webview 回退路径拿的是合流地址（页面里的 `<video>` 播不了 m3u8）。
  视频窗的其余视觉取值在 `view.videotokens.go`：网格（`videoStep`/
  `videoStep2`）、两栏共用的 `videoInset`/`videoPadX`、顶栏 5 颗键的
  `videoKeySize`(30)/`videoKeyRadius`(6)/`videoIconSize`(16)、字号 13/600、12、
  11、面板行高与内边距、hairline 与两条栏的厚玻璃材质（`videoTitleBar` /
  `videoControlBar` / `videoBarFill`）。**控件条的键与时间码不在这里**（它们走
  共享播放栏）。视频窗两条栏各在对着画面的那条边补一条 1px 低对比 hairline
  （`videoHairline`），弹幕区固定夹在两条栏之间（`videoDanmakuArea`）。
  口径在 `videotokens_test.go`（顶栏网格 / 中心线 / hairline / 四尺度两态不
  重叠）与 `videobar_sync_test.go`（控件集合 == 主窗 + 全屏 / 尺寸一致 / 图标
  同源 / 单层图标 / 热力几何 / 弹出式 / 拖动）。
- **视频窗上下两条栏是「厚玻璃」**（`view.videoTitleBar` / `videoControlBar` +
  `videoBarFill`）：底色 = `Panel` 令牌 + 不透明度下限（浅 .93 / 深 .95，原
  .88 / .92）。只抬 alpha、不动 RGB —— 色相层次与 1px 线 + 顶沿内高光 + 投影
  都得在，抬到 1.0 就成了把画面切成三条的纯色块。**别让它跟窗口透明度设置变
  薄**：主窗玻璃背后是应用自己的背景，这两条背后是视频画面。可读性口径在
  `videobar_material_test.go`（深浅 × 黑/白画面：`Ink`/`Muted` ≥ 4.5:1，
  图标与 `Blue` ≥ 3:1；并钉「渲染像素 = 材质叠画面」而不是纯色块）。
- **控件条自动隐藏的规则只在 `videoChromeIdle` 里**：指针坐标一变（含 1px）
  **当帧**唤醒（mygo 没有 OnMouseMove，但指针一动框架就重建一帧，root 每帧读
  `PointerPosition`）；静止 2.5s **且** `holdChrome` 为假才淡出（不在栏上、不在
  拖进度、不暂停/缓冲、面板弹层都关着）；指针离开窗口（`over` 真→假）跳过静置
  直接淡出；键盘与点画面/播放键都走 `idle.touch` 重置计时。收起后 33ms 采样是
  给「坐标变了没人重画」的平台兜底 —— **别在亮着（等静置）时也排采样定时器**：
  mygo 一个窗口只有一个 `After`（先到先得），16ms 采样会把双击窗口之类的定时器
  全量化掉。浮出 110ms（80~120ms 区间）、淡出 220ms。
- **双击的第二下会在松开帧再报一次 `Clicked`**（mygo 的 quietClick）：
  `videoClickArbiter.quiet` 必须吞掉它，否则双击之后挂着一个幽灵单击，300ms 后
  顺带切一次播放/暂停。
- **隐身模式 = 不续播**：`resolveResume` 在隐身时返回 0（云端不读、本地断点也
  不跳）。本地断点仍然照写，关掉隐身后能续上。用户 2026-11-10 明确要求。
- **`--liquid-chromatic` 色散边（下暖 / 上冷）**（`paintPlayerChromatic`）：
  带只画到圆角**切点**（`[x+radius, x+W-radius]`），裁剪矩形只向两端各外 1px
  且半径不变（外扩 1px + 半径 +1 的写法会让带在圆角处多探出 ~2.5px 的直线）；
  alpha = 令牌 × `chromaticHalfPixel`(0.5，补「框架画不出 0.5px」) × **宿主
  `::after` 的 opacity**（工具键 .52 / 抽屉选集搜索框 .5 / 主搜索药丸与播放键
  1.0）。三样缺一样都会被用户看出来（2026-11-10 两轮反馈都是这条）。
- **主窗的 `TitleBarStyle` 只在 Windows/macOS 给 `TitleBarHidden`，Linux 给
  default**（`titleBarStyleFor`）：Linux 上那是「隐藏标题栏但保留 GTK 自己的
  窗口按钮」，与两颗自绘键冲突。窗框剩下的部分由 mygo 对 `Frameless` /
  透明窗的处理决定，应用侧没有别的开关（见 `.pending-bugs.md` 2026-11-10 ⑤）。

- **桌面字幕的「鼠标穿透」是「穿透，但齿轮固定可点」**（`App.LyricsClickThrough`）：
  齿轮的位置/尺寸见 `view.LyricsGearInset` / `LyricsGearSize`（绘制与热区同源），
  热区几何是 `lyricsGearShapeRect`。Linux 把窗口的 **input shape 设成齿轮那一块**
  （`app/lyrics_clickthrough_linux.go`，purego 直调 GDK/cairo；符号缺失时退回
  mygo 的整窗穿透），Windows/macOS 是整窗穿透 + 指针轮询（`lyricsPollNeeded`）。
  设置页（字幕自己的面板）与主窗设置菜单 / macOS 原生菜单里各有一个开关，同源；
  设置页打开时永不穿透。
- **设置键既是按钮又是拖动手柄**（`Act.BeginLyricsDrag` / `MoveLyricsDrag` /
  `EndLyricsDrag`）：mygo 的 `Element.DragWindow()` 只在元素**没有**其它交互标志
  时生效（`ui/input.go` 的 `interactive&^flagDragWindow == 0`），按钮拿不到。
  **`Dragged()` 报的是每帧增量**（`forgetInput` 每帧清零），绝不能拿它累加位移
  —— 只用它判「越过 4 DIP 起步阈值」；位置按**指针绝对坐标**算（Begin 记
  「指针 − 窗口左上角」偏移，Move 每帧摆窗口）。原生拖动（Linux
  `begin_move_drag`）只在**非 layer surface** 时采用（gtk-layer-shell 里是空操作），
  拖动期间不写 KV（逐帧 OnMove 会把中间坐标/0,0 写进去），松手统一落盘。
  **拖动后必须吞掉那次 `Clicked()`**：mygo 的 `Released()` 只在元素调过
  `Draggable()` 时才算拖动，用 `Dragged()` 时松手照样给 click —— 判定要写在
  `Clicked()` 之前，且「吞点击」标记只能活到松手后两帧（拖到按钮外松手根本
  没有 click，靠「读到 click 才清」会把键永久点不开）。
- 设置键的显隐条件只能是 `over || a.LyricsGearHot`：**别**写成
  `|| LyricsClickThrough`（那会让它常显，用户 2026-11-11 报过这条回归）。

- **「跳到下一个高能」的选点规则只有一份**（`view.NextHighlightPoint`）：
  控制器真 seek、进度条右键菜单写副标题都从这里取。菜单副标题靠 0.8.1 的
  `MenuItem.Subtitle`（**系统菜单也显示**：macOS 14 第二行，其余平台跟在
  标签后成「标签 — 副标题」）。别再在菜单里抄一份阈值。

- **0.8.3 起应用内更新会删掉「旧版装过、新版没有」的文件**（updater 按
  `mygo build` 写进安装目录的 `.mygo-files` 清单；老安装第一次更新不删任何
  东西）。产物必须由 `go tool mygo build` 生成，别手工拼安装目录。

- **0.8.0 起禁用态用 `DimDisabled(false)`**（`app/internal/view/disabled.go`）：
  框架不再自动给禁用节点乘 0.5，禁用元素的不透明度就是**原版 CSS 的绝对值**
  （.34 / .42 / .55）。此前那套 `disabledDim(target) = target/0.5` 的补偿已删除
  —— 别再写回去。例外只有「原版 disabled 不改不透明度」的搜索提交键（.82），
  它还得靠回调自己挡（见 disabled.go 的总说明）。

- **`Context.WindowHidden` 要在 `Background` 早退之前问**（`Shell` / `MiniShell`
  开头）：框架靠「视图第一次问它」来决定隐藏/重现时各给一帧构建，放在早退之后
  就永远读不到。后台轮询（在线人数）据此暂停，见 `viewers.go`。

- **`internal/view` 单包测试要 `-timeout=30m`**：无头光栅 + 字体度量要 600~760s，
  默认 10m 会在跑完前 panic（不是回归）。

- **窗口键（最小化 / 关闭）一律自绘，不看 `Theme.WindowButtons`**：本仓库窗口是
  `Frameless`，合成器不提供标题栏；平铺合成器（Hyprland / Sway）该 API 返回
  「一个键都没有」，跟着不画就是 2026-10-17 报告 ③ 的回归（关不掉窗口）。

- **「只在某一层有意义」的状态要有每帧兜底**：`App.DrawerBack`（issue #37 ①
  的返回来源）除了各入口显式清，`Shell` 开头还有一条
  `if a.Drawer != DrawerParts { a.DrawerBack = "" }`。关闭抽屉的路径有五条以上，
  漏一条就会长出错误的「返回上一层」。

- **`App.SetDMABUFRenderer` 必须在 `mygo.App.Run()` 之前调**（`main.go` 末尾、
  `setupAppMenu()` 旁）：框架在 `Run → 后端 Init` 里据此设
  `WEBKIT_DISABLE_DMABUF_RENDERER`，之后再改字段没有任何地方会读。写在
  `WhenReady` 里是**静默失效**（编译/测试全绿、能力为零）。护栏见
  `TestDMABUFRendererSetBeforeRun`。

- **`playIndex` 的选集分支必须「先起播、再关面板」**：`startCurrent` 按
  `keepDrawerOnPlay` 决定抽屉去留，先关会把从列表恢复的抽屉再关掉（用户点一集
  播放后落回主区）。回归测试用 `startPlaybackStub` 做行为断言，别改回源码 grep。

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

- **三平台字体已统一**（2026-10，`app/internal/view/font.go` 的 `fontStack`）：
  拉丁用内置 Inter 子集（SIL OFL，`app/internal/view/fonts/`，注册在子包
  `fonts` 的 `init` 里 —— 必须早于 view 的包级变量初始化，那些度量常量在
  var 阶段就算好了；写进 view 自己的 init 会晚一步、冻住系统字体度量）。
  中文按 PingFang SC → Microsoft YaHei UI → Noto Sans CJK SC 回退（macOS 的
  PingFang 是基准）。三条硬规矩：
  1. 量宽 / 自绘成型一律用 `fontOf(size, weight)`，别再写裸 `ui.Font{Size: n}`
     （不带 Family 会悄悄落回系统字体，量出来的和画出来的不是同一个字体）；
  2. 中文行盒 ≠ 拉丁行盒（14px：中文 22、Inter 18）。固定高度容器里放中文
     （桌面字幕窗、多行标题逐行落点）用 `cjkLineBox` / `cjkTextLineBox`，
     按拉丁度量算会裁字；
  3. 改主题字体后，无头测试的 tester 要像生产一样 `c.SetTheme(a.Theme.uiTheme())`，
     否则渲染用系统字体、期望值用 Inter 度量，测试会假红；
  4. 同一行里混排拉丁与中文（标题栏品牌/设置、选集胶囊）要给**共用行盒**：
     `textLineBoxFor(s, size)` 量每段文字，取 `max(自然行盒)` 当盒高、各自补
     半个差值，否则盒顶会差 1.5px。
- **桌面字幕窗必须是 Panel**（`app/lyrics.go`）：只有面板窗保证「压在所有窗口
  之上、跨 Space / 虚拟桌面、全屏应用也压得住」。普通窗口的 `AlwaysOnTop` 各
  平台都只是尽力：macOS 只到 floating level（切 Space 就没了）、Wayland 上
  `keep_above` 是空操作、Windows 上非 topmost 窗口随便盖。副作用：Wayland 上
  有 gtk-layer-shell 的合成器（Sway/Hyprland/KDE/COSMIC）里它是 overlay 层
  surface，`gtk_window_begin_move_drag` 不生效 —— 该环境下拖不动窗口（位置仍
  由 KV 记忆）；GNOME 没有 layer-shell，仍是普通窗口、照常可拖但压不住别人。
- **Windows 桌面字幕的「边框/阴影感」有两个独立来源**，只关一个不够：
  1. **内容层**：字幕面板铺满整个窗口，给它挂外投影 `Shadow(...)` 时，被窗口
     裁掉的那一半会落在窗口内侧，四边各出现一圈暗边。面板一律不画外投影；
  2. **DWM**：Win11 给每个顶层窗口画 1px 边框（`DWMWA_BORDER_COLOR`），mygo 又
     对 Panel 请求了圆角偏好，于是 DWM 按圆角裁剪窗口并沿它描边 —— 与内容的
     14px 圆角不重合。`app/lyrics_windows.go` 用 `NativeHandle()` 直接调
     `DwmSetWindowAttribute`：边框 `DWMWA_COLOR_NONE`(0xFFFFFFFE，**不是 0**)、
     圆角 `DoNotRound`。窗口级 `DisableShadow` 只关掉 mygo 自己开的那层投影。
  系统投影由 `syncLyricsWindowShadow` 按 `LyricsBgOpacity > 0` 实时开关。

- **发布产物命名**：mygo 0.7.0 起磁盘镜像与 Windows 安装器**按 `slug` 命名**
  （`bili-fm-setup-<ver>-<arch>.exe`），0.6.x 是 `bili-FM Setup <ver> <arch>.exe`。
  `release-mygo.yml` 收集时要两代都匹配，且**找不到就 exit 1** —— 静默漏传会
  连带 store / Tauri 迁移 job 硬失败。可执行文件仍按 `name`（`bili-FM.exe`）。
  `mygo.json` 里的 `slug` 已显式钉成 `bili-fm`，别删。
- **Linux Wayland 无边框窗**：0.7.0 起框架给 Frameless 主窗加 GTK CSD
  （投影 + 描边，mygo #727）。透明窗（桌面字幕）、panel、X11 不受影响。
- **B 站音频编码不统一（HE-AAC）**：360P 合流（`platform=html5` 的 durl）在
  部分稿件里是 HE-AAC（`mp4a.40.5`，ASC AOT 5 / SBR），纯 Go 的 go-aac 只做
  AAC-LC，起播必报 `ErrUnsupportedSBR`（别试「当 LC 解」：SBR 在 fill element
  里，库会主动拒绝）。同一集 DASH 通常还有 AAC-LC 音轨（30232/30280，
  `mp4a.40.2`）。`media.ErrHEAAC` → `main.startCurrent` 静默换
  `GetDashAudioByCid` 重试，详见 PLAN 的「HE-AAC 稿件」一节。
- **DASH 音轨是 fMP4，且带 sidx**：`internal/media/fmp4.go` 直接读
  ftyp/moov/sidx，按 sidx 的分片按需拉 moof/mdat（起播只下前几十 KB），
  **不要**在新代码里对 DASH 音轨调 `m4a.NewReader` —— 它会扫完整条音轨的
  moof，1 小时约 75MB，慢网就是「一直起播中 / 解析失败」（2026-11-02 的
  bug）。只有无 sidx 的流才退回 go-m4a；`pipeline.fragmented` 决定 seek 是否
  原地，B 站 DASH 的 ftyp brand 常是 `isom`，只看 brand 会误判成普通 MP4。

- **CSP 已不存在**（原生版没有 webview 安全策略），旧 Tauri 的 CSP/WebView2 坑只作历史参考。
- **SponsorBlock（bsbsb.top）**：免费无 key，`GET /api/skipSegments?videoID=BV..&cid=..`。会话缓存只能存真实服务端结论，abort/超时/网络失败不得写入。
- **mini-mode**：state 是唯一事实源，`Act.SetMini` 是唯一写入口；布局/命中测试都要区分主窗与 mini。
- 原生 UI 画不了 `backdrop-filter`：玻璃用「半透明填充 + 1px 亮边 + 阴影」近似，窗口材质用 mygo `Vibrancy`。

## 硬性约束（每份 spec 都带）

- 禁止启动应用、禁止 GUI 自动化（研究靠读代码 + 无头 `-shot` 截图）。
- **升级依赖时 `GOPROXY=off` 不够**：新版本会带来缓存里没有的新依赖，`go mod tidy`
  会直接失败；给那一条命令临时加代理（如 `GOPROXY=https://proxy.golang.org,direct
  go mod tidy`），**不要全局 export** —— pi 的模型请求走本地网关，全局代理会把它掐死。
- **外观偏好落盘的是档位名**（`light` / `dark` / `auto` / `system`），**永远不落解析后的
  深浅**：落盘 light/dark 等于把「跟随系统」（或按时段）冻结成那一刻的值，重启后系统
  改了界面不再跟（`auto` 就是这么定的，`system` 同一条规矩）。
- **「跟随系统」只读不写**：只用 `mygo.Theme.IsDark()` 读值 + `mygo.Theme.OnUpdated`
  订阅变化，**不调 `SetSource`** —— 那会把 `IsDark()` 变成「本应用自己的选择」，
  「跟随系统」名存实亡。
- **输入区域 / 穿透这类平台实现替换必须保留回退**：新 API 不可用时（库缺失、窗口
  未 realize、后端没实现）要退回旧实现（如 `SetInputRegion` → 整窗 `SetIgnoreMouseEvents`
  + 指针轮询），用纯函数选路（`lyricsClickThroughRouteFor`）把「走哪条」做成可无头断言的两态。
- 不新增依赖需克制；验证 = 仓库根的 **`./check.sh`**（vet + 全包测试 +
  linux/windows/darwin × amd64/arm64 六平台编译，与 CI 门禁同口径；`./check.sh fast`
  只跑 vet+test）。改动 `internal/view` 另用 `-shot` 截图对照。
- `internal/view` 的像素测试以 **Noto Sans CJK SC** 为参考中文字体（见 font.go
  的 fontStack）：缺字会退成豆腐块，几十条行盒/墨迹/量宽断言集体假红。
  `build-check.yml` 因此先装 `fonts-noto-cjk` 再跑测试（ubuntu-26.04 不自带）；
  本机缺这套字体时不要相信那些红。时间/日期格式化的测试**必须自己钉时区**
  （CI 的 runner 是 UTC，本机是 CST），别依赖 `time.Local`。

## 用户裁决记录

- 永久否决：关窗行为选项、主题跟随系统、歌单导入导出。
- SponsorBlock 默认关，mini 模式不显示跳过入口（后台跳过仍生效）。
- 更新记录：`.pending-bugs.md`（每轮 commit 后追加一行）。

## 进度条对齐（2026-10-15）

- 原版 `tauri:src/styles/globals.css`（3067-3277 区间）/ mygo `app/internal/view/rangeslider.go` + `app/internal/view/playerbar.go` 进度条区域。
- 修复 3 项差异：① 禁用态 opacity 从 0.5 对齐到 0.42（`disabledDim` 抵消框架减半）；② 滑块 scale 从瞬时改为 100ms ease 过渡（`AnimateTo`）；③ 滑块高光 alpha 亮色 0.86→0.98、深色 0.78→0.90。
- 已知未对齐：Seek bubble 在触摸设备（`@media (hover:none), (pointer:coarse)`）原版 `display:none`，mygo 无法检测指针类型。
- volume thumb 圆心 1px 偏移（原版 `margin-top:-5px`）在 mygo 中已修正为居中，不回退。
