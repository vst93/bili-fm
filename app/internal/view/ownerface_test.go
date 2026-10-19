package view

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// ownerProbe 只渲染 UP 主行（.video-info-head），四周留 20px 边距、固定 noon
// 时段主题，便于按几何取像素。OwnerFace 留空（无网络图），头像本体显示的是
// 封面占位底色，玻璃罩 / 描边 / 投影都能测。
func ownerProbe(t *testing.T) (*App, *ui.Tester) {
	return ownerProbeOn(t, ui.Transparent)
}

// ownerProbeOn 与 ownerProbe 相同，但可指定主区背后的底色：玻璃罩（白 .22）
// 压在浅色底上只有 1 个像素的差，测不出信号；压在深色底上才明显。
func ownerProbeOn(t *testing.T, back ui.Color) (*App, *ui.Tester) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	a.Info = &Info{Bvid: "BV1owner", OwnerMid: 1, OwnerName: "示例 UP", OwnerFace: ""}
	// 头像恒为一张深色纯色图（见 solidImageStore 的注释）：玻璃罩白 .22 压在
	// 深色上才量得出 opacity 1 → .7 的差，且与「默认头像有没有解码好」无关。
	a.Images = solidImageStore{bmp: solidBitmap(ui.RGB(24, 32, 44))}
	tst := ui.NewTester(func(c *ui.Context) {
		bg := ui.Column(c).Fill().Padding(20)
		if back.A != 0 {
			bg.Background(back)
		}
		bg.Children(func() { a.videoInfoHead(c) })
	}, 600, 160)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	return a, tst
}

// solidImageStore 让所有图片请求都命中同一张纯色位图：头像/封面这类探针要的
// 是确定的底色，而不是「网络图解码好没好」。
//
// 背景：OwnerFace 为空时 ownerFace() 退 noFaceAvatar（真实 URL），默认缓存会在
// 后台把它抓下来 —— 头像上缘那块取样点的读数于是随解码时机在「浅底 255」与
// 「默认头像的灰」之间跳，TestOwnerGlassDimsOnHover 全量跑偶发假红（基线 HEAD
// 同样复现）。
type solidImageStore struct{ bmp *ui.Bitmap }

func (s solidImageStore) Bitmap(string, int) *ui.Bitmap        { return s.bmp }
func (s solidImageStore) Get(string, int) *ui.Bitmap           { return s.bmp }
func (s solidImageStore) Touch(string, int)                    {}
func (s solidImageStore) Retain(string, int)                   {}
func (s solidImageStore) Cancel(string, int)                   {}
func (s solidImageStore) Ambient(string, int, bool) *ui.Bitmap { return nil }
func (s solidImageStore) Pending() int                         { return 0 }

// solidBitmap 造一张 8×8 纯色位图（铺成 Fill/Cover 后整块都是这个色）。
func solidBitmap(col ui.Color) *ui.Bitmap {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = col.R, col.G, col.B, 255
	}
	return ui.NewBitmap(img)
}

