package video

// 原生视频窗（mygo v0.9.0 的 ui/video）—— 替代 webview 的主链路。
//
// 为什么换：webview 弹窗在 Windows 上带一整套 WebView2（实测主窗 57MB → 打开
// 视频 506MB），而且窗口是「页面」：控件、快捷键、进度回传全靠 JS 每秒轮询。
// ui/video 让系统自己的媒体框架解码（AVFoundation / Media Foundation /
// GStreamer），帧直接进窗口渲染器（GPU，不经过内存拷贝），声音由同一个引擎
// 出声 —— 音画同步是**框架内建的**，不需要我们拿两条时钟去对齐。
//
// 关键前提：B 站 playurl 的 `platform=html5` 分支（GetUrlByCid）返回的是
// **渐进式 MP4**（h264 + AAC-LC 复用在一个 mdat 里，已用 ffprobe 核实），
// 也就是「自带音轨的单一媒体源」，正好是 ui/video 吃的形状。所以不需要把
// DASH 的音轨/视频轨自己 mux 成 fMP4 —— 那条路只在「只能拿到 DASH」时才需要，
// 见 PLAN.md 的评估一节（成本与风险都写在那边）。
//
// 回退：没有媒体框架（Linux 缺 GStreamer、或系统版本太老）、格式不支持、
// 起播失败 → **不自己切 webview**，而是把「为什么 + 怎么修」交给主窗弹一个
// 应用内对话框（Manager.OnFallback），由用户自己选「切换到 WebView 播放」
// 还是「取消」—— 开关是用户的，不能被静默改掉（见 video.go 的 FallbackPrompt）。

import (
	"errors"
	"fmt"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"

	"github.com/vst93/bili-fm/app/internal/danmaku"
	"github.com/vst93/bili-fm/app/internal/view"
)

// playerAPI 是原生窗驱动播放器所需的最小面：*ui.VideoPlayer 满足它，测试里
// 也可以塞一个纯逻辑桩（无头测试不碰系统媒体框架）。
type playerAPI interface {
	Play()
	Pause()
	Playing() bool
	Ended() bool
	Ready() bool
	Buffering() bool
	Err() error
	Duration() time.Duration
	Position() time.Duration
	Seek(time.Duration)
	SetRate(float64)
	SetVolume(float64)
	SetMuted(bool)
	Close()
}

// uiPlayer 把 *ui.VideoPlayer 适配成 playerAPI。
type uiPlayer struct{ p *ui.VideoPlayer }

func (u uiPlayer) Play()                   { u.p.Play() }
func (u uiPlayer) Pause()                  { u.p.Pause() }
func (u uiPlayer) Playing() bool           { return u.p.Playing() }
func (u uiPlayer) Ended() bool             { return u.p.Ended() }
func (u uiPlayer) Ready() bool             { return u.p.Ready() }
func (u uiPlayer) Buffering() bool         { return u.p.Buffering() }
func (u uiPlayer) Err() error              { return u.p.Err() }
func (u uiPlayer) Duration() time.Duration { return u.p.Duration() }
func (u uiPlayer) Position() time.Duration { return u.p.Position() }
func (u uiPlayer) Seek(d time.Duration)    { u.p.Seek(d) }
func (u uiPlayer) SetRate(r float64)       { u.p.SetRate(r) }
func (u uiPlayer) SetVolume(v float64)     { u.p.SetVolume(v) }
func (u uiPlayer) SetMuted(m bool)         { u.p.SetMuted(m) }
func (u uiPlayer) Close()                  { u.p.Close() }

