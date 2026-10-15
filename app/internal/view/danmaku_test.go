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
	// color=0 → 原版 (0).toString(16).padStart(6,"0") = "#000000"。
	if got := danmakuInk(0, false); got != ui.Hex("#000000") {
		t.Errorf("danmakuInk(0) = %v, want #000000", got)
	}
	if got := danmakuInk(-1, false); got != ui.Hex("#000000") {
		t.Errorf("danmakuInk(-1) = %v, want #000000", got)
	}
	// 白色太亮 → 浅色模式深灰对比色。
	if got := danmakuInk(0xffffff, false); got != ui.Hex("#1a1a1a") {
		t.Errorf("danmakuInk(白, false) = %v", got)
	}
	// 深色模式：亮色弹幕直接用白字。
	if got := danmakuInk(0xffffff, true); got != ui.Hex("#ffffff") {
		t.Errorf("danmakuInk(白, true) = %v, want #ffffff", got)
	}
	// 亮度 160.1（#a0a0a1）在 160–170 之间 → 浅色模式 #333333。
	if got := danmakuInk(0xa0a0a1, false); got != ui.Hex("#333333") {
		t.Errorf("danmakuInk(#a0a0a1, false) = %v, want #333333", got)
	}
	// 亮度 192（#c0c0c0）在 180–200 → 深色模式 #e0e0e0。
	if got := danmakuInk(0xc0c0c0, true); got != ui.Hex("#e0e0e0") {
		t.Errorf("danmakuInk(#c0c0c0, true) = %v, want #e0e0e0", got)
	}
	// 亮度 175（#b0b0b0 → 176*0.854 ≈ 175）在 160–200 但 ≥170 → 保留原色。
	if got := danmakuInk(0xb0b0b0, false); got != ui.RGB(0xb0, 0xb0, 0xb0) {
		t.Errorf("danmakuInk(#b0b0b0) = %v, want 原色", got)
	}
	// 中等亮度（红 #ff0000 → 亮度 76）应保留原色。
	if got := danmakuInk(0xff0000, false); got != ui.RGB(255, 0, 0) {
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
	base := danmakuChipPaint(false, false, false)
	hover := danmakuChipPaint(false, true, false)
	cur := danmakuChipPaint(true, false, false)

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
	if got := danmakuChipPaint(true, true, false); got != cur {
		t.Errorf("当前组的胶囊不应被悬停覆盖：%+v", got)
	}
}

// TestDanmakuChipPaintLinux 钉住原版 `body.platform-linux .danmaku-message-row`
// 那条平台覆写：基础态底换白 .9、边换 slate-400 .28、box-shadow none；悬停态
// 的底/边被更特异的 `.danmaku-time-group:hover .danmaku-message-row` 盖回白
// .58 / 白 .68，但 box-shadow 仍归 Linux 的 none；当前组特异度同为 (0,3,0)，
// Linux 下与其它平台完全一样。
func TestDanmakuChipPaintLinux(t *testing.T) {
	base := danmakuChipPaint(false, false, true)
	if base.BG != ui.Hex("#ffffff").Alpha(0.90) {
		t.Errorf("Linux 基础底色 = %v, want 白 .9", base.BG)
	}
	if base.Border != ui.Hex("#94a3b8").Alpha(0.28) {
		t.Errorf("Linux 基础描边 = %v, want slate-400 .28", base.Border)
	}
	if base.Inset.A != 0 || base.Shadow.A != 0 || base.ShadowBlur != 0 {
		t.Errorf("Linux 基础态应 box-shadow:none（无内高光/投影）：%+v", base)
	}

	hover := danmakuChipPaint(false, true, true)
	if hover.BG != ui.Hex("#ffffff").Alpha(0.58) || hover.Border != ui.Hex("#ffffff").Alpha(0.68) {
		t.Errorf("Linux 悬停态底/边应盖回白 .58 / 白 .68：%+v", hover)
	}
	if hover.Inset.A != 0 || hover.Shadow.A != 0 || hover.ShadowBlur != 0 {
		t.Errorf("Linux 悬停态仍应 box-shadow:none：%+v", hover)
	}

	// 当前组不受 Linux 覆写影响。
	if got, want := danmakuChipPaint(true, false, true), danmakuChipPaint(true, false, false); got != want {
		t.Errorf("Linux 当前组 = %+v, want 与非 Linux 一致 %+v", got, want)
	}
}

