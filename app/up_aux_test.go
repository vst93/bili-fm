package main

import (
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

// UP 空间的关注状态与合集列表是 List 之外的两份辅助数据；它们必须和列表
// 的上下文 / 代号一起验收。A→B→A、刷新、切 tab 后旧响应都不能再落地，
// 新一轮请求的 pending 也不能被旧响应误解掉。
func TestUpAuxRejectsStaleListGeneration(t *testing.T) {
	a := view.NewApp(func() {})
	c := &controller{app: a}
	l := a.ListFor(view.DrawerUp)
	browseGen := a.BrowseGeneration

	// A：视频 tab 的第一次打开。
	a.UpMid, a.UpName, a.UpTab = 1, "A", view.UpTabVideos
	ctxA := a.ListContext(view.DrawerUp)
	genA := l.BeginFirstPage(ctxA)
	a.UpFollowPending = true
	a.SeriesList = []view.Series{{ID: 1}}

	// A -> B。
	a.UpMid, a.UpName = 2, "B"
	ctxB := a.ListContext(view.DrawerUp)
	genB := l.BeginFirstPage(ctxB)
	a.UpFollowPending = true
	a.SeriesList = []view.Series{{ID: 2}}

	// B -> A：同一上下文，但列表代号已经再次前进。
	a.UpMid, a.UpName = 1, "A"
	genA2 := l.BeginFirstPage(ctxA)
	a.UpFollowPending = true
	a.SeriesList = []view.Series{{ID: 99}}

	// 旧 A / B 的关注响应与合集响应都必须被拒绝；旧的拒绝不能清 pending。
	if c.completeUpFollow(1, l, ctxA, genA, browseGen, &bilibili.FollowStatus{IsFollowing: true, Follower: 10}, nil) {
		t.Fatal("A→B→A 后旧 A 的关注响应被接受了")
	}
	if !a.UpFollowPending || a.UpFollowed || a.UpFans != 0 {
		t.Fatalf("旧关注响应改坏 pending/状态：pending=%v followed=%v fans=%d",
			a.UpFollowPending, a.UpFollowed, a.UpFans)
	}
	if c.completeUpSeries(2, l, ctxB, genB, browseGen, []view.Series{{ID: 3}}) {
		t.Fatal("A→B→A 后旧 B 的合集响应被接受了")
	}
	if c.completeUpSeries(1, l, ctxA, genA, browseGen, []view.Series{{ID: 4}}) {
		t.Fatal("A→B→A 后第一轮 A 的合集响应被接受了")
	}

	// 当前 A 的响应才落地。
	if !c.completeUpSeries(1, l, ctxA, genA2, browseGen, []view.Series{{ID: 5}}) {
		t.Fatal("当前 A 的合集响应被拒绝了")
	}
	if got := a.SeriesList; len(got) != 1 || got[0].ID != 5 {
		t.Fatalf("当前合集列表没有落地：%+v", got)
	}
	if !c.completeUpFollow(1, l, ctxA, genA2, browseGen, &bilibili.FollowStatus{IsFollowing: true, Follower: 42}, nil) {
		t.Fatal("当前 A 的关注响应被拒绝了")
	}
	if a.UpFollowPending || !a.UpFollowed || a.UpFans != 42 {
		t.Fatalf("当前关注状态没有落地：pending=%v followed=%v fans=%d",
			a.UpFollowPending, a.UpFollowed, a.UpFans)
	}

	// 刷新：列表代号前进，旧关注响应再次被拒，pending 由新请求负责清掉。
	genA3 := l.BeginFirstPage(ctxA)
	a.UpFollowPending = true
	if c.completeUpFollow(1, l, ctxA, genA2, browseGen, &bilibili.FollowStatus{IsFollowing: false, Follower: 7}, nil) {
		t.Fatal("刷新后旧关注响应被接受了")
	}
	if !a.UpFollowPending {
		t.Fatal("旧关注响应清掉了新请求的 pending")
	}
	if !c.completeUpFollow(1, l, ctxA, genA3, browseGen, &bilibili.FollowStatus{IsFollowing: false, Follower: 8}, nil) {
		t.Fatal("刷新后的关注响应被拒绝了")
	}
	if a.UpFollowPending || a.UpFollowed || a.UpFans != 8 {
		t.Fatalf("刷新后的关注状态不对：pending=%v followed=%v fans=%d",
			a.UpFollowPending, a.UpFollowed, a.UpFans)
	}

	// 切到合集 tab：视频 tab 的辅助合集响应与合集 tab 的 loader 共用同一份
	// 代号，旧响应被拒，新响应落地。
	a.UpTab = view.UpTabSeries
	ctxSeries := a.ListContext(view.DrawerUp)
	genSeries := l.BeginFirstPage(ctxSeries)
	a.SeriesList = nil
	if c.completeUpSeries(1, l, ctxA, genA3, browseGen, []view.Series{{ID: 6}}) {
		t.Fatal("切到合集 tab 后视频 tab 的旧合集响应被接受了")
	}
	if !c.completeUpSeries(1, l, ctxSeries, genSeries, browseGen, []view.Series{{ID: 7}}) {
		t.Fatal("合集 tab 的新合集响应被拒绝了")
	}
	if got := a.SeriesList; len(got) != 1 || got[0].ID != 7 {
		t.Fatalf("合集 tab 的列表没有落地：%+v", got)
	}
}

// 列表完成 helper 是 loader 真正使用的验收逻辑：账号代号 / 上下文 / 请求
// 代号任一不符都不得清 Loading 或标成已初始化。
func TestListCompletionHelpersRejectStaleResults(t *testing.T) {
	a := view.NewApp(func() {})
	c := &controller{app: a}

	l := a.ListFor("popular")
	ctx := a.ListContext("popular")
	gen := l.BeginFirstPage(ctx)
	browseGen := a.BrowseGeneration

	if c.acceptListResult(l, ctx, gen, browseGen+1, true, true) {
		t.Fatal("旧账号代号的列表结果被接受了")
	}
	if !l.Loading || l.Initialized {
		t.Fatalf("被拒的旧结果改坏了列表：loading=%v init=%v", l.Loading, l.Initialized)
	}
	if !c.acceptListResult(l, ctx, gen, browseGen, true, true) {
		t.Fatal("当前列表结果被拒绝了")
	}
	if l.Loading || !l.Initialized {
		t.Fatalf("当前结果没有落地：loading=%v init=%v", l.Loading, l.Initialized)
	}

	// 搜索：新的搜索 / 清空都会推进 SearchRequestID；旧响应不能碰列表。
	a.Query = "旧词"
	a.SortOrder = view.SortTotal
	sl := a.ListFor(view.DrawerSearch)
	sctx := a.ListContext(view.DrawerSearch)
	a.SearchRequestID++
	oldReq := a.SearchRequestID
	sgen := sl.BeginFirstPage(sctx)

	a.Query = ""
	a.SearchRequestID++
	sl.Invalidate()
	if c.acceptSearchResult(oldReq, sl, sctx, sgen, true, true) {
		t.Fatal("清空搜索后旧搜索结果被接受了")
	}
	if sl.Loading || sl.Initialized || len(sl.Cards) != 0 {
		t.Fatalf("清空搜索后被旧结果写脏：%+v", sl)
	}

	a.Query = "新词"
	a.SearchRequestID++
	newReq := a.SearchRequestID
	sctx2 := a.ListContext(view.DrawerSearch)
	sgen2 := sl.BeginFirstPage(sctx2)
	if !c.acceptSearchResult(newReq, sl, sctx2, sgen2, true, true) {
		t.Fatal("当前搜索结果被拒绝了")
	}
	if sl.Loading || !sl.Initialized {
		t.Fatalf("当前搜索结果没有落地：loading=%v init=%v", sl.Loading, sl.Initialized)
	}
}
