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
//     标题栏右端，位置与主窗的窗控键对齐。**只此一颗**：控件条不再有关闭键
//     （用户 2026-11 报「上下各有一个关闭按钮」，重复且顶上那颗曾经退出应用）。
//
// 视频窗的两种「离开」（语义见 PLAN.md 的「视频窗关闭语义」一节）：
//
//   - **退出视频播放**（顶栏第四颗键 / 窗口态 Esc）：关视频窗回主窗，是否
//     续播音频按 video.RelayOnClose 判定。**不退应用。**
//   - **最小化**（顶栏第二颗键 / Ctrl+W）：窗口最小化，播放与声音继续
//     （最小化 ≠ 暂停），恢复后画面与进度接续。
//
// 退出应用只能从主窗发起：视频窗里**没有任何一颗按钮**能走到 quit()。

import (
	"math"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/vst93/bili-fm/app/internal/danmaku"
)

// 尺寸（DIP）。控件条与主窗播放栏同一套节奏：播放键 42、工具键 32、
// 倍速列 40、均衡/恰饭列 34、进度条 24、时间列 56、间距 8。顶栏四颗键
// 与主窗标题栏动作键同尺寸（30×30、图标 15）。
const (
	VideoChromeKey    float32 = 30
	videoChromePad    float32 = 10
	videoChromeGap    float32 = 8
	videoChromeTitleH float32 = 36
	videoChromeBarH   float32 = PlayerBarHeight
	videoChromeRadius float32 = 12
	videoChromeTimeW  float32 = 56
	videoChromeVolW   float32 = 68
	// videoChromeBarTwoH 是窄窗两行控件条的高度（4 + 24 + 2 + 42 + 4）：
	// 第一行放进度条（高 timelineH=24），第二行放键（播放键 42）。
	videoChromeBarTwoH float32 = 76
	// 渐进收纳的宽度档（从宽到窄）：
	//   < 900 收起音量滑块（滑块搬进共享音量弹层，键始终在）
	//   < 720 收起总时长
	//   < 560 收起下一集 / 均衡 / 跳过广告（下一集在视频窗本来就没有目标）
	//   < 520 进度条独占一行（两行布局）
	//   < 400 收起当前时间（只留进度条）
	videoChromeVolumeBar  float32 = 900
	videoChromeTimeDur    float32 = 720
	videoChromeKeysNarrow float32 = 560
	videoChromeTwoRows    float32 = 520
	videoChromeTimePos    float32 = 400
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
var VideoSpeeds = speedOptions

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
	// SetAlwaysOnTop 是顶栏「置顶」键的目标，各平台都只是「尽力」（见
	// applyPin）。它与全屏是两条独立的窗口属性，互不覆盖。
	SetAlwaysOnTop(bool)
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

	// OnClose 是「退出视频播放」：关掉视频窗、回主窗（是否续播音频由
	// video.RelayOnClose 决定）。它**不是**退出应用 —— 调用方必须把它接到
	// 「关这个视频窗」，不能接到主窗的 quit() / OnQuit / 窗口关闭回调。
	//
	// OnMinimize 是「最小化」：窗口最小化，播放与声音继续（最小化 ≠ 暂停）。
	// OnQuit 只由 Ctrl/Cmd+Q 触发（系统级退出，与 macOS 应用菜单同一个动作），
	// **任何按钮都不走它**。
	OnClose    func()
	OnMinimize func()
	OnQuit     func()
	// OnReplay 在「播完再点播放」把播放器拉回 0 时调用：窗口那边有一个
	// 「已经报过播完」的闩（endedFired），不在这里清掉，重播这一集播到结尾
	// 就不会再报一次播完（主窗的自动下一集 / 看完标记都不会触发）。
	OnReplay func()

	// Danmaku 是弹幕总开关。与主窗设置菜单的「弹幕」是同一个事实源：
	// 在这里点一下会走 OnDanmakuToggle 回到主窗落盘，再由 internal/video
	// 把状态同步回来。
	Danmaku bool
	// DanmakuOverlay 是这一集的弹幕层（轨道分配 / 滑窗 / 时间轴都在
	// internal/danmaku）。nil 表示没有数据（webview 回退 / 测试桩）。
	DanmakuOverlay *danmaku.Overlay
	// DanmakuSettings 是主窗落盘的基本设置（不透明度 / 字号 / 区域 / 上限）。
	DanmakuSettings danmaku.Settings
	// OnDanmakuToggle 在视频窗内点弹幕键时调用（参数是点击后的开关值）。
	OnDanmakuToggle func(on bool)

	// Pinned 是视频窗的「置顶」（顶栏最前面那颗图钉）。语义与主窗迷你模式
	// 的 Pinned 完全一致：默认 false（迷你模式默认不置顶），切换只改窗口的
	// always-on-top，不碰播放。落盘由调用方在 OnPinToggle 里做（新键
	// videoPinned；迷你模式那颗不落盘，所以两边各记各的）。
	Pinned bool
	// OnPinToggle 在视频窗点「置顶」键时调用（参数是点击后的状态）。
	OnPinToggle func(on bool)

	// Set 是主窗与视频窗共享的设置（音量 / 静音 / 倍速 / 均衡 / 广告跳过）。
	// internal/video 把主窗的 &App.PlayerSettings 传进来，两边是同一份内存：
	// 这里改音量，主窗立刻看见；主窗改倍速，这里下一帧就按新值播。
	// nil 时用本窗口自己的一份（无头测试 / 独立使用），见 settings()。
	Set *PlayerSettings
	// SponsorSegments 是当前这一集的恰饭分段（主窗查询后同步进来）。
	// 视频窗的进度条会画出这些段，开关打开时还会真的跳过。
	SponsorSegments []Segment

	inited bool
	// pinFull 是上一次下发置顶时的全屏状态：全屏进出后要重新下发一次
	// （见 View，DWM / AppKit / GTK 的顶层窗口等级在状态切换时可能被清）。
	pinFull  bool
	idle     videoChromeIdle
	clicks   videoClickArbiter
	fs       videoFullScreen
	seek     videoSeek
	barHover bool

	// 控件条上的两个弹层（与主窗同一份实现）与音量拖动提交的上一帧状态。
	showVolume bool
	showSpeed  bool
	wasVolHeld bool
	// bubble 是进度条的悬停时间气泡（与主窗共用 seekBubbleState.draw）。
	bubble seekBubbleState
	// detachSettings 注销共享设置监听（关窗时调用，避免闭包挂在设置对象上）。
	detachSettings func()
	// lastSponsorSeek 是上一次自动跳过恰饭段的时间：避免 seek 还没落地时
	// 连续几帧重复下发（与主窗的 skipped 表同一个目的，视频窗用时间闩）。
	lastSponsorSeek time.Time

	// dmFrame 是上一帧的时间点：弹幕本地时钟按它推进（暂停时不动）。
	dmFrame time.Time
	// danmakuDrawn 是上一帧真正建出的弹幕文字元素数（无头测试的渲染点
	// 计数；开关关掉时应为 0）。
	danmakuDrawn int
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
	// videoKeyExit 是「退出视频播放」（关窗回主窗），不是退出应用。
	videoKeyExit
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
//	Esc         退全屏 / 退出视频播放 关抽屉 / 关面板（视频窗开着时主窗转给 CloseVideo）
//	Cmd/Ctrl+W  最小化视频窗          最小化主窗
//	Cmd/Ctrl+Q  退出应用              退出应用
//
// 注意：表里唯一能退出应用的是 Cmd/Ctrl+Q（系统级，与 macOS 应用菜单同一个
// 动作）。它是**键盘**，不是按钮 —— 视频窗里没有任何一颗按钮能走到 quit()
// （退出应用只能从主窗发起，见 OnClose 的注释）。
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
		return videoKeyExit
	}
	return videoKeyNone
}

