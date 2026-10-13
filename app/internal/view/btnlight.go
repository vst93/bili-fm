package view

import (
	"image"
	"math"
	"sync"

	"github.com/egoist/mygo/ui"
)

// 本文件补齐小按钮上那层「玻璃聚光」——原版 globals.css 里一连串
// `background-image: radial-gradient(...)` 的静态径向高光。原生版此前所有按钮
// 都只有半透明填充 + 1px 描边，少的就是这道「光从左上角照进玻璃」的光泽，
// 于是按钮看着比原版平。
//
// 原版出现这层高光的位置（值取自 globals.css，逐字抄下）：
//
//	.nav-icon-btn          常态 80% 110% at 30% 8%，白 .38 → .12(30%) → .02(58%) → 透明(72%)
//	.top-tool-btn          常态 80% 110% at 30% 8%，白 .36 → .10(30%) → 透明(72%)
//	.search-submit-btn     同上（.36 / .10 / 0）
//	.app-title-bar button  悬停 80% 110% at 30% 8%，白 .38 → .12(30%) → .02(58%) → 透明(72%)
//	.mini-window-controls  悬停同 .app-title-bar button
//	#switch-window-mode    悬停 / 按下同 .app-title-bar button
//	以上四类 :active 时换成居中 50% 60% at 50% 50%，白 .16 → .04(38%) → 透明(65%)
//
// mygo 只有两色线性渐变，没有径向渐变接口，所以照 bloom.go / minibg.go 的做法
// 把渐变逐像素在 float 里烘成一张小位图（白 + alpha），每帧只多一个贴图图元。
// 位图按（几何 + 停靠点）缓存、所有按钮共用：按钮 32/34 DIP，@2x 恰好 64 像素，
// 64 见方的源图贴上去基本 1:1。

// btnLightSize 是烘焙位图的边长（像素）。按钮最大 34 DIP，@2x = 68 像素，
// 64 的源图略缩一点即可，径向光本就极软，看不出缩放。
const btnLightSize = 64

// btnLight 是一条径向玻璃高光：cx/cy 是圆心（占盒宽/高的比例），rx/ry 是横/纵
// 半径（占盒宽/高的比例），stops 是「到半径的比例 → 白色 alpha」的停靠点
// （CSS 的 `radial-gradient(<rx> <ry> at <cx> <cy>, ...)`）。
//
// 用定长数组而不是切片，好让 btnLight 可比较 —— 位图缓存拿它当 map 键。
type btnLight struct {
	stops          [4]gradStop
	n              int
	cx, cy, rx, ry float32
}

func newBtnLight(cx, cy, rx, ry float32, stops ...gradStop) btnLight {
	l := btnLight{cx: cx, cy: cy, rx: rx, ry: ry, n: len(stops)}
	copy(l.stops[:], stops)
	return l
}

// alphaAt 返回盒内坐标 (x,y)（盒 w×h）处这条高光的不透明度（0..1）。归一化距离
// d = √(((x−cx·w)/(rx·w))² + ((y−cy·h)/(ry·h))²)，alpha 沿 d 走停靠点、到最后一个
// （透明的）停靠点外归零 —— 与 CSS radial-gradient 的默认 farthest 尺寸语义一致。
func (l btnLight) alphaAt(x, y, w, h float32) float32 {
	if l.n == 0 || l.rx <= 0 || l.ry <= 0 {
		return 0
	}
	dx := (x - l.cx*w) / (l.rx * w)
	dy := (y - l.cy*h) / (l.ry * h)
	d := float32(math.Hypot(float64(dx), float64(dy)))
	if d >= l.stops[l.n-1].At {
		return 0
	}
	return float32(gradAt(l.stops[:l.n], d).A) / 255
}

