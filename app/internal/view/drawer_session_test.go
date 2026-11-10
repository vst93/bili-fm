package view

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestBackToDrawerReloadsOnlyInvalidatedSource 钉住返回上一层的两种语义：
//
//   - 普通已初始化来源：保持卡片 / 页码 / 滚动位置，绝不重拉（issue #37 ①）。
//   - 换账号把来源列表作废（空列表、Initialized=false）：返回时按 NeedsLoad
//     惰性重载，而不是停在一个空壳列表上。
func TestBackToDrawerReloadsOnlyInvalidatedSource(t *testing.T) {
	t.Run("initialized-source-keeps-session", func(t *testing.T) {
		a := NewApp(func() {})
		a.FolderID = 3
		l := a.ListFor("favorite")
		l.Context = a.ListContext("favorite")
		l.Initialized = true
		l.Cards = []Card{{Bvid: "BVkeep1"}, {Bvid: "BVkeep2"}}
		l.Page = 4
		l.GridScroll.Y = 320
		l.Scroll.Y = 180
		a.Drawer = "favorite"
		a.BrowseTo(&Info{Bvid: "BVparts"})

		reloads := 0
		a.Act.Reload = func() { reloads++ }
		a.backToDrawer()

		if a.Drawer != "favorite" {
			t.Fatalf("返回后 Drawer = %q，应为 favorite", a.Drawer)
		}
		if reloads != 0 {
			t.Fatalf("已初始化来源被重拉：reloads=%d", reloads)
		}
		if len(l.Cards) != 2 || l.Page != 4 || l.GridScroll.Y != 320 || l.Scroll.Y != 180 {
			t.Fatalf("返回后会话状态丢失：cards=%d page=%d grid=%v scroll=%v",
				len(l.Cards), l.Page, l.GridScroll.Y, l.Scroll.Y)
		}
	})

	t.Run("invalidated-source-reloads", func(t *testing.T) {
		a := NewApp(func() {})
		a.FolderID = 3
		l := a.ListFor("favorite")
		l.Context = a.ListContext("favorite")
		l.Initialized = true
		l.Cards = []Card{{Bvid: "BVold"}}
		l.Page = 4
		l.GridScroll.Y = 320
		a.Drawer = "favorite"
		a.BrowseTo(&Info{Bvid: "BVparts"})

		// 换账号：当前停在选集，来源列表被清空并作废。
		a.InvalidateBrowsing()
		if a.Drawer != DrawerParts {
			t.Fatalf("作废浏览数据不应换抽屉，Drawer=%q", a.Drawer)
		}
		reloads := 0
		a.Act.Reload = func() {
			reloads++
			// 模拟刷新入口：当前抽屉（返回后的 favorite）重新发起首页。
			l.BeginFirstPage(a.ListContext(a.Drawer))
		}
		a.backToDrawer()

		if a.Drawer != "favorite" {
			t.Fatalf("返回后 Drawer = %q，应为 favorite", a.Drawer)
		}
		if reloads != 1 {
			t.Fatalf("作废来源应惰性重载一次，reloads=%d", reloads)
		}
		if !l.Loading || l.Initialized || len(l.Cards) != 0 {
			t.Fatalf("重载没有从空首页开始：loading=%v init=%v cards=%d",
				l.Loading, l.Initialized, len(l.Cards))
		}
	})
}

// TestSearchDockReopensSearchStates 钉住搜索结果入口不再按「有没有卡片」
// 禁用：空成功 / 在途 / 失败 / 被换账号作废的搜索都应能重新打开；只有确实
// 需要第一页时才走 Reload（显式提交键仍由 runSearch 每次刷新，不经过这里）。
func TestSearchDockReopensSearchStates(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(a *App, l *List)
		reloads int
	}{
		{"empty-success", func(a *App, l *List) {
			l.Context = a.ListContext(DrawerSearch)
			l.Initialized = true // 首页成功、结果为空
		}, 0},
		{"in-flight", func(a *App, l *List) {
			l.Context = a.ListContext(DrawerSearch)
			l.Loading = true
		}, 0},
		{"failed", func(a *App, l *List) {
			l.Context = a.ListContext(DrawerSearch)
			l.Initialized = false
			l.Loading = false
		}, 1},
		{"account-invalidated", func(a *App, l *List) {
			l.Context = a.ListContext(DrawerSearch)
			l.Initialized = true
			l.Cards = []Card{{Bvid: "BVold"}}
			l.Invalidate()
		}, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.Query = "关键词"
			a.SortOrder = SortTotal
			l := a.ListFor(DrawerSearch)
			tc.setup(a, l)
			reloads := 0
			a.Act.Reload = func() { reloads++ }

			tt := shellTester(t, a)
			if err := tt.Click("搜索结果"); err != nil {
				t.Fatalf("搜索结果入口点不到：%v", err)
			}
			tt.Frame()
			if a.Drawer != DrawerSearch {
				t.Fatalf("点搜索结果后 Drawer = %q，应为 search", a.Drawer)
			}
			if reloads != tc.reloads {
				t.Fatalf("Reload %d 次，想 %d（NeedsLoad 判据不对）", reloads, tc.reloads)
			}
		})
	}

	// 没有关键词时入口仍是禁用态：空关键词的「搜索结果」没有可恢复内容。
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	reloads := 0
	a.Act.Reload = func() { reloads++ }
	tt := shellTester(t, a)
	_ = tt.Click("搜索结果") // 禁用键点不动；有些框架版本会返回错误。
	tt.Frame()
	if a.Drawer != "" || reloads != 0 {
		t.Fatalf("空关键词不应打开搜索：drawer=%q reloads=%d", a.Drawer, reloads)
	}
}

