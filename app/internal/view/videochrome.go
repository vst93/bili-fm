package view

// 原生视频窗（mygo 的 ui/video）的窗口外壳：标题栏、控件条、全屏、快捷键。
//
// 为什么这段在 view 里而不是 internal/video：视频窗的键必须与主窗**同一套**
// 形状与材质 —— 28×28、圆角、140ms 的悬停过渡、按压 0.94、tooltip、主题令牌
// （Ink/Muted/Line/Blue/Rose/Panel/Glass）。这些都在这个包里
// （theme.go / playerbar.go / headeraction.go / focus.go），internal/video
// 只有窗口与播放器。外壳放在这里，两边的键就是同一份代码，不会各写一套再
// 慢慢漂开；internal/video 只负责建窗、起播、把状态回传主窗。
//
// 对照原版（tauri:src/components/playerVideo.tsx + globals.css 的
// `body.video-open` 块）：
//
//   - 画面满铺整窗，控件浮在画面上，**默认隐去**，指针进顶部热区才唤回
//     （原版 TITLEBAR_HOTZONE_PX = 48，这里换成「指针一有移动就浮出，静止
//     2.5s 淡出」的统一规则 —— 原版只有顶部热区，底部原生控件条是浏览器画的，
//     原生版没有浏览器兜底，所以底部必须自己画，也就必须自己管自动隐藏）；
//   - 原版单击画面 = 播放/暂停、双击 = 全屏（`<video controls>` 的浏览器默认
//     行为，WebKit 与 Chromium 一致）。这里照做，但双击的第一下不能又切一次
//     播放 —— 单击动作延后 videoDoubleClick 提交，双击会把它撤销；
//   - 原版关闭键是右上角 38px 圆形按钮；这里按本应用的键族（28×28）放进
//     标题栏与控件条右端，位置与主窗的窗控键对齐。

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 尺寸（DIP）。VideoChromeKey 与主窗播放栏的工具键同尺寸（28×28）。
const (
	VideoChromeKey    float32 = 28
	videoChromePad    float32 = 10
	videoChromeGap    float32 = 6
	videoChromeTitleH float32 = 34
	videoChromeBarH   float32 = 44
	videoChromeRadius float32 = 12
	videoChromeTimeW  float32 = 42
	videoChromeVolW   float32 = 68
	// videoChromeTextKeyW 是文字键（倍速）的宽度：28×28 装不下 "1.25x"
	// （11px Inter 约 30px），文字键单独放宽，高度仍是 28。
	videoChromeTextKeyW float32 = 40
	// videoChromeBarTwoH 是窄窗两行控件条的高度（4 + 24 + 2 + 28 + 4）：
	// 第一行放进度条（高 timelineH=24），第二行放键（28）。
	videoChromeBarTwoH float32 = 62
	// videoChromeNarrow 以下进度条独占一行（一行里它只剩不到 80px）。
	videoChromeNarrow float32 = 440
	// videoChromeWide 以上才出现音量滑块（静音键始终在）。
	videoChromeWide float32 = 620
)

// 计时。视频窗的控件条自动隐藏与全屏过渡都按这几个值走。
const (
	// videoChromeIdleDelay 是指针静止多久后控件条开始淡出。原版是
	// 「鼠标离开顶部热区就收」，原生版把标题栏与控件条合成一套，所以用
	// 「静止 2.5s」而不是「离开热区」——2~3s 是播放器控件条的通行区间。
	videoChromeIdleDelay = 2500 * time.Millisecond
	// videoChromeShowFade / videoChromeHideFade 是浮出 / 淡出的时长。
	// 浮出要快（手已经动了，界面要跟上），淡出慢一档（不抢画面的注意力）。
	videoChromeShowFade = 140 * time.Millisecond
	videoChromeHideFade = 220 * time.Millisecond
	// videoFullScreenFade 是全屏进出时那层黑幕的淡出时长：它盖住「窗口尺寸
	// 突然变化 + 控件条重新布局」的那一下，原版没有（浏览器自己处理），
	// 原生窗口需要这一层过渡。起点不是全黑（0.85）—— 一瞬间刷成纯黑像闪屏，
	// 压到 0.85 再淡出读起来是一次「眨眼」，既盖住跳变又不刺眼。
	videoFullScreenFade            = 260 * time.Millisecond
	videoFullScreenFadeMax float32 = 0.85
	// videoFullScreenWait 是等窗口确认全屏状态的上限：macOS 的
	// `toggleFullScreen:` 与 GTK 的 fullscreen 都是异步的（合成器动画 / 状态
	// 事件），Wayland 下合成器还可能直接拒绝。超时后按窗口的实际状态收尾，
	// 不让「请求中」这个闩锁挂死。
	videoFullScreenWait = 1200 * time.Millisecond
	// videoDoubleClick 是单击 / 双击的判定窗口（与系统双击间隔同量级）。
	videoDoubleClick = 300 * time.Millisecond
	// videoSeekStep 是 ← / → 的步长（秒）。原版 playerVideo.tsx 的
	// SEEK_STEP_SECONDS = 10，媒体的 seekbackward/seekforward 也用它。
	videoSeekStep = 10.0
	// videoVolumeStep 是 ↑ / ↓ 的步长，与主窗 nudgeVolume(0.05) 一致。
	videoVolumeStep = 0.05
)

// VideoSpeeds 是倍速键的循环档位，与主窗播放栏的 speedOptions 完全一致
// （playerbar.go）。两处若漂开，同一个视频在两个窗口里会给出不同的档位。
var VideoSpeeds = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0}

// VideoPlayer 是外壳驱动播放器所需的最小面。*ui.VideoPlayer 的方法带返回值
// （SetRate 返回 *VideoPlayer），不满足它；internal/video 的 uiPlayer 适配器
// 与本文件无关，测试里塞纯逻辑桩即可。
type VideoPlayer interface {
	Play()
	Pause()
	Playing() bool
	Ended() bool
	Ready() bool
	Buffering() bool
	Duration() time.Duration
	Position() time.Duration
	Seek(time.Duration)
	SetRate(float64)
	SetVolume(float64)
	SetMuted(bool)
}

