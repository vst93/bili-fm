package view

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestCoverSurfaceStyle 钉住 #video-cover（碟片 / 方形封面）三态描边与投影的取值
// ——直接抄原版 globals.css 的 #video-cover / :hover / .is-playing 三条规则：
//
//	常态    inset 0 0 0 1px rgba(255,255,255,.10) + 0 8px 24px rgba(15,23,42,.10)
//	悬停    inset ... .15                        + 0 12px 32px rgba(14,165,233,.12)
//	播放中  inset ... .15                        + 0 12px 36px rgba(14,165,233,.16)
//
// 原生版之前只画了内侧 1px 白线（且固定 .10），唱片像平贴在玻璃环里、播放时
// 也没有蓝色光晕。这个测试锁住三态。
func TestCoverSurfaceStyle(t *testing.T) {
	base := coverSurfaceFor(false, false)
	if base.Inset != ui.Hex("#ffffff").Alpha(0.10) {
		t.Errorf("常态内描边 = %v，应为白 .10", base.Inset)
	}
	if base.Dy != 8 || base.Blur != 24 {
		t.Errorf("常态投影 = %v/%v，应为 0 8 24", base.Dy, base.Blur)
	}
	if base.Outer != ui.Hex("#0f172a").Alpha(0.10) {
		t.Errorf("常态投影色 = %v，应为 #0f172a .10", base.Outer)
	}

	hover := coverSurfaceFor(false, true)
	if hover.Inset != ui.Hex("#ffffff").Alpha(0.15) {
		t.Errorf("悬停内描边 = %v，应为白 .15", hover.Inset)
	}
	if hover.Dy != 12 || hover.Blur != 32 {
		t.Errorf("悬停投影 = %v/%v，应为 0 12 32", hover.Dy, hover.Blur)
	}
	if hover.Outer != ui.Hex("#0ea5e9").Alpha(0.12) {
		t.Errorf("悬停投影色 = %v，应为 #0ea5e9 .12", hover.Outer)
	}

	play := coverSurfaceFor(true, false)
	if play.Inset != ui.Hex("#ffffff").Alpha(0.15) {
		t.Errorf("播放中内描边 = %v，应为白 .15", play.Inset)
	}
	if play.Dy != 12 || play.Blur != 36 {
		t.Errorf("播放中投影 = %v/%v，应为 0 12 36", play.Dy, play.Blur)
	}
	if play.Outer != ui.Hex("#0ea5e9").Alpha(0.16) {
		t.Errorf("播放中投影色 = %v，应为 #0ea5e9 .16", play.Outer)
	}

	// playing 压过 hovered：.is-playing 与 :hover 同特异度，前者在样式表里更靠后。
	if coverSurfaceFor(true, true) != play {
		t.Error("播放中同时悬停：应以 .is-playing 的 0 12 36 为准")
	}
}

// coverProbe 渲染一张主窗，返回无头位图，便于对封面做像素测量。
func coverProbe(t *testing.T, mode string, playing bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.CoverMode = mode
	a.Playing = playing
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	giveDarkCover(a)
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 1000, 700)
	tst.Frame()
	return tst
}

// coverBox 是封面（深色 #0f172a）在窗口里的包围盒。顶行深色像素数 topRun
// 用来区分「圆」与「圆角方块」。
type coverBox struct {
	minX, minY, maxX, maxY int
	topRun                 int
}

func (b coverBox) w() int { return b.maxX - b.minX + 1 }
func (b coverBox) h() int { return b.maxY - b.minY + 1 }

// measureCover 在左栏（x < 300）里找封面的深色包围盒。文案/按钮等深色元素都在
// 右栏，不会混进来。
func measureCover(t *testing.T, tst *ui.Tester) coverBox {
	t.Helper()
	img := tst.Image()
	dark := func(x, y int) bool {
		i := img.PixOffset(x, y)
		return int(img.Pix[i]) < 60 && int(img.Pix[i+1]) < 80 && int(img.Pix[i+2]) < 100
	}
	box := coverBox{minY: -1}
	for y := 100; y < 700; y++ {
		any := false
		for x := 20; x < 300; x++ {
			if dark(x, y) {
				any = true
				break
			}
		}
		if any {
			if box.minY < 0 {
				box.minY = y
			}
			box.maxY = y
		}
	}
	if box.minY < 0 {
		t.Fatal("左栏里找不到封面深色区域")
	}
	cy := (box.minY + box.maxY) / 2
	for x := 0; x < 500; x++ {
		if dark(x, cy) {
			if box.minX == 0 && box.maxX == 0 {
				box.minX = x
			}
			box.maxX = x
		}
	}
	for x := 20; x < 300; x++ {
		if dark(x, box.minY) {
			box.topRun++
		}
	}
	return box
}

