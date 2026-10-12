package view

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// cardGlowProbe 把一张卡放在固定位置的纯色底上，供逐像素验证光晕。
// 背景取中灰：卡片玻璃叠上去仍是浅灰，白色光晕叠上来能看出亮度差。
func cardGlowProbe(t *testing.T) (*ui.Tester, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	spec := cardSpec{
		Key:    "glow-card",
		Title:  "标题",
		Fields: []cardField{{Kind: fieldAuthor, Text: "UP主"}},
	}
	tst := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.RGB(90, 90, 90))
		ui.Box(c).Absolute().Left(20).Top(20).Size(220, 170).Children(func() {
			a.renderCard(c, spec)
		})
	}, 300, 260)
	tst.Frame()
	return tst, ui.Rect{X: 20, Y: 20, W: 220, H: 170}
}

// rgbAt 读一个像素的三个通道。
func rgbAt(img *image.RGBA, x, y int) [3]int {
	i := img.PixOffset(x, y)
	return [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
}

// TestCardGlowLayerAlpha 钉住同心圆近似的 alpha 补偿公式：普通叠加下把累计
// 不透明度从 prev 补到 want 需要 (want-prev)/(1-prev)；want 不高于 prev 时
// 补 0（渐变越往内越亮，外层永远不需要再压暗）。
func TestCardGlowLayerAlpha(t *testing.T) {
	if got := cardGlowLayerAlpha(0, 0.6); math.Abs(float64(got-0.6)) > 1e-6 {
		t.Errorf("首层 = %v，应为 .6", got)
	}
	// 已铺 .6，补到 .8：inc = (.8-.6)/(1-.6) = .5 → 累计 .6 + .5*.4 = .8。
	if got := cardGlowLayerAlpha(0.6, 0.8); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Errorf("补偿层 = %v，应为 .5", got)
	}
	if got := cardGlowLayerAlpha(0.8, 0.6); got != 0 {
		t.Errorf("目标低于已铺值时应补 0，得到 %v", got)
	}
	if got := cardGlowLayerAlpha(1, 1); got != 0 {
		t.Errorf("已完全不透明时应补 0，得到 %v", got)
	}
}

// TestCardGlowRadialFalloff 是无头渲染实测：光晕从圆心向外单调变淡，到
// 0.7R（140px）处完全消失（原版 transparent 70%），中途有天空蓝的色相。
func TestCardGlowRadialFalloff(t *testing.T) {
	const bg = 120
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.RGB(bg, bg, bg))
		ui.Box(c).Absolute().Left(0).Top(0).Size(360, 280).
			Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, 180, 140) })
	}, 400, 300, 1)

	center := grayAt(img, 180, 140)
	r40 := grayAt(img, 180, 100)
	r90 := grayAt(img, 180, 50)
	r140 := grayAt(img, 180, 0)
	r180 := grayAt(img, 0, 140)
	t.Logf("圆心=%d r40=%d r90=%d r140=%d r180=%d（底 %d）",
		center, r40, r90, r140, r180, bg)
	if center <= r40 || r40 <= r90 {
		t.Fatalf("光晕没有向外单调变淡：圆心=%d r40=%d r90=%d", center, r40, r90)
	}
	if center-bg < 25 {
		t.Fatalf("圆心光晕太弱：%d（底 %d）", center, bg)
	}
	for _, v := range []int{r140, r180} {
		if v > bg+3 {
			t.Fatalf("0.7R 之外（>=140px）不该再有光晕：%d（底 %d）", v, bg)
		}
	}
	// r90 落在 40%~70% 段，色相是天空蓝（B > R）。
	sky := rgbAt(img, 180, 50)
	if sky[2] <= sky[0]+8 {
		t.Fatalf("中段没有天空蓝色相：rgb=%v", sky)
	}
}

// TestCardGlowClipsToCard 钉住光晕裁在卡片圆角内（原版是 overflow:hidden 卡
// 片里的 ::before）：卡片很小、圆心在卡片中心时，紧贴卡片外的像素不应被提亮。
func TestCardGlowClipsToCard(t *testing.T) {
	const bg = 120
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.RGB(bg, bg, bg))
		ui.Box(c).Absolute().Left(120).Top(90).Size(120, 90).
			Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, 60, 45) })
	}, 400, 300, 1)

	inside := grayAt(img, 140, 135)  // 卡内、圆心左 40px
	outLeft := grayAt(img, 110, 135) // 卡片左缘外 10px
	outTop := grayAt(img, 180, 80)   // 卡片上缘外 10px
	t.Logf("卡内=%d 左外=%d 上外=%d（底 %d）", inside, outLeft, outTop, bg)
	if inside <= bg+8 {
		t.Fatalf("卡片内应被光晕提亮：%d（底 %d）", inside, bg)
	}
	if outLeft > bg+3 || outTop > bg+3 {
		t.Fatalf("光晕溢出了卡片：左外=%d 上外=%d（底 %d）", outLeft, outTop, bg)
	}
}

// TestCardGlowFollowsPointer 是整卡实测：光晕跟着指针走——两个都在卡片封面
// 里的点 A/B，指针指向哪个，哪个就更亮（悬停的边框/上浮在两次之间不变，差异
// 只能来自光晕）。
func TestCardGlowFollowsPointer(t *testing.T) {
	tst, _ := cardGlowProbe(t)
	aPt := [2]int{60, 60}
	bPt := [2]int{190, 90}

	tst.Move(float32(aPt[0]), float32(aPt[1]))
	imgA := tst.Image()
	aa := grayAt(imgA, aPt[0], aPt[1])
	ba := grayAt(imgA, bPt[0], bPt[1])

	tst.Move(float32(bPt[0]), float32(bPt[1]))
	imgB := tst.Image()
	ab := grayAt(imgB, aPt[0], aPt[1])
	bb := grayAt(imgB, bPt[0], bPt[1])

	t.Logf("指针在 A：A=%d B=%d；指针在 B：A=%d B=%d", aa, ba, ab, bb)
	if aa <= ba {
		t.Errorf("指针在 A 时 A 应比 B 亮：A=%d B=%d", aa, ba)
	}
	if bb <= ab {
		t.Errorf("指针在 B 时 B 应比 A 亮：A=%d B=%d", ab, bb)
	}
	if aa-ab < 8 || bb-ba < 8 {
		t.Errorf("光晕随指针移动的亮度差太小：A %d→%d, B %d→%d", aa, ab, ba, bb)
	}

	// 指针移出卡片：光晕消失，两点的亮度回到同一水平（只剩卡片本身的底色）。
	tst.Move(280, 240)
	out := tst.Image()
	oa := grayAt(out, aPt[0], aPt[1])
	ob := grayAt(out, bPt[0], bPt[1])
	t.Logf("指针移出：A=%d B=%d", oa, ob)
	if diff := oa - ob; diff > 2 || diff < -2 {
		t.Errorf("指针移出后两点应只剩底色（相等）：A=%d B=%d", oa, ob)
	}
}
