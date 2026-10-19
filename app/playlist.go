package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"sync"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

// 播放列表（原版 playlist / seriesPlaylist）。
//
// 「我的列表」是用户从选集面板手动添加的；「合集列表」是从合集「播放全部」
// 加载的。两者都落盘；播放模式（顺序 / 单曲循环 / 随机）也落盘。
const (
	kvUserPlaylist   = "userPlaylist"
	kvSeriesPlaylist = "seriesPlaylist"
	kvPlayMode       = "playlistPlayMode"
	kvPlayingList    = "playingPlaylistType"
	kvPlayingIndex   = "playingPlaylistIndex"
)

// loadPlaylists 启动时读回播放列表与播放模式（原版从 dkv / localStorage 读）。
func (c *controller) loadPlaylists() {
	a := c.app
	if raw := c.kv.String(kvUserPlaylist); raw != "" {
		_ = json.Unmarshal([]byte(raw), &a.Playlist)
	}
	if raw := c.kv.String(kvSeriesPlaylist); raw != "" {
		_ = json.Unmarshal([]byte(raw), &a.SeriesPlaylist)
	}
	if m := c.kv.String(kvPlayMode); m == view.PlayModeSingle || m == view.PlayModeShuffle {
		a.PlayMode = m
	}
	// 正在播放的来源/下标：只在不与恢复的队列冲突时用（恢复队列后 Index 已指向那条）。
	if src := c.kv.String(kvPlayingList); src == view.ListUser || src == view.ListSeries {
		a.PlayingPlaylist = src
		if idx, err := strconv.Atoi(c.kv.String(kvPlayingIndex)); err == nil {
			a.PlaylistTab = src
			if idx >= 0 && idx < len(c.playlistOf(src)) {
				a.Index = idx
			}
		}
	}
}

// savePlaylists 把两个列表写盘（含各自正在播放的下标，原版也一并持久化）。
func (c *controller) savePlaylists() {
	a := c.app
	if b, err := json.Marshal(a.Playlist); err == nil {
		_ = c.kv.SetString(kvUserPlaylist, string(b))
	}
	if b, err := json.Marshal(a.SeriesPlaylist); err == nil {
		_ = c.kv.SetString(kvSeriesPlaylist, string(b))
	}
	_ = c.kv.SetString(kvPlayMode, a.PlayMode)
	// 正在播放的来源与下标：恢复时播放列表抽屉能标记到当时那条。
	_ = c.kv.SetString(kvPlayingList, a.PlayingPlaylist)
	_ = c.kv.SetString(kvPlayingIndex, strconv.Itoa(a.Index))
}

// playlistOf 返回某个来源的记录切片。
func (c *controller) playlistOf(list string) []view.PlayItem {
	if list == view.ListSeries {
		return c.app.SeriesPlaylist
	}
	return c.app.Playlist
}

// playPlaylist 播放某个列表里的第 index 条：把整个列表作为播放队列。
//
// 原版是「播放列表模式」（isPlaylistMode），上一首/下一首和播完自动续播都
// 在这个列表里走。这里用 App.PlayingPlaylist 表示当前来源。
// keepDrawer 表示「播完保持播放列表抽屉打开」—— 只有用户**在抽屉里点某一行**
// 时才是这样（他要接着往下挑）。自动切下一首、合集播放全部这些都不该弹抽屉。
func (c *controller) playPlaylist(list string, index int, keepDrawer bool) {
	a := c.app
	items := c.playlistOf(list)
	if index < 0 || index >= len(items) {
		return
	}
	queue := make([]view.Track, 0, len(items))
	for _, it := range items {
		queue = append(queue, trackFromPlayItem(it))
	}
	a.Queue = queue
	a.Index = index
	a.PlayingPlaylist = list
	a.PlaylistTab = list
	// 手动从播放列表点一条：原版 handlePlaylistVideoSelect 在 !preserveUi 时
	// setVideoInfo（浏览中 = 这条），弹幕 / 评论抽屉与选集抽屉都跟着它。
	// 自动切歌（preserveUi，keepDrawer=false）不动 videoInfo，这里也不动。
	if keepDrawer {
		it := items[index]
		a.BrowseInfo = browseInfoFromPlayItem(it)
	}
	c.startCurrent()
	if keepDrawer {
		// startCurrent 会把抽屉关掉；从抽屉点进来的要让它保持打开。
		a.Drawer = view.DrawerPlaylist
	}
	a.Win.Update(func() {})
}

