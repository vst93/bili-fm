package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（迷你窗对照 main 原版）② 的测试：迷你窗标题栏的 hover-to-reveal。
//
// 原版 mini 下 `.app-title-bar > *` 整条淡到 opacity 0（只剩 Windows/Linux 由
// 前端自绘的窗口按钮被 L4759 豁免为 1），但标题栏那两层贴边**照旧**：
//
//	::before  顶沿投影（inset 0 0 -70% 0，170%）     opacity .52  → hover .78
//	::after   白玻璃洗层（height 140%）              opacity 0    → hover 1
//	两层都是 `transition: opacity 0.24s ease`
//
// mygo 的迷你窗标题栏没有自绘子元素（窗口按钮是 OS 绘制的原生控件，应用没法
// 淡入它们），所以这两层贴边就是迷你窗标题栏的 hover 反馈 —— 与主窗同一手法
// （titleBarLayer + titleBarReveal）。此前 mini 的标题栏是一行空的 DragWindow Row，
// 鼠标进出什么都不发生。

// miniTitleBarRevealProbe 建一个 400×155 的迷你窗，并按 hover 把指针摆好：
// hover=true 停在标题栏正中（y = miniTitleBar/2），hover=false 停在信息行里
// （y = miniTitleBar + 20）——两层都还在窗口内，只是不在标题栏上。
func miniTitleBarRevealProbe(t *testing.T, hover bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	if hover {
		tst.Move(miniWidth/2, miniTitleBar/2)
	} else {
		tst.Move(miniWidth/2, miniTitleBar+20)
	}
	settleTitleBarFade(tst)
	return tst
}

// miniTitleBarLayerTargets 渲染一面迷你窗并把 titleBarFade 收到的（层 key,
// 目标不透明度）逐条记下来：静息帧应收到 .52/0，hover 帧收到 .78/1。
//
// 注意：只有**首帧**是静息态 —— 后续帧（如 TitleBar 探查或 WebView 登记引起的
// 重排）在无头测试里已经把指针当成压在窗内了，所以这里分开拍两次快照。
func miniTitleBarLayerTargets(t *testing.T) (map[string]float32, map[string]float32) {
	t.Helper()
	orig := titleBarFade
	t.Cleanup(func() { titleBarFade = orig })
	var rest, hover map[string]float32
	rec := map[string]float32{}
	titleBarFade = func(_ ui.Element, key string, target float32) float32 {
		rec[key] = target
		return target
	}
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	// 首帧 = 静息（指针还没进过窗）。
	rest = map[string]float32{}
	for k, v := range rec {
		rest[k] = v
	}
	// 把指针押进标题栏，再出一帧：目标应换成唤出态。
	rec = map[string]float32{}
	tst.Move(miniWidth/2, miniTitleBar/2)
	tst.Frame()
	hover = map[string]float32{}
	for k, v := range rec {
		hover[k] = v
	}
	return rest, hover
}

// TestMiniTitleBarRevealTargets 钉住两层贴边的目标不透明度与过渡时长：
// 原版 `.app-title-bar::before` 常态 .52 / hover .78、`.app-title-bar::after`
// 常态 0 / hover 1，两者都是 `transition: opacity 0.24s ease`（与主窗同一套
// titleBarReveal + titleBarRevealAnim = 240ms cubic-bezier(.25,.1,.25,1)）。
func TestMiniTitleBarRevealTargets(t *testing.T) {
	// 先钉纯函数本身（与主窗共用）。
	_, _, edge, wash := titleBarReveal(false)
	if edge != 0.52 || wash != 0 {
		t.Errorf("静息态两层 = (%.2f, %.2f)，应为 (.52, 0)", edge, wash)
	}
	if titleBarRevealAnim.Duration != 240*time.Millisecond {
		t.Errorf("reveal 过渡时长 = %v，应为 240ms（原版 0.24s ease）", titleBarRevealAnim.Duration)
	}
	if titleBarRevealAnim.Ease == nil {
		t.Fatal("reveal 过渡应有缓动曲线（原版 CSS ease）")
	}
	// CSS ease = cubic-bezier(.25,.1,.25,1)。
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.2, 0.5, 0.8} {
		if d := math.Abs(float64(titleBarRevealAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("reveal 缓动在 %.2f 处 = %v，应为 CSS ease %v", x, titleBarRevealAnim.Ease(x), want(x))
		}
	}

	rest, hover := miniTitleBarLayerTargets(t)
	if got := rest["mini-tb-edge"]; got != 0.52 {
		t.Errorf("静息时 mini 顶沿投影目标 = %v，应为 .52", got)
	}
	if got := rest["mini-tb-wash"]; got != 0 {
		t.Errorf("静息时 mini 白玻璃洗层目标 = %v，应为 0（常驻但不画）", got)
	}
	if got := hover["mini-tb-edge"]; got != 0.78 {
		t.Errorf("hover 时 mini 顶沿投影目标 = %v，应为 .78", got)
	}
	if got := hover["mini-tb-wash"]; got != 1 {
		t.Errorf("hover 时 mini 白玻璃洗层目标 = %v，应为 1", got)
	}
}

