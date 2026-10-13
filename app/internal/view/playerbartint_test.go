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

// TestPlayerBarTintTokens 钉住染色令牌：浅色不着色；五个烟熏玻璃时段取
// `Theme.PlayerSurface`（rgba(30,41,59,.16)），且与浅色值不同。
func TestPlayerBarTintTokens(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	if got := playerBarTint(light); got != ui.Transparent {
		t.Errorf("浅色播放栏不应着色，得到 %v", got)
	}
	dark := ThemeFor(PeriodAt(0))
	want := ui.Hex("#1e293b").Alpha(0.16)
	if got := playerBarTint(dark); got != want {
		t.Errorf("烟熏玻璃播放栏染色 = %v，应为 %v", got, want)
	}
	if light.PlayerSurface == dark.PlayerSurface {
		t.Error("两个时段的 --player-surface 应不同（浅色近白、深色压暗）")
	}
	for _, hour := range []int{0, 4, 19, 21, 23} {
		th := ThemeFor(PeriodAt(hour))
		if !th.Dark {
			t.Fatalf("%d 点（%s）应为烟熏玻璃时段", hour, th.Period.Name)
		}
		if got := playerBarTint(th); got != th.PlayerSurface {
			t.Errorf("%d 点染色 = %v，应为 %v", hour, got, th.PlayerSurface)
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