// trackFromPlayItem 把播放列表记录转成播放队列条目。
func trackFromPlayItem(it view.PlayItem) view.Track {
	return view.Track{
		Aid: it.Aid, Bvid: it.Bvid, Cid: it.Cid,
		Title: it.Title, Cover: it.Pic, Part: it.Part,
	}
}

// browseInfoFromPlayItem 把播放列表记录转成「浏览中」的信息。手动从播放列表
// 点一条时写 BrowseInfo（原版 handlePlaylistVideoSelect 的 setVideoInfo），
// 弹幕 / 评论抽屉与选集抽屉都跟着它；自动切歌不写。
func browseInfoFromPlayItem(it view.PlayItem) *view.Info {
	return &view.Info{
		Bvid: it.Bvid, Aid: it.Aid, Cid: it.Cid,
		Title: it.Title, Pic: it.Pic,
	}
}

// addToPlaylist 把一集加入「我的列表」（已存在则提示）。
// part 属于「浏览中」的那条视频（选集抽屉里的卡片），所以 bvid / 标题
// 取 Browse() 而不是正在播放的 a.Info。
func (c *controller) addToPlaylist(part view.Part) {
	a := c.app
	bi := a.Browse()
	if bi == nil || part.Cid == 0 {
		return
	}
	for _, it := range a.Playlist {
		if it.Cid == part.Cid {
			a.Notify("该选集已在播放列表中")
			return
		}
	}
	a.Playlist = append(a.Playlist, view.PlayItem{
		ID:         fmt.Sprintf("%s-%d", bi.Bvid, part.Cid),
		Bvid:       bi.Bvid,
		Aid:        bi.Aid,
		Cid:        part.Cid,
		Part:       part.Part,
		FirstFrame: part.FirstFrame,
		Title:      bi.Title,
		Pic:        bi.Pic,
	})
	c.savePlaylists()
	a.NotifyType("success", "已添加到播放列表")
	a.Win.Update(func() {})
}

// addAllToPlaylist 把当前视频的全部选集加入「我的列表」（自动去重）。
// 选集来自「浏览中」的那条（BrowseInfo），与选集抽屉一致。
func (c *controller) addAllToPlaylist() {
	a := c.app
	bi := a.Browse()
	if bi == nil || len(bi.Parts) == 0 {
		return
	}
	existing := map[int64]bool{}
	for _, it := range a.Playlist {
		existing[it.Cid] = true
	}
	added := 0
	for _, p := range bi.Parts {
		if existing[p.Cid] {
			continue
		}
		a.Playlist = append(a.Playlist, view.PlayItem{
			ID:         fmt.Sprintf("%s-%d", bi.Bvid, p.Cid),
			Bvid:       bi.Bvid,
			Aid:        bi.Aid,
			Cid:        p.Cid,
			Part:       p.Part,
			FirstFrame: p.FirstFrame,
			Title:      bi.Title,
			Pic:        bi.Pic,
		})
		added++
	}
	if added == 0 {
		a.Notify("所有选集已在播放列表中")
		return
	}
	c.savePlaylists()
	a.NotifyType("success", fmt.Sprintf("已添加 %d 集到播放列表", added))
	a.Win.Update(func() {})
}

// deletePlaylistItem 从「我的列表」删一条。
func (c *controller) deletePlaylistItem(list, id string) {
	a := c.app
	if list != view.ListUser {
		return
	}
	idx := -1
	for i, it := range a.Playlist {
		if it.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return
	}
	a.Playlist = append(a.Playlist[:idx:idx], a.Playlist[idx+1:]...)
	// 正在播的就是这条：退出播放列表模式。
	if a.PlayingPlaylist == view.ListUser && a.Index == idx {
		a.PlayingPlaylist = ""
	}
	if a.Index > idx {
		a.Index--
	}
	c.savePlaylists()
	a.Win.Update(func() {})
}

