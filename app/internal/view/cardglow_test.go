package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// cardGlowProbe 把一张卡放在固定位置的纯色底上，供逐像素验证光晕。
// 背景取中灰：卡片玻璃叠上去仍是浅灰，白色光晕叠上来能看出亮度差。
//
// 测量期间把占位图摘掉：本轮起「封面未就绪」的卡片会铺 /cover.png 占位图
// （原版 RetryImg 行为），它盖住整个封面盒，会把光晕区域的读数压成一片占位
// 色 —— 光晕本身仍照常画。
func cardGlowProbe(t *testing.T) (*ui.Tester, ui.Rect) {
	t.Helper()
	old := placeholderCover
	placeholderCover = nil
	t.Cleanup(func() { placeholderCover = old })
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	spec := cardSpec{
		Key:    "glow-card",
		Title:  "标题",
		Fields: [cardFieldsMax]cardField{{Kind: fieldAuthor, Text: "UP主"}}, NFields: 1,
	}
	tst := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.RGB(90, 90, 90))
		ui.Box(c).Absolute().Left(20).Top(20).Size(220, 170).Children(func() {
			a.renderCard(c, spec)
		})
	}, 300, 260)
	tst.Frame()
	return tst, ui.Rect{X: 20, Y: 20, W: 220, H: 170}
}

// rgbAt 读一个像素的三个通道。
func rgbAt(img *image.RGBA, x, y int) [3]int {
	i := img.PixOffset(x, y)
	return [3]int{int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])}
}

// TestCardGlowLayerAlpha 钉住同心圆近似的 alpha 补偿公式：普通叠加下把累计
// 不透明度从 prev 补到 want 需要 (want-prev)/(1-prev)；want 不高于 prev 时
// 补 0（渐变越往内越亮，外层永远不需要再压暗）。
func TestCardGlowLayerAlpha(t *testing.T) {
	if got := cardGlowLayerAlpha(0, 0.6); math.Abs(float64(got-0.6)) > 1e-6 {
		t.Errorf("首层 = %v，应为 .6", got)
	}
	// 已铺 .6，补到 .8：inc = (.8-.6)/(1-.6) = .5 → 累计 .6 + .5*.4 = .8。
	if got := cardGlowLayerAlpha(0.6, 0.8); math.Abs(float64(got-0.5)) > 1e-6 {
		t.Errorf("补偿层 = %v，应为 .5", got)
	}
	if got := cardGlowLayerAlpha(0.8, 0.6); got != 0 {
		t.Errorf("目标低于已铺值时应补 0，得到 %v", got)
	}
	if got := cardGlowLayerAlpha(1, 1); got != 0 {
		t.Errorf("已完全不透明时应补 0，得到 %v", got)
	}
}

// TestCardGlowRadialFalloff 是无头渲染实测：光晕从圆心向外单调变淡，到
// 0.7R（140px）处完全消失（原版 transparent 70%），中途有天空蓝的色相。
func TestCardGlowRadialFalloff(t *testing.T) {
	const bg = 120
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.RGB(bg, bg, bg))
		ui.Box(c).Absolute().Left(0).Top(0).Size(360, 280).
			Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, 180, 140) })
	}, 400, 300, 1)

	center := grayAt(img, 180, 140)
	r40 := grayAt(img, 180, 100)
	r90 := grayAt(img, 180, 50)
	r140 := grayAt(img, 180, 0)
	r180 := grayAt(img, 0, 140)
	t.Logf("圆心=%d r40=%d r90=%d r140=%d r180=%d（底 %d）",
		center, r40, r90, r140, r180, bg)
	if center <= r40 || r40 <= r90 {
		t.Fatalf("光晕没有向外单调变淡：圆心=%d r40=%d r90=%d", center, r40, r90)
	}
	if center-bg < 25 {
		t.Fatalf("圆心光晕太弱：%d（底 %d）", center, bg)
	}
	for _, v := range []int{r140, r180} {
		if v > bg+3 {
			t.Fatalf("0.7R 之外（>=140px）不该再有光晕：%d（底 %d）", v, bg)
		}
	}
	// r90 落在 40%~70% 段，色相是天空蓝（B > R）。
	sky := rgbAt(img, 180, 50)
	if sky[2] <= sky[0]+8 {
		t.Fatalf("中段没有天空蓝色相：rgb=%v", sky)
	}
}

