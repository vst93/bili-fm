package view

// 原生视频窗外壳（videochrome.go）的无头测试：自动隐藏计时、全屏状态机、
// 快捷键映射、小窗布局、深浅两态的对比度与像素、单击 / 双击仲裁。
//
// 这里不碰任何媒体框架：播放器与窗口都是纯逻辑桩（见 videoStubPlayer /
// videoStubWindow），窗口是 ui.NewTester 的无头渲染。

import (
	"math"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// ---------------------------------------------------------------- 桩

// videoStubPlayer 是 VideoPlayer 的纯逻辑桩：记录下发到播放器的每一个动作。
type videoStubPlayer struct {
	playing   bool
	ended     bool
	ready     bool
	buffering bool
	dur       time.Duration
	pos       time.Duration
	rate      float64
	vol       float64
	muted     bool
	seeks     []time.Duration
	plays     int
	pauses    int
}

func (s *videoStubPlayer) Play()                   { s.playing, s.ended, s.plays = true, false, s.plays+1 }
func (s *videoStubPlayer) Pause()                  { s.playing, s.pauses = false, s.pauses+1 }
func (s *videoStubPlayer) Playing() bool           { return s.playing }
func (s *videoStubPlayer) Ended() bool             { return s.ended }
func (s *videoStubPlayer) Ready() bool             { return s.ready }
func (s *videoStubPlayer) Buffering() bool         { return s.buffering }
func (s *videoStubPlayer) Duration() time.Duration { return s.dur }
func (s *videoStubPlayer) Position() time.Duration { return s.pos }
func (s *videoStubPlayer) Seek(d time.Duration)    { s.seeks = append(s.seeks, d); s.pos = d }
func (s *videoStubPlayer) SetRate(r float64)       { s.rate = r }
func (s *videoStubPlayer) SetVolume(v float64)     { s.vol = v }
func (s *videoStubPlayer) SetMuted(m bool)         { s.muted = m }

// videoStubWindow 是 VideoWindow 的纯逻辑桩。restore 为真时 SetFullScreen(false)
// 自己把 bounds 还原（Windows / macOS / GTK 的实际行为），为假时保持全屏尺寸
// （模拟不还原的合成器）。
type videoStubWindow struct {
	full      bool
	restore   bool
	windowed  mygo.Rectangle
	bounds    mygo.Rectangle
	sets      int
	restored  []mygo.Rectangle
	setBounds []mygo.Rectangle
}

func newVideoStubWindow() *videoStubWindow {
	b := mygo.Rectangle{X: 120, Y: 80, Width: 960, Height: 600}
	return &videoStubWindow{windowed: b, bounds: b}
}

func (w *videoStubWindow) SetFullScreen(v bool) {
	w.sets++
	w.full = v
	if v {
		w.bounds = mygo.Rectangle{X: 0, Y: 0, Width: 1920, Height: 1080}
		return
	}
	if w.restore {
		w.bounds = w.windowed
	}
}

func (w *videoStubWindow) IsFullScreen() bool     { return w.full }
func (w *videoStubWindow) Bounds() mygo.Rectangle { return w.bounds }
func (w *videoStubWindow) SetBounds(r mygo.Rectangle) {
	w.bounds = r
	w.setBounds = append(w.setBounds, r)
}

// videoChromeProbe 起一个无头视频窗：640×360、浅色主题、播放中、100s 时长。
func videoChromeProbe(t *testing.T, w, h int) (*VideoChrome, *videoStubPlayer, *videoStubWindow, *ui.Tester) {
	t.Helper()
	pl := &videoStubPlayer{playing: true, ready: true, dur: 100 * time.Second, pos: 30 * time.Second, rate: 1, vol: 1}
	win := newVideoStubWindow()
	ch := &VideoChrome{
		Theme:  ThemeFor(PeriodAt(12)),
		Title:  "测试稿件 · 第一部分",
		Pl:     pl,
		Win:    win,
		Speed:  1,
		Volume: 1,
	}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, w, h)
	tt.Frame()
	return ch, pl, win, tt
}

// ---------------------------------------------------------------- 自动隐藏

