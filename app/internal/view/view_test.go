package view

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPeriodAt(t *testing.T) {
	cases := map[int]string{
		0:  "midnight",
		2:  "midnight",
		3:  "predawn",
		6:  "dawn",
		12: "noon",
		23: "latenight",
		24: "midnight", // 超出的小时按取模处理
		-1: "latenight",
	}
	for h, want := range cases {
		if got := PeriodAt(h).Name; got != want {
			t.Errorf("PeriodAt(%d) = %q, want %q", h, got, want)
		}
	}
}

func TestBackdropBands(t *testing.T) {
	p := PeriodAt(12)
	bands := p.BackdropBands(12)
	if len(bands) != 12 {
		t.Fatalf("bands = %d, want 12", len(bands))
	}
	// 相邻带的接缝颜色应一致（否则背景会有可见拼接）。
	for i := 1; i < len(bands); i++ {
		if bands[i][0] != bands[i-1][1] {
			t.Errorf("band %d seam mismatch", i)
		}
	}
}

func TestFmtTime(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{0, "0:00"},
		{-5, "0:00"},
		{5, "0:05"},
		{65, "1:05"},
		{3600, "1:00:00"},
		{3725, "1:02:05"},
	}
	for _, c := range cases {
		if got := fmtTime(c.in); got != c.want {
			t.Errorf("fmtTime(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCompactCount(t *testing.T) {
	// 与原版 formatCompactCount 一致：≥1亿 x.x亿，≥100万 整数 w，≥1万 x.x w。
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{10000, "1.0w"},
		{42100, "4.2w"},
		{1234567, "123w"},
		{123456789, "1.2亿"},
	}
	for _, c := range cases {
		if got := compactCount(c.in); got != c.want {
			t.Errorf("compactCount(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStatText(t *testing.T) {
	info := &Info{Like: 3956, Coin: 0, Favorite: 6120}
	if got := statText(info, func(i *Info) int64 { return i.Like }); got != "3956" {
		t.Errorf("like = %q", got)
	}
	// 0 值要显示成 "0"（原版 formatCompactCount(stat?.coin ?? 0)）——
	// 不能拿「计数为空」当禁用判据，否则 0 币的视频投币键会被锁死。
	if got := statText(info, func(i *Info) int64 { return i.Coin }); got != "0" {
		t.Errorf("coin = %q, want \"0\"", got)
	}
	// 没有视频时原版也是 "0"（stat 未定义 → ?? 0）。
	if got := statText(nil, func(i *Info) int64 { return i.Like }); got != "0" {
		t.Errorf("nil info = %q, want \"0\"", got)
	}
}

// TestPlaylistBadgeInk 钉住角标底色：只有正在放合集才是紫色，其余是原版
// .nav-badge 的玫红（之前默认给了蓝色）。
func TestPlaylistBadgeInk(t *testing.T) {
	a := &App{Theme: Theme{Rose: ui.Hex("#e11d48")}}
	if got := a.playlistBadgeInk(); got != ui.Hex("#e11d48").Alpha(0.72) {
		t.Errorf("默认角标色 = %v，应为玫红 72%%", got)
	}
	a.PlayingPlaylist = ListUser
	if got := a.playlistBadgeInk(); got != ui.Hex("#e11d48").Alpha(0.72) {
		t.Errorf("我的列表角标色 = %v，应为玫红", got)
	}
	a.PlayingPlaylist = ListSeries
	if got := a.playlistBadgeInk(); got != ui.Hex("#a855f7") {
		t.Errorf("合集角标色 = %v，应为紫", got)
	}
}

// TestPlaylistBadgeCount 角标数字来源：正在放合集用合集列表长度，其余用
// 我的列表长度（原版 playingPlaylistType === "series" ? series : user）。
func TestPlaylistBadgeCount(t *testing.T) {
	a := &App{Playlist: []PlayItem{{ID: "u1"}, {ID: "u2"}}, SeriesPlaylist: []PlayItem{{ID: "s1"}}}
	if got := a.playlistBadgeCount(); got != 2 {
		t.Errorf("默认角标数 = %d，want 2", got)
	}
	if got := a.playlistBadgeText(); got != "2" {
		t.Errorf("默认角标文案 = %q，want 2", got)
	}
	a.PlayingPlaylist = ListSeries
	if got := a.playlistBadgeCount(); got != 1 {
		t.Errorf("合集角标数 = %d，want 1", got)
	}
}

func TestSplitLines(t *testing.T) {
	got := splitLines("a\nb\n\nc")
	want := []string{"a", "b", "", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitLines len = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAvatarInitial(t *testing.T) {
	if got := avatarInitial("音你所乐"); got != "音" {
		t.Errorf("avatarInitial = %q", got)
	}
	if got := avatarInitial(""); got != "?" {
		t.Errorf("empty avatarInitial = %q", got)
	}
}

func TestPlaylistActive(t *testing.T) {
	a := &App{
		Playlist:       []PlayItem{{ID: "u1"}},
		SeriesPlaylist: []PlayItem{{ID: "s1"}},
	}
	if got := a.playlistActive(ListUser); len(got) != 1 || got[0].ID != "u1" {
		t.Errorf("user list = %v", got)
	}
	if got := a.playlistActive(ListSeries); len(got) != 1 || got[0].ID != "s1" {
		t.Errorf("series list = %v", got)
	}
}

func TestPlaylistHas(t *testing.T) {
	a := &App{Playlist: []PlayItem{{Cid: 111}, {Cid: 222}}}
	if !a.playlistHas(111) || !a.playlistHas(222) {
		t.Error("playlistHas should find existing cids")
	}
	if a.playlistHas(333) {
		t.Error("playlistHas should not find missing cid")
	}
}

func TestSponsorDotColor(t *testing.T) {
	if sponsorDotColor("ok") != ui.Hex("#22c55e") {
		t.Error("ok 应为绿色")
	}
	if sponsorDotColor("error") != ui.Hex("#ef4444") {
		t.Error("error 应为红色")
	}
	if sponsorDotColor("loading") != ui.Hex("#f59e0b") {
		t.Error("loading 应为黄色")
	}
	if sponsorDotColor("off") != ui.Hex("#94a3b8") || sponsorDotColor("empty") != ui.Hex("#94a3b8") {
		t.Error("off/empty 应为灰色")
	}
}

// TestHasSource 钉住播放栏的禁用判据：原版是 `disabled={!src}`（还没有任何
// 曲目时播放/音量/倍速/均衡整排禁用），Go 侧对应「队列里没有当前曲目」。
func TestHasSource(t *testing.T) {
	empty := &App{}
	if empty.hasSource() {
		t.Error("空队列不应有 source")
	}
	// Index 越界（刚清空队列、还没选曲）也不该算有 source。
	oob := &App{Queue: []Track{{Cid: 1}}, Index: 3}
	if oob.hasSource() {
		t.Error("Index 越界不应有 source")
	}
	set := &App{Queue: []Track{{Cid: 1}}, Index: 0}
	if !set.hasSource() {
		t.Error("队列里有当前曲目就应有 source")
	}
}

// TestPlayerBarTips 钉住播放栏控件的文案（原版 main:src/components/player.tsx
// 的 aria-label / title）。之前这里是另一套文案：下一集 / 倍速 / 均衡 /
// 跳过赞助片段，与原版都对不上。
func TestPlayerBarTips(t *testing.T) {
	if got := playTip(true); got != "暂停" {
		t.Errorf("playTip(true) = %q, want 暂停", got)
	}
	if got := playTip(false); got != "播放" {
		t.Errorf("playTip(false) = %q, want 播放", got)
	}
	// 原版是「下一个视频」，不是「下一集」。
	if got := nextTip(); got != "下一个视频" {
		t.Errorf("nextTip() = %q, want 下一个视频", got)
	}
	// 倍速用原始数值（1 → "播放速度 1 倍"），不是 1.0x 的显示标签。
	speed := map[float64]string{
		1:    "播放速度 1 倍",
		0.75: "播放速度 0.75 倍",
		1.25: "播放速度 1.25 倍",
		3:    "播放速度 3 倍",
	}
	for rate, want := range speed {
		if got := speedTip(rate); got != want {
			t.Errorf("speedTip(%v) = %q, want %q", rate, got, want)
		}
	}
	if got := eqTip(true); got != "音量均衡: 开" {
		t.Errorf("eqTip(true) = %q", got)
	}
	if got := eqTip(false); got != "音量均衡: 关" {
		t.Errorf("eqTip(false) = %q", got)
	}
	if got := sponsorTip(); got != "自动跳过恰饭片段（SponsorBlock）" {
		t.Errorf("sponsorTip() = %q", got)
	}
}

// TestSponsorStatusTitle 钉住状态点 tooltip 的拼接（原版 SPONSOR_STATUS_LABEL
// 与 `label [+ " (N 段)"] [+ ": " + reason]`）。
func TestSponsorStatusTitle(t *testing.T) {
	cases := []struct {
		status string
		count  int
		reason string
		want   string
	}{
		{"off", 0, "", "SponsorBlock 未启用"},
		{"loading", 0, "", "SponsorBlock 查询中…"},
		{"ok", 3, "", "SponsorBlock 已加载 (3 段)"},
		{"ok", 0, "", "SponsorBlock 已加载 (0 段)"}, // ok 一定带段数
		{"empty", 0, "", "SponsorBlock 无片段"},
		{"error", 0, "sponsorblock 返回 500", "SponsorBlock 查询失败: sponsorblock 返回 500"},
		// 非 ok 状态即使带 count 也不显示段数。
		{"empty", 7, "", "SponsorBlock 无片段"},
		// 未知状态按「未启用」处理，不能渲染成空 tooltip。
		{"", 0, "", "SponsorBlock 未启用"},
	}
	for _, c := range cases {
		if got := sponsorStatusTitle(c.status, c.count, c.reason); got != c.want {
			t.Errorf("sponsorStatusTitle(%q, %d, %q) = %q, want %q",
				c.status, c.count, c.reason, got, c.want)
		}
	}
}

func TestNeedsLogin(t *testing.T) {
	a := &App{Drawer: "history"}
	if !a.needsLogin() {
		t.Error("history without login should need login")
	}
	a.LoggedIn = true
	if a.needsLogin() {
		t.Error("history with login should not need login")
	}
	a = &App{Drawer: "popular", RecTab: RecHot}
	if a.needsLogin() {
		t.Error("hot does not need login")
	}
	a.RecTab = RecRecommend
	if !a.needsLogin() {
		t.Error("recommend without login should need login")
	}
}

// TestProgressPlan 钉住进度条的交接不变量——这块反复出过问题：
// 拖动失效、松手跳回原地、双进度条、seek 期间弹回。
func TestProgressPlan(t *testing.T) {
	const settle = 250 * time.Millisecond
	const pos = 10.0   // 播放位置
	const want = 120.0 // 用户拖到的目标

	t.Run("拖动中不跟随也不 seek", func(t *testing.T) {
		seek, next := progressPlan(progressInput{held: true, pos: pos}, want, true, settle)
		if seek {
			t.Fatal("拖动中不应提交 seek")
		}
		if next != want {
			t.Fatalf("拖动中滑块应停在手上 %v，得到 %v", want, next)
		}
	})

	t.Run("松手当帧用用户的值 seek，且不被播放位置覆盖", func(t *testing.T) {
		seek, next := progressPlan(progressInput{held: false, pos: pos, settle: 0}, want, true, settle)
		if !seek {
			t.Fatal("松手应提交一次 seek")
		}
		if next != want {
			t.Fatalf("松手后应停在目标 %v，得到 %v（跟随把值抹掉了）", want, next)
		}
	})

	t.Run("seek 在跑时不跟随", func(t *testing.T) {
		seek, next := progressPlan(progressInput{held: false, busy: true, pos: pos, settle: time.Second}, want, false, settle)
		if seek || next != want {
			t.Fatalf("seek 中不应 seek/跟随，得到 seek=%v next=%v", seek, next)
		}
	})

	t.Run("刚过交接点不跟随", func(t *testing.T) {
		_, next := progressPlan(progressInput{held: false, pos: pos, settle: settle / 2}, want, false, settle)
		if next != want {
			t.Fatalf("交接窗口内不应跟随，得到 %v", next)
		}
	})

	t.Run("稳定后跟随播放位置", func(t *testing.T) {
		seek, next := progressPlan(progressInput{held: false, pos: pos, settle: time.Second}, want, false, settle)
		if seek {
			t.Fatal("稳定后不应 seek")
		}
		if next != pos {
			t.Fatalf("稳定后应跟随播放位置 %v，得到 %v", pos, next)
		}
	})

	t.Run("键盘改值提交 seek", func(t *testing.T) {
		seek, _ := progressPlan(progressInput{changed: true, pos: pos, settle: time.Second}, want, false, settle)
		if !seek {
			t.Fatal("键盘方向键改值应提交 seek")
		}
	})

	t.Run("正常播放帧不改值也不 seek", func(t *testing.T) {
		seek, next := progressPlan(progressInput{pos: pos, settle: time.Second}, pos, false, settle)
		if seek || next != pos {
			t.Fatalf("正常帧应为空操作，得到 seek=%v next=%v", seek, next)
		}
	})
}

func TestFanCount(t *testing.T) {
	cases := map[int64]string{0: "0", 123: "123", 9999: "9999", 10000: "1.0万", 12345: "1.2万", 123456: "12.3万"}
	for in, want := range cases {
		if got := fanCount(in); got != want {
			t.Errorf("fanCount(%d) = %q, want %q", in, got, want)
		}
	}
}

// TestBufferedFrac 钉住进度条缓冲段的换算与夹取（原版 --player-buffered）。
func TestBufferedFrac(t *testing.T) {
	cases := []struct {
		name     string
		buffered float64
		dur      float64
		want     float32
	}{
		{"未开始", 0, 100, 0},
		{"时长为零", 10, 0, 0},
		{"负值当未缓冲", -5, 100, 0},
		{"正常比例", 25, 100, 0.25},
		{"超过总时长夹到 1", 130, 100, 1},
	}
	for _, c := range cases {
		if got := bufferedFrac(c.buffered, c.dur); got != c.want {
			t.Errorf("%s：bufferedFrac(%v, %v) = %v, want %v", c.name, c.buffered, c.dur, got, c.want)
		}
	}
}

// TestDiscAdvance 钉住唱片随播放位置变化的方向语义：正常播放按挂钟匀速；
// 快进（正向跳变）正转多一点；快退（反向跳变）倒转；小抖动不当 seek。
func TestDiscAdvance(t *testing.T) {
	const period = 22.0 // discSpinPeriod
	wantNormal := float32(0.5 / period * 360)

	if got := discAdvance(0.5, 0.5, 1); got != wantNormal {
		t.Errorf("正常播放 = %v, want %v（匀速）", got, wantNormal)
	}
	// 倍速：位置涨得跟挂钟 × speed 一致，仍是匀速、不算 seek。
	if got := discAdvance(0.5, 1.0, 2); got != wantNormal {
		t.Errorf("2x 正常播放 = %v, want %v", got, wantNormal)
	}
	// 快进：明显的正增量（比正常多）。
	fwd := discAdvance(0.016, 30, 1)
	if fwd <= wantNormal || fwd <= 0 {
		t.Errorf("快进增量 = %v, 应明显大于正常 %v", fwd, wantNormal)
	}
	// 快退：负增量（倒转）。
	back := discAdvance(0.016, -30, 1)
	if back >= 0 {
		t.Errorf("快退增量 = %v, 应为负（倒转）", back)
	}
	// 小位移（抖动 / 上报误差）不当 seek。
	if got := discAdvance(0.5, 0.6, 1); got != wantNormal {
		t.Errorf("小位移不该当 seek：%v, want %v", got, wantNormal)
	}
	// 大跨度 seek 封顶（不把唱片甩好几圈）：补偿部分封顶 180°，再加上
	// 本帧正常的匀速增量。
	capDeg := discAdvance(1, 1, 1) + 180
	if got := discAdvance(1, 100000, 1); got > capDeg+0.001 {
		t.Errorf("大跨度 seek = %v, 应封顶到 %v", got, capDeg)
	}
}

// TestAdvanceDiscContinuous 钉住角度不取模：以前每转满一圈就 -=360，
// 帧间目标突然倒退一整圈，看起来就是「唱片在倒转」。
func TestAdvanceDiscContinuous(t *testing.T) {
	a := &App{Playing: true, Speed: 1}
	base := time.Unix(0, 0)
	a.advanceDisc(base) // 建立基准
	a.Pos = 22
	a.advanceDisc(base.Add(22 * time.Second))
	if a.discAngle < 359 || a.discAngle > 361 {
		t.Fatalf("22s 后角度 = %v, want ~360（不能取模回 0）", a.discAngle)
	}
	a.Pos = 23
	a.advanceDisc(base.Add(23 * time.Second))
	if a.discAngle < 375 || a.discAngle > 377 {
		t.Fatalf("23s 后角度 = %v, want ~376（一直累加）", a.discAngle)
	}
}

// TestCoverToggleActive 钉住封面三个小开关的激活态判据：原版 CSS 只给
// 「封面背景」（.cover-background-toggle）与「高级质感」（.cover-premium-toggle）
// 定义了 is-active；模式切换键（.cover-mode-toggle）从来没有 is-active。
// 之前统一传 true，模式键被误画成了蓝色激活态。
func TestCoverToggleActive(t *testing.T) {
	cases := []struct {
		key              string
		ambient, premium bool
		want             bool
	}{
		{"cover-mode", true, true, false},
		{"cover-mode", false, false, false},
		{"ambient", true, false, true},
		{"ambient", false, false, false},
		{"premium", false, true, true},
		{"premium", true, false, false},
	}
	for _, c := range cases {
		if got := coverToggleActive(c.key, c.ambient, c.premium); got != c.want {
			t.Errorf("coverToggleActive(%q, %v, %v) = %v, want %v",
				c.key, c.ambient, c.premium, got, c.want)
		}
	}
}

// TestCoverToggleStyle 钉住封面小开关的底色/边色/图标色（原版
// .cover-mode-toggle 的三条规则）。关键点：激活态**不改图标色**（原版 is-active
// 只改底色与描边，图标亮度仍由 hover 决定）；激活底色是 .28 而不是旧值 .45。
func TestCoverToggleStyle(t *testing.T) {
	eq := func(got, want ui.Color) bool {
		return got.R == want.R && got.G == want.G && got.B == want.B && got.A == want.A
	}
	// 激活：蓝底 .28 + 浅蓝边 .45，图标色仍 .70（不因激活变亮）。
	bg, border, ink := coverToggleStyle(true, false)
	if !eq(bg, ui.Hex("#0ea5e9").Alpha(0.28)) {
		t.Errorf("激活底色 = %+v, want %+v", bg, ui.Hex("#0ea5e9").Alpha(0.28))
	}
	if !eq(border, ui.Hex("#7dd3fc").Alpha(0.45)) {
		t.Errorf("激活边色 = %+v, want %+v", border, ui.Hex("#7dd3fc").Alpha(0.45))
	}
	if !eq(ink, ui.Hex("#ffffff").Alpha(0.70)) {
		t.Errorf("激活图标色 = %+v, want %+v（is-active 不改图标色）", ink, ui.Hex("#ffffff").Alpha(0.70))
	}
	// 激活 + 悬停：is-active 规则在 CSS 里更靠后，底色/描边仍是激活态；
	// 只有图标色由 hover 规则抬高到 .95。
	bg, border, ink = coverToggleStyle(true, true)
	if !eq(bg, ui.Hex("#0ea5e9").Alpha(0.28)) {
		t.Errorf("激活+悬停底色 = %+v, want %+v（is-active 盖过 hover）", bg, ui.Hex("#0ea5e9").Alpha(0.28))
	}
	if !eq(border, ui.Hex("#7dd3fc").Alpha(0.45)) {
		t.Errorf("激活+悬停边色 = %+v, want %+v", border, ui.Hex("#7dd3fc").Alpha(0.45))
	}
	if !eq(ink, ui.Hex("#ffffff").Alpha(0.95)) {
		t.Errorf("激活+悬停图标色 = %+v, want %+v", ink, ui.Hex("#ffffff").Alpha(0.95))
	}
	// 未激活：深底 .35、白边 .25、图标 .70。
	bg, border, ink = coverToggleStyle(false, false)
	if !eq(bg, ui.Hex("#0f172a").Alpha(0.35)) || !eq(border, ui.Hex("#ffffff").Alpha(0.25)) ||
		!eq(ink, ui.Hex("#ffffff").Alpha(0.70)) {
		t.Errorf("默认态 = (%+v, %+v, %+v)，want (%+v, %+v, %+v)",
			bg, border, ink,
			ui.Hex("#0f172a").Alpha(0.35), ui.Hex("#ffffff").Alpha(0.25), ui.Hex("#ffffff").Alpha(0.70))
	}
}

// TestIncognitoTip 钉住历史抽屉隐身开关的 tooltip 随开关态切换（原版
// historyList 的 Tooltip content）。之前无论开关都写着「开启后…」。
func TestIncognitoTip(t *testing.T) {
	if got := incognitoTip(true); got != "关闭后恢复云端播放记录与进度同步" {
		t.Errorf("隐身开启时的 tooltip = %q", got)
	}
	if got := incognitoTip(false); got != "开启后不读取或上报云端播放记录与进度" {
		t.Errorf("隐身关闭时的 tooltip = %q", got)
	}
}

// TestHistoryRefreshLabel 钉住历史抽屉刷新键的文案随 tab 变（原版 aria-label）。
func TestHistoryRefreshLabel(t *testing.T) {
	if got := historyRefreshLabel(HistHistory); got != "刷新历史记录" {
		t.Errorf("观看历史 tab 的刷新文案 = %q, want 刷新历史记录", got)
	}
	if got := historyRefreshLabel(HistWatchLater); got != "刷新稍后再看" {
		t.Errorf("稍后再看 tab 的刷新文案 = %q, want 刷新稍后再看", got)
	}
}

// TestPopoverClosesOnOutsideClick 钉住音量/倍速弹层的「点外部关闭」：
// 原版 player.tsx 在 document 上监听 mousedown，指针落在弹层外就关掉弹层；
// 原生版此前只有「再点一次同一个按钮」与 Esc 两条关闭路径，点别处弹层会一直浮着，
// 遮住播放栏、与下面的界面叠加。
func TestPopoverClosesOnOutsideClick(t *testing.T) {
	a := NewApp(func() {})
	// 音量/倍速键在「没有可播放视频」时是禁用态（原版 disabled={!src}），
	// 先给一条队列让它们可点。
	a.Queue = []Track{{Bvid: "BV1test", Title: "标题"}}
	a.Index = 0
	tt := shellTester(t, a)

	// 开：点音量键。（这一下也顺带验证打开它的那一次点击不会被捕获层吞掉、
	// 导致弹层刚开就自己关掉。）
	if err := tt.Click("音量"); err != nil {
		t.Fatalf("点不到音量键：%v", err)
	}
	tt.Frame()
	if !a.ShowVolume {
		t.Fatal("点音量键后弹层没打开，测试前提不成立")
	}

	// 点主区左上角（弹层外，窗口 800×600）：弹层应关闭。
	tt.ClickAt(100, 280)
	tt.Frame()
	if a.ShowVolume {
		t.Errorf("点弹层外部后音量弹层还开着（原版 document mousedown 会关）")
	}

	// 点在弹层内部（音量滑块中央）不该误关：
	// 主窗音量弹层 = Right(200) Bottom(7)、184×42 → 中心约 (508, 572)。
	a.ShowVolume = true
	tt.Frame()
	tt.ClickAt(550, 572)
	tt.Frame()
	if !a.ShowVolume {
		t.Errorf("点在弹层内部（滑块）把弹层关了，不该关")
	}

	// 倍速弹层同样：开、点外部关。
	a.ShowSpeed = true
	a.ShowVolume = false
	tt.Frame()
	if !a.ShowSpeed {
		t.Fatal("倍速弹层没打开，测试前提不成立")
	}
	tt.ClickAt(100, 280)
	tt.Frame()
	if a.ShowSpeed {
		t.Errorf("点弹层外部后倍速弹层还开着")
	}
}

// TestPopoverKeepsHoverAlive 钉住「弹层开着时整窗的悬停仍然有效」。
//
// 之前的「点弹层外关闭」是靠一块盖满窗口的捕获层（popoverScrim）实现的 ——
// 它是普通元素、参与命中测试，弹层开着时指针下最顶的就是它，于是标题栏、
// 卡片、进度条…整窗的悬停全部失灵（只有弹层自己不在此列）。原版只用 document
// 的 pointerdown 判定，hover 不受影响。现在改用框架的 PressedOutside
// （只读这一帧的按下记录、不参与命中测试），悬停恢复。
//
// 判据：音量弹层开着时，悬停进度条仍要浮出与关闭时同一道蓝环（采样点与
// TestPlayerBarTimelineHoverRing 相同）。
func TestPopoverKeepsHoverAlive(t *testing.T) {
	_, tst := playerBarProbe(t, func(a *App) { a.ShowVolume = true })
	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	tr := rangeTrackRect(r, timelineGeom)
	// 轨道上沿外侧 2px：静息是播放栏玻璃，悬停时落在 2px 蓝环里。
	x, y := int(tr.X+tr.W*0.25), int(tr.Y-2)
	rest := rgbAt(tst.Image(), x, y)
	tst.Move(r.X+r.W*0.25, r.Y+r.H/2)
	tst.Frame()
	hover := rgbAt(tst.Image(), x, y)
	restBR, hoverBR := rest[2]-rest[0], hover[2]-hover[0]
	t.Logf("弹层开着：轨道上方 静息=%v(B-R=%d) 悬停=%v(B-R=%d)", rest, restBR, hover, hoverBR)
	if hoverBR-restBR < 8 {
		t.Errorf("弹层开着时进度条的悬停被吃掉了：B-R %d → %d", restBR, hoverBR)
	}
}

// TestUserButtonLabels 钉住搜索药丸最右端头像键的文案（原版 searchForm）：
// 未登录是「登录账号」/「点击登录账号」，已登录是「切换登录账号」/
// 「点击登录其他账号」。之前原生版写的是「扫码登录」/「已登录 <名字>」。
func TestUserButtonLabels(t *testing.T) {
	a := NewApp(func() {})
	logins := 0
	a.Act.Login = func() { logins++ }
	tt := shellTester(t, a)
	if err := tt.Click("登录账号"); err != nil {
		t.Fatalf("未登录时找不到「登录账号」按钮：%v", err)
	}
	tt.Frame()
	if logins != 1 {
		t.Fatalf("点未登录头像：logins=%d，想 1", logins)
	}

	b := NewApp(func() {})
	b.LoggedIn = true
	b.UName = "某人"
	logins = 0
	b.Act.Login = func() { logins++ }
	tt2 := shellTester(t, b)
	if err := tt2.Click("切换登录账号"); err != nil {
		t.Fatalf("已登录时找不到「切换登录账号」按钮：%v", err)
	}
	tt2.Frame()
	if logins != 1 {
		t.Fatalf("点已登录头像：logins=%d，想 1", logins)
	}
}