// writeSessionShot 返回一个按 BILI_DRAWER_SESSION_SHOTS 导出 PNG 的写入器。
// 未设置环境变量时是 no-op，不改变测试结果。文件名单写清场景、主题与阶段，
// 方便人工对照 section / close-reopen / parts-return 三条路径。
func writeSessionShot(t *testing.T) func(name string, img *image.RGBA) {
	t.Helper()
	dir := os.Getenv("BILI_DRAWER_SESSION_SHOTS")
	if dir == "" {
		return func(string, *image.RGBA) {}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	return func(name string, img *image.RGBA) {
		t.Helper()
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
	}
}

// seedSessionCards 填入一份已初始化、无网络的固定上下文列表。封面地址只用于
// 记录请求：recordingCache 不解码，画面上仍是占位图；断言只看文案与滚动状态，
// 不声称真实封面已经加载。
func seedSessionCards(a *App, key, bvidPrefix, titlePrefix string, n int) *List {
	l := a.ListFor(key)
	l.Context = a.ListContext(key)
	l.Initialized = true
	l.Page = 1
	for i := 0; i < n; i++ {
		l.Cards = append(l.Cards, Card{
			Bvid:    fmt.Sprintf("%s%03d", bvidPrefix, i),
			Title:   fmt.Sprintf("%s %03d", titlePrefix, i),
			Cover:   fmt.Sprintf("https://i0.hdslb.com/bfs/archive/%s-%03d.jpg@240w.webp", bvidPrefix, i),
			Up:      "UP主",
			Views:   "1.2万",
			Pubdate: "09-22",
		})
	}
	return l
}

// TestDrawerSessionPartsReturnRenderedAToBToASameScroll 是无头整窗回归，专门
// 钉住已有的 issue #37 返回路径：A（动态列表）滚动到中段 → 经 BrowseTo 进
// B（选集）→ 点「返回上一层列表」回 A。两种主题都必须回到同一张可见卡片、
// 同一滚动偏移，且来源列表没有被重拉（Loading=false、Reload 不触发）。
//
// 这条不覆盖固定分区之间的 A→B→A；那种切抽屉（不走选集返回键）的路径由
// TestDrawerSessionSectionAToBToASameScroll 单独验证。
func TestDrawerSessionPartsReturnRenderedAToBToASameScroll(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			writeShot := writeSessionShot(t)
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.SetAppearance(mode, 0)
			// 只记录请求、不联网；封面用 240w 的稳定 fixture 地址（卡片请求
			// 的宽度也确实是 imagecache.WidthCard）。
			a.Images = newRecordingCache()

			l := seedSessionCards(a, "feed", "BVsession", "会话卡片", 90)
			a.Drawer = "feed"
			reloads := 0
			a.Act.Reload = func() { reloads++ }

			tt := shellTesterSize(t, a, 800, 600)
			tt.Scroll(400, 300, 0, 1200)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()

			first, last := l.Grid.Visible()
			if first <= 0 || l.GridScroll.Y <= 0 {
				t.Fatalf("没有真正滚到列表中段：visible=%d..%d gridY=%v", first, last, l.GridScroll.Y)
			}
			anchor := l.Cards[first*3].Bvid
			anchorTitle := l.Cards[first*3].Title
			scrollY := l.GridScroll.Y
			if !tt.HasText(anchorTitle) {
				t.Fatalf("锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("parts-return-"+mode+"-before", tt.Image())

			// A → B：与生产一样经 BrowseTo 进选集，并记住来源。
			info := &Info{
				Bvid: "BVdetail", Title: "详情", Cid: 1,
				Parts: []Part{{Cid: 1, Part: "P1"}},
			}
			a.BrowseTo(info)
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != DrawerParts {
				t.Fatalf("进选集后 Drawer = %q", a.Drawer)
			}

			// B → A：点真实的「返回上一层列表」键。
			if err := tt.Click("返回上一层列表"); err != nil {
				t.Fatalf("点不到返回键：%v", err)
			}
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "feed" {
				t.Fatalf("返回后 Drawer = %q，应为 feed", a.Drawer)
			}
			if reloads != 0 {
				t.Fatalf("返回已初始化列表触发了重拉：reloads=%d", reloads)
			}
			if l.Loading {
				t.Fatal("返回后来源列表处于 Loading（被重拉了）")
			}

			first2, last2 := l.Grid.Visible()
			if first2 != first || last2 != last {
				t.Fatalf("返回后可见行变化：%d..%d → %d..%d", first, last, first2, last2)
			}
			if got := l.Cards[first2*3].Bvid; got != anchor {
				t.Fatalf("返回后可见锚点卡片变了：%q → %q", anchor, got)
			}
			if got := l.GridScroll.Y; got != scrollY {
				t.Fatalf("返回后滚动偏移变了：%v → %v", scrollY, got)
			}
			if !tt.HasText(anchorTitle) {
				t.Fatalf("返回后锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("parts-return-"+mode+"-after", tt.Image())
		})
	}
}

// TestDrawerSessionSectionAToBToASameScroll 覆盖固定分区抽屉自己的 A→B→A：
// feed 滚到中段 → openSection("popular") → openSection("feed")。B 也用一份
// 已初始化的固定上下文，打开时不会联网；两次切换都走真实 Shell 渲染并推进
// 抽屉入场动画。回 A 后可见行、锚点卡片与 GridScroll 必须原样保留，且 A / B
// 的 Reload 都不触发。
func TestDrawerSessionSectionAToBToASameScroll(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			writeShot := writeSessionShot(t)
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.SetAppearance(mode, 0)
			a.Images = newRecordingCache()

			l := seedSessionCards(a, "feed", "BVsession", "会话卡片", 90)
			popular := seedSessionCards(a, "popular", "BVpopular", "热门卡片", 36)
			reloads := 0
			a.Act.Reload = func() { reloads++ }

			tt := shellTesterSize(t, a, 800, 600)
			// 从空抽屉真正经 openSection 打开 A。
			a.openSection("feed")
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "feed" {
				t.Fatalf("打开动态后 Drawer = %q，应为 feed", a.Drawer)
			}
			if reloads != 0 {
				t.Fatalf("打开已初始化的动态触发了重拉：reloads=%d", reloads)
			}

			tt.Scroll(400, 300, 0, 1200)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()

			first, last := l.Grid.Visible()
			if first <= 0 || l.GridScroll.Y <= 0 {
				t.Fatalf("没有真正滚到动态列表中段：visible=%d..%d gridY=%v", first, last, l.GridScroll.Y)
			}
			anchor := l.Cards[first*3].Bvid
			anchorTitle := l.Cards[first*3].Title
			scrollY := l.GridScroll.Y
			if !tt.HasText(anchorTitle) {
				t.Fatalf("锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("section-"+mode+"-before", tt.Image())

			// A → B：固定分区切换，不是选集返回路径。
			a.openSection("popular")
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "popular" {
				t.Fatalf("切到热门后 Drawer = %q，应为 popular", a.Drawer)
			}
			if reloads != 0 {
				t.Fatalf("打开已初始化的热门触发了重拉：reloads=%d", reloads)
			}
			if popular.Loading || !popular.Initialized || len(popular.Cards) != 36 {
				t.Fatalf("热门上下文被动了：loading=%v init=%v cards=%d",
					popular.Loading, popular.Initialized, len(popular.Cards))
			}
			if !tt.HasText(popular.Cards[0].Title) {
				t.Fatalf("热门卡片 %q 没有渲染出来", popular.Cards[0].Title)
			}

			// B → A：再次经 openSection 回动态，不经过选集返回键。
			a.openSection("feed")
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "feed" {
				t.Fatalf("切回动态后 Drawer = %q，应为 feed", a.Drawer)
			}
			if reloads != 0 {
				t.Fatalf("固定分区 A→B→A 触发了重拉：reloads=%d", reloads)
			}
			if l.Loading {
				t.Fatal("切回动态后列表处于 Loading（被重拉了）")
			}

			first2, last2 := l.Grid.Visible()
			if first2 != first || last2 != last {
				t.Fatalf("切回后可见行变化：%d..%d → %d..%d", first, last, first2, last2)
			}
			if got := l.Cards[first2*3].Bvid; got != anchor {
				t.Fatalf("切回后可见锚点卡片变了：%q → %q", anchor, got)
			}
			if got := l.GridScroll.Y; got != scrollY {
				t.Fatalf("切回后滚动偏移变了：%v → %v", scrollY, got)
			}
			if !tt.HasText(anchorTitle) {
				t.Fatalf("切回后锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("section-"+mode+"-after", tt.Image())
		})
	}
}

// TestDrawerSessionCloseSettledReopenRestoresSameScroll 验证关闭路径真正把抽屉
// 卸载（跑完退场动画、drawerShown 清空）后再经 openSection 重新打开。不是
// 只把 Drawer key 从 feed 切到别的再切回来：中间必须有一帧确实没有抽屉内容。
// 两种主题下都要求可见行、锚点卡片、GridScroll 原样恢复且不触发 Reload。
func TestDrawerSessionCloseSettledReopenRestoresSameScroll(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		t.Run(mode, func(t *testing.T) {
			writeShot := writeSessionShot(t)
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.SetAppearance(mode, 0)
			a.Images = newRecordingCache()

			l := seedSessionCards(a, "feed", "BVsession", "会话卡片", 90)
			reloads := 0
			a.Act.Reload = func() { reloads++ }

			tt := shellTesterSize(t, a, 800, 600)
			a.openSection("feed")
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "feed" {
				t.Fatalf("打开动态后 Drawer = %q，应为 feed", a.Drawer)
			}

			tt.Scroll(400, 300, 0, 1200)
			tt.Advance(200 * time.Millisecond)
			tt.Frame()

			first, last := l.Grid.Visible()
			if first <= 0 || l.GridScroll.Y <= 0 {
				t.Fatalf("没有真正滚到动态列表中段：visible=%d..%d gridY=%v", first, last, l.GridScroll.Y)
			}
			anchor := l.Cards[first*3].Bvid
			anchorTitle := l.Cards[first*3].Title
			scrollY := l.GridScroll.Y
			if !tt.HasText(anchorTitle) {
				t.Fatalf("锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("close-reopen-"+mode+"-before", tt.Image())

			// 再点同一入口：关闭，并让退场动画完整跑完再确认卸载。
			a.openSection("feed")
			if a.Drawer != "" {
				t.Fatalf("关闭后 Drawer = %q，应为空", a.Drawer)
			}
			tt.Frame()
			tt.Advance(2 * drawerIn)
			tt.Frame()
			if a.Drawer != "" {
				t.Fatalf("退场后 Drawer = %q，应为空", a.Drawer)
			}
			if a.drawerShown != "" || a.drawerOpenKey != "" || a.drawerClosing {
				t.Fatalf("抽屉还没真正卸载：shown=%q openKey=%q closing=%v",
					a.drawerShown, a.drawerOpenKey, a.drawerClosing)
			}
			if tt.HasText(anchorTitle) {
				t.Fatalf("抽屉已关闭但锚点卡片 %q 仍在画面里", anchorTitle)
			}
			writeShot("close-reopen-"+mode+"-hidden", tt.Image())

			// 完全隐藏后再经同一入口重开：恢复的是列表会话，不是上一次的
			// 抽屉 key 残留。
			a.openSection("feed")
			tt.Frame()
			tt.Advance(400 * time.Millisecond)
			tt.Frame()
			if a.Drawer != "feed" {
				t.Fatalf("重开后 Drawer = %q，应为 feed", a.Drawer)
			}
			if reloads != 0 {
				t.Fatalf("关闭再打开触发了重拉：reloads=%d", reloads)
			}
			if l.Loading {
				t.Fatal("重开后列表处于 Loading（被重拉了）")
			}

			first2, last2 := l.Grid.Visible()
			if first2 != first || last2 != last {
				t.Fatalf("重开后可见行变化：%d..%d → %d..%d", first, last, first2, last2)
			}
			if got := l.Cards[first2*3].Bvid; got != anchor {
				t.Fatalf("重开后可见锚点卡片变了：%q → %q", anchor, got)
			}
			if got := l.GridScroll.Y; got != scrollY {
				t.Fatalf("重开后滚动偏移变了：%v → %v", scrollY, got)
			}
			if !tt.HasText(anchorTitle) {
				t.Fatalf("重开后锚点卡片 %q 不在首屏文本里", anchorTitle)
			}
			writeShot("close-reopen-"+mode+"-after", tt.Image())
		})
	}
}
