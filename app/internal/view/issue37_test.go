package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 用户 issue #37 的两条交互改进（2026-11-05 报，维护者确认「下个版本优化上」）：
//
//	① 动态 / 收藏 / 历史等页面可以增加一个返回功能，点击返回回到前面滚动到的
//	   位置，这样想要连续添加视频到播放列表的时候就不需要重复点开并滚动到
//	   对应位置。
//	② 在播放列表中有视频的时候，播放键应该是可操作的状态，而不需要点开播放
//	   列表选择视频进行播放。

// TestPartsBackTarget 钉住 issue #37 ① 的判据：哪些抽屉进选集后长出「返回」。
//
// 只有**列表类**抽屉才算上一层。从主区 / 托盘 / 调试入口进来的选集不该有返回键
// （那里本来就没有「前面的滚动位置」可回）。
func TestPartsBackTarget(t *testing.T) {
	for _, drawer := range []string{
		"feed", "popular", "favorite", "history", // 四个分区列表
		DrawerSearch, DrawerUp, DrawerSeries, DrawerPlaylist,
	} {
		if got := partsBackTarget(drawer, Sections); got != drawer {
			t.Errorf("从 %q 进选集：返回目标 = %q，应为它自己", drawer, got)
		}
	}
	// 不该有返回键的入口：已经在选集里、弹幕抽屉、什么都没开、未知 key。
	for _, drawer := range []string{"", DrawerParts, DrawerDanmaku, "nope"} {
		if got := partsBackTarget(drawer, Sections); got != "" {
			t.Errorf("从 %q 进选集：返回目标 = %q，应为空（不该长出返回键）", drawer, got)
		}
	}
}

// TestBrowseToRemembersTheList 钉住 BrowseTo 真的记下了来源。
func TestBrowseToRemembersTheList(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = "history"
	a.BrowseTo(&Info{Bvid: "BV1"})
	if a.Drawer != DrawerParts {
		t.Fatalf("BrowseTo 后 Drawer = %q，应为选集", a.Drawer)
	}
	if a.DrawerBack != "history" {
		t.Fatalf("DrawerBack = %q，应记住来源 history", a.DrawerBack)
	}
}

// TestBackToDrawerKeepsScroll 是 issue #37 ① 的**核心**断言：返回上一层时
// 绝不能把列表的滚动位置抹掉。
//
// 滚动位置存在 List.GridScroll / List.Scroll 里；任何「重拉第一页」都会
// 调 ResetScroll 把它们清零 —— 那正是「要连续加歌就得重新滚到对应位置」的根因。
func TestBackToDrawerKeepsScroll(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = "favorite"
	list := a.ListFor("favorite")
	list.Cards = []Card{{Bvid: "BV1"}, {Bvid: "BV2"}}
	// 模拟「用户滚到了第 3 行」。
	list.GridScroll.Y = 320
	list.GridScroll.X = 0
	list.Scroll.Y = 320
	pageBefore := list.Page

	a.BrowseTo(&Info{Bvid: "BV9"})
	if a.Drawer != DrawerParts {
		t.Fatalf("进选集失败：Drawer = %q", a.Drawer)
	}

	a.backToDrawer()
	if a.Drawer != "favorite" {
		t.Fatalf("返回后 Drawer = %q，应为 favorite", a.Drawer)
	}
	if a.DrawerBack != "" {
		t.Fatalf("返回后 DrawerBack 应清空，得到 %q", a.DrawerBack)
	}
	if got := a.ListFor("favorite").GridScroll.Y; got != 320 {
		t.Fatalf("返回后滚动位置 = %v，应保留 320（重拉第一页会把它清零）", got)
	}
	if got := a.ListFor("favorite").Scroll.Y; got != 320 {
		t.Fatalf("返回后 Scroll.Y = %v，应保留 320", got)
	}
	if got := a.ListFor("favorite").Page; got != pageBefore {
		t.Fatalf("返回后页码 = %d，应仍是 %d（不能重拉第一页）", got, pageBefore)
	}
	if n := len(a.ListFor("favorite").Cards); n != 2 {
		t.Fatalf("返回后卡片数 = %d，应仍是 2（不能清空重拉）", n)
	}
}