// ---------------------------------------------------------------- 视图

// videoChromeLayout 是一帧的排布结论（纯函数，测试直接钉）。
//
// 收纳顺序按用户要求：窄了先收起**滑块**（音量条搬进共享弹层，键始终在），
// 再收起**文字时间**（先总时长、后当前时间），中间按宽度收掉次要的设置键。
// 任何一档都不允许控件溢出窗口或被裁切。
type videoChromeLayout struct {
	// TwoRows 表示进度条独占一行（窄窗）。
	TwoRows bool
	// VolumeBar 表示控件条里带内联音量滑块（只在最宽的一档出现）。
	VolumeBar bool
	// TimePos / TimeDur 是两枚时间标签是否显示。
	TimePos bool
	TimeDur bool
	// Next / EQ / Sponsor 是次要键是否显示。
	Next    bool
	EQ      bool
	Sponsor bool
}

func videoChromeLayoutFor(w float32) videoChromeLayout {
	lo := videoChromeLayout{
		TimePos:   w >= videoChromeTimePos,
		TimeDur:   w >= videoChromeTimeDur,
		Next:      w >= videoChromeKeysNarrow,
		EQ:        w >= videoChromeKeysNarrow,
		Sponsor:   w >= videoChromeKeysNarrow,
		VolumeBar: w >= videoChromeVolumeBar,
		TwoRows:   w < videoChromeTwoRows,
	}
	// 两行布局里进度条独占一行，当前时间到 400 才收起；一行布局在
	// 520..560 之间时时间列已收，进度条拿到全部余量。
	return lo
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

	// 广告跳过是共享设置：主窗开着它时，视频窗也按同一份分段真的跳过。
	// 只在播放中判（暂停在恰饭段上不应该自己跳走）。
	if v.Pl.Playing() {
		v.skipSponsor(now)
	}

	// 全屏状态以窗口为准（用户也可能用系统手势 / F11 进出）。
	v.fs.observe(now, v.Win)
	full := v.fs.full
	// 全屏进出后重新下发置顶：Windows 的全屏会重设窗口 style、macOS 的
	// 独立全屏 Space 会重建窗口层级、GTK 的 keep-above hint 在 state 事件
	// 之后可能被清掉 —— 只靠进全屏前那一次调用，退出全屏后就不一定还是
	// 置顶。这里按**当前**状态再下发一次，置顶既不因进全屏被取消，也不因
	// 出全屏被丢掉。
	if full != v.pinFull {
		v.pinFull = full
		v.applyPin()
	}

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

	// 弹幕覆盖层画在画面上、控件条之下。它自己 IgnoreInput，点击照常穿到
	// 下面那块「视频画面」上。
	v.danmaku(c)

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
	// 弹幕在播放时是连续运动：不请求逐帧重绘的话，只有别的动画（控件条
	// 过渡、缓冲描边）带着画面刷新时弹幕才动一下。
	if v.Danmaku && v.DanmakuOverlay != nil && v.DanmakuOverlay.HasItems() && v.Pl.Playing() {
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
	if v.Pl == nil {
		v.Pl = nopVideoPlayer{}
	}
	set := v.settings()
	set.Ensure()
	// 共享设置一变就落到这个窗口的播放器上：主窗改音量 / 倍速，下一帧
	// 视频窗就是新值；视频窗自己改，也是同一条路。EQ / 广告跳过没有
	// 视频引擎可落（主窗的监听负责），这里只跟着状态走。
	v.detachSettings = set.OnChange(func(k PlayerSettingKind) {
		switch k {
		case SettingVolume:
			if v.Pl != nil {
				v.Pl.SetVolume(set.Volume)
				v.Pl.SetMuted(set.Muted)
			}
		case SettingSpeed:
			if v.Pl != nil {
				v.Pl.SetRate(set.Speed)
			}
		}
	})
	// 起播前把共享设置先下发一次（播放器可能带着默认 1.0 起播）。
	v.Pl.SetVolume(set.Volume)
	v.Pl.SetMuted(set.Muted)
	v.Pl.SetRate(set.Speed)
	// 置顶：开窗就按当前状态下发一次（用户上次把视频窗置顶了，这次打开
	// 仍是置顶）。pinFull 与这一帧的全屏值对齐，免得第一帧重复下发。
	v.applyPin()
	v.pinFull = v.fs.full
}

// settings 返回共享设置；没有注入时建一份本窗口自己的（无头测试 / 独立
// 使用）。生产路径由 internal/video 注入主窗的 &App.PlayerSettings。
func (v *VideoChrome) settings() *PlayerSettings {
	if v.Set == nil {
		v.Set = &PlayerSettings{}
		v.Set.Ensure()
	}
	return v.Set
}

// Detach 注销共享设置监听。关窗时由 internal/video 调用：不注销的话，
// 每开一次视频窗就多挂一个闭包，还会往已经关掉的播放器上下发设置。
func (v *VideoChrome) Detach() {
	if v.detachSettings != nil {
		v.detachSettings()
		v.detachSettings = nil
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
	case !v.Pl.Playing(), v.Pl.Buffering(), v.seek.drag, v.barHover,
		v.showVolume, v.showSpeed, v.fs.inFlight:
		return true
	}
	return false
}

// titleBar 是顶部那条：标题（单行省略）+ 拖动区 + 四颗键。
//
// 四颗键从左到右：**置顶 / 弹幕 / 最小化 / 退出视频播放**。置顶是窗口动作，
// 排在整排最前面（用户本轮的要求），图标、键尺寸与激活态材质都用主窗迷你
// 模式那颗固定键的同一份（见 pinKey）；弹幕是这一集最常用的开关（原版浮层
// 里它在底部浏览器控件条上，原生版没有浏览器兜底，移到顶栏与标题同一行）；
// 最小化与退出视频播放是窗口动作，位置与主窗的窗控键对齐。
//
// 全屏时只收掉最小化（全屏窗口没有「最小化」这回事，系统也不给），弹幕与
// 退出视频播放仍在 —— 全屏里除了 Esc，还需要一条纯指针的出路。
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
		// 置顶排第一（最左）：它是窗口级动作，与主窗迷你模式的固定键同源。
		v.pinKey(c, t)
		// 弹幕：与上一轮同一个开关（同一个事实源：主窗设置菜单 /
		// macOS 菜单 / 这里三处一致），只是位置搬到了顶栏。
		v.danmakuKey(c, t)
		if !full {
			if b := playerBarTitleKey(c, t, "video-min", "最小化", "最小化", iconMinus, false); b.Clicked() {
				v.minimize()
			}
		}
		// 退出视频播放：关窗回主窗，**不是**退出应用（见 OnClose 的注释）。
		if b := playerBarTitleKey(c, t, "video-exit", "退出视频播放", "退出视频播放", iconClose, false); b.Clicked() {
			v.close()
		}
	})
	v.barHover = v.barHover || bar.Hovered() || bar.FocusWithin()
}

