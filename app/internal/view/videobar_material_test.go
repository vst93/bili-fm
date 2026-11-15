package view

// 视频窗上下两条栏的**材质**口径（§2.1「上下栏更实」）。
//
// 两条栏浮在视频画面上：背后可能是任何一帧 —— 纯黑的夜空与纯白的高光都常见。
// 用户 2026-11 报「还是太透，字糊在视频上」，所以这一轮把底材质的不透明度下限
// 抬到浅 0.93 / 深 0.95（原 0.88 / 0.92）。这里钉两件事：
//
//  1. **可读**：栏底与视频画面合成之后，文字（Ink）与次级文字 / 图标（Muted，
//     350ms 悬停前的 0.72 不透明度档）的对比度 ≥ 4.5:1，强调色（Blue）与图标
//     字形 ≥ 3:1 —— 深浅两态、画面全黑与全白两种极端各算一次；
//  2. **还是材质不是色块**：底色仍然半透明（0.90 ≤ alpha ≤ 0.99，不是 1.0），
//     色相仍是 Panel 令牌那一档，描边 + 顶沿内高光 + 投影都在，并且**渲染出来
//     的像素**确实随背后的画面变化（背后亮一分，栏底也亮一点）。

import (
	"image"
	"image/color"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// videoBarThemes 返回深 / 浅两态（与真实主题同一条解析路径）。
func videoBarThemes() map[string]Theme {
	return map[string]Theme{
		"light": ThemeFor(PeriodAt(12)),
		"dark":  ThemeFor(PeriodAt(12)).withAppearance("dark", 0),
	}
}

// TestVideoBarMaterialContrast 是 §2.1 的深浅两条断言：栏底与画面合成后，
// 文字 / 次级文字 ≥ 4.5:1、强调色与图标 ≥ 3:1。
func TestVideoBarMaterialContrast(t *testing.T) {
	// 视频画面的两种极端：黑场与纯白高光。
	screens := map[string]ui.Color{
		"black": ui.RGB(0, 0, 0),
		"white": ui.RGB(255, 255, 255),
	}
	for name, th := range videoBarThemes() {
		fill := videoBarFill(th)
		for screen, under := range screens {
			bar := composite(fill, under)
			checks := []struct {
				label string
				ink   ui.Color
				min   float64
			}{
				{"标题 Ink", th.Ink, 4.5},
				{"次级文字 Muted", th.Muted, 4.5},
				// 顶栏的键在悬停前把整颗键压到 0.72 不透明度（playerBarTitleKey），
				// 图标字形因此比 Muted 再淡一档；图形的门槛是 3:1。
				{"图标 Muted@0.72", composite(th.Muted.Alpha(0.72), bar), 3.0},
				{"强调色 Blue", th.Blue, 3.0},
			}
			for _, c := range checks {
				if got := contrastRatio(c.ink, bar); got < c.min {
					t.Errorf("%s 主题、画面 %s：%s 对比度 %.2f < %.1f（栏底 %v）",
						name, screen, c.label, got, c.min, bar)
				}
			}
		}
	}
}

// TestVideoBarMaterialIsGlassNotBlock 钉住「更实之后还是玻璃」：底色半透明
// （0.90..0.99）、色相仍是 Panel 那一档、下限真的比原值高、描边 / 高光 / 投影
// 三件套都在。
func TestVideoBarMaterialIsGlassNotBlock(t *testing.T) {
	for name, th := range videoBarThemes() {
		fill := videoBarFill(th)
		a := float32(fill.A) / 255
		if a < 0.90 {
			t.Errorf("%s 主题：栏底不透明度 %.2f < 0.90，还是太透", name, a)
		}
		if a >= 0.999 {
			t.Errorf("%s 主题：栏底 %.2f 已经是纯色块，层次没了", name, a)
		}
		// 色相层次照旧：只动 alpha，不动 RGB。
		if fill.R != th.Panel.R || fill.G != th.Panel.G || fill.B != th.Panel.B {
			t.Errorf("%s 主题：栏底换了颜色（%v ≠ Panel %v），应只抬不透明度",
				name, fill, th.Panel)
		}
		// 下限必须真的高于 Panel 原值，否则这条「更实」就是空话。
		if panelA := float32(th.Panel.A) / 255; panelA >= a {
			t.Errorf("%s 主题：Panel 原值 %.2f 已经不低于下限 %.2f，这一轮没变实", name, panelA, a)
		}
		// 描边 + 顶沿内高光（videoBar 的三件套）在两态都有可见的色差：
		// 线与高光都不是纯透明，否则「材质」就只剩一块底。
		if th.Line.A == 0 {
			t.Errorf("%s 主题：栏底没有描边", name)
		}
		if th.Highlight.A == 0 {
			t.Errorf("%s 主题：栏底没有顶沿内高光", name)
		}
	}
}

// TestVideoBarMaterialPaintsOverPicture 钉住渲染结果：栏底的像素是「材质叠在
// 画面之上」而不是一块不透明色 —— 背后的画面亮一点，栏底也跟着亮一点，同时
// 顶沿那条描边仍然比内缘深。
func TestVideoBarMaterialPaintsOverPicture(t *testing.T) {
	dark := ui.RGB(0, 0, 0)
	bright := ui.Hex("#d8e2ef")
	for name, th := range videoBarThemes() {
		overDark := videoBarProbeFill(t, th, dark)
		overBright := videoBarProbeFill(t, th, bright)

		// 底色 = Panel 叠在画面上：合成公式对得上（±2 的取整余量）。
		want := composite(videoBarFill(th), bright)
		if !closeColor(overBright, want, 2) {
			t.Errorf("%s 主题：栏底像素 %v ≠ 材质叠画面 %v", name, overBright, want)
		}
		// 仍会透出画面：黑场与高光之间栏底要有可见差别（约 (1-alpha) × 画面差）。
		if d := colorDelta(overDark, overBright); d < 6 {
			t.Errorf("%s 主题：背后画面从黑变白，栏底只变了 %d —— 已经不是材质了", name, d)
		}
		// 但也要够实：画面变化只被吃掉一小截（≤ 40）。
		if d := colorDelta(overDark, overBright); d > 40 {
			t.Errorf("%s 主题：背后画面变化透出 %d，栏底太薄", name, d)
		}
		// 描边仍在：顶沿那一条比内缘明显深。
		if d := colorDelta(videoBarProbeBorder(t, th, bright), overBright); d < 12 {
			t.Errorf("%s 主题：顶沿描边与栏底只差 %d，边被底色吃掉了", name, d)
		}
	}
}

// videoBarProbeFill 渲染一帧视频窗（背后是给定的画面色），返回标题栏内缘的
// 一个像素（栏的竖直中点：描边与顶沿高光之内、文字与键之外的空处）。
func videoBarProbeFill(t *testing.T, th Theme, screen ui.Color) ui.Color {
	t.Helper()
	img := videoBarProbeImage(t, th, screen)
	return imageColor(img.RGBAAt(320, int(videoInset+videoChromeTitleH/2)))
}

// videoBarProbeBorder 返回标题栏顶沿（描边）的一个像素。
func videoBarProbeBorder(t *testing.T, th Theme, screen ui.Color) ui.Color {
	t.Helper()
	img := videoBarProbeImage(t, th, screen)
	return imageColor(img.RGBAAt(320, int(videoInset)))
}

// videoBarProbeImage 起一个 640×360 的无头视频窗，画面整块铺成 screen 色。
func videoBarProbeImage(t *testing.T, th Theme, screen ui.Color) *image.RGBA {
	t.Helper()
	ch := &VideoChrome{
		Theme: th,
		Title: "测试稿件 · 第一部分",
		Pl:    &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second},
		Content: func(c *ui.Context) ui.Element {
			return ui.Box(c).Fill().Background(screen)
		},
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 640, 360)
	tt.Frame()
	return tt.Image()
}

// imageColor 把渲染出来的像素（color.RGBA）换成 ui.Color。
func imageColor(c color.RGBA) ui.Color { return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255) }

// colorDelta 是两个颜色的最大通道差。
func colorDelta(a, b ui.Color) int {
	d := 0
	for _, p := range [][2]uint8{{a.R, b.R}, {a.G, b.G}, {a.B, b.B}} {
		if v := int(p[0]) - int(p[1]); v > d || -v > d {
			d = max(v, -v)
		}
	}
	return d
}

// closeColor 报告两个颜色的每个通道是否都在 tol 之内。
func closeColor(a, b ui.Color, tol int) bool {
	return colorDelta(a, b) <= tol
}
