package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（框架行高硬底：单行元素用显式盒高 + 字形位移复现原版行高）的测试。
//
// 原版 CSS 里多处单行文本给了**比自然行盒矮**的 line-height（.c-cover-duration
// 1.2、.video-kicker 1、#video-owner-name 1.25、.mini-part 1.2、.nav-badge 1），
// 框架把每行高夹在 max(行高, ascent+descent) 之上、压不下去，于是原生版这些
// 元素比原版高一截，把周围布局推偏。修法见 linebox.go。

// TestTextLineBox 钉住换算：盒高 = 原版行盒，shift = 负 half-leading
// （(原版行盒 − 自然行盒)/2）。
func TestTextLineBox(t *testing.T) {
	cases := []struct {
		size, lh, wantH, wantShift float32
	}{
		{11, 1.2, 13.2, -1.65},    // .c-cover-duration
		{10, 1, 10, -2.5},         // .video-kicker
		{15, 1.25, 18.75, -1.875}, // #video-owner-name
		{12, 1.2, 14.4, -1.8},     // .mini-part
	}
	for _, tc := range cases {
		h, sh := textLineBox(tc.size, tc.lh)
		if math.Abs(float64(h-tc.wantH)) > 1e-4 {
			t.Errorf("textLineBox(%v,%v) 盒高 = %v，want %v", tc.size, tc.lh, h, tc.wantH)
		}
		if math.Abs(float64(sh-tc.wantShift)) > 1e-4 {
			t.Errorf("textLineBox(%v,%v) 位移 = %v，want %v", tc.size, tc.lh, sh, tc.wantShift)
		}
	}
	// 原版行盒不低于自然行盒时不需要上移（shift ≥ 0）。
	if _, sh := textLineBox(22, 1.5); sh < 0 {
		t.Errorf("line-height 1.5 不低于自然行盒，位移不应为负：%v", sh)
	}
}

// TestCoverDurationLineBox 真实跑一遍列表抽屉，量时长角标的盒高：原版
// .c-cover-duration 是 `font-size:11px; line-height:1.2; padding:2px 5px`
// ⇒ 2 + 13.2 + 2 = 17.2。此前用自然行盒（11px ≈ 16.5）⇒ 20.5，比原版高
// 约 3px、盖住更多封面。
func TestCoverDurationLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "标题", Cover: "", Duration: "1:23:45"}}
	a.Drawer = "feed"
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 900, 700)
	tt.Frame()
	r, ok := tt.Find("1:23:45")
	if !ok {
		t.Fatal("找不到时长角标")
	}
	const wantH = 13.2 // 原版 .c-cover-duration：line-height 1.2 × 11px
	t.Logf("时长角标 rect = %+v（高 %v，want %v）", r, r.H, wantH+4)
	if math.Abs(float64(r.H-(wantH+4))) > 0.01 {
		t.Errorf("时长角标盒高 %v，want %v（原版 .c-cover-duration line-height 1.2 + padding 4）", r.H, wantH+4)
	}
}

// TestVideoOwnerLineBox 真实跑一遍主区，量「UP 主」Kicker 与 UP 主名字的盒高：
// 原版 .video-kicker 10px/line-height 1 ⇒ 10、#video-owner-name 15px/1.25
// ⇒ 18.75。此前分别是 15 / 23（自然行盒），整块比原版高约 9px、把两行挤开。
func TestVideoOwnerLineBox(t *testing.T) {
	a := descApp()
	a.Info.OwnerName = "测试UP主名字"
	tt := shellTesterSize(t, a, 1000, 700)
	k, ok := tt.Find("UP 主")
	if !ok {
		t.Fatal("找不到 Kicker")
	}
	const wantK = 10 // 原版 .video-kicker：line-height 1 × 10px
	if math.Abs(float64(k.H-wantK)) > 0.01 {
		t.Errorf("Kicker 盒高 %v，want %v（.video-kicker line-height 1）", k.H, wantK)
	}
	n, ok := tt.Find("测试UP主名字")
	if !ok {
		t.Fatal("找不到 UP 主名字")
	}
	const wantN = 18.75 // 原版 #video-owner-name：line-height 1.25 × 15px
	if math.Abs(float64(n.H-wantN)) > 0.01 {
		t.Errorf("名字盒高 %v，want %v（#video-owner-name line-height 1.25）", n.H, wantN)
	}
	// 两行贴得更近：名字盒顶 ≈ Kicker 盒底 + 1px（.video-kicker 的 margin-bottom）。
	// 名字在 owner-name-row（flex）里，行高可能被按钮自身抬出零点几像素，
	// 这里只要求「贴得很近」（此前是 4.6px 的固定间距）。
	gap := n.Y - (k.Y + k.H)
	t.Logf("Kicker %+v / 名字 %+v / 行距 %v", k, n, gap)
	if gap < 0.5 || gap > 2.5 {
		t.Errorf("Kicker 与名字的行距 %v，应在 1（.video-kicker margin-bottom:1px）附近", gap)
	}
}

// TestMiniPartLineBox 真实跑一遍迷你窗，量分集行的盒高：原版 .mini-part
// 12px/line-height 1.2 ⇒ 14.4。此前是 18（自然行盒），信息列（Justify Center）
// 因此整体偏上。
func TestMiniPartLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Part: "P1 分集", Up: "UP"}
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tt.Frame()
	r, ok := tt.Find("P1 分集")
	if !ok {
		t.Fatal("找不到分集标题")
	}
	_, _ = r, ok
	const wantMH = 14.4 // 原版 .mini-part：line-height 1.2 × 12px
	t.Logf("分集行 rect = %+v（高 %v，want %v）", r, r.H, wantMH)
	if math.Abs(float64(r.H-wantMH)) > 0.01 {
		t.Errorf("分集行盒高 %v，want %v（.mini-part line-height 1.2）", r.H, wantMH)
	}
}

// TestNavBadgeLineBox 真实跑一遍主区，量播放列表角标的几何：原版 .nav-badge
// 是 `height:14px; min-width:14px; line-height:1; padding:0 3px`。此前用自然
// 行盒 + `Padding(1,4)`，角标比原版高约 1.5px。
func TestNavBadgeLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Playlist = make([]PlayItem, 0, 7)
	for i := 0; i < 7; i++ {
		a.Playlist = append(a.Playlist, PlayItem{ID: string(rune('a' + i)), Part: "P", Title: "T"})
	}
	a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题", Part: "P1", Up: "UP"}
	tt := shellTesterSize(t, a, 1000, 700)
	r, ok := tt.Find("播放列表数量")
	if !ok {
		t.Skip("找不到角标（Find 不认 Label）")
	}
	t.Logf("角标 rect = %+v", r)
	if r.H != 14 {
		t.Errorf("角标盒高 %v，want 14（.nav-badge height:14px）", r.H)
	}
	if r.W < 14 {
		t.Errorf("角标宽 %v，want ≥14（.nav-badge min-width:14px）", r.W)
	}
}
