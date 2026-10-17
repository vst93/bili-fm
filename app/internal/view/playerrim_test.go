package view

import (
	"image"
	"math"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（播放栏聚光描边 + 图标压印）的测试。原版每颗 `#player .player-button`
// 上有两层此前原生版没画：`::after` 的 1px 聚光描边（`--liquid-rim` 的白色
// conic-gradient）与 `svg` 的白色 drop-shadow 压印。压印用「下方 1px 的白色
// 副本」近似（mygo 没有 svg 滤镜）；描边到 v0.5.5 起改用真 conic 环
// （`FillPathConicGradient` + 外圈顺时针 / 内圈逆时针的环形路径），不再用
// 「上亮下淡的四条边带」。下面先钉住纯函数取值，再用无头渲染实测判据。

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

// TestPlayerRimPaintFor 钉住「四边线性近似」的取值（历史口径：上缘白 .85、
// 侧白 .40→.10、下缘白 .08，宽 1）。生产画法已改成真 conic 环（见
// TestPlayerRimConicStops / TestPlayerRimConicDirection），这个纯函数只留给
// 对照与回退（TestPlayerRimLinearApproximationPutLightOnTop 用它当反证）。
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
	// 上边最亮，往下渐淡（这是近似口径；conic 的真值见同文件的停靠点断言）。
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

// btnMidColumnLuma 取按钮矩形左/右边缘中点那一列像素的亮度（R+G+B）。
func btnMidColumnLuma(img *image.RGBA, r ui.Rect) (left, right int) {
	cy := int(r.Y + r.H/2)
	return lumaSum(rgbAt(img, int(r.X), cy)), lumaSum(rgbAt(img, int(r.X+r.W-1), cy))
}

func lumaSum(c [3]int) int { return c[0] + c[1] + c[2] }

// TestPlayerRimConicStops 钉住 --liquid-rim 的 conic 停靠点（原版 globals.css:987
// 的 `from 215deg at 30% -5%`：白 1 → .3@65° → .05@140° → .08@225° → .55@305°
// → 1@360°），并断言整圈 opacity 乘进每个点的 alpha。
func TestPlayerRimConicStops(t *testing.T) {
	if playerRimConicAngle != 215 {
		t.Errorf("conic 起始角 = %v，应为 215deg", playerRimConicAngle)
	}
	if playerRimConicCenter != (ui.Point{X: 0.30, Y: -0.05}) {
		t.Errorf("conic 圆心 = %+v，应为 30%% -5%%", playerRimConicCenter)
	}
	stops := playerRimConicStops(1.0)
	want := []struct {
		at float32
		a  float32
	}{{0, 1}, {65.0 / 360, 0.3}, {140.0 / 360, 0.05}, {225.0 / 360, 0.08}, {305.0 / 360, 0.55}, {1, 1}}
	if len(stops) != len(want) {
		t.Fatalf("停靠点 %d 个，应为 %d", len(stops), len(want))
	}
	for i, w := range want {
		if math.Abs(float64(stops[i].At-w.at)) > 1e-6 {
			t.Errorf("停靠点 %d 位置 %v，应为 %v", i, stops[i].At, w.at)
		}
		if got := float64(stops[i].Color.A) / 255; math.Abs(got-float64(w.a)) > 0.01 {
			t.Errorf("停靠点 %d alpha %.3f，应为 %.3f", i, got, w.a)
		}
	}
	half := playerRimConicStops(0.52)
	if half[0].Color.A >= stops[0].Color.A {
		t.Errorf("工具键（.52）描边应比播放键（1.0）淡：%v vs %v", half[0].Color, stops[0].Color)
	}
}

// TestPlayerRimConicDirection 无头实测 conic 环的定向光：同一颗音量键，把
// conic 环换成 no-op 再渲染一帧，量「边缘各点因描边而增加的亮度」。原版 conic
// 的最亮点被 `from 215deg at 30% -5%` 推到左上宽区（左中 ≈.66 / 下中 ≈.44 /
// 右中 ≈.20 / 上中 ≈.16），**不是**正上沿；旧的四边线性近似恰好把最亮点放在
// 上沿（反证见 TestPlayerRimLinearApproximationPutLightOnTop）。
func TestPlayerRimConicDirection(t *testing.T) {
	box := func(tst *ui.Tester, key string) (ui.Rect, *image.RGBA) {
		t.Helper()
		r, ok := tst.Find(key)
		if !ok {
			t.Fatalf("找不到 %q", key)
		}
		return r, tst.Image()
	}
	// 沿边缘取样：把矩形外圈 2px 内所有像素的亮度增量加起来（描边 1px 会落在
	// 边缘上并因抗锯齿摊到 2px）。
	edgeGain := func(a, b *image.RGBA, r ui.Rect, side string) int {
		gain := 0
		add := func(x, y int) {
			ia := a.PixOffset(x, y)
			ib := b.PixOffset(x, y)
			ga := int(a.Pix[ia]) + int(a.Pix[ia+1]) + int(a.Pix[ia+2])
			gb := int(b.Pix[ib]) + int(b.Pix[ib+1]) + int(b.Pix[ib+2])
			gain += ga - gb
		}
		x0, x1 := int(r.X)-1, int(r.X+r.W)
		y0, y1 := int(r.Y)-1, int(r.Y+r.H)
		switch side {
		case "left":
			for y := int(r.Y) + 2; y < int(r.Y+r.H)-2; y++ {
				for x := x0; x <= x0+1; x++ {
					add(x, y)
				}
			}
		case "right":
			for y := int(r.Y) + 2; y < int(r.Y+r.H)-2; y++ {
				for x := x1 - 1; x <= x1; x++ {
					add(x, y)
				}
			}
		case "top":
			for x := int(r.X) + 2; x < int(r.X+r.W)-2; x++ {
				for y := y0; y <= y0+1; y++ {
					add(x, y)
				}
			}
		case "bottom":
			for x := int(r.X) + 2; x < int(r.X+r.W)-2; x++ {
				for y := y1 - 1; y <= y1; y++ {
					add(x, y)
				}
			}
		}
		return gain
	}

	on, tstOn := playerBarProbe(t, nil)
	_ = on
	tstOn.Move(-1, -1)
	tstOn.Frame()
	r, imgOn := box(tstOn, "音量")

	orig := paintPlayerRimConicFn
	defer func() { paintPlayerRimConicFn = orig }()
	paintPlayerRimConicFn = func(p *ui.Painter, r ui.Rect, radius, opacity float32) {}
	_, tstOff := playerBarProbe(t, nil)
	tstOff.Move(-1, -1)
	tstOff.Frame()
	imgOff := tstOff.Image()

	left := edgeGain(imgOn, imgOff, r, "left")
	right := edgeGain(imgOn, imgOff, r, "right")
	top := edgeGain(imgOn, imgOff, r, "top")
	bottom := edgeGain(imgOn, imgOff, r, "bottom")
	t.Logf("音量键边缘亮度增量：左=%d 右=%d 上=%d 下=%d", left, right, top, bottom)
	if left <= 0 {
		t.Fatal("左侧边缘没有因描边变亮：conic 环整条没画出来？")
	}
	if left <= right {
		t.Errorf("conic 定向光应把最亮点放在左侧：左=%d 未大于右=%d", left, right)
	}
	if left <= top {
		t.Errorf("左侧是最亮区（原版左中 ≈.66 > 上中 ≈.16）：左=%d 未大于上=%d", left, top)
	}
	if left <= bottom {
		t.Errorf("左侧是最亮区（原版左中 ≈.66 > 下中 ≈.44）：左=%d 未大于下=%d", left, bottom)
	}
}

// TestPlayerRimRingPathGeometry 钉住环形路径的构造：Bounds 与外框一致，
// 且外框小于 2×环宽时退化为空路径（不画）。
func TestPlayerRimRingPathGeometry(t *testing.T) {
	r := ui.Rect{X: 10, Y: 20, W: 40, H: 32}
	path := playerRimRingPath(r, 8)
	b := path.Bounds()
	if math.Abs(float64(b.W-r.W)) > 0.5 || math.Abs(float64(b.H-r.H)) > 0.5 {
		t.Errorf("环路径 Bounds %v 与外框 %v 不一致", b, r)
	}
	if math.Abs(float64(b.X-r.X)) > 0.5 || math.Abs(float64(b.Y-r.Y)) > 0.5 {
		t.Errorf("环路径 Bounds 起点 %v 与外框 %v 不一致", b, r)
	}
	narrow := playerRimRingPath(ui.Rect{W: 1.5, H: 10}, 0)
	if w := narrow.Bounds().W; w != 0 {
		t.Errorf("过窄外框应返回空路径，得到宽 %v", w)
	}
}

// TestPlayerRimLinearApproximationPutLightOnTop 反证：旧的四边线性近似确实把
// 最亮点放在上沿（上中明显亮于左中）。拿旧画法与真 conic 画法在同一盒上比，
// 两者的最亮方位相反 —— 这条钉住本轮换成 conic 到底改掉了什么。
func TestPlayerRimLinearApproximationPutLightOnTop(t *testing.T) {
	const w, h = 34, 32
	luma := func(img *image.RGBA, x, y int) int {
		i := img.PixOffset(x, y)
		return int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
	}
	render := func(linear bool) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.RGB(20, 30, 40))
			ui.Box(c).Width(w).Height(h).Radius(8).
				Background(ui.RGB(100, 100, 100)).
				Draw(func(p *ui.Painter, r ui.Rect) {
					if linear {
						paint := playerRimPaintFor(1.0)
						p.Clip(r, 8, func() {
							p.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W, H: paint.Width}, paint.Top, 0)
							for _, x := range [2]float32{r.X, r.X + r.W - paint.Width} {
								p.FillGradient(ui.Rect{X: x, Y: r.Y, W: paint.Width, H: r.H}, ui.LinearGradient{
									From: paint.SideTop, To: paint.SideBottom, Angle: 180}, 0)
							}
							p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H - paint.Width, W: r.W, H: paint.Width}, paint.Bottom, 0)
						})
						return
					}
					paintPlayerRimConic(p, r, 8, 1)
				})
		}, w, h, 1)
	}
	lin, conic := render(true), render(false)
	linTop, linLeft := luma(lin, w/2, 0), luma(lin, 0, h/2)
	conicTop, conicLeft := luma(conic, w/2, 0), luma(conic, 0, h/2)
	t.Logf("线性近似 上中=%d 左中=%d；conic 上中=%d 左中=%d", linTop, linLeft, conicTop, conicLeft)
	if linTop <= linLeft {
		t.Errorf("线性近似应把最亮点放在上沿：上=%d 左=%d", linTop, linLeft)
	}
	if conicLeft <= conicTop {
		t.Errorf("conic 应把最亮点放在左中：左=%d 上=%d", conicLeft, conicTop)
	}
}

