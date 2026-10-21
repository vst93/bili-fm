package view

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestDrawerSnap 钉住下滑手势的落点：过中点（或带速度投影后过中点）就落到
// 最底（等关闭），否则回弹到原位。
func TestDrawerSnap(t *testing.T) {
	const h float32 = 400
	snap := drawerSnap(h)
	cases := []struct {
		in   ui.Point
		want ui.Point
	}{
		{ui.Point{}, ui.Point{}},
		{ui.Point{Y: 100}, ui.Point{}},
		{ui.Point{Y: 200}, ui.Point{}}, // 正好中点：Nearest 取先出现的 {} 那一侧
		{ui.Point{Y: 201}, ui.Point{Y: h}},
		{ui.Point{Y: h}, ui.Point{Y: h}},
	}
	for _, c := range cases {
		if got := snap(c.in); got != c.want {
			t.Errorf("drawerSnap(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

// TestDrawerDismissed 钉住「动画途中不关」：只有偏移到底且停稳才算关闭，
// 否则抽屉会在滑下去的中途就消失。
func TestDrawerDismissed(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		offsetY       float32
		transitioning bool
		want          bool
	}{
		{0, false, false},
		{200, false, false},
		{h, true, false}, // 到底了但还在滑：不能关
		{h, false, true},
		{h + 20, false, true},
	}
	for _, c := range cases {
		if got := drawerDismissed(c.offsetY, h, c.transitioning); got != c.want {
			t.Errorf("drawerDismissed(%v, %v, %v) = %v, want %v",
				c.offsetY, h, c.transitioning, got, c.want)
		}
	}
	// 高度退化时不能误判为关闭。
	if drawerDismissed(0, 0, false) {
		t.Error("drawerDismissed(0, 0, false) = true, want false")
	}
}

// TestDrawerFade 钉住遮罩随下滑的透明度：贴底 1、到底 0、中间线性，
// 且越界时夹在 [0, 1]。
func TestDrawerFade(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		offsetY float32
		want    float32
	}{
		{0, 1},
		{-10, 1}, // 上弹越界：夹回 1
		{200, 0.5},
		{h, 0},
		{h + 50, 0}, // 下甩越界：夹到 0
	}
	for _, c := range cases {
		if got := drawerFade(c.offsetY, h); got != c.want {
			t.Errorf("drawerFade(%v, %v) = %v, want %v", c.offsetY, h, got, c.want)
		}
	}
	// 高度退化（首帧还没布局）时不能除零，按不透明处理。
	if got := drawerFade(10, 0); got != 1 {
		t.Errorf("drawerFade(10, 0) = %v, want 1", got)
	}
}

func TestFieldStyle(t *testing.T) {
	// 列比例 / 伸展 / 对齐 / 隐藏阈值对齐原版 .card-meta-field.is-*。
	cases := []struct {
		kind  cardFieldKind
		basis float32
		grow  float32
		minW  float32
		align ui.Align
	}{
		{fieldAuthor, 40, 4, 0, ui.Start},
		{fieldViews, 30, 3, 150, ui.End},
		{fieldPubdate, 30, 4, 200, ui.End}, // 日期列 grow 4，可从富余里「借」宽
		{fieldDuration, 30, 3, 240, ui.End},
		{fieldExtra, 30, 3, 260, ui.End},
	}
	for _, c := range cases {
		basis, grow, minW, align := fieldStyle(c.kind)
		if basis != c.basis || grow != c.grow || minW != c.minW || align != c.align {
			t.Errorf("fieldStyle(%s) = (%v,%v,%v,%v), want (%v,%v,%v,%v)",
				c.kind, basis, grow, minW, align, c.basis, c.grow, c.minW, c.align)
		}
	}
}

func TestResolveFieldWidths(t *testing.T) {
	// 三列：basis 之和 = 100%，无富余，列宽就是各自比例。
	three := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "09-22"},
	}
	got := resolveFieldWidths(three, 230)
	want := []float32{92, 69, 69}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("三列宽度[%d] = %v, want %v", i, got[i], want[i])
		}
	}

	// 两列：富余空间按 grow 比例分完（列仍填满整行）。
	two := []cardField{
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldDuration, Text: "12:30"},
	}
	got = resolveFieldWidths(two, 300)
	if got[0] != 150 || got[1] != 150 {
		t.Errorf("两列宽度 = %v, want [150 150]", got)
	}

	// 发布时间列 grow 4（大于播放量的 3），富余里能「借」到更宽。
	two = []cardField{
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "2024-03-10"},
	}
	got = resolveFieldWidths(two, 300)
	if got[1] <= got[0] {
		t.Errorf("日期列应借到比播放量更宽: %v", got)
	}
	if diff := (got[0] + got[1]) - 300; diff > 0.01 || diff < -0.01 {
		t.Errorf("两列未填满整行: %v", got)
	}

	// 窄于阈值：整列隐藏（宽 0），富余只分给剩下的列。
	narrow := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldExtra, Text: "已看 30%"},
	}
	got = resolveFieldWidths(narrow, 200) // extra 阈值 260 → 隐藏
	if got[2] != 0 {
		t.Errorf("窄卡 extra 列宽 = %v, want 0", got[2])
	}
	// author(40) + views(30) 基准 70% = 140，富余 60 按 grow 4:3 分。
	if diff := (got[0] + got[1]) - 200; diff > 0.01 || diff < -0.01 {
		t.Errorf("剩余两列未填满整行: %v", got)
	}

	// 四列：basis 之和 130% > 行宽，按比例收缩到恰好填满（原版 flex-shrink）。
	four := []cardField{
		{Kind: fieldAuthor, Text: "UP"},
		{Kind: fieldViews, Text: "1.2万"},
		{Kind: fieldPubdate, Text: "2024-03-10"},
		{Kind: fieldExtra, Text: "已看 30%"},
	}
	got = resolveFieldWidths(four, 300)
	var sum float32
	for _, w := range got {
		sum += w
	}
	if sum > 300.01 || sum < 299.99 {
		t.Errorf("四列总宽 = %v, want 300", sum)
	}
	if got[2] >= 90 {
		t.Errorf("四列时日期列应被压缩到 <90，got %v", got[2])
	}
}