// TestVideoChromeAutoHide 钉住控件条的计时状态机：
//
//	指针静止 → 2.5s 后淡出（不是立刻、也不是永不）；
//	移动 → 立刻浮出（一帧内开始、140ms 内到 1）；
//	暂停 → 永不淡出（用户正在看状态 / 要操作）。
func TestVideoChromeAutoHide(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	if ch.idle.alpha != 1 {
		t.Fatalf("首帧控件条应完全可见，得到 %v", ch.idle.alpha)
	}
	// 静止 2s：还没到 2.5s，保持可见。
	tt.Advance(2 * time.Second)
	if ch.idle.alpha != 1 {
		t.Errorf("静止 2s 就淡出了（%v），应为 2.5s 之后", ch.idle.alpha)
	}
	// 再过 1.5s：越过静置阈值 + 220ms 淡出，应完全收起。
	tt.Advance(1500 * time.Millisecond)
	if ch.idle.alpha != 0 {
		t.Errorf("静止 3.5s 后控件条仍可见（alpha=%v）", ch.idle.alpha)
	}
	// 移动指针：立刻开始浮出，并在 140ms 内到 1。
	tt.Move(200, 200)
	tt.Frame()
	if ch.idle.alpha <= 0 {
		t.Fatalf("移动指针后控件条没有浮出（alpha=%v）", ch.idle.alpha)
	}
	tt.Advance(videoChromeShowFade)
	if ch.idle.alpha != 1 {
		t.Errorf("浮出 %v 后 alpha=%v，应为 1", videoChromeShowFade, ch.idle.alpha)
	}
	// 暂停：静止再久也不淡出。
	pl.Pause()
	tt.Advance(10 * time.Second)
	if ch.idle.alpha != 1 {
		t.Errorf("暂停时控件条淡出了（alpha=%v）", ch.idle.alpha)
	}
}

// TestVideoChromeAutoHideFullScreen 钉住「全屏与窗口模式共用同一套」：
// 全屏下静止同样淡出、移动同样浮出。
func TestVideoChromeAutoHideFullScreen(t *testing.T) {
	ch, _, win, tt := videoChromeProbe(t, 1280, 720)
	tt.Key(0, ui.KeyF)
	tt.Frame()
	if !win.full {
		t.Fatal("F 没有进入全屏")
	}
	tt.Advance(4 * time.Second)
	if ch.idle.alpha != 0 {
		t.Errorf("全屏下静止 4s 控件条没收起（alpha=%v）", ch.idle.alpha)
	}
	tt.Move(400, 300)
	tt.Frame()
	if ch.idle.alpha <= 0 {
		t.Error("全屏下移动指针控件条没有浮出")
	}
}

// TestVideoChromeHoldOnBarHover 钉住「鼠标停在控件条上不淡出」：指针停在
// 底部控件条里不动，静止 5s 仍是 1。
func TestVideoChromeHoldOnBarHover(t *testing.T) {
	ch, _, _, tt := videoChromeProbe(t, 640, 360)
	w, h := float32(640), float32(360)
	// 控件条中心（离底 10 + 44/2）。
	tt.Move(w/2, h-videoChromePad-videoChromeBarH/2)
	tt.Frame()
	if !ch.barHover {
		t.Fatalf("指针在控件条上，barHover 应为真（alpha=%v）", ch.idle.alpha)
	}
	tt.Advance(5 * time.Second)
	if ch.idle.alpha != 1 {
		t.Errorf("指针停在控件条上仍淡出了（alpha=%v）", ch.idle.alpha)
	}
}

// TestVideoChromeHiddenIgnoresInput 钉住「收起后点画面 = 播放/暂停」：
// 控件条在 alpha 0 时不接收指针，点击穿到画面上。
func TestVideoChromeHiddenIgnoresInput(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	tt.Advance(4 * time.Second)
	if ch.idle.alpha != 0 {
		t.Fatalf("控件条没收起（alpha=%v）", ch.idle.alpha)
	}
	before := pl.pauses
	tt.ClickAt(320, 180) // 画面正中
	tt.Advance(videoDoubleClick)
	if pl.pauses == before {
		t.Error("控件条收起后点画面没有触发播放/暂停")
	}
}

// ---------------------------------------------------------------- 单击 / 双击