// controlBar 是底部那条：与主窗播放栏**同一份控件代码**（playerbar_shared.go
// 的 playerTransportItems / playerTimelineItems / playerSettingsItems），
// 绑定目标是这个窗口的播放器（videoPlayerBind）。
//
// 没有关闭键：关闭只在顶栏有一颗（用户报的「上下各有一个关闭按钮」）。
// 也没有弹幕键：它已按用户要求移进顶栏那四颗里，同一个开关不做两处入口。
//
// 宽度不够时按 videoChromeLayout 渐进收纳：先收内联音量滑块、再收总时长、
// 再收次要键、最后收当前时间；520 以下进度条独占一行。
func (v *VideoChrome) controlBar(c *ui.Context, t Theme, alpha float32, full bool) {
	w, _ := c.Size()
	lo := videoChromeLayoutFor(w)
	var bar ui.Element
	if lo.TwoRows {
		bar = ui.Column(c).Absolute().Left(videoChromePad).Right(videoChromePad).Bottom(videoChromePad).
			Height(videoChromeBarTwoH).Padding(4, 12).Radius(videoChromeRadius).
			Gap(2)
	} else {
		bar = ui.Row(c).Absolute().Left(videoChromePad).Right(videoChromePad).Bottom(videoChromePad).
			Height(videoChromeBarH).Padding(0, 12).Radius(videoChromeRadius).
			AlignItems(ui.Center).Gap(videoChromeGap)
	}
	bar = t.videoBar(bar)
	bar.Opacity(alpha)
	if alpha <= 0.01 {
		bar.IgnoreInput(true)
	}

	b := v.videoPlayerBind(c, t, lo)
	var volumeSlot, speedSlot ui.Element
	bar.Children(func() {
		if lo.TwoRows {
			ui.Row(c).FillWidth().Height(timelineH).AlignItems(ui.Center).
				Gap(videoChromeGap).Children(func() {
				playerTimelineItems(c, b)
			})
			ui.Row(c).FillWidth().Height(42).AlignItems(ui.Center).
				Gap(videoChromeGap).Children(func() {
				playerTransportItems(c, b)
				playerSettingsItems(c, b, &volumeSlot, &speedSlot)
				if lo.VolumeBar {
					v.volumeSlider(c, t)
				}
				ui.Box(c).Grow(1)
				v.fullScreenKey(c, t, full)
			})
		} else {
			ui.Row(c).FillWidth().Grow(1).AlignItems(ui.Center).
				Gap(videoChromeGap).Children(func() {
				playerTransportItems(c, b)
				playerTimelineItems(c, b)
				playerSettingsItems(c, b, &volumeSlot, &speedSlot)
				if lo.VolumeBar {
					v.volumeSlider(c, t)
				}
				v.fullScreenKey(c, t, full)
			})
		}
		// 弹层与主窗同一份实现：倍速用 4 列网格（视频窗窄，7 个档位一行
		// 放不下），音量是 184×42 的横条。
		if v.showSpeed {
			playerSpeedPopover(c, t, b.Set, &v.showSpeed, speedSlot, 4, !b.ShowSpeed)
		}
		if v.showVolume {
			playerVolumePopover(c, t, b.Set, &v.showVolume, volumeSlot, &v.wasVolHeld,
				!b.ShowVolume, func() { b.Set.PersistVolume() })
		}
	})
	v.barHover = v.barHover || bar.Hovered() || bar.FocusWithin()
}