func TestPubdateCandidates(t *testing.T) {
	now := time.Date(2025, 9, 20, 12, 0, 0, 0, time.UTC)

	// 完整年月日 → 具体到抽象阶梯：ymd → ym → N个月前 → N年前。
	got := pubdateCandidates("2024-03-10", now)
	want := []string{"2024-03-10", "2024-03", "18个月前", "1年前"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidates[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	// 当月同日未到：月差少算一个月。
	got = pubdateCandidates("2024-03-25", now)
	if got[len(got)-2] != "17个月前" {
		t.Errorf("月差 = %q, want 17个月前", got[len(got)-2])
	}

	// 非完整年月日（相对形态 / MM-DD / 无法解析）原样作为唯一候选。
	for _, in := range []string{"昨天", "09-22", "2024-03", "不是日期"} {
		if c := pubdateCandidates(in, now); len(c) != 1 || c[0] != in {
			t.Errorf("pubdateCandidates(%q) = %v, want [%q]", in, c, in)
		}
	}
	if c := pubdateCandidates("  ", now); c != nil {
		t.Errorf("空白 = %v, want nil", c)
	}
}

func TestPickDateForm(t *testing.T) {
	cands := []string{"2024-03-10", "2024-03", "18个月前", "1年前"}
	// 放得下就取最具体的一档。
	if got := pickDateForm(cands, func(s string) bool { return len(s) <= 7 }); got != "2024-03" {
		t.Errorf("pickDateForm(width 7) = %q, want 2024-03", got)
	}
	// 一个都放不下：兜底取最后一档，绝不返回空。
	if got := pickDateForm(cands, func(string) bool { return false }); got != "1年前" {
		t.Errorf("pickDateForm(都不放) = %q, want 1年前", got)
	}
	if got := pickDateForm(nil, func(string) bool { return true }); got != "" {
		t.Errorf("pickDateForm(空) = %q, want empty", got)
	}
}

// TestDrawerAnimOffset 钉住用「布局位移」做抽屉动画的进度函数。
//
// 关键：面板的位移必须是**布局**（Bottom）而不是 pose（transform）——mygo 的
// 软件渲染器对任何带 transform 的 op 都走逐像素重绘，整棵抽屉子树（面板 +
// 卡片 + 文字）都会被拖慢（无头实测退场一帧 3.8s vs 布局路径 65ms）。这条测试
// 锁住：入场从 h 升到 0、退场从 0 降到 h、走完就不再请帧。
func TestDrawerAnimOffset(t *testing.T) {
	const h float32 = 400
	t0 := time.Unix(1000, 0)
	for _, c := range []struct {
		name    string
		moving  bool
		closing bool
		elapsed time.Duration
		wantY   float32
		wantRun bool
	}{
		{"未开始", false, false, 0, 0, false},
		{"入场起手", true, false, 0, h, true},
		{"入场结束", true, false, drawerIn, 0, false},
		{"退场起手", true, true, 0, 0, true},
		{"退场结束", true, true, drawerIn, h, false},
	} {
		gotY, gotRun := drawerAnimOffset(c.moving, c.closing, t0, t0.Add(c.elapsed), h)
		if gotY != c.wantY || gotRun != c.wantRun {
			t.Errorf("%s: drawerAnimOffset = (%v, %v), want (%v, %v)", c.name, gotY, gotRun, c.wantY, c.wantRun)
		}
	}
	// 中途：入场在上方（0<y<h）、退场在下方，且都在跑。
	mid := drawerIn / 2
	if y, run := drawerAnimOffset(true, false, t0, t0.Add(mid), h); y <= 0 || y >= h || !run {
		t.Errorf("入场中途 = (%v, %v)，应在 (0, %v) 且还在跑", y, run, h)
	}
	if y, run := drawerAnimOffset(true, true, t0, t0.Add(mid), h); y <= 0 || y >= h || !run {
		t.Errorf("退场中途 = (%v, %v)，应在 (0, %v) 且还在跑", y, run, h)
	}
}

// TestDrawerEnterStep 钉住抽屉入场的判定——入场动画消失的回归点。
//
// 点击打开抽屉的那一帧，`Clicked()` 会当场置 consumed，框架把整个视图重建
// 第二趟（input.go）。入场进度按「起始时间戳」算，只在帧内取一次时间，所以
// 第二趟若把「已经不是新抽屉」当成立场结束，就会把起始时间戳从此刻改掉，
// 进度从中间起跳——动画没了。drawerEnterStep 用「同一帧内 c.Now() 不变」
// 识别第二趟，让起始时间戳在整个入场帧里保持不变。
func TestDrawerEnterStep(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(16 * time.Millisecond)

	// 首次打开：进入入场帧，记下 key 与这一刻。
	entering, key, at := drawerEnterStep("feed", "", time.Time{}, t0)
	if !entering || key != "feed" || !at.Equal(t0) {
		t.Fatalf("首次打开 = (%v, %q, %v), want (true, feed, t0)", entering, key, at)
	}
	// 同一帧的第二次 build pass：时间戳不变，必须仍是入场帧。
	if entering, k, a := drawerEnterStep("feed", key, at, t0); !entering || k != "feed" || !a.Equal(t0) {
		t.Fatalf("同帧第二趟 = (%v, %q, %v), 应为入场帧", entering, k, a)
	}
	// 下一帧：入场结束，进度交给 drawerAnimOffset 从 h 滑到就位的 0。
	if entering, _, _ := drawerEnterStep("feed", key, at, t1); entering {
		t.Fatal("下一帧不该再是入场帧（否则永远停在底部）")
	}
	// 换抽屉（或关掉后重开另一个）：重新入场。
	if entering, k, a := drawerEnterStep("parts", key, at, t1); !entering || k != "parts" || !a.Equal(t1) {
		t.Fatalf("换抽屉 = (%v, %q, %v), want (true, parts, t1)", entering, k, a)
	}
}

// TestDrawerExitDone 钉住退场收尾的时机：退场起始的那一帧不能收尾（刚设的
// 起始位移还没被渲染过，立刻收尾等于没动画），动画走完且隔了一帧之后才把
// 抽屉真正移出视图。
func TestDrawerExitDone(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(16 * time.Millisecond)

	if drawerExitDone(t0, t0, false) {
		t.Error("退场起步帧不该收尾：起始位移还没被渲染过")
	}
	if drawerExitDone(t0, t1, true) {
		t.Error("动效还在跑时不该收尾")
	}
	if !drawerExitDone(t0, t1, false) {
		t.Error("隔了一帧且停稳后应收尾")
	}
}

// TestCardGridVirtualized 钉住卡片网格（订阅/动态、搜索、分区等）走 mygo 的
// 虚拟化 ui.List：几百张卡时也只构建可见的那几排，屏外卡片不建，滚走后顶部
// 卡片被回收。改回「ui.Column 每帧全建」会让滚动时每帧重建全部卡片。
func TestCardGridVirtualized(t *testing.T) {
	a := NewApp(func() {})
	cards := make([]Card, 300)
	for i := range cards {
		cards[i] = Card{
			Bvid: fmt.Sprintf("BV%04d", i), Title: fmt.Sprintf("视频标题%d", i),
			Up: "UP主", Views: "1.2万", Pubdate: "09-22",
		}
	}
	a.ListFor("feed").Cards = cards
	a.Drawer = "feed"

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))

	if !tst.HasText("视频标题0") {
		t.Fatal("首屏应构建第一排卡片")
	}
	if tst.HasText("视频标题299") {
		t.Fatal("最后一张在屏外，虚拟化网格不该构建它（说明又退回每帧全建）")
	}

	// 滚到底：末尾卡片建出来，顶部的被回收。
	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	if !tst.HasText("视频标题299") {
		t.Fatal("滚到底应构建末尾卡片")
	}
	if tst.HasText("视频标题0") {
		t.Fatal("滚走后顶部卡片应被虚拟化回收")
	}
}

