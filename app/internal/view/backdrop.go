package view

import "github.com/egoist/mygo/ui"

// 本文件由 src/styles/globals.css 的 --liquid-backdrop 生成，请勿手改。
//
// 原版按一天 11 个时段切换窗口背景：一条 165° 的多停靠点线性渐变，
// 从冷蓝（深夜）经暖金（清晨、午后）再回到蓝灰（夜晚）。这是 bili-FM
// 视觉签名的主体，所有玻璃层都浮在它上面。

// Period 是一个时段：名称、生效的小时区间 [From, To)，以及从上到下的渐变色。
type Period struct {
	Name     string
	From, To int
	Stops    []ui.Color
}

// Periods 是一天的时段，按时间顺序。
var Periods = []Period{
	{Name: "midnight", From: 0, To: 3, Stops: []ui.Color{ui.Hex("#b8c4d8"), ui.Hex("#c0ccde"), ui.Hex("#c8d2e2"), ui.Hex("#c4cee0"), ui.Hex("#bcc6d8")}},
	{Name: "predawn", From: 3, To: 6, Stops: []ui.Color{ui.Hex("#bcc6dc"), ui.Hex("#c4cedd"), ui.Hex("#ccd4e0"), ui.Hex("#d2d8e2"), ui.Hex("#ccd2de"), ui.Hex("#c4cedc")}},
	{Name: "dawn", From: 6, To: 7, Stops: []ui.Color{ui.Hex("#fde8d8"), ui.Hex("#f5e0e8"), ui.Hex("#ece6f0"), ui.Hex("#dde8f4"), ui.Hex("#d4e4f0"), ui.Hex("#d8e8f2")}},
	{Name: "morning", From: 7, To: 9, Stops: []ui.Color{ui.Hex("#fef0e0"), ui.Hex("#fde8e4"), ui.Hex("#f0eaf0"), ui.Hex("#e4eaf4"), ui.Hex("#dceaf2"), ui.Hex("#e0eef4"), ui.Hex("#d8e6f0")}},
	{Name: "midday", From: 9, To: 12, Stops: []ui.Color{ui.Hex("#e8f0fa"), ui.Hex("#e2eaf5"), ui.Hex("#f0eef6"), ui.Hex("#e4e8f2"), ui.Hex("#dce6f0"), ui.Hex("#e8eef4"), ui.Hex("#d6e4ee"), ui.Hex("#e0eaf2"), ui.Hex("#d8e6f0")}},
	{Name: "noon", From: 12, To: 14, Stops: []ui.Color{ui.Hex("#f0f4fa"), ui.Hex("#eaf0f6"), ui.Hex("#f2eef4"), ui.Hex("#e6eaf2"), ui.Hex("#e0e8f0"), ui.Hex("#eaeef4"), ui.Hex("#e2eaf2")}},
	{Name: "afternoon", From: 14, To: 17, Stops: []ui.Color{ui.Hex("#fef4e8"), ui.Hex("#f8ece0"), ui.Hex("#f0e8ec"), ui.Hex("#e8eaf0"), ui.Hex("#e0e6f0"), ui.Hex("#e4eaf0"), ui.Hex("#dce8f0")}},
	{Name: "golden", From: 17, To: 18, Stops: []ui.Color{ui.Hex("#feddc8"), ui.Hex("#fad0c8"), ui.Hex("#f0d0d8"), ui.Hex("#e4d0e0"), ui.Hex("#d8d0e8"), ui.Hex("#d0d0e4"), ui.Hex("#d4d4e4")}},
	{Name: "dusk", From: 18, To: 20, Stops: []ui.Color{ui.Hex("#f5d8c8"), ui.Hex("#e8c8d8"), ui.Hex("#d8c8e8"), ui.Hex("#c8d0e8"), ui.Hex("#c0cee0"), ui.Hex("#c8d4e4")}},
	{Name: "evening", From: 20, To: 22, Stops: []ui.Color{ui.Hex("#8a9cb8"), ui.Hex("#96a6c4"), ui.Hex("#a2b2ce"), ui.Hex("#9aacbe"), ui.Hex("#8c9eb0"), ui.Hex("#8294a6")}},
	{Name: "latenight", From: 22, To: 24, Stops: []ui.Color{ui.Hex("#aeb8d0"), ui.Hex("#bcc4d8"), ui.Hex("#c8d0e0"), ui.Hex("#c0c8da"), ui.Hex("#b4becd"), ui.Hex("#a8b2c4")}},
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

// Sample 把时段的多停靠点渐变按位置 t（0..1，从上到下）采样成一个颜色。
// ui.LinearGradient 只有两个颜色，画背景时按条带取相邻两点；接缝处颜色
// 相同，所以看不出拼接。
func (p Period) Sample(t float32) ui.Color {
	n := len(p.Stops)
	if n == 0 {
		return ui.Hex("#ffffff")
	}
	if n == 1 {
		return p.Stops[0]
	}
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	x := t * float32(n-1)
	i := int(x)
	if i >= n-1 {
		return p.Stops[n-1]
	}
	return p.Stops[i].Mix(p.Stops[i+1], x-float32(i))
}

// BackdropBands 把时段渐变切成 bands 条，返回每条带的起止颜色，
// 供视图叠出整块背景。
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
