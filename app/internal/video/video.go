// Package video 管理视频弹窗：一个临时的 webview 窗口。
//
// 为什么视频要单独开 webview：H.264 没有可用的纯 Go 解码器（唯一像样的
// eyevinn/hi264 还很新），自己实现等于重写一个解码器。而浏览器引擎本来就
// 带着 H.264 硬解、系统原生控件、画中画、全屏 —— 这些自己写反而做不好。
//
// 为什么是「临时」窗口：一个 webview 窗口在 Windows 上带着约 330MB 的
// WebView2 基础开销（浏览器/GPU/网络/存储等进程）。实测：原生 UI 主窗
// 只有 57MB，打开媒体窗口升到 506MB，**关掉后回落到 69MB**。所以看完就关，
// 内存能收回来。
//
// 音频不在这里：音频走 internal/media 的纯 Go 引擎，不需要 webview。
package video

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

// Options 描述要打开的视频。
type Options struct {
	// Src 是播放地址，应当是本地代理 URL（webview 带不了 Referer）。
	Src string
	// Title 显示在弹窗标题栏。
	Title string
	// Poster 是封面（可选）。
	Poster string
	// StartAt 是续播位置（秒）。
	StartAt float64
	// Speed 是当前倍速，跟随主窗。
	Speed float64
	// Muted 表示主窗是静音状态。
	Muted bool
	// Parent 是主窗，用于让弹窗跟随。
	Parent *mygo.Window
}

// State 是弹窗回传的播放状态。
type State struct {
	Time     float64
	Duration float64
	Paused   bool
	Muted    bool
	Speed    float64
	Ready    int
	Err      string
}

// RelayOnClose 报告关掉视频窗之后要不要把音频引擎接回来接着听（音视接力）。
//
// 语义以原版为准（tauri 分支 src/components/playerVideo.tsx 的
// closePlayerVideo）：
//
//   - **暂停时关窗不续播**。原版注释写明「不要在此恢复音频播放（isPlaying
//     保持 false），由用户手动点击播放键继续收听，避免 macOS 上音频与视频
//     抢播」—— 暂停是用户的明确意图，关窗不是「开始播放」的手势。
//   - 播放中关窗才接回（本仓库 webview / 原生两条路径既有的音视接力）。
//   - 没起来（Ready==0）、出错、或者自然播完（位置停在片尾）都不接：这时
//     接回来只会得到一段静音、一段错误提示音，或者贴着片尾的一两声。
//
// webview 路径的 State 来自页面回传（play/pause 事件已经即时上报），原生
// 路径来自关窗那一刻的播放器，两者判据一致。
func RelayOnClose(st State) bool {
	if st.Paused || st.Ready == 0 || st.Err != "" {
		return false
	}
	// 自然播完：ended 之后页面不再每秒回传，缓存的 Paused 可能还是旧的
	// 「在播」，但位置已经在末尾。用与主窗「播完再点播放」同一条判据。
	if st.Duration > 0 && st.Time >= st.Duration-0.5 {
		return false
	}
	return true
}

// Manager 管理唯一一个视频弹窗。
type Manager struct {
	mu  sync.Mutex
	win *mygo.Window
	st  State
	// opening 表示一次 Open 正在进行（异步的窗口创建落定之前，
	// 再来的 Open 直接拒绝，避免快速连点开出多个弹窗）。
	opening bool

	onState func(State)
	onClose func(State)
	onEnded func()
	onQuit  func()

	// ended 记录页面是否已经自然播完。页面在 ended 后不再每秒回传状态，
	// 单看 Paused/Time 会误判；主窗再点播放时用它决定先 seek(0)。
	ended bool

	// minimize 是最小化弹窗自身的函数：Open 时绑定到窗口的 Minimize，
	// Close / 窗口关闭时清空。抽成字段便于单测验证 Service.Minimize 的接线
	// （测试里没有真实窗口，注入一个计数函数即可）。
	minimize func()

	// native 是当前的原生视频窗（ui/video）；nil 表示走 webview 那条路径。
	// 两条路径互斥：同一时刻只有一个弹窗，State/Seek/SetSpeed 等都要按它分派。
	native *nativeWindow

	// nativeOK 是「本机还能不能试原生路径」的闩锁：一旦判定不可用（缺
	// GStreamer / 系统没有媒体框架 / 起播失败），本次运行不再反复重试，
	// 直接走 webview，避免每次开视频都先黑一下再回退。
	nativeOK      bool
	nativeChecked bool

	// onFallback 在原生路径不可用、已经落回 webview 时调用（主窗据此 toast）。
	onFallback func(reason string)
}

