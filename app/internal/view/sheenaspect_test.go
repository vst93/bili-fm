package view

import (
	"image"
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// 本轮（sheen 位图按盒宽高比烘焙）的测试。背景：原版 `--glass-sheen-static` /
// `--player-sheen` 是 `radial-gradient(...), linear-gradient(<角度>deg, ...)` 两层。
// 原生版此前把这条 background-image 逐像素烘成**一张正方形位图**，画的时候
// 拉伸到目标盒：径向层按盒的百分比定义，拉伸后与 CSS 完全一致；线性层的角度
// 却是设备空间固定值，非等比拉伸会把角度一起拉偏（300×36 的宽盒上实测预乘通道
// 平均差 ~9.5、峰值 ~48）。本轮的改法：按盒的宽高比烘一张（长边固定
// playerSheenSide、短边按比例），各组件改走 paintBakedSheen。

// --- ① 烘焙尺寸跟着盒的宽高比走 -------------------------------------------

func TestSheenBakeSizeFollowsBoxAspect(t *testing.T) {
	cases := []struct {
		w, h float32
		ew   int
		eh   int
	}{
		{32, 32, 128, 128}, // 方形按钮 ⇒ 用共用那张
		{42, 42, 128, 128}, // 播放键
		{300, 36, 128, 15}, // 选集胶囊（round(128*36/300)=15）
		{200, 32, 128, 20}, // .part-search 搜索框
		{120, 30, 128, 32}, // 抽屉表头 tab
		{184, 42, 128, 29}, // 音量弹层（round(128*42/184)=29）
		{36, 300, 15, 128}, // 竖长盒：长边还是 128
		{1, 1000, 1, 128},  // 极窄：短边夹到 1px
		{0, 10, 128, 128},  // 退化输入不放大会崩
		{10, 0, 128, 128},  //
		{-5, 10, 128, 128}, //
	}
	for _, c := range cases {
		gw, gh := sheenBakeSize(c.w, c.h)
		if gw != c.ew || gh != c.eh {
			t.Errorf("sheenBakeSize(%g,%g) = %d×%d，应为 %d×%d", c.w, c.h, gw, gh, c.ew, c.eh)
		}
		// 长边必须落在 playerSheenSide 上（内存有界）。
		if gw != playerSheenSide && gh != playerSheenSide {
			t.Errorf("sheenBakeSize(%g,%g) 的长边 = %d×%d，没有落在 %d 上", c.w, c.h, gw, gh, playerSheenSide)
		}
	}
}

// --- ② 按宽高比烘真的把角度对上了（与 CSS 参考实现比） ---------------------

// sheenPremulAt 读 NRGBA 位图在归一化坐标 (u,v) 处的像素，返回预乘 alpha 的
// RGBA（渲染器合成的口径）。用最近邻取样，对应「把位图拉伸到目标盒」时某个
// 归一化位置上的取值。
func sheenPremulAt(img *image.NRGBA, u, v float64) [4]int {
	b := img.Bounds()
	x := int(u * float64(b.Dx()))
	y := int(v * float64(b.Dy()))
	if x >= b.Dx() {
		x = b.Dx() - 1
	}
	if y >= b.Dy() {
		y = b.Dy() - 1
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	i := img.PixOffset(x, y)
	a := int(img.Pix[i+3])
	return [4]int{int(img.Pix[i+0]) * a / 255, int(img.Pix[i+1]) * a / 255, int(img.Pix[i+2]) * a / 255, a}
}

func sheenAbsInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// sheenAspectTol 是「位图宽高比 == 盒宽高比」的容差。短边是四舍五入到整数的
// （128×36/300 = 15.36 → 15），最大的相对误差在短边为 1–2px 时也只有 ~0.07，
// 真实盒的短边都在 15px 以上，取 0.02 足够松。
const sheenAspectTol = 0.02

// sheenAspectGap 返回位图宽高比与盒宽高比之差。这是不依赖 sheenBakeSize 的
// 独立判据：把烘焙尺寸写死成 128×128，非方形盒就会在这里露馅。
func sheenAspectGap(bmpW, bmpH int, boxW, boxH float32) float64 {
	if bmpW <= 0 || bmpH <= 0 || !(boxW > 0) || !(boxH > 0) {
		return 0
	}
	d := float64(bmpH)/float64(bmpW) - float64(boxH)/float64(boxW)
	return math.Abs(d)
}

// sheenPremulDrift 把「baked 位图拉伸到 boxW×boxH」与「直接按 boxW×boxH 逐像素
// 算的 CSS 参考 ref」逐点比，返回（平均、峰值）预乘通道差。
func sheenPremulDrift(baked, ref *image.NRGBA, boxW, boxH int) (float64, int) {
	sum, max, n := 0, 0, 0
	for yi := 0; yi < boxH; yi++ {
		for xi := 0; xi < boxW; xi++ {
			u := (float64(xi) + 0.5) / float64(boxW)
			v := (float64(yi) + 0.5) / float64(boxH)
			want := sheenPremulAt(ref, u, v)
			got := sheenPremulAt(baked, u, v)
			d := 0
			for k := 0; k < 4; k++ {
				d += sheenAbsInt(want[k] - got[k])
			}
			sum += d
			if d > max {
				max = d
			}
			n++
		}
	}
	return float64(sum) / float64(n), max
}

// sheenSquareBake 是旧行为那一张（正方形烘焙）。
func sheenSquareBake(spec sheenSpec) *image.NRGBA {
	return sheenPixels(spec, playerSheenSide, playerSheenSide)
}

// playerBarOnlyProbe 只渲染播放栏（不跑 Shell）——播放栏的 sheen 断言要按
// 「按钮盒」逐颗看，而 Shell 里还会带上视频详情区的选集胶囊（它固定走浅色的
// --glass-sheen-static，与时段无关，会把「深色时段铺 --player-sheen」的判据搅浑）。
func playerBarOnlyProbe(t *testing.T, mutate func(a *App)) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Dur = 200
	a.Pos = 100
	a.SeekValue = 100
	a.Volume = 0.5
	if mutate != nil {
		mutate(a)
	}
	tst := ui.NewTester(func(c *ui.Context) { a.playerBar(c) }, 1000, 120)
	tst.Frame()
	return tst
}

// TestSheenAspectBakeMatchesCss 是这一轮的核心判据：在几个真实盒子上，按宽高比
// 烘的位图拉伸回去，必须明显比旧的「正方形位图拉伸」更接近逐像素算的 CSS 参考。
func TestSheenAspectBakeMatchesCss(t *testing.T) {
	boxes := [][2]int{{300, 36}, {200, 32}, {120, 30}, {184, 42}}
	for _, box := range boxes {
		w, h := box[0], box[1]
		// CSS 参考：把同一条 sheen 直接按盒的像素尺寸逐像素算出来。
		ref := sheenPixels(sheenGlass, w, h)
		// 旧行为：正方形位图拉伸。
		sqMean, sqMax := sheenPremulDrift(sheenSquareBake(sheenGlass), ref, w, h)
		// 本轮：按盒宽高比烘的位图。
		aw, ah := sheenBakeSize(float32(w), float32(h))
		aspMean, aspMax := sheenPremulDrift(sheenPixels(sheenGlass, aw, ah), ref, w, h)
		t.Logf("%d×%d（烘 %d×%d）：正方形拉伸 平均 %.2f / 峰值 %d；按比例烘 平均 %.2f / 峰值 %d",
			w, h, aw, ah, sqMean, sqMax, aspMean, aspMax)
		if aspMean > 4 {
			t.Errorf("%d×%d：按比例烘后平均预乘差 %.2f > 4（角度还是偏的？）", w, h, aspMean)
		}
		if aspMax > 40 {
			t.Errorf("%d×%d：按比例烘后峰值预乘差 %d > 40", w, h, aspMax)
		}
		if sqMean < 2*aspMean {
			t.Errorf("%d×%d：正方形拉伸（%.2f）不比按比例烘（%.2f）差两倍以上，判据失效",
				w, h, sqMean, aspMean)
		}
	}
}

// TestSheenAspectBakeExactForLinearOnly：线性层的角度是这次修的唯一对象，所以
// 单独拿「只有线性层」的 sheen 验一次——除低位图的行采样量化外应当逐点吻合。
func TestSheenAspectBakeExactForLinearOnly(t *testing.T) {
	spec := sheenSpec{
		Linear: true, LAngle: 170,
		LStops: []sheenStop{
			{At: 0, Col: ui.Hex("#ffffff").Alpha(0.20)},
			{At: 1, Col: ui.Transparent},
		},
	}
	const w, h = 300, 36
	ref := sheenPixels(spec, w, h)
	_, sqMax := sheenPremulDrift(sheenSquareBake(spec), ref, w, h)
	aw, ah := sheenBakeSize(w, h)
	aspMean, aspMax := sheenPremulDrift(sheenPixels(spec, aw, ah), ref, w, h)
	t.Logf("线性层 300×36（烘 %d×%d）：正方形拉伸峰值 %d；按比例烘 平均 %.2f / 峰值 %d",
		aw, ah, sqMax, aspMean, aspMax)
	if sqMax < 8 {
		t.Errorf("正方形拉伸的峰值只有 %d，判据看不出角度偏差（阈值失效）", sqMax)
	}
	if aspMax > 8 {
		t.Errorf("按比例烘后线性层峰值仍有 %d（应只剩行采样量化）", aspMax)
	}
}

// --- ③ 位图缓存 -----------------------------------------------------------

func TestSheenBitmapForCache(t *testing.T) {
	b := &sheenBaked{spec: sheenGlass}
	if b.bitmapFor(32, 32) != b.bitmap() {
		t.Error("正方形盒应当直接用共用那张位图（bitmapFor(w,w) != bitmap()）")
	}
	a1 := b.bitmapFor(300, 36)
	a2 := b.bitmapFor(300, 36)
	if a1 == nil || a1 != a2 {
		t.Error("同一尺寸的盒应当命中同一张缓存位图")
	}
	if a1 == b.bitmapFor(120, 30) {
		t.Error("不同宽高比的盒不该共用同一张位图")
	}
	if w, h := a1.Size(); w != 128 || h != 15 {
		t.Errorf("300×36 的位图 = %d×%d，应为 128×15", w, h)
	}
	// 不同 sheen（浅色 / 烟熏）不能互相串。
	if sheenGlassBaked.bitmapFor(300, 36) == sheenDarkBaked.bitmapFor(300, 36) {
		t.Error("--glass-sheen-static 与 --player-sheen 的宽盒位图串了")
	}
}

// --- ④ 各组件的接线：非方形盒必须铺按比例烘的那张 -------------------------

// TestSheenBakedSheenWiring 逐个组件反证：把 playerSheenPaint 换成记录桩，每个
// 铺 sheen 的组件取到的位图尺寸都必须等于它自己盒子的 sheenBakeSize。改回
// `.bitmap()`（正方形）就会变红。
func TestSheenBakedSheenWiring(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	type hit struct {
		r    ui.Rect
		w, h int
	}
	var hits []hit
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		w, h := bmp.Size()
		hits = append(hits, hit{r, w, h})
	}

	ui.Render(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#334155"))
		partSearchGlass(ui.Row(c).Width(200).Height(36), 1)
		drawerTabGlass(ui.Row(c).Width(120).Height(30), 1)
		partPillGlass(ui.Row(c).Width(300).Height(36))
	}, 420, 120, 1)

	if len(hits) < 3 || len(hits)%3 != 0 {
		t.Fatalf("三个玻璃组件记到 %d 次 sheen 绘制（应为 3 的倍数）", len(hits))
	}
	for _, hh := range hits {
		ew, eh := sheenBakeSize(hh.r.W, hh.r.H)
		if hh.w != ew || hh.h != eh {
			t.Errorf("盒 %.0f×%.0f 上铺的位图 %d×%d，应为 %d×%d",
				hh.r.W, hh.r.H, hh.w, hh.h, ew, eh)
		}
		if hh.w == hh.h && hh.r.W != hh.r.H {
			t.Errorf("盒 %.0f×%.0f 上铺了正方形位图 %d×%d —— 又退回按正方形烘了",
				hh.r.W, hh.r.H, hh.w, hh.h)
		}
		// 独立于 sheenBakeSize 的判据：位图的宽高比必须就是盒的宽高比。
		if d := sheenAspectGap(hh.w, hh.h, hh.r.W, hh.r.H); d > sheenAspectTol {
			t.Errorf("盒 %.0f×%.0f 上铺的位图 %d×%d 宽高比偏了 %.4f",
				hh.r.W, hh.r.H, hh.w, hh.h, d)
		}
	}
}

