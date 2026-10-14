// bili-FM 的原生重写（mygo）。
//
// 结构：
//
//	internal/bilibili  B 站 API 客户端（自 Wails 版 service/bl.go 移植 + Rust 版补充）
//	internal/store     本地键值存储（与旧版 dkv 同格式，登录态无缝迁移）
//	internal/media     音频引擎（AAC 解码 + WSOLA 变速 + 均衡 + oto 输出，纯 Go）
//	internal/proxy     本地 http 代理（给视频弹窗用，127.0.0.1:4654）
//	internal/imagecache 封面抓取与缓存（原生界面直连，不走代理）
//	internal/video     视频弹窗（临时的 webview 窗口）
//	internal/view      原生界面（时段背景 + 玻璃令牌 + 主窗口）
//
// 视频为什么单独开 webview：H.264 没有可用的纯 Go 解码器。音频不依赖
// webview，全走纯 Go 引擎，所以只听音频时一个 webview 都不用起（实测
// Windows 上原生主窗 57MB，开媒体窗口 506MB，关掉回落到 69MB）。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
	"github.com/vst93/bili-fm/app/internal/mediactl"
	"github.com/vst93/bili-fm/app/internal/proxy"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

const audioUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// 主窗口尺寸。旧版默认就是 800×600（src-tauri/src/lib.rs 的 inner_size），
// 整套布局（搜索药丸 520、封面圆盘 304、播放栏 56）都是按这个宽度调的。
const (
	mainWidth  = 800
	mainHeight = 600
)

var audioHeaders = map[string]string{
	"User-Agent": audioUA,
	"Referer":    "https://www.bilibili.com/",
}

type controller struct {
	bl  *bilibili.BL
	kv  *store.KV
	app *view.App
	mp  *media.Player
	vid *video.Manager
	pxy *proxy.Server
	// media 是系统媒体中心（Linux MPRIS；其他平台 no-op）。
	media mediactl.Controller

	// 当前曲目的跳过分段
	sponsor   []bilibili.SponsorSegment
	skipped   map[int]bool
	sponsorOn bool
	lastPos   float64

	// quitting 为真表示真的在退出（而不是关窗到托盘）。
	quitting bool

	// loginGen 是登录轮询的代号：关闭面板/重新登录会让旧的轮询自尽。
	loginGen int

	// 进度同步的节流时间点（见 progress.go）。
	resumeWroteAt time.Time
	reportedKey   string
	reportedAt    time.Time

	// lastSponsorToast 是「已跳过恰饭片段」toast 的频控时间点。
	lastSponsorToast time.Time

	// miniPoll 是迷你窗位置轮询的停止信号（mygo 没有 window move 事件）。
	miniPoll chan struct{}
	// videoSilent 在「静默关弹窗」（切歌/退出）时置位，让 OnClose 不要接回音频。
	videoSilent bool
}

func main() {
	shot := flag.String("shot", "", "启动后截图到该路径并退出")
	shotDelay := flag.Duration("shot-delay", 3*time.Second, "截图前的等待时间")
	searchOnStart := flag.String("search", "", "启动后立刻搜索该关键词")
	sectionOnStart := flag.String("section", "popular", "启动后加载的抽屉（feed/popular/favorite/history，旧名 recommend/watchlater 仍可用）")
	noTray := flag.Bool("no-tray", false, "不装系统托盘（无头调试用）")
	noKeys := flag.Bool("no-keys", false, "不注册全局媒体键（无头调试用）")
	startMini := flag.Bool("mini", false, "以迷你模式启动（无头调试用）")
	loadInfo := flag.Bool("load-info", false, "启动后拉取当前队列第一条的详情（调试主区右栅用）")
	drawerOnStart := flag.String("drawer", "", "启动后直接打开某个抽屉（调试用）")
	winPos := flag.String("window-pos", "", "窗口位置 X,Y（调试用；不填则居中）")
	seriesOnStart := flag.Int64("series", 0, "启动后直接打开这个合集 id（调试用）")
	modalOnStart := flag.String("modal", "", "启动后直接打开某个对话框（调试用：about/shortcuts/login）")
	toastOnStart := flag.String("toast", "", "启动后弹一个 toast（调试用）")
	playOnStart := flag.Int("play", -1, "列表加载后自动播放第 N 条（调试用）")
	addAllOnStart := flag.Bool("addall", false, "加载详情后把全部分集加入播放列表（调试用）")
	videoOnStart := flag.Bool("video", false, "起播后自动打开视频弹窗（调试用）")
	foldersOnStart := flag.Int("folders", 0, "注入 N 个假收藏夹并打开收藏抽屉（调试用）")
	playlistOnStart := flag.Int("playlist", 0, "注入 N 条假播放列表并打开播放列表抽屉（调试用）")
	danmakuOnStart := flag.Int("danmaku", 0, "注入 N 条假弹幕并打开弹幕抽屉（调试用）")
	repliesOnStart := flag.Int("replies", 0, "注入 N 条假评论并打开评论 tab（调试用）")
	sponsorOnStart := flag.Int("sponsor", 0, "注入 N 段假广告段标记（调试用）")
	popOnStart := flag.String("pop", "", "启动后弹出播放栏弹层（调试用：speed/volume）")
	flag.Parse()

	// 单实例：第二个实例会把参数交给第一个（并自己退出），第一个把主窗调到前台。
	// 与旧版（Tauri 的 single-instance 插件）一致。
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}

	kv, err := store.OpenDefault()
	if err != nil {
		log.Fatalf("打开本地存储失败: %v", err)
	}
	log.Printf("%s", kv.Describe())
	bilibili.UseStore(kv)

	// 旧版（Wails / Tauri）迁移：键名改了名的配置（sponsor/incognito/播放模式）
	// 与换了位置的收藏（dkv playlist / Tauri playlist.json）在这里一次性转换。
	// 幂等，且不会覆盖新版已有的数据。
	store.Migrate(kv, store.MigrateOptions{Log: log.Printf})

	// 应用内更新：mygo build 用 mygo.json 里 updates 的私钥给每个平台的归档
	// 签名并写 update-<target>.json；应用用内置的公钥验签。
	//
	// 注意哪些安装方式能自更新：macOS 的 /Applications、Windows 的
	// %LOCALAPPDATA%\Programs、Linux 的 ~/.local/<app>.app 可以；
	// 由包管理器装的（deb/rpm/pacman 装到 /opt）不行 —— Updater.Enabled()
	// 会返回 false，用户点检查更新时会看到原因。
	mygo.Use(updater.Plugin)

	c := &controller{
		bl:      bilibili.NewBL(),
		kv:      kv,
		mp:      media.NewPlayer(),
		vid:     video.New(),
		skipped: map[int]bool{},
	}
	video.SetLog(func(s string) { log.Print(s) })

	// 本地代理：视频弹窗里的 <video> 带不了 Referer，必须经它转发。
	c.pxy = proxy.New()
	if err := c.pxy.Start(); err != nil {
		log.Printf("代理启动失败（视频弹窗可能不可用）: %v", err)
	}

	var app *view.App
	app = view.NewApp(func() {
		if app.Win != nil {
			app.Win.Update(func() {})
		}
	})
	c.app = app
	app.UName = kv.String("uname")
	app.Face = kv.String("face")
	app.LoggedIn = bilibili.LoginStatus || app.UName != ""
	app.Speed = 1
	app.Volume = 1
	app.Sponsor = kv.String("sponsor_skip") == "1"
	c.sponsorOn = app.Sponsor
	if app.Sponsor {
		app.SponsorStatus = "loading"
		app.SponsorCount = 0
	} else {
		app.SponsorStatus = "off"
	}
	// 隐身模式（原版 localStorage 的 incognitoMode）：不读也不写云端进度。
	app.Incognito = kv.String("incognito") == "true"
	// 显示偏好（倍速/音量/均衡/封面模式/氛围光/高级质感）。
	c.loadPrefs()
	c.applyVolume()
	// 播放列表与播放模式。
	c.loadPlaylists()

	c.wireActions()
	c.wirePlayer()
	c.wireVideo()
	c.setupMediaCenter()

	// 视频弹窗页面用 Video.* 调这些方法，所以要显式绑定成 Video（类型名默认是 Service）。
	mygo.BindAs("Video", video.NewService(c.vid))

	mygo.App.WhenReady(func() {
		opts := mygo.WindowOptions{
			Title: "bili-FM",
			// 旧版默认窗口就是 800×600（src-tauri/src/lib.rs 的 inner_size）。
			Width:     mainWidth,
			Height:    mainHeight,
			MinWidth:  400,
			MinHeight: 155,
			// 旧版是固定窗口（resizable(false)），布局就是按 800×600 调的；
			// 迷你模式是自己改尺寸，不受这个限制。
			DisableResize: true,
			// 保留原生窗口控件（macOS 的红绿灯、Windows/Linux 的标题栏按钮），
			// 页面自己画标题栏 —— 旧版在 macOS 上用的就是这种（Overlay）。
			TitleBarStyle: mygo.TitleBarHidden,
			Content:       ui.View(app.Shell),
		}
		// 调试用：把窗口摆到指定位置，避免它刚好落在鼠标下面（截图时
		// 会偶发误触，让抽屉自己开合）。
		if *winPos != "" {
			if x, y, ok := strings.Cut(*winPos, ","); ok {
				opts.X, _ = strconv.Atoi(x)
				opts.Y, _ = strconv.Atoi(y)
			}
		}
		app.Win = mygo.NewWindow(opts)

		// 第二个实例启动时把主窗显示并聚焦。
		mygo.App.OnSecondInstance(func(args []string, workingDir string) {
			app.Win.Show()
			app.Win.Focus()
		})

		// 默认进「热门与推荐」的热门 tab：未登录时推荐/动态/收藏/历史都是空的。
		// 旧版的「推荐」「稍后再看」现在是抽屉里的 tab，这里保留旧写法做兼容。
		start := *sectionOnStart
		switch start {
		case "recommend":
			start = "popular"
			app.RecTab = view.RecRecommend
		case "watchlater":
			start = "history"
			app.HistTab = view.HistWatchLater
		}
		app.Section = sectionIndex(start)
		if *startMini {
			c.setMini(true)
		}
		if *searchOnStart != "" {
			app.Query = *searchOnStart
			c.search(*searchOnStart)
		} else if start == "favorite" {
			// 收藏要先拉收藏夹列表，再拉第一个收藏夹的内容。
			c.loadFolders()
		} else {
			c.loadSection(start, 1)
		}

		// 调试用：注入假收藏夹，检查收藏抽屉表头的横向滚动。
		if *foldersOnStart > 0 {
			list := make([]view.Folder, *foldersOnStart)
			for i := range list {
				list[i] = view.Folder{ID: int64(i + 1), Title: fmt.Sprintf("收藏夹 %d", i+1), Count: int64(i*7 + 3)}
			}
			app.Folders = list
			app.FolderID = list[0].ID
			app.Section = sectionIndex("favorite")
			app.Drawer = "favorite"
			app.Win.Update(func() {})
		}

		// 调试用：注入假播放列表，检查拖拽排序的行布局。
		if *playlistOnStart > 0 {
			items := make([]view.PlayItem, *playlistOnStart)
			for i := range items {
				items[i] = view.PlayItem{
					ID:    fmt.Sprintf("fake-%d", i),
					Bvid:  "BVfake",
					Part:  fmt.Sprintf("第 %d 集", i+1),
					Title: fmt.Sprintf("假视频标题 %d", i+1),
				}
			}
			app.Playlist = items
			app.PlaylistTab = view.ListUser
			app.Drawer = view.DrawerPlaylist
			// 让某一条成为「当前播放」以检查选中项布局（-play 指定下标）。
			if *playOnStart >= 0 && *playOnStart < len(items) {
				app.PlayingPlaylist = view.ListUser
				app.Index = *playOnStart
				app.Playing = true
			}
			app.Win.Update(func() {})
		}

		// 调试用：注入假弹幕，检查虚拟化列表。
		if *danmakuOnStart > 0 {
			dms := make([]view.Danmaku, *danmakuOnStart)
			for i := range dms {
				dms[i] = view.Danmaku{Time: float64(i) * 1.5, Text: fmt.Sprintf("这是一条弹幕 %d", i+1), Color: 0xff0000}
			}
			app.Danmaku = dms
			app.DanmakuTab = view.TabDanmaku
			app.Drawer = view.DrawerDanmaku
			app.Win.Update(func() {})
		}

		// 调试用：注入假广告段，检查进度条上的分段标记。
		if *sponsorOnStart > 0 {
			list := make([]view.Segment, *sponsorOnStart)
			for i := range list {
				start := float64(i)*97 + 40
				list[i] = view.Segment{Start: start, End: start + 55}
			}
			app.SponsorSegments = list
			app.Sponsor = true
			app.SponsorStatus = "ok"
			app.SponsorCount = len(list)
			app.SponsorReason = ""
			// 无头环境里媒体不会真的加载，Dur 一直是 0 —— 标记按比例画，
			// 所以给个假总时长才能看到效果。
			app.Dur = 600
			app.Win.Update(func() {})
		}

		// 调试用：注入假评论，检查评论 tab 的排版。
		if *repliesOnStart > 0 {
			cms := make([]view.Comment, *repliesOnStart)
			for i := range cms {
				cms[i] = view.Comment{
					User:     fmt.Sprintf("用户%d", i+1),
					Content:  fmt.Sprintf("这是第 %d 条评论的内容，用来检查排版与换行是否正常。", i+1),
					Likes:    int64(i * 7),
					Level:    (i % 6) + 1,
					SendTime: int64(1700000000 - i*3600),
					Time:     fmt.Sprintf("2026-10-%02d 12:%02d", (i%28)+1, i%60),
				}
				if i%3 == 0 {
					cms[i].Replies = []view.Comment{
						{User: "回复者A", Content: "楼中楼第一条回复。", SendTime: cms[i].SendTime + 60},
						{User: "回复者B", Content: "楼中楼第二条回复，稍微长一点看看换行。", SendTime: cms[i].SendTime + 120},
					}
				}
			}
			app.Comments = cms
			app.RepliesHasMore = true
			app.DanmakuTab = view.TabReply
			app.Drawer = view.DrawerDanmaku
			app.Win.Update(func() {})
		}

		// 调试用：把队列第一条的详情拉下来填主区右栏（不真的播放）。
		// 打开抽屉要等详情回来（要知道 UP 的 mid），所以放在一起。
		if *loadInfo && app.Track != nil {
			t := *app.Track
			go func() {
				info := c.videoInfoOf(t.Bvid, &t)
				if info == nil {
					return
				}
				// 抽屉要在 Info 已经写进去之后再开（Win.Update 是排队到
				// 主线程执行的，放在同一个回调里才有顺序保证）。
				app.Win.Update(func() {
					app.Info = info
					if *addAllOnStart {
						c.addAllToPlaylist()
					}
					if *drawerOnStart != "" {
						c.debugOpenDrawer(*drawerOnStart, *seriesOnStart)
					}
				})
			}()
		} else if *drawerOnStart != "" {
			c.debugOpenDrawer(*drawerOnStart, *seriesOnStart)
		}

		// 调试用：直接弹一个对话框 / toast（截图对照用）。
		if *modalOnStart != "" {
			switch *modalOnStart {
			case "about":
				c.showAbout()
			case "shortcuts":
				c.showShortcuts()
			case "login":
				c.login()
			}
		}
		if *toastOnStart != "" {
			app.Notify(*toastOnStart)
		}
		switch *popOnStart {
		case "speed":
			app.ShowSpeed = true
		case "volume":
			app.ShowVolume = true
		}
		if *popOnStart != "" {
			app.Win.Update(func() {})
		}

		// 调试用：列表加载后自动播放第 N 条（等列表就绪）。
		if *playOnStart >= 0 {
			done := make(chan struct{}, 1)
			go func() {
				t := time.NewTicker(200 * time.Millisecond)
				defer t.Stop()
				for i := 0; i < 60; i++ {
					<-t.C
					app.Win.Update(func() {
						key := app.Drawer
						if key == "" {
							key = app.CurrentSection().Key
						}
						if len(app.ListFor(key).Cards) > *playOnStart {
							c.playIndex(*playOnStart)
							if *videoOnStart {
								time.AfterFunc(6*time.Second, func() {
									app.Win.Update(func() { c.openVideo() })
								})
							}
							select {
							case done <- struct{}{}:
							default:
							}
						}
					})
					select {
					case <-done:
						return
					default:
					}
				}
			}()
		}

		// 关闭窗口 = 隐藏到托盘（与旧版一致），从托盘或 Dock 恢复。
		//
		// 真正退出时不能拦：App.Quit() 的流程是「先问每个窗口能不能关，
		// 都同意才退」，而这里把「用户点关闭键」解释成隐藏并 PreventDefault，
		// 于是退出会被取消 —— 之前设置菜单的「退出应用」和托盘的
		// RoleQuit 都因此无效。
		//
		// 统一在 OnBeforeQuit 置位：它在「问窗口」**之前**触发，所以
		// 不管退出是谁发起的（菜单、托盘 Role、Cmd+Q、系统关机）都能生效。
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { c.quitting = true })
		app.Win.OnClose(func(e *mygo.CloseEvent) {
			if c.quitting {
				return
			}
			e.PreventDefault()
			app.Win.Hide()
		})

		if !*noTray {
			c.setupTray()
		}
		if !*noKeys {
			c.setupMediaKeys()
		}

		// 时段变化时换主题（原版也按小时切背景）。
		go func() {
			for range time.Tick(time.Minute) {
				before := app.Theme.Period.Name
				app.RefreshTheme()
				if app.Theme.Period.Name != before {
					app.Win.Update(func() {})
				}
			}
		}()

		if *shot != "" {
			go func() {
				time.Sleep(*shotDelay)
				png, err := app.Win.CapturePage()
				if err != nil {
					log.Printf("截图失败: %v", err)
				} else if err := os.WriteFile(*shot, png, 0o644); err != nil {
					log.Printf("写截图失败: %v", err)
				} else {
					log.Printf("截图已写入 %s (%d 字节)", *shot, len(png))
				}
				c.quit()
			}()
		}
	})

	// macOS 原生菜单栏（编辑 / 设置），对齐 Tauri 原版；其他平台是 no-op。
	// 在 Run 之前装：这样 mygo 就不会先装上它自带的英文默认菜单
	// （App / File / Edit / View / Window）。
	c.setupAppMenu()

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// debugOpenDrawer 打开一个抽屉（只给 -drawer / -series 用）。
func (c *controller) debugOpenDrawer(kind string, seriesID int64) {
	a := c.app
	switch {
	case kind == view.DrawerUp, kind == view.DrawerSeries:
		if a.Info == nil || a.Info.OwnerMid == 0 {
			log.Printf("调试：没有 UP mid，打不开 %q", kind)
			return
		}
		if kind == view.DrawerSeries {
			a.UpTab = view.UpTabSeries
		}
		c.openUp(a.Info.OwnerMid, a.Info.OwnerName)
		if seriesID != 0 {
			a.Drawer = view.DrawerSeries
			c.selectSeries(seriesID)
		}
	case func() bool { i := view.SectionIndex(kind); return view.Sections[i].Key == kind }():
		a.Section = view.SectionIndex(kind)
		a.Drawer = kind
		c.reload()
	default:
		a.Drawer = kind
	}
	a.Win.Update(func() {})
}