// nativeWindow 是一个原生视频窗。
type nativeWindow struct {
	win *mygo.Window
	pl  playerAPI

	// chrome 是窗口外壳（标题栏 / 控件条 / 全屏 / 快捷键），与主窗播放栏
	// 共用同一套键族与令牌，见 internal/view/videochrome.go。
	chrome *view.VideoChrome
	theme  view.Theme

	title   string
	startAt float64
	// settings 是主窗与视频窗共享的音频设置（音量/静音/倍速/均衡/广告
	// 跳过）。由 Options.Shared 注入；没有时建一份本窗口自己的。
	settings *view.PlayerSettings
	// segments 是当前这一集的恰饭分段（主窗查询后同步进来）：进度条画标记，
	// 开关打开时视频窗也会真的跳过。
	segments []view.Segment

	// 弹幕层：条目 / 开关 / 基本设置与视频管理器同源（Manager 每次变化都
	// 通过 win.Update 落到这里）。dm 永远非 nil，关掉开关时只是不画。
	dm              *danmaku.Overlay
	dmCid           int64
	dmOn            bool
	dmSettings      danmaku.Settings
	onDanmakuToggle func(bool)
	onDanmakuStyle  func(danmaku.Settings)

	// nativeVideo 是设置面板「播放引擎」里那一行开关的当前值：开窗时是
	// 主窗落盘的 nativeVideoPlayer（能在原生路径里就一定是开的），之后由
	// 主窗 SetNativeEnabled / 面板里的点击改它。
	nativeVideo    bool
	onNativeToggle func(bool)

	// pinned 是视频窗的置顶状态（顶栏最左那颗图钉）。它是窗口属性，不落盘在
	// 这里：点一下就回到主窗存 videoPinned（见 chromeFor 的 OnPinToggle）。
	pinned      bool
	onPinToggle func(bool)

	// 界面态（只在主线程 / 视图构建里读写）。
	reported   time.Time
	endedFired bool
	// suppressClose 关窗时不回传 onClose（回退到 webview 时用：那一集马上要在
	// 另一个窗口里继续播，不能让主窗以为「视频关了」去做音视接力）。
	suppressClose bool

	// 回调（Manager 注入）。
	onState  func(State)
	onClose  func(State)
	onEnded  func()
	onMin    func()
	onQuit   func()
	onFailed func(error)
}

// nativeEligible 报告这条播放地址能不能走原生链路。
//
// 只认 http(s) 与 file：ui/video 的 OpenURL 文档写明支持 file/http/https 和
// HLS，我们这条链路拿到的永远是本地代理的 http URL。空地址直接判否，避免
// 「开了个黑窗再回退」。
func nativeEligible(o Options) bool {
	if o.Src == "" {
		return false
	}
	switch {
	case len(o.Src) > 7 && o.Src[:7] == "http://":
		return true
	case len(o.Src) > 8 && o.Src[:8] == "https://":
		return true
	case len(o.Src) > 5 && o.Src[:5] == "file:":
		return true
	}
	return false
}

// seekTarget 把进度条的值夹进 [0, dur]，dur 未知时原样返回（秒）。
func seekTarget(value, dur float64) float64 {
	if value < 0 {
		return 0
	}
	if dur > 0 && value > dur {
		return dur
	}
	return value
}

// openPlayer 在主线程上打开一个播放器：ui/video 的引擎要在应用主线程上建
// （Media Foundation 的 COM、AVFoundation 的主队列、GStreamer 的 playbin
// 都按这个假设写）。返回 ErrUnsupported 时调用方落回 webview。
func openPlayer(src string) (playerAPI, error) {
	var (
		pl  *ui.VideoPlayer
		err error
	)
	done := make(chan struct{})
	mygo.RunOnMain(func() {
		pl, err = video.OpenURL(src)
		close(done)
	})
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		return nil, fmt.Errorf("video: 打开原生播放器超时")
	}
	if err != nil {
		return nil, err
	}
	return uiPlayer{p: pl}, nil
}