// TestCloseDrawerGoesBackFromParts 钉住「关掉选集」等价于「回到上一层列表」：
// 关闭键 / Esc / 点遮罩 / 下滑关闭都走 closeDrawer，从列表进来的选集会退回列表。
func TestCloseDrawerGoesBackFromParts(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = "feed"
	a.BrowseTo(&Info{Bvid: "BV1"})
	if a.Drawer != DrawerParts {
		t.Fatalf("Drawer = %q，应为选集", a.Drawer)
	}
	a.closeDrawer()
	if a.Drawer != "feed" {
		t.Fatalf("关掉从列表进来的选集后 Drawer = %q，应回到 feed", a.Drawer)
	}
}

// TestCloseDrawerWithNoBackCloses 保证没有上层时关掉就是关掉（不凭空冒出一个抽屉）。
func TestCloseDrawerWithNoBackCloses(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = DrawerParts // 从主区进来的选集：没有 DrawerBack
	a.closeDrawer()
	if a.Drawer != "" {
		t.Fatalf("没有上层时关掉选集应真的关掉，得到 Drawer = %q", a.Drawer)
	}
	// 普通列表抽屉关掉也照旧。
	a.Drawer = "favorite"
	a.closeDrawer()
	if a.Drawer != "" {
		t.Fatalf("关掉列表抽屉后 Drawer = %q，应为空", a.Drawer)
	}
}

// TestPartsBackKeyAppears 无头实测：从列表进选集后，表头真的出现「返回上一层列表」
// 这颗键，点它能回到列表。从主区进选集时不该出现。
func TestPartsBackKeyAppears(t *testing.T) {
	for _, from := range []string{"feed", DrawerSearch} {
		from := from
		a := NewApp(func() {})
		a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
		a.Drawer = from
		a.BrowseInfo = &Info{Bvid: "BV9", Cid: 9, Title: "浏览中",
			Parts: []Part{{Cid: 9, Part: "P1"}, {Cid: 10, Part: "P2"}}}
		a.DrawerBack = partsBackTarget(from, Sections)
		a.Drawer = DrawerParts

		tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
		if err := tt.Click("返回上一层列表"); err != nil {
			t.Fatalf("从 %q 进选集后点不到返回键：%v", from, err)
		}
		tt.Frame()
		if a.Drawer != from {
			t.Fatalf("点了返回后 Drawer = %q，应回到 %q", a.Drawer, from)
		}
	}

	// 反证：从主区进选集（没有 DrawerBack）时不该有这颗键。
	a := NewApp(func() {})
	a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.BrowseInfo = &Info{Bvid: "BV9", Cid: 9, Title: "浏览中",
		Parts: []Part{{Cid: 9, Part: "P1"}}}
	a.Drawer = DrawerParts
	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	if err := tt.Click("返回上一层列表"); err == nil {
		t.Fatal("从主区进选集时不该出现返回键")
	}
}

// TestPlaylistStart 钉住 issue #37 ② 的起播判据：播放列表里有东西时播放键可用。
func TestPlaylistStart(t *testing.T) {
	for _, tc := range []struct {
		name           string
		tab            string
		userN, seriesN int
		wantList       string
		wantOK         bool
	}{
		{"当前 tab 是「我的列表」且非空", ListUser, 3, 0, ListUser, true},
		{"当前 tab 是「合集列表」且非空", ListSeries, 3, 2, ListSeries, true},
		{"停在空的合集 tab，我的列表有歌", ListSeries, 2, 0, ListUser, true},
		{"停在空的我列表 tab，合集有歌", ListUser, 0, 2, ListSeries, true},
		{"两个都空", ListUser, 0, 0, "", false},
	} {
		list, index, ok := PlaylistStart(tc.tab, ListUser, tc.userN, tc.seriesN)
		if ok != tc.wantOK {
			t.Errorf("%s：ok = %v，想 %v", tc.name, ok, tc.wantOK)
			continue
		}
		if !ok {
			continue
		}
		if list != tc.wantList {
			t.Errorf("%s：list = %q，想 %q", tc.name, list, tc.wantList)
		}
		if index != 0 {
			t.Errorf("%s：index = %d，想 0（从第一条开始）", tc.name, index)
		}
	}
}

// TestPlaylistPlayable 钉住「播放键该不该可操作」的判据。
func TestPlaylistPlayable(t *testing.T) {
	if playlistPlayable(0, 0) {
		t.Fatal("两个列表都空时不该可播")
	}
	if !playlistPlayable(1, 0) || !playlistPlayable(0, 1) {
		t.Fatal("任一列表非空就该可播")
	}
}

