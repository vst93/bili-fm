package view

import "testing"

// 抽屉列表的「会话内恢复」：固定抽屉只保留最后一次上下文，A→B→A 或关掉
// 再打开都不该重新拉第一页，否则 search/issues #37 要的卡片 / 页码 / 滚动
// 位置都会被 ResetScroll 抹掉。

func seedDrawerList(a *App, key string, cards int) *List {
	l := a.ListFor(key)
	l.Context = a.ListContext(key)
	l.Initialized = true
	l.Cards = make([]Card, cards)
	for i := range l.Cards {
		l.Cards[i].Bvid = key + "-" + string(rune('A'+i))
	}
	l.Page = 3
	l.HasMore = true
	l.GridScroll.Y = 240
	l.Scroll.Y = 180
	return l
}

func TestOpenSectionAToBToARestoresTheList(t *testing.T) {
	a := NewApp(func() {})
	reloads := 0
	a.Act.Reload = func() { reloads++ }

	feed := seedDrawerList(a, "feed", 3)
	feed.Cards[0].Title = "保留的标题"
	a.openSection("feed")
	if reloads != 0 {
		t.Fatalf("首次打开已有内容的动态列表不应重拉，reloads=%d", reloads)
	}
	a.openSection("popular") // B：没有内容，会拉第一页
	if reloads != 1 {
		t.Fatalf("打开还没加载过的热门应拉一次，reloads=%d", reloads)
	}
	a.openSection("feed") // A→B→A：A 的上下文没变，应直接复用
	if reloads != 1 {
		t.Fatalf("A→B→A 不该重拉 A，reloads=%d", reloads)
	}
	if got := len(a.ListFor("feed").Cards); got != 3 {
		t.Fatalf("A 的卡片数 = %d，应保留 3", got)
	}
	if got := a.ListFor("feed").Cards[0].Title; got != "保留的标题" {
		t.Fatalf("A 的卡片内容变了：%q", got)
	}
	if got := a.ListFor("feed").Page; got != 3 {
		t.Fatalf("A 的页码 = %d，应保留 3", got)
	}
	if got := a.ListFor("feed").GridScroll.Y; got != 240 {
		t.Fatalf("A 的网格滚动 = %v，应保留 240", got)
	}
	if got := a.ListFor("feed").Scroll.Y; got != 180 {
		t.Fatalf("A 的滚动 = %v，应保留 180", got)
	}
}

func TestOpenSectionCloseReopenKeepsState(t *testing.T) {
	a := NewApp(func() {})
	reloads := 0
	a.Act.Reload = func() { reloads++ }
	seedDrawerList(a, "history", 2)

	a.openSection("history")
	a.openSection("history") // 再点一次：关闭
	if a.Drawer != "" {
		t.Fatalf("再点同一入口应收起抽屉，Drawer=%q", a.Drawer)
	}
	a.openSection("history") // 重新打开
	if reloads != 0 {
		t.Fatalf("关掉再打开不应重拉，reloads=%d", reloads)
	}
	l := a.ListFor("history")
	if len(l.Cards) != 2 || l.Page != 3 || l.GridScroll.Y != 240 {
		t.Fatalf("关掉再打开状态丢了：cards=%d page=%d scroll=%v", len(l.Cards), l.Page, l.GridScroll.Y)
	}
}

func TestOpenSectionKeepsSuccessfulEmptyList(t *testing.T) {
	a := NewApp(func() {})
	reloads := 0
	a.Act.Reload = func() { reloads++ }
	l := a.ListFor("feed")
	l.Context = a.ListContext("feed")
	l.Initialized = true // 首页成功、结果为空
	l.Page = 1

	a.openSection("feed")
	a.openSection("feed")
	a.openSection("feed")
	if reloads != 0 {
		t.Fatalf("空成功列表不该被当成「没加载」反复重拉，reloads=%d", reloads)
	}
	if !l.Initialized {
		t.Fatal("空成功列表的 Initialized 被清掉了")
	}
}

func TestOpenSectionRetriesFirstPageFailure(t *testing.T) {
	a := NewApp(func() {})
	reloads := 0
	a.Act.Reload = func() {
		reloads++
		l := a.ListFor(a.Drawer)
		ctx := a.ListContext(a.Drawer)
		gen := l.BeginFirstPage(ctx)
		// 模拟首页请求失败：Loading 清掉，但 Initialized 仍为 false。
		l.FinishLoad(ctx, gen, true, false)
	}
	// 第一次已经失败过：同一上下文仍在，但没成功，也不能被当成空成功。
	failed := a.ListFor("feed")
	failed.Context = a.ListContext("feed")
	failed.Initialized = false
	failed.Loading = false

	a.openSection("feed")
	if reloads != 1 {
		t.Fatalf("首次失败后再打开应重试，reloads=%d", reloads)
	}
	a.openSection("feed") // 关闭
	a.openSection("feed") // 再打开：仍是失败态，应再次重试
	if reloads != 2 {
		t.Fatalf("首页失败后关掉再打开仍应可重试，reloads=%d", reloads)
	}
}

