package view

import "github.com/egoist/mygo/ui"

// 进度条与音量条共用同一套外观。原版 CSS 里 `.player-timeline-track` 与
// `input.player-volume-slider::-webkit-slider-runnable-track` 的声明逐字相同
// （6px 轨道 + 1px 白边 + 青→蓝渐变填充 + 同一组 inset/外投影），两者的滑块
// 也共用一条 `::-webkit-slider-thumb` 规则。这里把这份共用外观抽成一处绘制
// 代码，避免两条各画一份又慢慢漂开。
//
// 尺寸取自原版 `#player .player-timeline-wrap` / `.player-timeline-track`：
const (
	timelineH      = 24 // 外层高（可点击区）
	timelineTrackH = 6  // 轨道高
	timelineInset  = 7  // 轨道左右内缩 = 滑块半径
	timelineThumb  = 14 // 滑块直径
)

// rangePaint 是一条输入条（进度 / 音量）这一帧的绘制输入。
type rangePaint struct {
	Frac       float32     // 已填充比例 0..1
	Hovered    bool        // 指针停在条上（原版 :hover）
	Held       bool        // 正在拖动 / 按下（原版 :active）
	Dark       bool        // 烟熏玻璃时段（原版深色块整组覆写轨道/滑块的影与滑块底色）
	Colors     rangeColors // 轨道描边与底色（随时段变，见 rangeColorsFor）
	Geom       rangeGeom   // 轨道与滑块行程的几何（见 timelineGeom / volumeGeom）
	ThumbScale float32     // 滑块缩放（由调用方用 AnimateTo 过渡后的值）
	// 进度条独有：已预读比例与广告段标记（音量条没有这两层）。
	Buffered float32
	Segs     []Segment
	Dur      float64
}

// rangeGeom 是一条输入条的几何：TrackInset 是轨道相对外层矩形两端的内缩，
// TravelInset 是滑块圆心行程相对轨道两端的内缩（原生 range 都是半个滑块）。
//
// 两条条的差别就在这两个值：原版进度条的轨道是 `.player-timeline-track`
// （left/right: 7px，滑块圆心正好走到它的两端），而音量条的轨道就是
// `input.player-volume-slider` 自己的 runnable track（不内缩、占满输入宽度，
// 滑块圆心两端各内缩半个滑块）。
type rangeGeom struct {
	TrackInset  float32
	TravelInset float32
}

var (
	// 进度条：轨道两端内缩 7（.player-timeline-track 的 left/right），
	// 滑块圆心走满轨道。
	timelineGeom = rangeGeom{TrackInset: timelineInset}
	// 音量条：轨道不内缩，滑块圆心两端各内缩半个滑块。
	volumeGeom = rangeGeom{TravelInset: timelineInset}
)

// rangeColors 是轨道这一帧的（描边, 底色）。
type rangeColors struct {
	Border ui.Color
	Rest   ui.Color
}

// rangeColorsFor 取轨道的两个令牌（原版 --player-track-border /
// --player-track-rest，见 Theme 上的注释）。
func rangeColorsFor(t Theme) rangeColors {
	return rangeColors{Border: t.PlayerTrackBorder, Rest: t.PlayerTrackRest}
}

// rangeSkin 是一条输入条这一帧随主题切换的一组材质值：轨道的内影 / 外投影，
// 滑块的描边 / 底色 / 玻璃高光 / 内影 / 外投影。
//
// 原版除 --player-track-border/rest 两个令牌外，深色时段还整组覆写了
// `.player-timeline-track`（与音量条 `::-webkit-slider-runnable-track`）以及滑块
// `::-webkit-slider-thumb` 的 box-shadow、滑块底色与描边（globals.css 的烟熏玻璃
// 块，5191 / 5211 起）。亮、深两套字面量都收在这里，画法只认这一份值。
type rangeSkin struct {
	TrackTop    ui.Color // 轨道顶内高光
	TrackBottom ui.Color // 轨道底内影
	TrackGlow   ui.Color // 轨道外投影（蓝，非悬停态）
	ThumbBorder ui.Color // 滑块 1px 描边
	ThumbFill   ui.Color // 滑块底色 #e0f2fe
	ThumbGlint  ui.Color // 滑块左上玻璃高光（径向的线性近似起点）
	ThumbTop    ui.Color // 滑块顶内高光
	ThumbBottom ui.Color // 滑块底内影（深色规则里没有这条 → Transparent）
	ThumbGlow   ui.Color // 滑块外投影（蓝，非悬停态）
}

