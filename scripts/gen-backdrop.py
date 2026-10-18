#!/usr/bin/env python3
"""从原版 globals.css 重新生成 app/internal/view/backdrop.go。

用法（仓库根）：
    git show main:src/styles/globals.css > /tmp/globals.css
    python3 scripts/gen-backdrop.py /tmp/globals.css > app/internal/view/backdrop.go

为什么要脚本：`--liquid-backdrop` 是 11 个时段的 165° 多停靠点渐变。手工抄写
容易丢两样东西（2026-10-24 就丢过，用户可感知）：
  1. 角度 165°（被写成 180°，背景失去斜向感、底部一行左右同色）；
  2. 每个停靠点的**百分比位置**（被按均匀分布隐式重建）。

脚本把角度与位置都抄下来，`TestBackdropMatchesCSS`（backdrop_test.go）再逐值
钉住，两边互为校验。
"""
import re
import sys

# 时段的名称与小时区间（原版 src/App.tsx 的 updateTimeOfDay）。
HOURS = [
    ("midnight", 0, 3), ("predawn", 3, 6), ("dawn", 6, 7), ("morning", 7, 9),
    ("midday", 9, 12), ("noon", 12, 14), ("afternoon", 14, 17), ("golden", 17, 18),
    ("dusk", 18, 20), ("evening", 20, 22), ("latenight", 22, 24),
]

TAIL = r'''
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
'''

HEADER = '''package view

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
'''


def parse(css):
    out = {}
    pat = re.compile(r'html\[data-time-of-day="([a-z]+)"\]\s*#root\s*\{', re.S)
    for m in pat.finditer(css):
        start, depth, i = m.end(), 1, m.end()
        while depth > 0 and i < len(css):
            if css[i] == '{':
                depth += 1
            elif css[i] == '}':
                depth -= 1
            i += 1
        body = css[start:i - 1]
        g = re.search(r'--liquid-backdrop:(.*?);', body, re.S)
        if not g:
            continue
        txt = g.group(1)
        ang = float(re.search(r'([\d.]+)deg', txt).group(1))
        stops = re.findall(r'(#[0-9a-fA-F]{6})\s+([\d.]+)%', txt)
        out[m.group(1)] = (ang, stops)
    return out


def main():
    css = open(sys.argv[1], encoding='utf-8').read() if len(sys.argv) > 1 else sys.stdin.read()
    data = parse(css)
    missing = [n for n, _, _ in HOURS if n not in data]
    if missing:
        sys.exit("globals.css 里缺少时段: %s" % ", ".join(missing))
    lines = [HEADER]
    for name, f, t in HOURS:
        ang, stops = data[name]
        body = ", ".join('{%g, ui.Hex("%s")}' % (float(p) / 100, c) for c, p in stops)
        lines.append('\t{Name: "%s", From: %d, To: %d, Angle: %g, Stops: []gradStop{%s}},\n'
                     % (name, f, t, ang, body))
    lines.append('}\n')
    lines.append(TAIL)
    sys.stdout.write("".join(lines))


if __name__ == '__main__':
    main()