// TestCoverSquareMatchesDiscSize 是「方形封面被放大到 292」的回归锁：
// 原版 #video-cover.cover-square 与碟片一样是 .cover-shell 的 90%（= 263），
// 之前用 Fill() 铺满 292，方块封面比碟片大了一圈（93%），右下角三个开关也被
// 挤到封面上。两者应同尺寸、同中心。
func TestCoverSquareMatchesDiscSize(t *testing.T) {
	disc := measureCover(t, coverProbe(t, "disc", false))
	square := measureCover(t, coverProbe(t, "square", false))
	t.Logf("碟片 %+v（%d×%d） 方形 %+v（%d×%d）", disc, disc.w(), disc.h(), square, square.w(), square.h())

	if d := square.w() - disc.w(); d < -3 || d > 3 {
		t.Errorf("方形封面宽 %d，碟片宽 %d：应同为 263 左右（差 %d）", square.w(), disc.w(), d)
	}
	if d := square.h() - disc.h(); d < -3 || d > 3 {
		t.Errorf("方形封面高 %d，碟片高 %d：应同为 263 左右（差 %d）", square.h(), disc.h(), d)
	}
	if square.w() > 280 {
		t.Errorf("方形封面宽 %d：仍是被 Fill() 放大到 292 的旧行为", square.w())
	}
	dcx, scx := (disc.minX+disc.maxX)/2, (square.minX+square.maxX)/2
	if d := scx - dcx; d < -2 || d > 2 {
		t.Errorf("方形封面中心 x=%d，碟片 x=%d：应同轴居中", scx, dcx)
	}
	dcy, scy := (disc.minY+disc.maxY)/2, (square.minY+square.maxY)/2
	if d := scy - dcy; d < -3 || d > 3 {
		t.Errorf("方形封面中心 y=%d，碟片 y=%d：应同轴居中（.cover-shell 是 "+
			"place-items:center，之前碟片只居中了横轴、比方块高 9px）", scy, dcy)
	}
	// 方块是大圆角方块：顶行深色像素远多于圆（圆的顶行只有中间一小段）。
	if square.topRun < square.w()*3/4 {
		t.Errorf("方形封面顶行深色宽度 %d / %d：不像是圆角方块", square.topRun, square.w())
	}
	if disc.topRun > disc.w()/3 {
		t.Errorf("碟片顶行深色宽度 %d / %d：不像是圆形", disc.topRun, disc.w())
	}
}

