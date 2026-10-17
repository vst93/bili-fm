package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏进度条 / 音量条对齐）的测试。原版把这两条的轨道与滑块 CSS
// 写成同一套（globals.css 的 .player-timeline-track 与
// input.player-volume-slider::-webkit-slider-runnable-track），所以这里也按
// 「一份画法、两处使用」来钉：令牌/几何用纯函数钉死，观感用无头渲染实测，
// 每条实测都配一个会在改错时变红的判据。

// playerBarProbe 建一个 1000×700 的主窗（默认亮色时段），装一条 200 秒的曲目
// 并把进度停在正中（滑块落在轨道中央，便于按几何取样）。mutate 用来开弹层等。
func playerBarProbe(t *testing.T, mutate func(a *App)) (*App, *ui.Tester) {
	return playerBarProbeAt(t, PeriodAt(12), mutate)
}

// playerBarProbeAt 同上，但指定时段（烟熏玻璃时段用来验深色材质是否真的接上）。
func playerBarProbeAt(t *testing.T, period Period, mutate func(a *App)) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(period)
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	a.Volume = 0.5
	if mutate != nil {
		mutate(a)
	}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	return a, tst
}

// volumePopoverRect 按视图的挂法（AttachTo(音量列, AnchorLeft, AnchorRight)
// + 右边距 volumePopoverEdgeGap、垂直居中）算出音量弹层矩形：弹层右边 =
// 列左 - edgeGap、中缝对齐列中心。传音量列的矩形（无头测试里用音量键的矩形，
// 它贴在列左侧，两者左边与中线一致）。
func volumePopoverRect(anchor ui.Rect, w, h, edgeGap float32) ui.Rect {
	return ui.Rect{X: anchor.X - edgeGap - w, Y: anchor.Y + anchor.H/2 - h/2, W: w, H: h}
}

// volumeTrackRect 按视图用的同一份几何算出音量弹层里的轨道矩形 ——
// 弹层几何与内边距都是 view 里的常量，测试按同一份推导取样，改一处就够。
func volumeTrackRect(anchor ui.Rect) ui.Rect {
	panel := volumePopoverRect(anchor, volumePopoverW, volumePopoverH, volumePopoverEdgeGap)
	sliderX := panel.X + volumePopoverPadL + volumeMuteSize + volumePopoverGap
	sliderW := float32(volumePopoverW - volumePopoverPadL - volumePopoverPadR - volumeMuteSize - volumePopoverGap)
	return rangeTrackRect(ui.Rect{X: sliderX, Y: panel.Y, W: sliderW, H: volumePopoverH}, volumeGeom)
}

// TestRangeColorsForTheme 钉住轨道那两个令牌随时段的取值（原版
// --player-track-border / --player-track-rest）：亮色时段白 .68 / 白 .32，
// 五个烟熏玻璃时段换成白 .28 / 白 .20。
func TestRangeColorsForTheme(t *testing.T) {
	light := rangeColorsFor(ThemeFor(PeriodAt(12)))
	if light.Border != ui.Hex("#ffffff").Alpha(0.68) ||
		light.Rest != ui.Hex("#ffffff").Alpha(0.32) {
		t.Errorf("亮色时段轨道 = %v / %v，应为白 .68 / 白 .32", light.Border, light.Rest)
	}
	dark := rangeColorsFor(ThemeFor(PeriodAt(0)))
	if dark.Border != ui.Hex("#ffffff").Alpha(0.28) ||
		dark.Rest != ui.Hex("#ffffff").Alpha(0.20) {
		t.Errorf("烟熏玻璃时段轨道 = %v / %v，应为白 .28 / 白 .20", dark.Border, dark.Rest)
	}
	if dark.Border.A >= light.Border.A {
		t.Error("深色时段的轨道描边应比亮色时段淡")
	}
}

// TestRangeThumbScale 钉住滑块三态的 transform：常态 1、悬停 1.1、按下 1.12
// （原版 input.player-timeline::-webkit-slider-thumb 的 :hover / :active）。
func TestRangeThumbScale(t *testing.T) {
	for _, c := range []struct {
		hovered, held bool
		want          float32
	}{
		{false, false, 1},
		{true, false, 1.1},
		{false, true, 1.12},
		{true, true, 1.12}, // 按下优先
	} {
		if got := rangeThumbScale(c.hovered, c.held); got != c.want {
			t.Errorf("rangeThumbScale(%v, %v) = %v，应为 %v", c.hovered, c.held, got, c.want)
		}
	}
}

