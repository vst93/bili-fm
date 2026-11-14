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
// 起播失败 → 关掉原生窗，落回 webview 那条路径，并 toast 告知用户（由
// Manager.OnFallback 接到主窗）。用户不会因为这条链路看不了视频。

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/video"
)

// playerAPI 是原生窗驱动播放器所需的最小面：*ui.VideoPlayer 满足它，测试里
// 也可以塞一个纯逻辑桩（无头测试不碰系统媒体框架）。
type playerAPI interface {
	Play()
	Pause()
	Playing() bool
	Ended() bool
	Ready() bool
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

	title   string
	startAt float64
	speed   float64
	muted   bool
	volume  float64

	// 界面态（只在主线程 / 视图构建里读写）。
	seekValue  float64
	dragging   bool
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

// speedStep 返回 speed 在倍速档位里的下一档（循环），没有命中时回 1.0。
// 抽成纯函数便于无头测试（倍速按钮的行为完全由它决定）。
func speedStep(speed float64, options []float64) float64 {
	if len(options) == 0 {
		return 1
	}
	for i, v := range options {
		if abs(v-speed) < 0.001 {
			return options[(i+1)%len(options)]
		}
	}
	return options[0]
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
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

// clockText 把秒格式化成播放器的时间文本（m:ss / h:mm:ss）。
func clockText(sec float64) string {
	if sec < 0 || sec != sec {
		sec = 0
	}
	t := int(sec)
	h, m, s := t/3600, (t%3600)/60, t%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%d:%02d", m, s)
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
	n := &nativeWindow{
		pl:      pl,
		title:   o.Title,
		startAt: o.StartAt,
		speed:   max(o.Speed, 0.5),
		muted:   o.Muted,
		volume:  1,
	}
	n.win = mygo.NewWindow(mygo.WindowOptions{
		Title:     o.Title,
		Width:     960,
		Height:    600,
		MinWidth:  320,
		MinHeight: 200,
		// 与 webview 弹窗同一套无边框外观：标题栏自绘（见 view），
		// 关闭键常驻右下角控件条，顶部 36px 是拖动区。
		Frameless: true,
		// 起播前那一帧是黑的（视频自己的底色），显式给黑色避免白闪。
		BackgroundColor: "#000000",
		Content:         ui.View(n.view),
	})
	if n.win == nil {
		pl.Close()
		return nil
	}
	n.win.OnClosed(func() {
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

// state 是给主窗回传的播放状态（与 webview 路径同一结构）。
func (n *nativeWindow) state() State {
	st := State{
		Duration: n.pl.Duration().Seconds(),
		Time:     n.pl.Position().Seconds(),
		Paused:   !n.pl.Playing(),
		Muted:    n.muted,
		Speed:    n.speed,
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

func (n *nativeWindow) play()  { n.apply(func() { n.pl.Play() }) }
func (n *nativeWindow) pause() { n.apply(func() { n.pl.Pause() }) }

func (n *nativeWindow) seek(seconds float64) {
	n.apply(func() {
		n.pl.Seek(time.Duration(seekTarget(seconds, n.pl.Duration().Seconds()) * float64(time.Second)))
	})
}

func (n *nativeWindow) setRate(v float64) {
	n.apply(func() {
		n.speed = max(v, 0.5)
		n.pl.SetRate(n.speed)
	})
}

func (n *nativeWindow) setMuted(m bool) {
	n.apply(func() {
		n.muted = m
		n.pl.SetMuted(m)
	})
}

func (n *nativeWindow) setVolume(v float64) {
	n.apply(func() {
		n.volume = v
		n.pl.SetVolume(v)
		n.pl.SetMuted(n.muted || v <= 0)
	})
}

func (n *nativeWindow) close() {
	if n.win == nil {
		return
	}
	n.win.Close()
}

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

// 控件尺寸（DIP）。与主窗播放栏的风格一致：小键 28、进度条 6、底栏 44。
const (
	nativeBarH      float32 = 36
	nativeControlsH float32 = 44
	nativeKey       float32 = 28
	nativeSeekH     float32 = 18
)

// view 画出原生视频窗：画面铺满，顶部自绘标题栏（悬停顶部条唤出，与原版
// playerVideo.tsx 的 48px 热区同一意图），底部一条控件（播放/暂停、时间、
// 进度、倍速、静音、音量、关闭）。
//
// 为什么标题栏要「隐去」：视频窗是无边框的，常驻一条深色栏会压住画面顶部
// （原版也是默认隐去、鼠标进入顶部热区才唤回）。这里靠「栏自己在静置时不透明度
// 为 0、但仍参与命中测试」实现唤出 —— 指针进到顶部 36px 就 Hovered() 为真。
func (n *nativeWindow) view(c *ui.Context) {
	c.SetTheme(ui.DarkTheme())
	root := c.Root()
	root.Background(ui.Hex("#000000"))
	n.bindKeys(c)

	dur := n.pl.Duration().Seconds()
	pos := n.pl.Position().Seconds()
	if !n.dragging {
		n.seekValue = pos
	}

	ui.Column(c).Fill().Children(func() {
		ui.Video(c, n.player()).Fill().Fit(ui.Contain)

		// 顶部标题栏：拖动区（DragWindow）+ 标题 + 关闭键。静置时全透明，
		// 指针进来（栏自身命中）才显形。
		bar := ui.Row(c).Absolute().Left(0).Right(0).Top(0).Height(nativeBarH).
			Padding(0, 10).AlignItems(ui.Center).Gap(10).DragWindow()
		shown := bar.Hovered()
		if shown {
			bar.Background(ui.Hex("#0c1420").Alpha(0.72))
		} else {
			bar.Opacity(0)
		}
		bar.Children(func() {
			ui.Text(c, n.title).FontSize(12).Grow(1).TextColor(ui.Hex("#e8eef7"))
			if shown {
				closeKey(c, "关闭", func() { n.close() })
			}
		})

		// 底部控件条：原生窗没有浏览器的原生控件，全靠这一条，所以常驻。
		ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Height(nativeControlsH).
			Padding(8, 12).AlignItems(ui.Center).Gap(10).
			Background(ui.Hex("#0c1420").Alpha(0.82)).Children(func() {
			label := "播放"
			if n.pl.Playing() {
				label = "暂停"
			}
			if b := ui.Button(c, label).Size(52, nativeKey).FontSize(12); b.Clicked() {
				switch {
				case n.pl.Playing():
					n.pl.Pause()
				case n.pl.Ended():
					n.endedFired = false
					n.pl.Seek(0)
					n.pl.Play()
				default:
					n.pl.Play()
				}
			}
			ui.Text(c, clockText(pos)).FontSize(11).TextColor(ui.Hex("#cbd5e1"))
			// 进度条：拖拽 / 方向键都改 n.seekValue，Changed 时 seek 一次。
			// 拖住期间不用回传位置覆盖滑块（否则滑块会被拽回去）。
			s := ui.SliderBase(c, &n.seekValue, 0, max(dur, 0.1)).
				Grow(1).Height(nativeSeekH).Label("播放进度").Step(0.1)
			s.Disabled(dur <= 0)
			_, _, held := s.Dragged()
			n.dragging = held
			if s.Changed() {
				n.pl.Seek(time.Duration(seekTarget(n.seekValue, dur) * float64(time.Second)))
			}
			ui.Text(c, clockText(dur)).FontSize(11).TextColor(ui.Hex("#94a3b8"))
			if b := ui.Button(c, speedLabel(n.speed)).Size(48, nativeKey).FontSize(12).
				Label("倍速"); b.Clicked() {
				n.speed = speedStep(n.speed, nativeSpeeds)
				n.pl.SetRate(n.speed)
			}
			mute := "静音"
			if n.muted || n.volume <= 0 {
				mute = "取消静音"
			}
			if b := ui.Button(c, mute).Size(72, nativeKey).FontSize(12).Label(mute); b.Clicked() {
				n.muted = !n.muted
				n.pl.SetMuted(n.muted)
			}
			vol := ui.SliderBase(c, &n.volume, 0, 1).Width(80).Height(nativeSeekH).
				Label("音量").Step(0.05)
			if vol.Changed() {
				n.pl.SetVolume(n.volume)
				if n.volume > 0 && n.muted {
					n.muted = false
					n.pl.SetMuted(false)
				}
			}
			closeKey(c, "关闭视频", func() { n.close() })
		})
	})
	n.report()
}

// speedLabel 是倍速键上的文案：3 位有效数字就够（0.5x / 1x / 1.25x / 3x），
// 与主窗倍速弹层显示的档位一致。
func speedLabel(v float64) string {
	s := strconv.FormatFloat(v, 'g', 3, 64)
	return s + "x"
}

// nativeSpeeds 与主窗播放栏的档位一致（playerbar.go 的 speedOptions）。
var nativeSpeeds = []float64{0.5, 0.75, 1.0, 1.25, 1.5, 2.0, 3.0}

func (n *nativeWindow) player() *ui.VideoPlayer {
	if u, ok := n.pl.(uiPlayer); ok {
		return u.p
	}
	return nil
}

// closeKey 是一颗小键（标题栏 / 控件条共用），带无障碍标签。
func closeKey(c *ui.Context, label string, fn func()) {
	b := ui.ButtonBase(c.Key("native-"+label)).Size(nativeKey*2.4, nativeKey).
		Center().Radius(6).Label(label)
	b.Children(func() { ui.Text(c, label).FontSize(12).TextColor(ui.Hex("#e8eef7")) })
	if b.Clicked() {
		fn()
	}
}

// bindKeys 注册视频窗的快捷键：Esc 关闭、空格播放/暂停、Ctrl/Cmd+W 最小化、
// Ctrl/Cmd+Q 退出 —— 与 webview 弹窗页面的 keydown listener 一一对应。
func (n *nativeWindow) bindKeys(c *ui.Context) {
	c.OnShortcut(0, ui.KeyEscape, func() { n.close() })
	c.OnShortcut(0, ui.KeySpace, func() {
		if c.ShortcutRepeated() {
			return
		}
		if n.pl.Playing() {
			n.pl.Pause()
		} else {
			n.pl.Play()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyW, func() {
		if cb := n.onMin; cb != nil {
			cb()
		}
	})
	c.OnShortcut(ui.Cmd, ui.KeyQ, func() {
		if cb := n.onQuit; cb != nil {
			go cb()
		}
	})
}

// nativeFailText 把原生路径的失败翻成一句给用户看的中文原因。
//
// 判据只有两类，都能在「用户看得懂」和「排查有用」之间取平衡：
//   - 缺媒体框架（ui/video 的 ErrUnsupported，Linux 缺 GStreamer 1.x /
//     gst-plugins-base 的 playbin+appsink，或系统版本太老）；
//   - 其它（解码器缺失、流断了、起播超时）。
func nativeFailText(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, video.ErrUnsupported) {
		return "系统缺少视频播放组件（Linux 需要 GStreamer 1.x + gst-plugins-base），已改用内置播放器"
	}
	return "原生视频播放失败（" + err.Error() + "），已改用内置播放器"
}

// actionFallbackText 是「连 webview 都开不起来」时的提示。
func actionFallbackText(err error) string {
	if err == nil {
		return "打开视频窗口失败"
	}
	return "打开视频窗口失败：" + err.Error()
}