// TestCardGlowClipsToCard 钉住光晕裁在卡片圆角内（原版是 overflow:hidden 卡
// 片里的 ::before）：卡片很小、圆心在卡片中心时，紧贴卡片外的像素不应被提亮。
func TestCardGlowClipsToCard(t *testing.T) {
	const bg = 120
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.RGB(bg, bg, bg))
		ui.Box(c).Absolute().Left(120).Top(90).Size(120, 90).
			Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, 60, 45) })
	}, 400, 300, 1)

	inside := grayAt(img, 140, 135)  // 卡内、圆心左 40px
	outLeft := grayAt(img, 110, 135) // 卡片左缘外 10px
	outTop := grayAt(img, 180, 80)   // 卡片上缘外 10px
	t.Logf("卡内=%d 左外=%d 上外=%d（底 %d）", inside, outLeft, outTop, bg)
	if inside <= bg+8 {
		t.Fatalf("卡片内应被光晕提亮：%d（底 %d）", inside, bg)
	}
	if outLeft > bg+3 || outTop > bg+3 {
		t.Fatalf("光晕溢出了卡片：左外=%d 上外=%d（底 %d）", outLeft, outTop, bg)
	}
}

// TestCardGlowFollowsPointer 是整卡实测：光晕跟着指针走——两个都在卡片封面
// 里的点 A/B，指针指向哪个，哪个就更亮（悬停的边框/上浮在两次之间不变，差异
// 只能来自光晕）。
func TestCardGlowFollowsPointer(t *testing.T) {
	tst, _ := cardGlowProbe(t)
	aPt := [2]int{60, 60}
	bPt := [2]int{190, 90}

	tst.Move(float32(aPt[0]), float32(aPt[1]))
	tst.Advance(200 * time.Millisecond) // 光晕 120ms 渐显（原版 opacity .12s）
	imgA := tst.Image()
	aa := grayAt(imgA, aPt[0], aPt[1])
	ba := grayAt(imgA, bPt[0], bPt[1])

	tst.Move(float32(bPt[0]), float32(bPt[1]))
	tst.Advance(200 * time.Millisecond)
	imgB := tst.Image()
	ab := grayAt(imgB, aPt[0], aPt[1])
	bb := grayAt(imgB, bPt[0], bPt[1])

	t.Logf("指针在 A：A=%d B=%d；指针在 B：A=%d B=%d", aa, ba, ab, bb)
	if aa <= ba {
		t.Errorf("指针在 A 时 A 应比 B 亮：A=%d B=%d", aa, ba)
	}
	if bb <= ab {
		t.Errorf("指针在 B 时 B 应比 A 亮：A=%d B=%d", ab, bb)
	}
	if aa-ab < 8 || bb-ba < 8 {
		t.Errorf("光晕随指针移动的亮度差太小：A %d→%d, B %d→%d", aa, ab, ba, bb)
	}

	// 指针移出卡片：光晕渐隐（原版 `transition: opacity .12s ease`，渐隐本身
	// 由 TestCardGlowFadesOut 逐帧钉）。这里量的是渐隐结束后两点只剩卡片底色 ——
	// 渐隐期间圆心停在离开处，A 点距离开点 133px、几乎照不到光，只能等它走完。
	tst.Move(280, 240)
	tst.Advance(200 * time.Millisecond)
	out := tst.Image()
	oa := grayAt(out, aPt[0], aPt[1])
	ob := grayAt(out, bPt[0], bPt[1])
	t.Logf("指针移出 200ms：A=%d B=%d", oa, ob)
	if diff := oa - ob; diff > 2 || diff < -2 {
		t.Errorf("指针移出后两点应只剩底色（相等）：A=%d B=%d", oa, ob)
	}
}