// videoPlayerBind 把视频窗的控件条绑到视频播放器：播放/暂停、进度、时间
// 各走各的；设置类键读写共享的 PlayerSettings。
func (v *VideoChrome) videoPlayerBind(c *ui.Context, t Theme, lo videoChromeLayout) PlayerBarBind {
	set := v.settings()
	dur := v.Pl.Duration().Seconds()
	pos := v.Pl.Position().Seconds()
	shown := pos
	if v.seek.drag {
		shown = v.seek.value
	}
	colors := videoRangeColors(t)
	return PlayerBarBind{
		Theme:      t,
		Set:        set,
		Playing:    v.Pl.Playing(),
		OnPlay:     v.togglePlay,
		HasSrc:     true,
		Pos:        shown,
		Dur:        dur,
		SeekValue:  &v.seek.value,
		OnFrame:    v.seekFrame(dur),
		OnSeek:     v.seekTo,
		Segments:   v.SponsorSegments,
		NextTip:    "视频窗没有换集：回主窗用播放列表",
		CanNext:    false,
		ShowVolume: v.showVolume,
		ShowSpeed:  v.showSpeed,
		OnVolumeKey: func() {
			v.showVolume = !v.showVolume
			v.showSpeed = false
			v.idle.touch(c.Now())
		},
		OnSpeedKey: func() {
			v.showSpeed = !v.showSpeed
			v.showVolume = false
			v.idle.touch(c.Now())
		},
		ShowPos:        lo.TimePos,
		ShowDur:        lo.TimeDur,
		ShowNext:       lo.Next,
		ShowEQ:         lo.EQ,
		ShowSponsor:    lo.Sponsor,
		OnPanel:        true,
		TimelineColors: &colors,
		TimelineSkinFor: func(hovered bool) rangeSkin {
			return videoRangeSkin(t, hovered)
		},
		TimelineOverlay: func(p *ui.Painter, r ui.Rect, rp rangePaint) {
			if pulse := videoBufferingPulse(c.Now(), v.Pl.Buffering()); pulse > 0 {
				p.Stroke(rangeTrackRect(r, rp.Geom), t.Blue.Alpha(pulse), timelineTrackH/2, 1)
			}
		},
		Bubble: &v.bubble,
	}
}

