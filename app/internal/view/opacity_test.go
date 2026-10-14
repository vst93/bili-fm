package view

import (
	"image"
	"testing"

	"github.com/egoist/mygo/ui"
)

// diffInRect 统计两张同尺寸帧在矩形 r 内颜色有出入的像素数。
func diffInRect(a, b *image.RGBA, r ui.Rect) (diff, total int) {
	for y := int(r.Y); y < int(r.Y+r.H); y++ {
		for x := int(r.X); x < int(r.X+r.W); x++ {
			total++
			i := a.PixOffset(x, y)
			if a.Pix[i] != b.Pix[i] || a.Pix[i+1] != b.Pix[i+1] || a.Pix[i+2] != b.Pix[i+2] {
				diff++
			}
		}
	}
	return diff, total
}

// TestNavIconOpacityTokens 钉住本轮补齐的几组透明度取值（原版 globals.css）：
//
//	.nav-icon-btn                                .92
//	.video-content-actions .nav-icon-btn         .82
//	.video-context-dock .nav-icon-btn            .78
//	.nav-icon-btn:disabled                       .34
//	.video-owner-picker-button 悬停/展开回到 1
//	#player .player-button:disabled svg          .34
func TestNavIconOpacityTokens(t *testing.T) {
	for _, tc := range []struct {
		kind navIconKind
		want float32
	}{
		{navIconGeneric, 0.92},
		{navIconContent, 0.82},
		{navIconDock, 0.78},
	} {
		if got := navIconRestOpacityFor(tc.kind); got != tc.want {
			t.Errorf("navIconRestOpacityFor(%v) = %v，应为 %v", tc.kind, got, tc.want)
		}
	}

	// 计数条随内容行基础透明度 / 禁用（.34）一起淡出。
	if got := navIconStatValueOpacity(false); got != 0.82 {
		t.Errorf("计数常态透明度 = %v，应为 .82", got)
	}
	if got := navIconStatValueOpacity(true); got != navIconDisabledOpacity {
		t.Errorf("计数禁用透明度 = %v，应为 %v（原版 .nav-icon-btn:disabled）",
			got, navIconDisabledOpacity)
	}

	// 合作 UP 主选择键：通用 .92，悬停或展开（aria-expanded）回到 1。
	for _, tc := range []struct {
		expanded, hovered bool
		want              float32
	}{
		{false, false, 0.92},
		{true, false, 1},
		{false, true, 1},
		{true, true, 1},
	} {
		if got := collabTriggerOpacity(tc.expanded, tc.hovered); got != tc.want {
			t.Errorf("collabTriggerOpacity(%v,%v) = %v，应为 %v",
				tc.expanded, tc.hovered, got, tc.want)
		}
	}

	// 播放栏禁用图标在按钮 .42 之上再压到 .34（合 .42×.34≈.14）。
	if got := playerDisabledIconOpacityFor(false); got != 1 {
		t.Errorf("非禁用播放图标透明度 = %v，应为 1", got)
	}
	if got := playerDisabledIconOpacityFor(true); got != 0.34 {
		t.Errorf("禁用播放图标透明度 = %v，应为 .34", got)
	}
}

// TestNavIconRestOpacityWired 无头实测：内容操作行 / 工具条的按钮真的吃到了
// 原版那条族级基础透明度 —— 把 navIconRestOpacityFor 临时换成直通（1.0）后，
// 同一按钮矩形里大量像素跟着变；反证这层淡出确实接在按钮上（去掉调用则 0 变化）。
func TestNavIconRestOpacityWired(t *testing.T) {
	cases := []struct {
		name  string
		label string
		setup func(a *App)
	}{
		{"内容操作行", "浏览器打开", func(a *App) {
			a.Info = &Info{Bvid: "BV1op", Aid: 1, Cid: 2, Title: "标题"}
			a.Track = &Track{Bvid: "BV1op", Cid: 2, Title: "标题"}
			a.Queue = []Track{*a.Track}
			a.Index = 0
		}},
		{"工具条", "播放列表", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			render := func() (*image.RGBA, ui.Rect, bool) {
				a := NewApp(func() {})
				a.Theme = ThemeFor(PeriodAt(12))
				if tc.setup != nil {
					tc.setup(a)
				}
				tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
				tt.Frame()
				tt.Frame()
				r, ok := tt.Find(tc.label)
				return tt.Image(), r, ok
			}

			on, rect, ok := render()
			if !ok {
				t.Fatalf("找不到按钮 %q", tc.label)
			}
			saved := navIconRestOpacityFor
			navIconRestOpacityFor = func(navIconKind) float32 { return 1 }
			defer func() { navIconRestOpacityFor = saved }()
			off, _, _ := render()

			diff, total := diffInRect(on, off, rect)
			t.Logf("%s：%d/%d 像素随族级基础透明度变化", tc.label, diff, total)
			if diff < total/10 {
				t.Errorf("%s 只有 %d/%d 像素变化（<10%%），基础透明度没接到按钮上？",
					tc.label, diff, total)
			}
		})
	}
}

