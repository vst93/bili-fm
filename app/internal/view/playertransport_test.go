package view

// 本轮（上一集 / 下一集恢复圆形键）的回归测试。
//
// 回归点：458c170 把控件条抽进 playerbar_shared.go 的 playerBarKeyMetrics 后，
// 工具键统一成 `Radius(m.KeyR)`（=Radius 8），传输键（32×32 的下一集，视频窗
// 里置灰）原本是 `Radius(RadiusPill)` 正圆，被一起带成了 8px 方块。原版
// player.tsx 的换集键是 `width/height: 32px; border-radius: 999px`。
//
// 三处钉住：
//  1. 令牌：TransportR 与 PlayR 同为 RadiusPill；工具键仍
//     是 KeyR=Radius(8)；尺寸（32/42/34/40）一个都不许动；
//  2. 像素：主窗与视频窗的传输键四角是**栏底色**（圆的），工具键四角是**键面**
//     （方的）。两窗共用同一份 metrics，所以两窗同时成立；
//  3. 反证：把 TransportR 改回 KeyR，像素断言翻红。
//
// 形状断言的底必须是深色：浅色栏上「键面」与「栏底」只差几个灰度单位，像素
// 断言会变成噪声；深色下差 20–80，圆的键在角上是 0。

import (
	"image"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// 键角上的「键面墨量」阈值（见 keyCornerInk）。
//
// 正圆键（半径 = 半边长）左上角 (3,3) 完全在键面之外，与栏底的差只剩抗锯齿
// 噪声（实测 0–2）；8px 方角键那里被键面 / 描边盖住（实测主窗 68、视频窗 30）。
const (
	cornerInkRound  = 2
	cornerInkSquare = 10
)

// playerShapeTheme 是形状断言的底：显式深色（浅色 KeyR 方块与深色栏底的
// 对比度才够量出「四角有没有键面」）。
func playerShapeTheme() Theme {
	return ThemeFor(PeriodAt(12)).withAppearance("dark", 1)
}

// playerShapeMain 渲染主窗底部播放栏（900 宽），曲目在播、队列里还有下一条
// （传输键是可用态），窗底铺纯黑。
func playerShapeMain(t *testing.T, th Theme) *ui.Tester {
	t.Helper()
	a := NewApp(func() {})
	a.Theme = th
	a.Track = &Track{Bvid: "BV1", Title: "标题", Up: "UP"}
	a.Queue = []Track{{Bvid: "BV1", Title: "标题", Up: "UP"}, {Bvid: "BV2", Title: "标题2", Up: "UP"}}
	a.Index = 0
	a.Playing = true
	a.PlayerSettings.Ensure()
	tt := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(a.Theme.uiTheme())
		c.Root().Background(ui.Hex("#000000"))
		a.playerBar(c)
	}, 900, 200)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	return tt
}

// playerShapeVideo 渲染视频窗（900×500，完整一档），控件条压在面板上。
func playerShapeVideo(t *testing.T, th Theme) *ui.Tester {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	ch := &VideoChrome{
		Theme: th,
		Title: "测试稿件",
		Pl:    pl,
		Win:   newVideoStubWindow(),
		Set:   &PlayerSettings{},
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 900, 500)
	tt.Advance(400 * time.Millisecond)
	tt.Frame()
	return tt
}

// keyCornerInk 量一颗键左上角是否被键面盖住：取 (X+3, Y+3) 与键外同高的
// (X-3, Y+3) 栏底色逐通道比，返回最大差。正圆键的角上是栏底色（≈0），
// 方角键的角上是键面 / 描边（远大于 0）。
func keyCornerInk(img *image.RGBA, r ui.Rect) int {
	px := func(x, y int) (int, int, int) {
		c := img.RGBAAt(x, y)
		return int(c.R), int(c.G), int(c.B)
	}
	br, bg, bb := px(int(r.X)-3, int(r.Y)+3)
	cr, cg, cb := px(int(r.X)+3, int(r.Y)+3)
	diff := func(a, b int) int {
		if a < b {
			return b - a
		}
		return a - b
	}
	return max(diff(cr, br), max(diff(cg, bg), diff(cb, bb)))
}