// newNativeWindow 建窗并开始播放。调用方（Manager）保证 pl 已在主线程打开。
func newNativeWindow(o Options, pl playerAPI) *nativeWindow {
	set := o.Shared
	if set == nil {
		// 没有注入共享设置（测试 / 独立使用）：按 Options 建一份本窗口的。
		set = &view.PlayerSettings{
			Speed:      max(o.Speed, 0.5),
			Volume:     1,
			LastVolume: 1,
			Muted:      o.Muted,
		}
		if o.Muted {
			set.Volume = 0
		}
	}
	set.Ensure()
	n := &nativeWindow{
		pl:         pl,
		theme:      o.Theme,
		title:      o.Title,
		startAt:    o.StartAt,
		settings:   set,
		segments:   o.SponsorSegments,
		dm:         &danmaku.Overlay{},
		dmCid:      o.DanmakuCid,
		dmOn:       true,
		dmSettings: danmaku.DefaultSettings(),
		pinned:     o.VideoPinned,
		// 能走到 newNativeWindow 就说明原生路径可用（Manager.tryNative 的
		// 判定），面板里那一行默认是开的。
		nativeVideo: true,
	}
	n.dm.SetItems(o.Danmaku)
	n.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     o.Title,
		Width:     960,
		Height:    600,
		MinWidth:  320,
		MinHeight: 200,
		// 与 webview 弹窗同一套无边框外观：标题栏自绘（见 view），
		// 顶栏五颗键：置顶 / 弹幕 / 设置（齿轮，弹出内置设置面板）/ 最小化 /
		// 退出视频播放；底部控件条没有关闭键。顶部 34px 是拖动区。
		//
		// **刻意不在这里声明 AlwaysOnTop / SkipTaskbar**：置顶是运行时可切的窗口
		// 属性（落盘值 VideoPinned，建窗后由外壳下发 SetAlwaysOnTop）。mygo 0.9.1
		// 的 Wayland overlay（框架层的 layer surface）只在创建时就带
		// `AlwaysOnTop + SkipTaskbar + Frameless` 的窗上生效，且一旦成了 layer
		// surface 就永远在上 —— 视频窗要留在任务栏里、要能取消置顶，所以不进去。
		// 详见 internal/view/videochrome.go 的 applyPin。
		Frameless: true,
		// 起播前那一帧是黑的（视频自己的底色），显式给黑色避免白闪。
		BackgroundColor: "#000000",
		Content:         ui.View(n.view),
	})
	if n.win == nil {
		pl.Close()
		return nil
	}
	// 起播参数按共享设置来（主窗静音 / 倍速在这一刻是什么，视频窗就是什么）。
	pl.SetVolume(set.Volume)
	pl.SetMuted(set.Muted)
	pl.SetRate(set.Speed)
	// 全屏是异步的（macOS 的 toggleFullScreen 动画、GTK 的 window-state-event、
	// Wayland 合成器的答复）。用户也可能用系统手势进出，所以状态以窗口为准，
	// 变化时补一帧让外壳按新的全屏状态重排（见 view.VideoChrome 的
	// videoFullScreen.observe）。
	n.win.OnEnterFullScreen(func() { n.invalidate() })
	n.win.OnLeaveFullScreen(func() { n.invalidate() })
	n.win.OnClosed(func() {
		n.chromeDetach()
		// 先取状态再关播放器：关掉之后 Position/Duration 都归零，主窗拿到
		// 的「关窗时停在几秒」就成了 0（关窗后要继续听这一集，见 main.go
		// 的 OnClose 接力）。closeState 关的是 n.pl —— 不是建窗时捕获的那个
		// 播放器（原地换集后它是旧的），漏关会让视频声音在窗口消失后继续放。
		st := n.closeState()
		if !n.suppressClose {
			if cb := n.onClose; cb != nil {
				cb(st)
			}
		}
	})
	return n
}

// invalidate 让窗口重画一帧（外壳每帧读 IsFullScreen 校正状态）。
func (n *nativeWindow) invalidate() {
	if n.win != nil {
		n.win.Invalidate()
	}
}

// closeState 关窗收尾：取这一刻的播放状态，再关掉**当前**播放器。
//
// 必须用 n.pl，不能用建窗时捕获的那个 pl：原地换集（switchNative）会把
// n.pl 换成新播放器、旧的那个当场 Close 掉。用捕获值去关，真正在放的新
// 播放器永远关不掉 —— 窗口没了声音还在（用户报的「关了视频窗，声音出来
// 了、主窗却显示暂停」），而状态又是从 n.pl 取的，两边就此分叉。
func (n *nativeWindow) closeState() State {
	if n.pl == nil {
		return State{}
	}
	st := n.state()
	n.pl.Close()
	return st
}