// ---------------------------------------------------------------- 界面动作

func (c *controller) wireActions() {
	a := c.app
	a.Act = view.Actions{
		LoadSection:     c.loadSection,
		Reload:          c.reload,
		SelectFolder:    c.selectFolder,
		OpenUp:          c.openUp,
		SelectSeries:    c.selectSeries,
		ToggleFollow:    c.follow,
		ToggleIncognito: c.toggleIncognito,
		Search:          c.search,
		UrlJump:         c.urlJump,
		OpenCard:        c.openCard,
		OpenBrowser:     c.openBrowser,
		CopyLink:        c.copyLink,
		Play:            c.playIndex,
		TogglePlay:      func() { dbgSlow("togglePlay", c.togglePlay) },
		Next:            func() { dbgSlow("next", func() { c.step(1) }) },
		Prev:            func() { dbgSlow("prev", func() { c.step(-1) }) },
		Seek:            func(v float64) { dbgSlow("seek", func() { c.seek(v) }) },
		SetSpeed:        c.setSpeed,
		ToggleEQ:        func() { dbgSlow("toggleEQ", c.toggleEQ) },
		ToggleSponsor:   c.toggleSponsor,
		SetVolume:       c.setVolume,
		PersistVolume:   c.persistVolume,
		ToggleMute:      c.toggleMute,
		OpenVideo:       c.openVideo,
		CloseVideo:      c.closeVideo,
		OpenParts:       c.openParts,
		ToggleDanmaku:   c.toggleDanmaku,
		LoadMore:        c.loadMore,
		Login:           c.login,
		CloseLogin:      c.closeLogin,
		ShowAbout:       c.showAbout,
		ShowShortcuts:   c.showShortcuts,
		CheckUpdate:     c.checkUpdate,
		CloseModal:      c.closeModal,
		// 主区的点赞 / 投币 / 收藏三个按钮走这里。漏接的话按钮看着能点、
		// 点了却什么都不发生（Act 为 nil，statButton 会静默跳过）。
		Like:          c.like,
		Coin:          c.coin,
		Favorite:      c.favorite,
		SetCoverMode:  c.setCoverMode,
		ToggleAmbient: c.toggleAmbient,
		TogglePremium: c.togglePremium,
		// 从播放列表抽屉里点某一行：播完保持抽屉打开（要接着挑）。
		PlayPlaylist:       func(list string, index int) { c.playPlaylist(list, index, true) },
		DeletePlaylistItem: c.deletePlaylistItem,
		ReorderPlaylist:    c.reorderPlaylist,
		ClearPlaylist:      c.clearPlaylist,
		CyclePlayMode:      c.cyclePlayMode,
		SwitchPlaylistTab:  c.switchPlaylistTab,
		AddToPlaylist:      c.addToPlaylist,
		AddAllToPlaylist:   c.addAllToPlaylist,
		SeriesPlayAll:      c.seriesPlayAll,
		SwitchDanmakuTab:   c.switchDanmakuTab,
		LoadComments:       c.loadComments,
		RemoveWatchLater:   c.removeWatchLater,
		SetMini:            c.setMini,
		TogglePin:          c.togglePin,
		Quit:               c.quit,
		Minimize:           func() { a.Win.Minimize() },
	}
}

