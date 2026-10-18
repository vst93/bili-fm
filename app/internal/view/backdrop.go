package view

import "github.com/egoist/mygo/ui"

// 本文件由 src/styles/globals.css 的 --liquid-backdrop 生成（`scripts/gen-backdrop.py`），
// 请勿手改。
//
// 原版按一天 11 个时段切换窗口背景：一条 **165° 的多停靠点**线性渐变，
// 从冷蓝（深夜）经暖金（清晨、午后）再回到蓝灰（夜晚）。这是 bili-FM
// 视觉签名的主体，所有玻璃层都浮在它上面。
//
// ⚠️ 两件此前抄漏、用户可感知的事（2026-10-24 修）：
//
//  1. **角度是 165°，不是 180°**。180° 是纯垂直渐变 —— 背景没有任何斜向感，
//     而且**底部一整行左右同色**（原版左下最暗、右下最亮的斜向落差整条丢失）。
//     实测（1000×700 无头渲染，与 CSS 165° 精确采样对照）最坏通道差 **29**
//     （dusk；golden 22 / midday 18 / noon 14）。
//  2. **停靠点的百分比位置要保留**。此前 `Stops` 只存颜色，位置按「均匀分布」
//     隐式重建；原版各时段的停靠点并不均分（midday 是 0/10/22/…/85/100%）。
//     纯位置误差实测最坏 **9** 通道差（golden；dawn 8 / morning 7 / afternoon 6）。
//
// 两者修复后的实测总偏差（`TestBackdropMatchesCSS` 钉住）：点对点 ≤1 灰阶；
// 反证见 `TestBackdropCounterProof`。

// Period 是一个时段：名称、生效的小时区间 [From, To)，以及原版那条 165°
// 多停靠点渐变。
type Period struct {
	Name     string
	From, To int
	// Angle 是渐变方向（CSS 的 `deg`，0° 朝上、顺时针）。原版 11 个时段全是 165。
	Angle float32
	// Stops 是渐变的颜色与**百分比位置**（At ∈ [0,1]，与 CSS 声明逐值一致）。
	Stops []gradStop
}

