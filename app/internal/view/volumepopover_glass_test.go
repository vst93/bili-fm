package view

import (
	"testing"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// 本轮（音量 / 倍速弹层的烟熏玻璃材质）测试。原版 globals.css 的五个烟熏玻璃
// 时段给两层弹层（.player-volume-popover / .player-speed-popover）覆写了整套
// 材质：
//
//	html:is([data-time-of-day="midnight"|"predawn"|"dusk"|"evening"|"latenight"])
//	#player .player-volume-popover,
//	#player .player-speed-popover {
//	    border-color: rgba(255, 255, 255, 0.2);
//	    background-color: rgba(71, 85, 105, 0.64);
//	    background-image: var(--player-sheen);
//	    box-shadow:
//	        inset 0 1px 0 rgba(255, 255, 255, 0.26),
//	        0 10px 24px rgba(15, 23, 42, 0.14);
//	}
//
// 此前原生版只落了底色的 Tint（playerPopoverTint）与洗光（volume/speedPopoverSheen
// 的深色分支），描边 / 内高光 / 投影这三层材质一直还是浅色值。这里先逐值钉住
// 四个纯函数，再用无头渲染证明它们真的接进了两条弹层。
//
// 相关既有测试：弹层洗光换层见 popoversheen_test.go，减少透明度实底见
// reducedtransparency_test.go。

// TestPlayerPopoverDarkTokenOverrides 逐值钉住烟熏玻璃时段对弹层材质的覆写：
// Tint（底色）、描边、inset 内高光、外投影四组；浅色时段各自回到浅色值，且
// 不能与深色值相同（否则等于没覆写）。
func TestPlayerPopoverDarkTokenOverrides(t *testing.T) {
	light := ThemeFor(PeriodAt(12))
	dark := ThemeFor(PeriodAt(0))
	if dark.Dark != true || light.Dark != false {
		t.Fatal("时段基态不对：midnight 应为深色、noon 应为浅色")
	}

	// 底色 Tint：浅色不着色，深色染 rgba(71,85,105,.64)。
	if got := playerPopoverTint(light); got != ui.Transparent {
		t.Errorf("浅色弹层 Tint = %v，应为 Transparent", got)
	}
	if got := playerPopoverTint(dark); got != ui.Hex("#475569").Alpha(0.64) {
		t.Errorf("深色弹层 Tint = %v，应为 #475569 .64", got)
	}

	// 描边：浅色沿用各弹层自己的玻璃边；深色统一覆写成白 .2。
	lightBorder := ui.Hex("#ffffff").Alpha(0.55)
	if got := playerPopoverBorder(light, lightBorder); got != lightBorder {
		t.Errorf("浅色弹层描边 = %v，应原样返回 %v", got, lightBorder)
	}
	if got := playerPopoverBorder(dark, lightBorder); got != ui.Hex("#ffffff").Alpha(0.20) {
		t.Errorf("深色弹层描边 = %v，应为白 .2", got)
	}

	// 内高光：浅色不另铺（框架玻璃的边缘折光近似 --glass-specular），
	// 深色补 `inset 0 1px 0 白 .26`。
	if got := playerPopoverInsets(light); got != nil {
		t.Errorf("浅色弹层不应另铺 inset，得到 %+v", got)
	}
	wantInsets := []shadowSpec{{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.26)}}
	gotInsets := playerPopoverInsets(dark)
	if len(gotInsets) != 1 || gotInsets[0] != wantInsets[0] {
		t.Errorf("深色弹层 inset = %+v，应为 %+v", gotInsets, wantInsets)
	}

	// 外投影：浅色 0 14 40 墨 .10（近似 --liquid-shadow-lifted），
	// 深色整条换 0 10 24 墨 .14（更紧、更深）。
	if got := playerPopoverLiftSpec(light); got != (shadowSpec{0, 14, 40, 0, shadowInk.Alpha(0.10)}) {
		t.Errorf("浅色弹层外投影 = %+v，应为 0 14 40 墨 .10", got)
	}
	if got := playerPopoverLiftSpec(dark); got != (shadowSpec{0, 10, 24, 0, shadowInk.Alpha(0.14)}) {
		t.Errorf("深色弹层外投影 = %+v，应为 0 10 24 墨 .14", got)
	}
}

