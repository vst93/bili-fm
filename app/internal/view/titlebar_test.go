package view

import (
	"image"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestTitleBarReveal 钉住标题栏 hover-to-reveal 的两态取值（原版 globals.css
// 的 .app-title-bar :hover / :focus-within）：
//
//	直接子元素（`> *`）        常态 .32  → 唤出 .82
//	`.title-bar-brand`         常态 .70  → 唤出 1
//	`::before` 顶沿投影        常态 .52  → 唤出 .78
//	`::after` 白玻璃渐隐       常态 0    → 唤出 1
//
// 原生版此前完全没有这一套：标题栏内容一直满不透明度、鼠标进来什么都不发生，
// 而原版是「静息时退到背景里、鼠标落上去才浮现」。
func TestTitleBarReveal(t *testing.T) {
	child, brand, edge, wash := titleBarReveal(false)
	if child != 0.32 || brand != 0.7 || edge != 0.52 || wash != 0 {
		t.Errorf("静息态 = (%.2f, %.2f, %.2f, %.2f)，应为 (.32, .70, .52, 0)",
			child, brand, edge, wash)
	}
	child, brand, edge, wash = titleBarReveal(true)
	if child != 0.82 || brand != 1 || edge != 0.78 || wash != 1 {
		t.Errorf("唤出态 = (%.2f, %.2f, %.2f, %.2f)，应为 (.82, 1, .78, 1)",
			child, brand, edge, wash)
	}
}

// TestGradBands 钉住多停靠点渐变的切片：相邻带接缝处颜色相同（否则会看出
// 拼接），端点等于首尾停靠点，位置按比例插值。
func TestGradBands(t *testing.T) {
	stops := []gradStop{
		{0, ui.Hex("#000000").Alpha(1)},
		{0.5, ui.Hex("#000000").Alpha(0.5)},
		{1, ui.Hex("#000000").Alpha(0)},
	}
	bands := gradBands(stops, 4)
	if len(bands) != 4 {
		t.Fatalf("bands = %d, want 4", len(bands))
	}
	for i := 1; i < len(bands); i++ {
		if bands[i][0] != bands[i-1][1] {
			t.Errorf("第 %d 条带的接缝颜色不一致", i)
		}
	}
	if bands[0][0] != stops[0].C {
		t.Errorf("首带起点 = %v，应为 %v", bands[0][0], stops[0].C)
	}
	if bands[3][1] != stops[2].C {
		t.Errorf("末带终点 = %v，应为 %v", bands[3][1], stops[2].C)
	}
	// 0.25 处应落在两个停靠点之间（alpha .75 ≈ 191/255）。
	if got := gradAt(stops, 0.25).A; got < 186 || got > 196 {
		t.Errorf("gradAt(0.25).A = %d，应为 .75 上下", got)
	}
	if got := gradAt(stops, -1).A; got != 255 {
		t.Errorf("gradAt(-1).A = %d，应为 255（钳到首停靠点）", got)
	}
	if got := gradAt(stops, 2).A; got != 0 {
		t.Errorf("gradAt(2).A = %d，应为 0（钳到末停靠点）", got)
	}
}

// titleBarLayerProbe 把一层标题栏渐变单独画在一块纯色底上，便于逐像素验证
// 渐变方向与范围（不必从整窗里反推背景贡献）。
func titleBarLayerProbe(stops []gradStop, height, opacity float32, bg ui.Color) *image.RGBA {
	return ui.Render(func(c *ui.Context) {
		c.Root().Background(bg)
		ui.Box(c).FillWidth().Height(TitleBarHeight).Children(func() {
			titleBarLayer(c, "probe-layer", stops, height, opacity)
		})
	}, 400, 300, 1)
}

func grayAt(img *image.RGBA, x, y int) int {
	i := img.PixOffset(x, y)
	return (int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])) / 3
}

// titleBarLayerWorstDiff 量「渲染 vs CSS 多停靠点精确采样」的最大逐通道差：
// 一层标题栏渐变画在纯色底上，按 180°（CSS 里两层都是 180deg）投影对照。
func titleBarLayerWorstDiff(t *testing.T, stops []gradStop, height, opacity float32, bg ui.Color) int {
	t.Helper()
	const w, h = 400, 300
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(bg)
		ui.Box(c).FillWidth().Height(TitleBarHeight).Children(func() {
			titleBarLayer(c, "probe-layer", stops, height, opacity)
		})
	}, w, h, 1)
	// 层的几何：贴顶、整宽、高 height（DIP）。渐变是 180°（纯垂直），所以
	// t 就是层内的相对高度。
	worst := 0
	for y := 1; y < int(height); y += 2 {
		for x := 2; x < w-2; x += 7 {
			tv := (float32(y) + 0.5) / height
			want := gradAt(stops, tv)
			a := opacity * float32(want.A) / 255
			exp := [3]int{
				int(float32(bg.R)*(1-a) + float32(want.R)*a + 0.5),
				int(float32(bg.G)*(1-a) + float32(want.G)*a + 0.5),
				int(float32(bg.B)*(1-a) + float32(want.B)*a + 0.5),
			}
			o := img.PixOffset(x, y)
			got := [3]int{int(img.Pix[o]), int(img.Pix[o+1]), int(img.Pix[o+2])}
			for i := 0; i < 3; i++ {
				if d := int(abs32(float32(got[i] - exp[i]))); d > worst {
					worst = d
				}
			}
		}
	}
	return worst
}

