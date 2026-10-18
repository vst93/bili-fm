package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（迷你窗对照 main 原版）④ 的测试：迷你信息行的拖动覆盖范围。
//
// 原版 `#min-video-info` 带 `data-tauri-drag-region="deep"`：整行（封面 / 标题 /
// 选集 / 窗口控制列）都是拖动区，deep 语义下按钮仍优先收点击。mygo 的信息行
// Row 带 `DragWindow()`，语义一致 —— 这里把两件事钉住：
//
//	命中链会沿父级上溯（ui/input.go 的 `appendHitChain`：从命中的元素一路
//	    append 到根，再选第一个 interactive 的元素作 target），所以带 DragWindow
//	   的**行**会把「行里任何非交互子元素」都带进拖动 —— 等价于 deep；
//	行里那些只是显示的子元素（封面底的 Box、封面图、高光层、标题、状态点、
//	   选集文案）不能让指针停在它们身上：封面 Box 与高光层标了 DragWindow /
//	   PassThrough，图片与文字跟着 DragWindow 的祖先走（同主窗标题栏
//	   `titleBarLayer` 的注释）。

func miniSource(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("读 %s 失败：%v", name, err)
	}
	return strings.Join(strings.Fields(string(body)), "")
}

// TestMiniInfoRowIsDeepDragRegion 钉住信息行的 Row 自己带 DragWindow（原版那一行
// 的 `data-tauri-drag-region="deep"` 对应物），且**同时**带 Radius/Opacity 一类的
// 视觉子元素从命中的祖先拿拖动 —— 只要 Row 上是 DragWindow，行里任何空白处按下
// 都会起拖（命中链上溯到它）。
func TestMiniInfoRowIsDeepDragRegion(t *testing.T) {
	src := miniSource(t, "mini.go")
	// 信息行：Height(miniInfoH) … DragWindow() … Children（DragWindow 在 Children 之前）。
	row := "ui.Row(c).FillWidth().Height(miniInfoH).Shrink(0).Padding(6,miniInfoPadH)." +
		"Gap(miniInfoGap).AlignItems(ui.Center).DragWindow().Children(func(){"
	if !strings.Contains(src, row) {
		t.Errorf("信息行不再带 DragWindow（原版是 data-tauri-drag-region=deep）：找不到 %q", row)
	}
	// 标题栏整条也是拖动区（迷你标题栏同时装了自绘的最小化/关闭键，
	// 原版 mini 下 `#wds-handle-group` 保持可见；键本身是交互元素，
	// 命中链会选它而不是拖动，空白处仍归标题栏）。
	if !strings.Contains(src, "ui.Row(c).FillWidth().Height(miniTitleBar).Shrink(0)."+
		"Padding(0,miniTitleBarPadX).DragWindow()") {
		t.Error("迷你标题栏不再带 DragWindow")
	}
}

// TestMiniInfoRowDisplayChildrenAreNotInteractive 钉住信息行里那些**只是显示**的
// 子元素不会把拖动吃成别的目标：封面底是普通 `ui.Box`（没有 Clicked/TrackPointer
// 之类的交互标志）、高光层是 PassThrough 的 Absolute Box。
//
// 只要行里这些子元素都不是「交互元素」，mygo 的 `appendHitChain`（ui/input.go：
// 从命中的元素一路 append 到根）就会把指针引到带 DragWindow 的那一行上 ——
// 这就是原版 `data-tauri-drag-region="deep"` 的语义。
func TestMiniInfoRowDisplayChildrenAreNotInteractive(t *testing.T) {
	src := miniSource(t, "mini.go")
	if !strings.Contains(src, "ui.Box(c).Size(miniCover,miniCover).Shrink(0).Radius(10)."+
		"Background(ui.Hex(\"#ffffff\").Alpha(0.58)).") {
		t.Error("封面底不再是普通 ui.Box（换了会吃指针的容器就要重核 deep 拖动语义）")
	}
	if !strings.Contains(src, "ui.Box(c).Absolute().Fill().PassThrough().LinearGradient(miniCoverShine())") {
		t.Error("封面高光层不再 PassThrough —— 它会吃掉封面上的指针，拖动区缺一块")
	}
}

// TestMiniInfoRowBlankDragDoesNotDrift 无头实测「deep 语义下按钮优先收点击、
// 行里空白处点击不会漂到任何控件」：在信息行的**空隙**（封面右缘到标题列之间，
// x≈77..86）按下再松开，不该触发任何动作（窗口模式 / 置顶 / 播放 / 下一集）。
//
// 这一条同时是「deep 语义没有被改成 hit-through」的反证：若为了补拖动把 DragWindow
// 挪到封面以外的兄弟上、而空隙落到别的可点元素（例如某颗键）上，计数就不再是 0。
func TestMiniInfoRowBlankDragDoesNotDrift(t *testing.T) {
	var setMini, pin, play, next int
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Act.SetMini = func(bool) { setMini++ }
	a.Act.TogglePin = func() { pin++ }
	a.Act.TogglePlay = func() { play++ }
	a.Act.Next = func() { next++ }
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()

	// 封面占 x=12..76，标题列从 x=87 起（miniInfoPadH 12 + 封面 64 + gap 11）。
	gapX := float32(miniInfoPadH+miniCover) + miniInfoGap/2
	for _, y := range []float32{miniTitleBar + 30, miniTitleBar + 60} {
		tst.ClickAt(gapX, y)
		tst.Frame()
	}
	if setMini+pin+play+next != 0 {
		t.Errorf("信息行空隙里点击漂到了控件：setMini=%d pin=%d play=%d next=%d",
			setMini, pin, play, next)
	}
	// 反过来：行里**控件**照旧点得到（deep 语义 = 按钮优先），这也是「空隙没有
	// 被某颗键盖住」的正面证据。
	r, ok := tst.Find("窗口置顶")
	if !ok {
		t.Fatal("找不到置顶键")
	}
	tst.ClickAt(r.X+r.W/2, r.Y+r.H/2)
	tst.Frame()
	if pin != 1 {
		t.Errorf("置顶键点击没有生效（pin=%d），deep 语义下按钮应优先收点击", pin)
	}
}