// TestVideoChromeClickArbitration 钉住单击 / 双击的分工（原版 `<video>` 的
// 浏览器行为）：单击切播放、双击切全屏，且双击**不**顺带切一次播放。
func TestVideoChromeClickArbitration(t *testing.T) {
	ch, pl, win, tt := videoChromeProbe(t, 640, 360)
	// 单击：动作延后到双击窗口结束才执行。
	tt.ClickAt(320, 180)
	if pl.pauses != 0 {
		t.Fatalf("单击立刻切了播放（应等 %v 的双击窗口）", videoDoubleClick)
	}
	tt.Advance(videoDoubleClick + 20*time.Millisecond)
	if pl.pauses != 1 {
		t.Errorf("单击应切一次播放，pauses=%d", pl.pauses)
	}
	// 双击：只切全屏，不再切播放。（两次双击之间要越过系统的双击间隔
	// 500ms，否则第三、四下会被算成三连击 / 四连击，mygo 不再报 DoubleClicked。）
	tt.Advance(600 * time.Millisecond)
	tt.ClickAt(320, 180)
	tt.ClickAt(320, 180)
	tt.Advance(videoDoubleClick + 20*time.Millisecond)
	if win.sets != 1 || !win.full {
		t.Errorf("双击没有进入全屏（sets=%d full=%v）", win.sets, win.full)
	}
	if pl.pauses != 1 {
		t.Errorf("双击顺带切了播放（pauses=%d，应为 1）", pl.pauses)
	}
	// 再双击：退出全屏。
	tt.Advance(600 * time.Millisecond)
	tt.ClickAt(320, 180)
	tt.ClickAt(320, 180)
	tt.Advance(videoDoubleClick + 20*time.Millisecond)
	if win.full {
		t.Error("再双击没有退出全屏")
	}
	if pl.pauses != 1 {
		t.Errorf("退出全屏时又切了播放（pauses=%d）", pl.pauses)
	}
	_ = ch
}

// ---------------------------------------------------------------- 全屏

// TestVideoFullScreenStateMachine 钉住全屏状态机：进入时记住窗口态的尺寸，
// 退出后恢复；平台自己还原时不重复 SetBounds。
func TestVideoFullScreenStateMachine(t *testing.T) {
	for _, restore := range []bool{true, false} {
		win := newVideoStubWindow()
		win.restore = restore
		var f videoFullScreen
		now := time.Unix(1700000000, 0)

		if !f.toggle(now, win) {
			t.Fatal("第一次切换应被接受")
		}
		if !win.full || f.saved != win.windowed {
			t.Fatalf("进入全屏后 full=%v saved=%+v，想要 saved=%+v", win.full, f.saved, win.windowed)
		}
		if got := f.fade(now); got != videoFullScreenFadeMax {
			t.Errorf("切换那一帧黑幕应为 %v，得到 %v", videoFullScreenFadeMax, got)
		}
		if got := f.fade(now.Add(videoFullScreenFade / 2)); got <= 0 || got >= videoFullScreenFadeMax {
			t.Errorf("过渡中途黑幕应在 (0, %v) 之间，得到 %v", videoFullScreenFadeMax, got)
		}
		if got := f.fade(now.Add(videoFullScreenFade)); got != 0 {
			t.Errorf("%v 后黑幕应为 0，得到 %v", videoFullScreenFade, got)
		}

		// 退出：先确认窗口状态，再按需恢复尺寸。
		f.observe(now.Add(time.Second), win)
		if !f.full {
			t.Fatal("窗口确认后 full 应为真")
		}
		if !f.toggle(now.Add(2*time.Second), win) {
			t.Fatal("第二次切换应被接受")
		}
		f.observe(now.Add(3*time.Second), win)
		if f.full {
			t.Fatal("退出后 full 应为假")
		}
		if win.bounds != win.windowed {
			t.Errorf("退出全屏后 bounds=%+v，想要 %+v", win.bounds, win.windowed)
		}
		// 平台自己还原时不该再 SetBounds（restore=true）。
		if restore && len(win.setBounds) != 0 {
			t.Errorf("平台已还原尺寸，不该再 SetBounds：%v", win.setBounds)
		}
		if !restore && len(win.setBounds) != 1 {
			t.Errorf("平台没还原时应补一次 SetBounds，实际 %v", win.setBounds)
		}
	}
}