// seekFrame 是视频窗的进度条状态机：拖动中停在目标值，松手跟随播放位置。
// 与主窗的 progressPlan 不同（那边有 seek 未落地 / 拖动预览的等待），
// 但输入接线与绘制是同一份 playerTimeline。
func (v *VideoChrome) seekFrame(dur float64) func(PlayerTimelineInput) (bool, float64) {
	return func(in PlayerTimelineInput) (bool, float64) {
		v.seek.drag = in.Held
		if in.Changed {
			return true, v.seek.value
		}
		if !in.Held {
			return false, clampF(v.Pl.Position().Seconds(), 0, maxF64(dur, 0))
		}
		return false, v.seek.value
	}
}

// volumeSlider 是宽窗（>=900）的内联音量条：与主窗音量弹层里的那条同一份
// 绘制（paintRangeBar），窄了先收起它（键仍在，弹层里还能调）。
func (v *VideoChrome) volumeSlider(c *ui.Context, t Theme) {
	set := v.settings()
	val := clampF(set.Volume, 0, 1)
	s := ui.SliderBase(c, &val, 0, 1).Width(videoChromeVolW).Height(timelineH).
		Label("音量滑块").Step(videoVolumeStep)
	focusOutline(s, 0, focusStudio)
	_, _, held := s.Dragged()
	if s.Changed() {
		set.SetVolume(val)
	}
	skin := videoRangeSkin(t, s.Hovered())
	s.Draw(func(p *ui.Painter, r ui.Rect) {
		paintRangeBar(p, r, rangePaint{
			Frac:       float32(clampF(val, 0, 1)),
			Hovered:    s.Hovered(),
			Held:       held,
			Dark:       t.Dark,
			Colors:     videoRangeColors(t),
			Geom:       volumeGeom,
			ThumbScale: rangeThumbScale(s.Hovered(), held),
			Skin:       &skin,
		})
	})
}

