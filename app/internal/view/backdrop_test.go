package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// cssLinearT 按 CSS 规范把窗口坐标 (x,y) 投影到 `linear-gradient(Adeg, …)` 的
// 渐变线上，返回 0..1 的位置：方向向量从 0°（上）顺时针转 A，渐变线过盒中心、
// 长度取盒在该方向上的投影（`|W·sinA| + |H·cosA|`）。与框架
// `ui/paint.go` 的 `gradient()` 同一套语义。
func cssLinearT(x, y, w, h, angleDeg float32) float32 {
	a := float64(angleDeg) * math.Pi / 180
	dx, dy := float32(math.Sin(a)), float32(-math.Cos(a))
	half := (abs32(w*dx) + abs32(h*dy)) / 2
	x0, y0 := w/2-dx*half, h/2-dy*half
	return ((x-x0)*dx + (y-y0)*dy) / (2 * half)
}

// backdropCSSPct 是原版 globals.css 各时段 --liquid-backdrop 的停靠点百分比
// （逐值抄录，与 backdrop.go 的生成源同源）。
var backdropCSSPct = map[string][]float32{
	"midnight":  {0, 25, 50, 75, 100},
	"predawn":   {0, 20, 40, 60, 80, 100},
	"dawn":      {0, 15, 30, 50, 70, 100},
	"morning":   {0, 12, 25, 42, 58, 75, 100},
	"midday":    {0, 10, 22, 35, 48, 60, 72, 85, 100},
	"noon":      {0, 15, 30, 45, 60, 75, 100},
	"afternoon": {0, 12, 25, 42, 58, 75, 100},
	"golden":    {0, 12, 25, 40, 55, 70, 100},
	"dusk":      {0, 15, 30, 50, 70, 100},
	"evening":   {0, 20, 40, 60, 80, 100},
	"latenight": {0, 20, 40, 60, 80, 100},
}

// backdropWorstDiff 在 1000×700 上量「渲染（用当前 Periods 的这一项）vs 给定
// 角度与停靠点的参考采样」的最大逐通道差（灰阶）。
func backdropWorstDiff(t *testing.T, p Period, angle float32, stops []gradStop) int {
	t.Helper()
	ref := Period{Name: p.Name, From: p.From, To: p.To, Angle: angle, Stops: stops}
	tst := ui.NewTester(func(c *ui.Context) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(p)
		ui.Box(c).Fill().Children(func() { a.backdrop(c) })
	}, 1000, 700)
	tst.Frame()
	img := tst.Image()
	worst := 0
	for y := 0; y < 700; y += 7 {
		for x := 0; x < 1000; x += 7 {
			want := ref.Sample(cssLinearT(float32(x)+0.5, float32(y)+0.5, 1000, 700, angle))
			o := img.PixOffset(x, y)
			d := max(int(abs32(float32(img.Pix[o])-float32(want.R))),
				max(int(abs32(float32(img.Pix[o+1])-float32(want.G))),
					int(abs32(float32(img.Pix[o+2])-float32(want.B)))))
			if d > worst {
				worst = d
			}
		}
	}
	return worst
}

