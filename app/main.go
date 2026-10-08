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
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
	"github.com/vst93/bili-fm/app/internal/proxy"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/video"
	"github.com/vst93/bili-fm/app/internal/view"
)

const audioUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

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

	// 当前曲目的跳过分段
	sponsor   []bilibili.SponsorSegment
	skipped   map[int]bool
	sponsorOn bool
	lastPos   float64

	// quitting 为真表示真的在退出（而不是关窗到托盘）。
	quitting  bool
	incognito bool
}

func main() {
	shot := flag.String("shot", "", "启动后截图到该路径并退出")
	shotDelay := flag.Duration("shot-delay", 3*time.Second, "截图前的等待时间")
	searchOnStart := flag.String("search", "", "启动后立刻搜索该关键词")
	sectionOnStart := flag.String("section", "popular", "启动后加载的分区")
	noTray := flag.Bool("no-tray", false, "不装系统托盘（无头调试用）")
	noKeys := flag.Bool("no-keys", false, "不注册全局媒体键（无头调试用）")
	startMini := flag.Bool("mini", false, "以迷你模式启动（无头调试用）")
	flag.Parse()

	kv, err := store.OpenDefault()
	if err != nil {
		log.Fatalf("打开本地存储失败: %v", err)
	}
	log.Printf("%s", kv.Describe())
	bilibili.UseStore(kv)

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
	app.LoggedIn = bilibili.LoginStatus || app.UName != ""
	app.Speed = 1
	app.Volume = 1
	app.Sponsor = kv.String("sponsor_skip") == "1"
	c.sponsorOn = app.Sponsor

	c.wireActions()
	c.wirePlayer()
	c.restoreQueue()

	mygo.Bind(video.NewService(c.vid))
	mygo.Bind(&uiService{c: c})

	mygo.App.WhenReady(func() {
		app.Win = mygo.NewWindow(mygo.WindowOptions{
			Title:     "bili-FM",
			Width:     1000,
			Height:    660,
			MinWidth:  400,
			MinHeight: 155,
			Frameless: true,
			Content:   ui.View(app.Shell),
		})

		// 默认进第一个能用的分区：未登录时推荐/动态/收藏/历史都是空的。
		start := *sectionOnStart
		if start == "recommend" && !app.LoggedIn {
			start = "popular"
		}
		app.Section = sectionIndex(start)
		if *startMini {
			c.setMini(true)
		}
		if *searchOnStart != "" {
			app.Query = *searchOnStart
			c.search(*searchOnStart)
		} else {
			c.loadSection(start, 1)
		}

		// 关闭窗口 = 隐藏到托盘（与旧版一致），从托盘或 Dock 恢复。
		// quitting 为真时不再拦截，否则 App.Quit() 关不掉窗口、进程退不出。
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

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

// ---------------------------------------------------------------- 界面动作

func (c *controller) wireActions() {
	a := c.app
	a.Act = view.Actions{
		LoadSection:   c.loadSection,
		Search:        c.search,
		Play:          c.playIndex,
		TogglePlay:    c.togglePlay,
		Next:          func() { c.step(1) },
		Prev:          func() { c.step(-1) },
		Seek:          c.seek,
		SetSpeed:      func(v float64) { c.mp.SetSpeed(v) },
		ToggleEQ:      c.toggleEQ,
		ToggleSponsor: c.toggleSponsor,
		SetVolume:     func(v float64) { c.mp.SetVolume(v) },
		OpenVideo:     c.openVideo,
		CloseVideo:    c.closeVideo,
		OpenParts:     c.openParts,
		ToggleDanmaku: c.toggleDanmaku,
		LoadMore:      c.loadMore,
		Login:         c.login,
		Quit:          func() { mygo.App.Quit() },
		Minimize:      func() { a.Win.Minimize() },
	}
}

func (c *controller) wirePlayer() {
	c.mp.OnProgress(func(pos, dur float64) {
		c.app.Pos, c.app.Dur = pos, dur
		c.maybeSkipSponsor(pos)
		c.app.Win.Update(func() {})
	})
	c.mp.OnEnded(func() {
		c.app.Playing = false
		c.step(1)
	})
	c.mp.OnError(func(err error) {
		log.Printf("播放错误: %v", err)
		c.app.Playing = false
		c.app.Buffering = false
		c.app.Win.Update(func() {})
	})
}

// quit 真正退出应用：先把状态写盘，再关掉弹窗与播放器。
func (c *controller) quit() {
	c.quitting = true
	c.saveQueue()
	c.vid.Close()
	c.mp.Close()
	mygo.App.Quit()
}

// openInfo 打开「详情」面板：视频信息、互动状态、评论。
func (c *controller) openInfo() {
	a := c.app
	a.ShowInfo = !a.ShowInfo
	if !a.ShowInfo {
		a.Win.Update(func() {})
		return
	}
	t := a.Current()
	if t == nil {
		a.Win.Update(func() {})
		return
	}
	a.Win.Update(func() {})

	go func() {
		liked, _ := c.bl.HasLiked(t.Bvid)
		coined := false
		if n, err := c.bl.HasCoin(t.Bvid); err == nil && n > 0 {
			coined = true
		}
		faved, _ := c.bl.HasFavorite(t.Aid)
		followed := false
		if fs, err := c.bl.IsFollowing(int(midOf(c))); err == nil && fs != nil {
			followed = fs.IsFollowing
		}
		c.app.Win.Update(func() {
			c.app.Liked, c.app.Coined, c.app.Faved, c.app.Followed = liked, coined, faved, followed
		})

		if r, err := c.bl.GetReplyList(t.Aid, 1); err == nil && r != nil {
			list := make([]view.Comment, 0, len(r.Items))
			for _, it := range r.Items {
				user := ""
				if m := toMap(it.Member); m != nil {
					user = pickStr(m, "uname")
				}
				list = append(list, view.Comment{
					User:    user,
					Content: it.Content.Message,
					Likes:   int64(it.Like),
					Time:    time.Unix(it.SendTime, 0).Format("2006-01-02"),
				})
			}
			c.app.Win.Update(func() { c.app.Comments = list })
		}
	}()
}

// midOf 返回当前登录用户的 mid（用于判断是否已关注）。
func midOf(c *controller) int64 {
	var mid int64
	fmt.Sscanf(c.kv.String("mid"), "%d", &mid)
	return mid
}

func (c *controller) like() {
	t := c.app.Current()
	if t == nil {
		return
	}
	want := 1
	if c.app.Liked {
		want = 2 // 2 表示取消点赞
	}
	go func() {
		if _, err := c.bl.LikeVideo(t.Bvid, want); err != nil {
			log.Printf("点赞失败: %v", err)
			return
		}
		c.app.Win.Update(func() { c.app.Liked = !c.app.Liked })
	}()
}

func (c *controller) coin() {
	t := c.app.Current()
	if t == nil || c.app.Coined {
		return
	}
	go func() {
		if _, err := c.bl.CoinVideo(t.Bvid, 1); err != nil {
			log.Printf("投币失败: %v", err)
			return
		}
		c.app.Win.Update(func() { c.app.Coined = true })
	}()
}

func (c *controller) favorite() {
	t := c.app.Current()
	if t == nil {
		return
	}
	want := !c.app.Faved
	go func() {
		if err := c.bl.SetFavorite(t.Aid, want); err != nil {
			log.Printf("收藏失败: %v", err)
			return
		}
		c.app.Win.Update(func() { c.app.Faved = want })
	}()
}

func (c *controller) follow() {
	t := c.app.Current()
	if t == nil {
		return
	}
	mid := midOf(c)
	if mid == 0 {
		return
	}
	want := !c.app.Followed
	go func() {
		var err error
		if want {
			_, err = c.bl.Follow(int(mid))
		} else {
			_, err = c.bl.Unfollow(int(mid))
		}
		if err != nil {
			log.Printf("关注操作失败: %v", err)
			return
		}
		c.app.Win.Update(func() { c.app.Followed = want })
	}()
}

// setMini 切换迷你模式：换界面 + 调整窗口尺寸与置顶。
func (c *controller) setMini(on bool) {
	a := c.app
	a.Mini = on
	if on {
		w, h := view.MiniSize()
		a.Win.SetSize(w, h)
		a.Win.SetAlwaysOnTop(true)
	} else {
		a.Win.SetSize(1000, 660)
		a.Win.SetAlwaysOnTop(false)
	}
	a.Win.Update(func() {})
}

// saveQueue 把播放队列与当前位置写进本地存储（键名带 mygo_ 前缀，
// 不与旧版 playlist 格式冲突）。
func (c *controller) saveQueue() {
	if len(c.app.Queue) == 0 {
		return
	}
	b, err := json.Marshal(c.app.Queue)
	if err != nil {
		return
	}
	_ = c.kv.SetString("mygo_queue", string(b))
	_ = c.kv.SetString("mygo_index", fmt.Sprint(c.app.Index))
}

// restoreQueue 启动时恢复上次的队列。
func (c *controller) restoreQueue() {
	raw := c.kv.String("mygo_queue")
	if raw == "" {
		return
	}
	var q []view.Track
	if json.Unmarshal([]byte(raw), &q) != nil || len(q) == 0 {
		return
	}
	idx := 0
	fmt.Sscanf(c.kv.String("mygo_index"), "%d", &idx)
	if idx < 0 || idx >= len(q) {
		idx = 0
	}
	c.app.Queue, c.app.Index = q, idx
	c.app.Track = &c.app.Queue[idx]
	log.Printf("恢复播放列表 %d 条，当前第 %d 条", len(q), idx+1)
}

// ---------------------------------------------------------------- 列表

func (c *controller) loadSection(section string, page int) {
	c.app.Loading = true
	c.app.Status = "加载中…"
	c.app.Win.Update(func() {})

	go func() {
		cards, more, err := c.fetchSection(section, page)
		c.app.Win.Update(func() {
			c.app.Loading = false
			if err != nil {
				c.app.Status = "加载失败：" + err.Error()
				return
			}
			if page <= 1 {
				c.app.Cards = cards
			} else {
				c.app.Cards = append(c.app.Cards, cards...)
			}
			c.app.Page = page
			c.app.HasMore = more
			c.app.Status = fmt.Sprintf("%d 条", len(c.app.Cards))
		})
	}()
}

func (c *controller) fetchSection(section string, page int) ([]view.Card, bool, error) {
	switch section {
	case "recommend":
		l, err := c.bl.GetBLRCMDList(page)
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(l.Items), true, nil
	case "popular":
		l, err := c.bl.GetBLPopularList(page)
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(l.Items), l.HasMore, nil
	case "feed":
		l, err := c.bl.GetBLFeedList("")
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(l.Items), l.HasMore, nil
	case "favorite":
		folders, err := c.bl.GetBLFavFolderList()
		if err != nil {
			return nil, false, err
		}
		if len(folders) == 0 {
			return nil, false, nil
		}
		// 取第一个收藏夹（默认收藏夹在最前）。
		fid := pickInt(toMap(folders[0]), "id", "fid", "media_id")
		items, err := c.bl.GetBLFavFolderListDetail(int(fid), page)
		if err != nil {
			return nil, false, err
		}
		return toCardsFromRaw(items), true, nil
	case "history":
		l, err := c.bl.GetBLHistoryList(0, 0, "", 30)
		if err != nil {
			return nil, false, err
		}
		return toCards(jsonToAny(l.List)), false, nil
	case "watchlater":
		l, err := c.bl.GetWatchLaterList()
		if err != nil {
			return nil, false, err
		}
		return toCards(jsonToAny(l.List)), false, nil
	}
	return nil, false, fmt.Errorf("未知分区 %s", section)
}

func (c *controller) loadMore() {
	c.loadSection(c.app.CurrentSection().Key, c.app.Page+1)
}

func (c *controller) search(query string) {
	c.app.Loading = true
	c.app.Status = "搜索中…"
	c.app.Win.Update(func() {})
	go func() {
		results := c.bl.SearchVideo(query, "totalrank")
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
			})
		}
		c.app.Win.Update(func() {
			c.app.Loading = false
			c.app.Cards = toCards(items)
			c.app.HasMore = false
			c.app.Status = fmt.Sprintf("搜索「%s」：%d 条", query, len(c.app.Cards))
		})
	}()
}