// ensureSettings 给直接构造的 nativeWindow（测试）补一份设置，并保证监听
// 还没挂上时也有可读的倍速/音量。
func (n *nativeWindow) ensureSettings() *view.PlayerSettings {
	if n.settings == nil {
		n.settings = &view.PlayerSettings{Volume: 1, LastVolume: 1, Speed: 1}
		n.settings.Ensure()
	}
	n.settings.Ensure()
	return n.settings
}

// state 是给主窗回传的播放状态（与 webview 路径同一结构）。
func (n *nativeWindow) state() State {
	n.ensureSettings()
	st := State{
		Duration: n.pl.Duration().Seconds(),
		Time:     n.pl.Position().Seconds(),
		Paused:   !n.pl.Playing(),
		Muted:    n.settings.Muted,
		Speed:    n.settings.Speed,
		Ready:    1,
	}
	if !n.pl.Ready() {
		st.Ready = 0
	}
	if err := n.pl.Err(); err != nil {
		st.Err = err.Error()
	}
	return st
}

// report 把状态回传主窗（约每秒一次），并处理「自然播完」。
// 只在视图构建（主线程）里调用。
func (n *nativeWindow) report() {
	if n.pl == nil {
		return
	}
	if err := n.pl.Err(); err != nil {
		if n.onFailed != nil {
			cb := n.onFailed
			n.onFailed = nil // 只回退一次
			cb(err)
		}
		return
	}
	if n.pl.Ended() && !n.endedFired {
		n.endedFired = true
		if cb := n.onEnded; cb != nil {
			cb()
		}
	}
	if time.Since(n.reported) < time.Second {
		return
	}
	n.reported = time.Now()
	if cb := n.onState; cb != nil {
		cb(n.state())
	}
}

// apply 在主线程上改播放器状态（外部调用都走这里）。
func (n *nativeWindow) apply(fn func()) {
	if n.win == nil {
		return
	}
	n.win.Update(fn)
}

// setTheme 换一份主题令牌（外壳的标题栏 / 控件条 / 设置面板 / 弹幕区底色
// 下一帧就用新值重画）。只改界面令牌：不碰播放器、进度、弹幕、全屏与置顶。
func (n *nativeWindow) setTheme(t view.Theme) {
	n.theme = t
	if n.chrome != nil {
		n.chrome.Theme = t
	}
}

func (n *nativeWindow) play()  { n.apply(func() { n.pl.Play() }) }
func (n *nativeWindow) pause() { n.apply(func() { n.pl.Pause() }) }

func (n *nativeWindow) seek(seconds float64) {
	n.apply(func() {
		n.pl.Seek(time.Duration(seekTarget(seconds, n.pl.Duration().Seconds()) * float64(time.Second)))
	})
}

// setRate / setMuted / setVolume 改的是共享设置；外壳注册的监听会把新值
// 落到这个窗口的播放器上（与视频窗内点音量键走同一条路）。
func (n *nativeWindow) setRate(v float64) {
	n.apply(func() { n.settings.SetSpeed(max(v, 0.5)) })
}

func (n *nativeWindow) setMuted(m bool) {
	n.apply(func() { n.settings.SetMuted(m) })
}

func (n *nativeWindow) setVolume(v float64) {
	n.apply(func() { n.settings.SetVolume(v) })
}

// chromeDetach 注销视频窗外壳挂在共享设置上的监听（关窗 / 退出播放时调）。
func (n *nativeWindow) chromeDetach() {
	if n.chrome != nil {
		n.chrome.Detach()
	}
}

func (n *nativeWindow) close() {
	if n.win == nil {
		return
	}
	n.win.Close()
}