// TestTitleBarLayersMatchCSS 钉住标题栏那两层（`::before` 顶沿投影、`::after`
// 白玻璃）现在走的是**真多停靠点**渐变，与 CSS 逐点一致（≤1 灰阶）。
//
// 反证：换回「4 条等高带拼」的旧写法 → 两层都超 1（实测 titleBarWash 最坏 8）。
func TestTitleBarLayersMatchCSS(t *testing.T) {
	white := ui.RGB(255, 255, 255)
	if worst := titleBarLayerWorstDiff(t, titleBarEdgeStops, TitleBarHeight*1.7, 0.52, white); worst > 1 {
		t.Errorf("::before 顶沿投影与 CSS 差 %d 灰阶（应 ≤1）", worst)
	}
	gray := ui.Hex("#808080")
	if worst := titleBarLayerWorstDiff(t, titleBarWashStops, TitleBarHeight*1.4, 1, gray); worst > 1 {
		t.Errorf("::after 白玻璃与 CSS 差 %d 灰阶（应 ≤1）", worst)
	}
}

// TestTitleBarLayersCounterProof 是 TestTitleBarLayersMatchCSS 的反证。
//
// 注意这里的反证余量**很薄**：两层都是 180°（纯垂直），渐变线与带边垂直，
// 所以逐带重投影只在带内保留比例，误差只有 **2 灰阶**。真正的分带之痛在 135°
// 的斜向层（dialogSheen / loginSheen 实测 38 / 17）。这条反证存在的意义是钉住
// 「180° 也不能退回分带」这个下限，不是声称分带在 180° 下会明显翻车。
func TestTitleBarLayersCounterProof(t *testing.T) {
	saved := titleBarLayerFn
	defer func() { titleBarLayerFn = saved }()
	titleBarLayerFn = func(c *ui.Context, key string, stops []gradStop, height, opacity float32) {
		box := ui.Box(c).Absolute().Left(0).Right(0).Top(0).Height(height).PassThrough()
		box.Opacity(titleBarFade(box, key, opacity))
		box.Children(func() {
			ui.Column(c).Fill().PassThrough().Children(func() {
				for _, b := range gradBands(stops, 4) {
					ui.Box(c).FillWidth().Grow(1).PassThrough().LinearGradient(ui.LinearGradient{
						From: b[0], To: b[1], Angle: 180,
					})
				}
			})
		})
	}
	worst := titleBarLayerWorstDiff(t, titleBarWashStops, TitleBarHeight*1.4, 1, ui.Hex("#808080"))
	if worst <= 1 {
		t.Errorf("退回分带拼法后 ::after 与 CSS 只差 %d 灰阶，反证不成立", worst)
	} else {
		t.Logf("::after 分带拼法实测最坏 %d 灰阶", worst)
	}
}

// TestTitleBarEdgeBand 是无头渲染实测：`.app-title-bar::before` 是顶沿往下
// 淡出的投影（整层 opacity .52，高度 170% 标题栏）。顶部最暗，越往下越浅，
// 到 170%（≈61px）处已经完全透明。
func TestTitleBarEdgeBand(t *testing.T) {
	img := titleBarLayerProbe(titleBarEdgeStops, TitleBarHeight*1.7, 0.52, ui.RGB(255, 255, 255))
	top := grayAt(img, 200, 0)
	mid := grayAt(img, 200, 30)
	end := grayAt(img, 200, 60)
	t.Logf("投影层 y=0:%d y=30:%d y=60:%d（白底 255）", top, mid, end)
	if top > 248 {
		t.Fatalf("顶沿没有投影：y=0 灰度 %d", top)
	}
	if !(top < mid && mid < end) {
		t.Fatalf("投影没有往下淡出：y=0:%d y=30:%d y=60:%d", top, mid, end)
	}
	if end < 253 {
		t.Fatalf("170%% 高度处投影仍未消失：y=60 灰度 %d", end)
	}
}

// TestTitleBarWash 是无头渲染实测：`.app-title-bar::after` 是唤出时浮出的
// 白玻璃（高度 140%，往下淡出）。顶上最亮，到 140%（≈50px）处回到背景。
func TestTitleBarWash(t *testing.T) {
	bg := ui.Hex("#808080")
	img := titleBarLayerProbe(titleBarWashStops, TitleBarHeight*1.4, 1, bg)
	base := grayAt(img, 200, 60)
	top := grayAt(img, 200, 0)
	mid := grayAt(img, 200, 15)
	low := grayAt(img, 200, 30)
	t.Logf("白玻璃层 y=0:%d y=15:%d y=30:%d y=60:%d（灰底 %d）", top, mid, low, base, base)
	if base != 128 {
		t.Fatalf("140%% 高度处不应再有白玻璃：y=60 灰度 %d，应为背景 128", base)
	}
	if top < mid || mid < low || low < base {
		t.Fatalf("白玻璃没有往下淡出：y=0:%d y=15:%d y=30:%d y=60:%d", top, mid, low, base)
	}
	if top-base < 30 {
		t.Fatalf("顶上白玻璃太弱：%d vs 背景 %d", top, base)
	}
}

