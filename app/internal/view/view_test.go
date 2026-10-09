package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

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
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{999, "999"},
		{10000, "1.0万"},
		{42100, "4.2万"},
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

func TestFmtBytes(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
	}
	for _, c := range cases {
		if got := fmtBytes(c.in); got != c.want {
			t.Errorf("fmtBytes(%d) = %q, want %q", c.in, got, c.want)
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