// ---------------------------------------------------------------- 播放

// playIndex 播放当前列表里的第 index 条。
func (c *controller) playIndex(index int) {
	if index < 0 || index >= len(c.app.Cards) {
		return
	}
	// 把整个列表作为播放队列，这样上一首/下一首能连续播放。
	queue := make([]view.Track, 0, len(c.app.Cards))
	for _, card := range c.app.Cards {
		queue = append(queue, card.Track)
	}
	c.app.Queue = queue
	c.app.Index = index
	c.startCurrent()
}

// openParts 时把分集当作队列。
func (c *controller) openParts(t view.Track) {
	c.app.ShowParts = !c.app.ShowParts
	if !c.app.ShowParts || t.Bvid == "" {
		return
	}
	go func() {
		info := c.bl.GetCList(t.Bvid)
		parts := make([]view.Part, 0, len(info.Pages))
		queue := make([]view.Track, 0, len(info.Pages))
		for _, p := range info.Pages {
			parts = append(parts, view.Part{Cid: int64(p.Cid), Page: p.Page, Part: p.Part})
			queue = append(queue, view.Track{
				Aid: int64(info.Aid), Bvid: t.Bvid, Cid: int64(p.Cid),
				Title: t.Title, Up: t.Up, Cover: t.Cover,
				Duration: t.Duration, Part: p.Part,
			})
		}
		c.app.Win.Update(func() {
			c.app.Parts = parts
			c.app.Queue = queue
			for i, q := range queue {
				if q.Cid == t.Cid {
					c.app.Index = i
				}
			}
		})
	}()
}