// newPartsInfo 造一个带 n 个分集的视频详情，分集封面为首帧图（cover 作为地址前缀）。
func newPartsInfo(n int, cover string) *Info {
	parts := make([]Part, n)
	for i := range parts {
		parts[i] = Part{
			Cid: int64(1000 + i), Page: i + 1,
			Part:       fmt.Sprintf("选集标题%d", i),
			FirstFrame: fmt.Sprintf("%s/part%d.png", cover, i),
		}
	}
	return &Info{Bvid: "BVtest", Cid: 1000, Title: "视频", Pic: cover + "/main.png", Parts: parts}
}

// TestPartsGridVirtualized 钉住选集抽屉也走虚拟化 ui.List：多 P 视频只构建可见
// 的那几排分集卡片。之前 partsBody 是 ui.Column 逐张全建——一集几百 P 时每帧
// 都重建全部卡片，滚动直接卡死。
func TestPartsGridVirtualized(t *testing.T) {
	a := NewApp(func() {})
	a.Images = newRecordingCache()
	a.Info = newPartsInfo(300, "cover")
	a.Drawer = DrawerParts

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	if !tst.HasText("选集标题0") {
		t.Fatal("首屏应构建第一排分集卡片")
	}
	if tst.HasText("选集标题299") {
		t.Fatal("屏外分集不该构建（说明又退回 ui.Column 每帧全建）")
	}

	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	if !tst.HasText("选集标题299") {
		t.Fatal("滚到底应构建末尾分集")
	}
	if tst.HasText("选集标题0") {
		t.Fatal("滚走后顶部分集应被虚拟化回收")
	}
}

// TestPartsLocateRow 钉住「定位到当前 / 选集搜索」目标行的换算：
// 优先选集搜索匹配的那一集，否则回退到正在播放的那一集，都没有返回 -1。
// 列数是参数（窄窗 2 / ≥640 3），同一分集在两种列数下落在不同的行上。
func TestPartsLocateRow(t *testing.T) {
	parts := make([]Part, 10)
	for i := range parts {
		parts[i] = Part{Cid: int64(i + 1)}
	}
	// 搜索匹配优先于正在播放（两者不同时看搜索）。
	if got, want := partsLocateRow(parts, &Track{Cid: 1}, 7, 3), 7/3; got != want {
		t.Errorf("三列：搜索匹配行 = %d, want %d", got, want)
	}
	// Cid 5 是下标 4（Cid = i+1）：三列在第 1 行。
	if got, want := partsLocateRow(parts, &Track{Cid: 5}, -1, 3), 4/3; got != want {
		t.Errorf("三列：当前播放行 = %d, want %d", got, want)
	}
	// 窄窗两列：第 7 集（下标 7）落在第 3 行而不是第 2 行，
	// 第 4 集（下标 4）落在第 2 行而不是第 1 行。
	if got, want := partsLocateRow(parts, &Track{Cid: 1}, 7, 2), 7/2; got != want {
		t.Errorf("两列：搜索匹配行 = %d, want %d", got, want)
	}
	if got, want := partsLocateRow(parts, &Track{Cid: 5}, -1, 2), 4/2; got != want {
		t.Errorf("两列：当前播放行 = %d, want %d", got, want)
	}
	if got := partsLocateRow(parts, nil, -1, 3); got != -1 {
		t.Errorf("无目标 = %d, want -1", got)
	}
	if got := partsLocateRow(parts, &Track{Cid: 999}, -1, 3); got != -1 {
		t.Errorf("当前播放不在列表 = %d, want -1", got)
	}
	if got := partsLocateRow(parts, nil, 99, 3); got != -1 {
		t.Errorf("越界下标 = %d, want -1", got)
	}
}

// TestPartsLocateScrollsGrid 钉住选集搜索的滚动：目标行在屏外时不构建，
// 所以不能再靠卡片自己的 ScrollIntoView，必须把目标**行**交给 ListState。
func TestPartsLocateScrollsGrid(t *testing.T) {
	a := NewApp(func() {})
	a.Images = newRecordingCache()
	a.Info = newPartsInfo(300, "cover")
	a.Drawer = DrawerParts
	a.locateNow = true
	a.locatePartIndex = 250 // 第 83 行，远在屏外

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	if !tst.HasText("选集标题250") {
		t.Fatal("定位应把目标分集滚进视野")
	}
	if a.locateNow || a.locatePartIndex != -1 {
		t.Fatalf("定位标志应在构建后清掉：locateNow=%v idx=%d", a.locateNow, a.locatePartIndex)
	}
}

// TestPartsGridLazyLoadsVisibleCovers 钉住选集封面也只按可见窗口预取：
// 之前 partCard 在构建期直接 Bitmap()，而 List 未量高前会多铺行，一开抽屉
// 就并发抓取所有屏外分集的封面。
func TestPartsGridLazyLoadsVisibleCovers(t *testing.T) {
	cs := newCoverServer(t)
	a := NewApp(func() {})
	a.Info = newPartsInfo(300, cs.URL)
	a.Drawer = DrawerParts

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1400, 900))
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		tst.Frame()
		if cs.count() > 0 && a.Images.Pending() == 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	n := cs.count()
	if n == 0 {
		t.Fatal("首屏可见封面一张都没请求")
	}
	if n > 60 {
		t.Fatalf("首屏请求了 %d 张封面（屏外分集也被请求，懒加载失效）", n)
	}
	if cs.requested("/part250.png") {
		t.Fatal("第 250 集在屏外，不该被请求")
	}

	tst.Scroll(700, 700, 0, 100000)
	for i := 0; i < 5; i++ {
		tst.Frame()
	}
	if !cs.requested("/part299.png") {
		t.Fatal("滚到底后最后一集的封面应被请求")
	}
}