// titleBarProbe 建一个 1000×700 的主窗（固定 noon 时段，背景可复现）。
func titleBarProbe(t *testing.T) *ui.Tester {
	return titleBarProbeAt(t, 1)
}

// titleBarProbeAt 同 titleBarProbe，但指定缩放（用 2x 把「某个 1x 档位凑出来的
// 假对齐」排除；墨迹量取按 scale 换算回 DIP）。
func titleBarProbeAt(t *testing.T, scale float32) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.SetScale(scale)
	tst.Frame()
	return tst
}

// inkCenterDIP 量出矩形 r 内墨色（三通道均 < thr）像素的纵向包围盒中位，
// 换算回 DIP。r 是 DIP 矩形，图是 device 像素（tst.Image() 随 SetScale 变），
// 所以两处都要乘 scale —— 否则 2x 下扫的是一半区域、量出来是错的
// （“2x 长得跟 1x 一样”的假象）。
func inkCenterDIP(t *testing.T, tst *ui.Tester, r ui.Rect, scale float32, thr int) float32 {
	t.Helper()
	img := tst.Image()
	b := img.Bounds()
	x0, x1 := int(r.X*scale), int((r.X+r.W)*scale)
	y0, y1 := int(r.Y*scale)-12, int((r.Y+r.H)*scale)+12
	lo, hi := 1<<30, -1
	for y := y0; y <= y1; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := x0; x < x1; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			c := rgbAt(img, x, y)
			if c[0] < thr && c[1] < thr && c[2] < thr {
				if y < lo {
					lo = y
				}
				if y > hi {
					hi = y
				}
			}
		}
	}
	if hi < 0 {
		t.Fatalf("矩形 %+v 内没量到墨迹（scale=%.1f）", r, scale)
	}
	return float32(lo+hi) / 2 / scale
}

// inkBottomDIP 同 inkCenterDIP，但返回墨迹的**底部**（最大 y）换算回 DIP ——
// 2026-10-18 用户追诉「文字底部还没对齐」，需要单独量这个指标。
func inkBottomDIP(t *testing.T, tst *ui.Tester, r ui.Rect, scale float32, thr int) float32 {
	t.Helper()
	img := tst.Image()
	b := img.Bounds()
	x0, x1 := int(r.X*scale), int((r.X+r.W)*scale)
	y0, y1 := int(r.Y*scale)-12, int((r.Y+r.H)*scale)+12
	hi := -1
	for y := y0; y <= y1; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := x0; x < x1; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			c := rgbAt(img, x, y)
			if c[0] < thr && c[1] < thr && c[2] < thr {
				if y > hi {
					hi = y
				}
			}
		}
	}
	if hi < 0 {
		t.Fatalf("矩形 %+v 内没量到墨迹（scale=%.1f）", r, scale)
	}
	return float32(hi) / scale
}

// darkestIn 返回一块矩形里最暗的像素（品牌字色是深色，越接近满不透明度越暗）。
func darkestIn(img *image.RGBA, x0, y0, x1, y1 int) [3]int {
	best := [3]int{1 << 20, 1 << 20, 1 << 20}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			i := img.PixOffset(x, y)
			c := [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
			if c[0]+c[1]+c[2] < best[0]+best[1]+best[2] {
				best = c
			}
		}
	}
	return best
}

// TestTitleBarRevealOnHover 是整窗的无头渲染实测：鼠标落在标题栏上时
// 品牌（`.title-bar-brand`，静息 .70）要亮起来、顶沿（白玻璃）要浮出来；
// 鼠标移开又退回去。这三条一起证明 reveal 真的接上了，而不只是几个常量。
//
// 本轮起这四层走 0.24s 过渡（原版 `transition: opacity 0.24s ease`），
// 所以每次移动指针后要等过渡跑完再量（settleTitleBarFade）。
func TestTitleBarRevealOnHover(t *testing.T) {
	tst := titleBarProbe(t)
	rect, ok := tst.Find("bili-FM")
	if !ok {
		t.Fatal("标题栏里找不到品牌名")
	}
	x0, y0 := int(rect.X), int(rect.Y)
	x1, y1 := int(rect.X+rect.W), int(rect.Y+rect.H)

	rest := darkestIn(tst.Image(), x0, y0, x1, y1)
	restTop := grayAt(tst.Image(), 500, 1)

	tst.Move(500, 18) // 落在标题栏的空白处
	settleTitleBarFade(tst)
	hover := darkestIn(tst.Image(), x0, y0, x1, y1)
	hoverTop := grayAt(tst.Image(), 500, 1)

	t.Logf("品牌最深像素 静息=%v 唤出=%v；顶沿 静息=%d 唤出=%d", rest, hover, restTop, hoverTop)

	if hover[0] >= rest[0] {
		t.Errorf("唤出后品牌没有变亮：静息 %v → 唤出 %v", rest, hover)
	}
	if rest[0]-hover[0] < 15 {
		t.Errorf("品牌 .70→1 的差别太小：静息 %v → 唤出 %v", rest, hover)
	}
	if hoverTop <= restTop {
		t.Errorf("唤出后顶沿没有浮出白玻璃：静息 %d → 唤出 %d", restTop, hoverTop)
	}

	// 移开指针（落到主区），整条退回静息态。
	tst.Move(500, 400)
	settleTitleBarFade(tst)
	back := darkestIn(tst.Image(), x0, y0, x1, y1)
	t.Logf("移开指针后品牌最深像素 = %v", back)
	if back[0] != rest[0] {
		t.Errorf("移开指针后品牌 = %v，应回到静息态 %v", back, rest)
	}
}

