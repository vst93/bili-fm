package view

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 「减少透明度」下播放栏必须**看得见**。
//
// 用户 2026-11-11（Windows，关掉了系统的「透明效果」）：
// 「底部的播放器栏的按钮完全看不见颜色，背景颜色看起来似乎也有点不太对劲」。
//
// 根因不在播放栏本身，而在它上面那层滚动边缘效果：`glass.ScrollEdge` 在
// `Preferences().ReduceTransparency` 下会把 `Background` 的 alpha 拉成 255
// 并 `FillRadii` 铺满**整块**（mygo 的 plugins/glass/edge.go）。那块是
// 「栏高 + 10」、绝对定位、在绘制顺序上排在播放栏**之后** —— 于是整条栏
// （含所有按钮）被刷成一块不透明色。实测被刷成 `#475569`（`PlayerEdgeFade`
// 的 RGB 配满 alpha），按钮完全不可见。
//
// 修法：这个分支改回**自绘真正的渐变**（只在顶端 PlayerBarPad 内完成，
// 下面全透明），效果比框架的背景采样弱，但不会遮住任何东西。
//
// 下面用「播放栏区域的颜色方差」做判据：被单色盖住时方差≈0，正常时按钮、
// 进度条、文字都会让方差明显大于 0。

// rtShellProbe 在给定偏好下渲染整窗，返回图像。
func rtShellProbe(t *testing.T, rt bool) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.SetReduceTransparency(rt)
	a.SetAppearance("light", 0)
	tt := ui.NewTesterWith(func(c *ui.Context) { a.Shell(c) },
		800, 600, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: rt}})
	tt.Frame()
	tt.Advance(300 * time.Millisecond)
	return tt
}

// barSpread 返回播放栏区域（含按钮的那一条）里像素亮度的极差。
func barSpread(tt *ui.Tester) (minL, maxL int) {
	img := tt.Image()
	b := img.Bounds()
	minL, maxL = 255, -1
	// 播放栏 56px 贴底；只扫下半部分，避开上沿那条渐变。
	y0 := b.Dy() - PlayerBarHeight + 8
	for y := y0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			o := img.PixOffset(x, y)
			l := (int(img.Pix[o]) + int(img.Pix[o+1]) + int(img.Pix[o+2])) / 3
			if l < minL {
				minL = l
			}
			if l > maxL {
				maxL = l
			}
		}
	}
	return minL, maxL
}

// TestPlayerBarVisibleUnderReducedTransparency 钉住播放栏在「减少透明度」下
// 仍有可见的内容对比（按钮 / 进度条 / 文字都在）。
func TestPlayerBarVisibleUnderReducedTransparency(t *testing.T) {
	normal := rtShellProbe(t, false)
	reduced := rtShellProbe(t, true)

	nMin, nMax := barSpread(normal)
	rMin, rMax := barSpread(reduced)
	t.Logf("播放栏亮度极差：常态 %d..%d（%d），减少透明度 %d..%d（%d）",
		nMin, nMax, nMax-nMin, rMin, rMax, rMax-rMin)

	// 被一块不透明色盖住时极差会掉到个位数（实测被刷成单色 #475569）。
	const want = 24
	if rMax-rMin < want {
		t.Fatalf("减少透明度下播放栏亮度极差只有 %d（%d..%d）—— 整条被一层实色盖住了，"+
			"按钮完全看不见（用户 2026-11-11 报的正是这个）",
			rMax-rMin, rMin, rMax)
	}
	// 常态本来就更丰富；这里只确保减少透明度没有把对比压到常态的零头。
	if nMax-nMin > 0 && (rMax-rMin)*4 < nMax-nMin {
		t.Errorf("减少透明度下播放栏对比 %d 远低于常态 %d", rMax-rMin, nMax-nMin)
	}
}