// fullScreenKey 是全屏键（全屏时换成「退出全屏」的四角内收图标）。与顶栏
// 四颗键同一套形状；它是控件条里唯一不属于主窗播放栏的键（窗口动作）。
func (v *VideoChrome) fullScreenKey(c *ui.Context, t Theme, full bool) {
	ic, tip := iconFullScreen, "全屏"
	if full {
		ic, tip = iconExitFullScreen, "退出全屏"
	}
	b := playerBarTitleKey(c, t, "video-fullscreen", tip, tip, ic, full)
	if b.Clicked() {
		v.toggleFullScreen(c.Now())
	}
}

// danmakuKey 是弹幕总开关（视频窗内的入口，紧跟在置顶后面）。
// 状态回到主窗落盘，再经 internal/video 同步回这个窗口；这里先翻转本地
// 状态，点完这一帧就见效。
func (v *VideoChrome) danmakuKey(c *ui.Context, t Theme) {
	tip := "弹幕 · 已开"
	if !v.Danmaku {
		tip = "弹幕 · 已关"
	}
	b := playerBarTitleKey(c, t, "video-danmaku", "弹幕", tip, iconDanmaku, v.Danmaku)
	if b.Clicked() {
		v.Danmaku = !v.Danmaku
		if v.OnDanmakuToggle != nil {
			v.OnDanmakuToggle(v.Danmaku)
		}
	}
}

