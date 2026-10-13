package view

import (
	"math"
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestLinearT 钉住 linearT 与 CSS 的 linear-gradient 渐变线定义一致，并确认
// overlayT（#root::after 的 165°）就是它的 165° 取值。
func TestLinearT(t *testing.T) {
	const w, h float32 = 400, 155
	if got := linearT(w/2, h/2, w, h, 145); abs32(got-0.5) > 1e-4 {
		t.Errorf("盒中心 t = %v，应为 0.5", got)
	}
	// 145° 指向右下：左上角起点、右下角终点。
	if linearT(0, 0, w, h, 145) >= linearT(w, h, w, h, 145) {
		t.Errorf("145° 应左上→右下：%v ≥ %v",
			linearT(0, 0, w, h, 145), linearT(w, h, w, h, 145))
	}
	// 垂直/水平基准。
	if linearT(0, 0, w, h, 90) >= linearT(w, 0, w, h, 90) {
		t.Error("90° 应左→右")
	}
	if linearT(0, 0, w, h, 180) >= linearT(0, h, w, h, 180) {
		t.Error("180° 应上→下")
	}
	// 越界夹到 [0,1]。
	if linearT(-500, -500, w, h, 145) != 0 || linearT(w+500, h+500, w, h, 145) != 1 {
		t.Error("越界坐标应夹到 [0,1]")
	}
	if overlayT(10, 20, w, h) != linearT(10, 20, w, h, 165) {
		t.Error("overlayT 应等价于 165° 的 linearT")
	}
}

// TestCircleFarthest 钉住 CSS `radial-gradient(circle at …)`（默认
// farthest-corner）的半径 = 圆心到四角的最大距离。
func TestCircleFarthest(t *testing.T) {
	// 圆心在正中：半径 = 半对角。
	if got, want := circleFarthest(50, 50, 100, 100), float32(math.Hypot(50, 50)); abs32(got-want) > 1e-3 {
		t.Errorf("圆心在正中时半径 = %v，应为 %v", got, want)
	}
	// 圆心偏左上：farthest 是右下角。
	if got, want := circleFarthest(10, 20, 100, 50), float32(math.Hypot(90, 30)); abs32(got-want) > 1e-3 {
		t.Errorf("圆心 (10,20) 的 farthest = %v，应为右下角 %v", got, want)
	}
}

// TestMiniBgBloomAlpha 钉住单条径向光晕的 alpha：圆心满值、stop 处到 0、
// 随距离单调递减。
func TestMiniBgBloomAlpha(t *testing.T) {
	bl := miniBgBlooms[0]
	const w, h = 400, 155
	cx, cy := bl.cx*w, bl.cy*h
	if got := miniBgBloomAlpha(bl, cx, cy, w, h); abs32(got-bl.alpha) > 1e-4 {
		t.Errorf("圆心 alpha = %v，应为 %v", got, bl.alpha)
	}
	rad := circleFarthest(cx, cy, w, h) * bl.stop
	if got := miniBgBloomAlpha(bl, cx+rad, cy, w, h); got != 0 {
		t.Errorf("stop 半径处 alpha = %v，应为 0", got)
	}
	if got := miniBgBloomAlpha(bl, cx+rad+40, cy, w, h); got != 0 {
		t.Errorf("stop 外 alpha = %v，应为 0", got)
	}
	prev := float32(2)
	for i := 0; i <= 10; i++ {
		v := miniBgBloomAlpha(bl, cx+rad*float32(i)/10, cy, w, h)
		if v > prev {
			t.Fatalf("alpha 应随距离单调减：第 %d 步 %v > %v", i, v, prev)
		}
		prev = v
	}
}

// TestMiniBgPixels 钉住纯函数产出的背景位图：不透明；左上蓝光晕圆心明显偏蓝、
// 右下绿光晕圆心明显偏绿；正中（两团光晕都够不到）只剩线性底。
func TestMiniBgPixels(t *testing.T) {
	const w, h = 400, 155
	img := miniBgPixels(w, h)
	if img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Fatalf("尺寸 = %v，应为 %dx%d", img.Bounds(), w, h)
	}
	at := func(fx, fy float32) (r, g, b, a int) {
		i := img.PixOffset(int(fx*w), int(fy*h))
		return int(img.Pix[i+0]), int(img.Pix[i+1]), int(img.Pix[i+2]), int(img.Pix[i+3])
	}
	br, _, bb, ba := at(0.12, 0.18)
	if ba != 255 {
		t.Errorf("背景应不透明，alpha = %d", ba)
	}
	if bb-br < 20 {
		t.Errorf("左上蓝光晕圆心 B-R = %d，应明显偏蓝（≥20）", bb-br)
	}
	gr, gg, gb, _ := at(0.92, 0.72)
	if gg-gr < 10 {
		t.Errorf("右下绿光晕圆心 G-R = %d，应明显偏绿（≥10）", gg-gr)
	}
	_ = gb
	cr, _, cb, _ := at(0.5, 0.5)
	if cb-cr > 12 {
		t.Errorf("正中（两团光晕之外）B-R = %d，应接近线性底（≤12）", cb-cr)
	}
	if !(bb-br > cb-cr && gg-gr > 0) {
		t.Errorf("光晕圆心应比中性处更偏色：蓝 %d/%d", bb-br, cb-cr)
	}
}