// TestCardTitleBadge 钉住标题前的小胶囊（原版 feedList 的 titlePrefix「充电专属」）：
// 卡片带 Badge 时标题行把胶囊画出来，没有时不画。
func TestCardTitleBadge(t *testing.T) {
	a := NewApp(func() {})
	a.ListFor("feed").Cards = []Card{
		{Bvid: "BV1", Title: "充电视频", Badge: "充电专属", Up: "UP主"},
	}
	a.Drawer = "feed"
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()
	if !tst.HasText("充电专属") {
		t.Fatal("带 Badge 的卡片应画出标题前胶囊")
	}
	if !tst.HasText("充电视频") {
		t.Fatal("胶囊不应吃掉标题")
	}

	b := NewApp(func() {})
	b.ListFor("feed").Cards = []Card{{Bvid: "BV2", Title: "普通视频", Up: "UP主"}}
	b.Drawer = "feed"
	tst2 := ui.NewTester(func(c *ui.Context) { b.Shell(c) }, 980, 720)
	tst2.Frame()
	if tst2.HasText("充电专属") {
		t.Fatal("没有 Badge 的卡片不该画出胶囊")
	}

	// 原版 titlePrefix 是 `py-0.5`（上下 2px）。mygo 是 border-box，且胶囊盒高
	// 锁在标题行盒 th 上（内边距不撑高行盒，见 cardfooter_test.go 的
	// TestCardTitleBadgeLineBox），所以「有上下内边距」只能钉常量 + 确认它真的
	// 接在胶囊上：只加 Padding 不改常量、或改了常量没接上都该红。
	if cardTitleBadgePadY != 2 {
		t.Errorf("胶囊上下内边距 = %d，应为 2（原版 py-0.5）", cardTitleBadgePadY)
	}
	src, err := os.ReadFile("drawer.go")
	if err != nil {
		t.Fatalf("读 drawer.go 失败：%v", err)
	}
	// 2026-10 内存排查后，胶囊的「负 half-leading」由**负的上内边距**实现
	// （不再是 Translate，见 linebox.go 文件头），所以上内边距是
	// cardTitleBadgePadY+tsh、下内边距仍是常量 cardTitleBadgePadY。
	if !bytes.Contains(src, []byte("Padding(cardTitleBadgePadY+tsh, 4, cardTitleBadgePadY, 4)")) {
		t.Error("标题前胶囊没接上 cardTitleBadgePadY（上下内边距）")
	}
}

// TestDrawerBackdropLayers 锁住抽屉打开时一帧的 backdrop 玻璃层数上界。
//
// 为什么是硬约束：mygo 对每个 `Backdrop: true` 的效果都要单独断一次渲染
// 通道 + 回读下采样 + 两次模糊（gpu/gpu.go:412、gpu/metal/metal.go:830），
// 层数直接乘进每帧——这正是抽屉滚动/入场卡顿的根因（见 .pending-bugs.md）。
// 原版 CSS 里真有 backdrop-filter 的只剩搜索药丸（.home-searchbar::before）
// 与播放栏（#player .player-controls）；本轮按用户要求给抽屉面板加回一层
// 20px 模糊，所以上界是 3。再多就是有人又给卡片/按钮盖了真玻璃。
//
// 计数在构建期（view.backdropLayers），不受绘制重试/阴影影响。
func TestDrawerBackdropLayers(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	l.Cards = make([]Card, 12)
	for i := range l.Cards {
		l.Cards[i] = Card{Bvid: fmt.Sprintf("BV%02d", i), Title: fmt.Sprintf("标题%d", i)}
	}
	a.Drawer = "feed"
	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	BackdropLayers() // 丢掉轮询期的计数，只量稳定后的这一帧
	tst.Frame()
	if got := BackdropLayers(); got > 3 {
		t.Fatalf("抽屉打开时 backdrop 层数 = %d, want <= 3（多了会让每帧多出渲染通道打断 + 回读 + 两次模糊）", got)
	}
}

// TestDrawerAnimatesByLayout 钉住入/退场真的在动，且位移是布局位移（Find
// 拿到的矩形随进度上/下移），不靠 pose。drawerAnimOffset 的进度数学由
// TestDrawerAnimOffset 钉住；为什么用布局而不是 transform（软件渲染器对带
// transform 的 op 逐像素重绘，退场一帧 3.8s vs 65ms）见 drawerAnimOffset 注释。
//
// 这里不能靠「真等一下再拍一帧」观察中途：`Frame()` 内部的 settle() 会连跑
// 最多 20 帧，320×220 下 20 帧 ≈ 260ms，足以把 220ms 的动画跑完，起手/中途
// 都会被 settle 吃掉。改为把入/退场时间戳钉在动画两端：未来 → 进度恒为 0
// （起手），过去一整个时长 → 动画已结束。
//
// 时间戳必须取自 **Tester 的时钟**（tst.Now()），不是 time.Now()：v0.5.5 起
// Tester 的时钟是冻结的（只在 Advance 时前进），生产代码里的 c.Now() 读到的是
// 它；用真实 time.Now() 写「过去一整个时长」时，冻结时钟其实还停在测试起手那
// 一刻，`now - exitAt` 仍小于 drawerIn，退场被判成「还在动」而不收尾 —— 正是
// 这条测试在 v0.5.5 升级后一直红的根因（反证：改成 time.Now() 立刻红）。
func TestDrawerAnimatesByLayout(t *testing.T) {
	a := NewApp(func() {})
	a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "标题"}}
	a.Drawer = "feed"
	a.drawerShown = "feed"
	a.drawerOpenKey = "feed"
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 320, 220)

	// 入场起手：时间戳在未来，面板停在窗口下方（偏移 h），标题应远低于就位位。
	a.drawerEnterAt = tst.Now().Add(time.Second)
	tst.Frame()
	start, ok := tst.Find("标题")
	if !ok {
		t.Fatal("抽屉打开后找不到卡片标题")
	}

	// 入场走完：时间戳在过去一整个时长，面板贴底就位，标题上移。
	a.drawerEnterAt = tst.Now().Add(-2 * drawerIn)
	tst.Frame()
	end, _ := tst.Find("标题")
	if !(end.Y < start.Y) {
		t.Fatalf("入场未把面板上移：起手 y=%v，就位 y=%v", start.Y, end.Y)
	}

	// 退场起手：退场时间戳在未来，面板停在贴底（偏移 0）——不该瞬间消失。
	a.Drawer = ""
	a.drawerShown = "feed"
	a.drawerClosing = true
	a.drawerOpenKey = ""
	a.drawerExitAt = tst.Now().Add(time.Second)
	tst.Frame()
	if _, ok := tst.Find("标题"); !ok {
		t.Fatal("退场起手面板就不见了（应在贴底继续渲染）")
	}

	// 退场走完：时间戳在过去一整个时长，面板移出视图。
	a.drawerExitAt = tst.Now().Add(-2 * drawerIn)
	tst.Frame()
	if _, ok := tst.Find("标题"); ok {
		t.Fatal("退场走完后面板应移出视图")
	}
}