// TestPlayerRimDisabledFlattens 反证：把 playerRimOpacityFn 换成恒 0（等于
// 不画描边）后，音量键边缘总亮度应整体回落 —— 证明「边缘更亮」真的来自描边，
// 而不是按钮自带的顶部内高光或投影。
func TestPlayerRimDisabledFlattens(t *testing.T) {
	on, tstOn := playerBarProbe(t, nil)
	_ = on
	tstOn.Move(-1, -1)
	tstOn.Frame()
	r, ok := tstOn.Find("音量")
	if !ok {
		t.Fatal("找不到音量键")
	}
	boxOn := edgeLumaSum(tstOn.Image(), r)

	orig := playerRimOpacityFn
	defer func() { playerRimOpacityFn = orig }()
	playerRimOpacityFn = func(bool, bool) float32 { return 0 }
	_, tstOff := playerBarProbe(t, nil)
	tstOff.Move(-1, -1)
	tstOff.Frame()
	boxOff := edgeLumaSum(tstOff.Image(), r)

	t.Logf("音量键边缘总亮度：挂描边=%d 关描边=%d", boxOn, boxOff)
	if boxOff >= boxOn {
		t.Errorf("关掉描边后边缘总亮度没变小（挂 %d vs 关 %d），说明亮度不是描边造成的",
			boxOn, boxOff)
	}
}