// TestPlayerPopoverDarkBorderWired 无头反证：深色弹层左侧那条 1px 深色描环
// （白 .2）真的画进了玻璃之上。手法：把 playerPopoverMaterial 换成「不铺描环」
// 的桩，其余不变，同一处左缘条带的亮度应下降 —— 证明描环不是只停在纯函数里。
func TestPlayerPopoverDarkBorderWired(t *testing.T) {
	// 弹层左缘中点那一条（避开圆角）：描环落在这 1~2px 上。
	edgeLuma := func() int {
		_, tst := playerBarProbeAt(t, PeriodAt(0), func(a *App) { a.ShowVolume = true })
		anchor, ok := tst.Find("音量")
		if !ok {
			t.Fatal("播放栏里找不到音量键")
		}
		r := volumePopoverRect(anchor, volumePopoverW, volumePopoverH, volumePopoverEdgeGap)
		img := tst.Image()
		sum := 0
		for y := int(r.Y) + 6; y < int(r.Y+r.H)-6; y++ {
			for dx := 0; dx < 2; dx++ {
				c := rgbAt(img, int(r.X)+dx, y)
				sum += c[0] + c[1] + c[2]
			}
		}
		return sum
	}

	with := edgeLuma()

	// 换成「深色也不铺描环」的桩（保留 Tint），其余材质不变。
	orig := playerPopoverMaterial
	defer func() { playerPopoverMaterial = orig }()
	playerPopoverMaterial = func(t Theme, b ui.Element, border ui.Color) ui.Element {
		if t.ReducedTransparency {
			return b.Background(ui.Hex(rtPopoverSurface)).Border(1, border)
		}
		return b.Material(glassLayer(glass.Glass{Tint: playerPopoverTint(t)}))
	}
	without := edgeLuma()

	t.Logf("深色弹层左缘亮度 有描环=%d 无描环=%d", with, without)
	if with <= without {
		t.Errorf("深色弹层 1px 描环没有落到玻璃上：有=%d 无=%d", with, without)
	}
	if with-without < 200 {
		t.Errorf("深色弹层描环太弱，可能画丢了：差 %d", with-without)
	}
}

// TestPlayerPopoverReducedTransparencySolid 无头实测：减少透明度下两层弹层换成
// 不透明的 `background-color: #e6edf4`（原版块写在末尾、盖过烟熏玻璃那条），
// 所以弹层内部空白处的像素就是这支实底，且不再有玻璃层。
func TestPlayerPopoverReducedTransparencySolid(t *testing.T) {
	want := ui.Hex(rtPopoverSurface) // #e6edf4
	sample := func(theme Theme) ([3]int, int) {
		BackdropLayers()
		// 只渲染播放栏：Shell 里其它区域（首页卡片等）也有玻璃，会把 backdrop
		// 层数搅浑。音量弹层挂在播放栏上，只渲染播放栏就够。
		tst := playerBarOnlyProbe(t, func(a *App) {
			a.Theme = theme
			a.ShowVolume = true
		})
		anchor, ok := tst.Find("音量")
		if !ok {
			t.Fatal("播放栏里找不到音量键")
		}
		r := volumePopoverRect(anchor, volumePopoverW, volumePopoverH, volumePopoverEdgeGap)
		// 上缘中点内 4px（圆角之外、静音键/轨道之上）落在弹层实底上。
		c := rgbAt(tst.Image(), int(r.X+r.W/2), int(r.Y)+4)
		return c, BackdropLayers()
	}

	got, layers := sample(ThemeFor(PeriodAt(12)).withReduceTransparency(true))
	t.Logf("减少透明度弹层内部像素 = %v（期望 #e6edf4 = %v），玻璃层 %d", got, want, layers)
	if got[0] != int(want.R) || got[1] != int(want.G) || got[2] != int(want.B) {
		t.Errorf("减少透明度弹层内部 = %v，应为不透明 #e6edf4", got)
	}
	if layers != 0 {
		t.Errorf("减少透明度弹层仍建了 %d 层玻璃，应为 0", layers)
	}

	// 对照：无偏好时弹层是半透明玻璃，内部像素明显不等于实底 #e6edf4。
	base, _ := sample(ThemeFor(PeriodAt(12)))
	if base == got {
		t.Errorf("无偏好弹层内部 = %v，与减少透明度实底相同，偏好没生效", base)
	}
}