// TestVideoFullScreenDebounce 钉住「连点双击不抖动」：窗口确认之前再来的
// 切换一律忽略；确认之后才接受下一次。
func TestVideoFullScreenDebounce(t *testing.T) {
	win := newVideoStubWindow()
	var f videoFullScreen
	now := time.Unix(1700000000, 0)
	if !f.toggle(now, win) {
		t.Fatal("第一次切换应被接受")
	}
	for i := 0; i < 4; i++ {
		if f.toggle(now.Add(time.Duration(i+1)*50*time.Millisecond), win) {
			t.Fatalf("第 %d 次连点被接受（过渡中应忽略）", i+2)
		}
	}
	if win.sets != 1 {
		t.Errorf("连点 5 次只应下发 1 次 SetFullScreen，实际 %d", win.sets)
	}
	// 窗口确认后，下一次切换恢复可用。
	f.observe(now.Add(500*time.Millisecond), win)
	if !f.toggle(now.Add(time.Second), win) {
		t.Error("窗口确认后切换应恢复可用")
	}
	if win.full {
		t.Error("确认后的切换应退出全屏")
	}
}

// TestVideoFullScreenTimeout 钉住「合成器不答复」时的收尾：Wayland 平铺
// 合成器可能拒绝全屏，闩锁不能挂死 —— 超时后按实际状态收尾。
func TestVideoFullScreenTimeout(t *testing.T) {
	win := newVideoStubWindow()
	var f videoFullScreen
	now := time.Unix(1700000000, 0)
	f.toggle(now, win)
	win.full = false // 合成器把请求咽了
	f.observe(now.Add(videoFullScreenWait/2), win)
	if !f.inFlight {
		t.Fatal("超时之前应仍在等确认")
	}
	f.observe(now.Add(videoFullScreenWait+time.Second), win)
	if f.inFlight {
		t.Error("超时后应放弃等待")
	}
	if f.full || f.requested {
		t.Errorf("超时后应按实际状态收尾：full=%v requested=%v", f.full, f.requested)
	}
}

// ---------------------------------------------------------------- 快捷键

