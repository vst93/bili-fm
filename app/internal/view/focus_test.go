package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestFocusRingGeometries 钉住每一条 `:focus-visible` 描边环的取值（颜色 /
// 宽度 / offset）与 focusRingRect 的几何换算。
//
// 取值全部取自原版 globals.css：
//
//	#player .player-button / #video-owner-name   2px solid var(--studio-blue)   offset 2
//	.home-user-button                            2px solid var(--studio-blue)   offset 3
//	.video-owner-picker-item                     2px solid var(--studio-blue)   offset -2
//	.top-tool-btn                                2px solid rgba(14,165,233,.45) offset 2
//	.watchlater-remove-btn                       2px solid rgba(14,165,233,.62) offset 2
//	.history-incognito-switch                    2px solid rgba(14,165,233,.44) offset 2
//	.app-title-bar-btn / .collect-scroll-btn     0 0 0 2px rgba(2,132,199,.4)   offset 0
func TestFocusRingGeometries(t *testing.T) {
	for _, c := range []struct {
		name       string
		ring       focusRing
		wantColor  ui.Color
		wantWidth  float32
		wantOffset float32
	}{
		{"player-button 系", focusStudio, ui.Hex("#0284c7"), 2, 2},
		{"home-user-button", focusStudioWide, ui.Hex("#0284c7"), 2, 3},
		{"合作 UP 主条目", focusStudioInset, ui.Hex("#0284c7"), 2, -2},
		{"top-tool-btn", focusSky45, ui.Hex("#0ea5e9").Alpha(0.45), 2, 2},
		{"稍后再看移除键", focusSky62, ui.Hex("#0ea5e9").Alpha(0.62), 2, 2},
		{"隐身开关", focusSky44, ui.Hex("#0ea5e9").Alpha(0.44), 2, 2},
		{"标题栏/迷你窗/滚动键", focusGlow, ui.Hex("#0284c7").Alpha(0.40), 2, 0},
	} {
		if c.ring.Color != c.wantColor {
			t.Errorf("%s：颜色 = %v，应为 %v", c.name, c.ring.Color, c.wantColor)
		}
		if c.ring.Width != c.wantWidth {
			t.Errorf("%s：宽度 = %v，应为 %v", c.name, c.ring.Width, c.wantWidth)
		}
		if c.ring.Offset != c.wantOffset {
			t.Errorf("%s：offset = %v，应为 %v", c.name, c.ring.Offset, c.wantOffset)
		}
		// 全部现用环都只往外画或贴着盒边（offset+width ≥ 0）；真出现负的外扩量，
		// 说明 ring 取值写错（描边会被推到盒里看不见）。
		if grow := c.ring.Offset + c.ring.Width; grow < 0 {
			t.Errorf("%s：外扩量 %v < 0", c.name, grow)
		}
	}
}

// TestFocusRingRectFollowsCSSOutline 逐条手算 focusRingRect：CSS 的
// `outline-offset` 是描边内缘离盒边的距离、宽度从内缘往外量，换算到
// Painter.Stroke（贴 rect 内缘、宽 width）就是把 rect/圆角外扩 offset+width。
func TestFocusRingRectFollowsCSSOutline(t *testing.T) {
	box := ui.Rect{X: 10, Y: 20, W: 100, H: 40}
	const radius float32 = 7
	for _, c := range []struct {
		name     string
		ring     focusRing
		wantRect ui.Rect
		wantRad  float32
	}{
		// offset 2 / width 2 → 外扩 4。
		{"offset 2", focusStudio, ui.Rect{X: 6, Y: 16, W: 108, H: 48}, 11},
		// offset 3 / width 2 → 外扩 5。
		{"offset 3", focusStudioWide, ui.Rect{X: 5, Y: 15, W: 110, H: 50}, 12},
		// offset -2 / width 2 → 外扩 0：描边正好画在盒的最外圈里。
		{"offset -2", focusStudioInset, ui.Rect{X: 10, Y: 20, W: 100, H: 40}, 7},
		// box-shadow spread 2、无 offset：外扩 2。
		{"offset 0", focusGlow, ui.Rect{X: 8, Y: 18, W: 104, H: 44}, 9},
	} {
		got, gotRad := focusRingRect(box, radius, c.ring)
		if got != c.wantRect {
			t.Errorf("%s：rect = %v，应为 %v", c.name, got, c.wantRect)
		}
		if gotRad != c.wantRad {
			t.Errorf("%s：圆角 = %v，应为 %v", c.name, gotRad, c.wantRad)
		}
	}
}