func TestOpenSectionDoesNotDuplicateInFlightLoad(t *testing.T) {
	a := NewApp(func() {})
	reloads := 0
	a.Act.Reload = func() { reloads++ }
	l := a.ListFor("feed")
	l.Context = a.ListContext("feed")
	l.Loading = true // 首页请求在途，还没成功
	l.Initialized = false

	a.openSection("feed")
	a.openSection("feed")
	a.openSection("feed")
	if reloads != 0 {
		t.Fatalf("同一上下文正在加载时不该重复发请求，reloads=%d", reloads)
	}
	if !l.Loading {
		t.Fatal("在途状态被误清掉了")
	}

	// 在途期间换了 tab（上下文变了）：必须明确重拉，旧请求作废。
	a.openSection("feed") // 关闭
	a.RecTab = RecRecommend
	a.openSection("popular")
	if reloads != 1 {
		t.Fatalf("上下文变了应重拉一次，reloads=%d", reloads)
	}
}

func TestOpenSeriesReusesMatchingList(t *testing.T) {
	a := NewApp(func() {})
	a.SeriesID = 7
	l := seedDrawerList(a, DrawerSeries, 2)
	calls := 0
	a.Act.SelectSeries = func(id int64) {
		calls++
		l.BeginFirstPage(a.ListContext(DrawerSeries))
	}

	a.openSeries()
	if calls != 0 {
		t.Fatalf("相同合集再次打开不该重新拉，calls=%d", calls)
	}
	a.openSeries() // 关闭
	a.openSeries() // 再打开
	if calls != 0 {
		t.Fatalf("关掉再打开相同合集不该重新拉，calls=%d", calls)
	}

	a.openSeries() // 关闭
	a.SeriesID = 8
	a.openSeries()
	if calls != 1 {
		t.Fatalf("换合集 id 必须重拉一次，calls=%d", calls)
	}
}

func TestListFirstPageInvalidatesOldRequests(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	ctx := a.ListContext("feed")
	oldGen := l.BeginFirstPage(ctx)
	// 模拟已经有一页内容 / 滚动 / 裁头记录。
	l.Cards = []Card{{Bvid: "old"}}
	l.Page = 4
	l.HasMore = true
	l.Dropped = true
	l.GridScroll.Y = 300

	// 手动刷新：清内容、回顶部、代号递增。
	newGen := l.BeginFirstPage(ctx)
	if newGen == oldGen {
		t.Fatal("手动刷新没有递增请求代号，旧响应可能落地")
	}
	if l.Cards != nil || l.Page != 0 || l.HasMore || l.Dropped || l.GridScroll.Y != 0 {
		t.Fatalf("首页重载没有复位：%+v", l)
	}
	if l.Initialized || !l.Loading {
		t.Fatalf("首页重载状态错误：Initialized=%v Loading=%v", l.Initialized, l.Loading)
	}
	if l.FinishLoad(ctx, oldGen, true, true) {
		t.Fatal("旧代号的响应被接受了")
	}
	if !l.Loading || l.Initialized {
		t.Fatal("旧响应不应清掉新请求的 Loading / 标成已初始化")
	}
	// 新请求失败：仍是可重试态。
	if !l.FinishLoad(ctx, newGen, true, false) {
		t.Fatal("当前请求的失败结果没被接受")
	}
	if l.Initialized || !l.NeedsLoad(ctx) {
		t.Fatal("首页失败后应保持未初始化、可重试")
	}
	// 成功但为空：已初始化，不再重拉。
	gen := l.BeginFirstPage(ctx)
	if !l.FinishLoad(ctx, gen, true, true) {
		t.Fatal("空成功结果没被接受")
	}
	if !l.Initialized || l.NeedsLoad(ctx) {
		t.Fatal("成功空列表应标记已初始化，打开时不再重拉")
	}
}

func TestSearchPageCompletesWhileDrawerHidden(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor(DrawerSearch)
	ctx := a.ListContext(DrawerSearch)
	gen := l.BeginFirstPage(ctx)
	if !l.FinishLoad(ctx, gen, true, true) {
		t.Fatal("首屏成功结果没被接受")
	}
	l.HasMore = true

	// 用户关掉搜索抽屉：Drawer 为空，但这次翻页仍属于当前搜索。
	a.Drawer = ""
	pgen, ok := l.BeginNextPage(ctx)
	if !ok {
		t.Fatal("隐藏抽屉后翻页请求不该被拒绝")
	}
	if !l.FinishLoad(ctx, pgen, false, true) {
		t.Fatal("抽屉隐藏时完成的搜索结果应保留，供重新打开使用")
	}
	if l.Loading {
		t.Fatal("隐藏抽屉完成的翻页留下了 Loading")
	}
}