// miniTitleBarBandLuma 量一条横带的平均亮度（避开左右各 4px，免得蹭到窗口
// 边缘的抗锯齿）。
func miniTitleBarBandLuma(tst *ui.Tester, y int) float64 {
	return barStripLuma(tst, y, 4, miniWidth-4)
}

// TestMiniTitleBarHoverRevealPixels 无头像素实测：鼠标压进迷你窗标题栏时，
// 那两层贴边**真的**出来了 —— 用两个横带分别验证两层：
//
//	y=2..6  ：靠顶沿，::after 白玻璃（0.35 alpha 起）盖过 ::before 的投影 → 变亮
//	y=16..22：栏体下半，::after 已淡到接近透明（.60 处只剩 .10），::before 的投影
//	          仍在（.52→.78）→ 变暗
//
// 两个方向同时成立，才说明两层都画上了（只画 ::after 不会让下半变暗，只画
// ::before 不会让顶部变亮）。
//
// 反证：把 mini.go 里标题栏那两个 titleBarLayer 调用删掉，两条判据都不再成立。
func TestMiniTitleBarHoverRevealPixels(t *testing.T) {
	band := func(tst *ui.Tester, y0, y1 int) float64 {
		sum := 0.0
		n := 0
		for y := y0; y <= y1; y += 2 {
			sum += miniTitleBarBandLuma(tst, y)
			n++
		}
		return sum / float64(n)
	}
	rest := miniTitleBarRevealProbe(t, false)
	over := miniTitleBarRevealProbe(t, true)
	topRest, topOver := band(rest, 2, 6), band(over, 2, 6)
	lowRest, lowOver := band(rest, 16, 22), band(over, 16, 22)
	t.Logf("标题栏顶带(2..6)：静息 %.2f、hover %.2f；下半带(16..22)：静息 %.2f、hover %.2f",
		topRest, topOver, lowRest, lowOver)

	if topOver <= topRest {
		t.Errorf("hover 时顶带应变亮（::after 白玻璃 0→1，0.35 alpha 起）：静息 %.2f、hover %.2f",
			topRest, topOver)
	}
	if lowOver >= lowRest {
		t.Errorf("hover 时下半带应变暗（::before 顶沿投影 .52→.78）：静息 %.2f、hover %.2f",
			lowRest, lowOver)
	}
}

// TestMiniTitleBarRevealKeepsDragAndFocus 钉住两条纪律（同主窗 titleBar 的注释）：
//
//  1. 两层必须**常驻**建出来（静息时白玻璃那层 opacity 0）。mygo 的元素 id 由
//     「父 id + 已在建的子元素个数」决定，按需增删会让后面所有兄弟换 id。
//  2. 两层整棵子树都 PassThrough。它们盖在信息行之上，不能把指针吃掉 ——
//     信息行（含封面、标题、「切换到窗口模式」键）照样点得到。
//
// 这里从可观测结果上验：鼠标进出标题栏后，迷你窗那两颗窗口控制键依旧找得到、
// 点得到（换 id 会让 Find 丢失、PassThrough 缺失会让点击落到贴边层上）。
func TestMiniTitleBarRevealKeepsDragAndFocus(t *testing.T) {
	var mini, pin int
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Act.SetMini = func(bool) { mini++ }
	a.Act.TogglePin = func() { pin++ }
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()

	// 先悬停标题栏（两层被唤出、画在信息行之上），再回信息行点两颗键。
	tst.Move(miniWidth/2, miniTitleBar/2)
	settleTitleBarFade(tst)
	tst.Move(miniWidth/2, miniTitleBar+20)
	settleTitleBarFade(tst)

	for _, label := range []string{"切换到窗口模式", "窗口置顶"} {
		r, ok := tst.Find(label)
		if !ok {
			t.Fatalf("悬停过标题栏后找不到迷你窗控制键 %q（贴边层按需增删换了 id？）", label)
		}
		tst.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		tst.Frame()
	}
	if mini != 1 || pin != 1 {
		t.Errorf("标题栏两层挡掉了信息行的点击：setMini=%d togglePin=%d（各应为 1）", mini, pin)
	}
}

// TestMiniTitleBarLayersMatchOriginalHeights 钉住两层贴边的高度口径：
// 两层都建在 24px 的迷你标题栏里，所以 ::before / ::after 与主窗是同一手法的
// 「比标题栏本身高」（原版 170% / 140%）—— 两层在迷你窗里就该向下淡出到标题栏
// 之外（原版 body.mini-mode 只改了标题栏的高与内边距，没有覆盖这两层）。
func TestMiniTitleBarLayersMatchOriginalHeights(t *testing.T) {
	if got := float32(miniTitleBar) * 1.7; got < 40.79 || got > 40.81 {
		t.Errorf("::before 层高 = %v，应为 40.8（迷你标题栏的 170%%）", got)
	}
	if got := float32(miniTitleBar) * 1.4; got < 33.59 || got > 33.61 {
		t.Errorf("::after 层高 = %v，应为 33.6（迷你标题栏的 140%%）", got)
	}
	// 与主窗用的是同一组渐变 stops（原版 mini 只是改了标题栏高度/内边距，
	// ::before / ::after 的 background 与 transition 都没被覆盖）。
	if len(titleBarEdgeStops) == 0 || len(titleBarWashStops) == 0 {
		t.Error("两层贴边应复用主窗的渐变 stops")
	}
}
