package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/media"
	"github.com/vst93/bili-fm/app/internal/store"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestWireActionsWiresEverything 用反射钉住 wireActions 把 Actions 的**每个**
// 字段都接上了。漏接在界面上表现为「按钮看着能点、点了什么都不发生」——
// 编译器不会报错、已有的界面测试也不会挂（它们自己把 Act 填好了）。
//
// 真发生过：主区的点赞 / 投币 / 收藏一直指向 view.Actions 的 Like/Coin/Favorite，
// 而 wireActions 从没给这三个字段赋过值（Act 为 nil，statButton 会静默跳过）。
func TestWireActionsWiresEverything(t *testing.T) {
	c := &controller{app: &view.App{}}
	c.wireActions()
	v := reflect.ValueOf(c.app.Act)
	tp := v.Type()
	for i := 0; i < tp.NumField(); i++ {
		f := v.Field(i)
		if f.Kind() == reflect.Func && f.IsNil() {
			t.Errorf("Actions.%s 没有接线（对应控件会静默失效）", tp.Field(i).Name)
		}
	}
}

func TestParseDurText(t *testing.T) {
	cases := map[string]int64{
		"":        0,
		"32:25":   32*60 + 25,
		"1:02:03": 3723,
		"00:00":   0,
	}
	for in, want := range cases {
		if got := parseDurText(in); got != want {
			t.Errorf("parseDurText(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSponsorReason(t *testing.T) {
	// 成功 / 无需查询：tooltip 里不附原因。
	if got := sponsorReason(nil); got != "" {
		t.Errorf("sponsorReason(nil) = %q, want empty", got)
	}
	// 失败：原版 sponsorStatus.reason 就是错误文案。
	if got := sponsorReason(errors.New("sponsorblock 返回 500")); got != "sponsorblock 返回 500" {
		t.Errorf("sponsorReason(err) = %q", got)
	}
}

func TestParseCountText(t *testing.T) {
	cases := map[string]int64{
		"":     0,
		"999":  999,
		"1.2万": 12000,
		"4.2亿": 420000000,
	}
	for in, want := range cases {
		if got := parseCountText(in); got != want {
			t.Errorf("parseCountText(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestFmtDurAndViews(t *testing.T) {
	if got := fmtDur(65); got != "1:05" {
		t.Errorf("fmtDur = %q", got)
	}
	if got := fmtDur(3725); got != "1:02:05" {
		t.Errorf("fmtDur = %q", got)
	}
	if got := fmtDur(0); got != "" {
		t.Errorf("fmtDur(0) = %q, want empty", got)
	}
	if got := fmtViews(42100); got != "4.2万" {
		t.Errorf("fmtViews = %q", got)
	}
}

func TestProgressLabel(t *testing.T) {
	if got := progressLabel(30, 100); got != "已看 30%" {
		t.Errorf("progressLabel = %q", got)
	}
	// -1 表示已看完，不能按比例算出负数。
	if got := progressLabel(-1, 100); got != "已看完" {
		t.Errorf("progressLabel(-1) = %q", got)
	}
	if got := progressLabel(0, 100); got != "" {
		t.Errorf("progressLabel(0) = %q, want empty", got)
	}
}

func TestBvidFromURL(t *testing.T) {
	cases := map[string]string{
		"https://www.bilibili.com/video/BV1xx411c7mD": "BV1xx411c7mD",
		"BV1xx411c7mD":                "BV1xx411c7mD",
		"看看这个 BV1Abc123 视频":           "BV1Abc123",
		"https://example.com/nothing": "",
		"":                            "",
	}
	for in, want := range cases {
		if got := bvidFromURL(in); got != want {
			t.Errorf("bvidFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRandOtherIndex(t *testing.T) {
	for _, n := range []int{0, 1} {
		if got := randomOtherIndex(n, 0); got != 0 {
			t.Errorf("randomOtherIndex(%d) = %d, want 0", n, got)
		}
	}
	for i := 0; i < 50; i++ {
		got := randomOtherIndex(4, 2)
		if got == 2 || got < 0 || got > 3 {
			t.Fatalf("randomOtherIndex(4,2) = %d", got)
		}
	}
}

func TestToCardsHistoryItem(t *testing.T) {
	// 观看历史：bvid/aid 在 history 子对象里，作者是 author_name。
	items := []any{map[string]any{
		"title":       "看过的视频",
		"cover":       "c.jpg",
		"author_name": "某UP",
		"view_at":     float64(1700000000),
		"progress":    float64(30),
		"duration":    float64(100),
		"history":     map[string]any{"bvid": "BVhist", "oid": float64(123)},
	}}
	cards := toCards(items)
	if len(cards) != 1 {
		t.Fatalf("数量 = %d", len(cards))
	}
	c := cards[0]
	if c.Bvid != "BVhist" || c.Track.Aid != 123 {
		t.Errorf("history card = %+v", c)
	}
	if c.Extra != "已看 30%" {
		t.Errorf("extra = %q", c.Extra)
	}
}

func TestToCardsFavoriteAndWatchLater(t *testing.T) {
	// 收藏：作者在 upper.name，播放量在 cnt_info.play。
	fav := []any{map[string]any{
		"bvid": "BVfav", "title": "收藏的视频", "pic": "p.jpg",
		"duration": float64(200),
		"upper":    map[string]any{"name": "收藏UP"},
		"cnt_info": map[string]any{"play": float64(12345)},
	}}
	c := toCards(fav)
	if len(c) != 1 || c[0].Up != "收藏UP" || c[0].Views != "1.2万" || c[0].Duration != "3:20" {
		t.Errorf("fav card = %+v", c)
	}

	// 稍后再看：owner.name + stat.view + progress=-1（已看完）。
	wl := []any{map[string]any{
		"bvid": "BVwl", "title": "稍后再看", "pic": "w.jpg",
		"duration": float64(60),
		"owner":    map[string]any{"name": "WLUP"},
		"stat":     map[string]any{"view": float64(999)},
		"progress": float64(-1),
	}}
	c = toCards(wl)
	if len(c) != 1 || c[0].Up != "WLUP" || c[0].Views != "999" || c[0].Extra != "已看完" {
		t.Errorf("watch later card = %+v", c)
	}
}

func TestToDanmakuListCaps(t *testing.T) {
	items := make([]bilibili.DanmakuItem, maxDanmaku+50)
	for i := range items {
		items[i] = bilibili.DanmakuItem{Content: fmt.Sprintf("dm%d", i), Time: float64(i), Color: 0xff0000}
	}
	got := toDanmakuList(items)
	if len(got) != maxDanmaku {
		t.Fatalf("len = %d, want %d", len(got), maxDanmaku)
	}
	if got[0].Text != "dm0" || got[0].Color != 0xff0000 {
		t.Errorf("first = %+v", got[0])
	}
}

func TestClampToAreas(t *testing.T) {
	primary := screenArea{0, 0, 1920, 1080}
	right := screenArea{1920, 0, 1920, 1080}

	// 点在屏内：原样保留。
	if x, y := clampToAreas(100, 100, 400, 155, []screenArea{primary}); x != 100 || y != 100 {
		t.Errorf("inside = (%d,%d)", x, y)
	}
	// 刻意挂在右下角（左上角仍在可用区内）：原样保留。
	if x, y := clampToAreas(1910, 1070, 400, 155, []screenArea{primary}); x != 1910 || y != 1070 {
		t.Errorf("hanging = (%d,%d)", x, y)
	}
	// 点在第二块屏里：原样保留。
	if x, y := clampToAreas(2500, 500, 400, 155, []screenArea{primary, right}); x != 2500 || y != 500 {
		t.Errorf("second display = (%d,%d)", x, y)
	}
	// 副屏被拔掉：夹回主屏右边缘内。
	if x, y := clampToAreas(2500, 500, 400, 155, []screenArea{primary}); x != 1520 || y != 500 {
		t.Errorf("rescued = (%d,%d), want (1520,500)", x, y)
	}
	// 负坐标（屏幕左边）：夹到 (0,0)。
	if x, y := clampToAreas(-800, -100, 400, 155, []screenArea{primary}); x != 0 || y != 0 {
		t.Errorf("negative = (%d,%d)", x, y)
	}
	// 没有屏幕信息：不动。
	if x, y := clampToAreas(123, 456, 400, 155, nil); x != 123 || y != 456 {
		t.Errorf("no areas = (%d,%d)", x, y)
	}
}

func TestSetCardsRetention(t *testing.T) {
	list := &view.List{}
	first := make([]view.Card, 100)
	setCards(list, first, true)
	if len(list.Cards) != 100 {
		t.Fatalf("replace len = %d", len(list.Cards))
	}
	more := make([]view.Card, 100)
	setCards(list, more, false)
	if len(list.Cards) != maxRetainedCards {
		t.Fatalf("retention len = %d, want %d", len(list.Cards), maxRetainedCards)
	}
	// 超上限时从头部释放，保留尾部。
	list2 := &view.List{}
	setCards(list2, make([]view.Card, maxRetainedCards+30), true)
	if len(list2.Cards) != maxRetainedCards {
		t.Fatalf("cap len = %d", len(list2.Cards))
	}
}

func TestToCards(t *testing.T) {
	items := []any{
		map[string]any{
			"bvid": "BV1", "pic": "//i0.hdslb.com/a.jpg", "title": "标题",
			"duration": float64(125),
			"owner":    map[string]any{"name": "作者"},
			"stat":     map[string]any{"view": float64(42000)},
			"aid":      float64(9),
		},
		// 没有 bvid 的条目（活动卡片）应被跳过。
		map[string]any{"title": "活动", "pic": "x"},
	}
	cards := toCards(items)
	if len(cards) != 1 {
		t.Fatalf("toCards 数量 = %d, want 1", len(cards))
	}
	c := cards[0]
	if c.Bvid != "BV1" || c.Title != "标题" || c.Up != "作者" {
		t.Errorf("card = %+v", c)
	}
	if c.Duration != "2:05" {
		t.Errorf("duration = %q, want 2:05", c.Duration)
	}
	if c.Views != "4.2万" {
		t.Errorf("views = %q, want 4.2万", c.Views)
	}
	if c.Track.Aid != 9 {
		t.Errorf("track aid = %d", c.Track.Aid)
	}
}

func TestToUpCards(t *testing.T) {
	items := []any{
		map[string]any{
			"modules": map[string]any{
				"module_dynamic": map[string]any{
					"major": map[string]any{
						"archive": map[string]any{
							"bvid": "BVup", "title": "空间视频", "cover": "c.jpg",
							"duration_text": "03:20", "aid": float64(7),
							"stat":  map[string]any{"play": "69.2万"},
							"badge": map[string]any{"text": "充电专属"},
						},
					},
				},
				"module_author": map[string]any{"name": "UP主", "pub_time": "3天前"},
			},
		},
		// 图文动态没有 archive，应跳过。
		map[string]any{"modules": map[string]any{"module_dynamic": map[string]any{"major": map[string]any{"opus": map[string]any{}}}}},
	}
	cards := toUpCards(items)
	if len(cards) != 1 {
		t.Fatalf("toUpCards 数量 = %d, want 1", len(cards))
	}
	c := cards[0]
	if c.Bvid != "BVup" || c.Views != "69.2万" || c.Duration != "03:20" || c.Up != "UP主" {
		t.Errorf("up card = %+v", c)
	}
	// UP 空间列表不带标题前胶囊（原版 upVideoList 不传 titlePrefix）。
	if c.Badge != "" {
		t.Errorf("toUpCards badge = %q, want 空", c.Badge)
	}
	if c.Track.Aid != 7 {
		t.Errorf("track aid = %d", c.Track.Aid)
	}

	// 动态列表（feedList）才解析「充电专属」胶囊。
	feed := toFeedCards(items)
	if len(feed) != 1 {
		t.Fatalf("toFeedCards 数量 = %d, want 1", len(feed))
	}
	if feed[0].Badge != "充电专属" {
		t.Errorf("feed badge = %q, want 充电专属", feed[0].Badge)
	}
}

// TestSeriesCardMeta 钉住合集（series）抽屉卡片的 meta 数据（原版
// seriesList 的 ListCard fields = [views, pubdate]）：播放量、发布时间都要有，
// 作者列留空（合集里都是同一个 UP，原版不显示作者）。
//
// 真发生过：这里此前只填了 Views / Duration，漏了 Pubdate —— 合集卡片的 meta
// 行只剩一个播放量。
func TestSeriesCardMeta(t *testing.T) {
	ar := bilibili.SeriesVideosResponse_Archive{
		Aid: 9, Bvid: "BVseries", Title: "合集里的视频",
		Pic: "p.jpg", Duration: 125, Pubdate: 1700000000,
	}
	ar.Stat.View = 42000
	c := seriesCard(ar, "某UP")
	if c.Bvid != "BVseries" || c.Title != "合集里的视频" || c.Cover != "p.jpg" {
		t.Errorf("seriesCard = %+v", c)
	}
	if c.Views != "4.2万" {
		t.Errorf("views = %q, want 4.2万", c.Views)
	}
	if c.Pubdate == "" {
		t.Errorf("pubdate 为空，合集卡片应有发布时间（原版 fields 含 pubdate）")
	}
	if c.Duration != "2:05" {
		t.Errorf("duration = %q, want 2:05", c.Duration)
	}
	// 原版合集卡片没有作者列：Up 必须留空，meta 行才会跳过作者列。
	if c.Up != "" {
		t.Errorf("up = %q, 合集卡片不该有作者列", c.Up)
	}
	// Track 仍要带上 UP 名，正在播放的信息才完整。
	if c.Track.Up != "某UP" || c.Track.Aid != 9 {
		t.Errorf("track = %+v", c.Track)
	}

	// pubdate 为 0（接口没给）时不显示（原版 `video?.pubdate ? … : null`）。
	ar.Pubdate = 0
	if got := seriesCard(ar, "某UP").Pubdate; got != "" {
		t.Errorf("pubdate=0 时 = %q, want 空", got)
	}
}

// TestSpecialBadge 钉住「普通视频不打标签，只有特殊徽标才打」：
// B 站的 archive.badge.text 对普通投稿会给出「投稿视频」「动态视频」这类
// 来源分类，原版 feedList 只在 text == "充电专属" 时才渲染胶囊，本构建
// 也不得把来源分类透传成标签（用户实机反馈）。特殊徽标（充电/合作/联合）
// 照旧显示。
func TestSpecialBadge(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", ""},
		{"投稿视频", ""},
		{"动态视频", ""},
		{"推荐视频", ""},
		{"充电专属", "充电专属"},
		{"合作视频", "合作视频"},
		{"联合投稿", "联合投稿"},
	}
	for _, c := range cases {
		if got := specialBadge(c.in); got != c.want {
			t.Errorf("specialBadge(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	// 端到端：动态接口里普通投稿的 badge.text（“投稿视频”）不应变成卡片胶囊。
	item := func(text string) []any {
		return []any{map[string]any{"modules": map[string]any{
			"module_dynamic": map[string]any{"major": map[string]any{"archive": map[string]any{
				"bvid": "BV1", "title": "普通投稿",
				"badge": map[string]any{"text": text},
			}}},
			"module_author": map[string]any{"name": "UP", "pub_time": "3天前"},
		}}}
	}
	if got := toFeedCards(item("投稿视频"))[0].Badge; got != "" {
		t.Errorf("普通投稿 badge = %q, want 空（不得显示来源分类）", got)
	}
	if got := toFeedCards(item("充电专属"))[0].Badge; got != "充电专属" {
		t.Errorf("充电专属 badge = %q, want 充电专属", got)
	}
}

func TestPlaylistReorderMath(t *testing.T) {
	// 直接验证 reorder 的核心切片操作（不依赖 controller）。
	reorder := func(list []view.PlayItem, from, to int) []view.PlayItem {
		it := list[from]
		out := append(list[:from:from], list[from+1:]...)
		rest := append(out[:to:to], it)
		return append(rest, out[to:]...)
	}
	base := []view.PlayItem{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}
	got := reorder(append([]view.PlayItem(nil), base...), 0, 2)
	want := []string{"b", "c", "a", "d"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("reorder 0->2 = %v, want %v", ids(got), want)
		}
	}
	got = reorder(append([]view.PlayItem(nil), base...), 3, 1)
	want = []string{"a", "d", "b", "c"}
	for i, id := range want {
		if got[i].ID != id {
			t.Fatalf("reorder 3->1 = %v, want %v", ids(got), want)
		}
	}
}

// TestReorderIndex 钉住原版 handlePlaylistReorder 对 currentPlaylistIndex 的
// 跟随逻辑（main:src/pages/index.tsx）。
func TestReorderIndex(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cur, from, to int
		want          int
	}{
		{"移动当前项往下", 2, 2, 4, 4},
		{"移动当前项往上", 2, 2, 0, 0},
		{"别的记录从下往上跨过当前项", 2, 4, 0, 3},
		{"别的记录从上往下跨过当前项", 2, 0, 4, 1},
		{"当前项之前的记录下移但没跨过", 2, 0, 1, 2},
		{"当前项之后的记录上移但没跨过", 2, 4, 3, 2},
		{"别的记录跨越到当前项下一位", 2, 0, 3, 1},
		{"没有当前项", -1, 0, 3, -1},
	} {
		if got := reorderIndex(tc.cur, tc.from, tc.to); got != tc.want {
			t.Errorf("%s：reorderIndex(%d, %d, %d) = %d，应为 %d",
				tc.name, tc.cur, tc.from, tc.to, got, tc.want)
		}
	}
}

// TestReorderPlaylistTracksCurrent 是上面那条的端到端版本：拖完行以后，
// a.Index 必须仍然指向同一条记录（正在播的那条）。改前 reorderPlaylist 只
// 重排切片、完全不动 a.Index，于是「当前播放」高亮会留在旧下标 —— 那里已经
// 是另一条记录，下一首/上一首与「定位当前」也跟着走错。
func TestReorderPlaylistTracksCurrent(t *testing.T) {
	for _, tc := range []struct {
		name          string
		cur, from, to int
		want          int
	}{
		{"拖走当前项", 2, 2, 4, 4},
		{"别的记录从下往上跨过当前项", 2, 4, 1, 3},
		{"别的记录从上往下跨过当前项", 2, 0, 4, 1},
		{"没跨过当前项（前移）", 2, 4, 3, 2},
		{"当前项就在头", 0, 3, 0, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kv, err := store.Open(t.TempDir(), false)
			if err != nil {
				t.Fatalf("store.Open: %v", err)
			}
			a := &view.App{
				Playlist: []view.PlayItem{
					{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}, {ID: "e"},
				},
				PlayingPlaylist: view.ListUser,
				PlaylistTab:     view.ListUser,
				Index:           tc.cur,
			}
			curID := a.Playlist[tc.cur].ID
			c := &controller{app: a, kv: kv}
			c.reorderPlaylist(view.ListUser, tc.from, tc.to)
			if a.Index != tc.want {
				t.Fatalf("Index = %d，应为 %d", a.Index, tc.want)
			}
			if got := a.Playlist[a.Index].ID; got != curID {
				t.Errorf("Index 指向 %q，应为正在播的 %q（列表 %v）", got, curID, ids(a.Playlist))
			}
		})
	}
}

// TestReorderPlaylistIgnoresIndexWhenNotPlayingFromUserList 钉住边界：当前播放
// 来源不是「我的列表」时，a.Index 是队列下标，拖我的列表不得碰它。
func TestReorderPlaylistIgnoresIndexWhenNotPlayingFromUserList(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a := &view.App{
		Playlist:        []view.PlayItem{{ID: "a"}, {ID: "b"}, {ID: "c"}},
		SeriesPlaylist:  []view.PlayItem{{ID: "s1"}, {ID: "s2"}},
		PlayingPlaylist: view.ListSeries,
		Index:           1,
	}
	c := &controller{app: a, kv: kv}
	c.reorderPlaylist(view.ListUser, 0, 2)
	if a.Index != 1 {
		t.Errorf("播合集时拖我的列表：Index = %d，应保持 1", a.Index)
	}
	// 合集列表本身也不该被拖（原版 series 行 draggable=false）。
	c.reorderPlaylist(view.ListSeries, 0, 1)
	if ids(a.SeriesPlaylist)[0] != "s1" {
		t.Errorf("合集列表被改动了：%v", ids(a.SeriesPlaylist))
	}
}

func ids(list []view.PlayItem) []string {
	out := make([]string, len(list))
	for i, it := range list {
		out[i] = it.ID
	}
	return out
}

func TestMetaDateText(t *testing.T) {
	// 相对时间随当前时间变：合法日期应该变成「N天前」之类，而不是原样返回。
	for _, in := range []string{"2024-05-01 12:00:00", "2024-05-01 12:00", "2024-05-01"} {
		got := metaDateText(in)
		if got == "" {
			t.Errorf("metaDateText(%q) = 空", in)
		}
		// relTime 是阶梯式：N天前 / MM-DD / N个月前 / N年前，均非空。
	}
	if got := metaDateText(""); got != "" {
		t.Errorf("metaDateText(\"\") = %q, want empty", got)
	}
	if got := metaDateText("不是日期"); got != "不是日期" {
		t.Errorf("metaDateText(非日期) = %q, want 原样", got)
	}
	// pubdate 为 0 时接口会给 1970 占位，应该不显示。
	if got := metaDateText("1970-01-01"); got != "" {
		t.Errorf("metaDateText(1970 占位) = %q, want empty", got)
	}
}

func TestToCardsViewsFallbackAndPubdate(t *testing.T) {
	// 播放量缺失时用弹幕数顶上（原版 viewsMetaField）。
	items := []any{map[string]any{
		"bvid": "BVdan", "title": "t", "pic": "p",
		"owner": map[string]any{"name": "UP"},
		"stat":  map[string]any{"danmaku": float64(12345)},
	}}
	c := toCards(items)
	if len(c) != 1 || c[0].Views != "1.2万" {
		t.Errorf("danmaku 回退 = %+v", c)
	}
	// 回退要标记出来，meta 行才会用评论图标（原版 viewsMetaField 的 icon）。
	if len(c) != 1 || !c[0].ViewsDanmaku {
		t.Errorf("danmaku 回退未标记 ViewsDanmaku = %+v", c)
	}

	// 收藏夹的 ctime / 推荐的 pubdate 都要变成相对时间（>1 年回落绝对日期，只要求非空）。
	items = []any{map[string]any{
		"bvid": "BVct", "title": "t2", "pic": "p",
		"upper": map[string]any{"name": "UP2"},
		"ctime": float64(1700000000),
	}}
	c = toCards(items)
	if len(c) != 1 || c[0].Pubdate == "" {
		t.Errorf("ctime pubdate = %+v", c)
	}
}

// TestToCardsRealZeroPlay 钉住「真实播放量 0」与「字段缺失」不能混同——
// 原版 viewsMetaField（修复轮 22 的问题 1）：字段存在就照实渲染 0，只有字段
// 缺失才回退弹幕数。此前 `n > 0` 的判断会把 0 播放的视频顶上弹幕数、挂评论
// 图标，正是原版专门修掉的混同。
func TestToCardsRealZeroPlay(t *testing.T) {
	// stat.view 明确为 0、且有弹幕数：要照实显示 0（播放图标），不回退弹幕。
	items := []any{map[string]any{
		"bvid": "BVzero", "title": "零播放", "pic": "p",
		"stat": map[string]any{"view": float64(0), "danmaku": float64(12345)},
	}}
	c := toCards(items)
	if len(c) != 1 || c[0].Views != "0" {
		t.Errorf("真实 0 播放应照实渲染 0 = %+v", c)
	}
	if len(c) == 1 && c[0].ViewsDanmaku {
		t.Errorf("真实 0 播放不该标记为弹幕回退 = %+v", c)
	}

	// 收藏夹：cnt_info.play 为真实 0，同样照实渲染，不回退 cnt_info.danmaku。
	fav := []any{map[string]any{
		"bvid": "BVzfav", "title": "收藏零播放", "pic": "p",
		"upper":    map[string]any{"name": "UP"},
		"cnt_info": map[string]any{"play": float64(0), "danmaku": float64(999)},
	}}
	c = toCards(fav)
	if len(c) != 1 || c[0].Views != "0" || c[0].ViewsDanmaku {
		t.Errorf("收藏真实 0 播放 = %+v", c)
	}

	// 字段整体缺失（连 stat 都没有）才回退弹幕数并挂评论图标。
	missing := []any{map[string]any{
		"bvid": "BVmiss", "title": "缺字段", "pic": "p",
		"danmaku": float64(5000),
	}}
	c = toCards(missing)
	if len(c) != 1 || c[0].Views != "5000" || !c[0].ViewsDanmaku {
		t.Errorf("字段缺失才回退弹幕 = %+v", c)
	}

	// stat.danmaku 为真实 0 时也照实渲染 0（不被当成缺失而落到外层 danmaku）。
	zeroDan := []any{map[string]any{
		"bvid": "BVzd", "title": "零弹幕", "pic": "p",
		"danmaku": float64(7777),
		"stat":    map[string]any{"danmaku": float64(0)},
	}}
	c = toCards(zeroDan)
	if len(c) != 1 || c[0].Views != "0" || !c[0].ViewsDanmaku {
		t.Errorf("stat.danmaku 真实 0 = %+v", c)
	}
}

// TestPickCountPresence 钉住 pickCount 的存在性语义（真实 0 ≠ 缺失）。
func TestPickCountPresence(t *testing.T) {
	if n, ok := pickCount(map[string]any{"view": float64(0)}, "view"); !ok || n != 0 {
		t.Errorf("pickCount(0) = (%d,%v), want (0,true)", n, ok)
	}
	if _, ok := pickCount(map[string]any{}, "view"); ok {
		t.Errorf("pickCount(缺字段) 应为 false")
	}
	if _, ok := pickCount(map[string]any{"view": ""}, "view"); ok {
		t.Errorf("pickCount(空串) 应为 false")
	}
	if _, ok := pickCount(map[string]any{"view": "x"}, "view"); ok {
		t.Errorf("pickCount(不可解析串) 应为 false")
	}
	if n, ok := pickCount(map[string]any{"view": "42"}, "view"); !ok || n != 42 {
		t.Errorf("pickCount(\"42\") = (%d,%v), want (42,true)", n, ok)
	}
}

func TestRelTimeLadder(t *testing.T) {
	// 对齐原版 dateFormLadder 的「最具体形态」。
	now := time.Now()
	// 「今天/昨天」的分界是自然日零点，跨零点跑测试会翻面，
	// 所以只用「确定在 7 天内」的输入，断言它落在阶梯的相对形态里。
	withinWeek := []struct {
		unix int64
	}{
		{now.Unix()},
		{now.Add(-2 * time.Hour).Unix()},
		{now.AddDate(0, 0, -1).Unix()},
		{now.AddDate(0, 0, -3).Unix()},
	}
	for _, c := range withinWeek {
		got := relTime(c.unix)
		switch {
		case got == "今天", got == "昨天", strings.HasSuffix(got, "天前"):
		default:
			t.Errorf("relTime(%s) = %q，7 天内应为 今天/昨天/N天前", time.Unix(c.unix, 0), got)
		}
	}
	// 当年（>7 天）→ MM-DD。
	target := now.AddDate(0, 0, -20)
	if got := relTime(target.Unix()); got != target.Format("01-02") {
		t.Errorf("relTime(当年-20天) = %q, want %q", got, target.Format("01-02"))
	}
	// 跨年 → 具体日期 yyyy-MM-DD（原版轮 30 的「具体日期优先」，不再压到 N年前）。
	old := now.AddDate(-2, 0, 0)
	if got := relTime(old.Unix()); got != old.Format("2006-01-02") {
		t.Errorf("relTime(2年前) = %q, want %q", got, old.Format("2006-01-02"))
	}
	// 相对旧的（>1 个月、同年）→ MM-DD（比「N个月前」更具体）。
	lastMonth := now.AddDate(0, -2, 0)
	if got := relTime(lastMonth.Unix()); got == "" {
		t.Errorf("relTime(2个月前) 为空")
	}
}

// TestKeepDrawerOnPlay 钉住「自动/后台起播不强制关抽屉」：播放列表模式下手动/
// 自动切歌不关（handlePlaylistVideoSelect 的 preserveUi），选集自动续播/播完
// 切集也不关（原版 handleVideoSelect 直接调用）；其余抽屉起播时关掉。
// 手动从选集面板点一集由 playIndex 先关面板，见
// TestPlayIndexClosesPartsDrawerOnSelect。
func TestKeepDrawerOnPlay(t *testing.T) {
	keep := map[string]bool{
		view.DrawerParts:    true,
		view.DrawerPlaylist: true,
		view.DrawerSearch:   false,
		"feed":              false,
		"favorite":          false,
		"":                  false,
	}
	for drawer, want := range keep {
		if got := keepDrawerOnPlay(drawer); got != want {
			t.Errorf("keepDrawerOnPlay(%q) = %v, want %v", drawer, got, want)
		}
	}
}

// TestPlayIndexClosesPartsDrawerOnSelect 钉住「手动从选集面板点一集会关面板」：
// 原版 PageList.handleVideoSelect 在 onVideoSelect 后调 handleOpenChange(false)，
// 面板关闭；原生版此前把这条路径也交给 keepDrawerOnPlay 保留，属于误读。
// 自动续播/播完切集仍走 startCurrent 的保留分支，不受这里影响。
func TestPlayIndexClosesPartsDrawerOnSelect(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	block := sourceBlock(t, src, "if c.app.Drawer == view.DrawerParts {")
	closeIdx := strings.Index(block, `c.app.Drawer = ""`)
	startIdx := strings.Index(block, "c.startCurrent()")
	if closeIdx < 0 {
		t.Fatal("playIndex 的选集分支没有关面板（缺少 c.app.Drawer = \"\"）")
	}
	if startIdx < 0 {
		t.Fatal("playIndex 的选集分支没有调用 c.startCurrent()")
	}
	if closeIdx > startIdx {
		t.Error("手动点选集应先关面板再起播（关面板必须在 startCurrent 之前）")
	}
}

// TestNoStartupQueueRestore 钉住「去掉启动时的历史播放项恢复」：启动路径
// （main.go / playlist.go）不得再引用 restoreQueue / mygo_queue —— 用户明确
// 要求不再自动恢复上次播放项（原版也不在启动时恢复）。这是回归闸门：谁再把
// 恢复逻辑加回来，这条会红。
func TestNoStartupQueueRestore(t *testing.T) {
	for _, file := range []string{"main.go", "playlist.go"} {
		src, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("读 %s 失败：%v", file, err)
		}
		for _, banned := range []string{"restoreQueue", "mygo_queue", "mygo_index"} {
			if bytes.Contains(src, []byte(banned)) {
				t.Errorf("%s 仍引用 %q（启动恢复应已移除）", file, banned)
			}
		}
	}
}

// TestFollowBlocked 钉住「UP 空间里关注/取关在途时连点忽略」的闸门（对应原版
// 关注键的 isFollowingLoading 禁用态）。没有这道闸门时，连点会先 follow 再
// unfollow，各发一次请求、状态来回跳。
func TestFollowBlocked(t *testing.T) {
	cases := []struct {
		up, pending, want bool
	}{
		{true, true, true},
		{true, false, false},
		{false, true, false},
		{false, false, false},
	}
	for _, c := range cases {
		if got := followBlocked(c.up, c.pending); got != c.want {
			t.Errorf("followBlocked(%v,%v) = %v, want %v", c.up, c.pending, got, c.want)
		}
	}
}

// TestSeriesPlayAllBlocked 钉住「播放全部」的重复点击闸门：一次全量拉取在途
// 时连点忽略（原版 seriesList 的 isPlayingAll 让按钮在请求期间禁用；否则每次
// 点击都会各起一套分页 + 详情请求）。
func TestSeriesPlayAllBlocked(t *testing.T) {
	if !seriesPlayAllBlocked(true) {
		t.Error("拉取在途时 seriesPlayAllBlocked = false，想 true")
	}
	if seriesPlayAllBlocked(false) {
		t.Error("空闲时 seriesPlayAllBlocked = true，想 false")
	}
}

// TestInteractToastTexts 钉住点赞 / 投币 / 收藏 / 关注的结果提示文案与方向
// （原版 videoInfo handleLike/handleCoin/handleFavorite、upVideoList 的关注）。
// 原生版此前这些成功路径全是静默的，用户点完看不出有没有生效。
func TestInteractToastTexts(t *testing.T) {
	if got := likeToastText(false); got != "点赞成功" {
		t.Errorf("未点赞时 likeToastText = %q", got)
	}
	if got := likeToastText(true); got != "取消点赞成功" {
		t.Errorf("已点赞时 likeToastText = %q", got)
	}
	if got := favToastText(false); got != "收藏成功" {
		t.Errorf("未收藏时 favToastText = %q", got)
	}
	if got := favToastText(true); got != "取消收藏成功" {
		t.Errorf("已收藏时 favToastText = %q", got)
	}
	if got := followToastText(true, "某个UP"); got != "已关注 某个UP" {
		t.Errorf("关注成功文案 = %q", got)
	}
	if got := followToastText(false, "某个UP"); got != "取消关注 某个UP" {
		t.Errorf("取关成功文案 = %q", got)
	}
	if toastCoinOK != "投币成功" || toastCoinAlready != "已经投过币了" {
		t.Errorf("投币文案 = %q / %q", toastCoinOK, toastCoinAlready)
	}
	// 失败提示走统一拼法。
	if got := actionFailText("点赞", errors.New("未登录")); got != "点赞失败：未登录" {
		t.Errorf("actionFailText = %q", got)
	}
	// eitherText 的两个方向都要对（关注 / 取关的定向文案靠它）。
	if eitherText(true, "甲", "乙") != "甲" || eitherText(false, "甲", "乙") != "乙" {
		t.Error("eitherText 取反了")
	}
}

// TestCoinOutcomeFor 钉住「已投过币仍可点、点了弹信息提示」这条分支。此前是
// 静默 return：投币键禁用判据只看有没有视频，所以键看着能点、点下去毫无反应。
func TestCoinOutcomeFor(t *testing.T) {
	cases := []struct {
		bvid   string
		coined bool
		want   coinOutcome
	}{
		{"", false, coinNoop},
		{"", true, coinNoop},
		{"BV1xx", true, coinAlready},
		{"BV1xx", false, coinRequest},
	}
	for _, c := range cases {
		if got := coinOutcomeFor(c.bvid, c.coined); got != c.want {
			t.Errorf("coinOutcomeFor(%q,%v) = %v, want %v", c.bvid, c.coined, got, c.want)
		}
	}
}

// TestInteractToastsWired 反证：这些提示必须真的出现在控制器里。纯函数测试
// 只能证明文案对，证明不了「接上了」—— 写了文案却没调用，用户依旧看不到提示。
func TestInteractToastsWired(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	for _, needle := range []string{
		`NotifyType("success", likeToastText(liked))`,
		`NotifyType("info", toastCoinAlready)`,
		`NotifyType("success", toastCoinOK)`,
		`NotifyType("success", favToastText(wasFaved))`,
		`NotifyType("success", followToastText(want, name))`,
		`actionFailText(fail, err)`,
		`eitherText(want, "无法关注该UP主", "无法取消关注该UP主")`,
	} {
		if !bytes.Contains(src, []byte(needle)) {
			t.Errorf("main.go 里找不到 %s（提示没接上？）", needle)
		}
	}
}

// TestAddToPlaylistUsesBrowseInfo 钉住选集卡片的「添加到播放列表」记录的是
// 「浏览中」那条视频（选集卡片属于它），而不是正在播放的那条 —— 否则浏览
// 另一条视频时加进来的记录会带上正在播放那条的 bvid / 标题。
func TestAddToPlaylistUsesBrowseInfo(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a := &view.App{
		Info:       &view.Info{Bvid: "BVplaying", Aid: 1, Title: "播放中"},
		BrowseInfo: &view.Info{Bvid: "BVbrowsed", Aid: 2, Title: "浏览中"},
	}
	c := &controller{app: a, kv: kv}
	c.addToPlaylist(view.Part{Cid: 42, Part: "P1"})
	if len(a.Playlist) != 1 {
		t.Fatalf("应加入 1 条记录，得到 %d", len(a.Playlist))
	}
	it := a.Playlist[0]
	if it.Bvid != "BVbrowsed" || it.Aid != 2 || it.Title != "浏览中" {
		t.Errorf("记录应来自浏览中那条：%+v", it)
	}
}

// TestAddAllToPlaylistUsesBrowseInfo 同上，钉住「全部添加」也读浏览中那条。
func TestAddAllToPlaylistUsesBrowseInfo(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a := &view.App{
		Info: &view.Info{Bvid: "BVplaying", Aid: 1, Title: "播放中", Parts: []view.Part{{Cid: 9}}},
		BrowseInfo: &view.Info{Bvid: "BVbrowsed", Aid: 2, Title: "浏览中", Parts: []view.Part{
			{Cid: 11, Part: "P1"}, {Cid: 12, Part: "P2"},
		}},
	}
	c := &controller{app: a, kv: kv}
	c.addAllToPlaylist()
	if len(a.Playlist) != 2 {
		t.Fatalf("应加入 2 条记录，得到 %d", len(a.Playlist))
	}
	for _, it := range a.Playlist {
		if it.Bvid != "BVbrowsed" || it.Aid != 2 || it.Title != "浏览中" {
			t.Errorf("记录应来自浏览中那条：%+v", it)
		}
	}
}

// TestBrowseWiringKeepsDisplayInfo 钉住 main.go 里「浏览」这条链路的接线：
// urlJump（列表点卡片 / 粘链接）与 openParts（打开当前视频的选集）只写
// BrowseInfo，选集抽屉与选集动作读 Browse() —— 都不能再写主区显示的 a.Info，
// 否则浏览另一条视频会把主区标题 / 封面 / 简介换掉（原版 displayVideoInfo
// 只跟 playingInfo）。
func TestBrowseWiringKeepsDisplayInfo(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	for _, needle := range []string{
		"a.BrowseTo(info)",       // urlJump：记浏览 + 开选集面板
		"a.BrowseInfo = info",    // openParts：打开当前视频的选集
		"info := c.app.Browse()", // playIndex：选集抽屉的队列来源
	} {
		if !bytes.Contains(src, []byte(needle)) {
			t.Errorf("main.go 里找不到 %s（浏览 / 显示的分离没接上？）", needle)
		}
	}
	// urlJump 不能再把浏览的那条写进主区显示。
	if bytes.Contains(src, []byte("a.Info = info")) {
		t.Error("main.go 里还有 `a.Info = info`：浏览不该覆盖主区显示")
	}
}

// TestPlayVideoWarningText 钉住「没有视频时点视频播放键」的提示 —— 原版
// videoInfo 的 handlePlayVideoClick 是 warning「请先选择一个视频」（原生版
// 此前写的是「先选一个视频」）。这条路径要真实的播放窗口才能构造出 App 状态，
// 无头测试跑不动，所以退一步做源码反证。
func TestPlayVideoWarningText(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}
	if !bytes.Contains(src, []byte(`NotifyType("warning", "请先选择一个视频")`)) {
		t.Error("openVideo 的提示应为「请先选择一个视频」（原版 videoInfo 的文案）")
	}
}

// mkComments 造 n 条可辨认的评论（User 是「p<页>-<序号>」）。
func mkComments(tag string, n int) []view.Comment {
	out := make([]view.Comment, n)
	for i := range out {
		out[i] = view.Comment{
			User:    fmt.Sprintf("%s-%d", tag, i),
			Content: fmt.Sprintf("%s 的第 %d 条评论", tag, i),
			Level:   i%6 + 1,
		}
	}
	return out
}

// TestRetainCommentsSlidingWindow 钉住评论分页的有界滑动窗口方向。
//
// 原版 danmakuList 的评论 tab 是「滚到底 + handleScroll 自动翻页」，页序即
// 列表序（第 1 页在最前），翻页只会往尾部追加更靠后的评论。原生版有内存上限
// （maxComments；列表体走虚拟化 ui.List，上限只在内存驻留），超上限时**必须
// 从头部释放**已经读过的那批，否则新页进不来：改前是 `a.Comments[:maxComments]`
// 截尾，超过上限后的第一页起整页被丢，列表「加载不动」且因为一直停在底部
// 而反复重拉。
func TestRetainCommentsSlidingWindow(t *testing.T) {
	const pageSize = 20
	// 触发释放的第一页：merged 首次超过上限，从头部丢掉一整页。
	dropPage := maxComments/pageSize + 1
	page := 0
	var got []view.Comment
	for page = 1; page <= dropPage; page++ {
		got = retainComments(got, mkComments(fmt.Sprintf("p%d", page), pageSize), page, maxComments)
		if len(got) > maxComments {
			t.Fatalf("第 %d 页后长度 %d 超过上限 %d", page, len(got), maxComments)
		}
		if page < dropPage && len(got) != page*pageSize {
			t.Fatalf("第 %d 页后长度 %d，want %d（还没到上限不该释放）", page, len(got), page*pageSize)
		}
	}
	if len(got) != maxComments {
		t.Fatalf("灌满后长度 %d，want %d", len(got), maxComments)
	}
	// 最新拉到的那一页必须整页都在（改前的截尾会把它们全丢掉）。
	for i := 0; i < pageSize; i++ {
		want := fmt.Sprintf("p%d-%d", dropPage, i)
		if got[len(got)-pageSize+i].User != want {
			t.Errorf("尾部第 %d 条 = %q，want %q —— 新页被截掉了（截尾而非丢头）",
				i, got[len(got)-pageSize+i].User, want)
		}
	}
	// 头部释放的是最早的整页：第 1 页全没了，第 2 页还在。
	if got[0].User != "p2-0" {
		t.Errorf("头部 = %q，want %q —— 应从头部按页释放", got[0].User, "p2-0")
	}
	for _, cm := range got {
		if strings.HasPrefix(cm.User, "p1-") {
			t.Fatalf("第 1 页的 %q 应该已被释放", cm.User)
		}
	}
}

// TestRetainCommentsFreesDroppedHead 释放头部之后不能还引用着那批评论：
// 直接对 `append` 出来的切片切尾会共享底层数组（cap 停在释放前的长度），
// Comment 里是 string 与 []Comment（都是指针），被丢掉的条目仍被那块数组
// 钉住 = 上限形同虚设。
func TestRetainCommentsFreesDroppedHead(t *testing.T) {
	prev := mkComments("p1", maxComments)
	got := retainComments(prev, mkComments("p2", 5), 2, maxComments)
	if len(got) != maxComments {
		t.Fatalf("长度 %d，want %d", len(got), maxComments)
	}
	// 丢的正好是最早的 5 条。
	if got[0].User != "p1-5" {
		t.Fatalf("头部 = %q，want %q", got[0].User, "p1-5")
	}
	// 共享底层数组时 cap 会是释放前的 125，于是 got[:cap(got)] 里能看到被丢的 p1-0。
	if cap(got) != len(got) {
		t.Fatalf("cap = %d，len = %d —— 返回的切片共享了释放前的底层数组", cap(got), len(got))
	}
	for _, cm := range got[:cap(got)] {
		if strings.HasPrefix(cm.User, "p1-0") || cm.User == "p1-4" {
			t.Fatalf("返回的切片在容量内仍引用被释放的 %q", cm.User)
		}
	}
}

// TestRetainCommentsFirstPageReplaces 重新拉第一页（换视频 / 刷新）整表替换，
// 不把新视频的评论追加到旧视频后面。
func TestRetainCommentsFirstPageReplaces(t *testing.T) {
	old := mkComments("old", 30)
	got := retainComments(old, mkComments("new", 20), 1, maxComments)
	if len(got) != 20 {
		t.Fatalf("长度 %d，want 20", len(got))
	}
	for _, cm := range got {
		if !strings.HasPrefix(cm.User, "new-") {
			t.Fatalf("第一页应整表替换，出现 %q", cm.User)
		}
	}
	// 空页 + 第一页：列表清空（不是保留旧的）。
	if got := retainComments(old, nil, 1, maxComments); got != nil {
		t.Errorf("第一页为空应得 nil，got %d 条", len(got))
	}
}

// TestAudioErrorMessage 钉住播放失败文案到原版 player.tsx onError 的映射。
//
// 原版是 HTML <audio> 的 error 事件按 MediaError.code 选文案：
//
//	4 SRC_NOT_SUPPORTED → 音频格式不支持或加载失败
//	3 DECODE            → 音频解码失败
//	2 NETWORK           → 网络错误，音频加载失败
//	其它                → 音频加载出错
//
// 原生版没有这套码，等价分类由 internal/media 的错误种类给出。
func TestAudioErrorMessage(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{"网络", media.WrapKind(media.KindNetwork, errors.New("media: 上游返回 403")), "网络错误，音频加载失败"},
		{"解码", media.WrapKind(media.KindDecode, errors.New("media: 打开 AAC 解码器失败")), "音频解码失败"},
		{"格式", media.WrapKind(media.KindUnsupported, errors.New("media: 解析 MP4 失败")), "音频格式不支持或加载失败"},
		{"输出设备", media.WrapKind(media.KindOutput, errors.New("media: 打开音频输出失败")), "音频加载出错"},
		{"没归类", errors.New("boom"), "音频加载出错"},
		{"nil", nil, "音频加载出错"},
		// 根因优先：解码器建不起来，但真因是一次网络失败。
		{"根因网络", media.WrapKind(media.KindDecode,
			fmt.Errorf("media: 打开 AAC 解码器失败: %w",
				media.WrapKind(media.KindNetwork, errors.New("media: 取块失败，上游返回 500")))),
			"网络错误，音频加载失败"},
	}
	for _, c := range cases {
		if got := audioErrorMessage(c.err); got != c.want {
			t.Errorf("%s: audioErrorMessage = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestAudioErrorReportingWired 是上一条 + internal/media 类别测试的接线反证。
//
// 纯函数只能证明「类别对了、文案对了」；这两条路径都要真的起一个音频设备才会
// 走到，无头测试跑不动，所以退一步做源码反证 —— 之前原生版这里运行中出错是
// **完全静默**的：只 log.Printf，用户看到的就是「放着放着停了」，没有任何提示。
func TestAudioErrorReportingWired(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("读 main.go 失败：%v", err)
	}

	// ① 运行中出错（wirePlayer 的 OnError 回调）：块内必须有分类文案的 toast。
	onErr := sourceBlock(t, src, "c.mp.OnError(func(err error) {")
	if !strings.Contains(onErr, `NotifyType("error", audioErrorMessage(err))`) {
		t.Errorf("OnError 块内没有 audioErrorMessage 的 toast（运行中出错会是静默的）：\n%s", onErr)
	}

	// ② 起播失败：同样走分类文案，而不是把 Go 的错误串直接贴给用户。
	play := sourceBlock(t, src, "if err := c.mp.Play(u.URL, audioHeaders); err != nil {")
	if !strings.Contains(play, `NotifyType("error", audioErrorMessage(err))`) {
		t.Errorf("起播失败没有走 audioErrorMessage：\n%s", play)
	}
	if strings.Contains(play, "err.Error()") {
		t.Error("起播失败仍在直接贴 Go 的错误串（原版是按类别给文案）")
	}
	// ③「起播被更新的起播取代」不是失败，必须整条忽略（既不弹提示，也不动
	// 界面状态，否则会把新那次正在进行的起播标成失败）。
	if !strings.Contains(play, "errors.Is(err, media.ErrSuperseded)") {
		t.Error("起播失败没有区分 ErrSuperseded（快速切歌会误报「起播失败」）")
	}
	if i, j := strings.Index(play, "errors.Is(err, media.ErrSuperseded)"),
		strings.Index(play, "audioErrorMessage(err)"); i >= 0 && j >= 0 && i > j {
		t.Error("ErrSuperseded 的判定必须在弹提示之前")
	}
}

// sourceBlock 从 src 里 needles 命中处开始，抓出到配对大括号为止的那一段源码，
// 供「某回调块里有没有调 X」这种接线反证用。
func sourceBlock(t *testing.T, src []byte, needle string) string {
	t.Helper()
	i := bytes.Index(src, []byte(needle))
	if i < 0 {
		t.Fatalf("main.go 里找不到 %q", needle)
	}
	s := src[i:]
	depth := 0
	for j, c := range s {
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return string(s[:j+1])
			}
		}
	}
	t.Fatalf("%q 之后的大括号没有配平", needle)
	return ""
}

// TestMuteToggleState 钉住原版静音键的两条分支：静音 = 把音量写成 0 并记住
// 当前音量；解除 = 恢复记住的音量（没有记录就回到 1，对齐 `lastVolumeRef || 1`）。
//
// 原版（main:src/components/player.tsx 的静音键 + handleVolumeChange）根本没有
// 独立的静音布尔：`audio.muted = value === 0`，静音就是把音量归零，所以音量
// 滑杆会一起走到最左；`lastVolumeRef` 只在 value > 0 时更新。此前原生版用独立
// 的 Muted 布尔 —— 静音时音量/滑杆停在原值。
func TestMuteToggleState(t *testing.T) {
	for _, c := range []struct {
		volume, last      float64
		wantVol, wantLast float64
	}{
		{0.8, 0.8, 0, 0.8}, // 有声音 → 归零并记住
		{0.5, 0.9, 0, 0.5}, // 记住的是「当前」音量，不是更早那个
		{0, 0.8, 0.8, 0.8}, // 已静音 → 恢复上次音量
		{0, 0, 1, 1},       // 从没听过声音 → 回到 1
		{0, -1, 1, 1},      // 非法记录也当没有
	} {
		vol, last := muteToggleState(c.volume, c.last)
		if vol != c.wantVol || last != c.wantLast {
			t.Errorf("muteToggleState(%v,%v) = (%v,%v)，想 (%v,%v)",
				c.volume, c.last, vol, last, c.wantVol, c.wantLast)
		}
	}
}

// TestToggleMuteMatchesOriginal 是控制器级实测：点一次静音键后
// Volume==0 且 Muted==true（滑杆因此归零），并**落盘 0**（原版 persistVolume(0)，
// 重启后仍是静音）；再点一次恢复音量、解除静音、落盘恢复值。
func TestToggleMuteMatchesOriginal(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a := &view.App{Volume: 0.8, LastVolume: 0.8}
	c := &controller{app: a, kv: kv}

	c.toggleMute()
	if a.Volume != 0 || !a.Muted {
		t.Errorf("静音后应 Volume=0 / Muted=true，得到 Volume=%v Muted=%v", a.Volume, a.Muted)
	}
	if a.LastVolume != 0.8 {
		t.Errorf("静音后应记住 0.8，得到 %v", a.LastVolume)
	}
	if got := kv.String("volume"); got != "0" {
		t.Errorf("静音应落盘 volume=0，得到 %q", got)
	}

	c.toggleMute()
	if a.Volume != 0.8 || a.Muted {
		t.Errorf("解除静音后应恢复 0.8 / Muted=false，得到 Volume=%v Muted=%v", a.Volume, a.Muted)
	}
	if got := kv.String("volume"); got != "0.8" {
		t.Errorf("解除静音应落盘 volume=0.8，得到 %q", got)
	}
}

// TestSetVolumeZeroMutes 钉住「滑杆拖到 0 也等于静音」（原版 handleVolumeChange
// 的 `audio.muted = value === 0`）：静音键的蓝调激活态与主窗音量图标都读
// Muted，拖到 0 而不置 Muted 会让它们继续显示「有声」。
func TestSetVolumeZeroMutes(t *testing.T) {
	kv, err := store.Open(t.TempDir(), false)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	a := &view.App{Volume: 0.6, LastVolume: 0.6}
	c := &controller{app: a, kv: kv}

	c.setVolume(0)
	if a.Volume != 0 || !a.Muted {
		t.Errorf("音量拖到 0 应视为静音，得到 Volume=%v Muted=%v", a.Volume, a.Muted)
	}
	if a.LastVolume != 0.6 {
		t.Errorf("拖到 0 不该覆盖记住的音量，得到 %v", a.LastVolume)
	}

	c.setVolume(0.3)
	if a.Volume != 0.3 || a.Muted {
		t.Errorf("从 0 拖回来应解除静音，得到 Volume=%v Muted=%v", a.Volume, a.Muted)
	}
	if a.LastVolume != 0.3 {
		t.Errorf(">0 的音量应被记住，得到 %v", a.LastVolume)
	}
}