// TestRangeTrackGeometry 钉住轨道几何与滑块行程：轨道左右各内缩一个滑块
// 半径、垂直居中；滑块圆心从轨道左端走到右端（原生 range 的行为，与
// PaddingX(timelineInset) 的取值区间一致）。
func TestRangeTrackGeometry(t *testing.T) {
	r := ui.Rect{X: 100, Y: 200, W: 300, H: timelineH}
	// 进度条：轨道两端各内缩 7，滑块圆心走满轨道。
	tr := rangeTrackRect(r, timelineGeom)
	if tr.X != r.X+timelineInset || tr.W != r.W-2*timelineInset || tr.H != timelineTrackH {
		t.Errorf("轨道 = %+v，应为 x=%v w=%v h=%v", tr, r.X+timelineInset, r.W-2*timelineInset, timelineTrackH)
	}
	if tr.Y+tr.H/2 != r.Y+r.H/2 {
		t.Errorf("轨道未垂直居中：%v vs %v", tr.Y+tr.H/2, r.Y+r.H/2)
	}
	if got := rangeThumbX(tr, timelineGeom, 0); got != tr.X {
		t.Errorf("frac=0 滑块圆心 = %v，应为轨道左端 %v", got, tr.X)
	}
	if got := rangeThumbX(tr, timelineGeom, 1); got != tr.X+tr.W {
		t.Errorf("frac=1 滑块圆心 = %v，应为轨道右端 %v", got, tr.X+tr.W)
	}
	if got := rangeThumbX(tr, timelineGeom, 2); got != tr.X+tr.W {
		t.Errorf("越界 frac 应夹到右端：%v", got)
	}

	// 音量条：轨道占满输入宽度（不内缩），滑块圆心两端各内缩半个滑块 ——
	// 与进度条同一条画法，差别只在这两个几何值上。
	vt := rangeTrackRect(r, volumeGeom)
	if vt.X != r.X || vt.W != r.W {
		t.Errorf("音量条轨道 = %+v，应占满输入宽度 %+v", vt, r)
	}
	if got := rangeThumbX(vt, volumeGeom, 0); got != vt.X+timelineInset {
		t.Errorf("音量条 frac=0 滑块圆心 = %v，应为 %v", got, vt.X+timelineInset)
	}
	if got := rangeThumbX(vt, volumeGeom, 1); got != vt.X+vt.W-timelineInset {
		t.Errorf("音量条 frac=1 滑块圆心 = %v，应为 %v", got, vt.X+vt.W-timelineInset)
	}
	nan := float32(0)
	nan = nan / nan // NaN
	if got := clamp01(nan); got != 0 {
		t.Errorf("NaN 应夹到 0：%v", got)
	}
}

// rangePaintProbe 在固定底色上画一条输入条，返回整张图 —— 用来把「同一条
// 画法、不同令牌」的观感差异在受控背景上量出来。dark 决定用哪一套材质
// （见 rangeSkinFor）。
func rangePaintProbe(colors rangeColors, frac float32, hovered, dark bool) *image.RGBA {
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Box(c).Size(160, timelineH).Background(ui.Hex("#3b4a63")).Children(func() {
			b := ui.Box(c).Fill()
			b.Draw(func(p *ui.Painter, r ui.Rect) {
				paintRangeBar(p, r, rangePaint{Frac: frac, Hovered: hovered, Dark: dark, Colors: colors})
			})
		})
	}, 160, timelineH)
	tst.Frame()
	return tst.Image()
}

// TestRangeTrackUsesThemeColors 是受控背景上的实测：同一条轨道换成烟熏玻璃
// 时段的令牌后，未播部分的底色与描边都明显变淡 —— 证明 paintRangeFill 真的
// 用了传进来的 rangeColors（而不是把亮色时段的白色写死）。
func TestRangeTrackUsesThemeColors(t *testing.T) {
	cy := timelineH / 2
	x := 130 // 未播部分（frac=0.2 之外）
	light := rgbAt(rangePaintProbe(rangeColorsFor(ThemeFor(PeriodAt(12))), 0.2, false, false), x, cy)
	dark := rgbAt(rangePaintProbe(rangeColorsFor(ThemeFor(PeriodAt(0))), 0.2, false, true), x, cy)
	t.Logf("未播部分 亮色=%v 深色=%v（底色 #3b4a63）", light, dark)
	if light[0] <= dark[0] {
		t.Errorf("亮色时段的轨道底色应比深色时段亮：%v vs %v", light, dark)
	}
	if light[0]-dark[0] < 20 {
		t.Errorf("两个时段的轨道底色差得太少：%v vs %v", light, dark)
	}
}