// TestDrawerExitKeepsPanel 钉住「退场有过渡、不瞬间消失」：关掉抽屉后的
// 第一帧，面板还要继续渲染（下滑淡出），而不是当即被移出视图。
func TestDrawerExitKeepsPanel(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	l.Cards = make([]Card, 6)
	for i := range l.Cards {
		l.Cards[i] = Card{Bvid: fmt.Sprintf("BV%02d", i), Title: fmt.Sprintf("标题%d", i)}
	}
	a.Drawer = "feed"

	// 小窗口让每帧足够快：settle 的 20 帧远达不到动效时长，所以帧末面板仍在
	// 过渡中。
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 320, 220)
	tst.Frame()
	if !tst.HasText("标题0") {
		t.Fatal("抽屉打开时应有卡片")
	}
	a.Drawer = ""
	// Tester.Frame() 的 settle 会连跑到动画结束（最多 20 帧）；机器负载高时
	// 一帧就可能跨过 drawerIn(220ms) 把面板收掉，断言偶发假失败。这里在触发
	// 构建前把退场时钟钉在「刚起步」那一刻（起始时间放到未来 ⇒ 进度恒为 0、
	// 永远不算走完），断言就只关乎「退场期间面板仍被渲染」这一件事。时间戳取
	// Tester 的冻结时钟（同 TestDrawerAnimatesByLayout）。退场状态的收尾由
	// TestDrawerExitState 覆盖。
	a.drawerClosing = true
	a.drawerExitAt = tst.Now().Add(2 * drawerIn)
	tst.Frame()
	if !tst.HasText("标题0") {
		t.Fatal("关抽屉后第一帧面板就消失（退场过渡丢失）")
	}
}

// TestDrawerExitState 钉住退场的状态收尾：退场一开始就清掉 drawerOpenKey
// （否则同一抽屉关掉后马上重开会没有入场动画），并且动画走完后最终真的
// 把面板移出视图（drawerShown 清空），不会一直留在场景里。
func TestDrawerExitState(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	l.Cards = []Card{{Bvid: "BV1", Title: "标题"}}
	a.Drawer = "feed"

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 500, 400))

	a.Drawer = ""
	tst.Frame()
	if a.drawerOpenKey != "" {
		t.Fatal("退场开始后 drawerOpenKey 应为空，否则同一抽屉重开没有入场动画")
	}
	// 再推时间，退场应走完并收尾（drawerShown 清空）。v0.5.5 冻结时钟用 Advance。
	tst.Advance(400 * time.Millisecond)
	if a.drawerShown != "" {
		t.Fatal("退场停稳后没有收尾，面板会一直留在场景里")
	}
}

// TestHistoryRefreshDisabledWhileLoading 钉住历史抽屉的刷新键在刷新中禁用
// （原版 historyList 的 isDisabled={isRefreshing}）。之前原生版的通用刷新键
// 任何时候都能点，连点会重复触发 Reload。
func TestHistoryRefreshDisabledWhileLoading(t *testing.T) {
	a := NewApp(func() {})
	a.Drawer = "history"
	a.HistTab = HistHistory
	reloads := 0
	a.Act.Reload = func() { reloads++ }

	l := a.ListFor("history")
	l.Loading = true
	tt := shellTester(t, a)
	// 刷新中：点击应被禁用挡住（找不到可点元素也不该触发 Reload）。
	_ = tt.Click("刷新历史记录")
	tt.Frame()
	if reloads != 0 {
		t.Fatalf("刷新中点击刷新键触发了 Reload：reloads=%d，想 0", reloads)
	}

	// 加载结束：刷新键恢复可点。
	l.Loading = false
	tt.Frame()
	if err := tt.Click("刷新历史记录"); err != nil {
		t.Fatalf("加载结束后点不到刷新键：%v", err)
	}
	tt.Frame()
	if reloads != 1 {
		t.Fatalf("刷新结束后点击刷新键：reloads=%d，想 1", reloads)
	}
}

// TestCanAutoLoadMore 钉住自动翻页的判定：有更多、不在加载中、上一次没失败，
// 且已经滚到底（首行不在顶部）。尤其是 LoadErr——mygo 的翻页触发在渲染里，
// 每帧都跑，错了就会在失败后每帧重发一次请求。
func TestCanAutoLoadMore(t *testing.T) {
	base := List{HasMore: true}
	loadErr := List{HasMore: true, LoadErr: true}
	loading := List{HasMore: true, Loading: true}
	noMore := List{}
	cases := []struct {
		name              string
		l                 List
		first, last, rows int
		want              bool
	}{
		{"滚到底且有更多", base, 5, 9, 10, true},
		{"未滚到底", base, 0, 4, 10, false},
		{"高窗首屏已到底", base, 0, 9, 10, true},
		{"没有更多", noMore, 5, 9, 10, false},
		{"加载中", loading, 5, 9, 10, false},
		{"上次失败", loadErr, 5, 9, 10, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := canAutoLoadMore(&c.l, c.first, c.last, c.rows); got != c.want {
				t.Fatalf("canAutoLoadMore = %v, want %v", got, c.want)
			}
		})
	}
}

// newFeedList 造一个有 n 张卡、还有下一页的 feed 抽屉，返回 App 与列表。
func newFeedList(t *testing.T, n int) (*App, *List) {
	t.Helper()
	a := NewApp(func() {})
	cards := make([]Card, n)
	for i := range cards {
		cards[i] = Card{
			Bvid: fmt.Sprintf("BV%04d", i), Title: fmt.Sprintf("视频标题%d", i),
			Up: "UP主", Views: "1.2万", Pubdate: "09-22",
		}
	}
	l := a.ListFor("feed")
	l.Cards, l.HasMore = cards, true
	a.Drawer = "feed"
	return a, l
}

// TestLoadMoreErrorShowsRetryRow 钉住翻页失败后网格末尾出现「加载失败，点击重试」
// 行（原版 historyList 的 .history-load-more-retry），错误态清掉后又消失。
func TestLoadMoreErrorShowsRetryRow(t *testing.T) {
	a, l := newFeedList(t, 30)
	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	if tst.HasText("加载失败，点击重试") {
		t.Fatal("没有加载失败时不该有重试行")
	}

	l.LoadErr = true
	tst.Frame()
	// 重试行在网格最末尾：滚到底把它带进视野（真实使用中用户此刻就在底部）。
	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	if !tst.HasText("加载失败，点击重试") {
		t.Fatal("翻页失败后应在网格末尾画重试行")
	}

	l.LoadErr = false
	tst.Frame()
	if tst.HasText("加载失败，点击重试") {
		t.Fatal("错误态清掉后重试行应消失")
	}
}

// TestLoadMoreErrorStopsAutoRetry 是本轮修的回归：翻页失败后，即使一直停在
// 列表底部，也不该每帧自动重发；只有点「加载失败，点击重试」才重试一次。
func TestLoadMoreErrorStopsAutoRetry(t *testing.T) {
	a, l := newFeedList(t, 30)
	calls := 0
	a.Act.LoadMore = func() { calls++; l.Loading = true }

	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))

	// 滚到底：第一次自动翻页。
	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	if calls != 1 {
		t.Fatalf("滚到底后自动翻页 %d 次，想 1", calls)
	}

	// 模拟这一页请求失败：装配层把 Loading 落下并置 LoadErr。
	l.Loading, l.LoadErr = false, true
	for i := 0; i < 8; i++ {
		tst.Frame()
	}
	if calls != 1 {
		t.Fatalf("翻页失败后在底部又自动重试了：calls=%d，想 1", calls)
	}

	// 重试行在网格末尾：滚到底把它带进视野。
	tst.Scroll(490, 400, 0, 100000)
	tst.Frame()
	// 点重试：清错误态并恰好再来一次。
	l.Loading = false
	if err := tst.Click("加载失败，点击重试"); err != nil {
		t.Fatalf("点不到重试行：%v", err)
	}
	tst.Frame()
	if calls != 2 {
		t.Fatalf("点重试后 calls=%d，想 2", calls)
	}
	if l.LoadErr {
		t.Fatal("点重试应清掉错误态")
	}
}

