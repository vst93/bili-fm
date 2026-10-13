package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 本轮（选集搜索框 .part-search 的液态玻璃材质）的测试。原版 globals.css 给这个
// 输入腔挂了三层此前原生版没画的材质：
//
//	::before  --glass-sheen-static 光泽 + 内凹两条内影（顶暗底亮）
//	:hover    ::before 底换成 --liquid-bg-hover(白 .30)，整框叠 --liquid-shadow 抬起
//	:focus-within  ::before 底 rgba(255,255,255,.05)、内影变浅、::after opacity 1
//	::after   --liquid-rim 1px 聚光描边（常态 opacity .5）
//
// 以及 `transition: border-color/box-shadow 0.2s ease`、`::before background-color
// 0.2s ease`、`::after opacity 0.2s ease` 的过渡。

// freezePartSearchFades 把本轮补上的色值 / 不透明度过渡换成瞬时写入，供像素
// 对照测试消除动画相位干扰（同 freezeNavIconFades 的手法）。过渡本身的正确性由
// TestPartSearchFadeWired 单独钉住。
func freezePartSearchFades(t *testing.T) {
	t.Helper()
	fade, rim, icon, shadow := partSearchFade, partSearchRimOpacity, partSearchIconFade, partSearchShadow
	partSearchFade = func(b ui.Element, key string, target ui.Color) ui.Color { return target }
	partSearchRimOpacity = func(b ui.Element, target float32) float32 { return target }
	partSearchIconFade = func(b ui.Element, key string, target ui.Color) ui.Color { return target }
	partSearchShadow = func(b ui.Element, key string, s shadowSpec) shadowSpec { return s }
	t.Cleanup(func() { partSearchFade, partSearchRimOpacity, partSearchIconFade, partSearchShadow = fade, rim, icon, shadow })
}

// TestPartSearchAnimTokens 钉住过渡参数：0.2s + CSS ease。
func TestPartSearchAnimTokens(t *testing.T) {
	if partSearchAnim.Duration != 200*time.Millisecond {
		t.Errorf("时长 = %v, want 200ms（原版 0.2s）", partSearchAnim.Duration)
	}
	if partSearchAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", partSearchAnim.Spring)
	}
	if partSearchAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(partSearchAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want CSS ease %v", x, partSearchAnim.Ease(x), want(x))
		}
	}
}

// TestPartSearchIconAnimTokens 钉住放大镜换色的过渡参数：原版
// `.part-search-icon { transition: color 0.16s ease }` —— 0.16s 曲线，**不是**
// 框体那条 0.2s。
func TestPartSearchIconAnimTokens(t *testing.T) {
	if partSearchIconAnim.Duration != 160*time.Millisecond {
		t.Errorf("放大镜过渡时长 = %v, want 160ms（原版 0.16s）", partSearchIconAnim.Duration)
	}
	if partSearchIconAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", partSearchIconAnim.Spring)
	}
	if partSearchIconAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(partSearchIconAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want CSS ease %v", x, partSearchIconAnim.Ease(x), want(x))
		}
	}
}