// settleTitleBarFade 等标题栏那 0.24s 过渡跑完再出帧。Tester 的时钟是真实
// 墙钟、settle 只跑 20 帧，240ms 的过渡未必能在 settle 里收敛（机器快时 20 帧
// 不到 240ms），所以显式睡过时长再出一帧。
// settleTitleBarFade 推过标题栏 hover-to-reveal 的 0.24s 过渡。v0.5.5 的 Tester
// 时钟冻结，用 Advance 推进（旧版靠墙钟 Sleep）。
func settleTitleBarFade(tst *ui.Tester) {
	tst.Advance(300 * time.Millisecond)
	tst.Frame()
}

// TestTitleBarRevealKeepsFocus 是一条回归，钉住 reveal 那两层的两条纪律：
//
//  1. 两层必须**常驻**建出来（静息时白玻璃那层 opacity 0）。mygo 的元素 id 由
//     「父 id + 已在建的子元素个数」决定，鼠标进/出标题栏时少建一个兄弟会让它
//     后面所有兄弟换 id —— 设置按钮的焦点与状态会跟着丢（曾经就是这样：悬停
//     标题栏后 Tab 到的设置键立刻失焦，`Focused("设置")` 变假）。
//  2. 两层的整棵子树都得 PassThrough。它们盖在标题栏按钮上，只要有一层参与命中
//     测试，在标题栏里按下的就是它（或它那个带 DragWindow 的祖先行）—— 设置键
//     就点不开了（曾经也真的点不开）。
func TestTitleBarRevealKeepsFocus(t *testing.T) {
	tst := titleBarProbe(t)
	tst.Key(0, ui.KeyTab)
	tst.Frame()
	if !tst.Focused("设置") {
		t.Fatal("Tab 一次后焦点不在设置键上，测试前提不成立")
	}
	for _, p := range [][2]float32{{500, 18}, {500, 400}, {500, 18}} {
		tst.Move(p[0], p[1])
		tst.Frame()
		if !tst.Focused("设置") {
			t.Fatalf("指针移到 (%v,%v) 后设置键失焦（reveal 那两层不能按需增删）", p[0], p[1])
		}
	}
	// 悬停（已唤出）时点设置键，玻璃面板照样打得开（面板是自定义 Popover，
	// 不再是框架的系统菜单）。
	if err := tst.Click("设置"); err != nil {
		t.Fatalf("点设置键失败：%v", err)
	}
	tst.Frame()
	for _, item := range settingsItems() {
		if _, ok := tst.Find(item); !ok {
			t.Fatalf("设置菜单打开后看不到条目 %q", item)
		}
	}
}

