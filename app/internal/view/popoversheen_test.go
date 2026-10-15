package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（音量 / 倍速弹层的整面洗光）测试。原版两层弹层都靠 `background-image`
// 铺一条洗光：
//
//	音量弹层  浅色 --glass-sheen-static，深色（烟熏玻璃时段）换成 --player-sheen；
//	倍速弹层  浅色没有 background-image（只有白 .82 底），深色换成 --player-sheen。
//
// 原生版此前拿框架 `glass.Glass` 当整层表面，而玻璃只有**边缘**折射与 1px 描光，
// 不含这条铺满整面的渐变 —— 弹层比原版平一档。这里先钉住「哪一层哪个时段铺哪条
// 位图」，再用无头渲染证明它真的落进了弹层矩形。

// popoverSheenHit 是一次 sheen 绘制（矩形 + 位图），供反证记录。
type popoverSheenHit struct {
	r   ui.Rect
	bmp *ui.Bitmap
}

// TestPopoverSheenTokens 钉住四组合的选层：音量弹层浅色走 --glass-sheen-static、
// 深色换 --player-sheen；倍速弹层浅色没有洗光（nil）、深色 --player-sheen。
func TestPopoverSheenTokens(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	dark := ThemeFor(PeriodAt(0))

	if got := volumePopoverSheen(light); got != sheenGlassBaked {
		t.Errorf("音量弹层浅色应铺 --glass-sheen-static，得到 %p", got)
	}
	if got := volumePopoverSheen(dark); got != sheenDarkBaked {
		t.Errorf("音量弹层深色应铺 --player-sheen，得到 %p", got)
	}
	if got := speedPopoverSheen(light); got != nil {
		t.Errorf("倍速弹层浅色原版没有 background-image，应返回 nil，得到 %p", got)
	}
	if got := speedPopoverSheen(dark); got != sheenDarkBaked {
		t.Errorf("倍速弹层深色应铺 --player-sheen，得到 %p", got)
	}
}

// recordPopoverSheen 渲染一遍播放栏（只渲染播放栏，避开视频详情区那颗固定走
// 浅色的选集胶囊），记录每次 sheen 绘制的矩形与位图。换桩后恢复。
func recordPopoverSheen(t *testing.T, mutate func(a *App)) []popoverSheenHit {
	t.Helper()
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	var got []popoverSheenHit
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		got = append(got, popoverSheenHit{r, bmp})
		orig(p, r, radius, bmp)
	}
	playerBarOnlyProbe(t, mutate)
	return got
}

// TestVolumePopoverSheenWired：浅色音量弹层（184×42）必须铺 --glass-sheen-static
// 位图、深色换成 --player-sheen，且按时段不同真的换层（不是同一张）。
func TestVolumePopoverSheenWired(t *testing.T) {
	find := func(hits []popoverSheenHit) (popoverSheenHit, bool) {
		for _, h := range hits {
			if h.r.W == volumePopoverW && h.r.H == volumePopoverH {
				return h, true
			}
		}
		return popoverSheenHit{}, false
	}

	light := recordPopoverSheen(t, func(a *App) { a.ShowVolume = true })
	lh, ok := find(light)
	if !ok {
		t.Fatalf("浅色音量弹层（%d×%d）没铺洗光；记录到 %d 条绘制", volumePopoverW, volumePopoverH, len(light))
	}
	if want := sheenGlassBaked.bitmapFor(lh.r.W, lh.r.H); lh.bmp != want {
		t.Errorf("浅色音量弹层铺的不是 --glass-sheen-static 位图（宽度 %g）", lh.r.W)
	}

	dark := recordPopoverSheen(t, func(a *App) {
		a.ShowVolume = true
		a.Theme = ThemeFor(PeriodAt(0))
	})
	dh, ok := find(dark)
	if !ok {
		t.Fatalf("深色音量弹层没铺洗光；记录到 %d 条绘制", len(dark))
	}
	if want := sheenDarkBaked.bitmapFor(dh.r.W, dh.r.H); dh.bmp != want {
		t.Errorf("深色音量弹层铺的不是 --player-sheen 位图（宽度 %g）", dh.r.W)
	}
	if lh.bmp == dh.bmp {
		t.Error("浅色 / 深色音量弹层铺了同一张位图，等于没按时段换层")
	}
}

// TestSpeedPopoverSheenWired：倍速弹层浅色不铺洗光（开弹层不新增面板尺寸的绘制），
// 深色补一条 --player-sheen 位图。面板高 = 档位胶囊的 auto 高度（11px 自然
// 行盒 + 上下 padding 8，原版 .player-speed-option 是 padding:4px 8px 无固定高）
// + 弹层上下 padding 4×2；面板很宽（7 个档位），用它把面板与 32×32 的播放键区分开。
func TestSpeedPopoverSheenWired(t *testing.T) {
	panelH := naturalLineBox(11) + 8 + 8
	panelHit := func(hits []popoverSheenHit) (popoverSheenHit, bool) {
		for _, h := range hits {
			if d := h.r.H - panelH; d > 0.01 || d < -0.01 {
				continue
			}
			if h.r.W > 120 {
				return h, true
			}
		}
		return popoverSheenHit{}, false
	}

	if _, ok := panelHit(recordPopoverSheen(t, func(a *App) { a.ShowSpeed = true })); ok {
		t.Error("浅色倍速弹层原版没有 background-image，不应铺洗光")
	}

	dark := func(a *App) {
		a.Theme = ThemeFor(PeriodAt(0))
		a.ShowSpeed = true
	}
	h, ok := panelHit(recordPopoverSheen(t, dark))
	if !ok {
		t.Fatal("深色倍速弹层没铺洗光")
	}
	if want := sheenDarkBaked.bitmapFor(h.r.W, h.r.H); h.bmp != want {
		t.Errorf("深色倍速弹层（%g×%g）铺的不是 --player-sheen 位图", h.r.W, h.r.H)
	}
}

// TestVolumePopoverSheenBrightens 无头实测（直接因果）：把 playerSheenPaint 换成
// 不画的桩后，深色音量弹层整块区域的总亮度必须下降 —— 证明那条洗光真的画进了
// 弹层矩形（不是只记了个调用）。
func TestVolumePopoverSheenBrightens(t *testing.T) {
	dark := func(a *App) {
		a.ShowVolume = true
		a.Theme = ThemeFor(PeriodAt(0))
	}

	luma := func(tst *ui.Tester) int {
		anchor, ok := tst.Find("音量")
		if !ok {
			t.Fatal("播放栏里找不到音量键")
		}
		r := volumePopoverRect(anchor, volumePopoverW, volumePopoverH, volumePopoverEdgeGap)
		img := tst.Image()
		sum := 0
		for y := int(r.Y) + 2; y < int(r.Y+r.H)-2; y++ {
			for x := int(r.X) + 2; x < int(r.X+r.W)-2; x++ {
				c := rgbAt(img, x, y)
				sum += c[0] + c[1] + c[2]
			}
		}
		return sum
	}

	_, on := playerBarProbeAt(t, PeriodAt(0), dark)
	with := luma(on)

	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {}
	_, off := playerBarProbeAt(t, PeriodAt(0), dark)
	without := luma(off)

	t.Logf("深色音量弹层总亮度：有洗光=%d 无洗光=%d", with, without)
	if with <= without {
		t.Errorf("深色音量弹层的洗光没有抬高区域亮度：有=%d 无=%d", with, without)
	}
}