// startCurrent 起播当前队列项：取播放地址 → 起播 → 拉弹幕与跳过分段。
func (c *controller) startCurrent() {
	t := c.app.Current()
	if t == nil {
		return
	}
	c.app.Buffering = true
	c.app.Status = "起播中…"
	c.app.ShowParts = false
	c.app.Danmaku = nil
	c.sponsor, c.skipped = nil, map[int]bool{}
	c.app.Win.Update(func() {})

	go func() {
		cid := t.Cid
		if cid == 0 {
			info := c.bl.GetCList(t.Bvid)
			if len(info.Pages) > 0 {
				cid = int64(info.Pages[0].Cid)
			}
		}
		if cid == 0 {
			c.app.Win.Update(func() {
				c.app.Buffering = false
				c.app.Status = "取分集失败"
			})
			return
		}
		u := c.bl.GetUrlByCid(int(t.Aid), int(cid))
		if u.URL == "" {
			c.app.Win.Update(func() {
				c.app.Buffering = false
				c.app.Status = "该视频暂时无法播放"
			})
			return
		}
		c.saveQueue()

		// 回填 cid 与分集标题
		c.app.Win.Update(func() {
			t.Cid = cid
			c.app.Track = t
			c.app.Pos, c.app.Dur = 0, 0
		})

		if err := c.mp.Play(u.URL, audioHeaders); err != nil {
			c.app.Win.Update(func() {
				c.app.Buffering = false
				c.app.Status = "起播失败：" + err.Error()
			})
			return
		}
		c.mp.SetSpeed(c.app.Speed)
		c.mp.SetEQ(c.app.EQ)
		c.mp.SetVolume(c.app.Volume)

		c.app.Win.Update(func() {
			c.app.Buffering = false
			c.app.Playing = true
			c.app.Dur = c.mp.Duration()
			c.app.Status = ""
		})

		// 弹幕与跳过分段：可选增强，失败不影响播放。
		if d, err := c.bl.GetDanmakuList(int(cid)); err == nil && d != nil {
			list := make([]view.Danmaku, 0, len(d.Items))
			for _, it := range d.Items {
				list = append(list, view.Danmaku{Time: it.Time, Text: it.Content})
			}
			c.app.Win.Update(func() { c.app.Danmaku = list })
		}
		if c.sponsorOn {
			segs := c.bl.GetSponsorSegments(t.Bvid, cid)
			c.app.Win.Update(func() { c.sponsor = segs })
		}
	}()
}