// TestStatValueOpacityWired 无头实测：没有视频时（按钮禁用）计数「0」应和按钮
// 一起淡出 —— 计数是按钮的绝对定位子元素（画在按钮下缘之外），这层透明度仍要
// 单独补。把 navIconStatValueOpacity 临时换成直通（1.0）后，按钮正下方 13px
// 计数条里应有明显像素变化。
func TestStatValueOpacityWired(t *testing.T) {
	render := func() (*image.RGBA, ui.Rect, bool) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		tt := ui.NewTester(func(c *ui.Context) { a.Shell(c) }, 800, 600)
		tt.Frame()
		tt.Frame()
		r, ok := tt.Find("点赞")
		return tt.Image(), r, ok
	}

	on, btn, ok := render()
	if !ok {
		t.Fatal("找不到点赞键")
	}
	// 原版数值挂在按钮正下方 `top: calc(100% + 1px)`、12px 行高。
	strip := ui.Rect{X: btn.X, Y: btn.Y + btn.H, W: btn.W, H: 13}

	saved := navIconStatValueOpacity
	navIconStatValueOpacity = func(bool) float32 { return 1 }
	defer func() { navIconStatValueOpacity = saved }()
	off, _, _ := render()

	diff, total := diffInRect(on, off, strip)
	t.Logf("计数条：%d/%d 像素随透明度变化", diff, total)
	if diff < 5 {
		t.Errorf("计数条只有 %d/%d 像素变化，计数没吃到禁用淡出？", diff, total)
	}
}

// TestPlayerDisabledIconOpacityWired 无头实测：没有媒体时（整排禁用）播放栏的
// svg 图标要在按钮 .42 之上再压到 .34（合 ≈.14）。把
// playerDisabledIconOpacityFor 换成直通后，禁用播放键矩形内应大量像素
// 变亮；有媒体（非禁用）时该开关不该有任何影响。
func TestPlayerDisabledIconOpacityWired(t *testing.T) {
	render := func(mini, withSource bool) (*image.RGBA, ui.Rect, bool) {
		a := NewApp(func() {})
		a.Theme = ThemeFor(PeriodAt(12))
		a.Mini = mini
		if withSource {
			a.Track = &Track{Bvid: "BV1", Cid: 1, Title: "标题"}
			a.Queue = []Track{*a.Track}
			a.Index = 0
		}
		// 只渲染播放栏（不渲染主区封面圆盘，它也叫「播放」且会自转）。
		build := func(c *ui.Context) { a.playerBar(c) }
		w, h := 800, 200
		if mini {
			build = func(c *ui.Context) { a.miniPlayerBar(c) }
			w, h = miniWidth, miniHeight
		}
		tt := ui.NewTester(build, w, h)
		tt.Frame()
		tt.Frame()
		r, ok := tt.Find("播放")
		return tt.Image(), r, ok
	}

	saved := playerDisabledIconOpacityFor
	defer func() { playerDisabledIconOpacityFor = saved }()

	for _, mini := range []bool{false, true} {
		name := "主窗"
		if mini {
			name = "迷你窗"
		}
		t.Run(name, func(t *testing.T) {
			// 直通 vs 真实：只有禁用时（无媒体）图标才该有差异。
			playerDisabledIconOpacityFor = func(bool) float32 { return 1 }
			stub, rect, ok := render(mini, false)
			if !ok {
				t.Fatal("找不到禁用的播放键")
			}
			playerDisabledIconOpacityFor = func(off bool) float32 {
				if off {
					return 0.34
				}
				return 1
			}
			real, _, _ := render(mini, false)

			diff, total := diffInRect(stub, real, rect)
			t.Logf("禁用播放键（%s）：%d/%d 像素随 svg 压暗变化", name, diff, total)
			if diff < 5 {
				t.Errorf("禁用播放键只有 %d/%d 像素变化，svg 没被额外压暗？", diff, total)
			}

			// 有媒体（非禁用）：两个 helper 都应返回 1，帧应一致。
			onA, rectA, _ := render(mini, true)
			playerDisabledIconOpacityFor = func(bool) float32 { return 1 }
			onB, _, _ := render(mini, true)
			if d, _ := diffInRect(onA, onB, rectA); d != 0 {
				t.Errorf("有媒体时播放图标不应受禁用压暗影响，却有 %d 像素变化", d)
			}
			playerDisabledIconOpacityFor = func(off bool) float32 {
				if off {
					return 0.34
				}
				return 1
			}
		})
	}
}