// New 建一个管理器。
func New() *Manager { return &Manager{nativeOK: true} }

// SetNativeEnabled 开关原生视频路径（设置项「原生视频播放」）。
// 关掉后所有 Open 直接走 webview —— 给「GStreamer 装是装了但播得不好」或
// 需要弹幕覆盖层 / 画中画的用户一条明确的退路，而不是只能靠失败回退。
func (m *Manager) SetNativeEnabled(on bool) {
	m.mu.Lock()
	m.nativeOK, m.nativeChecked = on, true
	m.mu.Unlock()
}

// NativeEnabled 报告设置项当前是否允许走原生路径。
func (m *Manager) NativeEnabled() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nativeOK
}

// OnFallback 设置「原生视频不可用，已落回 webview」的回调。
// reason 是给用户看的一句话原因（例如「系统缺少 GStreamer」）。
func (m *Manager) OnFallback(fn func(reason string)) { m.onFallback = fn }

// NativeActive 报告当前弹窗是否走原生路径（供设置/状态展示与测试）。
func (m *Manager) NativeActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.native != nil
}

// OnState 设置状态回调（约每秒一次）。
func (m *Manager) OnState(fn func(State)) { m.onState = fn }

// OnClose 设置关闭回调。
func (m *Manager) OnClose(fn func(State)) { m.onClose = fn }

// OnEnded 设置播放结束回调。
func (m *Manager) OnEnded(fn func()) { m.onEnded = fn }

// OnQuit 设置退出回调（弹窗里按 Ctrl/Cmd+Q）。
func (m *Manager) OnQuit(fn func()) { m.onQuit = fn }

// IsOpen 报告弹窗是否开着。
func (m *Manager) IsOpen() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.win != nil || m.native != nil
}

// State 返回最近一次回传的状态。
func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.st
}

// Ended 报告弹窗是否已经自然播完。
func (m *Manager) Ended() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ended
}

// Replay 让弹窗从开头重新播放（播完后主窗再点播放）。
func (m *Manager) Replay() {
	m.mu.Lock()
	m.ended = false
	w, n := m.win, m.native
	m.mu.Unlock()
	if n != nil {
		n.apply(func() {
			n.endedFired = false
			n.pl.Seek(0)
			n.pl.Play()
		})
		return
	}
	if w != nil {
		w.Page().Eval("(()=>{const v=document.getElementById('v');if(v){try{v.currentTime=0}catch(_){ }const p=v.play();if(p&&p.catch)p.catch(()=>{});}})()")
	}
}

// pageURL 把打开选项编成弹窗页面的地址（/video.html?src=…&muted=…）。
// 抽成纯函数便于测试：弹窗的起播状态（位置 / 倍速 / 静音）全靠这组参数。
func pageURL(o Options) string {
	q := url.Values{}
	q.Set("src", o.Src)
	q.Set("title", o.Title)
	q.Set("poster", o.Poster)
	q.Set("t", fmt.Sprintf("%.2f", o.StartAt))
	q.Set("speed", fmt.Sprintf("%.2f", max(o.Speed, 0.5)))
	if o.Muted {
		q.Set("muted", "1")
	}
	return "/video.html?" + q.Encode()
}