// rangeSkinFor 取这一帧的材质（var 便于测试换桩反证接线，生产恒为 rangeSkinForTheme）。
var rangeSkinFor = rangeSkinForTheme

// rangeSkinForTheme 取这一帧的材质。hovered 只影响轨道两条内影与滑块外投影：
// 原版 :hover 那组规则两个时段共用（顶内高光提亮到白 .94、底内影收成墨 .08、
// 滑块影放到蓝 .28），深色块不覆写悬停值 —— 这里也保持「悬停不受时段影响」。
//
// 与原版的一处已知取舍：深色块里音量条（input.player-volume-slider）的
// :hover 规则与深色规则特异度同为 (1,2,2) 且源序在前，于是原版深色下音量条的
// 悬停 box-shadow 会被深色常态度盖掉；而进度条（.player-timeline-track）的悬停
// (1,3,0) 仍胜出。两条条共用一个画法，如实复刻这条「特异度事故」会让深色下
// 两条表现不一致，故统一为悬停胜出。
func rangeSkinForTheme(dark, hovered bool) rangeSkin {
	// 亮色一套（:root 的字面量）。
	s := rangeSkin{
		TrackTop:    ui.Hex("#ffffff").Alpha(0.88),  // inset 0 1px 1px 白 .88
		TrackBottom: shadowInk.Alpha(0.08),          // inset 0 -1px 1px 墨 .08
		TrackGlow:   ui.Hex("#0ea5e9").Alpha(0.10),  // 0 2px 7px 蓝 .1
		ThumbBorder: ui.Hex("#ffffff").Alpha(0.90),
		ThumbFill:   ui.Hex("#e0f2fe").Alpha(0.62),
		// 原版滑块高光是 `radial-gradient(circle at 34% 24%, 白 .98 …)`，
		// 用 135° 线性近似时起点取 .98（见 paintRangeThumb）。
		ThumbGlint: ui.Hex("#ffffff").Alpha(0.98),
		ThumbTop:   ui.Hex("#ffffff").Alpha(0.92), // inset 0 1px 1px 白 .92
		// 亮色独有的底内影 `inset 0 -1px 2px rgba(2,132,199,.1)`。
		ThumbBottom: ui.Hex("#0284c7").Alpha(0.10),
		ThumbGlow:   ui.Hex("#0284c7").Alpha(0.24), // 0 2px 7px 蓝 .24
	}
	if dark {
		s.TrackTop = ui.Hex("#ffffff").Alpha(0.30)  // inset 0 1px 0 白 .3
		s.TrackBottom = shadowInk.Alpha(0.10)       // inset 0 -1px 1px 墨 .1
		s.TrackGlow = ui.Hex("#0ea5e9").Alpha(0.07) // 0 2px 7px 蓝 .07
		s.ThumbBorder = ui.Hex("#ffffff").Alpha(0.68)
		s.ThumbFill = ui.Hex("#e0f2fe").Alpha(0.48)
		s.ThumbGlint = ui.Hex("#ffffff").Alpha(0.90) // 深色径向白 .9
		s.ThumbTop = ui.Hex("#ffffff").Alpha(0.58)   // inset 0 1px 1px 白 .58
		s.ThumbBottom = ui.Transparent               // 深色规则没有底内影
		s.ThumbGlow = ui.Hex("#0284c7").Alpha(0.20)  // 0 2px 7px 蓝 .2
	}
	if hovered {
		s.TrackTop = ui.Hex("#ffffff").Alpha(0.94)  // inset 0 1px 1px 白 .94
		s.TrackBottom = shadowInk.Alpha(0.08)       // inset 0 -1px 1px 墨 .08
		s.ThumbGlow = ui.Hex("#0284c7").Alpha(0.28) // 0 2px 7px 蓝 .28
	}
	return s
}