// TestPlayerBarVolumeSliderPaint 是整窗的无头实测：音量弹层里的音量条现在
// 与进度条是同一套画法（原版两条 CSS 逐字相同）——轨道是青→蓝渐变、
// 未播部分是半透明白。框架默认的滑杆（细轨道 + 实心圆钮 + 纯色强调色）
// 画不出青色端，所以「左端 G>B」这一条就能把它区分开。
func TestPlayerBarVolumeSliderPaint(t *testing.T) {
	_, tst := playerBarProbe(t, func(a *App) { a.ShowVolume = true })
	img := tst.Image()
	// 音量键贴在音量列左侧（两者左边/中线一致），拿它当弹层的锚点算几何。
	anchor, ok := tst.Find("音量")
	if !ok {
		t.Fatal("播放栏里找不到音量键")
	}
	tr := volumeTrackRect(anchor)
	cy := int(tr.Y + tr.H/2)

	// 取值区间两端各内缩 7px（滑块半径），所以 frac 沿 tr.W 分布。
	sample := func(f float32) [3]int { return rgbAt(img, int(tr.X+tr.W*f), cy) }
	left, mid, right := sample(0.03), sample(0.3), sample(0.9)
	t.Logf("音量条 左端=%v 中段=%v 未播右端=%v（轨道 %+v）", left, mid, right, tr)

	if tr.W <= 0 || int(tr.X) <= 0 || int(tr.X+tr.W) >= 1000 {
		t.Fatalf("按几何算出的轨道不在窗口里：%+v", tr)
	}
	if left[1]-left[2] < 8 {
		t.Errorf("音量条左端 = %v，应为青色端（G>B，原版渐变起点 #2dd4bf）", left)
	}
	if mid[2]-mid[1] < 20 {
		t.Errorf("音量条中段 = %v，应已偏向蓝色端（B>G，原版渐变终点 #0284c7）", mid)
	}
	// 未播部分：原版是半透明白（--player-track-rest），压在弹层玻璃上仍然很亮。
	if right[0] < 235 || right[1] < 235 || right[2] < 235 {
		t.Errorf("音量条未播部分 = %v，应为近乎白的轨道底", right)
	}
}

// TestPlayerBarTimelineHoverRing 是整窗的无头实测：指针停在进度条上时，
// 轨道外圈要浮出原版 :hover 的 2px 蓝环（`0 0 0 2px rgba(14,165,233,.08)`）+
// 加强的外投影，移开又退回基态。
func TestPlayerBarTimelineHoverRing(t *testing.T) {
	_, tst := playerBarProbe(t, nil)
	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	tr := rangeTrackRect(r, timelineGeom)
	// 采样点：轨道上沿外侧 2px。基态这里是播放栏玻璃（外投影朝下，轨道上方
	// 没有影子），悬停时正好落在 2px 蓝环（spread 2、硬边）里。
	x, y := int(tr.X+tr.W*0.25), int(tr.Y-2)
	rest := rgbAt(tst.Image(), x, y)
	tst.Move(r.X+r.W*0.25, r.Y+r.H/2)
	tst.Frame()
	hover := rgbAt(tst.Image(), x, y)
	tst.Move(500, 400) // 移开
	tst.Frame()
	back := rgbAt(tst.Image(), x, y)

	restBR, hoverBR, backBR := rest[2]-rest[0], hover[2]-hover[0], back[2]-back[0]
	t.Logf("轨道上方 静息=%v(B-R=%d) 悬停=%v(B-R=%d) 移开=%v(B-R=%d)",
		rest, restBR, hover, hoverBR, back, backBR)
	if hoverBR-restBR < 8 {
		t.Errorf("悬停后轨道上方没有浮出蓝环：B-R %d → %d", restBR, hoverBR)
	}
	if backBR != restBR {
		t.Errorf("移开指针后轨道上方应退回基态：B-R %d → %d", restBR, backBR)
	}
}