// switchScript 生成「原地换集」的 JS：复用同一个弹窗窗口，只换 <video> 的
// 源与起播参数（页面里的 window.biliSwitch，见 frontend/video.html）。
// 抽成纯函数便于测试：换集漏掉任何一个参数都会「换集后位置/倍速/静音不对」。
func switchScript(o Options) string {
	payload, err := json.Marshal(map[string]any{
		"src":    o.Src,
		"title":  o.Title,
		"poster": o.Poster,
		"t":      o.StartAt,
		"speed":  max(o.Speed, 0.5),
		"muted":  o.Muted,
	})
	if err != nil { // map[string]any 里的值全是可序列化的，这里不会失败
		return "window.biliSwitch({})"
	}
	return "window.biliSwitch(" + string(payload) + ")"
}

// Open 打开（或复用）弹窗。已开着的弹窗**原地换集**，不再关掉重建。
// 有一次 Open 还在创建中时，后续的 Open 拒绝：快速连点不会开出多个。
func (m *Manager) Open(o Options) error {
	m.mu.Lock()
	if m.opening {
		m.mu.Unlock()
		return nil
	}
	m.opening = true
	w, n := m.win, m.native
	m.mu.Unlock()
	defer func() {
		m.mu.Lock()
		m.opening = false
		m.mu.Unlock()
	}()

	// 原生窗还开着：原地换集（换播放器、不换窗口）。原生窗没有浏览器进程
	// 那套开销，但也没有理由每次换集都重建窗口（窗口位置、尺寸会跳）。
	if n != nil {
		if err := m.switchNative(n, o); err == nil {
			return nil
		}
		m.CloseNative()
	}

	// 复用路径：弹窗还在，只换源。这是内存上最关键的一条 —— 见 switchTo。
	//
	// 换源失败（窗口正好在这一帧被用户关掉：OnClosed 还没跑完、IsDestroyed()
	// 仍是 false 的极短窗口）时**不吞掉**这一集，而是回落到下面的新建路径。
	// 此前是「复用就直接 return nil」——那一下 Eval 落空、下一集不播，用户看到
	// 的是「点下一集没反应」。
	if w != nil && !w.IsDestroyed() {
		if err := m.switchTo(w, o); err == nil {
			return nil
		}
	}

	m.Close()

	// 原生优先：ui/video 用系统媒体框架解码 + 出声，音画同步由框架保证，
	// 没有 WebView2 那套进程开销。判定不可用（缺媒体框架 / 起播失败）就
	// 落回 webview，并把原因告诉用户 —— 不能因为这条链路让人看不了视频。
	if reason, ok := m.tryNative(o); ok {
		return nil
	} else if reason != "" {
		m.reportFallback(reason)
	}

	return m.openWebView(o)
}

// openWebView 是原来的 webview 弹窗路径（回退用，也是唯一能播 webm / 弹幕
// 覆盖层 / 画中画的路径）。
func (m *Manager) openWebView(o Options) error {
	page := pageURL(o)

	w := mygo.NewWindow(mygo.WindowOptions{
		Title:     o.Title,
		URL:       page,
		Width:     960,
		Height:    600,
		MinWidth:  320,
		MinHeight: 200,
		Frameless: true,
		// mygo 0.6 的 UserAgentSuffix：让视频页在站点侧带上应用名/版本，
		// 同时保留 WebView2 自带的浏览器标识。
		Page: mygo.PageOptions{UserAgentSuffix: "bili-FM/" + mygo.App.Version()},
	})
	if w == nil {
		return fmt.Errorf("video: 创建弹窗失败")
	}

	m.mu.Lock()
	m.win, m.native, m.minimize = w, nil, w.Minimize
	m.st = State{Speed: o.Speed}
	m.mu.Unlock()

	w.OnClosed(func() {
		m.mu.Lock()
		if m.win == w {
			m.win, m.minimize = nil, nil
		}
		st := m.st
		cb := m.onClose
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	})
	return nil
}