// TestHasSourceWithPlaylistOnly 是 issue #37 ② 的**核心**断言：还没选过任何视频、
// 但播放列表里有条目时，播放栏的播放键必须是可操作的（不再 disabled）。
func TestHasSourceWithPlaylistOnly(t *testing.T) {
	a := NewApp(func() {})
	if a.hasSource() {
		t.Fatal("空队列 + 空播放列表：不该有源")
	}
	a.Playlist = []PlayItem{{Bvid: "BV1", Cid: 1, Title: "标题"}}
	if !a.hasSource() {
		t.Fatal("播放列表有 1 条但队列为空：播放键应可操作（issue #37 ②）")
	}
	// 合集列表同理（用户可能停在空的「我的列表」tab 上）。
	a.Playlist = nil
	a.SeriesPlaylist = []PlayItem{{Bvid: "BV2", Cid: 2, Title: "标题"}}
	if !a.hasSource() {
		t.Fatal("合集列表有 1 条但队列为空：播放键应可操作")
	}
}

// TestPlayButtonEnabledWithPlaylistOnly 无头实测：只有播放列表有歌时，播放栏
// 的播放键真的不再是禁用态（能点到、能发出 TogglePlay）。
func TestPlayButtonEnabledWithPlaylistOnly(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Playlist = []PlayItem{{Bvid: "BV1", Cid: 1, Title: "标题"}}
	plays := 0
	a.Act.TogglePlay = func() { plays++ }

	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	// 播放键的 Tooltip 是 playTip(false) = 「播放」。
	if err := tt.Click("播放"); err != nil {
		t.Fatalf("只有播放列表有歌时点不到播放键（应为可操作态）：%v", err)
	}
	tt.Frame()
	if plays != 1 {
		t.Fatalf("点了播放键：plays = %d，想 1", plays)
	}

	// 反证：两个列表都空时播放键仍是禁用态 —— 框架的 Disabled 让 Clicked()
	// 恒假，所以点下去不会发出 TogglePlay。
	//
	// 只渲染播放栏（不渲染整个 Shell）：主区的唱片键也带「播放」标签，
	// 它不受 src 约束，整窗里点「播放」会先命中唱片。
	b := NewApp(func() {})
	b.Theme = ThemeFor(PeriodAt(12))
	bare := 0
	b.Act.TogglePlay = func() { bare++ }
	tt2 := settleTester(ui.NewTester(func(c *ui.Context) { b.playerBar(c) }, 800, 600))
	if err := tt2.Click("播放"); err != nil {
		t.Fatalf("播放栏的播放键应仍能被命中测试找到（禁用不等于消失）：%v", err)
	}
	tt2.Frame()
	if bare != 0 {
		t.Fatalf("空队列 + 空播放列表时点了播放键：bare = %d，想 0（应保持禁用）", bare)
	}

	// 对照：同一个只有播放栏的窗口，播放列表有条目时点下去真的切播放。
	withList := NewApp(func() {})
	withList.Theme = ThemeFor(PeriodAt(12))
	withList.Playlist = []PlayItem{{Bvid: "BV1", Cid: 1, Title: "标题"}}
	withPlays := 0
	withList.Act.TogglePlay = func() { withPlays++ }
	tt3 := settleTester(ui.NewTester(func(c *ui.Context) { withList.playerBar(c) }, 800, 600))
	if err := tt3.Click("播放"); err != nil {
		t.Fatalf("播放列表有条目时点不到播放栏的播放键：%v", err)
	}
	tt3.Frame()
	if withPlays != 1 {
		t.Fatalf("播放列表有条目时点播放键：plays = %d，想 1", withPlays)
	}
}

// TestLongListsKeepScrollbars 钉住 mygo 0.8.0 的 KeepScrollbars 用法：卡片网格
// 与播放列表这两种**长列表**的滚动条要常驻（框架默认只在悬停时出现）。
//
// 为什么重要：卡片网格靠「滚到底自动翻页」，播放列表可能上百条 —— 拇指只在
// 悬停时闪一下，用户看不出自己在哪个位置、也判断不了「到底了没有」。
// 只对这两种长列表开；短列表（合集胶囊、UP 空间）保持原版的悬停行为。
func TestLongListsKeepScrollbars(t *testing.T) {
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败: %v", err)
	}
	code := string(src)
	// 卡片网格的 ui.List 链上必须有 KeepScrollbars(true)。
	i := strings.Index(code, "ui.List(c, st, rows+extra, func(r int) {")
	if i < 0 {
		t.Fatal("找不到卡片网格的 ui.List")
	}
	if end := strings.Index(code[i:], "\n}"); end > 0 {
		if !strings.Contains(code[i:i+end], "KeepScrollbars(true)") {
			t.Error("卡片网格（长列表 + 自动翻页）应 KeepScrollbars(true)")
		}
	}

	psrc, err := os.ReadFile("playlist.go")
	if err != nil {
		t.Fatalf("读 playlist.go 失败: %v", err)
	}
	if !strings.Contains(string(psrc), "KeepScrollbars(true)") {
		t.Error("播放列表（可能上百条）应 KeepScrollbars(true)")
	}
}