// TestFocusOutlinePixels 是无头像素实测：focusOutline 在聚焦时真画出一圈，
// 且位移是原版的 `outline-offset`（不是框架默认环的「离盒 3px」），并关掉了
// 框架默认环。
//
// 几何（右侧）：盒外 2..4px 是 focusOutline 的环（offset 2 + 2px 宽），盒外
// 1..3px 是框架默认环（Painter.FocusRing 的 3px 偏移 + 2px 宽）。取盒右侧只属于
// 各自的一段做像素判据：
//
//	+3.5px（只属于我们的环）聚焦后必须是 --studio-blue #0284c7（不透明）
//	+1.5px（只属于框架默认环）聚焦后必须还是背景（默认环被关掉）
//
// 把 focusOutline 里的 `e.FocusRing(false)` 删掉，+1.5 那段就会被框架的
// rgba(37,99,235,.55) 染蓝，本测红；把外扩量改成框架的 3（`grow := ring.Width+1`）
// 则 +3.5 那段落到环外，也会红。
func TestFocusOutlinePixels(t *testing.T) {
	build := func() (*ui.Tester, ui.Rect) {
		tst := ui.NewTester(func(c *ui.Context) {
			b := ui.ButtonBase(c.Key("target")).Size(60, 30).Label("目标")
			focusOutline(b, 0, focusStudio)
		}, 160, 100)
		tst.Frame()
		r, ok := tst.Find("目标")
		if !ok {
			t.Fatal("找不到目标键")
		}
		return tst, r
	}
	// 取盒右侧、垂直居中一行的像素：+1.5px 与 +3.5px。
	probe := func(tst *ui.Tester, r ui.Rect) (inner, outer [3]int) {
		y := int(r.Y + r.H/2)
		right := int(r.X + r.W)
		return rgbAt(tst.Image(), right+1, y), rgbAt(tst.Image(), right+3, y)
	}

	off, offRect := build()
	offInner, offOuter := probe(off, offRect)

	on, r := build()
	on.Key(0, ui.KeyTab)
	on.Frame()
	if !on.Focused("目标") {
		t.Fatal("Tab 后目标键没拿到键盘焦点")
	}
	onInner, onOuter := probe(on, r)
	t.Logf("未聚焦 +1.5=%v +3.5=%v；聚焦后 +1.5=%v +3.5=%v", offInner, offOuter, onInner, onOuter)

	// +3.5 是我们环的内芯：聚焦后必须是纯 --studio-blue，且与未聚焦不同。
	want := [3]int{2, 132, 199} // #0284c7
	if onOuter != want {
		t.Errorf("聚焦后盒外 +3.5px = %v，应为 --studio-blue %v", onOuter, want)
	}
	if onOuter == offOuter {
		t.Errorf("聚焦前后盒外 +3.5px 都是 %v：环没画出来", onOuter)
	}
	// +1.5 只属于框架默认环：关掉后应保持背景不变。
	if onInner != offInner {
		t.Errorf("聚焦后盒外 +1.5px 由 %v 变成 %v：框架默认环没被关掉（focusOutline 里缺 FocusRing(false)）", offInner, onInner)
	}
}

func mustFind(t *testing.T, tst *ui.Tester, label string) ui.Rect {
	t.Helper()
	r, ok := tst.Find(label)
	if !ok {
		t.Fatalf("找不到 %q", label)
	}
	return r
}

// tabTo 反复按 Tab 直到 label 拿到键盘焦点（`:focus-visible` 只在键盘焦点下
// 为真，鼠标点击不行 —— 原版浏览器同样如此）。
func tabTo(t *testing.T, tst *ui.Tester, label string, maxTabs int) {
	t.Helper()
	if tst.Focused(label) {
		return
	}
	for i := 0; i < maxTabs; i++ {
		tst.Key(0, ui.KeyTab)
		tst.Frame()
		if tst.Focused(label) {
			return
		}
	}
	t.Fatalf("按了 %d 次 Tab 也没轮到 %q", maxTabs, label)
}

// blueAt 是像素的蓝度（B−R）：环是蓝系（#0284c7 / #0ea5e9），页背景是中性浅色。
func blueAt(img *image.RGBA, x, y int) int {
	c := rgbAt(img, x, y)
	return c[2] - c[0]
}