// tryNative 尝试用原生路径打开 o。ok 表示「原生窗已经开起来了」；
// 否则 reason 非空时表示「试过但不可用」，调用方据此 toast + 走 webview。
//
// 判定顺序（每一步都能省掉一次黑窗）：
//  1. 上次已判定不可用（nativeChecked && !nativeOK）→ 直接跳过，不返回 reason
//     （用户已经被告知过一次，不必每次开视频都弹）。
//  2. 地址不是 http(s)/file → 跳过（不是故障，不 toast）。
//  3. ui/video.OpenURL 返回 ErrUnsupported → 记闩锁 + 给出原因。
//  4. 播放器起来了，但起播失败（Err 非空 / 一直不 Ready）→ 由 onFailed 关窗
//     回退。这里不阻塞等待，避免开窗卡住界面。
func (m *Manager) tryNative(o Options) (reason string, ok bool) {
	m.mu.Lock()
	ok = m.nativeOK
	checked := m.nativeChecked
	m.mu.Unlock()
	if checked && !ok {
		return "", false
	}
	if !nativeEligible(o) {
		return "", false
	}

	pl, err := openPlayer(o.Src)
	if err != nil {
		m.mu.Lock()
		m.nativeOK, m.nativeChecked = false, true
		m.mu.Unlock()
		return nativeFailText(err), false
	}

	n := newNativeWindow(o, pl)
	if n == nil {
		m.mu.Lock()
		m.nativeOK, m.nativeChecked = false, true
		m.mu.Unlock()
		return "创建视频窗口失败", false
	}
	n.onState = func(st State) {
		m.mu.Lock()
		m.st = st
		cb := m.onState
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	}
	n.onClose = func(st State) {
		m.mu.Lock()
		if m.native == n {
			m.native = nil
		}
		cb := m.onClose
		m.mu.Unlock()
		if cb != nil {
			cb(st)
		}
	}
	n.onEnded = func() {
		m.mu.Lock()
		m.ended = true
		cb := m.onEnded
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
	n.onMin = func() {
		m.mu.Lock()
		fn := m.minimize
		m.mu.Unlock()
		if fn != nil {
			fn()
		}
	}
	n.onQuit = func() {
		m.mu.Lock()
		cb := m.onQuit
		m.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
	// 起播失败（解码器缺失 / 流断了）：关掉原生窗，落回 webview 播同一集。
	//
	// 这段回调是在视图构建（主线程）里被 n.report 触发的，而关窗 + 建窗都会
	// 再进主线程消息循环 —— 直接做会嵌套。丢到 goroutine 上，顺序仍然确定
	// （先关后建）。
	n.onFailed = func(err error) {
		m.mu.Lock()
		m.nativeOK, m.nativeChecked = false, true
		m.mu.Unlock()
		reason := nativeFailText(err)
		n.suppressClose = true
		go func() {
			m.CloseNative()
			m.reportFallback(reason)
			if openErr := m.openWebView(o); openErr != nil {
				// webview 也开不起来：这是真的失败，交给主窗提示。
				m.mu.Lock()
				cb := m.onFallback
				m.mu.Unlock()
				if cb != nil {
					cb(actionFallbackText(openErr))
				}
			}
		}()
	}

	m.mu.Lock()
	m.native, m.win, m.minimize = n, nil, n.minimize
	m.st = State{Speed: n.speed}
	m.ended = false
	m.mu.Unlock()

	// 起播参数：位置 / 倍速 / 静音全部在回调接好之后下发一次（播放器默认从
	// 0 起播、1 倍速、不静音）。放在最后，保证这一帧的状态回传有人接。
	n.win.Update(func() {
		pl.SetRate(n.speed)
		pl.SetMuted(n.muted)
		pl.SetVolume(n.volume)
		if o.StartAt > 0 {
			pl.Seek(time.Duration(o.StartAt * float64(time.Second)))
		}
		n.reported = time.Time{} // 下一帧立刻回传一次
	})
	return "", true
}

// switchNative 在已开着的原生窗里原地换集。
func (m *Manager) switchNative(n *nativeWindow, o Options) error {
	if n.win == nil || n.win.IsDestroyed() {
		return fmt.Errorf("video: 原生视频窗已消失")
	}
	pl, err := openPlayer(o.Src)
	if err != nil {
		return err
	}
	n.win.Update(func() {
		old := n.pl
		n.pl = pl
		n.title, n.startAt = o.Title, o.StartAt
		n.speed = max(o.Speed, 0.5)
		n.muted = o.Muted
		n.endedFired = false
		n.reported = time.Time{}
		pl.SetRate(n.speed)
		pl.SetMuted(n.muted)
		pl.SetVolume(n.volume)
		if o.StartAt > 0 {
			pl.Seek(time.Duration(o.StartAt * float64(time.Second)))
		}
		if old != nil {
			old.Close()
		}
	})
	n.win.SetTitle(o.Title)
	n.focus()
	m.mu.Lock()
	m.st = State{Speed: o.Speed}
	m.ended = false
	m.mu.Unlock()
	return nil
}

// CloseNative 关掉原生窗（若有）。
func (m *Manager) CloseNative() {
	m.mu.Lock()
	n := m.native
	m.native, m.minimize = nil, nil
	m.ended = false
	m.mu.Unlock()
	if n != nil {
		n.close()
	}
}

// reportFallback 告诉主窗「原生不可用，已落回 webview」。
func (m *Manager) reportFallback(reason string) {
	m.mu.Lock()
	cb := m.onFallback
	m.mu.Unlock()
	if cb != nil {
		cb(reason)
	}
}

// switchTo 在已开着的弹窗里原地换集（不新建 webview 窗口）。
//
// 为什么必须复用：每新建一个弹窗就要起一套 WebView2 渲染进程，换集时
// 「旧窗口正在拆、新窗口正在建」会同时驻留两份，用户实测「切几个视频 +
// 翻几个抽屉后内存 315MB」里有一份就是这种叠加。原地换集只改 <video> 的
// src，进程、页面、解码器初始化都留着，切集的内存曲线是平的。
//
// 返回 error 表示「这一下没换成」（窗口已死 / 页面还没就绪）：调用方据此回落
// 到新建路径，而不是静默丢掉这一集。
func (m *Manager) switchTo(w *mygo.Window, o Options) error {
	p := w.Page()
	if p == nil {
		return fmt.Errorf("video: 弹窗页面已消失")
	}
	if _, err := p.Eval(switchScript(o)); err != nil {
		return err
	}
	w.SetTitle(o.Title)
	if w.IsMinimized() {
		w.Restore()
	}
	w.Show()
	w.Focus()
	m.mu.Lock()
	m.st = State{Speed: o.Speed}
	m.ended = false
	m.mu.Unlock()
	return nil
}

// Close 关闭弹窗（若开着）。两条路径都收：原生窗与 webview 互斥，但
// 回退过程中可能短暂同时存在（原生刚关、webview 正在建），所以都清。
func (m *Manager) Close() {
	m.mu.Lock()
	w, n := m.win, m.native
	m.win, m.native, m.minimize = nil, nil, nil
	m.ended = false
	m.mu.Unlock()
	if n != nil {
		n.close()
	}
	if w != nil {
		w.Close()
	}
}

// SetSpeed 同步倍速到弹窗（原生 / webview 两条路径都支持：原生走
// VideoPlayer.SetRate，webview 走 playbackRate）。
func (m *Manager) SetSpeed(v float64) {
	m.mu.Lock()
	w, n := m.win, m.native
	m.mu.Unlock()
	if n != nil {
		n.setRate(v)
		return
	}
	if w != nil {
		w.Page().Eval(fmt.Sprintf("(()=>{const v=document.getElementById('v');if(v)v.playbackRate=%f;})()", v))
	}
}

// Pause / Resume 供主窗控制弹窗（例如主窗按了空格）。
func (m *Manager) Pause() {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.pause()
		return
	}
	m.eval("document.getElementById('v')?.pause()")
}

func (m *Manager) Resume() {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.play()
		return
	}
	m.eval("document.getElementById('v')?.play()")
}