// minimize 最小化视频窗（顶栏第二颗键 / Ctrl+W）。
//
// 最小化 ≠ 暂停：这里只动窗口，不碰播放器 —— 声音与进度照常往前走，
// 恢复后画面从当前位置接上（player 是系统媒体框架，与窗口可见性无关）。
// 走 mygo 的 Window.Minimize：macOS `miniaturize:`、Windows
// `ShowWindow(SW_MINIMIZE)`、Linux `gtk_window_iconify`。Wayland 合成器
// 没有 iconify 协议，GTK 下可能不响应；这里**不**退化成隐藏（视频窗没有
// 托盘入口，藏了就回不来），窗口保持原样。
func (n *nativeWindow) minimize() {
	if n.win != nil {
		n.win.Minimize()
	}
}

func (n *nativeWindow) focus() {
	if n.win == nil {
		return
	}
	if n.win.IsMinimized() {
		n.win.Restore()
	}
	n.win.Show()
	n.win.Focus()
}

// ---------------------------------------------------------------- 视图

// view 画出原生视频窗。界面全在 view.VideoChrome 里：那里与主窗播放栏共用
// 同一套键族（28×28、圆角、悬停/按下/激活、tooltip、主题令牌）、同一份进度条
// 绘制代码、以及自动隐藏 / 全屏 / 快捷键三套状态机。这里只做两件事：
//
//  1. 把窗口自己持有的状态（播放器、倍速、音量、静音）同步给外壳 —— 主窗的
//     音量键 / 倍速菜单也会改它们，两边必须同源；
//  2. 每帧把状态回传主窗（约每秒一次，见 report）。
func (n *nativeWindow) view(c *ui.Context) {
	ch := n.chromeFor()
	ch.Title = n.title
	// 主题令牌以 n.theme 为准：主窗切深浅 / 跟随系统变档时 Manager.SetTheme
	// 会改写它，这里每帧同步给外壳 —— 视频窗是独立窗口，不跑主窗的 Shell，
	// 不推就会停在开窗那一刻的深浅上（关掉重开才生效）。
	ch.Theme = n.theme
	// 注意别把 nil 装进接口：`ch.Win = n.win` 在 n.win 为 nil 时会得到一个
	// 「非 nil 的接口装着 nil 指针」，外壳的 `w == nil` 判据就失效了，
	// 接着调 IsFullScreen 会直接卡死（测试里没有真窗口）。
	if n.pl != nil {
		ch.Pl = n.pl
	}
	if n.win != nil {
		ch.Win = n.win
	}
	// 设置是共享的（同一个指针），不需要每帧拷贝；弹幕状态以窗口为准同步
	// 进外壳（外壳自己也会在点击时翻转它）。
	ch.Danmaku = n.dmOn
	ch.DanmakuSettings = n.dmSettings
	ch.SponsorSegments = n.segments
	ch.Pinned = n.pinned
	ch.NativeVideo = n.nativeVideo
	ch.View(c)
	// 外壳自己会在 init 里把零值主题按当前时段补齐（直接构造 nativeWindow 的
	// 测试 / 独立使用）：回写一份，下一帧「ch.Theme = n.theme」才不会把它清回
	// 零值导致第二帧掉回无主题。
	n.theme = ch.Theme
	n.dmOn = ch.Danmaku
	n.pinned = ch.Pinned
	n.nativeVideo = ch.NativeVideo
	n.report()
}

