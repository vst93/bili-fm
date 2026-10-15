package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏时间标签墨色）的无头回归测试。
//
// 原版 `#player .player-time` 是 `color: var(--player-time-color, #64748b)`，
// 而 `--player-time-color` **只在**烟熏玻璃时段（midnight/predawn/dusk/
// evening/latenight）的 `#player` 块里定义为 `rgba(51,65,85,.72)`；浅色时段
// 根本没定义，落到规则里的兜底 `#64748b`。原生版此前把深色值
// `#334155`+.72 硬编码进 `timeTextAt`（主窗与 mini 共用），于是浅色时段的时间
// 文本比原版暗一档、也少一点蓝灰调。
//
// 这里钉两层：令牌按时段取对（纯函数），以及视图真的把按时段取的色送到时间
// 标签（换桩反证返回值进了 `.player-time` 的 color）。

// TestPlayerTimeInkTokens 钉住两套时段的 `--player-time-color`：
// 浅色是兜底 #64748b（不透明），烟熏玻璃是 rgba(51,65,85,.72)，两者必须不同。
func TestPlayerTimeInkTokens(t *testing.T) {
	lt := ThemeFor(PeriodAt(12))
	dt := ThemeFor(PeriodAt(0))

	if lt.Dark {
		t.Fatal("12 点不应是烟熏玻璃时段")
	}
	if !dt.Dark {
		t.Fatal("0 点应是烟熏玻璃时段")
	}

	if got := lt.PlayerTime; got != ui.Hex("#64748b") {
		t.Errorf("浅色时间墨色 = %v，应为兜底 #64748b", got)
	}
	if got := dt.PlayerTime; got != ui.Hex("#334155").Alpha(0.72) {
		t.Errorf("深色时间墨色 = %v，应为 rgba(51,65,85,.72)", got)
	}
	// 回归判据：两套时段不能再共用同一个值（此前两边都是深色值）。
	if lt.PlayerTime == dt.PlayerTime {
		t.Error("浅色与深色时间墨色相同，说明令牌没按时段切")
	}
	// playerTimeInk 默认就是取主题令牌。
	if got := playerTimeInk(lt); got != lt.PlayerTime {
		t.Errorf("playerTimeInk(浅色) = %v，应为 Theme.PlayerTime %v", got, lt.PlayerTime)
	}
	if got := playerTimeInk(dt); got != dt.PlayerTime {
		t.Errorf("playerTimeInk(深色) = %v，应为 Theme.PlayerTime %v", got, dt.PlayerTime)
	}
}

// timeLabelRedPixels 数时间标签矩形里「红压倒绿蓝」的像素。换桩返回纯红后，
// 只要这些像素出现，就证明返回值真的进了文字颜色（而不是被视图忽略）。
func timeLabelRedPixels(img *image.RGBA, r ui.Rect) int {
	n := 0
	x0, y0 := int(r.X), int(r.Y)
	x1, y1 := int(r.X+r.W), int(r.Y+r.H)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= img.Rect.Dx() || y >= img.Rect.Dy() {
				continue
			}
			i := img.PixOffset(x, y)
			rr, gg, bb := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			if rr > gg+40 && rr > bb+40 {
				n++
			}
		}
	}
	return n
}

// TestPlayerTimeInkWired 无头反证：视图把 playerTimeInk 的返回值送进时间标签。
//
// 手法：换桩成「记录主题 + 返回纯红」，渲染一帧后：
//   - 桩被调用 ≥2 次（当前时间 + 总时长），且每次收到的都是 app 的主题；
//   - 时间标签矩形里出现红像素 —— 返回值真的被用上（把 TextColor 写死别的
//     颜色、或让桩返回透明，这条即变红）。
func TestPlayerTimeInkWired(t *testing.T) {
	orig := playerTimeInk
	defer func() { playerTimeInk = orig }()

	var seen []Theme
	playerTimeInk = func(th Theme) ui.Color {
		seen = append(seen, th)
		return ui.Hex("#ff0000")
	}

	a, tst := playerBarProbe(t, nil)
	img := tst.Image()

	if len(seen) < 2 {
		t.Fatalf("时间墨色桩只被调用 %d 次，应至少 2 次（当前时间 + 总时长）", len(seen))
	}
	for i, th := range seen {
		if th.Period.Name != a.Theme.Period.Name {
			t.Errorf("第 %d 次调用收到的时段 %q，应为 app 时段 %q", i, th.Period.Name, a.Theme.Period.Name)
		}
	}

	label := fmtTime(a.shownPos())
	r, ok := tst.Find(label)
	if !ok {
		t.Fatalf("渲染帧里找不到时间标签 %q", label)
	}
	if n := timeLabelRedPixels(img, r); n == 0 {
		t.Errorf("时间标签 %q 区域没有红像素，playerTimeInk 的返回值没进文字颜色", label)
	}
}

// timeGlyphBounds 在无头渲染里量出某个时间标签**文字**的水平包围盒。
// Find 返回的是 56px 的列盒，不是文字本身的宽度——两者一对比就能看出文字
// 在盒内是居中的还是左贴的。
func timeGlyphBounds(t *testing.T, tst *ui.Tester, label string) (minX, maxX int, r ui.Rect, ok bool) {
	t.Helper()
	rect, ok := tst.Find(label)
	if !ok {
		return 0, 0, rect, false
	}
	out := tst.Image()
	minX, maxX = 1<<30, -1
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			if x < 0 || y < 0 || x >= out.Rect.Dx() || y >= out.Rect.Dy() {
				continue
			}
			i := out.PixOffset(x, y)
			lum := (int(out.Pix[i])*299 + int(out.Pix[i+1])*587 + int(out.Pix[i+2])*114) / 1000
			if lum < 150 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	return minX, maxX, rect, maxX >= 0
}

// TestPlayerTimeTextIsCenteredInColumn 是无头渲染实测：左右两端的时间文字要在
// 各自的 56px 列里**水平居中**（原版 .player-time 的 `text-align: center`）。
//
// 之前这里用 `.Center()` —— 它只是把**列盒**在父级里摆到居中，文字本身仍按
// 默认左对齐，于是 「1:05」 和 「1:02:05」 都贴着列的左边，两端时间看着没有
// 左右居中。修正为 TextAlign(Center) 后，文字包围盒的中心应落在列盒中心附近。
func TestPlayerTimeTextIsCenteredInColumn(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Pos = 65
	a.Dur = 3725
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()

	for _, label := range []string{"1:05", "1:02:05"} {
		minX, maxX, r, ok := timeGlyphBounds(t, tst, label)
		if !ok {
			t.Fatalf("找不到时间文本 %q：%v", label, tst.Texts())
		}
		boxCenter := r.X + r.W/2
		glyphCenter := float32(minX+maxX) / 2
		t.Logf("%q：列盒 %+v（中心 %.1f）文字 x %d..%d（中心 %.1f），偏 %.1f",
			label, r, boxCenter, minX, maxX, glyphCenter, glyphCenter-boxCenter)
		if d := glyphCenter - boxCenter; d > 2 || d < -2 {
			t.Errorf("%q 文字中心偏列盒中心 %.1fpx（应为居中）", label, d)
		}
	}
}