// TestDrawerCloseButtonPaintTokens 逐值钉住关闭键三态材质（原版
// globals.css:4112-4140 的 button[aria-label="Close"] 三条规则）。
func TestDrawerCloseButtonPaintTokens(t *testing.T) {
	base := drawerClosePaintFor(false, false)
	if base.BG != ui.Hex("#ffffff").Alpha(0.24) {
		t.Errorf("常态底 = %v，应为 --glass-bg 白 .24", base.BG)
	}
	if base.Border != ui.Hex("#ffffff").Alpha(0.55) {
		t.Errorf("常态描边 = %v，应为 --glass-border 白 .55", base.Border)
	}
	if base.Inset != ui.Hex("#ffffff").Alpha(0.90) {
		t.Errorf("常态内高光 = %v，应为 --glass-highlight 白 .9", base.Inset)
	}
	if base.BotIn != ui.Hex("#0f172a").Alpha(0.04) {
		t.Errorf("常态底部内暗线 = %v，应为墨 .04", base.BotIn)
	}
	if base.Shadow != ui.Hex("#0f172a").Alpha(0.08) || base.ShadowY != 6 || base.ShadowBlur != 16 {
		t.Errorf("常态投影 = %+v，应为 0 6px 16px 墨 .08", base)
	}
	if base.Scale != 1 {
		t.Errorf("常态缩放 = %v，应为 1", base.Scale)
	}

	hover := drawerClosePaintFor(true, false)
	if hover.BG != ui.Hex("#ffffff").Alpha(0.38) {
		t.Errorf("悬停底 = %v，应为 --glass-bg-hover 白 .38", hover.BG)
	}
	if hover.BotIn != ui.Hex("#0f172a").Alpha(0.05) {
		t.Errorf("悬停底部内暗线 = %v，应为墨 .05", hover.BotIn)
	}
	if hover.Shadow != ui.Hex("#0f172a").Alpha(0.10) || hover.ShadowY != 8 || hover.ShadowBlur != 20 {
		t.Errorf("悬停投影 = %+v，应为 0 8px 20px 墨 .10", hover)
	}

	pressed := drawerClosePaintFor(true, true)
	if pressed.BG != ui.Hex("#ffffff").Alpha(0.16) {
		t.Errorf("按下底 = %v，应为 --glass-bg-active 白 .16", pressed.BG)
	}
	if pressed.Scale != 0.9 {
		t.Errorf("按下缩放 = %v，应为 0.9", pressed.Scale)
	}
	// 原版 :active 只覆写 background 与 transform，按下沿用悬停投影。
	if pressed.Shadow != ui.Hex("#0f172a").Alpha(0.10) {
		t.Errorf("按下投影应沿用悬停 = 墨 .10，实际 %v", pressed.Shadow)
	}
}

// TestDrawerCloseButtonSurfaceWired 无头反证：把 drawerCloseSurface 换成
// no-op，关闭键矩形内应有一片像素随之变化（常驻材质真的接在关闭键上，
// 不再只在悬停时出现）。
func TestDrawerCloseButtonSurfaceWired(t *testing.T) {
	orig := drawerCloseSurface
	defer func() { drawerCloseSurface = orig }()

	render := func(stub bool) (*image.RGBA, ui.Rect) {
		if stub {
			drawerCloseSurface = func(b ui.Element, _, _ bool) ui.Element { return b }
		} else {
			drawerCloseSurface = orig
		}
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = DrawerSearch
		a.Query = "关键词"
		tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 680))
		r, ok := tt.Find("关闭")
		if !ok {
			t.Fatal("抽屉表头里找不到关闭键")
		}
		return tt.Image(), r
	}

	on, rect := render(false)
	off, _ := render(true)

	changed := 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if rgbAt(on, x, y) != rgbAt(off, x, y) {
				changed++
			}
		}
	}
	t.Logf("关掉关闭键材质后变化 %d 像素", changed)
	if changed < 100 {
		t.Errorf("关闭键常驻材质没接上：只变了 %d 个像素", changed)
	}
}

// TestCollectScrollIconSize 钉住收藏夹滚动键的箭头图标边长：原版 collectList
// 写的是 <Left size="16" theme="outline" /> / <Right size="16" .../>，原生版此前
// 画成 14px（比原版小一圈）。这里既钉常量，又用无头像素实测把 14/16 两档分开：
// 箭头是描边图形，16px 画布的墨迹像素数约是 14px 的两倍（实测 16 vs 8）。
func TestCollectScrollIconSize(t *testing.T) {
	if collectScrollIconSize != 16 {
		t.Fatalf("collectScrollIconSize = %v，应为 16（原版 <Left size=\"16\">）", collectScrollIconSize)
	}
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Drawer = "favorite"
	tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	r := mustFind(t, tst, "向右滚动")
	img := tst.Image()
	// 常态墨 #475569（亮背景上明显更暗）；数滚动键矩形内的墨迹像素。
	ink := 0
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			c := rgbAt(img, x, y)
			if c[0] < 130 && c[1] < 140 && c[2] < 150 {
				ink++
			}
		}
	}
	t.Logf("滚动键箭头墨迹像素 = %d（图标常量 %v）", ink, collectScrollIconSize)
	// 16px 画布实测 16 个墨迹像素；14px 只有 8（缩小后描边被抗锯齿抹淡）。
	if ink < 12 {
		t.Errorf("箭头墨迹像素 = %d，偏少（14px 画布只有 8）：图标应按原版 16px 画", ink)
	}
}

// TestCollectScrollBtnPaintTokens 逐值钉住收藏夹滚动键三态底色（原版
// globals.css 的 .collect-scroll-btn：常态墨 .06、:hover 浅蓝、:active 墨 .12）。
func TestCollectScrollBtnPaintTokens(t *testing.T) {
	base := collectScrollBtnPaintFor(false, false)
	if base != ui.Hex("#0f172a").Alpha(0.06) {
		t.Errorf("常态底 = %v，应为 rgba(15,23,42,.06)", base)
	}
	hover := collectScrollBtnPaintFor(true, false)
	if hover != ui.Hex("#0ea5e9").Alpha(0.14) {
		t.Errorf("悬停底 = %v，应为 rgba(14,165,233,.14) 浅蓝（此前误用了 :active 的深灰蓝）", hover)
	}
	pressed := collectScrollBtnPaintFor(true, true)
	if pressed != ui.Hex("#0f172a").Alpha(0.12) {
		t.Errorf("按下底 = %v，应为 rgba(15,23,42,.12)", pressed)
	}
	// 按下优先于悬停：仅按下时仍是深灰蓝，不走浅蓝。
	pressedNoHover := collectScrollBtnPaintFor(false, true)
	if pressedNoHover != ui.Hex("#0f172a").Alpha(0.12) {
		t.Errorf("仅按下底 = %v，应为深灰蓝 .12", pressedNoHover)
	}
}

