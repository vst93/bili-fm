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