// rangeThumbScale 是原版滑块的三态 transform（`input.player-timeline:active
// ::-webkit-slider-thumb` / `:hover` 那两条）：常态 1、悬停 1.1、按下 1.12。
func rangeThumbScale(hovered, held bool) float32 {
	switch {
	case held:
		return 1.12
	case hovered:
		return 1.1
	}
	return 1
}

// rangeTrackRect 返回轨道的几何：外层矩形两端各内缩 geom.TrackInset，垂直居中。
func rangeTrackRect(r ui.Rect, geom rangeGeom) ui.Rect {
	cy := r.Y + r.H/2
	return ui.Rect{
		X: r.X + geom.TrackInset,
		Y: cy - timelineTrackH/2,
		W: r.W - geom.TrackInset*2,
		H: timelineTrackH,
	}
}

// rangeThumbX 是滑块圆心 x：在轨道两端各内缩 geom.TravelInset 的区间上按
// frac 取点。这正好是原生 range 的滑块行程（与 SliderBase 的
// PaddingX(timelineInset) 取值区间一致，见 progress 里的说明）。
func rangeThumbX(track ui.Rect, geom rangeGeom, frac float32) float32 {
	span := track.W - geom.TravelInset*2
	return track.X + geom.TravelInset + span*clamp01(frac)
}

// clamp01 把比例夹到 [0,1] 并吃掉 NaN。
func clamp01(v float32) float32 {
	switch {
	case v != v: // NaN
		return 0
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}

// paintRangeShadow 画轨道的外投影（原版 .player-timeline-track 的
// `0 2px 7px rgba(14,165,233,.1)`）：悬停时换成 `0 0 0 2px rgba(14,165,233,.08)`
// 的 2px 蓝环 + 更强的 `0 2px 9px rgba(14,165,233,.16)`。
//
// 必须画在缓冲区/广告段之前 —— 原版这几层都是轨道盒子的投影与下层兄弟，
// 画晚了会把轨道里的青色/红色标记压掉。
func paintRangeShadow(p *ui.Painter, track ui.Rect, hovered bool, s rangeSkin) {
	if hovered {
		p.Shadow(track, timelineTrackH/2, 0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.08))
		p.Shadow(track, timelineTrackH/2, 0, 2, 9, 0, ui.Hex("#0ea5e9").Alpha(0.16))
		return
	}
	p.Shadow(track, timelineTrackH/2, 0, 2, 7, 0, s.TrackGlow)
}

// paintRangeFill 画轨道本体（不含缓冲区/广告段）：底色 → 已填充的青→蓝渐变
// → 两条 inset → 1px 白描边。原版这三者的顺序就是 CSS 的绘制顺序
// （background → inset box-shadow → border）。
func paintRangeFill(p *ui.Painter, track ui.Rect, frac float32, c rangeColors, s rangeSkin) {
	frac = clamp01(frac)
	// 未播部分：半透明白（原版 --player-track-rest）。
	p.Fill(track, c.Rest, timelineTrackH/2)
	if frac > 0.001 {
		// 已播部分：90° 青 → 蓝（原版渐变在 --player-progress 处硬停）。
		p.FillGradient(ui.Rect{X: track.X, Y: track.Y, W: track.W * frac, H: track.H},
			ui.LinearGradient{
				From:  ui.Hex("#2dd4bf").Alpha(0.78),
				To:    ui.Hex("#0284c7").Alpha(0.82),
				Angle: 90,
			}, timelineTrackH/2)
	}
	paintRangeInsets(p, track, s)
	p.Stroke(track, c.Border, timelineTrackH/2, 1)
}

