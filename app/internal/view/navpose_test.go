package view

import (
	"image"
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// inkDarkLimit 是「算是图标正文」的亮度上限。普通图标是 #475569（亮度约 83），
// 禁用态按 .3 压到约 203，所以阈值要放到 235 既能盖住禁用图标、又排得掉悬停
// 投影（约 244）。
const inkDarkLimit = 235

// inkTop 在一张白底帧的矩形里找最上面那行「图标正文」像素，返回其 y（找不到 -1）。
// 用于量图标是否整体上浮 —— 悬停高光/投影都很浅（超过阈值被排除在外），只有
// 图标本体会被算进去。矩形只盖按钮本体，数值/计数在按钮之外不干扰。
func inkTop(img *image.RGBA, r ui.Rect) int {
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			i := img.PixOffset(x, y)
			if lum := (299*int(img.Pix[i]) + 587*int(img.Pix[i+1]) + 114*int(img.Pix[i+2])) / 1000; lum < inkDarkLimit {
				return y
			}
		}
	}
	return -1
}

// TestNavIconPoseTokens 钉住本轮补齐的 `.nav-icon-btn` 位移手感（原版 globals.css）：
//
//	transition: ... transform 0.16s ease（CSS 默认缓动 cubic-bezier(.25,.1,.25,1)）
//	:hover           translateY(-1px)
//	:active          scale(.94) —— 源序在 :hover 之后，整条替换 ⇒ 按下只有缩放
func TestNavIconPoseTokens(t *testing.T) {
	if navIconAnim.Duration != 160*time.Millisecond {
		t.Errorf("位移时长 = %v, want 160ms（原版 transform 0.16s）", navIconAnim.Duration)
	}
	if navIconAnim.Spring != (ui.SpringOptions{}) {
		t.Errorf("应为曲线动画，不是弹簧：%+v", navIconAnim.Spring)
	}
	if navIconAnim.Ease == nil {
		t.Fatal("缺少缓动曲线")
	}
	want := ui.CubicBezier(0.25, 0.1, 0.25, 1)
	for _, x := range []float32{0.1, 0.3, 0.5, 0.8, 0.95} {
		if d := math.Abs(float64(navIconAnim.Ease(x) - want(x))); d > 1e-6 {
			t.Errorf("缓动在 %.2f 处 = %v, want %v", x, navIconAnim.Ease(x), want(x))
		}
	}

	for _, tc := range []struct {
		pressed, hovered bool
		want             ui.Pose
	}{
		{false, false, ui.Pose{}},
		{false, true, ui.Pose{Y: -1}},
		{true, false, ui.Pose{Scale: 0.94}},
		// 按下时鼠标仍在按钮上：原版 :active 覆盖 :hover 的 transform
		// （整条替换），所以是缩放而不是「上浮 + 缩放」。
		{true, true, ui.Pose{Scale: 0.94}},
	} {
		if got := navIconPoseFor(tc.pressed, tc.hovered); got != tc.want {
			t.Errorf("navIconPoseFor(%v,%v) = %+v, want %+v", tc.pressed, tc.hovered, got, tc.want)
		}
	}
}

// navPoseSubject 造一个能悬停/按下的最小场景：白底上只放一行要被测的按钮族，
// 这样帧里的深色像素就是图标本体（背景是白的、玻璃高光也是白的）。
type navPoseSubject struct {
	name  string
	label string
	build func(a *App, c *ui.Context)
	setup func(a *App)
}

func navPoseSubjects() []navPoseSubject {
	return []navPoseSubject{
		{
			name:  "内容操作行",
			label: "浏览器打开",
			setup: func(a *App) {
				a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
				a.Track = &Track{Bvid: "BV1", Cid: 2, Title: "标题"}
				a.Queue = []Track{*a.Track}
				a.Index = 0
			},
			build: func(a *App, c *ui.Context) { a.contentActions(c) },
		},
		{
			name:  "工具条",
			label: "播放列表",
			setup: func(a *App) {
				a.Info = &Info{Bvid: "BV1", Aid: 1, Cid: 2, Title: "标题"}
			},
			build: func(a *App, c *ui.Context) { a.contextDock(c) },
		},
	}
}

