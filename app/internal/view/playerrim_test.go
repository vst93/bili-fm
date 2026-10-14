package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏聚光描边 + 图标压印）的测试。原版每颗 `#player .player-button`
// 上有两层此前原生版没画：`::after` 的 1px 聚光描边（--liquid-rim 的白色
// conic-gradient，上边最亮往下渐淡）与 `svg` 的白色 drop-shadow 压印。mygo
// 画不了 conic-gradient / svg 滤镜，这里用「上亮下淡的四条边带」与「下方 1px
// 的白色副本」近似。下面先钉住纯函数取值，再用无头渲染实测两条判据。

// TestPlayerRimOpacity 钉住聚光描边整圈的不透明度（原版三条规则）：
// 工具键 `::after { opacity: .52 }`、播放键/静音键不吃覆写（1.0）、
// 深色（烟熏玻璃）时段统一压到 .3。
func TestPlayerRimOpacity(t *testing.T) {
	if got := playerRimOpacity(false, true); got != 0.52 {
		t.Errorf("工具键描边 opacity = %v，应为 .52", got)
	}
	if got := playerRimOpacity(false, false); got != 1.0 {
		t.Errorf("播放/静音键描边 opacity = %v，应为 1.0", got)
	}
	if got := playerRimOpacity(true, true); got != 0.30 {
		t.Errorf("深色时段工具键描边 opacity = %v，应为 .30", got)
	}
	if got := playerRimOpacity(true, false); got != 0.30 {
		t.Errorf("深色时段播放键描边 opacity = %v，应为 .30", got)
	}
	// 深色规则（1,2,2）盖过 .52 规则（1,2,1）：深色下工具键不是 .52。
	if playerRimOpacity(true, true) == 0.52 {
		t.Error("深色时段工具键描边仍是 .52，应被 .3 覆写")
	}
}

// TestPlayerRimPaintFor 钉住描边四边色：上缘白 .85、侧白 .40→.10（竖直渐变）、
// 下缘白 .08，再乘整圈 opacity；宽度 1（原版 ::after 的 padding: 1px）。
func TestPlayerRimPaintFor(t *testing.T) {
	p := playerRimPaintFor(1.0)
	if p.Top != ui.Hex("#ffffff").Alpha(0.85) {
		t.Errorf("描边上缘 = %v，应为白 .85", p.Top)
	}
	if p.SideTop != ui.Hex("#ffffff").Alpha(0.40) {
		t.Errorf("描边侧上 = %v，应为白 .40", p.SideTop)
	}
	if p.SideBottom != ui.Hex("#ffffff").Alpha(0.10) {
		t.Errorf("描边侧下 = %v，应为白 .10", p.SideBottom)
	}
	if p.Bottom != ui.Hex("#ffffff").Alpha(0.08) {
		t.Errorf("描边下缘 = %v，应为白 .08", p.Bottom)
	}
	if p.Width != 1 {
		t.Errorf("描边宽度 = %v，应为 1", p.Width)
	}
	// 上边最亮，往下渐淡。
	if !(p.Top.A > p.SideTop.A && p.SideTop.A > p.SideBottom.A && p.SideBottom.A >= p.Bottom.A) {
		t.Errorf("描边应上亮下淡：上=%v 侧上=%v 侧下=%v 下=%v", p.Top, p.SideTop, p.SideBottom, p.Bottom)
	}
	half := playerRimPaintFor(0.52)
	if half.Top.A >= p.Top.A || half.Bottom.A >= p.Bottom.A {
		t.Errorf("工具键（.52）描边应比播放键（1.0）淡：%v vs %v", half.Top, p.Top)
	}
}

// btnEdgeLuma 取按钮矩形上/下边中列那一行像素的亮度（R+G+B）。用直边中点
// （而不是圆按钮的极点）避开圆形端点的抗锯齿。
func btnEdgeLuma(img *image.RGBA, r ui.Rect) (top, bottom int) {
	cx := int(r.X + r.W/2)
	return lumaSum(rgbAt(img, cx, int(r.Y))), lumaSum(rgbAt(img, cx, int(r.Y+r.H-1)))
}

func lumaSum(c [3]int) int { return c[0] + c[1] + c[2] }