// TestOwnerFacePaintFor 钉住 UP 主头像的尺寸常量与两态样式。原版
// （main:src/styles/globals.css）：
//
//	#video-owner-face         44×44（!important，压过 JSX 的 40）+ 2px margin
//	                          + 1.5px 白 .7 描边
//	                          常态 box-shadow 蓝环 .18 + 墨影 .1
//	                          :hover 蓝环 .3 + 蓝影 .18 + scale(1.06)
//	.video-owner-wrap::before 48×48 玻璃罩，常态 opacity 1、:hover .7
//
// 原生版此前是 40×40、无描边/投影/玻璃罩、悬停只放大 1.05。
func TestOwnerFacePaintFor(t *testing.T) {
	if ownerFaceSize != 44 {
		t.Errorf("头像边长 = %d，应为 44（#video-owner-face 的 !important）", ownerFaceSize)
	}
	if ownerFaceMargin != 2 {
		t.Errorf("头像 margin = %d，应为 2", ownerFaceMargin)
	}
	if ownerGlassSize != 48 {
		t.Errorf("玻璃罩边长 = %d，应为 48（44 + 2×2）", ownerGlassSize)
	}
	if ownerWrapGap != 11 {
		t.Errorf("头像与名字列间距 = %d，应为 11（.video-owner-wrap 的 gap）", ownerWrapGap)
	}

	rest := ownerFacePaintFor(false)
	if rest.Scale != 1 {
		t.Errorf("常态缩放 = %v，应为 1", rest.Scale)
	}
	if len(rest.Shadows) != 2 {
		t.Fatalf("常态投影应两条，得到 %d", len(rest.Shadows))
	}
	if rest.Shadows[0] != (shadowSpec{0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.18)}) {
		t.Errorf("常态首条 = %+v，应为蓝环 .18", rest.Shadows[0])
	}
	if rest.Shadows[1] != (shadowSpec{0, 4, 12, 0, shadowInk.Alpha(0.10)}) {
		t.Errorf("常态次条 = %+v，应为墨影 0 4 12 .1", rest.Shadows[1])
	}

	hover := ownerFacePaintFor(true)
	if hover.Scale != 1.06 {
		t.Errorf("悬停缩放 = %v，应为 1.06（原版 hover:scale(1.06)）", hover.Scale)
	}
	if len(hover.Shadows) != 2 {
		t.Fatalf("悬停投影应两条，得到 %d", len(hover.Shadows))
	}
	if hover.Shadows[0] != (shadowSpec{0, 0, 0, 1, ui.Hex("#0ea5e9").Alpha(0.30)}) {
		t.Errorf("悬停首条 = %+v，应为蓝环 .3", hover.Shadows[0])
	}
	if hover.Shadows[1] != (shadowSpec{0, 8, 18, 0, ui.Hex("#0ea5e9").Alpha(0.18)}) {
		t.Errorf("悬停次条 = %+v，应为蓝影 0 8 18 .18", hover.Shadows[1])
	}

	if ownerFaceBorder() != ui.Hex("#ffffff").Alpha(0.70) {
		t.Errorf("描边色 = %v，应为白 .7", ownerFaceBorder())
	}

	if ownerGlassOpacity(false) != 1 || ownerGlassOpacity(true) != 0.7 {
		t.Errorf("玻璃罩透明度 = %v / %v，应为 1 / .7", ownerGlassOpacity(false), ownerGlassOpacity(true))
	}

	// 原版是一条 180° 三段渐变：白 .22 → 45% 白 .06 → 100% 蓝 .06。
	// 旧实现拆成两层（上半段 + 下半段），下半层的「透明起点」让白 .06 一直
	// 垫在蓝下面，底部多一层 6% 白；单层三段与 CSS 逐值一致。
	glass := ownerGlassGradient()
	want := []ui.GradientStop{
		{Color: ui.Hex("#ffffff").Alpha(0.22), At: 0},
		{Color: ui.Hex("#ffffff").Alpha(0.06), At: 0.45},
		{Color: ui.Hex("#0ea5e9").Alpha(0.06), At: 1},
	}
	if glass.Angle != 180 || len(glass.Stops) != len(want) {
		t.Fatalf("玻璃罩渐变 = %+v，应为 180° 三段", glass)
	}
	for i := range want {
		if glass.Stops[i] != want[i] {
			t.Errorf("玻璃罩停靠点 %d = %+v，应为 %+v", i, glass.Stops[i], want[i])
		}
	}
}

// TestOwnerFaceRenderedSize 是整块无头实测：头像本体必须渲染成 44×44。
// Find 命中的是主区头像（名字按钮的标签同名，但头像先构建、先登记）。
func TestOwnerFaceRenderedSize(t *testing.T) {
	_, tst := ownerProbe(t)
	r, ok := tst.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 主头像")
	}
	if int(r.W+0.5) != ownerFaceSize || int(r.H+0.5) != ownerFaceSize {
		t.Fatalf("头像实测 %v×%v，应为 %d×%d", r.W, r.H, ownerFaceSize, ownerFaceSize)
	}
	t.Logf("头像矩形 = %v", r)
}