// TestPartSearchFadeWired 实测描边 / 填充的色值过渡与 ::after 的不透明度过渡真的
// 以「过渡」而非「瞬时」落到 AnimateColor / AnimateTo：换目标的那一帧曲线 t=0
// ⇒ 返回**上一帧的值**；直接铺目标色会当帧跳到新值。
func TestPartSearchFadeWired(t *testing.T) {
	colA, colB := ui.Hex("#0f172a").Alpha(0.035), ui.Hex("#ffffff").Alpha(0.30)
	cur := colA
	rimTarget := float32(0.5)
	var cols []ui.Color
	var rims []float32
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		cols = append(cols, partSearchFade(b, "probe", cur))
		rims = append(rims, partSearchRimOpacity(b, rimTarget))
	}, 60, 60)
	tt.Frame()
	if len(cols) == 0 || len(rims) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if cols[0] != colA {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", colA, cols[0])
	}
	if rims[0] != 0.5 {
		t.Fatalf("首帧不透明度应直接取目标 0.5，得到 %v", rims[0])
	}

	cols, rims = nil, nil
	cur, rimTarget = colB, 1
	tt.Frame()
	if cols[0] == colB {
		t.Errorf("换目标的当帧就跳到新色 %v —— 没走 200ms 过渡", colB)
	}
	if cols[0] != colA {
		t.Errorf("色值过渡应从上一帧 %v 起步，得到 %v", colA, cols[0])
	}
	if rims[0] == 1 {
		t.Errorf("不透明度换目标的当帧就跳到 1 —— 没走 200ms 过渡")
	}
	if rims[0] != 0.5 {
		t.Errorf("不透明度过渡应从上一帧 0.5 起步，得到 %v", rims[0])
	}
}

// TestPartSearchIconFadeWired 实测放大镜换色确实走 160ms 过渡而非瞬时写入：换目标
// 的那一帧返回**上一帧颜色**。把 partsHeader 里的放大镜改回直接铺 paint.Icon 后，
// 本测试即红。
func TestPartSearchIconFadeWired(t *testing.T) {
	colA, colB := ui.Hex("#475569"), ui.Hex("#0284c7")
	cur := colA
	var cols []ui.Color
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		cols = append(cols, partSearchIconFade(b, "probe", cur))
	}, 60, 60)
	tt.Frame()
	if len(cols) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if cols[0] != colA {
		t.Fatalf("首帧应直接取目标 %v，得到 %v", colA, cols[0])
	}

	cols = nil
	cur = colB
	tt.Frame()
	if cols[0] == colB {
		t.Errorf("放大镜换色的当帧就跳到新色 %v —— 没走 160ms 过渡", colB)
	}
	if cols[0] != colA {
		t.Errorf("放大镜过渡应从上一帧 %v 起步，得到 %v", colA, cols[0])
	}
}

// TestPartSearchIconFadeWiring 反证放大镜真的接的是 `partSearchIconFade`（160ms
// 那条）而不是框体那条 200ms 的 `partSearchFade`：两者都换桩后，只有 icon fade
// 会被调到，且键名为 ps-icon。
func TestPartSearchIconFadeWiring(t *testing.T) {
	freezePartSearchFades(t)
	savedIcon, savedFade := partSearchIconFade, partSearchFade
	var iconKeys, boxKeys []string
	partSearchIconFade = func(b ui.Element, key string, target ui.Color) ui.Color {
		iconKeys = append(iconKeys, key)
		return savedIcon(b, key, target)
	}
	partSearchFade = func(b ui.Element, key string, target ui.Color) ui.Color {
		boxKeys = append(boxKeys, key)
		return savedFade(b, key, target)
	}
	defer func() { partSearchIconFade, partSearchFade = savedIcon, savedFade }()

	partSearchBoxFrame(t)
	found := false
	for _, k := range iconKeys {
		if k == "ps-icon" {
			found = true
		}
	}
	if !found {
		t.Errorf("放大镜没有走 partSearchIconFade（icon fade 键 %v）—— 是不是错接到框体那条 0.2s 过渡了？", iconKeys)
	}
	for _, k := range boxKeys {
		if k == "ps-icon" {
			t.Errorf("ps-icon 走了框体的 partSearchFade（%v）—— 应用 0.16s 的 partSearchIconFade", k)
		}
	}
}