// pinKey 是顶栏最前面（最左）那颗「置顶」键：与主窗迷你模式的固定键
// **同一个图标**（miniPinIcon：未置顶是 icon-park 描边图钉、置顶是实心变体）、
// **同一个 30×30 键尺寸**、同一条激活态规则（playerBarTitleKey 的 Selected：
// 蓝图标 + 淡蓝玻璃底 + 全不透明，浅深两态都跟随主题令牌）。键内字形按这一行
// 的 15px 画（迷你窗那颗是 16px —— 顶栏四颗键同一行，单独抬 1px 会凸出来）。
//
// 它是保持型开关（原版置顶键是 aria-pressed 的 toggle button）：把 Pinned 作为
// 按压态交给读屏。切换只动窗口的 always-on-top 与这个状态，不碰播放器。
func (v *VideoChrome) pinKey(c *ui.Context, t Theme) {
	tip := pick(v.Pinned, "取消置顶", "置顶")
	b := playerBarTitleKey(c, t, "video-pin", tip, tip, miniPinIcon(v.Pinned), v.Pinned)
	b.Role(ui.RoleToggleButton).Checked(v.Pinned)
	if b.Clicked() {
		v.togglePin()
	}
}

// togglePin 切换视频窗的置顶：先翻本地状态并下发，再把新状态交给调用方
// 落盘（internal/video → 主窗的 OnPinToggle）。
func (v *VideoChrome) togglePin() {
	v.Pinned = !v.Pinned
	v.applyPin()
	if v.OnPinToggle != nil {
		v.OnPinToggle(v.Pinned)
	}
}

// applyPin 把当前置顶状态下发给窗口。各平台都只是「尽力」，没有返回值可判：
//
//   - macOS：`setLevel:`（浮窗等级 / 复原），全屏是系统接管的独立 Space，
//     值本身不会被拒；
//   - Windows：SetWindowPos(HWND_TOPMOST)；另一个进程的独占全屏会压住它，
//     但调用本身不失败；
//   - Linux/GTK：gtk_window_set_keep_above；X11 下由 WM 采信，**Wayland 下
//     合成器可能直接忽略**（xdg-toplevel 没有 keep-above 协议），框架因此
//     只记住「请求过的状态」。
//
// 所以置顶失败**不影响**播放与其它键：那颗键翻的是一个窗口属性，画面、声音、
// 进度与控件条都不依赖它，也没有任何状态机以 IsAlwaysOnTop 为闸。
func (v *VideoChrome) applyPin() {
	if v.Win == nil {
		return
	}
	v.Win.SetAlwaysOnTop(v.Pinned)
}

// DanmakuDrawn 是上一帧真正画出的弹幕文字元素数。无头测试用它断言
// 「关掉开关后渲染点为 0」。
func (v *VideoChrome) DanmakuDrawn() int { return v.danmakuDrawn }

