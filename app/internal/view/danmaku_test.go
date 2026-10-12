package view

import (
	"fmt"
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