// TestOwnerFaceHoverShadow 是整块无头实测：常态头像下方是墨色投影（B-R 小），
// 悬停换成蓝调投影（B-R 明显变大）。原版 #video-owner-face 的两态 box-shadow。
func TestOwnerFaceHoverShadow(t *testing.T) {
	_, tst := ownerProbe(t)
	r, ok := tst.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 主头像")
	}
	px, py := int(r.X+r.W/2), int(r.Y+r.H+8)
	rest := rgbAt(tst.Image(), px, py)
	tst.Move(r.X+r.W/2, r.Y+r.H/2)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	hover := rgbAt(tst.Image(), px, py)
	t.Logf("头像下方 8px：静息=%v 悬停=%v", rest, hover)

	restBR, hoverBR := rest[2]-rest[0], hover[2]-hover[0]
	if restBR >= 12 {
		t.Errorf("静息投影应是墨色（B-R 小），得到 %v (B-R=%d)", rest, restBR)
	}
	if hoverBR-restBR < 12 {
		t.Errorf("悬停投影应转蓝：B-R %d → %d", restBR, hoverBR)
	}
}

// TestOwnerGlassDimsOnHover 是整块无头实测：头像上的磨砂玻璃罩常态 opacity 1、
// 悬停降到 .7，于是头像上缘那块被玻璃提亮的像素在悬停时会变暗。深色底是为了
// 放大玻璃罩的信号（浅色底上白 .22 只有 1 个像素的差）。
func TestOwnerGlassDimsOnHover(t *testing.T) {
	_, tst := ownerProbeOn(t, ui.Hex("#0f172a"))
	r, ok := tst.Find("UP 主空间")
	if !ok {
		t.Fatal("找不到 UP 主头像")
	}
	// 取头像上缘下方 3px（越过 1.5px 描边）、水平居中：这里玻璃罩的白色最强
	// （渐变 0% 处的白 .22），悬停时透明度降到 .7，应当变暗。
	//
	// 取样点必须落在**头像图**上：头像本体的 `CoverPlaceholder()` 是浅底，
	// 白 .22 叠上去直接饱和到 255（两态读数都是 [255 255 255]，此前只有默认
	// 头像图恰好解码完成时才偶发通过）。所以 ownerProbeOn 用 solidImageStore
	// 给头像喂一张确定的深色图，不依赖网络/解码时机。
	px, py := int(r.X+r.W/2), int(r.Y+3)
	rest := rgbAt(tst.Image(), px, py)
	tst.Move(r.X+r.W/2, r.Y+r.H/2)
	tst.Advance(400 * time.Millisecond)
	tst.Frame()
	hover := rgbAt(tst.Image(), px, py)
	t.Logf("头像上缘玻璃：静息=%v 悬停=%v", rest, hover)

	lum := func(c [3]int) int { return c[0] + c[1] + c[2] }
	if lum(rest)-lum(hover) < 12 {
		t.Errorf("悬停时玻璃罩应明显更透明、像素变暗：静息=%v 悬停=%v", rest, hover)
	}
}

// legacyOwnerGlassOverlay 是旧的两层画法（上半段 + 下半段），只为反证保留。
func legacyOwnerGlassOverlay(c *ui.Context, hovered bool) {
	op := ownerGlassOpacity(hovered)
	top := ui.Box(c).Absolute().Left(0).Top(0).
		Size(ownerGlassSize, ownerGlassSize).Radius(RadiusPill).PassThrough()
	top.Opacity(op)
	top.LinearGradient(ui.LinearGradient{
		From: ui.Hex("#ffffff").Alpha(0.22), To: ui.Hex("#ffffff").Alpha(0.06),
		Angle: 180, Start: 0, End: 0.45,
	})
	bottom := ui.Box(c).Absolute().Left(0).Top(0).
		Size(ownerGlassSize, ownerGlassSize).Radius(RadiusPill).PassThrough()
	bottom.Opacity(op)
	bottom.LinearGradient(ui.LinearGradient{
		From: ui.Transparent, To: ui.Hex("#0ea5e9").Alpha(0.06),
		Angle: 180, Start: 0.45, End: 1,
	})
	bottom.InsetShadow(0, 1, 2, 0, ui.Hex("#ffffff").Alpha(0.60))
	bottom.InsetShadow(0, -1, 2, 0, shadowInk.Alpha(0.08))
}