// danmaku 推进并画出弹幕层。时间轴以播放器位置为准，本地时钟按 dt*倍速
// 前进（见 danmaku.Overlay.Sync）；宽度用 ui 的真实字形测量，轨道间距才
// 与屏幕上的字对得上。
func (v *VideoChrome) danmaku(c *ui.Context) {
	v.danmakuDrawn = 0
	o := v.DanmakuOverlay
	if o == nil {
		return
	}
	w, h := c.Size()
	now := c.Now()
	var dt time.Duration
	if !v.dmFrame.IsZero() {
		dt = now.Sub(v.dmFrame)
	}
	v.dmFrame = now

	// 播完（自然结束）：清掉可视集，画面停在最后一帧时不再挂着弹幕。
	// 重播时 Ended 变回假，下一帧按新位置重建（SetItems 没动，条目还在）。
	if v.Pl.Ended() {
		o.Clear()
		return
	}

	s := v.DanmakuSettings
	s.On = s.On && v.Danmaku
	o.SetMeasure(func(text string, size float32) float32 {
		tw, _ := c.MeasureText(0, ui.Span{Text: text, Size: size})
		return tw
	})
	// 缓冲中位置不动，但 Playing() 可能仍为真：这时按暂停处理，弹幕不能
	// 自己往前跑（否则每次缓冲回来都要被 resync 拽回去）。
	playing := v.Pl.Playing() && !v.Pl.Buffering()
	o.Sync(v.Pl.Position().Seconds(), playing, v.settings().Speed, dt.Seconds(), float32(w), float32(h), s)
	if !s.On {
		return
	}
	size := s.FontSize()
	for _, sp := range o.Sprites() {
		// 还没从右沿进场 / 已经完全从左边出去：不建元素（渲染点计数只算
		// 真正可见的条目）。
		if sp.X >= float32(w) || sp.X+sp.W <= 0 {
			continue
		}
		el := ui.Text(c, sp.Text).FontSize(size).SingleLine().
			TextColor(danmakuInk(sp.Color, true)).
			// 视频画面明暗不可控：一层淡影保证白字在亮画面上也能读。
			TextShadow(0, 1, 2, ui.RGBA(0, 0, 0, 0.55)).
			Absolute().Left(sp.X).Top(sp.Y).IgnoreInput(true)
		if s.Opacity < 1 {
			el.Opacity(float32(s.Opacity))
		}
		v.danmakuDrawn++
	}
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

// videoBufferingPulse 是缓冲描边的呼吸不透明度（0.25 ~ 0.60，约 1.4s 一周期）。
func videoBufferingPulse(now time.Time, buffering bool) float32 {
	if !buffering {
		return 0
	}
	phase := float64(now.UnixNano()%int64(1400*time.Millisecond)) / float64(1400*time.Millisecond)
	return 0.25 + 0.35*float32(0.5+0.5*math.Sin(phase*2*math.Pi))
}

// skipSponsor 按共享设置里的广告跳过开关与主窗同步来的分段，把播放位置
// 推过恰饭段。判据与主窗 maybeSkipSponsor 同一套（时长容差、提前量、片尾
// 余量）；区别是这里没有 toast（视频窗不抢画面的提示），用一个 1s 的时间闩
// 防止 seek 未落地时连发。
func (v *VideoChrome) skipSponsor(now time.Time) {
	set := v.settings()
	if !set.Sponsor || len(v.SponsorSegments) == 0 || v.Pl == nil || !v.Pl.Ready() {
		return
	}
	if !v.lastSponsorSeek.IsZero() && now.Sub(v.lastSponsorSeek) < time.Second {
		return
	}
	pos := v.Pl.Position().Seconds()
	dur := v.Pl.Duration().Seconds()
	const (
		// 与主窗 maybeSkipSponsor 的两个值一致：提前 0.25s 触发、落到
		// 片尾后 0.05s（避免停在分段边界上又被判一次）。
		lead = 0.25
		pad  = 0.05
	)
	for _, seg := range v.SponsorSegments {
		if seg.VideoDuration > 0 && dur > 0 && math.Abs(seg.VideoDuration-dur) > sponsorTolerance {
			continue
		}
		if pos >= seg.Start-lead && pos < seg.End {
			v.lastSponsorSeek = now
			v.seekTo(seg.End + pad)
			return
		}
	}
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

// toggleMute / setVolume 改的是共享设置；本窗口的监听会把新值落到播放器，
// 主窗的监听会把同一个值落到音频引擎 —— 两边不各写一套。
func (v *VideoChrome) toggleMute() { v.settings().ToggleMute() }

func (v *VideoChrome) setVolume(x float64) { v.settings().SetVolume(x) }

func (v *VideoChrome) nudgeVolume(delta float64) {
	v.setVolume(v.settings().Volume + delta)
}

func (v *VideoChrome) toggleFullScreen(now time.Time) {
	v.fs.toggle(now, v.Win)
}

// close 是「退出视频播放」：把动作交给调用方（关视频窗、回主窗、按
// video.RelayOnClose 决定是否续播音频）。这里**不碰** OnQuit：视频窗的任何
// 按钮都不能退出应用，退出应用只能从主窗发起。
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
	case videoKeyExit:
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