// VideoWindow 是外壳要操作的那部分窗口。*mygo.Window 满足它（方法名与签名
// 与框架一致），测试里换成记录调用的桩。
//
// 平台差异（框架侧，见 mygo/internal/{darwin,windows,linux}/window.go）：
//   - macOS：SetFullScreen 走 `toggleFullScreen:`，**异步**（系统动画约 0.5s），
//     IsFullScreen 读 styleMask，动画结束才变；窗口自己保留退出前的 frame，
//     所以 SetBounds 一般不会被调用（只有 frame 真的变了才兜底）。
//   - Windows：SetFullScreen 同步改 style + SetWindowPlacement，框架自己保存
//     并恢复 placement，退出后尺寸位置由它还原。
//   - Linux/GTK：gtk_window_fullscreen / unfullscreen，Wayland 下是向合成器
//     请求（xdg_toplevel.set_fullscreen），X11 下由 WM 处理；状态从
//     window-state-event 回来，也是异步的。平铺合成器（Sway/Hyprland）可能
//     拒绝或改成「填满工作区」，所以必须按 IsFullScreen 的实际值收尾。
type VideoWindow interface {
	SetFullScreen(bool)
	IsFullScreen() bool
	Bounds() mygo.Rectangle
	SetBounds(mygo.Rectangle)
}

// VideoChrome 是原生视频窗的界面外壳。零值可用（主题按当前时段取）。
type VideoChrome struct {
	// Theme 是主窗这一帧的主题（main.openVideo 传 c.app.Theme）。深浅两态
	// 都直接用它：控件条是浮在画面上的**应用面板**，不是画面的一部分。
	Theme Theme
	Title string
	Pl    VideoPlayer
	Win   VideoWindow

	// Content 画画面本身（internal/video 传 ui.Video）。为 nil 时画一块黑底，
	// 布局与命中测试仍照常工作（无头测试不需要真播放器）。
	Content func(c *ui.Context) ui.Element

	// OnClose / OnMinimize / OnQuit 是外壳要的窗口动作，nil 表示不提供。
	OnClose    func()
	OnMinimize func()
	OnQuit     func()
	// OnReplay 在「播完再点播放」把播放器拉回 0 时调用：窗口那边有一个
	// 「已经报过播完」的闩（endedFired），不在这里清掉，重播这一集播到结尾
	// 就不会再报一次播完（主窗的自动下一集 / 看完标记都不会触发）。
	OnReplay func()

	// Danmaku 是弹幕开关的状态。本轮**不接数据**（下一轮做覆盖层）：
	// 键、状态、tooltip 都在，点击只翻转这个字段。
	Danmaku bool

	// Speed / Volume / Muted 与播放器同源。internal/video 每帧把窗口自己的
	// 字段同步进来、再同步回去，所以主窗改倍速 / 静音时这里跟着变。
	Speed  float64
	Volume float64
	Muted  bool

	inited   bool
	idle     videoChromeIdle
	clicks   videoClickArbiter
	fs       videoFullScreen
	seek     videoSeek
	barHover bool
}

// ---------------------------------------------------------------- 自动隐藏

// videoChromeIdle 是控件条自动隐藏的状态机：指针一有移动（或任何交互）就
// 记一次「刚活动过」，静止 videoChromeIdleDelay 后目标变成隐藏，不透明度按
// 各自的方向匀速过渡。暂停 / 悬停在控件条上 / 拖动进度条时 hold 为真 ——
// 这几种情况下用户正在跟控件打交道，藏起来就是跟他作对。
type videoChromeIdle struct {
	seen  bool
	px    float32
	py    float32
	over  bool
	last  time.Time
	frame time.Time
	alpha float32
}

// videoChromeFrame 是一帧的自动隐藏结论。
type videoChromeFrame struct {
	// Alpha 是控件条这一帧的不透明度（0 表示完全收起、不接收指针）。
	Alpha float32
	// Animating 表示这一帧在过渡中：调用方必须 c.AnimationFrame()。
	Animating bool
	// Wake 表示不在过渡、但在等静置计时到点：调用方 c.After(Wake) 就够。
	Wake time.Duration
}

// observe 推进一帧。指针位置每帧读一次（root.PointerPosition），位置变了就是
// 「动过」；hold 是「不许淡出」的条件（暂停、悬停控件条、拖动进度条、全屏
// 过渡中）。点击与按键另走 touch。
func (s *videoChromeIdle) observe(now time.Time, px, py float32, over, hold bool) videoChromeFrame {
	if !s.seen {
		s.seen = true
		s.px, s.py, s.over = px, py, over
		s.last, s.frame, s.alpha = now, now, 1
		return videoChromeFrame{Alpha: 1}
	}
	// 指针在窗内移动、或刚从外面进来，都算「动过」。指针在窗外时位置是
	// 上一次的读数，不能拿它判静止（否则鼠标移到别的窗口上，控件条永远不藏）。
	moving := over && (!s.over || px != s.px || py != s.py)
	s.px, s.py, s.over = px, py, over
	if moving || hold {
		s.last = now
	}
	dt := now.Sub(s.frame)
	s.frame = now

	target := float32(1)
	if !hold && now.Sub(s.last) >= videoChromeIdleDelay {
		target = 0
	}
	if s.alpha == target {
		if target == 1 && !hold {
			// 还亮着、在等静置计时：到点前不必逐帧。
			return videoChromeFrame{Alpha: 1, Wake: videoChromeIdleDelay - now.Sub(s.last)}
		}
		return videoChromeFrame{Alpha: s.alpha}
	}
	dur := videoChromeHideFade
	if target > s.alpha {
		dur = videoChromeShowFade
	}
	step := float32(1)
	if dt > 0 && dur > 0 {
		step = float32(dt) / float32(dur)
	}
	if target > s.alpha {
		s.alpha = minF(s.alpha+step, 1)
	} else {
		s.alpha = maxFloat32(s.alpha-step, 0)
	}
	if s.alpha == target {
		if target == 1 && !hold {
			return videoChromeFrame{Alpha: 1, Wake: videoChromeIdleDelay}
		}
		return videoChromeFrame{Alpha: s.alpha}
	}
	return videoChromeFrame{Alpha: s.alpha, Animating: true}
}

// touch 记一次用户交互（点击 / 按键）：控件条立刻浮出并重新计时。
func (s *videoChromeIdle) touch(now time.Time) {
	s.last = now
}

// ---------------------------------------------------------------- 单击 / 双击

// videoClickArbiter 把画面上的单击与双击分开。mygo 的双击**两次都报
// Clicked()**（第二下走 quietClick，见 ui/input.go 的 pointerUp），所以不能
// 见到 Clicked 就切播放：单击先挂起，等 videoDoubleClick；双击到了就撤销挂起
// 的那一下，只切全屏（原版 `<video>` 的浏览器行为正是如此）。
type videoClickArbiter struct {
	pending bool
	at      time.Time
}