// TestVideoKeyBindings 钉住快捷键映射表，以及「与主窗快捷键冲突时谁赢」。
//
// 结论：**视频窗全胜**。主窗的 shortcuts.go 注册在它自己的 ui.Context 上，
// 而 mygo 的快捷键是每窗口一份（ui/actions.go 的 action.window），焦点在
// 视频窗时主窗根本收不到这些键；唯一跨窗口的是 macOS 原生菜单的 Cmd+Q 加速键，
// 它与视频窗自己的 Cmd+Q 是同一个动作。Esc 是唯一「同一个键在两个窗口含义
// 不同」的一条，但两边不会同时收到。
func TestVideoKeyBindings(t *testing.T) {
	cases := []struct {
		mods ui.Modifiers
		key  ui.Key
		full bool
		want videoKeyAction
	}{
		{0, ui.KeySpace, false, videoKeyPlayPause},
		{0, ui.KeySpace, true, videoKeyPlayPause},
		{0, ui.KeyLeft, false, videoKeyBack},
		{0, ui.KeyRight, false, videoKeyForward},
		{0, ui.KeyUp, false, videoKeyVolumeUp},
		{0, ui.KeyDown, false, videoKeyVolumeDown},
		{0, ui.KeyM, false, videoKeyMute},
		{0, ui.KeyF, false, videoKeyFullScreen},
		{0, ui.KeyF, true, videoKeyFullScreen},
		// Esc 分层：全屏时先退全屏，窗口态才关窗。
		{0, ui.KeyEscape, false, videoKeyClose},
		{0, ui.KeyEscape, true, videoKeyExitFullScreen},
		{ui.Cmd, ui.KeyW, false, videoKeyMinimize},
		{ui.Cmd, ui.KeyQ, false, videoKeyQuit},
		// 带别的修饰键不抢。
		{ui.Shift, ui.KeySpace, false, videoKeyNone},
		{ui.Alt, ui.KeyLeft, false, videoKeyNone},
		// 表里没有的键不响应。
		{0, ui.KeyEnter, false, videoKeyNone},
	}
	for _, c := range cases {
		if got := videoKeyFor(c.mods, c.key, c.full); got != c.want {
			t.Errorf("videoKeyFor(mods=%v, key=%v, full=%v)=%v，想要 %v",
				c.mods, c.key, c.full, got, c.want)
		}
	}
	// 注册表必须覆盖上表里除「不响应」之外的每一个键，否则键位是哑的。
	for _, c := range cases {
		if c.want == videoKeyNone {
			continue
		}
		ok := false
		for _, b := range VideoKeyBindings {
			if b.Mods == c.mods && b.Key == c.key {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("键位 %v+%v 没有注册进 VideoKeyBindings", c.mods, c.key)
		}
	}
	// 音量是步进（按住连续加减），其余是一次性动作（自动重复会把暂停/播放来回切）。
	if !videoKeyVolumeUp.repeats() || !videoKeyVolumeDown.repeats() {
		t.Error("音量键应允许按住重复")
	}
	for _, a := range []videoKeyAction{videoKeyPlayPause, videoKeyFullScreen, videoKeyExitFullScreen, videoKeyClose, videoKeyBack} {
		if a.repeats() {
			t.Errorf("动作 %v 不该吃自动重复", a)
		}
	}
}

// TestVideoChromeKeysDrivePlayer 走一遍真实的无头窗口：空格 / ←→ / ↑↓ / M / F。
func TestVideoChromeKeysDrivePlayer(t *testing.T) {
	ch, pl, win, tt := videoChromeProbe(t, 640, 360)
	// 空格：暂停。
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if pl.playing {
		t.Fatal("空格没有暂停")
	}
	// 空格：继续。
	tt.Key(0, ui.KeySpace)
	tt.Frame()
	if !pl.playing {
		t.Fatal("空格没有继续播放")
	}
	// ←：倒退 10s（30s → 20s）。
	tt.Key(0, ui.KeyLeft)
	tt.Frame()
	if got := pl.pos.Seconds(); got < 19.5 || got > 20.5 {
		t.Errorf("← 之后位置 %.1fs，想要 20s", got)
	}
	// →：前进 10s。
	tt.Key(0, ui.KeyRight)
	tt.Frame()
	if got := pl.pos.Seconds(); got < 29.5 || got > 30.5 {
		t.Errorf("→ 之后位置 %.1fs，想要 30s", got)
	}
	// ↑ / ↓：音量 ±5%。
	tt.Key(0, ui.KeyUp)
	tt.Frame()
	if got := pl.vol; got < 1-1e-6 || got > 1 {
		// 音量已在上限，先降下来再试。
		tt.Key(0, ui.KeyDown)
		tt.Frame()
	}
	tt.Key(0, ui.KeyDown)
	tt.Frame()
	if pl.vol >= 1 {
		t.Errorf("↓ 之后音量 %v，应小于 1", pl.vol)
	}
	// M：静音。
	tt.Key(0, ui.KeyM)
	tt.Frame()
	if !pl.muted {
		t.Error("M 没有静音")
	}
	// F：全屏。
	tt.Key(0, ui.KeyF)
	tt.Frame()
	if !win.full {
		t.Error("F 没有进入全屏")
	}
	// Esc：先退全屏（不关窗）。
	closed := false
	ch.OnClose = func() { closed = true }
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if win.full {
		t.Error("Esc 没有退出全屏")
	}
	if closed {
		t.Error("全屏时 Esc 不该关窗")
	}
	// 窗口态 Esc：关窗。
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if !closed {
		t.Error("窗口态 Esc 应关窗")
	}
}

// TestVideoFullScreenKeepsPlayback 钉住「全屏切换不动播放状态」：进入 / 退出
// 全屏只碰窗口，播放 / 进度 / 音量 / 倍速一个都不许变（原版全屏是浏览器的
// 视频元素行为，与播放器状态无关）。
func TestVideoFullScreenKeepsPlayback(t *testing.T) {
	_, pl, win, tt := videoChromeProbe(t, 640, 360)
	before := *pl
	if err := tt.Click("全屏"); err != nil {
		t.Fatalf("点全屏键失败：%v", err)
	}
	tt.Frame()
	if !win.full {
		t.Fatal("点全屏键没有进入全屏")
	}
	if pl.playing != before.playing || pl.pos != before.pos || pl.rate != before.rate ||
		pl.vol != before.vol || len(pl.seeks) != len(before.seeks) {
		t.Errorf("进入全屏改动了播放状态：before=%+v after=%+v", before, *pl)
	}
	// 全屏后键位换成「退出全屏」。
	if !tt.HasText("退出全屏") {
		t.Errorf("全屏后没有「退出全屏」键：%v", tt.Texts())
	}
	if err := tt.Click("退出全屏"); err != nil {
		t.Fatalf("点退出全屏失败：%v", err)
	}
	tt.Frame()
	if win.full {
		t.Fatal("点退出全屏没有退出")
	}
	if pl.playing != before.playing || pl.pos != before.pos || pl.rate != before.rate ||
		pl.vol != before.vol || len(pl.seeks) != len(before.seeks) {
		t.Errorf("退出全屏改动了播放状态：before=%+v after=%+v", before, *pl)
	}
}

// TestVideoChromeWithoutWindow 钉住没有窗口时的健壮性：全屏键、Esc、F 都
// 不能 panic（无头测试与「窗口刚好关掉」的竞态都会走到这里）。
func TestVideoChromeWithoutWindow(t *testing.T) {
	ch := &VideoChrome{Theme: ThemeFor(PeriodAt(12)), Title: "x",
		Pl: &videoStubPlayer{playing: true, ready: true, dur: 10 * time.Second}, Speed: 1, Volume: 1}
	tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 480, 270)
	tt.Frame()
	tt.Key(0, ui.KeyF)
	tt.Frame()
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if ch.fs.full {
		t.Error("没有窗口时不该进入全屏")
	}
}