// TestPartSearchShadowInflightWired 实测「抬起」与聚焦光环的 box-shadow 真的以
// 0.2s 过渡（几何 + 色值一起动），而不是像此前那样当帧切。判据同
// TestPartSearchFadeWired：目标变化的那一帧，曲线 t=0 ⇒ 返回**上一帧的整套
// shadowSpec**（五项逐字段相等）。
func TestPartSearchShadowInflightWired(t *testing.T) {
	start := shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}
	next := shadowSpec{0, 16, 38, 0, shadowInk.Alpha(0.11)}
	cur := start
	var got []shadowSpec
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		got = append(got, partSearchShadow(b, "probe", cur))
	}, 60, 60)
	tt.Frame()
	if len(got) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if got[0] != start {
		t.Fatalf("首帧应直接取目标 %+v，得到 %+v", start, got[0])
	}

	got = nil
	cur = next
	tt.Frame()
	if got[0] == next {
		t.Errorf("换目标的当帧就跳到新影 %+v —— 没走 200ms 过渡", next)
	}
	if got[0] != start {
		t.Errorf("box-shadow 过渡应从上一帧 %+v 起步（五项逐字段相等），得到 %+v", start, got[0])
	}
}

// TestPartSearchShadowGrowsFromNone 实测常态（box-shadow: none）→ 抬起时几何真的
// 从零长出来：从全零目标切到 --liquid-shadow，换目标那一帧应仍是零几何，而不是
// 当帧就带上 0 8 24。
func TestPartSearchShadowGrowsFromNone(t *testing.T) {
	cur := shadowSpec{}
	var got []shadowSpec
	tt := ui.NewTester(func(c *ui.Context) {
		b := ui.Box(c)
		got = append(got, partSearchShadow(b, "probe", cur))
	}, 60, 60)
	tt.Frame() // 首帧：零
	cur = shadowSpec{0, 8, 24, 0, shadowInk.Alpha(0.07)}
	got = nil
	tt.Frame()
	if len(got) == 0 {
		t.Fatal("探针视图没有被构建")
	}
	if got[0] != (shadowSpec{}) {
		t.Errorf("常态 → 抬起应仍为零几何起步，得到 %+v", got[0])
	}
}

// TestPartSearchShadowWiring 反证 partsHeader 真的把抬起 / 光环接到
// partSearchShadow，把内影的颜色接到 partSearchFade（而不是直接铺当帧值）。
func TestPartSearchShadowWiring(t *testing.T) {
	savedShadow, savedFade := partSearchShadow, partSearchFade
	var shadowKeys, fadeKeys []string
	partSearchShadow = func(b ui.Element, key string, s shadowSpec) shadowSpec {
		shadowKeys = append(shadowKeys, key)
		return savedShadow(b, key, s)
	}
	partSearchFade = func(b ui.Element, key string, target ui.Color) ui.Color {
		fadeKeys = append(fadeKeys, key)
		return savedFade(b, key, target)
	}
	defer func() { partSearchShadow, partSearchFade = savedShadow, savedFade }()

	partSearchBoxFrame(t)

	has := func(keys []string, want string) bool {
		for _, k := range keys {
			if k == want {
				return true
			}
		}
		return false
	}
	for _, k := range []string{"ps-lift-lg", "ps-lift-sm", "ps-ring"} {
		if !has(shadowKeys, k) {
			t.Errorf("抬起 / 光环 %q 没走 partSearchShadow（拿到 %v）", k, shadowKeys)
		}
	}
	for _, k := range []string{"ps-inset-top", "ps-inset-bot"} {
		if !has(fadeKeys, k) {
			t.Errorf("内影 %q 没走 partSearchFade（拿到 %v）", k, fadeKeys)
		}
	}
}

