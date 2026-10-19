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
	blur  glass.Blur
	veil  ui.Color
	sheen *sheenBaked
}

// cssGlassLayer 记一层 backdrop（见 backdropcount.go）并给出材质。
// radius 是原版 blur() 的半径（DIP），veil 是 background-color，
// sheen 是 background-image（nil 表示这一帧不铺）。
func cssGlassLayer(radius float32, veil ui.Color, sheen *sheenBaked) cssGlass {
	return cssGlass{blur: blurLayer(glass.Blur{Radius: radius}), veil: veil, sheen: sheen}
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
		playerSheenPaint(p, box, radii[0], m.sheen.bitmapFor(box.W, box.H))
	}
}

// searchPillVeil 是原版 .home-searchbar::before 的 background-color
// `--liquid-bg: rgba(255,255,255,.22)`。
func searchPillVeil() ui.Color { return ui.Hex("#ffffff").Alpha(0.22) }

// liquidBlurRadius 是原版 `--liquid-blur: blur(20px)` 的半径（DIP）。
const liquidBlurRadius = 20