func (c *controller) togglePlay() {
	if c.app.Current() == nil {
		return
	}
	if c.mp.Playing() {
		c.mp.Pause()
		c.app.Playing = false
	} else {
		c.mp.Resume()
		c.app.Playing = true
	}
	c.app.Win.Update(func() {})
}

func (c *controller) step(delta int) {
	if len(c.app.Queue) == 0 {
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
	go func() {
		if err := c.mp.Seek(seconds); err != nil {
			log.Printf("跳转失败: %v", err)
			return
		}
		c.app.Win.Update(func() {
			c.app.Pos = seconds
			c.skipped = map[int]bool{}
		})
	}()
}

func (c *controller) toggleEQ() {
	c.app.EQ = !c.app.EQ
	c.mp.SetEQ(c.app.EQ)
	c.app.Win.Update(func() {})
}

func (c *controller) toggleSponsor() {
	c.app.Sponsor = !c.app.Sponsor
	c.sponsorOn = c.app.Sponsor
	if c.app.Sponsor {
		_ = c.kv.SetString("sponsor_skip", "1")
		if t := c.app.Current(); t != nil && t.Cid != 0 {
			segs := c.bl.GetSponsorSegments(t.Bvid, t.Cid)
			c.app.Win.Update(func() { c.sponsor = segs })
		}
	} else {
		_ = c.kv.SetString("sponsor_skip", "0")
	}
	c.app.Win.Update(func() {})
}

func (c *controller) toggleDanmaku() {
	c.app.ShowDanmaku = !c.app.ShowDanmaku
	if c.app.ShowDanmaku && len(c.app.Danmaku) == 0 {
		if t := c.app.Current(); t != nil && t.Cid != 0 {
			go func() {
				d, err := c.bl.GetDanmakuList(int(t.Cid))
				if err != nil || d == nil {
					return
				}
				list := make([]view.Danmaku, 0, len(d.Items))
				for _, it := range d.Items {
					list = append(list, view.Danmaku{Time: it.Time, Text: it.Content})
				}
				c.app.Win.Update(func() { c.app.Danmaku = list })
			}()
		}
	}
	c.app.Win.Update(func() {})
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
			c.seek(s.End)
			return
		}
	}
}