// chromeFor 惰性建出外壳（测试里直接构造 nativeWindow 时也走这条）。
func (n *nativeWindow) chromeFor() *view.VideoChrome {
	n.ensureSettings()
	if n.chrome != nil {
		return n.chrome
	}
	n.chrome = &view.VideoChrome{
		Theme: n.theme,
		Title: n.title,
		Set:   n.settings,
		// 回调都在调用时读字段：Manager 在 newNativeWindow 之后才接上
		// onMin / onQuit，闭包里捕获字段才能拿到后来的值。
		//
		// OnClose 是「退出视频播放」（关视频窗回主窗），**不是**退出应用：
		// 它只关这一个窗口，主窗与进程都留着。关窗延迟到这一帧之后 ——
		// VideoChrome 的点击是在视图构建里读出来的，而 mygo 的 Window.Close
		// 在主线程上是同步的（onMain 直接调用 → destroy → OnClosed → 关播放器）；
		// 在窗口自己的帧里拆掉自己，会让同一个帧继续用一个已销毁的窗口
		// （native.go 的 onFailed 出于同一个原因丢到 goroutine 上）。
		OnClose: func() {
			n.chromeDetach()
			go n.close()
		},
		OnMinimize: func() {
			if cb := n.onMin; cb != nil {
				cb()
			}
		},
		OnQuit: func() {
			if cb := n.onQuit; cb != nil {
				cb()
			}
		},
		// 「播完再点播放」要把「已报过播完」的闩复位（见 VideoChrome.OnReplay）。
		OnReplay: func() { n.endedFired = false },
		Content: func(c *ui.Context) ui.Element {
			return ui.Video(c, n.player()).Fill().Fit(ui.Contain)
		},
		// 弹幕：条目层由 internal/danmaku 管，外壳只负责按 Sprites 画字。
		Danmaku:         n.dmOn,
		DanmakuOverlay:  n.dm,
		DanmakuSettings: n.dmSettings,
		// 置顶（顶栏最左那颗图钉，与迷你模式的固定键同源）：窗口那边翻
		// 本地状态，这里再回主窗落盘（新键 videoPinned，默认 false）。
		Pinned: n.pinned,
		OnPinToggle: func(on bool) {
			n.pinned = on
			if cb := n.onPinToggle; cb != nil {
				cb(on)
			}
		},
		OnDanmakuToggle: func(on bool) {
			n.dmOn = on
			if cb := n.onDanmakuToggle; cb != nil {
				cb(on)
			}
		},
		// 设置面板里改弹幕基本设置：先落窗口（这一帧就按新值排版），再回
		// 主窗落盘 + 同步（主窗的 SetDanmakuSettings 会再把值送回来）。
		OnDanmakuSettings: func(s danmaku.Settings) {
			n.dmSettings = s
			if cb := n.onDanmakuStyle; cb != nil {
				cb(s)
			}
		},
		// 设置面板里的「原生视频播放」：这个开关的唯一入口。翻的是窗口
		// 本地状态 + 主窗落盘（Manager 的开关随即生效，下一集起播按新值）。
		NativeVideo: n.nativeVideo,
		OnNativeToggle: func(on bool) {
			n.nativeVideo = on
			if cb := n.onNativeToggle; cb != nil {
				cb(on)
			}
		},
	}
	return n.chrome
}

// setDanmaku 换这一集的弹幕（cid 不匹配说明是上一集的迟到响应，丢掉）。
func (n *nativeWindow) setDanmaku(cid int64, items []danmaku.Item) {
	if n.dmCid != 0 && cid != n.dmCid {
		return
	}
	n.dmCid = cid
	if n.dm != nil {
		n.dm.SetItems(items)
	}
}

// player 把播放器交给 ui.Video。测试里的纯逻辑桩不是 *ui.VideoPlayer，返回
// nil（ui.Video 对 nil 播放器只画黑底）。
func (n *nativeWindow) player() *ui.VideoPlayer {
	if u, ok := n.pl.(uiPlayer); ok {
		return u.p
	}
	return nil
}

// nativeFallbackPrompt 把原生路径的失败翻成一条给用户看的「为什么 + 怎么修」。
//
// 判据只有两类，都能在「用户看得懂」和「排查有用」之间取平衡：
//   - 缺媒体框架（ui/video 的 ErrUnsupported，Linux 缺 GStreamer 1.x /
//     gst-plugins-base 的 playbin+appsink，或系统版本太老）；
//   - 其它（解码器缺失、流断了、起播超时）。
//
// 平台相关的部分（缺什么、装哪个包）在 nativehint_<os>.go 里：这是三平台
// 唯一的差异点，入口与文案骨架保持一致。
func nativeFallbackPrompt(err error) *FallbackPrompt {
	if err == nil {
		return nil
	}
	reason := "当前系统无法使用原生视频播放"
	if errors.Is(err, video.ErrUnsupported) {
		reason += "：" + nativeMissingText()
	} else {
		reason += "（" + err.Error() + "）"
	}
	return &FallbackPrompt{Reason: reason, Hint: nativeFixHint()}
}