func (c *controller) wirePlayer() {
	// 这三个回调都跑在播放器的上报 goroutine 上，而它们读写的是界面状态，
	// 所以统一丢到 Win.Update（主线程）里执行，避免数据竞争。
	c.mp.OnProgress(func(pos, dur float64) {
		c.app.Win.Update(func() {
			// 拖动中：跳过全部进度副作用（原版 handleTimeUpdate 里
			// isSeekingRef 直接 return 的语义）——只记 Pos，
			// 不让 DBus/KV 写和 sponsor 判定挤占拖动的帧。
			if c.app.Seeking() {
				c.app.Pos, c.app.Dur = pos, dur
				return
			}
			c.app.Pos, c.app.Dur = pos, dur
			// 缓冲段（原版 audio.buffered）：只读原子量，不会阻塞主线程。
			c.app.Buffered = c.mp.Buffered()
			c.syncMediaPosition(pos)
			c.maybeSkipSponsor(pos)
			c.trackResume(pos) // 本地断点 5s 落盘 + 云端 30s 上报
		})
	})
	c.mp.OnEnded(func() {
		c.app.Win.Update(func() { c.handleEnded() })
	})
	c.mp.OnError(func(err error) {
		log.Printf("播放错误: %v", err)
		c.app.Win.Update(func() {
			c.app.Playing = false
			c.app.Buffering = false
			// 原版 <audio> 的 error 事件会弹一条 toast（player.tsx 的 onError）。
			// 此前原生版只写日志：用户看到的就是「放着放着停了」，进度条不动、
			// 也没有任何提示，只能自己去翻日志。
			c.app.NotifyType("error", audioErrorMessage(err))
		})
	})
}

// audioErrorMessage 把一次播放失败翻成给用户看的文案，对齐原版 player.tsx
// onError 里按 MediaError.code 选文案的那条 if 链：
//
//	4（MEDIA_ERR_SRC_NOT_SUPPORTED）→ 音频格式不支持或加载失败
//	3（MEDIA_ERR_DECODE）           → 音频解码失败
//	2（MEDIA_ERR_NETWORK）          → 网络错误，音频加载失败
//	其它                            → 音频加载出错
//
// 原生版的分类由 internal/media 的错误种类给出（见 media.Kind），不去猜
// 错误字符串。
func audioErrorMessage(err error) string {
	switch media.KindOf(err) {
	case media.KindNetwork:
		return "网络错误，音频加载失败"
	case media.KindDecode:
		return "音频解码失败"
	case media.KindUnsupported:
		return "音频格式不支持或加载失败"
	default:
		return "音频加载出错"
	}
}

// handleEnded 处理自然播完（主线程）。
func (c *controller) handleEnded() {
	// 单曲循环：直接重播当前曲目（不管选集/列表）。
	if c.app.PlayMode == view.PlayModeSingle {
		c.startCurrent()
		return
	}
	// 播放列表模式：在来源列表里续播。
	if c.app.PlayingPlaylist != "" {
		c.step(1)
		return
	}
	// 普通队列：还有下一条就继续，否则停。
	if len(c.app.Queue) > 1 {
		c.step(1)
		return
	}
	c.app.Playing = false
	c.app.Buffering = false
	c.syncMediaTrack()
}

// quit 真正退出应用：先把状态写盘，再关掉弹窗与播放器。
func (c *controller) quit() {
	c.quitting = true
	c.flushProgress(c.app.Pos) // 退出前把断点/进度补齐
	if c.media != nil {
		c.media.Close()
	}
	c.vid.Close()
	c.mp.Close()
	mygo.App.Quit()
}

// loadInteractionState 拉当前视频的互动状态（点赞 / 投币 / 收藏 / 关注）。
// 起播时调一次，让主区的按钮显示正确的激活态。
func (c *controller) loadInteractionState(info *view.Info) {
	if info == nil || info.Bvid == "" {
		return
	}
	bvid, aid, upMid := info.Bvid, info.Aid, info.OwnerMid
	// 请求代号：切歌后旧响应不得覆盖新视频的状态（原版 favoriteRequestIdRef）。
	c.app.InteractionReqID++
	reqID := c.app.InteractionReqID
	// 原版在 bvid/aid 变化时先把收藏状态置为「请求中」（isFavoriteUpdating），
	// 期间收藏键禁用 —— 否则会用上一条视频的收藏态去点，点到错的方向。
	c.app.FavPending = aid != 0
	c.app.Win.Update(func() {})
	go func() {
		liked, _ := c.bl.HasLiked(bvid)
		coined := false
		if n, err := c.bl.HasCoin(bvid); err == nil && n > 0 {
			coined = true
		}
		faved, _ := c.bl.HasFavorite(aid)
		followed := false
		// 注意是视频的 UP 主（不是登录用户自己）。
		if upMid != 0 {
			if fs, err := c.bl.IsFollowing(int(upMid)); err == nil && fs != nil {
				followed = fs.IsFollowing
			}
		}
		c.app.Win.Update(func() {
			if c.app.InteractionReqID != reqID {
				return // 已经切到下一条了，旧结果丢掉
			}
			c.app.Liked, c.app.Coined, c.app.Faved, c.app.Followed = liked, coined, faved, followed
			c.app.FavPending = false
		})
	}()
}

// loadComments 拉一页评论追加到列表（原版回复分页）。page 从 1 开始。
func (c *controller) loadComments(page int) {
	a := c.app
	t := a.Current()
	if t == nil {
		return
	}
	a.RepliesLoading = true
	a.Win.Update(func() {})
	aid := t.Aid
	go func() {
		r, err := c.bl.GetReplyList(aid, page)
		a.Win.Update(func() {
			a.RepliesLoading = false
			if err != nil || r == nil {
				return
			}
			list := make([]view.Comment, 0, len(r.Items))
			for _, it := range r.Items {
				// 完整日期时间（原版 formatDate：本地格式的 ctime）。
				timeText := time.Unix(it.SendTime, 0).Format("2006-01-02 15:04")
				cm := view.Comment{
					Content:  it.Content.Message,
					Likes:    int64(it.Like),
					Time:     timeText,
					SendTime: it.SendTime,
				}
				if m := toMap(it.Member); m != nil {
					cm.User = pickStr(m, "uname")
					cm.Avatar = pickStr(m, "avatar")
					cm.Level = int(pickInt(m, "level"))
					if li := subMap(m, "level_info"); li != nil {
						if lv := pickInt(li, "current_level"); lv > 0 {
							cm.Level = int(lv)
						}
					}
				}
				// 楼中楼预览只取前 3 条（原版如此），超出显示「查看更多回复...」。
				for i, rp := range it.Replies {
					sub := view.Comment{Content: rp.Content.Message, SendTime: rp.SendTime}
					if m := toMap(rp.Member); m != nil {
						sub.User = pickStr(m, "uname")
						sub.Level = int(pickInt(m, "level"))
					}
					if i < 3 {
						cm.Replies = append(cm.Replies, sub)
					} else {
						cm.RepliesMore = true
					}
				}
				list = append(list, cm)
			}
			// 上限：评论列表走虚拟化 ui.List（见 view.repliesBody），每帧只建可见
			// 那几行；上限现在主要约束内存驻留。超上限时从**头部**释放已读的那批
			//（见 retainComments）。
			a.Comments = retainComments(a.Comments, list, page, maxComments)
			a.ReplyPage = page
			a.RepliesHasMore = r.HasMore
			a.ReplyTotal = r.TotalCount
		})
	}()
}

// retainComments 把新拉到的一页评论并入已有列表，并裁到上限 limit。
//
// 评论按接口给的顺序排列（第 1 页在最前），翻页只往**尾部**追加更靠后的评论；
// 用户从上往下读，头部那批是已经读过的。所以超上限时要从**头部**释放，做成
// 「有界滑动窗口」：保留最近读到的 limit 条，尾部的新页一定进得来。
//
// 早前这里写反了（`a.Comments = a.Comments[:maxComments]`，截尾）：从第 7 页
// （每页 20 条，上限 120）起，每拉一页都会被立刻整页丢掉 —— 列表看上去
// 「加载不动」，而用户此时正停在底部，内容高度没变，自动翻页的「到底了」判据
// 一直成立，于是不停地重拉下一页（请求量随评论数无界增长，且新数据永远看不见）。
// 保留尾部、丢弃头部还与上限的目的自洽：每帧构建的行数恒为 limit。
//
// 返回值在发生丢弃时一定是**新数组**：直接切尾会共享底层数组，被丢掉的头部
// 评论（Comment 里是 string 与 []Comment，都是指针）仍被那块数组引用着，
// 等于没释放。pageNo <= 1 表示重新拉第一页，整表替换。
func retainComments(prev, page []view.Comment, pageNo, limit int) []view.Comment {
	if pageNo <= 1 {
		prev = nil
	}
	total := len(prev) + len(page)
	if total == 0 {
		return nil
	}
	keep := total
	if limit > 0 && total > limit {
		keep = limit
	}
	all := make([]view.Comment, 0, total)
	all = append(all, prev...)
	all = append(all, page...)
	if keep == total {
		return all
	}
	out := make([]view.Comment, keep)
	copy(out, all[total-keep:])
	return out
}

// switchDanmakuTab 切换「弹幕/评论」tab；切到评论时按需拉第一页。
func (c *controller) switchDanmakuTab(tab string) {
	a := c.app
	if tab == view.TabReply && len(a.Comments) == 0 && !a.RepliesLoading {
		c.loadComments(1)
	}
	// 切回弹幕：重新跟随当前时间。
	if tab == view.TabDanmaku {
		a.DanmakuAutoScroll = true
		a.Win.Update(func() {})
	}
}

// toViewSegments 把 bilibili 的跳过分段转成视图用的标记。
func toViewSegments(segs []bilibili.SponsorSegment) []view.Segment {
	out := make([]view.Segment, 0, len(segs))
	for _, s := range segs {
		out = append(out, view.Segment{Start: s.Start, End: s.End, VideoDuration: s.VideoDuration})
	}
	return out
}

