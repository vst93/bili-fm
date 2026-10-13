package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（浅色 ::before 的 --glass-sheen-static）测试。原版每颗
// `#player .player-button::before` 的 background-image 随（浅色/烟熏玻璃）×
// （工具/播放）×（播放态/悬停）×（主窗/mini）切换；上一轮只接了深色的
// --player-sheen，浅色键面一直比原版平一档。这里先钉住几条浅色渐变的位图
// 几何与色相，再钉住视图按时段/键族选对了哪一条。

func sheenAlphaAt(img *image.NRGBA, x, y int) float32 {
	return float32(img.Pix[img.PixOffset(x, y)+3]) / 255
}

func sheenRGBAAt(img *image.NRGBA, x, y int) (int, int, int, float32) {
	i := img.PixOffset(x, y)
	return int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2]), float32(img.Pix[i+3]) / 255
}

// TestSheenGlassPixels 钉住浅色通用 --glass-sheen-static 的位图几何：
// 径向中心在 (22%, -5%)（盒外偏上），左上最亮、往右下变淡；线性 170° 在 60%
// 处淡到 0，所以右下角应基本无光泽；峰值不超过径向白 .24。
func TestSheenGlassPixels(t *testing.T) {
	const n = 128
	img := sheenPixels(sheenGlass, n, n)
	if img == nil {
		t.Fatal("sheenPixels 返回 nil")
	}
	tl := sheenAlphaAt(img, 0, 0)
	mid := sheenAlphaAt(img, n/2, n/2)
	br := sheenAlphaAt(img, n-1, n-1)
	t.Logf("glass sheen alpha：左上=%v 中=%v 右下=%v", tl, mid, br)
	if !(tl > mid && mid > br) {
		t.Errorf("glass sheen 应从左上往右下单调变淡：左上=%v 中=%v 右下=%v", tl, mid, br)
	}
	if tl <= 0 {
		t.Errorf("左上角应有光泽，得到 %v", tl)
	}
	if br > 0.005 {
		t.Errorf("右下角应基本无光泽（线性 60%% 之外），得到 %v", br)
	}
	maxA := float32(0)
	for i := 3; i < len(img.Pix); i += 4 {
		if a := float32(img.Pix[i]) / 255; a > maxA {
			maxA = a
		}
	}
	if maxA > 0.30 || maxA < 0.20 {
		// 径向白 .24 与线性白 .07 合成后峰值会略高于 .24，但不应超过 .24+.07。
		t.Errorf("glass sheen 峰值 alpha = %v，应落在 .20~.30（径向 .24 + 线性 .07 合成）", maxA)
	}
	// 与深色 --player-sheen 必须是两条不同的渐变（峰值/几何不同）。
	dark := sheenPixels(sheenDark, n, n)
	if sheenAlphaAt(img, 0, 0) == sheenAlphaAt(dark, 0, 0) &&
		sheenAlphaAt(img, n/2, n/2) == sheenAlphaAt(dark, n/2, n/2) {
		t.Error("浅色 glass sheen 与深色 player-sheen 位图完全一致，令牌没区分")
	}
}

// TestSheenStopsPremultiplied 钉住色停按 CSS 的预乘 alpha 插值：白 .07 到
// transparent 的中点应仍是白色（alpha ≈ .035），而不是被透明的黑色拉灰。
func TestSheenStopsPremultiplied(t *testing.T) {
	stops := []sheenStop{
		{At: 0, Col: ui.Hex("#ffffff").Alpha(0.07)},
		{At: 0.5, Col: ui.Transparent},
	}
	r, g, b, a := sheenStopsAt(stops, 0.25)
	if a < 0.03 || a > 0.04 {
		t.Errorf("中点 alpha = %v，应 ≈ .035", a)
	}
	if !(r > 0.02 && g > 0.02 && b > 0.02) {
		t.Errorf("中点仍应是白（预乘插值），得到预乘 RGB=(%v,%v,%v)", r, g, b)
	}
}

// TestSheenPlayPixelsColored 钉住主窗播放键（浅色·播放中）那条带色相的光泽：
// 径向白在最上（顶部近白、alpha 明显），线性层是天空蓝（远离径向的右下角仍留
// 一点蓝调，B>R）——证明彩色线性层被正确合成，而不是被当成白色。
func TestSheenPlayPixelsColored(t *testing.T) {
	const n = 128
	img := sheenPixels(sheenPlay, n, n)
	tr, tg, tb, ta := sheenRGBAAt(img, n/2, 0)
	t.Logf("播放键 sheen 顶部中心：rgba=(%d,%d,%d,%v)", tr, tg, tb, ta)
	if ta < 0.4 {
		t.Errorf("播放键 sheen 顶部 alpha = %v，径向白应让它明显可见", ta)
	}
	if !(tr > 240 && tg > 240 && tb > 240) {
		t.Errorf("播放键 sheen 顶部应近白（径向层在上），得到 (%d,%d,%d)", tr, tg, tb)
	}
	br, bg, bb, ba := sheenRGBAAt(img, n-1, n-1)
	t.Logf("播放键 sheen 右下角：rgba=(%d,%d,%d,%v)", br, bg, bb, ba)
	if ba <= 0 {
		t.Error("播放键 sheen 右下角应有线性层留下的极淡色，得到 alpha=0")
	}
	if bb <= br {
		t.Errorf("播放键 sheen 右下角应偏蓝（天空蓝线性层），得到 R=%d B=%d", br, bb)
	}
}

