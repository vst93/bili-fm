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

// TestPlayerSheenDarkWired 无头反证：sheen 只在深色时段画——亮色时段视图不碰
// playerSheenPaint，深色时段每颗 `#player .player-button` 都碰。
func TestPlayerSheenDarkWired(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	calls := 0
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32) {
		calls++
		orig(p, r, radius)
	}

	_, light := playerBarProbe(t, nil)
	_ = light
	if calls != 0 {
		t.Errorf("亮色时段不应画深色 sheen，实测调用 %d 次", calls)
	}

	_, dark := playerBarProbe(t, func(a *App) { a.Theme = ThemeFor(PeriodAt(0)) })
	_ = dark
	if calls == 0 {
		t.Error("深色时段应画 sheen（playerSheenPaint 没被调用）")
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
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32) {}
	_, off := playerBarProbe(t, dark)
	off.Move(-1, -1)
	off.Frame()
	without := boxLuma(off.Image(), r)

	t.Logf("深色音量键总亮度：有 sheen=%d 无 sheen=%d", with, without)
	if with <= without {
		t.Errorf("深色 sheen 没有抬高键面亮度：有=%d 无=%d", with, without)
	}
}