// TestCardGlowFadesOut 钉住原版 `.is-pointer-hovered::before` 的
// `transition: opacity .12s ease`：指针离开卡片后光晕不是当帧消失，而是在
// 120ms 里降到 0；圆心停在离开时的位置（原版 activeCard 只在 pointermove 里
// 更新 --hover-x/y，离开后不再变）。
//
// 反证：把 cardGlowFadeAnim.Duration 改成 0（瞬时切换，改动前的行为）→
// 「移出当帧应仍有光」与「中点应介于两者之间」两条报红。
func TestCardGlowFadesOut(t *testing.T) {
	oldRadial := radialPaint
	t.Cleanup(func() { radialPaint = oldRadial })

	// 两遍同一时间轴采样：一遍真实光晕，一遍把光晕那层换成空实现；两者相减
	// 只剩光晕自身的贡献。必须这么量 —— 卡片底色/描边/投影自本轮起也走
	// 160ms 过渡（原版 `transition: background-color/border-color/box-shadow
	// .16s`），直接读绝对亮度会把「底色从 hover 渐隐回常态」误当成光晕。
	run := func(withGlow bool) [4]float32 {
		if withGlow {
			radialPaint = oldRadial
		} else {
			radialPaint = func(p *ui.Painter, bmp *ui.Bitmap, cx, cy, radius, opacity float32) {}
		}
		tst, box := cardGlowProbe(t)
		cx, cy := int(box.X+box.W/2), int(box.Y+box.H/2)
		var out [4]float32
		tst.Move(float32(cx), float32(cy))
		tst.Advance(200 * time.Millisecond)
		out[0] = float32(grayAt(tst.Image(), cx, cy))
		tst.Move(box.X+box.W+60, box.Y+box.H+60) // 移出卡片
		out[1] = float32(grayAt(tst.Image(), cx, cy))
		tst.Advance(60 * time.Millisecond)
		out[2] = float32(grayAt(tst.Image(), cx, cy))
		tst.Advance(140 * time.Millisecond)
		out[3] = float32(grayAt(tst.Image(), cx, cy))
		return out
	}
	with, without := run(true), run(false)
	glow := func(i int) float32 { return with[i] - without[i] }
	hover, first, mid, end := glow(0), glow(1), glow(2), glow(3)
	t.Logf("光晕贡献 悬停=%.1f 移出当帧=%.1f 60ms=%.1f 200ms=%.1f（绝对亮度 %.0f→%.0f→%.0f→%.0f）",
		hover, first, mid, end, with[0], with[1], with[2], with[3])

	if hover < 5 {
		t.Fatalf("悬停时光晕贡献只有 %.1f —— 采样点或光晕没画", hover)
	}
	if hover-first > 2 {
		t.Fatalf("移出当帧光晕就没了（%.1f → %.1f）—— 没有渐隐", hover, first)
	}
	if !(first > mid && mid > end) {
		t.Errorf("光晕没有单调渐隐：当帧 %.1f → 60ms %.1f → 200ms %.1f", first, mid, end)
	}
	if first-mid < 1 || mid-end < 1 {
		t.Errorf("渐隐太快/太慢，60ms 中点看不出过渡：%.1f → %.1f → %.1f", first, mid, end)
	}
	if end > 1 {
		t.Errorf("200ms 后光晕应已完全消失，仍剩 %.1f", end)
	}
}

