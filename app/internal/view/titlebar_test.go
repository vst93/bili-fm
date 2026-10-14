package view

import (
	"image"
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
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return tst
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
