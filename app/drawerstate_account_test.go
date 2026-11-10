package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/vst93/bili-fm/app/internal/view"
)

// 换账号后浏览数据必须整体作废旧代号的请求，播放列表 / 播放状态保持不动。
// 这条直接跑 controller 真正调用的入口，而不是只看源码里有没有那句调用。
func TestAccountInvalidationDropsBrowsingKeepsPlayback(t *testing.T) {
	a := view.NewApp(func() {})
	c := &controller{app: a}

	keys := []string{"feed", "popular", "favorite", view.DrawerSearch, view.DrawerUp, view.DrawerSeries}
	oldCtx := make(map[string]string, len(keys))
	oldGen := make(map[string]int, len(keys))
	for i, key := range keys {
		l := a.ListFor(key)
		l.Context = "old-" + key
		l.Gen = 100 + i
		l.Initialized = true
		l.Cards = []view.Card{{Bvid: "OLD" + key, Title: "旧账号内容"}}
		l.Loading = i%2 == 0
		l.Page = 5
		l.HasMore = true
		l.Dropped = true
		l.GridScroll.Y = 320
		l.Scroll.Y = 160
		oldCtx[key], oldGen[key] = l.Context, l.Gen
	}

	a.Folders = []view.Folder{{ID: 7, Title: "旧收藏夹", Count: 3}}
	a.FolderID = 7
	a.UpOffset = "up-offset"
	a.FeedOffset = "feed-offset"
	a.HistCursor = view.HistCursor{Max: 9, ViewAt: 123, Business: "archive"}
	a.SeriesList = []view.Series{{ID: 8, Title: "旧合集"}}
	a.UpFollowPending = true
	a.UpFollowed = true
	a.UpFans = 99
	a.SearchRequestID = 42
	genBefore := a.BrowseGeneration

	// 播放列表与播放状态不属于浏览列表，换账号必须原样保留。
	a.Playlist = []view.PlayItem{{ID: "p1", Bvid: "BVP"}}
	a.SeriesPlaylist = []view.PlayItem{{ID: "s1", Bvid: "BVS"}}
	a.PlayingPlaylist = view.ListUser
	a.Track = &view.Track{Bvid: "BVplaying", Title: "正在播"}
	a.Queue = []view.Track{{Bvid: "BVplaying"}}
	a.Index = 0
	wantPlaylist := append([]view.PlayItem(nil), a.Playlist...)
	wantSeries := append([]view.PlayItem(nil), a.SeriesPlaylist...)
	wantTrack := *a.Track

	c.invalidateBrowsingForAccount()

	if a.BrowseGeneration == genBefore {
		t.Fatal("换账号没有递增 BrowseGeneration")
	}
	if a.SearchRequestID <= 42 {
		t.Fatalf("换账号没有作废在途搜索：SearchRequestID=%d", a.SearchRequestID)
	}
	for _, key := range keys {
		l := a.ListFor(key)
		if l.Cards != nil || l.Context != "" || l.Initialized || l.Loading || l.Page != 0 ||
			l.HasMore || l.Dropped || l.LoadErr || l.GridScroll.Y != 0 || l.Scroll.Y != 0 {
			t.Errorf("%s 没被清干净：%+v", key, l)
		}
		if l.Gen <= oldGen[key] {
			t.Errorf("%s 的请求代号没前进：%d ≤ %d", key, l.Gen, oldGen[key])
		}
		// 旧账号请求即使现在才回来，也不得清 Loading / 标成已初始化。
		if l.FinishLoad(oldCtx[key], oldGen[key], true, true) {
			t.Errorf("%s 接受了旧账号的完成结果", key)
		}
		if l.Cards != nil || l.Loading {
			t.Errorf("%s 被旧结果写脏：%+v", key, l)
		}
	}
	if a.Folders != nil || a.FolderID != 0 {
		t.Errorf("收藏夹元数据没清：folders=%v fid=%d", a.Folders, a.FolderID)
	}
	if a.UpOffset != "" || a.FeedOffset != "" || a.HistCursor != (view.HistCursor{}) || a.SeriesList != nil {
		t.Errorf("分页游标 / 合集元数据没清：up=%q feed=%q hist=%+v series=%v",
			a.UpOffset, a.FeedOffset, a.HistCursor, a.SeriesList)
	}
	if a.UpFollowPending || a.UpFollowed || a.UpFans != 0 {
		t.Errorf("UP 关注状态没清：pending=%v followed=%v fans=%d",
			a.UpFollowPending, a.UpFollowed, a.UpFans)
	}

	if !reflect.DeepEqual(a.Playlist, wantPlaylist) {
		t.Errorf("换账号动了播放列表：%+v", a.Playlist)
	}
	if !reflect.DeepEqual(a.SeriesPlaylist, wantSeries) {
		t.Errorf("换账号动了合集播放列表：%+v", a.SeriesPlaylist)
	}
	if a.Track == nil || *a.Track != wantTrack || a.PlayingPlaylist != view.ListUser || a.Index != 0 {
		t.Errorf("换账号动了播放状态：track=%+v playlist=%q index=%d", a.Track, a.PlayingPlaylist, a.Index)
	}
}

