package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（深色 ::before 的 --player-sheen）测试。深色（烟熏玻璃）时段原版
// `#player .player-button::before` 除纯色白 .085 外还铺一层
// `background-image: var(--player-sheen)`（径向 + 线性两条白色渐变）。原生版
// 此前只画纯色底，这里先钉住位图几何，再用无头渲染实测它真的进了深色键面。

// TestPlayerSheenPixelsFalloff 钉住 sheen 位图的几何：左上（径向中心在
// 18% -20%）最亮、往右下单调变淡；径向 48% 之外只剩线性层的极淡底，线性到
// 56% 也淡到 0，所以右下角应基本无光泽。峰值不超过径向白 .2（中心在盒外，
// 盒内只会更淡）。
func TestPlayerSheenPixelsFalloff(t *testing.T) {
	const n = 128
	img := playerSheenPixels(n, n)
	if img == nil {
		t.Fatal("playerSheenPixels 返回 nil")
	}
	at := func(x, y int) float32 {
		return float32(img.Pix[img.PixOffset(x, y)+3]) / 255
	}
	tl := at(0, 0)
	mid := at(n/2, n/2)
	br := at(n-1, n-1)
	t.Logf("sheen alpha：左上=%v 中=%v 右下=%v", tl, mid, br)
	if !(tl > mid && mid > br) {
		t.Errorf("sheen 应从左上往右下单调变淡：左上=%v 中=%v 右下=%v", tl, mid, br)
	}
	if tl <= 0 {
		t.Errorf("左上角应有光泽，得到 %v", tl)
	}
	if br > 0.005 {
		t.Errorf("右下角应基本无光泽（径向 48%% + 线性 56%% 之外），得到 %v", br)
	}
	maxA := float32(0)
	for i := 3; i < len(img.Pix); i += 4 {
		if a := float32(img.Pix[i]) / 255; a > maxA {
			maxA = a
		}
	}
	if maxA > 0.2 {
		t.Errorf("sheen 峰值 alpha = %v，不应超过径向白 .2", maxA)
	}
}

// TestPlayerSheenBitmapCached 钉住 sheen 位图进程内只烘焙一张（懒加载缓存）：
// 两次取到同一个指针，避免每颗键每帧都重算一张位图。
func TestPlayerSheenBitmapCached(t *testing.T) {
	b1 := playerSheenBitmap()
	b2 := playerSheenBitmap()
	if b1 == nil || b2 == nil {
		t.Fatal("playerSheenBitmap 返回 nil")
	}
	if b1 != b2 {
		t.Error("sheen 位图应进程内共用一张（懒加载缓存）")
	}
}

// TestPlayerSheenWired 无头反证：视图按当前时段选对了 `::before` 光泽位图——
// 亮色时段每颗 `#player .player-button` 铺浅色 --glass-sheen-static，深色时段
// 统一换成 --player-sheen。把两者混用（或浅色干脆不画）都会变红。
// 注意：铺的是按按钮自身宽高比烘的那张（bitmapFor），不是共用的正方形那张——
// 播放栏的倍速键 40×32、EQ / 跳过赞助键 34×32，线性层的角度得跟着盒子走。
func TestPlayerSheenWired(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	type gotHit struct {
		r   ui.Rect
		bmp *ui.Bitmap
	}
	var got []gotHit
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		got = append(got, gotHit{r, bmp})
		orig(p, r, radius, bmp)
	}

	playerBarOnlyProbe(t, nil)
	if len(got) == 0 {
		t.Fatal("亮色时段应铺浅色 sheen（playerSheenPaint 没被调用）")
	}
	for _, h := range got {
		if want := sheenGlassBaked.bitmapFor(h.r.W, h.r.H); h.bmp != want {
			t.Errorf("亮色时段 %.0f×%.0f 的键上铺的不是 --glass-sheen-static 位图", h.r.W, h.r.H)
			break
		}
	}

	got = nil
	playerBarOnlyProbe(t, func(a *App) { a.Theme = ThemeFor(PeriodAt(0)) })
	if len(got) == 0 {
		t.Fatal("深色时段应铺 --player-sheen（playerSheenPaint 没被调用）")
	}
	for _, h := range got {
		if want := sheenDarkBaked.bitmapFor(h.r.W, h.r.H); h.bmp != want {
			t.Errorf("深色时段 %.0f×%.0f 的键上铺的不是 --player-sheen 位图", h.r.W, h.r.H)
			break
		}
	}
}