// TestOwnerGlassBottomHasNoExtraWhite 无头实测玻璃罩底部不再多一层 6% 白。
//
// 原版是一条 180° 三段渐变（白 .22 → 45% 白 .06 → 100% 蓝 .06）。旧实现拆成
// 两层：上半层在 45% 之后是**实心白 .06**（End=.45），下半层从 .45 起由透明
// 渐变到蓝 .06 —— 于是底部同时有白 .06 与蓝 .06，比原版多一层 6% 白，在深色
// 封面上是一层可见的奶雾。压深色底取样，底部亮度应比旧画法低。
func TestOwnerGlassBottomHasNoExtraWhite(t *testing.T) {
	measure := func(overlay func(*ui.Context, bool)) float32 {
		orig := ownerGlassOverlay
		defer func() { ownerGlassOverlay = orig }()
		ownerGlassOverlay = overlay

		_, tst := ownerProbeOn(t, ui.Hex("#101820"))
		face, ok := tst.Find("UP 主空间")
		if !ok {
			t.Fatal("找不到 UP 主头像")
		}
		img := tst.Image()
		cx := int(face.X + face.W/2)
		cy := int(face.Y + face.H)
		var sum float32
		var n int
		// 头像（44，margin 2）压在玻璃罩（48）里，四周留出 2px 玻璃环：
		// 环上只有玻璃渐变，压在深色底上，正是旧画法多出的那层白最显形的地方。
		for y := cy; y < cy+2; y++ {
			for x := cx - 5; x <= cx+5; x++ {
				i := img.PixOffset(x, y)
				sum += float32(int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2])) / 3
				n++
			}
		}
		return sum / float32(n)
	}

	newL := measure(ownerGlassOverlay)
	oldL := measure(legacyOwnerGlassOverlay)
	t.Logf("玻璃罩底部平均亮度 单层三段=%.1f 旧两层=%.1f", newL, oldL)
	if oldL-newL < 3 {
		t.Errorf("旧两层只比单层亮 %.1f，反证不足（应 ≥3：多出的那层白 .06）", oldL-newL)
	}
}

// TestOwnerGlassFadeIsAnimated 无头实测玻璃罩悬停是**渐变**而不是硬切：
// 原版 `.video-owner-wrap::before { transition: opacity 0.2s ease }`。
//
// 压在深色底上量玻璃环（头像四周 2px）的亮度：常态 opacity 1 最亮；
// 悬停目标 .7 更暗。刚移上去那一帧应几乎没变（过渡从 1 开始），推进 250ms
// 后才落到暗处。原生版此前直接 set 目标值，移上去第一帧就已经变暗（反证）。
func TestOwnerGlassFadeIsAnimated(t *testing.T) {
	if ownerGlassFadeAnim.Duration != 200*time.Millisecond {
		t.Fatalf("淡出时长 = %v，应为 200ms（原版 opacity .2s）", ownerGlassFadeAnim.Duration)
	}

	_, tst := ownerProbeOn(t, ui.Hex("#101820"))
	ringLum := func() float32 {
		face, ok := tst.Find("UP 主空间")
		if !ok {
			t.Fatal("找不到 UP 主头像")
		}
		img := tst.Image()
		cx := int(face.X + face.W/2)
		cy := int(face.Y + face.H)
		var sum float32
		var n int
		for y := cy; y < cy+2; y++ {
			for x := cx - 7; x <= cx+7; x++ {
				i := img.PixOffset(x, y)
				sum += float32(int(img.Pix[i])+int(img.Pix[i+1])+int(img.Pix[i+2])) / 3
				n++
			}
		}
		return sum / float32(n)
	}
	rest := ringLum()
	// 悬停在**名字**上（仍在 .video-owner-wrap 里）：玻璃罩会淡出，但头像
	// 不会 scale(1.06) 挡住那圈 2px 玻璃环。
	name, ok := tst.Find("示例 UP")
	if !ok {
		t.Fatal("找不到 UP 主名字")
	}
	tst.Move(name.X+name.W/2, name.Y+name.H/2)
	tst.Frame()
	fresh := ringLum()
	tst.Advance(250 * time.Millisecond)
	tst.Frame()
	faded := ringLum()
	t.Logf("玻璃环亮度 常态=%.1f 悬停首帧=%.1f 250ms 后=%.1f", rest, fresh, faded)

	if rest-fresh > 1.5 {
		t.Errorf("悬停首帧就暗了 %.1f —— 过渡没接上（硬切）", rest-fresh)
	}
	if rest-faded < 2 {
		t.Errorf("250ms 后只暗了 %.1f —— 悬停终态 .7 没落到玻璃罩上", rest-faded)
	}
}
