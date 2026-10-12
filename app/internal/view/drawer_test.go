package view

import (
	"fmt"
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

// TestDrawerMaskFade 钉住遮罩换算到 ui.Pose.Fade 的**方向**：Pose.Fade 与
// Opacity 相反（1 = 全透明、0 = 不透明），所以不能把 drawerFade（1 = 不透明）
// 直接当 Pose.Fade 用——那样会把整个抽屉面板一起涂成透明度 0（抽屉看不见）。
func TestDrawerMaskFade(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		offsetY  float32
		entering bool
		want     float32 // Pose.Fade：0 = 不透明
	}{
		{0, false, 0}, // 贴底：遮罩完全不透明 → Fade 0
		{200, false, 0.5},
		{h, false, 1}, // 拖到底：遮罩全透明 → Fade 1
		{h + 50, false, 1},
		{0, true, 1}, // 入场第一帧：从全透明起步，再由 AnimatePose 渐显
	}
	for _, c := range cases {
		if got := drawerMaskFade(c.offsetY, h, c.entering); got != c.want {
			t.Errorf("drawerMaskFade(%v, %v, %v) = %v, want %v",
				c.offsetY, h, c.entering, got, c.want)
		}
	}
	// 与 drawerFade 互补：maskFade + fade == 1（入场帧除外）。
	for _, y := range []float32{0, 100, 250, h} {
		if got := drawerMaskFade(y, h, false); got != 1-drawerFade(y, h) {
			t.Errorf("y=%v: maskFade=%v, 应为 1-fade=%v", y, got, 1-drawerFade(y, h))
		}
	}
}

// TestDrawerEnterStep 钉住抽屉入场的判定——入场动画消失的回归点。
//
// 点击打开抽屉的那一帧，`Clicked()` 会当场置 consumed，框架把整个视图重建
// 第二趟（input.go）。面板 pose 只在帧末 commit 时采样一次，所以第二趟若把
// 「已经不是新抽屉」当成立场结束，就会把起始 pose 从 h 改成 0，采到的是 0，
// 面板首帧即到位——动画没了。drawerEnterStep 用「同一帧内 c.Now() 不变」
// 识别第二趟，让起始 pose 在整个入场帧里保持 h。
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
	// 下一帧：入场结束，pose 目标交给 AnimatePose 从 h 弹回 0。
	if entering, _, _ := drawerEnterStep("feed", key, at, t1); entering {
		t.Fatal("下一帧不该再是入场帧（否则永远停在底部）")
	}
	// 换抽屉（或关掉后重开另一个）：重新入场。
	if entering, k, a := drawerEnterStep("parts", key, at, t1); !entering || k != "parts" || !a.Equal(t1) {
		t.Fatalf("换抽屉 = (%v, %q, %v), want (true, parts, t1)", entering, k, a)
	}
}

// TestDrawerExitDone 钉住退场收尾的时机：退场起始的那一帧不能收尾（刚设的
// 起始 pose 还没被 commit 采样，立刻收尾等于没动画），动效停稳且隔了一帧
// 之后才把抽屉真正移出视图。
func TestDrawerExitDone(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	t1 := t0.Add(16 * time.Millisecond)

	if drawerExitDone(t0, t0, false) {
		t.Error("退场起步帧不该收尾：起始 pose 还没被采样")
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

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tst.Frame()

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
}

// TestDrawerBackdropLayers 锁住抽屉打开时一帧的 backdrop 玻璃层数上界。
//
// 为什么是硬约束：mygo 对每个 `Backdrop: true` 的效果都要单独断一次渲染
// 通道 + 回读下采样 + 两次模糊（gpu/gpu.go:412、gpu/metal/metal.go:830），
// 层数直接乘进每帧——这正是抽屉滚动/入场卡顿的根因（见 .pending-bugs.md）。
// 原版 CSS 里真有 backdrop-filter 的只剩搜索药丸（.home-searchbar::before）
// 与播放栏（#player .player-controls）两层，所以上界是 2；抽展面板、
// 播放键、氛围光都已改成填充/烤进位图的模糊（原版那三层就不是
// backdrop-filter）。再多就是有人又给卡片/按钮盖了真玻璃。
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
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	// 等入场动画停稳（入场按真实时钟走），这样接下来的一帧是稳态一帧。
	for i := 0; i < 30; i++ {
		time.Sleep(20 * time.Millisecond)
		tst.Frame()
	}
	BackdropLayers() // 丢掉轮询期的计数，只量稳定后的这一帧
	tst.Frame()
	if got := BackdropLayers(); got > 2 {
		t.Fatalf("抽屉打开时 backdrop 层数 = %d, want <= 2（多了会让每帧多出渲染通道打断 + 回读 + 两次模糊）", got)
	}
}