// TestCardGridAutoLoadsAtBottom 钉住「搜索 / UP 空间滚到底会触发 LoadMore」。
// 这两个页面用户报不自动加载：搜索层是控制器的分页没接（见 main_test 的
// TestSearchPaginationWired），UP 层则依赖这份视图触发。这里用同一套卡片网格
// 把两种抽屉都跑到真正的列表底部，断言 Act.LoadMore 至少被调一次。
func TestCardGridAutoLoadsAtBottom(t *testing.T) {
	for _, drawer := range []string{DrawerSearch, DrawerUp} {
		t.Run(drawer, func(t *testing.T) {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			l := a.ListFor(drawer)
			for i := 0; i < 30; i++ {
				l.Cards = append(l.Cards, Card{
					Bvid:  fmt.Sprintf("BV%04d", i),
					Title: fmt.Sprintf("视频%d", i),
					Up:    "UP主",
				})
			}
			l.HasMore = true
			a.Drawer = drawer
			if drawer == DrawerUp {
				a.UpTab = UpTabVideos
			}
			calls := 0
			a.Act.LoadMore = func() {
				calls++
				// 模拟控制器已发出请求：后续帧不应继续重复触发。
				l.Loading = true
			}
			tst := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
			tst.Scroll(490, 400, 0, 100000)
			tst.Frame()
			if calls == 0 {
				first, last := l.Grid.Visible()
				t.Fatalf("%s 滚到底没有触发自动翻页（visible=%d,%d rows=%d）",
					drawer, first, last, (len(l.Cards)+2)/3)
			}
		})
	}
}

// TestPrefetchGateWaitsForStableWindow 钉住滚动中的封面预取节流：窗口第一帧
// 只登记、不发请求；同一窗口连续出现才真正 Bitmap；窗口一动又重新按住。
// 这是「快速滚动时不要为途经的每一屏都排请求，滚停后再按最终窗口加载」的
// 核心判据（配合 imagecache.Cancel 的在途 abort）。
func TestPrefetchGateWaitsForStableWindow(t *testing.T) {
	var g prefetchGate
	if g.stable(0, 10) {
		t.Fatal("第一次观测就放行：滚动的第一帧会立刻发请求")
	}
	if !g.stable(0, 10) {
		t.Fatal("同一窗口第二帧应放行")
	}
	if g.stable(2, 12) {
		t.Fatal("窗口变化后仍放行：快速滚动会为途经的每一屏发请求")
	}
	if !g.stable(2, 12) {
		t.Fatal("新窗口稳定后应放行")
	}

	a := NewApp(func() {})
	asked := newRecordingCache()
	a.Images = asked
	cards := make([]Card, 60)
	for i := range cards {
		cards[i] = Card{Bvid: fmt.Sprintf("BV%04d", i), Cover: fmt.Sprintf("cover%d", i)}
	}
	l := a.ListFor("feed")
	var st ui.ListState
	a.prefetchCardCovers(nil, &st, &l.prefetch, cards, 3)
	if got := asked.unique(); got != 0 {
		t.Fatalf("第一帧预取 %d 张，want 0（窗口还没稳定）", got)
	}
	a.prefetchCardCovers(nil, &st, &l.prefetch, cards, 3)
	if got := asked.unique(); got == 0 {
		t.Fatal("窗口稳定后仍没有请求封面")
	}
}

// TestDrawerHeaderCannotDragUp 钉住「抽屉表头只能往下拖关闭，不能往上拖」：
//
// 以前 Draggable 的 Bounds 只是橡胶带边界，往上拖仍会把面板拉出负偏移（用户
// 报「header 可以被鼠标按住向上拖动」）。现在 Rigid=true，到 0 就硬停住；
// 向下 0..h 仍可拖、松手仍会被 drawerSnap 甩到关闭。
func TestDrawerHeaderCannotDragUp(t *testing.T) {
	a := feedApp(30)
	tt := shellTester(t, a)

	// 抽屉面板顶在 54（800×600 下 h=546），表头在其中 54..102。取右侧空白处。
	tt.Press(600, 70)
	for i := 1; i <= 5; i++ {
		tt.Advance(10 * time.Millisecond)
		tt.Move(600, 70-float32(i)*14)
	}
	tt.Advance(10 * time.Millisecond)
	tt.Release(600, 0)
	tt.Advance(300 * time.Millisecond)
	tt.Frame()

	if a.drawerOffset.Y < -0.01 {
		t.Fatalf("抽屉向上拖后 offset.Y = %.2f（应被 Rigid 钳在 0）", a.drawerOffset.Y)
	}
	if a.Drawer != "feed" {
		t.Fatalf("向上拖不应关闭抽屉，Drawer=%q", a.Drawer)
	}
}

// TestDrawerHeaderDragDownStillCloses 是上一条的反证：Rigid 只挡住向上，
// 向下拖到底仍应按 drawerSnap 关闭（用户确认「向下拖动关闭倒是还可以」）。
func TestDrawerHeaderDragDownStillCloses(t *testing.T) {
	a := feedApp(30)
	tt := shellTester(t, a)

	tt.Press(600, 70)
	for i := 1; i <= 8; i++ {
		tt.Advance(10 * time.Millisecond)
		tt.Move(600, 70+float32(i)*60)
	}
	tt.Advance(10 * time.Millisecond)
	tt.Release(600, 550)
	tt.Advance(2 * time.Second)
	tt.Frame()

	if a.Drawer != "" {
		t.Fatalf("向下拖到底应关闭抽屉，Drawer=%q offset=%v", a.Drawer, a.drawerOffset)
	}
}

// TestListResetScroll 钉住刷新时把所有滚动状态一起复位：只清 Cards 或只
// ResetGrid，GridScroll 里的旧 Y 会让下一帧 TrackScroll 把新列表拽回原位置；
// 播放列表 / 评论还有独立的 Rows / Scroll 状态。
func TestListResetScroll(t *testing.T) {
	l := &List{}
	l.Scroll.Y = 120
	l.Grid.Key = func(int) any { return 1 }
	l.GridScroll.Y = 120
	l.Rows.Key = func(int) any { return 1 }
	l.RowsScroll.Y = 120
	l.liftHold.held = true
	l.prefetch.set = true
	l.rowsPrefetch.set = true

	l.ResetScroll()
	if l.Scroll.Y != 0 || l.GridScroll.Y != 0 || l.RowsScroll.Y != 0 {
		t.Fatalf("滚动偏移没复位：Scroll=%.0f Grid=%.0f Rows=%.0f", l.Scroll.Y, l.GridScroll.Y, l.RowsScroll.Y)
	}
	if l.Grid.Key != nil || l.Rows.Key != nil {
		t.Error("虚拟列表行锚点没复位（Grid/Rows）")
	}
	if l.liftHold.held || l.prefetch.set || l.rowsPrefetch.set {
		t.Error("滚动锁 / 预取闸门没复位")
	}
}