// TestSheenMiniPlayBluerThanPlay 钉住 mini 播放键的线性层比主窗播放键更浓
// （.12/.06 vs .06/.03）：同一像素的蓝调（B−R）× alpha 应更大。
func TestSheenMiniPlayBluerThanPlay(t *testing.T) {
	const n = 128
	blueness := func(spec sheenSpec) float32 {
		r, _, b, a := sheenRGBAAt(sheenPixels(spec, n, n), n-1, n-1)
		return float32(b-r) * a
	}
	mini, play := blueness(sheenMiniPlay), blueness(sheenPlay)
	t.Logf("右下角蓝调：mini=%v 主窗播放=%v", mini, play)
	if !(mini > play) {
		t.Errorf("mini 播放键线性层应比主窗播放键更浓：mini=%v play=%v", mini, play)
	}
}

// TestPlayerSheenForCascade 钉住 `::before` 光泽的层叠选择：按时段/键族/播放态/
// 悬停各落到原版对应的那条 background-image。
func TestPlayerSheenForCascade(t *testing.T) {
	cases := []struct {
		name                   string
		kind                   playerSheenKind
		dark, playing, hovered bool
		want                   *sheenBaked
	}{
		{"工具键·浅色", sheenKindStatic, false, false, false, sheenGlassBaked},
		{"工具键·浅色·悬停不改 image", sheenKindStatic, false, false, true, sheenGlassBaked},
		{"工具键·深色", sheenKindStatic, true, false, false, sheenDarkBaked},
		{"工具键·深色·悬停不改 image", sheenKindStatic, true, false, true, sheenDarkBaked},
		{"播放键·浅色·暂停", sheenKindPlay, false, false, false, sheenGlassBaked},
		{"播放键·浅色·暂停悬停", sheenKindPlay, false, false, true, sheenGlassBaked},
		{"播放键·浅色·播放中", sheenKindPlay, false, true, false, sheenPlayBaked},
		{"播放键·浅色·播放中悬停", sheenKindPlay, false, true, true, sheenPlayHoverBaked},
		{"播放键·深色·暂停", sheenKindPlay, true, false, false, sheenDarkBaked},
		{"播放键·深色·暂停悬停转浅色", sheenKindPlay, true, false, true, sheenGlassBaked},
		{"播放键·深色·播放中", sheenKindPlay, true, true, false, sheenDarkBaked},
		{"播放键·深色·播放中悬停转浅色", sheenKindPlay, true, true, true, sheenPlayHoverBaked},
		{"mini 播放键·浅色", sheenKindMiniPlay, false, false, false, sheenMiniPlayBaked},
		{"mini 播放键·浅色·播放中也同一面", sheenKindMiniPlay, false, true, false, sheenMiniPlayBaked},
		{"mini 播放键·浅色·悬停转主窗", sheenKindMiniPlay, false, false, true, sheenGlassBaked},
		{"mini 播放键·深色", sheenKindMiniPlay, true, false, false, sheenDarkBaked},
	}
	for _, tc := range cases {
		if got := playerSheenFor(tc.kind, tc.dark, tc.playing, tc.hovered); got != tc.want {
			t.Errorf("%s：选到 %p，want %p", tc.name, got, tc.want)
		}
	}
	// 明确否定：mini 播放键在浅色下**不**吃主窗播放中那条渐变。
	if playerSheenFor(sheenKindMiniPlay, false, true, false) == sheenPlayBaked {
		t.Error("mini 播放键浅色下误用了主窗播放中渐变")
	}
}

// TestPlayerGlassSheenBrightensLightButton 无头实测（直接因果）：浅色时段同一颗
// 音量键，挂 sheen 时键面总亮度必须高于把 playerSheenPaint 换成不画的桩时——
// 证明浅色键面真的铺上了 --glass-sheen-static，而不是只有深色才画。
func TestPlayerGlassSheenBrightensLightButton(t *testing.T) {
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

	_, on := playerBarProbe(t, nil)
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
	_, off := playerBarProbe(t, nil)
	off.Move(-1, -1)
	off.Frame()
	without := boxLuma(off.Image(), r)

	t.Logf("浅色音量键总亮度：有 sheen=%d 无 sheen=%d", with, without)
	if with <= without {
		t.Errorf("浅色 sheen 没有抬高键面亮度：有=%d 无=%d", with, without)
	}
}
