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

// TestKeepDrawerOnPlay 钉住「切换播放项不强制关抽屉」：原版从选集面板点一集
// （handleVideoSelect）不关面板，播放列表模式下手动/自动切歌也不关
// （handlePlaylistVideoSelect 的 preserveUi）；其余抽屉起播时关掉。
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
// （maxComments，列表不做虚拟化），超上限时**必须从头部释放**已经读过的那批，
// 否则新页进不来：改前是 `a.Comments[:maxComments]` 截尾，第 7 页（120 条上限
// 之下的第一页）起整页被丢，列表「加载不动」且因为一直停在底部而反复重拉。
func TestRetainCommentsSlidingWindow(t *testing.T) {
	const pageSize = 20
	page := 0
	var got []view.Comment
	// 7 页 × 20 条，上限 120：第 7 页是第一个会触发释放的页。
	for page = 1; page <= 7; page++ {
		got = retainComments(got, mkComments(fmt.Sprintf("p%d", page), pageSize), page, maxComments)
		if len(got) > maxComments {
			t.Fatalf("第 %d 页后长度 %d 超过上限 %d", page, len(got), maxComments)
		}
		if page <= 6 && len(got) != page*pageSize {
			t.Fatalf("第 %d 页后长度 %d，want %d（还没到上限不该释放）", page, len(got), page*pageSize)
		}
	}
	if len(got) != maxComments {
		t.Fatalf("灌满后长度 %d，want %d", len(got), maxComments)
	}
	// 最新拉到的那一页必须整页都在（改前的截尾会把它们全丢掉）。
	for i := 0; i < pageSize; i++ {
		want := fmt.Sprintf("p7-%d", i)
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