func (s *videoClickArbiter) single(now time.Time) {
	s.pending, s.at = true, now
}

// double 撤销挂起的单击并报告刚才有没有挂起。
func (s *videoClickArbiter) double() bool {
	had := s.pending
	s.pending = false
	return had
}

// due 报告挂起的单击是否该执行了。
func (s *videoClickArbiter) due(now time.Time) bool {
	if !s.pending || now.Sub(s.at) < videoDoubleClick {
		return false
	}
	s.pending = false
	return true
}

// ---------------------------------------------------------------- 全屏

// videoFullScreen 是全屏的状态机：记住窗口态的尺寸与位置，向窗口发出请求，
// 等窗口确认（异步平台），退出后按需恢复原尺寸。
//
// 三件事都由它负责：
//   - **不抖动**：请求发出后到窗口确认之前（inFlight）再来的切换一律忽略。
//     双击是两次快速点击，连点两下就是两次切换请求；不闩住的话
//     SetFullScreen(true/false) 会在同几帧里互相打断，macOS 上表现为窗口
//     抖两下、Linux 上表现为「点了没反应」。
//   - **恢复原尺寸与位置**：进入前记 Bounds()；确认退出后若当前 bounds 与
//     记下的不同（Wayland 合成器不还原、或用户在别的显示器上），SetBounds
//     补一次。Windows 的框架自己恢复 placement、macOS 的 AppKit 自己保留
//     frame，两处这个兜底都不会真的调用。
//   - **过渡**：每次接受的切换都把黑幕的淡出重新计时（见 fade）。
type videoFullScreen struct {
	full      bool // 窗口确认过的状态
	requested bool // 正在请求的目标
	inFlight  bool
	since     time.Time
	saved     mygo.Rectangle
	haveSaved bool
	fadeAt    time.Time
}

// toggle 请求切换全屏。返回 false 表示被闩住（过渡中）或没有窗口可用。
func (f *videoFullScreen) toggle(now time.Time, w VideoWindow) bool {
	if w == nil || f.inFlight {
		return false
	}
	next := !f.full
	if next {
		f.saved, f.haveSaved = w.Bounds(), true
	}
	f.requested, f.inFlight, f.since = next, true, now
	f.fadeAt = now
	w.SetFullScreen(next)
	return true
}

// exit 请求退出全屏（Esc）。窗口态时返回 false，调用方据此决定是否关窗。
func (f *videoFullScreen) exit(now time.Time, w VideoWindow) bool {
	if w == nil || f.full == false && !f.requested {
		return false
	}
	if f.inFlight || !f.full {
		return true // 已经在退：当作处理过
	}
	f.requested, f.inFlight, f.since = false, true, now
	f.fadeAt = now
	w.SetFullScreen(false)
	return true
}

// observe 读窗口的实际状态，处理确认、超时与退出后的尺寸恢复。
func (f *videoFullScreen) observe(now time.Time, w VideoWindow) {
	if w == nil {
		return
	}
	actual := w.IsFullScreen()
	if actual != f.full {
		f.full = actual
		f.inFlight = false
		f.requested = actual
		if !actual && f.haveSaved {
			// 平台自己还原时这里什么都不做（bounds 已经相等）。
			if b := w.Bounds(); b != f.saved {
				w.SetBounds(f.saved)
			}
			f.haveSaved = false
		}
		return
	}
	if f.inFlight && now.Sub(f.since) > videoFullScreenWait {
		// 窗口一直没确认（Wayland 合成器拒绝 / 平台不支持）：按实际状态收尾。
		f.inFlight = false
		f.requested = f.full
		if !f.full && f.haveSaved {
			f.haveSaved = false
		}
	}
}

// fade 是黑幕这一帧的不透明度：1 → 0，盖住窗口尺寸变化的那一下。
func (f *videoFullScreen) fade(now time.Time) float32 {
	if f.fadeAt.IsZero() {
		return 0
	}
	d := now.Sub(f.fadeAt)
	if d >= videoFullScreenFade {
		return 0
	}
	if d < 0 {
		return videoFullScreenFadeMax
	}
	return videoFullScreenFadeMax * (1 - float32(d)/float32(videoFullScreenFade))
}

func (f *videoFullScreen) fading(now time.Time) bool { return f.fade(now) > 0 }

// ---------------------------------------------------------------- 进度

// videoSeek 是进度条的状态：拖动期间滑块停在用户手上的值，松手后跟随播放。
type videoSeek struct {
	value float64
	drag  bool
}

// ---------------------------------------------------------------- 快捷键

// videoKeyAction 是视频窗里一个键要做的动作。
type videoKeyAction int

const (
	videoKeyNone videoKeyAction = iota
	videoKeyPlayPause
	videoKeyBack
	videoKeyForward
	videoKeyVolumeUp
	videoKeyVolumeDown
	videoKeyMute
	videoKeyFullScreen
	videoKeyExitFullScreen
	videoKeyClose
	videoKeyMinimize
	videoKeyQuit
)

// repeats 报告按住不放时是否连续执行。音量是**步进**（与原生 range 一致），
// 其余都是开关或一次性动作 —— 自动重复会让暂停/播放来回切、全屏反复进出。
func (a videoKeyAction) repeats() bool {
	return a == videoKeyVolumeUp || a == videoKeyVolumeDown
}

// VideoKeyBinding 是一条键位：注册用（外壳按它逐条挂 handler），也供测试
// 逐条核对「哪个键归谁」。
type VideoKeyBinding struct {
	Mods ui.Modifiers
	Key  ui.Key
}