// TestPartSearchGlassPaint 直接钉住 partSearchGlass 画出的两层材质：在 180×32 的
// 圆角框上叠 sheen + rim 后，框域里有明显像素被改动（画的是光泽/描边，不是空操作）。
func TestPartSearchGlassPaint(t *testing.T) {
	render := func(glass bool) *image.RGBA {
		return ui.Render(func(c *ui.Context) {
			// 深底：sheen / rim 都是白色光泽，铺在白底上量不出差别。
			c.Root().Background(ui.Hex("#334155"))
			b := ui.Row(c).Width(180).Height(32).Radius(Radius)
			if glass {
				b = partSearchGlass(b, 0.5)
			}
		}, 200, 60, 1)
	}
	on, off := render(true), render(false)
	diff, total := 0, 0
	for y := 0; y < on.Bounds().Dy(); y++ {
		for x := 0; x < on.Bounds().Dx(); x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("partSearchGlass：%d/%d 像素被改动", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受玻璃层影响（<5%%）—— sheen / rim 没画上？", diff, total)
	}
}

// partSearchBoxFrame 造一帧选集抽屉，返回搜索框所在的输入矩形与截图。
func partSearchBoxFrame(t *testing.T) (*ui.Tester, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = newPartsInfo(3, "cover")
	a.Track = &Track{Bvid: "BV1", Cid: 1000, Title: "视频"}
	a.Drawer = DrawerParts
	tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 980, 720)
	tt.Frame()
	r, ok := tt.Find("搜索选集")
	if !ok {
		t.Fatal("找不到选集搜索框（label「搜索选集」）")
	}
	return tt, r
}

// boxArea 把输入框矩形外扩到大致覆盖整个 .part-search 容器（左 40 / 右 14 /
// 上下 10），用于量整框的像素。
func boxArea(r ui.Rect, w, h int) (int, int, int, int) {
	x0, y0 := int(r.X)-40, int(r.Y)-10
	x1, y1 := int(r.X+r.W)+14, int(r.Y+r.H)+10
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > w {
		x1 = w
	}
	if y1 > h {
		y1 = h
	}
	return x0, y0, x1, y1
}

// TestPartSearchGlassWired 无头反证：抽屉表头的搜索框真的接上了 partSearchGlass
// —— 把 partSearchGlass 临时换成 no-op 后，搜索框区域的像素跟着变。
func TestPartSearchGlassWired(t *testing.T) {
	freezePartSearchFades(t)
	render := func(stub bool) (*image.RGBA, ui.Rect) {
		if stub {
			saved := partSearchGlass
			partSearchGlass = func(b ui.Element, rim float32) ui.Element { return b }
			defer func() { partSearchGlass = saved }()
		}
		tt, r := partSearchBoxFrame(t)
		return tt.Image(), r
	}
	on, rect := render(false)
	off, _ := render(true)
	img := on
	x0, y0, x1, y1 := boxArea(rect, img.Bounds().Dx(), img.Bounds().Dy())
	diff, total := 0, 0
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("选集搜索框：%d/%d 像素随 partSearchGlass 开关变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受玻璃层影响（<5%%）—— partsHeader 没接 partSearchGlass？", diff, total)
	}
}

// TestPartSearchHoverFillWired 无头实测：悬停把搜索框的底从内凹的深色薄底换成
// --liquid-bg-hover（白 .30），整框明显变亮。原版 `.part-search:hover::before
// { background-color: var(--liquid-bg-hover) }`；此前原生版悬停底不变。
func TestPartSearchHoverFillWired(t *testing.T) {
	freezePartSearchFades(t)

	meanLum := func(hover bool) float64 {
		tt, r := partSearchBoxFrame(t)
		if hover {
			tt.Move(r.X+r.W/2, r.Y+r.H/2)
			tt.Frame()
		}
		img := tt.Image()
		x0, y0, x1, y1 := boxArea(r, img.Bounds().Dx(), img.Bounds().Dy())
		sum, n := 0, 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				i := img.PixOffset(x, y)
				sum += int(img.Pix[i]) + int(img.Pix[i+1]) + int(img.Pix[i+2])
				n++
			}
		}
		return float64(sum) / float64(3*n)
	}

	off, on := meanLum(false), meanLum(true)
	t.Logf("搜索框平均亮度：常态 %.1f → 悬停 %.1f", off, on)
	if on-off < 3 {
		t.Errorf("悬停只让搜索框平均亮度 +%.1f（常态 %.1f / 悬停 %.1f）：悬停白底没接上？", on-off, off, on)
	}
}