// 同一列表换了 tab / 搜索词后，旧代号的响应不能覆盖新上下文。
func TestStaleListResultAfterContextChange(t *testing.T) {
	a := view.NewApp(func() {})
	l := a.ListFor("popular")

	hotCtx := a.ListContext("popular")
	hotGen := l.BeginFirstPage(hotCtx)
	a.RecTab = view.RecRecommend
	recCtx := a.ListContext("popular")
	recGen := l.BeginFirstPage(recCtx)

	if l.FinishLoad(hotCtx, hotGen, true, true) {
		t.Fatal("换 tab 后旧上下文的响应被接受了")
	}
	if l.Initialized || !l.Loading || l.Context != recCtx {
		t.Fatalf("旧响应改坏了新请求状态：%+v", l)
	}
	if !l.FinishLoad(recCtx, recGen, true, false) {
		t.Fatal("当前上下文的失败结果没被接受")
	}
	if l.Initialized || !l.NeedsLoad(recCtx) {
		t.Fatal("首页失败后应保持可重试")
	}
	// 新上下文成功但为空：标记已初始化，重新打开不再重拉。
	gen := l.BeginFirstPage(recCtx)
	if !l.FinishLoad(recCtx, gen, true, true) {
		t.Fatal("当前上下文的空成功结果没被接受")
	}
	if !l.Initialized || l.NeedsLoad(recCtx) {
		t.Fatal("空成功列表仍被当作需要重载")
	}
}

// 搜索翻页在抽屉关掉后完成时仍然要落地并清 Loading；否则重新打开搜索会
// 永远停在「加载中」，自动翻页也再不会触发。
func TestSearchPageCompletionWhileHidden(t *testing.T) {
	a := view.NewApp(func() {})
	a.Query = "关键词"
	a.SortOrder = view.SortTotal
	l := a.ListFor(view.DrawerSearch)
	ctx := a.ListContext(view.DrawerSearch)
	gen := l.BeginFirstPage(ctx)
	if !l.FinishLoad(ctx, gen, true, true) {
		t.Fatal("搜索首屏成功结果没被接受")
	}
	l.HasMore = true

	a.Drawer = "" // 用户关掉搜索抽屉去做别的
	pgen, ok := l.BeginNextPage(ctx)
	if !ok {
		t.Fatal("抽屉隐藏时不应拒绝下一页请求")
	}
	if !l.FinishLoad(ctx, pgen, false, true) {
		t.Fatal("抽屉隐藏时完成的搜索页被当作旧响应")
	}
	setCards(l, []view.Card{{Bvid: "BVnext"}}, false)
	l.Page = 2
	if l.Loading {
		t.Fatal("隐藏抽屉完成翻页后 Loading 仍为真")
	}
	if len(l.Cards) != 1 || l.Cards[0].Bvid != "BVnext" {
		t.Fatalf("隐藏抽屉完成的搜索结果没保留：%+v", l.Cards)
	}
	if l.NeedsLoad(ctx) {
		t.Fatal("隐藏期间完成的搜索列表重新打开时不应再重拉")
	}
}

// 封顶裁头必须释放被丢掉的头部卡片（复制尾部到新底层数组），整批替换还要
// 把 Dropped 复位，否则「播放全部」会误以为列表仍是完整全集。
func TestSetCardsRetentionReleasesTrimmedHead(t *testing.T) {
	l := &view.List{}
	all := make([]view.Card, maxRetainedCards+5)
	for i := range all {
		all[i] = view.Card{
			Bvid:  fmt.Sprintf("BV%04d", i),
			Title: "旧头部标题，不该被尾部切片继续引用",
		}
	}
	setCards(l, all, true)
	if len(l.Cards) != 127 {
		t.Fatalf("保留 %d 条，应为 127（溢出 5 条向上对齐到 6）", len(l.Cards))
	}
	if !l.Dropped {
		t.Fatal("裁头后 Dropped 应为真")
	}
	if cap(l.Cards) != len(l.Cards) {
		t.Fatalf("裁头后仍复用旧底层数组：len=%d cap=%d（旧头部字符串会被继续挂着）",
			len(l.Cards), cap(l.Cards))
	}

	// 整批替换回一小批：Dropped 必须复位，列表重新是完整首页。
	setCards(l, []view.Card{{Bvid: "BVfresh"}}, true)
	if l.Dropped {
		t.Fatal("整批替换后 Dropped 没有复位")
	}
	if len(l.Cards) != 1 || l.Cards[0].Bvid != "BVfresh" {
		t.Fatalf("替换结果不对：%+v", l.Cards)
	}
}
