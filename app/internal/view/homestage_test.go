package view

import (
	"testing"
)

// TestHomeStageHeightMatchesOriginal 钉住主区的高度口径：
// 主区 = 窗高 - 播放栏，**不减标题栏**。
//
// 原版 `.app-main { height: calc(100vh - 36px) }` 同时带 flex-grow: 1，
// 作为 `.app-shell`（h-screen 的 flex 列）的唯一子元素被撑到整窗 —— 于是
// `.home-stage { height: calc(100% - 56px) }` 的 100% 等于 100vh，主区高
// 100vh - 56（600 窗 = 544），标题栏那 36px 由主区往播放栏底下探进去消化
// （播放栏是 position: fixed，不占文档流）。
//
// 之前原生版把播放栏也摆在流里，主区只剩 600 - 36 - 56 = 508，右栏那块
// 366 高的固定盒纵向居中后就整块高了 18px —— 用户实测「UP 主离搜索框太近、
// 右栏组件位置不对」正是这 18px。
//
// 原版 800×600 截图的实测反推：标题首行墨迹顶 = 214（行高 1.18 × 22 ≈ 26，
// 第二行 239），#video-title 顶 = 210 ⇒ 盒顶 = 210 - 2(内边距) - 66(头部) = 142；
// 而 (544 - 20(主区上内边距) - 46(搜索药丸) - 32(now-playing 底距) - 366) / 2 = 40，
// 盒顶 = 36(标题栏) + 20 + 46 + 40 = 142。两边对上，证明主区就是 544。
func TestHomeStageHeightMatchesOriginal(t *testing.T) {
	if PlayerBarHeight != 56 {
		t.Fatalf("播放栏高 = %d, want 56（#player height）", PlayerBarHeight)
	}
	if got := float32(600) - PlayerBarHeight; got != 544 {
		t.Fatalf("600 窗主区高 = %v, want 544（原版 home-stage = 100vh - 56）", got)
	}
}

// TestVideoInfoBlockSitsWhereOriginalDoes 无头整窗实测右栏落点：800×600 下
//
//	UP 主行（头像盒）top = 146   （盒顶 142 + 内边距 2 + 头像 margin 2）
//	标题 top            = 210   （盒顶 142 + 2 + 头部 66）
//	工具条「搜索结果」top = 448   （内容自上排：66+52+14+46+12+36+14+34+14+12+3）
//
// 三个数字都与原版截图 / CSS 逐值推出来的位置一致。任何一处把主区高度口径改回
// 「窗高 - 标题栏 - 播放栏」都会让它们整体上移 18px。
func TestVideoInfoBlockSitsWhereOriginalDoes(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "短标题", Desc: "无描述"}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "短标题", Part: "P1"}
	a.Queue = []Track{*a.Track}
	a.Index = 0
	tt := shellTesterSize(t, a, 800, 600)

	cases := []struct {
		label string
		wantY float32
	}{
		{"UP 主空间", 146}, // UP 主行（头像）
		{"短标题", 210},    // #video-title（单行标题时 min-height 52）
		{"搜索结果", 448},   // .video-context-dock 第一颗键
	}
	for _, c := range cases {
		r, ok := tt.Find(c.label)
		if !ok {
			t.Fatalf("找不到 %q", c.label)
		}
		if r.Y != c.wantY {
			t.Errorf("%s top = %v, want %v（原版位置）", c.label, r.Y, c.wantY)
		}
	}

	// 播放栏仍贴底（把它移出文档流不能让它跑位）：800×600 的播放栏占 544..600，
	// 进度条垂直居中 ⇒ 中心 ≈ 572。
	bar, ok := tt.Find("播放进度")
	if !ok {
		t.Fatal("找不到播放进度条")
	}
	if cy := bar.Y + bar.H/2; cy < 545 || cy > 600 {
		t.Errorf("进度条中心 y = %v，不在贴底的播放栏里（544..600）", cy)
	}
}