// VideoKeyBindings 是视频窗监听的全部键位。主窗的 shortcuts.go 注册在**主窗
// 自己的 ui.Context** 上，而 mygo 的快捷键是每个窗口一份（ui/actions.go 的
// action.window + engine.actions），所以两套键互不可见：
//
//	键          视频窗（本表）        主窗（shortcuts.go）
//	空格        播放 / 暂停           播放 / 暂停（音频）
//	← →         倒退 / 前进 10s       上一集 / 下一集（视频窗开着时主窗 return）
//	↑ ↓         音量 ±5%              音量 ±5%（视频窗开着时主窗 return）
//	M           静音                  无
//	F           全屏                  无
//	Esc         退全屏 / 关视频窗     关抽屉 / 关面板（视频窗开着时主窗转给 CloseVideo）
//	Cmd/Ctrl+W  最小化视频窗          最小化主窗
//	Cmd/Ctrl+Q  退出应用              退出应用
//
// 结论：**焦点在视频窗时视频窗全胜**，因为主窗根本没收到这些键事件；唯一
// 跨窗口的是 macOS 原生菜单里的 Cmd+Q 加速键（应用级），它与视频窗自己的
// Cmd+Q 是同一个动作（退出），谁先到都一样。Esc 是唯一「同一个键在两个窗口
// 有不同含义」的一条，而两边不会同时收到它。
var VideoKeyBindings = []VideoKeyBinding{
	{0, ui.KeySpace},
	{0, ui.KeyLeft},
	{0, ui.KeyRight},
	{0, ui.KeyUp},
	{0, ui.KeyDown},
	{0, ui.KeyM},
	{0, ui.KeyF},
	{0, ui.KeyEscape},
	{ui.Cmd, ui.KeyW},
	{ui.Cmd, ui.KeyQ},
}

// videoKeyFor 把一次按键翻成动作。full 是全屏状态：Esc 在全屏时先退全屏，
// 窗口态时才关窗（原版 Esc 直接关视频浮层；原生版多了全屏，误关一整集的
// 代价比多按一次高，所以 Esc 分层：先退全屏、再关窗）。
func videoKeyFor(mods ui.Modifiers, key ui.Key, full bool) videoKeyAction {
	switch {
	case mods == ui.Cmd && key == ui.KeyW:
		return videoKeyMinimize
	case mods == ui.Cmd && key == ui.KeyQ:
		return videoKeyQuit
	case mods != 0:
		// 带别的修饰键（Cmd+←、Shift+Space…）不抢：留给系统 / 未来的输入。
		return videoKeyNone
	}
	switch key {
	case ui.KeySpace:
		return videoKeyPlayPause
	case ui.KeyLeft:
		return videoKeyBack
	case ui.KeyRight:
		return videoKeyForward
	case ui.KeyUp:
		return videoKeyVolumeUp
	case ui.KeyDown:
		return videoKeyVolumeDown
	case ui.KeyM:
		return videoKeyMute
	case ui.KeyF:
		return videoKeyFullScreen
	case ui.KeyEscape:
		if full {
			return videoKeyExitFullScreen
		}
		return videoKeyClose
	}
	return videoKeyNone
}

// ---------------------------------------------------------------- 视图

// videoChromeLayout 是一帧的排布结论（纯函数，测试直接钉）。
type videoChromeLayout struct {
	// TwoRows 表示进度条独占一行（窄窗）。
	TwoRows bool
	// VolumeBar 表示控件条里带音量滑块（静音键始终在）。
	VolumeBar bool
}

func videoChromeLayoutFor(w float32) videoChromeLayout {
	switch {
	case w < videoChromeNarrow:
		return videoChromeLayout{TwoRows: true}
	case w < videoChromeWide:
		return videoChromeLayout{}
	default:
		return videoChromeLayout{VolumeBar: true}
	}
}

// View 画出整个视频窗：画面铺满，标题栏与控件条浮在画面上，按自动隐藏的
// 状态淡入淡出，全屏切换时压一层黑幕做过渡。
func (v *VideoChrome) View(c *ui.Context) {
	now := c.Now()
	v.init(now)
	t := v.Theme
	c.SetTheme(t.uiTheme())

	root := c.Root()
	root.Background(ui.Hex("#000000"))

	// 全屏状态以窗口为准（用户也可能用系统手势 / F11 进出）。
	v.fs.observe(now, v.Win)
	full := v.fs.full

	px, py, over := root.PointerPosition()
	frame := v.idle.observe(now, px, py, over, v.holdChrome(full))
	alpha := frame.Alpha
	// 上一帧的悬停结论已经用掉，这一帧的两条栏重新读。
	v.barHover = false

	// 画面。Content 为 nil 时留一块黑底（无头测试与「播放器还没起来」）。
	if v.Content != nil {
		v.Content(c)
	} else {
		ui.Box(c).Fill().Background(ui.Hex("#000000"))
	}

	// 画面上的单击 / 双击。它铺满整窗、在控件条之下：控件条可见时点键不会
	// 落到这里，控件条收起时（IgnoreInput）点击正好穿到画面上。
	pic := ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).Label("视频画面")
	if pic.DoubleClicked() {
		v.clicks.double()
		v.toggleFullScreen(now)
	} else if pic.Clicked() {
		v.clicks.single(now)
	}
	if v.clicks.due(now) {
		v.togglePlay()
	}

	v.titleBar(c, t, alpha, full)
	v.controlBar(c, t, alpha, full)

	// 全屏进出的黑幕（画在控件条之上，连它们一起过渡）。
	if f := v.fs.fade(now); f > 0 {
		ui.Box(c).Absolute().Left(0).Right(0).Top(0).Bottom(0).
			Background(ui.Hex("#000000")).Opacity(f).IgnoreInput(true)
	}

	v.bindKeys(c, full)

	switch {
	case frame.Animating || v.fs.fading(now):
		c.AnimationFrame()
	case frame.Wake > 0:
		c.After(frame.Wake)
	}
	if v.clicks.pending {
		c.After(videoDoubleClick)
	}
	if v.Pl.Buffering() {
		// 缓冲中的呼吸描边要逐帧重画。
		c.AnimationFrame()
	}
}

// init 补齐零值：主题按当前时段取，倍速落在合法档位上，播放器缺失时换成
// 一个空实现（无头测试只画布局，不碰媒体框架）。
func (v *VideoChrome) init(now time.Time) {
	if v.inited {
		return
	}
	v.inited = true
	if v.Theme.Mode == "" && v.Theme.Ink.A == 0 {
		v.Theme = ThemeAt(now)
	}
	if v.Speed <= 0 {
		v.Speed = 1
	}
	if v.Pl == nil {
		v.Pl = nopVideoPlayer{}
	}
}

// nopVideoPlayer 是「还没有播放器」时的空实现：视图、快捷键与布局照常
// 工作，只是什么都不播。
type nopVideoPlayer struct{}

func (nopVideoPlayer) Play()                   {}
func (nopVideoPlayer) Pause()                  {}
func (nopVideoPlayer) Playing() bool           { return false }
func (nopVideoPlayer) Ended() bool             { return false }
func (nopVideoPlayer) Ready() bool             { return false }
func (nopVideoPlayer) Buffering() bool         { return false }
func (nopVideoPlayer) Duration() time.Duration { return 0 }
func (nopVideoPlayer) Position() time.Duration { return 0 }
func (nopVideoPlayer) Seek(time.Duration)      {}
func (nopVideoPlayer) SetRate(float64)         {}
func (nopVideoPlayer) SetVolume(float64)       {}
func (nopVideoPlayer) SetMuted(bool)           {}

