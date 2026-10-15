package view

import (
	"math"
	"testing"
)

// 本轮（播放列表行的行盒对齐）的测试。
//
// 原版 playlist.tsx 的记录行是两行 Tailwind 字号类：首行分集名 text-sm
// （14px / 20px 行高）、次行视频名 text-xs（12px / 16px 行高）。框架的自然
// 行盒是 22 / 18（实测字体度量），文字列因此是 40 而非原版的 36，每行比原版高
// 4px（行高 54→58，行距 58→62）。用显式盒高 + 字形上移压回（linebox.go 的
// textSmLH / textXsLH），与卡片页脚同一套手法。

// TestPlaylistRowLineBox 真实跑一遍播放列表抽屉，量两行文本的盒高：分集名
// 14/20 ⇒ 20、视频名 12/16 ⇒ 16。此前分别是 22 / 18（自然行盒）。
func TestPlaylistRowLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: "P1", Title: "T1"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = -1 // 无当前项，排除蓝色播放图标对首行盒高的干扰
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	part, ok := tt.Find("P0")
	if !ok {
		t.Fatal("找不到分集名 P0")
	}
	if math.Abs(float64(part.H-20)) > 0.01 {
		t.Errorf("分集名盒高 %v，want 20（text-sm 14/20）", part.H)
	}
	title, ok := tt.Find("T0")
	if !ok {
		t.Fatal("找不到视频名 T0")
	}
	if math.Abs(float64(title.H-16)) > 0.01 {
		t.Errorf("视频名盒高 %v，want 16（text-xs 12/16）", title.H)
	}
	// 两行紧贴：视频名盒顶 = 分集名盒底（原版两个 <p> 之间没有间距）。
	if gap := title.Y - (part.Y + part.H); math.Abs(float64(gap)) > 0.5 {
		t.Errorf("分集名与视频名的间距 %v，应为 0", gap)
	}
}

// TestPlaylistRowHeight 用两行的纵向间距反推行高：文字列压回 36 后，行高
// = max(36, 封面 36) + p-2(16) + 1px 边框×2 = 54，加列表 gap-1(4) 即相邻两行
// 文字顶相距 58。若退回自然行盒（文字列 40），行高 58、间距 62，本测试红。
func TestPlaylistRowHeight(t *testing.T) {
	a := NewApp(func() {})
	a.Playlist = []PlayItem{
		{ID: "a", Part: "P0", Title: "T0"},
		{ID: "b", Part: "P1", Title: "T1"},
	}
	a.PlayingPlaylist = ListUser
	a.Index = -1
	a.Drawer = DrawerPlaylist
	tt := shellTester(t, a)

	p0, ok := tt.Find("P0")
	if !ok {
		t.Fatal("找不到分集名 P0")
	}
	p1, ok := tt.Find("P1")
	if !ok {
		t.Fatal("找不到分集名 P1")
	}
	// 相邻两行的文字顶相距 = 行高 + gap(4)。文字列在行内垂直居中，两行同构，
	// 所以行首到文字顶的偏移相同、相减即得行高 + gap。
	delta := p1.Y - p0.Y
	const want = 54 + 4 // 行高 54（含 1px 边框）+ List gap 4
	if math.Abs(float64(delta-want)) > 0.5 {
		t.Errorf("相邻两行分集名相距 %v，want %v（行高 54 + gap 4）", delta, want)
	}
}
