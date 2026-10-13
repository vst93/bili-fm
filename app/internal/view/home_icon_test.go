package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// TestPartPillDotSizeAndTitleInk 钉住选集胶囊（.video-part-pill）两个与原版
// 对不上的细节：
//
//	.video-part-dot    7px（此前画成 6px，圆点在胶囊里显得发飘）
//	.video-part-title  #475569 / 600（此前染成胶囊本体的 #0369a1 / 700）
//
// 除常量外还做一次无头渲染实测：让来源是播放列表（圆点走玫红 t.Rose），量
// 玫红像素的包围盒宽度 —— 7px 的实心圆在 scale 1 下应量到 7。
func TestPartPillDotSizeAndTitleInk(t *testing.T) {
	if partDotSize != 7 {
		t.Errorf("圆点尺寸 = %v, want 7（原版 .video-part-dot 7px）", partDotSize)
	}
	if partTitleInk != ui.Hex("#475569") {
		t.Errorf("分集标题色 = %v, want #475569（原版 .video-part-title）", partTitleInk)
	}

	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.PlayingPlaylist = ListUser // 播放列表来源 → 圆点玫红（与蓝色文字区分开）
	a.Track = &Track{Bvid: "BV1", Part: "第一集"}
	img := ui.Render(func(c *ui.Context) {
		// 白底：玫红圆点与蓝色/灰色文字都不掺白。
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Padding(20).Children(func() { a.partPill(c) })
	}, 500, 120, 1)

	// 玫红 t.Rose = #e11d48 → (225,29,72)：R 高、G/B 低。
	minX, maxX, minY, maxY := 1<<30, -1, 1<<30, -1
	for y := 0; y < img.Bounds().Dy(); y++ {
		for x := 0; x < img.Bounds().Dx(); x++ {
			i := img.PixOffset(x, y)
			r, g, b := int(img.Pix[i]), int(img.Pix[i+1]), int(img.Pix[i+2])
			if r > 150 && r-g > 100 && r-b > 80 {
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}
	if maxX < 0 {
		t.Fatal("渲染图里找不到玫红圆点")
	}
	w, h := maxX-minX+1, maxY-minY+1
	t.Logf("圆点玫红包围盒 %d×%d", w, h)
	if w < 7 || h < 7 {
		t.Errorf("圆点包围盒 %d×%d 小于 7×7 —— 圆点还是旧的 6px？", w, h)
	}
	if w > 8 || h > 8 {
		t.Errorf("圆点包围盒 %d×%d 偏大（应约 7×7）", w, h)
	}
}

// TestHoverHighlightSpecs 钉住悬停 ::after 三条 box-shadow 的取值（不含 inset
// 标志，几何逐值）：
//
//	.nav-icon-btn / .search-submit-btn  inset 0 1px 1px 白 .9 / inset 0 -1px 1px 墨 .03 / 0 2px 10px 墨 .045
//	.top-tool-btn                       inset 0 1px 0    白 .9（顶缘 blur 0，其余同上）
func TestHoverHighlightSpecs(t *testing.T) {
	nav := navHoverShadowSpecs()
	if len(nav) != 3 {
		t.Fatalf("nav 悬停高光应 3 条，得到 %d", len(nav))
	}
	if nav[0] != (shadowSpec{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}) {
		t.Errorf("nav 第 1 条 = %+v, want inset 0 1px 1px 白 .9", nav[0])
	}
	if nav[1] != (shadowSpec{0, -1, 1, 0, ui.Hex("#0f172a").Alpha(0.03)}) {
		t.Errorf("nav 第 2 条 = %+v, want inset 0 -1px 1px 墨 .03", nav[1])
	}
	if nav[2] != (shadowSpec{0, 2, 10, 0, ui.Hex("#0f172a").Alpha(0.045)}) {
		t.Errorf("nav 第 3 条 = %+v, want 0 2px 10px 墨 .045", nav[2])
	}

	// 入口键顶缘是 `inset 0 1px 0`（blur 0）。
	entry := topToolHoverShadowSpecs(topToolEntry)
	if entry[0] != (shadowSpec{0, 1, 0, 0, ui.Hex("#ffffff").Alpha(0.90)}) {
		t.Errorf("入口键顶缘 = %+v, want inset 0 1px 0 白 .9（blur 0）", entry[0])
	}
	// 提交键顶缘是 `inset 0 1px 1px`（blur 1）。
	submit := topToolHoverShadowSpecs(topToolSubmit)
	if submit[0] != (shadowSpec{0, 1, 1, 0, ui.Hex("#ffffff").Alpha(0.90)}) {
		t.Errorf("提交键顶缘 = %+v, want inset 0 1px 1px 白 .9（blur 1）", submit[0])
	}
	// 两类键的后两条一致，且都比旧的近似值（白 .55 / blur 7）高一档。
	for i := 1; i < 3; i++ {
		if entry[i] != nav[i] || submit[i] != nav[i] {
			t.Errorf("两类小键第 %d 条应与 nav 族一致：entry=%+v submit=%+v", i, entry[i], submit[i])
		}
	}
}

// TestNavHoverHighlightWired 无头实测：主区操作行的图标键（.nav-icon-btn）
// 悬停时真的叠上了 ::after 内高光 —— 把 navHoverHighlight 临时换成 no-op 后，
// 同一按钮区域里有像素跟着变，反证它来自这个 helper。
func TestNavHoverHighlightWired(t *testing.T) {
	render := func() (*image.RGBA, ui.Rect) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
		a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
		a.Queue = []Track{*a.Track}
		a.Index = 0
		tt := shellTesterSize(t, a, 1000, 700)
		tt.Frame()
		r, ok := tt.Find("浏览器打开")
		if !ok {
			t.Fatal("找不到「浏览器打开」按钮")
		}
		// 悬停到按钮中心，再渲染一帧（::after 只在悬停态出现）。
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		return tt.Image(), r
	}

	on, rect := render()
	saved := navHoverHighlight
	navHoverHighlight = func(b ui.Element) ui.Element { return b }
	defer func() { navHoverHighlight = saved }()
	off, _ := render()

	diff, total := 0, 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("操作行图标键悬停：%d/%d 像素随 ::after 开关变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受内高光影响（<5%%）—— 悬停 ::after 没接上？", diff, total)
	}
}