// holdChrome 是「不许淡出」的条件：暂停 / 播完 / 缓冲中（用户在看状态）、
// 指针停在控件条上、正在拖进度条、全屏切换还没落定。
func (v *VideoChrome) holdChrome(full bool) bool {
	switch {
	case v.Pl == nil:
		return true
	case !v.Pl.Playing(), v.Pl.Buffering(), v.seek.drag, v.barHover, v.fs.inFlight:
		return true
	}
	return false
}

// titleBar 是顶部那条：标题（单行省略）+ 拖动区 + 最小化 / 关闭。
func (v *VideoChrome) titleBar(c *ui.Context, t Theme, alpha float32, full bool) {
	bar := ui.Row(c).Absolute().Left(videoChromePad).Right(videoChromePad).Top(videoChromePad).
		Height(videoChromeTitleH).Padding(0, 8).Radius(videoChromeRadius).
		AlignItems(ui.Center).Gap(videoChromeGap).DragWindow()
	bar = t.videoBar(bar)
	bar.Opacity(alpha)
	if alpha <= 0.01 {
		bar.IgnoreInput(true)
	}
	bar.Children(func() {
		ui.Text(c, v.Title).FontSize(12.5).TextColor(t.Ink).
			Grow(1).SingleLine().Ellipsis("…")
		if !full {
			if b := t.videoKey(c, "video-min", "最小化", "最小化", iconMinus, false, false); b.Clicked() {
				v.minimize()
			}
		}
		if b := t.videoKey(c, "video-close-title", "关闭视频", "关闭视频", iconClose, false, false); b.Clicked() {
			v.close()
		}
	})
	v.barHover = v.barHover || bar.Hovered() || bar.FocusWithin()
}

// controlBar 是底部那条：播放 / 时间 / 进度 / 倍速 / 弹幕 / 音量 / 全屏 / 关闭。
func (v *VideoChrome) controlBar(c *ui.Context, t Theme, alpha float32, full bool) {
	w, _ := c.Size()
	lo := videoChromeLayoutFor(w)
	var bar ui.Element
	if lo.TwoRows {
		bar = ui.Column(c).Absolute().Left(videoChromePad).Right(videoChromePad).Bottom(videoChromePad).
			Height(videoChromeBarTwoH).Padding(4, 10).Radius(videoChromeRadius).
			Gap(2)
	} else {
		bar = ui.Row(c).Absolute().Left(videoChromePad).Right(videoChromePad).Bottom(videoChromePad).
			Height(videoChromeBarH).Padding(0, 10).Radius(videoChromeRadius).
			AlignItems(ui.Center).Gap(videoChromeGap)
	}
	bar = t.videoBar(bar)
	bar.Opacity(alpha)
	if alpha <= 0.01 {
		bar.IgnoreInput(true)
	}
	bar.Children(func() {
		if lo.TwoRows {
			v.controlRows(c, t, full)
			return
		}
		v.controlRow(c, t, lo, full)
	})
	v.barHover = v.barHover || bar.Hovered() || bar.FocusWithin()
}

// controlRow 是宽窗的一行排布：所有控件在同一行。
func (v *VideoChrome) controlRow(c *ui.Context, t Theme, lo videoChromeLayout, full bool) {
	v.playKey(c, t)
	v.timeText(c, t, v.Pl.Position().Seconds(), videoChromeTimeW)
	v.progress(c, t)
	v.timeText(c, t, v.Pl.Duration().Seconds(), videoChromeTimeW)
	v.speedKey(c, t)
	v.danmakuKey(c, t)
	v.volumeKey(c, t)
	if lo.VolumeBar {
		v.volumeSlider(c, t)
	}
	v.fullScreenKey(c, t, full)
	if b := t.videoKey(c, "video-close", "关闭视频", "关闭视频", iconClose, false, false); b.Clicked() {
		v.close()
	}
}

// controlRows 是窄窗的两行排布：进度独占一行，键在第二行。
func (v *VideoChrome) controlRows(c *ui.Context, t Theme, full bool) {
	ui.Row(c).FillWidth().Height(timelineH).AlignItems(ui.Center).Gap(videoChromeGap).Children(func() {
		v.timeText(c, t, v.Pl.Position().Seconds(), videoChromeTimeW)
		v.progress(c, t)
		v.timeText(c, t, v.Pl.Duration().Seconds(), videoChromeTimeW)
	})
	ui.Row(c).FillWidth().Height(VideoChromeKey).AlignItems(ui.Center).Gap(videoChromeGap).Children(func() {
		v.playKey(c, t)
		v.speedKey(c, t)
		v.danmakuKey(c, t)
		v.volumeKey(c, t)
		ui.Box(c).Grow(1)
		v.fullScreenKey(c, t, full)
		if b := t.videoKey(c, "video-close", "关闭视频", "关闭视频", iconClose, false, false); b.Clicked() {
			v.close()
		}
	})
}

// playKey 是播放 / 暂停键（主键，与主窗一样用实心玻璃 + 强调色）。
func (v *VideoChrome) playKey(c *ui.Context, t Theme) {
	ic := iconPlayOne
	tip := "播放"
	if v.Pl.Playing() {
		ic, tip = iconPause, "暂停"
	}
	b := t.videoKey(c, "video-play", tip, tip, ic, true, false)
	if b.Clicked() {
		v.togglePlay()
	}
}