// reorderIndex 复刻原版 handlePlaylistReorder 对 currentPlaylistIndex 的跟随
// （main:src/pages/index.tsx）：
//
//	if (prevIdx === from) return to;
//	if (from < prevIdx && to >= prevIdx) return prevIdx - 1;
//	if (from > prevIdx && to <= prevIdx) return prevIdx + 1;
//	return prevIdx;
//
// 正在播的那条被拖走 → 下标跟着它到新位置；只是被别的记录跨过 → 顺势前后
// 挪一格；没被碰到就不动。
func reorderIndex(cur, from, to int) int {
	switch {
	case cur == from:
		return to
	case from < cur && to >= cur:
		return cur - 1
	case from > cur && to <= cur:
		return cur + 1
	}
	return cur
}

// reorderPlaylist 把「我的列表」第 from 条移到 to。
func (c *controller) reorderPlaylist(list string, from, to int) {
	a := c.app
	if list != view.ListUser {
		return
	}
	n := len(a.Playlist)
	if from < 0 || from >= n || to < 0 || to >= n || from == to {
		return
	}
	it := a.Playlist[from]
	a.Playlist = append(a.Playlist[:from:from], a.Playlist[from+1:]...)
	rest := append(a.Playlist[:to:to], it)
	a.Playlist = append(rest, a.Playlist[to:]...)
	// 与原版一致：当前播放的那条要跟着这次移动改下标。只有当前播放来源就是
	// 「我的列表」时 a.Index 才是这个列表的下标（播合集 / 单集时它是队列下标，
	// 拖我的列表与它无关）。缺这一步的表现：拖完行，「当前播放」高亮停在旧
	// 下标（那里已经是另一条记录），下一首/上一首与「定位当前」都跟着走错。
	if a.PlayingPlaylist == view.ListUser {
		a.Index = reorderIndex(a.Index, from, to)
	}
	c.savePlaylists()
	a.Win.Update(func() {})
}

// clearPlaylist 清空「我的列表」。
func (c *controller) clearPlaylist(list string) {
	a := c.app
	if list != view.ListUser {
		return
	}
	a.Playlist = nil
	if a.PlayingPlaylist == view.ListUser {
		a.PlayingPlaylist = ""
	}
	c.savePlaylists()
	a.Win.Update(func() {})
}

// cyclePlayMode 顺序 → 单曲循环 → 随机 → 顺序。
func (c *controller) cyclePlayMode() {
	a := c.app
	switch a.PlayMode {
	case view.PlayModeSequence:
		a.PlayMode = view.PlayModeSingle
	case view.PlayModeSingle:
		a.PlayMode = view.PlayModeShuffle
	default:
		a.PlayMode = view.PlayModeSequence
	}
	_ = c.kv.SetString(kvPlayMode, a.PlayMode)
	if c.media != nil {
		c.syncMediaTrack()
	}
	a.Win.Update(func() {})
}

// switchPlaylistTab 切换播放列表抽屉的表头 tab。
func (c *controller) switchPlaylistTab(list string) {
	c.app.PlaylistTab = list
	c.app.Win.Update(func() {})
}

// seriesPlayAllBlocked 报告这次「播放全部」是否该被忽略：已经有一次全量
// 拉取在途时连点忽略（原版 seriesList 的「播放全部」在 isPlayingAll 期间禁用）。
// 抽成纯函数便于单测。
func seriesPlayAllBlocked(pending bool) bool { return pending }