// TestPlayerSheenBrightensDarkButton 无头实测（直接因果）：同一颗深色音量键，
// 挂 sheen 时键面总亮度必须高于把 playerSheenPaint 换成不画的桩时。
func TestPlayerSheenBrightensDarkButton(t *testing.T) {
	boxLuma := func(img *image.RGBA, r ui.Rect) int {
		sum := 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				c := rgbAt(img, x, y)
				sum += c[0] + c[1] + c[2]
			}
		}
		return sum
	}
	dark := func(a *App) { a.Theme = ThemeFor(PeriodAt(0)) }

	_, on := playerBarProbe(t, dark)
	on.Move(-1, -1)
	on.Frame()
	r, ok := on.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	with := boxLuma(on.Image(), r)

	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {}
	_, off := playerBarProbe(t, dark)
	off.Move(-1, -1)
	off.Frame()
	without := boxLuma(off.Image(), r)

	t.Logf("深色音量键总亮度：有 sheen=%d 无 sheen=%d", with, without)
	if with <= without {
		t.Errorf("深色 sheen 没有抬高键面亮度：有=%d 无=%d", with, without)
	}
}

// TestPlayerBarSheenTokens 钉住播放栏**本体** `background-image` 的三分支：
// 原版 `#player .player-controls { background-image: var(--player-sheen,
// var(--glass-sheen-static)) }`（mini 那条同一条声明 + !important），
// `--player-sheen` 只在烟熏玻璃时段定义，减少透明度时置 `none`。
func TestPlayerBarSheenTokens(t *testing.T) {
	light, dark := ThemeFor(PeriodAt(12)), ThemeFor(PeriodAt(0))
	if got := playerBarSheen(light); got != sheenGlassBaked {
		t.Errorf("浅色时段播放栏应铺 --glass-sheen-static，得到 %p", got)
	}
	if got := playerBarSheen(dark); got != sheenDarkBaked {
		t.Errorf("烟熏玻璃时段播放栏应铺 --player-sheen，得到 %p", got)
	}
	for _, th := range []Theme{
		light.withReduceTransparency(true),
		dark.withReduceTransparency(true),
	} {
		if got := playerBarSheen(th); got != nil {
			t.Errorf("减少透明度时原版 `--player-sheen: none`，应返回 nil，得到 %p", got)
		}
	}
}

// TestPlayerBarSheenWired 无头实测：主窗与迷你窗的**栏体**（整条矩形，不是按钮）
// 都真的铺上了洗光，且用的是按栏宽高比烘的那张位图（线性层角度不被拉偏）。
// 反证：把 playerBarSheen 换成恒返回 nil 的桩，这条整条栏的绘制就消失。
func TestPlayerBarSheenWired(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	type hit struct {
		r      ui.Rect
		radius float32
		bmp    *ui.Bitmap
	}
	var got []hit
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		got = append(got, hit{r, radius, bmp})
		orig(p, r, radius, bmp)
	}

	// 找到「整条栏」那一次绘制：宽度等于窗口宽、高度等于栏高。
	findBar := func(w, h float32) *hit {
		for i := range got {
			if got[i].r.W == w && got[i].r.H == h {
				return &got[i]
			}
		}
		return nil
	}

	playerBarOnlyProbe(t, nil)
	bar := findBar(1000, PlayerBarHeight)
	if bar == nil {
		t.Fatal("主窗播放栏本体没有铺 background-image 洗光")
	}
	if want := sheenGlassBaked.bitmapFor(bar.r.W, bar.r.H); bar.bmp != want {
		t.Error("主窗播放栏本体铺的不是 --glass-sheen-static（按栏宽高比烘）")
	}
	if bar.radius != 0 {
		t.Errorf("播放栏无圆角，radius 应为 0，得到 %v", bar.radius)
	}

	got = nil
	miniProbe(t, false)
	mbar := findBar(miniWidth, miniPlayerH)
	if mbar == nil {
		t.Fatal("迷你窗播放栏本体没有铺 background-image 洗光")
	}
	if want := sheenGlassBaked.bitmapFor(mbar.r.W, mbar.r.H); mbar.bmp != want {
		t.Error("迷你窗播放栏本体铺的不是 --glass-sheen-static（按栏宽高比烘）")
	}

	// 反证：换桩后整条栏的洗光不再出现（按钮自己那条还在，所以只找整栏）。
	got = nil
	origBar := playerBarSheen
	playerBarSheen = func(Theme) *sheenBaked { return nil }
	defer func() { playerBarSheen = origBar }()
	playerBarOnlyProbe(t, nil)
	if findBar(1000, PlayerBarHeight) != nil {
		t.Error("换桩后整条栏仍在铺洗光 —— 说明它不是从 playerBarSheen 来的")
	}
}