// progress 是进度条：已播 + 缓冲状态 + 可拖动。外观与主窗播放栏的进度条
// **同一份绘制代码**（paintTimeline），尺寸与交互（SliderBase 的拖动 / 方向键）
// 也一致。
func (v *VideoChrome) progress(c *ui.Context, t Theme) {
	dur := v.Pl.Duration().Seconds()
	pos := v.Pl.Position().Seconds()
	if !v.seek.drag {
		v.seek.value = clampF(pos, 0, maxF64(dur, 0))
	}
	hi := maxF64(dur, 0.1)
	s := ui.SliderBase(c, &v.seek.value, 0, hi).Grow(1).Height(timelineH).
		PaddingX(timelineInset).Label("播放进度").Step(0.1)
	s.Disabled(dur <= 0).DimDisabled(false)
	if dur <= 0 {
		s.Opacity(progressDisabledOpacity)
	}
	focusOutline(s, 0, focusStudio)
	_, _, held := s.Dragged()
	v.seek.drag = held
	if s.Changed() {
		v.seekTo(v.seek.value)
	}
	frac := float32(0)
	if dur > 0 {
		frac = float32(clampF(v.seek.value/dur, 0, 1))
	}
	rp := rangePaint{
		Frac:       frac,
		Hovered:    s.Hovered(),
		Held:       held,
		Dark:       t.Dark,
		Colors:     videoRangeColors(t),
		Geom:       timelineGeom,
		ThumbScale: s.AnimateTo("video-thumb", rangeThumbScale(s.Hovered(), held), thumbScaleAnim),
	}
	skin := videoRangeSkin(t, s.Hovered())
	pulse := videoBufferingPulse(c.Now(), v.Pl.Buffering())
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		paintVideoTimeline(p, r, rp, skin, pulse, t.Blue)
	})
}

// videoRangeColors 是视频窗进度条 / 音量条这一帧的（描边, 底色）。
//
// 主窗那对令牌（--player-track-border / --player-track-rest）是**给半透明
// 玻璃**的：浅色时段是白 .68 / 白 .32，压在视频窗这块白面板（Panel，白 .88）
// 上就是白上白 —— 轨道整个看不见。所以浅色下换成墨色的低透明度；深色时段
// 面板本来就是深的，直接用原令牌。
func videoRangeColors(t Theme) rangeColors {
	if t.Dark {
		return rangeColorsFor(t)
	}
	return rangeColors{Border: t.Ink.Alpha(0.20), Rest: t.Ink.Alpha(0.10)}
}

// videoRangeSkin 是视频窗那条输入条的材质（轨道内影 / 滑块描边底色高光）。
// 与主窗共用同一份绘制代码（paintRangeFill / paintRangeThumb），只把浅色那套
// 白描边换成能压在白面板上的值：滑块底加深到 #bae6fd 实色、描边转白 .95、
// 外投影加重（否则 14px 的滑块在白面板上像一块没边的白斑）。
func videoRangeSkin(t Theme, hovered bool) rangeSkin {
	s := rangeSkinFor(t.Dark, hovered)
	if t.Dark {
		return s
	}
	s.TrackTop = ui.Hex("#ffffff").Alpha(0.92)
	s.TrackBottom = shadowInk.Alpha(0.05)
	s.ThumbBorder = ui.Hex("#ffffff").Alpha(0.95)
	s.ThumbFill = ui.Hex("#bae6fd").Alpha(0.96)
	s.ThumbGlow = ui.Hex("#0284c7").Alpha(0.32)
	s.ThumbBottom = ui.Hex("#0284c7").Alpha(0.16)
	return s
}

// paintVideoTimeline 画视频窗的进度条：轨道 + 已播 + 滑块（+ 缓冲呼吸环）。
//
// 与主窗的 paintTimeline 共用轨道 / 填充 / 滑块三块绘制，但**没有**章节刻线、
// 恰饭段与预读缓冲层 —— 那三样数据 ui/video 一个都不提供（只有 Buffering()
// 一个布尔），画出来就是编的。皮肤也单独调过（见 videoRangeSkin）。
func paintVideoTimeline(p *ui.Painter, r ui.Rect, rp rangePaint, s rangeSkin, buffering float32, accent ui.Color) {
	track := rangeTrackRect(r, rp.Geom)
	paintRangeShadow(p, track, rp.Hovered, s)
	paintRangeFill(p, track, rp.Frac, rp.Colors, s)
	paintRangeThumb(p, rangeThumbX(track, rp.Geom, rp.Frac), r.Y+r.H/2, rp.Hovered, rp.Held, rp.ThumbScale, s)
	if buffering > 0 {
		// 缓冲中：轨道上叠一圈呼吸的强调色。ui/video 只给 Buffering() 一个
		// 布尔（没有可预读的字节范围），所以这里画的是「正在缓冲」的状态，
		// 不是第二段进度 —— 不假装有一份我们拿不到的数据。
		p.Stroke(track, accent.Alpha(buffering), timelineTrackH/2, 1)
	}
}

// videoBufferingPulse 是缓冲描边的呼吸不透明度（0.25 ~ 0.60，约 1.4s 一周期）。
func videoBufferingPulse(now time.Time, buffering bool) float32 {
	if !buffering {
		return 0
	}
	phase := float64(now.UnixNano()%int64(1400*time.Millisecond)) / float64(1400*time.Millisecond)
	return 0.25 + 0.35*float32(0.5+0.5*math.Sin(phase*2*math.Pi))
}

// volumeKey 是静音键（音量 0 / 静音时显示带斜杠的喇叭，与主窗同一条判据）。
func (v *VideoChrome) volumeKey(c *ui.Context, t Theme) {
	muted := v.Muted || v.Volume <= 0
	tip := "静音"
	if muted {
		tip = "取消静音"
	}
	b := t.videoKey(c, "video-volume", tip, tip, volumeIconFor(muted), false, muted)
	if b.Clicked() {
		v.toggleMute()
	}
}

// volumeSlider 是音量滑块（与主窗的音量条同一份绘制代码）。
func (v *VideoChrome) volumeSlider(c *ui.Context, t Theme) {
	val := clampF(v.Volume, 0, 1)
	s := ui.SliderBase(c, &val, 0, 1).Width(videoChromeVolW).Height(timelineH).
		Label("音量").Step(videoVolumeStep)
	focusOutline(s, 0, focusStudio)
	_, _, held := s.Dragged()
	if s.Changed() {
		v.setVolume(val)
	}
	rp := rangePaint{
		Frac:       float32(clampF(val, 0, 1)),
		Hovered:    s.Hovered(),
		Held:       held,
		Dark:       t.Dark,
		Colors:     videoRangeColors(t),
		Geom:       volumeGeom,
		ThumbScale: rangeThumbScale(s.Hovered(), held),
	}
	skin := videoRangeSkin(t, s.Hovered())
	s.Draw(func(p *ui.Painter, r ui.Rect) { paintVideoTimeline(p, r, rp, skin, 0, t.Blue) })
}

// speedKey 是倍速键：点一下循环到下一档（与主窗的档位一致），文案 3 位有效数字。
func (v *VideoChrome) speedKey(c *ui.Context, t Theme) {
	label := videoSpeedLabel(v.Speed)
	b := t.videoTextKey(c, "video-speed", label, "倍速 · "+label, false)
	if b.Clicked() {
		v.Speed = videoSpeedStep(v.Speed, VideoSpeeds)
		v.Pl.SetRate(v.Speed)
	}
}