// TestTitleBarBrandAndSettingsAlign 钉住标题栏里「品牌行（logo + bili-FM）」
// 与「设置」钮的垂直对齐。用户实测（Linux）报告「设置按钮看起来下移了」，
// 2026-10-18 追诉：「文字底部仍未对齐、各平台都要检查」。
//
// 量了三层，每层都留了改前数值：
//
//  1. **盒中心**：.app-title-bar 是 flex + align-items:center，品牌文字盒
//     {40,7,46,22} 与设置钮盒 {94,4,44,28} 的中心**本来就重合**在栏中线
//     18.00（差 0）——这不是盒错位。
//  2. **墨迹**（同一字号 14 / 字重 500）：品牌 `bili-FM` 墨迹 [13..23]（高
//     10）、「设置」墨迹 [13..25]（高 12），**顶/基线已重合**，但 CJK 表意框
//     上下各多一个像素——于是中心差 +1.00、底部差 +2.00。这个高差是**字形
//     固有**（换成同一框内居中的自绘盒后数值不变；Chromium 渲染的**原版**
//     同样实测 Δ≈1.06，不是原生版独有），所以中心与底部**不能同时**对到 0。
//  3. 字权在上上轮已被统一到 500（此前 600 vs 400，差 1.5px）。
//
// 用户先后要求「中心对齐」与「底部对齐」，于是本轮同时钉**两个指标**，并把
// titleBarSettingsNudge 调到同时把两者收进 1px 的值（见 theme.go 的表）：
// nudge=1.0 时中心 max ≤0.5、底部 min ≥1.0（0.75 时底部 1.33、1.5 时中心飘
// 到 1.0）。断言：
//
//  1. titleBarFontWeight == 500（.app-title-bar 的继承字重）；
//
//  2. titleBarSettingsNudge 在 (0, 1.5] 内（0=不改、>1.5 中心会飘）；
//
//  3. 无头实测 **四档缩放**（1 / 1.25 / 1.5 / 2，Linux HiDPI 常用 1.25 与 1.5）
//     下，墨迹**中心差 ≤ 0.5px**（上上轮要求）且**底部差 ≤ 1.0px**（本轮要求，
//     字形高差 ~2px 时的下限）。跨缩放一起断言，把「某个档位凑出来的假对齐」排除。
//
//     反证：把 titleBarSettingsNudge 退回 0 → 1x 中心 1.00、1.5x 底部 2.00，两条报红；
//     退回到上一轮的 0.75 → 1.5x 底部 1.33 > 1.0，底部那条报红。
func TestTitleBarBrandAndSettingsAlign(t *testing.T) {
	if titleBarFontWeight != 500 {
		t.Fatalf("titleBarFontWeight = %d，应为 500（.app-title-bar font-weight:500）",
			titleBarFontWeight)
	}
	if titleBarSettingsNudge <= 0 || titleBarSettingsNudge > 1.5 {
		t.Fatalf("titleBarSettingsNudge = %v，应在 (0, 1.5] 内（0=不改、>1.5 中心会飘到 >1px）",
			titleBarSettingsNudge)
	}
	// 源码链：两边都用 titleBarFontWeight，且设置钮文字带光学位移。
	flat := flattenNoComments(t, "shell.go")
	if !strings.Contains(flat, `ui.Text(c,"bili-FM").FontSize(14).FontWeight(titleBarFontWeight)`) {
		t.Error("品牌文字没有用 titleBarFontWeight（应与设置钮同一字重 500）")
	}
	if !strings.Contains(flat, `ui.Text(c,"设置").FontSize(14).FontWeight(titleBarFontWeight).TextColor(t.Ink).Translate(0,-titleBarSettingsNudge)`) {
		t.Error("设置钮文字没有按 titleBarSettingsNudge 上移（CJK 墨迹天生偏低）")
	}

	// 无头实测：四档缩放下墨迹中心差 ≤ 0.5px、底部差 ≤ 1.5px。
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		tst := titleBarProbeAt(t, scale)
		brand, ok := tst.Find("bili-FM")
		if !ok {
			t.Fatal("标题栏里找不到品牌名")
		}
		settings, ok := tst.Find("设置")
		if !ok {
			t.Fatal("标题栏里找不到设置键")
		}
		want := float32(TitleBarHeight) / 2
		bc, sc := brand.Y+brand.H/2, settings.Y+settings.H/2
		t.Logf("scale=%.1f 品牌盒中心=%.2f 设置盒中心=%.2f（栏中线=%.2f）", scale, bc, sc, want)
		if d := bc - want; d < -0.5 || d > 0.5 {
			t.Errorf("scale=%.1f 品牌盒中心 %.2f 不在栏中线上（want %.2f）", scale, bc, want)
		}
		if d := sc - want; d < -0.5 || d > 0.5 {
			t.Errorf("scale=%.1f 设置键盒中心 %.2f 不在栏中线上（want %.2f）", scale, sc, want)
		}
		bInk, sInk := inkCenterDIP(t, tst, brand, scale, 150), inkCenterDIP(t, tst, settings, scale, 150)
		t.Logf("scale=%.1f 品牌墨迹中心=%.2f 设置墨迹中心=%.2f Δ=%.2f", scale, bInk, sInk, sInk-bInk)
		if d := sInk - bInk; d < -0.5 || d > 0.5 {
			t.Errorf("scale=%.1f 品牌与设置的墨迹中心差 %.2fpx > 0.5px（用户报的「设置钮下移」）",
				scale, d)
		}
		bBot, sBot := inkBottomDIP(t, tst, brand, scale, 150), inkBottomDIP(t, tst, settings, scale, 150)
		t.Logf("scale=%.1f 品牌墨迹底=%.2f 设置墨迹底=%.2f Δ=%.2f", scale, bBot, sBot, sBot-bBot)
		if d := sBot - bBot; d < -1.0 || d > 1.0 {
			t.Errorf("scale=%.1f 品牌与设置的墨迹底部差 %.2fpx > 1.0px（2026-10-18 用户报的「文字底部没对齐」）",
				scale, d)
		}
	}
}

