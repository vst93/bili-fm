package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（进度条悬停时间气泡的几何 / 内影对齐）测试。原版 .player-seek-bubble
// （globals.css:3160）此前只对齐了行为（淡入淡出 + 移开冻结），盒子本身还有三处
// 逐值偏离：
//
//	1. 盒高写死 18 —— 漏了 box-sizing 下 border 也是盒高的一部分（原版
//	   padding 2×2 + line-height 1.4×10 = 14 + border 1×2 = 20）；
//	2. 盒宽按 len(label)*6.2 估 —— 原版是 auto 宽度，短标签偏宽约 4px、
//	   长标签偏宽约 7px（气泡底比文字宽出一圈）；
//	3. box-shadow 只画了外投影，漏了第一层 `inset 0 1px 1px 白 .95` 顶内高光。

// TestSeekBubbleGeometryTokens 钉住气泡的几何与材质取值（原版逐条）。
func TestSeekBubbleGeometryTokens(t *testing.T) {
	if seekBubbleFont != 10 || seekBubbleLH != 1.4 {
		t.Errorf("字号/行高 = %v/%v，应为 10/1.4（原版 font-size:10px; line-height:1.4）",
			seekBubbleFont, seekBubbleLH)
	}
	if seekBubblePadY != 2 || seekBubblePadX != 6 {
		t.Errorf("内边距 = %v/%v，应为 2/6（原版 padding:2px 6px）", seekBubblePadY, seekBubblePadX)
	}
	if seekBubbleBorder != 1 || seekBubbleRadius != 7 {
		t.Errorf("描边/圆角 = %v/%v，应为 1/7", seekBubbleBorder, seekBubbleRadius)
	}
	// 盒高 = padding 上下 4 + 行盒 14 + border 上下 2 = 20（此前写死 18）。
	if seekBubbleH != 20 {
		t.Errorf("盒高 = %v，应为 20（padding 4 + 行盒 14 + border 2，原版 box-sizing:border-box）",
			seekBubbleH)
	}
	// 文本行盒压到 line-height 1.4×10 = 14，half-leading =（14 − 自然行盒）/2。
	// 气泡里是纯数字/冒号（拉丁），自然行盒取 Inter 的度量（本机 13），所以
	// half-leading 是正的 0.5（字形下移半像素）；旧系统字体是 15 → −0.5。
	if h, sh := textLineBox(seekBubbleFont, seekBubbleLH); h != 14 || sh != (14-naturalLineBox(seekBubbleFont))/2 {
		t.Errorf("textLineBox(10,1.4) = %v/%v，应为 14/（14−自然行盒）/2", h, sh)
	}
	// 顶内高光：inset 0 1px 1px 白 .95。
	if got := seekBubbleTopHighlight(); got != ui.Hex("#ffffff").Alpha(0.95) {
		t.Errorf("顶内高光 = %v，应为白 .95（inset 0 1px 1px）", got)
	}
}

// TestSeekBubbleWidth 钉住盒宽换算：盒宽 = 实测文字宽 + 左右 padding 2×6 + 左右
// border 1×2。用桩函数给一个已知的文字宽，避免依赖具体字体。
func TestSeekBubbleWidth(t *testing.T) {
	measure := func(s string) float32 { return float32(len(s)) * 10 }
	if got := seekBubbleWidth(measure, "0:48"); got != 4*10+14 {
		t.Errorf("seekBubbleWidth(0:48) = %v，应为 4×10+14 = %v", got, 4*10+14)
	}
	if got := seekBubbleWidth(measure, ""); got != 14 {
		t.Errorf("空文案盒宽 = %v，应为 14（只剩 padding+border）", got)
	}
}

// seekBubbleProbe2 悬停到进度条 1/4 处，返回 App、进度条矩形、气泡文案矩形与
// tester。文案与 seekbubblefade_test 用的是同一条（0:48）。
func seekBubbleProbe2(t *testing.T) (*App, ui.Rect, ui.Rect, *ui.Tester) {
	t.Helper()
	a, tst := playerBarProbe(t, nil)
	bar, ok := tst.Find("播放进度")
	if !ok {
		t.Fatal("播放栏里找不到进度条")
	}
	tst.Move(bar.X+bar.W*0.25, bar.Y+bar.H/2)
	tst.Advance(200 * time.Millisecond)
	tst.Frame()
	lbl, ok := tst.Find("0:48")
	if !ok {
		t.Fatalf("悬停后没有时间气泡；文案=%v", tst.Texts())
	}
	return a, bar, lbl, tst
}