// TestResetDanmakuScroll 钉住弹幕 / 评论抽屉的独立虚拟列表也会随刷新复位
// （它们不在 List.Grid/Rows 里）。
func TestResetDanmakuScroll(t *testing.T) {
	a := NewApp(func() {})
	a.danmakuList.Key = func(int) any { return 1 }
	a.repliesList.Key = func(int) any { return 1 }
	a.danmakuScrollIdx = 7
	a.ResetDanmakuScroll()
	if a.danmakuList.Key != nil || a.repliesList.Key != nil || a.danmakuScrollIdx != -1 {
		t.Error("ResetDanmakuScroll 没有复位弹幕/评论列表与跟随下标")
	}
}

// TestSetBackgroundPurgesBitmaps 钉住「隐藏/最小化时释放界面位图」：
// Background=true 后清空 shownCoverBmp 并调用 Images.Purge；恢复时复位唱片
// 旋转基准，避免把后台停转的时间当成一次 seek。
func TestSetBackgroundPurgesBitmaps(t *testing.T) {
	a := NewApp(func() {})
	rec := newRecordingCache()
	a.Images = rec
	a.shownCoverBmp = &ui.Bitmap{}

	a.SetBackground(true)
	if !a.Background || a.shownCoverBmp != nil {
		t.Fatal("进入后台应清掉 shownCoverBmp 并置 Background")
	}
	rec.mu.Lock()
	purged := rec.purged
	rec.mu.Unlock()
	if purged != 1 {
		t.Fatalf("进入后台应调用 Images.Purge 一次，实际 %d", purged)
	}

	a.SetBackground(false)
	if a.Background {
		t.Fatal("SetBackground(false) 没恢复 Background")
	}
}

// TestAutoLoadLatchedUntilNextScroll 钉住「一次到底只翻一页」的闩锁：
//
// maxRetainedCards 封顶后，追加下一页会从头部裁掉同样多，总行数不变，列表
// 仍停在底部——没有闩锁的话 canAutoLoadMore 每帧都为真，会连续翻页、内容在
// 用户脚下快速替换（八九页后的「闪烁」）。只有用户重新开始一次滚动手势
// （Scrolling false→true）才清闩锁。
func TestAutoLoadLatchedUntilNextScroll(t *testing.T) {
	a := feedApp(300)
	l := a.ListFor("feed")
	l.HasMore = true
	calls := 0
	a.Act.LoadMore = func() { calls++ }
	tst := shellTester(t, a)

	tst.Scroll(400, 300, 0, 100000)
	tst.Frame()
	if calls != 1 {
		t.Fatalf("滚到底第一次应翻一页，calls=%d", calls)
	}
	// 不追加内容、不置 Loading：没有闩锁的话这里会每帧重调。
	for i := 0; i < 6; i++ {
		tst.Frame()
	}
	if calls != 1 {
		t.Fatalf("到底后未重新滚动却连续翻页：calls=%d，应为 1", calls)
	}

	// 新的一次滚动手势：先离开底部再回来。
	tst.Scroll(400, 300, 0, -200)
	tst.Frame()
	tst.Scroll(400, 300, 0, 100000)
	tst.Frame()
	if calls != 2 {
		t.Fatalf("重新滚动到底应再翻一页，calls=%d，应为 2", calls)
	}
}

// TestCardElementKeyStableAcrossIndexShift 钉住卡片 key 不随 index 变化：
// 封顶滑窗每翻一页都会从头部裁掉一页，同一个 Bvid 的 index 变了；key 带
// index 会让整屏卡片被当成新元素重新挂载（闪烁/重排）。
func TestCardElementKeyStableAcrossIndexShift(t *testing.T) {
	a := cardElementKey(5, "BV1xx")
	b := cardElementKey(125, "BV1xx")
	if a != b {
		t.Fatalf("同一 Bvid 在不同 index 的 key 不同：%q vs %q", a, b)
	}
	if cardElementKey(7, "") == cardElementKey(8, "") {
		t.Fatal("没有 Bvid 的占位卡片应退回 index 作 key")
	}
}

// TestCardGridHeadTrimKeepsScrolling 钉住「封顶裁头后仍能继续向下翻页」：
//
// setCards 封顶后会从头部释放卡片；若裁掉的行数与追加的不对齐，旧首行的
// Bvid 会落到新行的中间，ui.List 按行键找不到锚点，视口继续钉在底部——
// 用户看到「滚到底只加载一页，必须上滚再下滚才再加载一页」。setCards 现在
// 把裁头量对齐到 2/3 列的公倍数，并由 List.Trimmed 让网格在下一帧把视口
// 上移同样的行数。这里的反证：不消费 Trimmed（或裁头不对齐）时视口仍停在
// 底部，测试红。
func TestCardGridHeadTrimKeepsScrolling(t *testing.T) {
	a := feedApp(128)
	l := a.ListFor("feed")
	l.HasMore = true
	calls, pending := 0, false
	a.Act.LoadMore = func() { calls++; pending = true }
	tst := shellTester(t, a)

	// 模拟「网络响应在两帧之间到达」：先滚到底触发加载，再追加一页并从
	// 头部释放 24 条（6 的倍数，保持 2/3 列下行首对齐）。
	apply := func() {
		if !pending {
			return
		}
		pending = false
		start := len(l.Cards)
		for i := 0; i < 24; i++ {
			l.Cards = append(l.Cards, Card{Bvid: fmt.Sprintf("NEW%04d", start+i)})
		}
		l.Cards = l.Cards[24:]
		l.Trimmed += 24
		l.Dropped = true
	}

	tst.Scroll(400, 300, 0, 100000)
	tst.Frame()
	if calls != 1 {
		t.Fatalf("滚到底应翻一页，calls=%d", calls)
	}
	apply()
	for i := 0; i < 3; i++ {
		tst.Frame()
	}
	first, last := l.Grid.Visible()
	rows := (len(l.Cards) + 2) / 3
	if last >= rows-1 {
		t.Fatalf("裁头后视口仍钉在底部：visible=%d..%d rows=%d（应上移到原内容处）", first, last, rows)
	}
	if l.Trimmed != 0 {
		t.Fatalf("Trimmed 没被消费：%d", l.Trimmed)
	}

	// 不向上滚，仅继续向下滚到底：应能再翻一页（旧实现闩锁会挡住）。
	tst.Scroll(400, 300, 0, 100000)
	tst.Frame()
	if calls != 2 {
		t.Fatalf("继续下滑到底应再翻一页，calls=%d（闩锁没有松开）", calls)
	}
}