// paintRangeInsets 画原版轨道那两条 inset 阴影：
//
//	inset 0 1px 1px rgba(255,255,255,.88)   顶部亮边（玻璃管的高光）
//	inset 0 -1px 1px rgba(15,23,42,.08)     底部内影
//
// 烟熏玻璃时段原版换成 `inset 0 1px 0 白 .3` + `inset 0 -1px 1px 墨 .1`
// （顶亮边淡一大截，深色玻璃上再铺那么亮会糊成一条白带）。
//
// mygo 的 Painter 没有 inset 阴影，用两条 1px 圆头线近似 —— 6px 高的轨道上
// 只占最外圈各 1px，中间 4px 仍是纯底色/渐变，与原版的观感一致。
func paintRangeInsets(p *ui.Painter, track ui.Rect, s rangeSkin) {
	p.Fill(ui.Rect{X: track.X + 1, Y: track.Y + 1, W: track.W - 2, H: 1}, s.TrackTop, 0.5)
	p.Fill(ui.Rect{X: track.X + 1, Y: track.Y + track.H - 2, W: track.W - 2, H: 1},
		s.TrackBottom, 0.5)
}

// paintRangeThumb 画滑块：淡蓝底 + 左上玻璃高光 + 两条 inset + 1px 白描边 +
// 蓝色投影；悬停放大到 1.1 并多一圈 3px 蓝晕，按下放大到 1.12（原版 thumb 的
// 三态）。描边 / 底色 / 高光 / 内影 / 外影都取自 rangeSkin，深色时段整体压暗一档。
//
// ThumbScale 由调用方用 AnimateTo 过渡后传入（原版 `transition: transform
// 100ms ease`），不再在函数内部瞬时计算。
func paintRangeThumb(p *ui.Painter, cx, cy float32, hovered, held bool, scale float32, s rangeSkin) {
	d := timelineThumb * scale
	k := ui.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}
	if hovered || held {
		// 0 0 0 3px rgba(14,165,233,.1)
		p.Shadow(k, d/2, 0, 0, 0, 3, ui.Hex("#0ea5e9").Alpha(0.10))
	}
	// 0 2px 7px rgba(2,132,199,.24)，悬停 .28；深色时段压到 .2。
	p.Shadow(k, d/2, 0, 2, 7, 0, s.ThumbGlow)
	p.Fill(k, s.ThumbFill, d/2)
	// 玻璃高光：原版 thumb 的第一层是 `circle at 34% 24%` 的径向白高光，
	// 这里用 135°（左上→右下）的白→透明线性渐变近似：14px 的圆上，
	// 径向与线性在肉眼看来都是「左上角一点亮、右下角没有」。
	p.FillGradient(k, ui.LinearGradient{
		From:  s.ThumbGlint,
		To:    ui.Hex("#ffffff").Alpha(0),
		Angle: 135,
	}, d/2)
	// 原版 thumb 的两条 inset 内影（`inset 0 1px 1px 白 .92` +
	// `inset 0 -1px 2px rgba(2,132,199,.1)`；深色只留顶内高光白 .58，
	// 底内影那条深色规则里没有）。圆上画不出 inset，用两条 1px 线近似，
	// 裁剪到圆内 —— 顶部那条只保留弦宽，不会溢出圆外。
	p.Clip(k, d/2, func() {
		p.Fill(ui.Rect{X: k.X + 1, Y: k.Y + 1, W: k.W - 2, H: 1}, s.ThumbTop, 0.5)
		if s.ThumbBottom.A > 0 {
			p.Fill(ui.Rect{X: k.X + 1, Y: k.Y + k.H - 2, W: k.W - 2, H: 1},
				s.ThumbBottom, 0.5)
		}
	})
	p.Stroke(k, s.ThumbBorder, d/2, 1)
}

// paintRangeBar 画一条完整的输入条：轨道 + 滑块。音量条用它；进度条因为多出
// 缓冲区与广告段两层，在 paintTimeline 里按同样的顺序单独拼。
func paintRangeBar(p *ui.Painter, r ui.Rect, rp rangePaint) {
	track := rangeTrackRect(r, rp.Geom)
	s := rangeSkinFor(rp.Dark, rp.Hovered)
	paintRangeShadow(p, track, rp.Hovered, s)
	paintRangeFill(p, track, rp.Frac, rp.Colors, s)
	// 音量条没有外部动画状态，用内部计算的瞬时 scale。
	scale := rp.ThumbScale
	if scale == 0 {
		scale = rangeThumbScale(rp.Hovered, rp.Held)
	}
	paintRangeThumb(p, rangeThumbX(track, rp.Geom, rp.Frac), r.Y+r.H/2, rp.Hovered, rp.Held, scale, s)
}
