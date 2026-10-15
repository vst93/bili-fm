package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（卡片页脚对齐）的测试。
//
// 原版卡片页脚 `[data-slot="footer"]` 有两条此前没对齐的取值：
//   - 洗色 `background: rgba(255,255,255,.22)`（悬停随 --hover-strength=1 涨到
//     `.22 + 1×.14 = .36`）；原生版误写成 `.12`（漏了一位），页脚暗一档。
//   - 标题是 HeroUI 的 text-small（14px / 20px 行高），meta 行是 Tailwind 的
//     text-xs（12px / 16px 行高）；框架的自然行盒是 22 / 18（≈1.5× 字号），
//     页脚因此比原版高 4px，每张卡都偏高。用显式盒高 + 字形上移压回（linebox.go）。
// 带标题前胶囊（feed 的「充电专属」）时胶囊也不再单独把标题行顶高 6px。

func TestCardFooterBG(t *testing.T) {
	if got := cardFooterBG(false); got != ui.Hex("#ffffff").Alpha(0.22) {
		t.Errorf("常态页脚洗色 = %v，应为白 .22", got)
	}
	if got := cardFooterBG(true); got != ui.Hex("#ffffff").Alpha(0.36) {
		t.Errorf("悬停页脚洗色 = %v，应为白 .36（.22 + --hover-strength 1×.14）", got)
	}
}

// TestCardFooterWashPixel 是无头渲染实测：把一张卡放在纯灰 90 的底上，页脚
// 内部（空处）的合成色 = 90×(1−.24)×(1−.22) + 255×(1−(1−.24)×(1−.22)) ≈ 157。
// 若页脚洗色退回 .12，同一点只有 ≈145（相差 12），本测试会红。
func TestCardFooterWashPixel(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	spec := cardSpec{
		Key:    "footer-card",
		Title:  "标题",
		Fields: []cardField{{Kind: fieldAuthor, Text: "UP主"}},
	}
	tst := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.RGB(90, 90, 90))
		ui.Box(c).Absolute().Left(20).Top(20).Size(220, 170).Children(func() {
			a.renderCard(c, spec)
		})
	}, 300, 260)
	tst.Frame()
	img := tst.Image()
	// 卡在 (20,20) 220×170：封面占上 100，页脚从 y=120 起；y=180 落在页脚
	// 空处（标题/ meta 之下、下内边距之上），不会采到文字。
	got := rgbAt(img, 120, 180)
	const want = 157
	if math.Abs(float64(got[0]-want)) > 2 || got[0] != got[1] || got[1] != got[2] {
		t.Errorf("页脚洗色像素 = %v，应为中性灰 ≈%d（白 .22 合成）", got, want)
	}
}

// TestCardFooterLineBox 真实跑一遍列表抽屉，量标题与 meta 行的盒高：原版标题
// text-small 14/20 ⇒ 20，meta text-xs 12/16 ⇒ 16。此前分别是 22 / 18（自然行盒），
// 页脚比原版高 4px。
func TestCardFooterLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "标题标题", Up: "UP主名字"}}
	a.Drawer = "feed"
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tt.Frame()

	title, ok := tt.Find("标题标题")
	if !ok {
		t.Fatal("找不到卡片标题")
	}
	if math.Abs(float64(title.H-20)) > 0.01 {
		t.Errorf("标题盒高 %v，want 20（text-small 14/20）", title.H)
	}
	meta, ok := tt.Find("UP主名字")
	if !ok {
		t.Fatal("找不到 meta 行的作者名")
	}
	if math.Abs(float64(meta.H-16)) > 0.01 {
		t.Errorf("meta 行盒高 %v，want 16（text-xs 12/16）", meta.H)
	}
	// 标题在页脚内、meta 在标题下：meta 盒顶 = 标题盒底 + 4（原版 mt-1）。
	if gap := meta.Y - (title.Y + title.H); math.Abs(float64(gap-4)) > 0.5 {
		t.Errorf("标题与 meta 的间距 %v，应为 4（mt-1）", gap)
	}
}

// TestCardTitleBadgeLineBox 钉住带标题前胶囊的卡片：胶囊与标题在同一行盒里
// （原版胶囊是 <b> 内的 inline span，padding 不影响行高），胶囊不再单独把标题
// 行顶高 6px（此前 26 vs 标题 22）。
func TestCardTitleBadgeLineBox(t *testing.T) {
	a := NewApp(func() {})
	a.ListFor("feed").Cards = []Card{{Bvid: "BV1", Title: "充电视频", Badge: "充电专属", Up: "UP主"}}
	a.Drawer = "feed"
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tt.Frame()

	badge, ok := tt.Find("充电专属")
	if !ok {
		t.Fatal("找不到标题前胶囊")
	}
	if math.Abs(float64(badge.H-20)) > 0.01 {
		t.Errorf("胶囊盒高 %v，want 20（与标题同一行盒）", badge.H)
	}
	title, ok := tt.Find("充电视频")
	if !ok {
		t.Fatal("找不到标题")
	}
	if math.Abs(float64(title.H-20)) > 0.01 {
		t.Errorf("标题盒高 %v，want 20", title.H)
	}
	if math.Abs(float64(badge.Y-title.Y)) > 0.5 {
		t.Errorf("胶囊与标题不在同一行：胶囊 Y=%v 标题 Y=%v", badge.Y, title.Y)
	}
}