// seriesPlayAll 把当前合集里的视频全部加载进「合集列表」并从第一条开始播放。
//
// 合集的列表接口只给 bvid/aid，没有 cid，所以要逐个拉详情（GetCList）取第一
// 集的 cid。分页 + 并发受控，失败的单条**跳过并各弹一条 warning**（原版
// index.tsx:1733 的 `视频 ${video.title} 加载失败，已跳过`，此前是静默丢）。
func (c *controller) seriesPlayAll() {
	a := c.app
	if a.SeriesID == 0 {
		// 原版 index.tsx:2383 的文案（提示用户去哪里选合集，而不是只说「没选」）。
		a.NotifyType("warning", "请先点击UP主头像或昵称，选择一个合集")
		return
	}
	// 已经有一次全量拉取在途：连点忽略（原版 seriesList 的 isPlayingAll 让
	// 「播放全部」在请求期间禁用，否则每次点击都会各起一套分页 + 详情请求）。
	if seriesPlayAllBlocked(a.SeriesPlayAllPending) {
		return
	}
	a.SeriesPlayAllPending = true
	// 原版是 info toast「正在加载合集...」（三个 ASCII 点，不是省略号）。
	a.Notify("正在加载合集...")
	a.Win.Update(func() {})

	mid, seriesID := a.UpMid, a.SeriesID
	name := a.SeriesName
	go func() {
		// 不管成败都要清 pending，否则失败时按钮会永远转圈。
		defer func() {
			a.Win.Update(func() { a.SeriesPlayAllPending = false })
		}()
		// 1. 起点是合集抽屉里**已经加载**的那些视频（原版 handleSeriesPlayAll
		//    的 loadedVideos 参数），只从「已加载到的下一页」往后翻——此前无条件
		//    从第 1 页重拉，滚过 5 页的合集就白拉 5 次分页。
		drawerList := a.ListFor(view.DrawerSeries)
		archives, seen, nextPage := seriesSeedArchives(drawerList)
		// 2. 往后翻页补齐（原版不设页数上限：靠「这一页没带回新条目」兜住
		//    接口重复返回同一页的死循环）。
		var fetched []bilibili.SeriesVideosResponse_Archive
		lastPage := nextPage - 1
		for page := nextPage; ; page++ {
			pageVideos, err := c.bl.GetSeriesVideos(int(mid), int(seriesID), page)
			if err != nil {
				// 原版这段分页请求抛异常时整段 catch 掉并弹 error toast
				// （index.tsx:1758 的「加载合集失败」）。
				a.NotifyType("error", actionFailText("加载合集", err))
				return
			}
			if len(pageVideos) == 0 {
				break
			}
			added := 0
			for _, ar := range pageVideos {
				if seen[ar.Bvid] {
					continue
				}
				seen[ar.Bvid] = true
				archives = append(archives, seriesArchive{bvid: ar.Bvid, pic: ar.Pic, title: ar.Title})
				fetched = append(fetched, ar)
				added++
			}
			lastPage = page
			if added == 0 {
				break
			}
		}
		if len(archives) == 0 {
			a.NotifyType("error", "合集里没有可播放的视频")
			return
		}
		// 3. 并发拉详情取第一集 cid（最多 6 个并发）。
		items := make([]view.PlayItem, len(archives))
		var wg sync.WaitGroup
		sem := make(chan struct{}, 6)
		for i := range archives {
			wg.Add(1)
			sem <- struct{}{}
			go func(i int) {
				defer wg.Done()
				defer func() { <-sem }()
				ar := archives[i]
				vi := c.bl.GetCList(ar.bvid)
				if len(vi.Pages) == 0 {
					// 原版这条是 warning toast 而不是 console.error：用户点「播放全部」
					// 后要能看出「哪一条被跳过了」，否则只会觉得合集少了几集。
					a.NotifyType("warning", seriesSkipMessage(ar.title, ar.bvid))
					return
				}
				p := vi.Pages[0]
				items[i] = view.PlayItem{
					ID:         fmt.Sprintf("%s-%d", vi.Bvid, p.Cid),
					Bvid:       vi.Bvid,
					Aid:        int64(vi.Aid),
					Cid:        int64(p.Cid),
					Part:       firstNonEmptyStr(p.Part, ar.title),
					FirstFrame: firstNonEmptyStr(p.FirstFrame, ar.pic),
					Title:      firstNonEmptyStr(vi.Title, ar.title),
					Pic:        firstNonEmptyStr(vi.Pic, ar.pic),
				}
			}(i)
		}
		wg.Wait()
		// 去掉拉详情失败的空条目。
		out := make([]view.PlayItem, 0, len(items))
		for _, it := range items {
			if it.Cid != 0 {
				out = append(out, it)
			}
		}
		if len(out) == 0 {
			a.NotifyType("error", "合集中的视频暂时都无法播放")
			return
		}
		a.Win.Update(func() {
			a.SeriesPlaylist = out
			a.PlaylistTab = view.ListSeries
			a.PlayMode = view.PlayModeSequence
			a.SeriesName = name
			// 原版末尾的 setSeriesVideos(allVideos) + setSeriesVideosPage(
			// max(seriesVideosPage, nextPage-1))：把这次补拉的分页并进合集
			// 抽屉，抽屉里也就有了全集（不用再逐页翻到底）。
			if len(fetched) > 0 {
				cards := make([]view.Card, 0, len(fetched))
				for _, ar := range fetched {
					cards = append(cards, seriesCard(ar, a.UpName))
				}
				setCards(drawerList, cards, false)
			}
			if lastPage > drawerList.Page {
				drawerList.Page = lastPage
			}
			// 上面那两个 break 都意味着「已经到底了」（空页 / 整页都是
			// 已见过的条目），所以抽屉不必再逐页翻。
			drawerList.HasMore = false
			drawerList.LoadErr = false
			// 「浏览中」跟着换到第一集：原版 handleSeriesPlayAll 走
			// handlePlaylistVideoSelect(0, …, "series") 且 preserveUi=false，
			// 那条路径会 setVideoInfo(第一集)。少了这一笔，弹幕/评论与选集抽屉
			// 会继续用**上一次浏览**的那条视频（BrowseInfo 陈旧），显示错内容。
			// 自动切歌（playPlaylist 的 keepDrawer=false）不该写，所以只能在这里补。
			a.BrowseInfo = browseInfoFromPlayItem(out[0])
			c.savePlaylists()
			a.NotifyType("success", fmt.Sprintf("已加载 %d 集到播放列表", len(out)))
			// 起播要在主线程做（它会改界面状态）。
			c.playPlaylist(view.ListSeries, 0, false)
		})
	}()
}