// TestPartPillNeverWiderThanItsColumn 是绘制实测：选集胶囊在**最窄窗**
// （400px，见 main.go 的 MinWidth）下不得溢出右栏。
//
// 此前胶囊是 MaxWidth(430) 的定宽盒，而右栏内容区只有 ~366 —— 窄窗里它会横向
// 溢到栏外。mygo 0.8.0 起 DIP 与百分比两种上限同时生效（取更小者，等价 CSS 的
// `max-width: min(78%, 34rem)`），所以补一条 MaxWidthPercent(100)。
func TestPartPillNeverWiderThanItsColumn(t *testing.T) {
	for _, w := range []int{400, 600, 800} {
		w := w
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
		a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题", Part: "P1"}
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Column(c).Padding(20).Children(func() { a.partPill(c) })
		}, w, 200)
		tst.Frame()
		r, ok := tst.Find("当前选集")
		if !ok {
			t.Fatalf("窗宽 %d：找不到选集胶囊", w)
		}
		// 可用宽度 = 窗宽 − 20×2 的内边距。
		if avail := float32(w) - 40; r.W > avail+0.5 {
			t.Errorf("窗宽 %d：胶囊宽 %.1f 超过可用宽 %.1f（应受 MaxWidthPercent 约束）",
				w, r.W, avail)
		}
	}
}

// TestDrawerBackResidueIsCleared 是审计发现的那类缺陷的回归：DrawerBack 只在
// 「选集」这一层有意义，一旦当前抽屉不是选集就必须清掉。
//
// 组合路径：历史/收藏 → 点卡片进选集（DrawerBack="history"）→ 点一集播放或
// 按 Esc → Drawer 回到列表（或空）→ **但 DrawerBack 若残留**，下次从主区点
// 「选集」就会错误地长出「返回上一层列表」，点它跳到那个早就不相关的历史列表。
func TestDrawerBackResidueIsCleared(t *testing.T) {
	// 情形 A：走 closeDrawer（关闭键 / Esc / 点遮罩 / 下滑关闭）。
	a := NewApp(func() {})
	a.Drawer = "history"
	a.BrowseTo(&Info{Bvid: "BV1"})
	a.closeDrawer()
	if a.DrawerBack != "" {
		t.Fatalf("closeDrawer 后 DrawerBack 应清空，得到 %q", a.DrawerBack)
	}

	// 情形 B：装配层（playIndex）在选集里点一集播放，走 CloseDrawer。
	b := NewApp(func() {})
	b.Drawer = "favorite"
	b.BrowseTo(&Info{Bvid: "BV1"})
	b.CloseDrawer()
	if b.DrawerBack != "" {
		t.Fatalf("CloseDrawer 后 DrawerBack 应清空，得到 %q", b.DrawerBack)
	}

	// 情形 C：**残留也自愈** —— 只要有别的路径把 Drawer 挪出选集，下一帧 Shell
	// 必须把 DrawerBack 清掉（这是覆盖所有关闭路径的那条不变量）。
	for _, drawer := range []string{"", "history", "feed", DrawerSearch} {
		a := NewApp(func() {})
		a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
		a.DrawerBack = "history" // 模拟残留
		a.Drawer = drawer
		ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600).Frame()
		if a.DrawerBack != "" {
			t.Errorf("Drawer = %q 时残留的 DrawerBack 应被清掉，得到 %q", drawer, a.DrawerBack)
		}
	}
}