// TestTitleBarIncognitoToggle 钉住标题栏右侧的常驻隐身开关（用户 2026-10-17 报告：
// 「隐身模式放设置菜单中太蠢了」，要求常驻可见、一次点击可达、状态一眼可见）。
//
// 三个要点：
//  1. 无需展开任何菜单，标题栏里就找得到它（Find 命中）；
//  2. 点它直接触发 ToggleIncognito，且**没有**打开任何弹层（管它是不是开关，
//     都要求「一下即点」）；
//  3. 窄窗（品牌行被隐藏）它仍然在——它挂在 actions 行，不像品牌行会在 ≤600px 消失。
func TestTitleBarIncognitoToggle(t *testing.T) {
	for _, w := range []int{1000, 500} {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		clicks := 0
		a.Act.ToggleIncognito = func() { clicks++; a.Incognito = !a.Incognito }
		// 用一个新的 controller 语义：view 只认 Act（ToggleIncognito），
		// a.Incognito 由测试自己翻，验证「一次点击即翻转」。
		tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, w, 700)
		tt.Frame()
		if _, ok := tt.Find("隐身模式"); !ok {
			t.Fatalf("窗宽 %d：标题栏里找不到常驻隐身键（无需展开菜单即应可见）", w)
		}
		// 没用 uid 区分 Label/Tooltip，Click 命中键中心即点中它。
		if err := tt.Click("隐身模式"); err != nil {
			t.Fatalf("窗宽 %d：点常驻隐身键失败：%v", w, err)
		}
		tt.Frame()
		if clicks != 1 {
			t.Fatalf("窗宽 %d：点击次数 = %d，应为 1（一次点击即切换）", w, clicks)
		}
		if a.ShowSettings {
			t.Fatalf("窗宽 %d：点隐身键后设置菜单被打开了（它不该打开任何弹层）", w)
		}
	}
}

// TestTitleBarIncognitoActiveInk 钉住常驻键的激活视觉与历史抽屉那颗同源：
// 开启时图标墨色取 incognitoInk(true)（#0e7490），不是标题栏通用的 --studio-blue。
func TestTitleBarIncognitoActiveInk(t *testing.T) {
	if got := incognitoInk(true); got != ui.Hex("#0e7490") {
		t.Fatalf("incognitoInk(true) = %v，应为 #0e7490", got)
	}
	flat := flattenNoComments(t, "settingsmenu.go")
	if !strings.Contains(flat, `ifa.Incognito{ink=incognitoInk(true)}`) {
		t.Error("标题栏常驻键在隐身开启时没有把墨色换成 incognitoInk(true)")
	}
	if !strings.Contains(flat, `pick(a.Incognito,iconMaskFilled,iconMask)`) {
		t.Error("标题栏常驻键的图标没有随开关态在 filled/outline 间切换")
	}
}

// TestTitleBarIncognitoOnMac 钉住常驻隐身键在 macOS 上也在：macOS 的品牌行不挂
// 设置钮（原版 isMac 分支），隐身键挂在 actions 行，**平台无关**，所以那里同样
// 一眼可见、一下即点。
func TestTitleBarIncognitoOnMac(t *testing.T) {
	orig := viewGOOS
	viewGOOS = "darwin"
	t.Cleanup(func() { viewGOOS = orig })

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	if _, ok := tst.Find("隐身模式"); !ok {
		t.Fatal("macOS 上找不到常驻隐身键（actions 行应平台无关）")
	}
	// macOS 上设置键不渲染（原版 isMac 分支），所以它不该出现在标题栏里。
	if _, ok := tst.Find("设置"); ok {
		t.Error("macOS 上仍有「设置」键（原版 isMac 分支不挂它）")
	}
}