// TestPlayerRimBrightensTopEdge 无头实测：音量键（工具键、半径 8 的直边）
// 上边的亮度应高于下边 —— 聚光描边「光聚在上边」。
func TestPlayerRimBrightensTopEdge(t *testing.T) {
	_, tst := playerBarProbe(t, nil)
	tst.Move(-1, -1)
	tst.Frame()
	r, ok := tst.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	top, bottom := btnEdgeLuma(tst.Image(), r)
	t.Logf("音量键上边亮度=%d 下边亮度=%d", top, bottom)
	if top <= bottom {
		t.Errorf("上边应比下边亮（聚光在上）：上=%d 下=%d", top, bottom)
	}
}

// TestPlayerRimDisabledFlattens 反证：把 playerRimOpacityFn 换成恒 0（等于
// 不画描边），音量键上/下边的亮度差应比挂描边时小 —— 证明「上边更亮」真的
// 来自描边，而不是按钮自带的顶部内高光或投影。
func TestPlayerRimDisabledFlattens(t *testing.T) {
	on, tstOn := playerBarProbe(t, nil)
	_ = on
	tstOn.Move(-1, -1)
	tstOn.Frame()
	r, ok := tstOn.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	topOn, bottomOn := btnEdgeLuma(tstOn.Image(), r)

	orig := playerRimOpacityFn
	defer func() { playerRimOpacityFn = orig }()
	playerRimOpacityFn = func(bool, bool) float32 { return 0 }
	_, tstOff := playerBarProbe(t, nil)
	tstOff.Move(-1, -1)
	tstOff.Frame()
	topOff, bottomOff := btnEdgeLuma(tstOff.Image(), r)

	t.Logf("上-下亮度差：挂描边=%d 关描边=%d", topOn-bottomOn, topOff-bottomOff)
	if topOff-bottomOff >= topOn-bottomOn {
		t.Errorf("关掉描边后上/下亮度差没变小（挂 %d vs 关 %d），说明差异不是描边造成的",
			topOn-bottomOn, topOff-bottomOff)
	}
}

// TestPlayerRimTopEdgeRises 无头实测（直接因果）：同一颗音量键，挂描边时
// 上边亮度必须高于关掉描边时 —— 比「上 vs 下」更直接地钉住描边画在了上边。
func TestPlayerRimTopEdgeRises(t *testing.T) {
	on, tstOn := playerBarProbe(t, nil)
	_ = on
	tstOn.Move(-1, -1)
	tstOn.Frame()
	r, ok := tstOn.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	topOn, _ := btnEdgeLuma(tstOn.Image(), r)

	orig := playerRimOpacityFn
	defer func() { playerRimOpacityFn = orig }()
	playerRimOpacityFn = func(bool, bool) float32 { return 0 }
	_, tstOff := playerBarProbe(t, nil)
	tstOff.Move(-1, -1)
	tstOff.Frame()
	topOff, _ := btnEdgeLuma(tstOff.Image(), r)

	t.Logf("音量键上边亮度：挂描边=%d 关描边=%d", topOn, topOff)
	if topOn <= topOff {
		t.Errorf("挂描边后上边没有变亮：挂=%d 关=%d", topOn, topOff)
	}
}

// TestPlayerRimOpacityWired 无头反证：视图给每颗键传对了「是不是工具键」——
// 播放键与静音键 false（1.0）、下一集/音量/倍速/均衡/恰饭 true（.52）；
// 烟熏玻璃时段 dark=true（统一 .3）。
func TestPlayerRimOpacityWired(t *testing.T) {
	orig := playerRimOpacityFn
	defer func() { playerRimOpacityFn = orig }()
	type call struct{ dark, tool bool }
	var calls []call
	playerRimOpacityFn = func(dark, tool bool) float32 {
		calls = append(calls, call{dark, tool})
		return orig(dark, tool)
	}
	_, tst := playerBarProbe(t, nil)
	_ = tst
	var tool, plain int
	for _, c := range calls {
		if c.dark {
			t.Errorf("亮色时段传了 dark=true")
		}
		if c.tool {
			tool++
		} else {
			plain++
		}
	}
	// 视图每帧可能构建多次，只钉两类键的比例（播放 1 : 工具 5）与两类都非空。
	if plain == 0 || tool == 0 {
		t.Errorf("两类键都应被调用：plain=%d tool=%d", plain, tool)
	}
	if plain*5 != tool {
		t.Errorf("播放键 : 工具键应 1:5，实测 %d:%d", plain, tool)
	}

	// 深色时段：所有键都收到 dark=true。
	calls = nil
	_, dark := playerBarProbe(t, func(a *App) { a.Theme = ThemeFor(PeriodAt(0)) })
	_ = dark
	if len(calls) == 0 {
		t.Fatal("深色时段没调用描边")
	}
	for _, c := range calls {
		if !c.dark {
			t.Errorf("深色时段传了 dark=false")
		}
	}
}

