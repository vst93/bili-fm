package view

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放列表空态对齐原版）的测试。
//
// 原版 playlist.tsx 的空态是：
//
//	<div className="flex flex-col items-center justify-center h-full text-gray-400">
//	  <p className="text-sm">播放列表为空</p>
//	  <p className="text-xs mt-1">从选集列表中点击 + 添加选集到播放列表</p>
//	</div>
//
// `h-full` + `justify-center` 让文案在**整个正文里竖直居中**（DrawerBody 是
// flex:1 + overflow-y:auto）。原生版此前把这两行放在 `ui.Scroll` 的内容流里、
// 只给了一段 `Margin(20,0,0,0)` 的近似：文案压在表头下约 28px（800×600 无头
// 实测文案块中心 156，原版 348），下面空出一大片。
//
// 两行还各是 Tailwind 的字号类：text-sm（14px / 20px 行高）、text-xs
// （12px / 16px 行高），副行有 `mt-1`（4）的间距。框架的自然行盒是 22 / 18，
// 所以这里也套 textLineBox 压回（见 linebox.go 的 textSmLH / textXsLH）。

// emptyDrawerBodyTop 返回播放列表抽屉正文（DrawerBody）的上下缘：面板高
// min(92vh, 100vh-54)、贴窗底，表头固定 48。
func emptyDrawerBodyTop(winH float32) (top, bottom float32) {
	h := min(winH*0.92, winH-54)
	return winH - h + 48, winH
}

// emptyStateRects 打开「我的列表」空态的播放列表抽屉，返回两行文案的矩形。
func emptyStateRects(t *testing.T, winW, winH int) (l1, l2 ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = true
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListUser
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, winW, winH)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()

	var ok bool
	l1, ok = tt.Find("播放列表为空")
	if !ok {
		t.Fatal("找不到空态首行「播放列表为空」")
	}
	l2, ok = tt.Find("从选集列表中点击 + 添加选集到播放列表")
	if !ok {
		t.Fatal("找不到空态副行")
	}
	return l1, l2
}

// TestPlaylistEmptyCenteredInBody 是整窗无头实测：空态文案块的中心应落在
// 正文（表头下缘到窗底）的正中。
//
// **反证**：把空态改回 `ui.Scroll` 内容流里的 `Margin(20,0,0,0)`（旧写法）
// → 800×600 下文案块中心掉到 156 附近（差 ~190），本测试红。
func TestPlaylistEmptyCenteredInBody(t *testing.T) {
	for _, winH := range []int{600, 700, 820} {
		l1, l2 := emptyStateRects(t, 800, winH)
		top, bottom := emptyDrawerBodyTop(float32(winH))
		got := (l1.Y + l2.Y + l2.H) / 2 // 文案块的视觉中心
		want := (top + bottom) / 2
		t.Logf("%dpx 高：文案块中心 %.1f，正文中心 %.1f（正文 %.0f..%.0f）", winH, got, want, top, bottom)
		if d := math.Abs(float64(got - want)); d > 2 {
			t.Errorf("%dpx 高：空态文案块中心 = %.1f，应约 %.1f（正文里竖直居中，差 %.1f）",
				winH, got, want, d)
		}
	}
}

// TestPlaylistEmptyLineBoxes 量两行的盒高与间距：text-sm 14/20 ⇒ 20、
// text-xs 12/16 ⇒ 16、副行 `mt-1` ⇒ 4。
//
// 行盒位移会让 Finder 报的矩形与布局盒差半个像素（首行 shift −0.5、副行
// shift −1），所以间距按 4 给 1px 容差；盒高不受位移影响、逐值钉住。
func TestPlaylistEmptyLineBoxes(t *testing.T) {
	l1, l2 := emptyStateRects(t, 800, 600)
	if math.Abs(float64(l1.H-20)) > 0.01 {
		t.Errorf("首行盒高 %v，want 20（text-sm 14/20）", l1.H)
	}
	if math.Abs(float64(l2.H-16)) > 0.01 {
		t.Errorf("副行盒高 %v，want 16（text-xs 12/16）", l2.H)
	}
	gap := l2.Y - (l1.Y + l1.H)
	t.Logf("首行 %+v / 副行 %+v / 间距 %v", l1, l2, gap)
	if math.Abs(float64(gap-4)) > 1 {
		t.Errorf("两行间距 %v，want 4（原版副行 mt-1）", gap)
	}
	// 两块文字都水平居中（原版 items-center：两行各自作为块居中）。
	if d := math.Abs(float64((l1.X + l1.W/2) - (l2.X + l2.W/2))); d > 0.5 {
		t.Errorf("两行中心 x 差 %v，应同为窗口中心（原版 items-center）", d)
	}
}

// TestPlaylistEmptySeriesTabCentered 钉住合集 tab 的空态也走同一个「正文居中」
// 容器：这一支是本 fork 自己写的一句文案（原版两个 tab 共用「播放列表为空」），
// 但位置与主列表空态一致。
func TestPlaylistEmptySeriesTabCentered(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.LoggedIn = true
	a.Drawer = DrawerPlaylist
	a.PlaylistTab = ListSeries
	tt := settleTester(ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600))
	r, ok := tt.Find("合集列表为空（在合集里点「播放全部」加载）")
	if !ok {
		t.Fatal("找不到合集 tab 的空态文案")
	}
	top, bottom := emptyDrawerBodyTop(600)
	got, want := r.Y+r.H/2, (top+bottom)/2
	if d := math.Abs(float64(got - want)); d > 2 {
		t.Errorf("合集 tab 空态中心 = %.1f，应约 %.1f（差 %.1f）", got, want, d)
	}
}