// edgeLumaSum 累加按钮矩形**外圈**（上下左右各 1px 边）像素的亮度。
func edgeLumaSum(img *image.RGBA, r ui.Rect) int {
	sum := 0
	for x := int(r.X); x < int(r.X+r.W); x++ {
		sum += lumaSum(rgbAt(img, x, int(r.Y)))
		sum += lumaSum(rgbAt(img, x, int(r.Y+r.H-1)))
	}
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		sum += lumaSum(rgbAt(img, int(r.X), y))
		sum += lumaSum(rgbAt(img, int(r.X+r.W-1), y))
	}
	return sum
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

// TestPlayerChromaticFringe 钉住 --liquid-chromatic 的暖/冷色散边真的接进了
// paintPlayerRim（原版 globals.css:996 的
// `0 0.5px 0 rgba(255,250,240,.5)` / `0 -0.5px 0 rgba(200,220,240,.2)`）。
//
// 框架的 Fill 吸附设备像素、画不出 0.5px，这里用「1px 带 × 半不透明度」近似，
// 画在描边之外（暖边贴下缘外、冷边贴上缘外）。反证：把 paintPlayerChromaticFn
// 换成 no-op → 播放栏逐像素相同（差异必须 >0）。
//
// 2026-10-19 修正：**旧断言本身写错了** —— 它拿 `ui.RGBA(255,250,240,128)` 当
// 期望值，而那正是生产代码的错误写法（ui.RGBA 的 alpha 是 0..1 浮点，传 128 被
// `alphaByte` 钳成 255），于是「期望值 == 被感染的实现」自证成绿，把用户 ①/② 的
// 真根因放过去了。现在按 **CSS 语义**钉 alpha 字节（.5 → 128、.2 → 51），并额外
// 断言它不是 255（满不透明）。
func TestPlayerChromaticFringe(t *testing.T) {
	// 纯函数取值逐条对齐原版：albedo 逐字节、alpha 按 CSS 小数。
	warm, cool := playerChromaticWarm(), playerChromaticCool()
	if warm.R != 255 || warm.G != 250 || warm.B != 240 {
		t.Errorf("暖边色 = %v，应为 rgb(255,250,240)", warm)
	}
	if cool.R != 200 || cool.G != 220 || cool.B != 240 {
		t.Errorf("冷边色 = %v，应为 rgb(200,220,240)", cool)
	}
	if warm.A != 128 {
		t.Errorf("暖边 alpha = %d，应为 128（rgba(255,250,240,.5)）", warm.A)
	}
	if cool.A != 51 {
		t.Errorf("冷边 alpha = %d，应为 51（rgba(200,220,240,.2)）", cool.A)
	}
	// 反证：满不透明（字节值被钳成 255）就是用户报的「实色线条」。
	if warm.A == 255 || cool.A == 255 {
		t.Errorf("色散边 alpha 被钳成 255（满不透明）：这正是「按钮上下边实色线条」的根因")
	}
	// 源码链：ui.RGBA 的 alpha 是 0..1，用 Hex(...).Alpha(小数) 才不会踩坑。
	flat := flattenNoComments(t, "playerbar.go")
	if strings.Contains(flat, "ui.RGBA(255,250,240,128)") || strings.Contains(flat, "ui.RGBA(200,220,240,51)") {
		t.Error("色散边仍在用 ui.RGBA 传字节 alpha（会被 alphaByte 钳成 255）")
	}
	if !strings.Contains(flat, "ui.Hex(\"#fffaf0\").Alpha(0.5)") ||
		!strings.Contains(flat, "ui.Hex(\"#c8dcf0\").Alpha(0.2)") {
		t.Error("色散边没有按 CSS 的 rgba(...，.5 / .2) 取小数 alpha")
	}
	// 源码链：paintPlayerRim 在 clip 之前调用色散边（画在描边之外）。
	if !strings.Contains(flat, "paintPlayerChromaticFn(p,r,radius)") {
		t.Error("paintPlayerRim 没有调用色散边绘制")
	}

	orig := paintPlayerChromaticFn
	defer func() { paintPlayerChromaticFn = orig }()

	render := func(stub bool) *ui.Tester {
		paintPlayerChromaticFn = orig
		if stub {
			paintPlayerChromaticFn = func(p *ui.Painter, r ui.Rect, radius float32) {}
		}
		_, tst := playerBarProbe(t, nil)
		return tst
	}
	on, off := render(false), render(true)
	ia, ib := on.Image(), off.Image()
	n := 0
	for i := range ia.Pix {
		if ia.Pix[i] != ib.Pix[i] {
			n++
		}
	}
	t.Logf("色散边差异通道 %d", n)
	if n == 0 {
		t.Fatal("色散边整条没画出来（与关掉时逐像素相同）")
	}
}