// ---------------------------------------------------------------- 布局

// TestVideoChromeNarrowLayout 钉住小窗（480×270）的布局：关键控件都在、
// 互不重叠、都在窗口内。窄窗（<440）改成两行，进度条不被挤成一条线。
func TestVideoChromeNarrowLayout(t *testing.T) {
	for _, size := range []struct{ w, h int }{{480, 270}, {420, 240}, {320, 200}, {900, 500}} {
		ch, _, _, tt := videoChromeProbe(t, size.w, size.h)
		_ = ch
		tt.Frame()
		want := []string{"播放", "暂停", "播放进度", "0:30", "1:40", "1x", "弹幕", "静音", "全屏", "关闭视频"}
		rects := map[string]ui.Rect{}
		for _, s := range want {
			r, ok := tt.Find(s)
			if !ok {
				// 「播放」与「暂停」只有一个存在。
				if s == "播放" || s == "暂停" {
					continue
				}
				t.Fatalf("%dx%d 缺少控件 %q（现有：%v）", size.w, size.h, s, tt.Texts())
			}
			if r.X < 0 || r.Y < 0 || r.X+r.W > float32(size.w)+0.5 || r.Y+r.H > float32(size.h)+0.5 {
				t.Errorf("%dx%d 的 %q 超出窗口：%+v", size.w, size.h, s, r)
			}
			rects[s] = r
		}
		// 两两不重叠。
		names := make([]string, 0, len(rects))
		for n := range rects {
			names = append(names, n)
		}
		for i := 0; i < len(names); i++ {
			for j := i + 1; j < len(names); j++ {
				a, b := rects[names[i]], rects[names[j]]
				if a.X < b.X+b.W && b.X < a.X+a.W && a.Y < b.Y+b.H && b.Y < a.Y+a.H {
					t.Errorf("%dx%d 的 %q 与 %q 重叠：%+v / %+v", size.w, size.h, names[i], names[j], a, b)
				}
			}
		}
		// 标题栏与控件条不重叠（240 高也不许压在一起）。
		title, ok1 := tt.Find("测试稿件 · 第一部分")
		bar, ok2 := tt.Find("播放进度")
		if ok1 && ok2 && title.Y+title.H > bar.Y {
			t.Errorf("%dx%d 标题栏与控件条重叠：标题底 %v > 进度顶 %v", size.w, size.h, title.Y+title.H, bar.Y)
		}
	}
}

