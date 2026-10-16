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
//（“2x 长得跟 1x 一样”的假象）。
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
func settleTitleBarFade(tst *ui.Tester) {
	time.Sleep(300 * time.Millisecond)
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
// 与「设置」钮的垂直对齐。用户实测（Linux）报告「设置按钮看起来下移了」。
//
// 量了三层，每层都留了改前数值：
//
//  1. **盒中心**：.app-title-bar 是 flex + align-items:center，品牌文字盒
//     {40,7,46,22} 与设置钮盒 {94,4,44,28} 的中心**本来就重合**在栏中线
//     18.00（差 0）——这不是盒错位。
//  2. **墨迹中心**（同一字号 14 / 字重 500）：品牌 `bili-FM` 18.00、
//     「设置」19.00，差 1.00px。字权重统一前是 600 vs 400，差 1.5px
//     （品牌 17.50 / 设置 19.00）——用户说的「下移」是这个。
//  3. **这个 1.0px 是字形固有**：同一字体下 CJK「设置」的墨迹带中位天生比
//     拉丁 `bili-FM` 低（两者基线在同一条上，差的是字形分布）。把两个串换成
//     同一个框内居中的自绘盒后仍然是同一组数值，说明与布局无关；Chromium
//     渲染的**原版**同样实测 Δ≈1.06，不是原生版独有。
//
// 用户要求「修到两者中心 Y 对齐」，于是给设置钮文字加 titleBarSettingsNudge
// 的光学上移。断言：
//
//  1. titleBarFontWeight == 500（.app-title-bar 的继承字重）；
//  2. titleBarSettingsNudge 在 (0, 1) 内（0=不改就别加这条，1 会在 1x/1.5x
//     过冲、2x 欠冲）；
//  3. 无头实测 1x **与 2x** 两档下，品牌与设置的墨迹包围盒中位差 ≤ 0.5px。
//     跨缩放一起断言，把「某个档位凑出来的假对齐」排除。
func TestTitleBarBrandAndSettingsAlign(t *testing.T) {
	if titleBarFontWeight != 500 {
		t.Fatalf("titleBarFontWeight = %d，应为 500（.app-title-bar font-weight:500）",
			titleBarFontWeight)
	}
	if titleBarSettingsNudge <= 0 || titleBarSettingsNudge >= 1 {
		t.Fatalf("titleBarSettingsNudge = %v，应在 (0,1) 内（0=不改、1=1x/1.5x 过冲）",
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

	// 无头实测：两档缩放下墨迹盒中位差 ≤ 0.5px。
	for _, scale := range []float32{1, 2} {
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
