package view

import (
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

	top := ownerGlassTopGradient()
	if top.From != ui.Hex("#ffffff").Alpha(0.22) || top.To != ui.Hex("#ffffff").Alpha(0.06) ||
		top.Angle != 180 || top.End != 0.45 {
		t.Errorf("玻璃罩上半段 = %+v，应为 180° 白 .22 → 45%% 白 .06", top)
	}
	bottom := ownerGlassBottomGradient()
	if bottom.From.A != 0 || bottom.To != ui.Hex("#0ea5e9").Alpha(0.06) ||
		bottom.Angle != 180 || bottom.Start != 0.45 || bottom.End != 1 {
		t.Errorf("玻璃罩下半段 = %+v，应为 180° 透明 → 100%% 蓝 .06（45%% 起）", bottom)
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