// Seek 让弹窗跳到指定秒数。主窗底部进度条在视频弹窗打开时也要控制视频，
// 而不是去 seek 已经暂停的音频引擎。
func (m *Manager) Seek(seconds float64) {
	if seconds < 0 {
		seconds = 0
	}
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.seek(seconds)
		return
	}
	m.eval(fmt.Sprintf("(()=>{const v=document.getElementById('v');if(v){v.currentTime=%f;}})()", seconds))
}

// SetMuted 同步静音（主窗静音时原生视频也要静音；webview 页面自己管）。
func (m *Manager) SetMuted(muted bool) {
	m.mu.Lock()
	n := m.native
	m.mu.Unlock()
	if n != nil {
		n.setMuted(muted)
	}
}

func (m *Manager) eval(js string) {
	m.mu.Lock()
	w := m.win
	m.mu.Unlock()
	if w != nil {
		w.Page().Eval(js)
	}
}

// ---------------------------------------------------------------- 页面 → Go

// Service 是暴露给弹窗页面的服务。
type Service struct{ m *Manager }

// NewService 建一个页面服务。
func NewService(m *Manager) *Service { return &Service{m: m} }

// Log 页面写日志。
func (s *Service) Log(msg string) { videoLog("弹窗 | " + msg) }

// Report 页面每秒回传状态。
func (s *Service) Report(t, dur float64, paused, muted bool, speed float64, ready int, err string) {
	s.m.mu.Lock()
	s.m.st = State{Time: t, Duration: dur, Paused: paused, Muted: muted, Speed: speed, Ready: ready, Err: err}
	cb := s.m.onState
	s.m.mu.Unlock()
	if cb != nil {
		cb(s.m.st)
	}
}