// ---------------------------------------------------------------- 视频弹窗

func (c *controller) openVideo() {
	t := c.app.Current()
	if t == nil || t.Cid == 0 {
		c.app.Status = "先选一个视频"
		c.app.Win.Update(func() {})
		return
	}
	go func() {
		// 弹窗要的是渐进式 MP4 地址，走本地代理（带 Referer）。
		u := c.bl.GetUrlByCid(int(t.Aid), int(t.Cid))
		if u.URL == "" {
			c.app.Win.Update(func() { c.app.Status = "取播放地址失败" })
			return
		}
		opts := video.Options{
			Src:     proxy.AudioURL(u.URL),
			Title:   t.Title,
			Poster:  proxy.ImageURL(t.Cover),
			StartAt: c.app.Pos,
			Speed:   c.app.Speed,
			Parent:  c.app.Win,
		}
		if err := c.vid.Open(opts); err != nil {
			c.app.Win.Update(func() { c.app.Status = "打开视频窗口失败：" + err.Error() })
			return
		}
		c.app.Win.Update(func() { c.app.VideoOpen = true })
	}()
}

func (c *controller) closeVideo() {
	c.vid.Close()
	c.app.VideoOpen = false
	c.app.Win.Update(func() {})
}

// ---------------------------------------------------------------- 登录

func (c *controller) login() {
	c.app.Status = "请在手机上确认登录…"
	c.app.Win.Update(func() {})
	go func() {
		qr, err := c.bl.GetLoginQRCode()
		if err != nil {
			c.app.Win.Update(func() { c.app.Status = "取二维码失败：" + err.Error() })
			return
		}
		c.app.Win.Update(func() { c.app.Status = "扫码地址：" + qr })
		// 轮询登录状态
		for i := 0; i < 90; i++ {
			time.Sleep(2 * time.Second)
			if c.bl.GetLoginQRCodeStatus() {
				bilibili.LoginStatus = true
				info := c.bl.GetBLUserInfo()
				c.app.Win.Update(func() {
					c.app.LoggedIn = true
					if info != nil {
						c.app.UName = info.Uname
					}
					c.app.Status = "登录成功"
				})
				c.loadSection(c.app.CurrentSection().Key, 1)
				return
			}
		}
		c.app.Win.Update(func() { c.app.Status = "登录超时" })
	}()
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

// uiService 是暴露给页面的杂项服务（目前只有平台信息与版本）。
type uiService struct{ c *controller }

func (s *uiService) Log(msg string) { log.Print("页面 | " + msg) }

// sectionIndex 返回分区在导航里的下标。
func sectionIndex(key string) int {
	for i, s := range view.Sections {
		if s.Key == key {
			return i
		}
	}
	return 0
}