// TestPlayerIconShadowWired 无头反证：视图真的给播放栏图标挂了白色压印，
// 且播放键用的是 .6、其余键用 .7（原版 .player-play-button svg 与
// .player-button svg 两条 drop-shadow 的颜色不同）。
func TestPlayerIconShadowWired(t *testing.T) {
	orig := playerIconShadowFn
	defer func() { playerIconShadowFn = orig }()
	type call struct {
		svg     *ui.SVG
		opacity float32
	}
	var calls []call
	playerIconShadowFn = func(b ui.Element, svg *ui.SVG, dx, dy, opacity float32) ui.Element {
		calls = append(calls, call{svg, opacity})
		return orig(b, svg, dx, dy, opacity)
	}
	_, tst := playerBarProbe(t, func(a *App) {
		a.Queue = []Track{{Bvid: "BV1", Cid: 1}, {Bvid: "BV2", Cid: 2}}
		a.Track = &a.Queue[0]
	})
	_ = tst
	if len(calls) == 0 {
		t.Fatal("播放栏图标没挂压印（playerIconShadowFn 没被调用）")
	}
	var play, other int
	for _, c := range calls {
		switch {
		case c.svg == iconPlayOne || c.svg == iconPause:
			play++
			if c.opacity != playerPlayIconShadowInk {
				t.Errorf("播放键图标压印 opacity = %v，应为 %v", c.opacity, playerPlayIconShadowInk)
			}
		default:
			other++
			if c.opacity != playerIconShadowInk {
				t.Errorf("工具键图标压印 opacity = %v，应为 %v", c.opacity, playerIconShadowInk)
			}
		}
	}
	if play == 0 {
		t.Error("播放键图标没挂压印")
	}
	if other == 0 {
		t.Error("工具键图标没挂压印")
	}
}

// TestPlayerIconShadowAddsWhite 无头实测：音量键图标区域的总亮度在挂了压印
// 后应变高（白色副本从图标下缘露出）。反证：把 playerIconShadowFn 换成不画
// 的桩，总亮度应回落。
func TestPlayerIconShadowAddsWhite(t *testing.T) {
	boxLuma := func(img *image.RGBA, r ui.Rect) int {
		sum := 0
		for y := int(r.Y); y < int(r.Y+r.H); y++ {
			for x := int(r.X); x < int(r.X+r.W); x++ {
				sum += lumaSum(rgbAt(img, x, y))
			}
		}
		return sum
	}

	on, tstOn := playerBarProbe(t, nil)
	_ = on
	tstOn.Move(-1, -1)
	tstOn.Frame()
	r, ok := tstOn.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	withShadow := boxLuma(tstOn.Image(), r)

	orig := playerIconShadowFn
	defer func() { playerIconShadowFn = orig }()
	playerIconShadowFn = func(b ui.Element, svg *ui.SVG, dx, dy, opacity float32) ui.Element {
		return b
	}
	_, tstOff := playerBarProbe(t, nil)
	tstOff.Move(-1, -1)
	tstOff.Frame()
	withoutShadow := boxLuma(tstOff.Image(), r)

	t.Logf("音量键图标区总亮度：有压印=%d 无压印=%d", withShadow, withoutShadow)
	if withShadow <= withoutShadow {
		t.Errorf("白色压印没有抬高图标区亮度：有=%d 无=%d", withShadow, withoutShadow)
	}
}