// TestPlayerBarSheenUsesBakedSize：播放栏每颗键上铺的位图尺寸必须等于它自己盒子的
// sheenBakeSize（方形键仍矮到共用那张 128×128，40×32 / 34×32 的键则各自按比例烘）。
func TestPlayerBarSheenUsesBakedSize(t *testing.T) {
	orig := playerSheenPaint
	defer func() { playerSheenPaint = orig }()
	total, nonSquare := 0, 0
	playerSheenPaint = func(p *ui.Painter, r ui.Rect, radius float32, bmp *ui.Bitmap) {
		total++
		w, h := bmp.Size()
		ew, eh := sheenBakeSize(r.W, r.H)
		if w != ew || h != eh {
			t.Errorf("键 %.0f×%.0f 上铺的位图 %d×%d，应为 %d×%d", r.W, r.H, w, h, ew, eh)
		}
		if r.W != r.H {
			nonSquare++
		} else if w != h {
			t.Errorf("方形键 %.0f×%.0f 上铺了非正方形位图 %d×%d", r.W, r.H, w, h)
		}
		// 独立于 sheenBakeSize 的判据：位图的宽高比必须就是键盒的宽高比。
		if d := sheenAspectGap(w, h, r.W, r.H); d > sheenAspectTol {
			t.Errorf("键 %.0f×%.0f 上铺的位图 %d×%d 宽高比偏了 %.4f", r.W, r.H, w, h, d)
		}
	}
	playerBarOnlyProbe(t, nil)
	if total == 0 {
		t.Fatal("播放栏没有铺 sheen（playerSheenPaint 没被调用）")
	}
	// 播放栏的倍速键 40×32、EQ / 跳过赞助键 34×32：非方形盒必须真的存在，
	// 否则这条判据只在正方形上打转（旧实现能蒙混过关）。
	if nonSquare == 0 {
		t.Error("播放栏一颗非方形键都没摸到，判据失效")
	}
}
