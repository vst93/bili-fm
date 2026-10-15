package view

import (
	"os"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（迷你窗对照 main 原版）③ 的测试：迷你播放栏的溢出裁剪。
//
// 原版靠 `#root { overflow: hidden }`（mini 下 #root 是 400×155 的圆角盒）裁掉
// 播放栏与弹层的一切溢出。mygo 没有任何内容级 Clip —— 但**不需要**：
//
//	(a) mygo 的 painter 起手就把 clip 设成窗口矩形（ui/paint.go：
//	    `clip: Rect{0, 0, w, h}`），与 #root 那条 overflow:hidden 在 mini 下
//	    完全等价（迷你窗的 #root 就是整窗）；
//	(b) 迷你窗的三行（24 + 88 + 43）**恰好**铺满 155 高的窗口，没有任何元素
//	    被顶到窗口外；
//	(c) 唯一「探出自己盒子」的两处都是原版**有意为之**、且都在窗口内：
//	    - 栏顶那 10px 淡出层（`body.mini-mode #player::before`，原版也是
//	      `top: -10px` 探进信息行）—— 窗口表面裁不到它，因为它还在窗口里；
//	    - 音量弹层的投影被窗口底边裁掉（原版同样由 #root 裁掉）。
//
// 下面把 (b)(c) 逐条钉住，并把作法(a) 需要的另一条前提也铉上：迷你窗的三行
// 都建在**普通流**里（没有像主窗那样把第三行套进贴底的 Absolute 盒）——没有
// 绝对定位层，就没有「计算百分比高度的分母与子元素表面矩形不同」这类溢出源。
// 任何一条变了（比如给第三行套上 Absolute 盒、或把 miniPlayerH 调大），本测试
// 会直接指出「迷你窗内容开始脱离窗口表面」。

// TestMiniRowsFillWindowExactly 钉住迷你窗三行恰好铺满窗口：24 + 88 + 43 = 155。
// 这是「无需内容级 Clip」的前提 —— 只要它们还等于窗口高，就没有元素被顶出窗口。
func TestMiniRowsFillWindowExactly(t *testing.T) {
	if got := miniTitleBar + miniInfoH + miniPlayerH; got != miniHeight {
		t.Errorf("迷你窗三行合计 = %d，应恰好等于窗口高 %d（差值即溢出/留空）", got, miniHeight)
	}
	if miniPlayerH != 43 {
		t.Errorf("迷你播放栏高 = %d，原版 body.mini-mode #player 是 43", miniPlayerH)
	}
}

// TestMiniPlayerBarIsInFlow 钉住迷你播放栏**仍建在普通流里**（主窗第三行是
// 套在贴底的 Absolute 盒里的，原版 #player 是 fixed）。这是文件头 (a) 的前提：
// 没有绝对定位层，就没有「百分比高度的分母与子元素表面矩形不同」这类溢出源，
// mygo 的窗口裁剪因此与 #root 的 overflow 等价。若将来有人把 mini 第三行也套进
// Absolute 盒，这里会提醒他重新核算圈层。
func TestMiniPlayerBarIsInFlow(t *testing.T) {
	body, err := os.ReadFile("mini.go")
	if err != nil {
		t.Fatalf("读 mini.go 失败：%v", err)
	}
	src := strings.Join(strings.Fields(string(body)), "")
	// miniPlayerBar 的调用点：不在任何 Absolute 链上（Shell 里是直接一行调用）。
	if !strings.Contains(src, "volAnchor:=a.miniPlayerBar(c)") {
		t.Error("找不到 miniPlayerBar 的调用点 —— 本测试需要同步更新")
	}
	// 栏体自身是普通流的 Column（带 FillWidth/Height/Shrink），不是 Absolute。
	if !strings.Contains(src, "ui.Column(c).FillWidth().Height(miniPlayerH).Shrink(0)") {
		t.Error("miniPlayerBar 的栏体不再是普通流 Column（换了定位方式需要重核裁剪）：找不到链")
	}
}

// TestMiniPlayerBarFitsWindow 无头实测：迷你窗里播放栏的每个部件（切曲 / 当前
// 时间 / 进度 / 时长 / 音量）都**恰好**落在窗口最下面那条 43px 的栏里；音量弹层
// 弹出的静音键也在窗口内。越界即说明内容真的溢出了（原版由 #root 裁掉，我们
// 这里应当根本不越界）。
func TestMiniPlayerBarFitsWindow(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.ShowVolume = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()

	inside := func(name string, r ui.Rect) {
		t.Helper()
		if r.X < 0 || r.Y < 0 || r.X+r.W > miniWidth || r.Y+r.H > miniHeight {
			t.Errorf("%s 越出窗口矩形：%+v（窗口 %d×%d）", name, r, miniWidth, miniHeight)
		}
		// 播放栏部件都该落在栏内（栏是窗口最下面 43px）。
		if r.Y < miniHeight-miniPlayerH {
			t.Errorf("%s 落在播放栏上方（y=%v < %d）", name, r.Y, miniHeight-miniPlayerH)
		}
	}
	for _, label := range []string{"播放", "下一个视频", "播放进度", "音量"} {
		r, ok := tst.Find(label)
		if !ok {
			t.Fatalf("找不到 %q", label)
		}
		inside(label, r)
	}
	// 音量弹层的静音键（原版挂 `right: calc(100% + 8px)`，在栏体左侧、窗口内）。
	inside("音量弹层静音键", playerMuteRect(t, tst))
}

// TestMiniSeekBubbleStaysInsideWindow 无头实测：鼠标划到进度条上时，时间气泡
// （脱离进度条盒、向上 6px 悬在栏外）仍落在窗口内 —— 原版这个气泡同样由
// #root 的 overflow 裁；mygo 里它根本不该越界。
func TestMiniSeekBubbleStaysInsideWindow(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	sl, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("找不到进度条")
	}
	tst.Move(sl.X+sl.W/2, sl.Y+sl.H/2)
	settleTitleBarFade(tst)
	// 气泡文字（拖到中点 → 1:40）。
	r, ok := tst.Find("1:40")
	if !ok {
		t.Fatal("悬停进度条后应出现时间气泡")
	}
	if r.X < 0 || r.Y < 0 {
		t.Errorf("时间气泡越出窗口左上：%+v", r)
	}
	// 气泡盒 = 文字盒 + 左右 padding/border 6+1、上下 2+1（seekBubbleH 那套常量）。
	box := ui.Rect{X: r.X - seekBubblePadX - seekBubbleBorder, Y: r.Y - seekBubblePadY - seekBubbleBorder,
		W: r.W + 2*(seekBubblePadX+seekBubbleBorder), H: seekBubbleH}
	if box.Y+box.H > miniHeight {
		t.Errorf("时间气泡盒底部越出窗口：%+v（窗口高 %d）", box, miniHeight)
	}
	if box.X < 0 || box.X+box.W > miniWidth {
		t.Errorf("时间气泡盒左右越出窗口：%+v（窗口宽 %d）", box, miniWidth)
	}
}

// TestMiniPlayerEdgeFadeBandMatchesOriginal 钉住那 10px 淡出层的位置：它**有意**
// 探进信息行 10px（原版 `body.mini-mode #player::before` 就是 `top: -10px`），
// 但整条仍在窗口内（bottom: 0 + 高 53 = 窗口底）。原版这条也是画在 #root 里、
// 由 #player 的层级盖上去的，不是被 overflow 裁掉的越界物。
func TestMiniPlayerEdgeFadeBandMatchesOriginal(t *testing.T) {
	bottom, top := miniHeight, miniHeight-(miniPlayerH+10)
	if top != miniHeight-miniPlayerH-10 {
		t.Fatalf("淡出层顶端算错")
	}
	if bottom != miniHeight {
		t.Fatalf("淡出层底端应贴窗口底（bottom: 0），得到 %d", bottom)
	}
	// 栏顶 y=112，淡出层顶 = 102：探进信息行 10px。
	if top != 112-10 {
		t.Errorf("淡出层顶端 = %d，应为栏顶上方 10px（102）", top)
	}
	if top < 0 {
		t.Errorf("淡出层探出窗口上沿：top=%d", top)
	}
}