// TestBackdropMatchesCSS 钉住整窗背景 `#root { background: var(--liquid-backdrop) }`
// 的两处**此前抄漏、用户可感知**的点（见 backdrop.go 的说明）：
//
//  1. 方向是 **165°**，不是 180°（180° 是纯垂直，背景没了斜向感、底部一行左右同色）；
//  2. 停靠点的**百分比位置**要保留（原版各时段并不均分）。
//
// 三条判据：
//
//  1. 11 个时段的 Angle 都是 165；
//  2. 每个时段的停靠点位置与 globals.css 逐值一致（backdropCSSPct）；
//  3. 无头像素实测：11 个时段、1000×700 整个窗口上，渲染结果与「按 CSS 165°
//     精确采样 `Period.Sample`」的逐通道差 ≤ 1 灰阶。
//
// 反证见 TestBackdropCounterProof（实测过）。
func TestBackdropMatchesCSS(t *testing.T) {
	// 判据 1 + 2。
	for _, p := range Periods {
		if p.Angle != 165 {
			t.Errorf("%s：Angle = %v，原版 --liquid-backdrop 是 165deg", p.Name, p.Angle)
		}
		want := backdropCSSPct[p.Name]
		if len(p.Stops) != len(want) {
			t.Errorf("%s：停靠点 %d 个，原版 %d 个", p.Name, len(p.Stops), len(want))
			continue
		}
		for i, pc := range want {
			if got := p.Stops[i].At * 100; abs32(got-pc) > 1e-3 {
				t.Errorf("%s：第 %d 个停靠点位置 %v%%，原版 %v%%", p.Name, i, got, pc)
			}
		}
	}

	// 判据 3：无头像素实测。
	for _, p := range Periods {
		worst := backdropWorstDiff(t, p, p.Angle, p.Stops)
		if worst > 1 {
			t.Errorf("%s：渲染与 CSS 165° 精确采样最大差 %d 灰阶，应 ≤1", p.Name, worst)
		}
	}

	// 按列采样接口的语义没变（端点等于首尾停靠点），供仍按列取色的调用点使用。
	noon := PeriodAt(12)
	if noon.Sample(0) != noon.Stops[0].C || noon.Sample(1) != noon.Stops[len(noon.Stops)-1].C {
		t.Error("Sample 的端点应等于首尾停靠点")
	}
}

// TestBackdropCounterProof 是 TestBackdropMatchesCSS 的反证：把两处修复各退回
// 旧写法，与**CSS 参考**（固定 165° + 原版百分比位置）的像素差应显著 > 1
// （相反地，TestBackdropMatchesCSS 判据 3 对同一参考要求 ≤1）。
//
// 实测值（1000×700，7px 网格）：
//
//	退回 180°：golden 22 / dusk 29 / midday 18 / noon 14 灰阶
//	退回均匀位置：golden 9 / dawn 8 / morning 7 / afternoon 6 灰阶
func TestBackdropCounterProof(t *testing.T) {
	// 参考 = 当前（正确的）色值 + 原版角度与位置。
	ref := func(name string) Period {
		p := periodByName(name)
		n := len(backdropCSSPct[name])
		st := make([]gradStop, n)
		for i := range st {
			st[i] = gradStop{At: backdropCSSPct[name][i] / 100, C: p.Stops[i].C}
		}
		return Period{Name: name, Angle: 165, Stops: st}
	}
	saved := append([]Period(nil), Periods...)
	defer func() { Periods = saved }()

	// 反证①：角度退回 180。
	for i := range Periods {
		Periods[i] = saved[i]
		Periods[i].Angle = 180
	}
	for _, name := range []string{"golden", "dusk", "midday", "noon"} {
		q := periodByName(name)
		r := ref(name)
		if worst := backdropWorstDiff(t, q, r.Angle, r.Stops); worst <= 1 {
			t.Errorf("%s：角度退回 180 后与 CSS 只差 %d 灰阶，反证不成立", name, worst)
		}
	}

	// 反证②：停靠点位置退回均匀分布。
	for i := range Periods {
		Periods[i] = saved[i]
		n := len(Periods[i].Stops)
		st := make([]gradStop, n)
		for j := range st {
			st[j] = gradStop{At: float32(j) / float32(n-1), C: saved[i].Stops[j].C}
		}
		Periods[i].Stops = st
	}
	for _, name := range []string{"dawn", "morning", "afternoon", "golden"} {
		q := periodByName(name)
		r := ref(name)
		if worst := backdropWorstDiff(t, q, r.Angle, r.Stops); worst <= 1 {
			t.Errorf("%s：位置退回均匀后与 CSS 只差 %d 灰阶，反证不成立", name, worst)
		}
	}
}