// mustKey 取一颗键的矩形（Tester.Find 按 label / tooltip 文本找）。
func mustKey(t *testing.T, tt *ui.Tester, win, label string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(label)
	if !ok {
		t.Fatalf("%s里找不到 %q 键（文本：%v）", win, label, tt.Texts())
	}
	return r
}

// TestPlayerBarKeyRadii 钉住令牌：传输键与播放键是正圆，工具键仍是 8px 方角，
// 尺寸一个都不动。
func TestPlayerBarKeyRadii(t *testing.T) {
	m := playerBarKeyMetrics

	// 传输键（上一集 / 下一集）与播放键：RadiusPill。
	if m.TransportR != RadiusPill {
		t.Errorf("传输键圆角 = %v，应为 RadiusPill(%v)", m.TransportR, RadiusPill)
	}
	if m.PlayR != RadiusPill {
		t.Errorf("播放键圆角 = %v，应为 RadiusPill(%v)", m.PlayR, RadiusPill)
	}
	for _, key := range []string{"prev", "next"} {
		if got := playerTransportRadius(key); got != RadiusPill {
			t.Errorf("%s 键圆角 = %v，应为 RadiusPill（458c170 起被统一成了 KeyR）", key, got)
		}
	}

	// 工具键：Radius(8)，不是正圆。
	if Radius != 8 {
		t.Fatalf("Radius = %v，本断言的前提是 8", Radius)
	}
	if m.KeyR != Radius {
		t.Errorf("工具键圆角 = %v，应为 Radius(%v)", m.KeyR, Radius)
	}

	// 尺寸 / 列宽：本轮只改形状，数值一个都不许动。
	sizes := []struct {
		name      string
		got, want float32
	}{
		{"工具键边长", m.Key, 32},
		{"播放键边长", m.Play, 42},
		{"均衡 / 恰饭键宽", m.Wide, 34},
		{"倍速键宽", m.SpeedW, 40},
		{"工具列宽", m.ToolSlot, 34},
		{"倍速列宽", m.SpeedSlot, 40},
		{"恰饭列宽", m.SponsorSlot, 40},
		{"传输键间距", m.TransportGap, 6},
		{"时间列宽", m.TimeW, 56},
	}
	for _, s := range sizes {
		if s.got != s.want {
			t.Errorf("%s = %v，应为 %v", s.name, s.got, s.want)
		}
	}
}

// TestPlayerBarTransportKeysRound 无头实测主窗：传输键（下一集）与播放键四角
// 是空的（半径 = 半边长），工具键四角是键面（8px 方角）。
func TestPlayerBarTransportKeysRound(t *testing.T) {
	tt := playerShapeMain(t, playerShapeTheme())
	img := tt.Image()

	for _, c := range []struct{ name, label string }{
		{"播放", "暂停"},
		{"下一集", nextTip()},
	} {
		r := mustKey(t, tt, "主窗", c.label)
		if got := keyCornerInk(img, r); got > cornerInkRound {
			t.Errorf("%s键四角有键面（墨量 %d，圆键应为 ≤ %d）：%+v", c.name, got, cornerInkRound, r)
		}
	}
	for _, c := range []struct{ name, label string }{
		{"音量", "音量"},
		{"倍速", speedTip(1)},
		{"均衡", eqTip(false)},
		{"恰饭", sponsorTip()},
	} {
		r := mustKey(t, tt, "主窗", c.label)
		if got := keyCornerInk(img, r); got < cornerInkSquare {
			t.Errorf("%s键四角是空的（墨量 %d，方键应 ≥ %d）：%+v", c.name, got, cornerInkSquare, r)
		}
	}
}