// TestDrawerStartPose 钉住入/退场的起手 pose 与「要不要请框架再建一帧」。
//
// 入场要 AnimationFrame：起手帧把面板摆到 h，请框架再建一帧，下一帧目标才
// 变成 0，AnimatePose 从 h 弹回 0。
//
// 退场**绝不能**要 AnimationFrame：`Context.AnimationFrame` 会把帧标成
// 「整个视图要重建」（它会置 rt.animating），而退场期间 `a.drawerClosing`
// 每帧都为真——一旦每帧都请帧，退场就会每帧重建整个抽屉视图（实测无头退场
// 每帧 passes=1、avg present 897ms）。退场只设目标，交给 AnimatePose 走
// 只重绘的 repose 路径。
func TestDrawerStartPose(t *testing.T) {
	const h float32 = 400
	cases := []struct {
		entering, closing bool
		wantY             float32
		wantFrame         bool
	}{
		{true, false, h, true},   // 入场：摆到底部外 + 请帧
		{false, true, h, false},  // 退场：目标设到 h，但不请帧（关键）
		{false, false, 0, false}, // 稳定在贴底
	}
	for _, c := range cases {
		y, frame := drawerStartPose(c.entering, c.closing, h)
		if y != c.wantY || frame != c.wantFrame {
			t.Errorf("drawerStartPose(%v, %v, %v) = (%v, %v), want (%v, %v)",
				c.entering, c.closing, h, y, frame, c.wantY, c.wantFrame)
		}
	}
}

// TestDrawerExitKeepsPanel 钉住「退场有过渡、不瞬间消失」：关掉抽屉后的
// 第一帧，面板还要继续渲染（下滑淡出），而不是当即被移出视图。之前退场
// 有一个更严重的 bug——退场每帧都请 `AnimationFrame`，导致每帧重建整个
// 抽屉视图（见 drawerStartPose 注释）；这条测试至少锁住「不瞬间消失」。
func TestDrawerExitKeepsPanel(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	l.Cards = make([]Card, 6)
	for i := range l.Cards {
		l.Cards[i] = Card{Bvid: fmt.Sprintf("BV%02d", i), Title: fmt.Sprintf("标题%d", i)}
	}
	a.Drawer = "feed"

	// 小窗口让每帧足够快：退场动画按真实时钟走，settle 的 20 帧远达不到
	// 动效时长，所以帧末面板仍在过渡中。
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 500, 400)
	tst.Frame()
	if !tst.HasText("标题0") {
		t.Fatal("抽屉打开时应有卡片")
	}
	a.Drawer = ""
	tst.Frame()
	if !tst.HasText("标题0") {
		t.Fatal("关抽屉后第一帧面板就消失（退场过渡丢失）")
	}
}

// TestDrawerExitState 钉住退场的状态收尾：退场一开始就清掉 drawerOpenKey
// （否则同一抽屉关掉后马上重开会没有入场动画——退场走只重绘路径后，收尾会
// 晚一帧，见 drawerStartPose），并且面板停稳后最终真的被移出视图
// （drawerShown 清空），不会一直留在场景里。
func TestDrawerExitState(t *testing.T) {
	a := NewApp(func() {})
	l := a.ListFor("feed")
	l.Cards = []Card{{Bvid: "BV1", Title: "标题"}}
	a.Drawer = "feed"

	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 500, 400)
	tst.Frame()

	a.Drawer = ""
	tst.Frame()
	if a.drawerOpenKey != "" {
		t.Fatal("退场开始后 drawerOpenKey 应为空，否则同一抽屉重开没有入场动画")
	}
	// 再跑几帧，退场应停稳并收尾（drawerShown 清空）。动效按真实时钟走：
	// ui.Snappy 的弹簧约 500ms 的程、~1.1s 停稳，每帧之间等一下让它真的停下。
	for i := 0; i < 20 && a.drawerShown != ""; i++ {
		time.Sleep(80 * time.Millisecond)
		tst.Frame()
	}
	if a.drawerShown != "" {
		t.Fatal("退场停稳后没有收尾，面板会一直留在场景里")
	}
}