// TestTopToolHoverHighlightWired 同上，针对搜索药丸的内容入口键（.top-tool-btn）。
// 它此前用的是「白 .55 描边 + inset .55」近似，本轮换成原版逐值的 ::after。
func TestTopToolHoverHighlightWired(t *testing.T) {
	render := func() (*image.RGBA, ui.Rect) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		tt := shellTesterSize(t, a, 1000, 700)
		tt.Frame()
		r, ok := tt.Find("动态")
		if !ok {
			t.Fatal("找不到「动态」入口键")
		}
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
		tt.Frame()
		return tt.Image(), r
	}

	on, rect := render()
	saved := topToolHoverHighlight
	topToolHoverHighlight = func(b ui.Element, k topToolKind) ui.Element { return b }
	defer func() { topToolHoverHighlight = saved }()
	off, _ := render()

	diff, total := 0, 0
	for y := int(rect.Y); y < int(rect.Y+rect.H); y++ {
		for x := int(rect.X); x < int(rect.X+rect.W); x++ {
			total++
			i := on.PixOffset(x, y)
			if on.Pix[i] != off.Pix[i] || on.Pix[i+1] != off.Pix[i+1] || on.Pix[i+2] != off.Pix[i+2] {
				diff++
			}
		}
	}
	t.Logf("内容入口键悬停：%d/%d 像素随 ::after 开关变化", diff, total)
	if diff < total/20 {
		t.Errorf("只有 %d/%d 像素受内高光影响（<5%%）—— 悬停 ::after 没接上？", diff, total)
	}
}

// TestHomeAvatarsRetained 钉住「登录头像 / UP 头像不定期闪动」的修法：两张
// 头像一直挂在主区（抽屉开着时也可见），每帧都要 Retain（缓存淘汰永不丢它们）
// —— 否则抽屉里一轮封面抓取（一屏十几张、缓存上限 64）就把它们挤出 LRU，
// 下一帧 Bitmap 未命中又重抓，表现为头像「闪一下」。
func TestHomeAvatarsRetained(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	rec := newRecordingCache()
	a.Images = rec
	a.LoggedIn = true
	a.Face = "http://example.invalid/me.png"
	a.Info = &Info{
		Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题",
		OwnerFace: "http://example.invalid/up.png",
	}
	a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
	a.Queue = []Track{*a.Track}
	a.Index = 0

	tst := shellTesterSize(t, a, 1000, 700)
	tst.Frame()

	rec.mu.Lock()
	retains := append([]string(nil), rec.retains...)
	rec.mu.Unlock()
	has := func(u string) bool {
		for _, r := range retains {
			if r == u {
				return true
			}
		}
		return false
	}
	if !has(a.Face) {
		t.Errorf("登录头像 %q 没被 Retain（retains=%v）", a.Face, retains)
	}
	if !has(a.Info.OwnerFace) {
		t.Errorf("UP 头像 %q 没被 Retain（retains=%v）", a.Info.OwnerFace, retains)
	}
}

// TestFaceHoverAnimMatchesOriginal 钉住 UP 头像 / 登录头像悬停放大 / 回落的
// 动画与原版一致：`transition: ... transform 0.18s ease`（globals.css:2178 /
// .home-user-avatar）。之前用 ui.Snappy（约 500ms 弹簧），悬停偏黏。
func TestFaceHoverAnimMatchesOriginal(t *testing.T) {
	if faceHoverAnim.Duration != 180*time.Millisecond {
		t.Errorf("时长 = %v, want 180ms（原版 0.18s）", faceHoverAnim.Duration)
	}
	if faceHoverAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", faceHoverAnim.Spring)
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	if faceHoverAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(faceHoverAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want %v", x, faceHoverAnim.Ease(x), want(x))
		}
	}
}