// TestFocusRingsOnRealControls 是整块无头实测：几处真实的 `:focus-visible`
// 接线真的画出环，且位移等于各自的 `outline-offset`（不是别处的值）。
//
// 对每个控件 Tab 到它，然后在盒子上沿外侧逐行量蓝度（B−R）。环应当只出现在
// 距上沿 `[offset, offset+width]` 那一条（像素对齐 snap 会把小数坐标取整，
// 给 ±1.5px 容差）；更外面的行必须保持中性（< 阈值）。把某处的 focusOutline
// 调用删掉、或把 offset 写成别处的值（比如把 focusSky62 换成长度 2 的
// focusGlow），窗口那几行就量不到蓝——本测红。
//
// 注意按钮聚焦会弹 tooltip（mygo 在键盘焦点下也弹），那条气泡是中性深蓝
// （B−R ≈ 29），低于所有阈值，不会误判成环。
func TestFocusRingsOnRealControls(t *testing.T) {
	miniProbe := func(t *testing.T) *ui.Tester {
		a := NewApp(func() {})
		a.Mini = true
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		a.Dur, a.Pos = 100, 10
		return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight))
	}
	historyProbe := func(watchLater bool) func(*testing.T) *ui.Tester {
		return func(t *testing.T) *ui.Tester {
			a := NewApp(func() {})
			a.Theme = ThemeFor(PeriodAt(12))
			a.ListFor("history").Cards = []Card{{Bvid: "BV1", Title: "稍后再看视频", Track: Track{Aid: 42, Bvid: "BV1"}}}
			a.Drawer = "history"
			if watchLater {
				a.HistTab = HistWatchLater
			} else {
				a.HistTab = HistHistory
			}
			return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
		}
	}
	mainProbe := func(t *testing.T) *ui.Tester {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		// 有在播的曲目：播放栏的按钮才不是禁用态（禁用的元素不进 Tab 序）。
		a.Queue = []Track{{Bvid: "BV1", Title: "标题", Up: "UP"}}
		a.Index, a.Playing, a.Dur, a.Pos = 0, true, 100, 10
		return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	}
	favoriteProbe := func(t *testing.T) *ui.Tester {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Drawer = "favorite"
		return settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720))
	}

	for _, c := range []struct {
		name    string
		build   func(*testing.T) *ui.Tester
		label   string
		offset  float32
		grow    float32
		minBlue int
		maxTabs int
	}{
		// `#player input[type="range"]:focus-visible`：offset 2 + 2px → 外扩 4。
		{"迷你窗进度条", miniProbe, "播放进度", 2, 4, 60, 10},
		// `.watchlater-remove-btn:focus-visible`：offset 2 + 2px → 外扩 4。
		{"稍后再看移除键", historyProbe(true), "从稍后再看移除", 2, 4, 60, 30},
		// `.history-incognito-switch:focus-visible`：offset 2 + 2px → 外扩 4。
		{"历史隐身开关", historyProbe(false), "隐身", 2, 4, 60, 30},
		// `.collect-scroll-btn:focus-visible` 的 `0 0 0 2px`（offset 0 + 2px）→ 外扩 2。
		{"收藏右滚键", favoriteProbe, "向右滚动", 0, 2, 40, 40},
		// `.top-tool-btn:focus-visible`：offset 2 + 2px → 外扩 4。
		{"主页工具栏键", mainProbe, "动态", 2, 4, 40, 8},
		// `.home-user-button:focus-visible`：offset 3 + 2px → 外扩 5。
		// Tab 预算放到 40：标题栏现在有若干可聚焦小键（迷你入口 / 隐身 / 自绘
		// 窗口控件），主区控件在 Tab 序里被推后（原版同样如此）。tabTo 会一直按
		// 到命中为止，给足预算即可，断言的是「环的几何」不是「按几次 Tab」。
		{"登录键", mainProbe, "登录账号", 3, 5, 60, 40},
		// `#player .player-button:focus-visible`：offset 2 + 2px → 外扩 4。
		// 用「音量均衡」工具键（标签唯一，不会撞上封面唱片那颗同样叫「播放」的键）。
		{"均衡键", mainProbe, "音量均衡: 关", 2, 4, 60, 32},
	} {
		t.Run(c.name, func(t *testing.T) {
			tst := c.build(t)
			r := mustFind(t, tst, c.label)
			tabTo(t, tst, c.label, c.maxTabs)
			img := tst.Image()

			// 沿盒上沿横排取三点（避开圆角），抵消文字/图标干扰。
			inRing, beyond := 0, 0
			for _, fx := range []float32{0.3, 0.5, 0.7} {
				x := int(r.X + r.W*fx)
				for y := int(r.Y) - int(c.grow) - 3; y <= int(r.Y)-1; y++ {
					dist := r.Y - float32(y)
					b := blueAt(img, x, y)
					if dist <= c.grow+1.5 && dist >= c.offset-1.5 {
						inRing = max(inRing, b)
					} else {
						beyond = max(beyond, b)
					}
				}
			}
			t.Logf("%s：rect=%v 环带蓝度=%d（距上沿 %.0f–%.0fpx）；环外=%d",
				c.name, r, inRing, c.offset, c.grow, beyond)

			if inRing < c.minBlue {
				t.Errorf("Tab 到 %q 后盒外 %.0f–%.0fpx 只有 %d 蓝度，环没画出来或偏移不对",
					c.label, c.offset, c.grow, inRing)
			}
			if beyond >= c.minBlue {
				t.Errorf("环带之外也有 %d 蓝度（应保持中性）：环画宽了或 offset 不对", beyond)
			}
		})
	}
}