// TestTitleBarIncognitoRestVisibility 是本轮（2026-10-17 第二轮）新增的回归，
// 钉住常驻隐身键在**静息态**（鼠标不在标题栏上）也有可辨的激活视觉 —— 用户对
// 这颗常驻入口的要求原文是「常驻可见、一次点击可达、**状态一眼可见**」。
//
// 背景：`.app-title-bar > *` 静息只到 .32、唤出才 .82（见 titleBarReveal，原版
// globals.css 如此），而 actions 这一行现在**只装隐身键**，于是 .32 唯一的可见
// 后果就是把「开着」压得几乎融进标题栏。无头实测（静息、1000×700、noon）：
//
//	               关闭态最暗亮度和   开启态最暗亮度和
//	改前（行恒 .32）        592              560      ← 两态只差 32/720，键内仅 5.6% 像素可辨
//	改后（开态行 .62）      592              434      ← 差 158
//
// 修法：隐身**开启**时给它所在行动静息不透明度加下限 tbIncognitoRestInk（.62），
// 关闭态照旧 .32（关着的开关本就该安静）。唤出态 .82 高于下限、不受影响。
//
// 反证：把 shell.go 里那段下限去掉（childTarget 直接用 child），第一条断言变红。
func TestTitleBarIncognitoRestVisibility(t *testing.T) {
	childRest, _, _, _ := titleBarReveal(false)
	if tbIncognitoRestInk <= childRest {
		t.Fatalf("tbIncognitoRestInk = %v，必须高于静息值（否则下限不起作用）", tbIncognitoRestInk)
	}
	flat := flattenNoComments(t, "shell.go")
	if !strings.Contains(flat, "ifa.Incognito&&childTarget<tbIncognitoRestInk{") {
		t.Error("隐身开启时没有给 actions 行的静息不透明度加下限（状态不「一眼可见」）")
	}

	// 无头实测：静息态下量键内最暗像素的亮度和（越小=激活视觉越明显）。
	darkestSum := func(on bool, hover bool) int {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Incognito = on
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.Frame()
		if hover {
			tst.Move(500, 18)
		} else {
			tst.Move(500, 400)
		}
		tst.Frame()
		r, ok := tst.Find("隐身模式")
		if !ok {
			t.Fatal("找不到常驻隐身键")
		}
		img := tst.Image()
		best := 1 << 30
		for y := int(r.Y); y <= int(r.Y+r.H); y++ {
			for x := int(r.X); x <= int(r.X+r.W); x++ {
				i := img.PixOffset(x, y)
				if s := int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2]); s < best {
					best = s
				}
			}
		}
		return best
	}
	restOff, restOn := darkestSum(false, false), darkestSum(true, false)
	t.Logf("静息态 键内最暗亮度和：关闭 %d 开启 %d（差 %d，白底 720）", restOff, restOn, restOn-restOff)
	if restOff-restOn < 80 {
		t.Errorf("静息态开启/关闭两态只差 %d（<80），激活视觉不够「一眼可见」（开 %d 关 %d）",
			restOff-restOn, restOn, restOff)
	}
	// 唤出态两态仍可辨（下限不参与，行是 .82）。
	hovOff, hovOn := darkestSum(false, true), darkestSum(true, true)
	t.Logf("唤出态 键内最暗亮度和：关闭 %d 开启 %d", hovOff, hovOn)
	if hovOff <= hovOn {
		t.Errorf("唤出态开启键不比关闭键深：开 %d 关 %d", hovOn, hovOff)
	}
}

// TestTitleBarSelfDrawnWindowControls 钉住 Linux/Windows 在**没有原生窗口控件**
// 时自绘的最小化 / 隐藏窗口两颗键（用户 2026-10-18 报告「Linux 下测试，关闭和
// 缩小操作按钮怎么也没有了」）。
//
// 背景：原版 Tauri 在非 macOS 上 decorations:false、前端自绘这两颗键
// （titleBar.tsx 的 #wds-handle-group）。mygo 用 TitleBarHidden 让 GTK 画原生
// 标题按钮，但 Wayland 下由合成器装饰窗口的桌面（KWin/Hyprland/Sway）与平铺
// 窗管根本不给原生按钮，于是 Linux 用户看到按钮消失。
//
// 判据：
//  1. `tb.Right == 0`（无原生占位）时，非 macOS 平台 Find 命中两颗键、点击触发
//     Minimize / HideWindow；
//  2. `tb.Right > 0`（原生控件在位）时**不**画自绘键（不重复）；
//  3. macOS 平台不画（红绿灯是系统原生的）。
//
// 反证：把 `!mac && tb.Right <= 0` 判据去掉（或恒真）→ 第 2/3 条红。
func TestTitleBarSelfDrawnWindowControls(t *testing.T) {
	orig := viewGOOS
	defer func() { viewGOOS = orig }()

	probe := func(goos string, tb ui.TitleBar) (*App, *ui.Tester) {
		viewGOOS = goos
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.SetTitleBar(tb)
		tst.Frame()
		return a, tst
	}

	// ① Linux、无原生控件：两颗键都在，点击分别走 Minimize / HideWindow。
	a, tst := probe("linux", ui.TitleBar{Height: 36})
	for _, lab := range []string{"最小化", "隐藏窗口"} {
		if _, ok := tst.Find(lab); !ok {
			t.Fatalf("Linux 无原生控件时找不到自绘键 %q", lab)
		}
	}
	minClicks, hideClicks := 0, 0
	a.Act.Minimize = func() { minClicks++ }
	a.Act.HideWindow = func() { hideClicks++ }
	if err := tst.Click("最小化"); err != nil {
		t.Fatalf("点最小化键失败：%v", err)
	}
	tst.Frame()
	if err := tst.Click("隐藏窗口"); err != nil {
		t.Fatalf("点隐藏窗口键失败：%v", err)
	}
	tst.Frame()
	if minClicks != 1 || hideClicks != 1 {
		t.Errorf("点击回调次数 minimize=%d hide=%d，应为 1/1", minClicks, hideClicks)
	}

	// ② Linux、原生控件在位：不画自绘键。
	_, tst2 := probe("linux", ui.TitleBar{Height: 36, Right: 140})
	for _, lab := range []string{"最小化", "隐藏窗口"} {
		if _, ok := tst2.Find(lab); ok {
			t.Errorf("原生控件在位时仍画了自绘键 %q（会与系统按钮重复）", lab)
		}
	}

	// ③ macOS：红绿灯是系统原生的，不自绘。
	_, tst3 := probe("darwin", ui.TitleBar{Height: 36, Left: 78})
	for _, lab := range []string{"最小化", "隐藏窗口"} {
		if _, ok := tst3.Find(lab); ok {
			t.Errorf("macOS 上画了自绘键 %q（应走系统红绿灯）", lab)
		}
	}
}