// renderNavPose 起一个 420×140 的无头窗口，白底只画被测按钮族；hover/press 二选一。
func renderNavPose(t *testing.T, s navPoseSubject, hover bool) (*image.RGBA, ui.Rect) {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = ThemeFor(PeriodAt(12))
	if s.setup != nil {
		s.setup(a)
	}
	tt := ui.NewTester(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#ffffff"))
		ui.Column(c).Padding(20).Children(func() { s.build(a, c) })
	}, 420, 140)
	tt.Frame()
	r, ok := tt.Find(s.label)
	if !ok {
		t.Fatalf("找不到按钮 %q", s.label)
	}
	if hover {
		tt.Move(r.X+r.W/2, r.Y+r.H/2)
	}
	tt.Frame()
	return tt.Image(), r
}

// TestNavIconHoverLiftWired 无头实测：两族 `.nav-icon-btn` 悬停时图标整体上浮
// 1px（此前原生版只有底色变化、没有位移）。用「最上一行深色像素」量图标位置：
// 静止 = 悬停但把 navIconPose 换成 no-op（同一背景、同一位置），悬停真实态应
// 恰好高 1px；换成 no-op 后与静止态完全一致 —— 反证这 1px 来自 navIconPose。
func TestNavIconHoverLiftWired(t *testing.T) {
	freezeNavIconFades(t)
	saved := navIconPose
	defer func() { navIconPose = saved }()

	for _, s := range navPoseSubjects() {
		t.Run(s.name, func(t *testing.T) {
			navIconPose = saved
			rest, rect := renderNavPose(t, s, false)

			// 悬停 + 真实位移。
			posed, _ := renderNavPose(t, s, true)

			// 悬停 + 位移关掉（只保留悬停的底色/高光）。
			navIconPose = func(ui.Element) {}
			stub, _ := renderNavPose(t, s, true)

			restTop, posedTop, stubTop := inkTop(rest, rect), inkTop(posed, rect), inkTop(stub, rect)
			t.Logf("%s：静止 top=%d、悬停 top=%d、悬停(无位移) top=%d", s.name, restTop, posedTop, stubTop)
			if restTop < 0 || posedTop < 0 || stubTop < 0 {
				t.Fatalf("矩形里量不到深色图标像素：rest=%d posed=%d stub=%d", restTop, posedTop, stubTop)
			}
			if stubTop != restTop {
				t.Errorf("关掉 navIconPose 后悬停位置 = %d，应与静止 %d 相同", stubTop, restTop)
			}
			if posedTop != restTop-1 {
				t.Errorf("悬停图标 top = %d，应为 %d（上浮 1px）", posedTop, restTop-1)
			}
		})
	}
}

// TestNavIconDisabledNoLift 无头实测：禁用按钮悬停不位移
// （原版 `.nav-icon-btn:disabled:hover { transform: none }`）。没有视频时
// 「浏览器打开」是禁用态，悬停它图标位置不该动。
func TestNavIconDisabledNoLift(t *testing.T) {
	s := navPoseSubject{
		name:  "禁用内容键",
		label: "浏览器打开",
		// 没有 Info → bvid == ""，三个内容图标键都禁用。
		build: func(a *App, c *ui.Context) { a.contentActions(c) },
	}
	rest, rect := renderNavPose(t, s, false)
	hovered, _ := renderNavPose(t, s, true)

	restTop, hoverTop := inkTop(rest, rect), inkTop(hovered, rect)
	t.Logf("禁用键：静止 top=%d、悬停 top=%d", restTop, hoverTop)
	if restTop < 0 || hoverTop < 0 {
		t.Fatalf("量不到禁用图标：rest=%d hover=%d", restTop, hoverTop)
	}
	if hoverTop != restTop {
		t.Errorf("禁用按钮悬停位移了（top %d → %d），应保持不动", restTop, hoverTop)
	}
}

// TestNavIconPressPoseTokens 钉住按下改走缩放、以及 `navIconPose` 读的是
// `Pressed()` 而不是 `Hovered()`（两者同帧都为真时取缩放）。
func TestNavIconPressPoseTokens(t *testing.T) {
	if got := navIconPoseFor(true, true); got != (ui.Pose{Scale: 0.94}) {
		t.Errorf("按下+悬停 = %+v, want scale .94", got)
	}
}
