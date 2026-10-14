package view

import (
	"fmt"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestGroupDanmaku(t *testing.T) {
	list := []Danmaku{
		{Time: 3.2, Text: "哈哈"},
		{Time: 3.8, Text: "哈哈"},   // 同一秒、同文本 → 计数 2
		{Time: 3.5, Text: " 哈哈 "}, // 归一化后同上 → 计数 3
		{Time: 3.9, Text: "厉害"},   // 同一秒、不同文本
		{Time: 12.1, Text: "前方高能"},
	}
	groups := groupDanmaku(list)
	if len(groups) != 2 {
		t.Fatalf("组数 = %d, want 2", len(groups))
	}
	if groups[0].Second != 3 || groups[1].Second != 12 {
		t.Fatalf("组的秒序 = %d,%d, want 3,12", groups[0].Second, groups[1].Second)
	}
	g := groups[0]
	if g.TotalCount != 4 {
		t.Errorf("第一组总条数 = %d, want 4", g.TotalCount)
	}
	if len(g.Entries) != 2 {
		t.Fatalf("第一组去重后 = %d 条, want 2", len(g.Entries))
	}
	// 「哈哈」出现 3 次，「厉害」1 次 → 排在前。
	if g.Entries[0].Text != "哈哈" || g.Entries[0].Count != 3 {
		t.Errorf("组内排序/计数 = %+v", g.Entries)
	}
	if g.Entries[1].Text != "厉害" || g.Entries[1].Count != 1 {
		t.Errorf("组内第二条 = %+v", g.Entries[1])
	}
}

func TestGroupDanmakuEmpty(t *testing.T) {
	if groups := groupDanmaku(nil); len(groups) != 0 {
		t.Errorf("空输入应得 0 组，得 %d", len(groups))
	}
}

func TestDanmakuInk(t *testing.T) {
	// 默认色（0 / 未提供）用正文色。
	if got := danmakuInk(0); got != ui.Hex("#1e293b") {
		t.Errorf("danmakuInk(0) = %v", got)
	}
	// 白色太亮 → 深灰对比色。
	if got := danmakuInk(0xffffff); got != ui.Hex("#1a1a1a") {
		t.Errorf("danmakuInk(白) = %v", got)
	}
	// 中等亮度（红 #ff0000 → 亮度 76）应保留原色。
	if got := danmakuInk(0xff0000); got != ui.RGB(255, 0, 0) {
		t.Errorf("danmakuInk(红) = %v", got)
	}
}

func TestNearestGroup(t *testing.T) {
	groups := []danmakuGroup{{Second: 0}, {Second: 4}, {Second: 10}}
	// 3.9 更近 4 而不是 0（原版 currentIndex 取最近的一组）。
	if got := nearestGroup(groups, 3.9); got != 1 {
		t.Errorf("nearestGroup(3.9) = %d, want 1", got)
	}
	// 2.0 与 0 / 4 等距，取前一组。
	if got := nearestGroup(groups, 2.0); got != 0 {
		t.Errorf("nearestGroup(2.0) = %d, want 0", got)
	}
	// 在全部组之前 / 之后各钳到首尾。
	if got := nearestGroup(groups, -5); got != 0 {
		t.Errorf("nearestGroup(-5) = %d, want 0", got)
	}
	if got := nearestGroup(groups, 99); got != 2 {
		t.Errorf("nearestGroup(99) = %d, want 2", got)
	}
	if got := nearestGroup(nil, 3); got != -1 {
		t.Errorf("nearestGroup(空) = %d, want -1", got)
	}
}

func TestFmtClock(t *testing.T) {
	cases := map[float64]string{0: "00:00", 5: "00:05", 75: "01:15", 3725: "62:05", -3: "00:00"}
	for in, want := range cases {
		if got := fmtClock(in); got != want {
			t.Errorf("fmtClock(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestDanmakuGroupsCache 钉住弹幕分组只在数据变了才重算：滚动时视图每帧
// 重建，每帧重跑 groupDanmaku（建 map + 排序）就是卡顿来源。
func TestDanmakuGroupsCache(t *testing.T) {
	a := &App{Danmaku: []Danmaku{
		{Time: 1, Text: "a"},
		{Time: 1, Text: "b"},
		{Time: 2, Text: "c"},
	}}
	g1 := a.danmakuGroups()
	if len(g1) != 2 {
		t.Fatalf("组数 = %d, want 2", len(g1))
	}
	g2 := a.danmakuGroups()
	if &g1[0] != &g2[0] {
		t.Fatal("同一弹幕再次读取应复用缓存，而不是重算")
	}
	// 换一份弹幕（新分配）→ 重算。
	a.Danmaku = []Danmaku{{Time: 5, Text: "x"}}
	g3 := a.danmakuGroups()
	if len(g3) != 1 || g3[0].Second != 5 {
		t.Fatalf("换弹幕后 = %+v, want 第二组为 5", g3)
	}
	// 清空 → 空分组（且不 panic）。
	a.Danmaku = nil
	if got := a.danmakuGroups(); len(got) != 0 {
		t.Fatalf("清空后应得 0 组，得 %d", len(got))
	}
}

// TestSortedCommentsCache 同理：评论排序带拷贝，缓存按「底层数组 + 长度」失效。
func TestSortedCommentsCache(t *testing.T) {
	a := &App{Comments: []Comment{
		{User: "a", SendTime: 1},
		{User: "b", SendTime: 3},
		{User: "c", SendTime: 2},
	}}
	s1 := a.sortedComments()
	if s1[0].User != "b" || s1[1].User != "c" || s1[2].User != "a" {
		t.Fatalf("倒序 = %s,%s,%s, want b,c,a", s1[0].User, s1[1].User, s1[2].User)
	}
	s2 := a.sortedComments()
	if &s1[0] != &s2[0] {
		t.Fatal("同一评论再次读取应复用缓存")
	}
	// append 新评论（长度变）→ 重算。
	a.Comments = append(a.Comments, Comment{User: "d", SendTime: 4})
	if s3 := a.sortedComments(); s3[0].User != "d" {
		t.Fatalf("append 后首条 = %s, want d", s3[0].User)
	}
}

// TestDanmakuListVirtualized 钉住弹幕列表走 mygo 的虚拟化 ui.List：几百个分组
// 时也只构建可见的那十几行，屏外分组不建（改回 ui.Column 会让整表每帧重建，
// 这个断言会失败）。
func TestDanmakuListVirtualized(t *testing.T) {
	a := NewApp(func() {})
	a.Pos = 0 // 不自动跟随，停在列表顶部
	// 400 个分组，每组一条互不相同的弹幕。
	dms := make([]Danmaku, 400)
	for i := range dms {
		dms[i] = Danmaku{Time: float64(i) * 1.5, Text: fmt.Sprintf("弹幕编号%d", i)}
	}
	a.Danmaku = dms
	a.Drawer = DrawerDanmaku

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()

	if !tst.HasText("弹幕编号0") {
		t.Fatal("首屏应构建第一组弹幕")
	}
	if tst.HasText("弹幕编号399") {
		t.Fatal("最后一组在屏外，虚拟化列表不该构建它（说明又退回每帧全建）")
	}
}

// TestDanmakuChipPaint 钉住弹幕小胶囊三种状态的配色（原版 .danmaku-message-row）。
// 重点是悬停比基础**更实**、当前组是极淡的蓝白 —— 之前这两处都写反了。
func TestDanmakuChipPaint(t *testing.T) {
	base := danmakuChipPaint(false, false)
	hover := danmakuChipPaint(false, true)
	cur := danmakuChipPaint(true, false)

	if base.BG != ui.Hex("#ffffff").Alpha(0.48) {
		t.Errorf("基础底色 = %v, want 白 .48", base.BG)
	}
	if hover.BG != ui.Hex("#ffffff").Alpha(0.58) {
		t.Errorf("悬停底色 = %v, want 白 .58", hover.BG)
	}
	if hover.BG.A <= base.BG.A {
		t.Errorf("悬停应比基础更实：base.A=%d hover.A=%d", base.BG.A, hover.BG.A)
	}
	if hover.Border.A <= base.Border.A {
		t.Errorf("悬停描边应比基础更亮：base=%d hover=%d", base.Border.A, hover.Border.A)
	}
	if cur.BG != ui.Hex("#f0f9ff").Alpha(0.76) {
		t.Errorf("当前组底色 = %v, want #f0f9ff .76", cur.BG)
	}
	if cur.Border != ui.Hex("#0ea5e9").Alpha(0.48) {
		t.Errorf("当前组描边 = %v, want #0ea5e9 .48", cur.Border)
	}
	// 当前组优先于悬停（原版 .is-current 规则排在 :hover 之后）。
	if got := danmakuChipPaint(true, true); got != cur {
		t.Errorf("当前组的胶囊不应被悬停覆盖：%+v", got)
	}
}

// TestReplyMetaText 钉住评论用户名下的「Lv.N · 时间」恒带 Lv（原版即使是 0 级也显示）。
func TestReplyMetaText(t *testing.T) {
	if got := replyMetaText(0, "2026-10-01 12:00"); got != "Lv.0 · 2026-10-01 12:00" {
		t.Errorf("0 级也要显示 Lv：%q", got)
	}
	if got := replyMetaText(6, "刚刚"); got != "Lv.6 · 刚刚" {
		t.Errorf("replyMetaText(6) = %q", got)
	}
}

// TestReplyInkMatchesSlateOverride 锁定评论列表的配色（原版 globals.css 的
// [data-slot="wrapper"] 覆写）。时间 / 点赞 / 查看更多都带 text-xs，走更特异的
// .text-xs.text-default-400 → #64748b，而不是基础 .text-default-400 的 #475569。
func TestReplyInkMatchesSlateOverride(t *testing.T) {
	cases := []struct{ name, got, want string }{
		{"昵称", replyInkName, "#0f172a"},
		{"时间/点赞/查看更多", replyInkMeta, "#64748b"},
		{"正文", replyInkBody, "#142033"},
		{"楼中楼昵称", replyInkPrim, "#0284c7"},
		{"楼中楼正文", replyInkSub, "#1e293b"},
		{"空态/加载中", replyInkMuted, "#94a3b8"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if replyInkMeta == "#475569" {
		t.Error("时间/点赞应走 .text-xs.text-default-400 的 #64748b，不是基础的 #475569")
	}
}

// TestDanmakuCurrentIndexIndependentOfAutoScroll 钉住当前组高亮与「自动跟随」
// 开关解耦：原版 is-current 由 currentIndex 决定，开关（isAutoScroll）只控制
// 滚动。此前 danmakuBody 写成 `if a.DanmakuAutoScroll && a.Pos > 0`，一关跟随
// 当前组就不再变蓝，也丢了停在 00:00 时高亮首组的行为。
func TestDanmakuCurrentIndexIndependentOfAutoScroll(t *testing.T) {
	a := NewApp(func() {})
	a.Danmaku = []Danmaku{{Time: 0, Text: "a"}, {Time: 30, Text: "b"}, {Time: 60, Text: "c"}}
	groups := a.danmakuGroups()
	a.Pos = 31

	a.DanmakuAutoScroll = true
	if got := a.danmakuCurrentIndex(groups); got != 1 {
		t.Errorf("自动跟随时当前组 = %d, want 1", got)
	}
	a.DanmakuAutoScroll = false
	if got := a.danmakuCurrentIndex(groups); got != 1 {
		t.Errorf("关闭跟随后当前组 = %d, want 1（高亮不该随开关消失）", got)
	}

	// 原版 currentTime 初始 0 → currentIndex 取首组，而不是 -1。
	a.Pos = 0
	if got := a.danmakuCurrentIndex(groups); got != 0 {
		t.Errorf("Pos=0 当前组 = %d, want 0", got)
	}
	// 负时间（未就绪）不标当前组。
	a.Pos = -1
	if got := a.danmakuCurrentIndex(groups); got != -1 {
		t.Errorf("Pos<0 当前组 = %d, want -1", got)
	}
	if got := a.danmakuCurrentIndex(nil); got != -1 {
		t.Errorf("空分组当前组 = %d, want -1", got)
	}
}

// blueLabelPixels 数某个矩形附近「#0284c7 实心蓝底」的像素（当前组的时间胶囊，
// 原版 .danmaku-time-group.is-current .danmaku-time-label 的 background）。
func blueLabelPixels(img *image.RGBA, r ui.Rect) int {
	n := 0
	x0, y0 := int(r.X)-6, int(r.Y)-3
	x1, y1 := int(r.X+r.W)+6, int(r.Y+r.H)+3
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			rr, gg, bb := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			// #0284c7 = (2,132,199)：蓝压倒红，且比绿更蓝。
			if bb > 150 && gg > 90 && gg < bb && rr < 80 {
				n++
			}
		}
	}
	return n
}

// TestDanmakuCurrentHighlightWired 无头反证：关闭自动跟随后，当前组的时间胶囊
// 仍是实心蓝底（原版 is-current 与开关无关）。把 danmakuCurrentIndex 改回
// `if a.DanmakuAutoScroll` 那条，这条即变红。
func TestDanmakuCurrentHighlightWired(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{
		{Time: 0, Text: "开头"},
		{Time: 30, Text: "中间"},
		{Time: 60, Text: "结尾"},
	}
	a.Drawer = DrawerDanmaku
	a.DanmakuAutoScroll = false // 关键：关掉跟随
	a.Pos = 30

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()

	r, ok := tst.Find("00:30")
	if !ok {
		t.Fatal("渲染帧里找不到当前组的时间标签 00:30")
	}
	if n := blueLabelPixels(tst.Image(), r); n == 0 {
		t.Error("关闭自动跟随后当前组时间胶囊没有蓝底，is-current 高亮被开关吃掉了")
	}
	// 非当前组保持灰字（无蓝底）。
	if r0, ok := tst.Find("00:00"); ok {
		if n := blueLabelPixels(tst.Image(), r0); n != 0 {
			t.Errorf("非当前组 00:00 不应有蓝底，数到 %d 个蓝像素", n)
		}
	}
}

// TestDanmakuAutoScrollResetOnClose 钉住关闭弹幕抽屉会把自动跟随重置为开
// （原版 danmakuList 的 handleOpenChange(false)：setIsAutoScroll(true) +
// lastScrollIndexRef.current = -1）。关掉跟随、关抽屉再打开，应重新跟随当前
// 播放位置。
func TestDanmakuAutoScrollResetOnClose(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{{Time: 0, Text: "a"}, {Time: 30, Text: "b"}}
	a.Drawer = DrawerDanmaku
	a.DanmakuAutoScroll = false
	a.Pos = 30

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame() // 打开一帧，记下 drawerShown = danmaku
	if a.DanmakuAutoScroll {
		t.Fatal("打开期间不应被重置")
	}

	a.Drawer = "" // 关抽屉
	tst.Frame()
	if !a.DanmakuAutoScroll {
		t.Error("关闭弹幕抽屉后应重置自动跟随为开（原版 handleOpenChange(false)）")
	}

	// 非弹幕抽屉关闭不受影响。
	a.Drawer = DrawerPlaylist
	a.DanmakuAutoScroll = false
	tst.Frame()
	a.Drawer = ""
	tst.Frame()
	if a.DanmakuAutoScroll {
		t.Error("关闭其他抽屉不应动弹幕自动跟随开关")
	}
}

// TestRepliesListVirtualized 钉住评论列表走 mygo 的虚拟化 ui.List：上百条评论
// 时也只构建可见的十几行，屏外评论不建（改回 ui.Column 会让整表每帧重建，
// 这个断言会失败）。与 TestDanmakuListVirtualized 同一机制。
func TestRepliesListVirtualized(t *testing.T) {
	a := NewApp(func() {})
	cms := make([]Comment, 200)
	for i := range cms {
		cms[i] = Comment{
			User:     fmt.Sprintf("用户%d", i),
			Content:  fmt.Sprintf("评论编号%d", i),
			SendTime: int64(1700000000 - i*60),
		}
	}
	a.Comments = cms
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabReply

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()

	if !tst.HasText("评论编号0") {
		t.Fatal("首屏应构建第一条评论")
	}
	if tst.HasText("评论编号199") {
		t.Fatal("最后一条评论在屏外，虚拟化列表不该构建它（说明又退回每帧全建）")
	}
}

// TestRepliesListAnchorOnWindowSlide 钉住「滑动窗口从头部释放已读评论时视口
// 不跳」：ui.List 的行键（commentKey）让锚点行跟着挪下标。行键若丢了（Key=nil），
// 丢头后同样的滚动偏移会对着新下标，首可见评论跳到更后面 —— 这个断言防的
// 就是那个回归。
func TestRepliesListAnchorOnWindowSlide(t *testing.T) {
	a := NewApp(func() {})
	cms := make([]Comment, 200)
	for i := range cms {
		cms[i] = Comment{
			User:     "用户",
			Content:  fmt.Sprintf("评论编号%03d", i),
			SendTime: int64(1700000000 - i*60),
		}
	}
	// 翻页追加的是更老的一页（SendTime 更小），不是重复头部那批。
	newer := make([]Comment, 20)
	for i := range newer {
		newer[i] = Comment{
			User:     "用户",
			Content:  fmt.Sprintf("追评%03d", i),
			SendTime: int64(1600000000 - i*60),
		}
	}
	a.Comments = cms
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabReply

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()
	tst.Scroll(490, 400, 0, 4000)
	tst.Frame()

	first, _ := a.repliesList.Visible()
	if first <= 0 {
		t.Fatalf("滚动后首行应不在顶部：first=%d", first)
	}
	anchor := a.sortedComments()[first].Content

	// 模拟 retainComments 的滑动窗口：丢掉头部 40 条已读评论，尾部追加 20 条。
	a.Comments = append(append([]Comment(nil), a.Comments[40:]...), newer...)
	tst.Frame()

	f2, _ := a.repliesList.Visible()
	if got := a.sortedComments()[f2].Content; got != anchor {
		t.Fatalf("丢头后首可见评论 = %s, want %s（视口跳了）", got, anchor)
	}
}

// TestCanLoadMoreComments 钉住评论自动翻页的判定：还有更多、不在加载中，且已经
// 滚到底（首行不在顶部、末行已进视野）。`first > 0` 避免首屏都还没滚动就每帧
// 重发下一页。
func TestCanLoadMoreComments(t *testing.T) {
	cases := []struct {
		name              string
		hasMore, loading  bool
		first, last, rows int
		want              bool
	}{
		{"首屏还没滚", true, false, 0, 9, 20, false},
		{"首屏不满也不发", true, false, 0, 4, 5, false},
		{"滚到底且有更多", true, false, 3, 19, 20, true},
		{"加载中", true, true, 3, 19, 20, false},
		{"没有更多", false, false, 3, 19, 20, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := canLoadMoreComments(c.hasMore, c.loading, c.first, c.last, c.rows); got != c.want {
				t.Fatalf("canLoadMoreComments = %v, want %v", got, c.want)
			}
		})
	}
}

// TestRepliesListKeyIdentity 钉住评论虚拟列表设了行键，且键取自评论身份
// （用户 + 发送时间 + 正文）——丢头不跳位依赖它。
func TestRepliesListKeyIdentity(t *testing.T) {
	a := NewApp(func() {})
	a.Comments = []Comment{
		{User: "甲", Content: "第一条", SendTime: 100},
		{User: "乙", Content: "第二条", SendTime: 50},
	}
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabReply

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()

	if a.repliesList.Key == nil {
		t.Fatal("评论虚拟列表应设置行键（丢头不跳位依赖它）")
	}
	if got, want := a.repliesList.Key(1), (commentKey{user: "乙", sendTime: 50, content: "第二条"}); got != want {
		t.Fatalf("第 2 行的键 = %#v, want %#v", got, want)
	}
}