// TestPlayerBarEdgeNotOpaqueUnderReducedTransparency 是上一条的**逐层**判据：
// 边缘效果那一层在减少透明度下不许把整块刷成不透明。
//
// 直接量「栏高 + 10」那块的底部（PlayerBarPad 之下的 46px）：自绘渐变只在
// 顶端 10DIP 内完成，那里必须是透明的，透出底下的内容。
func TestPlayerBarEdgeNotOpaqueUnderReducedTransparency(t *testing.T) {
	a := NewApp(func() {})
	a.SetReduceTransparency(true)
	a.SetAppearance("light", 0)
	tt := ui.NewTesterWith(func(c *ui.Context) {
		c.Root().Background(ui.Hex("#00ff00")) // 底下垫纯绿，被盖住就看不到
		a.playerBar(c)
	}, 400, 100, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Frame()
	tt.Advance(200 * time.Millisecond)

	img := tt.Image()
	// 播放栏本体占底部 56px，所以 y=100-56+20 已经落在栏体里；
	// 关键是它不能是 edge-fade 的 RGB（71,85,105）铺满。
	o := img.PixOffset(200, 100-20)
	r, g, b := img.Pix[o], img.Pix[o+1], img.Pix[o+2]
	t.Logf("栏体内一点 = #%02x%02x%02x", r, g, b)
	if r == 0x47 && g == 0x55 && b == 0x69 {
		t.Fatal("播放栏被 edge-fade 的实色 (#475569) 盖住了 —— glass.ScrollEdge 在" +
			"减少透明度下会整块填不透明，必须改用自绘渐变")
	}
}

// TestReducedTransparencyHintMentionsWhereToTurnItOn 钉住外观设置里那句提示
// 真的告诉用户「去哪儿开回来」。
//
// 用户 2026-11-11 明确要求：「关闭时给用户些简单的提示」。只写「系统已开启
// 减少透明度」等于让用户自己去找 —— Windows 上那个开关在
// 「设置 › 个性化 › 颜色 › 透明效果」，提示里必须写出来。
func TestReducedTransparencyHintMentionsWhereToTurnItOn(t *testing.T) {
	a := NewApp(func() {})
	a.SetAppearance("light", .2)
	a.SetReduceTransparency(true)
	a.ShowAppearance = true
	tt := shellTester(t, a)

	found := false
	for _, text := range tt.Texts() {
		if strings.Contains(text, "透明效果") && strings.Contains(text, "个性化") {
			found = true
			t.Logf("提示：%s", text)
			break
		}
	}
	if !found {
		t.Fatalf("减少透明度下没有告诉用户去哪儿开回来；当前文本：%v", tt.Texts())
	}
}

// TestMiniPlayerBarVisibleUnderReducedTransparency 是主窗那条的迷你窗版本。
//
// 迷你播放栏同样用过 glass.ScrollEdge（同一个坑）：减少透明度下它会把
// 「栏高 + 10」整块刷成不透明，盖住切曲 / 音量键。两处现在共用
// playerBarEdgeLayer，这条防止有人只修主窗、把迷你窗落回旧写法。
func TestMiniPlayerBarVisibleUnderReducedTransparency(t *testing.T) {
	a := NewApp(func() {})
	a.Mini = true
	a.SetReduceTransparency(true)
	a.SetAppearance("light", 0)
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	w, h := MiniSize()
	tt := ui.NewTesterWith(func(c *ui.Context) { a.Shell(c) },
		w, h, ui.TesterOptions{Preferences: ui.Preferences{ReduceTransparency: true}})
	tt.Frame()
	tt.Advance(300 * time.Millisecond)

	img := tt.Image()
	b := img.Bounds()
	// 迷你播放栏在最底部 miniPlayerH=43；扫它的下半部分。
	y0 := b.Dy() - miniPlayerH + 8
	minL, maxL := 255, -1
	for y := y0; y < b.Dy(); y++ {
		for x := 0; x < b.Dx(); x++ {
			o := img.PixOffset(x, y)
			l := (int(img.Pix[o]) + int(img.Pix[o+1]) + int(img.Pix[o+2])) / 3
			if l < minL {
				minL = l
			}
			if l > maxL {
				maxL = l
			}
		}
	}
	t.Logf("迷你播放栏亮度极差 = %d（%d..%d）", maxL-minL, minL, maxL)
	if maxL-minL < 24 {
		t.Fatalf("减少透明度下迷你播放栏亮度极差只有 %d —— 整条被实色盖住了", maxL-minL)
	}
}