// TestPlayerChromaticFringeFollowsCorners 钉住色散边（--liquid-chromatic）不再是
// 两条横贯整宽、切过圆角的**直角色线**（用户 2026-10-18 报告「很多按钮的上下边
// 有带颜色的线条，超出了圆角的范围……整体检查」；底栏播放器与抽屉按钮都中招）。
//
// 原版这两条是 box-shadow：形状是元素自己的圆角矩形平移半像素，只从边缘外露出
// 半条，**必然跟着圆角弯曲**。此前原生版直接画两条整宽 1px 直线（不裁圆角），
// 于是圆角处露出一段超出圆角的直角色线。修法是把这个「外扩 1px 的圆角矩形」
// 当作裁剪区。
//
// 判法（无头像素实测）：在按钮上缘之外那一行，比较「色散边开 / 关」的差异像素。
//   - **直边段**（避开圆角）必须有差异（色散边还在）；
//   - **圆角段**（x < 半径 + 外扩 1px）差异必须远小于直边段 —— 退回直角绘制时
//     圆角段与直边段一样满，当场红。
//
// 反证：把 paintPlayerChromatic 里的 Clip 去掉（放回两条整宽直线）→ 圆角段
// 差异从个位数升到 ~2×(半径+1) 满行，测试红。
func TestPlayerChromaticFringeFollowsCorners(t *testing.T) {
	orig := paintPlayerChromaticFn
	defer func() { paintPlayerChromaticFn = orig }()

	const side, radius = 32, 8
	render := func(stub bool) *image.RGBA {
		paintPlayerChromaticFn = orig
		if stub {
			paintPlayerChromaticFn = func(p *ui.Painter, r ui.Rect, radius float32) {}
		}
		tst := ui.NewTester(func(c *ui.Context) {
			ui.Box(c).Absolute().Left(100).Top(100).Width(side).Height(side).Radius(radius).
				Background(ui.Hex("#334155")).
				Draw(func(p *ui.Painter, r ui.Rect) { paintPlayerRim(p, r, radius, 1) })
		}, 260, 260)
		tst.Frame()
		return tst.Image()
	}
	on, off := render(false), render(true)

	// 冷边贴在上缘外 1px（y = 100-1 = 99）；暖边贴在下缘外 1px（y = 132）。
	diffAt := func(y, x0, x1 int) int {
		n := 0
		for x := x0; x <= x1; x++ {
			i := on.PixOffset(x, y)
			j := off.PixOffset(x, y)
			if on.Pix[i] != off.Pix[j] || on.Pix[i+1] != off.Pix[j+1] || on.Pix[i+2] != off.Pix[j+2] {
				n++
			}
		}
		return n
	}
	// 圆角段：x ∈ [盒左-2, 盒左+半径-1]；直边段取中段（半径 + 4 之后）。
	const left = 100
	cornerCool := diffAt(99, left-2, left+radius-1)
	straightCool := diffAt(99, left+radius+2, left+side-radius-3)
	cornerWarm := diffAt(132, left-2, left+radius-1)
	straightWarm := diffAt(132, left+radius+2, left+side-radius-3)
	t.Logf("冷边圆角段=%d 直边段=%d；暖边圆角段=%d 直边段=%d",
		cornerCool, straightCool, cornerWarm, straightWarm)

	if straightCool < 8 {
		t.Errorf("冷色散边直边段没画出来（%d < 8）", straightCool)
	}
	if straightWarm < 8 {
		t.Errorf("暖色散边直边段没画出来（%d < 8）", straightWarm)
	}
	// 圆角处只允许极少量（连续圆角把外扩弧与直角的差摊开的亚像素残留），
	// 退回直角绘制会是满行（圆角段宽 = 半径 + 外扩 ≈ 10）。
	const maxCorner = 6
	if cornerCool > maxCorner {
		t.Errorf("冷色散边在圆角区仍是直线（圆角段 %d > %d）：未按圆角裁切",
			cornerCool, maxCorner)
	}
	if cornerWarm > maxCorner {
		t.Errorf("暖色散边在圆角区仍是直线（圆角段 %d > %d）：未按圆角裁切",
			cornerWarm, maxCorner)
	}
}

