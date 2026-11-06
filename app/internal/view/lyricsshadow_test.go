package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLyricsTextShadowInk 钉住桌面字幕文字投影的判据（mygo 0.8.0 的
// Element.TextShadow）。
//
// 字幕窗背景透明度可以调到 0（纯文字浮在任意画面上），那时白字压在浅色内容上
// 几乎看不见 —— 一层很淡的深色投影把字形托起来。但不能无条件画：
//   - 面板底色够实时投影只会让字变糊（原版面板不透明时文字就是纯色）；
//   - 深色文字压在浅色画面上本来就清楚，再加深投影会糊成一团。
func TestLyricsTextShadowInk(t *testing.T) {
	white := ui.Hex("#ffffff")
	dark := ui.Hex("#0f172a")

	for _, tc := range []struct {
		name       string
		text       ui.Color
		bg, textOp float32
		wantCast   bool
	}{
		{"全透明面板 + 白字：要投影", white, 0, 1, true},
		{"半透明面板 + 白字：要投影", white, 0.3, 1, true},
		{"不透明面板 + 白字：不要", white, 0.9, 1, false},
		{"面板刚好到阈值：不要", white, 0.55, 1, false},
		{"深色字 + 透明面板：不要", dark, 0, 1, false},
		{"文字全透明：不要", white, 0, 0, false},
	} {
		got := lyricsTextShadowInk(tc.text, tc.bg, tc.textOp)
		casts := got != ui.Transparent
		if casts != tc.wantCast {
			t.Errorf("%s：投影 = %v（透明=%v），想 cast=%v", tc.name, got, !casts, tc.wantCast)
			continue
		}
		if !casts {
			continue
		}
		// 投影必须是深色，且 alpha 有上限（再重就成了描边）。
		if got.R > 60 || got.G > 60 || got.B > 60 {
			t.Errorf("%s：投影不是深色：%v", tc.name, got)
		}
		if a := float64(got.A) / 255; a > 0.55+0.01 {
			t.Errorf("%s：投影 alpha = %.3f，应 ≤ .55", tc.name, a)
		}
	}

	// 面板越透明 / 文字越淡，投影要越实（保证「托得住」的单调性）。
	lo := lyricsTextShadowInk(white, 0.5, 1)
	hi := lyricsTextShadowInk(white, 0.0, 1)
	if hi.A <= lo.A {
		t.Errorf("全透明面板的投影 alpha (%d) 应大于半透明面板的 (%d)", hi.A, lo.A)
	}
}

// TestLyricsTextIsDark 钉住深色判定（YIQ 感知亮度，与本仓库 lum 同系数）。
func TestLyricsTextIsDark(t *testing.T) {
	for _, c := range []struct {
		name string
		col  ui.Color
		want bool
	}{
		{"白", ui.Hex("#ffffff"), false},
		{"浅蓝", ui.Hex("#38bdf8"), false},
		{"浅黄", ui.Hex("#facc15"), false},
		{"深蓝黑", ui.Hex("#0f172a"), true},
		{"纯黑", ui.Hex("#000000"), true},
		{"深灰", ui.Hex("#1e293b"), true},
	} {
		if got := lyricsTextIsDark(c.col); got != c.want {
			t.Errorf("%s：lyricsTextIsDark = %v，想 %v", c.name, got, c.want)
		}
	}
}

// TestLyricsRendersTextShadow 是绘制实测的反证：全透明面板 + 白字时，字形周围
// 真的出现了深色像素（投影），而不透明面板时没有。
func TestLyricsRendersTextShadow(t *testing.T) {
	render := func(bgOpacity float64) [3]int {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(20)) // 深色时段，投影在浅底上更显
		a.LyricsBgOpacity = bgOpacity
		a.LyricsBgColor = "#ffffff"
		a.LyricsColor = "#ffffff"
		a.LyricCurrent = "字幕"
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Fill().Background(ui.Hex("#f8fafc"))
			a.lyricsPreview(c, 100, false)
		}, 400, 140)
		tst.Frame()
		img := tst.Image()
		// 取文字行下方一点点：投影落在字的下缘（TextShadow 的 y 偏移 1 + 模糊）。
		// 用整幅最暗的像素代表「有没有投影」。
		darkest := 255
		for y := 0; y < img.Rect.Dy(); y++ {
			for x := 0; x < img.Rect.Dx(); x++ {
				if l := lum(pixelAt(t, img, x, y)); l < darkest {
					darkest = l
				}
			}
		}
		return [3]int{darkest, 0, 0}
	}

	transparent := render(0)
	opaque := render(0.9)
	t.Logf("全透明面板最暗像素 %d，不透明面板 %d", transparent[0], opaque[0])
	if transparent[0] >= opaque[0] {
		t.Errorf("全透明面板最暗像素 %d，不透明面板 %d —— 透明面板应因投影更暗",
			transparent[0], opaque[0])
	}
}