// sponsorReason 把查询错误转成状态点 tooltip 里那段失败原因
// （原版 sponsorStatus.reason）。成功 / 无需查询时是空串。
func sponsorReason(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// currentMedia 返回互动操作的目标：优先用主区正在展示的视频（a.Info，和原版
// videoInfo 的 bvid/aid 一致），没有就退回正在播放的那条。
func (c *controller) currentMedia() (bvid string, aid int64) {
	if a := c.app.Info; a != nil && a.Bvid != "" {
		return a.Bvid, a.Aid
	}
	if t := c.app.Current(); t != nil {
		return t.Bvid, t.Aid
	}
	return "", 0
}

// 原版 videoInfo / upVideoList 的点赞、投币、收藏、关注在**成功时也会弹一条
// toast**，类型与文案逐字如下：
//
//	点赞  成功  点赞成功 / 取消点赞成功      （videoInfo handleLike）
//	投币  成功  投币成功                     （videoInfo handleCoin）
//	      信息  已经投过币了（已投过再点）
//	收藏  成功  收藏成功 / 取消收藏成功      （videoInfo handleFavorite）
//	关注  成功  已关注 {UP 名} / 取消关注 {UP 名}（upVideoList）
//
// 原生版此前这些**成功路径全是静默的**（只翻图标 / 改状态），用户点完看不出
// 有没有生效；投币已投过时甚至一声不响（`coin` 直接 return）。失败提示里同样
// 静默的是关注：原来只 log。这里把文案集中成纯函数（同 upFollowSpec /
// followBlocked），便于无头测试钉住，也避免四处硬编码后再跑偏。
const (
	toastLikeOK      = "点赞成功"
	toastUnlikeOK    = "取消点赞成功"
	toastCoinOK      = "投币成功"
	toastCoinAlready = "已经投过币了"
	toastFavOK       = "收藏成功"
	toastUnfavOK     = "取消收藏成功"
)

// likeToastText 按「本次点击之前是否已点过赞」选文案。
func likeToastText(wasLiked bool) string {
	if wasLiked {
		return toastUnlikeOK
	}
	return toastLikeOK
}

// favToastText 按「本次点击之前是否已收藏」选文案。
func favToastText(wasFaved bool) string {
	if wasFaved {
		return toastUnfavOK
	}
	return toastFavOK
}

// followToastText 拼关注 / 取关的成功文案（原版带 UP 名）。
func followToastText(followed bool, name string) string {
	if followed {
		return "已关注 " + name
	}
	return "取消关注 " + name
}

// actionFailText 拼「X失败：原因」的失败提示。mygo 的用户可见失败提示统一用
// 全角冒号（原版是半角 + 空格），这里保持与本工程其余提示一致。
func actionFailText(action string, err error) string {
	return action + "失败：" + err.Error()
}

func (c *controller) like() {
	bvid, _ := c.currentMedia()
	if bvid == "" {
		return
	}
	// 记下点击前的状态：请求回来时 a.Liked 可能已被别的路径改过。
	liked := c.app.Liked
	want := 1
	if liked {
		want = 2 // 2 表示取消点赞
	}
	go func() {
		if _, err := c.bl.LikeVideo(bvid, want); err != nil {
			c.app.NotifyType("error", actionFailText("点赞", err))
			return
		}
		c.app.Win.Update(func() {
			c.app.Liked = !liked
			c.app.NotifyType("success", likeToastText(liked))
		})
	}()
}

// coinOutcome 是点投币键的三种归宿（见 coinOutcomeFor）。
type coinOutcome int

const (
	coinNoop    coinOutcome = iota // 没视频可投：什么都不做
	coinAlready                    // 已经投过币了：弹一条信息提示
	coinRequest                    // 正常投币（一次两币）
)

// coinOutcomeFor 决定点投币键该干什么。原版投币键只按「有没有视频」禁用，
// 已投过币时**仍可点**，点一下弹「已经投过币了」；原生版此前是静默 return，
// 键看着能点却毫无反应。抽出来便于无头测试。
func coinOutcomeFor(bvid string, coined bool) coinOutcome {
	if bvid == "" {
		return coinNoop
	}
	if coined {
		return coinAlready
	}
	return coinRequest
}

func (c *controller) coin() {
	bvid, _ := c.currentMedia()
	switch coinOutcomeFor(bvid, c.app.Coined) {
	case coinNoop:
		return
	case coinAlready:
		c.app.NotifyType("info", toastCoinAlready)
		return
	}
	go func() {
		// 原版投的是一次两币（handleCoin 传 multiply: 2），不是一币。
		if _, err := c.bl.CoinVideo(bvid, 2); err != nil {
			c.app.NotifyType("error", actionFailText("投币", err))
			return
		}
		c.app.Win.Update(func() {
			c.app.Coined = true
			c.app.NotifyType("success", toastCoinOK)
		})
	}()
}

func (c *controller) favorite() {
	_, aid := c.currentMedia()
	// 请求中的连点直接用正在查/正在提交的状态拦掉（原版 isFavoriteUpdating）。
	if aid == 0 || c.app.FavPending {
		return
	}
	reqID := c.app.InteractionReqID
	c.app.FavPending = true
	c.app.Win.Update(func() {})
	// 记下点击前的状态，成功文案与失败动作名都按它定向。
	wasFaved := c.app.Faved
	want := !wasFaved
	go func() {
		err := c.bl.SetFavorite(aid, want)
		c.app.Win.Update(func() {
			// 提交期间切了歌：这次提交属于上一条，别把结果写过来。
			if c.app.InteractionReqID != reqID {
				return
			}
			c.app.FavPending = false
			if err == nil {
				c.app.Faved = want
				c.app.NotifyType("success", favToastText(wasFaved))
			}
		})
		if err != nil {
			c.app.NotifyType("error", actionFailText(eitherText(wasFaved, "取消收藏", "收藏"), err))
		}
	}()
}

// eitherText 是三元表达式的字符串版（cond 为真取 a，否则取 b）。名字避开
// cards.go 里已有的 pickStr（那是 map 取值）。
func eitherText(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// followBlocked 报告这次关注请求是否应该被忽略：在 UP 空间里已经有一次
// 关注/取关在途时（UpFollowPending），连点忽略 —— 对应原版 upVideoList 的
// 关注键在 isCheckingFollow / isFollowingLoading 时禁用。抽成纯函数便于单测。
func followBlocked(up, pending bool) bool { return up && pending }

// follow 关注 / 取关。
//
// 注意 mid 是**被关注的人**：UP 空间抽屉里是那个 UP 主，否则是当前视频的
// UP 主（a.Info.OwnerMid）。原来这里读的是本地存的 mid —— 那是我自己的
// mid，等于在关注自己。
func (c *controller) follow() {
	a := c.app
	var mid int64
	up := false
	if a.Drawer == view.DrawerUp && a.UpMid != 0 {
		mid, up = a.UpMid, true
	} else if a.Info != nil {
		mid = a.Info.OwnerMid
	}
	// 关注方向要在 mid 判空之前定下来，才能给出原版那种「无法关注 / 无法
	// 取消关注该UP主」的定向提示。
	want := !a.Followed
	if up {
		want = !a.UpFollowed
	}
	if mid == 0 {
		a.NotifyType("error", eitherText(want, "无法关注该UP主", "无法取消关注该UP主"))
		return
	}
	// 已经有一次关注/取关在途：连点忽略（原版关注键在 isFollowingLoading 时禁用，
	// 否则 follow + unfollow 会各发一次请求，状态来回跳）。
	if followBlocked(up, a.UpFollowPending) {
		return
	}
	// 成功提示里的 UP 名：UP 空间抽屉里是抽屉那个，否则是当前视频的 UP 主。
	name := a.UpName
	if !up && a.Info != nil {
		name = a.Info.OwnerName
	}
	if up {
		a.UpFollowPending = true
	}
	go func() {
		var err error
		if want {
			_, err = c.bl.Follow(int(mid))
		} else {
			_, err = c.bl.Unfollow(int(mid))
		}
		fail := eitherText(want, "关注", "取消关注")
		c.app.Win.Update(func() {
			if up {
				c.app.UpFollowPending = false
			}
			if err != nil {
				// 此前只 log，用户点了关注却看不到任何反馈。
				c.app.NotifyType("error", actionFailText(fail, err))
				return
			}
			c.app.Followed = want
			c.app.UpFollowed = want
			c.app.NotifyType("success", followToastText(want, name))
		})
	}()
}

// ---------------------------------------------------------------- UP 空间 / 合集

// openUp 打开某个 UP 主的空间：拉视频列表、合集列表和关注状态。
func (c *controller) openUp(mid int64, name string) {
	a := c.app
	a.UpMid, a.UpName = mid, name
	a.Drawer = view.DrawerUp
	a.UpOffset = ""
	a.ListFor(view.DrawerUp).Cards = nil
	a.ListFor(view.DrawerUp).Loading = true
	a.SeriesList = nil
	upTab := a.UpTab
	// 关注状态查回来之前，表头关注键转圈且禁用（原版 isCheckingFollow）。
	a.UpFollowPending = true
	a.Win.Update(func() {})

	go func() {
		st, err := c.bl.IsFollowing(int(mid))
		a.Win.Update(func() {
			// 不管成败都要清掉 pending，否则查失败时关注键会永远转圈。
			a.UpFollowPending = false
			if err == nil && st != nil {
				a.UpFollowed = st.IsFollowing
				a.UpFans = st.Follower
			}
		})
		series, _ := c.bl.GetSeriesList(int(mid))
		list := make([]view.Series, 0, len(series))
		for _, it := range series {
			m := toMap(it)
			list = append(list, view.Series{
				ID:    pickInt(m, "season_id", "id"),
				Title: pickStr(m, "name", "title"),
				Count: pickInt(m, "total"),
			})
		}
		a.Win.Update(func() { a.SeriesList = list })
		if upTab == view.UpTabSeries {
			a.Win.Update(func() { a.ListFor(view.DrawerUp).Loading = false })
			return
		}
		c.loadUpVideos(mid, "")
	}()
}

// loadUpVideos 拉 UP 空间「视频」tab 的一页（旧版用 offset 翻页，不是页码）。
// mid 由调用方在主线程取好传进来，不在网络 goroutine 里读界面状态。
func (c *controller) loadUpVideos(mid int64, offset string) {
	list := c.app.ListFor(view.DrawerUp)
	c.app.Win.Update(func() { list.Loading = true })

	go func() {
		l, err := c.bl.GetUpVideoList(int(mid), offset)
		c.app.Win.Update(func() {
			list.Loading = false
			if err != nil {
				list.LoadErr = offset != ""
				c.app.NotifyType("error", "加载失败："+err.Error())
				return
			}
			cards := toUpCards(l.Items)
			setCards(list, cards, offset == "")
			c.app.UpOffset = l.Offset
			list.HasMore = l.HasMore
			list.LoadErr = false
			c.app.Status = fmt.Sprintf("%d 条", len(list.Cards))
		})
	}()
}

// loadUpSeries 拉 UP 主的合集列表（UP 空间的「合集」tab）。
func (c *controller) loadUpSeries(mid int64) {
	list := c.app.ListFor(view.DrawerUp)
	c.app.Win.Update(func() { list.Loading = true })

	go func() {
		series, _ := c.bl.GetSeriesList(int(mid))
		out := make([]view.Series, 0, len(series))
		for _, it := range series {
			m := toMap(it)
			out = append(out, view.Series{
				ID:    pickInt(m, "season_id", "id"),
				Title: pickStr(m, "name", "title"),
				Count: pickInt(m, "total"),
			})
		}
		c.app.Win.Update(func() {
			list.Loading = false
			c.app.SeriesList = out
		})
	}()
}

// selectSeries 选一个合集：拉它的视频列表（旧版 selectSeries 走的就是这个）。
func (c *controller) selectSeries(id int64) {
	c.app.SeriesID = id
	c.app.ListFor(view.DrawerSeries).Cards = nil
	c.loadSeriesVideos(id, 1)
}

// loadSeriesVideos 拉某个合集的视频列表。id 显式传入，不读 app.SeriesID ——
// Win.Update 是排队到主线程执行的，在它之后立刻读会读到旧值。
func (c *controller) loadSeriesVideos(id int64, page int) {
	list := c.app.ListFor(view.DrawerSeries)
	c.app.Win.Update(func() { list.Loading = true })

	mid := c.app.UpMid
	go func() {
		archives, err := c.bl.GetSeriesVideos(int(mid), int(id), page)
		c.app.Win.Update(func() {
			list.Loading = false
			if err != nil {
				list.LoadErr = page > 1
				c.app.NotifyType("error", "加载失败："+err.Error())
				return
			}
			cards := make([]view.Card, 0, len(archives))
			for _, ar := range archives {
				cards = append(cards, view.Card{
					Bvid:     ar.Bvid,
					Cover:    ar.Pic,
					Title:    ar.Title,
					Duration: fmtDur(int64(ar.Duration)),
					Views:    fmtViews(int64(ar.Stat.View)),
					Track: view.Track{
						Aid: int64(ar.Aid), Bvid: ar.Bvid, Title: ar.Title,
						Up: c.app.UpName, Cover: ar.Pic, Duration: int64(ar.Duration),
					},
				})
			}
			setCards(list, cards, page <= 1)
			list.Page = page
			// 合集接口每页 30 条；不足一页就到到底了。
			list.HasMore = len(archives) >= 30
			list.LoadErr = false
			c.app.Status = fmt.Sprintf("%d 条", len(list.Cards))
		})
	}()
}

// removeWatchLater 把一条从「稍后再看」移除，然后刷新列表。
// 请求中的 aid 置灰防连点（原版 pendingAids）。
func (c *controller) removeWatchLater(aid int64) {
	if aid == 0 || c.app.WLRemovePending[aid] {
		return
	}
	if c.app.WLRemovePending == nil {
		c.app.WLRemovePending = map[int64]bool{}
	}
	c.app.WLRemovePending[aid] = true
	c.app.Win.Update(func() {})

	go func() {
		err := c.bl.RemoveFromWatchLater(aid)
		c.app.Win.Update(func() {
			delete(c.app.WLRemovePending, aid)
			if err != nil {
				c.app.NotifyType("error", "移除失败："+err.Error())
				return
			}
			c.app.Notify("已从稍后再看移除")
			c.reload()
		})
	}()
}

// toggleIncognito 切换隐身模式并落盘（原版 localStorage 的 incognitoMode）。
// 打开后不读也不写云端播放记录与进度，只用本地断点。
func (c *controller) toggleIncognito() {
	c.app.Incognito = !c.app.Incognito
	_ = c.kv.SetString("incognito", map[bool]string{true: "true", false: "false"}[c.app.Incognito])
	c.app.Win.Update(func() {})
}

// setMini 切换迷你模式：换界面 + 调整窗口尺寸与置顶，并记忆迷你窗位置。
func (c *controller) setMini(on bool) {
	a := c.app
	a.Mini = on
	if on {
		w, h := view.MiniSize()
		a.Win.SetSize(w, h)
		// 恢复上次的迷你窗位置（原版 miniWindowPosition）；不在任何屏幕里就夹回来。
		if p := c.kv.String(prefMiniPos); p != "" {
			if x, y, ok := strings.Cut(p, ","); ok {
				xi, _ := strconv.Atoi(x)
				yi, _ := strconv.Atoi(y)
				xi, yi = clampToDisplays(xi, yi, w, h)
				a.Win.SetPosition(xi, yi)
			}
		}
		// 迷你窗默认**不**置顶（原版 isMiniPinned 初始 false，进迷你模式
		// 也不调 set_window_always_on_top）；置顶由窗口上那颗图钉控制。
		a.Win.SetAlwaysOnTop(a.Pinned)
		c.startMiniPosPoll()
	} else {
		c.stopMiniPosPoll()
		a.Win.SetSize(mainWidth, mainHeight)
		// 与旧版一致：退出迷你模式先改回尺寸再居中。
		a.Win.Center()
		a.Win.SetAlwaysOnTop(false)
		a.Pinned = false
	}
	a.Win.Update(func() {})
}

// clampToDisplays 把窗口位置夹到连接的屏幕里（原版多屏救援逻辑的简化版）：
// 目标点还在某块屏的可用区里就原样保留（含用户刻意放在边上的情况），
// 否则夹到最近那块屏，并保证整个 w×h 窗口在屏内。
func clampToDisplays(x, y, w, h int) (int, int) {
	displays := mygo.Screen.Displays()
	areas := make([]screenArea, 0, len(displays))
	for _, d := range displays {
		areas = append(areas, screenArea{d.WorkArea.X, d.WorkArea.Y, d.WorkArea.Width, d.WorkArea.Height})
	}
	return clampToAreas(x, y, w, h, areas)
}

// screenArea 是一块屏幕的可用区（抽出来是为了能单测夹取逻辑）。
type screenArea struct{ X, Y, W, H int }

// clampToAreas 是 clampToDisplays 的纯逻辑版。
func clampToAreas(x, y, w, h int, areas []screenArea) (int, int) {
	if len(areas) == 0 {
		return x, y
	}
	clamp := func(v, lo, hi int) int {
		if hi < lo {
			hi = lo
		}
		if v < lo {
			return lo
		}
		if v > hi {
			return hi
		}
		return v
	}
	for _, a := range areas {
		// 左上角在可用区内就原样恢复（保留用户刻意挂在屏幕边缘的姿势）。
		if x >= a.X && x < a.X+a.W && y >= a.Y && y < a.Y+a.H {
			return x, y
		}
	}
	best := areas[0]
	bestDist := int(^uint(0) >> 1)
	for _, a := range areas {
		cx := clamp(x, a.X, a.X+a.W)
		cy := clamp(y, a.Y, a.Y+a.H)
		dx, dy := x-cx, y-cy
		if dist := dx*dx + dy*dy; dist < bestDist {
			bestDist, best = dist, a
		}
	}
	return clamp(x, best.X, best.X+max(0, best.W-w)), clamp(y, best.Y, best.Y+max(0, best.H-h))
}

// startMiniPosPoll 每秒读一次迷你窗位置，变了就落盘。mygo 没有 window move
// 事件，所以只能轮询；只在迷你模式期间跑。
func (c *controller) startMiniPosPoll() {
	c.stopMiniPosPoll()
	stop := make(chan struct{})
	c.miniPoll = stop
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		last := c.kv.String(prefMiniPos)
		for {
			select {
			case <-stop:
				return
			case <-t.C:
			}
			x, y := c.app.Win.Position()
			p := fmt.Sprintf("%d,%d", x, y)
			if p != last {
				last = p
				_ = c.kv.SetString(prefMiniPos, p)
			}
		}
	}()
}

func (c *controller) stopMiniPosPoll() {
	if c.miniPoll != nil {
		close(c.miniPoll)
		c.miniPoll = nil
	}
}

// togglePin 切换迷你窗的置顶。
func (c *controller) togglePin() {
	c.app.Pinned = !c.app.Pinned
	c.app.Win.SetAlwaysOnTop(c.app.Pinned)
	c.app.Win.Update(func() {})
}

// 播放队列的落盘与启动恢复已移除：用户明确要求去掉「启动时恢复上次
// 播放项」（原版也不在启动时自动恢复播放）。

// ---------------------------------------------------------------- 列表

// maxRetainedCards 是单个列表在内存里保留的卡片上限：超出后从头部释放
// （有界滑动窗口），避免长列表无限增长（原版 listRetention.ts 的同名上限）。
const maxRetainedCards = 160

// setCards 写入列表卡片：replace 为真时替换，否则追加；无论哪种都保持上限。
func setCards(list *view.List, cards []view.Card, replace bool) {
	if replace {
		list.Cards = cards
		// 整批替换（换关键词 / 刷新 / 换抽屉）时把虚拟网格的滚动复位到顶；
		// 追加翻页不动它，锚点跟着旧内容。
		list.ResetGrid()
	} else {
		list.Cards = append(list.Cards, cards...)
	}
	if len(list.Cards) > maxRetainedCards {
		list.Cards = list.Cards[len(list.Cards)-maxRetainedCards:]
	}
}

func (c *controller) loadSection(section string, page int) {
	a := c.app
	list := a.ListFor(section)
	a.Win.Update(func() { list.Loading = true; a.Status = "加载中…" })

	// 在起 goroutine 之前把这一帧的取值固定下来，避免和界面线程争。
	folderID, recTab, histTab := a.FolderID, a.RecTab, a.HistTab

	// 动态用 offset 游标翻页（不是页码）。
	if section == "feed" {
		offset := ""
		if page > 1 {
			offset = a.FeedOffset
		}
		go func() {
			l, err := c.bl.GetBLFeedList(offset)
			a.Win.Update(func() {
				list.Loading = false
				if err != nil {
					// 只有「翻页」失败才进入可重试的错误态；首屏失败仍走空态。
					list.LoadErr = page > 1
					a.NotifyType("error", "加载失败："+err.Error())
					return
				}
				setCards(list, toFeedCards(l.Items), page <= 1)
				list.Page = page
				list.HasMore = l.HasMore
				list.LoadErr = false
				a.FeedOffset = l.Offset
				a.Status = fmt.Sprintf("%d 条", len(list.Cards))
			})
		}()
		return
	}

	go func() {
		cards, more, err := c.fetchSection(section, page, folderID, recTab, histTab)
		a.Win.Update(func() {
			list.Loading = false
			if err != nil {
				list.LoadErr = page > 1
				a.NotifyType("error", "加载失败："+err.Error())
				return
			}
			setCards(list, cards, page <= 1)
			list.Page = page
			list.HasMore = more
			list.LoadErr = false
			a.Status = fmt.Sprintf("%d 条", len(list.Cards))
		})
	}()
}

// fetchSection 拉一个抽屉的第一页/下一页。
//
// 旧版把「热门与推荐」「历史」做成一个抽屉里两个 tab，所以这里要按 tab
// （recTab / histTab）选数据源，不能只看抽屉 key。
func (c *controller) fetchSection(section string, page int, folderID int64, recTab, histTab string) ([]view.Card, bool, error) {
	switch section {
	case "recommend": // 旧 key，等价于「热门与推荐」抽屉的推荐 tab
		l, err := c.bl.GetBLRCMDList(page)
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(l.Items), true, nil
	case "popular":
		if recTab == view.RecRecommend {
			l, err := c.bl.GetBLRCMDList(page)
			if err != nil {
				return nil, false, err
			}
			return toCardsFromRaw(l.Items), true, nil
		}
		l, err := c.bl.GetBLPopularList(page)
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(l.Items), l.HasMore, nil
	case "feed":
		// 动态走 loadSection 的 offset 分支，这里不会走到。
		return nil, false, fmt.Errorf("feed 请用 loadSection")
	case "favorite":
		if folderID == 0 {
			return nil, false, nil
		}
		items, err := c.bl.GetBLFavFolderListDetail(int(folderID), page)
		if err != nil {
			return nil, false, err
		}
		// 收藏夹接口每页 21 条；不足一页说明到底了。
		return toCardsFromRaw(items), len(items) >= 21, nil
	case "history":
		if histTab == view.HistWatchLater {
			l, err := c.bl.GetWatchLaterList()
			if err != nil {
				return nil, false, err
			}
			return toCards(jsonToAny(l.List)), false, nil
		}
		l, err := c.bl.GetBLHistoryList(0, 0, "", 30)
		if err != nil {
			return nil, false, err
		}
		return toCards(jsonToAny(l.List)), false, nil
	case "watchlater": // 旧 key，等价于「历史」抽屉的稍后再看 tab
		l, err := c.bl.GetWatchLaterList()
		if err != nil {
			return nil, false, err
		}
		return toCards(jsonToAny(l.List)), false, nil
	}
	return nil, false, fmt.Errorf("未知分区 %s", section)
}

// reload 是抽屉表头的刷新键、切 tab、切收藏夹的统一入口：重拉第一页。
func (c *controller) reload() {
	a := c.app
	// 顺手记住表头的 tab（原版把这两个 tab 存在 localStorage）。
	_ = c.kv.SetString("recommendTab", a.RecTab)
	_ = c.kv.SetString("historyTab", a.HistTab)
	switch a.Drawer {
	case view.DrawerSearch:
		c.search(a.Query)
	case view.DrawerUp:
		a.ListFor(view.DrawerUp).Cards = nil
		a.UpOffset = ""
		// 刷新时同时重查关注状态和粉丝数（原版 handleRefresh）。
		if mid := a.UpMid; mid != 0 {
			go func() {
				st, err := c.bl.IsFollowing(int(mid))
				if err != nil || st == nil {
					return
				}
				a.Win.Update(func() {
					a.UpFollowed = st.IsFollowing
					a.UpFans = st.Follower
				})
			}()
		}
		if a.UpTab == view.UpTabSeries {
			c.loadUpSeries(a.UpMid)
		} else {
			c.loadUpVideos(a.UpMid, "")
		}
	case view.DrawerSeries:
		a.ListFor(view.DrawerSeries).Cards = nil
		c.loadSeriesVideos(a.SeriesID, 1)
	case view.DrawerDanmaku:
		// 弹幕/评论抽屉的刷新：清掉再拉一次。
		if a.DanmakuTab == view.TabReply {
			a.Comments = nil
			a.ReplyPage = 0
			a.RepliesHasMore = false
			c.loadComments(1)
		} else {
			a.Danmaku = nil
			c.toggleDanmaku()
		}
	case "favorite":
		// 收藏夹列表还没拉过（或换账号了）就先拉它，再拉内容。
		if len(a.Folders) == 0 {
			a.ListFor("favorite").Loading = true
			a.Win.Update(func() {})
			c.loadFolders()
			return
		}
		a.ListFor("favorite").Cards = nil
		c.loadFolderDetail(a.FolderID, 1)
	default:
		key := a.CurrentSection().Key
		a.ListFor(key).Cards = nil
		c.loadSection(key, 1)
	}
}

// loadFolders 拉收藏夹列表（收藏抽屉的表头 tab），并选中第一个。
// 旧版也是默认取第一个收藏夹（默认收藏夹在最前）。
func (c *controller) loadFolders() {
	go func() {
		folders, err := c.bl.GetBLFavFolderList()
		if err != nil {
			c.app.Win.Update(func() {
				c.app.ListFor("favorite").Loading = false
				c.app.NotifyType("error", "收藏夹加载失败："+err.Error())
			})
			return
		}
		list := make([]view.Folder, 0, len(folders))
		for _, f := range folders {
			m := toMap(f)
			list = append(list, view.Folder{
				ID:    pickInt(m, "id", "fid", "media_id"),
				Title: pickStr(m, "title", "name"),
				Count: pickInt(m, "media_count"),
			})
		}
		var fid int64
		if len(list) > 0 {
			fid = list[0].ID
		}
		c.app.Win.Update(func() {
			c.app.Folders = list
			c.app.FolderID = fid
		})
		if fid == 0 {
			c.app.Win.Update(func() { c.app.ListFor("favorite").Loading = false })
			return
		}
		// 注意：不能在这里调 loadSection 让它去读 app.FolderID —— Win.Update 是
		// 排队到主线程执行的，这一行读到的还是旧值。收藏夹 id 显式传下去。
		c.loadFolderDetail(fid, 1)
	}()
}

// loadFolderDetail 拉某个收藏夹的内容。fid 显式传入，不读 app.FolderID。
func (c *controller) loadFolderDetail(fid int64, page int) {
	list := c.app.ListFor("favorite")
	c.app.Win.Update(func() { list.Loading = true })

	go func() {
		items, err := c.bl.GetBLFavFolderListDetail(int(fid), page)
		c.app.Win.Update(func() {
			list.Loading = false
			if err != nil {
				c.app.NotifyType("error", "加载失败："+err.Error())
				return
			}
			setCards(list, toCardsFromRaw(items), page <= 1)
			list.Page = page
			list.HasMore = len(items) >= 21
			c.app.Status = fmt.Sprintf("%d 条", len(list.Cards))
		})
	}()
}

// selectFolder 切收藏夹：只重拉列表，不重新拉收藏夹列表。
func (c *controller) selectFolder(id int64) {
	c.app.FolderID = id
	c.reload()
}

func (c *controller) loadMore() {
	a := c.app
	switch a.Drawer {
	case view.DrawerUp:
		if a.UpTab == view.UpTabSeries {
			return
		}
		c.loadUpVideos(a.UpMid, a.UpOffset)
	case view.DrawerSeries:
		c.loadSeriesVideos(a.SeriesID, a.ListFor(view.DrawerSeries).Page+1)
	default:
		key := a.CurrentSection().Key
		c.loadSection(key, a.ListFor(key).Page+1)
	}
}

// copyLink 复制当前视频的 B 站链接到剪贴板。
func (c *controller) copyLink() {
	bvid := ""
	if c.app.Info != nil {
		bvid = c.app.Info.Bvid
	}
	if bvid == "" && c.app.Track != nil {
		bvid = c.app.Track.Bvid
	}
	if bvid == "" {
		c.app.NotifyType("warning", "还没有在播放的视频")
		return
	}
	url := "https://www.bilibili.com/video/" + bvid
	mygo.Clipboard.WriteText(url)
	c.app.Notify("链接已复制：" + url)
}

// urlJump 直接打开一个 B 站视频链接：拉详情、填主区、打开选集抽屉。
// （原版 handleUrlJump：不自动播放，让用户自己在选集里选。）
func (c *controller) urlJump(url string) {
	a := c.app
	bvid := bvidFromURL(url)
	if bvid == "" {
		a.NotifyType("error", "未识别出有效的 B 站视频地址")
		return
	}
	go func() {
		info := c.videoInfoOf(bvid, nil)
		if info == nil || info.Bvid == "" {
			a.NotifyType("error", "获取视频信息失败")
			return
		}
		a.Win.Update(func() {
			a.Info = info
			a.Drawer = view.DrawerParts
		})
		// 预先把这一视频的分集建成播放队列，这样点分集时能直接播。
		c.loadInteractionState(info)
	}()
}

// openCard 点列表卡片：取这条的 bvid 走 urlJump（打开选集面板）。
func (c *controller) openCard(index int) {
	a := c.app
	cards := a.ListFor(a.Drawer).Cards
	if index < 0 || index >= len(cards) {
		return
	}
	if bvid := cards[index].Bvid; bvid != "" {
		c.urlJump("https://www.bilibili.com/video/" + bvid)
	}
}

// openBrowser 用系统浏览器打开当前视频。
func (c *controller) openBrowser() {
	bvid := ""
	if c.app.Info != nil {
		bvid = c.app.Info.Bvid
	}
	if bvid == "" && c.app.Track != nil {
		bvid = c.app.Track.Bvid
	}
	if bvid == "" {
		return
	}
	if err := mygo.Shell.OpenExternal("https://www.bilibili.com/video/" + bvid); err != nil {
		log.Printf("打开浏览器失败: %v", err)
	}
}

// bvidFromURL 从一段文本里取 BV 号（原版 urlToBVID）。
func bvidFromURL(s string) string {
	m := bvRe.FindString(s)
	return m
}

var bvRe = regexp.MustCompile(`BV[a-zA-Z0-9]+`)

func (c *controller) search(query string) {
	a := c.app
	// 空关键词：清结果并关抽屉（原版 handleSearch 的空值分支）。
	if q := strings.TrimSpace(query); q == "" {
		a.Query = ""
		list := a.ListFor(view.DrawerSearch)
		list.Cards, list.HasMore, list.Loading = nil, false, false
		list.LoadErr = false
		a.Drawer = ""
		a.Win.Update(func() {})
		return
	}
	// 请求代号：慢的旧响应不得覆盖新的（原版 searchRequestIdRef）。
	a.SearchRequestID++
	reqID := a.SearchRequestID

	list := a.ListFor(view.DrawerSearch)
	list.Loading = true
	a.Drawer = view.DrawerSearch
	a.Win.Update(func() {})

	// 搜索排序（表头的「综合 / 最多播放 / 最新发布」）。
	order := a.SortOrder
	if order == "" {
		order = view.SortTotal
	}

	go func() {
		results := c.bl.SearchVideo(query, order)
		items := make([]any, 0, len(results))
		for _, r := range results {
			items = append(items, map[string]any{
				"bvid":     r.Bvid,
				"aid":      float64(r.Aid),
				"pic":      r.PictureURL,
				"title":    r.Title,
				"duration": parseDurText(r.Length),
				"owner":    map[string]any{"name": r.Author},
				"stat":     map[string]any{"view": parseCountText(r.Views)},
				// 原版搜索卡片的 meta 三列：作者/播放量/发布时间。
				"pubdate": metaDateText(r.Date),
			})
		}
		a.Win.Update(func() {
			if a.SearchRequestID != reqID {
				return // 已经发起了更新的搜索
			}
			list.Loading = false
			// 运行态上限（原版 MAX_RETAINED_LIST_ITEMS）。
			cards := toCards(items)
			if len(cards) > maxRetainedCards {
				cards = cards[:maxRetainedCards]
			}
			list.Cards = cards
			list.HasMore = false
			list.LoadErr = false
			a.Status = fmt.Sprintf("搜索「%s」：%d 条", query, len(list.Cards))
		})
	}()
}

// ---------------------------------------------------------------- 播放

// playIndex 播放当前列表里的第 index 条。
func (c *controller) playIndex(index int) {
	// 选集抽屉：队列就是当前浏览视频的分集（以 a.Info 为准，而不是可能过期的
	// 旧队列）；点哪个分集就播哪个。
	if c.app.Drawer == view.DrawerParts {
		info := c.app.Info
		if info == nil || index < 0 || index >= len(info.Parts) {
			return
		}
		part := info.Parts[index]
		c.buildPartsQueue(info, part.Cid)
		c.app.Index = index
		c.startCurrent()
		return
	}
	cards := c.app.ListFor(c.app.Drawer).Cards
	if index < 0 || index >= len(cards) {
		return
	}
	// 把整个列表作为播放队列，这样上一首/下一首能连续播放。
	queue := make([]view.Track, 0, len(cards))
	for _, card := range cards {
		queue = append(queue, card.Track)
	}
	c.app.Queue = queue
	c.app.Index = index
	// 从普通列表进入：退出「播放列表模式」。
	c.app.PlayingPlaylist = ""
	c.startCurrent()
}

// openParts 打开选集抽屉时把分集当作播放队列（旧版：进详情后上一首/下一首
// 就在分 P 之间走）。已经拉过详情就直接用，否则补拉一次。
func (c *controller) openParts(t view.Track) {
	a := c.app
	if a.Drawer != view.DrawerParts || t.Bvid == "" {
		return
	}
	if a.Info != nil && a.Info.Bvid == t.Bvid && len(a.Info.Parts) > 0 {
		c.buildPartsQueue(a.Info, t.Cid)
		return
	}
	go func() {
		info := c.videoInfoOf(t.Bvid, &t)
		if info == nil || len(info.Parts) == 0 {
			return
		}
		a.Win.Update(func() {
			a.Info = info
			c.buildPartsQueue(info, t.Cid)
		})
	}()
}

// buildPartsQueue 把详情的分集变成播放队列，并把当前位置指到 cid。
func (c *controller) buildPartsQueue(info *view.Info, cid int64) {
	a := c.app
	queue := make([]view.Track, 0, len(info.Parts))
	for _, p := range info.Parts {
		queue = append(queue, view.Track{
			Aid: info.Aid, Bvid: info.Bvid, Cid: p.Cid,
			Title: info.Title, Up: info.OwnerName, Cover: info.Pic,
			Part: p.Part, FirstFrame: p.FirstFrame,
		})
	}
	a.Queue = queue
	a.PlayingPlaylist = ""
	for i, q := range queue {
		if q.Cid == cid {
			a.Index = i
		}
	}
	a.Win.Update(func() {})
}

// videoInfoOf 拉取视频详情（标题 / 简介 / UP 主 / 分集 / 互动数）。
// 主区右栏和「详情」抽屉都用它，所以拿不到时用列表卡片的字段兜底。
func (c *controller) videoInfoOf(bvid string, fallback *view.Track) *view.Info {
	vi := c.bl.GetCList(bvid)
	if vi.Bvid == "" && fallback == nil {
		return nil
	}
	info := &view.Info{
		Aid:       int64(vi.Aid),
		Bvid:      vi.Bvid,
		Title:     vi.Title,
		Desc:      vi.Desc,
		Pic:       vi.Pic,
		OwnerMid:  int64(vi.OwnerMid),
		OwnerName: vi.OwnerName,
		OwnerFace: vi.OwnerFace,
		Like:      vi.Stat.Like,
		Coin:      vi.Stat.Coin,
		Favorite:  vi.Stat.Favorite,
		View:      vi.Stat.View,
	}
	for _, p := range vi.Pages {
		info.Parts = append(info.Parts, view.Part{
			Cid: int64(p.Cid), Page: p.Page, Part: p.Part,
			Duration: int64(p.Duration), FirstFrame: p.FirstFrame,
		})
	}
	// 参与者 = 主 UP + staff（按 mid 去重），和原版 videoInfo 的 creators 一致。
	seenMid := map[int64]bool{int64(vi.OwnerMid): true}
	if vi.OwnerMid != 0 {
		info.Staff = append(info.Staff, view.Collaborator{
			Mid:  int64(vi.OwnerMid),
			Name: vi.OwnerName,
			Face: vi.OwnerFace,
		})
	}
	for _, st := range vi.Staff {
		mid := int64(st.Mid)
		if mid == 0 || seenMid[mid] {
			continue
		}
		seenMid[mid] = true
		info.Staff = append(info.Staff, view.Collaborator{Mid: mid, Name: st.Name, Title: st.Title, Face: st.Face})
	}
	if len(vi.Pages) > 0 {
		info.Cid = int64(vi.Pages[0].Cid)
	}
	if fallback != nil {
		if info.Title == "" {
			info.Title = fallback.Title
		}
		if info.Pic == "" {
			info.Pic = fallback.Cover
		}
		if info.OwnerName == "" {
			info.OwnerName = fallback.Up
		}
	}
	return info
}

// keepDrawerOnPlay 报告起播时该保留哪个抽屉。原版从选集面板点一集
// （handleVideoSelect）不会关面板；播放列表模式下手动/自动切歌也不关
// （handlePlaylistVideoSelect 的 preserveUi）。其余抽屉（搜索/分区/UP 空间…）
// 点卡片会先被换成选集抽屉，起播时再关掉。
func keepDrawerOnPlay(drawer string) bool {
	return drawer == view.DrawerPlaylist || drawer == view.DrawerParts
}

// startCurrent 起播当前队列项：取播放地址 → 起播 → 拉弹幕与跳过分段。
func (c *controller) startCurrent() {
	t := c.app.Current()
	if t == nil {
		return
	}
	// 切歌时静默关掉视频弹窗（不要它再跳回来接音频）。
	if c.app.VideoOpen {
		c.videoSilent = true
		c.vid.Close()
		c.app.VideoOpen = false
	}
	c.app.Buffering = true
	c.app.Buffered = 0
	c.app.Status = "起播中…"
	// 从选集、搜索结果等进入时关掉抽屉；选集/播放列表模式下要保留面板——
	// 原版在选集里点一集（handleVideoSelect）不会关选集面板，播放列表模式下
	// 手动/自动切歌也不关（handlePlaylistVideoSelect 的 preserveUi）。
	if !keepDrawerOnPlay(c.app.Drawer) {
		c.app.Drawer = ""
	}
	c.app.Danmaku = nil
	c.sponsor, c.skipped = nil, map[int]bool{}
	c.app.SponsorSegments = nil
	// 状态点的段数/原因属于上一条视频，换歌时清掉（开关重开后会重新查）。
	c.app.SponsorCount, c.app.SponsorReason = 0, ""
	// seek 状态机复位（换歌后跟随新进度）；丢弃起播期间排队的 seek。
	c.app.ResetSeek()
	c.app.HasPendingSeek = false
	c.app.Win.Update(func() {})

	// 主区右栅要立刻换成这条视频的信息，所以先把列表卡片里的字段填上，
	// 等 GetCList 回来再补简介、UP 主头像和分集。
	c.app.Info = &view.Info{
		Aid: t.Aid, Bvid: t.Bvid, Cid: t.Cid,
		Title: t.Title, Pic: t.Cover, OwnerName: t.Up,
	}
	c.syncMediaTrack()
	// 倍速/均衡/音量也在主线程取好，goroutine 里不读界面状态。
	speed, eq, vol := c.app.Speed, c.app.EQ, c.app.Volume
	// 请求代号：用户快速连点时，慢的旧起播响应不得覆盖新的
	// （原版 playbackRequestIdRef）。
	c.app.PlaybackRequestID++
	reqID := c.app.PlaybackRequestID
	stale := func() bool { return c.app.PlaybackRequestID != reqID }

	go func() {
		cid := t.Cid
		info := c.videoInfoOf(t.Bvid, t)
		if stale() {
			return
		}
		if info != nil {
			if cid == 0 {
				cid = info.Cid
			}
			c.app.Win.Update(func() {
				if stale() {
					return
				}
				c.app.Info = info
				c.syncMediaTrack()
			})
			// 互动状态（点赞/投币/收藏/关注）异步拉一次，填主区按钮的激活态。
			c.loadInteractionState(info)
		}
		if cid == 0 {
			c.app.Win.Update(func() {
				if stale() {
					return
				}
				c.app.Buffering = false
				c.app.NotifyType("warning", "取分集失败")
			})
			return
		}
		// 续播点要在起播前定好：media.Player.Seek 会重建解码器，起播后再跳
		// 会卡一下。云端最多等 800ms，超时用本地断点。
		resume := c.resolveResume(t.Aid, cid)
		if stale() {
			return
		}

		u := c.bl.GetUrlByCid(int(t.Aid), int(cid))
		if u.URL == "" {
			c.app.Win.Update(func() {
				if stale() {
					return
				}
				c.app.Buffering = false
				c.app.NotifyType("warning", "该视频暂时无法播放，可能已失效或受限")
			})
			return
		}

		// 回填 cid 与分集标题
		c.app.Win.Update(func() {
			if stale() {
				return
			}
			t.Cid = cid
			c.app.Track = t
			c.app.Pos, c.app.Dur = 0, 0
		})

		// 真正起播前再校验一次：这期间用户可能已经点了别的。
		if stale() {
			log.Printf("起播取消（已切到别的曲目）")
			return
		}
		if err := c.mp.Play(u.URL, audioHeaders); err != nil {
			// 起播被更新的起播取代不是失败：弹「起播失败」是误报，而且会把
			// 新那一次正在进行的缓冲状态抹掉。
			if errors.Is(err, media.ErrSuperseded) {
				log.Printf("起播被取代，忽略: %v", err)
				return
			}
			log.Printf("起播失败: %v", err)
			c.app.Win.Update(func() {
				c.app.Buffering = false
				// 文案对齐原版 <audio> 的分类（此前直接把 Go 的错误串贴给用户：
				// 「起播失败：media: 上游返回 403」）。
				c.app.NotifyType("error", audioErrorMessage(err))
			})
			return
		}
		c.mp.SetSpeed(speed)
		c.mp.SetEQ(eq)
		c.mp.SetVolume(vol)

		c.app.Win.Update(func() {
			c.app.Buffering = false
			c.app.Playing = true
			c.app.Dur = c.mp.Duration()
			c.app.Status = ""
			c.syncMediaTrack()
			// 起播期间用户请求过的跳转，现在执行（safeSeek 的排队语义）。
			if c.app.HasPendingSeek {
				c.app.HasPendingSeek = false
				go c.seek(c.app.PendingSeek)
			}
		})

		// 续播：本地/云端断点比 5 秒靠后才跳。
		if resume >= resumeMinSeconds {
			if err := c.mp.Seek(resume); err != nil {
				log.Printf("续播跳转失败: %v", err)
			} else {
				c.app.Win.Update(func() {
					c.app.Pos, c.app.SeekValue = resume, resume
				})
			}
		}
		// 换集后第一笔上报要立刻发（不要等 30 秒的节流窗口）。
		c.reportedKey, c.reportedAt = "", time.Time{}

		// 弹幕与跳过分段：可选增强，失败不影响播放。弹幕预取同样置
		// isLoadingDanmaku —— 若此时用户打开弹幕抽屉，表头刷新键会转圈，
		// 且 toggleDanmaku 不会重复拉一份。
		c.app.Win.Update(func() { c.app.DanmakuLoading = true })
		if d, err := c.bl.GetDanmakuList(int(cid)); err == nil && d != nil {
			list := toDanmakuList(d.Items)
			c.app.Win.Update(func() {
				c.app.DanmakuLoading = false
				c.app.Danmaku = list
			})
		} else {
			c.app.Win.Update(func() { c.app.DanmakuLoading = false })
		}
		if c.sponsorOn {
			c.app.Win.Update(func() {
				c.app.SponsorStatus = "loading"
				c.app.SponsorCount = 0
				c.app.SponsorReason = ""
			})
			segs, err := c.bl.FetchSponsorSegments(t.Bvid, cid)
			status := "empty"
			switch {
			case err != nil:
				status = "error"
			case len(segs) > 0:
				status = "ok"
			}
			c.app.Win.Update(func() {
				c.sponsor = segs
				c.app.SponsorSegments = toViewSegments(segs)
				c.app.SponsorStatus = status
				c.app.SponsorCount = len(segs)
				c.app.SponsorReason = sponsorReason(err)
			})
		}
	}()
}

func (c *controller) togglePlay() {
	// 视频弹窗开着时，空格控制的是弹窗里的视频（音频引擎此刻是暂停的，
	// 否则会双声）。
	if c.app.VideoOpen {
		if c.vid.State().Paused {
			c.vid.Resume()
		} else {
			c.vid.Pause()
		}
		return
	}
	if c.app.Current() == nil {
		return
	}
	if c.mp.Playing() {
		c.mp.Pause()
		c.app.Playing = false
		// 暂停时立刻补写本地断点并补报云端（原版 force=true）。
		c.flushProgress(c.app.Pos)
	} else {
		c.mp.Resume()
		c.app.Playing = true
	}
	c.syncMediaTrack()
	c.app.Win.Update(func() {})
}

func (c *controller) step(delta int) {
	if len(c.app.Queue) == 0 {
		return
	}
	// 普通队列只有一条（单集视频）时，上一首/下一首无意义（原版 pages<=1 直接返回）。
	if c.app.PlayingPlaylist == "" && len(c.app.Queue) <= 1 {
		return
	}
	// 切歌前把当前这条的断点补写 / 补报（原版在切歌时 force 上报）。
	c.flushProgress(c.app.Pos)

	// 播放列表模式：在来源列表里按播放模式选下一条。
	if c.app.PlayingPlaylist != "" && delta != 0 {
		items := c.playlistOf(c.app.PlayingPlaylist)
		n := len(items)
		if n == 0 {
			return
		}
		var next int
		if delta > 0 && c.app.PlayMode == view.PlayModeShuffle {
			next = randomOtherIndex(n, c.app.Index)
		} else {
			next = c.app.Index + delta
			if next < 0 {
				next = n - 1
			}
			if next >= n {
				next = 0
			}
		}
		// 自动切下一首：静默切换，不要把播放列表抽屉弹出来。
		c.playPlaylist(c.app.PlayingPlaylist, next, false)
		return
	}

	next := c.app.Index + delta
	if next < 0 {
		next = len(c.app.Queue) - 1
	}
	if next >= len(c.app.Queue) {
		next = 0
	}
	c.app.Index = next
	c.startCurrent()
}

func (c *controller) seek(seconds float64) {
	// 起播中（Buffering）不立即 seek：解码器还在初始化，与 safeSeek 的
	// readyState 保护同理——记下目标，等起播完成后执行（排队不丢弃）。
	if c.app.Buffering {
		c.app.PendingSeek = seconds
		c.app.HasPendingSeek = true
		c.app.SeekValue = seconds
		return
	}
	// 上一次 seek 还在跑：只更新目标（滑块已停在手上），跑完补一次。
	if c.app.SeekBusy {
		c.app.PendingSeek = seconds
		c.app.HasPendingSeek = true
		return
	}
	c.app.SeekBusy = true
	c.runSeek(seconds)
}

// runSeek 真正执行一次 seek；完成后若有排队的/期间新来的目标，补一次。
func (c *controller) runSeek(seconds float64) {
	// 跳转等于改了断点，立刻补写 + 补报（原版把 seek 当关键事件）。
	c.flushProgress(seconds)
	c.syncMediaPosition(seconds)
	go func() {
		err := c.mp.Seek(seconds)
		c.app.Win.Update(func() {
			c.app.SeekBusy = false
			c.app.SeekStableAt = time.Now() // seek 完成，可以恢复跟随
			if err != nil {
				log.Printf("跳转失败: %v", err)
				return
			}
			c.app.Pos = seconds
			c.skipped = map[int]bool{}
			// 期间用户又拖了：补一次到最新目标。
			if c.app.HasPendingSeek {
				c.app.HasPendingSeek = false
				next := c.app.PendingSeek
				go c.runSeek(next)
			}
		})
	}()
}

func (c *controller) toggleEQ() {
	c.app.EQ = !c.app.EQ
	c.mp.SetEQ(c.app.EQ)
	_ = c.kv.SetString(prefEQ, strconv.FormatBool(c.app.EQ))
	c.app.Win.Update(func() {})
}

func (c *controller) toggleSponsor() {
	a := c.app
	a.Sponsor = !a.Sponsor
	c.sponsorOn = a.Sponsor
	if a.Sponsor {
		_ = c.kv.SetString("sponsor_skip", "1")
		a.SponsorStatus = "loading"
		a.SponsorCount = 0
		a.SponsorReason = ""
		if t := a.Current(); t != nil && t.Cid != 0 {
			bvid, cid := t.Bvid, t.Cid
			go func() {
				segs, err := c.bl.FetchSponsorSegments(bvid, cid)
				status := "empty"
				switch {
				case err != nil:
					status = "error"
				case len(segs) > 0:
					status = "ok"
				}
				a.Win.Update(func() {
					c.sponsor = segs
					a.SponsorSegments = toViewSegments(segs)
					a.SponsorStatus = status
					a.SponsorCount = len(segs)
					a.SponsorReason = sponsorReason(err)
				})
			}()
		}
	} else {
		_ = c.kv.SetString("sponsor_skip", "0")
		a.SponsorStatus = "off"
		a.SponsorCount = 0
		a.SponsorReason = ""
	}
	a.Win.Update(func() {})
}

func (c *controller) toggleDanmaku() {
	a := c.app
	if a.Drawer != view.DrawerDanmaku {
		return
	}
	if a.DanmakuTab == view.TabReply {
		if len(a.Comments) == 0 && !a.RepliesLoading {
			c.loadComments(1)
		}
		return
	}
	if len(a.Danmaku) > 0 {
		return
	}
	t := a.Current()
	if t == nil || t.Cid == 0 {
		return
	}
	// 已经有一次弹幕拉取在途：连点忽略（原版刷新键 isLoading 期间禁用，
	// 起播预取与打开抽屉都共用同一个 isLoadingDanmaku）。
	if danmakuLoadBlocked(a.DanmakuLoading) {
		return
	}
	a.DanmakuLoading = true
	cid := t.Cid
	a.Win.Update(func() {})
	go func() {
		d, err := c.bl.GetDanmakuList(int(cid))
		// 不管成败都要清 loading，否则表头刷新键会永远转圈。
		c.app.Win.Update(func() {
			c.app.DanmakuLoading = false
			if err != nil || d == nil {
				return
			}
			c.app.Danmaku = toDanmakuList(d.Items)
		})
	}()
}

// danmakuLoadBlocked 报告这次弹幕拉取是否该被忽略：已经有一次在途时忽略
// （原版 isLoadingDanmaku 期间刷新键禁用，实际不会发第二次）。抽成纯函数便于单测。
func danmakuLoadBlocked(pending bool) bool { return pending }

// maxDanmaku / maxComments 是内存里保留的弹幕/评论条数上限（原版也是这个量级）。
// 两个列表都走虚拟化 ui.List，每帧成本只跟可见行数有关，所以上限现在约束的是
// 内存驻留而非每帧构建；评论上限从 120 提到 240，用户能往回读得更远（超上限仍
// 从头部释放已读那批，见 retainComments）。
const (
	maxDanmaku  = 400
	maxComments = 240
)

// toDanmakuList 把接口弹幕转成视图弹幕，并裁到上限。
func toDanmakuList(items []bilibili.DanmakuItem) []view.Danmaku {
	n := len(items)
	if n > maxDanmaku {
		n = maxDanmaku
	}
	out := make([]view.Danmaku, 0, n)
	for _, it := range items[:n] {
		out = append(out, view.Danmaku{Time: it.Time, Text: it.Content, Color: it.Color})
	}
	return out
}

// maybeSkipSponsor 在播放到跳过分段时自动跳过去。
func (c *controller) maybeSkipSponsor(pos float64) {
	if !c.sponsorOn || len(c.sponsor) == 0 {
		return
	}
	for i, s := range c.sponsor {
		if c.skipped[i] {
			continue
		}
		if pos >= s.Start && pos < s.End {
			c.skipped[i] = true
			log.Printf("跳过赞助分段 %.1f-%.1f (%s)", s.Start, s.End, s.Category)
			// 与原版一致：跳过时弹一条 toast，但频控（同一段只弹一次）。
			if time.Since(c.lastSponsorToast) > 5*time.Second {
				c.lastSponsorToast = time.Now()
				c.app.Notify("已跳过恰饭片段")
			}
			c.seek(s.End)
			return
		}
	}
}

// ---------------------------------------------------------------- 视频弹窗

// wireVideo 把视频弹窗的回调接回来：
//   - 弹窗播放时主区进度跟随它，并持续记断点/上报；
//   - 弹窗关掉后把音频接回来（跳到弹窗停下的位置继续听）；
//   - 弹窗自然播完按播放模式续播。
func (c *controller) wireVideo() {
	// 弹窗回调跑在窗口事件 goroutine 上，而它们读写界面状态，所以统一
	// 丢到 Win.Update（主线程）执行。
	c.vid.OnState(func(st video.State) {
		c.app.Win.Update(func() {
			if c.app.VideoOpen {
				if st.Duration > 0 {
					c.app.Dur = st.Duration
				}
				c.app.Pos = st.Time
			}
			c.trackResume(st.Time)
		})
	})
	c.vid.OnClose(func(st video.State) {
		c.app.Win.Update(func() {
			c.app.VideoOpen = false
			if c.quitting || c.videoSilent {
				c.videoSilent = false
				return
			}
			if c.app.Current() == nil {
				return
			}
			// 音视接力：跳到弹窗停下的位置继续听。
			c.flushProgress(st.Time)
			if st.Time > 0 {
				c.seek(st.Time)
			}
			if !st.Paused {
				c.mp.Resume()
				c.app.Playing = true
			}
		})
	})
	c.vid.OnEnded(func() {
		// 看完：B 站用 progress = -1 表示已看完，并清掉本地断点
		// （与 progress.go 的 resumeTailSeconds 语义一致）。
		if t := c.app.Current(); t != nil && !c.app.Incognito {
			c.reportProgress(t.Aid, t.Cid, -1)
		}
		if t2 := c.app.Current(); t2 != nil {
			c.clearResume(t2.Aid, t2.Cid)
		}
		c.app.Win.Update(func() { c.handleEnded() })
	})
}

func (c *controller) openVideo() {
	t := c.app.Current()
	if t == nil || t.Cid == 0 {
		c.app.NotifyType("warning", "请先选择一个视频")
		c.app.Win.Update(func() {})
		return
	}
	// 弹窗自带声音：先把音频引擎停下来，避免双声。失败时再恢复。
	wasPlaying := c.app.Playing
	c.mp.Pause()
	c.app.Playing = false
	c.app.Win.Update(func() {})
	go func() {
		// 弹窗要的是渐进式 MP4 地址，走本地代理（带 Referer）。
		u := c.bl.GetUrlByCid(int(t.Aid), int(t.Cid))
		if u.URL == "" {
			c.app.NotifyType("error", "取播放地址失败")
			c.restoreAudioAfterVideo(wasPlaying)
			return
		}
		opts := video.Options{
			Src:     proxy.AudioURL(u.URL),
			Title:   t.Title,
			Poster:  proxy.ImageURL(t.Cover),
			StartAt: c.app.Pos,
			Speed:   c.app.Speed,
			// 弹窗起播就跟随主窗的静音状态；主窗有声音时弹窗也应有声音
			// （无手势时引擎只放行静音起播，页面在第一次交互时再取消静音，
			// 见 frontend/video.html 的起播策略）。
			Muted:  c.app.Muted || c.app.Volume <= 0,
			Parent: c.app.Win,
		}
		if err := c.vid.Open(opts); err != nil {
			c.app.NotifyType("error", "打开视频窗口失败："+err.Error())
			c.restoreAudioAfterVideo(wasPlaying)
			return
		}
		c.app.Win.Update(func() { c.app.VideoOpen = true })
	}()
}

// restoreAudioAfterVideo 弹窗没开成时把音频恢复回来。
func (c *controller) restoreAudioAfterVideo(wasPlaying bool) {
	if wasPlaying {
		c.mp.Resume()
		c.app.Win.Update(func() { c.app.Playing = true })
	}
}

func (c *controller) closeVideo() {
	c.vid.Close()
	c.app.VideoOpen = false
	c.app.Win.Update(func() {})
}

// ---------------------------------------------------------------- 小工具

func toMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}

// parseDurText 把 "12:34" 或 "1:02:03" 转成秒。
func parseDurText(s string) int64 {
	if s == "" {
		return 0
	}
	parts := strings.Split(s, ":")
	var total int64
	for _, p := range parts {
		var n int64
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return 0
		}
		total = total*60 + n
	}
	return total
}

// parseCountText 把 "1.2万" 转回数字。
func parseCountText(s string) int64 {
	if s == "" {
		return 0
	}
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "亿"):
		mult, s = 1e8, strings.TrimSuffix(s, "亿")
	case strings.HasSuffix(s, "万"):
		mult, s = 1e4, strings.TrimSuffix(s, "万")
	}
	var f float64
	if _, err := fmt.Sscanf(s, "%f", &f); err != nil {
		return 0
	}
	return int64(f * float64(mult))
}

// sectionIndex 返回分区在导航里的下标。
func sectionIndex(key string) int {
	for i, s := range view.Sections {
		if s.Key == key {
			return i
		}
	}
	return 0
}