// TestDanmakuSepColorsLinux 钉住原版 `body.platform-linux .danmaku-time-group`
// 的分隔线覆写：基础 rgba(100,116,139,.18)、当前组 rgba(2,132,199,.34)；悬停的
// 蓝线被 Linux 规则按特异度盖掉（(0,2,1) > (0,2,0)），所以悬停 == 基础。
func TestDanmakuSepColorsLinux(t *testing.T) {
	base, hover, cur := danmakuSepColors(true)
	if base != ui.Hex("#64748b").Alpha(0.18) {
		t.Errorf("Linux 基础分隔线 = %v, want #64748b .18", base)
	}
	if hover != base {
		t.Errorf("Linux 悬停分隔线 = %v, want == 基础 %v（原版特异度盖掉）", hover, base)
	}
	if cur != ui.Hex("#0284c7").Alpha(0.34) {
		t.Errorf("Linux 当前组分隔线 = %v, want #0284c7 .34", cur)
	}

	// 非 Linux：基础 .14、悬停 sky-500 .24、当前组 studio-blue .30。
	nb, nh, nc := danmakuSepColors(false)
	if nb != ui.Hex("#64748b").Alpha(0.14) {
		t.Errorf("非 Linux 基础分隔线 = %v, want .14", nb)
	}
	if nh != ui.Hex("#0ea5e9").Alpha(0.24) {
		t.Errorf("非 Linux 悬停分隔线 = %v, want sky-500 .24", nh)
	}
	if nc != ui.Hex("#0284c7").Alpha(0.30) {
		t.Errorf("非 Linux 当前组分隔线 = %v, want .30", nc)
	}
}

// countChipBorderPixels 数弹幕胶囊左缘一条竖带上的「灰描边」像素（原版 Linux
// 覆写的 slate-400 .28 over 白底 ≈ (224,228,234)，R 明显低于白 253）。非 Linux
// 的胶囊描边是白 .52，在近白面板上约 253，数不到。
func countChipBorderPixels(img *image.RGBA, r ui.Rect) int {
	y := int(r.Y + r.H/2)
	n := 0
	for x := int(r.X) - 12; x < int(r.X); x++ {
		if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
			continue
		}
		i := img.PixOffset(x, y)
		if int(img.Pix[i]) < 242 { // 灰描边（≈224）vs 白 .52（≈253）
			n++
		}
	}
	return n
}