// TestPlayerChromaticFringeDrawerTop 是抽屉面板的同类断言（用户点名「抽屉打开时
// 的很多按钮」+「抽屉弹窗顶部的圆角上方有直角虚线细线」）。这里直接渲染打开
// 抽屉的主窗，在面板顶边外那一行（y = 面板顶 − 1）比较「开 / 关色散边」的差异
// 像素，断言它不越过面板左上圆角：圆角段应远小于直边段。
//
// 判法用**差分**（开 vs 关 paintPlayerChromaticFn）而不是数某个精确色值：
// 2026-10-19 发现旧版把期望色写死成 `rgb(200,220,240)`，那正是**被污染的
// 满不透明**值（见 playerChromaticWarm/Cool 的注释），于是测试一边跟着 bug 走、
// 一边把用户 ①/② 放过去。差分与色值无关，改多少 alpha 都成立。
//
// 实测（800×600、noon、feed 抽屉，面板顶 y=54，色散边行 y=53）：圆角段
// （x ≤ 10）差异 0，直边段（x ∈ [24,400]）差异 377。**反证**：把色散边换成
// 「两条不裁圆角的整宽直线」→ 圆角段立刻变成 11（x=0..10 逐像素可见），红。
func TestPlayerChromaticFringeDrawerTop(t *testing.T) {
	orig := paintPlayerChromaticFn
	defer func() { paintPlayerChromaticFn = orig }()

	// panelTopAt 从顶往下在 x=400 找面板上沿（避开搜栏与圆角）。
	panelTopAt := func(img *image.RGBA) int {
		for y := 0; y < 200; y++ {
			r, g, b, _ := img.At(400, y).RGBA()
			if r>>8 >= 252 && g>>8 >= 252 && b>>8 >= 252 {
				return y
			}
		}
		return -1
	}
	render := func(mode string) (*image.RGBA, int) {
		switch mode {
		case "on":
			paintPlayerChromaticFn = orig
		case "off":
			paintPlayerChromaticFn = func(p *ui.Painter, r ui.Rect, radius float32) {}
		case "noclip":
			// 反证用：退回两条不裁圆角的整宽直线。
			paintPlayerChromaticFn = func(p *ui.Painter, r ui.Rect, radius float32) {
				p.Fill(ui.Rect{X: r.X, Y: r.Y + r.H, W: r.W, H: 1}, playerChromaticWarm(), 0)
				p.Fill(ui.Rect{X: r.X, Y: r.Y - 1, W: r.W, H: 1}, playerChromaticCool(), 0)
			}
		}
		_, tst := drawerProbe(t, "feed")
		settleTester(tst)
		img := tst.Image()
		return img, panelTopAt(img)
	}

	on, top := render("on")
	off, _ := render("off")
	if top < 1 || top > 120 {
		t.Fatalf("抽屉面板顶边定位失败：top=%d", top)
	}
	fringeRow := top - 1
	// segment 数 [x0,x1] 上「开 / 关色散边」的差异像素。
	segment := func(img, ref *image.RGBA, x0, x1 int) int {
		n := 0
		for x := x0; x <= x1; x++ {
			i := img.PixOffset(x, fringeRow)
			j := ref.PixOffset(x, fringeRow)
			if img.Pix[i] != ref.Pix[j] || img.Pix[i+1] != ref.Pix[j+1] || img.Pix[i+2] != ref.Pix[j+2] {
				n++
			}
		}
		return n
	}
	corner := segment(on, off, 0, 10)
	straight := segment(on, off, 24, 400)
	t.Logf("抽屉顶边 y=%d 色散边差异：圆角段(x0..10)=%d 直边段(24..400)=%d", fringeRow, corner, straight)
	if straight < 100 {
		t.Errorf("抽屉顶边直边段没有色散边（%d）", straight)
	}
	if corner > 3 {
		t.Errorf("抽屉顶部圆角上方仍是直角细线（圆角段 %d > 3）", corner)
	}
	// 反证：不裁圆角的整宽直线在这里必须红（圆角段满行）。
	noclip, _ := render("noclip")
	if bad := segment(noclip, off, 0, 10); bad <= 3 {
		t.Errorf("反证失败：不裁圆角时圆角段只有 %d（应 > 3）—— 判法分不开两种画法", bad)
	} else {
		t.Logf("反证：不裁圆角 → 圆角段 %d（> 3，判法有效）", bad)
	}
}