// TestPlayerBarTimelineThumbSwells 是整窗的无头实测：指针停在条上时滑块按
// 原版放大 1.1（14 → 15.4px）并罩一圈 3px 蓝晕，所以滑块两侧原本是轨道底色的
// 像素会变成偏蓝的晕边；移开又退回。
func TestPlayerBarTimelineThumbSwells(t *testing.T) {
	_, tst := playerBarProbe(t, nil)
	r, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	tr := rangeTrackRect(r, timelineGeom)
	cx, cy := int(rangeThumbX(tr, timelineGeom, 0.5)), int(tr.Y+tr.H/2)
	// 采样点距离滑块圆心 8px（滑块半径静止 7、悬停 7.7，另有 3px 蓝晕）：
	//   * 左边落在已播渐变上（偏蓝、暗）—— 悬停时滑块长出的一圈白边会盖过来；
	//   * 右边落在未播的轨道底上（近白）—— 悬停时盖的是蓝色晕边。
	// 两侧一起就能把「滑块真的变大了 + 晕边真的画了」都钉住。
	lx, rx := cx-8, cx+8

	restL, restR := rgbAt(tst.Image(), lx, cy), rgbAt(tst.Image(), rx, cy)
	tst.Move(r.X+r.W*0.5, r.Y+r.H/2)
	tst.Advance(200 * time.Millisecond) // 推过 100ms 滑块肿胀（v0.5.5 冻结时钟）
	tst.Frame()
	hovL, hovR := rgbAt(tst.Image(), lx, cy), rgbAt(tst.Image(), rx, cy)
	t.Logf("滑块两侧 静息=%v / %v，悬停=%v / %v", restL, restR, hovL, hovR)

	// 左：红通道从「渐变蓝」跳到「滑块白边」（实测 61 → 243）。
	if hovL[0]-restL[0] < 50 {
		t.Errorf("悬停后滑块没有向外长大（左侧仍是轨道渐变）：%v → %v", restL, hovL)
	}
	// 右：晕边是蓝色（#0ea5e9 .10），B-R 应明显增大（实测 22 → 47）。
	restBR, hoverBR := restR[2]-restR[0], hovR[2]-hovR[0]
	if hoverBR-restBR < 12 {
		t.Errorf("悬停后滑块右侧没有浮出蓝晕：B-R %d → %d", restBR, hoverBR)
	}
}

// TestPlayerTimeLabelWeight 钉住时间标签的字重接线（原版
// `#player .player-time { font-weight: 600 }`）：同一串时间文字，用
// timeTextAt 画出来的墨迹明显比同字号/同色/同宽度的普通文本多。
func TestPlayerTimeLabelWeight(t *testing.T) {
	a := NewApp(func() {})
	var bold, plain ui.Rect
	tst := ui.NewTester(func(c *ui.Context) {
		ui.Row(c).Children(func() {
			ui.Box(c).Width(60).Children(func() {
				b := ui.Box(c).Width(56).Height(20)
				b.Draw(func(p *ui.Painter, r ui.Rect) { bold = r })
				b.Children(func() { a.timeText(c, 754) })
			})
			ui.Box(c).Width(60).Children(func() {
				b := ui.Box(c).Width(56).Height(20)
				b.Draw(func(p *ui.Painter, r ui.Rect) { plain = r })
				b.Children(func() {
					ui.Text(c, fmtTime(754)).FontSize(11).
						TextColor(ui.Hex("#334155").Alpha(0.72)).Width(56).Center()
				})
			})
		})
	}, 140, 24)
	tst.Frame()
	img := tst.Image()
	ink := func(r ui.Rect) int {
		sum := 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				c := rgbAt(img, x, y)
				sum += 255 - c[0]
			}
		}
		return sum
	}
	bi, pi := ink(bold), ink(plain)
	t.Logf("时间标签墨迹 600=%d 400=%d", bi, pi)
	if bi <= pi {
		t.Errorf("timeTextAt 的字重没有生效：墨迹 600=%d 普通=%d", bi, pi)
	}
}