// seriesArchive 是「播放全部」拼队列时用的一条合集视频：列表接口只给
// bvid / 封面 / 标题，cid 要逐条拉详情（GetCList）取第一集。
type seriesArchive struct {
	bvid  string
	pic   string
	title string
}

// seriesSeedArchives 决定「播放全部」从哪儿起步：原版 handleSeriesPlayAll 接的
// 是合集抽屉**当前已加载**的 seriesVideos（`loadedVideos` 参数），只从
// seriesVideosPage+1 往后翻页，不重拉第 1 页。
//
// 只有抽屉里确实装着这个合集的前缀时才敢用：`Page <= 0`（还没加载过）或
// `Dropped`（卡片曾因上限从头部释放，手里只剩尾部窗口）都退回第 1 页重拉。
func seriesSeedArchives(list *view.List) ([]seriesArchive, map[string]bool, int) {
	if list == nil || list.Page <= 0 || list.Dropped || len(list.Cards) == 0 {
		return nil, map[string]bool{}, 1
	}
	archives := make([]seriesArchive, 0, len(list.Cards))
	seen := make(map[string]bool, len(list.Cards))
	for _, cd := range list.Cards {
		if cd.Bvid == "" || seen[cd.Bvid] {
			continue
		}
		seen[cd.Bvid] = true
		archives = append(archives, seriesArchive{bvid: cd.Bvid, pic: cd.Cover, title: cd.Title})
	}
	if len(archives) == 0 {
		return nil, map[string]bool{}, 1
	}
	return archives, seen, list.Page + 1
}

// seriesSkipMessage 是「合集播放全部」里单条拉详情失败时的 warning 文案。
//
// 逐字对齐原版 index.tsx:1733 的 `` `视频 ${video.title} 加载失败，已跳过` ``；
// 标题缺失时退 bvid（原版会渲染出「视频 undefined」，这里给个能定位的串）。
func seriesSkipMessage(title, bvid string) string {
	return "视频 " + firstNonEmptyStr(title, bvid) + " 加载失败，已跳过"
}

func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// randomOtherIndex 返回一个不同于 current 的随机下标。
func randomOtherIndex(n, current int) int {
	if n <= 1 {
		return 0
	}
	next := rand.Intn(n)
	for next == current {
		next = rand.Intn(n)
	}
	return next
}
