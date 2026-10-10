package main

import (
	"fmt"
	"testing"

	"github.com/vst93/bili-fm/app/internal/bilibili"
	"github.com/vst93/bili-fm/app/internal/view"
)

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
							"stat": map[string]any{"play": "69.2万"},
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
	if c.Track.Aid != 7 {
		t.Errorf("track aid = %d", c.Track.Aid)
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
		// 超过一年的日期 relTime 会回落成绝对日期，所以只要求非空。
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