// danmakuKey 是弹幕开关位。本轮只留键与状态（下一轮接数据）。
func (v *VideoChrome) danmakuKey(c *ui.Context, t Theme) {
	tip := "弹幕 · 已开"
	if !v.Danmaku {
		tip = "弹幕 · 已关"
	}
	b := t.videoKey(c, "video-danmaku", "弹幕", tip, iconDanmaku, false, v.Danmaku)
	if b.Clicked() {
		v.Danmaku = !v.Danmaku
	}
}

// fullScreenKey 是全屏键（全屏时换成「退出全屏」的四角内收图标）。
func (v *VideoChrome) fullScreenKey(c *ui.Context, t Theme, full bool) {
	ic, tip := iconFullScreen, "全屏"
	if full {
		ic, tip = iconExitFullScreen, "退出全屏"
	}
	b := t.videoKey(c, "video-fullscreen", tip, tip, ic, false, full)
	if b.Clicked() {
		v.toggleFullScreen(c.Now())
	}
}

// timeText 是一枚时间标签。定宽，避免秒数进位时整条控件条跟着抖。
func (v *VideoChrome) timeText(c *ui.Context, t Theme, sec float64, width float32) {
	ui.Text(c, clockText(sec)).FontSize(11).Width(width).Shrink(0).
		TextAlign(ui.Center).TextColor(t.Muted)
}

// ---------------------------------------------------------------- 材质

// videoBar 给一条浮在画面上的控件条铺材质：应用的面板令牌 + 1px 线 + 投影。
//
// 深浅两态都用 Theme 的令牌：浅色外观是白面板（Ink 深墨），深色外观是深
// 面板（Ink 浅墨）。视频画面永远是深色，面板必须自己提供对比，所以它是不透明
// 度较高的 Panel（白 .88 / #152238 .92）而不是播放栏那种薄玻璃。
func (t Theme) videoBar(bar ui.Element) ui.Element {
	// 半透明面板 + 1px 线 + 顶沿内高光：原生界面画不了 backdrop-filter，
	// 这就是本仓库「玻璃」的三件套（与抽屉面板 / 播放栏同一个近似法）。
	bar.Background(t.Panel).Border(1, t.Line).InsetShadow(0, 1, 1, 0, t.Highlight)
	if t.Dark {
		bar.Shadow(0, 10, 28, 0, shadowInk.Alpha(0.42))
		return bar
	}
	bar.Shadow(0, 8, 24, 0, shadowInk.Alpha(0.22))
	return bar
}

// videoKeyPaint 是一颗键在一态下的材质。
type videoKeyPaint struct {
	Fill   ui.Color
	Border ui.Color
	Inset  ui.Color
	Ink    ui.Color
	Shadow ui.Color
}

// videoKeyPaintFor 解析键的材质。四态：常态 / 悬停 / 按下 / 激活（弹幕开、
// 静音、全屏中）。浅色外观用深墨的低透明度填充（白面板上白玻璃看不见），
// 深色外观用 Glass 三档 + Highlight 内高光 —— 与抽屉表头 / 播放栏的键族
// 同一套思路，只是底色换成面板。
func (t Theme) videoKeyPaintFor(hovered, pressed, active bool) videoKeyPaint {
	if t.Dark {
		p := videoKeyPaint{Fill: t.Glass, Border: t.GlassBorder, Inset: t.Highlight, Ink: t.Ink}
		switch {
		case active:
			p.Fill, p.Border, p.Ink = t.Blue.Alpha(0.22), t.Blue.Alpha(0.60), t.Blue
		case hovered:
			p.Fill, p.Border, p.Ink = t.GlassHover, t.GlassBorderBright, t.Blue
		}
		if pressed {
			p.Fill, p.Ink = t.GlassActive, t.Blue
		}
		return p
	}
	p := videoKeyPaint{Fill: t.Ink.Alpha(0.06), Border: t.Line, Ink: t.Muted}
	switch {
	case active:
		p.Fill, p.Border, p.Ink = t.Blue.Alpha(0.14), t.Blue.Alpha(0.45), t.Blue
	case hovered:
		p.Fill, p.Border, p.Ink = t.Blue.Alpha(0.10), t.Blue.Alpha(0.32), t.Blue
		p.Shadow = shadowInk.Alpha(0.06)
	}
	if pressed {
		p.Fill, p.Ink = t.Ink.Alpha(0.10), t.Blue
	}
	return p
}

// videoKey 画一颗 28×28 的图标键：主窗那套形状（圆角 8、1px 描边、悬停 /
// 按下 / 激活三档、140ms 过渡、按压 0.94、tooltip、焦点环）。primary 为真时
// 用强调色的实心键（播放键）。
func (t Theme) videoKey(c *ui.Context, key any, label, tip string, ic *ui.SVG, primary, active bool) ui.Element {
	return t.videoKeyInto(c, key, label, tip, primary, active, func(ink ui.Color) {
		ui.Icon(c, ic).Size(15, 15).TextColor(ink)
	})
}

// videoTextKey 是同一套形状的文字键（倍速）。
func (t Theme) videoTextKey(c *ui.Context, key any, text, tip string, active bool) ui.Element {
	return t.videoKeySized(c, key, text, tip, videoChromeTextKeyW, false, active, func(ink ui.Color) {
		ui.Text(c, text).FontSize(11).TextColor(ink)
	})
}

func (t Theme) videoKeyInto(c *ui.Context, key any, label, tip string, primary, active bool, build func(ink ui.Color)) ui.Element {
	return t.videoKeySized(c, key, label, tip, VideoChromeKey, primary, active, build)
}