// TestVideoChromeLayoutRules 钉住排布的纯函数：窄窗两行、宽窗才有音量滑块。
func TestVideoChromeLayoutRules(t *testing.T) {
	if lo := videoChromeLayoutFor(480); lo.TwoRows || lo.VolumeBar {
		t.Errorf("480 宽应是一行且不带音量滑块，得到 %+v", lo)
	}
	if lo := videoChromeLayoutFor(380); !lo.TwoRows {
		t.Error("380 宽应是两行")
	}
	if lo := videoChromeLayoutFor(900); !lo.VolumeBar || lo.TwoRows {
		t.Errorf("900 宽应带音量滑块且一行，得到 %+v", lo)
	}
}

// ---------------------------------------------------------------- 观感

// TestVideoChromeContrast 钉住「深浅两态都能看」：面板是不透明底，文字/图标
// 压在面板上的对比度必须够（正文 4.5:1，图标与强调色 3:1）。面板半透明，
// 所以按「面板叠在纯黑上」与「叠在纯白上」两种极端各算一次。
func TestVideoChromeContrast(t *testing.T) {
	themes := map[string]Theme{
		"light": ThemeFor(PeriodAt(12)),
		"dark":  ThemeFor(PeriodAt(12)).withAppearance("dark", 0),
	}
	for name, th := range themes {
		for _, under := range []ui.Color{ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)} {
			panel := composite(th.Panel, under)
			for label, c := range map[string]ui.Color{
				"Ink":   th.Ink,
				"Muted": th.Muted,
				"Blue":  th.Blue,
			} {
				min := 4.5
				if label == "Blue" {
					min = 3.0
				}
				if got := contrastRatio(composite(c, panel), panel); got < min {
					t.Errorf("%s 主题、面板叠在 %v 上：%s 的对比度 %.2f < %.1f", name, under, label, got, min)
				}
			}
		}
	}
}

// TestVideoChromePaintsChrome 钉住「控件条真的画出来了」：标题栏与控件条区域
// 必须有不等于画面底色（纯黑）的像素，深浅两态都一样。
func TestVideoChromePaintsChrome(t *testing.T) {
	for _, mode := range []string{"light", "dark"} {
		th := ThemeFor(PeriodAt(12))
		if mode == "dark" {
			th = th.withAppearance("dark", 0)
		}
		ch := &VideoChrome{Theme: th, Title: "标题", Pl: &videoStubPlayer{playing: true, ready: true,
			dur: 100 * time.Second, pos: 30 * time.Second}, Speed: 1, Volume: 1}
		tt := ui.NewTester(func(c *ui.Context) { ch.View(c) }, 640, 360)
		tt.Frame()
		img := tt.Image()
		nonBlack := func(x0, y0, x1, y1 int) int {
			n := 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					r, g, b, _ := img.RGBAAt(x, y).RGBA()
					if r>>8 > 8 || g>>8 > 8 || b>>8 > 8 {
						n++
					}
				}
			}
			return n
		}
		top := nonBlack(10, 10, 630, 10+int(videoChromeTitleH))
		bottom := nonBlack(10, 360-int(videoChromePad+videoChromeBarH), 630, 350)
		if top < 500 {
			t.Errorf("%s 主题：标题栏几乎没有像素（%d）", mode, top)
		}
		if bottom < 2000 {
			t.Errorf("%s 主题：控件条几乎没有像素（%d）", mode, bottom)
		}
	}
}

// TestVideoChromeDanmakuState 钉住弹幕开关位：键在、点击只翻转状态（本轮不接
// 数据，不许偷偷发请求或改播放器）。
func TestVideoChromeDanmakuState(t *testing.T) {
	ch, pl, _, tt := videoChromeProbe(t, 640, 360)
	before := *pl
	if err := tt.Click("弹幕"); err != nil {
		t.Fatalf("点弹幕键失败：%v", err)
	}
	tt.Frame()
	if !ch.Danmaku {
		t.Error("点了弹幕键，状态没打开")
	}
	if pl.playing != before.playing || len(pl.seeks) != len(before.seeks) {
		t.Error("弹幕开关不该碰播放器")
	}
	if err := tt.Click("弹幕"); err != nil {
		t.Fatalf("再点弹幕键失败：%v", err)
	}
	tt.Frame()
	if ch.Danmaku {
		t.Error("再点一次弹幕应关闭")
	}
}