// TestMiniBackgroundRendered 是无头渲染实测：迷你窗左上角被蓝光晕染蓝，
// 且明显比同高度的右侧背景更蓝 —— 证明 MiniShell 真的把背景位图画了出来
// （此前只画 145° 线性渐变，两点 B-R 都只有个位数）。
func TestMiniBackgroundRendered(t *testing.T) {
	tst := miniProbe(t, false)
	img := tst.Image()
	s := func(x, y int) int {
		i := img.PixOffset(x, y)
		return int(img.Pix[i+2]) - int(img.Pix[i]) // B-R
	}
	// (48,28) 是蓝光晕圆心附近（信息行上内边距、封面之上）；(330,28) 同高、
	// 离蓝光晕足够远（> stop 半径）。
	left, right := s(48, 28), s(330, 28)
	t.Logf("(48,28) B-R=%d  (330,28) B-R=%d", left, right)
	if left < 15 {
		t.Errorf("左上蓝光晕处 B-R = %d，应明显偏蓝（≥15）；背景光晕可能没画", left)
	}
	if left-right < 8 {
		t.Errorf("左侧应明显比右侧蓝：left B-R=%d right B-R=%d", left, right)
	}
}

// TestMiniPlayStyle 钉住迷你窗播放键的三色取自原版
// body.mini-mode #player .player-play-button（深色时段换成主窗深色面）。
func TestMiniPlayStyle(t *testing.T) {
	lt := ThemeFor(PeriodAt(12))
	fill, border, ink := miniPlayStyle(lt, false)
	if fill != ui.Hex("#bae6fd").Alpha(0.35) ||
		border != ui.Hex("#0284c7").Alpha(0.20) || ink != ui.Hex("#0284c7") {
		t.Errorf("常态 = %v / %v / %v", fill, border, ink)
	}
	fill, border, ink = miniPlayStyle(lt, true)
	if fill != ui.Hex("#bae6fd").Alpha(0.50) ||
		border != ui.Hex("#0284c7").Alpha(0.28) || ink != ui.Hex("#0284c7") {
		t.Errorf("悬停 = %v / %v / %v", fill, border, ink)
	}
	// 深色时段：主窗那条 `#player .player-play-button` 深色规则 (1,2,1) 源序在
	// mini 基态之后，盖过它——mini 播放键换成深色面；悬停仍按浅色值。
	dt := ThemeFor(PeriodAt(0))
	fill, border, ink = miniPlayStyle(dt, false)
	if fill != ui.Hex("#e0f2fe").Alpha(0.24) ||
		border != ui.Hex("#e0f2fe").Alpha(0.48) || ink != ui.Hex("#0369a1") {
		t.Errorf("深色常态 = %v / %v / %v", fill, border, ink)
	}
	fill, border, ink = miniPlayStyle(dt, true)
	if fill != ui.Hex("#bae6fd").Alpha(0.50) ||
		border != ui.Hex("#0284c7").Alpha(0.28) || ink != ui.Hex("#0284c7") {
		t.Errorf("深色悬停 = %v / %v / %v", fill, border, ink)
	}
}

// miniProbePlaying 建一个 400×155、处于指定播放状态的迷你窗。
func miniProbePlaying(t *testing.T, playing bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Mini = true
	a.Theme = ThemeFor(PeriodAt(12))
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Playing = playing
	tst := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, miniWidth, miniHeight)
	tst.Frame()
	return tst
}

// TestMiniPlayButtonFillRendered 是无头渲染实测：迷你窗播放键是原版那块
// 浅蓝填充（B-R 明显为正），且比同高的下一集键（仍是中性白 .22）更蓝。
func TestMiniPlayButtonFillRendered(t *testing.T) {
	img := miniProbePlaying(t, false).Image()
	br := func(x, y int) int {
		i := img.PixOffset(x, y)
		return int(img.Pix[i+2]) - int(img.Pix[i])
	}
	// (13,133)/(44,133) 都在各自按钮圆内、16px 图标之外。
	play, next := br(13, 133), br(44, 133)
	t.Logf("play B-R=%d  next B-R=%d", play, next)
	if play < 10 {
		t.Errorf("播放键填充 B-R = %d，应偏蓝（≥10）", play)
	}
	if play-next < 4 {
		t.Errorf("播放键应明显比中性白的下一集键偏蓝：play=%d next=%d", play, next)
	}
}

// TestMiniPlayButtonStateIndependent 是无头渲染实测：原版 mini 播放键的
// 底色与是否播放无关（mini 规则特异度盖过 [data-playing]），只有图标在
// play/pause 间切换。取按钮圆内四个纯填充点（避开图标与圆角边缘）比对。
func TestMiniPlayButtonStateIndependent(t *testing.T) {
	off := miniProbePlaying(t, false).Image()
	on := miniProbePlaying(t, true).Image()
	for _, p := range [][2]int{{13, 133}, {35, 133}, {24, 122}, {24, 145}} {
		i := off.PixOffset(p[0], p[1])
		j := on.PixOffset(p[0], p[1])
		for k := 0; k < 3; k++ {
			if d := int(off.Pix[i+k]) - int(on.Pix[j+k]); d < -2 || d > 2 {
				t.Errorf("填充点 %v 通道 %d：停=%d 播=%d，播放键底色不应随播放状态变化",
					p, k, off.Pix[i+k], on.Pix[j+k])
			}
		}
	}
}
