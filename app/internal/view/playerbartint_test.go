package view

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（烟熏玻璃时段播放栏底色）的无头测试。原版 `#player .player-controls`
// 的底色走 `background-color: var(--player-surface, rgba(241,248,253,.36))`：
// 浅色兜底是近白，五个烟熏玻璃时段覆写成 rgba(30,41,59,.16)。原生版两层
// 播放栏（主窗 + mini）的 Liquid Glass 此前都没染色，深色时段与浅色长得
// 一模一样——`Theme.PlayerSurface` 令牌也一直没有使用点。

// Keep each time-of-day tint while letting more of the backdrop through.
func TestPlayerBarTintTokens(t *testing.T) {
	for _, hour := range []int{0, 4, 12, 19, 21, 23} {
		th := ThemeFor(PeriodAt(hour))
		got := playerBarTint(th)
		if got.R != th.PlayerSurface.R || got.G != th.PlayerSurface.G || got.B != th.PlayerSurface.B {
			t.Fatalf("%d: lost time-of-day tint: %v", hour, got)
		}
		if got.A == 0 || got.A >= th.PlayerSurface.A {
			t.Fatalf("%d: player veil must be lighter than the original surface: %v", hour, got)
		}
		th = th.withPreferences(false, true)
		if playerBarTint(th).A != 255 {
			t.Fatal("reduced transparency must stay opaque")
		}
	}
}

// barStripLuma 采样一行像素的平均亮度（0..255）。取样行落在播放栏按钮之上
// 的纯玻璃区，所以亮度变化只可能来自玻璃本身的染色。
func barStripLuma(tst *ui.Tester, y, x0, x1 int) float64 {
	img := tst.Image()
	var sum float64
	n := 0
	for x := x0; x < x1; x++ {
		c := img.RGBAAt(x, y)
		sum += 0.2126*float64(c.R) + 0.7152*float64(c.G) + 0.0722*float64(c.B)
		n++
	}
	return sum / float64(n)
}

// TestPlayerBarTintWired 无头反证：给播放栏玻璃换一个不透明黑染色，栏体
// （避开按钮的纯玻璃行）应明显变暗——证明 `playerBarTint` 的返回值真的进了
// `glass.Glass{Tint: …}`。主窗与 mini 两层都要验（同一函数、两处调用点）。
func TestPlayerBarTintWired(t *testing.T) {
	orig := playerBarTint
	defer func() { playerBarTint = orig }()

	// 主窗：barTop = 音量键 Y − 12（栏 56 / 键 32 居中，差 12）。
	main := func(tint ui.Color) float64 {
		playerBarTint = func(Theme) ui.Color { return tint }
		_, tst := playerBarProbe(t, nil)
		r, ok := tst.Find("音量")
		if !ok {
			t.Fatal("主窗播放栏找不到音量键")
		}
		return barStripLuma(tst, int(r.Y)-12+4, 20, 980)
	}
	if clear, dark := main(ui.Transparent), main(ui.Hex("#000000")); dark >= clear {
		t.Errorf("主窗染色后播放栏应变暗：无色=%v 黑色=%v", clear, dark)
	}

	// mini：barTop = 音量键 Y − 8（栏 43 / 键 28 居中，差 8）。
	mini := func(tint ui.Color) float64 {
		playerBarTint = func(Theme) ui.Color { return tint }
		a := NewApp(func() {})
		a.Mini = true
		a.Theme = ThemeFor(PeriodAt(12))
		a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
		tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
		tst.Frame()
		r, ok := tst.Find("音量")
		if !ok {
			t.Fatal("mini 播放栏找不到音量键")
		}
		return barStripLuma(tst, int(r.Y)-8+4, 20, miniWidth-20)
	}
	if clear, dark := mini(ui.Transparent), mini(ui.Hex("#000000")); dark >= clear {
		t.Errorf("mini 染色后播放栏应变暗：无色=%v 黑色=%v", clear, dark)
	}
}