// TestCoverRingStyle 钉住玻璃环（.cover-shell::before）的样式取值——
// 直接抄原版 globals.css 的 .cover-shell::before 规则：
//
//	inset 3% → 292 × 0.94 ≈ 274；border 1px rgba(255,255,255,.2)；
//	box-shadow 0 8px 24px rgba(15,23,42,.12)；border-radius 50%；
//	background none（透明，不遮挡下面的封面）。
//
// 源码 flatten 手法：读 home.go 去掉空白后逐字匹配 ring 的创建链。
func TestCoverRingStyle(t *testing.T) {
	flat := flattenCover(t)
	// ring 的创建链：Size(side*0.94) + Radius(RadiusPill) + Border + Shadow + Center。
	chain := strings.Join(strings.Fields(
		`ring:=ui.Box(c).Size(side*0.94,side*0.94).Radius(RadiusPill).`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("ring 尺寸/圆角不对：找不到 %q", chain)
	}
	borderChain := strings.Join(strings.Fields(
		`Border(1,ui.Hex("#ffffff").Alpha(0.20)).`), "")
	if !strings.Contains(flat, borderChain) {
		t.Errorf("ring 边框不对：找不到 %q", borderChain)
	}
	shadowChain := strings.Join(strings.Fields(
		`Shadow(0,8,24,0,ui.Hex("#0f172a").Alpha(0.12)).`), "")
	if !strings.Contains(flat, shadowChain) {
		t.Errorf("ring 投影不对：找不到 %q", shadowChain)
	}
}

// TestCoverGroupOpacityTransition 钉住封面三键组的 opacity 过渡接线。
// 原版 `.cover-shell:hover .cover-mode-toggle { opacity: 0.85 }` 继承
// `.cover-mode-toggle { transition: opacity 0.2s ease }` —— 背景色切换有 0.2s 过渡。
// mygo 用 group.AnimateTo 给整组 opacity 加 200ms 过渡。
func TestCoverGroupOpacityTransition(t *testing.T) {
	flat := flattenCover(t)
	// group 的 opacity 由 AnimateTo 控制，target 在 shellHovered 时 0.85。
	chain := strings.Join(strings.Fields(
		`group.Opacity(group.AnimateTo("cover-toggles",target,`), "")
	if !strings.Contains(flat, chain) {
		t.Errorf("group opacity 没有用 AnimateTo 加过渡：找不到 %q", chain)
	}
	// target 在 shellHovered 时 0.85。
	targetChain := strings.Join(strings.Fields(
		`ifshellHovered{target=0.85}`), "")
	if !strings.Contains(flat, targetChain) {
		t.Errorf("shell hover 时 target 不是 0.85：找不到 %q", targetChain)
	}
}

// TestCoverBitmapPrefetch 钉住封面「先预载、后换源」行为。
// 原版 videoCover.tsx 用 shownCover 状态：新封面加载完成前继续显示旧封面。
// mygo 用 coverBitmap() 返回上一张（shownCoverBmp），没旧图时返回 nil。
func TestCoverBitmapPrefetch(t *testing.T) {
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	// 没有封面时返回 nil（不显示任何东西）。
	if a.coverBitmap() != nil {
		t.Error("无封面时 coverBitmap 应返回 nil")
	}
	// 有封面时返回位图。
	uri := "https://example.com/cover.jpg"
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP", Cover: uri}
	a.shownCoverBmp = nil // 清除旧图
	// 预载：coverBitmap 没图时返回 nil。
	if a.coverBitmap() != nil {
		t.Error("预载前 coverBitmap 应返回 nil")
	}
	// 模拟封面加载完成：直接设置 shownCoverBmp。
	// 注意：这里不测图片加载（异步），只测「有旧图时沿用旧图」的逻辑。
	a.shownCoverBmp = &ui.Bitmap{} // 占位
	if a.coverBitmap() == nil {
		t.Error("有旧图时 coverBitmap 应返回旧图")
	}
}

// TestCoverPlayingGlow 是无头渲染实测：播放中唱片下方应出现 #0ea5e9 的蓝色光晕
// （原版 #video-cover.is-playing 的 0 12px 36px rgba(14,165,233,.16)）。
// 窗口背景是浅蓝调（B-R ≈ 13），光晕会把该处推到 B-R ≥ 25。
func TestCoverPlayingGlow(t *testing.T) {
	paused := coverProbe(t, "disc", false)
	playing := coverProbe(t, "disc", true)
	box := measureCover(t, paused)
	cx := (box.minX + box.maxX) / 2
	blue := func(tst *ui.Tester, x, y int) int {
		img := tst.Image()
		i := img.PixOffset(x, y)
		return int(img.Pix[i+2]) - int(img.Pix[i])
	}
	// 唱片正下方 8px：静止时是底色 + 深色投影，播放中叠一层蓝色光晕。
	y := box.maxY + 8
	off, on := blue(paused, cx, y), blue(playing, cx, y)
	t.Logf("唱片下方 y=%d：静止 B-R=%d，播放 B-R=%d", y, off, on)
	if off > 22 {
		t.Errorf("静止时唱片下方就已偏蓝（B-R=%d）：底色本身不该这么蓝", off)
	}
	if on-off < 12 {
		t.Errorf("播放时唱片下方只比静止蓝 %d（%d→%d）：is-playing 的蓝色光晕缺失", on-off, off, on)
	}
}
