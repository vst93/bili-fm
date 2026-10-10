package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"

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
}

// savePlaylists 把两个列表写盘。
func (c *controller) savePlaylists() {
	a := c.app
	if b, err := json.Marshal(a.Playlist); err == nil {
		_ = c.kv.SetString(kvUserPlaylist, string(b))
	}
	if b, err := json.Marshal(a.SeriesPlaylist); err == nil {
		_ = c.kv.SetString(kvSeriesPlaylist, string(b))
	}
	_ = c.kv.SetString(kvPlayMode, a.PlayMode)
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
func (c *controller) playPlaylist(list string, index int) {
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
	c.saveQueue()
	c.startCurrent()
	// startCurrent 会把抽屉关掉；播放列表要在播放期间保持打开。
	a.Drawer = view.DrawerPlaylist
	a.Win.Update(func() {})
}

// trackFromPlayItem 把播放列表记录转成播放队列条目。
func trackFromPlayItem(it view.PlayItem) view.Track {
	return view.Track{
		Aid: it.Aid, Bvid: it.Bvid, Cid: it.Cid,
		Title: it.Title, Cover: it.Pic, Part: it.Part,
	}
}

// addToPlaylist 把一集加入「我的列表」（已存在则提示）。
func (c *controller) addToPlaylist(part view.Part) {
	a := c.app
	if a.Info == nil || part.Cid == 0 {
		return
	}
	for _, it := range a.Playlist {
		if it.Cid == part.Cid {
			a.Notify("该选集已在播放列表中")
			return
		}
	}
	a.Playlist = append(a.Playlist, view.PlayItem{
		ID:         fmt.Sprintf("%s-%d", a.Info.Bvid, part.Cid),
		Bvid:       a.Info.Bvid,
		Aid:        a.Info.Aid,
		Cid:        part.Cid,
		Part:       part.Part,
		FirstFrame: part.FirstFrame,
		Title:      a.Info.Title,
		Pic:        a.Info.Pic,
	})
	c.savePlaylists()
	a.NotifyType("success", "已添加到播放列表")
	a.Win.Update(func() {})
}

// addAllToPlaylist 把当前视频的全部选集加入「我的列表」（自动去重）。
func (c *controller) addAllToPlaylist() {
	a := c.app
	if a.Info == nil || len(a.Info.Parts) == 0 {
		return
	}
	existing := map[int64]bool{}
	for _, it := range a.Playlist {
		existing[it.Cid] = true
	}
	added := 0
	for _, p := range a.Info.Parts {
		if existing[p.Cid] {
			continue
		}
		a.Playlist = append(a.Playlist, view.PlayItem{
			ID:         fmt.Sprintf("%s-%d", a.Info.Bvid, p.Cid),
			Bvid:       a.Info.Bvid,
			Aid:        a.Info.Aid,
			Cid:        p.Cid,
			Part:       p.Part,
			FirstFrame: p.FirstFrame,
			Title:      a.Info.Title,
			Pic:        a.Info.Pic,
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
	c.saveQueue()
	a.Win.Update(func() {})
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

// seriesPlayAll 把当前合集里的视频全部加载进「合集列表」并从第一条开始播放。
//
// 合集的列表接口只给 bvid/aid，没有 cid，所以要逐个拉详情（GetCList）取第一
// 集的 cid。分页 + 并发受控，失败的单条跳过。
func (c *controller) seriesPlayAll() {
	a := c.app
	if a.SeriesID == 0 {
		a.NotifyType("warning", "请先到 UP 空间选择一个合集")
		return
	}
	a.Notify("正在加载合集…")
	a.Win.Update(func() {})

	mid, seriesID := a.UpMid, a.SeriesID
	name := a.SeriesName
	go func() {
		// 1. 拉全部分页的合集视频。
		var archives []struct {
			bvid  string
			pic   string
			title string
		}
		seen := map[string]bool{}
		for page := 1; page <= 50; page++ {
			list, err := c.bl.GetSeriesVideos(int(mid), int(seriesID), page)
			if err != nil || len(list) == 0 {
				break
			}
			added := 0
			for _, ar := range list {
				if seen[ar.Bvid] {
					continue
				}
				seen[ar.Bvid] = true
				archives = append(archives, struct {
					bvid  string
					pic   string
					title string
				}{ar.Bvid, ar.Pic, ar.Title})
				added++
			}
			if added == 0 {
				break
			}
		}
		if len(archives) == 0 {
			a.NotifyType("error", "合集里没有可播放的视频")
			return
		}
		// 2. 并发拉详情取第一集 cid（最多 6 个并发）。
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
			a.NotifyType("error", "合集里的视频暂时都无法播放")
			return
		}
		a.Win.Update(func() {
			a.SeriesPlaylist = out
			a.PlaylistTab = view.ListSeries
			a.PlayMode = view.PlayModeSequence
			a.SeriesName = name
			c.savePlaylists()
			a.NotifyType("success", fmt.Sprintf("已加载 %d 集到播放列表", len(out)))
			// 起播要在主线程做（它会改界面状态）。
			c.playPlaylist(view.ListSeries, 0)
		})
	}()
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