// TestVideoChromeTransportKeysRound 同一组断言在视频窗成立，并且两个窗口的
// **同名键尺寸相同** —— 这就是共用 playerBarKeyMetrics 的证据（视频窗没有
// 自己的键令牌，也没有特例）。
func TestVideoChromeTransportKeysRound(t *testing.T) {
	th := playerShapeTheme()
	mainTT := playerShapeMain(t, th)
	vidTT := playerShapeVideo(t, th)

	wins := []struct {
		name string
		tt   *ui.Tester
		next string
	}{
		{"主窗", mainTT, nextTip()},
		{"视频窗", vidTT, "视频窗没有换集：回主窗用播放列表"},
	}
	rects := map[string]map[string]ui.Rect{}
	for _, w := range wins {
		img := w.tt.Image()
		rects[w.name] = map[string]ui.Rect{}
		// 圆的：播放 + 下一集（视频窗没有换集能力，键置灰但形状不变）。
		for _, c := range []struct{ key, label string }{
			{"play", "暂停"},
			{"next", w.next},
		} {
			r := mustKey(t, w.tt, w.name, c.label)
			rects[w.name][c.key] = r
			if got := keyCornerInk(img, r); got > cornerInkRound {
				t.Errorf("%s的 %s 键四角有键面（墨量 %d，应为 ≤ %d）：%+v", w.name, c.key, got, cornerInkRound, r)
			}
		}
		// 方的：音量 / 倍速 / 恰饭（视频窗没有均衡键，主窗那颗见下）。
		for _, c := range []struct{ key, label string }{
			{"volume", "音量"},
			{"speed", speedTip(1)},
			{"sponsor", sponsorTip()},
		} {
			r := mustKey(t, w.tt, w.name, c.label)
			rects[w.name][c.key] = r
			if got := keyCornerInk(img, r); got < cornerInkSquare {
				t.Errorf("%s的 %s 键四角是空的（墨量 %d，应为 ≥ %d）：%+v", w.name, c.key, got, cornerInkSquare, r)
			}
		}
	}

	// 主窗仍有均衡键，且走同一套方键材质（视频窗没有它）。
	if r := mustKey(t, mainTT, "主窗", eqTip(false)); keyCornerInk(mainTT.Image(), r) < cornerInkSquare {
		t.Errorf("主窗均衡键四角是空的：%+v", r)
	}

	// 视频窗多出来的全屏键也走工具键那套（8px 方角）。
	fs := mustKey(t, vidTT, "视频窗", "全屏")
	if got := keyCornerInk(vidTT.Image(), fs); got < cornerInkSquare {
		t.Errorf("视频窗全屏键四角是空的（墨量 %d，应为 ≥ %d）：%+v", got, cornerInkSquare, fs)
	}

	// 两窗同名键的矩形**尺寸**相同（位置各随各的布局；32/42/40/34 一个都没动，
	// 形状同源）。
	for key, want := range rects["主窗"] {
		got := rects["视频窗"][key]
		if got.W != want.W || got.H != want.H {
			t.Errorf("%s 键两窗尺寸不同：主窗 %v×%v / 视频窗 %v×%v", key, want.W, want.H, got.W, got.H)
		}
	}
	if fs.W != rects["视频窗"]["volume"].W || fs.H != rects["视频窗"]["volume"].H {
		t.Errorf("全屏键尺寸 = %v×%v，应与其它工具键 32×32 相同", fs.W, fs.H)
	}
}

// TestPlayerBarTransportRadiusRegression 是反证：把 TransportR 改回 KeyR，
// 上面两条像素断言立刻变红（两窗同时）。工具键不受影响。
func TestPlayerBarTransportRadiusRegression(t *testing.T) {
	old := playerBarKeyMetrics.TransportR
	playerBarKeyMetrics.TransportR = playerBarKeyMetrics.KeyR
	defer func() { playerBarKeyMetrics.TransportR = old }()

	th := playerShapeTheme()
	mainTT := playerShapeMain(t, th)
	vidTT := playerShapeVideo(t, th)

	for _, w := range []struct {
		name string
		tt   *ui.Tester
		next string
	}{
		{"主窗", mainTT, nextTip()},
		{"视频窗", vidTT, "视频窗没有换集：回主窗用播放列表"},
	} {
		r := mustKey(t, w.tt, w.name, w.next)
		if got := keyCornerInk(w.tt.Image(), r); got <= cornerInkRound {
			t.Errorf("%s：TransportR 改回 KeyR 后传输键四角仍是空的（墨量 %d）—— 像素断言抓不住这个回归", w.name, got)
		}
	}

	// 工具键仍是方块（形状分家没把它们带歪）。
	r := mustKey(t, mainTT, "主窗", "音量")
	if got := keyCornerInk(mainTT.Image(), r); got < cornerInkSquare {
		t.Errorf("反证里工具键形状变了（墨量 %d，应为 ≥ %d）", got, cornerInkSquare)
	}
}