func (t Theme) videoKeySized(c *ui.Context, key any, label, tip string, width float32, primary, active bool, build func(ink ui.Color)) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Size(width, VideoChromeKey).Radius(Radius).Center().
		Label(label).Tooltip(tip)
	st := headerStateOf(b, active)
	p := t.videoKeyPaintFor(st.Hovered, st.Pressed, st.Selected)
	if primary {
		// 播放键是控件条上的主键：常态就用强调色的淡底 + 蓝边，悬停再加一档。
		p.Fill, p.Border = t.Blue.Alpha(0.16), t.Blue.Alpha(0.42)
		p.Ink = t.Blue
		if st.Hovered {
			p.Fill, p.Border = t.Blue.Alpha(0.26), t.Blue.Alpha(0.60)
		}
		if st.Pressed {
			p.Fill = t.Blue.Alpha(0.12)
		}
	}
	b.Background(headerFade(c, b, "video-key-fill", p.Fill, headerAnim)).
		Border(1, headerFade(c, b, "video-key-border", p.Border, headerAnim))
	if p.Inset.A > 0 {
		b.InsetShadow(0, 1, 1, 0, p.Inset)
	}
	if p.Shadow.A > 0 {
		b.Shadow(0, 2, 6, 0, p.Shadow)
	}
	pressScale(b)
	focusOutline(b, Radius, t.headerFocusRing())
	ink := headerFade(c, b, "video-key-ink", p.Ink, headerAnim)
	b.Children(func() { build(ink) })
	return b
}

// ---------------------------------------------------------------- 动作

func (v *VideoChrome) togglePlay() {
	switch {
	case v.Pl == nil:
	case v.Pl.Playing():
		v.Pl.Pause()
	case v.Pl.Ended():
		v.Pl.Seek(0)
		v.Pl.Play()
		if v.OnReplay != nil {
			v.OnReplay()
		}
	default:
		v.Pl.Play()
	}
}

func (v *VideoChrome) seekTo(sec float64) {
	if v.Pl == nil {
		return
	}
	dur := v.Pl.Duration().Seconds()
	v.Pl.Seek(time.Duration(clampF(sec, 0, maxF64(dur, sec)) * float64(time.Second)))
}

// nudge 按 ← / → 走一步（原版的 10s）。
func (v *VideoChrome) nudge(delta float64) {
	if v.Pl == nil {
		return
	}
	target := v.Pl.Position().Seconds() + delta
	dur := v.Pl.Duration().Seconds()
	if dur > 0 && target > dur {
		target = dur
	}
	v.seek.value = clampF(target, 0, maxF64(dur, 0))
	v.seek.drag = false
	v.seekTo(target)
}

func (v *VideoChrome) toggleMute() {
	v.Muted = !v.Muted
	if v.Pl != nil {
		v.Pl.SetMuted(v.Muted)
	}
}

func (v *VideoChrome) setVolume(x float64) {
	v.Volume = clampF(x, 0, 1)
	if v.Pl != nil {
		v.Pl.SetVolume(v.Volume)
	}
	if v.Volume > 0 && v.Muted {
		v.Muted = false
		if v.Pl != nil {
			v.Pl.SetMuted(false)
		}
	}
}

func (v *VideoChrome) nudgeVolume(delta float64) {
	v.setVolume(v.Volume + delta)
}

func (v *VideoChrome) toggleFullScreen(now time.Time) {
	v.fs.toggle(now, v.Win)
}

func (v *VideoChrome) close() {
	if v.OnClose != nil {
		v.OnClose()
	}
}

func (v *VideoChrome) minimize() {
	if v.OnMinimize != nil {
		v.OnMinimize()
	}
}

// bindKeys 注册视频窗的快捷键。Esc 的语义按全屏状态分层（见 videoKeyFor）。
func (v *VideoChrome) bindKeys(c *ui.Context, full bool) {
	for _, b := range VideoKeyBindings {
		mods, key := b.Mods, b.Key
		c.OnWindowShortcut(mods, key, func() {
			a := videoKeyFor(mods, key, full)
			if a == videoKeyNone {
				return
			}
			if c.ShortcutRepeated() && !a.repeats() {
				return
			}
			v.idle.touch(c.Now())
			v.do(a, c.Now())
		})
	}
}

// do 执行一个键位动作。
func (v *VideoChrome) do(a videoKeyAction, now time.Time) {
	switch a {
	case videoKeyPlayPause:
		v.togglePlay()
	case videoKeyBack:
		v.nudge(-videoSeekStep)
	case videoKeyForward:
		v.nudge(videoSeekStep)
	case videoKeyVolumeUp:
		v.nudgeVolume(videoVolumeStep)
	case videoKeyVolumeDown:
		v.nudgeVolume(-videoVolumeStep)
	case videoKeyMute:
		v.toggleMute()
	case videoKeyFullScreen:
		v.toggleFullScreen(now)
	case videoKeyExitFullScreen:
		v.fs.exit(now, v.Win)
	case videoKeyClose:
		v.close()
	case videoKeyMinimize:
		v.minimize()
	case videoKeyQuit:
		if v.OnQuit != nil {
			go v.OnQuit()
		}
	}
}

// ---------------------------------------------------------------- 纯函数

// clockText 把秒格式化成播放器的时间文本（m:ss / h:mm:ss）。
func clockText(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	t := int(sec)
	h, m, s := t/3600, (t%3600)/60, t%60
	if h > 0 {
		return strconv.Itoa(h) + ":" + pad2(m) + ":" + pad2(s)
	}
	return strconv.Itoa(m) + ":" + pad2(s)
}

func pad2(v int) string {
	if v < 10 {
		return "0" + strconv.Itoa(v)
	}
	return strconv.Itoa(v)
}

// videoSpeedLabel 是倍速键上的文案（0.5x / 1x / 1.25x / 3x）。
func videoSpeedLabel(v float64) string {
	return strconv.FormatFloat(v, 'g', 3, 64) + "x"
}

// videoSpeedStep 返回下一档倍速（循环）。未命中档位时回首档 —— 原地不动会
// 让按键看起来没反应。
func videoSpeedStep(speed float64, options []float64) float64 {
	if len(options) == 0 {
		return 1
	}
	for i, v := range options {
		if math.Abs(v-speed) < 0.001 {
			return options[(i+1)%len(options)]
		}
	}
	return options[0]
}

// maxFloat32 是 float32 的两值取大（theme.go 的 minF 只有取小）。
func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// maxF64 是 float64 的两值取大（clampF 的 hi 不能小于 lo）。
func maxF64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// 视频窗的两个专属图标（16 画布，与 iconEQ / iconPerson 同一套自绘风格）：
// 全屏是四角外张，退出全屏是四角内收。
var (
	iconFullScreen     = icon(`<path d="M2 6V2h4"/><path d="M10 2h4v4"/><path d="M14 10v4h-4"/><path d="M6 14H2v-4"/>`)
	iconExitFullScreen = icon(`<path d="M6 2v4H2"/><path d="M10 2v4h4"/><path d="M14 10h-4v4"/><path d="M2 10h4v4"/>`)
)