// TestPlayerBarSheenBrightensBar 无头实测（直接因果）：**烟熏玻璃**时段栏体
// 左侧内边距（x=1..16，18px padding 内没有子元素）在挂洗光时必须比把
// playerBarSheen 换成不画的桩更亮。用深色量：浅色栏体本身就是近白，白 .24
// 的径向光叠上去逐通道不变化（实测差值恒为 0），量不出因果。
func TestPlayerBarSheenBrightensBar(t *testing.T) {
	sum := func(tst *ui.Tester) int {
		img := tst.Image()
		total := 0
		for y := 648; y < 688; y++ {
			for x := 1; x < 16; x++ {
				c := rgbAt(img, x, y)
				total += c[0] + c[1] + c[2]
			}
		}
		return total
	}
	dark := func(a *App) { a.Theme = ThemeFor(PeriodAt(0)) }
	_, on := playerBarProbe(t, dark)
	on.Move(-1, -1)
	on.Frame()
	with := sum(on)

	orig := playerBarSheen
	defer func() { playerBarSheen = orig }()
	playerBarSheen = func(Theme) *sheenBaked { return nil }
	_, off := playerBarProbe(t, dark)
	off.Move(-1, -1)
	off.Frame()
	without := sum(off)

	t.Logf("深色栏体左侧总亮度：有洗光=%d 无洗光=%d", with, without)
	if with <= without {
		t.Errorf("播放栏本体的洗光没有抬高栏面亮度：有=%d 无=%d", with, without)
	}
}

// TestSheenBakeCacheCapped 钉住「按宽高比烘」的位图缓存必须封顶：播放栏本体
// 的宽度跟着窗口走，拖拽改尺寸时每 1px 就是一个新尺寸（512×29 约 59KB），
// 不封顶等于一次拖拽攒下上千张位图。
func TestSheenBakeCacheCapped(t *testing.T) {
	baked := &sheenBaked{spec: sheenGlass}
	for w := float32(600); w < 900; w++ {
		baked.bitmapFor(w, PlayerBarHeight)
	}
	if n := len(baked.byBox); n > sheenBakeCacheMax {
		t.Errorf("位图缓存没有封顶：%d 项 > %d", n, sheenBakeCacheMax)
	}
}

// TestPlayerBarSheenUnderInsetShadow 钉住播放栏洗光的**层序**：原版 CSS 里
// `background-image` 排在 `background-color` 之上、`inset box-shadow` **之下**
// （`#player .player-controls { background-color; background-image: var(--player-sheen);
// box-shadow: inset 0 1px 1px rgba(255,255,255,.88), inset 0 -1px 1px … }`），所以
// 洗光不能盖住顶缘那条内高光。
//
// 判法（直接因果）：把 sheen 的绘制换成一个**不透明黑**的桩（整条栏被它铺满），
// 再量栏顶两行：
//
//	y=栏顶    inset 白 .88 盖在黑上 → 再加 border-top 白 .72 → 亮（实测 235）
//	y=栏顶+1  inset 白 .88 的模糊尾 → 半亮（实测 38~40）
//
// 若洗光退回 `playerBarPaint` 的 Draw（画在 inset 内影之后），这两行就只剩
// 「黑 + border-top 白 .72」（实测 184）与「纯黑」（实测 7~8）—— 两条断言同时红。
// 反证已实测（临时把洗光挪回 Draw）：235→184、40→8。
func TestPlayerBarSheenUnderInsetShadow(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		p.Fill(r, ui.Hex("#000000"), radius)
	}

	_, tst := playerBarProbe(t, nil) // 浅色时段：顶缘内高光是白 .88
	tst.Move(-1, -1)
	tst.Frame()
	img := tst.Image()
	const barTop = 700 - PlayerBarHeight
	luma := func(x, y int) float64 {
		c := rgbAt(img, x, y)
		return (float64(c[0])*299 + float64(c[1])*587 + float64(c[2])*114) / 1000
	}

	// 栏顶那一行整条都是栏面（工具行垂直居中在 56 里，够不到 y=barTop）。
	rim, tail := luma(500, barTop), luma(500, barTop+1)
	t.Logf("不透明黑 sheen 下：栏顶行=%v 下一行=%v", rim, tail)
	if rim <= 210 {
		t.Errorf("栏顶行 luma=%v ≤210：顶缘内高光被洗光盖住了（层序退回 Draw 时实测 184）", rim)
	}
	if tail <= 20 {
		t.Errorf("栏顶下一行 luma=%v ≤20：内高光的模糊尾被洗光盖住了（层序退回 Draw 时实测 8）", tail)
	}
}
