package view

import (
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// cssGlass 复刻原版 CSS 的「两层玻璃」：`backdrop-filter` 只模糊，那层半透明
// 底色由我们自己铺。
//
// 原版（globals.css 的 .home-searchbar::before / #player .player-controls）：
//
//	backdrop-filter: var(--liquid-blur);   /* blur(20px) saturate(1.45) */
//	background-color: var(--liquid-bg);    /* 搜索药丸 rgba(255,255,255,.22) */
//	                                        /* 播放栏 --player-surface  rgba(241,248,253,.36) */
//	background-image: var(--glass-sheen-static);
//
// 为什么不用框架的 glass.Glass（Regular）：它的材质把背景的**黑抬到 54%**、
// 白留 100%（plugins/glass/glass.go 的 paint，取自 NSGlassEffectView 的实测），
// 等于一层很实的白纱。原版的 backdrop-filter 不改变亮度、只模糊，白底只有
// 22% / 36%。两者叠出来的观感差一大截：Regular 玻璃让背后的颜色出不来，用户
// 2026-10 报「搜索栏/播放栏透度不够、背后的背景没透出来」就是它。
//
// 之前用 `glass.Glass{Tint: 冷灰 .22}` 去压 Regular 的白，只会把表面压成灰，
// 仍然不透 —— 染色是**加法**，压不回被抬亮的那 54%。
//
// 1px 描光不在这里：原版的 rim 是 ::after 的 conic（--liquid-rim + 色散边），
// 由调用方用 paintPlayerRim 画；材质只负责 ::before 那一层。
type cssGlass struct {
	blur      glass.Blur
	veil      ui.Color
	sheen     *sheenBaked
	sheenFade float32
}

// cssGlassLayer 记一层 backdrop（见 backdropcount.go）并给出材质。
// radius 是原版 blur() 的半径（DIP），veil 是 background-color，
// sheen 是 background-image（nil 表示这一帧不铺）。sheenFade 是洗光整层
// 不透明度（1 = 原值）：窗口越透，固定的烘焙洗光越该一起收，否则它在
// veil 已经淡下去之后变成最大的一层白纱。
func cssGlassLayer(radius float32, veil ui.Color, sheen *sheenBaked, sheenFade float32) cssGlass {
	if sheenFade <= 0 {
		sheen = nil
	}
	return cssGlass{blur: blurLayer(glass.Blur{Radius: radius}), veil: veil, sheen: sheen, sheenFade: sheenFade}
}

// PaintMaterial paints the blur, then the veil and the sheen over it, as CSS
// paints background-color and background-image over what backdrop-filter
// blurred.
func (m cssGlass) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	m.blur.PaintMaterial(p, box, radii)
	if m.veil.A > 0 {
		// 这两个面（搜索药丸、播放栏）都是单一半径的圆角矩形。
		p.Fill(box, m.veil, radii[0])
	}
	if m.sheen != nil {
		paintBakedSheenFaded(p, box, radii[0], m.sheen, m.sheenFade)
	}
}

// paintBakedSheenFaded 按 fade 缩放一条已烘焙洗光的整层不透明度。
//
// 位图本身不带逐帧的 alpha 参数（playerSheenPaint 只画图），所以走
// Painter.ImageRegion 的 Tint：白色 × fade 只乘 alpha、不改色相，与
// 给 ::before 设 opacity 同义。fade >= 1 时仍走 playerSheenPaint，
// 保持默认外观与测试里的绘制桩完全不差。
func paintBakedSheenFaded(p *ui.Painter, r ui.Rect, radius float32, baked *sheenBaked, fade float32) {
	bmp := baked.bitmapFor(r.W, r.H)
	if bmp == nil || fade <= 0 {
		return
	}
	if fade >= 1 {
		playerSheenPaint(p, r, radius, bmp)
		return
	}
	paintFadedSheen(p, r, radius, bmp, fade)
}

// paintFadedSheen 画一条按 fade 调过整层不透明度的洗光。声明成 var 是为了
// 让无头测试能像 playerSheenPaint 一样记录「播放栏本体有没有铺、用的是不是
// 按栏宽高比烘的位图」（见 TestPlayerBarSheenWired）。
var paintFadedSheen = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap, fade float32) {
	p.Clip(r, radius, func() {
		p.ImageRegion(bmp, ui.Rect{}, r, ui.ImageOptions{Tint: ui.Hex("#ffffff").Alpha(fade)})
	})
}

// searchPillVeil 是原版 .home-searchbar::before 的 background-color
// `--liquid-bg: rgba(255,255,255,.22)`。
func searchPillVeil() ui.Color { return ui.Hex("#ffffff").Alpha(0.22) }

// liquidBlurRadius 是原版 `--liquid-blur: blur(20px)` 的半径（DIP）。
const liquidBlurRadius = 20
