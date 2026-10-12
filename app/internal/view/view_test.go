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
	// 0 值不显示（原版空值不渲染）。
	if got := statText(info, func(i *Info) int64 { return i.Coin }); got != "" {
		t.Errorf("coin = %q, want empty", got)
	}
	if got := statText(nil, func(i *Info) int64 { return i.Like }); got != "" {
		t.Errorf("nil info = %q, want empty", got)
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
