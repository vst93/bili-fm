package video

// 视频窗跟随主窗深浅色的接线测试（internal/video 侧）：
//
//   - Manager.SetTheme 把主窗的新令牌记下来并推给已经开着的原生窗外壳；
//   - 建窗时以 Manager 手里**最新**的那份为准，不用 Options 里的建窗快照
//     —— 「快照—建窗」之间主窗换了档，新窗也不能带着旧令牌开出来；
//   - 换令牌只改外壳，播放器一个动作都不发。
//
// 无头测试不建真窗口（n.win 为 nil，与 settings_test.go 同一做法）：
// Manager.SetTheme 在没有窗口时直接改字段，正好让断言落在「下一帧用哪份
// 令牌」这件事上。

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/view"
)

// TestManagerThemeFollowsWindow 钉住主题令牌从主窗到视频窗外壳的那条路。
func TestManagerThemeFollowsWindow(t *testing.T) {
	light := view.ThemeFor(view.PeriodAt(12))
	// view 的深浅色合成（withAppearance）是包内私有的：这里手工换成深色
	// 外壳实际用到的那几个令牌（Mode / Dark / Ink / Panel），与
	// TestVideoChromeFollowsThemeWhileOpen 里 withAppearance 的结果同义。
	dark := view.ThemeFor(view.PeriodAt(12))
	dark.Mode, dark.Dark = "dark", true
	dark.Ink = ui.Hex("#e2e8f0")
	dark.Muted = ui.Hex("#94a3b8")
	dark.Panel = ui.Hex("#152238")
	dark.Glass = ui.Hex("#18263a").Alpha(0.94)
	dark.Line = ui.Hex("#64748b").Alpha(0.45)

	m := New()
	// 主窗在开窗之前就切过一次主题（启动时 wireVideo 先推当前值）。
	m.SetTheme(light)
	if !m.hasTheme || m.theme.Ink != light.Ink {
		t.Fatalf("SetTheme 没有记住令牌：hasTheme=%v", m.hasTheme)
	}

	pl := &stubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	stale := dark // Options 里那份建窗快照（故意给旧的）
	var got Options
	m.openNative = func(string) (playerAPI, error) { return pl, nil }
	m.newNativeWin = func(o Options, p playerAPI) *nativeWindow {
		got = o
		return &nativeWindow{pl: p, title: o.Title, dm: &danmaku.Overlay{},
			settings: &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1}}
	}
	prompt, ok := m.tryNative(Options{Src: "http://127.0.0.1:1/x.mp4", Title: "一集", Theme: stale})
	if !ok || prompt != nil {
		t.Fatalf("原生路径没有开成：ok=%v prompt=%v", ok, prompt)
	}
	if got.Theme.Ink != stale.Ink {
		t.Fatalf("测试桩没拿到建窗快照（Options.Theme 变了）")
	}
	n := m.native
	if n == nil {
		t.Fatal("tryNative 没有登记原生窗")
	}
	if n.theme.Ink != light.Ink {
		t.Errorf("建窗用的是 Options 里的旧令牌，不是 Manager 手里的最新令牌")
	}

	// 现在「窗口开着」：渲染一帧，外壳应当用最新令牌。
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if n.chrome == nil {
		t.Fatal("视图没有建出外壳")
	}
	if n.chrome.Theme.Ink != light.Ink {
		t.Fatalf("外壳的令牌不是 Manager 下发的那份")
	}
	before := pxAt(tt.Image(), 16, 28)

	// 主窗切到深色（视频窗开着，不重建窗口）。
	beforePl := *pl
	beforeSeeks := len(pl.seeks)
	m.SetTheme(dark)
	if n.theme.Ink != dark.Ink || n.chrome.Theme.Ink != dark.Ink {
		t.Fatalf("SetTheme 没有落到原生窗 / 外壳上")
	}
	tt.Frame()
	after := pxAt(tt.Image(), 16, 28)
	if colorDist(before, after) < 40 {
		t.Errorf("切换主题后标题栏底色没变：%v → %v", before, after)
	}

	// 只换令牌：播放器一个动作都不发。
	if !pl.playing || pl.playing != beforePl.playing {
		t.Errorf("切主题改了播放状态：%v（之前 %v）", pl.playing, beforePl.playing)
	}
	if pl.pos != beforePl.pos {
		t.Errorf("切主题改了进度：%v（之前 %v）", pl.pos, beforePl.pos)
	}
	if len(pl.seeks) != beforeSeeks {
		t.Errorf("切主题触发了 seek：%d 次（之前 %d）", len(pl.seeks), beforeSeeks)
	}
	if pl.rate != beforePl.rate || pl.vol != beforePl.vol || pl.muted != beforePl.muted {
		t.Errorf("切主题改了播放器参数：rate %v→%v vol %v→%v muted %v→%v",
			beforePl.rate, pl.rate, beforePl.vol, pl.vol, beforePl.muted, pl.muted)
	}
}

// pxAt / colorDist 是主题测试用的最小像素工具。
func pxAt(img *image.RGBA, x, y int) ui.Color {
	c := img.RGBAAt(x, y)
	return ui.RGB(c.R, c.G, c.B)
}

func colorDist(a, b ui.Color) int {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	maxi := func(x, y int) int {
		if x > y {
			return x
		}
		return y
	}
	return maxi(d(a.R, b.R), maxi(d(a.G, b.G), d(a.B, b.B)))
}

// TestNativeViewFallsBackThenKeepsTheme 钉住没注入主题的原生窗（直接构造 /
// 独立使用）：第一帧外壳按当前时段补齐，之后每一帧都不许掉回零值（view 每帧
// 用 n.theme 覆盖外壳，补出来的那份要回写）。
func TestNativeViewFallsBackThenKeepsTheme(t *testing.T) {
	pl := &stubPlayer{playing: true, ready: true, dur: 60 * time.Second, pos: 1 * time.Second}
	n := &nativeWindow{pl: pl, title: "x", dm: &danmaku.Overlay{}}
	tt := ui.NewTester(n.view, 640, 360)
	tt.Frame()
	if n.chrome == nil {
		t.Fatal("视图没有建出外壳")
	}
	first := n.chrome.Theme
	if first.Mode == "" && first.Ink.A == 0 {
		t.Fatal("第一帧没有按时段补齐主题")
	}
	for i := 0; i < 3; i++ {
		tt.Frame()
		if got := n.chrome.Theme; got.Mode != first.Mode || got.Ink != first.Ink || got.Panel != first.Panel {
			t.Fatalf("第 %d 帧主题掉回了别的一份：%+v（第一帧 %+v）", i+2, got, first)
		}
	}
}