// ---------------------------------------------------------------- 纯函数

// TestVideoSpeedStep 钉住倍速档位：与主窗 speedOptions 完全一致、末档回首档、
// 未命中档位回首档（原地不动会让人以为按键没反应）。
func TestVideoSpeedStep(t *testing.T) {
	if len(VideoSpeeds) != len(speedOptions) {
		t.Fatalf("视频窗档位 %v 与主窗 %v 长度不一致", VideoSpeeds, speedOptions)
	}
	for i := range VideoSpeeds {
		if VideoSpeeds[i] != speedOptions[i] {
			t.Errorf("档位 %d：视频窗 %v ≠ 主窗 %v", i, VideoSpeeds[i], speedOptions[i])
		}
	}
	for i, v := range VideoSpeeds {
		want := VideoSpeeds[(i+1)%len(VideoSpeeds)]
		if got := videoSpeedStep(v, VideoSpeeds); got != want {
			t.Errorf("videoSpeedStep(%v)=%v，想要 %v", v, got, want)
		}
	}
	if got := videoSpeedStep(1.9, VideoSpeeds); got != VideoSpeeds[0] {
		t.Errorf("未命中档位应回首档，得到 %v", got)
	}
	if got := videoSpeedStep(1, nil); got != 1 {
		t.Errorf("空档位应回 1，得到 %v", got)
	}
	if got := videoSpeedLabel(1.25); got != "1.25x" {
		t.Errorf("倍速文案 %q，想要 1.25x", got)
	}
	if got := videoSpeedLabel(2); got != "2x" {
		t.Errorf("倍速文案 %q，想要 2x", got)
	}
}

// TestVideoClockText 钉住时间文案（与主窗播放栏同一种 m:ss / h:mm:ss）。
func TestVideoClockText(t *testing.T) {
	cases := map[float64]string{
		0: "0:00", 9.9: "0:09", 61: "1:01", 599: "9:59",
		3600: "1:00:00", 3725: "1:02:05",
		-1: "0:00",
	}
	for in, want := range cases {
		if got := clockText(in); got != want {
			t.Errorf("clockText(%v)=%q，想要 %q", in, got, want)
		}
	}
}

// TestVideoBufferingPulse 钉住缓冲呼吸：不缓冲时恒 0（不画），缓冲时在
// 0.25~0.60 之间随时间变化。
func TestVideoBufferingPulse(t *testing.T) {
	now := time.Unix(1700000000, 0)
	if got := videoBufferingPulse(now, false); got != 0 {
		t.Errorf("不缓冲时脉冲应为 0，得到 %v", got)
	}
	lo, hi := float32(1), float32(0)
	for i := 0; i < 20; i++ {
		v := videoBufferingPulse(now.Add(time.Duration(i)*80*time.Millisecond), true)
		if v < 0.24 || v > 0.61 {
			t.Fatalf("脉冲 %v 越界（应在 0.25~0.60）", v)
		}
		lo, hi = minF(lo, v), maxFloat32(hi, v)
	}
	if hi-lo < 0.2 {
		t.Errorf("脉冲几乎不动（%v~%v），看不出在缓冲", lo, hi)
	}
}

// ---------------------------------------------------------------- 工具

// composite 把半透明色叠在不透明底上。
func composite(c, under ui.Color) ui.Color {
	a := float32(c.A) / 255
	mix := func(fg, bg uint8) uint8 {
		return uint8(float32(fg)*a + float32(bg)*(1-a) + 0.5)
	}
	return ui.RGB(mix(c.R, under.R), mix(c.G, under.G), mix(c.B, under.B))
}

// relativeLuminance 是 WCAG 的相对亮度。
func relativeLuminance(c ui.Color) float64 {
	lin := func(v uint8) float64 {
		s := float64(v) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// contrastRatio 是 WCAG 的对比度。
func contrastRatio(a, b ui.Color) float64 {
	la, lb := relativeLuminance(a), relativeLuminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