// Periods 是一天的时段，按时间顺序。
var Periods = []Period{
	{Name: "midnight", From: 0, To: 3, Angle: 165, Stops: []gradStop{{0, ui.Hex("#b8c4d8")}, {0.25, ui.Hex("#c0ccde")}, {0.5, ui.Hex("#c8d2e2")}, {0.75, ui.Hex("#c4cee0")}, {1, ui.Hex("#bcc6d8")}}},
	{Name: "predawn", From: 3, To: 6, Angle: 165, Stops: []gradStop{{0, ui.Hex("#bcc6dc")}, {0.2, ui.Hex("#c4cedd")}, {0.4, ui.Hex("#ccd4e0")}, {0.6, ui.Hex("#d2d8e2")}, {0.8, ui.Hex("#ccd2de")}, {1, ui.Hex("#c4cedc")}}},
	{Name: "dawn", From: 6, To: 7, Angle: 165, Stops: []gradStop{{0, ui.Hex("#fde8d8")}, {0.15, ui.Hex("#f5e0e8")}, {0.3, ui.Hex("#ece6f0")}, {0.5, ui.Hex("#dde8f4")}, {0.7, ui.Hex("#d4e4f0")}, {1, ui.Hex("#d8e8f2")}}},
	{Name: "morning", From: 7, To: 9, Angle: 165, Stops: []gradStop{{0, ui.Hex("#fef0e0")}, {0.12, ui.Hex("#fde8e4")}, {0.25, ui.Hex("#f0eaf0")}, {0.42, ui.Hex("#e4eaf4")}, {0.58, ui.Hex("#dceaf2")}, {0.75, ui.Hex("#e0eef4")}, {1, ui.Hex("#d8e6f0")}}},
	{Name: "midday", From: 9, To: 12, Angle: 165, Stops: []gradStop{{0, ui.Hex("#e8f0fa")}, {0.1, ui.Hex("#e2eaf5")}, {0.22, ui.Hex("#f0eef6")}, {0.35, ui.Hex("#e4e8f2")}, {0.48, ui.Hex("#dce6f0")}, {0.6, ui.Hex("#e8eef4")}, {0.72, ui.Hex("#d6e4ee")}, {0.85, ui.Hex("#e0eaf2")}, {1, ui.Hex("#d8e6f0")}}},
	{Name: "noon", From: 12, To: 14, Angle: 165, Stops: []gradStop{{0, ui.Hex("#f0f4fa")}, {0.15, ui.Hex("#eaf0f6")}, {0.3, ui.Hex("#f2eef4")}, {0.45, ui.Hex("#e6eaf2")}, {0.6, ui.Hex("#e0e8f0")}, {0.75, ui.Hex("#eaeef4")}, {1, ui.Hex("#e2eaf2")}}},
	{Name: "afternoon", From: 14, To: 17, Angle: 165, Stops: []gradStop{{0, ui.Hex("#fef4e8")}, {0.12, ui.Hex("#f8ece0")}, {0.25, ui.Hex("#f0e8ec")}, {0.42, ui.Hex("#e8eaf0")}, {0.58, ui.Hex("#e0e6f0")}, {0.75, ui.Hex("#e4eaf0")}, {1, ui.Hex("#dce8f0")}}},
	{Name: "golden", From: 17, To: 18, Angle: 165, Stops: []gradStop{{0, ui.Hex("#feddc8")}, {0.12, ui.Hex("#fad0c8")}, {0.25, ui.Hex("#f0d0d8")}, {0.4, ui.Hex("#e4d0e0")}, {0.55, ui.Hex("#d8d0e8")}, {0.7, ui.Hex("#d0d0e4")}, {1, ui.Hex("#d4d4e4")}}},
	{Name: "dusk", From: 18, To: 20, Angle: 165, Stops: []gradStop{{0, ui.Hex("#f5d8c8")}, {0.15, ui.Hex("#e8c8d8")}, {0.3, ui.Hex("#d8c8e8")}, {0.5, ui.Hex("#c8d0e8")}, {0.7, ui.Hex("#c0cee0")}, {1, ui.Hex("#c8d4e4")}}},
	{Name: "evening", From: 20, To: 22, Angle: 165, Stops: []gradStop{{0, ui.Hex("#8a9cb8")}, {0.2, ui.Hex("#96a6c4")}, {0.4, ui.Hex("#a2b2ce")}, {0.6, ui.Hex("#9aacbe")}, {0.8, ui.Hex("#8c9eb0")}, {1, ui.Hex("#8294a6")}}},
	{Name: "latenight", From: 22, To: 24, Angle: 165, Stops: []gradStop{{0, ui.Hex("#aeb8d0")}, {0.2, ui.Hex("#bcc4d8")}, {0.4, ui.Hex("#c8d0e0")}, {0.6, ui.Hex("#c0c8da")}, {0.8, ui.Hex("#b4becd")}, {1, ui.Hex("#a8b2c4")}}},
}

// PeriodAt 返回某个小时所属的时段。
func PeriodAt(hour int) Period {
	hour = ((hour % 24) + 24) % 24
	for _, p := range Periods {
		if hour >= p.From && hour < p.To {
			return p
		}
	}
	return Periods[len(Periods)-1]
}

// Sample 把时段的多停靠点渐变按位置 t（0..1）采样成一个颜色，与 CSS 的
// `linear-gradient()` 在渐变线上的插值逐值一致（含**非均匀**停靠点位置）。
func (p Period) Sample(t float32) ui.Color {
	return gradAt(p.Stops, t)
}

// Fill 按原版的 165° 方向铺满整块盒：直接交给框架的**多停靠点**线性渐变
// （v0.5.5 起 `ui.LinearGradient.Stops` 与 CSS 的 `linear-gradient(Adeg,
// c1 p1%, ...)` 逐值等价，最多 32 个停靠点；本文件最多 9 个）。
//
// 不要再退回「按 n 条等高盒子拼 gradBands」的写法：那等于把渐变线当成垂直的、
// 逐带近似，既丢了 165° 的斜向（底部一行左右同色），也把停靠点位置压成均匀。
func (p Period) Fill(e ui.Element) ui.Element {
	stops := make([]ui.GradientStop, len(p.Stops))
	for i, s := range p.Stops {
		stops[i] = ui.GradientStop{Color: s.C, At: s.At}
	}
	return e.LinearGradient(ui.LinearGradient{Angle: p.Angle, Stops: stops})
}

// BackdropBands 把时段渐变切成 bands 条，返回每条带的起止颜色。**仅供不需要
// 斜向的场合**（如测试里核对按列采样）；整窗背景请用 Fill（见上）。
func (p Period) BackdropBands(bands int) [][2]ui.Color {
	if bands < 1 {
		bands = 1
	}
	out := make([][2]ui.Color, bands)
	for i := range out {
		out[i] = [2]ui.Color{
			p.Sample(float32(i) / float32(bands)),
			p.Sample(float32(i+1) / float32(bands)),
		}
	}
	return out
}