// 四条几何。前三条只差停靠点（都锚在左上 30% 8%），第四条是按下时的居中光。
var (
	// .nav-icon-btn（常态）/ .app-title-bar button（悬停）: 白 .38 → .12 → .02 → 0
	btnLightNav = newBtnLight(0.30, 0.08, 0.80, 1.10,
		gradStop{0, ui.Hex("#ffffff").Alpha(0.38)},
		gradStop{0.30, ui.Hex("#ffffff").Alpha(0.12)},
		gradStop{0.58, ui.Hex("#ffffff").Alpha(0.02)},
		gradStop{0.72, ui.Hex("#ffffff").Alpha(0)},
	)
	// .top-tool-btn / .search-submit-btn（常态）: 白 .36 → .10 → 0
	btnLightTool = newBtnLight(0.30, 0.08, 0.80, 1.10,
		gradStop{0, ui.Hex("#ffffff").Alpha(0.36)},
		gradStop{0.30, ui.Hex("#ffffff").Alpha(0.10)},
		gradStop{0.72, ui.Hex("#ffffff").Alpha(0)},
	)
	// .app-title-bar button.app-title-bar-close（悬停，暖红）: 白 .30 → 0
	btnLightClose = newBtnLight(0.30, 0.08, 0.80, 1.10,
		gradStop{0, ui.Hex("#ffffff").Alpha(0.30)},
		gradStop{0.72, ui.Hex("#ffffff").Alpha(0)},
	)
	// 以上各族的 :active（居中）: 白 .16 → .04(38%) → 0(65%)
	btnLightPress = newBtnLight(0.50, 0.50, 0.50, 0.60,
		gradStop{0, ui.Hex("#ffffff").Alpha(0.16)},
		gradStop{0.38, ui.Hex("#ffffff").Alpha(0.04)},
		gradStop{0.65, ui.Hex("#ffffff").Alpha(0)},
	)
)

// titleBarBtnLight 返回标题栏 / 迷你窗按钮这一帧该铺的高光：按下用居中光，
// 悬停用左上光，其余不铺（原版这两族的常态没有 background-image）。
func titleBarBtnLight(pressed, hovered bool) (btnLight, bool) {
	switch {
	case pressed:
		return btnLightPress, true
	case hovered:
		return btnLightNav, true
	}
	return btnLight{}, false
}

// btnLightBitmap 缓存把某条高光烘成的位图。key 是 btnLight 的值（可比较），
// 于是四种几何各烘一张，所有按钮共用。
var (
	btnLightMu    sync.Mutex
	btnLightCache = map[btnLight]*ui.Bitmap{}
)

func (l btnLight) bitmap() *ui.Bitmap {
	btnLightMu.Lock()
	defer btnLightMu.Unlock()
	if b, ok := btnLightCache[l]; ok {
		return b
	}
	b := ui.NewBitmap(btnLightPixels(btnLightSize, l))
	btnLightCache[l] = b
	return b
}

// btnLightPixels 把高光逐像素烘成一张 size×size 的直通 alpha 位图（NRGBA：
// 白 + 算出的 alpha）。逐像素在 float 里算一次，才不会像「几十层低 alpha 的
// 同心椭圆」那样在 8 位预乘合成里丢精度（见 bloom.go 的同一教训）。
func btnLightPixels(size int, l btnLight) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	f := float32(size)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			a := l.alphaAt(float32(x)+0.5, float32(y)+0.5, f, f)
			i := img.PixOffset(x, y)
			img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 255, 255, 255, to8(a)
		}
	}
	return img
}

// draw 把这条高光铺到按钮上：`Element.Draw` 画在元素自身的背景/边框/内影之后、
// 图标等内容之前，正好对应原版 background-image 的层级；再按按钮圆角 Clip，
// 免得四角溢出（位图铺满整个矩形）。不放任何子节点，也就不会参与命中测试。
func (l btnLight) draw(e ui.Element, radius float32) {
	if l.n == 0 {
		return
	}
	bmp := l.bitmap()
	e.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, radius, func() { p.Image(bmp, r, ui.FillBox) })
	})
}
