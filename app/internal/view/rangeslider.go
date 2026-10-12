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
	Frac    float32     // 已填充比例 0..1
	Hovered bool        // 指针停在条上（原版 :hover）
	Held    bool        // 正在拖动 / 按下（原版 :active）
	Colors  rangeColors // 轨道描边与底色（随时段变，见 rangeColorsFor）
	Geom    rangeGeom   // 轨道与滑块行程的几何（见 timelineGeom / volumeGeom）
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
func paintRangeShadow(p *ui.Painter, track ui.Rect, hovered bool) {
	if hovered {
		p.Shadow(track, timelineTrackH/2, 0, 0, 0, 2, ui.Hex("#0ea5e9").Alpha(0.08))
		p.Shadow(track, timelineTrackH/2, 0, 2, 9, 0, ui.Hex("#0ea5e9").Alpha(0.16))
		return
	}
	p.Shadow(track, timelineTrackH/2, 0, 2, 7, 0, ui.Hex("#0ea5e9").Alpha(0.10))
}

// paintRangeFill 画轨道本体（不含缓冲区/广告段）：底色 → 已填充的青→蓝渐变
// → 两条 inset → 1px 白描边。原版这三者的顺序就是 CSS 的绘制顺序
// （background → inset box-shadow → border）。
func paintRangeFill(p *ui.Painter, track ui.Rect, frac float32, hovered bool, c rangeColors) {
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
	paintRangeInsets(p, track, hovered)
	p.Stroke(track, c.Border, timelineTrackH/2, 1)
}

// paintRangeInsets 画原版轨道那两条 inset 阴影：
//
//	inset 0 1px 1px rgba(255,255,255,.88)   顶部亮边（玻璃管的高光）
//	inset 0 -1px 1px rgba(15,23,42,.08)     底部内影
//
// mygo 的 Painter 没有 inset 阴影，用两条 1px 圆头线近似 —— 6px 高的轨道上
// 只占最外圈各 1px，中间 4px 仍是纯底色/渐变，与原版的观感一致。
func paintRangeInsets(p *ui.Painter, track ui.Rect, hovered bool) {
	top := ui.Hex("#ffffff").Alpha(0.88)
	if hovered {
		top = ui.Hex("#ffffff").Alpha(0.94) // 悬停时 inset 亮边也提亮
	}
	p.Fill(ui.Rect{X: track.X + 1, Y: track.Y + 1, W: track.W - 2, H: 1}, top, 0.5)
	p.Fill(ui.Rect{X: track.X + 1, Y: track.Y + track.H - 2, W: track.W - 2, H: 1},
		shadowInk.Alpha(0.08), 0.5)
}

// paintRangeThumb 画滑块：淡蓝底 + 左上玻璃高光 + 1px 白描边 + 蓝色投影；
// 悬停放大到 1.1 并多一圈 3px 蓝晕，按下放大到 1.12（原版 thumb 的三态）。
func paintRangeThumb(p *ui.Painter, cx, cy float32, hovered, held bool) {
	d := timelineThumb * rangeThumbScale(hovered, held)
	k := ui.Rect{X: cx - d/2, Y: cy - d/2, W: d, H: d}
	if hovered || held {
		// 0 0 0 3px rgba(14,165,233,.1)
		p.Shadow(k, d/2, 0, 0, 0, 3, ui.Hex("#0ea5e9").Alpha(0.10))
	}
	// 0 2px 7px rgba(2,132,199,.24)，悬停 .28。
	shade := float32(0.24)
	if hovered || held {
		shade = 0.28
	}
	p.Shadow(k, d/2, 0, 2, 7, 0, ui.Hex("#0284c7").Alpha(shade))
	p.Fill(k, ui.Hex("#e0f2fe").Alpha(0.62), d/2)
	// 玻璃高光：原版 thumb 的第一层是 `circle at 34% 24%` 的径向白高光，
	// 这里用 135°（左上→右下）的白→透明线性渐变近似：14px 的圆上，
	// 径向与线性在肉眼看来都是「左上角一点亮、右下角没有」。
	p.FillGradient(k, ui.LinearGradient{
		From:  ui.Hex("#ffffff").Alpha(0.86),
		To:    ui.Hex("#ffffff").Alpha(0),
		Angle: 135,
	}, d/2)
	p.Stroke(k, ui.Hex("#ffffff").Alpha(0.90), d/2, 1)
}

// paintRangeBar 画一条完整的输入条：轨道 + 滑块。音量条用它；进度条因为多出
// 缓冲区与广告段两层，在 paintTimeline 里按同样的顺序单独拼。
func paintRangeBar(p *ui.Painter, r ui.Rect, rp rangePaint) {
	track := rangeTrackRect(r, rp.Geom)
	paintRangeShadow(p, track, rp.Hovered)
	paintRangeFill(p, track, rp.Frac, rp.Hovered, rp.Colors)
	paintRangeThumb(p, rangeThumbX(track, rp.Geom, rp.Frac), r.Y+r.H/2, rp.Hovered, rp.Held)
}