// TestOpenPartsFromMainHasNoBack 钉住主区「选集」入口的边界：那里没有上一层，
// 必须显式清掉 DrawerBack，不能沿用上一次从列表进选集记下的来源。
func TestOpenPartsFromMainHasNoBack(t *testing.T) {
	a := NewApp(func() {})
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.DrawerBack = "history" // 上一次从列表进选集留下的
	a.openParts()
	if a.Drawer != DrawerParts {
		t.Fatalf("Drawer = %q，应为选集", a.Drawer)
	}
	if a.DrawerBack != "" {
		t.Fatalf("从主区打开选集时 DrawerBack 应清空，得到 %q（否则会长出错误的返回键）", a.DrawerBack)
	}

	// 反向：从列表进来时返回键仍要可用（别把上面的修复写成「永远清空」）。
	b := NewApp(func() {})
	b.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
	b.Drawer = "history"
	b.BrowseTo(&Info{Bvid: "BV1"})
	if b.DrawerBack != "history" {
		t.Fatalf("从历史列表进选集：DrawerBack = %q，应为 history", b.DrawerBack)
	}
}

// TestEscFromPartsGoesBackToList 钉住 Esc 走的是 closeDrawer（不是裸写
// Drawer=""）：从列表点进选集后按 Esc，应回到那个列表而不是落回主区。
//
// 这条是审计指出的缺陷 ①：提交说明与 PLAN 都写「closeDrawer 是关闭键 / Esc /
// 点遮罩 / 下滑关闭的统一入口」，但 Esc 分支当时仍是 a.Drawer = ""。
func TestEscFromPartsGoesBackToList(t *testing.T) {
	a := NewApp(func() {})
	a.Info = &Info{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.Drawer = "history"
	a.BrowseTo(&Info{Bvid: "BV9", Cid: 9, Title: "浏览中",
		Parts: []Part{{Cid: 9, Part: "P1"}}})

	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if a.Drawer != "history" {
		t.Fatalf("从历史列表进选集后按 Esc：Drawer = %q，应回到 history（不是落回主区）", a.Drawer)
	}
	if a.DrawerBack != "" {
		t.Fatalf("按 Esc 回到列表后 DrawerBack 应清空，得到 %q", a.DrawerBack)
	}
}

// TestAllDrawerClosePathsShareOneEntry 钉住 issue #37 ① 的**统一入口**约定：
// 关闭键 / Esc / 点遮罩 / 下滑关闭 / 各入口的「再点一次关掉」，都必须能回到
// 来时的列表（并在没有上层时清空 DrawerBack）。
//
// 这条测试是审计发现「Esc 分支仍裸写 a.Drawer=""、与文档不符」之后的回归护栏：
// 它逐个可切换的入口调用一遍，断言从列表进选集再关掉，都会回到那个列表。
func TestAllDrawerClosePathsShareOneEntry(t *testing.T) {
	// 每个「再点一次就关掉」的入口：从该抽屉点进选集，再调用同一入口。
	for _, tc := range []struct {
		name string
		open func(*App)
		list string
	}{
		{"分区入口（动态）", func(a *App) { a.openSection("feed") }, "feed"},
		{"合集入口", func(a *App) { a.openSeries() }, DrawerSeries},
		{"弹幕入口", func(a *App) { a.openDanmaku() }, DrawerDanmaku},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			a := NewApp(func() {})
			a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
			a.SeriesID = 7
			// 先打开列表，再从列表进选集（记下来源）。
			a.Drawer = tc.list
			a.BrowseTo(&Info{Bvid: "BV9", Cid: 9, Title: "浏览中",
				Parts: []Part{{Cid: 9, Part: "P1"}}})
			// 再调用同一入口：应当「关掉选集」，回到列表而不是落回主区。
			tc.open(a)
			if a.Drawer != tc.list {
				t.Fatalf("%s：再次触发入口后 Drawer = %q，应回到 %q", tc.name, a.Drawer, tc.list)
			}
			if a.DrawerBack != "" {
				t.Fatalf("%s：返回后 DrawerBack = %q，应清空", tc.name, a.DrawerBack)
			}
		})
	}

	// 没有上层的选集：任何关闭路径都该真的关掉（并清空）。
	for _, tc := range []struct {
		name  string
		close func(*App)
	}{
		{"closeDrawer", func(a *App) { a.closeDrawer() }},
		{"CloseDrawer（装配层）", func(a *App) { a.CloseDrawer() }},
		{"backToDrawer", func(a *App) { a.backToDrawer() }},
	} {
		a := NewApp(func() {})
		a.Drawer = DrawerParts
		tc.close(a)
		if a.Drawer != "" {
			t.Errorf("%s：没有上层时应收成空抽屉，得到 %q", tc.name, a.Drawer)
		}
		if a.DrawerBack != "" {
			t.Errorf("%s：没有上层时 DrawerBack 应为空，得到 %q", tc.name, a.DrawerBack)
		}
	}
}