// TestCardGlowReduceMotion 钉住原版 pointerEffectsEnabled 里的
// `(prefers-reduced-motion: no-preference)`：桌面要求减少动态效果时，
// App.tsx 直接 clearAll()，那层径向光晕整层不画（悬停的底/边/上浮是静态
// 状态，照旧）。用 radialPaint 计数做判据，不去猜某个像素色。
//
// 反证：把 cardGlowLayer 里的 `!reduceMotion(c)` 去掉 → 第二条报红。
func TestCardGlowReduceMotion(t *testing.T) {
	old := radialPaint
	t.Cleanup(func() { radialPaint = old })
	var painted int
	radialPaint = func(p *ui.Painter, bmp *ui.Bitmap, cx, cy, radius, opacity float32) {
		painted++
		old(p, bmp, cx, cy, radius, opacity)
	}
	tst, box := cardGlowProbe(t)
	tst.Move(box.X+box.W/2, box.Y+box.H/2)
	tst.Advance(200 * time.Millisecond)
	if painted == 0 {
		t.Fatal("常态悬停时没画光晕（radialPaint 未被调用）")
	}

	painted = 0
	tst.SetPreferences(ui.Preferences{ReduceMotion: true})
	tst.Frame()
	if painted != 0 {
		t.Errorf("ReduceMotion 下仍画了 %d 次光晕（原版 pointerEffectsEnabled 要求 no-preference）", painted)
	}
}

// TestCardPressPaint 钉住抽屉卡片按下态的取值（原版 globals.css:4138 的
// `[data-slot="wrapper"] [role="button"][class*="bg-content"]:active`）：
//
//	background: radial(18% 10%, 白 .7, transparent 34%),
//	            radial(86% 88%, 天蓝 .13, transparent 38%),
//	            linear-gradient(145deg, 白 .52, 白 .32 54%, #e2f2fb .42)
//	box-shadow: inset 0 1px 0 白 .84, 0 0 0 2px 天蓝 .16, 0 4px 12px 墨 .10
//	transform:  translateY(0) scale(.985)
func TestCardPressPaint(t *testing.T) {
	g := cardPressGradient()
	want := []ui.GradientStop{
		{Color: ui.Hex("#ffffff").Alpha(0.52), At: 0},
		{Color: ui.Hex("#ffffff").Alpha(0.32), At: 0.54},
		{Color: ui.Hex("#e2f2fb").Alpha(0.42), At: 1},
	}
	if g.Angle != 145 || len(g.Stops) != len(want) {
		t.Fatalf("按下态底 = %+v，应为 145° 三段", g)
	}
	for i := range want {
		if g.Stops[i] != want[i] {
			t.Errorf("停靠点 %d = %+v，应为 %+v", i, g.Stops[i], want[i])
		}
	}
	if len(cardPressTopStops) != 2 || cardPressTopStops[0].C != ui.Hex("#ffffff").Alpha(0.70) ||
		cardPressTopStops[1].At != 0.34 || cardPressTopStops[1].C.A != 0 {
		t.Errorf("左上高光停靠点 = %+v，应为 白 .7 → 34%% 透明", cardPressTopStops)
	}
	if len(cardPressBottomStops) != 2 || cardPressBottomStops[0].C != ui.Hex("#0ea5e9").Alpha(0.13) ||
		cardPressBottomStops[1].At != 0.38 || cardPressBottomStops[1].C.A != 0 {
		t.Errorf("右下反光停靠点 = %+v，应为 天蓝 .13 → 38%% 透明", cardPressBottomStops)
	}
	pp := cardPressPaintFor()
	if pp.Inset != ui.Hex("#ffffff").Alpha(0.84) ||
		pp.Ring != ui.Hex("#0ea5e9").Alpha(0.16) ||
		pp.Shadow != ui.Hex("#0f172a").Alpha(0.10) {
		t.Errorf("按下态投影 = %+v，应为 白 .84 / 天蓝 .16 / 墨 .10", pp)
	}
	if cardPressScale != 0.985 {
		t.Errorf("按下缩放 = %v，应为 .985", cardPressScale)
	}
}