// Minimize 最小化视频弹窗（弹窗里按 Ctrl/Cmd+W）。最小化的是弹窗自身 ——
// 它是当前获得键盘焦点的窗口，与主窗 Ctrl/Cmd+W（最小化主窗）的语义一致，
// 都是「最小化当前窗口」。弹窗最小化后视频仍在播放（音频不停），可从
// 任务栏 / Dock 还原；不开窗时静默忽略。
func (s *Service) Minimize() {
	s.m.mu.Lock()
	fn := s.m.minimize
	s.m.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// Quit 退出整个应用（弹窗里按 Ctrl/Cmd+Q）。原版 index.tsx 的快捷键
// listener 在 `isPlayVideo` 分支之前执行，所以视频模式下 Ctrl/Cmd+Q 同样
// 生效；原生版的弹窗是独立窗口，主窗快捷键到不了这里，必须由弹窗页面接。
func (s *Service) Quit() {
	s.m.mu.Lock()
	cb := s.m.onQuit
	s.m.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

// Close 页面请求关闭弹窗。
func (s *Service) Close() {
	go func() {
		time.Sleep(20 * time.Millisecond) // 让这次调用先返回
		s.m.Close()
	}()
}

// Ended 页面报告播放结束。
func (s *Service) Ended() {
	s.m.mu.Lock()
	s.m.ended = true
	cb := s.m.onEnded
	s.m.mu.Unlock()
	if cb != nil {
		go cb()
	}
}

var videoLog = func(string) {}

// SetLog 注入日志函数（避免这个包依赖调用方的日志实现）。
func SetLog(fn func(string)) { videoLog = fn }

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