// TestDanmakuChipLinuxOpaqueWired 是无头渲染实测：把平台切成 Linux，弹幕胶囊
// 左缘应出现 slate-400 灰描边（原版 body.platform-linux .danmaku-message-row 的
// border-color）；切回非 Linux 时描边是白 .52、在近白面板上不可见。反证：
// danmakuGroupRow 不把 isLinux() 传给 danmakuChipPaint，Linux 帧就数不到灰边。
func TestDanmakuChipLinuxOpaqueWired(t *testing.T) {
	orig := viewGOOS
	defer func() { viewGOOS = orig }()

	render := func() int {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Danmaku = []Danmaku{{Time: 0, Text: "平台配色测试"}}
		a.Pos = -1 // 不标当前组：当前组胶囊不受 Linux 覆写影响
		a.Drawer = DrawerDanmaku
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
		tst.Frame()
		r, ok := tst.Find("平台配色测试")
		if !ok {
			t.Fatal("渲染帧里找不到弹幕文字")
		}
		return countChipBorderPixels(tst.Image(), r)
	}

	viewGOOS = "linux"
	if n := render(); n == 0 {
		t.Error("Linux 下胶囊左缘没有灰描边（platform-linux 覆写没接上）")
	}
	viewGOOS = "darwin"
	if n := render(); n != 0 {
		t.Errorf("非 Linux 下胶囊描边应是白 .52（不可见），却数到 %d 个灰像素", n)
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

// strictBlueBox 在 r 附近扫「当前组时间胶囊实心蓝底」的包围盒。只匹配接近
// #0284c7 (2,132,199) 的像素，避开同色系但半透明的外投影（投影混白后 R 通道
// 会明显抬高）。
func strictBlueBox(img *image.RGBA, r ui.Rect) (ui.Rect, bool) {
	x0, y0 := int(r.X)-12, int(r.Y)-12
	x1, y1 := int(r.X+r.W)+12, int(r.Y+r.H)+12
	minX, minY, maxX, maxY := 1<<30, 1<<30, -1, -1
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			rr, gg, bb := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			if rr < 20 && gg > 115 && gg < 150 && bb > 180 && bb < 215 {
				if x < minX {
					minX = x
				}
				if y < minY {
					minY = y
				}
				if x > maxX {
					maxX = x
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		return ui.Rect{}, false
	}
	return ui.Rect{X: float32(minX), Y: float32(minY), W: float32(maxX - minX + 1), H: float32(maxY - minY + 1)}, true
}

// TestDanmakuTimeLabelCapsuleFitContent 钉住当前组时间标签的胶囊几何。原版
// `.danmaku-time-label` 是 `width: fit-content` + `line-height: 1` +
// `padding: 3px 5px`：13px 行盒 + 上下 3px => 19px 高，宽度只包住时间文本。
// mygo 的行高取字体度量的硬底（13px 字号的自然行盒约 20px），`Padding(3,5)`
// 会把胶囊撑到 26px；左列默认 cross 轴 stretch 又会把背景铺满 64px。
// 改成显式 `Height(19)` + `Padding(0,5)` + 左列 `AlignItems(Start)` 后，
// 高 19、宽约 46，与原版一致。
func TestDanmakuTimeLabelCapsuleFitContent(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{{Time: 0, Text: "开头"}, {Time: 30, Text: "中间"}}
	a.Drawer = DrawerDanmaku
	a.Pos = 30

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()

	r, ok := tst.Find("00:30")
	if !ok {
		t.Fatal("渲染帧里找不到当前组的时间标签 00:30")
	}
	if r.H > 20 {
		t.Errorf("时间标签布局高 = %v，应 ≈19（原版 line-height:1 + 上下 padding 3）", r.H)
	}
	if r.W >= 60 {
		t.Errorf("时间标签布局宽 = %v，原版 width:fit-content 不该铺满 64px 左列", r.W)
	}

	box, ok := strictBlueBox(tst.Image(), r)
	if !ok {
		t.Fatal("当前组时间胶囊没有实心蓝底")
	}
	t.Logf("时间胶囊: 布局 %v，蓝底 %v", r, box)
	if box.H > 21 {
		t.Errorf("时间胶囊蓝底高 = %v，应 ≈19", box.H)
	}
	if box.W >= 60 {
		t.Errorf("时间胶囊蓝底宽 = %v，原版 width:fit-content 不该铺满 64px 左列", box.W)
	}
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

// TestDanmakuListResetsOnDataChange 钉住弹幕虚拟列表在**数据来源**变化
// （DanmakuCid 变化）时重置滚动位置。抽屉跟的是「浏览中」那条（原版
// videoInfo），不再是正在播放的 Track：播放 A 时浏览 B，B 的弹幕列表不该沿用
// A 的滚动锚点。改前这里比的是 a.Track.Cid，播放 A 期间浏览 B 不会触发重置。
func TestDanmakuListResetsOnDataChange(t *testing.T) {
	a := NewApp(func() {})
	dms := make([]Danmaku, 200)
	for i := range dms {
		dms[i] = Danmaku{Time: float64(i), Text: fmt.Sprintf("弹幕编号%03d", i)}
	}
	a.Danmaku = dms
	a.DanmakuCid = 20
	a.Drawer = DrawerDanmaku
	a.DanmakuTab = TabDanmaku
	a.DanmakuAutoScroll = false // 关掉跟随，免得它把列表滚回当前组

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()
	tst.Scroll(490, 400, 0, 4000)
	tst.Frame()

	if _, last := a.danmakuList.Visible(); last <= 0 {
		t.Fatalf("滚动后末可见行应不在顶部：last=%d", last)
	}

	// 换了数据来源（浏览到另一条视频）：滚动位置与行锚点重置。
	a.DanmakuCid = 70
	tst.Frame()
	if first, _ := a.danmakuList.Visible(); first != 0 {
		t.Fatalf("换数据来源后弹幕列表应回到顶部：first=%d", first)
	}
	if a.danmakuCid != 70 {
		t.Fatalf("重置后应记住新的来源 cid：%d", a.danmakuCid)
	}
}

// bandAvg 采样一帧里某矩形下方 dyOff 起、宽 2px 的水平带的平均亮度
// （R+G+B 均值）。带 x 范围收窄 2px 避开边缘。
func bandAvg(img *image.RGBA, r ui.Rect, dyOff int) float64 {
	y := int(r.Y+r.H) + dyOff
	if y < 0 || y >= img.Rect.Dy() {
		return -1
	}
	sum, n := 0, 0
	for x := int(r.X) + 2; x < int(r.X+r.W)-2 && x < img.Rect.Dx(); x++ {
		i := img.PixOffset(x, y)
		sum += int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
		n += 3
	}
	if n == 0 {
		return -1
	}
	return float64(sum) / float64(n)
}

// TestDanmakuMessageTextShadow 钉住弹幕文字的白色柔光阴影（原版
// .danmaku-message-text 的 `text-shadow: 0 1px 1px rgba(255,255,255,.68)`）。
//
// 框架没有文字阴影 API：白层是一块文字行盒大小的白 .68、Absolute 垫在
// 原文字下方 1px（先声明画在下层），原色文字 Absolute 盖回来。白层只在
// 文字行盒区域（右缘让开 ×N），文字下缘外露出 1~2px 的白晕：该行亮度
// ≈252（白 .68 over chip 底 247）。此前完全没有这层，文字下缘就是
// chip 底 247。把白层的 Box 删掉，这条即变红。
func TestDanmakuMessageTextShadow(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{{Time: 0, Text: "测试弹幕文字"}}
	a.Drawer = DrawerDanmaku
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()

	r, ok := tst.Find("测试弹幕文字")
	if !ok {
		t.Fatal("渲染帧里找不到弹幕文字")
	}
	img := tst.Image()
	// 白层 = 文字行盒下移 1px：文字 ink 底缘（r.Y+r.H）下方 0..1px
	// 仍在白层内（行盒底 +1px 偏移，白层底 = r.Y+r.H+1）。
	below0 := bandAvg(img, r, 0)
	below1 := bandAvg(img, r, 1)
	t.Logf("文字下缘带亮度: dy0=%.1f dy1=%.1f", below0, below1)
	// 白层 .68 白 over chip 底 ≈252；无白层时 chip 底 ≈247。
	if below0 < 251 || below1 < 251 {
		t.Errorf("文字下缘白晕缺失：dy0=%.1f dy1=%.1f，应 >251（白 .68 over chip 底）", below0, below1)
	}
	// 白层底缘之外（dy=2）应回到 chip 底 ≈247。
	if below2 := bandAvg(img, r, 2); below2 > 250 {
		t.Errorf("dy=2 处亮度 %.1f 仍偏亮：白层覆盖超出行盒区域", below2)
	}
}

// TestDanmakuRepeatCountNotFogged 钉住有 ×N 胶囊时白层不盖住胶囊：
// 白层右缘让开 gap 5 + 胶囊 34px，胶囊保持清晰（蓝字 + 浅蓝底），
// 不被白 .68 雾罩。反证：把白层 Box 的 Right 改成 0（覆盖到胶囊区），
// 胶囊区域亮度会被抬到 ≈252。
func TestDanmakuRepeatCountNotFogged(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{{Time: 0, Text: "测试弹幕文字"}, {Time: 0.5, Text: "测试弹幕文字"}}
	a.Drawer = DrawerDanmaku
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()

	if _, ok := tst.Find("测试弹幕文字"); !ok {
		t.Fatal("渲染帧里找不到弹幕文字")
	}
	rr, ok := tst.Find("×2")
	if !ok {
		t.Fatal("渲染帧里找不到 ×2 胶囊")
	}
	img := tst.Image()
	// 胶囊底色 rgba(224,242,254,.7) over chip 底，中心区域亮度明显低于
	// 白层的 ≈252（胶囊底色偏蓝灰，G/B 通道高但 R 通道低）。
	cx := int(rr.X + rr.W/2)
	cy := int(rr.Y + rr.H/2)
	i := img.PixOffset(cx, cy)
	cr, cg, cb := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
	t.Logf("胶囊中心像素: (%d,%d,%d)", cr, cg, cb)
	// 白层 .68 白 over chip 底 = 252 中性白；胶囊底色偏蓝（R 通道低）。
	// 若白层盖住胶囊，中心像素会被抬成 ≈252 中性白。
	if cr > 235 && cg > 235 && cb > 235 {
		t.Errorf("胶囊中心像素 (%d,%d,%d) 接近中性白 252：白层盖住了 ×N 胶囊", cr, cg, cb)
	}
}

// TestDanmakuRepeatCountHeight 钉住 ×N 胶囊的显式盒高（原版
// `.danmaku-repeat-count` 的 `height:18px; line-height:16px`）。
// 框架自然行盒约 15px（10×1.5），比原版 18px 矮 3px；显式 Height + Translate
// 把胶囊压回原版高度。改回不设 Height 这条会量到约 15。
func TestDanmakuRepeatCountHeight(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Danmaku = []Danmaku{{Time: 0, Text: "测试弹幕文字"}, {Time: 0.5, Text: "测试弹幕文字"}}
	a.Drawer = DrawerDanmaku
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 720)
	tst.Frame()

	rr, ok := tst.Find("×2")
	if !ok {
		t.Fatal("渲染帧里找不到 ×2 胶囊")
	}
	t.Logf("×2 胶囊 rect = %+v", rr)
	if rr.H < 17.5 || rr.H > 18.5 {
		t.Errorf("×2 胶囊盒高 = %v，应 ≈18（原版 height:18px）", rr.H)
	}
}

// TestDanmakuViewportCaps 钉住弹幕分组与消息列的两个 dvh 上限（原版
// `.danmaku-time-group { max-height: 80dvh }` 与
// `.danmaku-message-list { max-height: calc(80dvh - 20px) }`）。
func TestDanmakuViewportCaps(t *testing.T) {
	cases := []struct{ h, group, list float32 }{
		{400, 320, 300},
		{720, 576, 556},
		{1000, 800, 780},
	}
	for _, c := range cases {
		if got := danmakuGroupMaxHeight(c.h); got < c.group-0.05 || got > c.group+0.05 {
			t.Errorf("danmakuGroupMaxHeight(%v) = %v，应 ≈%v（80dvh）", c.h, got, c.group)
		}
		if got := danmakuMessageListMaxHeight(c.h); got < c.list-0.05 || got > c.list+0.05 {
			t.Errorf("danmakuMessageListMaxHeight(%v) = %v，应 ≈%v（80dvh-20）", c.h, got, c.list)
		}
	}
	// 极小视口：calc 会算成负数，钳到 0。
	if got := danmakuMessageListMaxHeight(10); got != 0 {
		t.Errorf("danmakuMessageListMaxHeight(10) = %v，应钳到 0", got)
	}
}

// TestDanmakuTimeTotalLineHeight 钉住 `.danmaku-time-total` 的
// `font-size:11px; line-height:1.2`（行盒 13.2px）。mygo 的行高硬底约 17px，
// 显式 Height + Translate 后「N 条」的盒高应回到 13 附近；改回不设 Height
// 这条会量到约 17。
func TestDanmakuTimeTotalLineHeight(t *testing.T) {
	a := &App{
		Danmaku:           []Danmaku{{Time: 0, Text: "a"}, {Time: 0.5, Text: "b"}},
		Drawer:            DrawerDanmaku,
		DanmakuTab:        TabDanmaku,
		DanmakuAutoScroll: false,
	}
	a.Theme = ThemeFor(PeriodAt(12))
	tt := ui.NewTester(func(c *ui.Context) { a.danmakuBody(c) }, 500, 400)
	tt.Frame()
	r, ok := tt.Find("2 条")
	if !ok {
		t.Fatal("弹幕抽屉里找不到「2 条」总数")
	}
	t.Logf("「2 条」rect = %+v", r)
	if r.H < 13 || r.H > 13.5 {
		t.Errorf("「2 条」盒高 = %v，应 ≈13.2（原版 line-height:1.2）", r.H)
	}
}

// danmakuOverflowApp 造一个直接跑 danmakuBody 的 App：第一组同一秒塞 120 条
// 互不相同的弹幕（远超 80dvh-20 的消息列上限），第二组一条，用来验证分组内
// 滚动而不是把整行无限拉长。
func danmakuOverflowApp() *App {
	list := make([]Danmaku, 0, 121)
	for i := 0; i < 120; i++ {
		list = append(list, Danmaku{Time: 0, Text: fmt.Sprintf("弹幕%03d", i)})
	}
	list = append(list, Danmaku{Time: 1, Text: "第二组"})
	a := &App{
		Danmaku:           list,
		Drawer:            DrawerDanmaku,
		DanmakuTab:        TabDanmaku,
		DanmakuAutoScroll: false,
	}
	a.Theme = ThemeFor(PeriodAt(12))
	return a
}

// TestDanmakuMessageListInnerScroll 无头实测两件事：
//  1. 第一组被 80dvh 封顶：第二组不会因为 120 条弹幕被推到很下面；
//  2. 在消息列上滚轮时，内滚动先动（第一条弹幕上移），第二组不动
//     （外层列表没被带着滚）——对应原版 `.danmaku-message-list` 的
//     `overflow-y:auto`。
func TestDanmakuMessageListInnerScroll(t *testing.T) {
	a := danmakuOverflowApp()
	tt := ui.NewTester(func(c *ui.Context) { a.danmakuBody(c) }, 500, 400)
	tt.Frame()

	second, ok := tt.Find("第二组")
	if !ok {
		t.Fatal("找不到第二组（第一组可能没有被 80dvh 封顶，把第二组推出了视口）")
	}
	// 第一组封顶后，第二组应还在 400px 视口内；不封顶时 120 条会把第二组
	// 推到 y≈700（Find 也找不到）。这里留一点 List 行高量测的余量。
	if second.Y > 360 {
		t.Errorf("第二组 y = %v，第一组没有被 80dvh 封顶", second.Y)
	}

	// 用一条滚后仍在视口内的弹幕（024）当探针：内滚动生效时它会上移，
	// 外层分组（第二组）不动。
	probe, ok := tt.Find("弹幕024")
	if !ok {
		t.Fatal("找不到探针弹幕 024")
	}
	beforeProbe, beforeSecond := probe.Y, second.Y
	tt.Scroll(probe.X+probe.W/2, probe.Y+probe.H/2, 0, 80)

	afterProbe, ok := tt.Find("弹幕024")
	if !ok {
		t.Fatal("滚动后找不到探针弹幕 024")
	}
	afterSecond, ok := tt.Find("第二组")
	if !ok {
		t.Fatal("滚动后找不到第二组")
	}
	if afterProbe.Y >= beforeProbe-40 {
		t.Errorf("消息列内滚动没生效：探针弹幕 y %v → %v", beforeProbe, afterProbe.Y)
	}
	if afterSecond.Y != beforeSecond {
		t.Errorf("内滚动把外层列表也带着动了：第二组 y %v → %v", beforeSecond, afterSecond.Y)
	}
}