// TestSeekBubbleWidthMeasured 无头实测：气泡盒宽来自**实测文字宽**（原版 auto
// 宽度），不再是用 len(label)*6.2 估算的经验值；文字盒压回原版行盒 14。
//
// 反证：把 seekBubbleWidth 换回 len(label)*6.2+14，盒宽会变成 4×6.2+14 = 38.8
// （比实测 35 宽约 4px），第一条断言红。
func TestSeekBubbleWidthMeasured(t *testing.T) {
	a, bar, lbl, _ := seekBubbleProbe2(t)

	var tw float32
	tst := ui.NewTester(func(c *ui.Context) {
		tw, _ = c.MeasureText(0, ui.Span{
			Text: a.seekBubbleLabel, Size: seekBubbleFont, Weight: 650, Features: tabularNums[0],
		})
	}, 100, 100)
	tst.Frame()

	if want := tw + 14; math.Abs(float64(a.seekBubbleWidth-want)) > 0.01 {
		t.Errorf("盒宽 = %v，应为实测文字宽 %v + 14 = %v", a.seekBubbleWidth, tw, want)
	}
	if math.Abs(float64(lbl.W-tw)) > 0.01 {
		t.Errorf("文字矩形宽 = %v，应为实测文字宽 %v", lbl.W, tw)
	}
	// 旧的经验值估算（len*6.2+14）应被丢弃 —— 它比实测值宽。
	if old := float32(len(a.seekBubbleLabel))*6.2 + 14; a.seekBubbleWidth >= old {
		t.Errorf("盒宽 %v 仍 ≥ 旧估算 %v：没有改成实测文字宽", a.seekBubbleWidth, old)
	}
	// 文字盒压回原版行盒（line-height 1.4×10 = 14）。
	if lbl.H != 14 {
		t.Errorf("文字盒高 = %v，应为 14（原版 line-height 1.4×10）", lbl.H)
	}
	// 文字在气泡里水平居中：文字中心 ≈ 气泡盒中心（进度条左 + 缓存的左边界 + 宽/2）。
	// 气泡的左边界是相对进度条的，换算到窗口坐标要加进度条自身的 X。
	boxCenter := bar.X + a.seekBubbleLeft + a.seekBubbleWidth/2
	if d := math.Abs(float64(lbl.X + lbl.W/2 - boxCenter)); d > 0.75 {
		t.Errorf("文字水平偏移 %v（应居中于气泡盒）", d)
	}
}

// bubbleFrame 渲染一帧悬停气泡，返回该帧的拷贝、气泡盒矩形与顶/底两条取样带
// （盒内 y 方向：顶部 1px 内高光 vs 底部 1px）。
func bubbleFrame(t *testing.T, highlight func() ui.Color) (*image.RGBA, image.Rectangle, image.Rectangle) {
	t.Helper()
	orig := seekBubbleTopHighlight
	seekBubbleTopHighlight = highlight
	defer func() { seekBubbleTopHighlight = orig }()

	a, bar, lbl, tst := seekBubbleProbe2(t)
	img := cloneRGBA(tst.Image())
	// 气泡盒：左 = 进度条左 + 缓存的左边界；上 = 文字顶 −（border 1 + padding 2）。
	box := ui.Rect{X: bar.X + a.seekBubbleLeft, Y: lbl.Y - 3, W: a.seekBubbleWidth, H: seekBubbleH}
	top := image.Rect(int(box.X)+1, int(box.Y)+1, int(box.X+box.W)-1, int(box.Y)+3)
	bottom := image.Rect(int(box.X)+1, int(box.Y+box.H)-3, int(box.X+box.W)-1, int(box.Y+box.H)-1)
	return img, top, bottom
}

// TestSeekBubbleTopHighlightWired 换桩反证顶内高光真的接进了绘制：把
// seekBubbleTopHighlight 换成全透明，气泡**顶缘内侧**那一条明显变暗，而底缘
// 附近几乎不变（原版这层 inset 只压在顶缘、没有底部对应项）。
func TestSeekBubbleTopHighlightWired(t *testing.T) {
	real, top, bottom := bubbleFrame(t, func() ui.Color { return ui.Hex("#ffffff").Alpha(0.95) })
	stub, _, _ := bubbleFrame(t, func() ui.Color { return ui.Transparent })

	if n := regionDiff(real, stub, top); n == 0 {
		t.Error("顶缘像素与去掉内高光时完全一致：inset 0 1px 1px 白 .95 没接上")
	}
	if n := regionDiff(real, stub, bottom); n != 0 {
		t.Errorf("底缘像素也变了（%d 处）：内高光应只压顶缘", n)
	}
	if regionBrightness(real, top) <= regionBrightness(stub, top) {
		t.Error("带内高光的顶缘不比去掉时亮：内高光方向/颜色不对")
	}
}

// regionBrightness 求 rect 内 RGB 之和（用于比较「哪一帧更亮」）。
func regionBrightness(img *image.RGBA, rect image.Rectangle) int {
	sum := 0
	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			i := img.PixOffset(x, y)
			sum += int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
		}
	}
	return sum
}