// TestCardPressRendered 无头实测：按住卡片时整卡换到「按下态」亮玻璃
// （左上白高光 .7 + 145° 白底），松开回落到悬停态。
//
// 取卡片左上 46×26 一块（封面区，按下底最亮处）的平均亮度。反证：把
// renderCard 里的 pressed 恒置 false（不接按下态），按下前后亮度不变。
func TestCardPressRendered(t *testing.T) {
	tst, box := cardGlowProbe(t)
	patch := func() float32 {
		img := tst.Image()
		var sum float32
		var n int
		for y := int(box.Y) + 4; y < int(box.Y)+30; y++ {
			for x := int(box.X) + 4; x < int(box.X)+50; x++ {
				i := img.PixOffset(x, y)
				sum += float32(int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2])) / 3
				n++
			}
		}
		return sum / float32(n)
	}
	rest := patch()
	tst.Move(box.X+box.W/2, box.Y+box.H/2)
	tst.Advance(200 * time.Millisecond) // 光晕渐显完再量悬停亮度
	hover := patch()
	tst.Press(box.X+box.W/2, box.Y+box.H/2)
	tst.Frame()
	pressed := patch()
	tst.Release(box.X+box.W/2, box.Y+box.H/2)
	tst.Frame()
	released := patch()
	t.Logf("左上亮度 常态=%.1f 悬停=%.1f 按下=%.1f 松开=%.1f", rest, hover, pressed, released)

	if pressed-hover < 30 {
		t.Errorf("按下只比悬停亮 %.1f —— 按下态背景没接上", pressed-hover)
	}
	if released-hover > 6 || hover-released > 6 {
		t.Errorf("松开后亮度 %.1f 没回到悬停态 %.1f —— 按下态没撤销", released, hover)
	}
}

// ------------------------------------------------------------ 旧写法（反证用）

// cardGlowSteps 是**旧**写法（同心圆近似，本轮已换掉）的层数：200px 半径上
// 16 层 ≈ 13.3px 一环。保留常量是为了让反证能按同样的环数重建旧画法。
const cardGlowSteps = 16

// cardGlowLayerAlpha 返回在外层累计不透明度为 prev 时，要把该点补到 want
// 这一环需要的额外不透明度（普通 alpha 叠加：a ∘ b = a + b(1-a)）。
//
// 这是旧写法（paintRadialStops 的同心圆）的补偿公式，现已不在生产路径上；
// 留着是为了让 TestCardGlowLayerAlpha 继续钉住「阶梯近似为什么会长这样」。
func cardGlowLayerAlpha(prev, want float32) float32 {
	if want <= prev {
		return 0
	}
	if d := 1 - prev; d > 0 {
		return (want - prev) / d
	}
	return 0
}

// cardRadialSteps 是旧写法按半径给的环数（约 4px 一环）。
func cardRadialSteps(radius float32) int {
	return max(cardGlowSteps, int(radius/4)+1)
}

// radialGradientBitmapStepped 是旧写法（paintRadialStops）的逐像素等价物：
// 把同一条渐变按 steps 个同心环量化，环内取**环外缘**的值 —— 那正是
// 「一串同心实心圆、每圈补一次 alpha」叠出来的结果（每点累计 alpha 等于该
// 半径处外缘环的 want 值）。用来做反证：它和解析曲线的误差就是本轮修掉的台阶。
func radialGradientBitmapStepped(stops []gradStop, steps int) *ui.Bitmap {
	n := radialBakeSize
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	c := float32(n) / 2
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx := float32(x) + 0.5 - c
			dy := float32(y) + 0.5 - c
			t := float32(math.Sqrt(float64(dx*dx+dy*dy))) / c
			// 落在第几个环里，取该环外缘的 t。
			ring := math.Ceil(float64(t) * float64(steps))
			col := gradAt(stops, float32(ring)/float32(steps))
			i := img.PixOffset(x, y)
			img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = col.R, col.G, col.B, col.A
		}
	}
	return ui.NewBitmap(img)
}