// TestRangeSkinForTokens 逐值钉住 rangeSkinForTheme 的两套材质：亮色取
// `:root` 的字面量，深色取原版烟熏玻璃块对轨道/滑块的那组覆写；悬停只提亮
// 轨道顶内高光与滑块外影，两个时段共用同一条（深色块不覆写 :hover）。
func TestRangeSkinForTokens(t *testing.T) {
	white := ui.Hex("#ffffff")
	blue := ui.Hex("#0284c7")
	teal := ui.Hex("#0ea5e9")
	light := rangeSkinForTheme(false, false)
	dark := rangeSkinForTheme(true, false)
	hover := rangeSkinForTheme(false, true)
	darkHover := rangeSkinForTheme(true, true)

	check := func(name string, got, want ui.Color) {
		t.Helper()
		if got != want {
			t.Errorf("%s = %v，应为 %v", name, got, want)
		}
	}
	// 亮色（:root 字面量）。
	check("亮色轨道顶内高光", light.TrackTop, white.Alpha(0.88))
	check("亮色轨道底内影", light.TrackBottom, shadowInk.Alpha(0.08))
	check("亮色轨道外影", light.TrackGlow, teal.Alpha(0.10))
	check("亮色滑块描边", light.ThumbBorder, white.Alpha(0.90))
	check("亮色滑块底色", light.ThumbFill, ui.Hex("#e0f2fe").Alpha(0.62))
	check("亮色滑块高光", light.ThumbGlint, white.Alpha(0.98))
	check("亮色滑块顶内高光", light.ThumbTop, white.Alpha(0.92))
	check("亮色滑块底内影", light.ThumbBottom, blue.Alpha(0.10))
	check("亮色滑块外影", light.ThumbGlow, blue.Alpha(0.24))
	// 深色（烟熏玻璃块逐值）。
	check("深色轨道顶内高光", dark.TrackTop, white.Alpha(0.30))
	check("深色轨道底内影", dark.TrackBottom, shadowInk.Alpha(0.10))
	check("深色轨道外影", dark.TrackGlow, teal.Alpha(0.07))
	check("深色滑块描边", dark.ThumbBorder, white.Alpha(0.68))
	check("深色滑块底色", dark.ThumbFill, ui.Hex("#e0f2fe").Alpha(0.48))
	check("深色滑块高光", dark.ThumbGlint, white.Alpha(0.90))
	check("深色滑块顶内高光", dark.ThumbTop, white.Alpha(0.58))
	check("深色滑块底内影", dark.ThumbBottom, ui.Transparent)
	check("深色滑块外影", dark.ThumbGlow, blue.Alpha(0.20))
	// 悬停：提亮顶内高光 + 收底内影 + 加蓝外影，时段不参与。
	check("悬停轨道顶内高光", hover.TrackTop, white.Alpha(0.94))
	check("悬停轨道底内影", hover.TrackBottom, shadowInk.Alpha(0.08))
	check("悬停滑块外影", hover.ThumbGlow, blue.Alpha(0.28))
	check("深色+悬停轨道顶内高光", darkHover.TrackTop, white.Alpha(0.94))
	check("深色+悬停轨道底内影", darkHover.TrackBottom, shadowInk.Alpha(0.08))
	check("深色+悬停滑块外影", darkHover.ThumbGlow, blue.Alpha(0.28))

	// 单调性：深色整套比亮色淡（否则就是把亮色值抄错了）。
	if dark.TrackTop.A >= light.TrackTop.A ||
		dark.ThumbBorder.A >= light.ThumbBorder.A ||
		dark.ThumbFill.A >= light.ThumbFill.A ||
		dark.ThumbTop.A >= light.ThumbTop.A {
		t.Error("深色时段的轨道/滑块材质应整体比亮色淡")
	}
}

// TestRangeTrackDarkInset 是受控背景上实测：同一条轨道（同一份 --player-track-*
// 令牌）换成深色材质后，顶部那条内高光明显变淡 —— 证明 paintRangeInsets 真的
// 用了深色的 TrackTop（而不是仍铺亮色的白 .88）。
func TestRangeTrackDarkInset(t *testing.T) {
	track := rangeTrackRect(ui.Rect{X: 0, Y: 0, W: 160, H: timelineH}, timelineGeom)
	// frac=0.2，x=130 落在未播部分（不受青→蓝渐变影响）。
	light := rgbAt(rangePaintProbe(rangeColorsFor(ThemeFor(PeriodAt(12))), 0.2, false, false),
		130, int(track.Y)+1)
	dark := rgbAt(rangePaintProbe(rangeColorsFor(ThemeFor(PeriodAt(0))), 0.2, false, true),
		130, int(track.Y)+1)
	t.Logf("轨道顶内高光 亮色=%v 深色=%v", light, dark)
	if light[0]-dark[0] < 40 {
		t.Errorf("深色时段轨道顶内高光应明显变淡：%v vs %v", light, dark)
	}
}