// TestTitleBarMiniEntryRestored 钉住标题栏右侧的「切换到迷你模式」入口（原
// #switch-window-mode）。用户 2026-10-18 要求「看看 Linux 下能不能恢复迷你模式」：
// 原版 Tauri 在 Linux 禁用迷你模式（webkit2gtk 的无边界窗口改不了尺寸），但 mygo
// 的 Linux 后端直接驱动 GTK（gtkWindowResize），运行时改尺寸正常，所以三平台都
// 能进迷你模式。该入口曾在 `53f7cb5` 被整体移除，导致迷你模式只剩 `--mini`
// 启动参数可达——本轮恢复。
//
// 判据：Linux 与 Windows 上 Find 命中该键、点击触发 SetMini(true)；且图标是
// iconMini（原版 ZoomInternal）。
//
// 反证：把 miniEntryButton 调用删掉 → Find 失败，第一条报红。
func TestTitleBarMiniEntryRestored(t *testing.T) {
	orig := viewGOOS
	defer func() { viewGOOS = orig }()
	for _, goos := range []string{"linux", "windows"} {
		viewGOOS = goos
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		clicks := 0
		a.Act.SetMini = func(on bool) {
			if on {
				clicks++
			}
		}
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
		tst.Frame()
		if _, ok := tst.Find("切换到迷你模式"); !ok {
			t.Fatalf("%s：标题栏里找不到「切换到迷你模式」入口", goos)
		}
		if err := tst.Click("切换到迷你模式"); err != nil {
			t.Fatalf("%s：点迷你入口失败：%v", goos, err)
		}
		tst.Frame()
		if clicks != 1 {
			t.Fatalf("%s：迷你入口点击次数 = %d，应为 1", goos, clicks)
		}
	}
	// 源码链：入口键用 iconMini（15px）并触发 SetMini(true)。
	flat := flattenNoComments(t, "shell.go")
	if !strings.Contains(flat, "ui.Icon(c,iconMini).Size(15,15)") {
		t.Error("迷你入口没用 iconMini 15px（原版 <ZoomInternal size=\"15\">）")
	}
	if !strings.Contains(flat, `b.Clicked()&&a.Act.SetMini!=nil{a.Act.SetMini(true)}`) {
		t.Error("迷你入口点击没有触发 SetMini(true)")
	}
}

// TestTitleBarWindowControlsGeometry 钉住标题栏自绘窗口控制键组（原版
// #wds-handle-group）的几何：`flex items-center gap-1 px-1 py-0.5`
// （main:src/components/titleBar.tsx:411）。
//
// 用户 2026-10-18 晚间 ①：「浮窗应用不应有放大按钮，只留缩小 + 关闭，
// 样式也要对齐原版」。放大键在窗口选项里已经关掉（DisableMaximize），
// 这里量的是**样式**：两颗键 30×30（`.app-title-bar-btn`）、间距 gap-1 = 4px、
// 组左右各 4px（px-1）——关闭键右缘离窗口右边 = 栏内边距 16 + 组 4 = 20。
//
// 反证：把 Gap 退回 1px、去掉 Padding(0, 4) → 两条断言分别红（间距 1、
// 右缘 16）。
func TestTitleBarWindowControlsGeometry(t *testing.T) {
	tst := titleBarProbeAt(t, 1)
	min, ok := tst.Find("最小化")
	if !ok {
		t.Fatalf("标题栏里找不到「最小化」键：%v", tst.Texts())
	}
	cl, ok := tst.Find("隐藏窗口")
	if !ok {
		t.Fatalf("标题栏里找不到「隐藏窗口」键：%v", tst.Texts())
	}
	t.Logf("最小化 %+v / 隐藏窗口 %+v", min, cl)
	for name, r := range map[string]ui.Rect{"最小化": min, "隐藏窗口": cl} {
		if r.W != 30 || r.H != 30 {
			t.Errorf("%s 键 %v×%v，应为 30×30（.app-title-bar-btn）", name, r.W, r.H)
		}
		if min.Y != cl.Y {
			t.Errorf("两颗键不在同一水平线上：%v vs %v", min.Y, cl.Y)
		}
	}
	// 期望值写死成原版 CSS 的数字（不引用布局常量）：否则「常量改错、
	// 断言跟着改错」会互相掩护，测试永远绿（本轮反证时踩过）。
	if gap := cl.X - (min.X + min.W); gap != 4 {
		t.Errorf("两颗键间距 %v，应为 4（原版 gap-1 = 4px）", gap)
	}
	if right := 1000 - (cl.X + cl.W); right != 20 {
		t.Errorf("关闭键右缘距窗口右边 %v，应为 20（栏 padding 16 + 组 px-1 4）", right)
	}
}