// TestCardGlowRadialMatchesCSS 是逐半径的因果测试：光晕在半径 r 处的合成
// 灰阶必须等于 CSS 的解析值（`radial-gradient(200px circle at …, 白 .6,
// 天蓝 .2 40%, transparent 70%)` × 整层 opacity .9，叠在中灰底上）。
//
// 本轮之前用「一串同心实心圆」近似，环内 alpha 恒定 ⇒ 半径方向是**阶梯**：
// 实测最大误差 4~5 灰阶（200px 半径 / 16 环，每环 13.3px 差 ≈17/255 alpha），
// 浅色卡面上能看出同心台阶。换成烘好的位图后误差 ≤1 灰阶。
//
// 反证：把 radialPaint 换成 stepped（按 16 环量化，等价旧写法）→ 误差回到 4+。
func TestCardGlowRadialMatchesCSS(t *testing.T) {
	const bg = 120
	// 解析值：白 .6 → 天蓝 .2@40% → 透明@70%，整层 .9。
	// 灰阶按三通道均值算（grayAt 量的就是它）。
	analytic := func(r float32) float32 {
		col := gradAt(cardGlowStops, r/cardGlowRadius)
		a := cardGlowOpacity * float32(col.A) / 255
		mean := (float32(col.R) + float32(col.G) + float32(col.B)) / 3
		return a*mean + (1-a)*bg
	}
	measure := func() float32 {
		img := ui.Render(func(c *ui.Context) {
			c.Root().Background(ui.RGB(bg, bg, bg))
			ui.Box(c).Absolute().Left(0).Top(0).Size(360, 280).
				Draw(func(p *ui.Painter, r ui.Rect) { paintCardGlow(p, r, 180, 140) })
		}, 400, 300, 1)
		var worst float32
		for r := float32(0); r <= 150; r += 3 {
			if d := abs32(float32(grayAt(img, int(180+r), 140)) - analytic(r)); d > worst {
				worst = d
			}
		}
		return worst
	}

	got := measure()
	t.Logf("位图渐变：与 CSS 解析值的最大误差 = %.2f 灰阶", got)
	// 阈值 3：位图逐像素烘好后残差只剩 8 位合成/取整（实测 1.9）；
	// 旧的分环写法实测 12.9，两者之间有 6 倍余量。
	if got > 3 {
		t.Fatalf("光晕与 CSS 解析值最大差 %.2f 灰阶 > 3 —— 径向渐变不是逐像素的", got)
	}

	// 反证：换回旧的分环写法，误差必须明显变大（证明本测试量的就是「台阶」）。
	ringBmp := radialGradientBitmapStepped(cardGlowStops, cardGlowSteps)
	img := ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.RGB(bg, bg, bg))
		ui.Box(c).Absolute().Left(0).Top(0).Size(360, 280).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.Clip(r, 12, func() {
					paintRadialGradient(p, ringBmp, 180, 140, cardGlowRadius, cardGlowOpacity)
				})
			})
	}, 400, 300, 1)
	var worst float32
	for r := float32(0); r <= 150; r += 3 {
		got := float32(grayAt(img, int(180+r), 140))
		if d := abs32(got - analytic(r)); d > worst {
			worst = d
		}
	}
	t.Logf("同心圆旧写法（%d 环）：最大误差 = %.2f 灰阶", cardGlowSteps, worst)
	if worst < 3 {
		t.Fatalf("反证失败：分环写法的误差只有 %.2f 灰阶，本测试量不出台阶", worst)
	}
}

// TestRadialBakeCached 钉住位图只烘一次（三条渐变各一张，进程内复用）：
// 每次悬停都重烘 512² 会白烧 0.3ms/帧。
func TestRadialBakeCached(t *testing.T) {
	a, b := cardGlowBmp(), cardGlowBmp()
	if a == nil || a != b {
		t.Fatalf("cardGlowBmp 没有缓存：%p vs %p", a, b)
	}
	if cardPressTopBmp() == nil || cardPressBottomBmp() == nil {
		t.Fatal("按下态的两条径向渐变没有烘成位图")
	}
	// 烘焙边长两头都要卡：太小拉伸后能看出插值台阶，太大白占显存
	// （每张 边长² × 4 字节）。
	if radialBakeSize < 128 || radialBakeSize > 512 {
		t.Fatalf("烘焙边长 %d 超出 [128,512]：小了有台阶、大了白占显存", radialBakeSize)
	}
}