// TestRangeThumbDarkMaterial 是受控背景上实测：深色材质下整颗滑块（底色 .48 /
// 高光 .78 / 描边 .68 / 外影 .2）应比亮色一档暗 —— 证明 paintRangeThumb 真的
// 用了传进来的 rangeSkin。
func TestRangeThumbDarkMaterial(t *testing.T) {
	center := ui.Rect{X: 0, Y: 0, W: 160, H: timelineH}
	track := rangeTrackRect(center, timelineGeom)
	cx, cy := int(rangeThumbX(track, timelineGeom, 0.5)), timelineH/2
	ink := func(dark bool) int {
		img := rangePaintProbe(rangeColorsFor(ThemeFor(PeriodAt(0))), 0.5, false, dark)
		sum := 0
		for y := cy - 7; y <= cy+7; y++ {
			for x := cx - 7; x <= cx+7; x++ {
				c := rgbAt(img, x, y)
				sum += c[0] + c[1] + c[2]
			}
		}
		return sum
	}
	light, dark := ink(false), ink(true)
	t.Logf("滑块区总亮度 亮色=%d 深色=%d", light, dark)
	if light-dark < 2000 {
		t.Errorf("深色时段滑块应整体暗一档：亮色=%d 深色=%d", light, dark)
	}
}

// TestPlayerBarTimelineDarkWired 是整窗的反证：深色时段渲染时把 rangeSkinFor
// 换成「永远取亮色材质」的桩，进度条顶内高光随之变亮 —— 证明 rangePaint.Dark
// 真的从 timelinePaint / 音量弹层流到了绘制层（删掉任一处 Dark: a.Theme.Dark
// 即变红）。
func TestPlayerBarTimelineDarkWired(t *testing.T) {
	topAt := func() [3]int {
		_, tst := playerBarProbeAt(t, PeriodAt(0), nil)
		r, ok := tst.Find("播放进度")
		if !ok {
			t.Fatal("播放栏里找不到进度条")
		}
		tr := rangeTrackRect(r, timelineGeom)
		// tr.X + 25% 落在已播渐变里，取轨道顶内高光那一行。
		return rgbAt(tst.Image(), int(tr.X+tr.W*0.25), int(tr.Y)+1)
	}
	dark := topAt()
	saved := rangeSkinFor
	rangeSkinFor = func(_, hovered bool) rangeSkin { return rangeSkinForTheme(false, hovered) }
	defer func() { rangeSkinFor = saved }()
	light := topAt()
	t.Logf("深色时段进度条顶内高光 深色材质=%v 强制亮色材质=%v", dark, light)
	if light[0]-dark[0] < 40 {
		t.Errorf("深色时段没有换用深色轨道材质：%v vs %v", dark, light)
	}
}

// TestPlayerBarVolumeDarkWired 与上一条同理，但钉的是音量弹层那条音量条
// （volumePopoverAt 里另一处 Dark: a.Theme.Dark）。深色时段渲染时把
// rangeSkinFor 换成亮色桩，音量条轨道顶内高光应随之变亮。
func TestPlayerBarVolumeDarkWired(t *testing.T) {
	topAt := func() [3]int {
		_, tst := playerBarProbeAt(t, PeriodAt(0), func(a *App) { a.ShowVolume = true })
		anchor, ok := tst.Find("音量")
		if !ok {
			t.Fatal("播放栏里找不到音量键")
		}
		tr := volumeTrackRect(anchor)
		return rgbAt(tst.Image(), int(tr.X+tr.W*0.25), int(tr.Y)+1)
	}
	dark := topAt()
	saved := rangeSkinFor
	rangeSkinFor = func(_, hovered bool) rangeSkin { return rangeSkinForTheme(false, hovered) }
	defer func() { rangeSkinFor = saved }()
	light := topAt()
	t.Logf("深色时段音量条顶内高光 深色材质=%v 强制亮色材质=%v", dark, light)
	if light[0]-dark[0] < 40 {
		t.Errorf("深色时段音量条没有换用深色轨道材质：%v vs %v", dark, light)
	}
}
